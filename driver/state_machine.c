/**
 * state_machine.c
 * 打印机驱动状态机实现
 */

#include "state_machine.h"
#include "printer_simulator.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <time.h>

/* ==================== 状态转换动作函数 ==================== */

/* 初始化打印机 */
static int action_init_printer(void* context) {
    printf("[STATE_MACHINE] 初始化打印机...\n");
    return 0;
}

/* 检查硬件错误 */
static int action_check_hardware(void* context) {
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    if (printer && printer->error != HARDWARE_OK) {
        state_machine_send_event(ctx, EVENT_HARDWARE_ERROR);
    }
    return 0;
}

/* 启动打印 */
static int action_start_print(void* context) {
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    printf("[STATE_MACHINE] 开始打印任务...\n");
    if (printer) {
        printer->status = PRINTER_PRINTING;
    }
    return 0;
}

/* 处理打印页面 */
static int action_process_page(void* context) {
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    if (printer && printer->current_task) {
        printer->current_task->printed_pages++;
        printf("[STATE_MACHINE] 已打印 %d/%d 页\n", 
               printer->current_task->printed_pages,
               printer->current_task->page_count);
    }
    return 0;
}

/* 暂停打印 */
static int action_pause_print(void* context) {
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    printf("[STATE_MACHINE] 暂停打印...\n");
    if (printer) {
        printer->status = PRINTER_PAUSED;
    }
    return 0;
}

/* 恢复打印 */
static int action_resume_print(void* context) {
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    printf("[STATE_MACHINE] 恢复打印...\n");
    if (printer) {
        printer->status = PRINTER_PRINTING;
    }
    return 0;
}

/* 完成打印 */
static int action_finish_print(void* context) {
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    printf("[STATE_MACHINE] 完成打印任务\n");
    if (printer) {
        printer->status = PRINTER_IDLE;
        if (printer->current_task) {
            printer->page_count += printer->current_task->printed_pages;
        }
    }
    return 0;
}

/* 处理错误 */
static int action_handle_error(void* context) {
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    ctx->error_count++;
    printf("[STATE_MACHINE] 处理硬件错误 (计数: %d)\n", ctx->error_count);
    if (printer) {
        printer->status = PRINTER_ERROR;
    }
    return 0;
}

/* 恢复错误 */
static int action_recover_error(void* context) {
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    printf("[STATE_MACHINE] 错误已恢复\n");
    if (printer) {
        printer->error = HARDWARE_OK;
        printer->status = PRINTER_IDLE;
    }
    return 0;
}

/* 补充纸张 */
static int action_refill_paper(void* context) {
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    printf("[STATE_MACHINE] 补充纸张\n");
    if (printer) {
        printer->paper_pages = printer->paper_max;
    }
    return 0;
}

/* 补充碳粉 */
static int action_refill_toner(void* context) {
    printf("[STATE_MACHINE] 补充碳粉\n");
    StateMachineContext* ctx = (StateMachineContext*)context;
    Printer* printer = (Printer*)ctx->printer_device;
    if (printer) {
        printer->toner_percentage = 100;
    }
    return 0;
}

