#include <stdio.h>
#include <assert.h>
#include <string.h>

#include "../driver/state_machine.h"
#include "../driver/printer_simulator.h"

// 辅助断言函数
void assert_state(StateMachineContext* ctx, DriverState expected, const char* step) {
    if (ctx->current_state != expected) {
        printf("[FAILED] %s: Expected %s but got %s\n", 
            step, state_to_string(expected), state_to_string(ctx->current_state));
        assert(ctx->current_state == expected);
    } else {
        printf("[OK] %s: Currently in %s\n", step, state_to_string(ctx->current_state));
    }
}

// 模拟发送事件并直接处理（为了同步验证）
void step_event(StateMachineContext* ctx, DriverEvent event) {
    printf("\n--> Sending Event: %s\n", event_to_string(event));
    state_machine_send_event(ctx, event);
    
    // state_machine_process_events 在原代码中会一次性处理队列。
    // 为了防止需要包含整个大系统，需要我们在这里调用
    // state_machine.c 会被链接，因此可以直接使用
    state_machine_process_events(ctx);
}

int main() {
    printf("===================================================\n");
    printf("        State Machine Implementation Test\n");
    printf("===================================================\n\n");

    // 假设一个打印机设备对象
    Printer dummy_printer;
    memset(&dummy_printer, 0, sizeof(Printer));
    
    StateMachineContext* ctx = state_machine_init(&dummy_printer);
    assert(ctx != NULL);
    
    assert_state(ctx, STATE_INIT, "Initial State");

    // 1. 初始化和重置流程 (INIT -> RESET -> IDLE)
    printf("\n--- Test Flow 1: Initialization ---\n");
    step_event(ctx, EVENT_INIT);
    assert_state(ctx, STATE_RESET, "After EVENT_INIT");
    
    step_event(ctx, EVENT_CYCLE);
    assert_state(ctx, STATE_IDLE, "After EVENT_CYCLE (Reset)");

    // 2. 正常打印流程 (IDLE -> PRINT_START -> PRINT_RUNNING -> PRINT_PAGE -> PRINT_RUNNING -> PRINT_FINISH -> IDLE)
    printf("\n--- Test Flow 2: Normal Printing ---\n");
    step_event(ctx, EVENT_JOB_SUBMITTED);
    assert_state(ctx, STATE_PRINT_START, "Job Submitted");

    step_event(ctx, EVENT_JOB_START);
    assert_state(ctx, STATE_PRINT_RUNNING, "Job Started");

    step_event(ctx, EVENT_JOB_PROGRESS);
    assert_state(ctx, STATE_PRINT_PAGE, "Printing Page");

    step_event(ctx, EVENT_JOB_PROGRESS);
    assert_state(ctx, STATE_PRINT_RUNNING, "Back to Running");

    step_event(ctx, EVENT_JOB_COMPLETE);
    assert_state(ctx, STATE_PRINT_FINISH, "Job Completed");

    step_event(ctx, EVENT_CYCLE);
    assert_state(ctx, STATE_IDLE, "Back to IDLE after Finish");

    // 3. 打印暂停恢复流程 (IDLE ... PRINT_RUNNING -> PRINT_PAUSE -> PRINT_RESUME -> PRINT_RUNNING ...)
    printf("\n--- Test Flow 3: Pause & Resume ---\n");
    step_event(ctx, EVENT_JOB_SUBMITTED);
    step_event(ctx, EVENT_JOB_START);
    assert_state(ctx, STATE_PRINT_RUNNING, "Setup for Pause test");
    
    step_event(ctx, EVENT_JOB_PAUSE);
    assert_state(ctx, STATE_PRINT_PAUSE, "Job Paused");
    
    step_event(ctx, EVENT_JOB_RESUME);
    assert_state(ctx, STATE_PRINT_RESUME, "Job Resumed");
    
    step_event(ctx, EVENT_JOB_PROGRESS);
    assert_state(ctx, STATE_PRINT_RUNNING, "Back to Running after Resume");
    
    step_event(ctx, EVENT_JOB_CANCEL);
    assert_state(ctx, STATE_IDLE, "Job Cancelled to return to IDLE (Bonus transition in code)");

    // 4. 维护流程 (IDLE -> MAINTENANCE -> IDLE)
    printf("\n--- Test Flow 4: Maintenance ---\n");
    step_event(ctx, EVENT_REFILL_PAPER);
    assert_state(ctx, STATE_MAINTENANCE, "Paper Refill initiates Maintenance");
    
    step_event(ctx, EVENT_CYCLE);
    assert_state(ctx, STATE_IDLE, "Back to IDLE after Maintenance");

    // 5. 错误处理流程 (IDLE -> ERROR -> ERROR_RECOVERY -> IDLE)
    printf("\n--- Test Flow 5: Error Handling ---\n");
    step_event(ctx, EVENT_HARDWARE_ERROR);
    assert_state(ctx, STATE_ERROR, "Hardware Error triggered");
    
    step_event(ctx, EVENT_CLEAR_ERROR);
    assert_state(ctx, STATE_ERROR_RECOVERY, "Error Cleared, in Recovery");
    
    step_event(ctx, EVENT_CYCLE);
    assert_state(ctx, STATE_IDLE, "Recovered to IDLE");

    // 6. 运行状态下的硬件错误
    printf("\n--- Test Flow 6: Error while Running ---\n");
    step_event(ctx, EVENT_JOB_SUBMITTED);
    step_event(ctx, EVENT_JOB_START);
    assert_state(ctx, STATE_PRINT_RUNNING, "Setup for Running Error test");
    
    step_event(ctx, EVENT_HARDWARE_ERROR);
    assert_state(ctx, STATE_ERROR, "Error during printing transitions directly to ERROR state");
    
    step_event(ctx, EVENT_CLEAR_ERROR);
    step_event(ctx, EVENT_CYCLE);
    assert_state(ctx, STATE_IDLE, "Recovered to IDLE back again");

    printf("\n===================================================\n");
    printf("        ALL TESTS PASSED SUCCESSFULLY!\n");
    printf("===================================================\n");
    
    state_machine_free(ctx);
    return 0;
}
