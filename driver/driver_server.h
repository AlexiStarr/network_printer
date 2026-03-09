/**
 * driver_server.h
 * 打印机驱动服务器头文件
 * 处理与 Go 后端的通信
 */

#ifndef DRIVER_SERVER_H
#define DRIVER_SERVER_H

#include "printer_simulator.h"

/* 命令类型 */
typedef enum {
    CMD_GET_STATUS,      /* 获取状态 */
    CMD_GET_QUEUE,       /* 获取队列 */
    CMD_SUBMIT_JOB,      /* 提交任务 */
    CMD_CANCEL_JOB,      /* 取消任务 */
    CMD_PAUSE_JOB,       /* 暂停任务 */
    CMD_RESUME_JOB,      /* 恢复任务 */
    CMD_REFILL_PAPER,    /* 补充纸张 */
    CMD_REFILL_TONER,    /* 补充碳粉 */
    CMD_CLEAR_ERROR,     /* 清除错误 */
    CMD_SIMULATE_ERROR,  /* 模拟错误 */
    CMD_UNKNOWN
} CommandType;

/* 请求结构 */
typedef struct {
    CommandType cmd;
    char* params;
} Request;

/* 响应结构 */
typedef struct {
    int success;
    char* data;
} Response;

/**
 * 解析客户端请求
 */
Request* parse_request(const char* json);

/**
 * 释放请求
 */
void free_request(Request* req);

/**
 * 处理请求
 */
Response* handle_request(Printer* printer, Request* req);

/**
 * 释放响应
 */
void free_response(Response* resp);

/**
 * 启动驱动服务器
 */
int start_driver_server(int port);

/**
 * 停止驱动服务器
 */
void stop_driver_server();

#endif /* DRIVER_SERVER_H */
