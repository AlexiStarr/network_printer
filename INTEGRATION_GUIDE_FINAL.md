# 虚拟打印机系统 - 最终集成指南

## 概述

本文档说明如何完成虚拟打印机系统的最后集成步骤，包括：
- MySQL数据库集成
- 二进制协议集成（driver_server.c）
- 实时进度显示集成（HTML）
- 权限管理（仅admin可查看PDF历史）

## 系统架构

```
┌─────────────────────────────────────────────────────────┐
│         printer_control.html (Web 前端)                  │
│    - 实时进度显示 (progress-display.js)                  │
│    - WebSocket 连接 (ws://localhost:8080/ws/progress)    │
└──────────────────┬──────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────┐
│         main.go (Go 后端 - 端口 8080)                   │
│    - MySQL 数据库 (打印历史 + PDF管理 + 用户管理)         │
│    - ProgressTracker (实时进度追踪)                     │
│    - PDFManager (PDF 存储管理 - 最多10个)               │
│    - WebSocket Hub (实时通知推送)                        │
│    - API 端点 (REST)                                    │
│      - /api/pdf/recent (GET) - 仅admin                 │
│      - /api/pdf/download (GET) - 仅admin               │
│      - /ws/progress (WebSocket) - 进度推送              │
└──────────────────┬──────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────┐
│      driver_server.c (C 驱动 - 端口 9999)              │
│    - 二进制协议处理 (protocol.c)                         │
│    - 状态机 (state_machine.c)                           │
│    - 协议处理器 (protocol_handler.c)                     │
│    - 打印机模拟器 (printer_simulator.c)                  │
└─────────────────────────────────────────────────────────┘
```

## 前提条件

### 1. MySQL 数据库设置

```bash
# 创建数据库
mysql -u root -p -e "CREATE DATABASE printer_db CHARACTER SET utf8mb4;"

# 查看创建的数据库
mysql -u root -p -e "SHOW DATABASES;"
```

### 2. Go 依赖

```bash
cd backend
go mod tidy
go mod download

# 验证依赖
go mod graph
```

### 3. C 编译工具

- Windows: MinGW 或 MSVC
- Linux: gcc/clang
- macOS: Xcode Command Line Tools

## 部署步骤

### 第一步：配置 MySQL 连接

编辑 `backend/main.go` 第 1677 行：

```go
mysqlDB, dbErr := NewMySQLDatabase(
    "root",              // MySQL 用户名
    "password",          // MySQL 密码
    "localhost",         // MySQL 主机
    "3306",              // MySQL 端口
    "printer_db"         // 数据库名
)
```

**重要**: 将 `password` 替换为实际的 MySQL 密码。

### 第二步：启动 MySQL 数据库

```bash
# Windows (if using services)
net start MySQL80

# Linux
sudo systemctl start mysql

# macOS (if using Homebrew)
brew services start mysql
```

### 第三步：编译并运行后端

```bash
cd backend

# 编译
go build -o backend main.go

# 运行
./backend
# 或
go run main.go
```

**预期输出**:
```
================================
  网络打印机后端服务 v1.0
================================
[Info] 使用 MySQL 作为主数据库
[Backend] 服务启动成功，监听端口 8080
[Backend] WebSocket 地址: ws://localhost:8080/ws/progress
```

### 第四步：编译 C 驱动

```bash
cd driver

# Windows (MinGW)
gcc -o printer_driver *.c -lpthread -lws2_32 -o build/printer_driver

# Linux
gcc -o printer_driver *.c -lpthread -ldl

# 运行驱动
./printer_driver
```

**预期输出**:
```
[Driver] 初始化中...
[Driver] 打印机初始化成功
[Driver] 驱动服务器启动成功，监听端口 9999
```

### 第五步：访问 Web 前端

打开浏览器访问: http://localhost:8080

**登录凭证**:
- 用户名: `admin` / 密码: `admin123`（管理员）
- 用户名: `user` / 密码: `user123`（普通用户）
- 用户名: `technician` / 密码: `tech123`（技术员）

