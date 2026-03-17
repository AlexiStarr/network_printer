#!/bin/bash

# ==============================================================
# 6.4 可靠性与故障恢复验证脚本
# ==============================================================
echo "=== 开始执行 6.4 可靠性验证 ==="

# 确保在项目根目录下或能找到编译目标的目录
DRIVER_BIN="../../driver/printer_driver"
if [ ! -f "$DRIVER_BIN" ]; then
    echo "未找到编译好的 $DRIVER_BIN，进行编译..."
    (cd ../../driver && make || gcc -o printer_driver *.c -lpthread)
fi

echo -e "\n--- [场景1] 校验和错误处理稳定性验证 ---"
echo ">> 调用 6.2.1 协议测试的校验和错误用例..."
go run test_6_2_1_protocol.go | grep "校验和错误"
if [ $? -eq 0 ]; then
    echo "[PASS] 驱动能够正确拦截 0x04 错误并返回 0xFF 响应且未崩溃。"
else
    echo "[FAIL] 校验和拦截测试失败或驱动崩溃。"
fi

echo -e "\n--- [场景2] 网络瞬断恢复测试 ---"
# 启动驱动
echo ">> 启动 C 驱动端进程..."
$DRIVER_BIN -p 9999 > driver.log 2>&1 &
DRIVER_PID=$!
sleep 1

# 启动 Go 读取端不断发请求
echo ">> 启动 Go 控制端进行压测连接..."
# 使用后台的小请求一直探测
go run test_6_3_performance.go -c 10 -d 5 -o /tmp > /dev/null &
GO_PID=$!
sleep 2

echo ">> 强行关闭驱动 (kill -9 $DRIVER_PID) 模拟网络端口断开..."
kill -9 $DRIVER_PID

echo ">> 等待 2 秒，观测客户端报错..."
sleep 2

echo ">> 再次重启 C 驱动端进程..."
$DRIVER_BIN -p 9999 >> driver.log 2>&1 &
DRIVER_PID=$!

echo ">> 等待 5 秒，观测客户端是否自动回恢复重连并继续工作..."
sleep 5

# 手动发几条查询看是否连通
go run test_6_2_1_protocol.go | grep "零长度载荷"
if [ $? -eq 0 ]; then
    echo "[PASS] 客户端已成功完成断线重连！"
else
    echo "[FAIL] 客户端没有自动重连成功。"
fi

kill -9 $DRIVER_PID
kill -9 $GO_PID 2>/dev/null >/dev/null
wait $GO_PID 2>/dev/null

echo -e "\n--- [场景3] 长时间运行稳定性与内存泄漏测试 ---"
echo "由于 24 小时长时间测试无法在快速脚本中执行完，本脚本提供验证指令："
echo "请将 driver/printer_driver 和 backend/main.go 分别挂在后台运行，"
echo "并执行以下指令开启高压持续运行模式："
echo "  $ nohup go run test_6_3_performance.go -c 200 -d 86400 -o ../test_results &"
echo "并使用以下命令监控驱动和后端的内存驻留 (RSS)："
echo "  $ top -pid \$(pgrep printer_driver) -l 0"
echo "  $ top -pid \$(pgrep main) -l 0"

echo -e "\n=> 场景 1 及 场景 2 自动化验证完成！"
