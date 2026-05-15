/*
 * 监控模块
 * 负责定时扫描监控路径并执行清理操作
 *
 * 工作原理:
 *   1. 按照配置的间隔时间定时轮询
 *   2. 遍历所有监控路径
 *   3. 对每个路径执行清理操作
 *   4. 更新扫描状态
 */

package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"netdrive-clear/cleaner"
	"netdrive-clear/config"
	"netdrive-clear/scanner"
	"netdrive-clear/webdav"
)

// Monitor 监控器
type Monitor struct {
	manager    *webdav.Manager       // WebDAV 管理器
	cleaner    *cleaner.Cleaner      // 清理器
	scanner    *scanner.ScanState    // 扫描状态
	config     *config.MonitorConfig // 监控配置
	watchPaths []config.WatchPath    // 监控路径列表

	ctx     context.Context    // 上下文
	cancel  context.CancelFunc // 取消函数
	wg      sync.WaitGroup     // 等待组
	running bool               // 是否运行中
	mu      sync.RWMutex       // 并发锁

	onScanStart    func(path string)                              // 扫描开始回调
	onScanComplete func(path string, result *cleaner.CleanResult) // 扫描完成回调
	onError        func(err error)                                // 错误回调
}

// MonitorStats 监控统计信息
type MonitorStats struct {
	Running        bool      `json:"running"`         // 是否运行中
	LastScanTime   time.Time `json:"last_scan_time"`  // 最后扫描时间
	PathsMonitored int       `json:"paths_monitored"` // 监控路径数量
	TotalScans     int64     `json:"total_scans"`     // 总扫描次数
	TotalCleaned   int64     `json:"total_cleaned"`   // 总清理文件数
}

// NewMonitor 创建新的监控器
// manager: WebDAV 管理器
// cl: 清理器
// sc: 扫描状态
// cfg: 监控配置
// paths: 监控路径列表
// 返回监控器实例
func NewMonitor(manager *webdav.Manager, cl *cleaner.Cleaner, sc *scanner.ScanState, cfg *config.MonitorConfig, paths []config.WatchPath) *Monitor {
	return &Monitor{
		manager:    manager,
		cleaner:    cl,
		scanner:    sc,
		config:     cfg,
		watchPaths: paths,
	}
}

// Start 启动监控
// 返回错误信息
func (m *Monitor) Start() error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}

	// 创建可取消的上下文
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.running = true
	m.mu.Unlock()

	// 启动监控协程
	m.wg.Add(1)
	go m.run()

	return nil
}

// Stop 停止监控
func (m *Monitor) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	// 取消上下文，停止监控循环
	m.cancel()
	m.running = false
	m.mu.Unlock()

	// 等待协程结束
	m.wg.Wait()
}

// run 监控主循环
func (m *Monitor) run() {
	defer m.wg.Done()

	// 创建定时器
	ticker := time.NewTicker(time.Duration(m.config.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	// 立即执行一次扫描
	m.scanAll()

	// 定时扫描循环
	for {
		select {
		case <-m.ctx.Done():
			// 上下文被取消，退出循环
			return
		case <-ticker.C:
			// 定时触发扫描
			m.scanAll()
		}
	}
}

// scanAll 扫描所有监控路径
func (m *Monitor) scanAll() {
	// 检查 WebDAV 服务器是否可用
	if m.manager == nil {
		if m.onError != nil {
			m.onError(fmt.Errorf("没有可用的 WebDAV 管理器"))
		}
		return
	}
	
	// 检查是否有可用服务器
	servers := m.manager.ListServers()
	if len(servers) == 0 {
		if m.onError != nil {
			m.onError(fmt.Errorf("没有可用的 WebDAV 服务器，请先在 Web 界面配置"))
		}
		return
	}

	for _, wp := range m.watchPaths {
		if !wp.Enabled {
			// 跳过禁用的路径
			continue
		}

		// 触发扫描开始回调
		if m.onScanStart != nil {
			m.onScanStart(wp.Path)
		}

		// 执行清理
		result, err := m.cleaner.CleanPath(m.ctx, wp.Path)
		if err != nil {
			// 触发错误回调
			if m.onError != nil {
				m.onError(err)
			}
			continue
		}

		// 更新扫描状态
		m.scanner.MarkScanned(wp.Path, time.Now(), 0, 0)

		// 触发扫描完成回调
		if m.onScanComplete != nil {
			m.onScanComplete(wp.Path, result)
		}
	}
}

// GetStats 获取监控统计信息
// 返回统计信息
func (m *Monitor) GetStats() MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return MonitorStats{
		Running:        m.running,
		LastScanTime:   time.Now(),
		PathsMonitored: len(m.watchPaths),
		TotalScans:     m.cleaner.GetDeletedCount(),
	}
}

// IsRunning 检查监控是否运行中
// 返回是否运行中
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}
