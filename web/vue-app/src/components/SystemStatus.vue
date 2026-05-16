<template>
  <div class="card">
    <div class="card-header">
      <span class="header-icon">📊</span>
      <h2>系统状态</h2>
    </div>
    <div class="card-body">
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-icon">🗑️</div>
          <div class="stat-info">
            <div class="stat-value">{{ statusData.cleanedCount || '-' }}</div>
            <div class="stat-label">累计清理</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon">📁</div>
          <div class="stat-info">
            <div class="stat-value">{{ statusData.pathCount || '-' }}</div>
            <div class="stat-label">监控路径数</div>
          </div>
        </div>
        <div class="stat-card">
          <div class="stat-icon">🔄</div>
          <div class="stat-info">
            <div :class="['stat-value', statusData.monitorStatus === 'running' ? 'status-running' : 'status-stopped']">
              {{ statusData.monitorStatus === 'running' ? '🟢 运行中' : '⚪ 已停止' }}
            </div>
            <div class="stat-label">监控状态</div>
          </div>
        </div>
      </div>

      <div class="action-buttons">
        <button class="btn btn-secondary" @click="handleScan">
          🔍 扫描并清理
        </button>
        <button 
          class="btn" 
          :class="statusData.monitorStatus === 'running' ? 'btn-danger' : 'btn-success'"
          @click="toggleMonitor"
        >
          {{ statusData.monitorStatus === 'running' ? '⏹️ 停止监控' : '▶️ 启动监控' }}
        </button>
        <button class="btn btn-primary" @click="refreshStatus">
          🔄 刷新状态
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getStatus, startMonitor, stopMonitor, scanAndClean } from '../utils/api'

const emit = defineEmits(['refresh'])

const statusData = ref({
  cleanedCount: 0,
  pathCount: 0,
  monitorStatus: 'stopped'
})

const loadStatus = async () => {
  try {
    const data = await getStatus()
    statusData.value = {
      cleanedCount: data.cleaned_count || 0,
      pathCount: data.path_count || 0,
      monitorStatus: data.monitor_running ? 'running' : 'stopped'
    }
  } catch (error) {
    console.error('加载状态失败:', error)
  }
}

const refreshStatus = () => {
  loadStatus()
  emit('refresh')
}

const toggleMonitor = async () => {
  try {
    if (statusData.value.monitorStatus === 'running') {
      await stopMonitor()
      statusData.value.monitorStatus = 'stopped'
    } else {
      await startMonitor()
      statusData.value.monitorStatus = 'running'
    }
    emit('refresh')
  } catch (error) {
    alert('操作失败: ' + error.message)
  }
}

const handleScan = async () => {
  try {
    await scanAndClean()
    emit('refresh')
  } catch (error) {
    alert('扫描失败: ' + error.message)
  }
}

onMounted(() => {
  loadStatus()
})
</script>

<style scoped>
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  background-color: #f8f9fa;
  border-radius: 8px;
}

.stat-icon {
  font-size: 2rem;
}

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 1.25rem;
  font-weight: 700;
  color: #2c3e50;
}

.stat-value.status-running {
  color: #28a745;
}

.stat-value.status-stopped {
  color: #6c757d;
}

.stat-label {
  font-size: 0.875rem;
  color: #6c757d;
}

.action-buttons {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.header-icon {
  font-size: 1.25rem;
}
</style>
