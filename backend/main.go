package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// ==================== 二进制协议常量 ====================

const (
	PROTOCOL_MAGIC   = 0xDEADBEEF
	PROTOCOL_VERSION = 1
	// 命令值必须与 binary_protocol.go 及 C 驱动完全一致
	CMD_GET_STATUS     = 0x01
	CMD_GET_QUEUE      = 0x02
	CMD_SUBMIT_JOB     = 0x10
	CMD_CANCEL_JOB     = 0x11
	CMD_PAUSE_JOB      = 0x12
	CMD_RESUME_JOB     = 0x13
	CMD_REFILL_PAPER   = 0x20
	CMD_REFILL_TONER   = 0x21
	CMD_CLEAR_ERROR    = 0x22
	CMD_SIMULATE_ERROR = 0x23
	CMD_ACK            = 0xFE
	CMD_ERROR_RESP     = 0xFF
)

// Database 数据库管理器
type Database struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewDatabase 创建新的数据库管理器
func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// 创建表
	schema := `
	CREATE TABLE IF NOT EXISTS print_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER,
		filename TEXT,
		pages INTEGER,
		printed_pages INTEGER,
		status TEXT,
		created_at DATETIME,
		completed_at DATETIME,
		user_id TEXT,
		priority INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE,
		password_hash TEXT,
		role TEXT,
		created_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT,
		action TEXT,
		details TEXT,
		timestamp DATETIME
	);

	CREATE TABLE IF NOT EXISTS task_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER UNIQUE,
		filename TEXT,
		pages INTEGER,
		priority INTEGER,
		status TEXT,
		created_at DATETIME,
		started_at DATETIME,
		completed_at DATETIME
	);
	`

	_, err = db.Exec(schema)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// RecordPrintJob 记录打印任务
func (d *Database) RecordPrintJob(taskID int, filename string, pages int, userID string, priority int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		`INSERT INTO print_history (task_id, filename, pages, status, created_at, user_id, priority) 
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		taskID, filename, pages, "submitted", time.Now(), userID, priority,
	)
	return err
}

// UpdatePrintJob 更新打印任务
func (d *Database) UpdatePrintJob(taskID int, printedPages int, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE print_history SET printed_pages = ?, status = ? WHERE task_id = ?`
	if status == "completed" {
		query = `UPDATE print_history SET printed_pages = ?, status = ?, completed_at = ? WHERE task_id = ?`
		_, err := d.db.Exec(query, printedPages, status, time.Now(), taskID)
		return err
	}

	_, err := d.db.Exec(query, printedPages, status, taskID)
	return err
}

// GetPrintHistory 获取打印历史
func (d *Database) GetPrintHistory(userID string, limit int) ([]map[string]interface{}, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var rows *sql.Rows
	var err error

	if userID == "" {
		rows, err = d.db.Query(
			`SELECT task_id, filename, pages, printed_pages, status, created_at, user_id, priority 
			 FROM print_history ORDER BY created_at DESC LIMIT ?`, limit,
		)
	} else {
		rows, err = d.db.Query(
			`SELECT task_id, filename, pages, printed_pages, status, created_at, user_id, priority 
			 FROM print_history WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`, userID, limit,
		)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var taskID, pages, printedPages, priority int
		var filename, status, userID string
		var createdAt time.Time

		err := rows.Scan(&taskID, &filename, &pages, &printedPages, &status, &createdAt, &userID, &priority)
		if err != nil {
			return nil, err
		}

		history = append(history, map[string]interface{}{
			"task_id":       taskID,
			"filename":      filename,
			"pages":         pages,
			"printed_pages": printedPages,
			"status":        status,
			"created_at":    createdAt.Format(time.RFC3339),
			"user_id":       userID,
			"priority":      priority,
		})
	}

	return history, nil
}

// CreateUser 创建用户
func (d *Database) CreateUser(username, password, role string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = d.db.Exec(
		`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		username, string(hash), role, time.Now(),
	)
	return err
}

// VerifyUser 验证用户密码
func (d *Database) VerifyUser(username, password string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var hash string
	err := d.db.QueryRow(
		`SELECT password_hash FROM users WHERE username = ?`,
		username,
	).Scan(&hash)

	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil, nil
}

// GetUserRole 获取用户角色
func (d *Database) GetUserRole(username string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var role string
	err := d.db.QueryRow(
		`SELECT role FROM users WHERE username = ?`,
		username,
	).Scan(&role)
	return role, err
}

// RecordAuditLog 记录审计日志
func (d *Database) RecordAuditLog(userID, action, details string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		`INSERT INTO audit_log (user_id, action, details, timestamp) VALUES (?, ?, ?, ?)`,
		userID, action, details, time.Now(),
	)
	return err
}

// DeleteUser 删除用户
func (d *Database) DeleteUser(username string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM users WHERE username = ?`, username)
	return err
}

