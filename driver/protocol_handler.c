/**
 * protocol_handler.c
 * 二进制协议处理器
 * 用于处理来自Go后端的二进制协议请求
 */

#include "protocol.h"
#include "printer_simulator.h"
#include "state_machine.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

/* ==================== 前向声明 ==================== */

/**
 * 验证并解析协议头
 */
static ProtocolHeader* verify_and_parse_header(uint8_t* buffer, size_t len);

/**
 * 编码ACK响应
 */
static int encode_ack(uint8_t* buffer, size_t max_len, uint32_t sequence, uint8_t status);

/**
 * 获取命令名称字符串
 */
static const char* get_command_name(uint8_t cmd);

/* ==================== 命令处理器 ==================== */

/**
 * 处理GET_STATUS命令
 */
static int handle_get_status(Printer* printer, uint8_t* response_buf, size_t max_len) {
    if (printer == NULL || response_buf == NULL) return -1;
    
    StatusResponse status;
    status.status = printer->status;
    status.error = printer->error;
    status.paper_pages = printer->paper_pages;
    status.toner_percentage = printer->toner_percentage;
    status.temperature = printer->temperature;
    status.page_count = printer->page_count;
    status.queue_size = printer->queue_size;
    status.current_task_id = (printer->current_task != NULL) ? printer->current_task->task_id : 0;
    memset(status.reserved, 0, sizeof(status.reserved));
    
    return (int)encode_status_response(response_buf, max_len, &status);
}

/**
 * 处理GET_QUEUE命令
 */
static int handle_get_queue(Printer* printer, uint8_t* response_buf, size_t max_len) {
    if (printer == NULL || response_buf == NULL) return -1;
    
    /* 编码队列信息 */
    size_t pos = 0;
    
    /* 编码头 */
    ProtocolHeader hdr;
    hdr.magic = PROTOCOL_MAGIC;
    hdr.version = PROTOCOL_VERSION;
    hdr.cmd = CMD_GET_QUEUE;
    hdr.sequence = 0;
    
    /* 计算数据长度：队列大小(2字节) + 每个任务的QueueItem */
    size_t data_len = 2 + (printer->queue_size * sizeof(QueueItem));
    if (printer->current_task != NULL) {
        data_len += sizeof(QueueItem);
    }
    
    hdr.length = data_len;
    
    /* 写入头 */
    memcpy(response_buf, &hdr, PROTOCOL_HEADER_SIZE);
    pos = PROTOCOL_HEADER_SIZE;
    
    /* 写入队列大小 */
    uint16_t size = printer->queue_size + (printer->current_task != NULL ? 1 : 0);
    memcpy(response_buf + pos, &size, 2);
    pos += 2;
    
    /* 写入当前正在打印的任务 */
    if (printer->current_task != NULL) {
        QueueItem item;
        item.task_id = printer->current_task->task_id;
        item.status = printer->current_task->status;
        item.total_pages = printer->current_task->page_count;
        item.printed_pages = printer->current_task->printed_pages;
        item.priority = 0;
        item.paper_size = 0;
        item.submit_time = printer->current_task->submit_time;
        strncpy(item.filename, printer->current_task->filename, sizeof(item.filename) - 1);
        item.filename[sizeof(item.filename) - 1] = '\0';
        
        memcpy(response_buf + pos, &item, sizeof(QueueItem));
        pos += sizeof(QueueItem);
    }
    
    /* 写入队列中的任务 */
    for (int i = 0; i < printer->queue_size; i++) {
        QueueItem item;
        item.task_id = printer->queue[i].task_id;
        item.status = printer->queue[i].status;
        item.total_pages = printer->queue[i].page_count;
        item.printed_pages = printer->queue[i].printed_pages;
        item.priority = 0;
        item.paper_size = 0;
        item.submit_time = printer->queue[i].submit_time;
        strncpy(item.filename, printer->queue[i].filename, sizeof(item.filename) - 1);
        item.filename[sizeof(item.filename) - 1] = '\0';
        
        memcpy(response_buf + pos, &item, sizeof(QueueItem));
        pos += sizeof(QueueItem);
    }
    
    /* 计算校验和 */
    uint32_t checksum = calculate_checksum(response_buf, pos);
    memcpy(response_buf + pos, &checksum, 4);
    pos += 4;
    
    return (int)pos;
}

