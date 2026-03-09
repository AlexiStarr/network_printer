# API请求示例与测试脚本

## 🔐 认证示例

### 用户登录
```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "user1",
    "password": "password123"
  }'

# 响应示例：
# {
#   "token": "a1b2c3d4e5f6...",
#   "role": "user"
# }
```

### 用户登出
```bash
curl -X POST http://localhost:8080/api/logout \
  -H "Authorization: Bearer YOUR_TOKEN"

# 响应示例：
# {
#   "success": true
# }
```

---

## 📊 状态查询

### 获取打印机状态
```bash
curl -X GET http://localhost:8080/api/status \
  -H "Authorization: Bearer YOUR_TOKEN"

# 响应示例：
# {
#   "status": "idle",
#   "paper_pages": 500,
#   "toner_percentage": 85,
#   "temperature": 45,
#   "page_count": 12345,
#   "firmware_version": "v1.0",
#   "error": "HARDWARE_OK",
#   "model": "NetworkPrinter-X1000",
#   "serial_number": "SN123456789"
# }
```

### 获取打印队列
```bash
curl -X GET http://localhost:8080/api/queue \
  -H "Authorization: Bearer YOUR_TOKEN"

# 响应示例：
# {
#   "queue": [
#     {
#       "task_id": 1,
#       "filename": "document.pdf",
#       "pages": 25,
#       "priority": 10,
#       "status": "printing"
#     },
#     {
#       "task_id": 2,
#       "filename": "report.docx",
#       "pages": 15,
#       "priority": 5,
#       "status": "submitted"
#     }
#   ],
#   "queue_size": 2
# }
```

### 获取统计信息
```bash
curl -X GET http://localhost:8080/api/stats \
  -H "Authorization: Bearer YOUR_TOKEN"

# 响应示例：
# {
#   "total_pages_printed": 12345,
#   "timestamp": "2026-03-07T10:30:45Z",
#   "uptime": "running",
#   "queue_size": 2
# }
```

---

## 📝 任务管理

### 提交打印任务
```bash
curl -X POST http://localhost:8080/api/submit \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "filename": "presentation.pptx",
    "pages": 50,
    "priority": 7
  }'

# 响应示例：
# {
#   "success": true,
#   "task_id": 123
# }
```

### 暂停任务
```bash
curl -X POST http://localhost:8080/api/pause \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "task_id": 123
  }'

# 响应示例：
# {
#   "success": true
# }
```

### 恢复任务
```bash
curl -X POST http://localhost:8080/api/resume \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "task_id": 123
  }'

# 响应示例：
# {
#   "success": true
# }
```

### 取消任务
```bash
curl -X POST http://localhost:8080/api/cancel \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "task_id": 123
  }'

# 响应示例：
# {
#   "success": true
# }
```

---

## 🔧 维护操作

### 补充纸张
```bash
curl -X POST http://localhost:8080/api/refill-paper \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "pages": 500
  }'

# 响应示例：
# {
#   "success": true,
#   "paper_pages": 500
# }
```

### 补充碳粉
```bash
curl -X POST http://localhost:8080/api/refill-toner \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'

# 响应示例：
# {
#   "success": true,
#   "toner_percentage": 100
# }
```

### 清除错误
```bash
curl -X POST http://localhost:8080/api/clear-error \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'

# 响应示例：
# {
#   "success": true
# }
```

### 模拟硬件错误（仅管理员）
```bash
curl -X POST http://localhost:8080/api/simulate-error \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "error": "PAPER_EMPTY"
  }'

# 支持的错误类型：
# - PAPER_EMPTY          (缺纸)
# - TONER_LOW            (碳粉不足)
# - TONER_EMPTY          (缺碳粉)
# - HEAT_UNAVAILABLE     (加热器故障)
# - MOTOR_FAILURE        (电机故障)
# - SENSOR_FAILURE       (传感器故障)

# 响应示例：
# {
#   "success": true
# }
```

---

## 📋 历史与查询

### 获取打印历史
```bash
curl -X GET http://localhost:8080/api/history \
  -H "Authorization: Bearer YOUR_TOKEN"

# 响应示例：
# {
#   "history": [
#     {
#       "task_id": 120,
#       "filename": "report.pdf",
#       "pages": 20,
#       "printed_pages": 20,
#       "status": "completed",
#       "created_at": "2026-03-07T09:00:00Z",
#       "completed_at": "2026-03-07T09:02:15Z",
#       "user_id": "user1",
#       "priority": 5
#     },
#     {
#       "task_id": 119,
#       "filename": "document.docx",
#       "pages": 15,
#       "printed_pages": 15,
#       "status": "completed",
#       "created_at": "2026-03-07T08:45:30Z",
#       "completed_at": "2026-03-07T08:47:45Z",
#       "user_id": "user1",
#       "priority": 3
#     }
#   ]
# }
```

---

## 👥 用户管理（仅管理员）

