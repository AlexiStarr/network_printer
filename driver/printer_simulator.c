/**
 * printer_simulator.c
 * 网络打印机硬件模拟器实现
 */

#include "printer_simulator.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <time.h>

/* 初始化打印机 */
Printer* printer_init() {
    Printer* printer = (Printer*)malloc(sizeof(Printer));
    if (printer == NULL) return NULL;

    /* 初始化基本信息 */
    strcpy(printer->model, "NetworkPrinter Pro X200");
    strcpy(printer->serial_number, "NP-X200-2024001");
    strcpy(printer->firmware_version, "3.2.1");

    /* 初始化耗材 */
    printer->paper_pages = 500;
    printer->toner_percentage = 100;

    /* 初始化硬件状态 */
    printer->status = PRINTER_IDLE;
    printer->error = HARDWARE_OK;
    printer->temperature = 25;
    printer->page_count = 0;

    /* 初始化打印队列 */
    memset(printer->queue, 0, sizeof(printer->queue));
    printer->queue_size = 0;
    printer->next_task_id = 1;

    /* 初始化当前任务 */
    printer->current_task = NULL;
    printer->print_speed = 20; /* 20 页/分钟 */

    return printer;
}

/* 释放打印机资源 */
void printer_free(Printer* printer) {
    if (printer != NULL) {
        free(printer);
    }
}

/* 提交打印任务 */
int printer_submit_job(Printer* printer, const char* filename, int page_count) {
    if (printer == NULL || page_count <= 0) {
        return -1;
    }

    if (printer->queue_size >= 100) {
        return -1; /* 队列满 */
    }

    /* 检查错误状态 */
    if (printer->error != HARDWARE_OK) {
        return -1; /* 硬件错误 */
    }

    PrintTask* task = &printer->queue[printer->queue_size];
    task->task_id = printer->next_task_id;
    strncpy(task->filename, filename, sizeof(task->filename) - 1);
    task->page_count = page_count;
    task->printed_pages = 0;
    task->submit_time = time(NULL);
    task->start_time = 0;
    task->status = PRINTER_IDLE;

    printer->queue_size++;
    int task_id = printer->next_task_id;
    printer->next_task_id++;

    return task_id;
}

/* 取消打印任务 */
int printer_cancel_job(Printer* printer, int task_id) {
    if (printer == NULL) return -1;

    for (int i = 0; i < printer->queue_size; i++) {
        if (printer->queue[i].task_id == task_id) {
            /* 不允许取消正在打印的任务 */
            if (printer->current_task && printer->current_task->task_id == task_id) {
                return -1;
            }
            /* 移除任务 */
            for (int j = i; j < printer->queue_size - 1; j++) {
                printer->queue[j] = printer->queue[j + 1];
            }
            printer->queue_size--;
            return 0;
        }
    }

    return -1;
}

/* 暂停打印任务 */
int printer_pause_job(Printer* printer, int task_id) {
    if (printer == NULL) return -1;

    /* 如果是当前正在打印的任务，暂停它 */
    if (printer->current_task && printer->current_task->task_id == task_id) {
        printer->current_task->status = PRINTER_PAUSED;
        return 0;
    }

    /* 检查队列中的任务 */
    for (int i = 0; i < printer->queue_size; i++) {
        if (printer->queue[i].task_id == task_id) {
            printer->queue[i].status = PRINTER_PAUSED;
            return 0;
        }
    }

    return -1; /* 任务不存在 */
}

/* 恢复打印任务 */
int printer_resume_job(Printer* printer, int task_id) {
    if (printer == NULL) return -1;

    /* 如果是当前正在打印的任务，恢复它 */
    if (printer->current_task && printer->current_task->task_id == task_id) {
        printer->current_task->status = PRINTER_IDLE; /* 恢复到空闲状态，等待打印 */
        return 0;
    }

    /* 检查队列中的任务 */
    for (int i = 0; i < printer->queue_size; i++) {
        if (printer->queue[i].task_id == task_id) {
            printer->queue[i].status = PRINTER_IDLE;
            return 0;
        }
    }

    return -1; /* 任务不存在 */
}

/* 处理一个打印周期 */
void printer_process_cycle(Printer* printer) {
    if (printer == NULL) return;

    /* 检查纸张和碳粉 */
    if (printer->paper_pages <= 0) {
        printer->error = ERROR_PAPER_EMPTY;
        printer->status = PRINTER_ERROR;
        return;
    }

    if (printer->toner_percentage <= 0) {
        printer->error = ERROR_TONER_EMPTY;
        printer->status = PRINTER_ERROR;
        return;
    }

    if (printer->toner_percentage < 10) {
        printer->error = ERROR_TONER_LOW;
        printer->status = PRINTER_ERROR;
        return;
    }

    /* 当前有打印任务 */
    if (printer->current_task != NULL) {
        PrintTask* task = printer->current_task;

        /* Fix Bug 3: 当前任务暂停时跳过本周期，不打印 */
        if (task->status == PRINTER_PAUSED) {
            printer->status = PRINTER_IDLE;
            return;
        }

        if (task->start_time == 0) {
            task->start_time = time(NULL);
            task->status = PRINTER_PRINTING;
            printer->status = PRINTER_PRINTING;
            printer->temperature = 180; /* 加热 */
        }

        /* 模拟打印（每个周期打印一页） */
        if (task->printed_pages < task->page_count) {
            task->printed_pages++;
            printer->page_count++;
            printer->paper_pages--;
            /* Fix Bug 2: 原 (80+rand()%40)/100 整数除法恒为0，碳粉永不减少
               改为每页消耗 1% */
            printer->toner_percentage -= 1;

            if (printer->toner_percentage < 0) {
                printer->toner_percentage = 0;
            }

            /* 打印完成 */
            if (task->printed_pages >= task->page_count) {
                task->status = PRINTER_IDLE;
                printer->current_task = NULL;
                printer->status = PRINTER_IDLE;
                printer->temperature = 25; /* 冷却 */
            }
        }
    } else {
        /* Fix Bug 4: 从队列中取下一个【非暂停】任务 */
        for (int i = 0; i < printer->queue_size; i++) {
            if (printer->queue[i].status != PRINTER_PAUSED) {
                printer->current_task = &printer->queue[i];
                /* 将选中任务从队列中移除 */
                for (int j = i; j < printer->queue_size - 1; j++) {
                    printer->queue[j] = printer->queue[j + 1];
                }
                printer->queue_size--;
                break;
            }
        }
    }
}

