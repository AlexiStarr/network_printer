/**
 * driver_server.c
 * 打印机驱动服务器实现
 */

#include "driver_server.h"
#include "platform.h"
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

/* JSON 解析辅助函数 */
static char* json_get_string(const char* json, const char* key) {
    char search_str[256];
    snprintf(search_str, sizeof(search_str), "\"%s\":\"", key);
    
    const char* start = strstr(json, search_str);
    if (start == NULL) return NULL;
    
    start += strlen(search_str);
    const char* end = strchr(start, '"');
    if (end == NULL) return NULL;
    
    char* result = (char*)malloc(end - start + 1);
    strncpy(result, start, end - start);
    result[end - start] = '\0';
    
    return result;
}

static int json_get_int(const char* json, const char* key) {
    char search_str[256];
    snprintf(search_str, sizeof(search_str), "\"%s\":", key);
    
    const char* start = strstr(json, search_str);
    if (start == NULL) return -1;
    
    start += strlen(search_str);
    return atoi(start);
}

/* 解析客户端请求 */
Request* parse_request(const char* json) {
    if (json == NULL) return NULL;
    
    Request* req = (Request*)malloc(sizeof(Request));
    if (req == NULL) return NULL;
    
    /* 确定命令类型 */
    if (strstr(json, "\"cmd\":\"get_status\"")) {
        req->cmd = CMD_GET_STATUS;
    } else if (strstr(json, "\"cmd\":\"get_queue\"")) {
        req->cmd = CMD_GET_QUEUE;
    } else if (strstr(json, "\"cmd\":\"submit_job\"")) {
        req->cmd = CMD_SUBMIT_JOB;
    } else if (strstr(json, "\"cmd\":\"cancel_job\"")) {
        req->cmd = CMD_CANCEL_JOB;
    } else if (strstr(json, "\"cmd\":\"pause_job\"")) {
        req->cmd = CMD_PAUSE_JOB;
    } else if (strstr(json, "\"cmd\":\"resume_job\"")) {
        req->cmd = CMD_RESUME_JOB;
    } else if (strstr(json, "\"cmd\":\"refill_paper\"")) {
        req->cmd = CMD_REFILL_PAPER;
    } else if (strstr(json, "\"cmd\":\"refill_toner\"")) {
        req->cmd = CMD_REFILL_TONER;
    } else if (strstr(json, "\"cmd\":\"clear_error\"")) {
        req->cmd = CMD_CLEAR_ERROR;
    } else if (strstr(json, "\"cmd\":\"simulate_error\"")) {
        req->cmd = CMD_SIMULATE_ERROR;
    } else {
        req->cmd = CMD_UNKNOWN;
    }
    
    /* 复制参数 */
    req->params = (char*)malloc(strlen(json) + 1);
    strcpy(req->params, json);
    
    return req;
}

/* 释放请求 */
void free_request(Request* req) {
    if (req != NULL) {
        if (req->params != NULL) {
            free(req->params);
        }
        free(req);
    }
}

