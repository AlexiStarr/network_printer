#!/bin/bash

################################################################################
#                                                                              #
#              网络打印机操作系统 (Network Printer OS)                         #
#                   Interactive Management Console v1.0                        #
#                                                                              #
#  一个优雅的、功能完整的打印机管理系统交互界面                                 #
#                                                                              #
################################################################################

set -o pipefail

# ============================================================================
# 配置与常量定义
# ============================================================================

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$SCRIPT_DIR"
readonly BACKEND_BINARY="$PROJECT_ROOT/backend/printer_backend"
readonly DRIVER_BINARY="$PROJECT_ROOT/driver/printer_driver"
readonly LOG_DIR="/tmp/printer_system"
readonly BACKEND_LOG="$LOG_DIR/backend.log"
readonly DRIVER_LOG="$LOG_DIR/driver.log"

# API 端点
readonly API_BASE_URL="http://localhost:8080"
readonly DRIVER_PORT=9999
readonly BACKEND_PORT=8080

# 创建日志目录
mkdir -p "$LOG_DIR" 2>/dev/null

# ============================================================================
# 颜色定义与UI美化
# ============================================================================

# 颜色代码
readonly BLACK='\033[0;30m'
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly MAGENTA='\033[0;35m'
readonly CYAN='\033[0;36m'
readonly WHITE='\033[1;37m'
readonly BOLD='\033[1m'
readonly UNDERLINE='\033[4m'
readonly DIM='\033[2m'
readonly RESET='\033[0m'

# 特殊符号
readonly SUCCESS="✓"
readonly ERROR="✗"
readonly INFO="ℹ"
readonly WARN="⚠"
readonly ARROW="→"
readonly BULLET="•"

# ============================================================================
# 全局变量
# ============================================================================

CURRENT_USER=""
CURRENT_TOKEN=""
CURRENT_ROLE=""
SESSION_START_TIME=""
AUTO_REFRESH=true
REFRESH_INTERVAL=5

# 缓存数据
CACHED_STATUS=""
CACHED_QUEUE=""
CACHED_HISTORY=""
LAST_CACHE_TIME=0

# ============================================================================
# 工具函数
# ============================================================================

# 打印错误信息
print_error() {
    echo -e "${RED}${ERROR}${RESET} $*" >&2
}

# 打印成功信息
print_success() {
    echo -e "${GREEN}${SUCCESS}${RESET} $*"
}

# 打印信息
print_info() {
    echo -e "${BLUE}${INFO}${RESET} $*"
}

# 打印警告信息
print_warn() {
    echo -e "${YELLOW}${WARN}${RESET} $*"
}

