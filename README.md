# NetDrive Clear - 云盘清理工具

基于 WebDAV 协议的云盘自动清理工具，使用 Go 语言开发。支持自动删除广告文件和小视频，具备增量扫描和实时监控功能。

## 功能特性

- ✅ **自动删除广告文件** - 支持 `.txt`, `.html`, `.url`, `.lnk` 等扩展名
- ✅ **自动删除小视频** - 可配置最小视频大小阈值
- ✅ **细粒度增量扫描** - 每个子文件夹独立指纹，只有变化的文件夹才扫描
- ✅ **多级子目录支持** - 支持任意深度的嵌套目录监控
- ✅ **实时监控** - 定时轮询检测目录变化，自动触发清理
- ✅ **Web 管理界面** - 配置和状态可视化
- ✅ **多服务器支持** - 可配置多个 WebDAV 服务器

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
  "webdav": {
    "servers": [
      {
        "name": "115网盘",
        "url": "http://localhost:19798/dav",
        "username": "",
        "password": ""
      }
    ]
  },
  "paths": {
    "watch_paths": [
      { "path": "/BON_115网盘/BM", "enabled": true }
    ]
  }
}
```

### 3. 运行

```bash
# 使用配置文件
./netdrive-clear.exe

# 或通过命令行指定端口
./netdrive-clear.exe -port 8080
```

### 4. 访问 Web 界面

打开浏览器访问：`http://localhost:8080`

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-config` | `config.json` | 配置文件路径 |
| `-port` | `8080` | Web 管理界面端口 |

## 技术原理

### 监控机制

由于 WebDAV 不支持文件系统事件通知，监控通过**定时轮询**实现：

1. **指纹计算**：为每个子文件夹计算指纹（修改时间_文件数_最新文件时间）
2. **变化检测**：对比指纹变化，只扫描发生变化的文件夹
3. **增量清理**：跳过未变化的子目录，减少 API 调用

### 指纹格式

```
指纹 = 修改时间戳_文件数量_最新文件时间戳
例如: "1699999999_5_1700000001"
```

### 目录结构示例

```
监控目录: /BON_115网盘/BM
├── 2026-01-11/        ← 指纹变化 → 扫描
│   ├── URE-129-C/     ← 指纹未变 → 跳过
│   ├── WAAA-549-C/    ← 指纹未变 → 跳过
│   └── WAAA-573-C/    ← 指纹变化 → 扫描
│       └── video.mp4  ← 被检测并删除
└── 2026-01-12/        ← 指纹未变 → 跳过
```

## 项目结构

```
.
├── main.go           # 主程序入口
├── config/
│   └── config.go     # 配置管理
├── webdav/
│   ├── client.go     # WebDAV 客户端封装
│   └── manager.go    # WebDAV 管理器
├── cleaner/
│   └── cleaner.go    # 清理逻辑（支持指纹缓存跳过）
├── scanner/
│   └── state.go      # 增量扫描状态管理
├── monitor/
│   └── monitor.go    # 实时监控（细粒度子文件夹检测）
├── web/
│   ├── server.go     # Web API 服务器
│   └── index.html    # 管理界面
├── log/
│   └── log.go        # 日志管理
├── config.json       # 配置文件
└── go.mod
```

## 配置说明

### Cleaner（清理器配置）

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `ad_extensions` | array | `[".txt", ".html", ".url", ".lnk"]` | 广告文件扩展名列表 |
| `video_extensions` | array | `[".mp4", ".mkv", ".avi", ".m4v", ".mov", ".wmv", ".flv"]` | 视频文件扩展名列表 |
| `min_video_size_mb` | int | `10` | 最小视频大小（MB），小于此值的视频将被删除 |

### Monitor（监控配置）

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | `true` | 是否启用监控 |
| `interval_seconds` | int | `30` | 监控检测间隔（秒） |

### WebDAV（服务器配置）

| 参数 | 类型 | 说明 |
|------|------|------|
| `name` | string | 服务器名称（用于标识） |
| `url` | string | WebDAV 服务地址 |
| `username` | string | 用户名（可选） |
| `password` | string | 密码（可选） |

## License

MIT
