package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
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

func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", "./assistmgr.db")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS wlan_history (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            ssid TEXT NOT NULL,
            password TEXT,
            timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
        );
        
        CREATE TABLE IF NOT EXISTS perf_log (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            cpu_usage FLOAT,
            mem_usage FLOAT,
            disk_usage FLOAT,
            timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		log.Fatal(err)
	}
	return db
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

func handleWLANScan(w http.ResponseWriter, r *http.Request) {
	// 执行扫描命令
	fmt.Println("start scan handle")
	// cmd := exec.Command("wpa_cli", "-i", "wlan0", "scan")
	// if err := cmd.Run(); err != nil {
	// 	http.Error(w, fmt.Sprintf("Scan init failed: %v", err), http.StatusInternalServerError)
	// 	return
	// }

	// 等待扫描完成
	time.Sleep(2 * time.Second)

	// 获取扫描结果
	// cmd = exec.Command("wpa_cli", "-i", "wlan0", "scan_result")

	// output, err := cmd.CombinedOutput()
	output := `ssid / frequency / signal level / flags / ssid
			a4:39:b3:cd:5e:45       2437    -34     [WPA2-PSK-CCMP][ESS]    Redmi_EA72 
			04:6b:25:1c:bf:7f       2452    -45     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    802-2.4G 
			ec:f8:eb:99:3a:b1       2462    -58     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    702-2.4G 
			04:6b:25:0c:9a:23       2422    -59     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    703-2.4G 
			04:6b:25:17:c7:b3       2412    -66     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    705-2.4G 
			04:6b:25:23:4d:63       2412    -69     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    ChinaNet-xKCZ 
			f4:32:3d:a7:f4:f7       2412    -71     [WPA-PSK-CCMP+TKIP][ESS]        905-2.4G
			ec:f8:eb:96:95:ef       2432    -71     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    801
			04:6b:25:2f:65:c1       2457    -78     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    806-2.4G
			8c:81:72:86:b4:37       2462    -80     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    1002-2.4G
			04:6b:25:2c:2d:6b       2417    -86     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    807-2.4G
			04:6b:25:18:56:fb       2452    -93     [WPA-PSK-CCMP+TKIP][WPA2-PSK-CCMP+TKIP][ESS]    602-2.4G
			aa:39:b3:cd:5e:45       2437    -33     [ESS]`
	// if err != nil {
	// 	http.Error(w, fmt.Sprintf("Scan results failed: %v", err), http.StatusInternalServerError)
	// 	return
	// }

	// 解析结果
	networks := parseWifiOutput(string(output))

	// 返回JSON响应
	// w.Header().Set("Content-Type", "application/json")
	// fmt.Println(networks)
	// json.NewEncoder(w).Encode(map[string][]WifiNetwork{
	// 	"networks": networks,
	// })
	jsonString, err := json.Marshal(map[string][]WifiNetwork{
		"networks": networks,
	})
	if err != nil {
		log.Println(err)
	}
	fmt.Println(string(jsonString))
	w.Write(jsonString)
}