# 打印标题（带分隔线）
print_title() {
    local title="$1"
    local width=80
    local padding=$(( ($width - ${#title}) / 2 ))
    
    echo -e "${CYAN}${BOLD}"
    printf "%*s\n" "$width" | tr ' ' '═'
    printf "%${padding}s%s\n" "" "$title"
    printf "%*s\n" "$width" | tr ' ' '═'
    echo -e "${RESET}"
}

# 打印子标题
print_subtitle() {
    echo -e "${CYAN}${BOLD}┌─ $1${RESET}"
}

# 打印分隔线
print_separator() {
    echo -e "${DIM}$(printf '%80s' | tr ' ' '─')${RESET}"
}

# 显示加载动画
show_spinner() {
    local pid=$1
    local msg="${2:-加载中}"
    local frames=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
    local i=0
    
    echo -ne "${BLUE}${msg}${RESET} "
    
    while kill -0 "$pid" 2>/dev/null; do
        echo -ne "\b${frames[$((i++ % 10))]}"
        sleep 0.1
    done
    
    wait "$pid"
    local exit_code=$?
    echo -ne "\b"
    
    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}${SUCCESS}${RESET}"
    else
        echo -e "${RED}${ERROR}${RESET}"
    fi
    
    return $exit_code
}

# 暂停等待用户回车
pause_for_user() {
    local msg="${1:-按 Enter 继续...}"
    echo -ne "\n${DIM}$msg${RESET}"
    read -r
}

# 获取用户输入（带验证）
get_input() {
    local prompt="$1"
    local default="${2:-}"
    local input
    
    # 将提示信息输出到标准错误流，确保立即显示
    if [ -z "$default" ]; then
        echo -ne "${CYAN}${ARROW}${RESET} ${BOLD}$prompt${RESET}: " >&2
    else
        echo -ne "${CYAN}${ARROW}${RESET} ${BOLD}$prompt${RESET} [${YELLOW}$default${RESET}]: " >&2
    fi
    
    read -r input
    
    if [ -z "$input" ] && [ -n "$default" ]; then
        echo "$default"
    else
        echo "$input"
    fi
}

# 获取确认 (Y/N)
confirm() {
    local prompt="${1:-确认？}"
    local response
    
    echo -ne "${YELLOW}${WARN}${RESET} $prompt [${BOLD}y/N${RESET}]: "
    read -r -n 1 response
    echo
    
    [[ "$response" =~ ^[Yy]$ ]]
}

# JSON 解析辅助函数
json_get() {
    local json="$1"
    local key="$2"
    echo "$json" | grep -o "\"$key\":\"[^\"]*\"" | cut -d'"' -f4 || echo "$json" | grep -o "\"$key\":[0-9]*" | cut -d':' -f2
}

# ============================================================================
# 系统管理函数
# ============================================================================

# 检查依赖项
check_dependencies() {
    local deps=("curl" "jq" "lsof" "nc")
    local missing=()
    
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            missing+=("$dep")
        fi
    done
    
    if [ ${#missing[@]} -gt 0 ]; then
        print_error "缺少依赖项: ${missing[*]}"
        print_info "请使用以下命令安装: brew install ${missing[*]}"
        return 1
    fi
    
    return 0
}

# 检查服务是否运行
check_service() {
    local port=$1
    local name=$2
    
    if lsof -Pi ":$port" -sTCP:LISTEN -t >/dev/null 2>&1; then
        print_success "$name 运行中"
        return 0
    else
        print_error "$name 未运行"
        return 1
    fi
}

# 启动驱动程序
start_driver() {
    if check_service $DRIVER_PORT "驱动程序"; then
        return 0
    fi
    
    if [ ! -f "$DRIVER_BINARY" ]; then
        print_error "驱动程序不存在: $DRIVER_BINARY"
        print_info "请先编译: cd $PROJECT_ROOT && bash build.sh driver"
        return 1
    fi
    
    print_info "启动驱动程序..."
    
    (
        cd "$PROJECT_ROOT/driver"
        ./printer_driver > "$DRIVER_LOG" 2>&1
    ) &
    
    local pid=$!
    sleep 2
    
    if kill -0 "$pid" 2>/dev/null; then
        print_success "驱动程序已启动 (PID: $pid)"
        return 0
    else
        print_error "启动驱动程序失败"
        return 1
    fi
}

# 启动后端服务
start_backend() {
    if check_service $BACKEND_PORT "后端服务"; then
        return 0
    fi
    
    if [ ! -f "$BACKEND_BINARY" ]; then
        print_error "后端程序不存在: $BACKEND_BINARY"
        print_info "请先编译: cd $PROJECT_ROOT && bash build.sh backend"
        return 1
    fi
    
    print_info "启动后端服务..."
    
    (
        cd "$PROJECT_ROOT/backend"
        ./printer_backend > "$BACKEND_LOG" 2>&1
    ) &
    
    local pid=$!
    sleep 2
    
    if check_service $BACKEND_PORT "后端服务"; then
        print_success "后端服务已启动 (PID: $pid)"
        return 0
    else
        print_error "启动后端服务失败"
        return 1
    fi
}

# 启动所有服务
start_all_services() {
    print_title "系统启动"
    
    start_driver || return 1
    start_backend || return 1
    
    print_success "所有服务已启动"
    sleep 1
}

# ============================================================================
# 登录与身份认证
# ============================================================================

# 用户登录
user_login() {
    print_title "用户登录"
    
    print_info "演示账号："
    echo -e "  • ${BOLD}admin${RESET} / admin123 (${YELLOW}管理员${RESET})"
    echo -e "  • ${BOLD}user${RESET} / user123 (${YELLOW}普通用户${RESET})"
    echo -e "  • ${BOLD}technician${RESET} / tech123 (${YELLOW}技术员${RESET})"
    
    print_separator
    
    local username password
    username=$(get_input "用户名")
    
    # 隐藏密码输入
    echo -ne "${CYAN}${ARROW}${RESET} ${BOLD}密码${RESET}: "
    read -rs password
    echo
    
    print_info "认证中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\": \"$username\", \"password\": \"$password\"}" 2>/dev/null)
    
    # 调试输出
    if [ -z "$response" ]; then
        print_error "登录失败：无法连接到服务器"
        sleep 2
        return 1
    fi
    
    # 使用更健壮的json解析
    local token=""
    local role=""
    
    # 尝试用jq解析
    if command -v jq &> /dev/null; then
        token=$(echo "$response" | jq -r '.token // empty' 2>/dev/null)
        role=$(echo "$response" | jq -r '.role // empty' 2>/dev/null)
    fi
    
    # 如果jq失败，用grep/sed
    if [ -z "$token" ]; then
        token=$(echo "$response" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
        role=$(echo "$response" | grep -o '"role":"[^"]*"' | cut -d'"' -f4)
    fi
    
    if [ -n "$token" ] && [ "$token" != "null" ]; then
        CURRENT_USER="$username"
        CURRENT_TOKEN="$token"
        CURRENT_ROLE="$role"
        SESSION_START_TIME=$(date "+%Y-%m-%d %H:%M:%S")
        
        print_success "登录成功！欢迎 ${BOLD}$username${RESET} ($role)"
        sleep 1
        return 0
    else
        print_error "登录失败：用户名或密码错误"
        # 调试信息
        echo "响应: $response" >&2
        sleep 2
        return 1
    fi
}

# 用户登出
user_logout() {
    if [ -z "$CURRENT_TOKEN" ]; then
        print_warn "未登录任何用户"
        return 1
    fi
    
    print_info "正在登出 $CURRENT_USER..."
    
    curl -s -X POST "$API_BASE_URL/api/auth/logout" \
        -H "Authorization: Bearer $CURRENT_TOKEN" >/dev/null 2>&1
    
    CURRENT_USER=""
    CURRENT_TOKEN=""
    CURRENT_ROLE=""
    SESSION_START_TIME=""
    
    print_success "已安全登出"
    sleep 1
}

# ============================================================================
# 打印机状态获取
# ============================================================================

# 获取打印机状态
get_printer_status() {
    if [ -z "$CURRENT_TOKEN" ]; then
        return 1
    fi
    
    curl -s "$API_BASE_URL/api/status" \
        -H "Authorization: Bearer $CURRENT_TOKEN" 2>/dev/null
}

# 获取任务队列
get_print_queue() {
    if [ -z "$CURRENT_TOKEN" ]; then
        return 1
    fi
    
    curl -s "$API_BASE_URL/api/queue" \
        -H "Authorization: Bearer $CURRENT_TOKEN" 2>/dev/null
}

# 获取打印历史
get_print_history() {
    if [ -z "$CURRENT_TOKEN" ]; then
        return 1
    fi
    
    curl -s "$API_BASE_URL/api/history" \
        -H "Authorization: Bearer $CURRENT_TOKEN" 2>/dev/null
}

# ============================================================================
# 仪表板与实时监控
# ============================================================================

# 显示打印机仪表板
show_dashboard() {
    while true; do
        clear
        
        # 标题栏
        echo -e "${CYAN}${BOLD}"
        cat << 'EOF'
╔════════════════════════════════════════════════════════════════════════════╗
║                     打印机操作系统 - 实时监控仪表板                          ║
╚════════════════════════════════════════════════════════════════════════════╝
EOF
        echo -e "${RESET}"
        
        # 用户信息栏
        echo -e "${MAGENTA}┌─ 用户会话${RESET}"
        echo -e "│ 用户名: ${BOLD}$CURRENT_USER${RESET} | 角色: ${YELLOW}$CURRENT_ROLE${RESET} | 登录时间: ${DIM}$SESSION_START_TIME${RESET}"
        echo -e "│ Token: ${DIM}${CURRENT_TOKEN:0:20}...${RESET}"
        echo -e "${MAGENTA}└─────────────────────────────────────────────────────────────────────────────${RESET}\n"
        
        # 获取状态
        local status=$(get_printer_status)
        
        if [ -z "$status" ]; then
            print_error "无法获取打印机状态"
            pause_for_user
            return 1
        fi
        
        # 解析状态
        local printer_status=$(echo "$status" | jq -r '.status // "unknown"' 2>/dev/null)
        local paper=$(echo "$status" | jq -r '.paper_pages // 0' 2>/dev/null)
        local toner=$(echo "$status" | jq -r '.toner_percentage // 0' 2>/dev/null)
        local error=$(echo "$status" | jq -r '.error // "无"' 2>/dev/null)
        local page_count=$(echo "$status" | jq -r '.page_count // 0' 2>/dev/null)
        
        # 状态指示灯
        echo -e "${CYAN}┌─ 打印机状态${RESET}"
        
        case "$printer_status" in
            "idle")
                echo -e "│ 状态: ${GREEN}● 就绪 (IDLE)${RESET}"
                ;;
            "printing")
                echo -e "│ 状态: ${YELLOW}● 打印中 (PRINTING)${RESET}"
                ;;
            "error")
                echo -e "│ 状态: ${RED}● 错误 (ERROR)${RESET}"
                ;;
            *)
                echo -e "│ 状态: ${DIM}○ 离线 (OFFLINE)${RESET}"
                ;;
        esac
        
        echo -e "│ 页数统计: ${BOLD}$page_count${RESET} 页"
        echo -e "${CYAN}└─────────────────────────────────────────────────────────────────────────────${RESET}\n"
        
        # 耗材信息
        echo -e "${CYAN}┌─ 耗材状态${RESET}"
        
        # 纸张条形图
        local paper_bar=""
        local paper_color="$GREEN"
        if [ "$paper" -lt 50 ]; then
            paper_color="$RED"
        elif [ "$paper" -lt 200 ]; then
            paper_color="$YELLOW"
        fi
        
        for ((i=0; i<20; i++)); do
            if [ $((i * 50)) -lt "$paper" ]; then
                paper_bar="${paper_bar}█"
            else
                paper_bar="${paper_bar}░"
            fi
        done
        
        echo -e "│ 📄 纸张: ${paper_color}$paper 张${RESET}"
        echo -e "│   [$paper_bar] $((paper / 50))%"
        
        # 碳粉条形图
        local toner_bar=""
        local toner_color="$GREEN"
        if [ "$toner" -lt 20 ]; then
            toner_color="$RED"
        elif [ "$toner" -lt 50 ]; then
            toner_color="$YELLOW"
        fi
        
        for ((i=0; i<20; i++)); do
            if [ $((i * 5)) -lt "$toner" ]; then
                toner_bar="${toner_bar}█"
            else
                toner_bar="${toner_bar}░"
            fi
        done
        
        echo -e "│ 🖨  碳粉: ${toner_color}$toner %${RESET}"
        echo -e "│   [$toner_bar] $toner%"
        
        if [ "$error" != "无" ]; then
            echo -e "│ ${RED}⚠  错误: $error${RESET}"
        fi
        
        echo -e "${CYAN}└─────────────────────────────────────────────────────────────────────────────${RESET}\n"
        
        # 任务队列
        local queue=$(get_print_queue)
        local queue_count=$(echo "$queue" | jq '.queue | length // 0' 2>/dev/null)
        
        echo -e "${CYAN}┌─ 待打印队列 ($queue_count 个任务)${RESET}"
        
        if [ "$queue_count" -eq 0 ]; then
            echo -e "│ 无待打印任务"
        else
            local count=0
            echo "$queue" | jq -r '.queue[] | "\(.task_id) - \(.filename) (\(.pages) 页) - \(.status)"' 2>/dev/null | while read -r line; do
                count=$((count + 1))
                if [ $count -le 5 ]; then
                    echo -e "│ $BULLET $line"
                fi
            done
            
            if [ "$queue_count" -gt 5 ]; then
                echo -e "│ ... 还有 $((queue_count - 5)) 个任务"
            fi
        fi
        
        echo -e "${CYAN}└─────────────────────────────────────────────────────────────────────────────${RESET}\n"
        
        # 快捷菜单
        echo -e "${MAGENTA}快捷操作:${RESET} ${DIM}[1]${RESET} 提交任务  ${DIM}[2]${RESET} 耗材管理  ${DIM}[3]${RESET} 返回菜单  ${DIM}[q]${RESET} 退出"
        echo -ne "${CYAN}${ARROW}${RESET} 选择操作: "
        
        read -r -t 10 choice || choice="auto"
        
        case "$choice" in
            1) show_submit_job_dialog ;;
            2) show_supplies_menu ;;
            3) return 0 ;;
            q) return 2 ;;
            *) continue ;;
        esac
    done
}

