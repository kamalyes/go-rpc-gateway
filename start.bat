@echo off
REM go-rpc-gateway 快速启动脚本 (Windows版)

setlocal EnableDelayedExpansion

set APP_NAME=go-rpc-gateway
set VERSION=v1.0.0

echo 🚀 %APP_NAME% %VERSION% 快速启动脚本
echo ==================================

REM 检查Go环境
go version >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ 错误: 未找到Go环境，请先安装Go
    exit /b 1
)

REM 检查当前目录
if not exist "go.mod" (
    echo ❌ 错误: 请在项目根目录下运行此脚本
    exit /b 1
)

REM 下载依赖
echo 📦 下载依赖...
go mod tidy

REM 设置环境变量
if "%APP_ENV%"=="" set APP_ENV=development
echo 🌍 运行环境: %APP_ENV%

REM 根据参数选择配置
set CONFIG_PATH=./config
if "%1"=="dev" (
    set CONFIG_PATH=./config/gateway-dev.yaml
    set APP_ENV=development
) else if "%1"=="development" (
    set CONFIG_PATH=./config/gateway-dev.yaml
    set APP_ENV=development
) else if "%1"=="prod" (
    set CONFIG_PATH=./config/gateway-prod.yaml
    set APP_ENV=production
) else if "%1"=="production" (
    set CONFIG_PATH=./config/gateway-prod.yaml
    set APP_ENV=production
) else if "%1"=="test" (
    set CONFIG_PATH=./config/gateway-test.yaml
    set APP_ENV=test
) else if not "%1"=="" (
    set CONFIG_PATH=%1
)

echo 📄 配置文件: %CONFIG_PATH%

REM 编译并运行
echo 🏗️ 编译并启动服务...
go run cmd/gateway/main.go -config="%CONFIG_PATH%"