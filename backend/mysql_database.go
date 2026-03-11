/**
 * mysql_database.go
 * MySQL 数据库管理器
 * 支持打印历史记录、PDF 存储和用户管理
 */

package main

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

const (
	// MySQL 连接字符串格式
	MySQLDSNFormat = "%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true"

	// PDF 存储配置
	PDFStorageDir  = "./pdf_storage"
	MaxRecentTasks = 10
)

// MySQLDatabase MySQL 数据库管理器
type MySQLDatabase struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewMySQLDatabase 创建新的 MySQL 数据库管理器
func NewMySQLDatabase(user, password, host, port, dbname string) (*MySQLDatabase, error) {
	// 第一步：不指定数据库名连接，确保 printer_db 存在
	// 常见失败原因：数据库未创建（MySQL 不会自动创建 DB，与 SQLite 不同）
	dsnNoDB := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=true&timeout=5s",
		user, password, host, port)
	dbInit, err := sql.Open("mysql", dsnNoDB)
	if err != nil {
		return nil, fmt.Errorf("MySQL 驱动初始化失败: %w", err)
	}
	// 测试基础连接（用户名/密码/端口）
	if err := dbInit.Ping(); err != nil {
		dbInit.Close()
		return nil, fmt.Errorf("MySQL 连接测试失败（请检查用户名/密码/端口/MySQL是否启动）: %w", err)
	}
	// 自动建库
	_, err = dbInit.Exec(fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbname))
	dbInit.Close()
	if err != nil {
		return nil, fmt.Errorf("创建数据库 %s 失败: %w", dbname, err)
	}

	// 第二步：携带数据库名重新连接
	dsn := fmt.Sprintf(MySQLDSNFormat, user, password, host, port, dbname)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接MySQL失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("MySQL 连接测试失败: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// 创建表
	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	log.Printf("[MySQL] 已成功连接到 %s:%s/%s", host, port, dbname)
	return &MySQLDatabase{db: db}, nil
}