/* 获取打印机状态信息（JSON 格式） */
void printer_get_status(Printer* printer, char* status_json, int buffer_size) {
    if (printer == NULL || status_json == NULL) return;

    const char* error_str;
    switch (printer->error) {
        case HARDWARE_OK:
            error_str = "OK";
            break;
        case ERROR_PAPER_EMPTY:
            error_str = "PAPER_EMPTY";
            break;
        case ERROR_TONER_LOW:
            error_str = "TONER_LOW";
            break;
        case ERROR_TONER_EMPTY:
            error_str = "TONER_EMPTY";
            break;
        case ERROR_HEAT_UNAVAILABLE:
            error_str = "HEAT_UNAVAILABLE";
            break;
        case ERROR_MOTOR_FAILURE:
            error_str = "MOTOR_FAILURE";
            break;
        case ERROR_SENSOR_FAILURE:
            error_str = "SENSOR_FAILURE";
            break;
        default:
            error_str = "UNKNOWN";
    }

    const char* status_str;
    switch (printer->status) {
        case PRINTER_IDLE:
            status_str = "idle";
            break;
        case PRINTER_PRINTING:
            status_str = "printing";
            break;
        case PRINTER_ERROR:
            status_str = "error";
            break;
        case PRINTER_OFFLINE:
            status_str = "offline";
            break;
        default:
            status_str = "unknown";
    }

    snprintf(status_json, buffer_size,
        "{"
        "\"model\":\"%s\","
        "\"serial_number\":\"%s\","
        "\"firmware_version\":\"%s\","
        "\"status\":\"%s\","
        "\"error\":\"%s\","
        "\"temperature\":%d,"
        "\"page_count\":%d,"
        "\"paper_pages\":%d,"
        "\"toner_percentage\":%d"
        "}",
        printer->model,
        printer->serial_number,
        printer->firmware_version,
        status_str,
        error_str,
        printer->temperature,
        printer->page_count,
        printer->paper_pages,
        printer->toner_percentage
    );
}

/* 获取打印队列信息 */
void printer_get_queue(Printer* printer, char* queue_json, int buffer_size) {
    if (printer == NULL || queue_json == NULL) return;

    int pos = 0;
    pos += snprintf(queue_json + pos, buffer_size - pos, "{\"tasks\":[");

    if (printer->current_task != NULL) {
        PrintTask* task = printer->current_task;
        pos += snprintf(queue_json + pos, buffer_size - pos,
            "{\"task_id\":%d,\"filename\":\"%s\",\"page_count\":%d,\"printed_pages\":%d,\"status\":\"printing\"}",
            task->task_id, task->filename, task->page_count, task->printed_pages
        );
    }

    for (int i = 0; i < printer->queue_size; i++) {
        if (pos > strlen("{\"tasks\":[")) {
            pos += snprintf(queue_json + pos, buffer_size - pos, ",");
        }
        PrintTask* task = &printer->queue[i];
        pos += snprintf(queue_json + pos, buffer_size - pos,
            "{\"task_id\":%d,\"filename\":\"%s\",\"page_count\":%d,\"printed_pages\":%d,\"status\":\"queued\"}",
            task->task_id, task->filename, task->page_count, task->printed_pages
        );
    }

    pos += snprintf(queue_json + pos, buffer_size - pos, "],\"queue_size\":%d}", printer->queue_size);
}

/* 补充纸张 */
void printer_refill_paper(Printer* printer, int pages) {
    if (printer != NULL) {
        printer->paper_pages += pages;
        if (printer->paper_pages > 5000) {
            printer->paper_pages = 5000; /* 最多 5000 页 */
        }
    }
}

/* 补充碳粉 */
void printer_refill_toner(Printer* printer) {
    if (printer != NULL) {
        printer->toner_percentage = 100;
        if (printer->error == ERROR_TONER_EMPTY || printer->error == ERROR_TONER_LOW) {
            printer->error = HARDWARE_OK;
            printer->status = PRINTER_IDLE;
        }
    }
}

/* 清除硬件错误 */
void printer_clear_error(Printer* printer) {
    if (printer != NULL) {
        printer->error = HARDWARE_OK;
        if (printer->status == PRINTER_ERROR) {
            printer->status = PRINTER_IDLE;
        }
    }
}

/* 模拟硬件故障 */
void printer_simulate_error(Printer* printer, HardwareError error) {
    if (printer != NULL) {
        printer->error = error;
        printer->status = PRINTER_ERROR;
    }
}