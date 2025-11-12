@echo off
setlocal enabledelayedexpansion

:: {{.ProjectName}} 标签注入脚本
:: 使用 protoc-go-inject-tag 为生成的 Go 结构体注入标签

chcp 65001 >nul

:: 图标定义
set "ICON_SUCCESS=✓"
set "ICON_ERROR=✗"
set "ICON_WARNING=⚠"
set "ICON_INFO=i"

:: 获取项目根目录
cd /d %~dp0..

:: 初始化变量
set "VERBOSE="
set "FORCE="
set "INPUT_DIR=proto"

:: 解析命令行参数
:parse_args
if "%~1"=="" goto main
if /i "%~1"=="--verbose" set "VERBOSE=true" && shift && goto parse_args
if /i "%~1"=="-v" set "VERBOSE=true" && shift && goto parse_args
if /i "%~1"=="--force" set "FORCE=true" && shift && goto parse_args
if /i "%~1"=="-f" set "FORCE=true" && shift && goto parse_args
if /i "%~1"=="--input" set "INPUT_DIR=%~2" && shift && shift && goto parse_args
if /i "%~1"=="-i" set "INPUT_DIR=%~2" && shift && shift && goto parse_args
if /i "%~1"=="--help" goto show_help
if /i "%~1"=="-h" goto show_help

