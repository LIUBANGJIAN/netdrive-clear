# NetDrive Clear - 云盘清理工具

基于 CloudDrive2 gRPC API 的自动清理工具，使用 Go 语言开发。

## 功能特性

- ✅ **自动删除广告文件** - 支持 `.txt`, `.html`, `.url`, `.lnk` 等扩展名
- ✅ **自动删除小视频** - 可配置最小视频大小阈值
- ✅ **增量扫描** - 基于目录 mtime，避免重复处理
- ✅ **实时监控** - 定时轮询，自动触发清理
- ✅ **Web 管理界面** - 配置和状态可视化

## 快速开始

### 1. 下载并安装

```bash
# 下载编译好的二进制文件
# 或自行编译
go build -o netdrive-clear.exe .
```

### 2. 配置

编辑 `config.json`：

```json
{
  "cd2": {
    "server": "http://localhost:19798",
    "token": "你的API令牌"
  },
  "cleaner": {
    "ad_extensions": [".txt", ".html", ".url", ".lnk"],
    "video_extensions": [".mp4", ".mkv", ".avi"],
    "min_video_size_mb": 10
  },
  "monitor": {
    "enabled": true,
    "interval_seconds": 30
  },
  "web": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "paths": {
    "watch_paths": [
      { "path": "/115", "enabled": true, "scan_mode": "incremental" }
    ]
  }
}
```

### 3. 运行

```bash
# 使用配置文件中的 token
./netdrive-clear.exe

# 或通过命令行指定
./netdrive-clear.exe -token "你的令牌" -port 8080
```

### 4. 访问 Web 界面

打开浏览器访问：`http://localhost:8080`

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-config` | `config.json` | 配置文件路径 |
| `-token` | (空) | CloudDrive2 API Token |
| `-server` | `http://localhost:19798` | CD2 服务器地址 |
| `-port` | `8080` | Web 管理界面端口 |

## API 令牌获取

1. 登录 CloudDrive2 Web UI
2. 进入设置 → API 令牌
3. 创建新令牌并设置权限

## 项目结构

```
.
├── main.go           # 主程序入口
├── config/
│   └── config.go     # 配置管理
├── cd2/
│   ├── client.go     # CD2 gRPC 客户端封装
│   ├── clouddrive.proto
│   └── pb/           # 生成的 protobuf 代码
├── cleaner/
│   └── cleaner.go    # 清理逻辑
├── scanner/
│   └── state.go      # 增量扫描状态
├── monitor/
│   └── monitor.go    # 实时监控
├── web/
│   ├── server.go     # Web API 服务器
│   └── index.html    # 管理界面
├── config.json       # 配置文件
└── go.mod
```

## License

MIT