/**
 * 处理SUBMIT_JOB命令
 */
static int handle_submit_job(Printer* printer, const uint8_t* req_data, size_t req_len, 
                            uint8_t* response_buf, size_t max_len) {
    if (printer == NULL || req_data == NULL || response_buf == NULL) return -1;
    
    SubmitJobRequest req;
    char filename[256];
    
    if (decode_submit_job_request(req_data, req_len, &req, filename, sizeof(filename)) != 0) {
        return (int)encode_error_response(response_buf, max_len, ERR_INVALID_PARAM, "Invalid job request");
    }
    
    int task_id = printer_submit_job(printer, filename, req.pages);
    if (task_id < 0) {
        if (printer->error != HARDWARE_OK) {
            return (int)encode_error_response(response_buf, max_len, ERR_HARDWARE_ERROR, "Hardware error");
        }
        return (int)encode_error_response(response_buf, max_len, ERR_QUEUE_FULL, "Queue is full");
    }
    
    printf("[PROTOCOL] 任务已提交: ID=%d, 文件=%s, 页数=%d\n", task_id, filename, req.pages);
    
    /* 编码成功响应 */
    size_t pos = 0;
    ProtocolHeader hdr;
    hdr.magic = PROTOCOL_MAGIC;
    hdr.version = PROTOCOL_VERSION;
    hdr.cmd = CMD_SUBMIT_JOB;
    hdr.sequence = req.task_id;
    hdr.length = 4;  /* task_id */
    
    memcpy(response_buf, &hdr, PROTOCOL_HEADER_SIZE);
    pos = PROTOCOL_HEADER_SIZE;
    
    uint32_t result_id = task_id;
    memcpy(response_buf + pos, &result_id, 4);
    pos += 4;
    
    uint32_t checksum = calculate_checksum(response_buf, pos);
    memcpy(response_buf + pos, &checksum, 4);
    pos += 4;
    
    return (int)pos;
}

/**
 * 处理CANCEL_JOB命令
 */
static int handle_cancel_job(Printer* printer, const uint8_t* req_data, size_t req_len,
                            uint8_t* response_buf, size_t max_len) {
    uint32_t task_id = *(uint32_t*)req_data;
    
    int result = printer_cancel_job(printer, task_id);
    if (result != 0) {
        return (int)encode_error_response(response_buf, max_len, ERR_JOB_NOT_FOUND, "Job not found");
    }
    
    printf("[PROTOCOL] 任务已取消: ID=%d\n", task_id);
    return (int)encode_ack(response_buf, max_len, task_id, (uint8_t)result);
}

/**
 * 处理PAUSE_JOB命令
 */
static int handle_pause_job(Printer* printer, const uint8_t* req_data, size_t req_len,
                           uint8_t* response_buf, size_t max_len) {
    uint32_t task_id = *(uint32_t*)req_data;
    
    int result = printer_pause_job(printer, task_id);
    if (result != 0) {
        return (int)encode_error_response(response_buf, max_len, ERR_JOB_NOT_FOUND, "Job not found");
    }
    
    printf("[PROTOCOL] 任务已暂停: ID=%d\n", task_id);
    return (int)encode_ack(response_buf, max_len, task_id, (uint8_t)result);
}

/**
 * 处理RESUME_JOB命令
 */
static int handle_resume_job(Printer* printer, const uint8_t* req_data, size_t req_len,
                            uint8_t* response_buf, size_t max_len) {
    uint32_t task_id = *(uint32_t*)req_data;
    
    int result = printer_resume_job(printer, task_id);
    if (result != 0) {
        return (int)encode_error_response(response_buf, max_len, ERR_JOB_NOT_FOUND, "Job not found");
    }
    
    printf("[PROTOCOL] 任务已恢复: ID=%d\n", task_id);
    return (int)encode_ack(response_buf, max_len, task_id, (uint8_t)result);
}

/**
 * 处理REFILL_PAPER命令
 */
