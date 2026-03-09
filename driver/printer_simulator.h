/**
 * printer_simulator.h
 * 网络打印机硬件模拟器头文件
 * 定义打印机的硬件结构和功能接口
 */

#ifndef PRINTER_SIMULATOR_H
#define PRINTER_SIMULATOR_H

#include <time.h>

/* 打印机状态枚举 */
typedef enum {
    PRINTER_IDLE,           /* 空闲状态 */
    PRINTER_PRINTING,       /* 正在打印 */
    PRINTER_ERROR,          /* 错误状态 */
    PRINTER_PAUSED,         /* 暂停状态 */
    PRINTER_OFFLINE         /* 离线状态 */
} PrinterStatus;

/* 硬件故障类型 */
typedef enum {
    HARDWARE_OK,            /* 正常 */
    ERROR_PAPER_EMPTY,      /* 缺纸 */
    ERROR_TONER_LOW,        /* 碳粉不足 */
    ERROR_TONER_EMPTY,      /* 缺少碳粉 */
    ERROR_HEAT_UNAVAILABLE, /* 加热器故障 */
    ERROR_MOTOR_FAILURE,    /* 电机故障 */
    ERROR_SENSOR_FAILURE    /* 传感器故障 */
} HardwareError;

/* 打印任务结构 */
typedef struct {
    int task_id;            /* 任务 ID */
    char filename[256];     /* 文件名 */
    int page_count;         /* 页数 */
    int printed_pages;      /* 已打印页数 */
    time_t submit_time;     /* 提交时间 */
    time_t start_time;      /* 开始时间 */
    PrinterStatus status;   /* 状态 */
} PrintTask;

/* 打印机硬件模拟结构 */
typedef struct {
    /* 基本信息 */
    char model[128];        /* 型号 */
    char serial_number[128];/* 序列号 */
    char firmware_version[64]; /* 固件版本 */

    /* 耗材状态 */
    int paper_pages;        /* 纸张剩余页数 */
    int toner_percentage;   /* 碳粉百分比 */

    /* 硬件状态 */
    PrinterStatus status;   /* 打印机状态 */
    HardwareError error;    /* 硬件错误 */
    int temperature;        /* 打印头温度（摄氏度） */
    int page_count;         /* 总打印页数 */

    /* 打印队列 */
    PrintTask queue[100];   /* 打印任务队列 */
    int queue_size;         /* 队列中的任务数 */
    int next_task_id;       /* 下一个任务 ID */

    /* 当前打印任务 */
    PrintTask* current_task; /* 当前任务指针 */
    int print_speed;        /* 打印速度（页/分钟） */

} Printer;

/* 函数声明 */

/**
 * 初始化打印机
 * 返回指向新创建的 Printer 结构的指针
 */
Printer* printer_init();

/**
 * 释放打印机资源
 */
void printer_free(Printer* printer);

/**
 * 提交打印任务
 * 返回任务 ID，失败返回 -1
 */
int printer_submit_job(Printer* printer, const char* filename, int page_count);

/**
 * 取消打印任务
 */
int printer_cancel_job(Printer* printer, int task_id);

/**
 * 暂停打印任务
 */
int printer_pause_job(Printer* printer, int task_id);

/**
 * 恢复打印任务
 */
int printer_resume_job(Printer* printer, int task_id);

/**
 * 处理一个打印周期
 */
void printer_process_cycle(Printer* printer);

/**
 * 获取打印机状态信息
 */
void printer_get_status(Printer* printer, char* status_json, int buffer_size);

/**
 * 获取打印队列信息
 */
void printer_get_queue(Printer* printer, char* queue_json, int buffer_size);

/**
 * 补充纸张
 */
void printer_refill_paper(Printer* printer, int pages);

/**
 * 补充碳粉
 */
void printer_refill_toner(Printer* printer);

/**
 * 清除硬件错误
 */
void printer_clear_error(Printer* printer);

/**
 * 模拟硬件故障
 */
void printer_simulate_error(Printer* printer, HardwareError error);

#endif /* PRINTER_SIMULATOR_H */