/* ==================== 状态转换表 ==================== */
const StateTransition state_transitions[] = {
    /* 初始化流程 */
    { STATE_INIT,              EVENT_INIT,              STATE_RESET,           action_init_printer },
    { STATE_RESET,             EVENT_CYCLE,            STATE_IDLE,            NULL },
    
    /* 空闲状态 */
    { STATE_IDLE,              EVENT_JOB_SUBMITTED,    STATE_PRINT_START,     action_check_hardware },
    { STATE_IDLE,              EVENT_HARDWARE_ERROR,   STATE_ERROR,           action_handle_error },
    { STATE_IDLE,              EVENT_REFILL_PAPER,     STATE_MAINTENANCE,     action_refill_paper },
    { STATE_IDLE,              EVENT_REFILL_TONER,     STATE_MAINTENANCE,     action_refill_toner },
    
    /* 维护状态 */
    { STATE_MAINTENANCE,       EVENT_CYCLE,            STATE_IDLE,            NULL },
    { STATE_MAINTENANCE,       EVENT_HARDWARE_ERROR,   STATE_ERROR,           action_handle_error },
    
    /* 打印流程 */
    { STATE_PRINT_START,       EVENT_JOB_START,        STATE_PRINT_RUNNING,   action_start_print },
    { STATE_PRINT_START,       EVENT_HARDWARE_ERROR,   STATE_ERROR,           action_handle_error },
    
    { STATE_PRINT_RUNNING,     EVENT_JOB_PROGRESS,     STATE_PRINT_PAGE,      action_process_page },
    { STATE_PRINT_RUNNING,     EVENT_JOB_PAUSE,        STATE_PRINT_PAUSE,     action_pause_print },
    { STATE_PRINT_RUNNING,     EVENT_JOB_COMPLETE,     STATE_PRINT_FINISH,    action_finish_print },
    { STATE_PRINT_RUNNING,     EVENT_HARDWARE_ERROR,   STATE_ERROR,           action_handle_error },
    { STATE_PRINT_RUNNING,     EVENT_JOB_CANCEL,       STATE_IDLE,            NULL },
    
    { STATE_PRINT_PAGE,        EVENT_JOB_PROGRESS,     STATE_PRINT_RUNNING,   NULL },
    { STATE_PRINT_PAGE,        EVENT_JOB_COMPLETE,     STATE_PRINT_FINISH,    action_finish_print },
    { STATE_PRINT_PAGE,        EVENT_HARDWARE_ERROR,   STATE_ERROR,           action_handle_error },
    
    { STATE_PRINT_PAUSE,       EVENT_JOB_RESUME,       STATE_PRINT_RESUME,    action_resume_print },
    { STATE_PRINT_PAUSE,       EVENT_JOB_CANCEL,       STATE_IDLE,            NULL },
    { STATE_PRINT_PAUSE,       EVENT_HARDWARE_ERROR,   STATE_ERROR,           action_handle_error },
    
    { STATE_PRINT_RESUME,      EVENT_JOB_PROGRESS,     STATE_PRINT_RUNNING,   NULL },
    
    { STATE_PRINT_FINISH,      EVENT_CYCLE,            STATE_IDLE,            NULL },
    
    /* 错误处理 */
    { STATE_ERROR,             EVENT_CLEAR_ERROR,      STATE_ERROR_RECOVERY,  action_recover_error },
    { STATE_ERROR,             EVENT_REFILL_PAPER,     STATE_MAINTENANCE,     action_refill_paper },
    { STATE_ERROR,             EVENT_REFILL_TONER,     STATE_MAINTENANCE,     action_refill_toner },
    { STATE_ERROR_RECOVERY,    EVENT_CYCLE,            STATE_IDLE,            NULL },
};

const int state_transitions_count = sizeof(state_transitions) / sizeof(state_transitions[0]);

/* ==================== 辅助函数 ==================== */
const char* state_to_string(DriverState state) {
    switch (state) {
        case STATE_INIT: return "INIT";
        case STATE_RESET: return "RESET";
        case STATE_IDLE: return "IDLE";
        case STATE_WARMING_UP: return "WARMING_UP";
        case STATE_PRINT_START: return "PRINT_START";
        case STATE_PRINT_RUNNING: return "PRINT_RUNNING";
        case STATE_PRINT_PAGE: return "PRINT_PAGE";
        case STATE_PRINT_PAUSE: return "PRINT_PAUSE";
        case STATE_PRINT_RESUME: return "PRINT_RESUME";
        case STATE_PRINT_FINISH: return "PRINT_FINISH";
        case STATE_ERROR: return "ERROR";
        case STATE_ERROR_RECOVERY: return "ERROR_RECOVERY";
        case STATE_ERROR_HANDLED: return "ERROR_HANDLED";
        case STATE_MAINTENANCE: return "MAINTENANCE";
        case STATE_OFFLINE: return "OFFLINE";
        case STATE_TERMINATING: return "TERMINATING";
        default: return "UNKNOWN";
    }
}

