import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000
})

export async function getServers() {
  const response = await api.get('/webdav/servers')
  return response.data
}

export async function addServer(server) {
  const response = await api.post('/webdav/servers', server)
  return response.data
}

export async function updateServer(name, server) {
  const response = await api.put(`/webdav/servers/${encodeURIComponent(name)}`, server)
  return response.data
}

export async function deleteServer(name) {
  const response = await api.delete(`/webdav/servers/${encodeURIComponent(name)}`)
  return response.data
}

export async function toggleServerEnabled(name, enabled) {
  const response = await api.put(`/webdav/servers/${encodeURIComponent(name)}/enabled`, { enabled })
  return response.data
}

export async function getServerStatus(name) {
  const response = await api.get(`/webdav/servers/${encodeURIComponent(name)}/status`)
  return response.data
}

export async function getPaths() {
  const response = await api.get('/paths')
  return response.data
}

export async function addPath(path) {
  const response = await api.post('/paths', path)
  return response.data
}

export async function removePath(path) {
  const response = await api.post('/paths/remove', { path })
  return response.data
}

export async function getStatus() {
  const response = await api.get('/status')
  return response.data
}

export async function startMonitor() {
  const response = await api.post('/monitor/start')
  return response.data
}

export async function stopMonitor() {
  const response = await api.post('/monitor/stop')
  return response.data
}

export async function getMonitorStats() {
  const response = await api.get('/monitor/stats')
  return response.data
}

export async function getLogs(count = 100) {
  const response = await api.get('/logs', { params: { count } })
  return response.data
}

export async function clearLogs() {
  const response = await api.delete('/logs')
  return response.data
}

export async function scanAndClean() {
  const response = await api.post('/clean')
  return response.data
}

export async function getCleanerConfig() {
  const response = await api.get('/config/cleaner')
  return response.data
}

export async function updateCleanerConfig(config) {
  const response = await api.put('/config/cleaner', config)
  return response.data
}

export default api
