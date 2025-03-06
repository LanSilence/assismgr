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
	"strings"
	"unsafe"
)

const (
	ON  bool = true
	OFF bool = false
)

func handleGetStatus(w http.ResponseWriter) {
	data, err := os.ReadFile("/data/ledstatus")
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
	if _, err := os.Stat("/data/ledstatus"); os.IsNotExist(err) {
		// 文件不存在时创建并初始化ON
		if err := switchLed(ON); err != nil {
			return fmt.Errorf("初始化失败: %w", err)
		}
		return nil
	}

	// 读取保存的状态
	data, err := os.ReadFile("/data/ledstatus")
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

// 控制LED并保存状态
func switchLed(status bool) error {
	// 准备控制信号
	ledValue := []byte{'0'}
	if status == ON {
		ledValue = []byte{'1'}
	}

	// 定义要控制的LED设备
	ledDevices := []string{
		"/sys/class/leds/firefly:blue:power/brightness",
		"/sys/class/leds/firefly:yellow:power/brightness",
	}

	// 批量写入LED设备
	for _, dev := range ledDevices {
		if err := writeLedDevice(dev, ledValue); err != nil {
			return err
		}
	}

	// 保存当前状态
	return saveLedStatus(status)
}

// 写入单个LED设备
func writeLedDevice(path string, data []byte) error {
	// if err := os.WriteFile(path, data, 0644); err != nil {
	// 	fmt.Printf("设备控制失败(%s): %w (可能需要root权限)", path, err)
	// 	return fmt.Errorf("设备控制失败(%s): %w (可能需要root权限)", path, err)
	// }
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
	tempFile := "/data/ledstatus.tmp"
	if err := os.WriteFile(tempFile, []byte(statusStr), 0644); err != nil {
		return fmt.Errorf("临时文件写入失败: %w", err)
	}

	// 原子替换文件
	if err := os.Rename(tempFile, "/data/ledstatus"); err != nil {
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
	return os.WriteFile("/data/bootfile", byteSlice, 0644)
}

func systemStartUp() {
	data, err := os.ReadFile("/data/bootfile")
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
