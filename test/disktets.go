package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
)

func main() {
	mountPoint := "/" // 替换为你的挂载点

	// 获取磁盘使用情况
	usage, err := disk.Usage(mountPoint)
	if err != nil {
		fmt.Printf("获取磁盘信息失败: %v\n", err)
		return
	}

	// 获取 / 挂载点对应的设备
	partitions, err := disk.Partitions(false)
	if err != nil {
		fmt.Printf("获取分区信息失败: %v\n", err)
		return
	}
	var device string
	for _, p := range partitions {
		if p.Mountpoint == mountPoint {
			device = p.Device // 例如 /dev/sda1
			break
		}
	}
	if device == "" {
		fmt.Println("未找到挂载点对应的设备")
		return
	}

	// 使用 lsblk -no pkname 获取物理磁盘名
	cmd := exec.Command("lsblk", "-no", "pkname", device)
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("lsblk 命令执行失败: %v\n", err)
		return
	}
	phyDisk := "/dev/" + string(output)
	phyDisk = phyDisk[:len(phyDisk)-1] // 去掉换行

	diskName := strings.TrimPrefix(phyDisk, "/dev/")
	sizePath := "/sys/block/" + diskName + "/size"
	data, err := os.ReadFile(sizePath)
	if err != nil {
		fmt.Printf("读取物理磁盘大小失败: %v\n", err)
		return
	}
	sectorsStr := strings.TrimSpace(string(data))
	sectors, err := strconv.ParseUint(sectorsStr, 10, 64)
	if err != nil {
		fmt.Printf("解析磁盘扇区数失败: %v\n", err)
		return
	}
	totalBytes := sectors * 512
	fmt.Printf("物理磁盘 %s 总空间: %s\n", phyDisk, formatBytes(totalBytes))
	fmt.Printf("已用空间: %s\n", formatBytes(totalBytes-usage.Free))
	fmt.Printf("剩余空间: %s\n", formatBytes(usage.Free))
}

func formatBytes(bytes uint64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%dB", bytes)
	case bytes < 1048576:
		return fmt.Sprintf("%.2fKB", float64(bytes)/1024)
	case bytes < 1<<30:
		return fmt.Sprintf("%.2fMB", float64(bytes)/1048576)
	default:
		return fmt.Sprintf("%.2fGB", float64(bytes)/(1<<30))
	}
}
