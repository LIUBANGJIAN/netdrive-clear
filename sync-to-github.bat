@echo off
chcp 65001 >nul
echo 开始同步代码到 GitHub...
cd /d "%~dp0"

REM 检查是否已初始化 Git
if not exist ".git" (
    echo 首次运行，正在初始化 Git 仓库...
    git init
    git remote add origin https://github.com/LIUBANGJIAN/netdrive-clear.git
    echo 已添加远程仓库: https://github.com/LIUBANGJIAN/netdrive-clear
)

REM 添加所有更改
echo 添加更改...
git add .

REM 检查是否有更改
git diff --cached --quiet
if %errorlevel% equ 0 (
    echo 没有新的更改需要提交
    exit /b 0
)

REM 提交更改
echo 提交更改...
for /f "delims=" %%i in ('date /t') do set date_now=%%i
for /f "delims=" %%i in ('time /t') do set time_now=%%i
git commit -m "更新代码 - %date_now% %time_now%"

REM 推送到 GitHub
echo 推送到 GitHub...
git push -u origin main --force

echo 同步完成!
pause