# ============================================================================
# 打印任务管理
# ============================================================================

# 显示提交任务对话框
show_submit_job_dialog() {
    clear
    print_title "提交新打印任务"
    
    local filename=$(get_input "文档名称" "document.pdf")
    local pages=$(get_input "页数" "10")
    local priority=$(get_input "优先级 (0-100)" "50")
    
    if ! [[ "$pages" =~ ^[0-9]+$ ]]; then
        print_error "页数必须是数字"
        pause_for_user
        return 1
    fi
    
    if ! [[ "$priority" =~ ^[0-9]+$ ]]; then
        print_error "优先级必须是数字"
        pause_for_user
        return 1
    fi
    
    echo -e "\n${CYAN}┌─ 确认信息${RESET}"
    echo -e "│ 文档: ${BOLD}$filename${RESET}"
    echo -e "│ 页数: ${BOLD}$pages${RESET} 页"
    echo -e "│ 优先级: ${BOLD}$priority${RESET}"
    echo -e "${CYAN}└─────────────────────────────────────────────────────────────────────────────${RESET}\n"
    
    if ! confirm "确认提交任务？"; then
        print_warn "已取消"
        pause_for_user
        return 1
    fi
    
    print_info "提交任务中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/job/submit" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $CURRENT_TOKEN" \
        -d "{\"filename\": \"$filename\", \"pages\": $pages, \"priority\": $priority}" 2>/dev/null)
    
    local task_id=$(echo "$response" | jq -r '.task_id // empty' 2>/dev/null)
    
    if [ -n "$task_id" ] && [ "$task_id" != "null" ]; then
        print_success "任务已提交 (任务ID: $task_id)"
    else
        print_error "提交失败"
    fi
    
    pause_for_user
}

