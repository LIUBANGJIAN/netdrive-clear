/*
 * 简化的监控模块
 * 参考 app.py 的监控设计
 * 使用文件系统事件监听实现实时监控
 */

package monitor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"netdrive-clear/config"
	"netdrive-clear/log"
)

// SimpleMonitor 简化监控器
type SimpleMonitor struct {
	mu          sync.RWMutex
	watcher     *fsnotify.Watcher
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
	logger      *log.Logger
	config      *config.CleanerConfig
	watchPaths  []string
	onClean     func(path string, deleted bool, reason string)
}

// NewSimpleMonitor 创建简化监控器
func NewSimpleMonitor(cfg *config.CleanerConfig, logger *log.Logger) (*SimpleMonitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &SimpleMonitor{
		watcher:    watcher,
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		config:     cfg,
		watchPaths: make([]string, 0),
	}, nil
}

// Start 启动监控
func (m *SimpleMonitor) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	// 启动事件监听协程
	go m.run()

	if m.logger != nil {
		m.logger.Info("实时监控已启动")
	}
	return nil
}

// Stop 停止监控
func (m *SimpleMonitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.cancel()
	m.mu.Unlock()

	// 关闭监听器
	m.watcher.Close()

	if m.logger != nil {
		m.logger.Info("实时监控已停止")
	}
}

// run 事件监听循环
func (m *SimpleMonitor) run() {
	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			// 只处理文件创建事件
			if event.Has(fsnotify.Create) {
				// 等待文件写入完成
				time.Sleep(500 * time.Millisecond)
				m.handleFile(event.Name)
			}

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			if m.logger != nil {
				m.logger.Error("监控错误: %v", err)
			}
		case <-m.ctx.Done():
			return
		}
	}
}

// handleFile 处理文件事件
func (m *SimpleMonitor) handleFile(path string) {
	// 检查是否为目录
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		// 新目录，添加监控
		m.AddPath(path)
		return
	}

	// 检查是否需要删除
	if shouldDelete, reason := m.shouldDelete(path, info.Size()); shouldDelete {
		if err := os.Remove(path); err != nil {
			if m.logger != nil {
				m.logger.Error("删除失败 [%s]: %v", path, err)
			}
		} else {
			if m.logger != nil {
				m.logger.Success("删除成功 [%s]: %.2f MB, 原因: %s",
					path, float64(info.Size())/1024/1024, reason)
			}
			if m.onClean != nil {
				m.onClean(path, true, reason)
			}
		}
	}
}

// shouldDelete 判断文件是否应该删除
func (m *SimpleMonitor) shouldDelete(path string, size int64) (bool, string) {
	ext := strings.ToLower(filepath.Ext(path))

	// 检查广告文件
	for _, adExt := range m.config.AdExtensions {
		if ext == adExt {
			return true, "广告文件"
		}
	}

	// 检查小视频文件
	for _, videoExt := range m.config.VideoExtensions {
		if ext == videoExt && size > 0 {
			minSize := int64(m.config.MinVideoSizeMB) * 1024 * 1024
			if size < minSize {
				return true, fmt.Sprintf("小视频 (%.2f MB < %d MB)",
					float64(size)/1024/1024, m.config.MinVideoSizeMB)
			}
		}
	}

	return false, ""
}

// AddPath 添加监控路径
func (m *SimpleMonitor) AddPath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	for _, p := range m.watchPaths {
		if p == path {
			return nil
		}
	}

	// 添加监控
	if err := m.watcher.Add(path); err != nil {
		return err
	}

	// 递归添加子目录
	err := filepath.Walk(path, func(subPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return m.watcher.Add(subPath)
		}
		return nil
	})

	if err == nil {
		m.watchPaths = append(m.watchPaths, path)
		if m.logger != nil {
			m.logger.Info("添加监控路径: %s", path)
		}
	}

	return err
}

// RemovePath 移除监控路径
func (m *SimpleMonitor) RemovePath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.watcher.Remove(path); err != nil {
		return err
	}

	// 从列表中移除
	for i, p := range m.watchPaths {
		if p == path {
			m.watchPaths = append(m.watchPaths[:i], m.watchPaths[i+1:]...)
			break
		}
	}

	if m.logger != nil {
		m.logger.Info("移除监控路径: %s", path)
	}
	return nil
}

// SetOnClean 设置清理回调
func (m *SimpleMonitor) SetOnClean(callback func(path string, deleted bool, reason string)) {
	m.onClean = callback
}

// IsRunning 检查是否运行中
func (m *SimpleMonitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetWatchPaths 获取监控路径列表
func (m *SimpleMonitor) GetWatchPaths() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.watchPaths...)
}