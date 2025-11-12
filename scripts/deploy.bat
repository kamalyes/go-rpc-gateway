@echo off
setlocal enabledelayedexpansion

:: {{.ProjectName}} 部署脚本
:: 用于 Windows 服务管理

:: 图标定义
set "ICON_SUCCESS=✓"
set "ICON_ERROR=✗"
set "ICON_WARNING=⚠"
set "ICON_INFO=i"
set "ICON_ROCKET=→"
set "ICON_DEPLOY=D"
set "ICON_STOP=S"
set "ICON_STATUS=?"

:: 获取项目根目录
set "PROJECT_ROOT=%~dp0.."
cd /d "%PROJECT_ROOT%"

:: 配置变量
set "APP_NAME={{.ProjectName}}"
set "SERVICE_NAME={{.ProjectName}}_service"
set "BUILD_DIR=build"
set "SERVICE_PORT=8080"

:: 如果未设置，使用默认值
if "%DEPLOY_DIR%"=="" set "DEPLOY_DIR=C:\Program Files\%APP_NAME%"
if "%LOG_DIR%"=="" set "LOG_DIR=C:\ProgramData\%APP_NAME%\logs"

:: 显示帮助信息
:show_help
echo [94m%APP_NAME% Windows 部署工具[0m
echo.
echo [96m用法:[0m
echo   scripts\deploy.bat ^<命令^> [选项]
echo.
echo [96m命令:[0m
echo   install           %ICON_DEPLOY% 安装 Windows 服务
echo   start             %ICON_ROCKET% 启动服务
echo   stop              %ICON_STOP%  停止服务
echo   restart           R 重启服务
echo   status            %ICON_STATUS% 查看服务状态
echo   uninstall         U 卸载服务
echo   help              %ICON_INFO%  显示帮助信息
echo.
echo [96m选项:[0m
echo   --port PORT       指定服务端口 (默认: 8080)
echo   --deploy-dir DIR  指定部署目录
echo   --log-dir DIR     指定日志目录
echo   --force           强制执行
echo.
echo [93m%ICON_INFO% 注意: 某些操作需要管理员权限[0m
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

:: 检查管理员权限
:check_admin
net session >nul 2>&1
if errorlevel 1 (
    call :log_error "此操作需要管理员权限，请以管理员身份运行"
    exit /b 1
)
exit /b 0

:: 检查构建文件
:check_build
if not exist "%BUILD_DIR%\%APP_NAME%.exe" (
    call :log_error "构建文件不存在，请先运行 scripts\build.bat"
    exit /b 1
)
exit /b 0

:: 创建目录
:create_directories
call :log_info "创建必要目录..."

if not exist "%DEPLOY_DIR%" mkdir "%DEPLOY_DIR%"
if not exist "%LOG_DIR%" mkdir "%LOG_DIR%"

if errorlevel 1 (
    call :log_error "创建目录失败"
    exit /b 1
)

call :log_success "目录创建完成"
exit /b 0

:: 检查服务是否存在
:service_exists
sc query "%SERVICE_NAME%" >nul 2>&1
exit /b %errorlevel%

:: 检查服务是否运行
:service_running
for /f "tokens=3" %%a in ('sc query "%SERVICE_NAME%" ^| findstr "STATE"') do (
    if "%%a"=="RUNNING" exit /b 0
)
exit /b 1

:: 安装服务
:install_service
call :log_info "安装 %APP_NAME% Windows 服务..."

call :check_admin
if errorlevel 1 exit /b 1

call :check_build
if errorlevel 1 exit /b 1

call :create_directories
if errorlevel 1 exit /b 1

:: 复制文件
call :log_info "复制应用程序到 %DEPLOY_DIR%"
copy "%BUILD_DIR%\%APP_NAME%.exe" "%DEPLOY_DIR%\" >nul
if errorlevel 1 (
    call :log_error "复制应用程序失败"
    exit /b 1
)

:: 复制配置文件
if exist "config.yaml" (
    call :log_info "复制配置文件"
    copy "config.yaml" "%DEPLOY_DIR%\" >nul
)

:: 创建服务
call :log_info "创建 Windows 服务..."
sc create "%SERVICE_NAME%" ^
    binPath= "\"%DEPLOY_DIR%\%APP_NAME%.exe\"" ^
    start= auto ^
    DisplayName= "%APP_NAME% gRPC Gateway Service" ^
    depend= "Tcpip" >nul

if errorlevel 1 (
    call :log_error "创建服务失败"
    exit /b 1
)

call :log_success "%APP_NAME% Windows 服务安装完成"
call :log_info "服务名称: %SERVICE_NAME%"
call :log_info "使用以下命令管理服务:"
call :log_info "  启动: net start %SERVICE_NAME%"
call :log_info "  停止: net stop %SERVICE_NAME%"
call :log_info "  状态: sc query %SERVICE_NAME%"
exit /b 0

:: 启动服务
:start_service
call :log_info "启动 %APP_NAME% 服务..."

call :service_exists
if errorlevel 1 (
    call :log_error "服务未安装，请先运行 install 命令"
    exit /b 1
)

call :service_running
if not errorlevel 1 (
    call :log_warning "服务已在运行"
    exit /b 0
)

net start "%SERVICE_NAME%" >nul 2>&1
if errorlevel 1 (
    call :log_error "%APP_NAME% 服务启动失败"
    sc query "%SERVICE_NAME%"
    exit /b 1
) else (
    call :log_success "%APP_NAME% 服务启动成功"
    call :show_service_status
)
exit /b 0

:: 停止服务
:stop_service
call :log_info "停止 %APP_NAME% 服务..."

call :service_exists
if errorlevel 1 (
    call :log_warning "服务未安装"
    exit /b 0
)

call :service_running
if errorlevel 1 (
    call :log_warning "服务未运行"
    exit /b 0
)

net stop "%SERVICE_NAME%" >nul 2>&1
if errorlevel 1 (
    call :log_error "%APP_NAME% 服务停止失败"
    exit /b 1
) else (
    call :log_success "%APP_NAME% 服务已停止"
)
exit /b 0

:: 重启服务
:restart_service
call :log_info "重启 %APP_NAME% 服务..."

call :stop_service
timeout /t 2 >nul
call :start_service
exit /b %errorlevel%

:: 显示服务状态
:show_service_status
echo.
echo [96m服务状态:[0m
sc query "%SERVICE_NAME%"
exit /b 0

:: 卸载服务
:uninstall_service
call :log_info "卸载 %APP_NAME% Windows 服务..."

call :check_admin
if errorlevel 1 exit /b 1

call :service_exists
if errorlevel 1 (
    call :log_warning "服务未安装"
    goto cleanup_files
)

:: 停止服务
call :service_running
if not errorlevel 1 (
    call :stop_service
    timeout /t 2 >nul
)

:: 删除服务
sc delete "%SERVICE_NAME%" >nul 2>&1
if errorlevel 1 (
    call :log_error "删除服务失败"
    exit /b 1
) else (
    call :log_success "Windows 服务已删除"
)

:cleanup_files
:: 删除文件（可选）
if "%FORCE%"=="true" (
    call :log_warning "删除部署文件..."
    if exist "%DEPLOY_DIR%" rmdir /s /q "%DEPLOY_DIR%"
    if exist "%LOG_DIR%" rmdir /s /q "%LOG_DIR%"
) else (
    call :log_info "部署文件保留在: %DEPLOY_DIR%"
    call :log_info "日志文件保留在: %LOG_DIR%"
    call :log_info "使用 --force 选项可完全删除"
)

call :log_success "%APP_NAME% 服务已卸载"
exit /b 0

:: 解析参数
set "COMMAND="
set "FORCE="

:parse_args
if "%~1"=="" goto check_command
if /i "%~1"=="install" set "COMMAND=install" && shift && goto parse_args
if /i "%~1"=="start" set "COMMAND=start" && shift && goto parse_args
if /i "%~1"=="stop" set "COMMAND=stop" && shift && goto parse_args
if /i "%~1"=="restart" set "COMMAND=restart" && shift && goto parse_args
if /i "%~1"=="status" set "COMMAND=status" && shift && goto parse_args
if /i "%~1"=="uninstall" set "COMMAND=uninstall" && shift && goto parse_args
if /i "%~1"=="help" goto show_help
if /i "%~1"=="--port" set "SERVICE_PORT=%~2" && shift && shift && goto parse_args
if /i "%~1"=="--deploy-dir" set "DEPLOY_DIR=%~2" && shift && shift && goto parse_args
if /i "%~1"=="--log-dir" set "LOG_DIR=%~2" && shift && shift && goto parse_args
if /i "%~1"=="--force" set "FORCE=true" && shift && goto parse_args

call :log_error "未知选项: %~1"
goto show_help

:check_command
if "%COMMAND%"=="" goto show_help

:: 执行命令
if "%COMMAND%"=="install" goto install_service
if "%COMMAND%"=="start" goto start_service
if "%COMMAND%"=="stop" goto stop_service
if "%COMMAND%"=="restart" goto restart_service
if "%COMMAND%"=="status" goto show_service_status
if "%COMMAND%"=="uninstall" goto uninstall_service

call :log_error "未知命令: %COMMAND%"
goto show_help