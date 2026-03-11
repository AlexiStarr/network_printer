/**
 * binary_protocol.go
 * 二进制协议编码和解码实现 (Go 版本)
 * 与 C 驱动相互兼容
 */

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ==================== 协议常量定义 ====================

const (
	ProtocolMagic   = 0xDEADBEEF
	ProtocolVersion = 1
	MaxPayloadSize  = 65536
)

// 命令类型
const (
	CmdGetStatus     uint8 = 0x01
	CmdGetQueue      uint8 = 0x02
	CmdGetHistory    uint8 = 0x03
	CmdSubmitJob     uint8 = 0x10
	CmdCancelJob     uint8 = 0x11
	CmdPauseJob      uint8 = 0x12
	CmdResumeJob     uint8 = 0x13
	CmdRefillPaper   uint8 = 0x20
	CmdRefillToner   uint8 = 0x21
	CmdClearError    uint8 = 0x22
	CmdSimulateError uint8 = 0x23
	CmdSetPaperMax   uint8 = 0x24
	CmdPrintData     uint8 = 0x30
	CmdDataChunk     uint8 = 0x31
	CmdAck           uint8 = 0xFE
	CmdError         uint8 = 0xFF
)

// 错误代码
const (
	ErrSuccess        uint8 = 0x00
	ErrInvalidCmd     uint8 = 0x01
	ErrInvalidParam   uint8 = 0x02
	ErrBufferOverflow uint8 = 0x03
	ErrChecksumFail   uint8 = 0x04
	ErrHardwareError  uint8 = 0x05
	ErrQueueFull      uint8 = 0x06
	ErrJobNotFound    uint8 = 0x07
	ErrPaperFull      uint8 = 0x08
	ErrUnknown        uint8 = 0xFF
)

// ==================== 协议结构体定义 ====================

// ProtocolHeader 协议头
type ProtocolHeader struct {
	Magic    uint32
	Version  uint8
	Command  uint8
	Length   uint16
	Sequence uint32
}

// StatusResponse 状态查询响应
type StatusResponse struct {
	Status        uint8
	Error         uint8
	PaperPages    uint16
	TonerPercent  uint16
	Temperature   uint8
	PageCount     uint32
	QueueSize     uint16
	CurrentTaskID uint8
	Reserved      [3]uint8
}

// SubmitJobRequest 提交任务请求
type SubmitJobRequest struct {
	TaskID      uint32
	Pages       uint16
	Priority    uint8
	PaperSize   uint8
	FilenameLen uint16
}

// TaskProgress 任务进度
type TaskProgress struct {
	TaskID           uint32
	Status           uint8
	PrintedPages     uint16
	ProgressPercent  uint32
	EstimatedTimeSec uint16
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	ErrorCode uint8
	DetailLen uint16
}

// ==================== 校验和计算 ====================

// CalculateChecksum 计算校验和
func CalculateChecksum(data []byte) uint32 {
	var sum uint32

	// 字节求和
	for _, b := range data {
		sum += uint32(b)
		// 循环左移
		sum = (sum << 1) | (sum >> 31)
	}

	// 与magic异或
	return sum ^ ProtocolMagic
}

// VerifyChecksum 验证校验和
func VerifyChecksum(data []byte, expected uint32) bool {
	calculated := CalculateChecksum(data)
	return calculated == expected
}

// ==================== 编码函数 ====================

// EncodeHeader 编码协议头
func EncodeHeader(cmd uint8, dataLen uint16, sequence uint32) []byte {
	buf := new(bytes.Buffer)

	hdr := &ProtocolHeader{
		Magic:    ProtocolMagic,
		Version:  ProtocolVersion,
		Command:  cmd,
		Length:   dataLen,
		Sequence: sequence,
	}

	binary.Write(buf, binary.LittleEndian, hdr)
	return buf.Bytes()
}

// EncodeStatusResponse 编码状态响应
func EncodeStatusResponse(status *StatusResponse, sequence uint32) ([]byte, error) {
	buf := new(bytes.Buffer)

	// 写入状态数据
	if err := binary.Write(buf, binary.LittleEndian, status); err != nil {
		return nil, err
	}

	payload := buf.Bytes()

	// 创建完整数据包
	header := EncodeHeader(CmdGetStatus, uint16(len(payload)), sequence)
	checksum := CalculateChecksum(header)
	checksum ^= CalculateChecksum(payload)

	packet := new(bytes.Buffer)
	packet.Write(header)
	packet.Write(payload)
	binary.Write(packet, binary.LittleEndian, checksum)

	return packet.Bytes(), nil
}

// EncodeTaskProgress 编码任务进度
func EncodeTaskProgress(progress *TaskProgress) ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.LittleEndian, progress); err != nil {
		return nil, err
	}

	payload := buf.Bytes()
	header := EncodeHeader(CmdDataChunk, uint16(len(payload)), progress.TaskID)
	checksum := CalculateChecksum(header)
	checksum ^= CalculateChecksum(payload)

	packet := new(bytes.Buffer)
	packet.Write(header)
	packet.Write(payload)
	binary.Write(packet, binary.LittleEndian, checksum)

	return packet.Bytes(), nil
}

// EncodeErrorResponse 编码错误响应
func EncodeErrorResponse(errCode uint8, detail string) ([]byte, error) {
	payload := new(bytes.Buffer)

	errResp := &ErrorResponse{
		ErrorCode: errCode,
		DetailLen: uint16(len(detail)),
	}

	if err := binary.Write(payload, binary.LittleEndian, errResp); err != nil {
		return nil, err
	}

	payload.WriteString(detail)

	payloadBytes := payload.Bytes()
	header := EncodeHeader(CmdError, uint16(len(payloadBytes)), 0)
	checksum := CalculateChecksum(header)
	checksum ^= CalculateChecksum(payloadBytes)

	packet := new(bytes.Buffer)
	packet.Write(header)
	packet.Write(payloadBytes)
	binary.Write(packet, binary.LittleEndian, checksum)

	return packet.Bytes(), nil
}

