package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"
)

const (
	ON  bool = true
	OFF bool = false
)

func handleGetStatus(w http.ResponseWriter) {
	data, err := os.ReadFile("/mnt/data/ledstatus")
	if err != nil {
		if os.IsNotExist(err) {
			sendErrorResponse(w, http.StatusNotFound, "状态文件不存在")
		} else {
			sendErrorResponse(w, http.StatusInternalServerError, "无法读取状态")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "%s"}`, strings.ToUpper(string(data)))
}

type Ledstatus struct {
	STATUS string `json:status`
}

func handlePostRequest(w http.ResponseWriter, r *http.Request) {
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendErrorResponse(w, http.StatusBadRequest, "无效的请求体")
		fmt.Println("无效的请求体")
		return
	}
	fmt.Println(string(body))
	defer r.Body.Close()

	// 解析状态
	var statusJson Ledstatus
	// status := strings.ToLower(strings.TrimSpace(string(body)))
	json.Unmarshal(body, &statusJson)
	if statusJson.STATUS != "ON" && statusJson.STATUS != "OFF" {
		sendErrorResponse(w, http.StatusBadRequest, "无效的状态值，必须为on或off")
		return
	}

	// 转换状态并执行控制
	if err := switchLed(statusJson.STATUS == "ON"); err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "状态更新失败")
		log.Println(err)
		return
	}

	// 返回更新后的状态
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "%s"}`, strings.ToUpper(statusJson.STATUS))
}

func sendErrorResponse(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"error": {"code": %d, "message": "%s"}}`, code, message)
}

// 初始化LED状态
func ledInit() error {
	// 检查状态文件是否存在
	if _, err := os.Stat("/mnt/data/ledstatus"); os.IsNotExist(err) {
		// 文件不存在时创建并初始化ON
		if err := switchLed(ON); err != nil {
			return fmt.Errorf("初始化失败: %w", err)
		}
		return nil
	}

	// 读取保存的状态
	data, err := os.ReadFile("/mnt/data/ledstatus")
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}

	// 解析状态值
	switch strings.TrimSpace(string(data)) {
	case "ON":
		return switchLed(ON)
	case "OFF":
		return switchLed(OFF)
	default:
		// 无效状态时重置为ON
		return switchLed(ON)
	}
}

func controlLed(status bool) {
	var Ledstatus string

	if status {
		Ledstatus = getLedStatus()
	} else {
		Ledstatus = "off"
	}
	cmd := exec.Command("led-control", Ledstatus)
	cmd.Run()
}

// 控制LED并保存状态
func switchLed(status bool) error {

	controlLed(status)

	// 保存当前状态
	return saveLedStatus(status)
}

// 写入单个LED设备
func writeLedDevice(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Printf("设备控制失败(%s): %w (可能需要root权限)", path, err)
		return fmt.Errorf("设备控制失败(%s): %w (可能需要root权限)", path, err)
	}
	return nil
}

// 持久化保存状态
func saveLedStatus(status bool) error {

	// 生成状态字符串
	statusStr := "OFF"
	if status == ON {
		statusStr = "ON"
	}

	// 原子化写入操作
	tempFile := "/mnt/data/ledstatus.tmp"
	if err := os.WriteFile(tempFile, []byte(statusStr), 0644); err != nil {
		return fmt.Errorf("临时文件写入失败: %w", err)
	}

	// 原子替换文件
	if err := os.Rename(tempFile, "/mnt/data/ledstatus"); err != nil {
		return fmt.Errorf("文件替换失败: %w", err)
	}

	return nil
}

type BootControl struct {
	ActiveSlot     byte     // 当前活动分区（0=A, 1=B）
	RetryCount     byte     // 剩余重试次数
	SuccessfulBoot byte     // 上次是否成功启动
	Reserved       [13]byte // 对齐到 16 字节
}

func WriteBootControl(bc *BootControl) error {
	// 将结构体指针转换为字节数组指针
	size := int(unsafe.Sizeof(*bc))
	byteSlice := (*[16]byte)(unsafe.Pointer(bc))[:size:size]

	// 原子写入（避免系统崩溃导致数据半写入）
	return os.WriteFile("/mnt/ata/bootfile", byteSlice, 0644)
}

func systemStartUp() {
	data, err := os.ReadFile("/mnt/data/bootfile")
	if err != nil {
		log.Println(err)
	}

	buf := bytes.NewReader(data)
	var bc BootControl
	err = binary.Read(buf, binary.LittleEndian, &bc) // 根据实际字节序选择
	if err != nil {
		log.Println(err)
	}
	log.Println(bc)
	if bc.SuccessfulBoot == 1 {
		log.Println("boot success!")
		return
	}
	bc.SuccessfulBoot = 1
	err = WriteBootControl(&bc)
	if err != nil {
		log.Println(err)
	}
}

const (
	STATUS_LED_OFF   = 0 // LED关闭状态
	STATUS_SYSTEM_ON = 1 // 系统开机状态，ip未获取
	STATUS_IP_OK     = 2 // IP已获取
	STATUS_NETWORK   = 3 // 网络已连接
	STATUS_UNKNOWN   = 4 // 未知状态
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
