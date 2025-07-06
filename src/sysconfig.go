package main

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

const configPath = "/opt/config/frp/frpc.toml"

func sysconfigInit() {
	handleAuthRoute("/sysconfig/get", getConfigHandler)
	handleAuthRoute("/sysconfig/save", saveConfigHandler)
	handleAuthRoute("/sysconfig/restart", restartFrpcHandler)
}

func restartFrpcHandler(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("systemctl", "restart", "frpc")
	err := cmd.Run()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte("restart frp successfully"))
}

func getConfigHandler(w http.ResponseWriter, r *http.Request) {
	// 确保配置文件目录存在
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		http.Error(w, "Failed to create config directory", http.StatusInternalServerError)
		return
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在则返回空内容
			w.Write([]byte(""))
			return
		}
		http.Error(w, "Failed to read config file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}

func saveConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// 确保配置文件目录存在
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		http.Error(w, "Failed to create config directory", http.StatusInternalServerError)
		return
	}

	// 写入配置文件
	if err := os.WriteFile(configPath, body, 0644); err != nil {
		http.Error(w, "Failed to save config file", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Config saved successfully"))
}
