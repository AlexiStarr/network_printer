#!/bin/bash

# 编译和运行脚本

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}  网络打印机系统 编译脚本${NC}"
echo -e "${GREEN}================================${NC}\n"

# 项目目录
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DRIVER_DIR="$PROJECT_DIR/driver"
BACKEND_DIR="$PROJECT_DIR/backend"
TESTS_DIR="$PROJECT_DIR/tests"

# 检查命令参数
if [ "$1" == "driver" ] || [ "$1" == "all" ]; then
    echo -e "${YELLOW}[1/3] 编译 C 语言驱动程序...${NC}"
    cd "$DRIVER_DIR"
    
    # 编译驱动
    gcc -std=c17 -Wall -pthread -o printer_driver \
        printer_simulator.c \
        driver_server.c \
        main_driver.c
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 驱动程序编译成功${NC}"
    else
        echo -e "${RED}✗ 驱动程序编译失败${NC}"
        exit 1
    fi
fi

if [ "$1" == "backend" ] || [ "$1" == "all" ]; then
    echo -e "\n${YELLOW}[2/3] 编译 Go 后端服务...${NC}"
    cd "$BACKEND_DIR"
    
    # 下载依赖
    go mod download
    
    # 编译后端
    go build -o printer_backend main.go
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 后端服务编译成功${NC}"
    else
        echo -e "${RED}✗ 后端服务编译失败${NC}"
        exit 1
    fi
fi

if [ "$1" == "test" ] || [ "$1" == "all" ]; then
    echo -e "\n${YELLOW}[3/3] 准备测试工具...${NC}"
    cd "$TESTS_DIR"
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 测试工具准备完成${NC}"
    fi
fi

echo -e "\n${GREEN}✓ 所有编译完成！${NC}"
echo -e "\n${YELLOW}使用说明：${NC}"
echo -e "1. 启动驱动程序: $DRIVER_DIR/printer_driver"
echo -e "2. 启动后端服务: $BACKEND_DIR/printer_backend"
echo -e "3. 运行自动化测试: bash $TESTS_DIR/auto_test.sh"
echo -e "4. 运行交互式测试: bash $TESTS_DIR/run_test.sh\n"
echo -e "${YELLOW}注意：${NC}"
echo -e "  - 驱动程序运行在端口 9999"
echo -e "  - 后端服务运行在端口 8080"
echo -e "  - 请确保这些端口未被其他程序占用\n"
