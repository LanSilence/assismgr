package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// 命令处理函数类型
type CommandHandler func(args []string)

// 命令注册表
var commandRegistry = map[string]CommandHandler{}

// 注册命令
func registerCommand(name string, handler CommandHandler) {
	commandRegistry[name] = handler
}

// 监听串口并分发命令
var writer *bufio.Writer

func SerialListenLoop(dev string) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0600)
	if err != nil {
		log.Printf("打开串口失败: %v", err)
		return
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	writer = bufio.NewWriter(f)

	// 初始化行缓冲
	lineBuf := make([]byte, 0, 128)
	for {
		// 逐个字符读取
		char, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				continue
			}
			log.Printf("串口读取错误: %v", err)
			break
		}

		// 处理特殊字符
		switch char {
		case '\r', '\n': // 回车/换行
			if len(lineBuf) > 0 {
				line := string(lineBuf)
				writer.WriteString("\r\n")
				go parseAndDispatch(line)
				lineBuf = lineBuf[:0] // 清空缓冲
			}
		case '\x7F', '\x08': // Backspace/Del
			if len(lineBuf) > 0 {
				lineBuf = lineBuf[:len(lineBuf)-1]
				writer.WriteString("\b \b") // 回退并擦除
			}
		default: // 普通字符
			if char >= 32 && char <= 126 { // 可打印ASCII字符
				lineBuf = append(lineBuf, char)
				writer.WriteByte(char)
			}
		}
		writer.Flush()
	}
}

// 解析命令并分发
func parseAndDispatch(line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}
	cmd := parts[0]
	if handler, ok := commandRegistry[cmd]; ok {
		handler(parts[1:])
	} else {
		log.Printf("未知命令: %s", cmd)
		messageOutput("未知命令: " + cmd)
	}
}

func messageOutput(msg string) {
	if writer != nil {
		writer.WriteString(msg + "\n")
		writer.Flush()
	} else {
		log.Println(msg)
	}
}

func getipaddr() string {
	cmd := exec.Command("hostname", "-I")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("获取IP地址失败: %v", err)

		return ""
	}
	ip := strings.TrimSpace(out.String())
	if ip == "" {
		log.Println("未获取到IP地址")
		return ""
	}
	messageOutput("当前IP地址: " + ip)
	return ip
}

func ipcmd(args []string) {
	getipaddr()
}

// wifi命令处理
func wifiCommand(args []string) {

	if getipaddr() != "" {
		log.Println("当前已连接网络，IP地址：" + getipaddr())
		messageOutput("当前已连接网络，IP地址：" + getipaddr())
		return
	}
	fs := flag.NewFlagSet("wifi", flag.ContinueOnError)
	ssid := fs.String("s", "", "WiFi SSID")
	password := fs.String("p", "", "WiFi password")
	fs.SetOutput(new(bytes.Buffer)) // 防止flag包自动输出到stderr
	if err := fs.Parse(args); err != nil {
		log.Printf("wifi命令参数解析失败: %v", err)
		messageOutput("wifi命令参数解析失败: " + err.Error())
		return
	}
	if *ssid == "" || *password == "" {
		log.Printf("wifi命令参数错误: ssid或password为空, args=%v", args)
		messageOutput("wifi命令参数错误: ssid或password不能为空")
		return
	}
	log.Printf("尝试连接WiFi: ssid=%s password=%s", *ssid, *password)
	messageOutput("尝试连接WiFi: ssid=" + *ssid + " password=" + *password)
	// 使用CommandContext添加超时控制
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nmcli", "device", "wifi", "connect", *ssid, "password", *password)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("nmcli连接失败: %v, 输出: %s", err, out.String())
		messageOutput("nmcli连接失败: " + err.Error() + ", 输出: " + out.String())
		return
	}
	log.Printf("nmcli连接成功: %s", out.String())

	// ifconfig检查IP
	for i := 0; i < 10; i++ {
		cmd = exec.Command("ifconfig")
		out.Reset()
		cmd.Stdout = &out
		err = cmd.Run()
		if err != nil {
			log.Printf("ifconfig失败: %v", err)
			return
		}
		if strings.Contains(out.String(), "inet ") {
			// 获取并打印所有IP地址
			lines := strings.Split(out.String(), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "inet ") {
					fields := strings.Fields(line)
					if len(fields) > 1 {
						ip := fields[1]
						log.Printf("已获取到IP: %s", ip)
						messageOutput("已获取到IP: " + ip)
					}
				}
			}
			break
		}
	}
	// ping测试
	cmd = exec.Command("ping", "-c", "2", "www.baidu.com")
	out.Reset()
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Printf("ping失败: %v, 输出: %s", err, out.String())
		return
	}
	log.Printf("网络连通: %s", out.String())
}

// 初始化注册所有命令
func InitSerialCommands() {
	cmd := exec.Command("modprobe", "g_serial")
	if err := cmd.Run(); err != nil {
		log.Printf("加载g_serial模块失败: %v", err)
		return
	}
	log.Println("g_serial模块加载成功")

	// 注册核心命令
	registerCommand("wifi", wifiCommand)
	registerCommand("ipaddr", ipcmd)
	registerCommand("help", helpCommand)

	// 启动串口监听
	go SerialListenLoop("/dev/ttyGS0")
	log.Println("串口监听已启动")
}

// 帮助命令
func helpCommand(args []string) {
	helpText := `可用命令:
wifi -s SSID -p PASSWORD  连接WiFi
ipaddr                    获取IP地址
help                      显示帮助信息`
	messageOutput(helpText)
}
