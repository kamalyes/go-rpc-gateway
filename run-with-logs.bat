@echo off
rem Gateway日志测试脚本 (Windows版本)

echo 🏗️  构建Gateway主程序...

rem 构建主程序
cd cmd\gateway
go build -o ..\..\bin\gateway.exe .
cd ..\..

rem 创建日志目录
if not exist logs mkdir logs
if not exist bin mkdir bin

echo 🚀 启动Gateway (日志将保存到 logs\ 目录)
echo 按 Ctrl+C 退出

rem 运行Gateway
bin\gateway.exe -log-dir=logs -log-level=info

echo.
echo ✅ Gateway已停止
echo.
echo 📁 查看日志文件:
echo    type logs\gateway.log
echo    或者直接查看 logs\ 目录下的日志文件
pause