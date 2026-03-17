/**
 * driver_server.c
 * 打印机驱动服务器实现
 * 已集成二进制协议、状态机和协议处理器
 */

#include "driver_server.h"
#include "platform.h"
#include "protocol.h"
#include "state_machine.h"
#include "protocol_handler.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <ctype.h>

#define BUFFER_SIZE 4096
#define MAX_CLIENTS 10

/* 全局变量 */
static SOCKET server_sock = INVALID_SOCK;
static int running = 0;
static Printer* global_printer = NULL;
static thread_t server_thread;
static thread_t process_thread;
static struct DriverState* driver_state = NULL;

/* ============================================
 * 二进制协议处理已由 protocol_handler.c 实现
 * 本文件不再包含 JSON 解析代码
 * ============================================
 */

/* 处理请求 */
Response* handle_request(Printer* printer, Request* req) {
    // 此函数已被 protocol_handle_request 替代，保留以兼容旧代码
    return NULL;
}

/* 释放响应 */
void free_response(Response* resp) {
    if (resp != NULL) {
        if (resp->data != NULL) {
            free(resp->data);
        }
        free(resp);
    }
}

/* 处理客户端连接 */
#ifdef _WIN32
static unsigned int __stdcall handle_client(void* arg)
#else
static void* handle_client(void* arg)
#endif
{
    SOCKET client_sock = *(SOCKET*)arg;
    free(arg);
    
    unsigned char buffer[BUFFER_SIZE];
    unsigned char response_buf[BUFFER_SIZE];
    int accumulated_bytes = 0;
    
    printf("[Driver] 客户端已连接，准备接收二进制协议数据\n");
    
    while (running) {
        memset(buffer + accumulated_bytes, 0, BUFFER_SIZE - accumulated_bytes);
        int bytes = recv(client_sock, (char*)(buffer + accumulated_bytes), BUFFER_SIZE - accumulated_bytes - 1, 0);
        
        if (bytes <= 0) {
            break; /* 客户端断开连接 */
        }
        
        accumulated_bytes += bytes;
        
        /* 检查是否有完整的数据包 */
        while (accumulated_bytes >= 16) {
            ProtocolHeader* header = (ProtocolHeader*)buffer;
            
            if (header->magic != PROTOCOL_MAGIC) {
                printf("[Driver] 错误: 无效的魔法数字 0x%X\n", header->magic);
                accumulated_bytes = 0;
                break;
            }
            
            if (header->version != PROTOCOL_VERSION) {
                printf("[Driver] 错误: 不支持的协议版本 %d\n", header->version);
                accumulated_bytes = 0;
                break;
            }
            
            uint16_t data_len = header->length;
            int total_packet_size = 12 + data_len + 4;
            
            if (accumulated_bytes < total_packet_size) {
                break;
            }
            
            uint32_t received_checksum = 0;
            memcpy(&received_checksum, buffer + 12 + data_len, 4);
            uint32_t calculated_checksum = calculate_checksum(buffer, 12 + data_len);
            if (received_checksum != calculated_checksum) {
                printf("[Driver] 错误: 校验和验证失败\n");
            }
            
            printf("[Driver] 收到完整的二进制协议数据包, 命令: %d, 长度: %d\n", header->cmd, data_len);
            
            int response_len = protocol_handle_request(global_printer, buffer, total_packet_size, response_buf, sizeof(response_buf));
            if (response_len > 0) {
                send(client_sock, (const char*)response_buf, response_len, 0);
                printf("[Driver] 已发送二进制协议响应, 长度: %d\n", response_len);
            }
            
            accumulated_bytes -= total_packet_size;
            if (accumulated_bytes > 0) {
                memmove(buffer, buffer + total_packet_size, accumulated_bytes);
            }
        }
    }
    
    closesocket(client_sock);
    printf("[Driver] 客户端连接已关闭\n");
#ifdef _WIN32
    thread_exit(0);
    return 0;
#else
    thread_exit(NULL);
    return NULL;
#endif
}

/* 处理打印循环 */
#ifdef _WIN32
static unsigned int __stdcall printer_process_loop(void* arg)
#else
static void* printer_process_loop(void* arg)
#endif
{
    while (running) {
        if (global_printer != NULL) {
            printer_process_cycle(global_printer);
        }
        sleep_ms(100); /* 每 100ms 处理一个周期 */
    }
#ifdef _WIN32
    thread_exit(0);
    return 0;
#else
    thread_exit(NULL);
    return NULL;
#endif
}

