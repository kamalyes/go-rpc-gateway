@echo off
echo ========================================
echo    Go RPC Gateway 启动脚本
echo ========================================

:: 检查是否存在配置目录
if not exist "logs" mkdir logs
if not exist "config" mkdir config

:: 检查配置文件
if not exist "config\example.yaml" (
    echo 警告: 配置文件 config\example.yaml 不存在
    echo 将使用默认配置启动...
    echo.
)

:: 编译项目
echo 正在编译 Gateway...
go build -o bin\gateway.exe .\cmd\gateway
if errorlevel 1 (
    echo 编译失败!
    pause
    exit /b 1
)

echo 编译完成!
echo.

:: 启动网关
echo 启动 Go RPC Gateway...
echo 使用配置文件: config\example.yaml
echo 日志目录: logs\
echo.

bin\gateway.exe -config config\example.yaml -log-level debug -log-dir logs

pause