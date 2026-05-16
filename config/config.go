/*
 * 配置管理模块
 * 负责加载和保存配置文件
 *
 * 支持功能:
 *   1. 从文件加载配置
 *   2. 支持环境变量覆盖配置
 *   3. 自动生成默认配置
 *   4. 配置文件持久化
 */

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CD2Config 旧版 CloudDrive2 配置（兼容旧格式）
type CD2Config struct {
	Server string `json:"server"` // 服务器地址
	Token  string `json:"token"`  // 访问令牌
}

// Config 主配置结构
type Config struct {
	CD2     CD2Config     `json:"cd2"`     // 旧版 CloudDrive2 配置（兼容）
	WebDAV  WebDAVConfig  `json:"webdav"`  // WebDAV 服务器配置
	Cleaner CleanerConfig `json:"cleaner"` // 清理规则配置
	Scan    ScanConfig    `json:"scan"`    // 扫描配置
	Monitor MonitorConfig `json:"monitor"` // 监控配置
	Web     WebConfig     `json:"web"`     // Web 服务配置
	Paths   PathsConfig   `json:"paths"`   // 监控路径配置
	Log     LogConfig     `json:"log"`     // 日志配置
}

// WebDAVConfig WebDAV 服务器配置
type WebDAVConfig struct {
	Servers []WebDAVServer `json:"servers"` // WebDAV 服务器列表
}

// WebDAVServer 单个 WebDAV 服务器配置
type WebDAVServer struct {
	Name     string `json:"name"`     // 服务器名称
	Server   string `json:"server"`   // 服务器地址，如 http://localhost:19798/webdav
	Username string `json:"username"` // 用户名
	Password string `json:"password"` // 密码
	Enabled  bool   `json:"enabled"`  // 是否启用
}

// CleanerConfig 清理规则配置
type CleanerConfig struct {
	AdExtensions    []string `json:"ad_extensions"`     // 广告文件扩展名列表
	VideoExtensions []string `json:"video_extensions"`  // 视频文件扩展名列表
	MinVideoSizeMB  int64    `json:"min_video_size_mb"` // 最小视频文件大小（MB）
}

// ScanConfig 扫描配置
type ScanConfig struct {
	StateFile           string `json:"state_file"`            // 扫描状态保存文件路径
	PollIntervalSeconds int    `json:"poll_interval_seconds"` // 轮询间隔（秒）
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	Enabled         bool `json:"enabled"`          // 是否启用监控
	IntervalSeconds int  `json:"interval_seconds"` // 监控间隔（秒）
}

// WebConfig Web 服务配置
type WebConfig struct {
	Host string `json:"host"` // 监听地址
	Port int    `json:"port"` // 监听端口
}

// PathsConfig 监控路径配置
type PathsConfig struct {
	WatchPaths []WatchPath `json:"watch_paths"` // 监控路径列表
}

// WatchPath 监控路径项
type WatchPath struct {
	Path     string `json:"path"`      // 云盘路径，如 /115
	Enabled  bool   `json:"enabled"`   // 是否启用该路径监控
	ScanMode string `json:"scan_mode"` // 扫描模式: full(全量), incremental(增量)
}

// LogConfig 日志配置
type LogConfig struct {
	LogPath   string `json:"log_path"`    // 日志文件路径
	MaxSizeMB int64  `json:"max_size_mb"` // 最大文件大小（MB）
	MaxCache  int    `json:"max_cache"`   // 最大内存缓存条数
}