## 功能验证

### 1. 二进制协议（Driver Integration）

验证 C 驱动是否使用二进制协议与后端通信：

```bash
# 在驱动日志中应该看到
[Driver] 客户端已连接，准备接收二进制协议数据
[Driver] 收到完整的二进制协议数据包, 命令: 1, 长度: 12
[Driver] 已发送二进制协议响应, 长度: 20
```

### 2. 实时进度显示

1. 新建打印任务（提交打印任务页面）
2. 观察仪表盘的"实时打印进度"卡片
3. 应该看到任务的实时进度更新

**进度推送消息类型**:
- `progress` - 进度更新（每100毫秒）
- `completed` - 任务完成
- `error` - 错误发生
- `paused` - 任务暂停
- `resumed` - 任务恢复
- `cancelled` - 任务取消
- `submitted` - 新任务提交

### 3. PDF 历史（仅 Admin）

**Admin 用户操作**:
```bash
# 获取最近 10 个 PDF
curl -H "Authorization: <admin_token>" http://localhost:8080/api/pdf/recent

# 下载 PDF（task_id=1）
curl -H "Authorization: <admin_token>" \
     "http://localhost:8080/api/pdf/download?task_id=1" \
     -o task_1.pdf
```

**普通用户尝试访问**:
```bash
# 应该返回 403 Forbidden
curl -H "Authorization: <user_token>" http://localhost:8080/api/pdf/recent
# 结果: {"error": "仅管理员有权限访问PDF历史"}
```

## API 端点文档

### 认证

```
POST /api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin123"
}

Response:
{
  "token": "abc123...",
  "role": "admin",
  "username": "admin"
}
```

### 打印历史（mysql_database.go）

```
GET /api/history
Authorization: Bearer <token>

Response:
{
  "history": [
    {
      "task_id": 1,
      "filename": "report.pdf",
      "pages": 10,
      "printed_pages": 10,
      "status": "completed",
      "created_at": "2024-01-15T10:30:00Z",
      "user_id": "admin",
      "priority": 0
    }
  ]
}
```

### PDF 历史（仅 Admin）

```
GET /api/pdf/recent
Authorization: Bearer <admin_token>

Response:
{
  "pdfs": [
    {
      "task_id": 10,
      "filename": "print_task_10_1704893400.pdf",
      "file_size": 245632,
      "file_hash": "d41d8cd98f00b204e9800998ecf8427e",
      "created_at": "2024-01-10T15:30:00Z"
    }
  ],
  "count": 1
}

# 404 如果没有记录
```

### PDF 下载（仅 Admin）

```
GET /api/pdf/download?task_id=10
Authorization: Bearer <admin_token>

Response: PDF 二进制数据
Header: Content-Type: application/pdf
Header: Content-Disposition: attachment; filename="print_task_10.pdf"
```

### WebSocket 进度（实时）

```
ws://localhost:8080/ws/progress

# 服务器推送消息示例:
{
  "type": "progress",
  "timestamp": "2024-01-15T10:30:00Z",
  "progress": {
    "task_id": 5,
    "filename": "document.pdf",
    "total_pages": 50,
    "printed_pages": 25,
    "progress_percent": 50,
    "status": "printing",
    "estimated_time_sec": 300,
    "temperature": 65,
    "paper_remaining": 450,
    "toner_percent": 85
  }
}

# 任务完成事件:
{
  "type": "completed",
  "timestamp": "2024-01-15T10:35:00Z",
  "progress": {...},
  "message": "打印任务已完成"
}
```

## 数据库架构

### print_history 表
```sql
CREATE TABLE print_history (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    task_id INT NOT NULL UNIQUE,
    filename VARCHAR(255),
    pages INT,
    printed_pages INT,
    status VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    user_id VARCHAR(100),
    priority INT,
    pdf_path VARCHAR(255),  -- PDF 存储路径
    INDEX idx_user_id (user_id),
    INDEX idx_status (status),
    INDEX idx_created_at (created_at)
);
```

