#!/bin/bash
# run_state_machine_test.sh

# 切换到测试脚本所在目录
cd "$(dirname "$0")"

echo "编译 State Machine 测试..."
gcc -o test_sm test_state_machine.c ../driver/state_machine.c -I../driver/

if [ $? -eq 0 ]; then
    echo "编译成功，开始执行测试:"
    echo "--------------------------"
    ./test_sm
    echo "--------------------------"
    # 清理产生的文件
    rm test_sm
else
    echo "编译失败!"
fi
