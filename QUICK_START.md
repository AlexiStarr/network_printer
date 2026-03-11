快速开始指南 (Quick Start Guide)
================================================================================

项目: 网络打印机系统 (Network Printer System)
版本: v2.0 (Binary Protocol + MySQL)
状态: ✓ 已完全修复，可立即使用

================================================================================
前置依赖
================================================================================

【Windows 系统】
┌─ 必需软件
│ 1. MinGW (GCC 编译器)
│    下载: https://www.mingw-w64.org/
│    或: winget install mingw
│
│ 2. Go 编译器 (1.21+)
│    下载: https://go.dev/dl
│
│ 3. MySQL Server (可选，带 SQLite 备用)
│    下载: https://dev.mysql.com/downloads/mysql/
└─

【Linux/Mac 系统】
┌─ 安装命令
│ Ubuntu/Debian:
│   sudo apt-get update
│   sudo apt-get install build-essential gcc curl jq lsof netcat-openbsd
│   brew install golang mysql
│
│ macOS (使用 Homebrew):
│   brew install gcc golang mysql curl jq netcat
│
│ CentOS/RHEL:
│   sudo yum groupinstall "Development Tools"
│   sudo yum install mysql-server curl jq lsof nc
└─

================================================================================
编译步骤
================================================================================

【Windows 用户】
┌─ 编译指令
│ 1. 打开 PowerShell 或 CMD
│ 2. 进入项目目录:
│    cd D:\code\network_printer_system
│
│ 3. 编译全部:
│    build.bat all
│
│ 或仅编译驱动:
│    build.bat driver
│
│ 或仅编译后端:
│    build.bat backend
│
│ 清理编译文件:
│    build.bat clean
└─

【Linux/Mac 用户】
┌─ 编译指令
│ 1. 打开终端
│ 2. 进入项目目录:
│    cd ~/network_printer_system
│
│ 3. 编译全部:
│    bash build.sh all
│
│ 或仅编译驱动:
│    bash build.sh driver
│
│ 或仅编译后端:
│    bash build.sh backend
│
│ 清理编译文件:
│    bash build.sh clean
└─

【编译结果】
┌─ 成功标志
│ Windows:
│   ✓ driver\printer_driver.exe (已创建)
│   ✓ backend\printer_backend.exe (已创建)
│
│ Linux/Mac:
│   ✓ driver/printer_driver (已创建)
│   ✓ backend/printer_backend (已创建)
└─

================================================================================
启动服务
================================================================================

【Windows 用户】
┌─ 启动方法 (自动)
│ 1. 运行启动脚本:
│    start.bat
│
│ 说明:
│   • 自动检查所有依赖
│   • 自动启动驱动程序 (端口 9999)
│   • 自动启动后端服务 (端口 8080)
│   • 自动打开浏览器
│
│ 预期输出:
│   [OK] ✓ 所有编译文件都已找到
│   [OK] ✓ 端口检查完成
│   [3/5] 启动驱动程序...
│   [4/5] 启动后端服务...
│   [5/5] 检查后端可用性...
│   ================================
│     ✓ 系统启动完成！
│   ================================
└─

【Linux/Mac 用户】
┌─ 启动方法 (自动)
│ 1. 运行启动脚本:
│    bash start.sh
│
│ 说明:
│   • 自动检查所有依赖
│   • 清理旧的驱动/后端进程
│   • 启动驱动程序 (后台运行)
│   • 启动后端服务 (后台运行)
│   • 自动打开浏览器
│   • 保持脚本运行以便查看日志
│
│ 按 Ctrl+C 停止脚本（但服务继续运行）
│
│ 日志位置:
│   /tmp/printer_system/driver.log
│   /tmp/printer_system/backend.log
└─

【手动启动 (高级用户)】
┌─ 如果需要手动控制
│ 终端 1 - 启动驱动:
│   Windows: .\driver\printer_driver.exe
│   Linux/Mac: ./driver/printer_driver
│
│ 终端 2 - 启动后端:
│   Windows: .\backend\printer_backend.exe
│   Linux/Mac: ./backend/printer_backend
│
│ 终端 3 - 交互式控制 (Linux/Mac):
│   bash printer_os.sh
└─

================================================================================
访问 Web 界面
================================================================================

【Web 地址】
┌─ 主页面
│ 地址: http://localhost:8080
│ 说明: 实时打印机监控和控制
│
│ API 文档: http://localhost:8080/api/docs (如果启用)
└─

【默认账户】
┌─ 演示账号 (在 printer_os.sh 中显示)
│ 管理员账户:
│   用户名: admin
│   密码: admin123
│
│ 普通用户:
│   用户名: user
│   密码: user123
│
│ 技术员账户:
│   用户名: technician
│   密码: tech123
└─

【功能】
┌─ 可用操作
│ ✓ 实时监控打印机状态
│ ✓ 查看纸张和碳粉用量
│ ✓ 提交打印任务
│ ✓ 查看待打印队列
│ ✓ 暂停/恢复/取消任务
│ ✓ 补充耗材
│ ✓ 查看打印历史
│ ✓ 管理员功能:
│   - 用户管理
│   - 系统统计
│   - 日志查看
│   - 故障模拟与清除
└─

================================================================================
交互式命令行界面 (Linux/Mac)
================================================================================

【启动方法】
┌─ 运行交互式管理系统
│ bash printer_os.sh
│
│ 说明:
│   • 自动启动所有后台服务
│   • 显示漂亮的命令行界面
│   • 支持命令行操作而无需浏览器
│   • 功能完整的用户认证
│   • 实时仪表板
└─

