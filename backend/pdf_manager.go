/**
 * pdf_manager.go
 * PDF 文件管理和存储系统
 * 支持最近10个任务的PDF存储和检索
 */

package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ==================== PDF 管理 ====================

// PDFManager PDF 文件管理器
type PDFManager struct {
	storageDir     string
	maxStoredFiles int
	maxStorageSize int64 // 最大存储容量（字节）
	mu             sync.RWMutex
	fileIndex      map[int]PDFFileInfo // taskID -> PDFFileInfo
}

// PDFFileInfo PDF 文件信息
type PDFFileInfo struct {
	TaskID       int
	Filename     string
	FilePath     string
	FileSize     int64
	FileHash     string
	CreatedAt    time.Time
	LastAccessed time.Time
	AccessCount  int
}

// NewPDFManager 创建新的 PDF 管理器
func NewPDFManager(storageDir string, maxFiles int, maxSizeMB int64) (*PDFManager, error) {
	// 创建存储目录
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("创建PDF存储目录失败: %w", err)
	}

	pm := &PDFManager{
		storageDir:     storageDir,
		maxStoredFiles: maxFiles,
		maxStorageSize: maxSizeMB * 1024 * 1024,
		fileIndex:      make(map[int]PDFFileInfo),
	}

	// 加载已存在的文件索引
	if err := pm.loadExistingFiles(); err != nil {
		return nil, fmt.Errorf("加载现有PDF文件失败: %w", err)
	}

	return pm, nil
}

// loadExistingFiles 加载已存在的PDF文件
func (pm *PDFManager) loadExistingFiles() error {
	files, err := ioutil.ReadDir(pm.storageDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".pdf" {
			// 尝试从文件名中提取 taskID
			// 格式: print_task_<taskID>_<timestamp>.pdf
			var taskID int
			if _, err := fmt.Sscanf(file.Name(), "print_task_%d_", &taskID); err == nil {
				filePath := filepath.Join(pm.storageDir, file.Name())
				hash, _ := calculateFileHash(filePath)

				pm.fileIndex[taskID] = PDFFileInfo{
					TaskID:       taskID,
					Filename:     file.Name(),
					FilePath:     filePath,
					FileSize:     file.Size(),
					FileHash:     hash,
					CreatedAt:    file.ModTime(),
					LastAccessed: file.ModTime(),
					AccessCount:  0,
				}
			}
		}
	}

	return nil
}

// StorePDF 存储新的 PDF 文件
func (pm *PDFManager) StorePDF(taskID int, pdfData []byte) (PDFFileInfo, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 检查是否已存在
	if _, ok := pm.fileIndex[taskID]; ok {
		return PDFFileInfo{}, fmt.Errorf("任务 %d 的 PDF 已存在", taskID)
	}

	// 检查文件数量是否超过限制
	if len(pm.fileIndex) >= pm.maxStoredFiles {
		// 删除最旧的文件
		pm.removeOldestFile()
	}

	// 检查存储空间
	totalSize := pm.calculateTotalSize()
	if totalSize+int64(len(pdfData)) > pm.maxStorageSize {
		return PDFFileInfo{}, fmt.Errorf("存储空间不足: 需要 %d 字节, 可用 %d 字节",
			len(pdfData), pm.maxStorageSize-totalSize)
	}

	// 生成文件名
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("print_task_%d_%d.pdf", taskID, timestamp)
	filePath := filepath.Join(pm.storageDir, filename)

	// 写入文件
	if err := ioutil.WriteFile(filePath, pdfData, 0644); err != nil {
		return PDFFileInfo{}, fmt.Errorf("写入PDF文件失败: %w", err)
	}

	// 计算哈希值
	hash := fmt.Sprintf("%x", md5.Sum(pdfData))

	// 记录文件信息
	now := time.Now()
	fileInfo := PDFFileInfo{
		TaskID:       taskID,
		Filename:     filename,
		FilePath:     filePath,
		FileSize:     int64(len(pdfData)),
		FileHash:     hash,
		CreatedAt:    now,
		LastAccessed: now,
		AccessCount:  0,
	}

	pm.fileIndex[taskID] = fileInfo

	return fileInfo, nil
}

// RetrievePDF 获取PDF文件数据
func (pm *PDFManager) RetrievePDF(taskID int) ([]byte, error) {
	pm.mu.Lock()

	fileInfo, ok := pm.fileIndex[taskID]
	if !ok {
		pm.mu.Unlock()
		return nil, fmt.Errorf("任务 %d 的 PDF 不存在", taskID)
	}

	// 更新访问信息
	fileInfo.AccessCount++
	fileInfo.LastAccessed = time.Now()
	pm.fileIndex[taskID] = fileInfo

	pm.mu.Unlock()

	// 读取文件
	data, err := ioutil.ReadFile(fileInfo.FilePath)
	if err != nil {
		return nil, fmt.Errorf("读取PDF文件失败: %w", err)
	}

	return data, nil
}

