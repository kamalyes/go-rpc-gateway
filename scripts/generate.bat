@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

echo 🔧 生成 Protobuf 文件...

REM 获取项目根目录
cd /d %~dp0..

REM 检查 GOPATH 是否设置
if "%GOPATH%"=="" (
    echo ❌ GOPATH 环境变量未设置
    echo    请设置 GOPATH 环境变量
    pause
    exit /b 1
)

REM 检查 protoc 是否安装
where protoc >nul 2>nul
if !errorlevel! neq 0 (
    echo ❌ protoc 未安装，请先安装 Protocol Buffers
    echo    下载地址: https://github.com/protocolbuffers/protobuf/releases
    echo    或使用 chocolatey: choco install protoc
    pause
    exit /b 1
) else (
    echo ✅ protoc 已安装
)

REM 检查 Go 环境
where go >nul 2>nul
if !errorlevel! neq 0 (
    echo ❌ Go 未安装，请先安装 Go 环境
    echo    下载地址: https://golang.org/dl/
    pause
    exit /b 1
) else (
    echo ✅ Go 环境已安装
)

echo 🔍 检查必需的 protoc 插件...

set "missing_plugins="
set "all_plugins_found=true"

REM 检查 protoc-gen-go
where protoc-gen-go >nul 2>nul
if !errorlevel! neq 0 (
    echo ❌ protoc-gen-go 未找到
    set "missing_plugins=!missing_plugins! google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    set "all_plugins_found=false"
) else (
    echo ✅ protoc-gen-go 已安装
)

REM 检查 protoc-gen-go-grpc
where protoc-gen-go-grpc >nul 2>nul
if !errorlevel! neq 0 (
    echo ❌ protoc-gen-go-grpc 未找到
    set "missing_plugins=!missing_plugins! google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    set "all_plugins_found=false"
) else (
    echo ✅ protoc-gen-go-grpc 已安装
)

REM 检查 protoc-gen-grpc-gateway
where protoc-gen-grpc-gateway >nul 2>nul
if !errorlevel! neq 0 (
    echo ❌ protoc-gen-grpc-gateway 未找到
    set "missing_plugins=!missing_plugins! github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest"
    set "all_plugins_found=false"
) else (
    echo ✅ protoc-gen-grpc-gateway 已安装
)

REM 检查 protoc-gen-openapiv2
where protoc-gen-openapiv2 >nul 2>nul
if !errorlevel! neq 0 (
    echo ❌ protoc-gen-openapiv2 未找到
    set "missing_plugins=!missing_plugins! github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest"
    set "all_plugins_found=false"
) else (
    echo ✅ protoc-gen-openapiv2 已安装
)

REM 检查 protoc-go-inject-tag
where protoc-go-inject-tag >nul 2>nul
if !errorlevel! neq 0 (
    echo ❌ protoc-go-inject-tag 未找到
    set "missing_plugins=!missing_plugins! github.com/favadi/protoc-go-inject-tag@latest"
    set "all_plugins_found=false"
) else (
    echo ✅ protoc-go-inject-tag 已安装
)

if "!all_plugins_found!"=="false" (
    echo.
    echo 📦 检测到缺失插件，正在安装...
    for %%p in (!missing_plugins!) do (
        echo 安装 %%p
        go install %%p
        if !errorlevel! neq 0 (
            echo ❌ 安装 %%p 失败
            pause
            exit /b 1
        )
    )
    echo ✅ 所有插件安装完成
)

echo ✅ 所有必需插件可用
echo.

REM 检查 proto 目录是否存在
if not exist proto (
    echo ❌ proto 目录不存在，请先创建 proto 目录并添加 .proto 文件
    pause
    exit /b 1
)

REM 检查是否有 proto 文件
dir /b proto\*.proto >nul 2>nul
if !errorlevel! neq 0 (
    echo ❌ proto 目录中没有 .proto 文件
    pause
    exit /b 1
)

REM 检查和设置 Protobuf Include 文件
echo 🔍 检查 Protobuf Include 文件...
for /f "tokens=*" %%i in ('where protoc') do set "PROTOC_PATH_CHECK=%%i"
for %%i in ("%PROTOC_PATH_CHECK%") do set "PROTOC_DIR_CHECK=%%~dpi"
set "PROTOC_INCLUDE_CHECK=%PROTOC_DIR_CHECK%..\include"

if not exist "%PROTOC_INCLUDE_CHECK%\google\protobuf\timestamp.proto" (
    echo 📦 需要设置 Protobuf Include 文件
    echo 🚀 正在自动设置...
    
    if exist "scripts\setup-protobuf-includes.bat" (
        call "scripts\setup-protobuf-includes.bat"
        if !errorlevel! neq 0 (
            echo ❌ Protobuf Include 文件设置失败
            pause
            exit /b 1
        )
    ) else (
        echo ❌ 未找到 Protobuf Include 设置脚本
        echo 💡 请运行: scripts\setup-protobuf-includes.bat
        pause
        exit /b 1
    )
) else (
    echo ✅ Protobuf Include 文件已存在
)

