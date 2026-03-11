chcp 65001
@echo off
REM 网络打印机系统 Windows 编译脚本
REM 支持 GCC/MinGW 编译器

setlocal enabledelayedexpansion

echo ==================================
echo   网络打印机系统 Windows 编译脚本
echo ==================================
echo.

REM 设置项目目录
set PROJECT_DIR=%~dp0
set DRIVER_DIR=%PROJECT_DIR%driver
set BACKEND_DIR=%PROJECT_DIR%backend

REM ==================== 验证编译器 ====================
where gcc >nul 2>&1
if %errorlevel% equ 0 (
    echo [Info] 检测到 GCC 编译器
) else (
    echo [Error] 未找到 GCC 编译器，请安装 MinGW
    exit /b 1
)

where go >nul 2>&1
if %errorlevel% equ 0 (
    echo [Info] 检测到 Go 编译器
) else (
    echo [Error] 未找到 Go 编译器
    echo 请从 https://go.dev/dl 下载并安装
    exit /b 1
)

echo [Info] 编译器检查通过
echo.

REM 处理命令行参数
if "%1"=="driver" goto compile_driver
if "%1"=="backend" goto compile_backend
if "%1"=="all" goto compile_all
if "%1"=="clean" goto clean_all

echo 用法: build.bat [driver^|backend^|all^|clean]
echo.
echo 示例:
echo   build.bat driver      - 仅编译驱动程序
echo   build.bat backend     - 仅编译后端服务
echo   build.bat all         - 编译全部（默认）
echo   build.bat clean       - 清理编译文件
echo.
exit /b 1

REM ==================== 编译驱动 ====================
:compile_driver
echo.
echo [Step 1/2] 编译 C 语言驱动程序...
echo.
cd /d "%DRIVER_DIR%"

if not exist "printer_simulator.c" (
    echo [Error] 找不到源文件: printer_simulator.c
    exit /b 1
)

echo 编译源文件...
gcc -std=c17 -Wall -Wextra -O2 -o printer_driver.exe ^
    printer_simulator.c ^
    driver_server.c ^
    main_driver.c ^
    protocol.c ^
    protocol_handler.c ^
    state_machine.c ^
    -lws2_32 -lpthread

if %errorlevel% neq 0 (
    echo.
    echo [Error] ✗ 驱动程序编译失败
    echo 请检查编译错误信息
    exit /b 1
)

echo [Success] ✓ 驱动程序编译成功：%DRIVER_DIR%\printer_driver.exe
goto end_driver

:end_driver
exit /b 0

REM ==================== 编译后端 ====================
:compile_backend
echo.
echo [Step 2/2] 编译 Go 后端服务...
echo.
cd /d "%BACKEND_DIR%"

if not exist "main.go" (
    echo [Error] 找不到源文件: main.go
    exit /b 1
)

if not exist "go.mod" (
    echo [Error] 找不到 go.mod 文件
    echo 请先运行: go mod init
    exit /b 1
)

echo 下载依赖...
sudo go mod download
if %errorlevel% neq 0 (
    echo [Warning] 依赖下载可能失败，继续编译...
)

echo 整理依赖...
sudo go mod tidy
if %errorlevel% neq 0 (
    echo [Warning] 依赖整理可能失败，继续编译...
)

echo 编译二进制文件...
sudo go build -o printer_backend.exe .
if %errorlevel% neq 0 (
    echo.
    echo [Error] ✗ 后端编译失败
    echo 请检查编译错误信息
    exit /b 1
)

if not exist "printer_backend.exe" (
    echo.
    echo [Error] ✗ 后端编译失败
    echo 请检查编译错误信息
    exit /b 1
)

echo [Success] ✓ 后端编译成功：%BACKEND_DIR%\printer_backend.exe
goto end_backend

:end_backend
exit /b 0

REM ==================== 编译全部 ====================
:compile_all
call :compile_driver
if %errorlevel% neq 0 exit /b 1

call :compile_backend
if %errorlevel% neq 0 exit /b 1

echo.
echo ==================================
echo   ✓ 全部编译完成！
echo ==================================
echo.
echo 下一步操作:
echo   1. 启动脚本：start.bat
echo   或
echo   2. 手动启动驱动程序：%DRIVER_DIR%\printer_driver.exe
echo   3. 手动启动后端服务：%BACKEND_DIR%\printer_backend.exe
echo   4. 打开浏览器访问：http://localhost:8080
echo.
exit /b 0

REM ==================== 清理编译文件 ====================
:clean_all
echo.
echo [Cleaning] 清理编译文件...
echo.

if exist "%DRIVER_DIR%\printer_driver.exe" (
    del /f /q "%DRIVER_DIR%\printer_driver.exe"
    echo ✓ 已删除: printer_driver.exe
)

if exist "%BACKEND_DIR%\printer_backend.exe" (
    del /f /q "%BACKEND_DIR%\printer_backend.exe"
    echo ✓ 已删除: printer_backend.exe
)

echo.
echo [Success] ✓ 清理完成
exit /b 0

