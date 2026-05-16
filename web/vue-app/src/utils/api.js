// 原生JavaScript API调用工具
const baseURL = '/api';

async function request(url, options = {}) {
  const response = await fetch(`${baseURL}${url}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers
    },
    ...options
  });
  
  const text = await response.text();
  let data;
  try {
    data = text ? JSON.parse(text) : {};
  } catch {
    data = {};
  }
  
  if (!response.ok) {
    throw new Error(data.error || '请求失败');
  }
  
  return data;
}

export async function getServers() {
  return await request('/webdav/servers');
}

export async function addServer(server) {
  return await request('/webdav/servers', {
    method: 'POST',
    body: JSON.stringify(server)
  });
}

export async function updateServer(name, server) {
  return await request(`/webdav/servers/${encodeURIComponent(name)}`, {
    method: 'PUT',
    body: JSON.stringify(server)
  });
}

export async function deleteServer(name) {
  return await request(`/webdav/servers/${encodeURIComponent(name)}`, {
    method: 'DELETE'
  });
}

export async function toggleServerEnabled(name, enabled) {
  return await request(`/webdav/servers/${encodeURIComponent(name)}/enabled`, {
    method: 'PUT',
    body: JSON.stringify({ enabled })
  });
}

export async function getServerStatus(name) {
  return await request(`/webdav/servers/${encodeURIComponent(name)}/status`);
}

export async function getPaths() {
  return await request('/paths');
}

export async function addPath(path) {
  return await request('/paths', {
    method: 'POST',
    body: JSON.stringify(path)
  });
}

export async function removePath(path) {
  return await request('/paths/remove', {
    method: 'POST',
    body: JSON.stringify({ path })
  });
}

export async function getStatus() {
  return await request('/status');
}

export async function startMonitor() {
  return await request('/monitor/start', { method: 'POST' });
}

export async function stopMonitor() {
  return await request('/monitor/stop', { method: 'POST' });
}

export async function getMonitorStats() {
  return await request('/monitor/stats');
}

export async function getLogs(count = 100) {
  return await request(`/logs?count=${count}`);
}

export async function clearLogs() {
  return await request('/logs', { method: 'DELETE' });
}

export async function scanAndClean() {
  return await request('/clean', { method: 'POST' });
}

export async function getCleanerConfig() {
  return await request('/config/cleaner');
}

export async function updateCleanerConfig(config) {
  return await request('/config/cleaner', {
    method: 'PUT',
    body: JSON.stringify(config)
  });
}

export default {
  getServers,
  addServer,
  updateServer,
  deleteServer,
  toggleServerEnabled,
  getServerStatus,
  getPaths,
  addPath,
  removePath,
  getStatus,
  startMonitor,
  stopMonitor,
  getMonitorStats,
  getLogs,
  clearLogs,
  scanAndClean,
  getCleanerConfig,
  updateCleanerConfig
};
