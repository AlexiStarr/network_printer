/**
 * progress_tracker.go
 * 打印任务进度追踪系统
 * 支持实时进度同步、完成通知和队列管理
 */

package main

import (
	"fmt"
	"sync"
	"time"
)

// ==================== 任务进度定义 ====================

// PrintJobProgress 打印任务进度信息
type PrintJobProgress struct {
	TaskID          int        `json:"task_id"`
	FileName        string     `json:"filename"`
	TotalPages      int        `json:"total_pages"`
	PrintedPages    int        `json:"printed_pages"`
	ProgressPercent int        `json:"progress_percent"`
	Status          string     `json:"status"`             // "queued", "printing", "paused", "completed", "error", "cancelled"
	EstimatedTime   int        `json:"estimated_time_sec"` // 预计剩余时间（秒）
	StartTime       time.Time  `json:"start_time"`
	CompletedTime   *time.Time `json:"completed_time"`
	Temperature     int        `json:"temperature"`
	PaperRemaining  int        `json:"paper_remaining"`
	TonerPercent    int        `json:"toner_percent"`
}

// PrintJobNotification 打印任务通知
type PrintJobNotification struct {
	Type      string            `json:"type"` // "progress", "completed", "error", "cancelled"
	Timestamp time.Time         `json:"timestamp"`
	Progress  *PrintJobProgress `json:"progress,omitempty"`
	Message   string            `json:"message,omitempty"`
	ErrorCode string            `json:"error_code,omitempty"`
}

// ProgressTracker 进度追踪器
type ProgressTracker struct {
	jobs map[int]*PrintJobProgress
	mu   sync.RWMutex

	// 用于事件通知
	notifyChan chan *PrintJobNotification
	listeners  map[string]chan *PrintJobNotification
	listenerMu sync.RWMutex

	// 统计信息
	totalCompleted int
	totalFailed    int
	totalCancelled int
}

// NewProgressTracker 创建新的进度追踪器
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		jobs:       make(map[int]*PrintJobProgress),
		notifyChan: make(chan *PrintJobNotification, 100),
		listeners:  make(map[string]chan *PrintJobNotification),
	}
}

// RegisterListener 注册进度监听器
func (pt *ProgressTracker) RegisterListener(clientID string) chan *PrintJobNotification {
	pt.listenerMu.Lock()
	defer pt.listenerMu.Unlock()

	ch := make(chan *PrintJobNotification, 50)
	pt.listeners[clientID] = ch
	return ch
}

// UnregisterListener 注销监听器
func (pt *ProgressTracker) UnregisterListener(clientID string) {
	pt.listenerMu.Lock()
	defer pt.listenerMu.Unlock()

	if ch, ok := pt.listeners[clientID]; ok {
		close(ch)
		delete(pt.listeners, clientID)
	}
}

// UpdateProgress 更新任务进度
func (pt *ProgressTracker) UpdateProgress(taskID int, printedPages int, totalPages int,
	temperature, paperRemaining, tonerPercent int) error {
	pt.mu.Lock()

	job, ok := pt.jobs[taskID]
	if !ok {
		pt.mu.Unlock()
		return fmt.Errorf("任务不存在: %d", taskID)
	}

	// 更新进度信息
	job.PrintedPages = printedPages
	job.TotalPages = totalPages
	job.Temperature = temperature
	job.PaperRemaining = paperRemaining
	job.TonerPercent = tonerPercent

	// 计算进度百分比
	if totalPages > 0 {
		job.ProgressPercent = (printedPages * 100) / totalPages
	}

	// 计算预计剩余时间（假设平均打印速度为20页/分钟）
	remainingPages := totalPages - printedPages
	if remainingPages > 0 {
		// 预计时间（秒）= 剩余页数 / (20页/分钟 / 60秒)
		job.EstimatedTime = (remainingPages * 60) / 20
	} else {
		job.EstimatedTime = 0
	}

	// 检查是否完成
	if printedPages >= totalPages {
		job.Status = "completed"
		job.ProgressPercent = 100
		now := time.Now()
		job.CompletedTime = &now

		pt.mu.Unlock()

		// 发送完成通知
		pt.notifyListeners(&PrintJobNotification{
			Type:      "completed",
			Timestamp: now,
			Progress:  pt.GetProgress(taskID),
			Message:   fmt.Sprintf("打印任务 #%d 已完成", taskID),
		})

		pt.mu.Lock()
		pt.totalCompleted++
		pt.mu.Unlock()
	} else {
		pt.mu.Unlock()

		// 发送进度通知
		pt.notifyListeners(&PrintJobNotification{
			Type:      "progress",
			Timestamp: time.Now(),
			Progress:  pt.GetProgress(taskID),
			Message:   fmt.Sprintf("打印任务 #%d 进度: %d%%", taskID, job.ProgressPercent),
		})
	}

	return nil
}

