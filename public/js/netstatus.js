// network-monitor.js
class NetworkMonitor {
    constructor() {
      this.updateInterval = 5000; // 5秒刷新间隔
      this.timer = null;
      this.lastUpdateTime = 0;
  
      // 初始化监控
      this.init();
    }
  
    // 初始化方法
    init() {
      // 首次立即更新
      this.updateNetworkStatus();
      
      // 设置定时器
      this.timer = setInterval(() => {
        this.updateNetworkStatus();
      }, this.updateInterval);
  
      // 页面可见性处理
      document.addEventListener('visibilitychange', () => {
        if (document.hidden) {
          this.pauseMonitoring();
        } else {
          this.resumeMonitoring();
        }
      });
    }
  
    // 获取网络状态数据
    async fetchNetworkStatus() {
      try {
        const startTime = Date.now();
        const response = await authFetch('/netstatus');
  
        if (!response.ok) throw new Error('网络响应异常');
        const data = await response.json();
        return {
          ...data,
          latency: Date.now() - startTime // 计算实际延迟
        };
      } catch (error) {
        console.error('获取网络状态失败:', error);
        return null;
      }
    }
  
    // 更新界面显示
    updateStatusDisplay(data) {
      const statusElements = {
        latency: document.getElementById('latencyValue'),
        download: document.getElementById('downloadSpeed'),
        upload: document.getElementById('uploadSpeed'),
        status: document.getElementById('onlineStatus')
      };
  
      if (!data) {
        // 离线状态处理
        statusElements.status.classList.remove('online');
        statusElements.latency.textContent = '-- ms';
        statusElements.download.textContent = '-- Mbps';
        statusElements.upload.textContent = '-- Mbps';
        return;
      }
  
      // 更新数据
      statusElements.latency.textContent = `${Math.min(data.latency, 9999)} ms`;
      statusElements.download.textContent = `${data.downspeed}`;
      statusElements.upload.textContent = `${data.upspeed}`;
      statusElements.status.classList.toggle('online', data.netstatus);
    }
  
    // 执行状态更新
    async updateNetworkStatus() {
      const currentTime = Date.now();
      if (currentTime - this.lastUpdateTime < 1000) return; // 防抖处理
      
      this.lastUpdateTime = currentTime;
      const statusData = await this.fetchNetworkStatus();
      this.updateStatusDisplay(statusData);
    }
  
    // 暂停监控
    pauseMonitoring() {
      clearInterval(this.timer);
      this.timer = null;
    }
  
    // 恢复监控
    resumeMonitoring() {
      if (!this.timer) {
        this.timer = setInterval(() => {
          this.updateNetworkStatus();
        }, this.updateInterval);
      }
    }
  }
  
  // 页面加载后启动监控
  document.addEventListener('DOMContentLoaded', () => {
    // 检查用户授权
    if (!localStorage.getItem('authToken')) {
      window.location.href = '/login';
      return;
    }
  
    // 初始化网络监控
    new NetworkMonitor();
  
    // 开发环境模拟数据（正式环境移除）
    // if (process.env.NODE_ENV === 'development') {
    //   window.mockNetworkData = {
    //     downloadSpeed: 96.4,
    //     uploadSpeed: 32.7,
    //     isOnline: true
    //   };
    // }
  });