REM 检查是否需要 Google APIs 和 gRPC-Gateway 依赖
findstr /c:"google/api/annotations.proto" /c:"google/protobuf" /c:"protoc-gen-openapiv2" proto\*.proto >nul 2>nul
if !errorlevel! equ 0 (
    echo 🔍 检测到 Google APIs 和 gRPC-Gateway 依赖...
    
    REM 检查 GOPATH 中的 googleapis
    if not exist "%GOPATH%\src\github.com\googleapis\google" (
        echo 📦 需要下载 Google APIs 依赖到 GOPATH
        echo 🚀 正在自动下载...
        
        if exist "scripts\setup-dependencies.bat" (
            call "scripts\setup-dependencies.bat"
            if !errorlevel! neq 0 (
                echo ❌ 依赖下载失败
                pause
                exit /b 1
            )
        ) else (
            echo ❌ 未找到依赖下载脚本
            echo 💡 请运行: scripts\setup-dependencies.bat
            pause
            exit /b 1
        )
    ) else (
        echo ✅ 在 GOPATH 中找到 Google APIs 依赖
    )
    
    REM 检查 GOPATH 中的 grpc-gateway
    if not exist "%GOPATH%\src\github.com\grpc-ecosystem\grpc-gateway" (
        echo 📦 需要下载 gRPC-Gateway 依赖到 GOPATH
        echo 🚀 正在自动下载...
        
        if exist "scripts\setup-dependencies.bat" (
            call "scripts\setup-dependencies.bat"
            if !errorlevel! neq 0 (
                echo ❌ 依赖下载失败
                pause
                exit /b 1
            )
        ) else (
            echo ❌ 未找到依赖下载脚本
            echo 💡 请运行: scripts\setup-dependencies.bat
            pause
            exit /b 1
        )
    ) else (
        echo ✅ 在 GOPATH 中找到 gRPC-Gateway 依赖
    )
)

REM 清理旧的生成文件
echo 🧹 清理旧的生成文件...
del /q proto\*.pb.go 2>nul
del /q proto\*_grpc.pb.go 2>nul
del /q proto\*.gw.go 2>nul
del /q proto\*.swagger.json 2>nul

REM 生成基础 protobuf 和 gRPC 代码
echo 🚀 生成 gRPC 代码...

REM 获取 protoc 安装路径
for /f "tokens=*" %%i in ('where protoc') do set "PROTOC_PATH=%%i"
for %%i in ("%PROTOC_PATH%") do set "PROTOC_DIR=%%~dpi"
set "PROTOC_INCLUDE=%PROTOC_DIR%..\include"

echo 📁 Protoc 路径: %PROTOC_PATH%
echo 📁 Include 路径: %PROTOC_INCLUDE%

REM 设置 googleapis 和 grpc-gateway 路径（从 GOPATH）
set "GOOGLEAPIS_PATH="
set "GRPC_GATEWAY_PATH="

if exist "%GOPATH%\src\github.com\googleapis" (
    set "GOOGLEAPIS_PATH=-I %GOPATH%\src\github.com\googleapis"
    echo ✅ 使用 GOPATH 中的 googleapis
) else (
    echo ⚠️ 警告：GOPATH 中未找到 googleapis
)

if exist "%GOPATH%\src\github.com\grpc-ecosystem\grpc-gateway" (
    set "GRPC_GATEWAY_PATH=-I %GOPATH%\src\github.com\grpc-ecosystem\grpc-gateway"
    echo ✅ 使用 GOPATH 中的 grpc-gateway
) else (
    echo ⚠️ 警告：GOPATH 中未找到 grpc-gateway
)

protoc -I"%PROTOC_INCLUDE%" %GOOGLEAPIS_PATH% %GRPC_GATEWAY_PATH% -I. --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/*.proto

if !errorlevel! neq 0 (
    echo ❌ 基础代码生成失败
    pause
    exit /b 1
)

REM 生成 gRPC-Gateway 代码
echo 🌐 生成 gRPC-Gateway 代码...
protoc -I"%PROTOC_INCLUDE%" %GOOGLEAPIS_PATH% %GRPC_GATEWAY_PATH% -I. --grpc-gateway_out=. --grpc-gateway_opt=paths=source_relative --grpc-gateway_opt=generate_unbound_methods=true proto/*.proto

if !errorlevel! neq 0 (
    echo ⚠️  gRPC-Gateway 代码生成失败，跳过...
)

REM 生成 OpenAPI/Swagger 文档
echo 📖 生成 OpenAPI 文档...
if not exist docs\swagger mkdir docs\swagger
protoc -I"%PROTOC_INCLUDE%" %GOOGLEAPIS_PATH% %GRPC_GATEWAY_PATH% -I. --openapiv2_out=docs\swagger --openapiv2_opt=logtostderr=true --openapiv2_opt=json_names_for_fields=false proto/*.proto

if !errorlevel! neq 0 (
    echo ⚠️  OpenAPI 文档生成失败，跳过...
)

REM 注入标签
echo 🏷️  注入结构体标签...
if exist "scripts\inject-tags.bat" (
    call "scripts\inject-tags.bat" --force
    if !errorlevel! neq 0 (
        echo ❌ 标签注入失败
        pause
        exit /b 1
    )
) else (
    echo ❌ 未找到标签注入脚本
    echo 💡 请运行: scripts\inject-tags.bat
    echo 🔧 或使用简单模式...
    protoc-go-inject-tag -input="proto/*.pb.go"
    if !errorlevel! neq 0 (
        echo ❌ 标签注入失败
        pause
        exit /b 1
    )
)

echo ✅ Protobuf 文件生成完成！
echo ✅ 结构体标签注入完成！
echo.
echo 生成的文件：
echo 📁 Go 文件：
dir /b proto\*.pb.go 2>nul
dir /b proto\*_grpc.pb.go 2>nul
dir /b proto\*.gw.go 2>nul
echo.
if exist docs\swagger (
    echo 📁 Swagger 文档：
    dir /b docs\swagger\*.json 2>nul
)

echo.
echo 📝 下一步：
echo    1. 运行 go mod tidy 更新依赖
echo    2. 运行 go run main.go 启动服务

pause