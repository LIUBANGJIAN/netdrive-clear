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
// path: WebDAV 目录路径
// 返回指纹字符串 (格式: "文件数_最后修改时间戳")
func (m *Manager) GetFingerprint(ctx context.Context, path string) (string, error) {
	client := m.GetClient()
	if client == nil {
		return "", fmt.Errorf("没有可用的 WebDAV 服务器")
	}

	files, err := client.ListFiles(ctx, path)
	if err != nil {
		return "", err
	}

	// 计算最新修改时间
	var latestModTime int64
	for _, f := range files {
		if !f.IsDir {
			mt := f.ModifyTime.Unix()
			if mt > latestModTime {
				latestModTime = mt
			}
		}
	}

	// 指纹 = 文件数_最新修改时间
	return fmt.Sprintf("%d_%d", len(files), latestModTime), nil
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
