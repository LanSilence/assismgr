package main

import (
	"bytes"
	"log"
	"net/http"
	"os/exec"
)

func initAdvance() {
	// 定义 /update 接口
	handleAuthRoute("/update", updateSystem)

	// 定义 /reboot 接口
	handleAuthRoute("/reboot", rebootSystem)

	// 定义 /reset 接口
	handleAuthRoute("/reset", resetSystem)
}

// 更新系统
func updateSystem(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("sudo", "apt", "update", "&&", "sudo", "apt", "upgrade", "-y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("系统更新失败: %v\n", err)
		http.Error(w, "系统更新失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("系统更新成功"))
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
	// 这里可以根据实际需求实现恢复出厂设置的逻辑
	// 例如：删除配置文件、重置数据库等
	cmd := exec.Command("sudo", "rm", "-rf", "/data/*")
	err := cmd.Run()
	if err != nil {
		log.Printf("恢复出厂设置失败: %v\n", err)
		http.Error(w, "恢复出厂设置失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("恢复出厂设置成功"))
}
