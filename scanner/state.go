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
	"fmt"
	"os"
	"sync"
	"time"
)

// ScanState 扫描状态管理器
type ScanState struct {
	mu                    sync.RWMutex                 // 读写锁
	PathStates            map[string]*PathState        `json:"path_states"`             // 路径状态映射
	SubFolderFingerprints map[string]map[string]string `json:"sub_folder_fingerprints"` // 子文件夹指纹映射表: 监控路径 -> 子文件夹路径 -> 指纹
	stateFile             string                       // 状态保存文件路径
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
		PathStates:            make(map[string]*PathState),
		SubFolderFingerprints: make(map[string]map[string]string),
		stateFile:             stateFile,
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

// NeedsScan 判断路径是否需要扫描（使用指纹比较）
// path: 路径
// currentFingerprint: 当前指纹 (格式: "文件数_目录修改时间戳")
// 返回是否需要扫描
func (ss *ScanState) NeedsScan(path string, currentFingerprint string) bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	state, exists := ss.PathStates[path]
	if !exists {
		// 从未扫描过，需要扫描
		return true
	}

	// 指纹比较：如果指纹没变，说明目录没有变化
	return state.HasChanged || state.FileCount != ss.parseFingerprintCount(currentFingerprint)
}

// parseFingerprintCount 解析指纹中的文件数
// fingerprint: 指纹 (格式: "文件数_目录修改时间戳")
// 返回文件数
func (ss *ScanState) parseFingerprintCount(fingerprint string) int {
	var count int
	_, err := fmt.Sscanf(fingerprint, "%d", &count)
	if err != nil {
		return 0
	}
	return count
}

// GetFingerprint 生成目录指纹
// path: 目录路径
// 返回指纹字符串 (格式: "文件数_目录修改时间戳")
func (ss *ScanState) GetFingerprint(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d_%d", len(files), info.ModTime().Unix()), nil
}

// UpdateFingerprint 更新目录指纹
// path: 目录路径
// fingerprint: 新的指纹
func (ss *ScanState) UpdateFingerprint(path string, fingerprint string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	state, exists := ss.PathStates[path]
	if !exists {
		state = &PathState{}
		ss.PathStates[path] = state
	}

	state.FileCount = ss.parseFingerprintCount(fingerprint)
	state.HasChanged = false
	ss.Save()
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

// GetChangedSubFolders 获取监控目录下所有指纹发生变化的子文件夹
// monitorPath: 监控路径（如 /BON_115网盘/BM）
// currentFingerprints: 当前各子文件夹的指纹
// 返回发生变化且需要扫描的子文件夹列表
func (ss *ScanState) GetChangedSubFolders(monitorPath string, currentFingerprints map[string]string) []string {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	var changed []string

	// 获取该监控路径的已有指纹
	storedFingerprints, exists := ss.SubFolderFingerprints[monitorPath]
	if !exists {
		// 首次扫描，所有子文件夹都需要扫描
		for path := range currentFingerprints {
			changed = append(changed, path)
		}
		return changed
	}

	// 比较每个子文件夹的指纹
	for subPath, currentFP := range currentFingerprints {
		storedFP, wasTracked := storedFingerprints[subPath]
		if !wasTracked || storedFP != currentFP {
			// 新增的子文件夹或指纹发生变化
			changed = append(changed, subPath)
		}
	}

	return changed
}

// UpdateSubFolderFingerprints 更新监控路径下所有子文件夹的指纹
// monitorPath: 监控路径
// fingerprints: 子文件夹指纹映射表
func (ss *ScanState) UpdateSubFolderFingerprints(monitorPath string, fingerprints map[string]string) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.SubFolderFingerprints == nil {
		ss.SubFolderFingerprints = make(map[string]map[string]string)
	}

	ss.SubFolderFingerprints[monitorPath] = fingerprints
	ss.Save()
}

// GetSubFolderFingerprint 获取指定子文件夹的指纹
// monitorPath: 监控路径
// subFolderPath: 子文件夹路径
// 返回指纹字符串，如果不存在返回空字符串
func (ss *ScanState) GetSubFolderFingerprint(monitorPath, subFolderPath string) string {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if fingerprints, exists := ss.SubFolderFingerprints[monitorPath]; exists {
		return fingerprints[subFolderPath]
	}
	return ""
}
