/*
 * Web 服务器模块
 * 提供 RESTful API 和 Web 管理界面
 *
 * API 端点列表:
 *   GET    /api/health          - 健康检查
 *   GET    /api/config          - 获取配置
 *   POST   /api/config          - 更新配置
 *   POST   /api/clean           - 执行清理
 *   POST   /api/clean/:path     - 清理指定路径
 *   GET    /api/status          - 获取状态
 *   POST   /api/monitor/start   - 启动监控
 *   POST   /api/monitor/stop    - 停止监控
 *   GET    /api/monitor/stats   - 获取监控统计
 *   GET    /api/paths           - 获取监控路径列表
 *   POST   /api/paths           - 添加监控路径
 *   DELETE /api/paths/:path     - 删除监控路径
 *   GET    /api/logs            - 获取日志
 *   DELETE /api/logs            - 清空日志
 *   GET    /api/webdav/servers  - 获取 WebDAV 服务器列表
 *   POST   /api/webdav/servers  - 添加 WebDAV 服务器
 *   DELETE /api/webdav/servers/:name - 删除 WebDAV 服务器
 *   PUT    /api/webdav/servers/:name/current - 设置当前服务器
 *   GET    /api/directory        - 获取目录列表
 */

package web

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"netdrive-clear/cleaner"
	"netdrive-clear/config"
	"netdrive-clear/log"
	"netdrive-clear/monitor"
	"netdrive-clear/scanner"
	"netdrive-clear/webdav"
)

// Server Web 服务器
type Server struct {
	engine        *gin.Engine        // Gin 引擎实例
	httpServer    *http.Server       // HTTP 服务器实例
	webdavManager *webdav.Manager    // WebDAV 管理器
	cleaner       *cleaner.Cleaner   // 清理器
	scanner       *scanner.ScanState // 扫描状态
	monitor       *monitor.Monitor   // 监控器
	config        *config.Config     // 配置
	configPath    string             // 配置文件路径
	logger        *log.Logger        // 日志管理器

	mu           sync.RWMutex         // 读写锁
	lastResult   *cleaner.CleanResult // 最后一次清理结果
	lastScanPath string               // 最后扫描的路径
}

// NewServer 创建新的 Web 服务器
func NewServer(cfg *config.Config, configPath string, manager *webdav.Manager, cl *cleaner.Cleaner, sc *scanner.ScanState, mon *monitor.Monitor, logger *log.Logger) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()

	s := &Server{
		engine:        engine,
		webdavManager: manager,
		cleaner:       cl,
		scanner:       sc,
		monitor:       mon,
		config:        cfg,
		configPath:    configPath,
		logger:        logger,
	}

	s.setupRoutes()
	return s
}

// setupRoutes 设置 API 路由
func (s *Server) setupRoutes() {
	s.engine.GET("/api/health", s.handleHealth)

	s.engine.GET("/api/config", s.handleGetConfig)
	s.engine.POST("/api/config", s.handleUpdateConfig)

	s.engine.POST("/api/clean", s.handleClean)
	s.engine.POST("/api/clean/reset", s.handleCleanReset)

	s.engine.GET("/api/status", s.handleStatus)

	s.engine.POST("/api/monitor/start", s.handleMonitorStart)
	s.engine.POST("/api/monitor/stop", s.handleMonitorStop)
	s.engine.GET("/api/monitor/stats", s.handleMonitorStats)

	s.engine.GET("/api/paths", s.handleGetPaths)
	s.engine.POST("/api/paths", s.handleAddPath)
	s.engine.POST("/api/paths/remove", s.handleRemovePath)

	s.engine.GET("/api/webdav/servers", s.handleGetWebDAVServers)
	s.engine.POST("/api/webdav/servers", s.handleAddWebDAVServer)
	s.engine.PUT("/api/webdav/servers/:name", s.handleUpdateWebDAVServer)
	s.engine.DELETE("/api/webdav/servers/:name", s.handleDeleteWebDAVServer)
	s.engine.PUT("/api/webdav/servers/:name/current", s.handleSetCurrentWebDAVServer)
	s.engine.PUT("/api/webdav/servers/:name/enabled", s.handleSetServerEnabled)
	s.engine.GET("/api/webdav/servers/:name/status", s.handleGetServerStatus)

	s.engine.GET("/api/config/cleaner", s.handleGetCleanerConfig)
	s.engine.PUT("/api/config/cleaner", s.handleUpdateCleanerConfig)

	s.engine.GET("/api/directory", s.handleGetDirectory)

	s.engine.GET("/api/logs", s.handleGetLogs)
	s.engine.DELETE("/api/logs", s.handleClearLogs)

	s.engine.GET("/", s.handleIndex)
	s.engine.Static("/static", "./web/static")
}

