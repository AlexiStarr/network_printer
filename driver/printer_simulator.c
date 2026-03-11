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
    printer->paper_pages = 0;     /* 初始纸张为空，需要管理员主动补充 */
    printer->paper_max = 500;     /* 纸张最大容量 */
    printer->toner_percentage = 100;

    /* 初始化硬件状态 */
    printer->status = PRINTER_IDLE;
    printer->error = HARDWARE_OK;
    printer->temperature = 25;
    printer->temperature_max = 100;
    printer->page_count = 0;

    /* 初始化打印队列 */
    memset(printer->queue, 0, sizeof(printer->queue));
    printer->queue_size = 0;
    printer->next_task_id = 1;

    /* 初始化当前任务 */
    printer->current_task = NULL;
    printer->print_speed = 20; /* 20 页/分钟 */
    
    /* 初始化温度管理 */
    printer->active_cycles = 0;

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
            printer->temperature = 80; /* 加热到80℃ */
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
                printer->temperature = 25; /* 冷却回常温 */
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
    
    /* 管理温度: 根据活跃任务数和空闲时间动态调整 */
    int active_tasks = printer_get_active_task_count(printer);
    
    if (printer->current_task == NULL) {
        /* 没有在打印：温度逐步冷却 */
        if (printer->temperature > 25) {
            printer->temperature -= 2;
            if (printer->temperature < 25) printer->temperature = 25;
        }
        printer->active_cycles = 0;
    } else {
        /* 正在打印：温度基于队列长度动态变化 */
        printer->active_cycles++;
        
        /* 队列越长，温度越高（加快打印速度） */
        int queue_pressure = 50 + (active_tasks * 10);  /* 50-100℃ */
        if (queue_pressure > 85) queue_pressure = 85;
        
        if (printer->temperature < queue_pressure) {
            printer->temperature += 3;
        } else if (printer->temperature > queue_pressure) {
            printer->temperature -= 1;
        }
        
        /* 确保不超过最大温度 */
        if (printer->temperature > printer->temperature_max) {
            printer->temperature = printer->temperature_max;
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
        if (pos > (int)strlen("{\"tasks\":[")) {
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
        /* 检查是否超过最大值 */
        int new_pages = printer->paper_pages + pages;
        if (new_pages > printer->paper_max) {
            printf("[PRINTER] 警告：纸仓已满，请稍后再加吧~ (当前:%d, 最大:%d)\n", 
                   printer->paper_pages, printer->paper_max);
            printer->paper_pages = printer->paper_max;  /* 限制到最大值 */
            return;
        }
        
        printer->paper_pages += pages;
        printf("[PRINTER] 补充纸张 %d 页，当前库存：%d/%d\n", 
               pages, printer->paper_pages, printer->paper_max);
        
        /* 如果之前因为缺纸报错，现在清除 */
        if (printer->error == ERROR_PAPER_EMPTY) {
            printer->error = HARDWARE_OK;
            if (printer->status == PRINTER_ERROR) {
                printer->status = PRINTER_IDLE;
            }
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
        
        /* 【改进】根据错误类型改变硬件状态，而不只是记录错误 */
        switch (error) {
            case ERROR_PAPER_EMPTY:
                /* 缺纸：纸张清零 */
                printer->paper_pages = 0;
                printf("[PRINTER] 模拟缺纸错误 - 纸张已清零\n");
                break;
                
            case ERROR_TONER_EMPTY:
                /* 缺碳粉：碳粉清零 */
                printer->toner_percentage = 0;
                printf("[PRINTER] 模拟缺碳粉错误 - 碳粉已清零\n");
                break;
                
            case ERROR_TONER_LOW:
                /* 碳粉不足：设置为5% */
                printer->toner_percentage = 5;
                printf("[PRINTER] 模拟碳粉不足错误 - 碳粉设置为5%%\n");
                break;
                
            case ERROR_HEAT_UNAVAILABLE:
                /* 加热器故障：温度设置为常温 */
                printer->temperature = 25;
                printf("[PRINTER] 模拟加热器故障 - 温度已重置\n");
                break;
                
            case ERROR_MOTOR_FAILURE:
                /* 电机故障：暂停当前任务 */
                if (printer->current_task != NULL) {
                    printer->current_task->status = PRINTER_PAUSED;
                }
                printf("[PRINTER] 模拟电机故障 - 已暂停当前任务\n");
                break;
                
            case ERROR_SENSOR_FAILURE:
                /* 传感器故障：设置为离线状态 */
                printer->status = PRINTER_OFFLINE;
                printf("[PRINTER] 模拟传感器故障 - 打印机离线\n");
                break;
                
            default:
                printf("[PRINTER] 模拟了未知硬件错误\n");
        }
    }
}

/* 设置纸张最大容量 */
void printer_set_paper_max(Printer* printer, int max_pages) {
    if (printer != NULL) {
        if (max_pages <= 0) {
            printf("[PRINTER] 错误：纸张最大值必须 > 0\n");
            return;
        }
        
        printer->paper_max = max_pages;
        
        /* 如果当前纸张超过最大值，进行调整 */
        if (printer->paper_pages > max_pages) {
            printf("[PRINTER] 警告：纸张已清零 - 纸仓已满，请稍后再加吧~\n");
            printer->paper_pages = max_pages;  /* 限制到最大值 */
            printer->error = ERROR_PAPER_EMPTY;
            printer->status = PRINTER_ERROR;
        }
    }
}

/* 获取打印队列中的活跃任务数 */
int printer_get_active_task_count(const Printer* printer) {
    if (printer == NULL) return 0;
    
    int count = 0;
    
    /* 计算当前正在打印的任务 */
    if (printer->current_task != NULL) {
        count++;
    }
    
    /* 计算队列中的任务 */
    count += printer->queue_size;
    
    return count;
}