func parseWifiOutput(output string) []WifiNetwork {
	var networks []WifiNetwork
	lines := strings.Split(output, "\n")

	// 正则匹配：bssid, frequency, signal, flags, ssid
	re := regexp.MustCompile(`^(\S+)\s+(\d+)\s+(-?\d+)\s+(.*?)\s{2,}(.*)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "bssid") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) != 6 {
			continue
		}

		// 解析flags
		flags := parseFlags(matches[4])

		// 转换数值类型
		freq, _ := strconv.Atoi(matches[2])
		signal, _ := strconv.Atoi(matches[3])

		networks = append(networks, WifiNetwork{
			BSSID:     matches[1],
			Frequency: freq,
			Signal:    signal,
			Flags:     flags,
			SSID:      matches[5],
		})
	}
	return networks
}

func parseFlags(flagsStr string) []string {
	// 提取类似[WPA2-PSK-CCMP][ESS]的标记
	flags := strings.Split(flagsStr, "][")
	for i := range flags {
		flags[i] = strings.Trim(flags[i], "[]")
	}
	return flags
}

func handleConnectWLAN(w http.ResponseWriter, r *http.Request) {
	result := "fail"
	fmt.Println(r.Body)
	ssid := r.FormValue("ssid")
	password := r.FormValue("password")
	// cmd := exec.Command("nmcli", "device", "wifi", "connect", ssid, "password", password)
	cmd := exec.Command("ls", "-ls")
	output, err := cmd.CombinedOutput()
	log.Printf("Connecting to %s: passwd :%s %s", ssid, password, output)
	if err != nil {
		result = "fail"
	} else {
		result = "success"
	}

	fmt.Fprintf(w, `{"status":"%s","output":"%s"}`, result, "ok")
}

/*
func handleConnectWLAN(w http.ResponseWriter, r *http.Request) {

	ssid := r.FormValue("ssid")
	password := r.FormValue("password")
	interfaceName := "wlan0" // 根据实际情况修改接口名

	// 生成 PSK（安全方式）
	pskCmd := exec.Command("wpa_passphrase", ssid, password)
	pskOutput, err := pskCmd.CombinedOutput()
	if err != nil {
		log.Printf("生成 PSK 失败: %v\n输出: %s", err, pskOutput)
		fmt.Fprintf(w, `{"status":"fail","reason":"psk_fail"}`)
		return
	}

	// 提取 PSK 值
	psk := extractPSK(string(pskOutput))
	if psk == "" {
		log.Printf("无法解析 PSK: %s", pskOutput)
		fmt.Fprintf(w, `{"status":"fail","reason":"psk_parse"}`)
		return
	}

	// 执行 wpa_cli 命令（需要 sudo 权限）
	cmds := []string{
		fmt.Sprintf("add_network"),                       // 返回网络ID
		fmt.Sprintf("set_network 0 ssid '\"%s\"'", ssid), // 设置 SSID
		fmt.Sprintf("set_network 0 psk %s", psk),         // 设置 PSK
		"enable_network 0",                               // 启用网络
		"save_config",                                    // 保存配置
	}

	var output strings.Builder
	for _, cmd := range cmds {
		fullCmd := exec.Command("sudo", "wpa_cli", "-i", interfaceName, strings.Fields(cmd)...)
		cmdOutput, err := fullCmd.CombinedOutput()
		output.WriteString(fmt.Sprintf("[CMD] %s\n%s\n", cmd, cmdOutput))

		if err != nil || !strings.Contains(string(cmdOutput), "OK") {
			log.Printf("命令执行失败: %s\n错误: %v\n输出: %s", cmd, err, cmdOutput)
			fmt.Fprintf(w, `{"status":"fail","reason":"cmd_fail","step":"%s"}`, cmd)
			return
		}
	}

	// 获取 IP 地址
	dhclientCmd := exec.Command("sudo", "dhclient", interfaceName)
	if dhclientOutput, err := dhclientCmd.CombinedOutput(); err != nil {
		log.Printf("DHCP 失败: %v\n输出: %s", err, dhclientOutput)
		output.WriteString(fmt.Sprintf("[DHCP] %s\n", dhclientOutput))
	}

	// 返回最终结果
	fmt.Fprintf(w, `{"status":"success","output":"%s"}`, strings.ReplaceAll(output.String(), "\"", "'"))
}

// 从 wpa_passphrase 输出中提取 PSK
func extractPSK(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "psk=") && !strings.Contains(line, "#") {
			parts := strings.Split(line, "psk=")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}
*/

func startWebSocket(db *sql.DB) {
	wsHandler := func(w http.ResponseWriter, r *http.Request) {
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
				db.Exec("INSERT INTO perf_log (cpu_usage, mem_usage, disk_usage) VALUES (?, ?, ?)",
					info.CPUUsage, info.MemUsage, info.DiskUsage)
				err := conn.WriteJSON(info)
				if err != nil {
					log.Println("WebSocket write error:", err)
					conn.Close()
					break
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

func main() {
	db := initDB()
	defer db.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./public/index.html")
	})
	// 配置静态文件服务
	fs := http.FileServer(http.Dir("./public"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 其他路由配置
	http.HandleFunc("/scan", handleWLANScan)
	http.HandleFunc("/connect", handleConnectWLAN)
	startWebSocket(db)

	log.Println("Starting AssistMgr on :4000")
	log.Fatal(http.ListenAndServe(":4000", nil))
}
