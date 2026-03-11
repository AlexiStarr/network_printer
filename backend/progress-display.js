/**
 * progress-display.js
 * 实时打印进度显示模块
 * 用于 printer_control.html 的WebSocket进度更新
 */

class PrintProgressTracker {
  constructor() {
    this.ws = null;
    this.activeJobs = new Map();
    this.completedJobs = [];
    this.failedJobs = [];
    this.maxRecentJobs = 10;
  }

  /**
   * 连接到WebSocket服务器
   */
  connect(serverUrl) {
    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(serverUrl);
        
        this.ws.onopen = () => {
          console.log('[Progress] WebSocket connected');
          resolve();
        };

        this.ws.onmessage = (event) => {
          this.handleMessage(JSON.parse(event.data));
        };

        this.ws.onerror = (error) => {
          console.error('[Progress] WebSocket error:', error);
          reject(error);
        };

        this.ws.onclose = () => {
          console.log('[Progress] WebSocket disconnected');
        };
      } catch (error) {
        reject(error);
      }
    });
  }

  /**
   * 处理来自服务器的消息
   */
  handleMessage(message) {
    if (!message.type) return;

    console.log(`[Progress] Received: ${message.type}`, message);

    switch (message.type) {
      case 'progress':
        this.updateProgress(message.progress);
        break;
      case 'completed':
        this.markCompleted(message.progress);
        break;
      case 'error':
        this.markError(message.progress, message.error_code, message.message);
        break;
      case 'paused':
        this.markPaused(message.progress);
        break;
      case 'resumed':
        this.markResumed(message.progress);
        break;
      case 'cancelled':
        this.markCancelled(message.progress);
        break;
      case 'submitted':
        this.addJob(message.progress);
        break;
      default:
        console.warn('[Progress] Unknown message type:', message.type);
    }
  }

  /**
   * 更新任务进度
   */
  updateProgress(progress) {
    const taskId = progress.task_id;
    this.activeJobs.set(taskId, progress);
    
    // 更新UI
    this.updateJobUI(taskId, progress);
  }

  /**
   * 标记任务完成
   */
  markCompleted(progress) {
    const taskId = progress.task_id;
    
    // 移出活跃任务
    this.activeJobs.delete(taskId);
    
    // 添加到已完成列表
    this.completedJobs.unshift(progress);
    if (this.completedJobs.length > this.maxRecentJobs) {
      this.completedJobs.pop();
    }
    
    // 显示完成通知
    this.showNotification('success', `打印任务 #${taskId} 已完成`);
    
    // 更新UI
    this.updateJobUI(taskId, progress, 'completed');
  }

  /**
   * 标记任务错误
   */
  markError(progress, errorCode, errorMsg) {
    const taskId = progress.task_id;
    
    // 移出活跃任务
    this.activeJobs.delete(taskId);
    
    // 添加到失败列表
    this.failedJobs.unshift({
      ...progress,
      error_code: errorCode,
      error_msg: errorMsg
    });
    if (this.failedJobs.length > this.maxRecentJobs) {
      this.failedJobs.pop();
    }
    
    // 显示错误通知
    this.showNotification('error', `任务 #${taskId} 错误: ${errorMsg}`);
    
    // 更新UI
    this.updateJobUI(taskId, progress, 'error');
  }

  /**
   * 标记任务暂停
   */
  markPaused(progress) {
    const taskId = progress.task_id;
    if (this.activeJobs.has(taskId)) {
      progress.status = 'paused';
      this.activeJobs.set(taskId, progress);
      this.updateJobUI(taskId, progress);
    }
  }

  /**
   * 标记任务恢复
   */
  markResumed(progress) {
    const taskId = progress.task_id;
    if (this.activeJobs.has(taskId)) {
      progress.status = 'printing';
      this.activeJobs.set(taskId, progress);
      this.updateJobUI(taskId, progress);
    }
  }

  /**
   * 标记任务取消
   */
  markCancelled(progress) {
    const taskId = progress.task_id;
    this.activeJobs.delete(taskId);
    
    this.showNotification('warning', `打印任务 #${taskId} 已取消`);
    this.updateJobUI(taskId, progress, 'cancelled');
  }

  /**
   * 添加新任务
   */
  addJob(progress) {
    const taskId = progress.task_id;
    this.activeJobs.set(taskId, progress);
    this.updateJobUI(taskId, progress);
    this.showNotification('info', `新任务 #${taskId}: ${progress.filename}`);
  }

  /**
   * 更新UI中的任务显示
   */
  updateJobUI(taskId, progress, status = null) {
    const elemId = `job-${taskId}`;
    let elem = document.getElementById(elemId);
    
    if (!elem) {
      elem = this.createJobElement(taskId);
      const container = document.getElementById('active-jobs-list');
      if (container) {
        container.insertBefore(elem, container.firstChild);
      }
    }

    // 更新任务信息
    const statusClass = status || progress.status;
    elem.className = `job-item status-${statusClass}`;
    
    const progressPercent = progress.progress_percent || 0;
    elem.innerHTML = `
      <div class="job-header">
        <span class="job-id">任务 #${taskId}</span>
        <span class="job-filename">${progress.filename}</span>
        <span class="job-status">${this.getStatusText(statusClass)}</span>
      </div>
      <div class="job-progress">
        <div class="progress-bar">
          <div class="progress-fill" style="width: ${progressPercent}%">
            <span class="progress-text">${progressPercent}%</span>
          </div>
        </div>
      </div>
      <div class="job-details">
        <span>打印进度: ${progress.printed_pages}/${progress.total_pages} 页</span>
        <span>温度: ${progress.temperature}℃</span>
        <span>纸张: ${progress.paper_remaining} 页</span>
        <span>碳粉: ${progress.toner_percent}%</span>
        <span>预计: ${progress.estimated_time_sec}秒</span>
      </div>
      <div class="job-actions">
        <button onclick="printerUI.pauseJob(${taskId})">暂停</button>
        <button onclick="printerUI.cancelJob(${taskId})">取消</button>
      </div>
    `;
  }

  /**
   * 创建任务元素
   */
  createJobElement(taskId) {
    const elem = document.createElement('div');
    elem.id = `job-${taskId}`;
    elem.className = 'job-item';
    return elem;
  }

  /**
   * 获取状态文本
   */
  getStatusText(status) {
    const statusMap = {
      'queued': '等待中',
      'printing': '打印中',
      'paused': '已暂停',
      'completed': '已完成',
      'error': '错误',
      'cancelled': '已取消'
    };
    return statusMap[status] || status;
  }

  /**
   * 显示通知
   */
  showNotification(type, message) {
    console.log(`[${type.toUpperCase()}] ${message}`);
    
    // 可以集成到页面UI
    const notifContainer = document.getElementById('notifications');
    if (notifContainer) {
      const notif = document.createElement('div');
      notif.className = `notification notif-${type}`;
      notif.textContent = message;
      notifContainer.appendChild(notif);
      
      // 3秒后移除
      setTimeout(() => notif.remove(), 3000);
    }
  }

  /**
   * 获取所有活跃任务
   */
  getActiveTasks() {
    return Array.from(this.activeJobs.values());
  }

  /**
   * 获取最近完成的任务
   */
  getCompletedTasks() {
    return this.completedJobs;
  }

  /**
   * 获取失败的任务
   */
  getFailedTasks() {
    return this.failedJobs;
  }

  /**
   * 断开连接
   */
  disconnect() {
    if (this.ws) {
      this.ws.close();
    }
  }
}

