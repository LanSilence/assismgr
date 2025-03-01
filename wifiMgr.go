package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func initWifiMgr() {
	http.HandleFunc("/ap-status", apStatusHandler)
	http.HandleFunc("/toggle-ap", toggleAPHandler)
	http.HandleFunc("/scan", handleWLANScan)
	http.HandleFunc("/connect", handleConnectWLAN)
}

func parseWifiOutput(output string) []WifiNetwork {
	var networks []WifiNetwork
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// 调试：显示原始行内容

		// 预处理行
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "bssid") {
			continue
		}

		// 分割字段（自动处理任意空格/TAB）
		fields := strings.Fields(line)

		// 基础字段校验
		if len(fields) < 4 {
			continue
		}

		// 解析基础字段
		bssid := fields[0]
		freqStr := fields[1]
		signalStr := fields[2]

		// 动态识别flags字段（所有以[开头的连续字段）
		var flagsParts []string
		ssidStart := 3 // flags字段起始位置
		for i := 3; i < len(fields); i++ {
			if strings.HasPrefix(fields[i], "[") {
				flagsParts = append(flagsParts, fields[i])
				ssidStart = i + 1
			} else {
				break
			}
		}

		// 合并flags和SSID
		flags := strings.Join(flagsParts, " ")
		ssid := strings.Join(fields[ssidStart:], " ")

		// 类型转换
		freq, err := strconv.Atoi(freqStr)
		if err != nil {
			continue
		}

		signal, err := strconv.Atoi(signalStr)
		if err != nil {
			continue
		}

		// 调试输出

		networks = append(networks, WifiNetwork{
			BSSID:     bssid,
			Frequency: freq,
			Signal:    signal,
			Flags:     parseFlags(flags),
			SSID:      ssid,
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

func handleWLANScan(w http.ResponseWriter, r *http.Request) {
	// 执行扫描命令
	// fmt.Println("start scan handle")
	cmd := exec.Command("wpa_cli", "-i", "wlan0", "scan")
	if err := cmd.Run(); err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprintf("Scan init failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 等待扫描完成
	time.Sleep(2 * time.Second)

	// 获取扫描结果
	cmd = exec.Command("wpa_cli", "-i", "wlan0", "scan_result")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprintf("Scan results failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 解析结果
	networks := parseWifiOutput(string(output))
	jsonString, err := json.Marshal(map[string][]WifiNetwork{
		"networks": networks,
	})
	if err != nil {
		log.Println(err)
	}

	w.Write(jsonString)
}

// 检查hostapd状态
func isHostapdRunning() bool {
	cmd := exec.Command("systemctl", "is-active", "hostapd")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "active"
}

// AP状态端点
func apStatusHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]bool{
		"apRunning": isHostapdRunning(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// 切换AP端点
func toggleAPHandler(w http.ResponseWriter, r *http.Request) {
	isRunning := isHostapdRunning()
	action := "stop"
	if !isRunning {
		action = "start"
	}

	if action == "start" {
		exec.Command("ifconfig", "wlan1", "up")
	} else {
		exec.Command("ifconfig", "wlan1", "down")
	}
	cmd := exec.Command("sudo", "systemctl", action, "hostapd")
	if err := cmd.Run(); err != nil {
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("热点已%s", map[string]string{"start": "开启", "stop": "关闭"}[action]),
	}
	json.NewEncoder(w).Encode(response)
}
