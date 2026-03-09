/**
 * test_driver.c
 * 驱动程序测试客户端 (C语言实现)
 * 直接测试 C 驱动的各项功能
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <time.h>

#define DRIVER_HOST "localhost"
#define DRIVER_PORT 9090
#define BUFFER_SIZE 4096

/* 颜色定义 */
#define GREEN   "\x1b[32m"
#define YELLOW  "\x1b[33m"
#define BLUE    "\x1b[34m"
#define RED     "\x1b[31m"
#define RESET   "\x1b[0m"

/**
 * 连接到驱动服务器
 */
int connect_to_driver() {
    int sock = socket(AF_INET, SOCK_STREAM, 0);
    if (sock < 0) {
        printf(RED "✗ 无法创建套接字\n" RESET);
        return -1;
    }

    struct sockaddr_in server_addr;
    memset(&server_addr, 0, sizeof(server_addr));
    server_addr.sin_family = AF_INET;
    server_addr.sin_port = htons(DRIVER_PORT);
    
    if (inet_pton(AF_INET, "127.0.0.1", &server_addr.sin_addr) <= 0) {
        printf(RED "✗ 无效的 IP 地址\n" RESET);
        close(sock);
        return -1;
    }

    if (connect(sock, (struct sockaddr*)&server_addr, sizeof(server_addr)) < 0) {
        printf(RED "✗ 无法连接到驱动服务器 (localhost:%d)\n" RESET, DRIVER_PORT);
        printf(RED "  请确保驱动程序已启动\n" RESET);
        close(sock);
        return -1;
    }

    return sock;
}

/**
 * 发送命令并接收响应
 */
int send_command(int sock, const char* cmd, char* response) {
    /* 发送命令 */
    if (send(sock, cmd, strlen(cmd), 0) < 0) {
        printf(RED "✗ 发送命令失败\n" RESET);
        return -1;
    }

    /* 接收响应 */
    int n = recv(sock, response, BUFFER_SIZE - 1, 0);
    if (n < 0) {
        printf(RED "✗ 接收响应失败\n" RESET);
        return -1;
    }

    response[n] = '\0';
    return n;
}

/**
 * 测试：获取打印机状态
 */
void test_get_status(int sock) {
    printf("\n" BLUE "━━━━━━━━━━━━━━━━━━━━━━" RESET "\n");
    printf(YELLOW "测试 1: 获取打印机状态\n" RESET);
    
    const char* cmd = "{\"cmd\":\"get_status\"}";
    char response[BUFFER_SIZE];
    
    if (send_command(sock, cmd, response) > 0) {
        printf(GREEN "✓ 响应:" RESET "\n%s\n", response);
    }
}

/**
 * 测试：获取打印队列
 */
void test_get_queue(int sock) {
    printf("\n" BLUE "━━━━━━━━━━━━━━━━━━━━━━" RESET "\n");
    printf(YELLOW "测试 2: 获取打印队列\n" RESET);
    
    const char* cmd = "{\"cmd\":\"get_queue\"}";
    char response[BUFFER_SIZE];
    
    if (send_command(sock, cmd, response) > 0) {
        printf(GREEN "✓ 响应:" RESET "\n%s\n", response);
    }
}

/**
 * 测试：提交打印任务
 */
void test_submit_job(int sock, int* task_id) {
    printf("\n" BLUE "━━━━━━━━━━━━━━━━━━━━━━" RESET "\n");
    printf(YELLOW "测试 3: 提交打印任务\n" RESET);
    
    const char* cmd = "{\"cmd\":\"submit_job\",\"filename\":\"document.pdf\",\"pages\":10}";
    char response[BUFFER_SIZE];
    
    if (send_command(sock, cmd, response) > 0) {
        printf(GREEN "✓ 响应:" RESET "\n%s\n", response);
        
        /* 提取 task_id */
        char* pos = strstr(response, "task_id");
        if (pos) {
            sscanf(pos, "task_id\":%d", task_id);
        }
    }
}

/**
 * 测试：模拟打印进度
 */
void test_print_progress(int sock) {
    printf("\n" BLUE "━━━━━━━━━━━━━━━━━━━━━━" RESET "\n");
    printf(YELLOW "测试 4: 监控打印进度 (10秒)\n" RESET);
    
    for (int i = 0; i < 5; i++) {
        printf(GREEN "\n  轮次 %d:\n" RESET, i + 1);
        
        const char* cmd = "{\"cmd\":\"get_queue\"}";
        char response[BUFFER_SIZE];
        
        if (send_command(sock, cmd, response) > 0) {
            /* 简单解析：查找 printed_pages */
            char* pos = strstr(response, "printed_pages");
            if (pos) {
                int pages;
                sscanf(pos, "printed_pages\":%d", &pages);
                printf("  已打印: %d 页\n", pages);
            }
        }
        
        if (i < 4) sleep(2);
    }
}

