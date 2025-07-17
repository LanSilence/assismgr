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