### 列出所有用户
```bash
curl -X GET http://localhost:8080/api/users \
  -H "Authorization: Bearer ADMIN_TOKEN"

# 响应示例：
# {
#   "users": [
#     {
#       "username": "admin",
#       "role": "admin",
#       "created_at": "2026-03-01T00:00:00Z"
#     },
#     {
#       "username": "user1",
#       "role": "user",
#       "created_at": "2026-03-02T10:30:00Z"
#     },
#     {
#       "username": "technician1",
#       "role": "technician",
#       "created_at": "2026-03-03T14:00:00Z"
#     }
#   ]
# }
```

### 添加新用户
```bash
curl -X POST http://localhost:8080/api/users \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser",
    "password": "SecurePassword123",
    "role": "user"
  }'

# 角色选项：
# - "user"       (普通用户)
# - "technician" (技术员)
# - "admin"      (管理员)

# 响应示例：
# {
#   "success": "true",
#   "message": "用户已创建"
# }
```

### 删除用户
```bash
curl -X DELETE http://localhost:8080/api/users \
  -H "Authorization: Bearer ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "olduser"
  }'

# 响应示例：
# {
#   "success": "true",
#   "message": "用户已删除"
# }
```

---

## 🔌 WebSocket实时通信

### 连接WebSocket
```javascript
// JavaScript客户端示例
const token = "YOUR_TOKEN";
const ws = new WebSocket(`ws://localhost:8080/ws`);

ws.onopen = function() {
    console.log("WebSocket已连接");
};

ws.onmessage = function(event) {
    const data = JSON.parse(event.data);
    console.log("收到实时更新:", data);
    
    // 处理不同类型的事件
    switch(data.event) {
        case 'job_submitted':
            console.log(`任务 ${data.task_id} 已提交`);
            break;
        case 'job_completed':
            console.log(`任务 ${data.task_id} 已完成`);
            break;
        case 'job_cancelled':
            console.log(`任务 ${data.task_id} 已取消`);
            break;
        case 'paper_refilled':
            console.log(`纸张已补充: ${data.pages} 页`);
            break;
        case 'toner_refilled':
            console.log('碳粉已补充');
            break;
        case 'error_cleared':
            console.log('错误已清除');
            break;
        case 'error_simulated':
            console.log(`错误已模拟: ${data.error}`);
            break;
    }
};

ws.onerror = function(error) {
    console.error("WebSocket错误:", error);
};

ws.onclose = function() {
    console.log("WebSocket已断开连接");
    // 重新连接逻辑...
};
```

### WebSocket事件示例

#### 任务提交事件
```json
{
  "event": "job_submitted",
  "task_id": 123,
  "filename": "document.pdf",
  "pages": 25,
  "priority": 5
}
```

#### 任务完成事件
```json
{
  "event": "job_completed",
  "task_id": 123
}
```

#### 任务取消事件
```json
{
  "event": "job_cancelled",
  "task_id": 123
}
```

#### 纸张补充事件
```json
{
  "event": "paper_refilled",
  "pages": 500
}
```

#### 碳粉补充事件
```json
{
  "event": "toner_refilled"
}
```

#### 错误清除事件
```json
{
  "event": "error_cleared"
}
```

#### 错误模拟事件
```json
{
  "event": "error_simulated",
  "error": "PAPER_EMPTY"
}
```

---

## 📜 完整测试脚本

### Bash脚本 (test_api.sh)
```bash
#!/bin/bash