// SubmitJob 提交新的打印任务
func (pt *ProgressTracker) SubmitJob(taskID int, filename string, pages int) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, ok := pt.jobs[taskID]; ok {
		return fmt.Errorf("任务已存在: %d", taskID)
	}

	now := time.Now()
	job := &PrintJobProgress{
		TaskID:          taskID,
		FileName:        filename,
		TotalPages:      pages,
		PrintedPages:    0,
		ProgressPercent: 0,
		Status:          "queued",
		EstimatedTime:   (pages * 60) / 20, // 预计完成时间
		StartTime:       now,
		Temperature:     25,
		PaperRemaining:  -1, // 未知
		TonerPercent:    -1, // 未知
	}

	pt.jobs[taskID] = job

	// 发送提交通知
	pt.notifyListeners(&PrintJobNotification{
		Type:      "submitted",
		Timestamp: now,
		Progress:  job,
		Message:   fmt.Sprintf("打印任务 #%d 已提交到队列", taskID),
	})

	return nil
}

// CancelJob 取消打印任务
func (pt *ProgressTracker) CancelJob(taskID int, reason string) error {
	pt.mu.Lock()

	job, ok := pt.jobs[taskID]
	if !ok {
		pt.mu.Unlock()
		return fmt.Errorf("任务不存在: %d", taskID)
	}

	job.Status = "cancelled"
	now := time.Now()
	job.CompletedTime = &now

	pt.totalCancelled++

	pt.mu.Unlock()

	// 发送取消通知
	pt.notifyListeners(&PrintJobNotification{
		Type:      "cancelled",
		Timestamp: now,
		Progress:  pt.GetProgress(taskID),
		Message:   fmt.Sprintf("打印任务 #%d 已取消: %s", taskID, reason),
	})

	return nil
}

// MarkJobError 标记任务发生错误
func (pt *ProgressTracker) MarkJobError(taskID int, errorCode string, errorMsg string) error {
	pt.mu.Lock()

	job, ok := pt.jobs[taskID]
	if !ok {
		pt.mu.Unlock()
		return fmt.Errorf("任务不存在: %d", taskID)
	}

	job.Status = "error"
	now := time.Now()
	job.CompletedTime = &now

	pt.totalFailed++

	pt.mu.Unlock()

	// 发送错误通知
	pt.notifyListeners(&PrintJobNotification{
		Type:      "error",
		Timestamp: now,
		Progress:  pt.GetProgress(taskID),
		Message:   errorMsg,
		ErrorCode: errorCode,
	})

	return nil
}

// PauseJob 暂停打印任务
func (pt *ProgressTracker) PauseJob(taskID int) error {
	pt.mu.Lock()

	job, ok := pt.jobs[taskID]
	if !ok {
		pt.mu.Unlock()
		return fmt.Errorf("任务不存在: %d", taskID)
	}

	if job.Status == "printing" {
		job.Status = "paused"
	}

	pt.mu.Unlock()

	// 发送暂停通知
	pt.notifyListeners(&PrintJobNotification{
		Type:      "paused",
		Timestamp: time.Now(),
		Progress:  pt.GetProgress(taskID),
		Message:   fmt.Sprintf("打印任务 #%d 已暂停", taskID),
	})

	return nil
}

