/*
 * WebDAV 客户端模块
 * 通过 WebDAV 协议连接 CloudDrive2
 * 使用第三方库 github.com/studio-b12/gowebdav
 *
 * 功能特性:
 *   1. 支持 HTTP 和 HTTPS
 *   2. 支持 Basic/Digest 认证
 *   3. 支持多服务器配置
 *   4. 目录遍历和文件操作
 */

package webdav

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/studio-b12/gowebdav"
)

// Client WebDAV 客户端
type Client struct {
	server     string           // 服务器地址
	username   string           // 用户名
	password   string           // 密码
	httpClient *http.Client     // HTTP 客户端
	gowebdav   *gowebdav.Client // 第三方 WebDAV 客户端
}

// FileInfo 文件信息结构
type FileInfo struct {
	Name       string    // 文件名
	FullPath   string    // 完整路径
	Size       int64     // 文件大小
	IsDir      bool      // 是否为目录
	ModifyTime time.Time // 修改时间
}

// Config WebDAV 配置
type Config struct {
	Server   string `json:"server"`   // 服务器地址，如 http://localhost:19798/webdav
	Username string `json:"username"` // 用户名
	Password string `json:"password"` // 密码
}

// NewClient 创建新的 WebDAV 客户端
func NewClient(server, username, password string) (*Client, error) {
	// 创建 HTTP 传输层
	transport := &http.Transport{
		MaxIdleConns:      10,
		IdleConnTimeout:   30 * time.Second,
		DisableKeepAlives: false,
	}

	// 创建 gowebdav 客户端
	gowebdavClient := gowebdav.NewClient(server, username, password)
	gowebdavClient.SetTransport(transport)
	gowebdavClient.SetTimeout(30 * time.Second)

	return &Client{
		server:     server,
		username:   username,
		password:   password,
		httpClient: &http.Client{Transport: transport, Timeout: 30 * time.Second},
		gowebdav:   gowebdavClient,
	}, nil
}

// Clone 创建客户端副本
func (c *Client) Clone() *Client {
	return &Client{
		server:     c.server,
		username:   c.username,
		password:   c.password,
		httpClient: c.httpClient,
		gowebdav:   c.gowebdav,
	}
}

// ListFiles 列出目录下的所有文件和子目录
func (c *Client) ListFiles(ctx context.Context, path string) ([]FileInfo, error) {
	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 使用 gowebdav 客户端列出目录
	files, err := c.gowebdav.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var result []FileInfo
	for _, f := range files {
		// 跳过当前目录和上级目录
		if f.Name() == "." || f.Name() == ".." {
			continue
		}

		fullPath := path
		if fullPath == "/" {
			fullPath = "/" + f.Name()
		} else {
			fullPath = path + "/" + f.Name()
		}

		result = append(result, FileInfo{
			Name:       f.Name(),
			FullPath:   fullPath,
			Size:       f.Size(),
			IsDir:      f.IsDir(),
			ModifyTime: f.ModTime(),
		})
	}

	return result, nil
}

// DeleteFile 删除文件
func (c *Client) DeleteFile(ctx context.Context, path string) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return c.gowebdav.Remove(path)
}

// Mkdir 创建目录
func (c *Client) Mkdir(ctx context.Context, path string) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return c.gowebdav.MkdirAll(path, 0755)
}

// GetFile 获取文件信息
func (c *Client) GetFile(ctx context.Context, path string) (*FileInfo, error) {
	files, err := c.ListFiles(ctx, filepath.Dir(path))
	if err != nil {
		return nil, err
	}

	name := filepath.Base(path)
	for _, f := range files {
		if f.Name == name {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("文件不存在: %s", path)
}

// TestConnection 测试连接
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.ListFiles(ctx, "/")
	return err
}

// WebDAVFileSystem 创建 webdav.FileSystem（保留接口兼容）
func (c *Client) WebDAVFileSystem() interface{} {
	return &webdavClient{client: c}
}

// webdavClient 实现 webdav.FileSystem 接口（保留接口兼容）
type webdavClient struct {
	client *Client
}

func (w *webdavClient) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return w.client.Mkdir(ctx, name)
}

func (w *webdavClient) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (interface {
	Close() error
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Seek(int64, int) (int64, error)
	Stat() (os.FileInfo, error)
}, error) {
	return nil, os.ErrPermission
}

func (w *webdavClient) RemoveAll(ctx context.Context, name string) error {
	return w.client.DeleteFile(ctx, name)
}

func (w *webdavClient) Rename(ctx context.Context, oldName, newName string) error {
	return os.ErrPermission
}

func (w *webdavClient) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	files, err := w.client.ListFiles(ctx, filepath.Dir(name))
	if err != nil {
		return nil, err
	}

	base := filepath.Base(name)
	for _, f := range files {
		if f.Name == base {
			return &fileInfo{
				name:    f.Name,
				size:    f.Size,
				isDir:   f.IsDir,
				modTime: f.ModifyTime,
			}, nil
		}
	}

	return nil, os.ErrNotExist
}

// fileInfo 实现 os.FileInfo 接口
type fileInfo struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
}

func (f *fileInfo) Name() string { return f.name }
func (f *fileInfo) Size() int64  { return f.size }
func (f *fileInfo) IsDir() bool  { return f.isDir }
func (f *fileInfo) Mode() os.FileMode {
	if f.isDir {
		return os.ModeDir | 0555
	}
	return 0444
}
func (f *fileInfo) ModTime() time.Time { return f.modTime }
func (f *fileInfo) Sys() interface{}   { return nil }