// Start 启动 Web 服务器
func (s *Server) Start(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.engine,
	}
	return s.httpServer.ListenAndServe()
}

// Stop 停止 Web 服务器
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// handleHealth 处理健康检查请求
func (s *Server) handleHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	err := s.webdavManager.TestConnection(ctx)

	c.JSON(http.StatusOK, gin.H{
		"webdav_connected": err == nil,
		"current_server":   s.webdavManager.GetCurrent(),
	})
}

// handleGetConfig 处理获取配置请求
func (s *Server) handleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.config)
}

// handleUpdateConfig 处理更新配置请求
func (s *Server) handleUpdateConfig(c *gin.Context) {
	var newCfg config.Config
	if err := c.ShouldBindJSON(&newCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	*s.config = newCfg
	if err := s.config.Save(s.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.logger != nil {
		s.logger.Info("配置已更新")
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleClean 处理清理所有路径请求
func (s *Server) handleClean(c *gin.Context) {
	// 检查是否有指定路径
	path := c.Query("path")

	if path != "" {
		// 清理指定路径
		if s.logger != nil {
			s.logger.Info("开始清理路径: %s", path)
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Minute)
		defer cancel()

		result, err := s.cleaner.CleanPath(ctx, path)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("清理路径失败: %s - %v", path, err)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		s.mu.Lock()
		s.lastResult = result
		s.lastScanPath = path
		s.mu.Unlock()

		if s.logger != nil {
			s.logger.Success("路径清理完成: %s - 清理 %d 个文件", path, result.DeletedCount)
		}

		c.JSON(http.StatusOK, result)
	} else {
		// 全量扫描
		if s.logger != nil {
			s.logger.Info("开始全量扫描...")
		}
		s.cleanPaths(s.config.Paths.WatchPaths)
		c.JSON(http.StatusOK, s.getLastResult())
	}
}

// handleCleanReset 处理重置清理统计请求
func (s *Server) handleCleanReset(c *gin.Context) {
	s.cleaner.ResetDeletedCount()

	if s.logger != nil {
		s.logger.Info("已重置清理统计")
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleStatus 处理获取状态请求
func (s *Server) handleStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	testErr := s.webdavManager.TestConnection(ctx)

	s.mu.RLock()
	lastResult := s.lastResult
	lastPath := s.lastScanPath
	s.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"version":            "v2.0.8",
		"webdav_connected":   testErr == nil,
		"current_server":     s.webdavManager.GetCurrent(),
		"server_count":       len(s.webdavManager.ListServers()),
		"monitor_running":    s.monitor.IsRunning(),
		"last_result":        lastResult,
		"last_scan_path":     lastPath,
		"total_cleaned":      s.cleaner.GetDeletedCount(),
		"total_cleaned_size": s.cleaner.GetDeletedSize(),
	})
}

// handleMonitorStart 处理启动监控请求
func (s *Server) handleMonitorStart(c *gin.Context) {
	if err := s.monitor.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.logger != nil {
		s.logger.Success("监控已启动")
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleMonitorStop 处理停止监控请求
func (s *Server) handleMonitorStop(c *gin.Context) {
	s.monitor.Stop()

	if s.logger != nil {
		s.logger.Info("监控已停止")
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleMonitorStats 处理获取监控统计请求
func (s *Server) handleMonitorStats(c *gin.Context) {
	stats := s.monitor.GetStats()
	c.JSON(http.StatusOK, stats)
}

// handleGetPaths 处理获取监控路径列表请求
func (s *Server) handleGetPaths(c *gin.Context) {
	c.JSON(http.StatusOK, s.config.Paths.WatchPaths)
}

// handleAddPath 处理添加监控路径请求
func (s *Server) handleAddPath(c *gin.Context) {
	var wp config.WatchPath
	if err := c.ShouldBindJSON(&wp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.config.Paths.WatchPaths = append(s.config.Paths.WatchPaths, wp)
	if err := s.config.Save(s.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.logger != nil {
		s.logger.Success("已添加监控路径: %s", wp.Path)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleRemovePath 处理删除监控路径请求
func (s *Server) handleRemovePath(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	path := req.Path

	for i, wp := range s.config.Paths.WatchPaths {
		if wp.Path == path {
			s.config.Paths.WatchPaths = append(s.config.Paths.WatchPaths[:i], s.config.Paths.WatchPaths[i+1:]...)
			break
		}
	}

	if err := s.config.Save(s.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.logger != nil {
		s.logger.Info("已删除监控路径: %s", path)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleGetWebDAVServers 处理获取 WebDAV 服务器列表请求
func (s *Server) handleGetWebDAVServers(c *gin.Context) {
	// 从配置文件获取服务器列表（包含启用/禁用状态）
	servers := s.config.WebDAV.Servers
	current := s.webdavManager.GetCurrent()

	c.JSON(http.StatusOK, gin.H{
		"servers": servers,
		"current": current,
	})
}

// handleAddWebDAVServer 处理添加 WebDAV 服务器请求
func (s *Server) handleAddWebDAVServer(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Server   string `json:"server"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" || req.Server == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and server are required"})
		return
	}

	if err := s.webdavManager.AddServer(req.Name, req.Server, req.Username, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.webdavManager.GetCurrent() == "" {
		s.webdavManager.SetCurrent(req.Name)
	}

	server := config.WebDAVServer{
		Name:     req.Name,
		Server:   req.Server,
		Username: req.Username,
		Password: req.Password,
		Enabled:  true,
	}
	s.config.WebDAV.Servers = append(s.config.WebDAV.Servers, server)
	if err := s.config.Save(s.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.logger != nil {
		s.logger.Success("已添加 WebDAV 服务器: %s (%s)", req.Name, req.Server)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleDeleteWebDAVServer 处理删除 WebDAV 服务器请求
func (s *Server) handleDeleteWebDAVServer(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	s.webdavManager.RemoveServer(name)

	for i, sv := range s.config.WebDAV.Servers {
		if sv.Name == name {
			s.config.WebDAV.Servers = append(s.config.WebDAV.Servers[:i], s.config.WebDAV.Servers[i+1:]...)
			break
		}
	}

	if err := s.config.Save(s.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.logger != nil {
		s.logger.Info("已删除 WebDAV 服务器: %s", name)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleSetCurrentWebDAVServer 处理设置当前 WebDAV 服务器请求
func (s *Server) handleSetCurrentWebDAVServer(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := s.webdavManager.TestServerConnection(ctx, name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("连接测试失败: %v", err)})
		return
	}

	if err := s.webdavManager.SetCurrent(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.logger != nil {
		s.logger.Success("已切换到 WebDAV 服务器: %s", name)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleGetDirectory 处理获取目录列表请求
func (s *Server) handleGetDirectory(c *gin.Context) {
	serverName := c.Query("server")
	path := c.Query("path")
	if path == "" {
		path = "/"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var files []webdav.FileInfo
	var err error

	if serverName != "" {
		// 按指定服务器浏览
		files, err = s.webdavManager.ListFilesByServer(ctx, serverName, path)
	} else {
		// 使用当前服务器
		files, err = s.webdavManager.ListFiles(ctx, path)
	}

	if err != nil {
		if s.logger != nil {
			s.logger.Error("获取目录列表失败: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var dirs []gin.H
	for _, f := range files {
		if f.IsDir {
			dirs = append(dirs, gin.H{
				"name": f.Name,
				"path": f.FullPath,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"server":      serverName,
		"path":        path,
		"directories": dirs,
	})
}

// handleUpdateWebDAVServer 处理更新 WebDAV 服务器请求
func (s *Server) handleUpdateWebDAVServer(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	var req struct {
		Name     string `json:"name"`
		Server   string `json:"server"`
		Username string `json:"username"`
		Password string `json:"password"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取新名称（如果有）
	newName := name
	if req.Name != "" {
		newName = req.Name
	}

	// 如果启用，添加或更新服务器到管理器
	if req.Enabled && req.Server != "" {
		// 先移除旧服务器（如果存在）
		s.webdavManager.RemoveServer(name)
		// 添加新配置的服务器
		if err := s.webdavManager.AddServer(newName, req.Server, req.Username, req.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 如果之前没有当前服务器，设置为当前服务器
		if s.webdavManager.GetCurrent() == "" {
			s.webdavManager.SetCurrent(newName)
		}
	} else if !req.Enabled {
		// 如果禁用，从管理器中移除（但保留配置）
		s.webdavManager.RemoveServer(name)
	}

	// 更新配置文件
	found := false
	for i, sv := range s.config.WebDAV.Servers {
		if sv.Name == name {
			s.config.WebDAV.Servers[i].Name = newName
			s.config.WebDAV.Servers[i].Server = req.Server
			s.config.WebDAV.Servers[i].Username = req.Username
			s.config.WebDAV.Servers[i].Password = req.Password
			s.config.WebDAV.Servers[i].Enabled = req.Enabled
			found = true
			break
		}
	}

	// 如果没有找到，添加新服务器配置
	if !found {
		s.config.WebDAV.Servers = append(s.config.WebDAV.Servers, config.WebDAVServer{
			Name:     newName,
			Server:   req.Server,
			Username: req.Username,
			Password: req.Password,
			Enabled:  req.Enabled,
		})
	}

	if err := s.config.Save(s.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.logger != nil {
		s.logger.Success("已更新 WebDAV 服务器: %s", name)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleSetServerEnabled 处理设置服务器启用状态请求
func (s *Server) handleSetServerEnabled(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新配置文件
	found := false
	for i, sv := range s.config.WebDAV.Servers {
		if sv.Name == name {
			s.config.WebDAV.Servers[i].Enabled = req.Enabled
			found = true

			// 如果启用，添加到管理器
			if req.Enabled {
				if err := s.webdavManager.AddServer(sv.Name, sv.Server, sv.Username, sv.Password); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				if s.webdavManager.GetCurrent() == "" {
					s.webdavManager.SetCurrent(name)
				}
			} else {
				// 如果禁用，从管理器中移除
				s.webdavManager.RemoveServer(name)
			}
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	if err := s.config.Save(s.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.logger != nil {
		s.logger.Success("已%s WebDAV 服务器: %s", map[bool]string{true: "启用", false: "禁用"}[req.Enabled], name)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleGetCleanerConfig 处理获取清理器配置请求
func (s *Server) handleGetCleanerConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ad_extensions":     s.config.Cleaner.AdExtensions,
		"video_extensions":  s.config.Cleaner.VideoExtensions,
		"min_video_size_mb": s.config.Cleaner.MinVideoSizeMB,
	})
}

// handleUpdateCleanerConfig 处理更新清理器配置请求
func (s *Server) handleUpdateCleanerConfig(c *gin.Context) {
	var req struct {
		AdExtensions    []string `json:"ad_extensions"`
		VideoExtensions []string `json:"video_extensions"`
		MinVideoSizeMB  int64    `json:"min_video_size_mb"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.AdExtensions != nil {
		s.config.Cleaner.AdExtensions = req.AdExtensions
	}
	if req.VideoExtensions != nil {
		s.config.Cleaner.VideoExtensions = req.VideoExtensions
	}
	if req.MinVideoSizeMB > 0 {
		s.config.Cleaner.MinVideoSizeMB = req.MinVideoSizeMB
	}

	if err := s.config.Save(s.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新cleaner实例的配置
	if s.cleaner != nil {
		s.cleaner.UpdateConfig(&s.config.Cleaner)
	}

	if s.logger != nil {
		s.logger.Success("已更新清理器配置")
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleGetServerStatus 处理获取服务器连接状态请求
func (s *Server) handleGetServerStatus(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	err := s.webdavManager.TestServerConnection(ctx, name)

	c.JSON(http.StatusOK, gin.H{
		"server":    name,
		"connected": err == nil,
		"error": func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
	})
}

// handleGetLogs 处理获取日志请求
func (s *Server) handleGetLogs(c *gin.Context) {
	if s.logger == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "日志模块未初始化"})
		return
	}

	count := 100
	if c.Query("count") != "" {
		fmt.Sscanf(c.Query("count"), "%d", &count)
	}

	logs := s.logger.GetLogs(count)

	// 转换日志格式，适配前端
	type FrontendLogEntry struct {
		Time    string `json:"time"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}

	frontendLogs := make([]FrontendLogEntry, len(logs))
	for i, logEntry := range logs {
		// 转换级别名称
		logType := "info"
		switch logEntry.Level {
		case "WARN":
			logType = "warning"
		case "ERROR":
			logType = "error"
		case "SUCCESS":
			logType = "success"
		}

		frontendLogs[i] = FrontendLogEntry{
			Time:    logEntry.Time,
			Type:    logType,
			Message: logEntry.Message,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  frontendLogs,
		"total": s.logger.GetLogCount(),
	})
}

// handleClearLogs 处理清空日志请求
func (s *Server) handleClearLogs(c *gin.Context) {
	if s.logger == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "日志模块未初始化"})
		return
	}

	if err := s.logger.ClearLogs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleIndex 处理首页请求
func (s *Server) handleIndex(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.File("./web/index.html")
}

// cleanPaths 清理多个路径
func (s *Server) cleanPaths(paths []config.WatchPath) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	for _, wp := range paths {
		if !wp.Enabled {
			continue
		}
		result, err := s.cleaner.CleanPath(ctx, wp.Path)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("清理路径失败: %s - %v", wp.Path, err)
			}
			continue
		}
		s.mu.Lock()
		s.lastResult = result
		s.lastScanPath = wp.Path
		s.mu.Unlock()
	}
}

// getLastResult 获取最后一次清理结果
func (s *Server) getLastResult() *cleaner.CleanResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastResult
}
