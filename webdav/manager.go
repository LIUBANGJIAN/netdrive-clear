/*
 * WebDAV 管理器模块
 * 管理多个 WebDAV 服务器连接
 *
 * 功能特性:
 *   1. 多服务器管理
 *   2. 服务器切换
 *   3. 统一的文件操作接口
 */

package webdav

import (
	"context"
	"fmt"
	"sync"
)

// Manager WebDAV 管理器
type Manager struct {
	clients map[string]*Client // 服务器名称 -> 客户端
	current string             // 当前选中的服务器名称
	mu      sync.RWMutex       // 并发锁
}

// NewManager 创建新的 WebDAV 管理器
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
	}
}

// AddServer 添加 WebDAV 服务器
func (m *Manager) AddServer(name, server, username, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, err := NewClient(server, username, password)
	if err != nil {
		return err
	}

	m.clients[name] = client
	return nil
}

// RemoveServer 移除 WebDAV 服务器
func (m *Manager) RemoveServer(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.clients, name)
	if m.current == name {
		m.current = ""
	}
}

// SetCurrent 设置当前使用的服务器
func (m *Manager) SetCurrent(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.clients[name]; !ok {
		return fmt.Errorf("服务器不存在: %s", name)
	}

	m.current = name
	return nil
}

// GetCurrent 获取当前服务器名称
func (m *Manager) GetCurrent() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// GetClient 获取当前客户端
func (m *Manager) GetClient() *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current == "" {
		return nil
	}
	return m.clients[m.current]
}

// GetClientByName 根据名称获取客户端
func (m *Manager) GetClientByName(name string) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clients[name]
}

// ListServers 列出所有服务器
func (m *Manager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// ListFiles 列出目录下的文件
func (m *Manager) ListFiles(ctx context.Context, path string) ([]FileInfo, error) {
	client := m.GetClient()
	if client == nil {
		return nil, fmt.Errorf("没有可用的 WebDAV 服务器")
	}
	return client.ListFiles(ctx, path)
}

// ListFilesByServer 列出指定服务器目录下的文件
func (m *Manager) ListFilesByServer(ctx context.Context, serverName, path string) ([]FileInfo, error) {
	client := m.GetClientByName(serverName)
	if client == nil {
		return nil, fmt.Errorf("服务器不存在: %s", serverName)
	}
	return client.ListFiles(ctx, path)
}

// GetFingerprint 获取目录指纹（用于增量扫描）
// 递归收集所有子目录的修改时间，实现精确的增量扫描
// path: WebDAV 目录路径
// 返回指纹字符串
func (m *Manager) GetFingerprint(ctx context.Context, path string) (string, error) {
	client := m.GetClient()
	if client == nil {
		return "", fmt.Errorf("没有可用的 WebDAV 服务器")
	}

	// 递归收集文件数和最新修改时间
	var fileCount int64
	var maxModTime int64

	err := m.collectFingerprintData(ctx, client, path, &fileCount, &maxModTime)
	if err != nil {
		return "", err
	}

	// 指纹 = 文件数_最新修改时间
	return fmt.Sprintf("%d_%d", fileCount, maxModTime), nil
}

// SubFolderFingerprint 子文件夹指纹信息
type SubFolderFingerprint struct {
	Path        string // 子文件夹路径
	FolderTime  int64  // 文件夹本身的修改时间
	FileCount   int64  // 文件夹内文件总数
	MaxFileTime int64  // 文件夹内最新文件的修改时间
	Fingerprint string // 指纹字符串
}

// GetSubFolderFingerprints 获取监控目录下所有直接子文件夹的指纹
// 用于细粒度监控：每个子文件夹独立监控，只有变化的文件夹才扫描
// path: 监控目录路径（如 /BON_115网盘/BM）
// 返回子文件夹指纹映射表
func (m *Manager) GetSubFolderFingerprints(ctx context.Context, path string) (map[string]string, error) {
	client := m.GetClient()
	if client == nil {
		return nil, fmt.Errorf("没有可用的 WebDAV 服务器")
	}

	result := make(map[string]string)

	files, err := client.ListFiles(ctx, path)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if f.IsDir {
			// 只处理直接子文件夹，递归收集其文件信息
			var fileCount int64
			var maxFileTime int64

			// 递归收集该子文件夹内所有文件的统计信息
			m.collectFolderFileStats(ctx, client, f.FullPath, &fileCount, &maxFileTime)

			// 指纹 = 文件夹修改时间_文件数_文件夹内最新文件时间
			fingerprint := fmt.Sprintf("%d_%d_%d", f.ModifyTime.Unix(), fileCount, maxFileTime)
			result[f.FullPath] = fingerprint
		}
	}

	return result, nil
}