// DeletePDF 删除指定任务的PDF
func (pm *PDFManager) DeletePDF(taskID int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	fileInfo, ok := pm.fileIndex[taskID]
	if !ok {
		return fmt.Errorf("任务 %d 的 PDF 不存在", taskID)
	}

	// 删除文件
	if err := os.Remove(fileInfo.FilePath); err != nil {
		return fmt.Errorf("删除PDF文件失败: %w", err)
	}

	// 移除索引
	delete(pm.fileIndex, taskID)

	return nil
}

// GetRecentPDFs 获取最近的N个PDF文件信息
func (pm *PDFManager) GetRecentPDFs(count int) []PDFFileInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if count <= 0 {
		count = 10 // 默认获取最近10个
	}

	// 将文件信息转换为切片
	var fileInfos []PDFFileInfo
	for _, info := range pm.fileIndex {
		fileInfos = append(fileInfos, info)
	}

	// 按创建时间排序（最新在前）
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].CreatedAt.After(fileInfos[j].CreatedAt)
	})

	// 限制返回数量
	if len(fileInfos) > count {
		fileInfos = fileInfos[:count]
	}

	return fileInfos
}

// GetPDFInfo 获取PDF文件信息
func (pm *PDFManager) GetPDFInfo(taskID int) (*PDFFileInfo, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	fileInfo, ok := pm.fileIndex[taskID]
	if !ok {
		return nil, fmt.Errorf("任务 %d 的 PDF 不存在", taskID)
	}

	return &fileInfo, nil
}

// GetStorageStats 获取存储统计信息
func (pm *PDFManager) GetStorageStats() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	totalSize := pm.calculateTotalSize()
	usagePercent := 0
	if pm.maxStorageSize > 0 {
		usagePercent = int((totalSize * 100) / pm.maxStorageSize)
	}

	return map[string]interface{}{
		"total_files":   len(pm.fileIndex),
		"max_files":     pm.maxStoredFiles,
		"total_size":    totalSize,
		"max_size":      pm.maxStorageSize,
		"usage_percent": usagePercent,
		"available":     pm.maxStorageSize - totalSize,
	}
}

// removeOldestFile 删除最旧的文件
func (pm *PDFManager) removeOldestFile() {
	var oldestTaskID int
	var oldestTime time.Time

	for taskID, info := range pm.fileIndex {
		if oldestTime.IsZero() || info.CreatedAt.Before(oldestTime) {
			oldestTaskID = taskID
			oldestTime = info.CreatedAt
		}
	}

	if !oldestTime.IsZero() {
		if info, ok := pm.fileIndex[oldestTaskID]; ok {
			os.Remove(info.FilePath)
			delete(pm.fileIndex, oldestTaskID)
		}
	}
}

// calculateTotalSize 计算所有PDF文件的总大小
func (pm *PDFManager) calculateTotalSize() int64 {
	var total int64
	for _, info := range pm.fileIndex {
		total += info.FileSize
	}
	return total
}

// ==================== 辅助函数 ====================

// calculateFileHash 计算文件的MD5哈希
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// ExportPDFList 导出PDF列表（用于API返回）
func (pm *PDFManager) ExportPDFList() []map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []map[string]interface{}

	for _, info := range pm.fileIndex {
		result = append(result, map[string]interface{}{
			"task_id":       info.TaskID,
			"filename":      info.Filename,
			"file_size":     info.FileSize,
			"file_hash":     info.FileHash,
			"created_at":    info.CreatedAt.Format(time.RFC3339),
			"last_accessed": info.LastAccessed.Format(time.RFC3339),
			"access_count":  info.AccessCount,
		})
	}

	return result
}

// ==================== 清理和维护 ====================

// CleanupOldFiles 清理超过保留期的文件
func (pm *PDFManager) CleanupOldFiles(retentionDays int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	deletedCount := 0

	for taskID, info := range pm.fileIndex {
		if info.CreatedAt.Before(cutoffTime) {
			if err := os.Remove(info.FilePath); err != nil {
				// 记录错误但继续清理
				fmt.Printf("删除过期PDF失败: %v\n", err)
				continue
			}
			delete(pm.fileIndex, taskID)
			deletedCount++
		}
	}

	fmt.Printf("清理完成：删除了 %d 个过期PDF文件\n", deletedCount)
	return nil
}

// OptimizeStorage 优化存储空间
func (pm *PDFManager) OptimizeStorage() {
	pm.mu.Lock()

	// 按访问时间排序
	var fileInfos []PDFFileInfo
	for _, info := range pm.fileIndex {
		fileInfos = append(fileInfos, info)
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].AccessCount > fileInfos[j].AccessCount
	})

	// 删除访问量最少的文件直到满足存储要求
	for _, info := range fileInfos {
		if pm.calculateTotalSize() <= pm.maxStorageSize {
			break
		}

		os.Remove(info.FilePath)
		delete(pm.fileIndex, info.TaskID)
	}

	pm.mu.Unlock()
}
