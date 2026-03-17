# 🖨️ 网络打印机控制系统

[![Go](https://img.shields.io/badge/Go-1.16+-blue.svg)](https://golang.org)
[![C](https://img.shields.io/badge/C-99-green.svg)](https://en.wikipedia.org/wiki/C_(programming_language))
[![JavaScript](https://img.shields.io/badge/JavaScript-ES6+-yellow.svg)](https://developer.mozilla.org/en-US/docs/Web/JavaScript)
[![MySQL](https://img.shields.io/badge/MySQL-8.0+-orange.svg)](https://www.mysql.com)
[![Python](https://img.shields.io/badge/Python-3-blue.svg)](https://www.python.org)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20Windows-brightgreen.svg)](#)
[![License](https://img.shields.io/badge/License-MIT-red.svg)](LICENSE)

一个**企业级、功能完整、架构清晰**的网络打印机控制系统。采用Go后端、C驱动、现代Web前端，支持完整的打印任务管理、硬件监控、真实的PDF文件处理和实时通知。

**✨ 现已支持 Linux 和 Windows 跨平台运行！**

## ✨ 核心特性

### 🎯 功能完整
- ✅ **真实的PDF处理** - 完整的真实PDF文件上传、存储、解析和打印
- ✅ **自定义二进制通信协议** - Go后端与C驱动间基于高性能二进制协议通信
- ✅ **硬件抽象状态机 (State Machine)** - 驱动层使用状态机管理所有行为模式
- ✅ **高精度实时进度追踪** - 实时作业进度追踪与可视化
- ✅ **优先级队列** - 堆数据结构确保高优先级优先处理
- ✅ **用户认证与权限管理** - 基于角色的访问控制（RBAC）
- ✅ **数据持久化** - 引入 MySQL 关系型数据库自动迁移，完整的审计与状态记录
- ✅ **专业测试套件** - 配备完善的单元、集成和性能测试栈，附带 Python 数据可视化
- ✅ **耗材管理与错误模拟** - 包含完整的各种耗材管理、硬件错误状态与监控机制

### 🏗️ 架构完善
```text
┌─────────────────────────────────────┐
│      Web前端 (HTML/JS/CSS)          │ 现代化UI + 实时进度展示
├─────────────────────────────────────┤
│    Go后端 (localhost:8080)          │ REST API + WebSocket + PDF管理
├─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─┤
│    二进制协议 (Binary Protocol)     │ 二进制高速数据流，心跳维持
├─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─┤
│   C驱动 (localhost:9999)            │ 硬件状态机 (State Machine) 架构
└─────────────────────────────────────┘
```

## 📦 快速开始

### 前置要求
```bash
Go 1.16+
GCC/Clang 或 MinGW (Windows)
MySQL Server (需启动并配置好连接信息)
Python 3 (可选，仅用于生成性能图表)
现代浏览器
```

### 启动步骤

> 💡 **数据库提示**：项目内置 Auto-Migrate 机制，启动后端前只需确保 MySQL 数据库连通，系统会自动创建表结构，无需手动执行 DDL 脚本。

#### 1️⃣ 编译与启动驱动
```bash
cd driver
gcc -o printer_driver printer_simulator.c driver_server.c main_driver.c state_machine.c protocol.c protocol_handler.c -lpthread
./printer_driver
```

#### 2️⃣ 启动后端
```bash
cd backend
# 确保数据库配置与您的本地 MySQL 一致
go run main.go
```

#### 3️⃣ 打开前端
```bash
open backend/printer_control.html
```

## 📚 文档导航

所有最新设计文档均按照软件工程规范整理于 `docs/` 目录：

| 文档 | 描述 | 链接 |
|------|------|------|
| 📐 架构设计 | 系统完整架构设计与模块划分 | [ARCHITECTURE_DESIGN_DOCUMENT.md](docs/ARCHITECTURE_DESIGN_DOCUMENT.md) |
| 🔌 二进制协议 | Go与C之间的自定义二进制协议全解 | [BINARY_PROTOCOL_COMPLETE.md](docs/BINARY_PROTOCOL_COMPLETE.md) |
| ⚙️ 状态机设计 | 硬件驱动的抽象状态机转移图与设计 | [STATE_MACHINE_DIAGRAM.md](docs/STATE_MACHINE_DIAGRAM.md) |
| 📖 产品说明 | 产品功能体系、用户角色、用例分析 | [PRODUCT_DOCUMENTATION.md](docs/PRODUCT_DOCUMENTATION.md) |

## 🧪 测试体系

系统包含一个庞大的专业测试套件，置于 `tests/` 目录：

1. **Go 自动化白盒测试**：对通信协议、状态验证、伸缩性能的源码级测试（如 `test_6_2_1_protocol.go`）
2. **Bash 驱动可靠性测试**：一键进行大规模自动化系统熔断测试 (`test_6_4_reliability.sh`)
3. **Python 自动图表渲染**：使用 `generate_thesis_charts.py` 生成可视化压力图表到 `test_results/`

## 🔌 API端点速查

```bash
# ... 其他端点保持一致
# 任务
POST   /api/submit                 # 提交任务 (现支持 multipart/form-data 真实PDF文件上传)
POST   /api/cancel                 # 取消任务
POST   /api/pause                  # 暂停任务
POST   /api/resume                 # 恢复任务
```
*注：提交打印任务时，请通过表单选择真实的 PDF 文件提交，API 会解析并存储至 `pdf_storage` 目录下。*

## 📁 项目结构

```text
network_printer_system/
├── backend/                       # Go后端服务
│   ├── main.go                   # 主程序
│   ├── binary_protocol.go        # 二进制协议实现
│   ├── mysql_database.go         # MySQL数据库与ORM
│   ├── pdf_manager.go            # 真实PDF文档处理
│   ├── progress_tracker.go       # 步进级任务跟踪
│   ├── pdf_storage/              # PDF 本地化缓存存放
│   └── progress-display.js       # 前端进度条渲染件
├── driver/                        # C驱动程序 (状态机模式)
│   ├── state_machine.c/.h        # 打印机核心状态机
│   ├── protocol.c/.h             # 二进制解包/封包协议
│   ├── driver_server.c/.h        # 服务连接池管理
│   └── main_driver.c             # 驱动入口
├── docs/                          # 最新设计文档
│   ├── ARCHITECTURE_DESIGN_DOCUMENT.md
│   ├── BINARY_PROTOCOL_COMPLETE.md
│   └── ...
├── tests/                         # 测试套件与基准数据
│   ├── performance_tests/        # 协议、状态机单元测试
│   ├── test_results/             # 压测结果与图表(CSV/PDF)
│   └── generate_thesis_charts.py # 图表生成脚本
└── ...其他基础脚本与旧版入口
```

## 📝 许可证
本项目采用MIT许可证。详见 [LICENSE](LICENSE) 文件。