# 显示任务管理菜单
show_task_management_menu() {
    while true; do
        clear
        print_title "任务管理"
        
        echo -e "${CYAN}请选择操作:${RESET}\n"
        echo -e "  ${DIM}[1]${RESET} 查看队列"
        echo -e "  ${DIM}[2]${RESET} 查看历史"
        echo -e "  ${DIM}[3]${RESET} 提交任务"
        echo -e "  ${DIM}[4]${RESET} 暂停任务"
        echo -e "  ${DIM}[5]${RESET} 恢复任务"
        echo -e "  ${DIM}[6]${RESET} 取消任务"
        
        if [ "$CURRENT_ROLE" = "admin" ]; then
            echo -e "  ${DIM}[7]${RESET} 模拟错误"
            echo -e "  ${DIM}[8]${RESET} 清除错误"
        fi
        
        echo -e "  ${DIM}[0]${RESET} 返回主菜单"
        echo
        
        read -r choice
        
        case "$choice" in
            1) show_queue_view ;;
            2) show_history_view ;;
            3) show_submit_job_dialog ;;
            4) show_pause_job_dialog ;;
            5) show_resume_job_dialog ;;
            6) show_cancel_job_dialog ;;
            7) [ "$CURRENT_ROLE" = "admin" ] && show_simulate_error_dialog ;;
            8) [ "$CURRENT_ROLE" = "admin" ] && show_clear_error_dialog ;;
            0) return 0 ;;
            *) print_error "无效选择" ;;
        esac
    done
}

