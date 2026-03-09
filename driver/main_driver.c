/**
 * main_driver.c
 * 网络打印机驱动程序主程序
 */

#include "driver_server.h"
#include <signal.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
#include <string.h>

static volatile int keep_running = 1;

/* 信号处理 */
static void signal_handler(int sig) {
    if (sig == SIGINT || sig == SIGTERM) {
        printf("\n收到终止信号，正在关闭驱动...\n");
        keep_running = 0;
        stop_driver_server();
        exit(0);
    }
}

int main() {
    printf("================================\n");
    printf("  网络打印机驱动程序 v1.0\n");
    printf("================================\n\n");
    
    /* 注册信号处理 */
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);
    
    /* 启动驱动服务器 */
    int port = 9999;
    if (start_driver_server(port) < 0) {
        fprintf(stderr, "错误：启动驱动服务器失败（errno=%d: %s）\n", errno, strerror(errno));
        fprintf(stderr, "可能原因：\n");
        fprintf(stderr, "  1. 端口 %d 已被其他程序占用\n", port);
        fprintf(stderr, "  2. 没有足够的权限\n");
        fprintf(stderr, "  3. 内存不足\n");
        return 1;
    }
    
    printf("\n驱动程序运行中，按 Ctrl+C 退出...\n\n");
    
    /* 保持运行 */
    while (keep_running) {
        sleep(1);
    }
    
    return 0;
}
