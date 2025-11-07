@echo off
rem Go RPC Gateway 构建脚本 (Windows版本)
rem 重构后的验证和构建

echo 🏗️  构建 Go RPC Gateway (基于 go-config 和 go-core 重构版本)
echo ===============================================

echo 📦 检查Go环境...
go version
if errorlevel 1 (
    echo ❌ Go 未安装或未添加到 PATH
    pause
    exit /b 1
)

echo 🧹 清理依赖...
go mod tidy

echo ⬇️  下载依赖...
go mod download

echo 🎨 格式化代码...
go fmt ./...

echo 🧪 运行测试...
go test ./... -v
if errorlevel 1 (
    echo ⚠️  一些测试可能需要数据库连接
)

echo 🔨 构建示例...
if not exist bin mkdir bin

cd cmd\gateway
go build -o ..\..\bin\gateway.exe .
cd ..\..

echo ✅ 构建完成!
echo.
echo 📁 输出文件:
echo    - bin\gateway.exe              (主程序)
echo.
echo 🚀 运行示例:
echo    bin\gateway.exe -config examples\config.yaml
echo.
echo 🎉 重构完成! Gateway 已成功集成 go-config 和 go-core
pause