/*
 * CloudDrive2 gRPC 客户端模块
 * 封装 CD2 的 gRPC API 调用，提供简洁的接口
 */

package cd2

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "netdrive-clear/cd2/pb/cd2"
)

// Client CloudDrive2 客户端
type Client struct {
	conn   *grpc.ClientConn           // gRPC 连接
	client pb.CloudDriveFileSrvClient // gRPC 客户端接口
	token  string                     // API 令牌
	server string                     // 服务器地址
}

// FileInfo 文件信息结构
type FileInfo struct {
	Name       string    // 文件名
	FullPath   string    // 完整路径
	Size       int64     // 文件大小（字节）
	IsDir      bool      // 是否为目录
	FileType   string    // 文件类型
	CreateTime time.Time // 创建时间
	ModifyTime time.Time // 修改时间
}

// ScanResult 扫描结果结构
type ScanResult struct {
	Files []FileInfo // 文件列表
	Error error      // 错误信息
}

// NewClient 创建新的 CD2 客户端
// server: CD2 服务器地址，如 localhost:19798
// token: API 令牌
// 返回客户端实例和错误信息
func NewClient(server, token string) (*Client, error) {
	// 移除 URL 前缀
	server = strings.TrimPrefix(server, "http://")
	server = strings.TrimPrefix(server, "https://")

	// 建立 gRPC 连接（不安全模式）
	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("连接CD2失败: %w", err)
	}

	return &Client{
		conn:   conn,
		client: pb.NewCloudDriveFileSrvClient(conn),
		token:  token,
		server: server,
	}, nil
}

// Close 关闭客户端连接
func (c *Client) Close() error {
	return c.conn.Close()
}

// withAuth 为上下文添加授权信息
// ctx: 上下文
// 返回添加了授权头的上下文
func (c *Client) withAuth(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}
	// 添加 Authorization 头
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("Authorization", "Bearer "+c.token))
}

// GetSystemInfo 获取系统信息
// ctx: 上下文
// 返回系统就绪状态、用户名、产品名称和错误信息
func (c *Client) GetSystemInfo(ctx context.Context) (bool, string, string, error) {
	resp, err := c.client.GetSystemInfo(c.withAuth(ctx), &emptypb.Empty{})
	if err != nil {
		return false, "", "", err
	}
	return resp.SystemReady, resp.UserName, resp.UserName, nil
}

// SetToken 设置 API 令牌
// token: 新的令牌
func (c *Client) SetToken(token string) {
	c.token = token
}

// Reconnect 重新连接到服务器
// server: 新的服务器地址
// token: 新的令牌
// 返回错误信息
func (c *Client) Reconnect(server, token string) error {
	// 关闭旧连接
	if c.conn != nil {
		c.conn.Close()
	}

	// 移除 URL 前缀
	server = strings.TrimPrefix(server, "http://")
	server = strings.TrimPrefix(server, "https://")

	// 建立新连接
	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("重新连接CD2失败: %w", err)
	}

	c.conn = conn
	c.client = pb.NewCloudDriveFileSrvClient(conn)
	c.token = token
	c.server = server

	return nil
}

// GetToken 通过用户名密码获取令牌
// ctx: 上下文
// username: 用户名
// password: 密码
// 返回令牌和错误信息
func (c *Client) GetToken(ctx context.Context, username, password string) (string, error) {
	resp, err := c.client.GetToken(ctx, &pb.GetTokenRequest{
		UserName: username,
		Password: password,
	})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("获取令牌失败: %s", resp.ErrorMessage)
	}
	c.token = resp.Token
	return resp.Token, nil
}

// GetSubFiles 获取目录下的子文件
// ctx: 上下文
// path: 目录路径
// forceRefresh: 是否强制刷新缓存
// 返回文件列表和错误信息
func (c *Client) GetSubFiles(ctx context.Context, path string, forceRefresh bool) ([]FileInfo, error) {
	req := &pb.ListSubFileRequest{
		Path:         path,
		ForceRefresh: forceRefresh,
	}

	// 调用流式 API
	stream, err := c.client.GetSubFiles(c.withAuth(ctx), req)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for {
		resp, err := stream.Recv()
		if err != nil {
			// 检查是否是正常的流结束 (io.EOF)
			if err == io.EOF {
				break
			}
			// 返回错误
			return files, err
		}
		if resp != nil && resp.SubFiles != nil {
			for _, f := range resp.SubFiles {
				files = append(files, convertFile(f))
			}
		}
	}

	return files, nil
}

// DeleteFile 删除单个文件
// ctx: 上下文
// path: 文件路径
// 返回错误信息
func (c *Client) DeleteFile(ctx context.Context, path string) error {
	_, err := c.client.DeleteFile(c.withAuth(ctx), &pb.FileRequest{Path: path})
	return err
}

// DeleteFiles 批量删除文件
// ctx: 上下文
// paths: 文件路径列表
// 返回错误信息
func (c *Client) DeleteFiles(ctx context.Context, paths []string) error {
	_, err := c.client.DeleteFiles(c.withAuth(ctx), &pb.MultiFileRequest{Paths: paths})
	return err
}

// FindFileByPath 根据路径查找文件信息
// ctx: 上下文
// path: 文件路径
// 返回文件信息和错误信息
func (c *Client) FindFileByPath(ctx context.Context, path string) (*FileInfo, error) {
	resp, err := c.client.FindFileByPath(c.withAuth(ctx), &pb.FindFileByPathRequest{Path: path})
	if err != nil {
		return nil, err
	}
	info := convertFile(resp)
	return &info, nil
}

// RuntimeInfo 运行时信息结构
type RuntimeInfo struct {
	Version   string
	BuildTime string
}

// GetRuntimeInfo 获取运行时信息
// ctx: 上下文
// 返回运行时信息和错误信息
func (c *Client) GetRuntimeInfo(ctx context.Context) (*RuntimeInfo, error) {
	resp, err := c.client.GetRuntimeInfo(c.withAuth(ctx), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return &RuntimeInfo{
		Version:   resp.Version,
		BuildTime: resp.BuildTime,
	}, nil
}

// MountPoint 挂载点信息
type MountPoint struct {
	CloudID     string
	CloudName   string
	MountPath   string
	DriveLetter string
	IsMounted   bool
}

// GetMountPoints 获取所有挂载点
// ctx: 上下文
// 返回挂载点列表和错误信息
func (c *Client) GetMountPoints(ctx context.Context) ([]MountPoint, error) {
	resp, err := c.client.GetMountPoints(c.withAuth(ctx), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	var mountPoints []MountPoint
	for _, mp := range resp.MountPoints {
		mountPoints = append(mountPoints, MountPoint{
			CloudID:     mp.CloudId,
			CloudName:   mp.CloudName,
			MountPath:   mp.MountPath,
			DriveLetter: mp.DriveLetter,
			IsMounted:   mp.IsMounted,
		})
	}
	return mountPoints, nil
}
func convertFile(f *pb.CloudDriveFile) FileInfo {
	info := FileInfo{
		Name:     f.Name,
		FullPath: f.FullPath,
		Size:     f.Size,
		IsDir:    f.IsDirectory,
		FileType: f.FileType,
	}

	if f.CreateTime != nil {
		info.CreateTime = f.CreateTime.AsTime()
	}
	if f.ModifyTime != nil {
		info.ModifyTime = f.ModifyTime.AsTime()
	}

	return info
}
