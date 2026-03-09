#!/bin/bash

# 🎯 打印机控制系统 - 完整快速启动脚本
# 用法: bash quick_start.sh

set -e

PROJECT_DIR="/Users/liuxingyu/Documents/codeRepository/network_printer_system"

echo ""
echo "╔════════════════════════════════════════════════╗"
echo "║    打印机控制系统 - 快速启动                      ║"
echo "║    Network Printer Control System v1.0         ║"
echo "╚════════════════════════════════════════════════╝"
echo ""

# 步骤1: 编译
echo "📦 [步骤1/4] 编译程序..."
cd "$PROJECT_DIR/driver"
echo "   • 编译C驱动..."
gcc -o printer_driver printer_simulator.c driver_server.c main_driver.c -lpthread 2>/dev/null
echo "     ✅ 驱动编译完成"

cd "$PROJECT_DIR/backend"
echo "   • 编译Go后端..."
go build -o printer_backend 2>/dev/null
echo "     ✅ 后端编译完成"
echo ""

# 步骤2: 清理旧进程
echo "🛑 [步骤2/4] 清理旧进程..."
pkill -f "printer_driver" 2>/dev/null || true
pkill -f "printer_backend" 2>/dev/null || true
sleep 1
echo "   ✅ 旧进程已清理"
echo ""

# 步骤3: 启动服务
echo "🚀 [步骤3/4] 启动服务..."
cd "$PROJECT_DIR/driver"
echo "   • 启动驱动程序..."
nohup ./printer_driver > /tmp/printer_driver.log 2>&1 &
DRIVER_PID=$!
sleep 1
echo "     ✅ 驱动已启动 (PID: $DRIVER_PID)"

cd "$PROJECT_DIR/backend"
echo "   • 启动后端服务..."
nohup ./printer_backend > /tmp/printer_backend.log 2>&1 &
BACKEND_PID=$!
sleep 2
echo "     ✅ 后端已启动 (PID: $BACKEND_PID)"
echo ""

# 步骤4: 打开浏览器
echo "🌐 [步骤4/4] 打开前端..."
FRONTEND_PATH="$PROJECT_DIR/printer_control_improved.html"
open "$FRONTEND_PATH" 2>/dev/null || echo "   📁 请手动打开: $FRONTEND_PATH"
echo ""

echo "╔════════════════════════════════════════════════╗"
echo "║            ✅ 系统已启动，就绪！                 ║"
echo "╚════════════════════════════════════════════════╝"
echo ""
echo "📍 前端地址:"
echo "   file://$FRONTEND_PATH"
echo ""
echo "🔐 默认登录凭证:"
echo "   用户名: admin"
echo "   密码:   admin123"
echo ""
echo "   或者:"
echo "   用户名: user"
echo "   密码:   user123"
echo ""
echo "   或者:"
echo "   用户名: technician"
echo "   密码:   tech123"
echo ""
echo "📝 后台日志:"
echo "   驱动:   tail -f /tmp/printer_driver.log"
echo "   后端:   tail -f /tmp/printer_backend.log"
echo ""
echo "🛑 停止系统:"
echo "   pkill -f 'printer_driver|printer_backend'"
echo ""
