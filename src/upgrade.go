package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

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
	var filename string
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
			filename = p.FileName()
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
	isDelta := strings.HasSuffix(strings.ToLower(filename), ".xdelta") ||
		strings.HasSuffix(strings.ToLower(filename), ".patch")
	originalPath := filepath.Join(uploadDir, filename)
	installPath := originalPath
	f, err := os.Create(originalPath)
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

	if isDelta {
		setUpgradeStatus("merging", 50, "合并差分包")

		// 创建合并路径
		installPath = filepath.Join(uploadDir, "merged-update-"+filepath.Base(originalPath)+".raucb")

		// 调用合并函数
		if err := mergeDelta(originalPath, installPath); err != nil {
			setUpgradeStatus("failed", 0, "合并失败: "+err.Error())
			http.Error(w, "差分包合并失败", http.StatusInternalServerError)
			return
		}

		setUpgradeStatus("merged", 100, "差分包合并完成")
	}

	// 6. 启动安装
	go func() {
		if err := doRaucInstall(installPath); err != nil {
			setUpgradeStatus("failed", 0, "安装失败: "+err.Error())
			return
		}
		if err := updateBaseFile(installPath); err != nil {
			setUpgradeStatus("warning", 100, "安装成功，但更新基础文件失败: "+err.Error())
			return
		}
		setUpgradeStatus("done", 100, "升级完成")
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"upload_complete"}`))
}

// 新增函数：更新基础文件
func updateBaseFile(installedPath string) error {
	basePath := "/mnt/data/upgrades/base.raucb"

	// 删除旧基础文件
	if err := os.Remove(basePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除旧基础文件失败: %w", err)
	}

	// 复制为新基础文件
	src, err := os.Open(installedPath)
	if err != nil {
		return fmt.Errorf("打开安装文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(basePath)
	if err != nil {
		return fmt.Errorf("创建基础文件失败: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	return nil
}

func mergeDelta(deltaPath, outputPath string) error {
	// 基础文件路径 (确保存在)
	basePath := "/mnt/data/upgrades/base.raucb"
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return fmt.Errorf("基础文件不存在，请先上传完整包")
	}

	// 执行xdelta3命令合并文件
	cmd := exec.Command("xdelta3", "-d", "-B", "200M", "-W", "12", "-s", basePath, deltaPath, outputPath)

	// 运行命令
	if err := cmd.Run(); err != nil {
		// 获取错误输出
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("%s\n%s", err, string(ee.Stderr))
		}
		return err
	}

	// 验证输出文件
	if info, err := os.Stat(outputPath); err != nil || info.Size() == 0 {
		return fmt.Errorf("合并后的文件无效")
	}

	return nil
}

func doRaucInstall(pkg string) error {
	setUpgradeStatus("installing", 0, "开始安装升级包")

	for i := 0; i <= 10; i++ {
		time.Sleep(time.Second * 2)
		setUpgradeStatus("installing", i*10, "安装中: ")
	}
	// cmd := exec.Command("rauc", "install", pkg)
	// stdoutPipe, err := cmd.StdoutPipe()
	// if err != nil {
	// 	return fmt.Errorf("创建输出管道失败: %v", err)
	// }

	// if err := cmd.Start(); err != nil {
	// 	return fmt.Errorf("启动RAUC失败: %v", err)
	// }

	// // 解析RAUC输出获取精确进度
	// scanner := bufio.NewScanner(stdoutPipe)
	// for scanner.Scan() {
	// 	line := scanner.Text()
	// 	raucOutput = append(raucOutput, line)

	// 	// 解析进度信息 (例如: " 45% Copying image to rootfs.0")
	// 	if strings.Contains(line, "%") {
	// 		parts := strings.Fields(line)
	// 		if len(parts) > 0 {
	// 			percentStr := strings.TrimSuffix(parts[0], "%")
	// 			if percent, err := strconv.Atoi(percentStr); err == nil {
	// 				setUpgradeStatus("installing", percent, "安装中: ")
	// 			}
	// 		}
	// 	}
	// }

	// if err := cmd.Wait(); err != nil {
	// 	return fmt.Errorf("RAUC安装失败: %v", err)
	// }

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
