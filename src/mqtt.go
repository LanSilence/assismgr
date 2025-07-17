package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	hamqtt "github.com/LanSilence/hamqtt/pkg/mqtt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// 默认参数文件路径
const defaultConfigFile = "./HaPerfMonitor_config.json"

type Config struct {
	Server   string `json:"server"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Pass     string `json:"pass"`
	ClientID string `json:"client_id"`
}

// 读取配置文件
func loadConfigFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func getsysLedStatus(ledName string) map[string]string {
	// 获取系统LED状态
	ledStatus := make(map[string]string)

	// 读取保存的状态
	states, err := readLedStates()
	if err != nil {
		return nil
	}
	if states[ledName] == "on" || states[ledName] == "ON" {
		ledStatus["state"] = "ON" // 如果状态不是on，则默认为off
	} else if states[ledName] == "off" {
		ledStatus["state"] = "OFF"
		ledStatus["effect"] = "off" // 如果状态是off，则返回OFF
	} else {
		ledStatus["state"] = "ON"
		ledStatus["effect"] = states[ledName] // 返回当前LED的效果状态
	}
	return ledStatus
}

// 清理 LED 名称使其符合 MQTT 主题要求
func sanitizeLedName(name string) string {
	// 替换所有非法字符为下划线
	replacer := strings.NewReplacer(
		":", "_",
		"/", "_",
		" ", "_",
		"#", "_",
		"+", "_",
		"$", "_",
	)

	// 移除开头结尾的特殊字符
	clean := replacer.Replace(name)
	clean = strings.Trim(clean, "_")

	// 如果清理后为空，使用默认名称
	if clean == "" {
		return "led"
	}

	return clean
}

func resigterLed(ledName string, client *hamqtt.MQTTClient) {

	safeName := sanitizeLedName(ledName)
	lightOption := hamqtt.LightOptions{
		SupportsBrightness:    false, // 暂未支持
		SupportsRGB:           false, // 暂未支持
		SupportsEffects:       true,
		Effect_command_topic:  "homeassistant/light/" + safeName + "/set_effect", //
		Effect_state_topic:    "homeassistant/light/" + safeName + "/effect",
		Effect_value_template: "{{ value_json.effect }}", //
		EffectList: []string{
			LED_MODE_ON,        // 亮
			LED_MODE_OFF,       // 关
			LED_MODE_HEARTBEAT, // 心跳
			LED_MODE_SLOW,      // 慢闪
			LED_MODE_FAST,      // 快闪
		},
	}
	client.RegisterSensor(
		hamqtt.MqttEntity{
			Name:              safeName,
			Description:       "Conctrol Led for" + ledName,
			DeviceClass:       "light",
			UnitOfMeasurement: "",
			ValueTemplate:     "value_json.state",
			ExternalOptions:   &lightOption,
		},
		func(client mqtt.Client, msg mqtt.Message) {
			log.Println("Received command:", string(msg.Payload()))
			log.Println("Received Topic:", msg.Topic())
			var cmd map[string]interface{}
			err := json.Unmarshal(msg.Payload(), &cmd)
			if err != nil {
				log.Println("JSON解析失败: " + err.Error())
				return
			}

			if effect, ok := cmd["effect"].(string); ok {
				setLedMode(ledName, effect)
			} else {
				if cmd["state"] == "OFF" {
					setLedMode(ledName, LED_MODE_OFF)
				} else {
					setLedMode(ledName, LED_MODE_ON)
				}
			}

		}, // no command handler
		func() interface{} {
			return getsysLedStatus(ledName) // return current sensor value
		})
}

func mqttStartLed(client *hamqtt.MQTTClient) {
	if client == nil {
		fmt.Println("MQTT client is nil, cannot register LED sensor")
		return
	}

	// 使用 Go 的 Glob 函数匹配路径
	ledPaths, err := filepath.Glob("/sys/class/leds/*")
	if err != nil {
		fmt.Printf("查找LED设备失败: %v\n", err)
		return
	}

	if len(ledPaths) == 0 {
		fmt.Println("未找到任何LED设备")
		return
	}

	for _, ledPath := range ledPaths {
		if ledPath == "" {
			continue // 跳过空行
		}
		ledName := filepath.Base(ledPath) // 提取LED名称
		resigterLed(ledName, client)
		fmt.Println("LED " + ledName + " registered successfully")
	}

}

func HaPerMonitor(configPath string) {
	// 支持 -c 指定参数文件路径

	cfg, err := loadConfigFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置文件读取失败: %v\n", err)
		return
	}

	mqttCfg := hamqtt.MQTTConfig{
		Server:   cfg.Server,
		Port:     cfg.Port,
		User:     cfg.User,
		Pass:     cfg.Pass,
		ClientID: cfg.ClientID,
	}
	var client *hamqtt.MQTTClient
	var i int
	for i = 0; i < 20; i++ {
		client, err = hamqtt.NewMQTTClient(mqttCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "MQTT连接失败: %v\n", err)
			time.Sleep(10 * time.Second)
			continue
		} else {
			fmt.Println("MQTT连接成功")
			break
		}
	}
	if i >= 20 {
		fmt.Println("MQTT连接失败，退出连接")
		return
	}
	client.RegisterSensor(
		hamqtt.MqttEntity{
			Name:              "disk_usage",
			Description:       "Disk Usage",
			DeviceClass:       "data_size",
			UnitOfMeasurement: "GB",
			ValueTemplate:     "value_json.usage",
		},
		nil, // no command handler
		func() interface{} {
			return getDiskUsage() // return current sensor value
		})
	client.RegisterSensor(
		hamqtt.MqttEntity{
			Name:              "disk_total",
			Description:       "Disk Total",
			DeviceClass:       "data_size",
			UnitOfMeasurement: "GB",
			ValueTemplate:     "value_json.total",
		},
		nil, // no command handler
		nil)
	client.RegisterSensor(
		hamqtt.MqttEntity{
			Name:              "disk_free",
			Description:       "Disk Free",
			DeviceClass:       "data_size",
			UnitOfMeasurement: "GB",
			ValueTemplate:     "value_json.free",
		},
		nil, // no command handler
		nil)
	mqttStartLed(client)
	defer client.Stop()
	fmt.Println("连接成功，开始上报系统信息...")

	// 阻塞主线程
	select {}
}
