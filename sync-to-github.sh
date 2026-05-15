#!/bin/bash
# 自动同步代码到 GitHub

echo "开始同步代码到 GitHub..."

cd "$(dirname "$0")"

# 检查是否已初始化 Git
if [ ! -d ".git" ]; then
    echo "首次运行，正在初始化 Git 仓库..."
    git init
    git remote add origin https://github.com/LIUBANGJIAN/netdrive-clear.git
    echo "已添加远程仓库: https://github.com/LIUBANGJIAN/netdrive-clear"
fi

# 添加所有更改
echo "添加更改..."
git add .

# 检查是否有更改
if git diff --cached --quiet; then
    echo "没有新的更改需要提交"
    exit 0
fi

# 提交更改
echo "提交更改..."
git commit -m "更新代码 - $(date '+%Y-%m-%d %H:%M:%S')"

# 推送到 GitHub
echo "推送到 GitHub..."
git push -u origin main --force

echo "同步完成!"
