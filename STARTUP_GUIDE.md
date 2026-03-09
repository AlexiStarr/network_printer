# 🚀 打印机控制系统 - 快速开始指南

## ✅ 系统已就绪！

### 📍 访问方式

**🌐 前端页面（本地打开）：**
```
/Users/liuxingyu/Documents/codeRepository/network_printer_system/printer_control_improved.html
```

或者直接在浏览器中打开文件：在Finder中找到此文件，双击打开。

### 🔐 默认登录凭证

| 用户名 | 密码 | 角色 |
|--------|------|------|
| admin | admin123 | 管理员 |
| user | user123 | 普通用户 |
| technician | tech123 | 技术员 |

### 📱 服务地址

- **后端API**: http://localhost:8080
- **WebSocket**: ws://localhost:8080/ws

### 🔧 启动/停止服务

**一键启动所有服务：**
```bash
bash /Users/liuxingyu/Documents/codeRepository/network_printer_system/start_system.sh
```

**停止所有服务：**
```bash
pkill -f "printer_driver|printer_backend"
```

**查看日志：**
```bash
# 查看驱动日志
tail -f /tmp/printer_driver.log

# 查看后端日志
tail -f /tmp/printer_backend.log
```

---

## 🎯 功能说明

### 📊 仪表板
- 实时查看打印机状态
- 监控纸张和碳粉用量
- 查看队列任务数

### 📄 提交任务
- 提交新的打印任务
- 设置优先级（低/中/高）
- 指定页数

### ⏳ 任务管理
- 查看打印队列
- 暂停/恢复打印任务
- 取消任务

### 📋 打印历史
- 查看历史打印记录
- 追踪打印进度

### 🔧 维护
- 补充纸张
- 补充碳粉
- 清除硬件错误
- **管理员功能**：模拟故障进行测试

### 👥 用户管理（仅管理员）
- 添加新用户
- 管理用户角色
- 删除用户

### 🔍 诊断（仅管理员）
- 查看系统诊断信息
- 调试打印机状态

---

## 📝 常见问题

### Q: 登录时出现"load failed"
**A:** 请确保后端服务正常运行。执行：
```bash
bash /Users/liuxingyu/Documents/codeRepository/network_printer_system/start_system.sh
```

### Q: 前端无法连接到后端
**A:** 确保：
1. 后端运行在 localhost:8080
2. 驱动运行在 localhost:9999
3. 检查防火墙设置

### Q: 页面样式显示不对
**A:** 清除浏览器缓存（Ctrl+Shift+Delete 或 Cmd+Shift+Delete）

### Q: 无法修改用户
**A:** 确保使用管理员账户登录，在"用户管理"页面操作

---

## 🐛 系统已修复的Bug

✅ Token过期自动清理  
✅ WebSocket消息处理完善  
✅ 队列排序性能优化 (O(n²) → O(n log n))  
✅ C驱动内存泄漏修复  
✅ 任务ID同步改进  
✅ 用户输入验证加强  

详见：[BUG_FIXES.md](BUG_FIXES.md)

---

## 📚 更多文档

- [完整功能说明](FEATURES_SUMMARY.md)
- [API文档](docs/API_DOCUMENTATION.md)
- [API使用示例](API_EXAMPLES.md)
- [测试场景](QUICK_START_TESTING.md)
- [项目文档](PROJECT_DOCUMENTATION.md)

---

## ✨ 系统信息

- **版本**: v1.0
- **后端**: Go + SQLite
- **驱动**: C + POSIX Thread
- **前端**: HTML5 + CSS3 + JavaScript
- **状态**: ✅ 生产环境就绪

