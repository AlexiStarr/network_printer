# API 完整文档

## 基础信息

- **基础 URL**：`http://localhost:8080`
- **驱动通信端口**：`9090`
- **通信协议**：HTTP REST API / TCP JSON
- **内容类型**：`application/json`

## 1. 系统管理接口

### 1.1 健康检查

**请求**
```http
GET /health HTTP/1.1
Host: localhost:8080
```

**响应** (200 OK)
```json
{
  "status": "ok"
}
```

**用途**：检查后端服务是否正常运行

---

### 1.2 获取打印机状态

**请求**
```http
GET /api/status HTTP/1.1
Host: localhost:8080
```

**响应** (200 OK)
```json
{
  "model": "NetworkPrinter Pro X200",
  "serial_number": "NP-X200-2024001",
  "firmware_version": "3.2.1",
  "status": "idle|printing|error|offline",
  "error": "OK|PAPER_EMPTY|TONER_LOW|TONER_EMPTY|HEAT_UNAVAILABLE|MOTOR_FAILURE|SENSOR_FAILURE",
  "temperature": 25,
  "page_count": 0,
  "paper_pages": 500,
  "toner_percentage": 100
}
```

**字段说明**：
| 字段 | 类型 | 说明 |
|------|------|------|
| model | string | 打印机型号 |
| serial_number | string | 序列号 |
| firmware_version | string | 固件版本 |
| status | string | 当前状态 |
| error | string | 错误状态 |
| temperature | number | 打印头温度（摄氏度） |
| page_count | number | 总打印页数 |
| paper_pages | number | 纸张剩余页数 |
| toner_percentage | number | 碳粉百分比 (0-100) |

---

### 1.3 获取系统统计信息

**请求**
```http
GET /api/stats HTTP/1.1
Host: localhost:8080
```

**响应** (200 OK)
```json
{
  "total_pages_printed": 150,
  "timestamp": "2024-02-28T10:30:45Z",
  "uptime": "running"
}
```

**字段说明**：
| 字段 | 类型 | 说明 |
|------|------|------|
| total_pages_printed | number | 累计打印页数 |
| timestamp | string | 当前时间戳 (ISO 8601) |
| uptime | string | 系统运行状态 |

---

## 2. 打印任务接口

### 2.1 提交打印任务

**请求**
```http
POST /api/job/submit HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Content-Length: 50

{
  "filename": "document.pdf",
  "pages": 10
}
```

**请求参数**：
| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| filename | string | 是 | 文件名 |
| pages | number | 是 | 打印页数 (>0) |

**响应** (200 OK - 成功)
```json
{
  "success": true,
  "task_id": 1
}
```

**响应** (500 Internal Server Error - 失败)
```json
{
  "success": false,
  "error": "Failed to submit job"
}
```

**可能的错误**：
- 硬件出错（缺纸、缺粉等）
- 页数无效
- 队列已满

---

### 2.2 取消打印任务

**请求**
```http
POST /api/job/cancel HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Content-Length: 20

{
  "task_id": 1
}
```

**请求参数**：
| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| task_id | number | 是 | 任务 ID |

**响应** (200 OK - 成功)
```json
{
  "success": true
}
```

**响应** (500 Internal Server Error - 失败)
```json
{
  "success": false,
  "error": "Failed to cancel job"
}
```

**可能的错误**：
- 任务不存在
- 任务正在打印（无法取消）
- 任务已完成

---

### 2.3 获取打印队列

**请求**
```http
GET /api/queue HTTP/1.1
Host: localhost:8080
```

**响应** (200 OK)
```json
{
  "tasks": [
    {
      "task_id": 1,
      "filename": "document.pdf",
      "page_count": 10,
      "printed_pages": 5,
      "status": "printing"
    },
    {
      "task_id": 2,
      "filename": "report.docx",
      "page_count": 15,
      "printed_pages": 0,
      "status": "queued"
    }
  ],
  "queue_size": 1
}
```

**字段说明**：
| 字段 | 类型 | 说明 |
|------|------|------|
| tasks | array | 任务列表 |
| task_id | number | 任务编号 |
| filename | string | 文件名 |
| page_count | number | 总页数 |
| printed_pages | number | 已打印页数 |
| status | string | 任务状态 (printing/queued/completed) |
| queue_size | number | 等待中的任务数 |

---

## 3. 耗材管理接口

### 3.1 补充纸张

**请求**
```http
POST /api/supplies/refill-paper HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Content-Length: 20

{
  "pages": 500
}
```

**请求参数**：
| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| pages | number | 是 | 补充的纸张页数 |

**响应** (200 OK)
```json
{
  "success": true,
  "paper_pages": 1000
}
```

**约束**：
- 最多可存储 5000 页纸张

---

### 3.2 补充碳粉

