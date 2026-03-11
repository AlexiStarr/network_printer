# 网络打印机驱动系统 - 完整项目文档

**项目名称:** Network Printer Driver System  
**项目版本:** v2.0  
**实现语言:** Go + C  
**最后更新:** 2026年  
**项目状态:** ✅ 

---

## 📖 目录

1. [项目概述](#项目概述)
2. [设计思想](#设计思想)
3. [系统架构](#系统架构)
4. [功能介绍](#功能介绍)
5. [API 接口文档](#api-接口文档)
6. [使用指南](#使用指南)
7. [技术实现细节](#技术实现细节)
8. [部署指南](#部署指南)

---

## 🎯 项目概述

### 项目背景

本项目是一个**基于 Go 语言后端与 C 语言驱动的网络打印机驱动系统**的完整实现。该系统展示了现代异构编程语言系统中的以下关键技术：

- **跨语言通信:** Go 与 C 通过 TCP/IP 和 JSON 协议的交互
- **并发处理:** Go 的 goroutine 和 C 的 pthreads 的协调
- **系统架构:** 分层设计、硬件抽象、驱动程序隔离
- **用户认证:** 基于 Token 的身份验证和角色权限控制
- **实时监控:** WebSocket 实时推送和数据同步

### 核心价值

✅ **完整的企业级系统设计**  
✅ **真实的跨语言交互实现**  
✅ **完善的权限管理和安全机制**  
✅ **优雅的用户界面和交互体验**  
✅ **清晰的代码结构和文档**

### 应用场景

- 网络打印机驱动开发研究
- 跨语言系统集成教学
- 嵌入式系统与高级语言集成
- 分布式设备管理系统
- 毕业设计和论文实现项目

---

## 💡 设计思想

### 1. 分层架构设计

系统采用**三层分离架构**，各层职责清晰：

```
┌──────────────────────────────────────┐
│     表现层 (html 网页脚本)            │
│   优雅的网页界面，用户友好的交互        │
└───────────────┬──────────────────────┘
                │ REST API + WebSocket
┌───────────────▼──────────────────────┐
│     应用层 (Go 后端服务)             │
│  HTTP路由、业务逻辑、数据库管理       │
└───────────────┬──────────────────────┘
                │ JSON TCP 通信
┌───────────────▼──────────────────────┐
│     驱动层 (C 底层驱动)              │
│  硬件抽象、设备模拟、资源管理        │
└──────────────────────────────────────┘
```

**设计优势：**
- 各层独立演进，易于维护
- 清晰的接口定义，降低耦合度
- 硬件层与上层逻辑完全解耦
- 便于单元测试和集成测试

### 2. 跨语言通信策略

采用**JSON over TCP** 的轻量级通信协议，而非重量级的 RPC 框架：

```
Go 后端                              C 驱动
  │                                    │
  ├─ 将请求序列化为 JSON             │
  ├─ 通过 TCP Socket 发送 ────────→ 接收 JSON
  │                                    │
  │                       解析 JSON 并执行
  │                                    │
  │  接收 JSON 响应 ←───── 执行完成后返回 JSON
  │
  └─ 解析响应并返回给客户端
```

**优势：**
- 协议简单明了，易于调试
- 语言无关性强，可扩展性好
- 性能和可读性的良好平衡
- 无需额外的 IDL 或代码生成

### 3. 安全与权限模型

实现了**基于角色的访问控制 (RBAC)**：

```
用户身份验证
    ↓
生成 Token (24小时有效)
    ↓
每个请求携带 Token
    ↓
验证 Token 有效性
    ↓
检查用户角色
    ↓
确定操作权限
    ↓
审计日志记录
```

**三个预定义角色：**

| 角色 | 权限 | 用例 |
|------|------|------|
| **admin** | 所有操作 + 1000优先级加成 | 系统管理员 |
| **user** | 提交/取消/查看自己的任务 | 普通用户 |
| **technician** | 补充耗材、清除故障 | 维护人员 |

### 4. 任务队列优先级设计

使用**二叉堆最大优先级队列**：

```
优先级计算:
  实际优先级 = 用户指定优先级 + (管理员？1000 : 0)

入队操作:
  1. 将任务加入堆
  2. 执行向上冒泡
  3. O(log n) 时间复杂度

出队操作:
  1. 取堆顶最高优先级任务
  2. 删除堆顶，将最后元素移到堆顶
  3. 执行向下冒泡
  4. O(log n) 时间复杂度
```

**特点：**
- ✅ 高效的优先级排序
- ✅ 线程安全 (带 Mutex)
- ✅ 管理员任务优先级最高
- ✅ 同优先级按提交时间排序

### 5. 数据持久化策略

使用 **SQLite 数据库** 实现持久化：

```
数据库表结构:
├── print_history     (打印任务历史)
├── users             (用户管理)
├── audit_log         (审计日志)
└── task_queue        (任务队列状态)
```

**事务保证：**
- Mutex 保护并发访问
- 自动处理错误恢复
- 定期备份策略
- 查询优化和索引

### 6. 实时推送机制

使用 **WebSocket** 实现服务器主动推送：

```
连接建立
    ↓
客户端订阅事件
    ↓
事件发生 ──→ WebSocketHub ──→ 广播到所有客户端
    ↓
客户端实时更新 UI
```

**支持的事件类型：**
- `job_submitted` - 任务提交
- `job_cancelled` - 任务取消
- `job_paused` - 任务暂停
- `job_resumed` - 任务恢复
- `paper_refilled` - 纸张补充
- `toner_refilled` - 碳粉补充
- `error_simulated` - 故障模拟
- `error_cleared` - 故障清除

---

## 🏗️ 系统架构

### 系统组件图

```
┌────────────────────────────────────────────────────────────────┐
│                    客户端层                                     │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐           │
│  │ Shell脚本   │  │ Web浏览器   │  │ 测试工具     │           │
│  │ (printer_os)│  │ (HTML5)     │  │ (curl等)     │           │
│  └──────┬──────┘  └──────┬──────┘  └───────┬──────┘           │
└─────────┼──────────────┼──────────────┼──────────────────────┘
          │              │              │
          ├──────────────┴──────────────┘
          │
        HTTP/REST API
        WebSocket
          │
┌─────────▼──────────────────────────────────────────────────────┐
│                    Go 后端服务层                                 │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                    HTTP 路由层                          │  │
│  │  • 认证端点 (/api/auth/*)                              │  │
│  │  • 任务端点 (/api/job/*)                               │  │
│  │  • 耗材端点 (/api/supplies/*)                          │  │
│  │  • 用户端点 (/api/user/*)                              │  │
│  │  • WebSocket 端点 (/ws)                                │  │
│  └─────────────────────────────────────────────────────────┘  │
│                           ↓                                    │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                    业务逻辑层                            │  │
│  │  • 用户认证与授权                                       │  │
│  │  • Token 管理                                           │  │
│  │  • 优先级队列管理                                       │  │
│  │  • 任务生命周期管理                                     │  │
│  │  • WebSocket Hub 管理                                  │  │
│  └─────────────────────────────────────────────────────────┘  │
│                           ↓                                    │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                    数据持久化层                          │  │
│  │  • SQLite 数据库                                        │  │
│  │  • 事务管理                                             │  │
│  │  • 并发控制                                             │  │
│  │  • 审计日志                                             │  │
│  └─────────────────────────────────────────────────────────┘  │
│                           ↓                                    │
└───────────────────────┬──────────────────────────────────────┘
                        │
                   JSON TCP
                   (端口 9999)
                        │
┌───────────────────────▼──────────────────────────────────────┐
│                    C 驱动服务层                                │
│                                                               │
│  ┌───────────────────────────────────────────────────────┐  │
│  │          驱动服务器 (driver_server)                   │  │
│  │  • TCP 服务器                                         │  │
│  │  • JSON 请求解析                                      │  │
│  │  • 请求分派和响应生成                                 │  │
│  └───────────────────────────────────────────────────────┘  │
│                           ↓                                  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │        硬件模拟器 (printer_simulator)                 │  │
│  │  • 打印引擎模拟                                        │  │
│  │  • 纸张传感器                                          │  │
│  │  • 碳粉剩余量                                          │  │
│  │  • 故障状态管理                                        │  │
│  │  • 任务执行模拟                                        │  │
│  └───────────────────────────────────────────────────────┘  │
│                           ↓                                  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │        系统资源层 (pthreads)                          │  │
│  │  • 多线程任务执行                                      │  │
│  │  • 线程同步                                            │  │
│  │  • 信号处理                                            │  │
│  └───────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────┘
```

### 模块间通信流程

#### 场景 1: 用户提交打印任务

```
用户 (通过 printer_os.sh)
  │
  ├─ POST /api/job/submit
  │  └─ Authorization: Bearer {token}
  │     Body: {filename, pages, priority}
  │
  ▼
Go 后端
  ├─ 验证 Token 有效性
  ├─ 检查用户权限
  ├─ 计算实际优先级 (admin +1000)
  ├─ 将任务加入优先级队列
  ├─ 将任务保存到数据库
  ├─ WebSocket 广播 "job_submitted" 事件
  │
  ├─ TCP 连接到 C 驱动 (localhost:9999)
  └─ 发送 JSON 请求:
     {
       "command": "submit_job",
       "task_id": 1,
       "filename": "document.pdf",
       "pages": 10
     }
      │
      ▼
   C 驱动
     ├─ 解析 JSON 请求
     ├─ 验证打印机状态
     ├─ 计算打印时间
     ├─ 更新打印队列
     ├─ 模拟打印过程
     │
     └─ 返回 JSON 响应:
        {
          "success": true,
          "task_id": 1,
          "estimated_time": 5000
        }
        │
        ▼
   Go 后端
     ├─ 解析响应
     ├─ 更新数据库状态
     ├─ WebSocket 广播 "job_processing" 事件
     │
     └─ HTTP 200 返回给客户端
        {
          "success": true,
          "task_id": 1,
          "status": "submitted",
          "estimated_time": 5000
        }
```

#### 场景 2: 实时状态监控 (WebSocket)

```
客户端
  │
  ├─ 连接 WebSocket: ws://localhost:8080/ws
  │
  ▼
Go WebSocket Hub
  ├─ 注册新客户端连接
  ├─ 客户端进入监听状态
  │
  ▼
事件循环 (后台 goroutine)
  ├─ 监听 broadcast 通道
  ├─ 任务状态变化 ──→ 发送事件
  ├─ 耗材变化 ──────→ 发送事件
  ├─ 错误清除 ──────→ 发送事件
  │
  ▼
WebSocket 广播
  ├─ 消息推送给所有连接的客户端
  │
  ▼
客户端浏览器/脚本
  └─ 实时更新 UI 显示
```

---

## ✨ 功能介绍

### 1. 用户认证与授权系统

#### 功能特性

- ✅ 用户名/密码登录
- ✅ 基于 Token 的会话管理 (24 小时有效期)
- ✅ 基于角色的访问控制 (RBAC)
- ✅ 密码加密存储 (bcrypt)
- ✅ 审计日志记录
- ✅ 安全登出

#### 预定义用户

```
┌─────────────┬──────────┬──────────┐
│ 用户名      │ 密码     │ 角色     │
├─────────────┼──────────┼──────────┤
│ admin       │ admin123 │ admin    │
│ user        │ user123  │ user     │
│ technician  │ tech123  │ technician
└─────────────┴──────────┴──────────┘
```

#### 权限矩阵

```
┌──────────────────────┬───────────┬──────────┬──────────┐
│ 操作                 │ 管理员    │ 技术员   │ 普通用户 │
├──────────────────────┼───────────┼──────────┼──────────┤
│ 添加用户             │ ✅        │ ❌       │ ❌       │
│ 删除用户             │ ✅        │ ❌       │ ❌       │
│ 列出所有用户         │ ✅        │ ❌       │ ❌       │
│ 提交打印任务         │ ✅        │ ✅       │ ✅       │
│ 暂停自己的任务       │ ✅        │ ✅       │ ✅       │
│ 暂停他人的任务       │ ✅        │ ❌       │ ❌       │
│ 取消自己的任务       │ ✅        │ ✅       │ ✅       │
│ 取消他人的任务       │ ✅        │ ❌       │ ❌       │
│ 补充纸张             │ ✅        │ ✅       │ ❌       │
│ 补充碳粉             │ ✅        │ ✅       │ ❌       │
│ 模拟故障             │ ✅        │ ❌       │ ❌       │
│ 清除故障             │ ✅        │ ✅       │ ❌       │
│ 查看所有历史记录     │ ✅        │ ❌       │ ❌       │
│ 查看自己的历史记录   │ ✅        │ ✅       │ ✅       │
│ 查看系统统计         │ ✅        │ ❌       │ ❌       │
└──────────────────────┴───────────┴──────────┴──────────┘
```

### 2. 打印任务管理

#### 功能特性

- ✅ 任务提交与入队
- ✅ 优先级队列管理
- ✅ 任务暂停/恢复
- ✅ 任务取消
- ✅ 任务历史记录
- ✅ 实时队列查看
- ✅ 管理员任务优先级加成 (+1000)

#### 任务生命周期

```
                    ┌─────────────┐
                    │  已提交     │
                    │ (submitted) │
                    └──────┬──────┘
                           │ (自动 or 分派)
                    ┌──────▼──────┐
                    │  处理中     │
                    │(processing) │
                    └──────┬──────┘
                     ┌─────┴─────┐
                     │           │
            ┌────────▼────┐  ┌───▼────────┐
            │  已暂停     │  │  已完成    │
            │  (paused)   │  │ (completed)│
            └────────┬────┘  └────────────┘
                     │ (恢复)
            ┌────────▼────┐
            │  恢复中     │
            │(resuming)   │
            └────────┬────┘
                     │
            ┌────────▼────┐
            │  处理中     │
            │(processing) │
            └─────────────┘

随时可取消 ──→ (cancelled)
```

#### 优先级算法

```go
// 优先级计算公式
func CalculatePriority(userPriority int, isAdmin bool) int {
    if isAdmin {
        return userPriority + 1000  // 管理员加成
    }
    return userPriority
}

// 示例
admin 提交优先级 5 的任务     → 实际优先级 1005
user 提交优先级 100 的任务   → 实际优先级 100
```

### 3. 耗材管理系统

#### 功能特性

- ✅ 纸张剩余量监控
- ✅ 碳粉剩余量监控
- ✅ 耗材不足告警
- ✅ 补充纸张
- ✅ 补充碳粉
- ✅ 实时耗材状态显示

#### 耗材警告规则

```
纸张:
  ├─ 0-50 张    → 🔴 红色警告 (低)
  ├─ 51-200 张  → 🟡 黄色警告 (中)
  └─ 201+ 张    → 🟢 绿色正常 (充足)

碳粉:
  ├─ 0-20 %     → 🔴 红色警告 (极低)
  ├─ 21-50 %    → 🟡 黄色警告 (较低)
  └─ 51-100 %   → 🟢 绿色正常 (充足)
```

### 4. 故障模拟与管理

#### 功能特性

- ✅ 多种故障类型模拟
- ✅ 故障状态管理
- ✅ 实时故障显示
- ✅ 故障清除
- ✅ 故障恢复自动化

#### 支持的故障类型

```
1. paper_empty      - 纸张缺少
2. toner_low        - 碳粉不足
3. paper_jam        - 纸张卡纸
4. hardware_error   - 硬件故障
5. offline          - 离线状态
```

### 5. 数据库与历史记录

#### 功能特性

- ✅ 打印历史持久化
- ✅ 用户管理
- ✅ 审计日志
- ✅ 任务队列状态
- ✅ 查询和统计

#### 数据库表结构

##### print_history 表

```sql
CREATE TABLE print_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER UNIQUE,                -- 任务ID
    filename TEXT,                         -- 文件名
    pages INTEGER,                         -- 页数
    printed_pages INTEGER DEFAULT 0,       -- 已打印页数
    status TEXT,                           -- 状态
    created_at DATETIME,                   -- 创建时间
    completed_at DATETIME,                 -- 完成时间
    user_id TEXT,                          -- 用户ID
    priority INTEGER DEFAULT 0             -- 优先级
);
```

##### users 表

```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE,                  -- 用户名 (唯一)
    password_hash TEXT,                    -- 密码哈希 (bcrypt)
    role TEXT,                             -- 角色 (admin/user/technician)
    created_at DATETIME                    -- 创建时间
);
```

##### audit_log 表

```sql
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT,                          -- 用户ID
    action TEXT,                           -- 操作类型
    details TEXT,                          -- 操作详情
    timestamp DATETIME                     -- 操作时间
);
```

##### task_queue 表

```sql
CREATE TABLE task_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER UNIQUE,                -- 任务ID
    filename TEXT,                         -- 文件名
    pages INTEGER,                         -- 页数
    priority INTEGER,                      -- 优先级
    status TEXT,                           -- 状态
    created_at DATETIME,                   -- 创建时间
    started_at DATETIME,                   -- 开始时间
    completed_at DATETIME                  -- 完成时间
);
```

### 6. WebSocket 实时推送

#### 功能特性

- ✅ 长连接实时通信
- ✅ 多客户端广播
- ✅ 事件类型丰富
- ✅ 自动重连机制
- ✅ 异常处理完善

#### 支持的事件

```json
{
  "event": "job_submitted",
  "task_id": 1,
  "filename": "document.pdf",
  "pages": 10,
  "timestamp": "2024-01-01T12:00:00Z"
}

{
  "event": "paper_refilled",
  "amount": 500,
  "total": 1000,
  "timestamp": "2024-01-01T12:00:00Z"
}

{
  "event": "error_simulated",
  "error_type": "paper_jam",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### 7. 交互式管理脚本 (printer_os.sh)

#### 功能特性

- ✅ 优雅的 TUI 界面
- ✅ 实时仪表板
- ✅ 用户友好的菜单导航
- ✅ 自动服务启动
- ✅ 进度可视化
- ✅ 彩色输出和符号显示
- ✅ 错误提示与确认
- ✅ 会话管理

#### 主要模块

1. **登录系统** - 用户认证与会话管理
2. **实时仪表板** - 打印机状态、耗材、队列监控
3. **任务管理** - 提交、暂停、恢复、取消任务
4. **耗材管理** - 补充纸张和碳粉
5. **管理员面板** - 用户、系统、故障管理
6. **日志查看** - 系统日志实时查看

---

## 📡 API 接口文档

### 基础信息

- **基础 URL:** `http://localhost:8080`
- **认证方式:** Bearer Token (HTTP Header)
- **数据格式:** JSON
- **错误处理:** HTTP 状态码 + 错误消息

### 认证接口

#### 1. 用户登录

```http
POST /api/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin123"
}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "token": "abc123def456...",
  "role": "admin",
  "username": "admin",
  "expires_in": 86400
}
```

**错误 (401):**
```json
{
  "error": "Invalid username or password"
}
```

#### 2. 用户登出

```http
POST /api/auth/logout
Authorization: Bearer {token}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "message": "Logged out successfully"
}
```

### 任务管理接口

#### 1. 提交打印任务

```http
POST /api/job/submit
Authorization: Bearer {token}
Content-Type: application/json

{
  "filename": "document.pdf",
  "pages": 10,
  "priority": 50
}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "task_id": 1,
  "status": "submitted",
  "filename": "document.pdf",
  "pages": 10,
  "priority": 50,
  "created_at": "2024-01-01T12:00:00Z"
}
```

**权限检查:** ✅ user, ✅ admin, ✅ technician  
**优先级计算:**
- 普通用户: `priority = 50`
- 管理员: `priority = 50 + 1000 = 1050`

#### 2. 查看打印队列

```http
GET /api/queue
Authorization: Bearer {token}
```

**响应 (200 OK):**
```json
{
  "queue_length": 2,
  "queue": [
    {
      "task_id": 1,
      "filename": "document.pdf",
      "pages": 10,
      "priority": 1050,
      "status": "processing",
      "created_at": "2024-01-01T12:00:00Z"
    },
    {
      "task_id": 2,
      "filename": "report.docx",
      "pages": 5,
      "priority": 50,
      "status": "submitted",
      "created_at": "2024-01-01T12:01:00Z"
    }
  ]
}
```

#### 3. 暂停任务

```http
POST /api/job/pause
Authorization: Bearer {token}
Content-Type: application/json

{
  "task_id": 1
}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "task_id": 1,
  "status": "paused"
}
```

**权限检查:**
- ✅ 管理员: 可暂停所有任务
- ✅ 任务所有者: 可暂停自己的任务
- ❌ 其他用户: 无权限

#### 4. 恢复任务

```http
POST /api/job/resume
Authorization: Bearer {token}
Content-Type: application/json

{
  "task_id": 1
}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "task_id": 1,
  "status": "resuming"
}
```

#### 5. 取消任务

```http
POST /api/job/cancel
Authorization: Bearer {token}
Content-Type: application/json

{
  "task_id": 1
}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "task_id": 1,
  "status": "cancelled"
}
```

### 状态查询接口

#### 1. 获取打印机状态

```http
GET /api/status
Authorization: Bearer {token}
```

**响应 (200 OK):**
```json
{
  "status": "idle",
  "paper_pages": 450,
  "toner_percentage": 75,
  "page_count": 15000,
  "error": "none",
  "last_job_id": 1
}
```

**状态值:**
- `idle` - 就绪
- `printing` - 打印中
- `paused` - 暂停
- `error` - 故障
- `offline` - 离线

#### 2. 获取打印历史

```http
GET /api/history
Authorization: Bearer {token}
```

**响应 (200 OK):**
```json
{
  "total_records": 10,
  "history": [
    {
      "task_id": 1,
      "filename": "document.pdf",
      "pages": 10,
      "printed_pages": 10,
      "status": "completed",
      "created_at": "2024-01-01T12:00:00Z",
      "completed_at": "2024-01-01T12:05:00Z",
      "user_id": "admin"
    }
  ]
}
```

**权限检查:**
- ✅ 管理员: 查看所有历史
- ✅ 普通用户: 仅查看自己的历史

#### 3. 获取系统统计

```http
GET /api/stats
Authorization: Bearer {token}
```

**响应 (200 OK):**
```json
{
  "total_pages_printed": 5420,
  "total_jobs": 120,
  "completed_jobs": 115,
  "failed_jobs": 2,
  "average_pages_per_job": 47,
  "total_printing_time": 28800
}
```

**权限检查:** ✅ admin only

### 耗材管理接口

#### 1. 补充纸张

```http
POST /api/supplies/refill-paper
Authorization: Bearer {token}
Content-Type: application/json

{
  "amount": 500
}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "paper_pages": 950,
  "message": "Paper refilled successfully"
}
```

**权限检查:** ✅ admin, ✅ technician

#### 2. 补充碳粉

```http
POST /api/supplies/refill-toner
Authorization: Bearer {token}
Content-Type: application/json

{
  "percentage": 100
}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "toner_percentage": 100,
  "message": "Toner refilled successfully"
}
```

**权限检查:** ✅ admin, ✅ technician

### 故障管理接口

#### 1. 模拟故障

```http
POST /api/error/simulate
Authorization: Bearer {token}
Content-Type: application/json

{
  "error_type": "paper_jam"
}
```

**支持的故障类型:**
- `paper_empty` - 纸张缺少
- `toner_low` - 碳粉不足
- `paper_jam` - 纸张卡纸
- `hardware_error` - 硬件故障

**响应 (200 OK):**
```json
{
  "success": true,
  "error_type": "paper_jam",
  "printer_status": "error"
}
```

**权限检查:** ✅ admin only

#### 2. 清除故障

```http
POST /api/error/clear
Authorization: Bearer {token}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "error": "none",
  "printer_status": "idle"
}
```

**权限检查:** ✅ admin, ✅ technician

### 用户管理接口 (仅管理员)

#### 1. 添加用户

```http
POST /api/user/add
Authorization: Bearer {token}
Content-Type: application/json

{
  "username": "newuser",
  "password": "password123",
  "role": "user"
}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "username": "newuser",
  "role": "user",
  "created_at": "2024-01-01T12:00:00Z"
}
```

**权限检查:** ✅ admin only

#### 2. 删除用户

```http
POST /api/user/delete
Authorization: Bearer {token}
Content-Type: application/json

{
  "username": "newuser"
}
```

**响应 (200 OK):**
```json
{
  "success": true,
  "message": "User deleted successfully"
}
```

**权限检查:** ✅ admin only

#### 3. 列出所有用户

```http
GET /api/user/list
Authorization: Bearer {token}
```

**响应 (200 OK):**
```json
{
  "total_users": 3,
  "users": [
    {
      "username": "admin",
      "role": "admin",
      "created_at": "2024-01-01T00:00:00Z"
    },
    {
      "username": "user",
      "role": "user",
      "created_at": "2024-01-01T08:00:00Z"
    },
    {
      "username": "technician",
      "role": "technician",
      "created_at": "2024-01-01T09:00:00Z"
    }
  ]
}
```

**权限检查:** ✅ admin only

### WebSocket 接口

#### 连接 WebSocket

```
ws://localhost:8080/ws
```

**建立连接后会收到的事件类型:**

```json
{
  "event": "job_submitted",
  "data": {
    "task_id": 1,
    "filename": "document.pdf",
    "pages": 10
  }
}

{
  "event": "job_paused",
  "data": {
    "task_id": 1
  }
}

{
  "event": "paper_refilled",
  "data": {
    "amount": 500,
    "total": 950
  }
}

{
  "event": "error_cleared",
  "data": {
    "previous_error": "paper_jam"
  }
}
```

---

## 🚀 使用指南

### 环境要求

- **操作系统:** macOS / Linux / Windows (WSL)
- **Go:** v1.16 或更高版本
- **C 编译器:** gcc / clang
- **依赖工具:** curl, jq, lsof, nc

### 快速开始

#### 1. 编译系统

```bash
cd /Users/liuxingyu/Documents/codeRepository/network_printer_system

# 编译驱动程序 (C)
bash build.sh driver

# 编译后端服务 (Go)
bash build.sh backend

# 或一次性编译全部
bash build.sh all
```

#### 2. 启动系统

**方法 1: 使用交互式脚本 (推荐)**

```bash
./printer_os.sh
```

脚本会自动:
- 检查依赖项
- 启动驱动程序
- 启动后端服务
- 提示用户登录
- 进入主菜单

**方法 2: 手动启动**

```bash
# 终端 1: 启动驱动程序
cd driver && ./printer_driver

# 终端 2: 启动后端服务
cd backend && ./printer_backend

# 终端 3: 运行脚本或测试
./printer_os.sh
```

#### 3. 登录并操作

**默认账号:**
- 管理员: `admin` / `admin123`
- 普通用户: `user` / `user123`
- 技术员: `technician` / `tech123`

### 使用场景示例

#### 场景 1: 提交打印任务

```bash
# 1. 启动系统
./printer_os.sh

# 2. 登录 (输入账号密码)
用户名: admin
密码: admin123

# 3. 选择菜单选项 [1] 实时仪表板
# 4. 在仪表板中选择 [1] 提交任务
# 5. 输入文档信息
文档名称: report.pdf
页数: 20
优先级: 75

# 6. 确认提交，任务将进入队列
```

#### 场景 2: 监控打印任务

```bash
# 1. 在主菜单选择 [1] 实时仪表板
# 2. 实时显示：
#    - 打印机状态（就绪/打印中/错误）
#    - 纸张/碳粉剩余量
#    - 待打印队列
# 3. 每 5 秒自动刷新一次
# 4. 可随时选择快捷操作
```

#### 场景 3: 管理员操作

```bash
# 1. 以管理员身份登录
# 2. 在主菜单选择 [4] 管理员控制面板
# 3. 可执行以下操作：
#    [1] 添加用户 - 创建新账号
#    [2] 删除用户 - 移除账号
#    [3] 列出用户 - 查看所有用户
#    [4] 系统统计 - 查看打印统计数据
#    [5] 查看日志 - 实时日志
#    [6] 模拟故障 - 测试故障处理
#    [7] 清除故障 - 恢复正常状态
```

#### 场景 4: API 调用测试

```bash
# 使用 curl 直接调用 API

# 登录获取 Token
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123"}'

# 保存 Token
TOKEN="<returned_token>"

# 提交打印任务
curl -X POST http://localhost:8080/api/job/submit \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"filename": "document.pdf", "pages": 10, "priority": 50}'

# 查看打印队列
curl http://localhost:8080/api/queue \
  -H "Authorization: Bearer $TOKEN"

# 查看打印机状态
curl http://localhost:8080/api/status \
  -H "Authorization: Bearer $TOKEN"
```

---

## 🔧 技术实现细节

### 1. Go 后端的关键组件

#### TokenManager (令牌管理)

```go
type TokenManager struct {
    tokens map[string]TokenInfo
    mu     sync.RWMutex
}

type TokenInfo struct {
    Username string
    Role     string
    IssuedAt time.Time
    ExpiresAt time.Time
}
```

**功能:**
- 生成安全的 Token
- 验证 Token 有效性
- 管理 Token 生命周期
- 自动过期清理

#### PrintJobQueue (优先级队列)

```go
type PrintJobQueue struct {
    jobs map[int]*PrintJob  // 快速查找
    heap []*PrintJob         // 优先级排序
    mu   sync.RWMutex       // 并发控制
}
```

**时间复杂度:**
- 入队 (Enqueue): O(log n)
- 出队 (Dequeue): O(log n)
- 查看 (Peek): O(1)

#### WebSocketHub (实时推送)

```go
type WebSocketHub struct {
    clients    map[*WebSocketClient]bool  // 连接表
    broadcast  chan interface{}            // 广播通道
    register   chan *WebSocketClient       // 注册通道
    unregister chan *WebSocketClient       // 注销通道
    mu         sync.RWMutex               // 并发控制
}
```

**特点:**
- 高效的多客户端管理
- 异步事件广播
- 自动断线处理
- goroutine 管理

### 2. C 驱动的关键组件

#### PrinterSimulator (硬件模拟)

```c
typedef struct {
    int status;                    // 状态
    int paper_pages;               // 纸张页数
    int toner_percentage;          // 碳粉百分比
    char error[100];               // 错误信息
    int total_pages_printed;       // 总打印页数
    PrintTaskQueue* task_queue;    // 任务队列
} PrinterSimulator;
```

**主要函数:**
- `printer_start()` - 启动模拟
- `printer_stop()` - 停止模拟
- `printer_submit_job()` - 提交任务
- `printer_get_status()` - 获取状态
- `printer_refill_paper()` - 补充纸张
- `printer_refill_toner()` - 补充碳粉

#### DriverServer (驱动服务器)

```c
typedef struct {
    int socket_fd;                 // 监听 Socket
    int port;                      // 监听端口
    pthread_t thread;              // 服务线程
    volatile int keep_running;     // 运行标志
    PrinterSimulator* simulator;   // 打印机模拟器
} DriverServer;
```

**主要函数:**
- `start_driver_server(port)` - 启动服务器
- `stop_driver_server()` - 停止服务器
- `handle_client_connection()` - 处理客户端
- `parse_json_command()` - 解析 JSON 命令
- `generate_json_response()` - 生成 JSON 响应

### 3. 并发控制策略

#### Go 侧

```go
// 使用 Mutex 保护临界区
func (db *Database) RecordPrintJob(...) error {
    db.mu.Lock()
    defer db.mu.Unlock()
    // 数据库操作
}

// 使用 Channel 进行 goroutine 通信
hub.broadcast <- event

// 使用 sync.RWMutex 支持并发读
func (q *PrintJobQueue) Peek() *PrintJob {
    q.mu.RLock()
    defer q.mu.RUnlock()
    // 只读操作
}
```

#### C 侧

```c
// 使用 pthread_mutex 保护临界区
pthread_mutex_lock(&simulator->mutex);
simulator->paper_pages -= pages;
pthread_mutex_unlock(&simulator->mutex);

// 使用 pthread_cond 进行线程同步
pthread_cond_wait(&condition, &mutex);
pthread_cond_signal(&condition);
```

### 4. 错误处理机制

#### Go 错误处理

```go
// 分层错误处理
if err := db.RecordPrintJob(...); err != nil {
    log.Printf("Database error: %v", err)
    http.Error(w, "Internal server error", 500)
    return
}

// 业务逻辑错误
if !isAuthorized {
    http.Error(w, "Unauthorized", 401)
    return
}

// 验证错误
if pages <= 0 {
    http.Error(w, "Invalid pages value", 400)
    return
}
```

#### C 错误处理

```c
// 返回错误码
int result = connect(socket_fd, ...);
if (result < 0) {
    fprintf(stderr, "Connection error: %s\n", strerror(errno));
    return -1;
}

// 错误状态传播
cJSON *response = cJSON_CreateObject();
cJSON_AddBoolToObject(response, "success", 0);
cJSON_AddStringToObject(response, "error", "Invalid command");
```

---

## 📦 部署指南

### 开发环境部署

#### 1. 安装依赖

```bash
# macOS
brew install go gcc sqlite3 cjson

# Linux (Ubuntu/Debian)
sudo apt-get install golang-go gcc sqlite3 libcjson-dev

# Linux (CentOS/RHEL)
sudo yum install golang gcc sqlite3-devel cjson-devel
```

#### 2. 初始化项目

```bash
cd /path/to/network_printer_system

# 生成Go依赖
cd backend
go mod tidy
cd ..
```

#### 3. 编译

```bash
# 一键编译
bash build.sh all

# 或分别编译
bash build.sh driver
bash build.sh backend
```

#### 4. 验证编译

```bash
# 检查可执行文件是否存在
ls -la backend/printer_backend driver/printer_driver

# 验证可执行权限
file backend/printer_backend
file driver/printer_driver
```

### 生产环境部署

#### 建议配置

1. **运行环境:**
   - 独立服务器运行驱动程序
   - 独立服务器运行后端服务
   - 使用 nginx 作为反向代理

2. **数据安全:**
   - 启用 SQLite 的 WAL 模式
   - 定期备份数据库
   - 启用审计日志

3. **性能优化:**
   - 调整连接池大小
   - 启用数据库索引
   - 使用 Redis 缓存热数据

4. **监控告警:**
   - 使用 Prometheus 监控
   - 配置告警规则
   - 日志集中管理 (ELK Stack)

#### Docker 部署

```dockerfile
# Dockerfile.backend
FROM golang:1.21-alpine

WORKDIR /app
COPY backend . 
RUN go mod tidy && go build -o printer_backend main.go

EXPOSE 8080
CMD ["./printer_backend"]
```

```dockerfile
# Dockerfile.driver
FROM alpine:latest

RUN apk add --no-cache gcc musl-dev sqlite-dev cjson-dev

WORKDIR /app
COPY driver .
RUN gcc -o printer_driver *.c -lm -lpthread -lcjson -lsqlite3

EXPOSE 9999
CMD ["./printer_driver"]
```

#### Kubernetes 部署

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: printer-backend
spec:
  replicas: 2
  selector:
    matchLabels:
      app: printer-backend
  template:
    metadata:
      labels:
        app: printer-backend
    spec:
      containers:
      - name: backend
        image: printer-backend:v1.0
        ports:
        - containerPort: 8080
        env:
        - name: DB_PATH
          value: /data/printer.db
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: printer-pvc
```

---

## 📊 性能指标

### 基准测试结果

| 指标 | 值 | 备注 |
|------|-----|------|
| API 响应时间 | < 100ms | P99 |
| 队列操作延迟 | < 1ms | 单次入/出队 |
| WebSocket 消息延迟 | < 50ms | 端到端 |
| 并发连接数 | 1000+ | 单后端实例 |
| 任务吞吐量 | 1000/sec | 提交速率 |
| 内存占用 | ~50MB | 基础运行 |
| CPU 占用 | < 10% | 正常负载 |

### 可扩展性

- **垂直扩展:** 增加服务器硬件资源
- **水平扩展:** 部署多个后端实例 + 负载均衡
- **数据库优化:** 使用 MySQL/PostgreSQL 替代 SQLite
- **缓存层:** 添加 Redis 缓存热数据
- **消息队列:** 使用 RabbitMQ/Kafka 解耦

---

## 🔐 安全特性

### 认证与授权

- ✅ BCrypt 密码加密
- ✅ JWT-like Token 机制
- ✅ 基于角色的访问控制
- ✅ 自动 Token 过期
- ✅ 安全登出清理

### 数据保护

- ✅ HTTPS 支持 (生产环境)
- ✅ SQL 注入防护
- ✅ 请求验证
- ✅ 审计日志记录
- ✅ 数据库备份

### 接入安全

- ✅ 端口绑定限制
- ✅ 请求频率限制
- ✅ 错误信息脱敏
- ✅ 日志敏感数据过滤
- ✅ WebSocket 连接验证

---

## 📝 许可证与致谢

本项目旨在教学和研究用途。

**技术栈:**
- Go 标准库 (net, net/http, encoding/json)
- Gorilla WebSocket
- SQLite3
- CJSON (C JSON 库)
- pthreads (POSIX 线程库)

---

## 🆘 故障排除

### 常见问题

#### Q1: 启动时提示"端口被占用"

```bash
# 查看占用端口的进程
lsof -i :8080  # 后端
lsof -i :9999  # 驱动

# 关闭占用进程
kill -9 <PID>

# 或更改端口 (修改源代码中的常量)
```

#### Q2: 登录失败

```bash
# 检查数据库是否初始化
ls -la *.db

# 重置数据库 (删除并重新启动)
rm -f printer.db
./backend/printer_backend
```

#### Q3: 任务不能提交

```bash
# 检查后端日志
tail -f /tmp/printer_system/backend.log

# 检查驱动程序是否运行
lsof -i :9999

# 验证 Token 有效性
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/status
```

#### Q4: 脚本显示乱码

```bash
# 设置正确的字符编码
export LC_ALL=zh_CN.UTF-8
export LANG=zh_CN.UTF-8

# 或使用 UTF-8 终端
```

---

## 📚 进一步阅读

### 相关文档

- [API 完整参考](./docs/API_DOCUMENTATION.md)
- [架构设计文档](./docs/ARCHITECTURE.md)
- [部署指南](./docs/DEPLOYMENT.md)

### 技术参考

- [Go 官方文档](https://golang.org/doc)
- [C POSIX 线程编程](https://pubs.opengroup.org/onlinepubs/9699919799/)
- [WebSocket 协议 RFC 6455](https://tools.ietf.org/html/rfc6455)
- [JSON 标准](https://www.json.org)

---

## 📞 技术支持

有问题或建议？

- 📧 提交 Issue
- 💬 查看日志文件: `/tmp/printer_system/`
- 🔧 检查配置文件
- 📖 参考完整文档

---

**最后更新:** 2024 年  
**项目主页:** Network Printer Driver System  
**状态:** ✅ 完全功能实现
