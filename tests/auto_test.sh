#!/bin/bash

# 网络打印机系统手动化测试脚本

BASE_URL="http://localhost:8080"
DRIVER_ADDR="localhost:9999"

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# 测试计数
PASSED=0
FAILED=0

# 辅助函数：发送请求和检查结果
function test_api() {
    local method=$1
    local endpoint=$2
    local data=$3
    local description=$4
    local expected=$5
    
    echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}测试: ${description}${NC}"
    echo -e "Method: ${method} ${GREEN}${endpoint}${NC}"
    
    if [ -n "$data" ]; then
        echo -e "数据: ${data}"
        response=$(curl -s -X "$method" "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" \
            -d "$data" 2>&1)
    else
        response=$(curl -s -X "$method" "$BASE_URL$endpoint" \
            -H "Content-Type: application/json" 2>&1)
    fi
    
    echo -e "响应:"
    echo "$response" | python3 -m json.tool 2>/dev/null || echo "$response"
    
    # 验证响应
    if [ -n "$expected" ]; then
        if echo "$response" | grep -q "$expected"; then
            echo -e "${GREEN}✓ 通过${NC}"
            ((PASSED++))
            return 0
        else
            echo -e "${RED}✗ 失败 (期望包含: $expected)${NC}"
            ((FAILED++))
            return 1
        fi
    else
        if [[ $response == *"success"* ]] || [[ $response == *"status"* ]] || [[ $response == *"ok"* ]]; then
            echo -e "${GREEN}✓ 通过${NC}"
            ((PASSED++))
            return 0
        else
            echo -e "${RED}✗ 失败${NC}"
            ((FAILED++))
            return 1
        fi
    fi
}

# 系统状态测试
function test_system_status() {
    echo -e "\n${GREEN}════════════════════════════════════${NC}"
    echo -e "${GREEN}    1. 系统状态测试${NC}"
    echo -e "${GREEN}════════════════════════════════════${NC}"
    
    test_api "GET" "/health" "" "健康检查" "ok"
    test_api "GET" "/api/status" "" "获取打印机状态" "status"
    test_api "GET" "/api/stats" "" "获取系统统计" "timestamp"
}

# 打印任务管理测试
function test_job_management() {
    echo -e "\n${GREEN}════════════════════════════════════${NC}"
    echo -e "${GREEN}    2. 打印任务管理测试${NC}"
    echo -e "${GREEN}════════════════════════════════════${NC}"
    
    test_api "POST" "/api/job/submit" \
        '{"filename":"report1.pdf","pages":10}' \
        "提交任务 1: report1.pdf (10页)" "success"
    
    sleep 1
    
    test_api "POST" "/api/job/submit" \
        '{"filename":"document.docx","pages":5}' \
        "提交任务 2: document.docx (5页)" "success"
    
    sleep 1
    
    test_api "GET" "/api/queue" "" "查看打印队列" "queue"
    
    sleep 1
    
    test_api "POST" "/api/job/cancel" \
        '{"task_id":1}' \
        "取消任务 1" "success"
    
    sleep 1
    
    test_api "GET" "/api/queue" "" "验证任务取消后的队列" "queue"
}

# 耗材管理测试
function test_supplies_management() {
    echo -e "\n${GREEN}════════════════════════════════════${NC}"
    echo -e "${GREEN}    3. 耗材管理测试${NC}"
    echo -e "${GREEN}════════════════════════════════════${NC}"
    
    test_api "GET" "/api/status" "" "查看当前耗材状态" "paper"
    
    sleep 1
    
    test_api "POST" "/api/supplies/refill-paper" \
        '{"pages":500}' \
        "补充纸张 (500张)" "success"
    
    sleep 1
    
    test_api "POST" "/api/supplies/refill-toner" \
        '' \
        "补充碳粉" "success"
    
    sleep 1
    
    test_api "GET" "/api/status" "" "验证补充结果" "paper"
}