// createTables 创建数据库表结构
func createTables(db *sql.DB) error {
	// schema := `
	// -- 打印历史表
	// CREATE TABLE IF NOT EXISTS print_history (
	// 	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	// 	task_id INT NOT NULL UNIQUE,
	// 	filename VARCHAR(255) NOT NULL,
	// 	pages INT NOT NULL,
	// 	printed_pages INT DEFAULT 0,
	// 	status VARCHAR(50) NOT NULL,
	// 	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	// 	completed_at TIMESTAMP NULL,
	// 	user_id VARCHAR(100) NOT NULL,
	// 	priority INT DEFAULT 0,
	// 	pdf_path VARCHAR(255) NULL,
	// 	paper_size VARCHAR(50) DEFAULT 'A4',
	// 	print_speed INT DEFAULT 20,
	// 	duration_seconds INT DEFAULT 0,
	// 	INDEX idx_user_id (user_id),
	// 	INDEX idx_status (status),
	// 	INDEX idx_created_at (created_at)
	// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	// -- 用户表
	// CREATE TABLE IF NOT EXISTS users (
	// 	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	// 	username VARCHAR(100) NOT NULL UNIQUE,
	// 	password_hash VARCHAR(255) NOT NULL,
	// 	role VARCHAR(50) NOT NULL,
	// 	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	// 	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	// 	is_active BOOLEAN DEFAULT TRUE,
	// 	INDEX idx_username (username)
	// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	// -- 审计日志表
	// CREATE TABLE IF NOT EXISTS audit_log (
	// 	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	// 	user_id VARCHAR(100) NOT NULL,
	// 	action VARCHAR(100) NOT NULL,
	// 	details TEXT,
	// 	ip_address VARCHAR(45),
	// 	timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	// 	INDEX idx_user_id (user_id),
	// 	INDEX idx_timestamp (timestamp)
	// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	// -- 任务队列表
	// CREATE TABLE IF NOT EXISTS task_queue (
	// 	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	// 	task_id INT NOT NULL UNIQUE,
	// 	filename VARCHAR(255) NOT NULL,
	// 	pages INT NOT NULL,
	// 	priority INT DEFAULT 0,
	// 	status VARCHAR(50) NOT NULL,
	// 	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	// 	started_at TIMESTAMP NULL,
	// 	completed_at TIMESTAMP NULL,
	// 	user_id VARCHAR(100),
	// 	progress_percent INT DEFAULT 0,
	// 	INDEX idx_status (status),
	// 	INDEX idx_created_at (created_at)
	// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	// -- PDF 存储表
	// CREATE TABLE IF NOT EXISTS pdf_storage (
	// 	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	// 	task_id INT NOT NULL UNIQUE,
	// 	pdf_filename VARCHAR(255) NOT NULL,
	// 	pdf_size BIGINT NOT NULL,
	// 	file_hash VARCHAR(64),
	// 	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	// 	accessed_at TIMESTAMP NULL,
	// 	access_count INT DEFAULT 0,
	// 	INDEX idx_task_id (task_id),
	// 	INDEX idx_created_at (created_at)
	// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

	// -- 打印机状态历史表
	// CREATE TABLE IF NOT EXISTS printer_status_history (
	// 	id BIGINT PRIMARY KEY AUTO_INCREMENT,
	// 	timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	// 	status VARCHAR(50) NOT NULL,
	// 	error VARCHAR(100),
	// 	temperature INT,
	// 	paper_pages INT,
	// 	toner_percentage INT,
	// 	queue_size INT,
	// 	page_count INT,
	// 	INDEX idx_timestamp (timestamp)
	// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	// `

	// 分别执行每个CREATE TABLE语句
	statements := []string{
		`CREATE TABLE IF NOT EXISTS print_history (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			task_id INT NOT NULL UNIQUE,
			filename VARCHAR(255) NOT NULL,
			pages INT NOT NULL,
			printed_pages INT DEFAULT 0,
			status VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP NULL,
			user_id VARCHAR(100) NOT NULL,
			priority INT DEFAULT 0,
			pdf_path VARCHAR(255) NULL,
			paper_size VARCHAR(50) DEFAULT 'A4',
			print_speed INT DEFAULT 20,
			duration_seconds INT DEFAULT 0,
			INDEX idx_user_id (user_id),
			INDEX idx_status (status),
			INDEX idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			username VARCHAR(100) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			is_active BOOLEAN DEFAULT TRUE,
			INDEX idx_username (username)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS audit_log (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			user_id VARCHAR(100) NOT NULL,
			action VARCHAR(100) NOT NULL,
			details TEXT,
			ip_address VARCHAR(45),
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			INDEX idx_timestamp (timestamp)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS task_queue (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			task_id INT NOT NULL UNIQUE,
			filename VARCHAR(255) NOT NULL,
			pages INT NOT NULL,
			priority INT DEFAULT 0,
			status VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			started_at TIMESTAMP NULL,
			completed_at TIMESTAMP NULL,
			user_id VARCHAR(100),
			progress_percent INT DEFAULT 0,
			INDEX idx_status (status),
			INDEX idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS pdf_storage (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			task_id INT NOT NULL UNIQUE,
			pdf_filename VARCHAR(255) NOT NULL,
			pdf_size BIGINT NOT NULL,
			file_hash VARCHAR(64),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			accessed_at TIMESTAMP NULL,
			access_count INT DEFAULT 0,
			INDEX idx_task_id (task_id),
			INDEX idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS printer_status_history (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			status VARCHAR(50) NOT NULL,
			error VARCHAR(100),
			temperature INT,
			paper_pages INT,
			toner_percentage INT,
			queue_size INT,
			page_count INT,
			INDEX idx_timestamp (timestamp)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}

	return nil
}

// Close 关闭数据库连接
func (m *MySQLDatabase) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// RecordPrintJob 记录打印任务
func (m *MySQLDatabase) RecordPrintJob(taskID int, filename string, pages int, userID string, priority int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		INSERT INTO print_history 
		(task_id, filename, pages, status, created_at, user_id, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE 
		pages=VALUES(pages), priority=VALUES(priority)
	`, taskID, filename, pages, "submitted", time.Now(), userID, priority)

	return err
}

// UpdatePrintJob 更新打印任务
func (m *MySQLDatabase) UpdatePrintJob(taskID int, printedPages int, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	query := `UPDATE print_history SET printed_pages = ?, status = ?`
	args := []interface{}{printedPages, status, taskID}

	if status == "completed" {
		query = `UPDATE print_history SET printed_pages = ?, status = ?, completed_at = ?`
		args = []interface{}{printedPages, status, time.Now(), taskID}
	}

	query += ` WHERE task_id = ?`

	_, err := m.db.Exec(query, args...)
	return err
}

// UpdatePrintJobWithPDF 更新打印任务并存储PDF路径
func (m *MySQLDatabase) UpdatePrintJobWithPDF(taskID int, printedPages int, status string, pdfPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	query := `UPDATE print_history SET printed_pages = ?, status = ?, pdf_path = ?`
	args := []interface{}{printedPages, status, pdfPath, taskID}

	if status == "completed" {
		query = `UPDATE print_history 
		         SET printed_pages = ?, status = ?, completed_at = ?, pdf_path = ?`
		args = []interface{}{printedPages, status, time.Now(), pdfPath, taskID}
	}

	query += ` WHERE task_id = ?`

	_, err := m.db.Exec(query, args...)
	return err
}

