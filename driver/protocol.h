/**
 * protocol.h
 * 二进制通信协议定义
 * 用于Go后端和C驱动之间的高效数据传输
 */

#ifndef PROTOCOL_H
#define PROTOCOL_H

#include <stdint.h>
#include <stddef.h>

/* ==================== 协议版本 ==================== */
#define PROTOCOL_VERSION 1
#define PROTOCOL_MAGIC 0xDEADBEEF  /* 0xBEEFDEAD in little endian */

/* ==================== 命令类型 ==================== */
typedef enum {
    /* 查询命令 */
    CMD_GET_STATUS = 0x01,        /* 获取打印机状态 */
    CMD_GET_QUEUE = 0x02,         /* 获取打印队列 */
    CMD_GET_HISTORY = 0x03,       /* 获取打印历史 */
    
    /* 控制命令 */
    CMD_SUBMIT_JOB = 0x10,        /* 提交打印任务 */
    CMD_CANCEL_JOB = 0x11,        /* 取消打印任务 */
    CMD_PAUSE_JOB = 0x12,         /* 暂停打印任务 */
    CMD_RESUME_JOB = 0x13,        /* 恢复打印任务 */
    
    /* 维护命令 */
    CMD_REFILL_PAPER = 0x20,      /* 补充纸张 */
    CMD_REFILL_TONER = 0x21,      /* 补充碳粉 */
    CMD_CLEAR_ERROR = 0x22,       /* 清除错误 */
    CMD_SIMULATE_ERROR = 0x23,    /* 模拟硬件错误 */
    CMD_SET_PAPER_MAX = 0x24,     /* 设置纸张最大值 */
    
    /* 数据流命令 */
    CMD_PRINT_DATA = 0x30,        /* 打印数据 */
    CMD_DATA_CHUNK = 0x31,        /* 数据块传输 */
    
    /* 响应和特殊命令 */
    CMD_ACK = 0xFE,               /* 确认消息 */
    CMD_ERROR = 0xFF,             /* 错误消息 */
} CommandType;

/* ==================== 错误代码 ==================== */
typedef enum {
    ERR_SUCCESS = 0x00,
    ERR_INVALID_CMD = 0x01,
    ERR_INVALID_PARAM = 0x02,
    ERR_BUFFER_OVERFLOW = 0x03,
    ERR_CHECKSUM_FAIL = 0x04,
    ERR_HARDWARE_ERROR = 0x05,
    ERR_QUEUE_FULL = 0x06,
    ERR_JOB_NOT_FOUND = 0x07,
    ERR_PAPER_FULL = 0x08,
    ERR_UNKNOWN = 0xFF,
} ErrorCode;

/* ==================== 硬件错误类型 ==================== */
typedef enum {
    HARDWARE_OK = 0x00,
    ERROR_PAPER_EMPTY = 0x01,
    ERROR_TONER_LOW = 0x02,
    ERROR_TONER_EMPTY = 0x03,
    ERROR_HEAT_UNAVAILABLE = 0x04,
    ERROR_MOTOR_FAILURE = 0x05,
    ERROR_SENSOR_FAILURE = 0x06,
} HardwareError;

/* ==================== 打印机状态 ==================== */
typedef enum {
    PRINTER_IDLE = 0x00,
    PRINTER_PRINTING = 0x01,
    PRINTER_PAUSED = 0x02,
    PRINTER_ERROR = 0x03,
    PRINTER_OFFLINE = 0x04,
} PrinterStatus;

/* ==================== 数据结构 ==================== */

/* 协议头结构体 */
typedef struct __attribute__((packed)) {
    uint32_t magic;              /* 0xDEADBEEF */
    uint8_t version;             /* 协议版本 */
    uint8_t cmd;                 /* 命令类型 */
    uint16_t length;             /* 数据段长度 (不包括头和校验和) */
    uint32_t sequence;           /* 序列号，用于匹配请求和响应 */
} ProtocolHeader;

/* 状态查询响应体 */
typedef struct __attribute__((packed)) {
    uint8_t status;              /* 打印机状态 */
    uint8_t error;               /* 硬件错误代码 */
    uint16_t paper_pages;        /* 纸张页数 */
    uint16_t toner_percentage;   /* 碳粉百分比 */
    uint8_t temperature;         /* 温度 (单位：℃) */
    uint32_t page_count;         /* 累计打印页数 */
    uint16_t queue_size;         /* 队列中的任务数 */
    uint8_t current_task_id;     /* 当前任务ID */
    uint8_t reserved[3];         /* 预留字段，用于字节对齐 */
} StatusResponse;

/* 提交打印任务请求体 */
typedef struct __attribute__((packed)) {
    uint32_t task_id;            /* 任务ID (由后端分配) */
    uint16_t pages;              /* 页数 */
    uint8_t priority;            /* 优先级 (0-255) */
    uint8_t paper_size;          /* 纸张大小 (A4=0, A3=1, Letter=2, ...) */
    uint16_t filename_len;       /* 文件名长度 */
    /* 变长数据：filename_len 字节的文件名 */
} SubmitJobRequest;

/* 任务状态更新 */
typedef struct __attribute__((packed)) {
    uint32_t task_id;            /* 任务ID */
    uint8_t status;              /* 任务状态 */
    uint16_t printed_pages;      /* 已打印页数 */
    uint32_t progress_percent;   /* 进度百分比 (0-100) */
    uint16_t estimated_time_sec; /* 预计剩余时间 (秒) */
} TaskProgress;

/* 队列项目 */
typedef struct __attribute__((packed)) {
    uint32_t task_id;
    uint8_t status;
    uint16_t total_pages;
    uint16_t printed_pages;
    uint8_t priority;
    uint8_t paper_size;
    uint32_t submit_time;        /* Unix时间戳 */
    char filename[64];           /* 文件名 (固定长度，以\0结尾) */
} QueueItem;

/* 错误响应体 */
typedef struct __attribute__((packed)) {
    uint8_t error_code;          /* 错误代码 */
    uint16_t detail_len;         /* 错误详情长度 */
    /* 变长数据：detail_len 字节的错误详情字符串 */
} ErrorResponse;

/* ==================== 协议常量 ==================== */
#define PROTOCOL_HEADER_SIZE sizeof(ProtocolHeader)  /* 12字节 */
#define MAX_PAYLOAD_SIZE 65536                       /* 64KB */
#define MAX_PACKET_SIZE (PROTOCOL_HEADER_SIZE + MAX_PAYLOAD_SIZE + 4)  /* 加上校验和 */

/* ==================== 校验和计算 ==================== */
uint32_t calculate_checksum(const uint8_t* data, size_t len);

/* ==================== 协议编码/解码函数 ==================== */

/* 编码打印机状态响应 */
size_t encode_status_response(uint8_t* buffer, size_t max_len, 
                              const StatusResponse* status);

/* 解码提交任务请求 */
int decode_submit_job_request(const uint8_t* buffer, size_t len,
                              SubmitJobRequest* req, char* filename, size_t filename_max);

/* 编码任务进度 */
size_t encode_task_progress(uint8_t* buffer, size_t max_len,
                            const TaskProgress* progress);

/* 编码错误响应 */
size_t encode_error_response(uint8_t* buffer, size_t max_len,
                             ErrorCode error, const char* detail);

#endif /* PROTOCOL_H */