// GetAllFoldersFingerprints 获取监控目录下所有层级文件夹的指纹
// 用于多级嵌套场景：支持检测深层文件夹的变化（如 /BM/1234/567/）
// path: 监控目录路径（如 /BON_115网盘/BM）
// 返回所有文件夹指纹映射表
func (m *Manager) GetAllFoldersFingerprints(ctx context.Context, path string) (map[string]string, error) {
	client := m.GetClient()
	if client == nil {
		return nil, fmt.Errorf("没有可用的 WebDAV 服务器")
	}

	result := make(map[string]string)

	// 递归遍历所有层级
	err := m.collectAllFoldersFingerprints(ctx, client, path, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// collectAllFoldersFingerprints 递归收集所有层级文件夹的指纹
func (m *Manager) collectAllFoldersFingerprints(ctx context.Context, client *Client, path string, result *map[string]string) error {
	files, err := client.ListFiles(ctx, path)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir {
			// 收集该文件夹的指纹
			var fileCount int64
			var maxFileTime int64

			// 递归收集该文件夹内所有文件的统计信息
			m.collectFolderFileStats(ctx, client, f.FullPath, &fileCount, &maxFileTime)

			// 指纹 = 文件夹修改时间_文件数_文件夹内最新文件时间
			fingerprint := fmt.Sprintf("%d_%d_%d", f.ModifyTime.Unix(), fileCount, maxFileTime)
			(*result)[f.FullPath] = fingerprint

			// 递归处理子目录
			if err := m.collectAllFoldersFingerprints(ctx, client, f.FullPath, result); err != nil {
				continue
			}
		}
	}

	return nil
}

// collectFolderFileStats 递归收集文件夹内所有文件的统计信息
func (m *Manager) collectFolderFileStats(ctx context.Context, client *Client, path string, fileCount *int64, maxFileTime *int64) error {
	files, err := client.ListFiles(ctx, path)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir {
			// 递归处理子目录
			if err := m.collectFolderFileStats(ctx, client, f.FullPath, fileCount, maxFileTime); err != nil {
				continue
			}
		} else {
			*fileCount++
			mt := f.ModifyTime.Unix()
			if mt > *maxFileTime {
				*maxFileTime = mt
			}
		}
	}

	return nil
}

// GetFolderFingerprints 获取指定服务器上目录下所有层级文件夹的指纹
// 用于增量扫描时跳过未变化的子目录
// serverName: 服务器名称
// path: 目录路径
// 返回文件夹指纹映射表
func (m *Manager) GetFolderFingerprints(ctx context.Context, serverName, path string) (map[string]string, error) {
	client := m.GetClientByName(serverName)
	if client == nil {
		return nil, fmt.Errorf("服务器不存在: %s", serverName)
	}

	result := make(map[string]string)

	// 递归遍历所有层级
	err := m.collectAllFoldersFingerprints(ctx, client, path, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// collectFingerprintData 递归收集指纹数据（文件数和最新修改时间）
func (m *Manager) collectFingerprintData(ctx context.Context, client *Client, path string, fileCount *int64, maxModTime *int64) error {
	files, err := client.ListFiles(ctx, path)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir {
			// 递归处理子目录
			if err := m.collectFingerprintData(ctx, client, f.FullPath, fileCount, maxModTime); err != nil {
				continue
			}
		} else {
			*fileCount++
			mt := f.ModifyTime.Unix()
			if mt > *maxModTime {
				*maxModTime = mt
			}
		}
	}

	return nil
}

// DeleteFile 删除文件（使用当前服务器）
func (m *Manager) DeleteFile(ctx context.Context, path string) error {
	client := m.GetClient()
	if client == nil {
		return fmt.Errorf("没有可用的 WebDAV 服务器")
	}
	return client.DeleteFile(ctx, path)
}

// DeleteFileByServer 删除指定服务器上的文件
func (m *Manager) DeleteFileByServer(ctx context.Context, serverName, path string) error {
	client := m.GetClientByName(serverName)
	if client == nil {
		return fmt.Errorf("服务器不存在: %s", serverName)
	}
	return client.DeleteFile(ctx, path)
}

// TestConnection 测试当前服务器连接
func (m *Manager) TestConnection(ctx context.Context) error {
	client := m.GetClient()
	if client == nil {
		return fmt.Errorf("没有可用的 WebDAV 服务器")
	}
	return client.TestConnection(ctx)
}

// TestServerConnection 测试指定服务器连接
func (m *Manager) TestServerConnection(ctx context.Context, name string) error {
	client := m.GetClientByName(name)
	if client == nil {
		return fmt.Errorf("服务器不存在: %s", name)
	}
	return client.TestConnection(ctx)
}

// GetWebDAVFileSystem 获取当前服务器的 WebDAV FileSystem
func (m *Manager) GetWebDAVFileSystem() interface{} {
	client := m.GetClient()
	if client == nil {
		return nil
	}
	return client.WebDAVFileSystem()
}

// GetServerInfo 获取服务器信息
type ServerInfo struct {
	Name      string `json:"name"`
	Server    string `json:"server"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Enabled   bool   `json:"enabled"`
	HasClient bool   `json:"has_client"`
}

// GetServersInfo 获取所有服务器信息
func (m *Manager) GetServersInfo() []ServerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]ServerInfo, 0, len(m.clients))
	for name, client := range m.clients {
		infos = append(infos, ServerInfo{
			Name:      name,
			Server:    client.server,
			Username:  client.username,
			Password:  client.password,
			Enabled:   true,
			HasClient: true,
		})
	}
	return infos
}
