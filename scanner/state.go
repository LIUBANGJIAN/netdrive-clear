/*
 * 扫描状态管理模块
 * 负责管理增量扫描的状态，避免重复处理未变动的目录
 *
 * 增量扫描原理:
 *   1. 记录每个目录的最后扫描时间和最后修改时间
 *   2. 下次扫描时比较当前修改时间与记录的修改时间
 *   3. 如果修改时间未变化，则跳过该目录
 */

package scanner

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// ScanState 扫描状态管理器
type ScanState struct {
	mu         sync.RWMutex          // 读写锁
	PathStates map[string]*PathState `json:"path_states"` // 路径状态映射
	stateFile  string                // 状态保存文件路径
}

// PathState 路径状态
type PathState struct {
	LastScanTime   time.Time `json:"last_scan_time"`   // 最后扫描时间
	LastModifyTime time.Time `json:"last_modify_time"` // 最后修改时间（目录）
	HasChanged     bool      `json:"has_changed"`      // 是否有变化
	FileCount      int       `json:"file_count"`       // 文件数量
	DirectoryCount int       `json:"directory_count"`  // 目录数量
}

// NewScanState 创建新的扫描状态管理器
// stateFile: 状态保存文件路径
// 返回扫描状态管理器实例
func NewScanState(stateFile string) *ScanState {
	ss := &ScanState{
		PathStates: make(map[string]*PathState),
		stateFile:  stateFile,
	}
	// 加载已保存的状态
	ss.Load()
	return ss
}

// Load 从文件加载扫描状态
// 返回错误信息
func (ss *ScanState) Load() error {
	data, err := os.ReadFile(ss.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空状态
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &ss.PathStates)
}

// Save 将扫描状态保存到文件
// 返回错误信息
func (ss *ScanState) Save() error {
	data, err := json.MarshalIndent(ss.PathStates, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ss.stateFile, data, 0644)
}

// MarkScanned 标记路径已扫描
// path: 路径
// modifyTime: 当前修改时间
// fileCount: 文件数量
// dirCount: 目录数量
func (ss *ScanState) MarkScanned(path string, modifyTime time.Time, fileCount, dirCount int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	state, exists := ss.PathStates[path]
	if !exists {
		state = &PathState{}
		ss.PathStates[path] = state
	}

	state.LastScanTime = time.Now()
	state.LastModifyTime = modifyTime
	state.HasChanged = false
	state.FileCount = fileCount
	state.DirectoryCount = dirCount

	// 保存到文件
	ss.Save()
}

// NeedsScan 判断路径是否需要扫描
// path: 路径
// currentModifyTime: 当前修改时间
// 返回是否需要扫描
func (ss *ScanState) NeedsScan(path string, currentModifyTime time.Time) bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	state, exists := ss.PathStates[path]
	if !exists {
		// 从未扫描过，需要扫描
		return true
	}

	// 如果当前修改时间晚于记录的修改时间，说明有变化
	if currentModifyTime.After(state.LastModifyTime) {
		return true
	}

	// 如果标记为有变化，也需要扫描
	return state.HasChanged
}

// MarkChanged 标记路径有变化
// path: 路径
func (ss *ScanState) MarkChanged(path string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	state, exists := ss.PathStates[path]
	if !exists {
		state = &PathState{}
		ss.PathStates[path] = state
	}

	state.HasChanged = true
	ss.Save()
}

// GetState 获取路径状态
// path: 路径
// 返回路径状态
func (ss *ScanState) GetState(path string) *PathState {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.PathStates[path]
}

// UpdateDirCache 更新目录缓存指纹
// 当监控发现变动时，同步更新该目录的缓存指纹
// dirPath: 目录路径
func (ss *ScanState) UpdateDirCache(dirPath string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// 检查目录是否存在
	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		return
	}

	// 获取目录下的文件列表
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	// 更新或创建路径状态
	state, exists := ss.PathStates[dirPath]
	if !exists {
		state = &PathState{}
		ss.PathStates[dirPath] = state
	}

	state.LastModifyTime = info.ModTime()
	state.FileCount = len(files)

	// 保存到文件
	ss.Save()
}