【主要功能】
┌─ 菜单选项
│ [1] 📊 实时仪表板
│     • 打印机状态指示灯
│     • 纸张和碳粉条形图
│     • 实时任务队列
│
│ [2] 📋 任务管理
│     • 查看队列
│     • 查看历史
│     • 提交新任务
│     • 暂停/恢复/取消任务
│
│ [3] 🛠  耗材管理
│     • 补充纸张
│     • 补充碳粉
│
│ [4] 👨‍💼 管理员控制面板 (仅 admin 用户)
│     • 用户管理
│     • 系统统计
│     • 日志查看
│     • 故障模拟
│
│ [9] 🔑 用户登录/登出
│ [0] 💤 退出系统
└─

================================================================================
故障排查
================================================================================

【问题】GCC 编译器未找到
┌─ 解决方案
│ Windows:
│   1. 下载 MinGW: https://www.mingw-w64.org/
│   2. 安装到 C:\MinGW
│   3. 添加到 PATH: C:\MinGW\bin
│   4. 重启终端，重试编译
│
│ Linux:
│   sudo apt-get install build-essential
│
│ macOS:
│   brew install gcc
└─

【问题】Go 编译器版本太低
┌─ 解决方案
│ 检查版本:
│   go version
│
│ 需要: Go 1.21+
│
│ 升级:
│   1. 卸载旧版本: https://go.dev/dl
│   2. 运行新版本安装程序
│   3. 验证: go version
└─

【问题】端口 8080 或 9999 已被占用
┌─ 解决方案
│ Windows (PowerShell):
│   netstat -ano | findstr :8080
│   taskkill /PID <PID> /F
│
│ Linux/Mac:
│   lsof -i :8080
│   kill <PID>
│
│ 或修改代码使用不同端口
└─

【问题】MySQL 连接失败
┌─ 解决方案
│ 1. 检查 MySQL 是否运行
│ 2. 验证数据库是否创建
│ 3. 检查 main.go 中的连接字符串:
│    "root:password@tcp(localhost:3306)/printer_db"
│ 4. 修改用户名、密码、数据库名
│ 5. 重新编译后端
└─

【问题】编译时出现 "could not import" 错误
┌─ 解决方案
│ 运行:
│   cd backend
│   go mod download
│   go mod tidy
│   go build -o printer_backend main.go
└─

【问题】驱动程序启动后立即关闭
┌─ 解决方案
│ 1. 检查日志文件
│    Windows: 检查控制台输出
│    Linux: tail -f /tmp/printer_system/driver.log
│
│ 2. 常见原因:
│    • 端口 9999 已被占用
│    • 缺少依赖库
│    • 编译错误
│
│ 3. 试试手动运行查看错误:
│    ./driver/printer_driver
└─

【问题】后端服务无响应
┌─ 解决方案
│ 1. 检查是否启动:
│    netstat -an | grep 8080
│
│ 2. 检查日志:
│    Linux: tail -f /tmp/printer_system/backend.log
│
│ 3. 检查 MySQL 连接
│
│ 4. 手动启动查看错误:
│    ./backend/printer_backend
└─

================================================================================
性能优化建议
================================================================================

【Windows 优化】
┌─ 编译优化
│ 已使用: -O2 (优化级别 2)
│ 推荐: 保持默认
│
│ 运行时:
│   • 关闭不必要的后台程序
│   • 确保足够的 RAM (推荐 4GB+)
│   • 固态硬盘推荐
└─

【Linux/Mac 优化】
┌─ 系统调优
│ 文件描述符限制:
│   ulimit -n 65536
│
│ TCP 连接优化:
│   sysctl -w net.ipv4.tcp_tw_reuse=1
│
│ MySQL 性能:
│   • 增加 max_connections
│   • 启用查询缓存
│   • 增加 innodb_pool_size
└─

================================================================================
部署建议
================================================================================

【开发环境】
└─ 使用 start.bat 或 start.sh 快速启动

【测试环境】
└─ 1. 使用 Docker 容器化
│ 2. 创建 systemd 服务
│ 3. 配置日志轮转

【生产环境】
├─ 1. 使用 systemd 服务管理
│ 2. Nginx 反向代理
│ 3. Let's Encrypt SSL 证书
│ 4. 数据库备份策略
│ 5. 监控和告警系统
└─

================================================================================
常用命令速查表
================================================================================

编译相关:
  build.bat all              # Windows 编译全部
  build.bat driver           # Windows 仅编译驱动
  build.bat clean            # Windows 清理编译
  bash build.sh all          # Linux 编译全部
  bash build.sh clean        # Linux 清理编译

启动相关:
  start.bat                  # Windows 自动启动
  bash start.sh              # Linux/Mac 自动启动
  bash printer_os.sh         # Linux/Mac 交互式界面

访问:
  http://localhost:8080      # Web 界面
  http://localhost:9999      # 驱动 API (内部)

日志 (Linux/Mac):
  tail -f /tmp/printer_system/driver.log
  tail -f /tmp/printer_system/backend.log

================================================================================
更多信息
================================================================================

文档:
  • BUG_FIXES_COMPLETE.md      - 修复详情
  • PROJECT_DOCUMENTATION.md   - 系统设计
  • API_DOCUMENTATION.md       - API 参考
  • README.md                  - 项目概览

支持:
  • 查看 CHANGES_SUMMARY.md 了解最近的改进
  • 查看 CGO_MIGRATION.md 了解 CGO 迁移
  • 查看 INTEGRATION_GUIDE_FINAL.md 了解集成

================================================================================
祝你好运！🚀
================================================================================

系统已完全修复，所有脚本已更新。现在可以立即编译和运行。

有任何问题，请查阅相应的文档或日志文件。

Happy Printing! 🖨️
