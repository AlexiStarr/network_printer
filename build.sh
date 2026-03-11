#!/bin/bash

################################################################################
#                                                                              #
#              网络打印机系统 Linux/Mac 编译脚本                               #
#                      Cross-Platform Build Script v2.0                        #
#                                                                              #
################################################################################

set -e  # 出错时立即退出

# ============================================================================
# 颜色定义与工具函数
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
    echo -e "${GREEN}[Success]${NC} ✓ $1"
}

print_error() {
    echo -e "${RED}[Error]${NC} ✗ $1" >&2
}

print_warning() {
    echo -e "${YELLOW}[Warning]${NC} $1"
}

# ============================================================================
# 配置与检查
# ============================================================================

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DRIVER_DIR="$PROJECT_DIR/driver"
BACKEND_DIR="$PROJECT_DIR/backend"
TESTS_DIR="$PROJECT_DIR/tests"

# 检测操作系统
OS="$(uname -s)"
case "$OS" in
    Linux*)     OS_TYPE="Linux";;
    Darwin*)    OS_TYPE="macOS";;
    *)          OS_TYPE="Unknown";;
esac

print_info "检测到操作系统: $OS_TYPE"

# 检查编译器
if ! command -v gcc &> /dev/null; then
    print_error "未找到 GCC 编译器"
    echo "请安装: "
    echo "  Ubuntu/Debian:  sudo apt-get install build-essential"
    echo "  macOS:          brew install gcc"
    echo "  CentOS/RHEL:    sudo yum groupinstall 'Development Tools'"
    exit 1
fi

if ! command -v go &> /dev/null; then
    print_error "未找到 Go 编译器"
    echo "请从 https://go.dev/dl 下载并安装"
    exit 1
fi

GCC_VERSION=$(gcc --version | head -n1)
GO_VERSION=$(go version)

print_info "GCC 版本: $GCC_VERSION"
print_info "Go 版本: $GO_VERSION"
echo

# ============================================================================
# 编译函数
# ============================================================================

compile_driver() {
    echo -e "\n${YELLOW}[Step 1/3] 编译 C 语言驱动程序...${NC}"
    cd "$DRIVER_DIR"
    
    # 检查源文件
    local required_files=("printer_simulator.c" "driver_server.c" "main_driver.c" 
                         "protocol.c" "protocol_handler.c" "state_machine.c")
    
    for file in "${required_files[@]}"; do
        if [ ! -f "$file" ]; then
            print_error "找不到源文件: $file"
            return 1
        fi
    done
    
    print_info "执行编译命令..."
    
    # 编译驱动（根据操作系统调整编译选项）
    if [ "$OS_TYPE" = "macOS" ]; then
        gcc -std=c17 -Wall -Wextra -O2 -o printer_driver \
            printer_simulator.c \
            driver_server.c \
            main_driver.c \
            protocol.c \
            protocol_handler.c \
            state_machine.c \
            -lpthread
    else
        gcc -std=c17 -Wall -Wextra -O2 -o printer_driver \
            printer_simulator.c \
            driver_server.c \
            main_driver.c \
            protocol.c \
            protocol_handler.c \
            state_machine.c \
            -lpthread
    fi
    
    if [ $? -eq 0 ]; then
        print_success "驱动程序编译成功"
        print_info "输出文件: $DRIVER_DIR/printer_driver"
    else
        print_error "驱动程序编译失败"
        return 1
    fi
}