### pdf_storage 表
```sql
CREATE TABLE pdf_storage (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    task_id INT NOT NULL UNIQUE,
    filename VARCHAR(255),
    file_size BIGINT,
    file_hash VARCHAR(32),
    created_at TIMESTAMP,
    storage_path VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    INDEX idx_task_id (task_id),
    INDEX idx_created_at (created_at)
);
```

## 性能配置

### MySQL 连接池

编辑 `backend/mysql_database.go` 第 49-52 行：

```go
db.SetMaxOpenConns(25)      // 最大连接数
db.SetMaxIdleConns(5)       // 空闲连接数
db.SetConnMaxLifetime(5 * time.Minute)  // 连接最大生命周期
```

### PDF 存储限制

编辑 `backend/main.go` 第 1694 行：

```go
pdfManager, err := NewPDFManager(
    "./pdf_storage",  // 存储目录
    10,               // 最多保存 10 个 PDF
    1024              // 最多 1GB 存储空间
)
```

### 驱动缓冲区大小

编辑 `driver/driver_server.c` 第 17 行：

```c
#define BUFFER_SIZE 4096  // 二进制数据包缓冲区
```

## 故障排除

### 问题 1: MySQL 连接失败

**症状**:
```
[Warning] MySQL 初始化失败: dial tcp localhost:3306: connect: connection refused
[Info] 降级到使用 SQLite 数据库...
```

**解决方案**:
1. 确认 MySQL 服务已启动
2. 检查 MySQL 用户名和密码是否正确
3. 检查防火墙是否阻止 3306 端口
4. 使用 `mysql -u root -p` 测试连接

### 问题 2: WebSocket 连接失败

**症状**:
```
[Main] Progress tracker connection failed (WebSocket route may not be fully integrated yet)
```

**解决方案**:
1. 确认后端服务已启动
2. 浏览器控制台检查 WebSocket URL
3. 检查防火墙是否阻止 ws:// 连接
4. 刷新页面重新建立连接

### 问题 3: 二进制协议数据包校验和错误

**症状**:
```
[Driver] 错误: 校验和验证失败
```

**解决方案**:
1. 检查 `protocol.c` 中的 `protocol_calculate_checksum()` 实现
2. 检查 `binary_protocol.go` 中的 `CalculateChecksum()` 实现
3. 确保两边使用相同的校验和算法
4. 查看 `protocol.h` 中的协议定义

### 问题 4: PDF 存储路径权限错误

**症状**:
```
[Warning] PDF 管理器初始化失败: mkdir pdf_storage: permission denied
```

**解决方案**:
1. 检查当前目录的写权限
2. 手动创建 `./pdf_storage` 目录
3. 确保目录权限正确 (chmod 755)

## 测试脚本

### 完整端到端测试

```bash
#!/bin/bash

# 1. 登录
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

echo "登录令牌: $TOKEN"

# 2. 提交打印任务
TASK_ID=$(curl -s -X POST http://localhost:8080/api/job/submit \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"filename":"test.pdf","pages":50}' | jq -r '.task_id')

echo "任务ID: $TASK_ID"

# 3. 获取任务状态
curl -s -X GET http://localhost:8080/api/status \
  -H "Authorization: Bearer $TOKEN" | jq .

# 4. 获取PDF历史
curl -s -X GET http://localhost:8080/api/pdf/recent \
  -H "Authorization: Bearer $TOKEN" | jq .

# 5. 下载PDF
curl -s -X GET "http://localhost:8080/api/pdf/download?task_id=$TASK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -o "task_$TASK_ID.pdf"

echo "PDF已下载: task_$TASK_ID.pdf"
```

## 许可证

本项目为某大学本科毕业设计（论文）
课题: "基于Go和C语言的跨语言交互通信实现研究"

## 维护者

学生姓名: [待填]
指导老师: [待填]
学校: [待填]
完成日期: 2024-01-15
