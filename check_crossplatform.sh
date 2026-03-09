#!/bin/bash
# 跨平台检查脚本 - 验证 Windows/Linux 兼容性

echo "================================"
echo "   项目跨平台支持检查"
echo "================================"
echo ""

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_DIR"

echo "[检查] 文件完整性..."
echo ""

# 检查新增文件
NEW_FILES=(
    "driver/platform.h"
    "build.bat"
    "build.ps1"
    "start.bat"
    "WINDOWS_SETUP.md"
    "CROSSPLATFORM_SUMMARY.md"
)

for file in "${NEW_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "  ✅ $file"
    else
        echo "  ❌ $file (缺失)"
    fi
done

echo ""
echo "[检查] 修改文件..."
echo ""

MODIFIED_FILES=(
    "driver/driver_server.c"
    "driver/main_driver.c"
    "README.md"
)

for file in "${MODIFIED_FILES[@]}"; do
    if [ -f "$file" ]; then
        # 检查是否包含平台相关代码
        if grep -q "platform.h\|_WIN32\|SOCKET\|thread_t" "$file" 2>/dev/null; then
            echo "  ✅ $file (已适配跨平台)"
        elif [ "$(basename $file)" = "README.md" ]; then
            if grep -q "Windows\|WINDOWS_SETUP" "$file"; then
                echo "  ✅ $file (已更新文档)"
            else
                echo "  ⚠️ $file (未完全更新)"
            fi
        else
            echo "  ⚠️ $file (未应用修改)"
        fi
    else
        echo "  ❌ $file (缺失)"
    fi
done

echo ""
echo "[检查] 原有文件状态（应保持不变）..."
echo ""

ORIGINAL_FILES=(
    "backend/main.go"
    "driver/printer_simulator.c"
    "driver/printer_simulator.h"
    "printer_control.html"
    "printer_control_improved.html"
    "build.sh"
    "quick_start.sh"
)

for file in "${ORIGINAL_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "  ✅ $file"
    else
        echo "  ❌ $file (缺失)"
    fi
done

echo ""
echo "[检查] 代码质量..."
echo ""

# 检查 platform.h 是否完整
if grep -q "#ifdef _WIN32" driver/platform.h && grep -q "#else" driver/platform.h; then
    echo "  ✅ platform.h 条件编译正确"
else
    echo "  ❌ platform.h 条件编译不完整"
fi

# 检查 driver_server.c 是否包含 platform.h
if grep -q "#include \"platform.h\"" driver/driver_server.c; then
    echo "  ✅ driver_server.c 已包含 platform.h"
else
    echo "  ❌ driver_server.c 未包含 platform.h"
fi

# 检查 main_driver.c 是否包含 platform.h
if grep -q "#include \"platform.h\"" driver/main_driver.c; then
    echo "  ✅ main_driver.c 已包含 platform.h"
else
    echo "  ❌ main_driver.c 未包含 platform.h"
fi

echo ""
echo "[检查] 编译脚本..."
echo ""

# 检查 build.bat 是否存在且有内容
if [ -f "build.bat" ] && [ -s "build.bat" ]; then
    LINES=$(wc -l < build.bat)
    echo "  ✅ build.bat ($LINES 行)"
else
    echo "  ❌ build.bat (缺失或为空)"
fi

# 检查 build.ps1 是否存在且有内容
if [ -f "build.ps1" ] && [ -s "build.ps1" ]; then
    LINES=$(wc -l < build.ps1)
    echo "  ✅ build.ps1 ($LINES 行)"
else
    echo "  ❌ build.ps1 (缺失或为空)"
fi

# 检查 start.bat 是否存在
if [ -f "start.bat" ] && [ -s "start.bat" ]; then
    echo "  ✅ start.bat (启动脚本)"
else
    echo "  ❌ start.bat (缺失或为空)"
fi

echo ""
echo "[检查] 文档..."
echo ""

if [ -f "WINDOWS_SETUP.md" ]; then
    LINES=$(wc -l < WINDOWS_SETUP.md)
    echo "  ✅ WINDOWS_SETUP.md ($LINES 行)"
else
    echo "  ❌ WINDOWS_SETUP.md (缺失)"
fi

if [ -f "CROSSPLATFORM_SUMMARY.md" ]; then
    LINES=$(wc -l < CROSSPLATFORM_SUMMARY.md)
    echo "  ✅ CROSSPLATFORM_SUMMARY.md ($LINES 行)"
else
    echo "  ❌ CROSSPLATFORM_SUMMARY.md (缺失)"
fi

echo ""
echo "================================"
echo "   检查完成！"
echo "================================"
echo ""
echo "下一步："
echo "  1. Linux 用户：bash build.sh all"
echo "  2. Windows 用户：build.bat all"
echo ""
