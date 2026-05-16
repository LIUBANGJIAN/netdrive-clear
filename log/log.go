/*
 * 日志模块 - 完整重写版本
 * 提供规范化的日志输出和管理功能
 *
 * 功能特性:
 *   1. 支持多种日志级别: DEBUG, INFO, WARN, ERROR, SUCCESS
 *   2. 控制台彩色输出
 *   3. 日志同时输出到控制台和文件
 *   4. 支持日志文件滚动
 *   5. 支持日志清空和获取
 *   6. 支持日志级别过滤
 *   7. 使用CST时区，不依赖系统时区文件
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
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	SUCCESS
)

// 日志级别名称
var levelNames = []string{
	"DEBUG",
	"INFO",
	"WARN",
	"ERROR",
	"SUCCESS",
}

// 日志级别颜色（ANSI转义码）
var levelColors = []string{
	"\033[36m", // DEBUG - 青色
	"\033[32m", // INFO - 绿色
	"\033[33m", // WARN - 黄色
	"\033[31m", // ERROR - 红色
	"\033[32m", // SUCCESS - 绿色
}

// 重置颜色
const colorReset = "\033[0m"

// Logger 日志管理器
type Logger struct {
	mu       sync.Mutex // 并发锁
	logFile  *os.File   // 日志文件句柄
	logPath  string     // 日志文件路径
	maxSize  int64      // 最大文件大小（字节）
	logs     []LogEntry // 内存日志缓存
	maxCache int        // 最大缓存条数
	minLevel LogLevel   // 最小输出级别
	console  bool       // 是否输出到控制台

	subscribers []chan<- LogEntry // 日志订阅者通道
	subMu       sync.Mutex        // 订阅者锁
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
	MinLevel  string `json:"min_level"`   // 最小日志级别
	Console   bool   `json:"console"`     // 是否输出到控制台
}

// 默认配置
const (
	defaultLogPath   = "data/app.log"
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

	// 解析最小日志级别
	minLevel := parseLevel(cfg.MinLevel)

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
		minLevel: minLevel,
		console:  cfg.Console,
	}, nil
}

// parseLevel 解析日志级别字符串
func parseLevel(levelStr string) LogLevel {
	switch levelStr {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "SUCCESS":
		return SUCCESS
	default:
		return DEBUG // 默认输出所有级别
	}
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

// Debug 输出调试日志
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...))
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
	// 检查日志级别过滤
	if level < l.minLevel {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 使用CST时区（UTC+8），不依赖系统时区文件
	now := time.Now().UTC()
	cstTime := now.Add(time.Hour * 8)
	timeStr := cstTime.Format("2006-01-02 15:04:05 CST")

	// 获取级别名称和颜色
	levelName := levelNames[level]
	levelColor := levelColors[level]

	// 格式化日志内容（带颜色）
	colorLogStr := fmt.Sprintf("%s[%s] [%s]%s %s\n",
		levelColor,
		timeStr,
		levelName,
		colorReset,
		message,
	)

	// 格式化日志内容（无颜色，用于文件）
	fileLogStr := fmt.Sprintf("[%s] [%s] %s\n",
		timeStr,
		levelName,
		message,
	)

	// 输出到控制台
	if l.console || true { // 默认输出到控制台
		fmt.Print(colorLogStr)
	}

	// 检查文件大小，需要滚动
	if err := l.checkAndRotate(); err != nil {
		fmt.Printf("%s[ERROR] [%s] 日志文件滚动失败: %v%s\n",
			levelColors[ERROR], timeStr, err, colorReset)
	}

	// 写入文件
	if l.logFile != nil {
		if _, err := l.logFile.WriteString(fileLogStr); err != nil {
			fmt.Printf("%s[ERROR] [%s] 写入日志文件失败: %v%s\n",
				levelColors[ERROR], timeStr, err, colorReset)
		}
	}

	// 添加到内存缓存
	entry := LogEntry{
		Time:    timeStr,
		Level:   levelName,
		Message: message,
	}

	l.logs = append(l.logs, entry)

	// 如果缓存超过限制，移除最旧的日志
	if len(l.logs) > l.maxCache {
		l.logs = l.logs[len(l.logs)-l.maxCache:]
	}

	// 通知所有订阅者
	l.notifySubscribers(entry)
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
		now := time.Now().UTC().Add(time.Hour * 8)
		oldPath := l.logPath + "." + now.Format("20060102_150405")
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
// count: 获取日志条数，0表示获取全部
// 返回日志条目列表
func (l *Logger) GetLogs(count int) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	if count <= 0 || count > len(l.logs) {
		count = len(l.logs)
	}

	start := len(l.logs) - count
	return append([]LogEntry(nil), l.logs[start:]...) // 返回副本
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

// SetMinLevel 设置最小日志级别
func (l *Logger) SetMinLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

// GetMinLevel 获取当前最小日志级别
func (l *Logger) GetMinLevel() LogLevel {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.minLevel
}

// Subscribe 订阅日志
// channel: 接收日志的通道
// bufferSize: 通道缓冲区大小
func (l *Logger) Subscribe(channel chan<- LogEntry) {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	l.subscribers = append(l.subscribers, channel)
}

// Unsubscribe 取消订阅日志
// channel: 要取消的通道
func (l *Logger) Unsubscribe(channel chan<- LogEntry) {
	l.subMu.Lock()
	defer l.subMu.Unlock()

	for i, ch := range l.subscribers {
		if ch == channel {
			l.subscribers = append(l.subscribers[:i], l.subscribers[i+1:]...)
			break
		}
	}
}

// notifySubscribers 通知所有订阅者
func (l *Logger) notifySubscribers(entry LogEntry) {
	l.subMu.Lock()
	defer l.subMu.Unlock()

	for _, ch := range l.subscribers {
		select {
		case ch <- entry:
			// 发送成功
		default:
			// 通道已满，丢弃
		}
	}
}
