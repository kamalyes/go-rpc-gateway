@echo off
setlocal enabledelayedexpansion

:: {{.ProjectName}} 开发工具脚本 v1.0
:: 提供项目全生命周期管理功能

:: 图标定义（使用 Unicode 字符）
set "ICON_SUCCESS=✓"
set "ICON_ERROR=✗"
set "ICON_WARNING=⚠"
set "ICON_INFO=i"
set "ICON_ROCKET=→"
set "ICON_GEAR=⚙"
set "ICON_CLEAN=⌫"
set "ICON_TEST=T"
set "ICON_BUILD=B"

:: 获取项目根目录
set "PROJECT_ROOT=%~dp0.."
cd /d "%PROJECT_ROOT%"

:: 初始化变量
set "VERBOSE="
set "QUIET="
set "FORCE="
set "COVERAGE="
set "BENCH="
set "ALL="
set "COMMAND="

:: 解析命令行参数
:parse_args
if "%~1"=="" goto check_command
if /i "%~1"=="gen" set "COMMAND=generate" && shift && goto parse_args
if /i "%~1"=="generate" set "COMMAND=generate" && shift && goto parse_args
if /i "%~1"=="tags" set "COMMAND=inject" && shift && goto parse_args
if /i "%~1"=="inject" set "COMMAND=inject" && shift && goto parse_args
if /i "%~1"=="setup" set "COMMAND=setup" && shift && goto parse_args
if /i "%~1"=="deps" set "COMMAND=setup" && shift && goto parse_args
if /i "%~1"=="run" set "COMMAND=run" && shift && goto parse_args
if /i "%~1"=="start" set "COMMAND=run" && shift && goto parse_args
if /i "%~1"=="build" set "COMMAND=build" && shift && goto parse_args
if /i "%~1"=="test" set "COMMAND=test" && shift && goto parse_args
if /i "%~1"=="clean" set "COMMAND=clean" && shift && goto parse_args
if /i "%~1"=="help" goto show_help
if /i "%~1"=="h" goto show_help
if /i "%~1"=="-h" goto show_help
if /i "%~1"=="--help" goto show_help
if /i "%~1"=="--verbose" set "VERBOSE=true" && shift && goto parse_args
if /i "%~1"=="-v" set "VERBOSE=true" && shift && goto parse_args
if /i "%~1"=="--quiet" set "QUIET=true" && shift && goto parse_args
if /i "%~1"=="-q" set "QUIET=true" && shift && goto parse_args
if /i "%~1"=="--force" set "FORCE=true" && shift && goto parse_args
if /i "%~1"=="-f" set "FORCE=true" && shift && goto parse_args
if /i "%~1"=="--coverage" set "COVERAGE=true" && shift && goto parse_args
if /i "%~1"=="--bench" set "BENCH=true" && shift && goto parse_args
if /i "%~1"=="--all" set "ALL=true" && shift && goto parse_args

echo [91m%ICON_ERROR% 未知选项: %~1[0m
echo 使用 'scripts\dev.bat help' 查看帮助
exit /b 1

:check_command
if "%COMMAND%"=="" goto show_help
goto main_logic

:: 显示帮助信息
:show_help
echo [94m{{.ProjectName}} 开发工具脚本[0m
echo.
echo [96m用法:[0m
echo   scripts\dev.bat ^<命令^> [选项]
echo.
echo [96m命令:[0m
echo   gen, generate     %ICON_GEAR%  生成 Protobuf 代码
echo   tags, inject      🏷   注入结构体标签
echo   setup, deps       📦  下载 Google APIs 依赖
echo   run, start        %ICON_ROCKET% 启动开发服务
echo   build             %ICON_BUILD% 构建项目
echo   test              %ICON_TEST%  运行测试
echo   clean             %ICON_CLEAN% 清理项目文件
echo   help, h           %ICON_INFO%  显示此帮助信息
echo.
echo [96m选项:[0m
echo   --verbose, -v     显示详细输出
echo   --quiet, -q       静默模式
echo   --force, -f       强制执行（跳过检查）
echo.
echo [96m示例:[0m
echo   scripts\dev.bat generate       # 生成 protobuf 代码
echo   scripts\dev.bat tags           # 注入结构体标签
echo   scripts\dev.bat run --verbose  # 启动服务并显示详细日志
echo   scripts\dev.bat build --force  # 强制重新构建
echo   scripts\dev.bat test --coverage # 运行测试并生成覆盖率
echo.
echo [93m%ICON_INFO% 更多信息请查看 scripts\README.md[0m
exit /b 0

:: 日志函数
:log_info
if not "%QUIET%"=="true" echo [94m%ICON_INFO% %~1[0m
exit /b 0

:log_success
if not "%QUIET%"=="true" echo [92m%ICON_SUCCESS% %~1[0m
exit /b 0

:log_warning
echo [93m%ICON_WARNING% %~1[0m
exit /b 0

:log_error
echo [91m%ICON_ERROR% %~1[0m
exit /b 0

:: 检查脚本是否存在
:check_script
set "script_name=%~1"
set "script_path=scripts\%script_name%.bat"

if not exist "%script_path%" (
    call :log_error "脚本 %script_path% 不存在"
    exit /b 1
)

exit /b 0

:: 执行脚本
:execute_script
set "script_name=%~1"
set "script_args=%~2"

call :check_script "%script_name%"
if errorlevel 1 exit /b 1

call :log_info "执行 %script_name% 脚本..."

:: 构建命令行参数
set "args=%script_args%"
if "%VERBOSE%"=="true" set "args=!args! --verbose"
if "%QUIET%"=="true" set "args=!args! --quiet"
if "%FORCE%"=="true" set "args=!args! --force"

:: 执行脚本
call "scripts\%script_name%.bat" %args%
if errorlevel 1 (
    call :log_error "%script_name% 执行失败"
    exit /b 1
) else (
    call :log_success "%script_name% 执行成功"
    exit /b 0
)

:: 主逻辑
:main_logic
if "%COMMAND%"=="generate" (
    call :execute_script "generate" ""
) else if "%COMMAND%"=="inject" (
    call :execute_script "inject-tags" ""
) else if "%COMMAND%"=="setup" (
    call :execute_script "setup-googleapis" ""
) else if "%COMMAND%"=="run" (
    call :execute_script "run" ""
) else if "%COMMAND%"=="build" (
    set "args="
    if "%ALL%"=="true" set "args=--all"
    call :execute_script "build" "!args!"
) else if "%COMMAND%"=="test" (
    set "args="
    if "%COVERAGE%"=="true" set "args=!args! --coverage"
    if "%BENCH%"=="true" set "args=!args! --bench"
    call :execute_script "test" "!args!"
) else if "%COMMAND%"=="clean" (
    call :execute_script "clean" ""
) else (
    call :log_error "未知命令: %COMMAND%"
    goto show_help
)

if errorlevel 1 exit /b 1

call :log_success "开发工具脚本执行完成 %ICON_ROCKET%"
exit /b 0