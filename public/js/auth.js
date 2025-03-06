
// 封装 WebSocket 连接函数
function createAuthWebSocket(url) {
    const token = localStorage.getItem('authToken');
    const wsUrl = new URL(url);
    
    // 通过 URL 参数传递 Token（或使用子协议）
    wsUrl.searchParams.set('token', token);
    
    return new WebSocket(wsUrl.toString());
}


async function authFetch(url, options = {}) {
    // 自动添加 Authorization 头
    const headers = new Headers(options.headers || {});
    const token = localStorage.getItem('authToken');
    
    if (token) {
        headers.set('Authorization', `${token}`);
    }

    // 合并选项
    const mergedOptions = {
        ...options,
        headers
    };
    // 发起请求并处理 401 错误
    try {
        const response = await fetch(url, mergedOptions);
        if (response.status === 401) {
            handleUnauthorized();
            return Promise.reject('会话过期，请重新登录');
        }
        return response;
    } catch (error) {
        console.error('请求失败:', error);
        throw error;
    }
}

// 统一处理未授权
function handleUnauthorized() {
    localStorage.removeItem('authToken');
    alert('会话已过期，即将跳转登录页面');
    window.location.href = '/login';
}