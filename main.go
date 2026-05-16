/*
 * NetDrive Clear - 云盘清理工具
 * 基于 CloudDrive2 WebDAV 实现自动清理广告文件和小视频
 *
 * 功能特性:
 *   1. 自动删除广告文件 (.txt, .html, .url, .lnk 等)
 *   2. 自动删除小于指定大小的视频文件
 *   3. 增量扫描，基于目录 mtime 避免重复处理
 *   4. 实时监控，定时轮询自动触发清理
 *   5. Web 管理界面，支持配置和状态查看
 *   6. 多架构支持 (x86, ARM)，适合群晖 NAS 使用
 *   7. 配置文件持久化，支持 Docker 部署
 *
 * 使用方式:
 *   ./netdrive-clear -config config.json -port 8080
 *
 * 环境变量:
 *   WEB_HOST        - Web 服务监听地址
 *   WEB_PORT        - Web 服务监听端口
 *   MONITOR_INTERVAL - 监控间隔（秒）
 *   MIN_VIDEO_SIZE  - 最小视频大小（MB）
 *
 * 作者: NetDrive Clear Team
 * 版本: v2.0.1
 */

package main

import (
	"flag"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"
	"syscall"

	// 导入自定义包
	"netdrive-clear/cleaner"   // 清理模块
	"netdrive-clear/config"    // 配置管理
	mylog "netdrive-clear/log" // 日志模块
	"netdrive-clear/monitor"   // 监控模块
	"netdrive-clear/scanner"   // 扫描状态管理
	"netdrive-clear/web"       // Web 服务器
	"netdrive-clear/webdav"    // WebDAV 客户端
)

// 命令行参数定义
var (
	configFile = flag.String("config", "data/config.json", "配置文件路径")
	port       = flag.Int("port", 8080, "Web 管理界面端口")
)

// 全局日志实例
var logger *mylog.Logger

// main 函数 - 程序入口
func main() {
	// 解析命令行参数
	flag.Parse()

	// 打印欢迎信息
	printBanner()

	// 确保数据目录存在
	if err := ensureDataDir(); err != nil {
		stdlog.Fatalf("[错误] 创建数据目录失败: %v", err)
	}

	// 加载配置文件
	cfg, configPath, err := loadConfig()
	if err != nil {
		stdlog.Fatalf("[错误] 加载配置文件失败: %v", err)
	}

	logger, err = initLogger(cfg)
	if err != nil {
		stdlog.Fatalf("[错误] 初始化日志失败: %v", err)
	}
	defer logger.Close()

	logger.Info("配置文件路径: %s", configPath)
	logger.Info("==================================")
	logger.Info("NetDrive Clear 服务启动中...")

	// 调试：打印配置信息
	logger.Info("配置文件中 WebDAV 服务器数量: %d", len(cfg.WebDAV.Servers))
	for i, server := range cfg.WebDAV.Servers {
		logger.Info("  服务器 %d: name=%s, enabled=%v", i+1, server.Name, server.Enabled)
	}
	logger.Info("配置文件中监控路径数量: %d", len(cfg.Paths.WatchPaths))
	for i, path := range cfg.Paths.WatchPaths {
		logger.Info("  路径 %d: %s", i+1, path)
	}

	// 初始化 WebDAV 管理器
	webdavManager, _ := initWebDAV(cfg)

	// 初始化各模块
	cleanerInstance := cleaner.NewCleaner(webdavManager, &cfg.Cleaner, logger)
	scanState := scanner.NewScanState(cfg.Scan.StateFile)
	monitorInstance := monitor.NewMonitor(webdavManager, cleanerInstance, scanState, &cfg.Monitor, cfg.Paths.WatchPaths)

	// 初始化 Web 服务器
	webServer := web.NewServer(cfg, configPath, webdavManager, cleanerInstance, scanState, monitorInstance, logger)

	// 根据配置决定是否启动监控
	if cfg.Monitor.Enabled {
		logger.Info("正在启动实时监控...")
		if err := monitorInstance.Start(); err != nil {
			logger.Warn("启动监控失败: %v", err)
		} else {
			logger.Success("监控已启动")
		}
	}

	// 打印服务信息
	printServiceInfo(cfg)
	logger.Info("Web 管理界面: http://localhost:%d", *port)

	// 启动信号监听，优雅退出
	startSignalHandler(monitorInstance, webServer)

	// 启动 Web 服务器
	logger.Info("Web 服务器启动中...")
	if err := webServer.Start(cfg.Web.Host, *port); err != nil {
		logger.Error("Web 服务器启动失败: %v", err)
		stdlog.Fatalf("[错误] Web 服务器启动失败: %v", err)
	}
}