// ListUsers 列出所有用户
func (d *Database) ListUsers() ([]map[string]interface{}, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		`SELECT username, role, created_at FROM users ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var username, role string
		var createdAt time.Time

		err := rows.Scan(&username, &role, &createdAt)
		if err != nil {
			return nil, err
		}

		users = append(users, map[string]interface{}{
			"username":   username,
			"role":       role,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}

	return users, nil
}

// UserExists 检查用户是否存在
func (d *Database) UserExists(username string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ==================== WebSocket 相关 ====================

// WebSocketHub WebSocket 连接管理
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool
	broadcast  chan interface{}
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex
}

// WebSocketClient WebSocket 客户端
type WebSocketClient struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan interface{}
}

// NewWebSocketHub 创建新的 WebSocket Hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		broadcast:  make(chan interface{}, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
}

// Run WebSocket Hub 主循环
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Println("[WebSocket] 客户端已连接")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Println("[WebSocket] 客户端已断开连接")

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// 客户端发送缓冲已满，跳过
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 广播消息
func (h *WebSocketHub) Broadcast(data interface{}) {
	h.broadcast <- data
}

// ==================== 身份认证相关 ====================

// TokenManager Token 管理器
type TokenManager struct {
	tokens map[string]TokenInfo
	mu     sync.RWMutex
}

// TokenInfo Token 信息
type TokenInfo struct {
	Username  string
	ExpiresAt time.Time
	Role      string
}

// NewTokenManager 创建新的 Token 管理器
func NewTokenManager() *TokenManager {
	return &TokenManager{
		tokens: make(map[string]TokenInfo),
	}
}

// GenerateToken 生成 Token
func (tm *TokenManager) GenerateToken(username, role string, duration time.Duration) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	token := fmt.Sprintf("%x", sha256.Sum256([]byte(username+time.Now().String())))
	tm.tokens[token] = TokenInfo{
		Username:  username,
		ExpiresAt: time.Now().Add(duration),
		Role:      role,
	}
	return token
}

// VerifyToken 验证 Token
func (tm *TokenManager) VerifyToken(token string) (TokenInfo, bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	info, ok := tm.tokens[token]
	if !ok {
		return TokenInfo{}, false
	}

	// Bug #1 修复: 检查过期并自动清理过期token，防止内存泄漏
	if time.Now().After(info.ExpiresAt) {
		delete(tm.tokens, token)
		return TokenInfo{}, false
	}
	return info, true
}

// RevokeToken 撤销 Token
func (tm *TokenManager) RevokeToken(token string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.tokens, token)
}

// ==================== 优先级队列相关 ====================

// PrintJobQueue 打印任务优先级队列
type PrintJobQueue struct {
	jobs map[int]*PrintJob
	heap []*PrintJob
	mu   sync.RWMutex
}

// PrintJob 打印任务
type PrintJob struct {
	TaskID    int
	Filename  string
	Pages     int
	Priority  int
	Status    string
	CreatedAt time.Time
	UserID    string
}

// NewPrintJobQueue 创建新的打印任务队列
func NewPrintJobQueue() *PrintJobQueue {
	return &PrintJobQueue{
		jobs: make(map[int]*PrintJob),
		heap: make([]*PrintJob, 0),
	}
}

// Enqueue 入队
func (q *PrintJobQueue) Enqueue(job *PrintJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.jobs[job.TaskID] = job
	q.heap = append(q.heap, job)
	q.bubbleUp(len(q.heap) - 1)
}

// Dequeue 出队
func (q *PrintJobQueue) Dequeue() *PrintJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.heap) == 0 {
		return nil
	}

	job := q.heap[0]
	q.heap[0] = q.heap[len(q.heap)-1]
	q.heap = q.heap[:len(q.heap)-1]

	if len(q.heap) > 0 {
		q.bubbleDown(0)
	}

	delete(q.jobs, job.TaskID)
	return job
}

// Peek 查看队首
func (q *PrintJobQueue) Peek() *PrintJob {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if len(q.heap) == 0 {
		return nil
	}
	return q.heap[0]
}

// bubbleUp 向上冒泡
func (q *PrintJobQueue) bubbleUp(index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if q.heap[index].Priority <= q.heap[parent].Priority {
			break
		}
		q.heap[index], q.heap[parent] = q.heap[parent], q.heap[index]
		index = parent
	}
}

// bubbleDown 向下冒泡
func (q *PrintJobQueue) bubbleDown(index int) {
	for {
		left := 2*index + 1
		right := 2*index + 2
		largest := index

		if left < len(q.heap) && q.heap[left].Priority > q.heap[largest].Priority {
			largest = left
		}
		if right < len(q.heap) && q.heap[right].Priority > q.heap[largest].Priority {
			largest = right
		}
		if largest == index {
			break
		}

		q.heap[index], q.heap[largest] = q.heap[largest], q.heap[index]
		index = largest
	}
}

// GetQueueSize 获取队列大小
func (q *PrintJobQueue) GetQueueSize() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.heap)
}

// ==================== 二进制协议编码/解码 ====================

// calculateBinaryChecksum 计算校验和，算法与 binary_protocol.go / C 驱动完全一致：
// 每字节先累加再循环左移，最后与 PROTOCOL_MAGIC 异或
func calculateBinaryChecksum(data []byte) uint32 {
	var sum uint32
	for _, b := range data {
		sum += uint32(b)
		sum = (sum << 1) | (sum >> 31) // 循环左移 1 位
	}
	return sum ^ PROTOCOL_MAGIC
}

// 编码获取状态命令
func encodeGetStatusRequest(sequence uint32) []byte {
	buf := new(bytes.Buffer)

	// 头部 (12 字节)
	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC)) // 魔法数字
	buf.WriteByte(PROTOCOL_VERSION)                                // 版本
	buf.WriteByte(byte(CMD_GET_STATUS))                            // 命令
	binary.Write(buf, binary.LittleEndian, uint16(0))              // 数据长度(0)
	binary.Write(buf, binary.LittleEndian, sequence)               // 序列号

	headerAndData := buf.Bytes()

	// 校验和
	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// 编码提交任务命令（payload 布局与 SubmitJobRequest 结构体一致）
func encodeSubmitJobRequest(filename string, pages int, sequence uint32) []byte {
	buf := new(bytes.Buffer)

	// payload: TaskID(4) + Pages(2) + Priority(1) + PaperSize(1) + FilenameLen(2) + filename bytes
	dataBuf := new(bytes.Buffer)
	binary.Write(dataBuf, binary.LittleEndian, uint32(0))             // TaskID（由驱动分配，填 0）
	binary.Write(dataBuf, binary.LittleEndian, uint16(pages))         // Pages
	dataBuf.WriteByte(0)                                              // Priority（默认 0）
	dataBuf.WriteByte(0)                                              // PaperSize（默认 0 = A4）
	binary.Write(dataBuf, binary.LittleEndian, uint16(len(filename))) // FilenameLen
	dataBuf.WriteString(filename)                                     // filename bytes（不加 null）

	dataBytes := dataBuf.Bytes()

	// 头部 (12 字节)
	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC))
	buf.WriteByte(PROTOCOL_VERSION)
	buf.WriteByte(byte(CMD_SUBMIT_JOB))
	binary.Write(buf, binary.LittleEndian, uint16(len(dataBytes)))
	binary.Write(buf, binary.LittleEndian, sequence)

	buf.Write(dataBytes)

	headerAndData := buf.Bytes()
	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// 编码取消任务命令
func encodeCancelJobRequest(taskID int, sequence uint32) []byte {
	buf := new(bytes.Buffer)

	// 数据部分
	dataBuf := new(bytes.Buffer)
	binary.Write(dataBuf, binary.LittleEndian, uint32(taskID))

	dataBytes := dataBuf.Bytes()

	// 头部
	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC))
	buf.WriteByte(PROTOCOL_VERSION)
	buf.WriteByte(byte(CMD_CANCEL_JOB))
	binary.Write(buf, binary.LittleEndian, uint16(len(dataBytes)))
	binary.Write(buf, binary.LittleEndian, sequence)

	// 数据
	buf.Write(dataBytes)

	headerAndData := buf.Bytes()

	// 校验和
	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// 编码暂停任务命令
func encodePauseJobRequest(taskID int, sequence uint32) []byte {
	buf := new(bytes.Buffer)

	dataBuf := new(bytes.Buffer)
	binary.Write(dataBuf, binary.LittleEndian, uint32(taskID))
	dataBytes := dataBuf.Bytes()

	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC))
	buf.WriteByte(PROTOCOL_VERSION)
	buf.WriteByte(byte(CMD_PAUSE_JOB))
	binary.Write(buf, binary.LittleEndian, uint16(len(dataBytes)))
	binary.Write(buf, binary.LittleEndian, sequence)

	buf.Write(dataBytes)
	headerAndData := buf.Bytes()

	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// 编码恢复任务命令
func encodeResumeJobRequest(taskID int, sequence uint32) []byte {
	buf := new(bytes.Buffer)

	dataBuf := new(bytes.Buffer)
	binary.Write(dataBuf, binary.LittleEndian, uint32(taskID))
	dataBytes := dataBuf.Bytes()

	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC))
	buf.WriteByte(PROTOCOL_VERSION)
	buf.WriteByte(byte(CMD_RESUME_JOB))
	binary.Write(buf, binary.LittleEndian, uint16(len(dataBytes)))
	binary.Write(buf, binary.LittleEndian, sequence)

	buf.Write(dataBytes)
	headerAndData := buf.Bytes()

	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// 编码获取队列命令
func encodeGetQueueRequest(sequence uint32) []byte {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC))
	buf.WriteByte(PROTOCOL_VERSION)
	buf.WriteByte(byte(CMD_GET_QUEUE))
	binary.Write(buf, binary.LittleEndian, uint16(0)) // 数据长度为0
	binary.Write(buf, binary.LittleEndian, sequence)

	headerAndData := buf.Bytes()

	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// 编码补充纸张命令
func encodeRefillPaperRequest(pages int, sequence uint32) []byte {
	buf := new(bytes.Buffer)

	dataBuf := new(bytes.Buffer)
	binary.Write(dataBuf, binary.LittleEndian, uint32(pages))
	dataBytes := dataBuf.Bytes()

	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC))
	buf.WriteByte(PROTOCOL_VERSION)
	buf.WriteByte(byte(CMD_REFILL_PAPER))
	binary.Write(buf, binary.LittleEndian, uint16(len(dataBytes)))
	binary.Write(buf, binary.LittleEndian, sequence)

	buf.Write(dataBytes)
	headerAndData := buf.Bytes()

	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// 编码补充碳粉命令
func encodeRefillTonerRequest(sequence uint32) []byte {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC))
	buf.WriteByte(PROTOCOL_VERSION)
	buf.WriteByte(byte(CMD_REFILL_TONER))
	binary.Write(buf, binary.LittleEndian, uint16(0)) // 数据长度为0
	binary.Write(buf, binary.LittleEndian, sequence)

	headerAndData := buf.Bytes()

	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// 编码清除错误命令
func encodeClearErrorRequest(sequence uint32) []byte {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC))
	buf.WriteByte(PROTOCOL_VERSION)
	buf.WriteByte(byte(CMD_CLEAR_ERROR))
	binary.Write(buf, binary.LittleEndian, uint16(0))
	binary.Write(buf, binary.LittleEndian, sequence)

	headerAndData := buf.Bytes()

	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// 编码模拟错误命令
func encodeSimulateErrorRequest(errorType int, sequence uint32) []byte {
	buf := new(bytes.Buffer)

	dataBuf := new(bytes.Buffer)
	dataBuf.WriteByte(byte(errorType))
	dataBytes := dataBuf.Bytes()

	binary.Write(buf, binary.LittleEndian, uint32(PROTOCOL_MAGIC))
	buf.WriteByte(PROTOCOL_VERSION)
	buf.WriteByte(byte(CMD_SIMULATE_ERROR))
	binary.Write(buf, binary.LittleEndian, uint16(len(dataBytes)))
	binary.Write(buf, binary.LittleEndian, sequence)

	buf.Write(dataBytes)
	headerAndData := buf.Bytes()

	checksum := calculateBinaryChecksum(headerAndData)
	binary.Write(buf, binary.LittleEndian, checksum)

	return buf.Bytes()
}

// parseBinaryResponse 解析 C 驱动返回的二进制响应数据包。
// 协议头布局（12 字节）：Magic(4) | Version(1) | Command(1) | Length(2) | Sequence(4)
func parseBinaryResponse(data []byte) (map[string]interface{}, error) {
	// 检查最小长度：12(头) + 0(数据) + 4(校验和)
	if len(data) < 16 {
		return nil, fmt.Errorf("响应过短: %d 字节", len(data))
	}

	// 解析头部
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != PROTOCOL_MAGIC {
		return nil, fmt.Errorf("魔法数字错误: 0x%X", magic)
	}

	version := data[4]
	if version != PROTOCOL_VERSION {
		return nil, fmt.Errorf("协议版本错误: %d", version)
	}

	cmd := data[5]
	// Bug #4 修复：Length 字段在偏移 6-7，而非 8-9（8-11 是 Sequence）
	dataLen := binary.LittleEndian.Uint16(data[6:8])

	// 检查完整性
	totalLen := 12 + int(dataLen) + 4
	if len(data) < totalLen {
		return nil, fmt.Errorf("响应不完整: 期望 %d 字节，实际 %d 字节", totalLen, len(data))
	}

	// 验证校验和
	payloadData := data[:12+int(dataLen)]
	receivedChecksum := binary.LittleEndian.Uint32(data[12+int(dataLen) : 12+int(dataLen)+4])
	calculatedChecksum := calculateBinaryChecksum(payloadData)
	if receivedChecksum != calculatedChecksum {
		return nil, fmt.Errorf("校验和验证失败: recv=0x%X, calc=0x%X", receivedChecksum, calculatedChecksum)
	}

	responseData := data[12 : 12+int(dataLen)]
	result := make(map[string]interface{})

	// Bug #7 修复：按命令类型解码二进制载荷，而非尝试 JSON
	switch cmd {
	case CMD_ACK:
		// ACK 无载荷，表示成功
		result["success"] = true
		if dataLen >= 4 {
			result["task_id"] = float64(binary.LittleEndian.Uint32(responseData[0:4]))
		}

	case CMD_ERROR_RESP:
		// 错误响应：ErrorCode(1) + DetailLen(2) + detail string
		result["success"] = false
		if len(responseData) >= 1 {
			result["error_code"] = responseData[0]
		}
		if len(responseData) >= 3 {
			detailLen := binary.LittleEndian.Uint16(responseData[1:3])
			if len(responseData) >= 3+int(detailLen) {
				result["error"] = string(responseData[3 : 3+detailLen])
			}
		}

	case CMD_GET_STATUS:
		// StatusResponse 布局（与 binary_protocol.go 保持一致）：
		// Status(1) + Error(1) + PaperPages(2) + TonerPercent(2) +
		// Temperature(1) + PageCount(4) + QueueSize(2) + CurrentTaskID(1) + Reserved(3)
		if len(responseData) >= 17 {
			statusByte := responseData[0]
			statusStr := "idle"
			switch statusByte {
			case 1:
				statusStr = "printing"
			case 2:
				statusStr = "error"
			case 3:
				statusStr = "paused"
			}
			result["success"] = true
			result["status"] = statusStr
			result["error_code"] = responseData[1]
			result["paper_pages"] = float64(binary.LittleEndian.Uint16(responseData[2:4]))
			result["toner_percent"] = float64(binary.LittleEndian.Uint16(responseData[4:6]))
			result["temperature"] = float64(responseData[6])
			result["page_count"] = float64(binary.LittleEndian.Uint32(responseData[7:11]))
			result["queue_size"] = float64(binary.LittleEndian.Uint16(responseData[11:13]))
			result["current_task_id"] = float64(responseData[13])
		}

	case CMD_GET_QUEUE:
		// 队列响应：Count(2) + TaskProgress 条目数组
		result["success"] = true
		if len(responseData) >= 2 {
			count := binary.LittleEndian.Uint16(responseData[0:2])
			result["queue_size"] = float64(count)
		}

	default:
		// 未知响应类型，返回成功+原始字节（供调试）
		result["success"] = true
		result["raw_data"] = responseData
	}

	return result, nil
}

// readFullPacket 从 TCP 连接中完整读取一个二进制协议数据包
// Bug #6 修复：单次 conn.Read() 不保证读到完整包，需先读头再读剩余字节
func readFullPacket(conn net.Conn) ([]byte, error) {
	// 先读 12 字节头部
	header := make([]byte, 12)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, fmt.Errorf("读取协议头失败: %w", err)
	}

	// 验证 magic
	magic := binary.LittleEndian.Uint32(header[0:4])
	if magic != PROTOCOL_MAGIC {
		return nil, fmt.Errorf("魔法数字错误: 0x%X", magic)
	}

	// 读取 Length（偏移 6-7）
	dataLen := binary.LittleEndian.Uint16(header[6:8])

	// 读取 payload + 4 字节校验和
	rest := make([]byte, int(dataLen)+4)
	if _, err := io.ReadFull(conn, rest); err != nil {
		return nil, fmt.Errorf("读取协议载荷失败: %w", err)
	}

	return append(header, rest...), nil
}

// ==================== 驱动客户端相关 ====================

// DriverClient 与 C 驱动通信的客户端
type DriverClient struct {
	addr     string
	mu       sync.Mutex
	sequence uint32 // 命令序列号
}

// NewDriverClient 创建新的驱动客户端
func NewDriverClient(addr string) *DriverClient {
	return &DriverClient{
		addr:     addr,
		sequence: 0,
	}
}

// sendBinaryCommand 使用二进制协议发送命令
func (dc *DriverClient) sendBinaryCommand(cmdType byte, filename string, pages int, taskID int, errorType int) (map[string]interface{}, error) {
	dc.mu.Lock()
	dc.sequence++
	sequence := dc.sequence
	dc.mu.Unlock()

	// 连接到驱动服务器
	conn, err := net.Dial("tcp", dc.addr)
	if err != nil {
		return nil, fmt.Errorf("无法连接到驱动: %v", err)
	}
	defer conn.Close()

	// 构建请求
	var request []byte
	switch cmdType {
	case CMD_GET_STATUS:
		request = encodeGetStatusRequest(sequence)
	case CMD_GET_QUEUE:
		request = encodeGetQueueRequest(sequence)
	case CMD_SUBMIT_JOB:
		request = encodeSubmitJobRequest(filename, pages, sequence)
	case CMD_CANCEL_JOB:
		request = encodeCancelJobRequest(taskID, sequence)
	case CMD_PAUSE_JOB:
		request = encodePauseJobRequest(taskID, sequence)
	case CMD_RESUME_JOB:
		request = encodeResumeJobRequest(taskID, sequence)
	case CMD_REFILL_PAPER:
		request = encodeRefillPaperRequest(pages, sequence)
	case CMD_REFILL_TONER:
		request = encodeRefillTonerRequest(sequence)
	case CMD_CLEAR_ERROR:
		request = encodeClearErrorRequest(sequence)
	case CMD_SIMULATE_ERROR:
		request = encodeSimulateErrorRequest(errorType, sequence)
	default:
		return nil, fmt.Errorf("不支持的命令类型: %d", cmdType)
	}

	// 发送二进制请求
	log.Printf("[DriverClient] 发送二进制请求 (命令: %d, 长度: %d 字节)", cmdType, len(request))
	_, err = conn.Write(request)
	if err != nil {
		return nil, err
	}

	// Bug #6 修复：使用 readFullPacket 确保从 TCP 流中读取完整数据包
	responseBytes, err := readFullPacket(conn)
	if err != nil {
		return nil, fmt.Errorf("读取驱动响应失败: %v", err)
	}

	log.Printf("[DriverClient] 接收二进制响应 (长度: %d 字节)", len(responseBytes))

	// 解析二进制响应
	result, err := parseBinaryResponse(responseBytes)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return result, nil
}

// sendCommand 发送命令到驱动程序（完全二进制协议支持）
func (dc *DriverClient) sendCommand(cmd map[string]interface{}) (map[string]interface{}, error) {
	if cmdVal, ok := cmd["cmd"]; ok {
		cmdStr := fmt.Sprintf("%v", cmdVal)

		// 提取通用参数
		var filename string
		var pages, taskID, errorType int = 0, 0, 0
		if f, ok := cmd["filename"]; ok {
			filename = fmt.Sprintf("%v", f)
		}
		if p, ok := cmd["pages"]; ok {
			switch v := p.(type) {
			case int:
				pages = v
			case float64:
				pages = int(v)
			}
		}
		if tid, ok := cmd["task_id"]; ok {
			switch v := tid.(type) {
			case int:
				taskID = v
			case float64:
				taskID = int(v)
			}
		}
		if et, ok := cmd["error_type"]; ok {
			switch v := et.(type) {
			case int:
				errorType = v
			case float64:
				errorType = int(v)
			}
		}

		// 所有命令都使用二进制协议
		switch cmdStr {
		case "get_status":
			return dc.sendBinaryCommand(CMD_GET_STATUS, "", 0, 0, 0)
		case "get_queue":
			return dc.sendBinaryCommand(CMD_GET_QUEUE, "", 0, 0, 0)
		case "submit_job":
			return dc.sendBinaryCommand(CMD_SUBMIT_JOB, filename, pages, 0, 0)
		case "cancel_job":
			return dc.sendBinaryCommand(CMD_CANCEL_JOB, "", 0, taskID, 0)
		case "pause_job":
			return dc.sendBinaryCommand(CMD_PAUSE_JOB, "", 0, taskID, 0)
		case "resume_job":
			return dc.sendBinaryCommand(CMD_RESUME_JOB, "", 0, taskID, 0)
		case "refill_paper":
			return dc.sendBinaryCommand(CMD_REFILL_PAPER, "", pages, 0, 0)
		case "refill_toner":
			return dc.sendBinaryCommand(CMD_REFILL_TONER, "", 0, 0, 0)
		case "clear_error":
			return dc.sendBinaryCommand(CMD_CLEAR_ERROR, "", 0, 0, 0)
		case "simulate_error":
			return dc.sendBinaryCommand(CMD_SIMULATE_ERROR, "", 0, 0, errorType)
		default:
			return nil, fmt.Errorf("不支持的命令: %s", cmdStr)
		}
	}

	// 默认使用 JSON 回退
	return dc.sendJSONCommand(cmd)
}

// sendJSONCommand 使用 JSON 格式发送命令（回退方案）
func (dc *DriverClient) sendJSONCommand(cmd map[string]interface{}) (map[string]interface{}, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	log.Printf("[DriverClient] 使用 JSON 回退模式发送命令: %v", cmd)

	// 连接到驱动服务器
	conn, err := net.Dial("tcp", dc.addr)
	if err != nil {
		return nil, fmt.Errorf("无法连接到驱动: %v", err)
	}
	defer conn.Close()

	// 发送命令
	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}

	_, err = conn.Write(data)
	if err != nil {
		return nil, err
	}

	// 接收响应
	response := make([]byte, 8192)
	n, err := conn.Read(response)
	if err != nil && err != io.EOF {
		return nil, err
	}

	// 解析响应
	var result map[string]interface{}
	err = json.Unmarshal(response[:n], &result)
	if err != nil {
		return nil, fmt.Errorf("无法解析驱动响应: %v", err)
	}

	return result, nil
}

// ==================== HTTP 处理器相关 ====================

// PrinterHandler 打印机处理器
type PrinterHandler struct {
	driver          *DriverClient
	db              *Database      // SQLite for backward compatibility
	mysqlDB         *MySQLDatabase // MySQL for production
	tokenMgr        *TokenManager
	wsHub           *WebSocketHub
	printQueue      *PrintJobQueue
	progressTracker *ProgressTracker
	pdfManager      *PDFManager
	nextTaskID      int
	nextTaskIDMu    sync.Mutex
}

// NewPrinterHandler 创建新的打印机处理器
func NewPrinterHandler(driver *DriverClient, db *Database, tokenMgr *TokenManager, wsHub *WebSocketHub) *PrinterHandler {
	handler := &PrinterHandler{
		driver:     driver,
		db:         db,
		tokenMgr:   tokenMgr,
		wsHub:      wsHub,
		printQueue: NewPrintJobQueue(),
		nextTaskID: 1,
	}

	// 启动状态同步goroutine
	go handler.statusSyncLoop()

	return handler
}

// statusSyncLoop 状态同步循环
func (ph *PrinterHandler) statusSyncLoop() {
	ticker := time.NewTicker(2 * time.Second) // 每2秒同步一次
	defer ticker.Stop()

	for range ticker.C {
		ph.syncDriverStatus()
	}
}

// syncDriverStatus 从驱动程序同步状态
func (ph *PrinterHandler) syncDriverStatus() {
	result, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd": "get_status",
	})

	if err != nil {
		log.Printf("[Backend] 状态同步失败: %v", err)
		return
	}

	// 解析驱动程序状态
	if status, ok := result["status"].(string); ok {
		// 这里可以根据需要处理状态变化
		log.Printf("[Backend] 驱动状态: %s", status)
	}

	// 同步任务状态（如果驱动程序支持）
	queueResult, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd": "get_queue",
	})

	if err != nil {
		log.Printf("[Backend] 队列同步失败: %v", err)
		return
	}

	// 更新任务状态（这里可以扩展为更复杂的同步逻辑）
	ph.updateTaskStatuses(queueResult)
}

// updateTaskStatuses 根据驱动程序队列更新任务状态
func (ph *PrinterHandler) updateTaskStatuses(queueResult map[string]interface{}) {
	// 这里可以实现更复杂的同步逻辑
	// 例如：检查驱动程序中的任务状态，更新后端队列
}

// getNextTaskID 获取下一个任务ID
func (ph *PrinterHandler) getNextTaskID() int {
	ph.nextTaskIDMu.Lock()
	defer ph.nextTaskIDMu.Unlock()
	id := ph.nextTaskID
	ph.nextTaskID++
	return id
}

// getTokenInfo 从请求中获取 Token 信息
func (ph *PrinterHandler) getTokenInfo(r *http.Request) (TokenInfo, bool) {
	token := r.Header.Get("Authorization")
	if token == "" {
		// 检查 cookie
		cookie, err := r.Cookie("auth_token")
		if err == nil {
			token = cookie.Value
		}
	}
	if token == "" {
		return TokenInfo{}, false
	}
	// 移除 "Bearer " 前缀
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	return ph.tokenMgr.VerifyToken(token)
}

// Login 登录
func (ph *PrinterHandler) Login(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 用户登录")

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "{\"error\": \"解析请求失败\"}", http.StatusBadRequest)
		return
	}

	verified, err := ph.db.VerifyUser(req.Username, req.Password)
	if err != nil || !verified {
		http.Error(w, "{\"error\": \"用户名或密码错误\"}", http.StatusUnauthorized)
		ph.db.RecordAuditLog(req.Username, "login_failed", "Invalid credentials")
		return
	}

	role, err := ph.db.GetUserRole(req.Username)
	if err != nil {
		http.Error(w, "{\"error\": \"获取用户角色失败\"}", http.StatusInternalServerError)
		return
	}

	token := ph.tokenMgr.GenerateToken(req.Username, role, 24*time.Hour)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Set-Cookie", fmt.Sprintf("auth_token=%s; Path=/; HttpOnly; Max-Age=86400", token))
	json.NewEncoder(w).Encode(map[string]string{"token": token, "role": role})

	ph.db.RecordAuditLog(req.Username, "login_success", "")
}

// Logout 登出
func (ph *PrinterHandler) Logout(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 用户登出")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	// Fix Bug 7: 同时撤销 Header 和 Cookie 中携带的 token
	token := r.Header.Get("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	if token != "" {
		ph.tokenMgr.RevokeToken(token)
	}
	// 同时检查 Cookie
	if cookie, err := r.Cookie("auth_token"); err == nil && cookie.Value != "" {
		ph.tokenMgr.RevokeToken(cookie.Value)
	}
	// 清除客户端 Cookie
	w.Header().Set("Set-Cookie", "auth_token=; Path=/; HttpOnly; Max-Age=0")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})

	ph.db.RecordAuditLog(tokenInfo.Username, "logout", "")
}

// GetPrintHistory 获取打印历史
func (ph *PrinterHandler) GetPrintHistory(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 获取打印历史")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	limit := 100
	var userID string

	// 如果是管理员，可以查看所有记录；否则只查看自己的
	if tokenInfo.Role != "admin" {
		userID = tokenInfo.Username
	}

	history, err := ph.db.GetPrintHistory(userID, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"history": history})

	ph.db.RecordAuditLog(tokenInfo.Username, "view_history", "")
}

// GetStatus 获取打印机状态
func (ph *PrinterHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 获取打印机状态")

	result, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd": "get_status",
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetQueue 获取打印队列
func (ph *PrinterHandler) GetQueue(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 获取打印队列")

	// 将堆中的任务按优先级排序后添加到队列
	ph.printQueue.mu.RLock()
	queue := make([]map[string]interface{}, 0, len(ph.printQueue.jobs))

	// Bug #3 修复: 使用高效的快速排序而非O(n²)冒泡排序
	sortedJobs := make([]*PrintJob, 0, len(ph.printQueue.heap))
	sortedJobs = append(sortedJobs, ph.printQueue.heap...)

	// 快速排序（按优先级降序）
	sort.Slice(sortedJobs, func(i, j int) bool {
		return sortedJobs[i].Priority > sortedJobs[j].Priority
	})

	for _, job := range sortedJobs {
		queue = append(queue, map[string]interface{}{
			"task_id":  job.TaskID,
			"filename": job.Filename,
			"pages":    job.Pages,
			"status":   job.Status,
		})
	}
	ph.printQueue.mu.RUnlock()

	response := map[string]interface{}{
		"queue":      queue,
		"queue_size": len(queue),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SubmitJob 提交打印任务
func (ph *PrinterHandler) SubmitJob(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 提交打印任务")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	var req struct {
		Filename string `json:"filename"`
		Pages    int    `json:"pages"`
		Priority int    `json:"priority"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "{\"error\": \"解析请求失败\"}", http.StatusBadRequest)
		return
	}

	// Bug #5 修复: 确保任务ID始终与驱动同步
	driverResult, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd":      "submit_job",
		"filename": req.Filename,
		"pages":    req.Pages,
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	// 验证驱动成功，优先使用驱动分配的 task_id
	driverSuccess, okSuccess := driverResult["success"].(bool)
	if !okSuccess || !driverSuccess {
		http.Error(w, "{\"error\": \"驱动程序提交失败\"}", http.StatusInternalServerError)
		return
	}

	var taskID int
	if driverID, ok2 := driverResult["task_id"].(float64); ok2 && driverID > 0 {
		taskID = int(driverID)
	} else {
		// 驱动未返回有效ID，使用本地ID
		taskID = ph.getNextTaskID()
	}

	// 记录到数据库
	ph.db.RecordPrintJob(taskID, req.Filename, req.Pages, tokenInfo.Username, req.Priority)

	// 计算实际优先级（管理员优先级最高）
	actualPriority := req.Priority
	if tokenInfo.Role == "admin" {
		actualPriority = req.Priority + 1000 // 管理员任务额外加 1000
	}

	// 加入优先级队列
	job := &PrintJob{
		TaskID:    taskID,
		Filename:  req.Filename,
		Pages:     req.Pages,
		Priority:  actualPriority,
		Status:    "submitted",
		CreatedAt: time.Now(),
		UserID:    tokenInfo.Username,
	}
	ph.printQueue.Enqueue(job)

	// 广播到 WebSocket 客户端
	ph.wsHub.Broadcast(map[string]interface{}{
		"event":    "job_submitted",
		"task_id":  taskID,
		"filename": req.Filename,
		"pages":    req.Pages,
		"priority": req.Priority,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"task_id": taskID,
	})

	ph.db.RecordAuditLog(tokenInfo.Username, "submit_job", fmt.Sprintf("task_id=%d", taskID))
}

