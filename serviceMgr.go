package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

// 定义服务结构体
type Service struct {
	Name      string `json:"name"`
	IsEnableD bool   `json:"isEnabled"`
	IsActive  bool   `json:"isActive"`
}

// 可安装的服务列表
var availableServices = []string{
	"hostapd",
	"home-assistant",
	"nginx",
	"apache2",
	"mysql-server",
	"redis-server",
	"frpc",
}

func initServiceMgr() {
	// 定义 /services 接口
	http.HandleFunc("/services", getServices)

	// 定义 /service/install 接口
	http.HandleFunc("/service/install", installService)

	// 定义 /service/remove 接口
	http.HandleFunc("/service/enable", enableService)

	// 定义 /service/stop 接口
	http.HandleFunc("/service/ctrl", ctrlService)

	// 定义 /service/restart 接口
	http.HandleFunc("/service/restart", restartService)

}

// 获取服务列表
func getServices(w http.ResponseWriter, r *http.Request) {
	var services []Service

	// 检查每个服务是否已安装
	for _, name := range availableServices {

		enable := isServiceEnable(name)
		active := isServiceActive(name)
		services = append(services, Service{
			Name:      name,
			IsEnableD: enable,
			IsActive:  active,
		})
		fmt.Println(services)
	}

	// 返回 JSON 响应
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(services); err != nil {
		log.Printf("JSON 编码失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
	}
}

// 检查服务是否已安装
func isServiceEnable(name string) bool {
	cmd := exec.Command("systemctl", "is-enabled", name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	fmt.Println(name, string(output))
	result := strings.TrimSpace(string(output))
	return result == "enabled"
}

func isServiceActive(name string) bool {
	cmd := exec.Command("systemctl", "is-active", name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	fmt.Println(name, string(output))
	result := strings.TrimSpace(string(output))
	return result == "active"
}

// 安装服务
func installService(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("name")
	if service == "" {
		http.Error(w, "服务名称不能为空", http.StatusBadRequest)
		return
	}

	cmd := exec.Command("apt", "install", "-y", service)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("安装服务 %s 失败: %v\n", service, err)
		http.Error(w, "安装服务失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("服务安装成功"))
}

// 设置服务
func enableService(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("name")
	status := r.URL.Query().Get("status")
	if service == "" {
		http.Error(w, "服务名称不能为空", http.StatusBadRequest)
		return
	}
	if status == "" {
		http.Error(w, "服务状态错误", http.StatusBadRequest)
		return
	}

	if status != "enbale" {
		status = "disable"
	}
	cmd := exec.Command("systemctl", status, service)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("服务 %s %s 失败: %v\n", status, service, err)
		http.Error(w, "操作服务失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("服务操作成功"))
}

// 停止服务
func ctrlService(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("name")
	if service == "" {
		http.Error(w, "服务名称不能为空", http.StatusBadRequest)
		return
	}
	ctrl := r.URL.Query().Get("ctrl")
	if ctrl != "start" {
		ctrl = "stop"
	}
	cmd := exec.Command("sudo", "systemctl", ctrl, service)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("%s服务 %s 失败: %v\n", ctrl, service, err)
		http.Error(w, "操作服务失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("服务操作成功"))
}

// 重启服务
func restartService(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("name")
	if service == "" {
		http.Error(w, "服务名称不能为空", http.StatusBadRequest)
		return
	}

	cmd := exec.Command("sudo", "systemctl", "restart", service)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("重启服务 %s 失败: %v\n", service, err)
		http.Error(w, "重启服务失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("服务重启成功"))
}
