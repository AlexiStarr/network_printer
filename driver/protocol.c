/**
 * protocol.c
 * 二进制协议编码解码实现
 */

#include "protocol.h"
#include <string.h>
#include <stdio.h>

/* ==================== 校验和计算 ==================== */
uint32_t calculate_checksum(const uint8_t* data, size_t len) {
    uint32_t sum = 0;
    for (size_t i = 0; i < len; i++) {
        sum += data[i];
        sum = (sum << 1) | (sum >> 31);  /* 循环左移 */
    }
    return sum ^ 0xDEADBEEF;  /* 与magic结合 */
}

/* ==================== 协议编码函数 ==================== */

/**
 * 编码协议头
 * 返回写入的字节数
 */
static size_t encode_header(uint8_t* buffer, size_t max_len,
                           uint8_t cmd, uint16_t data_len, uint32_t sequence) {
    if (max_len < PROTOCOL_HEADER_SIZE) return 0;
    
    ProtocolHeader* hdr = (ProtocolHeader*)buffer;
    hdr->magic = PROTOCOL_MAGIC;
    hdr->version = PROTOCOL_VERSION;
    hdr->cmd = cmd;
    hdr->length = data_len;
    hdr->sequence = sequence;
    
    return PROTOCOL_HEADER_SIZE;
}

/**
 * 编码打印机状态响应
 */
size_t encode_status_response(uint8_t* buffer, size_t max_len, 
                              const StatusResponse* status) {
    if (max_len < PROTOCOL_HEADER_SIZE + sizeof(StatusResponse) + 4) {
        return 0;
    }
    
    /* 编码头 */
    size_t pos = encode_header(buffer, max_len, CMD_GET_STATUS, 
                              sizeof(StatusResponse), 0);
    if (pos == 0) return 0;
    
    /* 复制状态数据 */
    memcpy(buffer + pos, status, sizeof(StatusResponse));
    pos += sizeof(StatusResponse);
    
    /* 计算校验和 */
    uint32_t checksum = calculate_checksum(buffer, pos);
    memcpy(buffer + pos, &checksum, 4);
    pos += 4;
    
    return pos;
}

/**
 * 编码任务进度
 */
size_t encode_task_progress(uint8_t* buffer, size_t max_len,
                            const TaskProgress* progress) {
    if (max_len < PROTOCOL_HEADER_SIZE + sizeof(TaskProgress) + 4) {
        return 0;
    }
    
    /* 编码头 */
    size_t pos = encode_header(buffer, max_len, CMD_DATA_CHUNK,
                              sizeof(TaskProgress), progress->task_id);
    if (pos == 0) return 0;
    
    /* 复制进度数据 */
    memcpy(buffer + pos, progress, sizeof(TaskProgress));
    pos += sizeof(TaskProgress);
    
    /* 计算校验和 */
    uint32_t checksum = calculate_checksum(buffer, pos);
    memcpy(buffer + pos, &checksum, 4);
    pos += 4;
    
    return pos;
}

/**
 * 编码错误响应
 */
size_t encode_error_response(uint8_t* buffer, size_t max_len,
                             ErrorCode error, const char* detail) {
    if (max_len < PROTOCOL_HEADER_SIZE + sizeof(ErrorResponse) + 4) {
        return 0;
    }
    
    size_t detail_len = detail ? strlen(detail) : 0;
    if (detail_len > 256) detail_len = 256;  /* 限制详情长度 */
    
    /* 编码头 */
    size_t pos = encode_header(buffer, max_len, CMD_ERROR,
                              sizeof(ErrorResponse) + detail_len, 0);
    if (pos == 0) return 0;
    
    /* 编码错误响应体 */
    ErrorResponse* err_resp = (ErrorResponse*)(buffer + pos);
    err_resp->error_code = error;
    err_resp->detail_len = detail_len;
    pos += sizeof(ErrorResponse);
    
    /* 复制错误详情 */
    if (detail && detail_len > 0) {
        memcpy(buffer + pos, detail, detail_len);
        pos += detail_len;
    }
    
    /* 计算校验和 */
    uint32_t checksum = calculate_checksum(buffer, pos);
    memcpy(buffer + pos, &checksum, 4);
    pos += 4;
    
    return pos;
}