// ==================== CSS 样式 ====================

const PROGRESS_CSS = `
.job-item {
  background: var(--surface);
  border: 1px solid var(--border);
  padding: 16px;
  margin-bottom: 12px;
  border-radius: 4px;
  transition: all 0.3s;
}

.job-item.status-printing {
  border-left: 4px solid var(--blue);
}

.job-item.status-completed {
  border-left: 4px solid var(--green);
  opacity: 0.8;
}

.job-item.status-error {
  border-left: 4px solid var(--red);
}

.job-item.status-paused {
  border-left: 4px solid var(--accent);
  opacity: 0.6;
}

.job-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
  font-size: 13px;
  font-weight: 500;
}

.job-id {
  color: var(--accent);
  font-family: var(--mono);
}

.job-filename {
  color: var(--text);
  flex: 1;
  margin-left: 12px;
}

.job-status {
  color: var(--muted);
  font-family: var(--mono);
  font-size: 11px;
}

.job-progress {
  margin-bottom: 12px;
}

.progress-bar {
  width: 100%;
  height: 24px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 2px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--blue), var(--accent));
  display: flex;
  align-items: center;
  justify-content: center;
  transition: width 0.5s;
}

.progress-text {
  color: var(--text);
  font-size: 11px;
  font-weight: 600;
  text-shadow: 0 1px 2px rgba(0,0,0,0.5);
}

.job-details {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 12px;
  margin-bottom: 12px;
  font-size: 11px;
  color: var(--muted);
  font-family: var(--mono);
}

.job-actions {
  display: flex;
  gap: 8px;
}

.job-actions button {
  padding: 6px 12px;
  font-size: 11px;
  cursor: pointer;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text);
  border-radius: 2px;
  transition: all 0.2s;
}

.job-actions button:hover {
  border-color: var(--accent);
  color: var(--accent);
}

.notification {
  padding: 12px 16px;
  margin-bottom: 8px;
  border-radius: 4px;
  font-size: 12px;
  animation: slideIn 0.3s;
}

.notif-success {
  background: rgba(61, 220, 132, 0.1);
  color: var(--green);
  border-left: 3px solid var(--green);
}

.notif-error {
  background: rgba(255, 71, 87, 0.1);
  color: var(--red);
  border-left: 3px solid var(--red);
}

.notif-warning {
  background: rgba(232, 160, 32, 0.1);
  color: var(--accent);
  border-left: 3px solid var(--accent);
}

.notif-info {
  background: rgba(91, 141, 238, 0.1);
  color: var(--blue);
  border-left: 3px solid var(--blue);
}

@keyframes slideIn {
  from {
    transform: translateX(-100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}
`;

// ==================== 导出 ====================
if (typeof module !== 'undefined' && module.exports) {
  module.exports = PrintProgressTracker;
}
