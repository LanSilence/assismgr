package main

import (
	"log"
	"net/http"
	"os/exec"
)

func initAdvance() {

	// 定义 /reboot 接口
	handleAuthRoute("/reboot", rebootSystem)

	// 定义 /reset 接口
	handleAuthRoute("/reset", resetSystem)

	// 定义 /upload_update 接口
	handleAuthRoute("/upload_update", uploadUpdateHandler)

	// 定义 /upgrade_progress 接口
	handleAuthRoute("/upgrade_progress", upgradeProgressHandler)
}

// 重启系统
func rebootSystem(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("sudo", "reboot")
	err := cmd.Run()
	if err != nil {
		log.Printf("系统重启失败: %v\n", err)
		http.Error(w, "系统重启失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("系统正在重启..."))
}

// 恢复出厂设置
func resetSystem(w http.ResponseWriter, r *http.Request) {
	// 直接删除文件
	cmd := exec.Command("sh", "-c", "rm -rf /mnt/data/* /mnt/overlay/* && sync")
	err := cmd.Run()
	if err != nil {
		log.Printf("恢复出厂设置失败: %v\n", err)
		http.Error(w, "恢复出厂设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("恢复出厂设置成功 需要手动重启系统"))
}