compile_backend() {
    echo -e "\n${YELLOW}[Step 2/3] 编译 Go 后端服务...${NC}"
    cd "$BACKEND_DIR"
    
    # 检查源文件
    if [ ! -f "main.go" ]; then
        print_error "找不到源文件: main.go"
        return 1
    fi
    
    if [ ! -f "go.mod" ]; then
        print_error "找不到 go.mod 文件"
        print_info "请先运行: go mod init"
        return 1
    fi
    
    print_info "下载依赖..."
    if go mod download; then
        print_success "依赖下载成功"
    else
        print_warning "依赖下载可能失败，继续编译..."
    fi
    
    print_info "整理依赖..."
    if go mod tidy; then
        print_success "依赖整理成功"
    else
        print_warning "依赖整理可能失败，继续编译..."
    fi
    
    print_info "编译二进制文件..."
    if go build -o printer_backend main.go; then
        print_success "后端编译成功"
        print_info "输出文件: $BACKEND_DIR/printer_backend"
    else
        print_error "后端编译失败"
        return 1
    fi
}

prepare_tests() {
    echo -e "\n${YELLOW}[Step 3/3] 准备测试工具...${NC}"
    cd "$TESTS_DIR"
    
    local test_files=("auto_test.sh" "simple_test.sh")
    for file in "${test_files[@]}"; do
        if [ -f "$file" ]; then
            chmod +x "$file"
            print_info "✓ $file"
        fi
    done
    
    print_success "测试工具已准备完成"
}

# ============================================================================
# 清理函数
# ============================================================================

clean_build() {
    echo -e "\n${YELLOW}[Cleaning] 清理编译文件...${NC}"
    echo
    
    # 清理驱动
    [ -f "$DRIVER_DIR/printer_driver" ] && rm -f "$DRIVER_DIR/printer_driver" && echo "✓ 已删除: printer_driver"
    [ -f "$DRIVER_DIR/printer_driver.exe" ] && rm -f "$DRIVER_DIR/printer_driver.exe" && echo "✓ 已删除: printer_driver.exe"
    find "$DRIVER_DIR" -name "*.o" -delete 2>/dev/null && echo "✓ 已删除: 驱动目标文件"
    
    # 清理后端
    [ -f "$BACKEND_DIR/printer_backend" ] && rm -f "$BACKEND_DIR/printer_backend" && echo "✓ 已删除: printer_backend"
    [ -f "$BACKEND_DIR/printer_backend.exe" ] && rm -f "$BACKEND_DIR/printer_backend.exe" && echo "✓ 已删除: printer_backend.exe"
    
    echo
    print_success "清理完成"
}

# ============================================================================
# 主程序
# ============================================================================

echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}  网络打印机系统 编译脚本${NC}"
echo -e "${GREEN}================================${NC}"
echo

# 处理命令行参数
case "${1:-all}" in
    driver)
        compile_driver || exit 1
        ;;
    backend)
        compile_backend || exit 1
        ;;
    test)
        prepare_tests || exit 1
        ;;
    all)
        compile_driver || exit 1
        compile_backend || exit 1
        prepare_tests || exit 1
        ;;
    clean)
        clean_build
        exit 0
        ;;
    *)
        echo "用法: $0 [driver|backend|test|all|clean]"
        echo
        echo "示例:"
        echo "  $0 driver      - 仅编译驱动程序"
        echo "  $0 backend     - 仅编译后端服务"
        echo "  $0 test        - 仅准备测试工具"
        echo "  $0 all         - 编译全部（默认）"
        echo "  $0 clean       - 清理编译文件"
        echo
        exit 1
        ;;
esac

# ============================================================================
# 编译完成总结
# ============================================================================

echo
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}  ✓ 全部编译完成！${NC}"
echo -e "${GREEN}================================${NC}"
echo
echo -e "${YELLOW}下一步操作:${NC}"
echo "  1. 启动脚本：bash start.sh"
echo "  或"
echo "  2. 手动启动:"
echo "     - 驱动程序: $DRIVER_DIR/printer_driver"
echo "     - 后端服务: $BACKEND_DIR/printer_backend"
echo "  3. 打开浏览器访问: http://localhost:8080"
echo
echo -e "${YELLOW}端口信息:${NC}"
echo "  - 驱动程序运行在端口 9999"
echo "  - 后端服务运行在端口 8080"
echo "  - 请确保这些端口未被其他程序占用"
echo

