<template>
  <div class="card">
    <div class="card-header">
      <span class="header-icon">🌐</span>
      <h2>WebDAV 服务器配置</h2>
    </div>
    <div class="card-body">
      <div class="section">
        <div class="section-header">
          <span class="section-title">已配置的服务器</span>
          <button class="btn btn-primary btn-sm" @click="showAddModal = true">
            ➕ 添加服务器
          </button>
        </div>
        
        <div v-if="servers.length === 0" class="empty-state">
          暂无 WebDAV 服务器，请点击上方按钮添加
        </div>
        
        <div v-else class="server-list">
          <div 
            v-for="(server, index) in servers" 
            :key="server.name" 
            class="server-item"
            :class="{ disabled: !server.enabled }"
          >
            <div class="checkbox">
              <input 
                type="checkbox" 
                :checked="server.enabled" 
                @change="toggleEnabled(server.name, !server.enabled)"
              >
            </div>
            <div class="server-index">{{ index + 1 }}</div>
            <div class="server-info">
              <div class="server-name">
                {{ server.name }}
                <span :class="['badge', getStatusClass(server)]">{{ getStatusText(server) }}</span>
              </div>
              <div class="server-url">{{ server.server }}</div>
            </div>
            <div class="server-actions">
              <button class="btn btn-primary btn-sm" @click="editServer(server)">编辑</button>
              <button class="btn btn-danger btn-sm" @click="confirmDelete(server.name)">删除</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 添加/编辑服务器模态框 -->
    <div v-if="showAddModal" class="modal-overlay" @click.self="closeModal">
      <div class="modal-content">
        <div class="modal-header">
          <h3>{{ editingServer ? '编辑服务器' : '添加服务器' }}</h3>
          <button class="modal-close" @click="closeModal">✕</button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>服务器名称</label>
            <input 
              v-model="formData.name" 
              type="text" 
              class="form-control" 
              placeholder="请输入服务器名称"
              :disabled="!!editingServer"
            >
          </div>
          <div class="form-group">
            <label>服务器地址</label>
            <input 
              v-model="formData.server" 
              type="text" 
              class="form-control" 
              placeholder="如: http://localhost:19798/webdav"
            >
          </div>
          <div class="form-group">
            <label>用户名</label>
            <input 
              v-model="formData.username" 
              type="text" 
              class="form-control" 
              placeholder="请输入用户名"
            >
          </div>
          <div class="form-group">
            <label>密码</label>
            <input 
              v-model="formData.password" 
              type="password" 
              class="form-control" 
              placeholder="请输入密码"
            >
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeModal">取消</button>
          <button 
            class="btn btn-primary" 
            @click="editingServer ? updateServerData() : addServerData()"
            :disabled="!isFormValid"
          >
            {{ editingServer ? '保存修改' : '添加服务器' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getServers, addServer, updateServer, deleteServer, toggleServerEnabled, getServerStatus } from '../utils/api'

const emit = defineEmits(['refresh'])

const servers = ref([])
const showAddModal = ref(false)
const editingServer = ref(null)
const formData = ref({
  name: '',
  server: '',
  username: '',
  password: ''
})

const isFormValid = computed(() => {
  return formData.value.name.trim() && formData.value.server.trim()
})

const loadServers = async () => {
  try {
    const data = await getServers()
    servers.value = data.servers || []
  } catch (error) {
    console.error('加载服务器失败:', error)
  }
}

const addServerData = async () => {
  try {
    await addServer({
      name: formData.value.name.trim(),
      server: formData.value.server.trim(),
      username: formData.value.username.trim(),
      password: formData.value.password
    })
    closeModal()
    loadServers()
    emit('refresh')
  } catch (error) {
    alert('添加服务器失败: ' + error.message)
  }
}

const editServer = (server) => {
  editingServer.value = server
  formData.value = {
    name: server.name,
    server: server.server,
    username: server.username,
    password: server.password
  }
  showAddModal.value = true
}

const updateServerData = async () => {
  try {
    await updateServer(editingServer.value.name, {
      server: formData.value.server.trim(),
      username: formData.value.username.trim(),
      password: formData.value.password
    })
    closeModal()
    loadServers()
    emit('refresh')
  } catch (error) {
    alert('更新服务器失败: ' + error.message)
  }
}

const confirmDelete = (name) => {
  if (confirm(`确定要删除服务器 "${name}" 吗？`)) {
    deleteServerData(name)
  }
}

const deleteServerData = async (name) => {
  try {
    await deleteServer(name)
    loadServers()
    emit('refresh')
  } catch (error) {
    alert('删除服务器失败: ' + error.message)
  }
}

const toggleEnabled = async (name, enabled) => {
  try {
    await toggleServerEnabled(name, enabled)
    loadServers()
    emit('refresh')
  } catch (error) {
    alert('切换服务器状态失败: ' + error.message)
  }
}

const getStatusClass = (server) => {
  return server.enabled ? 'badge-success' : 'badge-warning'
}

const getStatusText = (server) => {
  return server.enabled ? '✅ 已启用' : '⏸️ 已禁用'
}

const closeModal = () => {
  showAddModal.value = false
  editingServer.value = null
  formData.value = {
    name: '',
    server: '',
    username: '',
    password: ''
  }
}

onMounted(() => {
  loadServers()
})
</script>

<style scoped>
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

.server-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.server-item {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem;
  background-color: #f8f9fa;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.server-item:hover {
  background-color: #e9ecef;
}

.server-item.disabled {
  opacity: 0.6;
}

.checkbox input {
  width: 1.25rem;
  height: 1.25rem;
  cursor: pointer;
}

.server-index {
  width: 2rem;
  height: 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #667eea;
  color: white;
  border-radius: 50%;
  font-size: 0.875rem;
  font-weight: 600;
}

.server-info {
  flex: 1;
}

.server-name {
  font-weight: 600;
  color: #2c3e50;
  margin-bottom: 0.25rem;
}

.server-url {
  font-size: 0.875rem;
  color: #6c757d;
}

.server-actions {
  display: flex;
  gap: 0.5rem;
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
  max-width: 480px;
  max-height: 90vh;
  overflow: hidden;
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
