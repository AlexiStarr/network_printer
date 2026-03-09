@echo off
REM 网络打印机系统启动脚本

setlocal enabledelayedexpansion

echo.
echo ================================
echo   网络打印机系统
echo ================================
echo.

set PROJECT_DIR=%~dp0
set DRIVER_DIR=%PROJECT_DIR%driver
set BACKEND_DIR=%PROJECT_DIR%backend

REM 检查驱动
if not exist "%DRIVER_DIR%\printer_driver.exe" (
    echo [Error] 找不到驱动: %DRIVER_DIR%\printer_driver.exe
    pause
    exit /b 1
)

REM 检查后端
if not exist "%BACKEND_DIR%\printer_backend.exe" (
    echo [Error] 找不到后端: %BACKEND_DIR%\printer_backend.exe
    pause
    exit /b 1
)

echo [Info] 启动驱动...
start "Printer Driver" cmd /k "cd /d %DRIVER_DIR% && printer_driver.exe"

timeout /t 2 /nobreak

echo [Info] 启动后端...
start "Printer Backend" cmd /k "cd /d %BACKEND_DIR% && printer_backend.exe"

timeout /t 3 /nobreak

echo.
echo [OK] 启动完成
echo.
echo 地址: http://localhost:8080
echo.
pause

REM 打开浏览器
start "" "http://localhost:8080"