const char* event_to_string(DriverEvent event) {
    switch (event) {
        case EVENT_INIT: return "INIT";
        case EVENT_RESET: return "RESET";
        case EVENT_SHUTDOWN: return "SHUTDOWN";
        case EVENT_JOB_SUBMITTED: return "JOB_SUBMITTED";
        case EVENT_JOB_START: return "JOB_START";
        case EVENT_JOB_PROGRESS: return "JOB_PROGRESS";
        case EVENT_JOB_PAUSE: return "JOB_PAUSE";
        case EVENT_JOB_RESUME: return "JOB_RESUME";
        case EVENT_JOB_CANCEL: return "JOB_CANCEL";
        case EVENT_JOB_COMPLETE: return "JOB_COMPLETE";
        case EVENT_HARDWARE_ERROR: return "HARDWARE_ERROR";
        case EVENT_ERROR_RECOVERED: return "ERROR_RECOVERED";
        case EVENT_PAPER_LOW: return "PAPER_LOW";
        case EVENT_TONER_LOW: return "TONER_LOW";
        case EVENT_TEMP_HIGH: return "TEMP_HIGH";
        case EVENT_REFILL_PAPER: return "REFILL_PAPER";
        case EVENT_REFILL_TONER: return "REFILL_TONER";
        case EVENT_CLEAR_ERROR: return "CLEAR_ERROR";
        case EVENT_MAINTENANCE_MODE: return "MAINTENANCE_MODE";
        case EVENT_TIMEOUT: return "TIMEOUT";
        case EVENT_CYCLE: return "CYCLE";
        default: return "UNKNOWN";
    }
}

/* ==================== 状态机实现 ==================== */

StateMachineContext* state_machine_init(void* printer_device) {
    StateMachineContext* ctx = (StateMachineContext*)malloc(sizeof(StateMachineContext));
    if (ctx == NULL) return NULL;
    
    memset(ctx, 0, sizeof(StateMachineContext));
    ctx->current_state = STATE_INIT;
    ctx->previous_state = STATE_INIT;
    ctx->printer_device = printer_device;
    ctx->state_entry_time = (uint32_t)time(NULL);
    
    return ctx;
}

void state_machine_free(StateMachineContext* ctx) {
    if (ctx != NULL) {
        free(ctx);
    }
}

int state_machine_send_event(StateMachineContext* ctx, DriverEvent event) {
    if (ctx == NULL) return -1;
    if (ctx->event_count >= 16) return -1;  /* 事件缓冲满 */
    
    ctx->pending_events[ctx->event_count++] = event;
    return 0;
}

int state_machine_process_events(StateMachineContext* ctx) {
    if (ctx == NULL) return -1;
    
    int processed = 0;
    while (ctx->event_count > 0) {
        DriverEvent event = ctx->pending_events[0];
        
        /* 移除已处理的事件 */
        for (int i = 0; i < ctx->event_count - 1; i++) {
            ctx->pending_events[i] = ctx->pending_events[i + 1];
        }
        ctx->event_count--;
        
        /* 查找状态转换 */
        for (int i = 0; i < state_transitions_count; i++) {
            if (state_transitions[i].current_state == ctx->current_state &&
                state_transitions[i].event == event) {
                
                printf("[STATE_MACHINE] %s + %s -> %s\n",
                       state_to_string(ctx->current_state),
                       event_to_string(event),
                       state_to_string(state_transitions[i].next_state));
                
                /* 执行转换动作 */
                if (state_transitions[i].action != NULL) {
                    state_transitions[i].action(ctx);
                }
                
                /* 更新状态 */
                ctx->previous_state = ctx->current_state;
                ctx->current_state = state_transitions[i].next_state;
                ctx->state_entry_time = (uint32_t)time(NULL);
                ctx->state_changes++;
                processed++;
                
                break;
            }
        }
        
        ctx->total_events_processed++;
    }
    
    return processed;
}

DriverState state_machine_get_state(StateMachineContext* ctx) {
    if (ctx == NULL) return STATE_INIT;
    return ctx->current_state;
}

void state_machine_force_state(StateMachineContext* ctx, DriverState state) {
    if (ctx != NULL) {
        ctx->previous_state = ctx->current_state;
        ctx->current_state = state;
        ctx->state_entry_time = (uint32_t)time(NULL);
    }
}

int state_machine_run_cycle(StateMachineContext* ctx) {
    if (ctx == NULL) return -1;
    
    /* 发送定期事件 */
    state_machine_send_event(ctx, EVENT_CYCLE);
    
    /* 处理所有待处理事件 */
    return state_machine_process_events(ctx);
}