/* 服务器主循环 */
#ifdef _WIN32
static unsigned int __stdcall server_loop(void* arg)
#else
static void* server_loop(void* arg)
#endif
{
    struct sockaddr_in client_addr;
    socklen_t client_addr_len = sizeof(client_addr);
    
    while (running) {
        SOCKET* client_sock = (SOCKET*)malloc(sizeof(SOCKET));
        if (client_sock == NULL) continue;
        
        *client_sock = accept(server_sock, (struct sockaddr*)&client_addr, &client_addr_len);
        if (*client_sock == INVALID_SOCKET) {
            free(client_sock);
            sleep_ms(10);
            continue;
        }
        
        printf("[Driver] 新客户端连接: %s:%d\n", inet_ntoa(client_addr.sin_addr), ntohs(client_addr.sin_port));
        
        thread_t client_thread;
        if (thread_create(&client_thread, handle_client, client_sock) == 0) {
            thread_detach(client_thread);
        } else {
            printf("[Driver] 错误：无法创建客户端线程\n");
#ifdef _WIN32
            closesocket(*client_sock);
#else
            close(*client_sock);
#endif
            free(client_sock);
        }
    }
    
#ifdef _WIN32
    thread_exit(0);
    return 0;
#else
    thread_exit(NULL);
    return NULL;
#endif
}

/* 启动驱动服务器 */
int start_driver_server(int port) {
    // ... 此函数代码保持不变 ...
    if (running) return -1; /* 已在运行 */
    
    printf("[Driver] 初始化中...\n");
    
    /* 初始化平台 */
    if (platform_init() < 0) {
        fprintf(stderr, "[Driver] 错误：平台初始化失败\n");
        return -1;
    }
    
    /* 初始化打印机 */
    global_printer = printer_init();
    if (global_printer == NULL) {
        fprintf(stderr, "[Driver] 错误：打印机初始化失败\n");
        platform_cleanup();
        return -1;
    }
    printf("[Driver] 打印机初始化成功\n");
    
    /* 创建套接字 */
    server_sock = socket(AF_INET, SOCK_STREAM, 0);
    if (server_sock == INVALID_SOCKET) {
        fprintf(stderr, "[Driver] 错误：无法创建套接字: %s\n", get_socket_error_msg());
        platform_cleanup();
        printer_free(global_printer);
        return -1;
    }
    printf("[Driver] 套接字创建成功\n");
    
    /* 设置套接字选项 */
    int opt = 1;
    if (setsockopt(server_sock, SOL_SOCKET, SO_REUSEADDR, (const char*)&opt, sizeof(opt)) == SOCKET_ERROR) {
        fprintf(stderr, "[Driver] 警告：设置 SO_REUSEADDR 失败\n");
    }
    
    /* 绑定 */
    struct sockaddr_in server_addr;
    memset(&server_addr, 0, sizeof(server_addr));
    server_addr.sin_family = AF_INET;
    server_addr.sin_addr.s_addr = htonl(INADDR_ANY);
    server_addr.sin_port = htons(port);
    
    printf("[Driver] 尝试绑定到端口 %d...\n", port);
    if (bind(server_sock, (struct sockaddr*)&server_addr, sizeof(server_addr)) == SOCKET_ERROR) {
        fprintf(stderr, "[Driver] 错误：端口 %d 绑定失败（可能已被其他程序占用）: %s\n", port, get_socket_error_msg());
        closesocket(server_sock);
        platform_cleanup();
        printer_free(global_printer);
        return -1;
    }
    printf("[Driver] 端口 %d 绑定成功\n", port);
    
    /* 监听 */
    if (listen(server_sock, MAX_CLIENTS) == SOCKET_ERROR) {
        fprintf(stderr, "[Driver] 错误：监听失败: %s\n", get_socket_error_msg());
        closesocket(server_sock);
        platform_cleanup();
        printer_free(global_printer);
        return -1;
    }
    
    running = 1;
    
    /* 启动服务器线程和打印处理线程 */
    thread_create(&server_thread, server_loop, NULL);
    thread_create(&process_thread, printer_process_loop, NULL);
    
    printf("[Driver] 驱动服务器启动成功，监听端口 %d\n", port);
    
    return 0;
}

/* 停止驱动服务器 */
void stop_driver_server() {
    if (!running) return;
    
    running = 0;
    
    closesocket_safe(server_sock);
    
    thread_join(server_thread);
    thread_join(process_thread);
    
    if (global_printer != NULL) {
        printer_free(global_printer);
        global_printer = NULL;
    }
    
    platform_cleanup();
    
    printf("[Driver] 驱动服务器已停止\n");
}