// GetRecentPrintHistory 获取最近N个打印任务的历史
// 当limit=-1时获取10个最近任务
func (m *MySQLDatabase) GetRecentPrintHistory(userID string, limit int) ([]map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = MaxRecentTasks // 默认最近10个任务
	}

	var rows *sql.Rows
	var err error

	if userID == "" {
		rows, err = m.db.Query(`
			SELECT task_id, filename, pages, printed_pages, status, created_at, user_id, priority, pdf_path
			FROM print_history 
			ORDER BY created_at DESC 
			LIMIT ?
		`, limit)
	} else {
		rows, err = m.db.Query(`
			SELECT task_id, filename, pages, printed_pages, status, created_at, user_id, priority, pdf_path
			FROM print_history 
			WHERE user_id = ?
			ORDER BY created_at DESC 
			LIMIT ?
		`, userID, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var taskID, pages, printedPages, priority int
		var filename, status, userID, pdfPath sql.NullString
		var createdAt time.Time

		err := rows.Scan(&taskID, &filename, &pages, &printedPages, &status, &createdAt, &userID, &priority, &pdfPath)
		if err != nil {
			return nil, err
		}

		record := map[string]interface{}{
			"task_id":       taskID,
			"filename":      filename.String,
			"pages":         pages,
			"printed_pages": printedPages,
			"status":        status.String,
			"created_at":    createdAt.Format(time.RFC3339),
			"user_id":       userID.String,
			"priority":      priority,
		}

		if pdfPath.Valid {
			record["pdf_path"] = pdfPath.String
		}

		history = append(history, record)
	}

	return history, nil
}

// CreateUser 创建用户
func (m *MySQLDatabase) CreateUser(username, password, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = m.db.Exec(`
		INSERT INTO users (username, password_hash, role, created_at)
		VALUES (?, ?, ?, ?)
	`, username, string(hash), role, time.Now())

	return err
}

// VerifyUser 验证用户密码
func (m *MySQLDatabase) VerifyUser(username, password string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var hash string
	err := m.db.QueryRow(`
		SELECT password_hash FROM users WHERE username = ? AND is_active = TRUE
	`, username).Scan(&hash)

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
func (m *MySQLDatabase) GetUserRole(username string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var role string
	err := m.db.QueryRow(`
		SELECT role FROM users WHERE username = ? AND is_active = TRUE
	`, username).Scan(&role)
	return role, err
}

// RecordAuditLog 记录审计日志
func (m *MySQLDatabase) RecordAuditLog(userID, action, details string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		INSERT INTO audit_log (user_id, action, details, timestamp)
		VALUES (?, ?, ?, ?)
	`, userID, action, details, time.Now())
	return err
}

// DeleteUser 删除用户（软删除）
func (m *MySQLDatabase) DeleteUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		UPDATE users SET is_active = FALSE WHERE username = ?
	`, username)
	return err
}

// ListUsers 列出所有活跃用户
func (m *MySQLDatabase) ListUsers() ([]map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT username, role, created_at FROM users 
		WHERE is_active = TRUE
		ORDER BY created_at DESC
	`)
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
func (m *MySQLDatabase) UserExists(username string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int
	err := m.db.QueryRow(`
		SELECT COUNT(*) FROM users WHERE username = ? AND is_active = TRUE
	`, username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RecordPrinterStatus 记录打印机状态历史
func (m *MySQLDatabase) RecordPrinterStatus(status, error string, temperature, paperPages, tonerPercent, queueSize, pageCount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		INSERT INTO printer_status_history
		(status, error, temperature, paper_pages, toner_percentage, queue_size, page_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, status, error, temperature, paperPages, tonerPercent, queueSize, pageCount)

	return err
}

// StorePDFInfo 存储PDF信息
func (m *MySQLDatabase) StorePDFInfo(taskID int, pdfFilename string, fileSize int64, fileHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		INSERT INTO pdf_storage (task_id, pdf_filename, pdf_size, file_hash, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, taskID, pdfFilename, fileSize, fileHash, time.Now())

	return err
}

// GetPDFInfo 获取PDF信息
func (m *MySQLDatabase) GetPDFInfo(taskID int) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 不能对 map 的索引表达式取地址，需用临时变量接收扫描结果
	var filename, fileHash string
	var fileSize, accessCount int64
	var createdAt time.Time

	err := m.db.QueryRow(`
		SELECT pdf_filename, pdf_size, file_hash, created_at, access_count
		FROM pdf_storage
		WHERE task_id = ?
	`, taskID).Scan(
		&filename,
		&fileSize,
		&fileHash,
		&createdAt,
		&accessCount,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"filename":     filename,
		"size":         fileSize,
		"hash":         fileHash,
		"created_at":   createdAt,
		"access_count": accessCount,
	}

	return result, err
}
