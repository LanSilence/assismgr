// 更新系统
// 全局状态变量
let upgradeStatus = {
  uploading: false,
  upgrading: false,
  progress: 0,
  status: ''
};

async function updateSystem() {
    const fileInput = document.getElementById('updateFile');
    const progressBar = document.getElementById('uploadProgress');
    const statusElement = document.getElementById('updateStatus');
    
    if (fileInput && fileInput.files.length > 0) {
        // 重置状态
        resetUpgradeStatus();
        upgradeStatus.uploading = true;
        updateStatusDisplay('文件上传中...', 0, 'info');

        const formData = new FormData();
        formData.append('updateFile', fileInput.files[0]);
        
        try {
            // 显示上传进度条
            progressBar.style.display = 'inline-block';
            progressBar.value = 0;
            
            // 上传文件
            await uploadFile(formData, (progress) => {
                progressBar.value = progress;
                updateStatusDisplay(`文件上传中`, progress, 'info');
            });
            
            // 上传完成
            updateStatusDisplay('文件上传完成，开始系统升级...', 100, 'success');
            await new Promise(resolve => setTimeout(resolve, 1000)); // 短暂延迟
            
            // 开始升级并监控进度
            upgradeStatus.uploading = false;
            upgradeStatus.upgrading = true;
            await pollUpgradeProgress();
            
            // 升级完成
            // updateStatusDisplay('系统升级完成！', 100, 'success');
            // alert('系统升级完成');
            
        } catch (error) {
            updateStatusDisplay(`系统升级失败: ${error.message}`, 0, 'error');
            alert(`系统升级失败: ${error.message}`);
        } finally {
            progressBar.style.display = 'none';
            resetUpgradeStatus();
        }
    } else {
        // URL方式升级
        const url = document.getElementById('updateUrl')?.value;
        if (!url) {
            updateStatusDisplay('请选择升级包或输入URL', 0, 'error');
            return;
        }
        
        try {
            // 重置状态
            resetUpgradeStatus();
            upgradeStatus.upgrading = true;
            updateStatusDisplay('开始下载远程升级包...', 0, 'info');
            
            // 显示进度条
            const progressBar = document.getElementById('uploadProgress');
            progressBar.style.display = 'inline-block';
            progressBar.value = 0;
            
            // 发送URL下载请求
            const response = await authFetch('/upload_update', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ url })
            });
            
            if (!response.ok) throw new Error('请求失败');
            
            // 开始监控升级进度
            await pollUpgradeProgress();
            // updateStatusDisplay('系统升级完成！', 100, 'success');
            // alert('系统升级完成');
        } catch (error) {
            updateStatusDisplay(`系统升级失败: ${error.message}`, 0, 'error');
            alert(`系统升级失败: ${error.message}`);
        } finally {
            document.getElementById('uploadProgress').style.display = 'none';
            resetUpgradeStatus();
        }
    }
}
let cancelController = null;