// CancelJob 取消打印任务
func (ph *PrinterHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 取消打印任务")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	var req struct {
		TaskID int `json:"task_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "{\"error\": \"解析请求失败\"}", http.StatusBadRequest)
		return
	}

	// Bug #8 修复：访问 jobs map 前必须持有读锁，防止数据竞争
	ph.printQueue.mu.RLock()
	job, exists := ph.printQueue.jobs[req.TaskID]
	ph.printQueue.mu.RUnlock()

	if exists && tokenInfo.Role != "admin" && job.UserID != tokenInfo.Username {
		http.Error(w, "{\"error\": \"没有权限删除此任务\"}", http.StatusForbidden)
		return
	}

	result, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd":     "cancel_job",
		"task_id": req.TaskID,
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	// 检查驱动程序的响应
	success, ok := result["success"].(bool)
	if !ok {
		http.Error(w, "{\"error\": \"驱动程序响应格式错误\"}", http.StatusInternalServerError)
		return
	}

	// 只有当驱动程序成功取消时，才更新后端状态
	if success {
		// Fix Bug 6: 从内存队列中同时移除 map 和堆中的对应条目
		ph.printQueue.mu.Lock()
		delete(ph.printQueue.jobs, req.TaskID)
		// 重建堆，过滤掉已取消的任务
		newHeap := make([]*PrintJob, 0, len(ph.printQueue.heap))
		for _, j := range ph.printQueue.heap {
			if j.TaskID != req.TaskID {
				newHeap = append(newHeap, j)
			}
		}
		ph.printQueue.heap = newHeap
		ph.printQueue.mu.Unlock()

		// 更新数据库
		ph.db.UpdatePrintJob(req.TaskID, 0, "cancelled")

		// 广播到 WebSocket 客户端
		ph.wsHub.Broadcast(map[string]interface{}{
			"event":   "job_cancelled",
			"task_id": req.TaskID,
		})

		ph.db.RecordAuditLog(tokenInfo.Username, "cancel_job", fmt.Sprintf("task_id=%d", req.TaskID))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// RefillPaper 补充纸张
func (ph *PrinterHandler) RefillPaper(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 补充纸张")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	// 检查是否有管理员权限
	if tokenInfo.Role != "admin" && tokenInfo.Role != "technician" {
		http.Error(w, "{\"error\": \"没有权限\"}", http.StatusForbidden)
		return
	}

	var req struct {
		Pages int `json:"pages"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "{\"error\": \"解析请求失败\"}", http.StatusBadRequest)
		return
	}

	result, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd":   "refill_paper",
		"pages": req.Pages,
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	// 广播到 WebSocket 客户端
	ph.wsHub.Broadcast(map[string]interface{}{
		"event": "paper_refilled",
		"pages": req.Pages,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)

	ph.db.RecordAuditLog(tokenInfo.Username, "refill_paper", fmt.Sprintf("pages=%d", req.Pages))
}