// ResumeJob 恢复打印任务
func (pt *ProgressTracker) ResumeJob(taskID int) error {
	pt.mu.Lock()

	job, ok := pt.jobs[taskID]
	if !ok {
		pt.mu.Unlock()
		return fmt.Errorf("任务不存在: %d", taskID)
	}

	if job.Status == "paused" {
		job.Status = "printing"
	}

	pt.mu.Unlock()

	// 发送恢复通知
	pt.notifyListeners(&PrintJobNotification{
		Type:      "resumed",
		Timestamp: time.Now(),
		Progress:  pt.GetProgress(taskID),
		Message:   fmt.Sprintf("打印任务 #%d 已恢复", taskID),
	})

	return nil
}

// GetProgress 获取任务进度
func (pt *ProgressTracker) GetProgress(taskID int) *PrintJobProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if job, ok := pt.jobs[taskID]; ok {
		// 返回副本以避免竞态条件
		jobCopy := *job
		return &jobCopy
	}
	return nil
}

// GetAllProgress 获取所有任务的进度
func (pt *ProgressTracker) GetAllProgress() []PrintJobProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var result []PrintJobProgress
	for _, job := range pt.jobs {
		result = append(result, *job)
	}
	return result
}

// RemoveJob 从追踪器中移除已完成的任务
func (pt *ProgressTracker) RemoveJob(taskID int) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if _, ok := pt.jobs[taskID]; !ok {
		return fmt.Errorf("任务不存在: %d", taskID)
	}

	delete(pt.jobs, taskID)
	return nil
}

// GetStatistics 获取统计信息
func (pt *ProgressTracker) GetStatistics() map[string]interface{} {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return map[string]interface{}{
		"total_jobs":      len(pt.jobs),
		"total_completed": pt.totalCompleted,
		"total_failed":    pt.totalFailed,
		"total_cancelled": pt.totalCancelled,
	}
}

// notifyListeners 通知所有监听器
func (pt *ProgressTracker) notifyListeners(notification *PrintJobNotification) {
	pt.listenerMu.RLock()
	defer pt.listenerMu.RUnlock()

	for _, ch := range pt.listeners {
		select {
		case ch <- notification:
		default:
			// 通道满，跳过此消息
		}
	}
}

// ==================== 队列管理 ====================

// PrintQueue 打印队列管理器
type PrintQueue struct {
	queue []*PrintJobProgress
	mu    sync.RWMutex
}

// NewPrintQueue 创建新的打印队列
func NewPrintQueue() *PrintQueue {
	return &PrintQueue{
		queue: make([]*PrintJobProgress, 0),
	}
}

// Push 将任务加入到队列尾部
func (pq *PrintQueue) Push(job *PrintJobProgress) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.queue = append(pq.queue, job)
}

// Pop 从队列头部移除任务
func (pq *PrintQueue) Pop() *PrintJobProgress {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.queue) == 0 {
		return nil
	}

	job := pq.queue[0]
	pq.queue = pq.queue[1:]
	return job
}

// Peek 查看队列头部的任务但不移除
func (pq *PrintQueue) Peek() *PrintJobProgress {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if len(pq.queue) == 0 {
		return nil
	}
	return pq.queue[0]
}

// Remove 从队列中移除指定的任务
func (pq *PrintQueue) Remove(taskID int) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for i, job := range pq.queue {
		if job.TaskID == taskID {
			pq.queue = append(pq.queue[:i], pq.queue[i+1:]...)
			return true
		}
	}
	return false
}

// Size 获取队列大小
func (pq *PrintQueue) Size() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return len(pq.queue)
}

// GetAll 获取队列中所有的任务
func (pq *PrintQueue) GetAll() []*PrintJobProgress {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	result := make([]*PrintJobProgress, len(pq.queue))
	copy(result, pq.queue)
	return result
}

// Clear 清空队列
func (pq *PrintQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	pq.queue = make([]*PrintJobProgress, 0)
}
