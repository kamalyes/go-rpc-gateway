@echo off
setlocal enabledelayedexpansion
chcp 65001 >nul

echo 🧪 运行 {{.ProjectName}} 测试...

REM 获取项目根目录
cd /d %~dp0..

REM 检查是否需要生成 protobuf 文件
if exist proto (
    dir /b proto\*.proto >nul 2>nul
    if !errorlevel! equ 0 (
        dir /b proto\*.pb.go >nul 2>nul
        if !errorlevel! neq 0 (
            echo 🔧 生成 protobuf 文件...
            call scripts\generate.bat
            if !errorlevel! neq 0 (
                echo ❌ protobuf 生成失败
                pause
                exit /b 1
            )
        )
    )
)

echo 📦 更新依赖...
go mod tidy
if !errorlevel! neq 0 (
    echo ❌ 依赖更新失败
    pause
    exit /b 1
)

echo 🔍 运行 go vet 检查...
go vet .\...
if !errorlevel! neq 0 (
    echo ❌ go vet 检查失败
    pause
    exit /b 1
)

echo 🧪 运行单元测试...
if "%1"=="--coverage" (
    echo 📊 包含覆盖率统计...
    go test -v -race -coverprofile=coverage.out .\...
    if !errorlevel! neq 0 (
        echo ❌ 测试失败
        pause
        exit /b 1
    )
    
    echo 📋 生成覆盖率报告...
    go tool cover -html=coverage.out -o coverage.html
    echo ✅ 覆盖率报告已生成: coverage.html
    
    echo 📊 覆盖率统计：
    go tool cover -func=coverage.out | findstr /E "total:"
) else if "%1"=="--bench" (
    echo ⚡ 运行性能测试...
    go test -v -bench=. -benchmem .\...
) else (
    go test -v -race .\...
    if !errorlevel! neq 0 (
        echo ❌ 测试失败
        pause
        exit /b 1
    )
)

echo.
echo ✅ 测试完成！

REM 提供一些有用的测试命令提示
echo.
echo 💡 其他测试命令：
echo    scripts\test.bat --coverage    # 生成覆盖率报告
echo    scripts\test.bat --bench       # 运行性能测试
echo    go test -short .\...           # 跳过长时间运行的测试
echo    go test -run TestSpecific       # 运行特定测试

pause