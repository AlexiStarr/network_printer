#!/bin/bash

# 简单测试客户端 (C 语言实现)
# 这是一个 curl 替代品，用于测试驱动程序和后端

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

BACKEND_URL="http://localhost:8080"

echo -e "${GREEN}════════════════════════════════════${NC}"
echo -e "${GREEN}  网络打印机系统 快速测试${NC}"
echo -e "${GREEN}════════════════════════════════════${NC}"
echo ""

# 检查 curl
if ! command -v curl &> /dev/null; then
    echo -e "${RED}✗ curl 未安装${NC}"
    exit 1
fi

# 1. 健康检查
echo -e "${BLUE}1. 健康检查...${NC}"
curl -s "$BACKEND_URL/health" | python3 -m json.tool 2>/dev/null || echo "连接失败"
echo ""

# 2. 获取状态
echo -e "${BLUE}2. 获取打印机状态...${NC}"
curl -s "$BACKEND_URL/api/status" | python3 -m json.tool 2>/dev/null || echo "{}"
echo ""

# 3. 提交任务
echo -e "${BLUE}3. 提交打印任务...${NC}"
TASK_RESPONSE=$(curl -s -X POST "$BACKEND_URL/api/job/submit" \
    -H "Content-Type: application/json" \
    -d '{"filename":"test.pdf","pages":5}')
echo "$TASK_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$TASK_RESPONSE"
TASK_ID=$(echo "$TASK_RESPONSE" | grep -o '"task_id":[0-9]*' | head -1 | cut -d: -f2)
echo ""

# 4. 获取队列
echo -e "${BLUE}4. 获取打印队列...${NC}"
curl -s "$BACKEND_URL/api/queue" | python3 -m json.tool 2>/dev/null || echo "{}"
echo ""

# 5. 等待并查看进度
echo -e "${BLUE}5. 等待打印（10秒）...${NC}"
for i in {1..5}; do
    sleep 2
    progress=$(curl -s "$BACKEND_URL/api/queue" | grep -o '"printed_pages":[0-9]*' | head -1 | cut -d: -f2)
    echo "进度: $progress 页"
done
echo ""

# 6. 获取统计
echo -e "${BLUE}6. 获取系统统计...${NC}"
curl -s "$BACKEND_URL/api/stats" | python3 -m json.tool 2>/dev/null || echo "{}"
echo ""

# 7. 补充纸张
echo -e "${BLUE}7. 补充纸张 (500张)...${NC}"
curl -s -X POST "$BACKEND_URL/api/supplies/refill-paper" \
    -H "Content-Type: application/json" \
    -d '{"pages":500}' | python3 -m json.tool 2>/dev/null || echo "{}"
echo ""

# 8. 最终状态
echo -e "${BLUE}8. 最终打印机状态...${NC}"
curl -s "$BACKEND_URL/api/status" | python3 -m json.tool 2>/dev/null || echo "{}"
echo ""

echo -e "${GREEN}✓ 测试完成！${NC}"
