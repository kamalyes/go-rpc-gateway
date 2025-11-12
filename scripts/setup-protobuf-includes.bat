@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

echo 🔧 设置 Protobuf Include 文件...

REM 获取 protoc 路径
for /f "tokens=*" %%i in ('where protoc') do set "PROTOC_PATH=%%i"
for %%i in ("%PROTOC_PATH%") do set "PROTOC_DIR=%%~dpi"
set "PROTOC_ROOT=%PROTOC_DIR%.."
set "PROTOC_INCLUDE=%PROTOC_ROOT%\include"

echo 📁 Protoc 路径: %PROTOC_PATH%
echo 📁 Protoc 根目录: %PROTOC_ROOT%
echo 📁 Include 目录: %PROTOC_INCLUDE%

REM 创建 include 目录
if not exist "%PROTOC_INCLUDE%" (
    echo 📁 创建 include 目录...
    mkdir "%PROTOC_INCLUDE%"
)

REM 创建 google/protobuf 目录
if not exist "%PROTOC_INCLUDE%\google\protobuf" (
    echo 📁 创建 google\protobuf 目录...
    mkdir "%PROTOC_INCLUDE%\google\protobuf"
)

REM 下载标准 protobuf 文件
echo 📥 下载标准 protobuf 文件...

REM 下载 descriptor.proto
echo 📋 下载 descriptor.proto...
powershell -Command "Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/protocolbuffers/protobuf/main/src/google/protobuf/descriptor.proto' -OutFile '%PROTOC_INCLUDE%\google\protobuf\descriptor.proto'"

REM 下载 timestamp.proto
echo 📋 下载 timestamp.proto...
powershell -Command "Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/protocolbuffers/protobuf/main/src/google/protobuf/timestamp.proto' -OutFile '%PROTOC_INCLUDE%\google\protobuf\timestamp.proto'"

REM 下载 wrappers.proto
echo 📋 下载 wrappers.proto...
powershell -Command "Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/protocolbuffers/protobuf/main/src/google/protobuf/wrappers.proto' -OutFile '%PROTOC_INCLUDE%\google\protobuf\wrappers.proto'"

REM 下载 struct.proto
echo 📋 下载 struct.proto...
powershell -Command "Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/protocolbuffers/protobuf/main/src/google/protobuf/struct.proto' -OutFile '%PROTOC_INCLUDE%\google\protobuf\struct.proto'"

REM 下载 any.proto
echo 📋 下载 any.proto...
powershell -Command "Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/protocolbuffers/protobuf/main/src/google/protobuf/any.proto' -OutFile '%PROTOC_INCLUDE%\google\protobuf\any.proto'"

REM 下载 empty.proto
echo 📋 下载 empty.proto...
powershell -Command "Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/protocolbuffers/protobuf/main/src/google/protobuf/empty.proto' -OutFile '%PROTOC_INCLUDE%\google\protobuf\empty.proto'"

REM 下载 duration.proto
echo 📋 下载 duration.proto...
powershell -Command "Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/protocolbuffers/protobuf/main/src/google/protobuf/duration.proto' -OutFile '%PROTOC_INCLUDE%\google\protobuf\duration.proto'"

REM 下载 field_mask.proto
echo 📋 下载 field_mask.proto...
powershell -Command "Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/protocolbuffers/protobuf/main/src/google/protobuf/field_mask.proto' -OutFile '%PROTOC_INCLUDE%\google\protobuf\field_mask.proto'"

REM 检查是否成功
if exist "%PROTOC_INCLUDE%\google\protobuf\timestamp.proto" (
    echo ✅ 标准 protobuf 文件设置完成！
    echo 📁 Include 路径: %PROTOC_INCLUDE%
) else (
    echo ❌ 设置失败，请手动下载 protobuf 文件
    echo 💡 请访问: https://github.com/protocolbuffers/protobuf
    exit /b 1
)

echo.
echo 🎉 完成！现在可以运行 generate.bat 了

pause