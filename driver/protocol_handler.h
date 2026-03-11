/**
 * protocol_handler.h
 * 二进制协议处理器头文件
 */

#ifndef PROTOCOL_HANDLER_H
#define PROTOCOL_HANDLER_H

#include "protocol.h"
#include "printer_simulator.h"
#include <stdint.h>
#include <stddef.h>

/**
 * 处理协议请求的主入口函数
 * 
 * @param printer: 打印机对象指针
 * @param request_packet: 完整的二进制请求数据包
 * @param request_len: 请求数据包长度
 * @param response_buf: 响应缓冲区
 * @param response_max_len: 响应缓冲区最大长度
 * 
 * @return: 成功返回响应数据包大小，失败返回 -1
 */
int protocol_handle_request(Printer* printer, 
                           const uint8_t* request_packet, 
                           size_t request_len,
                           uint8_t* response_buf, 
                           size_t response_max_len);

#endif /* PROTOCOL_HANDLER_H */