// RefillToner 补充碳粉
func (ph *PrinterHandler) RefillToner(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 补充碳粉")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	// 检查是否有管理员权限
	if tokenInfo.Role != "admin" && tokenInfo.Role != "technician" {
		http.Error(w, "{\"error\": \"没有权限\"}", http.StatusForbidden)
		return
	}

	result, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd": "refill_toner",
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	// 广播到 WebSocket 客户端
	ph.wsHub.Broadcast(map[string]interface{}{
		"event": "toner_refilled",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)

	ph.db.RecordAuditLog(tokenInfo.Username, "refill_toner", "")
}

// ClearError 清除错误
func (ph *PrinterHandler) ClearError(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 清除错误")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	result, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd": "clear_error",
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	// 广播到 WebSocket 客户端
	ph.wsHub.Broadcast(map[string]interface{}{
		"event": "error_cleared",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)

	ph.db.RecordAuditLog(tokenInfo.Username, "clear_error", "")
}

// SimulateError 模拟硬件错误
func (ph *PrinterHandler) SimulateError(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 模拟硬件错误")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	// 检查是否有管理员权限
	if tokenInfo.Role != "admin" {
		http.Error(w, "{\"error\": \"没有权限\"}", http.StatusForbidden)
		return
	}

	var req struct {
		Error string `json:"error"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "{\"error\": \"解析请求失败\"}", http.StatusBadRequest)
		return
	}

	// 将错误类型转换为数值（根据C驱动中定义的错误类型）
	errorTypeMap := map[string]int{
		"paper_jam":   1,
		"paper_empty": 2,
		"toner_low":   3,
		"temperature": 4,
	}
	errorType := errorTypeMap[req.Error]
	if errorType == 0 && req.Error != "" {
		// 默认错误类型
		errorType = 1
	}

	result, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd":        "simulate_error",
		"error_type": errorType,
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	// 广播到 WebSocket 客户端
	ph.wsHub.Broadcast(map[string]interface{}{
		"event": "error_simulated",
		"error": req.Error,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)

	ph.db.RecordAuditLog(tokenInfo.Username, "simulate_error", fmt.Sprintf("error=%s", req.Error))
}

// PauseJob 暂停打印任务
func (ph *PrinterHandler) PauseJob(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 暂停打印任务")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	var req struct {
		TaskID int `json:"task_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "{\"error\": \"解析请求失败\"}", http.StatusBadRequest)
		return
	}

	// Fix Bug 8a: 检查队列中的任务（队列中未开始的任务）
	ph.printQueue.mu.RLock()
	job, inQueue := ph.printQueue.jobs[req.TaskID]
	ph.printQueue.mu.RUnlock()

	if inQueue && tokenInfo.Role != "admin" && job.UserID != tokenInfo.Username {
		http.Error(w, "{\"error\": \"没有权限暂停此任务\"}", http.StatusForbidden)
		return
	}

	// 无论是否在内存队列，都向驱动发送暂停指令（正在打印的任务由驱动直接管理，不在 Go 队列中）
	_, driverErr := ph.driver.sendCommand(map[string]interface{}{
		"cmd":     "pause_job",
		"task_id": req.TaskID,
	})
	if driverErr != nil {
		log.Printf("[Backend] 驱动程序暂停失败: %v", driverErr)
		if !inQueue {
			http.Error(w, "{\"error\": \"任务不存在\"}", http.StatusNotFound)
			return
		}
	}

	if inQueue {
		job.Status = "paused"
	}
	ph.db.UpdatePrintJob(req.TaskID, 0, "paused")

	// 广播到 WebSocket 客户端
	ph.wsHub.Broadcast(map[string]interface{}{
		"event":   "job_paused",
		"task_id": req.TaskID,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
	ph.db.RecordAuditLog(tokenInfo.Username, "pause_job", fmt.Sprintf("task_id=%d", req.TaskID))
}

// ResumeJob 恢复打印任务
func (ph *PrinterHandler) ResumeJob(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 恢复打印任务")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	var req struct {
		TaskID int `json:"task_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "{\"error\": \"解析请求失败\"}", http.StatusBadRequest)
		return
	}

	// Fix Bug 8b: 检查队列中的任务（暂停的队列任务）
	ph.printQueue.mu.RLock()
	job, inQueue := ph.printQueue.jobs[req.TaskID]
	ph.printQueue.mu.RUnlock()

	if inQueue && tokenInfo.Role != "admin" && job.UserID != tokenInfo.Username {
		http.Error(w, "{\"error\": \"没有权限恢复此任务\"}", http.StatusForbidden)
		return
	}

	// 无论是否在内存队列，都向驱动发送恢复指令（暂停中的正在打印任务由驱动直接管理）
	_, driverErr := ph.driver.sendCommand(map[string]interface{}{
		"cmd":     "resume_job",
		"task_id": req.TaskID,
	})
	if driverErr != nil {
		log.Printf("[Backend] 驱动程序恢复失败: %v", driverErr)
		if !inQueue {
			http.Error(w, "{\"error\": \"任务不存在\"}", http.StatusNotFound)
			return
		}
	}

	if inQueue {
		job.Status = "printing"
	}
	ph.db.UpdatePrintJob(req.TaskID, 0, "printing")

	// 广播到 WebSocket 客户端
	ph.wsHub.Broadcast(map[string]interface{}{
		"event":   "job_resumed",
		"task_id": req.TaskID,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
	ph.db.RecordAuditLog(tokenInfo.Username, "resume_job", fmt.Sprintf("task_id=%d", req.TaskID))
}

// AddUser 添加新用户
func (ph *PrinterHandler) AddUser(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 添加新用户")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	// 只有管理员可以添加用户
	if tokenInfo.Role != "admin" {
		http.Error(w, "{\"error\": \"只有管理员可以添加用户\"}", http.StatusForbidden)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "{\"error\": \"解析请求失败\"}", http.StatusBadRequest)
		return
	}

	// 验证输入
	if req.Username == "" || req.Password == "" {
		http.Error(w, "{\"error\": \"用户名和密码不能为空\"}", http.StatusBadRequest)
		return
	}

	// Bug #9 修复: 增强用户输入验证
	if len(req.Username) < 3 || len(req.Username) > 32 {
		http.Error(w, "{\"error\": \"用户名长度必须在3-32个字符之间\"}", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 || len(req.Password) > 128 {
		http.Error(w, "{\"error\": \"密码长度必须在8-128个字符之间\"}", http.StatusBadRequest)
		return
	}

	if req.Role == "" {
		req.Role = "user"
	}

	// 验证角色值
	validRoles := map[string]bool{"user": true, "technician": true, "admin": true}
	if !validRoles[req.Role] {
		http.Error(w, "{\"error\": \"无效的角色类型\"}", http.StatusBadRequest)
		return
	}

	// 检查用户是否已存在
	exists, err := ph.db.UserExists(req.Username)
	if err != nil {
		http.Error(w, "{\"error\": \"数据库错误\"}", http.StatusInternalServerError)
		return
	}

	if exists {
		http.Error(w, "{\"error\": \"用户已存在\"}", http.StatusConflict)
		return
	}

	// 创建用户
	err = ph.db.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		http.Error(w, "{\"error\": \"创建用户失败\"}", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"success": "true", "message": "用户已创建"})

	ph.db.RecordAuditLog(tokenInfo.Username, "add_user", fmt.Sprintf("username=%s,role=%s", req.Username, req.Role))
}

// DeleteUserHandler 删除用户
func (ph *PrinterHandler) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 删除用户")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	// 只有管理员可以删除用户
	if tokenInfo.Role != "admin" {
		http.Error(w, "{\"error\": \"只有管理员可以删除用户\"}", http.StatusForbidden)
		return
	}

	var req struct {
		Username string `json:"username"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "{\"error\": \"解析请求失败\"}", http.StatusBadRequest)
		return
	}

	// 防止删除自己
	if req.Username == tokenInfo.Username {
		http.Error(w, "{\"error\": \"不能删除自己\"}", http.StatusBadRequest)
		return
	}

	// 删除用户
	err = ph.db.DeleteUser(req.Username)
	if err != nil {
		http.Error(w, "{\"error\": \"删除用户失败\"}", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"success": "true", "message": "用户已删除"})

	ph.db.RecordAuditLog(tokenInfo.Username, "delete_user", fmt.Sprintf("username=%s", req.Username))
}

// ListUsersHandler 列出所有用户
func (ph *PrinterHandler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 列出用户")

	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	// 只有管理员可以列出用户
	if tokenInfo.Role != "admin" {
		http.Error(w, "{\"error\": \"只有管理员可以查看用户列表\"}", http.StatusForbidden)
		return
	}

	users, err := ph.db.ListUsers()
	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": users})

	ph.db.RecordAuditLog(tokenInfo.Username, "list_users", "")
}

// Health 健康检查
func (ph *PrinterHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// GetStats 获取系统统计信息
func (ph *PrinterHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 获取系统统计信息")

	result, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd": "get_status",
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusInternalServerError)
		return
	}

	// 解析页数信息
	pageCount := 0
	if val, ok := result["page_count"]; ok {
		if fval, ok := val.(float64); ok {
			pageCount = int(fval)
		}
	}

	stats := map[string]interface{}{
		"total_pages_printed": pageCount,
		"timestamp":           time.Now().Format(time.RFC3339),
		"uptime":              "running",
		"queue_size":          ph.printQueue.GetQueueSize(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// WebSocket 处理器
func (ph *PrinterHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] WebSocket 连接请求")

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("[WebSocket] 升级失败:", err)
		return
	}

	client := &WebSocketClient{
		hub:  ph.wsHub,
		conn: conn,
		send: make(chan interface{}, 256),
	}

	ph.wsHub.register <- client

	// 如果是进度通知路由，注册进度监听器
	path := r.URL.Path
	isProgressRoute := path == "/ws/progress" || strings.Contains(path, "progress")

	var progressListener chan *PrintJobNotification
	var clientID string

	if isProgressRoute && ph.progressTracker != nil {
		clientID = fmt.Sprintf("client_%s_%.0f", conn.RemoteAddr(), float64(time.Now().UnixNano()))
		progressListener = ph.progressTracker.RegisterListener(clientID)
		log.Printf("[WebSocket] 进度监听器已注册: %s", clientID)
	}

	// 处理客户端消息
	go func() {
		defer func() {
			ph.wsHub.unregister <- client
			if isProgressRoute && ph.progressTracker != nil && clientID != "" {
				ph.progressTracker.UnregisterListener(clientID)
				log.Printf("[WebSocket] 进度监听器已注销: %s", clientID)
			}
			conn.Close()
		}()

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		for {
			var msg map[string]interface{}
			err := conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[WebSocket] 错误: %v", err)
				}
				return
			}

			// Bug #2 修复: 处理客户端消息（心跳和订阅管理）
			if msgType, ok := msg["type"].(string); ok {
				switch msgType {
				case "ping":
					client.send <- map[string]string{"type": "pong"}
				case "subscribe":
					client.send <- map[string]string{"type": "subscribed"}
				case "get_progress":
					if ph.progressTracker != nil {
						// 发送所有活跃任务的进度
						_ = msg // 占位符
					}
				default:
					log.Printf("[WebSocket] 未知消息类型: %s", msgType)
				}
			}
		}
	}()

	// 发送消息给客户端
	go func() {
		defer conn.Close()
		for {
			select {
			case message, ok := <-client.send:
				// Bug #10 修复：client.send 关闭后 ok=false，须退出，否则会无限循环读零值
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteJSON(message); err != nil {
					return
				}
			case progressNotif, ok := <-progressListener:
				if progressListener == nil {
					continue
				}
				if !ok || progressNotif == nil {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteJSON(progressNotif); err != nil {
					return
				}
			}
		}
	}()
}

// GetRecentPDFs 获取最近的10个PDF（仅admin有权限）
func (ph *PrinterHandler) GetRecentPDFs(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 获取最近的PDF列表")

	// 检查权限
	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	// 仅 admin 有权限访问
	if tokenInfo.Role != "admin" {
		http.Error(w, "{\"error\": \"仅管理员有权限访问PDF历史\"}", http.StatusForbidden)
		return
	}

	if ph.pdfManager == nil {
		http.Error(w, "{\"error\": \"PDF管理器未初始化\"}", http.StatusInternalServerError)
		return
	}

	// 获取最近10个PDF
	recentPDFs := ph.pdfManager.GetRecentPDFs(10)

	// 构造响应
	type PDFInfo struct {
		TaskID    int    `json:"task_id"`
		Filename  string `json:"filename"`
		FileSize  int64  `json:"file_size"`
		FileHash  string `json:"file_hash"`
		CreatedAt string `json:"created_at"`
	}

	var pdfList []PDFInfo
	for _, pdf := range recentPDFs {
		pdfList = append(pdfList, PDFInfo{
			TaskID:    pdf.TaskID,
			Filename:  pdf.Filename,
			FileSize:  pdf.FileSize,
			FileHash:  pdf.FileHash,
			CreatedAt: pdf.CreatedAt.Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"pdfs":  pdfList,
		"count": len(pdfList),
	})

	// 记录审计日志
	if ph.mysqlDB != nil {
		ph.mysqlDB.RecordAuditLog(tokenInfo.Username, "view_pdf_history", fmt.Sprintf("获取了%d个PDF", len(pdfList)))
	}
}

// DownloadPDF 下载PDF文件（仅admin有权限）
func (ph *PrinterHandler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	log.Println("[Backend] 请求: 下载PDF")

	// 检查权限
	tokenInfo, ok := ph.getTokenInfo(r)
	if !ok {
		http.Error(w, "{\"error\": \"未授权\"}", http.StatusUnauthorized)
		return
	}

	// 仅 admin 有权限下载
	if tokenInfo.Role != "admin" {
		http.Error(w, "{\"error\": \"仅管理员有权限下载PDF\"}", http.StatusForbidden)
		return
	}

	if ph.pdfManager == nil {
		http.Error(w, "{\"error\": \"PDF管理器未初始化\"}", http.StatusInternalServerError)
		return
	}

	// 从查询参数获取 task_id
	taskIDStr := r.URL.Query().Get("task_id")
	if taskIDStr == "" {
		http.Error(w, "{\"error\": \"缺少task_id参数\"}", http.StatusBadRequest)
		return
	}

	taskID, err := strconv.Atoi(taskIDStr)
	if err != nil {
		http.Error(w, "{\"error\": \"无效的task_id\"}", http.StatusBadRequest)
		return
	}

	// 获取PDF文件
	pdfData, err := ph.pdfManager.RetrievePDF(taskID)
	if err != nil {
		http.Error(w, fmt.Sprintf("{\"error\": \"%v\"}", err), http.StatusNotFound)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"print_task_%d.pdf\"", taskID))
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfData)))

	// 发送PDF数据
	w.Write(pdfData)

	// 记录审计日志
	if ph.mysqlDB != nil {
		ph.mysqlDB.RecordAuditLog(tokenInfo.Username, "download_pdf", fmt.Sprintf("下载了任务%d的PDF", taskID))
	}
}

