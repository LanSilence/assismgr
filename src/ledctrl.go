package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	LED_STATE_PATH = "/mnt/data/assismgr/ledstatus"
)

var ledList []string

var ledStateMutex sync.Mutex

// saveLedStatus 保存 LED 状态到 JSON 文件
// ledName: LED 名称（如 "sys_led"）
// status: true 表示 ON，false 表示 OFF
func saveLedStatus(ledName string, ledMode string) error {
	// 加锁确保线程安全
	ledStateMutex.Lock()
	defer ledStateMutex.Unlock()

	// 1. 尝试读取现有状态
	states, err := readLedStates()
	if err != nil {
		// 如果文件不存在或无效，初始化空状态
		states = make(map[string]string)
	}

	// 2. 更新指定 LED 的状态
	states[ledName] = ledMode

	// 3. 将更新后的状态写回文件
	return writeLedStates(states)
}

// readLedStates 从文件读取所有 LED 状态
func readLedStates() (map[string]string, error) {
	// 检查文件是否存在
	if _, err := os.Stat(LED_STATE_PATH); os.IsNotExist(err) {
		return make(map[string]string), nil
	}

	// 读取文件内容
	data, err := os.ReadFile(LED_STATE_PATH)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 解析 JSON
	var states map[string]string
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	return states, nil
}

// writeLedStates 将 LED 状态写入文件
func writeLedStates(states map[string]string) error {
	// 创建 JSON 数据
	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON编码失败: %w", err)
	}

	// 写入文件（使用 0644 权限：用户读写，组只读，其他只读）
	if err := os.WriteFile(LED_STATE_PATH, data, 0644); err != nil {
		log.Println("写入文件失败:", err)
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}
func getAllLed() []string {

	var ledList []string
	// 使用 Go 的 Glob 函数匹配路径
	ledPaths, err := filepath.Glob("/sys/class/leds/*")
	if err != nil {
		fmt.Printf("查找LED设备失败: %v\n", err)
		return nil
	}

	if len(ledPaths) == 0 {
		fmt.Println("未找到任何LED设备")
		return nil
	}

	for _, ledPath := range ledPaths {
		if ledPath == "" {
			continue // 跳过空行
		}
		ledList = append(ledList, filepath.Base(ledPath))
	}
	return ledList
}

func ledInit() error {
	// 检查状态文件是否存在
	ledList = getAllLed()
	if len(ledList) == 0 {
		return fmt.Errorf("未找到任何LED设备")
	}

	if _, err := os.Stat(LED_STATE_PATH); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(LED_STATE_PATH), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		// 文件不存在时创建并初始化ON
		log.Println("LED状态文件不存在，创建新文件并初始化状态")
		states := make(map[string]string)
		for _, ledName := range ledList {
			if ledName == "sys_led" {
				states[ledName] = getLedStatus()
			} else {
				states[ledName] = LED_MODE_ON // 默认状态为ON
			}
		}
		writeLedStates(states)
		switchLed(true) // 初始化 sys_led 为 ON
		return nil
	}
	ledState, _ := readLedStates()
	if (ledState["sys_led"] == "OFF") || (ledState["sys_led"] == "off") {
		ledState["sys_led"] = "off" // 如果都是关闭的状态，直接关掉
	} else {
		ledState["sys_led"] = getLedStatus() // 否则获取当前LED状态
	}
	// 遍历所有LED，设置状态
	for _, ledName := range ledList {
		setLedMode(ledName, ledState[ledName])
	}

	// 读取保存的状态
	states, err := readLedStates()
	if err != nil {
		return fmt.Errorf("读取LED状态失败: %w", err)
	}
	// 遍历所有LED，设置状态
	for _, ledName := range ledList {
		if ledName == "sys_led" {
			switchLed(states[ledName] == "on")
		} else {
			exec.Command("led-control", ledName, states[ledName]).Run()
		}
	}
	return nil
}

// 只控制sys_led
func controlSysLed(status bool) error {
	var Ledstatus string

	if status {
		Ledstatus = getLedStatus()
	} else {
		Ledstatus = "off"
	}
	cmd := exec.Command("led-control", "sys_led", Ledstatus)
	cmd.Run()
	return saveLedStatus("sys_led", Ledstatus)
}

