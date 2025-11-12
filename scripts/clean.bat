@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

echo 🧹 清理 {{.ProjectName}} 项目...

REM 获取项目根目录
cd /d %~dp0..

echo 🗑️  删除构建文件...
if exist build (
    rmdir /s /q build
    echo ✅ 已删除 build\ 目录
)

echo 🗑️  删除生成的 protobuf 文件...
if exist proto (
    del /q proto\*.pb.go 2>nul
    del /q proto\*_grpc.pb.go 2>nul
    echo ✅ 已删除生成的 .pb.go 文件
)

echo 🗑️  删除数据库文件...
del /q *.db 2>nul
del /q *.sqlite 2>nul
del /q *.sqlite3 2>nul
echo ✅ 已删除数据库文件

echo 🗑️  删除临时文件...
del /q *.tmp 2>nul
del /q *.log 2>nul
del /q Thumbs.db 2>nul
echo ✅ 已删除临时文件

echo 🗑️  清理 Go 模块缓存...
go clean -cache 2>nul
go clean -modcache 2>nul

echo 🗑️  删除测试覆盖率文件...
del /q coverage.out 2>nul
del /q *.cover 2>nul

echo.
echo ✅ 清理完成！
echo.
echo 保留的文件：
echo   - 源代码文件 (*.go)
echo   - 配置文件 (config.yaml)
echo   - Proto 定义文件 (*.proto)
echo   - 脚本文件 (scripts\)
echo   - 文档文件 (README.md, *.md)

pause