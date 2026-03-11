@echo off
REM 网络打印机系统启动脚本 (Windows)
REM 功能：自动检测依赖、启动驱动和后端、打开浏览器

chcp 65001 > nul
setlocal enabledelayedexpansion

echo.
echo ================================
echo   网络打印机系统启动脚本 v2.0
echo ================================
echo.

set PROJECT_DIR=%~dp0
set DRIVER_DIR=%PROJECT_DIR%driver
set BACKEND_DIR=%PROJECT_DIR%backend
set DRIVER_PORT=9999
set BACKEND_PORT=8080
set BACKEND_URL=http://localhost:%BACKEND_PORT%

REM ==================== 检查编译文件 ====================
echo [1/5] 检查编译文件...
if not exist "%DRIVER_DIR%\printer_driver.exe" (
    echo [Error] 找不到驱动程序: %DRIVER_DIR%\printer_driver.exe
    echo.
    echo 请先运行编译脚本:
    echo   build.bat all
    echo.
    pause
    exit /b 1
)

if not exist "%BACKEND_DIR%\printer_backend.exe" (
    echo [Error] 找不到后端程序: %BACKEND_DIR%\printer_backend.exe
    echo.
    echo 请先运行编译脚本:
    echo   build.bat all
    echo.
    pause
    exit /b 1
)

echo [OK] ✓ 所有编译文件都已找到
echo.

REM ==================== 检查端口占用 ====================
echo [2/5] 检查端口占用情况...

netstat -ano | findstr /R ":%DRIVER_PORT% " >nul 2>&1
if %errorlevel% equ 0 (
    echo [Warning] 端口 %DRIVER_PORT% 可能已被占用
    echo.
    netstat -ano | findstr /R ":%DRIVER_PORT% "
    echo.
)

netstat -ano | findstr /R ":%BACKEND_PORT% " >nul 2>&1
if %errorlevel% equ 0 (
    echo [Warning] 端口 %BACKEND_PORT% 可能已被占用
    echo.
    netstat -ano | findstr /R ":%BACKEND_PORT% "
    echo.
)

echo [OK] ✓ 端口检查完成
echo.

REM ==================== 启动驱动 ====================
echo [3/5] 启动驱动程序...
echo.
start "Printer Driver" cmd /k "cd /d %DRIVER_DIR% && echo [驱动程序启动中...] & echo. & printer_driver.exe"

REM 等待驱动启动
timeout /t 3 /nobreak

echo.

REM ==================== 启动后端 ====================
echo [4/5] 启动后端服务...
echo.
start "Printer Backend" cmd /k "cd /d %BACKEND_DIR% && echo [后端服务启动中...] & echo. & printer_backend.exe"

REM 等待后端启动
timeout /t 4 /nobreak

echo.

REM ==================== 检查后端可用性 ====================
echo [5/5] 检查后端可用性...

setlocal enabledelayedexpansion
set "max_retries=10"
set "attempt=0"

:retry_backend
set /a attempt=!attempt! + 1

if !attempt! gtr !max_retries! (
    echo [Warning] 后端服务启动可能超时
    echo 请检查后端窗口是否正常运行
    echo.
    goto open_browser
)

REM 简单的连接测试
powershell -Command "try { $null = [System.Net.Sockets.TcpClient]::new().Connect('localhost', %BACKEND_PORT%); exit 0 } catch { exit 1 }" >nul 2>&1

if %errorlevel% equ 0 (
    echo [OK] ✓ 后端服务已启动并可访问
    goto open_browser
)

echo [Info] 等待后端服务启动... ^(!attempt!/%max_retries!^)
timeout /t 1 /nobreak > nul
goto retry_backend

:open_browser
echo.
echo ================================
echo   ✓ 系统启动完成！
echo ================================
echo.
echo 驱动程序: %DRIVER_DIR%\printer_driver.exe (端口:%DRIVER_PORT%)
echo 后端服务: %BACKEND_DIR%\printer_backend.exe (端口:%BACKEND_PORT%)
echo.
echo 访问网址: %BACKEND_URL%
echo.
echo 正在打开浏览器...
echo.

REM 打开浏览器
timeout /t 2 /nobreak
start "" "%BACKEND_URL%"

echo [OK] 浏览器已打开
echo.
echo 按任意键关闭此窗口...
pause > nul

exit /b 0
