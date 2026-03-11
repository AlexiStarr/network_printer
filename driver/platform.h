/**
 * platform.h
 * 平台抽象层 - 处理Windows/Linux跨平台兼容性
 * 不修改原有功能，仅添加条件编译
 */

#ifndef PLATFORM_H
#define PLATFORM_H

#include <string.h>
#include <time.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

/* ==================== 平台检测 ==================== */
#ifdef _WIN32
    #define PLATFORM_WINDOWS 1
    #define PLATFORM_LINUX 0
#else
    #define PLATFORM_WINDOWS 0
    #define PLATFORM_LINUX 1
#endif

/* ==================== Windows 平台 ==================== */
#ifdef _WIN32
    #include <winsock2.h>
    #include <ws2tcpip.h>
    #include <windows.h>
    #include <process.h>
    
    #ifdef _MSC_VER
        #pragma comment(lib, "ws2_32.lib")
    #else
        /* MinGW/GCC: 链接 ws2_32 库通过命令行 -lws2_32 */
        #pragma GCC diagnostic ignored "-Wunknown-pragmas"
    #endif
    
    /* Socket 类型定义 */
    typedef int socklen_t;
    typedef HANDLE thread_t;
    typedef DWORD thread_id_t;
    
    /* 线程创建宏 */
    typedef unsigned int (__stdcall *thread_func_t)(void*);
    #define thread_create(thread_ptr, func, arg) \
        *(thread_ptr) = (HANDLE)_beginthreadex(NULL, 0, (thread_func_t)(func), (arg), 0, NULL); \
        (*(thread_ptr) != NULL ? 0 : -1)
    #define thread_join(thread) WaitForSingleObject((thread), INFINITE); CloseHandle((thread))
    #define thread_detach(thread) CloseHandle((thread))
    #define thread_exit(code) _endthreadex(code)
    
    /* 互斥锁 */
    typedef CRITICAL_SECTION mutex_t;
    #define mutex_init(m) InitializeCriticalSection(m)
    #define mutex_lock(m) EnterCriticalSection(m)
    #define mutex_unlock(m) LeaveCriticalSection(m)
    #define mutex_destroy(m) DeleteCriticalSection(m)
    
    /* 信号处理 */
    #define signal_set_handler(sig, handler) signal(sig, handler)
    
    /* 睡眠 */
    #define sleep_ms(ms) Sleep(ms)
    #define sleep_sec(sec) Sleep((sec) * 1000)
    
    /* 环境初始化 */
    static inline int platform_init(void) {
        WSADATA wsa_data;
        int res = WSAStartup(MAKEWORD(2, 2), &wsa_data);
        if (res != 0) {
            fprintf(stderr, "WSAStartup 失败: %d\n", res);
            return -1;
        }
        return 0;
    }
    
    static inline void platform_cleanup(void) {
        WSACleanup();
    }
    
    /* 关闭 Socket */
    #define closesocket_safe(sock) \
        if ((sock) != INVALID_SOCKET) { \
            closesocket((sock)); \
            (sock) = INVALID_SOCKET; \
        }
    
    #define INVALID_SOCK INVALID_SOCKET
    #define SOCK_ERROR SOCKET_ERROR


/* ==================== Linux/macOS 平台 ==================== */
#else
    #include <unistd.h>
    #include <sys/socket.h>
    #include <netinet/in.h>
    #include <arpa/inet.h>
    #include <pthread.h>
    #include <signal.h>
    #include <errno.h>
    #include <fcntl.h>
    
    /* Socket 类型定义 */
    typedef int SOCKET;
    typedef pthread_t thread_t;
    typedef pthread_t thread_id_t;
    
    /* 线程创建宏 */
    #define thread_create(thread_ptr, func, arg) pthread_create(thread_ptr, NULL, func, arg)
    #define thread_join(thread) pthread_join((thread), NULL)
    #define thread_detach(thread) pthread_detach((thread))
    #define thread_exit(code) pthread_exit((void*)(intptr_t)(code))
    
    /* 互斥锁 */
    typedef pthread_mutex_t mutex_t;
    #define mutex_init(m) pthread_mutex_init(m, NULL)
    #define mutex_lock(m) pthread_mutex_lock(m)
    #define mutex_unlock(m) pthread_mutex_unlock(m)
    #define mutex_destroy(m) pthread_mutex_destroy(m)
    
    /* 睡眠 */
    #define sleep_ms(ms) usleep((ms) * 1000)
    #define sleep_sec(sec) sleep(sec)
    
    /* 环境初始化 */
    #define platform_init() 0
    #define platform_cleanup()
    
    /* 关闭 Socket */
    #define closesocket_safe(sock) \
        if ((sock) >= 0) { \
            close((sock)); \
            (sock) = -1; \
        }
    
    #define INVALID_SOCK -1
    #define INVALID_SOCKET (-1)    /* 新增：与非Windows平台兼容 */
    #define SOCK_ERROR -1
    #define SOCKET_ERROR -1
    #define closesocket close

#endif

/* ==================== 通用函数 ==================== */

/**
 * 获取错误字符串
 */
static inline const char* get_socket_error_msg(void) {
#ifdef _WIN32
    static char msg[256];
    int err = WSAGetLastError();
    FormatMessageA(FORMAT_MESSAGE_FROM_SYSTEM, NULL, err, 0, msg, sizeof(msg), NULL);
    return msg;
#else
    return strerror(errno);
#endif
}

/**
 * 设置 Socket 为非阻塞模式
 */
static inline int set_nonblocking(SOCKET sock) {
#ifdef _WIN32
    unsigned long mode = 1;
    return ioctlsocket(sock, FIONBIO, &mode) == NO_ERROR ? 0 : -1;
#else
    int flags = fcntl(sock, F_GETFL, 0);
    return fcntl(sock, F_SETFL, flags | O_NONBLOCK) == 0 ? 0 : -1;
#endif
}

#endif /* PLATFORM_H */