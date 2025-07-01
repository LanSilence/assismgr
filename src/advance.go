package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func initAdvance() {
	// 定义 /update 接口
	handleAuthRoute("/update", updateSystem)

	// 定义 /reboot 接口
	handleAuthRoute("/reboot", rebootSystem)

	// 定义 /reset 接口
	handleAuthRoute("/reset", resetSystem)

	// 定义 /upload_update 接口
	handleAuthRoute("/upload_update", uploadUpdateHandler)

	// 定义 /upgrade_progress 接口
	handleAuthRoute("/upgrade_progress", upgradeProgressHandler)
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
	cmd := exec.Command("rm", "-rf", "/data/*")
	err := cmd.Run()
	if err != nil {
		log.Printf("恢复出厂设置失败: %v\n", err)
		http.Error(w, "恢复出厂设置失败", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("恢复出厂设置成功"))
}

var (
	upgradeProgressLock sync.Mutex
	upgradeProgress     int    // 0-100
	upgradeStatus       string // "idle", "uploading", "installing", "done", "failed"
	upgradeMessage      string
	raucOutput          []string
)

func uploadUpdateHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 先设置状态再处理
	setUpgradeStatus("uploading", 0, "开始上传")

	// 2. 直接使用 MultipartReader 流式处理，避免内存问题
	reader, err := r.MultipartReader()
	if err != nil {
		setUpgradeStatus("failed", 0, "创建多部分读取器失败: "+err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 3. 查找文件部分
	var part *multipart.Part
	for {
		p, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			setUpgradeStatus("failed", 0, "读取文件部分失败: "+err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if p.FormName() == "updateFile" {
			part = p
			break
		}
		p.Close()
	}

	if part == nil {
		setUpgradeStatus("failed", 0, "未找到文件部分")
		http.Error(w, "未找到文件", http.StatusBadRequest)
		return
	}
	defer part.Close()

	// 4. 创建目标文件
	uploadDir := "/mnt/data/upgrades"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		setUpgradeStatus("failed", 0, "创建目录失败: "+err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	localPath := filepath.Join(uploadDir, "update.raucb")
	f, err := os.Create(localPath)
	if err != nil {
		setUpgradeStatus("failed", 0, "创建文件失败: "+err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// 5. 流式复制并更新进度
	buf := make([]byte, 32<<10) // 32KB缓冲区
	var total int64
	for {
		n, err := part.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				setUpgradeStatus("failed", 0, "写入文件失败: "+err.Error())
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			total += int64(n)

			// 更新进度 (如果有Content-Length)
			if r.ContentLength > 0 {
				progress := int(float64(total) / float64(r.ContentLength) * 80)
				setUpgradeStatus("uploading", progress, fmt.Sprintf("上传中: %d/%d bytes", total, r.ContentLength))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			setUpgradeStatus("failed", 0, "读取文件失败: "+err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// 6. 启动安装
	go func() {
		if err := doRaucInstall(localPath); err != nil {
			setUpgradeStatus("failed", 0, "安装失败: "+err.Error())
		} else {
			setUpgradeStatus("done", 100, "升级完成")
		}
		os.Remove(localPath) // 删除升级文件释放空间
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"upload_complete"}`))
}

func doRaucInstall(pkg string) error {
	setUpgradeStatus("installing", 80, "开始安装升级包")

	cmd := exec.Command("rauc", "install", pkg)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建输出管道失败: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动RAUC失败: %v", err)
	}

	// 解析RAUC输出获取精确进度
	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := scanner.Text()
		raucOutput = append(raucOutput, line)

		// 解析进度信息 (例如: " 45% Copying image to rootfs.0")
		if strings.Contains(line, "%") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				percentStr := strings.TrimSuffix(parts[0], "%")
				if percent, err := strconv.Atoi(percentStr); err == nil {
					// 将RAUC进度映射到80-100%范围 (因为上传占0-80%)
					// progress := 80 + int(float64(percent)*0.2)
					setUpgradeStatus("installing", percent, "安装中: ")
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("RAUC安装失败: %v", err)
	}

	return nil
}

func upgradeProgressHandler(w http.ResponseWriter, r *http.Request) {
	upgradeProgressLock.Lock()
	defer upgradeProgressLock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"progress":  upgradeProgress,
		"status":    upgradeStatus,
		"message":   upgradeMessage,
		"output":    raucOutput,
		"timestamp": time.Now().Unix(),
	})
}

func setUpgradeStatus(status string, progress int, message string) {
	upgradeProgressLock.Lock()
	defer upgradeProgressLock.Unlock()

	upgradeStatus = status
	upgradeProgress = progress
	upgradeMessage = message

	// 限制日志大小
	if len(raucOutput) > 100 {
		raucOutput = raucOutput[len(raucOutput)-100:]
	}
}

func resetUpgradeStatus() {
	upgradeProgressLock.Lock()
	defer upgradeProgressLock.Unlock()

	upgradeStatus = "idle"
	upgradeProgress = 0
	upgradeMessage = ""
	raucOutput = nil
}