**请求**
```http
POST /api/supplies/refill-toner HTTP/1.1
Host: localhost:8080
```

**响应** (200 OK)
```json
{
  "success": true,
  "toner_percentage": 100
}
```

**说明**：
- 碳粉恢复到 100%
- 自动清除碳粉相关的错误状态

---

## 4. 错误处理接口

### 4.1 清除错误

**请求**
```http
POST /api/error/clear HTTP/1.1
Host: localhost:8080
```

**响应** (200 OK)
```json
{
  "success": true
}
```

**说明**：
- 清除所有硬件错误
- 恢复打印机到空闲状态

---

### 4.2 模拟硬件故障

**请求**
```http
POST /api/error/simulate HTTP/1.1
Host: localhost:8080
Content-Type: application/json
Content-Length: 30

{
  "error": "PAPER_EMPTY"
}
```

**请求参数**：
| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| error | string | 是 | 故障类型 |

**支持的故障类型**：
| 故障类型 | 说明 |
|---------|------|
| `PAPER_EMPTY` | 缺纸 |
| `TONER_LOW` | 碳粉不足 |
| `TONER_EMPTY` | 缺少碳粉 |
| `HEAT_UNAVAILABLE` | 加热器故障 |
| `MOTOR_FAILURE` | 电机故障 |
| `SENSOR_FAILURE` | 传感器故障 |

**响应** (200 OK - 成功)
```json
{
  "success": true
}
```

**响应** (500 Internal Server Error - 失败)
```json
{
  "success": false,
  "error": "Invalid error type"
}
```

---

## 5. 错误码参考

### HTTP 状态码
| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 400 | 请求参数错误 |
| 500 | 服务器错误 |

### 业务错误
| 错误信息 | 原因 |
|--------|------|
| Failed to submit job | 提交任务失败（硬件错误或队列满） |
| Failed to cancel job | 取消任务失败（任务不存在或正在打印） |
| Invalid parameters | 参数格式错误 |
| Unknown command | 未知命令（驱动错误） |

---

## 6. 典型使用场景

### 场景 1：提交并监控单个打印任务

```bash
# 1. 提交任务
curl -X POST http://localhost:8080/api/job/submit \
  -H "Content-Type: application/json" \
  -d '{"filename":"report.pdf","pages":20}' \
  | jq '.task_id' > task_id.txt

TASK_ID=$(cat task_id.txt)

# 2. 循环查询状态
for i in {1..10}; do
  echo "轮次 $i:"
  curl http://localhost:8080/api/queue | jq .
  sleep 2
done

# 3. 最终统计
curl http://localhost:8080/api/stats | jq .
```

### 场景 2：处理硬件故障

```bash
# 1. 模拟缺纸
curl -X POST http://localhost:8080/api/error/simulate \
  -H "Content-Type: application/json" \
  -d '{"error":"PAPER_EMPTY"}'

# 2. 验证状态
curl http://localhost:8080/api/status | jq '.error, .status'

# 3. 补充纸张
curl -X POST http://localhost:8080/api/supplies/refill-paper \
  -H "Content-Type: application/json" \
  -d '{"pages":1000}'

# 4. 清除错误
curl -X POST http://localhost:8080/api/error/clear

# 5. 确认恢复
curl http://localhost:8080/api/status | jq '.error, .status'
```

### 场景 3：批量提交任务

```bash
#!/bin/bash

# 提交 5 个任务
for i in {1..5}; do
  echo "提交任务 $i..."
  curl -X POST http://localhost:8080/api/job/submit \
    -H "Content-Type: application/json" \
    -d "{\"filename\":\"doc$i.pdf\",\"pages\":$((i*5))}"
  sleep 0.5
done

# 查看队列
echo "队列状态:"
curl http://localhost:8080/api/queue | jq '.tasks | length'
```

---

## 7. 数据流示意图

```
客户端请求
   │
   ├─→ POST /api/job/submit
   │    └─→ Go 后端解析请求
   │         └─→ 转换为 JSON 并发送到驱动
   │              └─→ C 驱动处理请求
   │                   ├─→ 验证硬件状态
   │                   ├─→ 添加到队列
   │                   └─→ 返回结果 JSON
   │         └─→ Go 后端解析响应
   │              └─→ 返回 HTTP 响应
   │
   └─→ 响应 {"success": true, "task_id": 1}
```

---

## 8. 性能建议

### 请求间隔
- 避免频繁的状态查询
- 推荐间隔：500ms - 1s

### 并发连接
- 最大推荐并发：100 connections
- 驱动能处理更多，但客户端应限流

### 数据大小
- 单个请求限制：< 8KB
- 响应大小：通常 < 4KB

---

## 9. 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0 | 2024-02-28 | 初始版本 |

---

**文档最后更新**：2024-02-28
