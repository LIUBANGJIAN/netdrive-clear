/*
 * 清理模块
 * 负责扫描目录并删除符合条件的文件
 *
 * 清理规则:
 *   1. 广告文件: 根据扩展名匹配删除
 *   2. 小视频文件: 文件大小小于指定阈值的视频文件
 */

package cleaner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"netdrive-clear/config"
	"netdrive-clear/log"
	"netdrive-clear/webdav"
)

// Cleaner 清理器
type Cleaner struct {
	manager     *webdav.Manager       // WebDAV 管理器
	config      *config.CleanerConfig // 清理配置
	deleted     int64                 // 已删除文件计数
	deletedSize int64                 // 已删除文件总大小（字节）
	logger      *log.Logger           // 日志管理器
	mu          sync.Mutex            // 并发锁
}

// CleanResult 清理结果结构
type CleanResult struct {
	DeletedCount int64     // 删除文件数量
	DeletedSize  int64     // 删除文件总大小（字节）
	DeletedFiles []string  // 删除的文件列表
	Errors       []string  // 错误信息列表
	ScannedFiles int64    // 扫描文件数量
	ScannedDirs  int64    // 扫描目录数量
	Duration     float64   // 扫描耗时（秒）
	Skipped      bool      // 是否跳过
	Message      string    // 附加消息
}

// NewCleaner 创建新的清理器
// manager: WebDAV 管理器
// cfg: 清理配置
// logger: 日志管理器
// 返回清理器实例
func NewCleaner(manager *webdav.Manager, cfg *config.CleanerConfig, logger *log.Logger) *Cleaner {
	return &Cleaner{
		manager: manager,
		config:  cfg,
		logger:  logger,
	}
}

// CleanPath 清理指定路径（使用所有已启用的服务器）
// ctx: 上下文
// path: 要清理的路径
// 返回清理结果和错误信息
func (c *Cleaner) CleanPath(ctx context.Context, path string) (*CleanResult, error) {
	result := &CleanResult{}
	startTime := time.Now()

	// 记录清理开始
	if c.logger != nil {
		c.logger.Info("========== 清理任务开始 ==========")
		c.logger.Info("清理路径: %s", path)
		c.logger.Info("开始时间: %s", startTime.Format("2006-01-02 15:04:05"))
	}

	// 检查 WebDAV 管理器是否可用
	if c.manager == nil {
		if c.logger != nil {
			c.logger.Error("清理失败: 没有可用的 WebDAV 管理器")
		}
		return nil, fmt.Errorf("没有可用的 WebDAV 管理器")
	}

	// 获取所有服务器列表
	servers := c.manager.ListServers()
	if len(servers) == 0 {
		if c.logger != nil {
			c.logger.Error("清理失败: 没有可用的 WebDAV 服务器")
		}
		return nil, fmt.Errorf("没有可用的 WebDAV 服务器，请先在 Web 界面配置")
	}

	if c.logger != nil {
		c.logger.Info("可用服务器数量: %d", len(servers))
		for i, server := range servers {
			c.logger.Info("  [%d] %s", i+1, server)
		}
	}

	// 遍历所有服务器进行清理
	for _, serverName := range servers {
		if c.logger != nil {
			c.logger.Info("开始清理服务器 [%s] ...", serverName)
		}

		serverResult, err := c.cleanPathOnServer(ctx, serverName, path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("服务器 %s: %v", serverName, err))
			if c.logger != nil {
				c.logger.Error("服务器 [%s] 清理失败: %v", serverName, err)
			}
			continue
		}

		result.DeletedCount += serverResult.DeletedCount
		result.DeletedSize += serverResult.DeletedSize
		result.DeletedFiles = append(result.DeletedFiles, serverResult.DeletedFiles...)
		result.Errors = append(result.Errors, serverResult.Errors...)

		if c.logger != nil {
			c.logger.Success("服务器 [%s] 清理完成: 删除 %d 个文件 (%.2f MB)",
				serverName, serverResult.DeletedCount, float64(serverResult.DeletedSize)/1024/1024)
		}
	}

	// 记录清理结束
	duration := time.Since(startTime)
	if c.logger != nil {
		c.logger.Info("========== 清理任务结束 ==========")
		c.logger.Info("总删除文件数: %d", result.DeletedCount)
		c.logger.Info("总删除文件大小: %.2f MB", float64(result.DeletedSize)/1024/1024)
		c.logger.Info("总错误数: %d", len(result.Errors))
		c.logger.Info("耗时: %s", duration.String())
		c.logger.Info("==================================")
	}

	return result, nil
}

