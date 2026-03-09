chcp 65001
@echo off
REM 网络打印机系统 Windows 编译脚本
REM 支持 Visual Studio 和 MinGW

setlocal enabledelayedexpansion

echo ==================================
echo   网络打印机系统 Windows 编译脚本
echo ==================================
echo.

REM 设置项目目录
set PROJECT_DIR=%~dp0
set DRIVER_DIR=%PROJECT_DIR%driver
set BACKEND_DIR=%PROJECT_DIR%backend

REM 检查编译器
where gcc >nul 2>&1
if %errorlevel% equ 0 (
    echo [Info] 检测到 GCC 编译器
    set COMPILER=gcc
) else (
    echo [Error] 未找到 GCC 编译器，请安装 MinGW
    exit /b 1
)

REM 处理命令行参数
if "%1"=="driver" goto compile_driver
if "%1"=="backend" goto compile_backend
if "%1"=="all" goto compile_all
if "%1"=="clean" goto clean_all

echo 用法: build.bat [driver^|backend^|all^|clean]
exit /b 1

REM ==================== 编译驱动 ====================
:compile_driver
echo.
echo [1/2] 编译 C 语言驱动程序...
cd /d "%DRIVER_DIR%"

echo 编译源文件...
%COMPILER% -std=c99 -Wall -Wextra -o printer_driver.exe ^
    printer_simulator.c ^
    driver_server.c ^
    main_driver.c ^
    -lws2_32

if %errorlevel% neq 0 (
    echo [Error] 驱动程序编译失败
    exit /b 1
)

echo [Success] 驱动程序编译成功！
exit /b 0

REM ==================== 编译后端 ====================
:compile_backend
echo.
echo [2/2] 编译 Go 后端服务...
cd /d "%BACKEND_DIR%"

echo 下载依赖...
call sudo go mod download
if %errorlevel% neq 0 (
    echo [Error] 依赖下载失败
    exit /b 1
)

echo 编译后端...
call go build -o printer_backend.exe main.go
if %errorlevel% neq 0 (
    echo [Error] 后端编译失败
    exit /b 1
)

echo [Success] 后端编译成功！
exit /b 0

REM ==================== 编译全部 ====================
:compile_all
call :compile_driver
if %errorlevel% neq 0 exit /b 1
call :compile_backend
if %errorlevel% neq 0 exit /b 1

echo.
echo 启动步骤:
echo   1. 启动驱动程序：%DRIVER_DIR%\printer_driver.exe
echo   2. 启动后端服务：%BACKEND_DIR%\printer_backend.exe
echo   3. 打开浏览器访问：http://localhost:8080
echo.
exit /b 0

REM ==================== 清理 ====================
:clean_all
echo.
echo [Cleaning] 清理编译文件...

cd /d "%DRIVER_DIR%"
if exist "printer_driver.exe" del printer_driver.exe
if exist "printer_driver.o" del printer_driver.o

cd /d "%BACKEND_DIR%"
if exist "printer_backend.exe" del printer_backend.exe

echo [Success] 清理完成！
exit /b 0

