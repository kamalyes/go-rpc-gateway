@echo off
REM 构建并运行带pprof的Gateway示例

echo 🚀 Building Gateway with PProf integration...

REM 设置环境变量
set PPROF_TOKEN=gateway-debug-2024

REM 构建项目
echo 📦 Running go mod tidy...
go mod tidy

echo 📦 Building gateway-pprof example...
cd cmd\gateway-pprof
go build -o ..\..\bin\gateway-pprof.exe .

if %ERRORLEVEL% equ 0 (
    echo ✅ Build successful!
    echo.
    echo 🔧 To run the example:
    echo    .\bin\gateway-pprof.exe
    echo.
    echo 📊 Then access:
    echo    Web UI: http://localhost:8080/
    echo    Health: http://localhost:8080/health
    echo    PProf:  http://localhost:8080/debug/pprof/?token=%PPROF_TOKEN%
    echo.
    echo 💡 Authentication token: %PPROF_TOKEN%
) else (
    echo ❌ Build failed!
    exit /b 1
)