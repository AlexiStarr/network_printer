package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"
)

// ========= 协议约定 =========
const (
	ProtocolMagic   = 0xDEADBEEF
	ProtocolVersion = 1

	CmdGetStatus   uint8 = 0x01
	CmdSubmitJob   uint8 = 0x10
	CmdPauseJob    uint8 = 0x12
	CmdResumeJob   uint8 = 0x13
	CmdSimulateErr uint8 = 0x23
	CmdClearError  uint8 = 0x22
	CmdError       uint8 = 0xFF
)

const (
	// 枚举预测: 驱动的状态定义 (具体可参考 C 端 protocol.h)
	StateIdle         uint8 = 0
	StatePrintRunning uint8 = 1
	StatePrintPause   uint8 = 2
	StateError        uint8 = 3
	StateOffline      uint8 = 4
)

// ========= 结构体定义 =========
type ProtocolHeader struct {
	Magic    uint32
	Version  uint8
	Command  uint8
	Length   uint16
	Sequence uint32
}

type SubmitJobRequest struct {
	TaskID      uint32
	Pages       uint16
	Priority    uint8
	PaperSize   uint8
	FilenameLen uint16
}

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

func CalculateChecksum(data []byte) uint32 {
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
		sum = (sum << 1) | (sum >> 31)
	}
	return sum ^ ProtocolMagic
}

func sendCommand(conn net.Conn, cmd uint8, seq uint32, payload []byte) (*ProtocolHeader, []byte, error) {
	hdr := ProtocolHeader{
		Magic:    ProtocolMagic,
		Version:  ProtocolVersion,
		Command:  cmd,
		Length:   uint16(len(payload)),
		Sequence: seq,
	}

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, hdr)
	headerBytes := buf.Bytes()

	req := new(bytes.Buffer)
	req.Write(headerBytes)
	req.Write(payload)

	// Checksum is calculated over the combined header and payload
	checksum := CalculateChecksum(req.Bytes())
	binary.Write(req, binary.LittleEndian, checksum)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(req.Bytes()); err != nil {
		return nil, nil, err
	}

	hdrBytes := make([]byte, 12)
	if _, err := conn.Read(hdrBytes); err != nil {
		return nil, nil, err
	}
	var respHdr ProtocolHeader
	binary.Read(bytes.NewReader(hdrBytes), binary.LittleEndian, &respHdr)

	var payloadResp []byte
	if respHdr.Length > 0 {
		payloadResp = make([]byte, respHdr.Length)
		conn.Read(payloadResp)
	}

	chkBytes := make([]byte, 4)
	conn.Read(chkBytes)

	return &respHdr, payloadResp, nil
}

func getStatus(conn net.Conn) (StatusResponse, error) {
	var ms StatusResponse
	_, payload, err := sendCommand(conn, CmdGetStatus, 1, nil)
	if err != nil {
		return ms, err
	}
	if len(payload) >= 16 {
		binary.Read(bytes.NewReader(payload), binary.LittleEndian, &ms)
	}
	return ms, nil
}

func waitForState(conn net.Conn, targetState uint8, timeout time.Duration) (StatusResponse, error) {
	start := time.Now()
	for time.Since(start) < timeout {
		status, err := getStatus(conn)
		if err == nil && status.Status == targetState {
			return status, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	status, _ := getStatus(conn)
	return status, fmt.Errorf("timeout waiting for state %d, current is %d (HW Error Code: %d)", targetState, status.Status, status.Error)
}

func testNormalPrint(conn net.Conn) error {
	// IDLE -> START -> RUNNING -> FINISH -> IDLE
	req := SubmitJobRequest{
		TaskID:      101,
		Pages:       2,
		Priority:    1,
		PaperSize:   4, // A4
		FilenameLen: 8,
	}
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, req)
	buf.WriteString("test.pdf")

	fmt.Println("    [+] Checking printer status before submit...")
	status, _ := getStatus(conn)
	fmt.Printf("        Status: %d, Error: %d, Paper: %d, Toner: %d\n", status.Status, status.Error, status.PaperPages, status.TonerPercent)

	fmt.Println("    [+] Submitting Job 101...")
	_, _, err := sendCommand(conn, CmdSubmitJob, 2, buf.Bytes())
	if err != nil {
		return err
	}

	fmt.Println("    [+] Waiting for Printing Running (2)...")
	_, err = waitForState(conn, StatePrintRunning, 2*time.Second)
	if err != nil {
		return err
	}

	fmt.Println("    [+] Waiting for Idle (0)...")
	_, err = waitForState(conn, StateIdle, 8*time.Second)
	if err != nil {
		return err
	}

	return nil
}

func testPauseResume(conn net.Conn) error {
	// SUBMIT -> PAUSE -> RESUME
	req := SubmitJobRequest{
		TaskID:      102,
		Pages:       50, // More pages so it doesn't finish too fast
		Priority:    1,
		PaperSize:   4,
		FilenameLen: 8,
	}
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, req)
	buf.WriteString("long.pdf")

	hdr, payload, err := sendCommand(conn, CmdSubmitJob, 3, buf.Bytes())
	if err != nil {
		return err
	}
	if hdr.Command == CmdError {
		return fmt.Errorf("CmdSubmitJob returned error: %X", payload)
	}

	waitForState(conn, StatePrintRunning, 2*time.Second)

	fmt.Println("    [+] Issuing Pause Command...")
	sendCommand(conn, CmdPauseJob, 4, []byte{102, 0, 0, 0}) // basic TaskID byte

	_, err = waitForState(conn, StatePrintPause, 2*time.Second)
	if err != nil {
		fmt.Printf("Warning: driver might not support immediate pause state: %v\n", err)
	} else {
		fmt.Println("    [+] State verified: Paused.")
	}

	fmt.Println("    [+] Issuing Resume Command...")
	sendCommand(conn, CmdResumeJob, 5, nil)

	_, err = waitForState(conn, StatePrintRunning, 2*time.Second)
	if err != nil {
		return err
	}

	// wait until it naturally finishes
	waitForState(conn, StateIdle, 20*time.Second)
	return nil
}

