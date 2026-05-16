<template>
  <div class="card">
    <div class="card-header">
      <span class="header-icon">📁</span>
      <h2>监控路径管理</h2>
    </div>
    <div class="card-body">
      <div class="section">
        <div class="section-header">
          <span class="section-title">已添加的监控路径</span>
          <button class="btn btn-primary btn-sm" @click="showAddPathModal = true">
            ➕ 添加路径
          </button>
        </div>

        <div v-if="paths.length === 0" class="empty-state">
          暂无监控路径，请点击上方按钮添加
        </div>

        <div v-else class="path-list">
          <div 
            v-for="(path, index) in paths" 
            :key="path.path" 
            class="path-item"
          >
            <div class="path-index">{{ index + 1 }}</div>
            <div class="path-info">
              <div class="path-path">{{ path.path }}</div>
              <div class="path-mode">扫描模式: {{ path.scan_mode === 'full' ? '全量扫描' : '增量扫描' }}</div>
            </div>
            <div class="path-actions">
              <button class="btn btn-danger btn-sm" @click="confirmRemove(path.path)">删除</button>
            </div>
          </div>
        </div>
      </div>

      <div class="settings-section">
        <div class="section-header">
          <span class="section-title">监控设置</span>
        </div>
        <div class="settings-grid">
          <div class="setting-item">
            <label>最小视频大小 (MB)</label>
            <input 
              v-model.number="settings.minVideoSize" 
              type="number" 
              class="form-control"
              min="1"
            >
          </div>
          <div class="setting-item">
            <label>监控间隔 (秒)</label>
            <input 
              v-model.number="settings.monitorInterval" 
              type="number" 
              class="form-control"
              min="10"
            >
          </div>
        </div>
        <button class="btn btn-primary" @click="saveSettings">
          💾 保存设置
        </button>
      </div>
    </div>

    <!-- 添加路径模态框 -->
    <div v-if="showAddPathModal" class="modal-overlay" @click.self="closeAddPathModal">
      <div class="modal-content">
        <div class="modal-header">
          <h3>添加监控路径</h3>
          <button class="modal-close" @click="closeAddPathModal">✕</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>云盘路径</label>
            <input 
              v-model="newPath.path" 
              type="text" 
              class="form-control" 
              placeholder="如: /115网盘/电影"
            >
          </div>
          <div class="form-group">
            <label>扫描模式</label>
            <select v-model="newPath.scan_mode" class="form-control">
              <option value="full">全量扫描</option>
              <option value="incremental">增量扫描</option>
            </select>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeAddPathModal">取消</button>
          <button 
            class="btn btn-primary" 
            @click="addPathData"
            :disabled="!newPath.path.trim()"
          >
            添加路径
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getPaths, addPath, removePath, getCleanerConfig, updateCleanerConfig } from '../utils/api'

const emit = defineEmits(['refresh'])

const paths = ref([])
const showAddPathModal = ref(false)
const newPath = ref({
  path: '',
  scan_mode: 'incremental',
  enabled: true
})
const settings = ref({
  minVideoSize: 10,
  monitorInterval: 30
})

const loadPaths = async () => {
  try {
    const data = await getPaths()
    paths.value = data.watch_paths || []
  } catch (error) {
    console.error('加载路径失败:', error)
  }
}

const loadSettings = async () => {
  try {
    const data = await getCleanerConfig()
    settings.value.minVideoSize = data.min_video_size_mb || 10
  } catch (error) {
    console.error('加载设置失败:', error)
  }
}

const closeAddPathModal = () => {
  showAddPathModal.value = false
  newPath.value = {
    path: '',
    scan_mode: 'incremental',
    enabled: true
  }
}

const addPathData = async () => {
  try {
    await addPath({
      path: newPath.value.path.trim(),
      scan_mode: newPath.value.scan_mode,
      enabled: true
    })
    closeAddPathModal()
    loadPaths()
    emit('refresh')
  } catch (error) {
    alert('添加路径失败: ' + error.message)
  }
}

const confirmRemove = (path) => {
  if (confirm(`确定要删除监控路径 "${path}" 吗？`)) {
    removePathData(path)
  }
}

const removePathData = async (path) => {
  try {
    await removePath(path)
    loadPaths()
    emit('refresh')
  } catch (error) {
    alert('删除路径失败: ' + error.message)
  }
}

const saveSettings = async () => {
  try {
    await updateCleanerConfig({
      min_video_size_mb: settings.value.minVideoSize
    })
    alert('设置保存成功')
    emit('refresh')
  } catch (error) {
    alert('保存设置失败: ' + error.message)
  }
}

onMounted(() => {
  loadPaths()
  loadSettings()
})
</script>

<style scoped>
.section {
  margin-bottom: 1.5rem;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.section-title {
  font-weight: 600;
  color: #2c3e50;
}

.path-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.path-item {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem;
  background-color: #f8f9fa;
  border-radius: 8px;
}

.path-index {
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #28a745;
  color: white;
  border-radius: 50%;
  font-size: 0.875rem;
  font-weight: 600;
}

.path-info {
  flex: 1;
}

.path-path {
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 0.25rem;
}

.path-mode {
  font-size: 0.875rem;
  color: #6c757d;
}

.path-actions {
  display: flex;
  gap: 0.5rem;
}

.settings-section {
  padding-top: 1rem;
  border-top: 1px solid #eee;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
  margin-bottom: 1rem;
}

.setting-item {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.setting-item label {
  font-weight: 500;
  color: #495057;
}

/* 模态框样式 */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: white;
  border-radius: 12px;
  width: 90%;
  max-width: 400px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid #eee;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.125rem;
}

.modal-close {
  background: none;
  border: none;
  font-size: 1.25rem;
  cursor: pointer;
  color: #6c757d;
  padding: 0.25rem;
}

.modal-body {
  padding: 1.25rem;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.25rem;
  border-top: 1px solid #eee;
}

.header-icon {
  font-size: 1.25rem;
}
</style>