# 查看队列
show_queue_view() {
    clear
    print_title "待打印队列"
    
    local queue=$(get_print_queue)
    local count=$(echo "$queue" | jq '.queue | length // 0' 2>/dev/null)
    
    echo -e "${CYAN}共 $count 个任务：${RESET}\n"
    
    if [ "$count" -eq 0 ]; then
        echo "无待打印任务"
    else
        echo -e "  ${BOLD}任务ID${RESET}  ${BOLD}文档名${RESET}        ${BOLD}页数${RESET}  ${BOLD}状态${RESET}"
        echo "$queue" | jq -r '.queue[] | "  \(.task_id | tostring | ascii_downcase)      \(.filename)   \(.pages)   \(.status)"' 2>/dev/null
    fi
    
    pause_for_user
}

# 查看历史
show_history_view() {
    clear
    print_title "打印历史"
    
    local history=$(get_print_history)
    local count=$(echo "$history" | jq '.history | length // 0' 2>/dev/null)
    
    echo -e "${CYAN}共 $count 条记录：${RESET}\n"
    
    if [ "$count" -eq 0 ]; then
        echo "无打印历史"
    else
        echo -e "  ${BOLD}任务ID${RESET}  ${BOLD}文档${RESET}          ${BOLD}页数${RESET}  ${BOLD}状态${RESET}     ${BOLD}时间${RESET}"
        echo "$history" | jq -r '.history[] | "  \(.task_id | tostring | ascii_downcase)      \(.filename)  \(.pages)  \(.status)  \(.completed_at)"' 2>/dev/null | head -10
    fi
    
    pause_for_user
}

# 暂停任务对话框
show_pause_job_dialog() {
    clear
    print_title "暂停任务"
    
    local task_id=$(get_input "任务ID")
    
    if ! [[ "$task_id" =~ ^[0-9]+$ ]]; then
        print_error "任务ID必须是数字"
        pause_for_user
        return 1
    fi
    
    if ! confirm "确认暂停任务 $task_id？"; then
        pause_for_user
        return 1
    fi
    
    print_info "暂停中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/job/pause" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $CURRENT_TOKEN" \
        -d "{\"task_id\": $task_id}" 2>/dev/null)
    
    local success=$(echo "$response" | jq -r '.success // false' 2>/dev/null)
    
    if [ "$success" = "true" ]; then
        print_success "任务已暂停"
    else
        print_error "操作失败"
    fi
    
    pause_for_user
}

# 恢复任务对话框
show_resume_job_dialog() {
    clear
    print_title "恢复任务"
    
    local task_id=$(get_input "任务ID")
    
    if ! [[ "$task_id" =~ ^[0-9]+$ ]]; then
        print_error "任务ID必须是数字"
        pause_for_user
        return 1
    fi
    
    if ! confirm "确认恢复任务 $task_id？"; then
        pause_for_user
        return 1
    fi
    
    print_info "恢复中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/job/resume" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $CURRENT_TOKEN" \
        -d "{\"task_id\": $task_id}" 2>/dev/null)
    
    local success=$(echo "$response" | jq -r '.success // false' 2>/dev/null)
    
    if [ "$success" = "true" ]; then
        print_success "任务已恢复"
    else
        print_error "操作失败"
    fi
    
    pause_for_user
}

# 取消任务对话框
show_cancel_job_dialog() {
    clear
    print_title "取消任务"
    
    local task_id=$(get_input "任务ID")
    
    if ! [[ "$task_id" =~ ^[0-9]+$ ]]; then
        print_error "任务ID必须是数字"
        pause_for_user
        return 1
    fi
    
    if ! confirm "确认取消任务 $task_id？这是不可逆操作。"; then
        pause_for_user
        return 1
    fi
    
    print_info "取消中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/job/cancel" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $CURRENT_TOKEN" \
        -d "{\"task_id\": $task_id}" 2>/dev/null)
    
    local success=$(echo "$response" | jq -r '.success // false' 2>/dev/null)
    
    if [ "$success" = "true" ]; then
        print_success "任务已取消"
    else
        print_error "操作失败"
    fi
    
    pause_for_user
}

