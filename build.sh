#!/bin/bash
# NetDrive Clear Docker 构建和上传脚本
# 支持从 token.txt 文件读取凭证

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 默认值
IMAGE_NAME="netdrive-clear"
DOCKERFILE="Dockerfile"
PLATFORMS="linux/amd64,linux/arm64"
TAG="latest"
DEBUG=false
HTTP_PROXY=""
HTTPS_PROXY=""

# 打印消息
print_msg() {
    echo -e "${GREEN}[NetDrive Clear]${NC} $1"
}

# 打印错误
print_error() {
    echo -e "${RED}[错误]${NC} $1" >&2
}

# 打印调试信息
print_debug() {
    if [ "$DEBUG" = true ]; then
        echo -e "${YELLOW}[调试]${NC} $1"
    fi
}

# 显示帮助
show_help() {
    cat << EOF
NetDrive Clear Docker 构建和上传脚本

用法: $0 [选项]

选项:
  -u, --username <用户名>    Docker Hub 用户名
  -t, --tag <标签>          镜像标签 (默认: latest)
  -p, --platforms <平台>    目标平台 (默认: linux/amd64,linux/arm64)
  -x, --proxy <代理>        HTTP/HTTPS 代理地址 (如: http://192.168.10.254:7890)
  -d, --debug               启用调试模式
  -h, --help                显示帮助信息

环境变量:
  DOCKER_USERNAME           Docker Hub 用户名
  DOCKER_PASSWORD           Docker Hub 密码或访问令牌

示例:
  $0 -u myuser -t v1.0.0
  DOCKER_USERNAME=myuser DOCKER_PASSWORD=mytoken $0 -t v1.0.0
EOF
}

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -u|--username)
                DOCKER_USERNAME="$2"
                shift 2
                ;;
            -t|--tag)
                TAG="$2"
                shift 2
                ;;
            -p|--platforms)
                PLATFORMS="$2"
                shift 2
                ;;
            -x|--proxy)
                HTTP_PROXY="$2"
                HTTPS_PROXY="$2"
                shift 2
                ;;
            -d|--debug)
                DEBUG=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                print_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# 加载凭证
load_credentials() {
    # 1. 检查命令行参数或环境变量
    if [ -n "$DOCKER_USERNAME" ] && [ -n "$DOCKER_PASSWORD" ]; then
        print_msg "使用环境变量中的凭证"
        return
    fi
    
    # 2. 尝试从 token.txt 文件读取
    if [ -f "token.txt" ]; then
        print_msg "从 token.txt 文件读取凭证..."
        local content=$(cat token.txt | tr -d '\r')
        print_debug "文件内容: '$content'"
        
        # 解析格式: 用户名:token
        DOCKER_USERNAME=$(echo "$content" | cut -d':' -f1 | tr -d ' ')
        DOCKER_PASSWORD=$(echo "$content" | cut -d':' -f2- | tr -d ' ')
        
        print_debug "解析用户名: '$DOCKER_USERNAME'"
        print_debug "解析密码长度: ${#DOCKER_PASSWORD}"
        
        if [ -z "$DOCKER_USERNAME" ] || [ -z "$DOCKER_PASSWORD" ]; then
            print_error "token.txt 格式错误，应为: 用户名:token"
            exit 1
        fi
        return
    fi
    
    # 3. 交互式输入
    print_msg "请输入 Docker Hub 凭证:"
    read -p "用户名: " DOCKER_USERNAME
    read -s -p "密码/访问令牌: " DOCKER_PASSWORD
    echo
}

# 登录 Docker Hub
login_docker() {
    if [ -z "$DOCKER_USERNAME" ] || [ -z "$DOCKER_PASSWORD" ]; then
        print_error "Docker Hub 凭证未设置"
        exit 1
    fi
    
    print_msg "登录 Docker Hub..."
    print_debug "用户名: $DOCKER_USERNAME"
    print_debug "密码长度: ${#DOCKER_PASSWORD}"
    
    # 使用管道方式登录（这是最可靠的方式）
    if ! echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin; then
        print_error "登录失败，请检查凭证"
        exit 1
    fi
    
    print_msg "登录成功！"
}

# 设置 Docker Buildx
setup_buildx() {
    print_msg "设置 Docker Buildx..."
    
    # 检查是否已安装 buildx
    if ! docker buildx version &>/dev/null; then
        print_error "Docker Buildx 未安装，请确保 Docker 版本 >= 19.03"
        exit 1
    fi
    
    # 检查构建器是否存在
    BUILDER_NAME="netdrive-clear-builder"
    if docker buildx ls | grep -q "$BUILDER_NAME"; then
        print_debug "构建器已存在，切换使用..."
        docker buildx use "$BUILDER_NAME"
    else
        print_debug "创建新的构建器..."
        docker buildx create --name "$BUILDER_NAME" --use
    fi
    
    # 检查构建器状态
    print_msg "初始化构建器环境..."
    docker buildx inspect "$BUILDER_NAME" --bootstrap
}

# 构建并推送镜像
build_and_push() {
    local full_tag="${DOCKER_USERNAME}/${IMAGE_NAME}:${TAG}"
    
    print_msg "Dockerfile: $DOCKERFILE"
    print_msg "目标平台: $PLATFORMS"
    print_msg "镜像标签: $full_tag"
    print_msg "开始构建（这可能需要几分钟）..."
    
    # 构建并推送
    if ! docker buildx build \
        --platform "$PLATFORMS" \
        --tag "$full_tag" \
        --push \
        --provenance false \
        -f "$DOCKERFILE" .; then
        print_error "构建失败"
        exit 1
    fi
    
    print_msg "构建完成！镜像已推送到 Docker Hub"
}

# 主函数
main() {
    # 解析参数
    parse_args "$@"
    
    # 设置代理环境变量
    if [ -n "$HTTP_PROXY" ]; then
        export HTTP_PROXY="$HTTP_PROXY"
        export HTTPS_PROXY="$HTTPS_PROXY"
        export NO_PROXY="localhost,127.0.0.1"
        print_msg "使用代理: $HTTP_PROXY"
    fi
    
    # 加载凭证
    load_credentials
    
    # 登录 Docker Hub
    login_docker
    
    # 设置 Buildx
    setup_buildx
    
    # 构建并推送
    build_and_push
    
    print_msg "🎉 所有操作完成！"
}

# 执行主函数
main "$@"