// ensureDataDir 确保数据目录存在
func ensureDataDir() error {
	return os.MkdirAll("data", 0755)
}

// initLogger 初始化日志模块
func initLogger(cfg *config.Config) (*mylog.Logger, error) {
	return mylog.NewLogger(&mylog.Config{
		LogPath:   cfg.Log.LogPath,
		MaxSizeMB: cfg.Log.MaxSizeMB,
		MaxCache:  cfg.Log.MaxCache,
	})
}

// initWebDAV 初始化 WebDAV 管理器
func initWebDAV(cfg *config.Config) (*webdav.Manager, error) {
	manager := webdav.NewManager()

	// 添加所有配置的服务器
	for _, server := range cfg.WebDAV.Servers {
		if server.Enabled {
			logger.Info("添加 WebDAV 服务器: %s (%s)", server.Name, server.Server)
			if err := manager.AddServer(server.Name, server.Server, server.Username, server.Password); err != nil {
				logger.Warn("添加服务器 %s 失败: %v", server.Name, err)
				continue
			}

			// 如果没有设置当前服务器，设置为第一个
			if manager.GetCurrent() == "" {
				manager.SetCurrent(server.Name)
			}
		}
	}

	if manager.GetCurrent() == "" {
		logger.Warn("当前没有配置 WebDAV 服务器，请在 Web 界面中添加")
	} else {
		logger.Success("WebDAV 服务器初始化完成，当前服务器: %s", manager.GetCurrent())
	}

	return manager, nil
}

// printBanner 打印程序欢迎信息
func printBanner() {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════════╗
║                    NetDrive Clear v2.0.14                       ║
║              CloudDrive2 云盘自动清理工具                        ║
║                                                                ║
║  功能: 自动删除广告文件、小视频文件、增量扫描、实时监控            ║
║  支持: Docker / x86 / ARM / 群晖 NAS / WebDAV                  ║
╚══════════════════════════════════════════════════════════════════╝
`)
}

// loadConfig 加载配置文件
func loadConfig() (*config.Config, string, error) {
	// 优先使用环境变量指定的配置路径
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		*configFile = envPath
	}

	cfg, configPath, err := config.LoadOrCreate(*configFile)
	if err != nil {
		return nil, "", err
	}

	return cfg, configPath, nil
}

// printServiceInfo 打印服务信息
func printServiceInfo(cfg *config.Config) {
	fmt.Printf("\n")
	fmt.Printf("┌──────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("│                    服务已启动                               │\n")
	fmt.Printf("├──────────────────────────────────────────────────────────────┤\n")
	fmt.Printf("│ Web 管理界面:  http://localhost:%d                         │\n", *port)
	fmt.Printf("│ WebDAV 服务器: %d 个                                        │\n", len(cfg.WebDAV.Servers))
	fmt.Printf("│ 监控路径数量:  %d                                          │\n", len(cfg.Paths.WatchPaths))
	fmt.Printf("│ 监控状态:      %s                                          │\n", boolToStatus(cfg.Monitor.Enabled))
	fmt.Printf("├──────────────────────────────────────────────────────────────┤\n")
	fmt.Printf("│ 按 Ctrl+C 优雅退出                                          │\n")
	fmt.Printf("└──────────────────────────────────────────────────────────────┘\n")
	fmt.Printf("\n")
}

// boolToStatus 将布尔值转换为状态字符串
func boolToStatus(b bool) string {
	if b {
		return "🟢 已启用"
	}
	return "⚪ 已禁用"
}

// startSignalHandler 启动信号监听
func startSignalHandler(monitor *monitor.Monitor, webServer *web.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		stdlog.Println("\n[信息] 收到退出信号，正在停止服务...")

		if logger != nil {
			logger.Info("收到退出信号，正在停止服务...")
		}

		// 停止监控
		monitor.Stop()
		stdlog.Println("[信息] 监控已停止")
		if logger != nil {
			logger.Info("监控已停止")
		}

		// 停止 Web 服务器
		if err := webServer.Stop(); err != nil {
			stdlog.Printf("[警告] Web 服务器停止时出错: %v", err)
			if logger != nil {
				logger.Warn("Web 服务器停止时出错: %v", err)
			}
		} else {
			stdlog.Println("[信息] Web 服务器已停止")
			if logger != nil {
				logger.Info("Web 服务器已停止")
			}
		}

		if logger != nil {
			logger.Info("程序已优雅退出")
		}
		stdlog.Println("[信息] 程序已优雅退出")
		os.Exit(0)
	}()
}