static int handle_refill_paper(Printer* printer, const uint8_t* req_data, size_t req_len,
                              uint8_t* response_buf, size_t max_len) {
    int pages = *(int*)req_data;
    
    printer_refill_paper(printer, pages);
    
    printf("[PROTOCOL] 补充纸张: %d页\n", pages);
    return (int)encode_ack(response_buf, max_len, 0, (uint8_t)printer->paper_pages);
}

/**
 * 处理REFILL_TONER命令
 */
static int handle_refill_toner(Printer* printer, uint8_t* response_buf, size_t max_len) {
    printer_refill_toner(printer);
    
    printf("[PROTOCOL] 补充碳粉\n");
    return (int)encode_ack(response_buf, max_len, 0, (uint8_t)printer->toner_percentage);
}

/**
 * 处理CLEAR_ERROR命令
 */
static int handle_clear_error(Printer* printer, uint8_t* response_buf, size_t max_len) {
    printer_clear_error(printer);
    
    printf("[PROTOCOL] 清除错误\n");
    return (int)encode_ack(response_buf, max_len, 0, (uint8_t)printer->error);
}

/**
 * 处理SIMULATE_ERROR命令
 */
static int handle_simulate_error(Printer* printer, const uint8_t* req_data, size_t req_len,
                                uint8_t* response_buf, size_t max_len) {
    if (req_len < 1) {
        return (int)encode_error_response(response_buf, max_len, ERR_INVALID_PARAM, "Invalid error type");
    }
    
    HardwareError error = (HardwareError)req_data[0];
    printer_simulate_error(printer, error);
    
    printf("[PROTOCOL] 模拟硬件错误: %d\n", error);
    return (int)encode_ack(response_buf, max_len, 0, (uint8_t)printer->error);
}

/**
 * 处理SET_PAPER_MAX命令
 */
static int handle_set_paper_max(Printer* printer, const uint8_t* req_data, size_t req_len,
                               uint8_t* response_buf, size_t max_len) {
    if (req_len < 4) {
        return (int)encode_error_response(response_buf, max_len, ERR_INVALID_PARAM, "Invalid paper max");
    }
    
    int max_pages = *(int*)req_data;
    printer_set_paper_max(printer, max_pages);
    
    printf("[PROTOCOL] 设置纸张最大值: %d页\n", max_pages);
    return (int)encode_ack(response_buf, max_len, 0, (uint8_t)ERR_SUCCESS);
}

/* ==================== 请求路由 ==================== */

/**
 * 处理协议请求的主路由器
 * 返回响应数据包的大小，失败返回-1
 */
int protocol_handle_request(Printer* printer, const uint8_t* request_packet, size_t request_len,
                           uint8_t* response_buf, size_t response_max_len) {
    if (printer == NULL || request_packet == NULL || response_buf == NULL) {
        return -1;
    }
    
    /* 解析协议头 */
    ProtocolHeader* hdr = verify_and_parse_header((uint8_t*)request_packet, request_len);
    if (hdr == NULL) {
        return (int)encode_error_response(response_buf, response_max_len, ERR_INVALID_CMD, "Invalid packet format");
    }
    
    /* 提取数据部分 */
    const uint8_t* data = request_packet + PROTOCOL_HEADER_SIZE;
    size_t data_len = hdr->length;
    
    printf("[PROTOCOL] 收到命令: %s (seq=%u)\n", get_command_name(hdr->cmd), hdr->sequence);
    
    /* 根据命令类型处理 */
    switch (hdr->cmd) {
        case CMD_GET_STATUS:
            return handle_get_status(printer, response_buf, response_max_len);
            
        case CMD_GET_QUEUE:
            return handle_get_queue(printer, response_buf, response_max_len);
            
        case CMD_SUBMIT_JOB:
            return handle_submit_job(printer, data, data_len, response_buf, response_max_len);
            
        case CMD_CANCEL_JOB:
            return handle_cancel_job(printer, data, data_len, response_buf, response_max_len);
            
        case CMD_PAUSE_JOB:
            return handle_pause_job(printer, data, data_len, response_buf, response_max_len);
            
        case CMD_RESUME_JOB:
            return handle_resume_job(printer, data, data_len, response_buf, response_max_len);
            
        case CMD_REFILL_PAPER:
            return handle_refill_paper(printer, data, data_len, response_buf, response_max_len);
            
        case CMD_REFILL_TONER:
            return handle_refill_toner(printer, response_buf, response_max_len);
            
        case CMD_CLEAR_ERROR:
            return handle_clear_error(printer, response_buf, response_max_len);
            
        case CMD_SIMULATE_ERROR:
            return handle_simulate_error(printer, data, data_len, response_buf, response_max_len);
            
        case CMD_SET_PAPER_MAX:
            return handle_set_paper_max(printer, data, data_len, response_buf, response_max_len);
            
        default:
            printf("[PROTOCOL] 未知命令: 0x%02x\n", hdr->cmd);
            return (int)encode_error_response(response_buf, response_max_len, ERR_INVALID_CMD, "Unknown command");
    }
}