/**
 * 编码ACK消息
 */
size_t encode_ack(uint8_t* buffer, size_t max_len, uint32_t sequence) {
    if (max_len < PROTOCOL_HEADER_SIZE + 4) return 0;
    
    size_t pos = encode_header(buffer, max_len, CMD_ACK, 0, sequence);
    if (pos == 0) return 0;
    
    /* 计算校验和 */
    uint32_t checksum = calculate_checksum(buffer, pos);
    memcpy(buffer + pos, &checksum, 4);
    pos += 4;
    
    return pos;
}

/* ==================== 协议解码函数 ==================== */

/**
 * 验证和解析协议头
 * 返回：成功返回协议头指针，失败返回NULL
 */
static ProtocolHeader* verify_and_parse_header(uint8_t* buffer, size_t len) {
    if (len < PROTOCOL_HEADER_SIZE + 4) return NULL;
    
    ProtocolHeader* hdr = (ProtocolHeader*)buffer;
    
    /* 验证magic */
    if (hdr->magic != PROTOCOL_MAGIC) return NULL;
    
    /* 验证版本 */
    if (hdr->version != PROTOCOL_VERSION) return NULL;
    
    /* 验证长度 */
    if (PROTOCOL_HEADER_SIZE + hdr->length + 4 != len) return NULL;
    
    /* 验证校验和 */
    uint32_t received_checksum;
    memcpy(&received_checksum, buffer + PROTOCOL_HEADER_SIZE + hdr->length, 4);
    uint32_t calculated_checksum = calculate_checksum(buffer, 
                                                      PROTOCOL_HEADER_SIZE + hdr->length);
    if (received_checksum != calculated_checksum) return NULL;
    
    return hdr;
}

/**
 * 解码提交任务请求
 */
int decode_submit_job_request(const uint8_t* buffer, size_t len,
                              SubmitJobRequest* req, char* filename, size_t filename_max) {
    ProtocolHeader* hdr = verify_and_parse_header((uint8_t*)buffer, len);
    if (hdr == NULL || hdr->cmd != CMD_SUBMIT_JOB) return -1;
    
    const uint8_t* data = buffer + PROTOCOL_HEADER_SIZE;
    size_t data_len = hdr->length;
    
    if (data_len < sizeof(SubmitJobRequest)) return -1;
    
    /* 复制请求头 */
    memcpy(req, data, sizeof(SubmitJobRequest));
    
    /* 检查文件名长度 */
    if (req->filename_len > filename_max || req->filename_len > data_len - sizeof(SubmitJobRequest)) {
        return -1;
    }
    
    /* 复制文件名 */
    const uint8_t* filename_data = data + sizeof(SubmitJobRequest);
    memcpy(filename, filename_data, req->filename_len);
    filename[req->filename_len] = '\0';  /* 确保以null结尾 */
    
    return 0;
}

/**
 * 解析任意协议包的命令类型
 */
uint8_t get_packet_command(const uint8_t* buffer, size_t len) {
    if (len < PROTOCOL_HEADER_SIZE) return CMD_ERROR;
    
    ProtocolHeader* hdr = (ProtocolHeader*)buffer;
    if (hdr->magic != PROTOCOL_MAGIC) return CMD_ERROR;
    
    return hdr->cmd;
}

/**
 * 解析序列号
 */
uint32_t get_packet_sequence(const uint8_t* buffer, size_t len) {
    if (len < PROTOCOL_HEADER_SIZE) return 0;
    
    ProtocolHeader* hdr = (ProtocolHeader*)buffer;
    return hdr->sequence;
}

/**
 * 获取作业赋予值
 */
uint16_t get_packet_data_length(const uint8_t* buffer, size_t len) {
    if (len < PROTOCOL_HEADER_SIZE) return 0;
    
    ProtocolHeader* hdr = (ProtocolHeader*)buffer;
    return hdr->length;
}
