#!/bin/bash

################################################################################
#                                                                              #
#              网络打印机系统启动脚本 (Linux/Mac)                              #
#                      Service Launcher v2.0                                   #
#                                                                              #
################################################################################

set -e

# ============================================================================
# 颜色定义
# ============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}[Info]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[OK]${NC} ✓ $1"
}

print_error() {
    echo -e "${RED}[Error]${NC} ✗ $1" >&2
}

print_warning() {
    echo -e "${YELLOW}[Warning]${NC} $1"
}

# ============================================================================
# 配置
# ============================================================================

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DRIVER_DIR="$PROJECT_DIR/driver"
BACKEND_DIR="$PROJECT_DIR/backend"
DRIVER_PORT=9999
BACKEND_PORT=8080
BACKEND_URL="http://localhost:$BACKEND_PORT"
LOG_DIR="/tmp/printer_system"

# 创建日志目录
mkdir -p "$LOG_DIR"

# ============================================================================
# 检查并清理旧进程
# ============================================================================

cleanup_old_processes() {
    print_info "清理旧进程..."
    
    # 查找并终止旧的驱动程序进程
    if pgrep -f "printer_driver" > /dev/null 2>&1; then
        print_warning "检测到旧的驱动程序进程，正在清理..."
        pkill -f "printer_driver" 2>/dev/null || true
        sleep 1
    fi
    
    # 查找并终止旧的后端进程
    if pgrep -f "printer_backend" > /dev/null 2>&1; then
        print_warning "检测到旧的后端进程，正在清理..."
        pkill -f "printer_backend" 2>/dev/null || true
        sleep 1
    fi
}

# ============================================================================
# 主程序
# ============================================================================

echo
echo "================================"
echo "  网络打印机系统启动脚本 v2.0"
echo "================================"
echo

# [1] 检查编译文件
echo "[1/6] 检查编译文件..."
if [ ! -f "$DRIVER_DIR/printer_driver" ]; then
    print_error "找不到驱动程序: $DRIVER_DIR/printer_driver"
    echo "请先运行编译脚本:"
    echo "  bash build.sh all"
    echo
    exit 1
fi

if [ ! -f "$BACKEND_DIR/printer_backend" ]; then
    print_error "找不到后端程序: $BACKEND_DIR/printer_backend"
    echo "请先运行编译脚本:"
    echo "  bash build.sh all"
    echo
    exit 1
fi

print_success "所有编译文件都已找到"
echo

# [2] 检查端口占用
echo "[2/6] 检查端口占用情况..."

check_port() {
    local port=$1
    local name=$2
    
    if command -v lsof &> /dev/null; then
        if lsof -i :$port > /dev/null 2>&1; then
            print_warning "端口 $port ($name) 可能已被占用"
            return 1
        fi
    elif command -v netstat &> /dev/null; then
        if netstat -ln 2>/dev/null | grep -q ":$port "; then
            print_warning "端口 $port ($name) 可能已被占用"
            return 1
        fi
    else
        print_warning "无法检查端口占用状态（需要 lsof 或 netstat）"
    fi
    return 0
}

check_port $DRIVER_PORT "驱动程序" || true
check_port $BACKEND_PORT "后端服务" || true

print_success "端口检查完成"
echo

# [3] 清理旧进程
echo "[3/6] 清理旧进程..."
cleanup_old_processes
print_success "旧进程清理完成"
echo

# [4] 启动驱动程序
echo "[4/6] 启动驱动程序..."
cd "$DRIVER_DIR"

# 创建驱动日志文件
DRIVER_LOG="$LOG_DIR/driver.log"

# 在后台启动驱动程序
"$DRIVER_DIR/printer_driver" > "$DRIVER_LOG" 2>&1 &
DRIVER_PID=$!

print_info "驱动程序已启动 (PID: $DRIVER_PID)"
print_info "日志文件: $DRIVER_LOG"

# 等待驱动启动
sleep 2

# 检查驱动是否仍在运行
if ! kill -0 $DRIVER_PID 2>/dev/null; then
    print_error "驱动程序启动失败！"
    echo "日志内容:"
    cat "$DRIVER_LOG"
    exit 1
fi

print_success "驱动程序已启动"
echo

# [5] 启动后端服务
echo "[5/6] 启动后端服务..."
cd "$BACKEND_DIR"

# 创建后端日志文件
BACKEND_LOG="$LOG_DIR/backend.log"

# 在后台启动后端程序
"$BACKEND_DIR/printer_backend" > "$BACKEND_LOG" 2>&1 &
BACKEND_PID=$!

print_info "后端服务已启动 (PID: $BACKEND_PID)"
print_info "日志文件: $BACKEND_LOG"

# 等待后端启动
sleep 2

# 检查后端是否仍在运行
if ! kill -0 $BACKEND_PID 2>/dev/null; then
    print_error "后端服务启动失败！"
    echo "日志内容:"
    cat "$BACKEND_LOG"
    # 清理驱动
    kill $DRIVER_PID 2>/dev/null || true
    exit 1
fi

print_success "后端服务已启动"
echo

# [6] 检查后端可用性
echo "[6/6] 检查后端可用性..."

max_retries=10
attempt=0

while [ $attempt -lt $max_retries ]; do
    attempt=$((attempt + 1))
    
    # 使用 curl 检查后端是否可访问
    if command -v curl &> /dev/null; then
        if curl -s "$BACKEND_URL" > /dev/null 2>&1; then
            print_success "后端服务已启动并可访问"
            break
        fi
    else
        # 如果没有 curl，只检查 TCP 连接
        if nc -z localhost $BACKEND_PORT 2>/dev/null; then
            print_success "后端服务已启动并可访问"
            break
        fi
    fi
    
    if [ $attempt -lt $max_retries ]; then
        echo "等待后端服务启动... ($attempt/$max_retries)"
        sleep 1
    else
        print_warning "后端服务启动可能超时"
        echo "请检查日志: $BACKEND_LOG"
    fi
done

echo

# ============================================================================
# 启动完成
# ============================================================================

echo "================================"
echo "  ✓ 系统启动完成！"
echo "================================"
echo
echo "驱动程序: $DRIVER_DIR/printer_driver (PID: $DRIVER_PID, 端口: $DRIVER_PORT)"
echo "后端服务: $BACKEND_DIR/printer_backend (PID: $BACKEND_PID, 端口: $BACKEND_PORT)"
echo
echo "访问网址: $BACKEND_URL"
echo
echo "日志位置:"
echo "  - 驱动日志: $DRIVER_LOG"
echo "  - 后端日志: $BACKEND_LOG"
echo
echo "停止服务: bash stop.sh (如果有此脚本)"
echo "或者手动停止:"
echo "  kill $DRIVER_PID"
echo "  kill $BACKEND_PID"
echo

# 尝试打开浏览器
if command -v xdg-open &> /dev/null; then
    # Linux
    xdg-open "$BACKEND_URL" 2>/dev/null &
    print_info "浏览器已打开"
elif command -v open &> /dev/null; then
    # macOS
    open "$BACKEND_URL" 2>/dev/null &
    print_info "浏览器已打开"
else
    print_info "请手动打开浏览器访问: $BACKEND_URL"
fi

echo
print_info "按 Ctrl+C 停止此脚本（但服务将继续运行）"
echo

# 保持脚本运行，方便查看日志和停止
trap 'echo; print_info "脚本已停止"; exit 0' INT TERM

while true; do
    sleep 1
done