/**
 * 测试：补充纸张
 */
void test_refill_paper(int sock) {
    printf("\n" BLUE "━━━━━━━━━━━━━━━━━━━━━━" RESET "\n");
    printf(YELLOW "测试 5: 补充纸张\n" RESET);
    
    const char* cmd = "{\"cmd\":\"refill_paper\",\"pages\":500}";
    char response[BUFFER_SIZE];
    
    if (send_command(sock, cmd, response) > 0) {
        printf(GREEN "✓ 响应:" RESET "\n%s\n", response);
    }
}

/**
 * 测试：模拟硬件故障
 */
void test_simulate_error(int sock) {
    printf("\n" BLUE "━━━━━━━━━━━━━━━━━━━━━━" RESET "\n");
    printf(YELLOW "测试 6: 模拟缺纸故障\n" RESET);
    
    const char* cmd = "{\"cmd\":\"simulate_error\",\"error\":\"PAPER_EMPTY\"}";
    char response[BUFFER_SIZE];
    
    if (send_command(sock, cmd, response) > 0) {
        printf(GREEN "✓ 响应:" RESET "\n%s\n", response);
        
        /* 查看状态 */
        printf("\n验证错误状态:\n");
        const char* status_cmd = "{\"cmd\":\"get_status\"}";
        if (send_command(sock, status_cmd, response) > 0) {
            printf("%s\n", response);
        }
    }
}

/**
 * 测试：清除错误
 */
void test_clear_error(int sock) {
    printf("\n" BLUE "━━━━━━━━━━━━━━━━━━━━━━" RESET "\n");
    printf(YELLOW "测试 7: 清除错误\n" RESET);
    
    const char* cmd = "{\"cmd\":\"clear_error\"}";
    char response[BUFFER_SIZE];
    
    if (send_command(sock, cmd, response) > 0) {
        printf(GREEN "✓ 响应:" RESET "\n%s\n", response);
        
        /* 验证 */
        printf("\n验证状态已恢复:\n");
        const char* status_cmd = "{\"cmd\":\"get_status\"}";
        if (send_command(sock, status_cmd, response) > 0) {
            printf("%s\n", response);
        }
    }
}

/**
 * 交互菜单
 */
void show_menu() {
    printf("\n" GREEN "════════════════════════════════════" RESET "\n");
    printf(GREEN "  驱动程序测试工具 (C语言客户端)\n" RESET);
    printf(GREEN "════════════════════════════════════" RESET "\n\n");
    printf("测试选项:\n");
    printf("  1. 快速演示 (推荐)\n");
    printf("  2. 获取打印机状态\n");
    printf("  3. 获取打印队列\n");
    printf("  4. 提交打印任务\n");
    printf("  5. 监控打印进度\n");
    printf("  6. 补充纸张\n");
    printf("  7. 模拟硬件故障\n");
    printf("  8. 清除错误\n");
    printf("  0. 退出\n");
    printf("\n请选择 (0-8): ");
}

/**
 * 快速演示
 */
void quick_demo(int sock) {
    printf(GREEN "\n开始快速演示...\n" RESET);
    
    test_get_status(sock);
    sleep(1);
    
    int task_id = 0;
    test_submit_job(sock, &task_id);
    sleep(1);
    
    test_print_progress(sock);
    sleep(1);
    
    test_refill_paper(sock);
    sleep(1);
    
    test_get_status(sock);
    
    printf(GREEN "\n✓ 快速演示完成！\n" RESET);
}

/**
 * 主程序
 */
int main() {
    printf(GREEN "════════════════════════════════════" RESET "\n");
    printf(GREEN "  驱动程序 C 语言测试客户端\n" RESET);
    printf(GREEN "════════════════════════════════════" RESET "\n\n");
    
    /* 检查驱动连接 */
    printf("正在连接到驱动服务器 (localhost:9090)...\n");
    
    int sock = connect_to_driver();
    if (sock < 0) {
        return 1;
    }
    
    printf(GREEN "✓ 已连接到驱动服务器\n" RESET);
    
    /* 交互循环 */
    int choice;
    int task_id = 0;
    
    while (1) {
        show_menu();
        scanf("%d", &choice);
        getchar(); /* 清除换行符 */
        
        switch (choice) {
            case 0:
                printf(GREEN "\n再见！\n" RESET);
                close(sock);
                return 0;
            case 1:
                quick_demo(sock);
                break;
            case 2:
                test_get_status(sock);
                break;
            case 3:
                test_get_queue(sock);
                break;
            case 4:
                test_submit_job(sock, &task_id);
                break;
            case 5:
                test_print_progress(sock);
                break;
            case 6:
                test_refill_paper(sock);
                break;
            case 7:
                test_simulate_error(sock);
                break;
            case 8:
                test_clear_error(sock);
                break;
            default:
                printf(RED "无效选择\n" RESET);
        }
    }
    
    close(sock);
    return 0;
}
