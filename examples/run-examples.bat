@echo off
rem Go RPC Gateway Examples Runner (Windows)
rem 批量运行和测试所有示例

setlocal EnableDelayedExpansion

set "EXAMPLES_DIR=%~dp0"
set "ROOT_DIR=%EXAMPLES_DIR%.."

echo 🚀 Go RPC Gateway Examples Runner
echo ==================================
echo.

rem 检查Go环境
go version >nul 2>&1
if errorlevel 1 (
    echo ❌ Go not found. Please install Go 1.21 or later.
    pause
    exit /b 1
)

for /f "tokens=*" %%i in ('go version') do set "GO_VERSION=%%i"
echo 📦 Go version: !GO_VERSION!
echo 📁 Examples directory: %EXAMPLES_DIR%
echo 📁 Root directory: %ROOT_DIR%
echo.

rem 构建主程序
echo 🔨 Building main gateway...
cd /d "%ROOT_DIR%"
if not exist "bin" mkdir bin

cd cmd\gateway
go build -o ..\..\bin\gateway.exe .
cd /d "%ROOT_DIR%"

if exist "bin\gateway.exe" (
    echo ✅ Main gateway built successfully
) else (
    echo ❌ Failed to build main gateway
    pause
    exit /b 1
)
echo.

rem 主逻辑
if "%1"=="test" goto :test_all
if "%1"=="run" goto :run_specific
if "%1"=="list" goto :list_examples
if "%1"=="help" goto :show_help
if "%1"=="-h" goto :show_help
if "%1"=="--help" goto :show_help
if "%1"=="" goto :show_help

echo ❌ Unknown option: %1
echo.
goto :show_help

:test_all
echo 🧪 Testing all examples...
echo =========================
echo.

set "success_count=0"
set "total_count=0"
set "failed_examples="

for /d %%d in ("%EXAMPLES_DIR%\*") do (
    set "example_name=%%~nd"
    set "example_dir=%%d"
    
    rem 只处理以数字开头的目录
    echo !example_name! | findstr "^[0-9]" >nul
    if not errorlevel 1 (
        set /a total_count+=1
        call :run_example "!example_name!" "!example_dir!"
        if !errorlevel! equ 0 (
            set /a success_count+=1
        ) else (
            if "!failed_examples!"=="" (
                set "failed_examples=!example_name!"
            ) else (
                set "failed_examples=!failed_examples!, !example_name!"
            )
        )
        echo.
    )
)

echo 📊 Test Results
echo ===============
echo ✅ Successful: !success_count!/!total_count!

if not "!failed_examples!"=="" (
    echo ❌ Failed examples: !failed_examples!
    pause
    exit /b 1
) else (
    echo 🎉 All examples passed!
)

pause
exit /b 0

:run_specific
if "%2"=="" (
    echo ❌ Please specify an example name
    echo Use '%0 list' to see available examples
    pause
    exit /b 1
)

set "example_name=%2"
set "example_dir=%EXAMPLES_DIR%\%example_name%"

echo 🎯 Running specific example: %example_name%
echo ==================================

if not exist "%example_dir%" (
    echo ❌ Example not found: %example_name%
    echo Available examples:
    for /d %%d in ("%EXAMPLES_DIR%\*") do (
        set "name=%%~nd"
        echo !name! | findstr "^[0-9]" >nul
        if not errorlevel 1 echo   - !name!
    )
    pause
    exit /b 1
)

cd /d "%example_dir%"

echo 📍 Current directory: %CD%
echo 📦 Building and running...
echo.

go run main.go
pause
exit /b 0

:list_examples
echo 📚 Available Examples
echo ====================
echo.

for /d %%d in ("%EXAMPLES_DIR%\*") do (
    set "example_name=%%~nd"
    set "example_dir=%%d"
    
    rem 只处理以数字开头的目录
    echo !example_name! | findstr "^[0-9]" >nul
    if not errorlevel 1 (
        echo 📁 !example_name!
        
        rem 尝试读取描述
        set "main_file=!example_dir!\main.go"
        if exist "!main_file!" (
            for /f "tokens=*" %%l in ('findstr /c:"Description:" "!main_file!" 2^>nul ^| head -1') do (
                set "line=%%l"
                set "description=!line:*Description:=!"
                set "description=!description: *=!"
                if not "!description!"=="" echo    📝 !description!
            )
        )
        echo.
    )
)

pause
exit /b 0

:run_example
set "name=%~1"
set "dir=%~2"

if not exist "%dir%" (
    echo ❌ Example directory not found: %dir%
    exit /b 1
)

echo 🔄 Running example: %name%
echo    Directory: %dir%

cd /d "%dir%"

if not exist "main.go" (
    echo ❌ main.go not found in %dir%
    exit /b 1
)

echo    🔨 Building...
go build -o "example_%name%.exe" main.go
if errorlevel 1 (
    echo    ❌ Build failed for %name%
    exit /b 1
)

echo    ✅ Build successful
echo    🧪 Build test passed

if exist "example_%name%.exe" del "example_%name%.exe"

echo    ✅ Example %name% verified successfully
exit /b 0

:show_help
echo Usage: %0 [OPTION] [EXAMPLE_NAME]
echo.
echo Options:
echo   test              Test all examples (build verification)
echo   run ^<example^>     Run a specific example
echo   list              List all available examples
echo   help              Show this help message
echo.
echo Examples:
echo   %0 test                           # Test all examples
echo   %0 run 01-quickstart            # Run quickstart example
echo   %0 run 04-pprof                 # Run pprof example
echo   %0 list                          # List all examples
echo.
echo Available examples:
for /d %%d in ("%EXAMPLES_DIR%\*") do (
    set "name=%%~nd"
    echo !name! | findstr "^[0-9]" >nul
    if not errorlevel 1 echo   - !name!
)

pause
exit /b 0