# 配置
BASE_URL="http://localhost:8080"
USERNAME="user1"
PASSWORD="password123"
ADMIN_USER="admin"
ADMIN_PASS="admin123"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 获取Token
echo -e "${YELLOW}[1] 用户登录...${NC}"
LOGIN_RESPONSE=$(curl -s -X POST $BASE_URL/api/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")

USER_TOKEN=$(echo $LOGIN_RESPONSE | grep -o '"token":"[^"]*' | cut -d'"' -f4)
echo -e "${GREEN}✓ 用户Token: $USER_TOKEN${NC}"

echo -e "${YELLOW}[2] 管理员登录...${NC}"
ADMIN_LOGIN=$(curl -s -X POST $BASE_URL/api/login \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}")

ADMIN_TOKEN=$(echo $ADMIN_LOGIN | grep -o '"token":"[^"]*' | cut -d'"' -f4)
echo -e "${GREEN}✓ 管理员Token: $ADMIN_TOKEN${NC}"

# 获取打印机状态
echo -e "${YELLOW}[3] 获取打印机状态...${NC}"
STATUS=$(curl -s -X GET $BASE_URL/api/status \
  -H "Authorization: Bearer $USER_TOKEN")
echo -e "${GREEN}✓ 状态: $STATUS${NC}"

# 提交打印任务
echo -e "${YELLOW}[4] 提交打印任务...${NC}"
SUBMIT=$(curl -s -X POST $BASE_URL/api/submit \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"filename":"test.pdf","pages":10,"priority":5}')

TASK_ID=$(echo $SUBMIT | grep -o '"task_id":[0-9]*' | cut -d':' -f2)
echo -e "${GREEN}✓ 任务ID: $TASK_ID${NC}"

# 获取队列
echo -e "${YELLOW}[5] 获取打印队列...${NC}"
QUEUE=$(curl -s -X GET $BASE_URL/api/queue \
  -H "Authorization: Bearer $USER_TOKEN")
echo -e "${GREEN}✓ 队列: $QUEUE${NC}"

# 暂停任务
echo -e "${YELLOW}[6] 暂停任务...${NC}"
PAUSE=$(curl -s -X POST $BASE_URL/api/pause \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"task_id\":$TASK_ID}")
echo -e "${GREEN}✓ 暂停结果: $PAUSE${NC}"

# 恢复任务
echo -e "${YELLOW}[7] 恢复任务...${NC}"
RESUME=$(curl -s -X POST $BASE_URL/api/resume \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"task_id\":$TASK_ID}")
echo -e "${GREEN}✓ 恢复结果: $RESUME${NC}"

# 补充纸张
echo -e "${YELLOW}[8] 补充纸张...${NC}"
REFILL=$(curl -s -X POST $BASE_URL/api/refill-paper \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pages":500}')
echo -e "${GREEN}✓ 补充结果: $REFILL${NC}"

# 获取打印历史
echo -e "${YELLOW}[9] 获取打印历史...${NC}"
HISTORY=$(curl -s -X GET $BASE_URL/api/history \
  -H "Authorization: Bearer $USER_TOKEN")
echo -e "${GREEN}✓ 历史: $HISTORY${NC}"

# 用户登出
echo -e "${YELLOW}[10] 用户登出...${NC}"
LOGOUT=$(curl -s -X POST $BASE_URL/api/logout \
  -H "Authorization: Bearer $USER_TOKEN")
echo -e "${GREEN}✓ 登出结果: $LOGOUT${NC}"

echo -e "${GREEN}[✓] 所有测试完成${NC}"
```

### Python脚本 (test_api.py)
```python
#!/usr/bin/env python3

import requests
import json
import time
from websocket import create_connection

BASE_URL = "http://localhost:8080"
WS_URL = "ws://localhost:8080/ws"

class PrinterClient:
    def __init__(self):
        self.token = None
        self.admin_token = None
    
    def login(self, username, password):
        """用户登录"""
        response = requests.post(
            f"{BASE_URL}/api/login",
            json={"username": username, "password": password}
        )
        data = response.json()
        if "token" in data:
            print(f"✓ 登录成功: {username}")
            if username == "admin":
                self.admin_token = data["token"]
            else:
                self.token = data["token"]
            return data["token"]
        else:
            print(f"✗ 登录失败: {data}")
            return None
    
    def get_status(self):
        """获取打印机状态"""
        response = requests.get(
            f"{BASE_URL}/api/status",
            headers={"Authorization": f"Bearer {self.token}"}
        )
        print(f"✓ 打印机状态: {response.json()}")
        return response.json()
    
    def submit_job(self, filename, pages, priority=5):
        """提交打印任务"""
        response = requests.post(
            f"{BASE_URL}/api/submit",
            headers={"Authorization": f"Bearer {self.token}"},
            json={"filename": filename, "pages": pages, "priority": priority}
        )
        data = response.json()
        if data.get("success"):
            print(f"✓ 任务已提交, ID: {data['task_id']}")
            return data["task_id"]
        else:
            print(f"✗ 提交失败: {data}")
            return None
    
    def get_queue(self):
        """获取打印队列"""
        response = requests.get(
            f"{BASE_URL}/api/queue",
            headers={"Authorization": f"Bearer {self.token}"}
        )
        print(f"✓ 打印队列: {response.json()}")
        return response.json()
    
    def cancel_job(self, task_id):
        """取消任务"""
        response = requests.post(
            f"{BASE_URL}/api/cancel",
            headers={"Authorization": f"Bearer {self.token}"},
            json={"task_id": task_id}
        )
        print(f"✓ 任务已取消: {task_id}")
        return response.json()
    
    def run_tests(self):
        """运行完整测试"""
        print("=" * 50)
        print("打印机控制系统 - API测试")
        print("=" * 50)
        
        # 登录
        print("\n[1] 用户登录")
        self.login("user1", "password123")
        
        # 获取状态
        print("\n[2] 获取打印机状态")
        self.get_status()
        
        # 提交任务
        print("\n[3] 提交打印任务")
        task_id = self.submit_job("test.pdf", 25, 5)
        
        # 获取队列
        print("\n[4] 获取打印队列")
        self.get_queue()
        
        # 取消任务
        if task_id:
            print("\n[5] 取消任务")
            self.cancel_job(task_id)
        
        print("\n" + "=" * 50)
        print("✓ 所有测试完成")
        print("=" * 50)

if __name__ == "__main__":
    client = PrinterClient()
    client.run_tests()
```

---

## 🔒 错误响应示例

### 未授权错误
```json
{
  "error": "未授权"
}
```

### 权限不足
```json
{
  "error": "没有权限"
}
```

### 任务不存在
```json
{
  "error": "任务不存在"
}
```

### 服务器错误
```json
{
  "error": "Internal Server Error"
}
```

---

这些示例可以直接在命令行、Postman或任何HTTP客户端中使用。
