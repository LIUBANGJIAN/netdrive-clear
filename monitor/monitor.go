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

// SetCallbacks 设置回调函数
// onScanStart: 扫描开始回调
// onScanComplete: 扫描完成回调
// onError: 错误回调
func (m *Monitor) SetCallbacks(onScanStart func(path string), onScanComplete func(path string, result *cleaner.CleanResult), onError func(err error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onScanStart = onScanStart
	m.onScanComplete = onScanComplete
	m.onError = onError
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

	isFirstScan := true

	// 监控模式：首次扫描立即执行，后续按配置的间隔持续检测
	if isFirstScan {
		isFirstScan = false
		if m.onScanStart != nil {
			m.onScanStart("[监控] 启动监控，首次扫描...")
		}
		// 首次扫描：获取所有子文件夹指纹并扫描
		changedFolders := m.quickCheck()
		if len(changedFolders) > 0 {
			m.monitorScan("[监控-首次]", changedFolders)
		}
	}

	// 持续监控循环
	for {
		m.mu.RLock()
		interval := m.config.IntervalSeconds
		m.mu.RUnlock()

		ticker := time.NewTicker(time.Duration(interval) * time.Second)

		if m.onScanStart != nil {
			m.onScanStart(fmt.Sprintf("[监控] 持续监控中 (检测间隔: %d秒)", interval))
		}

		select {
		case <-m.ctx.Done():
			ticker.Stop()
			if m.onError != nil {
				m.onError(fmt.Errorf("[监控] 监控已停止"))
			}
			return
		case <-ticker.C:
			ticker.Stop()
			// 定时检测：检查子文件夹指纹变化
			changedFolders := m.quickCheck()
			if len(changedFolders) > 0 {
				if m.onScanStart != nil {
					m.onScanStart(fmt.Sprintf("[监控] 检测到 %d 个子文件夹变化", len(changedFolders)))
				}
				m.monitorScan("[监控-变化]", changedFolders)
			}
		}
	}
}

// quickCheck 快速检查所有层级子文件夹指纹是否有变化
// 返回发生变化的子文件夹列表，空列表表示无变化
func (m *Monitor) quickCheck() map[string][]string {
	result := make(map[string][]string)

	for _, wp := range m.watchPaths {
		if !wp.Enabled {
			continue
		}

		// 获取监控目录下所有层级文件夹的指纹（支持多级嵌套）
		fingerprints, err := m.manager.GetAllFoldersFingerprints(m.ctx, wp.Path)
		if err != nil {
			continue
		}

		// 找出发生变化的子文件夹
		changed := m.scanner.GetChangedSubFolders(wp.Path, fingerprints)
		if len(changed) > 0 {
			result[wp.Path] = changed
		}
	}

	return result
}

// monitorScan 执行监控扫描（细粒度：只扫描变化的子文件夹）
func (m *Monitor) monitorScan(source string, changedFolders map[string][]string) {
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
			continue
		}

		changed, hasChanged := changedFolders[wp.Path]
		if !hasChanged || len(changed) == 0 {
			continue
		}

		// 获取监控目录下所有层级文件夹的当前指纹
		fingerprints, err := m.manager.GetAllFoldersFingerprints(m.ctx, wp.Path)
		if err != nil {
			if m.onError != nil {
				m.onError(fmt.Errorf("获取子文件夹指纹失败: %v", err))
			}
			continue
		}

		// 只扫描发生变化的子文件夹
		for _, subFolder := range changed {
			// 触发扫描开始回调
			if m.onScanStart != nil {
				m.onScanStart(fmt.Sprintf("%s 扫描子文件夹: %s", source, subFolder))
			}

			// 执行清理（只扫描该子文件夹）
			result, err := m.cleaner.CleanPath(m.ctx, subFolder)
			if err != nil {
				if m.onError != nil {
					m.onError(err)
				}
				continue
			}

			// 触发扫描完成回调
			if m.onScanComplete != nil {
				m.onScanComplete(subFolder, result)
			}
		}

		// 更新该监控路径下所有子文件夹的指纹
		m.scanner.UpdateSubFolderFingerprints(wp.Path, fingerprints)
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

// UpdatePaths 更新监控路径列表
// UpdatePaths 更新监控路径
func (m *Monitor) UpdatePaths(paths []config.WatchPath) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.watchPaths = paths
}

// UpdateConfig 更新监控配置（用于运行时动态更新）
func (m *Monitor) UpdateConfig(cfg *config.MonitorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}