// DefaultConfig 返回默认配置
// 返回默认配置对象
func DefaultConfig() *Config {
	return &Config{
		CD2: CD2Config{
			Server: "http://localhost:19798",
			Token:  "",
		},
		WebDAV: WebDAVConfig{
			Servers: []WebDAVServer{}, // 默认空配置，用户需要添加
		},
		Cleaner: CleanerConfig{
			AdExtensions:    []string{".txt", ".html", ".url", ".lnk", ".desktop", ".ini", ".inf"},
			VideoExtensions: []string{".mp4", ".mkv", ".avi", ".mov", ".rmvb", ".flv", ".wmv"},
			MinVideoSizeMB:  10,
		},
		Scan: ScanConfig{
			StateFile:           "data/scan_state.json",
			PollIntervalSeconds: 30,
		},
		Monitor: MonitorConfig{
			Enabled:         false,
			IntervalSeconds: 30,
		},
		Web: WebConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Paths: PathsConfig{
			WatchPaths: []WatchPath{},
		},
		Log: LogConfig{
			LogPath:   "data/app.log",
			MaxSizeMB: 10,
			MaxCache:  1000,
		},
	}
}

// Load 从文件加载配置
// path: 配置文件路径
// 返回配置对象和错误信息
func Load(path string) (*Config, error) {
	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 文件不存在，返回默认配置
		return DefaultConfig(), nil
	}

	// 读取配置文件内容
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 内容
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 应用环境变量覆盖
	cfg.ApplyEnvOverrides()

	return &cfg, nil
}

// ApplyEnvOverrides 应用环境变量覆盖配置
func (c *Config) ApplyEnvOverrides() {
	if env := os.Getenv("WEB_HOST"); env != "" {
		c.Web.Host = env
	}
	if env := os.Getenv("WEB_PORT"); env != "" {
		c.Web.Port = parseInt(env, 8080)
	}
	if env := os.Getenv("MONITOR_INTERVAL"); env != "" {
		c.Monitor.IntervalSeconds = parseInt(env, 30)
	}
	if env := os.Getenv("MIN_VIDEO_SIZE"); env != "" {
		c.Cleaner.MinVideoSizeMB = int64(parseInt(env, 10))
	}
}

// parseInt 字符串转整数，失败返回默认值
func parseInt(s string, def int) int {
	var v int
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return def
	}
	return v
}

// Save 将配置保存到文件
// path: 保存路径
// 返回错误信息
func (c *Config) Save(path string) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 序列化为格式化的 JSON
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(path, data, 0644)
}

// LoadOrCreate 加载配置，如果不存在则创建默认配置
// path: 配置文件路径（如果为空则自动检测）
// 返回配置对象、实际使用的配置文件路径和错误信息
func LoadOrCreate(path string) (*Config, string, error) {
	// 优先使用环境变量指定的配置路径
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		cfg, err := Load(envPath)
		if err != nil {
			return nil, "", err
		}
		// 如果文件不存在，保存默认配置
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			if err := cfg.Save(envPath); err != nil {
				return nil, "", err
			}
		}
		return cfg, envPath, nil
	}

	// 如果指定了路径，直接使用
	if path != "" {
		cfg, err := Load(path)
		if err != nil {
			return nil, "", err
		}
		// 如果文件不存在，保存默认配置
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := cfg.Save(path); err != nil {
				return nil, "", err
			}
		}
		return cfg, path, nil
	}

	// 自动检测配置文件路径
	// 尝试以下位置：
	// 1. ./data/config.json (当前目录的 data 子目录)
	// 2. /app/data/config.json (Docker 容器默认数据目录)
	// 3. ./config.json (当前目录)
	autoPaths := []string{
		"data/config.json",
		"/app/data/config.json",
		"config.json",
	}

	for _, autoPath := range autoPaths {
		if _, err := os.Stat(autoPath); err == nil {
			// 文件存在，加载配置
			cfg, err := Load(autoPath)
			if err != nil {
				continue
			}
			// 应用环境变量覆盖
			cfg.ApplyEnvOverrides()
			return cfg, autoPath, nil
		}
	}

	// 未找到任何配置文件，创建默认配置
	// 默认使用 data/config.json
	defaultPath := "data/config.json"
	cfg := DefaultConfig()
	if err := cfg.Save(defaultPath); err != nil {
		return nil, "", err
	}
	return cfg, defaultPath, nil
}