echo [91m%ICON_ERROR% 未知选项: %~1[0m
echo 使用 --help 查看帮助信息
exit /b 1

:show_help
echo [94m标签注入脚本使用说明[0m
echo.
echo [94m用法:[0m
echo   scripts\inject-tags.bat [选项]
echo.
echo [94m选项:[0m
echo   --verbose, -v     显示详细输出
echo   --force, -f       强制执行（忽略检查）
echo   --input, -i DIR   指定输入目录 (默认: proto)
echo   --help, -h        显示帮助信息
echo.
echo [94m功能:[0m
echo   - 自动安装 protoc-go-inject-tag 工具
echo   - 为生成的 .pb.go 文件注入结构体标签
echo   - 支持 JSON、GORM、Validator 等标签
exit /b 0

:: 日志函数
:log_info
echo [94m%ICON_INFO% %~1[0m
exit /b 0

:log_success
echo [92m%ICON_SUCCESS% %~1[0m
exit /b 0

:log_warning
echo [93m%ICON_WARNING% %~1[0m
exit /b 0

:log_error
echo [91m%ICON_ERROR% %~1[0m
exit /b 0

:log_verbose
if "%VERBOSE%"=="true" echo [94m🔍 %~1[0m
exit /b 0

:: 检查 Go 环境
:check_go
where go >nul 2>nul
if !errorlevel! neq 0 (
    call :log_error "Go 未安装，请先安装 Go 环境"
    exit /b 1
)

if "%VERBOSE%"=="true" (
    for /f "tokens=*" %%i in ('go version') do call :log_verbose "Go 版本: %%i"
)
exit /b 0

:: 检查并安装 protoc-go-inject-tag
:install_protoc_go_inject_tag
where protoc-go-inject-tag >nul 2>nul
if !errorlevel! neq 0 (
    call :log_info "安装 protoc-go-inject-tag..."
    
    go install github.com/favadi/protoc-go-inject-tag@latest
    if !errorlevel! neq 0 (
        call :log_error "protoc-go-inject-tag 安装失败"
        call :log_error "请检查网络连接和 Go 环境配置"
        exit /b 1
    ) else (
        call :log_success "protoc-go-inject-tag 安装成功"
    )
) else (
    call :log_verbose "protoc-go-inject-tag 已安装"
)
exit /b 0

:: 检查输入目录
:check_input_dir
if not exist "%INPUT_DIR%" (
    call :log_error "输入目录不存在: %INPUT_DIR%"
    call :log_error "请先运行生成脚本或指定正确的目录"
    exit /b 1
)

call :log_verbose "检查输入目录: %INPUT_DIR%"
exit /b 0

:: 查找 .pb.go 文件
:find_pb_files
set "pb_file_count=0"
for %%f in ("%INPUT_DIR%\*.pb.go") do (
    if not "%%~nf"=="%%~nf_grpc" (
        set /a pb_file_count+=1
        if "%VERBOSE%"=="true" call :log_verbose "  - %%f"
    )
)

if !pb_file_count! equ 0 (
    call :log_warning "在 %INPUT_DIR% 目录中没有找到 .pb.go 文件"
    call :log_warning "请先运行 scripts\generate.bat 生成 protobuf 代码"
    
    if not "%FORCE%"=="true" exit /b 1
    exit /b 1
) else (
    call :log_info "找到 !pb_file_count! 个 .pb.go 文件"
)
exit /b 0

:: 备份原文件
:backup_files
call :log_info "备份原始文件..."

:: 生成备份目录名
for /f "tokens=2 delims==" %%a in ('wmic os get localdatetime /value') do set datetime=%%a
set backup_dir=%INPUT_DIR%\backup_%datetime:~0,8%_%datetime:~8,6%

mkdir "%backup_dir%" 2>nul

:: 复制文件
for %%f in ("%INPUT_DIR%\*.pb.go") do (
    if not "%%~nf"=="%%~nf_grpc" (
        copy "%%f" "%backup_dir%\" >nul
    )
)

call :log_success "文件备份到: %backup_dir%"
echo %backup_dir% > .inject_tags_backup_path
exit /b 0

:: 注入标签
:inject_tags
call :log_info "开始注入结构体标签..."

set "input_pattern=%INPUT_DIR%\*.pb.go"

if "%VERBOSE%"=="true" (
    call :log_verbose "执行命令: protoc-go-inject-tag -input=\"%input_pattern%\""
)

protoc-go-inject-tag -input="%input_pattern%"
if !errorlevel! neq 0 (
    call :log_error "标签注入失败"
    exit /b 1
) else (
    call :log_success "标签注入完成"
)
exit /b 0

:: 验证注入结果
:verify_injection
call :log_info "验证标签注入结果..."

set "has_tags="
for %%f in ("%INPUT_DIR%\*.pb.go") do (
    if not "%%~nf"=="%%~nf_grpc" (
        findstr /c:"json:" /c:"gorm:" /c:"validate:" "%%f" >nul 2>nul
        if !errorlevel! equ 0 (
            set "has_tags=true"
            if "%VERBOSE%"=="true" call :log_verbose "文件 %%f 包含注入的标签"
        )
    )
)

if "%has_tags%"=="true" (
    call :log_success "标签注入验证通过"
) else (
    call :log_warning "未发现注入的标签，请检查 proto 文件中的 @gotags 注释"
)
exit /b 0

:: 显示使用提示
:show_usage_tips
echo.
call :log_info "使用提示："
echo   1. 在 proto 文件中使用 @gotags 注释定义标签
echo   2. 运行 scripts\generate.bat 生成 protobuf 代码
echo   3. 运行此脚本注入结构体标签
echo.
call :log_info "标签示例："
echo   // @gotags: json:"username" gorm:"uniqueIndex" validate:"required"
echo   string username = 1;
echo.
call :log_info "更多信息请查看: proto\README.md"
exit /b 0

:: 主函数
:main
call :log_info "{{.ProjectName}} 标签注入工具启动..."
echo.

:: 检查环境
call :check_go
if errorlevel 1 exit /b 1

call :install_protoc_go_inject_tag
if errorlevel 1 exit /b 1

call :check_input_dir
if errorlevel 1 exit /b 1

:: 查找文件
call :find_pb_files
if errorlevel 1 (
    if not "%FORCE%"=="true" (
        call :show_usage_tips
        exit /b 1
    )
)

:: 备份和注入
call :backup_files
if errorlevel 1 exit /b 1

call :inject_tags
if errorlevel 1 (
    :: 失败时提供恢复选项
    if exist .inject_tags_backup_path (
        set /p backup_path=<.inject_tags_backup_path
        call :log_error "注入失败，可以使用以下命令恢复原文件："
        call :log_error "  copy !backup_path!\*.pb.go %INPUT_DIR%\"
    )
    exit /b 1
)

call :verify_injection
call :log_success "标签注入流程完成 🎉"

:: 清理备份路径文件
del .inject_tags_backup_path 2>nul

call :show_usage_tips
exit /b 0