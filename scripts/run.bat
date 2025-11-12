@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

echo 🚀 启动 {{.ProjectName}} 服务...

REM 获取项目根目录
cd /d %~dp0..

REM 检查 go.mod 文件
if not exist go.mod (
    echo ❌ 未找到 go.mod 文件
    echo 请先运行: go mod init github.com/Divine-Dragon-Voyage/engine-im-push-service
    pause
    exit /b 1
)

REM 检查依赖
echo 📦 检查并更新依赖...
go mod tidy
if !errorlevel! neq 0 (
    echo ❌ 依赖更新失败
    pause
    exit /b 1
)

REM 检查是否需要生成 protobuf 文件
if exist proto (
    dir /b proto\*.proto >nul 2>nul
    if !errorlevel! equ 0 (
        dir /b proto\*.pb.go >nul 2>nul
        if !errorlevel! neq 0 (
            echo 🔧 检测到 proto 文件，自动生成 gRPC 代码...
            call scripts\generate.bat
            if !errorlevel! neq 0 (
                echo ❌ protobuf 代码生成失败
                pause
                exit /b 1
            )
        )
    )
)

REM 编译检查
echo 🔍 编译检查...
go build -o nul .
if !errorlevel! neq 0 (
    echo ❌ 编译失败，请检查代码错误
    pause
    exit /b 1
)

REM 启动服务
echo 🌟 启动服务中...
echo 按 Ctrl+C 停止服务
echo ----------------------------------------
go run main.go

pause