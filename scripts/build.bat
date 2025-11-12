@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

echo 🔨 构建 {{.ProjectName}} 项目...

REM 获取项目根目录
cd /d %~dp0..

REM 设置构建变量
set APP_NAME={{.ProjectName}}
set VERSION=1.0.0

REM 获取当前时间
for /f "tokens=1-4 delims=/- " %%a in ('date /t') do (
    set BUILD_DATE=%%a-%%b-%%c
)
for /f "tokens=1-2 delims=: " %%a in ('time /t') do (
    set BUILD_TIME=%%a:%%b
)
set BUILD_TIME=%BUILD_DATE%_%BUILD_TIME%

REM 尝试获取 Git 提交信息
set GIT_COMMIT=unknown
where git >nul 2>nul
if !errorlevel! equ 0 (
    if exist .git (
        for /f %%i in ('git rev-parse --short HEAD 2^>nul') do set GIT_COMMIT=%%i
    )
)

REM 构建标志
set LDFLAGS=-w -s -X main.Version=%VERSION% -X main.BuildTime=%BUILD_TIME%
if not "%GIT_COMMIT%"=="unknown" (
    set LDFLAGS=%LDFLAGS% -X main.GitCommit=%GIT_COMMIT%
)

echo 📦 更新依赖...
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
        echo 🔧 生成 protobuf 文件...
        call scripts\generate.bat
        if !errorlevel! neq 0 (
            echo ❌ protobuf 生成失败
            pause
            exit /b 1
        )
    )
)

echo 🏗️  编译项目...
echo    应用名称: %APP_NAME%
echo    版本: %VERSION%
echo    构建时间: %BUILD_TIME%
echo    Git 提交: %GIT_COMMIT%

REM 创建构建目录
if not exist build mkdir build

REM 构建当前平台
echo 🎯 构建当前平台...
go build -ldflags "%LDFLAGS%" -o "build\%APP_NAME%.exe" .
if !errorlevel! neq 0 (
    echo ❌ 构建失败
    pause
    exit /b 1
)

REM 构建多平台（可选）
if "%1"=="--all" (
    echo 🌍 构建多平台版本...
    
    echo 构建 linux/amd64...
    set GOOS=linux
    set GOARCH=amd64
    go build -ldflags "%LDFLAGS%" -o "build\%APP_NAME%-linux-amd64" .
    
    echo 构建 windows/amd64...
    set GOOS=windows
    set GOARCH=amd64
    go build -ldflags "%LDFLAGS%" -o "build\%APP_NAME%-windows-amd64.exe" .
    
    echo 构建 darwin/amd64...
    set GOOS=darwin
    set GOARCH=amd64
    go build -ldflags "%LDFLAGS%" -o "build\%APP_NAME%-darwin-amd64" .
    
    echo 构建 darwin/arm64...
    set GOOS=darwin
    set GOARCH=arm64
    go build -ldflags "%LDFLAGS%" -o "build\%APP_NAME%-darwin-arm64" .
    
    REM 重置环境变量
    set GOOS=
    set GOARCH=
)

echo.
echo ✅ 构建完成！
echo 构建文件位于 build/ 目录：
dir /b build\

echo.
echo 🚀 运行方式：
echo    .\build\%APP_NAME%.exe

pause