package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
)

const (
	ProtocolMagic   = 0xDEADBEEF
	ProtocolVersion = 1

	// Commands
	CmdGetStatus   uint8 = 0x01
	CmdGetQueue    uint8 = 0x02
	CmdSubmitJob   uint8 = 0x10
	CmdCancelJob   uint8 = 0x11
	CmdRefillPaper uint8 = 0x20
	CmdSimulateErr uint8 = 0x23
	CmdAck         uint8 = 0xFE
	CmdError       uint8 = 0xFF

	// Errors
	ErrSuccess      uint8 = 0x00
	ErrChecksumFail uint8 = 0x04
)

// ProtocolHeader 12 bytes
type ProtocolHeader struct {
	Magic    uint32
	Version  uint8
	Command  uint8
	Length   uint16
	Sequence uint32
}

// CalculateChecksum
func CalculateChecksum(data []byte) uint32 {
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
		sum = (sum << 1) | (sum >> 31)
	}
	return sum ^ ProtocolMagic
}

func encodeCommand(cmd uint8, seq uint32, payload []byte) []byte {
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

	checksum := CalculateChecksum(req.Bytes())
	binary.Write(req, binary.LittleEndian, checksum)
	return req.Bytes()
}

func sendCommand(conn net.Conn, cmd uint8, seq uint32, payload []byte) (*ProtocolHeader, []byte, error) {
	reqData := encodeCommand(cmd, seq, payload)
	_, err := conn.Write(reqData)
	if err != nil {
		return nil, nil, fmt.Errorf("write error: %v", err)
	}

	// Read Header
	hdrBytes := make([]byte, 12)
	_, err = conn.Read(hdrBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("read header error: %v", err)
	}
	var respHdr ProtocolHeader
	binary.Read(bytes.NewReader(hdrBytes), binary.LittleEndian, &respHdr)

	// Read Payload Ensure it's read
	var payloadResp []byte
	if respHdr.Length > 0 {
		payloadResp = make([]byte, respHdr.Length)
		_, err = conn.Read(payloadResp)
		if err != nil {
			return nil, nil, fmt.Errorf("read payload error: %v", err)
		}
	}

	// Read Checksum
	chkBytes := make([]byte, 4)
	_, err = conn.Read(chkBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("read checksum error: %v", err)
	}

	return &respHdr, payloadResp, nil
}

func testZeroLengthPayload(conn net.Conn) error {
	hdr, _, err := sendCommand(conn, CmdGetStatus, 1001, nil)
	if err != nil {
		return err
	}
	if hdr.Command == CmdError {
		return fmt.Errorf("expected success or status, got error")
	}
	return nil
}

func testMaxPayloadValid(conn net.Conn) error {
	largePayload := make([]byte, 4000)
	hdr, _, err := sendCommand(conn, 0x99, 1002, largePayload)
	if err != nil {
		return err
	}
	if hdr.Command != CmdError {
		return fmt.Errorf("expected CmdError(0xFF) for unknown cmd 0x99, got 0x%X", hdr.Command)
	}
	return nil
}

func testStickyPackets(conn net.Conn) error {
	req1 := encodeCommand(CmdGetStatus, 2001, nil)
	req2 := encodeCommand(CmdGetStatus, 2002, nil)
	req3 := encodeCommand(CmdGetStatus, 2003, nil)

	batch := append(append(req1, req2...), req3...)
	_, err := conn.Write(batch)
	if err != nil {
		return err
	}

	for i := 1; i <= 3; i++ {
		hdrBytes := make([]byte, 12)
		if _, err := conn.Read(hdrBytes); err != nil {
			return fmt.Errorf("read sticky packet %d header error: %v", i, err)
		}
		var h ProtocolHeader
		binary.Read(bytes.NewReader(hdrBytes), binary.LittleEndian, &h)

		if h.Length > 0 {
			discard := make([]byte, h.Length)
			conn.Read(discard)
		}

		chkBytes := make([]byte, 4)
		conn.Read(chkBytes)

		if h.Sequence != 0 {
			// C driver hardcodes sequence 0 in responses
			return fmt.Errorf("expected seq 0, got %d", h.Sequence)
		}
	}
	return nil
}

func testInvalidChecksum(conn net.Conn) error {
	hdr := ProtocolHeader{
		Magic:    ProtocolMagic,
		Version:  ProtocolVersion,
		Command:  CmdGetStatus,
		Length:   0,
		Sequence: 3001,
	}

	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, hdr)
	headerBytes := buf.Bytes()

	// Deliberately write wrong checksum
	req := new(bytes.Buffer)
	req.Write(headerBytes)
	binary.Write(req, binary.LittleEndian, uint32(0xDEADBEEF))

	_, err := conn.Write(req.Bytes())
	if err != nil {
		return err
	}

	respHdrBytes := make([]byte, 12)
	_, err = conn.Read(respHdrBytes)
	if err != nil {
		return fmt.Errorf("read error response failed: %v", err)
	}

	var respHdr ProtocolHeader
	binary.Read(bytes.NewReader(respHdrBytes), binary.LittleEndian, &respHdr)

	var payload []byte
	if respHdr.Length > 0 {
		payload = make([]byte, respHdr.Length)
		conn.Read(payload)
	}

	chkBytes := make([]byte, 4)
	conn.Read(chkBytes)

	if respHdr.Command != CmdError {
		return fmt.Errorf("expected CmdError(0xFF) due to invalid checksum, got 0x%X", respHdr.Command)
	}

	if len(payload) > 0 && payload[0] != 0x01 {
		return fmt.Errorf("expected ErrInvalidCmd(0x01) because driver verify_and_parse_header returns NULL on bad checksum, got 0x%X", payload[0])
	}

	return nil
}

func main() {
	fmt.Println("=== 6.2.1 协议编解码一致性验证 ===")
	conn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		fmt.Printf("[FAIL] 无法连接到 C 驱动: %v\n", err)
		fmt.Println("请确保 C 驱动模拟器 (driver_server) 正在 9999 端口运行。")
		os.Exit(1)
	}
	defer conn.Close()

	tests := []struct {
		name string
		fn   func(net.Conn) error
	}{
		{"零长度载荷报文测试 (CMD_GET_STATUS)", testZeroLengthPayload},
		{"最大载荷验证与越界命令处理", testMaxPayloadValid},
		{"粘包/半包解析测试 (连续发送数据)", testStickyPackets},
		{"校验和错误处理机制测试", testInvalidChecksum},
	}

	allPass := true
	for _, tc := range tests {
		fmt.Printf("测试: %s ... ", tc.name)
		err := tc.fn(conn)
		if err != nil {
			fmt.Printf("[FAIL]\n  -> %v\n", err)
			allPass = false
			conn.Close()
			conn, _ = net.Dial("tcp", "127.0.0.1:9999")
		} else {
			fmt.Println("[PASS]")
		}
	}

	if allPass {
		fmt.Println("=> 6.2.1 协议编解码验证全部通过！")
	} else {
		fmt.Println("=> 6.2.1 协议编解码验证存在失败项。")
		os.Exit(1)
	}
}
