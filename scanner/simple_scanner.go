/*
 * 简化的扫描器模块
 * 参考 app.py 的增量扫描设计
 * 使用目录指纹（文件数_修改时间）实现增量扫描
 */

package scanner

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"netdrive-clear/log"
)

// SimpleScanner 简化扫描器
type SimpleScanner struct {
	mu     sync.RWMutex
	cache  map[string]string // 路径 -> 指纹(文件数_修改时间戳)
	logger *log.Logger
}

// NewSimpleScanner 创建简化扫描器
func NewSimpleScanner(logger *log.Logger) *SimpleScanner {
	return &SimpleScanner{
		cache:  make(map[string]string),
		logger: logger,
	}
}

// GetFingerprint 生成目录指纹
func (s *SimpleScanner) GetFingerprint(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	
	files, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	
	return s.formatFingerprint(len(files), info.ModTime()), nil
}

// formatFingerprint 格式化指纹
func (s *SimpleScanner) formatFingerprint(fileCount int, mtime time.Time) string {
	return string(rune(fileCount)) + "_" + string(rune(mtime.Unix()))
}

// NeedsScan 判断是否需要扫描
func (s *SimpleScanner) NeedsScan(path string) (bool, error) {
	s.mu.RLock()
	oldFingerprint, exists := s.cache[path]
	s.mu.RUnlock()
	
	if !exists {
		return true, nil
	}
	
	newFingerprint, err := s.GetFingerprint(path)
	if err != nil {
		return true, err
	}
	
	return oldFingerprint != newFingerprint, nil
}

// UpdateCache 更新缓存
func (s *SimpleScanner) UpdateCache(path string) error {
	fingerprint, err := s.GetFingerprint(path)
	if err != nil {
		return err
	}
	
	s.mu.Lock()
	s.cache[path] = fingerprint
	s.mu.Unlock()
	
	if s.logger != nil {
		s.logger.Debug("更新缓存: %s -> %s", path, fingerprint)
	}
	return nil
}

// LoadCache 从文件加载缓存
func (s *SimpleScanner) LoadCache(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	s.mu.Lock()
	err = json.Unmarshal(data, &s.cache)
	s.mu.Unlock()
	return err
}

// SaveCache 保存缓存到文件
func (s *SimpleScanner) SaveCache(path string) error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.cache, "", "  ")
	s.mu.RUnlock()
	
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ClearCache 清空缓存
func (s *SimpleScanner) ClearCache() {
	s.mu.Lock()
	s.cache = make(map[string]string)
	s.mu.Unlock()
}

// GetCacheSize 获取缓存大小
func (s *SimpleScanner) GetCacheSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cache)
}