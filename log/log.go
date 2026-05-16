/*
 * 日志模块
 * 提供规范化的日志输出和管理功能
 *
 * 功能特性:
 *   1. 支持多种日志级别: INFO, WARN, ERROR, SUCCESS
 *   2. 日志同时输出到控制台和文件
 *   3. 支持日志文件滚动
 *   4. 支持日志清空
 *   5. 支持获取日志内容
 */

package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel 日志级别
type LogLevel string

const (
	INFO    LogLevel = "INFO"    // 信息级别
	WARN    LogLevel = "WARN"    // 警告级别
	ERROR   LogLevel = "ERROR"   // 错误级别
	SUCCESS LogLevel = "SUCCESS" // 成功级别
)

// Logger 日志管理器
type Logger struct {
	mu       sync.Mutex // 并发锁
	logFile  *os.File   // 日志文件句柄
	logPath  string     // 日志文件路径
	maxSize  int64      // 最大文件大小（字节）
	logs     []LogEntry // 内存日志缓存
	maxCache int        // 最大缓存条数
}

// LogEntry 日志条目
type LogEntry struct {
	Time    string `json:"time"`    // 日志时间
	Level   string `json:"level"`   // 日志级别
	Message string `json:"message"` // 日志消息
}

// Config 日志配置
type Config struct {
	LogPath   string `json:"log_path"`    // 日志文件路径
	MaxSizeMB int64  `json:"max_size_mb"` // 最大文件大小（MB）
	MaxCache  int    `json:"max_cache"`   // 最大内存缓存条数
}

// 默认配置
const (
	defaultLogPath   = "logs/app.log"
	defaultMaxSizeMB = 10
	defaultMaxCache  = 1000
)

// NewLogger 创建新的日志管理器
// cfg: 日志配置
// 返回日志管理器实例
func NewLogger(cfg *Config) (*Logger, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	if cfg.LogPath == "" {
		cfg.LogPath = defaultLogPath
	}
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = defaultMaxSizeMB
	}
	if cfg.MaxCache <= 0 {
		cfg.MaxCache = defaultMaxCache
	}

	// 确保日志目录存在
	logDir := filepath.Dir(cfg.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	// 打开日志文件
	file, err := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &Logger{
		logFile:  file,
		logPath:  cfg.LogPath,
		maxSize:  cfg.MaxSizeMB * 1024 * 1024,
		logs:     make([]LogEntry, 0, cfg.MaxCache),
		maxCache: cfg.MaxCache,
	}, nil
}

// Close 关闭日志管理器
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

// Info 输出信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, args...))
}

// Warn 输出警告日志
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, args...))
}

// Error 输出错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...))
}

// Success 输出成功日志
func (l *Logger) Success(format string, args ...interface{}) {
	l.log(SUCCESS, fmt.Sprintf(format, args...))
}

// log 输出日志
// level: 日志级别
// message: 日志消息
func (l *Logger) log(level LogLevel, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 使用CST时区（UTC+8），如果加载失败则回退到UTC
	now := time.Now()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 如果时区加载失败，使用UTC时间
		timeStr := now.Format("2006-01-02 15:04:05 UTC")
		// 格式化日志内容
		logStr := fmt.Sprintf("[%s] [%s] %s\n",
			timeStr,
			level,
			message,
		)
		// 输出到控制台
		fmt.Print(logStr)

		// 写入日志文件
		if l.logFile != nil {
			l.logFile.WriteString(logStr)
		}

		// 添加到内存缓存
		l.logs = append(l.logs, LogEntry{
			Time:    timeStr,
			Level:   string(level),
			Message: message,
		})

		// 如果缓存超过最大值，移除最老的记录
		if len(l.logs) > l.maxCache {
			l.logs = l.logs[len(l.logs)-l.maxCache:]
		}

		// 检查日志文件大小
		l.checkAndRotate()
		return
	}

	cstTime := now.In(loc)
	timeStr := cstTime.Format("2006-01-02 15:04:05 CST")

	// 格式化日志内容
	logStr := fmt.Sprintf("[%s] [%s] %s\n",
		timeStr,
		level,
		message,
	)

	// 输出到控制台
	fmt.Print(logStr)

	// 检查文件大小，需要滚动
	if err := l.checkAndRotate(); err != nil {
		fmt.Printf("[ERROR] 日志文件滚动失败: %v\n", err)
	}

	// 写入文件
	if l.logFile != nil {
		if _, err := l.logFile.WriteString(logStr); err != nil {
			fmt.Printf("[ERROR] 写入日志文件失败: %v\n", err)
		}
	}

	// 添加到内存缓存
	entry := LogEntry{
		Time:    timeStr,
		Level:   string(level),
		Message: message,
	}

	l.logs = append(l.logs, entry)

	// 如果缓存超过限制，移除最旧的日志
	if len(l.logs) > l.maxCache {
		l.logs = l.logs[len(l.logs)-l.maxCache:]
	}
}

// checkAndRotate 检查并滚动日志文件
func (l *Logger) checkAndRotate() error {
	if l.logFile == nil {
		return nil
	}

	stat, err := l.logFile.Stat()
	if err != nil {
		return err
	}

	if stat.Size() >= l.maxSize {
		// 关闭当前文件
		if err := l.logFile.Close(); err != nil {
			return err
		}

		// 重命名旧文件
		oldPath := l.logPath + "." + time.Now().Format("20060102_150405")
		if err := os.Rename(l.logPath, oldPath); err != nil {
			return err
		}

		// 创建新文件
		newFile, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}

		l.logFile = newFile
	}

	return nil
}

// GetLogs 获取最近的日志
// count: 获取日志条数
// 返回日志条目列表
func (l *Logger) GetLogs(count int) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	if count <= 0 || count > len(l.logs) {
		count = len(l.logs)
	}

	start := len(l.logs) - count
	return l.logs[start:]
}

// ClearLogs 清空日志
// 返回错误信息
func (l *Logger) ClearLogs() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 清空内存缓存
	l.logs = make([]LogEntry, 0, l.maxCache)

	// 清空日志文件
	if l.logFile != nil {
		if err := l.logFile.Truncate(0); err != nil {
			return err
		}
		if err := l.logFile.Sync(); err != nil {
			return err
		}
	}

	return nil
}

// GetLogCount 获取日志条数
// 返回日志条数
func (l *Logger) GetLogCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.logs)
}

// GetLogFileSize 获取日志文件大小（字节）
// 返回文件大小
func (l *Logger) GetLogFileSize() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile == nil {
		return 0
	}

	stat, err := l.logFile.Stat()
	if err != nil {
		return 0
	}

	return stat.Size()
}