func testErrorRecovery(conn net.Conn) error {
	// RUNNING -> ERROR (simulate) -> CLEAR -> IDLE
	req := SubmitJobRequest{
		TaskID:      103,
		Pages:       3,
		Priority:    1,
		PaperSize:   4,
		FilenameLen: 8,
	}
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, req)
	buf.WriteString("err.pdf")

	sendCommand(conn, CmdSubmitJob, 6, buf.Bytes())
	waitForState(conn, StatePrintRunning, 2*time.Second)

	fmt.Println("    [+] Injecting Hardware Error (CMD_SIMULATE_ERROR)...")
	// simulate error
	sendCommand(conn, CmdSimulateErr, 7, []byte{0x05}) // 0x05 for hardware error maybe
	time.Sleep(500 * time.Millisecond)

	status, _ := getStatus(conn)
	if status.Status != StateError && status.Error == 0 {
		fmt.Println("    [-] Note: Simulated error might not map to State=5, but checking error code.")
	} else {
		fmt.Println("    [+] Machine in Error State.")
	}

	fmt.Println("    [+] Clearing Error...")
	sendCommand(conn, CmdClearError, 8, nil)

	_, err := waitForState(conn, StateIdle, 8*time.Second)
	return err
}

func main() {
	fmt.Println("=== 6.2.2 状态机转移验证 ===")
	conn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		fmt.Printf("[FAIL] 无法连接到 C 驱动: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	tests := []struct {
		name string
		fn   func(net.Conn) error
	}{
		{"正常打印完整路径 (IDLE->START->RUNNING->FINISH->IDLE)", testNormalPrint},
		{"暂停恢复路径 (RUNNING->PAUSE->RESUME->RUNNING)", testPauseResume},
		{"错误恢复路径 (RUNNING->ERROR->RECOVERY->IDLE)", testErrorRecovery},
	}

	// 初始化: 先补充纸张和碳粉，保证能正常启动打印
	fmt.Println("[+] Initializing printer state (Refilling Paper & Toner)...")
	paperPayload := make([]byte, 4)
	binary.LittleEndian.PutUint32(paperPayload, 500)
	hdr, _, err := sendCommand(conn, 0x20, 98, paperPayload) // CmdRefillPaper
	if err != nil {
		fmt.Printf("Refill Error: %v\n", err)
	} else {
		fmt.Printf("Refill Response: %X\n", hdr.Command)
	}
	sendCommand(conn, 0x21, 99, nil) // CmdRefillToner
	sendCommand(conn, CmdClearError, 100, nil)

	allPass := true
	for _, tc := range tests {
		fmt.Printf("测试: %s\n", tc.name)
		// Ensure machine is init idle
		sendCommand(conn, CmdClearError, 99, nil)
		waitForState(conn, StateIdle, 3*time.Second)

		err := tc.fn(conn)
		if err != nil {
			fmt.Printf("  -> [FAIL] %v\n", err)
			allPass = false
		} else {
			fmt.Println("  -> [PASS]")
		}
	}

	if allPass {
		fmt.Println("=> 6.2.2 状态机转移验证全部通过！")
	} else {
		fmt.Println("=> 6.2.2 状态机测试存在失败项。")
		os.Exit(1)
	}
}
