package main

import (
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

func getOSVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(data), "\n")
	var prettyName, versionId string
	for _, line := range lines {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			prettyName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			versionId = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
		}
	}

	if prettyName != "" && versionId != "" {
		return prettyName + " (" + versionId + ")"
	}
	return "unknown"
}

type LinuxSystemInfo struct {
	Version   string
	BuildTime string
	Arch      string
}

func getLinuxSystemInfo() LinuxSystemInfo {
	cmd := exec.Command("uname", "-a")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return LinuxSystemInfo{"unknown", "unknown", "unknown"}
	}

	info := LinuxSystemInfo{}
	parts := strings.Split(strings.TrimSpace(string(out)), " ")
	if len(parts) >= 8 {
		// 处理两种格式:
		// 1. Linux hostname 6.12.0-haos #26 SMP Thu Jun 26 22:04:55 CST 2025 aarch64...
		// 2. Linux hostname 6.12.0-haos #2 SMP PREEMPT Thu Jul 3 14:13:02 UTC 2025 aarch64...
		info.Version = parts[2] // 内核版本

		// 使用正则表达式提取日期时间(支持单/双数字日期)
		re := regexp.MustCompile(`([A-Z][a-z]{2} [A-Z][a-z]{2} \s*\d{1,2} \d{2}:\d{2}:\d{2} [A-Z]{3,4} \d{4})`)
		matches := re.FindStringSubmatch(string(out))
		if len(matches) > 0 {
			info.BuildTime = matches[0]
		}

		// 提取架构(从后往前找第一个aarch64/x86_64/armv7等)
		for i := len(parts) - 1; i >= 0; i-- {
			if strings.HasPrefix(parts[i], "aarch64") ||
				strings.HasPrefix(parts[i], "x86_64") ||
				strings.HasPrefix(parts[i], "armv7") {
				info.Arch = parts[i]
				break
			}
		}
	}
	return info
}

func getCPUModel() string {
	// 尝试从/proc/cpuinfo获取
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, "model name") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// 尝试从lscpu获取
	if cmd := exec.Command("lscpu"); cmd != nil {
		out, err := cmd.CombinedOutput()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			models := make(map[string]bool)
			for _, line := range lines {
				if strings.Contains(line, "Model name:") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						models[strings.TrimSpace(parts[1])] = true
					}
				}
			}
			if len(models) > 0 {
				var uniqueModels []string
				for model := range models {
					uniqueModels = append(uniqueModels, model)
				}
				return strings.Join(uniqueModels, " + ")
			}
		}
	}

	// 回退到设备树检查
	if _, err := os.Stat("/sys/firmware/devicetree/base/compatible"); err == nil {
		if data, err := os.ReadFile("/sys/firmware/devicetree/base/compatible"); err == nil {
			return string(data)
		}
	}

	return "unknown"
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	sysInfo := getLinuxSystemInfo()
	cpuInfo := getCPUModel() + " (" + strconv.Itoa(runtime.NumCPU()) + " Cores)"

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{
		"version": "` + getOSVersion() + `",
		"linux_version": "` + sysInfo.Version + `",
		"build_time": "` + sysInfo.BuildTime + `",
		"arch": "` + sysInfo.Arch + `",
		"cpu_info": "` + cpuInfo + `"
	}`))
}
