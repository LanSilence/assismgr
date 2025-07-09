package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/disk"
)

func getDiskUsage() map[string]float64 {
	mountPointAll := []string{"/mnt/data", "/"} // 替换为你的挂载点
	var mountPoint string
	diskUage := make(map[string]float64)
	// 获取磁盘使用情况
	var usage *disk.UsageStat

	partitions, err := disk.Partitions(false)
	if err != nil {
		fmt.Printf("获取分区信息失败: %v\n", err)
		return nil
	}

	mountPointMap := make(map[string]bool)
	for _, p := range partitions {
		mountPointMap[p.Mountpoint] = true
	}

	for _, candidate := range mountPointAll {
		// 检查候选路径是否是挂载点
		if _, exists := mountPointMap[candidate]; exists {
			usage, err = disk.Usage(candidate)
			if err == nil {
				mountPoint = candidate
				break
			}
		}
	}

	// 回退机制：如果未找到，尝试直接获取根目录
	if mountPoint == "" {
		usage, err = disk.Usage("/")
		if err == nil {
			mountPoint = "/"
		}
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
		return nil
	}

	// 使用 lsblk -no pkname 获取物理磁盘名
	cmd := exec.Command("lsblk", "-no", "pkname", device)
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("lsblk 命令执行失败: %v\n", err)
		return nil
	}
	phyDisk := "/dev/" + string(output)
	phyDisk = phyDisk[:len(phyDisk)-1] // 去掉换行

	diskName := strings.TrimPrefix(phyDisk, "/dev/")
	sizePath := "/sys/block/" + diskName + "/size"
	data, err := os.ReadFile(sizePath)
	if err != nil {
		fmt.Printf("读取物理磁盘大小失败: %v\n", err)
		return nil
	}
	sectorsStr := strings.TrimSpace(string(data))
	sectors, err := strconv.ParseUint(sectorsStr, 10, 64)
	if err != nil {
		fmt.Printf("解析磁盘扇区数失败: %v\n", err)
		return nil
	}
	totalBytes := sectors * 512

	diskUage["total"] = float64(totalBytes) / 1073741824
	diskUage["usage"] = float64(totalBytes-usage.Free) / 1073741824
	diskUage["free"] = float64(usage.Free) / 1073741824
	return diskUage
}
