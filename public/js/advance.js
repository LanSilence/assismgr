// 更新系统
async function updateSystem() {
    try {
        const response = await authFetch('/update', {
            method: 'POST',
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        alert('系统更新成功');
    } catch (error) {
        console.error('系统更新失败:', error);
        alert('系统更新失败，请稍后重试。');
    }
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