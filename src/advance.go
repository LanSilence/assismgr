package main

import (
	"bufio"
	"encoding/json"
	"errors"
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

var (
	errDownloadCancelled = errors.New("download cancelled by user")
)

// 进度跟踪Reader
type progressTrackingReader struct {
	Reader        io.Reader
	ContentLength int64
	Callback      func(readBytes int64)
	CancelChan    <-chan struct{}
	readBytes     int64
	once          sync.Once
}

func (r *progressTrackingReader) Read(p []byte) (n int, err error) {
	select {
	case <-r.CancelChan:
		return 0, errDownloadCancelled
	default:
	}

	n, err = r.Reader.Read(p)
	if n > 0 {
		r.readBytes += int64(n)
		r.once.Do(func() {
			if r.ContentLength <= 0 && r.Reader != nil {
				if lr, ok := r.Reader.(*io.LimitedReader); ok {
					r.ContentLength = lr.N + r.readBytes
				}
			}
		})
		if r.Callback != nil {
			r.Callback(r.readBytes)
		}
	}
	return
}

func initAdvance() {
	// 初始化取消通道
	cancelChan = make(chan struct{})

	// 定义 /reboot 接口
	handleAuthRoute("/reboot", rebootSystem)

	// 定义 /reset 接口
	handleAuthRoute("/reset", resetSystem)

	// 定义 /upload_update 接口
	handleAuthRoute("/upload_update", uploadUpdateHandler)

	// 定义 /upgrade_progress 接口
	handleAuthRoute("/upgrade_progress", upgradeProgressHandler)

	// 定义 /cancel_upgrade 接口
	handleAuthRoute("/cancel_upgrade", cancelUpgradeHandler)
}

// 取消升级
func cancelUpgradeHandler(w http.ResponseWriter, r *http.Request) {

	if upgradeStatus == "uploading" || upgradeStatus == "downloading" || upgradeStatus == "installing" {
		close(cancelChan)
		time.Sleep(time.Second * 2)
		cancelChan = make(chan struct{}) // 重新创建通道
		setUpgradeStatus("cancelled", 0, "升级已取消")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"cancelled"}`))
	} else {
		http.Error(w, "没有正在进行的升级", http.StatusBadRequest)
	}
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
	// 直接删除文件
	cmd := exec.Command("sh", "-c", "rm -rf /mnt/data/* /mnt/overlay/* && sync")
	err := cmd.Run()
	if err != nil {
		log.Printf("恢复出厂设置失败: %v\n", err)
		http.Error(w, "恢复出厂设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("恢复出厂设置成功 需要手动重启系统"))
}

var (
	upgradeProgressLock sync.Mutex
	upgradeProgress     int    // 0-100
	upgradeStatus       string // "idle", "uploading", "downloading", "installing", "done", "failed", "cancelled"
	upgradeMessage      string
	raucOutput          []string
	cancelChan          chan struct{} // 用于取消升级
)

func uploadUpdateHandler(w http.ResponseWriter, r *http.Request) {
	setUpgradeStatus("uploading", 0, "开始上传")

	contentType := r.Header.Get("Content-Type")
	switch {
	case strings.Contains(contentType, "multipart/form-data"):
		handleFileUpload(w, r)
	case strings.Contains(contentType, "application/json"):
		handleURLDownload(w, r)
	default:
		setUpgradeStatus("failed", 0, "不支持的Content-Type")
		http.Error(w, "不支持的Content-Type", http.StatusBadRequest)
	}
}

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	part, err := getMultipartFile(r, "updateFile")
	if err != nil {
		handleUploadError(w, err.Error())
		return
	}
	defer part.Close()

	localPath, err := createUpgradeFile("/mnt/data/upgrades", "update.raucb")
	if err != nil {
		handleUploadError(w, err.Error())
		return
	}

	err = copyWithProgress(part, localPath, r.ContentLength, func(total int64, progress int) {
		setUpgradeStatus("uploading", progress, fmt.Sprintf("上传中: %d/%d bytes", total, r.ContentLength))
	})
	if err != nil {
		setUpgradeStatus("failed", 0, err.Error())
		handleUploadError(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"upload_complete"}`))
	startBackgroundInstall(localPath)
}