# ============================================================================
# 耗材管理
# ============================================================================

# 显示耗材管理菜单
show_supplies_menu() {
    while true; do
        clear
        print_title "耗材管理"
        
        echo -e "${CYAN}请选择操作:${RESET}\n"
        echo -e "  ${DIM}[1]${RESET} 补充纸张"
        echo -e "  ${DIM}[2]${RESET} 补充碳粉"
        echo -e "  ${DIM}[3]${RESET} 返回"
        echo
        
        read -r choice
        
        case "$choice" in
            1) show_refill_paper_dialog ;;
            2) show_refill_toner_dialog ;;
            3) return 0 ;;
            *) print_error "无效选择" ;;
        esac
    done
}

# 补充纸张
show_refill_paper_dialog() {
    clear
    print_title "补充纸张"
    
    local amount=$(get_input "补充张数" "500")
    
    if ! [[ "$amount" =~ ^[0-9]+$ ]]; then
        print_error "数量必须是数字"
        pause_for_user
        return 1
    fi
    
    print_info "补充纸张中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/supplies/refill-paper" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $CURRENT_TOKEN" \
        -d "{\"amount\": $amount}" 2>/dev/null)
    
    local success=$(echo "$response" | jq -r '.success // false' 2>/dev/null)
    
    if [ "$success" = "true" ]; then
        print_success "纸张已补充 (+$amount 张)"
    else
        print_error "操作失败"
    fi
    
    pause_for_user
}

# 补充碳粉
show_refill_toner_dialog() {
    clear
    print_title "补充碳粉"
    
    local amount=$(get_input "补充百分比 (0-100)" "100")
    
    if ! [[ "$amount" =~ ^[0-9]+$ ]] || [ "$amount" -lt 0 ] || [ "$amount" -gt 100 ]; then
        print_error "百分比必须在 0-100 之间"
        pause_for_user
        return 1
    fi
    
    print_info "补充碳粉中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/supplies/refill-toner" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $CURRENT_TOKEN" \
        -d "{\"percentage\": $amount}" 2>/dev/null)
    
    local success=$(echo "$response" | jq -r '.success // false' 2>/dev/null)
    
    if [ "$success" = "true" ]; then
        print_success "碳粉已补充 (+$amount %)"
    else
        print_error "操作失败"
    fi
    
    pause_for_user
}

# ============================================================================
# 管理员功能
# ============================================================================

# 显示管理员菜单
show_admin_menu() {
    if [ "$CURRENT_ROLE" != "admin" ]; then
        print_error "仅管理员可访问"
        pause_for_user
        return 1
    fi
    
    while true; do
        clear
        print_title "管理员控制面板"
        
        echo -e "${CYAN}用户管理:${RESET}"
        echo -e "  ${DIM}[1]${RESET} 添加用户"
        echo -e "  ${DIM}[2]${RESET} 删除用户"
        echo -e "  ${DIM}[3]${RESET} 列出所有用户"
        
        echo -e "\n${CYAN}系统管理:${RESET}"
        echo -e "  ${DIM}[4]${RESET} 查看系统统计"
        echo -e "  ${DIM}[5]${RESET} 查看日志"
        
        echo -e "\n${CYAN}故障管理:${RESET}"
        echo -e "  ${DIM}[6]${RESET} 模拟故障"
        echo -e "  ${DIM}[7]${RESET} 清除故障"
        
        echo -e "\n  ${DIM}[0]${RESET} 返回主菜单"
        echo
        
        read -r choice
        
        case "$choice" in
            1) show_add_user_dialog ;;
            2) show_delete_user_dialog ;;
            3) show_list_users_view ;;
            4) show_system_stats ;;
            5) show_logs ;;
            6) show_simulate_error_dialog ;;
            7) show_clear_error_dialog ;;
            0) return 0 ;;
            *) print_error "无效选择" ;;
        esac
    done
}

# 添加用户对话框
show_add_user_dialog() {
    clear
    print_title "添加新用户"
    
    local username=$(get_input "用户名")
    local password=$(get_input "密码")
    
    echo -e "\n${CYAN}选择用户角色:${RESET}"
    echo -e "  ${DIM}[1]${RESET} 管理员 (admin)"
    echo -e "  ${DIM}[2]${RESET} 普通用户 (user)"
    echo -e "  ${DIM}[3]${RESET} 技术员 (technician)"
    echo -n "选择: "
    
    read -r role_choice
    
    local role="user"
    case "$role_choice" in
        1) role="admin" ;;
        2) role="user" ;;
        3) role="technician" ;;
        *) print_error "无效选择"; pause_for_user; return 1 ;;
    esac
    
    echo -e "\n${CYAN}┌─ 确认信息${RESET}"
    echo -e "│ 用户名: ${BOLD}$username${RESET}"
    echo -e "│ 角色: ${YELLOW}$role${RESET}"
    echo -e "${CYAN}└─────────────────────────────────────────────────────────────────────────────${RESET}\n"
    
    if ! confirm "确认添加用户？"; then
        print_warn "已取消"
        pause_for_user
        return 1
    fi
    
    print_info "添加用户中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/user/add" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $CURRENT_TOKEN" \
        -d "{\"username\": \"$username\", \"password\": \"$password\", \"role\": \"$role\"}" 2>/dev/null)
    
    local success=$(echo "$response" | jq -r '.success // false' 2>/dev/null)
    
    if [ "$success" = "true" ]; then
        print_success "用户已添加"
    else
        local error=$(echo "$response" | jq -r '.error // "未知错误"' 2>/dev/null)
        print_error "添加失败: $error"
    fi
    
    pause_for_user
}