/* 处理请求 */
Response* handle_request(Printer* printer, Request* req) {
    if (printer == NULL || req == NULL) return NULL;
    
    Response* resp = (Response*)malloc(sizeof(Response));
    if (resp == NULL) return NULL;
    
    // Bug #4 修复: 使用动态分配替代固定缓冲区大小
    // 预分配8KB，如不足会自动扩展
    size_t buffer_size = 8192;
    resp->data = (char*)malloc(buffer_size);
    if (resp->data == NULL) {
        free(resp);
        return NULL;
    }
    
    switch (req->cmd) {
        case CMD_GET_STATUS: {
            printer_get_status(printer, resp->data, buffer_size);
            resp->success = 1;
            break;
        }
        case CMD_GET_QUEUE: {
            printer_get_queue(printer, resp->data, buffer_size);
            resp->success = 1;
            break;
        }
        case CMD_SUBMIT_JOB: {
            char* filename = json_get_string(req->params, "filename");
            int pages = json_get_int(req->params, "pages");
            
            if (filename != NULL && pages > 0) {
                int task_id = printer_submit_job(printer, filename, pages);
                if (task_id > 0) {
                    snprintf(resp->data, buffer_size, "{\"success\":true,\"task_id\":%d}", task_id);
                    resp->success = 1;
                } else {
                    strcpy(resp->data, "{\"success\":false,\"error\":\"Failed to submit job\"}");
                    resp->success = 0;
                }
                free(filename);  // Bug #4: 释放字符串内存
            } else {
                strcpy(resp->data, "{\"success\":false,\"error\":\"Invalid parameters\"}");
                resp->success = 0;
                if (filename != NULL) free(filename);  // Bug #4: 确保释放
            }
            break;
        }
        case CMD_CANCEL_JOB: {
            int task_id = json_get_int(req->params, "task_id");
            int result = printer_cancel_job(printer, task_id);
            if (result == 0) {
                snprintf(resp->data, buffer_size, "{\"success\":true}");
                resp->success = 1;
            } else {
                strcpy(resp->data, "{\"success\":false,\"error\":\"Failed to cancel job\"}");
                resp->success = 0;
            }
            break;
        }
        case CMD_PAUSE_JOB: {
            int task_id = json_get_int(req->params, "task_id");
            int result = printer_pause_job(printer, task_id);
            if (result == 0) {
                snprintf(resp->data, buffer_size, "{\"success\":true}");
                resp->success = 1;
            } else {
                strcpy(resp->data, "{\"success\":false,\"error\":\"Failed to pause job\"}");
                resp->success = 0;
            }
            break;
        }
        case CMD_RESUME_JOB: {
            int task_id = json_get_int(req->params, "task_id");
            int result = printer_resume_job(printer, task_id);
            if (result == 0) {
                snprintf(resp->data, buffer_size, "{\"success\":true}");
                resp->success = 1;
            } else {
                strcpy(resp->data, "{\"success\":false,\"error\":\"Failed to resume job\"}");
                resp->success = 0;
            }
            break;
        }
        case CMD_REFILL_PAPER: {
            int pages = json_get_int(req->params, "pages");
            printer_refill_paper(printer, pages);
            snprintf(resp->data, buffer_size, "{\"success\":true,\"paper_pages\":%d}", printer->paper_pages);
            resp->success = 1;
            break;
        }
        case CMD_REFILL_TONER: {
            printer_refill_toner(printer);
            snprintf(resp->data, buffer_size, "{\"success\":true,\"toner_percentage\":%d}", printer->toner_percentage);
            resp->success = 1;
            break;
        }
        case CMD_CLEAR_ERROR: {
            printer_clear_error(printer);
            strcpy(resp->data, "{\"success\":true}");
            resp->success = 1;
            break;
        }
        case CMD_SIMULATE_ERROR: {
            char* error_str = json_get_string(req->params, "error");
            if (error_str != NULL) {
                HardwareError error = HARDWARE_OK;
                if (strcmp(error_str, "PAPER_EMPTY") == 0) error = ERROR_PAPER_EMPTY;
                else if (strcmp(error_str, "TONER_LOW") == 0) error = ERROR_TONER_LOW;
                else if (strcmp(error_str, "TONER_EMPTY") == 0) error = ERROR_TONER_EMPTY;
                else if (strcmp(error_str, "HEAT_UNAVAILABLE") == 0) error = ERROR_HEAT_UNAVAILABLE;
                else if (strcmp(error_str, "MOTOR_FAILURE") == 0) error = ERROR_MOTOR_FAILURE;
                else if (strcmp(error_str, "SENSOR_FAILURE") == 0) error = ERROR_SENSOR_FAILURE;
                
                printer_simulate_error(printer, error);
                strcpy(resp->data, "{\"success\":true}");
                resp->success = 1;
                free(error_str);
            } else {
                strcpy(resp->data, "{\"success\":false,\"error\":\"Invalid error type\"}");
                resp->success = 0;
            }
            break;
        }
        default:
            strcpy(resp->data, "{\"success\":false,\"error\":\"Unknown command\"}");
            resp->success = 0;
    }
    
    return resp;
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
static unsigned int __stdcall handle_client(void* arg) {
    SOCKET client_sock = *(SOCKET*)arg;
    free(arg);
    
    char buffer[BUFFER_SIZE];
    
    while (running) {
        memset(buffer, 0, BUFFER_SIZE);
        int bytes = recv(client_sock, buffer, BUFFER_SIZE - 1, 0);
        
        if (bytes <= 0) {
            break; /* 客户端断开连接 */
        }
        
        /* 解析并处理请求 */
        Request* req = parse_request(buffer);
        if (req != NULL) {
            Response* resp = handle_request(global_printer, req);
            if (resp != NULL) {
                send(client_sock, resp->data, (int)strlen(resp->data), 0);
                free_response(resp);
            }
            free_request(req);
        }
    }
    
    closesocket(client_sock);
    thread_exit(0);
    return 0;
}

/* 处理打印循环 */
static void* printer_process_loop(void* arg) {
    while (running) {
        if (global_printer != NULL) {
            printer_process_cycle(global_printer);
        }
        sleep_ms(100); /* 每 100ms 处理一个周期 */
    }
    thread_exit(0);
    return NULL;
}

/* 服务器主循环 */
static unsigned int __stdcall server_loop(void* arg) {
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
        
        /* 为每个客户端创建一个线程 */
        thread_t client_thread;
        thread_create(&client_thread, handle_client, client_sock);
        thread_detach(client_thread);
    }
    
    thread_exit(0);
    return 0;
}

/* 启动驱动服务器 */
int start_driver_server(int port) {
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