/**
 * state_machine.h
 * 打印机驱动状态机定义
 * 用于处理复杂的任务流程和硬件状态转换
 */

#ifndef STATE_MACHINE_H
#define STATE_MACHINE_H

#include <stdint.h>
#include <time.h>

/* ==================== 驱动状态定义 ==================== */
typedef enum {
    /* 初始化阶段 */
    STATE_INIT = 0,
    STATE_RESET = 1,
    
    /* 空闲阶段 */
    STATE_IDLE = 2,
    STATE_WARMING_UP = 3,
    
    /* 打印阶段 */
    STATE_PRINT_START = 10,
    STATE_PRINT_RUNNING = 11,
    STATE_PRINT_PAGE = 12,
    STATE_PRINT_PAUSE = 13,
    STATE_PRINT_RESUME = 14,
    STATE_PRINT_FINISH = 15,
    
    /* 错误处理阶段 */
    STATE_ERROR = 20,
    STATE_ERROR_RECOVERY = 21,
    STATE_ERROR_HANDLED = 22,
    
    /* 维护阶段 */
    STATE_MAINTENANCE = 30,
    STATE_OFFLINE = 31,
    
    /* 终止状态 */
    STATE_TERMINATING = 99,
} DriverState;

/* ==================== 事件类型定义 ==================== */
typedef enum {
    /* 系统事件 */
    EVENT_INIT = 0,
    EVENT_RESET = 1,
    EVENT_SHUTDOWN = 2,
    
    /* 任务事件 */
    EVENT_JOB_SUBMITTED = 10,
    EVENT_JOB_START = 11,
    EVENT_JOB_PROGRESS = 12,
    EVENT_JOB_PAUSE = 13,
    EVENT_JOB_RESUME = 14,
    EVENT_JOB_CANCEL = 15,
    EVENT_JOB_COMPLETE = 16,
    
    /* 硬件事件 */
    EVENT_HARDWARE_ERROR = 20,
    EVENT_ERROR_RECOVERED = 21,
    EVENT_PAPER_LOW = 22,
    EVENT_TONER_LOW = 23,
    EVENT_TEMP_HIGH = 24,
    
    /* 用户事件 */
    EVENT_REFILL_PAPER = 30,
    EVENT_REFILL_TONER = 31,
    EVENT_CLEAR_ERROR = 32,
    EVENT_MAINTENANCE_MODE = 33,
    
    /* 定时器事件 */
    EVENT_TIMEOUT = 40,
    EVENT_CYCLE = 41,
} DriverEvent;

/* ==================== 状态转换规则 ==================== */
typedef struct {
    DriverState current_state;
    DriverEvent event;
    DriverState next_state;
    int (*action)(void* context);  /* 执行的动作函数 */
} StateTransition;

/* ==================== 状态机上下文 ==================== */
typedef struct {
    DriverState current_state;
    DriverState previous_state;
    
    /* 事件缓冲 */
    DriverEvent pending_events[16];
    int event_count;
    
    /* 事件处理时间 */
    uint32_t event_timestamp;
    uint32_t state_entry_time;
    
    /* 硬件设备指针 */
    void* printer_device;
    
    /* 统计信息 */
    uint32_t total_events_processed;
    uint32_t state_changes;
    uint32_t error_count;
} StateMachineContext;

/* ==================== 状态转换表 ==================== */
extern const StateTransition state_transitions[];
extern const int state_transitions_count;

/* ==================== 状态机接口 ==================== */

/**
 * 初始化状态机
 */
StateMachineContext* state_machine_init(void* printer_device);

/**
 * 释放状态机资源
 */
void state_machine_free(StateMachineContext* ctx);

/**
 * 发送事件到状态机
 */
int state_machine_send_event(StateMachineContext* ctx, DriverEvent event);

/**
 * 处理所有待处理事件
 */
int state_machine_process_events(StateMachineContext* ctx);

/**
 * 获取当前状态的字符串描述
 */
const char* state_to_string(DriverState state);

/**
 * 获取事件的字符串描述
 */
const char* event_to_string(DriverEvent event);

/**
 * 主状态机处理循环
 */
int state_machine_run_cycle(StateMachineContext* ctx);

/**
 * 获取当前状态
 */
DriverState state_machine_get_state(StateMachineContext* ctx);

/**
 * 强制转换到指定状态 (调试用)
 */
void state_machine_force_state(StateMachineContext* ctx, DriverState state);

#endif /* STATE_MACHINE_H */