# 删除用户对话框
show_delete_user_dialog() {
    clear
    print_title "删除用户"
    
    local username=$(get_input "用户名")
    
    print_warn "删除用户是不可逆操作！"
    
    if ! confirm "确认删除用户 $username？"; then
        print_warn "已取消"
        pause_for_user
        return 1
    fi
    
    if ! confirm "请再次确认"; then
        print_warn "已取消"
        pause_for_user
        return 1
    fi
    
    print_info "删除用户中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/user/delete" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $CURRENT_TOKEN" \
        -d "{\"username\": \"$username\"}" 2>/dev/null)
    
    local success=$(echo "$response" | jq -r '.success // false' 2>/dev/null)
    
    if [ "$success" = "true" ]; then
        print_success "用户已删除"
    else
        local error=$(echo "$response" | jq -r '.error // "未知错误"' 2>/dev/null)
        print_error "删除失败: $error"
    fi
    
    pause_for_user
}

# 列出所有用户
show_list_users_view() {
    clear
    print_title "用户列表"
    
    print_info "获取用户列表..."
    
    local response=$(curl -s "$API_BASE_URL/api/user/list" \
        -H "Authorization: Bearer $CURRENT_TOKEN" 2>/dev/null)
    
    local count=$(echo "$response" | jq '.users | length // 0' 2>/dev/null)
    
    echo -e "\n${CYAN}共 $count 个用户：${RESET}\n"
    
    if [ "$count" -eq 0 ]; then
        echo "无用户"
    else
        echo -e "  ${BOLD}用户名${RESET}        ${BOLD}角色${RESET}          ${BOLD}创建时间${RESET}"
        echo "$response" | jq -r '.users[] | "  \(.username)   \(.role)   \(.created_at)"' 2>/dev/null
    fi
    
    pause_for_user
}

# 查看系统统计
show_system_stats() {
    clear
    print_title "系统统计"
    
    print_info "获取统计数据..."
    
    local stats=$(curl -s "$API_BASE_URL/api/stats" \
        -H "Authorization: Bearer $CURRENT_TOKEN" 2>/dev/null)
    
    if [ -z "$stats" ]; then
        print_error "无法获取统计数据"
        pause_for_user
        return 1
    fi
    
    echo -e "\n${CYAN}打印统计:${RESET}"
    echo -e "  总打印页数: $(echo "$stats" | jq '.total_pages // 0' 2>/dev/null)"
    echo -e "  总任务数: $(echo "$stats" | jq '.total_jobs // 0' 2>/dev/null)"
    echo -e "  完成任务数: $(echo "$stats" | jq '.completed_jobs // 0' 2>/dev/null)"
    
    pause_for_user
}

# 查看日志
show_logs() {
    clear
    print_title "系统日志"
    
    echo -e "\n${CYAN}后端日志 (最后20行):${RESET}\n"
    tail -20 "$BACKEND_LOG" 2>/dev/null || echo "无日志"
    
    pause_for_user
}

# 模拟错误对话框
show_simulate_error_dialog() {
    clear
    print_title "模拟故障"
    
    echo -e "${CYAN}选择故障类型:${RESET}"
    echo -e "  ${DIM}[1]${RESET} 纸张缺少"
    echo -e "  ${DIM}[2]${RESET} 碳粉不足"
    echo -e "  ${DIM}[3]${RESET} 纸张卡纸"
    echo -e "  ${DIM}[4]${RESET} 硬件故障"
    echo -n "选择: "
    
    read -r error_choice
    
    local error_type=""
    case "$error_choice" in
        1) error_type="paper_empty" ;;
        2) error_type="toner_low" ;;
        3) error_type="paper_jam" ;;
        4) error_type="hardware_error" ;;
        *) print_error "无效选择"; pause_for_user; return 1 ;;
    esac
    
    print_info "模拟故障中..."
    
    local response=$(curl -s -X POST "$API_BASE_URL/api/error/simulate" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $CURRENT_TOKEN" \
        -d "{\"error_type\": \"$error_type\"}" 2>/dev/null)
    
    local success=$(echo "$response" | jq -r '.success // false' 2>/dev/null)
    
    if [ "$success" = "true" ]; then
        print_success "故障已模拟"
    else
        print_error "操作失败"
    fi
    
    pause_for_user
}