// 控制LED并保存状态
func switchLed(status bool) error {

	return controlSysLed(status)

}

func setLedMode(ledName string, ledMode string) error {
	// 加锁确保线程安全
	ledStateMutex.Lock()
	defer ledStateMutex.Unlock()

	// 1. 尝试读取现有状态
	states, err := readLedStates()
	if err != nil {
		return fmt.Errorf("读取LED状态失败: %w", err)
	}

	// 如果LED不存在，返回错误
	if _, exists := states[ledName]; !exists {
		return fmt.Errorf("LED %s 不存在", ledName)
	}

	// 2. 更新指定 LED 的模式
	states[ledName] = ledMode
	exec.Command("led-control", ledName, ledMode).Run()
	// 3. 将更新后的状态写回文件
	return writeLedStates(states)
}

const (
	STATUS_LED_OFF   = 0 // LED关闭状态
	STATUS_SYSTEM_ON = 1 // 系统开机状态，ip未获取
	STATUS_IP_OK     = 2 // IP已获取
	STATUS_NETWORK   = 3 // 网络已连接
	STATUS_UNKNOWN   = 4 // 未知状态
)

const (
	LED_MODE_OFF       = "off"
	LED_MODE_ON        = "on"
	LED_MODE_HEARTBEAT = "heartbeat"
	LED_MODE_SLOW      = "slow"
	LED_MODE_FAST      = "fast"
	LED_MODE_DEFAULT   = "default" // 默认状态
)

var ledStatusMap = map[int]string{
	STATUS_SYSTEM_ON: "fast",      // 系统开机状态，ip未获取
	STATUS_IP_OK:     "slow",      // IP已获取
	STATUS_NETWORK:   "heartbeat", // 网络已连接
	STATUS_UNKNOWN:   "on",        // 未知状态
	STATUS_LED_OFF:   "off",       // LED关闭状态
}

func getStoredLedStatus() string {
	// 读取LED状态文件
	data, err := os.ReadFile("/mnt/data/ledstatus")
	if err != nil {
		log.Println("读取LED状态文件失败:", err)
		return "ON" // 如果读取失败，返回OFF状态
	}
	status := strings.TrimSpace(string(data))
	if status == "OFF" {
		return "OFF"
	}
	return "ON"
}

func getLedStatus() string {
	var ledstatus int
	var out bytes.Buffer
	// 执行 ping 命令获取网络状态
	netstatus := isOnlineStatus

	if !netstatus {
		ledstatus = STATUS_IP_OK
	} else {
		ledstatus = STATUS_NETWORK
		return ledStatusMap[ledstatus]
	}

	// 执行 ifconfig 命令获取网络状态
	for i := 0; i < 10; i++ {
		cmd := exec.Command("ifconfig")
		out.Reset()
		cmd.Stdout = &out
		err := cmd.Run()
		if err != nil {
			break
		}
		if strings.Contains(out.String(), "inet ") {

			lines := strings.Split(out.String(), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "inet ") {
					fields := strings.Fields(line)
					if len(fields) > 1 {
						ledstatus = STATUS_IP_OK
						return ledStatusMap[ledstatus]
					}
				}
			}
		}
	}

	return ledStatusMap[STATUS_SYSTEM_ON] // 如果没有获取到IP，则返回系统开机状态
}

func updateLed() {
	// 读取LED状态文件
	var preLedStatus string

	go checkInternet()
	for {

		// 每10次检查一次LED状态
		Ledstatus := getLedStatus()

		if Ledstatus != preLedStatus { // 如果状态发生变化
			cmd := exec.Command("led-control", Ledstatus)
			cmd.Run()
			preLedStatus = Ledstatus
			// 如果当前LED状态是OFF，并且网络状态是正常的，则需要关闭LED
			if getStoredLedStatus() == "OFF" && Ledstatus == ledStatusMap[STATUS_NETWORK] {
				cmd := exec.Command("led-control", "off") // 关闭LED
				cmd.Run()
			}
		}
		time.Sleep(10 * time.Second) // 每秒检查一次
	}
}