// EncodeAck 编码ACK消息
func EncodeAck(sequence uint32) ([]byte, error) {
	header := EncodeHeader(CmdAck, 0, sequence)
	checksum := CalculateChecksum(header)

	packet := new(bytes.Buffer)
	packet.Write(header)
	binary.Write(packet, binary.LittleEndian, checksum)

	return packet.Bytes(), nil
}

// ==================== 解码函数 ====================

// DecodePacket 解析完整的数据包
func DecodePacket(packet []byte) (*ProtocolHeader, []byte, error) {
	if len(packet) < 16 { // 12字节头 + 4字节校验和
		return nil, nil, fmt.Errorf("数据包太小")
	}

	// 解析头
	reader := bytes.NewReader(packet[:12])
	hdr := &ProtocolHeader{}
	if err := binary.Read(reader, binary.LittleEndian, hdr); err != nil {
		return nil, nil, fmt.Errorf("解析头失败: %w", err)
	}

	// 验证magic和版本
	if hdr.Magic != ProtocolMagic {
		return nil, nil, fmt.Errorf("invalid magic: %x", hdr.Magic)
	}

	if hdr.Version != ProtocolVersion {
		return nil, nil, fmt.Errorf("unsupported protocol version: %d", hdr.Version)
	}

	// 验证长度
	expectedLen := 12 + hdr.Length + 4
	if len(packet) != int(expectedLen) {
		return nil, nil, fmt.Errorf("数据包长度不匹配: 期望 %d, 实际 %d", expectedLen, len(packet))
	}

	// 验证校验和
	receivedChecksum := binary.LittleEndian.Uint32(packet[12+hdr.Length:])
	calculatedChecksum := CalculateChecksum(packet[:12+hdr.Length])

	if receivedChecksum != calculatedChecksum {
		return nil, nil, fmt.Errorf("校验和验证失败")
	}

	// 提取数据部分
	data := packet[12 : 12+hdr.Length]

	return hdr, data, nil
}

// DecodeSubmitJobRequest 解码提交任务请求
func DecodeSubmitJobRequest(data []byte) (*SubmitJobRequest, string, error) {
	if len(data) < binary.Size(&SubmitJobRequest{}) {
		return nil, "", fmt.Errorf("数据不足以解析请求头")
	}

	reader := bytes.NewReader(data)
	req := &SubmitJobRequest{}

	if err := binary.Read(reader, binary.LittleEndian, req); err != nil {
		return nil, "", fmt.Errorf("解析请求失败: %w", err)
	}

	// 提取文件名
	if int(req.FilenameLen) > len(data)-binary.Size(req) {
		return nil, "", fmt.Errorf("文件名长度超出")
	}

	filenameBytes := data[binary.Size(req) : binary.Size(req)+int(req.FilenameLen)]

	return req, string(filenameBytes), nil
}

// DecodeTaskStatus 解码任务状态
func DecodeTaskStatus(data []byte) (*TaskProgress, error) {
	if len(data) < binary.Size(&TaskProgress{}) {
		return nil, fmt.Errorf("数据不足")
	}

	reader := bytes.NewReader(data)
	progress := &TaskProgress{}

	if err := binary.Read(reader, binary.LittleEndian, progress); err != nil {
		return nil, fmt.Errorf("解析进度失败: %w", err)
	}

	return progress, nil
}

// ==================== 辅助函数 ====================

// GetCommandName 获取命令名称
func GetCommandName(cmd uint8) string {
	switch cmd {
	case CmdGetStatus:
		return "GET_STATUS"
	case CmdGetQueue:
		return "GET_QUEUE"
	case CmdGetHistory:
		return "GET_HISTORY"
	case CmdSubmitJob:
		return "SUBMIT_JOB"
	case CmdCancelJob:
		return "CANCEL_JOB"
	case CmdPauseJob:
		return "PAUSE_JOB"
	case CmdResumeJob:
		return "RESUME_JOB"
	case CmdRefillPaper:
		return "REFILL_PAPER"
	case CmdRefillToner:
		return "REFILL_TONER"
	case CmdClearError:
		return "CLEAR_ERROR"
	case CmdSimulateError:
		return "SIMULATE_ERROR"
	case CmdSetPaperMax:
		return "SET_PAPER_MAX"
	case CmdAck:
		return "ACK"
	case CmdError:
		return "ERROR"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02x)", cmd)
	}
}

// GetErrorName 获取错误名称
func GetErrorName(code uint8) string {
	switch code {
	case ErrSuccess:
		return "SUCCESS"
	case ErrInvalidCmd:
		return "INVALID_CMD"
	case ErrInvalidParam:
		return "INVALID_PARAM"
	case ErrBufferOverflow:
		return "BUFFER_OVERFLOW"
	case ErrChecksumFail:
		return "CHECKSUM_FAIL"
	case ErrHardwareError:
		return "HARDWARE_ERROR"
	case ErrQueueFull:
		return "QUEUE_FULL"
	case ErrJobNotFound:
		return "JOB_NOT_FOUND"
	case ErrPaperFull:
		return "PAPER_FULL"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02x)", code)
	}
}

// CopyStructToBinary 将结构体转换为二进制
func CopyStructToBinary(s interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, s)
	return buf.Bytes(), err
}

// CopyBinaryToStruct 将二进制转换为结构体
func CopyBinaryToStruct(data []byte, s interface{}) error {
	reader := bytes.NewReader(data)
	return binary.Read(reader, binary.LittleEndian, s)
}