/* ==================== 实现 ==================== */

static const char* get_command_name(uint8_t cmd) {
    switch (cmd) {
        case CMD_GET_STATUS: return "GET_STATUS";
        case CMD_GET_QUEUE: return "GET_QUEUE";
        case CMD_SUBMIT_JOB: return "SUBMIT_JOB";
        case CMD_CANCEL_JOB: return "CANCEL_JOB";
        case CMD_PAUSE_JOB: return "PAUSE_JOB";
        case CMD_RESUME_JOB: return "RESUME_JOB";
        case CMD_REFILL_PAPER: return "REFILL_PAPER";
        case CMD_REFILL_TONER: return "REFILL_TONER";
        case CMD_CLEAR_ERROR: return "CLEAR_ERROR";
        case CMD_SIMULATE_ERROR: return "SIMULATE_ERROR";
        case CMD_SET_PAPER_MAX: return "SET_PAPER_MAX";
        case CMD_ACK: return "ACK";
        case CMD_ERROR: return "ERROR";
        default: return "UNKNOWN";
    }
}

/**
 * 验证并解析协议头
 */
static ProtocolHeader* verify_and_parse_header(uint8_t* buffer, size_t len) {
    if (len < PROTOCOL_HEADER_SIZE + 4) return NULL;
    
    ProtocolHeader* hdr = (ProtocolHeader*)buffer;
    
    if (hdr->magic != PROTOCOL_MAGIC) return NULL;
    if (hdr->version != PROTOCOL_VERSION) return NULL;
    if (PROTOCOL_HEADER_SIZE + hdr->length + 4 != len) return NULL;
    
    uint32_t received_checksum;
    memcpy(&received_checksum, buffer + PROTOCOL_HEADER_SIZE + hdr->length, 4);
    uint32_t calculated_checksum = calculate_checksum(buffer, PROTOCOL_HEADER_SIZE + hdr->length);
    
    if (received_checksum != calculated_checksum) return NULL;
    
    return hdr;
}

/**
 * 编码ACK响应
 */
static int encode_ack(uint8_t* buffer, size_t max_len, uint32_t sequence, uint8_t status) {
    if (max_len < PROTOCOL_HEADER_SIZE + 1 + 4) {
        printf("[PROTOCOL] 缓冲区过小: 需要 %zu, 实际 %zu\n", PROTOCOL_HEADER_SIZE + 1 + 4, max_len);
        return -1;
    }
    
    ProtocolHeader hdr;
    hdr.magic = PROTOCOL_MAGIC;
    hdr.version = PROTOCOL_VERSION;
    hdr.cmd = CMD_ACK;
    hdr.sequence = sequence;
    hdr.length = 1;  /* 状态码 */
    
    memcpy(buffer, &hdr, PROTOCOL_HEADER_SIZE);
    buffer[PROTOCOL_HEADER_SIZE] = status;
    
    uint32_t checksum = calculate_checksum(buffer, PROTOCOL_HEADER_SIZE + 1);
    memcpy(buffer + PROTOCOL_HEADER_SIZE + 1, &checksum, 4);
    
    return PROTOCOL_HEADER_SIZE + 1 + 4;
}