# 清除错误对话框
show_clear_error_dialog() {
    clear
    print_title "清除故障"
    
    if confirm "确认清除所有故障？"; then
        print_info "清除中..."
        
        local response=$(curl -s -X POST "$API_BASE_URL/api/error/clear" \
            -H "Authorization: Bearer $CURRENT_TOKEN" 2>/dev/null)
        
        local success=$(echo "$response" | jq -r '.success // false' 2>/dev/null)
        
        if [ "$success" = "true" ]; then
            print_success "故障已清除"
        else
            print_error "操作失败"
        fi
    fi
    
    pause_for_user
}

# ============================================================================
# 主菜单与程序流程
# ============================================================================

# 显示主菜单
show_main_menu() {
    while true; do
        clear
        
        # 标题
        echo -e "${CYAN}${BOLD}"
        cat << 'EOF'
╔════════════════════════════════════════════════════════════════════════════╗
║                                                                            ║
║               🖨  网络打印机操作系统 (Printer OS) v1.0                     ║
║                                                                            ║
║              交互式打印机管理系统 • 优雅 • 高效 • 完整功能                 ║
║                                                                            ║
╚════════════════════════════════════════════════════════════════════════════╝
EOF
        echo -e "${RESET}\n"
        
        # 用户信息
        if [ -n "$CURRENT_USER" ]; then
            echo -e "${MAGENTA}┌─ 当前用户${RESET}"
            echo -e "│ 用户: ${BOLD}$CURRENT_USER${RESET} | 角色: ${YELLOW}$CURRENT_ROLE${RESET}"
            echo -e "│ 登录时间: ${DIM}$SESSION_START_TIME${RESET}"
            echo -e "${MAGENTA}└─────────────────────────────────────────────────────────────────────────────${RESET}\n"
        fi
        
        # 菜单选项
        echo -e "${CYAN}主菜单:${RESET}\n"
        echo -e "  ${DIM}[1]${RESET} 📊 实时仪表板"
        echo -e "  ${DIM}[2]${RESET} 📋 任务管理"
        echo -e "  ${DIM}[3]${RESET} 🛠  耗材管理"
        
        if [ "$CURRENT_ROLE" = "admin" ]; then
            echo -e "  ${DIM}[4]${RESET} 👨‍💼 管理员控制面板"
        fi
        
        echo -e "\n${CYAN}用户:${RESET}"
        
        if [ -n "$CURRENT_USER" ]; then
            echo -e "  ${DIM}[9]${RESET} 🚪 登出"
        else
            echo -e "  ${DIM}[9]${RESET} 🔑 登录"
        fi
        
        echo -e "  ${DIM}[0]${RESET} 💤 退出系统"
        echo
        
        read -r choice
        
        case "$choice" in
            1) show_dashboard ;;
            2) show_task_management_menu ;;
            3) show_supplies_menu ;;
            4) [ "$CURRENT_ROLE" = "admin" ] && show_admin_menu ;;
            9)
                if [ -n "$CURRENT_USER" ]; then
                    user_logout
                else
                    user_login
                fi
                ;;
            0)
                clear
                echo -e "${CYAN}${BOLD}感谢使用网络打印机操作系统${RESET}"
                echo -e "${DIM}再见！${RESET}\n"
                return 0
                ;;
            *) print_error "无效选择" ;;
        esac
    done
}

# 显示欢迎屏幕
show_welcome() {
    clear
    
    echo -e "${CYAN}${BOLD}"
    cat << 'EOF'
╔════════════════════════════════════════════════════════════════════════════╗
║                                                                            ║
║               🖨  网络打印机操作系统 - 欢迎                               ║
║                      Version 1.0 Professional                             ║
║                                                                            ║
╚════════════════════════════════════════════════════════════════════════════╝
EOF
    echo -e "${RESET}\n"
    
    echo -e "${YELLOW}正在启动系统...${RESET}\n"
    
    print_info "检查依赖项..."
    if ! check_dependencies; then
        print_error "依赖项检查失败"
        exit 1
    fi
    print_success "依赖项检查完成"
    
    echo
    print_info "启动后端服务..."
    if ! start_all_services; then
        print_error "服务启动失败"
        exit 1
    fi
    
    sleep 1
    
    echo -e "\n${GREEN}${BOLD}系统启动完成！${RESET}\n"
    echo -e "${DIM}按 Enter 键继续...${RESET}"
    read -r
}

# ============================================================================
# 主程序入口
# ============================================================================

main() {
    # 显示欢迎屏幕
    show_welcome
    
    # 进入主菜单循环
    while true; do
        # 如果未登录，先登录
        if [ -z "$CURRENT_USER" ]; then
            clear
            print_title "用户认证"
            
            if ! user_login; then
                continue
            fi
        fi
        
        # 显示主菜单
        show_main_menu
        
        if [ $? -eq 0 ]; then
            break
        fi
    done
}

# 运行主程序
main "$@"
