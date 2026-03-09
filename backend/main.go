package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// ==================== 数据库相关 ====================

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

// ==================== 驱动客户端相关 ====================

// DriverClient 与 C 驱动通信的客户端
type DriverClient struct {
	addr string
	mu   sync.Mutex
}

// NewDriverClient 创建新的驱动客户端
func NewDriverClient(addr string) *DriverClient {
	return &DriverClient{
		addr: addr,
	}
}

// sendCommand 发送命令到驱动程序
func (dc *DriverClient) sendCommand(cmd map[string]interface{}) (map[string]interface{}, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

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
	driver       *DriverClient
	db           *Database
	tokenMgr     *TokenManager
	wsHub        *WebSocketHub
	printQueue   *PrintJobQueue
	nextTaskID   int
	nextTaskIDMu sync.Mutex
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

	// 检查权限：只有管理员或任务所有者可以取消任务
	job, exists := ph.printQueue.jobs[req.TaskID]
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

	result, err := ph.driver.sendCommand(map[string]interface{}{
		"cmd":   "simulate_error",
		"error": req.Error,
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

	// 处理客户端消息
	go func() {
		defer func() {
			ph.wsHub.unregister <- client
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
				default:
					log.Printf("[WebSocket] 未知消息类型: %s", msgType)
				}
			}
		}
	}()

	// 发送消息给客户端
	go func() {
		defer conn.Close()
		for message := range client.send {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := conn.WriteJSON(message)
			if err != nil {
				return
			}
		}
	}()
}

func main() {
	fmt.Println("================================")
	fmt.Println("  网络打印机后端服务 v1.0")
	fmt.Println("================================")

	// 初始化数据库
	db, err := NewDatabase("printer.db")
	if err != nil {
		log.Fatal("数据库初始化失败:", err)
	}

	// 创建默认用户（如果不存在）
	db.CreateUser("admin", "admin123", "admin")
	db.CreateUser("user", "user123", "user")
	db.CreateUser("technician", "tech123", "technician")

	// 创建驱动客户端
	driver := NewDriverClient("localhost:9999")

	// 创建 Token 管理器
	tokenMgr := NewTokenManager()

	// 创建 WebSocket Hub
	wsHub := NewWebSocketHub()
	go wsHub.Run()

	// 创建处理器
	handler := NewPrinterHandler(driver, db, tokenMgr, wsHub)

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
	// 将 printer_control_improved.html 与后端可执行文件放在同一目录即可
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
