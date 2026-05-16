<template>
  <div class="card">
    <div class="card-header">
      <span class="header-icon">⚙️</span>
      <h2>清理规则设置</h2>
    </div>
    <div class="card-body">
      <div class="form-group">
        <label>广告文件扩展名（逗号分隔）</label>
        <input 
          v-model="config.adExtensions" 
          type="text" 
          class="form-control"
          placeholder=".txt,.html,.url,.lnk"
        >
        <small class="form-hint">这些扩展名的文件将被视为广告文件删除</small>
      </div>

      <div class="form-group">
        <label>视频文件扩展名（逗号分隔）</label>
        <input 
          v-model="config.videoExtensions" 
          type="text" 
          class="form-control"
          placeholder=".mp4,.mkv,.avi,.mov"
        >
        <small class="form-hint">这些扩展名的文件将被视为视频文件进行大小检查</small>
      </div>

      <div class="form-group">
        <label>最小视频大小 (MB)</label>
        <input 
          v-model.number="config.minVideoSize" 
          type="number" 
          class="form-control"
          min="1"
        >
        <small class="form-hint">小于此大小的视频文件将被删除</small>
      </div>

      <button class="btn btn-primary" @click="saveConfig">
        💾 保存设置
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getCleanerConfig, updateCleanerConfig } from '../utils/api'

const emit = defineEmits(['refresh'])

const config = ref({
  adExtensions: '',
  videoExtensions: '',
  minVideoSize: 10
})

const loadConfig = async () => {
  try {
    const data = await getCleanerConfig()
    config.value = {
      adExtensions: (data.ad_extensions || []).join(','),
      videoExtensions: (data.video_extensions || []).join(','),
      minVideoSize: data.min_video_size_mb || 10
    }
  } catch (error) {
    console.error('加载配置失败:', error)
  }
}

const saveConfig = async () => {
  try {
    await updateCleanerConfig({
      ad_extensions: config.value.adExtensions.split(',').map(s => s.trim()).filter(Boolean),
      video_extensions: config.value.videoExtensions.split(',').map(s => s.trim()).filter(Boolean),
      min_video_size_mb: config.value.minVideoSize
    })
    alert('设置保存成功')
    emit('refresh')
  } catch (error) {
    alert('保存设置失败: ' + error.message)
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.form-group {
  margin-bottom: 1.25rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: #495057;
}

.form-hint {
  display: block;
  margin-top: 0.25rem;
  font-size: 0.8rem;
  color: #6c757d;
}

.header-icon {
  font-size: 1.25rem;
}
</style>