function getAuthToken() {
    // 从localStorage获取令牌
    return localStorage.getItem('authToken') || '';
    
    // 或者如果是Cookie方式：
    // return document.cookie.split('; ').find(row => row.startsWith('token='))?.split('=')[1] || '';
}
// 文件上传函数（封装XHR）
// 文件上传函数（封装XHR）
async function uploadFile(formData) {
    // 创建新的AbortController用于本次上传
    cancelController = new AbortController();
    
    return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        
        // 跟踪上传进度
        xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) {
                const percent = Math.round((e.loaded / e.total) * 100);
                updateStatusDisplay(`上传中`, percent, 'info');
            }
        };

        xhr.onload = () => {
            cancelController = null; // 清理
            if (xhr.status === 200) {
                try {
                    const resp = JSON.parse(xhr.responseText);
                    if (resp.status === "upload_complete") {
                        resolve();
                    } else {
                        reject(new Error("上传响应异常"));
                    }
                } catch (e) {
                    reject(new Error("解析响应失败"));
                }
            } else {
                reject(new Error(`上传失败: ${xhr.status}`));
            }
        };

        xhr.onerror = () => {
            cancelController = null; // 清理
            reject(new Error("网络错误"));
        };

        xhr.onabort = () => {
            cancelController = null; // 清理
            reject(new Error("上传已取消"));
        };

        // 绑定中止信号
        cancelController.signal.addEventListener('abort', () => {
            xhr.abort();
        });

        xhr.open("POST", "/upload_update", true);
        xhr.setRequestHeader("Authorization", getAuthToken());
        xhr.send(formData);
    });
}
// 轮询升级进度
async function pollUpgradeProgress() {
    const progressBar = document.getElementById('uploadProgress');
    progressBar.style.display = 'inline-block';
    progressBar.value = 0;
    
    let finished = false;
    let retryCount = 0;
    const maxRetries = 30; // 最大重试次数（约30秒超时）
    
    while (!finished && retryCount < maxRetries) {
        await new Promise(r => setTimeout(r, 1000));
        
        try {
            const resp = await authFetch('/upgrade_progress');
            if (!resp.ok) throw new Error('进度获取失败');
            
            const data = await resp.json();
            if (data.progress !== undefined) {
                progressBar.value = data.progress;
                updateStatusDisplay(data.message, data.progress, 'info');
            }
            
            if (data.status === 'done') {
                finished = true;
                progressBar.value = 100;
                updateStatusDisplay('系统升级完成！', 100, 'success');
            } else if (data.status === 'failed') {
                finished = true;
                throw new Error(data.message || '升级过程中出现错误');
            }
            
            retryCount = 0; // 重置重试计数器
        } catch (e) {
            retryCount++;
            if (retryCount >= maxRetries) {
                throw new Error('获取升级进度超时');
            }
        }
    }
    
    progressBar.style.display = 'none';
    if (!finished) {
        throw new Error('升级过程未正常完成');
    }
}

// 更新状态显示
function updateStatusDisplay(message, progress, type) {
    const progressBar = document.getElementById('uploadProgress');
    const cancelBtn = document.getElementById('cancelUpgradeBtn');
    
    // 控制进度条和取消按钮显示
    if (type === 'info' && (message.includes('上传') || message.includes('下载') || message.includes('安装'))) {
        progressBar.style.display = 'inline-block';
        cancelBtn.style.display = 'inline-block';
    } else {
        progressBar.style.display = 'none';
        cancelBtn.style.display = 'none';
    }
    
    progressBar.value = Number.isFinite(progress) ? progress : 0;
    const statusElement = document.getElementById('updateStatus');
    if (!statusElement) return;
    
    const progressText = Number.isFinite(progress) ? ` (${progress}%)` : '';
    statusElement.textContent = message + progressText;
    statusElement.style.display = 'block';
    statusElement.className = `status-${type}`;
    
    // 记录当前状态
    upgradeStatus.status = message;
}

// 取消升级
async function cancelUpgrade() {
    try {
        const response = await authFetch('/cancel_upgrade', {
            method: 'POST'
        });
        
        if (!response.ok) throw new Error('取消请求失败');
        
        const result = await response.json();
        if (result.status === 'cancelled') {
            updateStatusDisplay('升级已取消', 0, 'info');
            setTimeout(() => {
                document.getElementById('uploadProgress').style.display = 'none';
                document.getElementById('cancelUpgradeBtn').style.display = 'none';
            }, 2000);
         cancelController.abort();
        }
    } catch (error) {
        updateStatusDisplay(`取消失败: ${error.message}`, 0, 'error');
    }
}

// 重置升级状态
function resetUpgradeStatus() {
    upgradeStatus = {
        uploading: false,
        upgrading: false,
        progress: 0,
        status: ''
    };
}

// 重启系统
async function rebootSystem() {
    try {
        const response = await authFetch('/reboot', {
            method: 'POST',
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        alert('系统正在重启...');
    } catch (error) {
        console.error('系统重启失败:', error);
        alert('系统重启失败，请稍后重试。');
    }
}

// 恢复出厂设置
// 恢复出厂设置
async function resetSystem() {
    // 显示确认弹窗
    const confirmed = confirm("您确定要恢复出厂设置吗？此操作将删除所有配置并重置系统，且不可恢复！");
    if (!confirmed) {
        return; // 如果用户取消，则退出
    }

    try {
        const response = await authFetch('/reset', {
            method: 'POST',
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        alert('恢复出厂设置成功');
    } catch (error) {
        console.error('恢复出厂设置失败:', error);
        alert('恢复出厂设置失败，请稍后重试。');
    }
}