func handleURLDownload(w http.ResponseWriter, r *http.Request) {
	url, err := getDownloadURL(r.Body)
	if err != nil {
		handleUploadError(w, err.Error())
		return
	}

	// 立即返回响应
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"download_started"}`))

	// 异步执行下载
	go func() {
		localPath, err := createUpgradeFile("/mnt/data/upgrades", "update.raucb")
		if err != nil {
			setUpgradeStatus("failed", 0, err.Error())
			return
		}

		resp, err := downloadFile(url)
		if err != nil {
			setUpgradeStatus("failed", 0, err.Error())
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			setUpgradeStatus("failed", 0, "下载失败: "+resp.Status)
			return
		}

		// 设置初始状态
		setUpgradeStatus("downloading", 0, "下载开始")

		// 执行下载并更新状态
		downloadWithProgress(resp.Body, localPath, resp.ContentLength, cancelChan)

		// 下载完成后启动安装
		startBackgroundInstall(localPath)
	}()
}

// 辅助函数区域
func getMultipartFile(r *http.Request, fieldName string) (*multipart.Part, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("创建多部分读取器失败: %w", err)
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取文件部分失败: %w", err)
		}
		if part.FormName() == fieldName {
			return part, nil
		}
		part.Close()
	}

	return nil, errors.New("未找到文件部分")
}

func createUpgradeFile(uploadDir, fileName string) (string, error) {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	localPath := filepath.Join(uploadDir, fileName)
	f, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	f.Close() // 关闭文件，写入操作将在复制函数中重新打开或追加

	return localPath, nil
}

func copyWithProgress(src io.Reader, dstPath string, contentLength int64, progressCallback func(int64, int)) error {
	dst, err := os.OpenFile(dstPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer dst.Close()

	buf := make([]byte, 32<<10)
	var total int64

	for {
		select {
		case <-cancelChan:
			return errors.New("操作已取消")
		default:
			n, err := src.Read(buf)
			if n > 0 {
				if _, wErr := dst.Write(buf[:n]); wErr != nil {
					return fmt.Errorf("写入文件失败: %w", wErr)
				}
				total += int64(n)

				if contentLength > 0 && progressCallback != nil {
					progress := int(float64(total) / float64(contentLength) * 100)
					progressCallback(total, progress)
				}
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("读取数据失败: %w", err)
			}
		}
	}
}

func getDownloadURL(body io.ReadCloser) (string, error) {
	var req struct{ URL string }
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return "", fmt.Errorf("解析请求失败: %w", err)
	}
	if req.URL == "" {
		return "", errors.New("URL不能为空")
	}
	return req.URL, nil
}

func downloadFile(url string) (*http.Response, error) {
	client := &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("下载文件失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("下载失败: %s", resp.Status)
	}

	return resp, nil
}

func downloadWithProgress(src io.Reader, dstPath string, contentLength int64, cancelCh <-chan struct{}) error {
	dst, err := os.OpenFile(dstPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer dst.Close()

	buf := make([]byte, 32<<10)
	var total int64

	for {
		select {
		case <-cancelCh:
			return errors.New("download cancelled")
		default:
			n, err := src.Read(buf)
			if n > 0 {
				if _, wErr := dst.Write(buf[:n]); wErr != nil {
					return fmt.Errorf("写入文件失败: %w", wErr)
				}
				total += int64(n)

				if contentLength > 0 {
					progress := int(float64(total) / float64(contentLength) * 100)
					setUpgradeStatus("downloading", progress, fmt.Sprintf("下载中: %d/%d bytes", total, contentLength))
				}
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("下载失败: %w", err)
			}
		}
	}
}

func startBackgroundInstall(localPath string) {
	go func() {
		if err := doRaucInstall(localPath); err != nil {
			setUpgradeStatus("failed", 0, "安装失败: "+err.Error())
		} else {
			setUpgradeStatus("done", 100, "升级完成")
		}
		os.Remove(localPath)
	}()
}

func handleUploadError(w http.ResponseWriter, message string) {
	setUpgradeStatus("failed", 0, message)
	http.Error(w, message, http.StatusInternalServerError)
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

	done := make(chan error, 1)
	go func() {
		// 解析RAUC输出获取精确进度
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			select {
			case <-cancelChan:
				return
			default:
				line := scanner.Text()
				raucOutput = append(raucOutput, line)

				// 解析进度信息 (例如: " 45% Copying image to rootfs.0")
				if strings.Contains(line, "%") {
					parts := strings.Fields(line)
					if len(parts) > 0 {
						percentStr := strings.TrimSuffix(parts[0], "%")
						if percent, err := strconv.Atoi(percentStr); err == nil {
							setUpgradeStatus("installing", percent, "安装中: ")
						}
					}
				}
			}
		}
		done <- cmd.Wait()
	}()

	select {
	case <-cancelChan:
		// 终止RAUC进程
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("终止RAUC进程失败: %v", err)
		}
		return fmt.Errorf("升级已取消")
	case err := <-done:
		if err != nil {
			return fmt.Errorf("RAUC安装失败: %v", err)
		}
		return nil
	}
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