// cleanPathOnServer 在指定服务器上清理路径
func (c *Cleaner) cleanPathOnServer(ctx context.Context, serverName, path string) (*CleanResult, error) {
	result := &CleanResult{}

	// 获取目录下的文件列表
	files, err := c.manager.ListFilesByServer(ctx, serverName, path)
	if err != nil {
		return nil, fmt.Errorf("扫描目录失败: %w", err)
	}

	if c.logger != nil {
		c.logger.Info("扫描目录 [%s]: 发现 %d 个文件/目录", path, len(files))
	}

	// 遍历文件，递归处理目录
	for _, file := range files {
		if file.IsDir {
			// 递归清理子目录（在同一服务器上）
			if c.logger != nil {
				c.logger.Info("进入子目录: %s", file.FullPath)
			}

			subResult, err := c.cleanPathOnServer(ctx, serverName, file.FullPath)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file.FullPath, err))
				if c.logger != nil {
					c.logger.Warn("子目录清理失败 [%s]: %v", file.FullPath, err)
				}
				continue
			}
			// 合并子目录的清理结果
			result.DeletedCount += subResult.DeletedCount
			result.DeletedSize += subResult.DeletedSize
			result.DeletedFiles = append(result.DeletedFiles, subResult.DeletedFiles...)
			result.Errors = append(result.Errors, subResult.Errors...)
		} else {
			// 检查是否需要删除
			shouldDelete, reason := c.shouldDelete(file)
			if shouldDelete {
				// 执行删除（在指定服务器上）
				if c.logger != nil {
					c.logger.Info("删除文件 [%s]: %.2f MB, 原因: %s",
						file.FullPath, float64(file.Size)/1024/1024, reason)
				}

				err := c.manager.DeleteFileByServer(ctx, serverName, file.FullPath)
				if err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file.FullPath, err))
					if c.logger != nil {
						c.logger.Error("删除失败 [%s]: %v", file.FullPath, err)
					}
				} else {
					// 更新计数
					c.mu.Lock()
					c.deleted++
					c.deletedSize += file.Size
					c.mu.Unlock()

					result.DeletedCount++
					result.DeletedSize += file.Size
					result.DeletedFiles = append(result.DeletedFiles,
						fmt.Sprintf("%s (%.2f MB, %s)", file.FullPath, float64(file.Size)/1024/1024, reason))

					if c.logger != nil {
						c.logger.Success("删除成功 [%s]: %.2f MB",
							file.FullPath, float64(file.Size)/1024/1024)
					}
				}
			}
			// 不符合删除规则的文件不记录日志
		}
	}

	return result, nil
}

// shouldDelete 判断文件是否应该被删除
// file: 文件信息
// 返回是否删除和删除原因
func (c *Cleaner) shouldDelete(file webdav.FileInfo) (bool, string) {
	// 获取文件扩展名（小写）
	ext := strings.ToLower(getExt(file.Name))

	// 检查是否为广告文件
	if c.isAdExtension(ext) {
		return true, "广告文件"
	}

	// 检查是否为小视频文件
	if c.isVideoExtension(ext) && file.Size > 0 {
		minSize := c.config.MinVideoSizeMB * 1024 * 1024
		if file.Size < int64(minSize) {
			return true, fmt.Sprintf("小视频文件 (%.2f MB < %d MB)",
				float64(file.Size)/1024/1024, c.config.MinVideoSizeMB)
		}
	}

	return false, ""
}

// isAdExtension 判断是否为广告文件扩展名
// ext: 文件扩展名（带点号）
// 返回是否为广告文件扩展名
func (c *Cleaner) isAdExtension(ext string) bool {
	for _, e := range c.config.AdExtensions {
		if e == ext {
			return true
		}
	}
	return false
}

// isVideoExtension 判断是否为视频文件扩展名
// ext: 文件扩展名（带点号）
// 返回是否为视频文件扩展名
func (c *Cleaner) isVideoExtension(ext string) bool {
	for _, e := range c.config.VideoExtensions {
		if e == ext {
			return true
		}
	}
	return false
}

// getExt 获取文件扩展名
// name: 文件名
// 返回扩展名（带点号），如 .txt
func getExt(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx == -1 || idx == len(name)-1 {
		return ""
	}
	return name[idx:]
}

// GetDeletedCount 获取已删除文件总数
// 返回已删除文件数量
func (c *Cleaner) GetDeletedCount() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deleted
}

// GetDeletedSize 获取已删除文件总大小
// 返回已删除文件总大小（字节）
func (c *Cleaner) GetDeletedSize() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deletedSize
}

// ResetDeletedCount 重置已删除文件计数
func (c *Cleaner) ResetDeletedCount() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleted = 0
	c.deletedSize = 0
}

// UpdateConfig 更新清理器配置
// cfg: 新的清理配置
func (c *Cleaner) UpdateConfig(cfg *config.CleanerConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = cfg
}
