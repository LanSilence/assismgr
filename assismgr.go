package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type SystemInfo struct {
	CPUUsage  float64 `json:"cpu_usage"`
	MemUsage  float64 `json:"mem_usage"`
	DiskUsage float64 `json:"disk_usage"`
}

func parseSSIDs(output string) []string {
	var ssids []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "ESSID") {
			ssid := strings.TrimSpace(strings.Split(line, "=")[1])
			ssids = append(ssids, ssid)
		}
	}
	return ssids
}

type WifiNetwork struct {
	BSSID     string   `json:"bssid"`
	Frequency int      `json:"frequency"`
	Signal    int      `json:"signal"`
	Flags     []string `json:"flags"`
	SSID      string   `json:"ssid"`
}

func startWebSocket() {
	wsHandler := func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			log.Println(r.URL, "Invalid token")
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
			return
		}

		_, err := validateToken(token)
		if err != nil {
			log.Println(r.URL, "Invalid token")
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			return
		}
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				info := getSystemInfo()
				err := conn.WriteJSON(info)
				if err != nil {
					log.Println("WebSocket write error:", err)
					conn.Close()
					return
				}
			}
		}
	}

	http.HandleFunc("/ws", wsHandler)
}

func getSystemInfo() SystemInfo {
	var cpuPercent float64
	var memPercent, diskPercent float64

	// CPU
	percentages, _ := cpu.Percent(time.Second, false) // 第二个参数设为 false 表示不按CPU核单独统计
	cpuPercent = percentages[0]

	// Memory
	mem, _ := mem.VirtualMemory()
	memPercent = float64(mem.Used) / float64(mem.Total) * 100

	// Disk
	disks, _ := disk.Usage("/")
	diskPercent = float64(disks.Used) / float64(disks.Total) * 100

	return SystemInfo{
		CPUUsage:  cpuPercent,
		MemUsage:  memPercent,
		DiskUsage: diskPercent,
	}
}

func netStauts(w http.ResponseWriter, r *http.Request) {

	type NetWorkStatus struct {
		Netstaus  bool    `json:"netstaus"`
		Downspeed float32 `json:"downspeed"`
		Upspeed   float32 `json:"upspeed"`
	}

	var netstaus bool
	cmd := exec.Command("ping", "-c 2", "www.baidu.com")
	err := cmd.Run()
	if err != nil {
		netstaus = false
	} else {
		netstaus = true
	}
	status := NetWorkStatus{
		Netstaus:  netstaus,
		Downspeed: 34.54,
		Upspeed:   10.33,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("JSON 编码失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
	}
}

func getServerLogs(w http.ResponseWriter, r *http.Request) {
	// 读取日志文件
	data, err := os.ReadFile("./log.txt")
	if err != nil {
		log.Printf("读取日志文件失败: %v\n", err)
		http.Error(w, "无法读取日志文件", http.StatusInternalServerError)
		return
	}

	// 构建响应
	type LogResponse struct {
		Output string `json:"output"`
	}
	response := LogResponse{
		Output: string(data),
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 返回 JSON 响应
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON 编码失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
	}
}

func getSystemLogs(w http.ResponseWriter, r *http.Request) {
	// 调用 journalctl 命令获取系统日志
	cmd := exec.Command("journalctl", "-n", "100") // 获取最近的 100 条日志
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("调用 journalctl 失败: %v\n", err)
		http.Error(w, "无法获取系统日志", http.StatusInternalServerError)
		return
	}

	// 构建响应
	type LogResponse struct {
		Output string `json:"output"`
	}
	response := LogResponse{
		Output: out.String(),
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 返回 JSON 响应
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("JSON 编码失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
	}
}

// 鉴权中间件
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从 Header 获取 Token
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			log.Println(r.URL, "No token!")
			http.ServeFile(w, r, "./public/login.html")
			return
		}

		// 解析验证 Token
		claims, err := validateToken(tokenString)
		if err != nil {
			log.Println("Invalid token")
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
			return
		}

		// 将用户名存入请求上下文
		ctx := context.WithValue(r.Context(), "username", claims.Subject)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func handleAuthRoute(pattern string, handler http.HandlerFunc) {
	http.HandleFunc(pattern, authMiddleware(handler))
}

func httpSwitchLed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST")

	switch r.Method {
	case http.MethodGet:
		handleGetStatus(w)
	case http.MethodPost:
		handlePostRequest(w, r)
	default:
		sendErrorResponse(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

func main() {

	systemStartUp()
	ledInit()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./public/index.html")
	})
	// 配置静态文件服务
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	// 其他路由配置
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./public/images/favicon.ico")
	})
	initLogin()
	handleAuthRoute("/serverlogs", getServerLogs)
	handleAuthRoute("/systemlogs", getSystemLogs)
	handleAuthRoute("/netstatus", netStauts)
	handleAuthRoute("/ledstatus", httpSwitchLed)
	initServiceMgr()
	initAdvance()
	initWifiMgr()
	startWebSocket()
	log.Println("Starting AssistMgr on :4000")
	log.Fatal(http.ListenAndServe(":4000", nil))
}
