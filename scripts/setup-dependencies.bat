@echo off
setlocal enabledelayedexpansion

:: Engine IM Push Service Google APIs 和 gRPC-Gateway 依赖下载脚本
:: 下载 gRPC-Gateway 所需的依赖 proto 文件到 GOPATH

chcp 65001 >nul

echo 📦 下载 Google APIs 和 gRPC-Gateway 依赖到 GOPATH...

:: 检查 GOPATH
if "%GOPATH%"=="" (
    echo ❌ GOPATH 未设置，请先设置 GOPATH 环境变量
    echo    示例: set GOPATH=C:\Users\%USERNAME%\go
    pause
    exit /b 1
)

:: 配置变量
set "GOOGLEAPIS_VERSION=master"
set "GRPC_GATEWAY_VERSION=v2.19.0"
set "GOPATH_SRC_DIR=%GOPATH%\src\github.com"
set "GOOGLEAPIS_DIR=%GOPATH_SRC_DIR%\googleapis"
set "GRPC_GATEWAY_DIR=%GOPATH_SRC_DIR%\grpc-ecosystem\grpc-gateway"

echo 🔍 GOPATH: %GOPATH%
echo 🎯 目标目录 1: %GOOGLEAPIS_DIR%
echo 🎯 目标目录 2: %GRPC_GATEWAY_DIR%

:: 创建目录
if not exist "%GOPATH_SRC_DIR%" (
    echo 📁 创建目录: %GOPATH_SRC_DIR%
    mkdir "%GOPATH_SRC_DIR%" 2>nul
)

if not exist "%GOPATH_SRC_DIR%\googleapis" (
    echo 📁 创建目录: %GOPATH_SRC_DIR%\googleapis
    mkdir "%GOPATH_SRC_DIR%\googleapis" 2>nul
)

if not exist "%GOPATH_SRC_DIR%\grpc-ecosystem" (
    echo 📁 创建目录: %GOPATH_SRC_DIR%\grpc-ecosystem
    mkdir "%GOPATH_SRC_DIR%\grpc-ecosystem" 2>nul
)

:: 下载 Google APIs
echo 🚀 下载 Google APIs...
:: 检查是否已存在
if exist "%GOOGLEAPIS_DIR%" (
    echo ⚠️  Google APIs 已存在，是否重新下载？ [y/N]
    set /p response=
    if /i not "!response!"=="y" (
        echo ✅ 跳过 Google APIs 下载
        goto download_grpc_gateway
    )
    rmdir /s /q "%GOOGLEAPIS_DIR%" 2>nul
)

:: 检查下载工具
where git >nul 2>nul
if !errorlevel! equ 0 (
    echo 📥 使用 Git 下载 googleapis...
    git clone --depth=1 --branch="%GOOGLEAPIS_VERSION%" https://github.com/googleapis/googleapis.git "%GOOGLEAPIS_DIR%"
    
    if !errorlevel! neq 0 (
        echo ❌ Git 下载 googleapis 失败
        goto error_exit
    )
) else (
    echo ❌ 需要 Git 来下载依赖，请安装 Git
    goto error_exit
)

:download_grpc_gateway
echo 🚀 下载 gRPC-Gateway...
:: 检查是否已存在
if exist "%GRPC_GATEWAY_DIR%" (
    echo ⚠️  gRPC-Gateway 已存在，是否重新下载？ [y/N]
    set /p response=
    if /i not "!response!"=="y" (
        echo ✅ 跳过 gRPC-Gateway 下载
        goto verify_downloads
    )
    rmdir /s /q "%GRPC_GATEWAY_DIR%" 2>nul
)

where git >nul 2>nul
if !errorlevel! equ 0 (
    echo 📥 使用 Git 下载 grpc-gateway...
    git clone --depth=1 --branch="%GRPC_GATEWAY_VERSION%" https://github.com/grpc-ecosystem/grpc-gateway.git "%GRPC_GATEWAY_DIR%"
    
    if !errorlevel! neq 0 (
        echo ❌ Git 下载 grpc-gateway 失败
        goto error_exit
    )
) else (
    echo ❌ 需要 Git 来下载依赖，请安装 Git
    goto error_exit
)

:verify_downloads
:: 验证下载
echo 🔍 验证下载结果...

set "validation_failed=false"

if exist "%GOOGLEAPIS_DIR%\google\api\annotations.proto" (
    echo ✅ Google APIs annotations.proto 存在
) else (
    echo ❌ Google APIs annotations.proto 缺失
    set "validation_failed=true"
)

if exist "%GOOGLEAPIS_DIR%\google\api\http.proto" (
    echo ✅ Google APIs http.proto 存在
) else (
    echo ❌ Google APIs http.proto 缺失
    set "validation_failed=true"
)

if exist "%GRPC_GATEWAY_DIR%\protoc-gen-openapiv2\options\annotations.proto" (
    echo ✅ gRPC-Gateway openapiv2 annotations.proto 存在
) else (
    echo ❌ gRPC-Gateway openapiv2 annotations.proto 缺失
    set "validation_failed=true"
)

if "%validation_failed%"=="true" (
    echo ❌ 依赖验证失败，请检查下载
    goto error_exit
)

echo.
echo ✅ 所有依赖下载完成到 GOPATH！
echo.
echo 📁 已下载的关键文件：
echo   - %GOOGLEAPIS_DIR%\google\api\annotations.proto
echo   - %GOOGLEAPIS_DIR%\google\api\http.proto  
echo   - %GOOGLEAPIS_DIR%\google\protobuf\timestamp.proto
echo   - %GOOGLEAPIS_DIR%\google\protobuf\wrappers.proto
echo   - %GRPC_GATEWAY_DIR%\protoc-gen-openapiv2\options\annotations.proto
echo.
echo 🎉 现在可以使用 gRPC-Gateway 功能了！
echo 💡 提示：generate.bat 将自动从 GOPATH 查找这些文件

echo.
echo ✅ 设置完成！
echo 💡 提示：
echo   - 依赖文件已放置在 GOPATH 下
echo   - 运行 scripts\generate.bat 将自动从 GOPATH 使用这些依赖
echo   - 如需更新依赖，请重新运行此脚本

pause
exit /b 0

:error_exit
echo ❌ 依赖下载失败
pause
exit /b 1