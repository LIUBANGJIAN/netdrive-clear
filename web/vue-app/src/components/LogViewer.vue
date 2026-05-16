<template>
  <div class="card">
    <div class="card-header">
      <span class="header-icon">📝</span>
      <h2>操作日志</h2>
      <div class="header-actions">
        <button class="btn btn-secondary btn-sm" @click="refreshLogs">🔄 刷新</button>
        <button class="btn btn-danger btn-sm" @click="confirmClear">🗑️ 清空日志</button>
      </div>
    </div>
    <div class="card-body">
      <div ref="logContainer" class="log-container">
        <div v-if="logs.length === 0" class="empty-state">
          暂无日志
        </div>
        <div 
          v-for="(log, index) in logs" 
          :key="index" 
          :class="['log-entry', log.level.toLowerCase()]"
        >
          <span class="log-time">{{ log.time }}</span>
          <span :class="['log-level', log.level.toLowerCase()]">[{{ log.level }}]</span>
          <span class="log-message">{{ log.message }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { getLogs, clearLogs } from '../utils/api'

const logs = ref([])
const logContainer = ref(null)
let refreshInterval = null

const loadLogs = async () => {
  try {
    const data = await getLogs(100)
    logs.value = data.logs || []
    nextTick(() => {
      if (logContainer.value) {
        logContainer.value.scrollTop = logContainer.value.scrollHeight
      }
    })
  } catch (error) {
    console.error('加载日志失败:', error)
  }
}

const refreshLogs = () => {
  loadLogs()
}

const confirmClear = () => {
  if (confirm('确定要清空所有日志吗？')) {
    clearLogsData()
  }
}

const clearLogsData = async () => {
  try {
    await clearLogs()
    logs.value = []
  } catch (error) {
    alert('清空日志失败: ' + error.message)
  }
}

onMounted(() => {
  loadLogs()
  refreshInterval = setInterval(loadLogs, 5000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
})
</script>

<style scoped>
.header-actions {
  margin-left: auto;
  display: flex;
  gap: 0.5rem;
}

.log-container {
  max-height: 300px;
  overflow-y: auto;
  background-color: #1a1a1a;
  border-radius: 8px;
  padding: 0.75rem;
}

.log-entry {
  display: flex;
  gap: 0.75rem;
  padding: 0.375rem 0;
  border-bottom: 1px solid #2a2a2a;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 0.875rem;
}

.log-entry:last-child {
  border-bottom: none;
}

.log-time {
  color: #888;
  flex-shrink: 0;
}

.log-level {
  font-weight: 600;
  flex-shrink: 0;
  min-width: 50px;
}

.log-level.info {
  color: #4dabf7;
}

.log-level.success {
  color: #51cf66;
}

.log-level.warn {
  color: #fcc419;
}

.log-level.error {
  color: #ff6b6b;
}

.log-message {
  color: #e0e0e0;
  flex: 1;
}

.empty-state {
  text-align: center;
  padding: 2rem;
  color: #666;
}

.header-icon {
  font-size: 1.25rem;
}
</style>