func main() {
	fmt.Println("================================")
	fmt.Println("  网络打印机后端服务 v1.0")
	fmt.Println("================================")

	// 初始化数据库层（支持 MySQL 和 SQLite）
	var sqliteDB *Database
	var mysqlDB *MySQLDatabase

	// 尝试初始化 MySQL 数据库
	var dbErr error
	mysqlDB, dbErr = NewMySQLDatabase("root", "nihao", "localhost", "3306", "printer_db")
	if dbErr != nil {
		log.Printf("[Warning] MySQL 初始化失败: %v\n", dbErr)
		log.Println("[Info] 降级到使用 SQLite 数据库...")

		// 降级方案：使用 SQLite
		sqliteDB, dbErr = NewDatabase("printer.db")
		if dbErr != nil {
			log.Fatal("SQLite 初始化失败:", dbErr)
		}

		// 创建默认用户
		sqliteDB.CreateUser("admin", "admin123", "admin")
		sqliteDB.CreateUser("user", "user123", "user")
		sqliteDB.CreateUser("technician", "tech123", "technician")

		log.Println("[Info] 使用 SQLite 作为主数据库")
	} else {
		// 创建默认用户
		mysqlDB.CreateUser("admin", "admin123", "admin")
		mysqlDB.CreateUser("user", "user123", "user")
		mysqlDB.CreateUser("technician", "tech123", "technician")

		log.Println("[Info] 使用 MySQL 作为主数据库")
	}

	// 初始化进度追踪器
	progressTracker := NewProgressTracker()

	// 初始化 PDF 管理器
	pdfManager, err := NewPDFManager("./pdf_storage", 10, 1024) // 最多10个文件，1GB限制
	if err != nil {
		log.Printf("[Warning] PDF 管理器初始化失败: %v\n", err)
	}

	// 启动进度追踪器的后台通知处理
	go func() {
		for notification := range progressTracker.notifyChan {
			// 处理通知... 这里可以集成到WebSocket
			_ = notification
		}
	}()

	// 创建驱动客户端
	driver := NewDriverClient("localhost:9999")

	// 创建 Token 管理器
	tokenMgr := NewTokenManager()

	// 创建 WebSocket Hub
	wsHub := NewWebSocketHub()
	go wsHub.Run()

	// 创建处理器（使用 SQLite 作为主数据库）
	var primaryDB *Database
	if sqliteDB != nil {
		primaryDB = sqliteDB
	} else {
		// 创建一个临时的 SQLite 数据库用于兼容性
		// 实际上 MySQLDatabase 可以和 Database 一起使用，但需要分开
		primaryDB, _ = NewDatabase(":memory:")
	}

	handler := NewPrinterHandler(driver, primaryDB, tokenMgr, wsHub)

	// 设置 MySQL 数据库（如果可用）
	handler.mysqlDB = mysqlDB

	// 设置进度追踪器和 PDF 管理器
	handler.progressTracker = progressTracker
	handler.pdfManager = pdfManager

	// CORS 中间件（修正版）
	// 注意：Access-Control-Allow-Origin: * 与 Allow-Credentials: true 不可共存
	// 对 file:// 页面（Origin 为 "null"）需反射 "null"，而非使用通配符 *
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" || origin == "null" {
				w.Header().Set("Access-Control-Allow-Origin", "null")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	// 创建路由
	r := mux.NewRouter()
	r.Use(corsMiddleware)

	// 托管前端 HTML（推荐：从 http://localhost:8080 打开页面，彻底避免跨域问题）
	// 将 printer_control.html 与后端可执行文件放在同一目录即可
	r.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		http.ServeFile(w, req, "printer_control.html")
	}).Methods("GET")

	// 认证相关端点
	r.HandleFunc("/api/auth/login", handler.Login).Methods("POST")
	r.HandleFunc("/api/auth/logout", handler.Logout).Methods("POST")

	// WebSocket 端点
	r.HandleFunc("/ws", handler.HandleWebSocket)

	// API 端点
	r.HandleFunc("/health", handler.Health).Methods("GET")
	r.HandleFunc("/api/status", handler.GetStatus).Methods("GET")
	r.HandleFunc("/api/queue", handler.GetQueue).Methods("GET")
	r.HandleFunc("/api/stats", handler.GetStats).Methods("GET")
	r.HandleFunc("/api/history", handler.GetPrintHistory).Methods("GET")
	r.HandleFunc("/api/job/submit", handler.SubmitJob).Methods("POST")
	r.HandleFunc("/api/job/cancel", handler.CancelJob).Methods("POST")
	r.HandleFunc("/api/supplies/refill-paper", handler.RefillPaper).Methods("POST")
	r.HandleFunc("/api/supplies/refill-toner", handler.RefillToner).Methods("POST")
	r.HandleFunc("/api/error/clear", handler.ClearError).Methods("POST")
	r.HandleFunc("/api/error/simulate", handler.SimulateError).Methods("POST")

	// 任务管理端点
	r.HandleFunc("/api/job/pause", handler.PauseJob).Methods("POST")
	r.HandleFunc("/api/job/resume", handler.ResumeJob).Methods("POST")

	// PDF 管理端点（仅admin有权限）
	r.HandleFunc("/api/pdf/recent", handler.GetRecentPDFs).Methods("GET")
	r.HandleFunc("/api/pdf/download", handler.DownloadPDF).Methods("GET")

	// 用户管理端点
	r.HandleFunc("/api/user/add", handler.AddUser).Methods("POST")
	r.HandleFunc("/api/user/delete", handler.DeleteUserHandler).Methods("POST")
	r.HandleFunc("/api/user/list", handler.ListUsersHandler).Methods("GET")

	// 启动服务器
	port := 8080
	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("\n[Backend] 服务启动成功，监听端口 %d\n", port)
	fmt.Printf("[Backend] WebSocket 地址: ws://localhost:%d/ws\n", port)

	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