# 错误模拟测试
function test_error_simulation() {
    echo -e "\n${GREEN}════════════════════════════════════${NC}"
    echo -e "${GREEN}    4. 错误模拟测试${NC}"
    echo -e "${GREEN}════════════════════════════════════${NC}"
    
    test_api "POST" "/api/error/simulate" \
        '{"error":"PAPER_EMPTY"}' \
        "模拟缺纸错误" "success"
    
    sleep 1
    
    test_api "GET" "/api/status" "" "验证错误状态" "PAPER_EMPTY"
    
    sleep 1
    
    test_api "POST" "/api/error/clear" \
        '' \
        "清除错误" "success"
    
    sleep 1
    
    test_api "POST" "/api/error/simulate" \
        '{"error":"TONER_LOW"}' \
        "模拟碳粉不足" "success"
    
    sleep 1
    
    test_api "POST" "/api/error/clear" \
        '' \
        "清除错误" "success"
}

# 显示菜单
function show_menu() {
    echo -e "\n${GREEN}════════════════════════════════════${NC}"
    echo -e "${GREEN}  网络打印机系统测试菜单${NC}"
    echo -e "${GREEN}════════════════════════════════════${NC}\n"
    
    echo -e "${YELLOW}请选择要执行的测试:${NC}\n"
    echo "1. 系统状态测试"
    echo "2. 打印任务管理测试"
    echo "3. 耗材管理测试"
    echo "4. 错误模拟测试"
    echo "5. 运行所有测试"
    echo "0. 退出"
    echo ""
}

# 显示测试结果
function show_results() {
    echo -e "\n${GREEN}════════════════════════════════════${NC}"
    echo -e "${GREEN}    测试完成${NC}"
    echo -e "${GREEN}════════════════════════════════════${NC}\n"
    
    echo -e "通过: ${GREEN}${PASSED}${NC}"
    echo -e "失败: ${RED}${FAILED}${NC}"
    
    if [ $FAILED -eq 0 ] && [ $PASSED -gt 0 ]; then
        echo -e "\n${GREEN}所有测试通过！${NC}\n"
    elif [ $FAILED -eq 0 ]; then
        echo -e "\n${YELLOW}未执行任何测试${NC}\n"
    else
        echo -e "\n${RED}有 ${FAILED} 个测试失败${NC}\n"
    fi
}

# 主程序
function main() {
    # 首次连接检查
    echo -e "${GREEN}════════════════════════════════════${NC}"
    echo -e "${GREEN}  网络打印机系统测试${NC}"
    echo -e "${GREEN}════════════════════════════════════${NC}\n"
    
    echo -e "${YELLOW}检查服务连接...${NC}"
    
    # 检查后端服务
    if ! curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e "${RED}✗ 无法连接到后端服务: $BASE_URL${NC}"
        echo -e "${YELLOW}请确保:${NC}"
        echo "  1. 驱动程序已启动: ./driver/printer_driver"
        echo "  2. 后端服务已启动: ./backend/printer_backend"
        exit 1
    fi
    
    echo -e "${GREEN}✓ 后端服务连接正常${NC}"
    
    # 主循环
    while true; do
        show_menu
        
        read -p "请输入选项 (0-5): " choice
        
        case $choice in
            1)
                PASSED=0
                FAILED=0
                test_system_status
                show_results
                ;;
            2)
                PASSED=0
                FAILED=0
                test_job_management
                show_results
                ;;
            3)
                PASSED=0
                FAILED=0
                test_supplies_management
                show_results
                ;;
            4)
                PASSED=0
                FAILED=0
                test_error_simulation
                show_results
                ;;
            5)
                PASSED=0
                FAILED=0
                test_system_status
                test_job_management
                test_supplies_management
                test_error_simulation
                show_results
                ;;
            0)
                echo -e "${YELLOW}退出测试${NC}\n"
                exit 0
                ;;
            *)
                echo -e "${RED}无效的选项，请重新选择${NC}"
                ;;
        esac
        
        read -p "按 Enter 键返回菜单..."
    done
}

# 运行主程序
main
