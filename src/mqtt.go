package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	hamqtt "github.com/LanSilence/hamqtt/pkg/mqtt"
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
	for i = 0; i < 5; i++ {
		client, err = hamqtt.NewMQTTClient(mqttCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "MQTT连接失败: %v\n", err)
			time.Sleep(2 * time.Second)
			continue
		} else {
			fmt.Println("MQTT连接成功")
			break
		}
	}
	if i >= 5 {
		fmt.Println("MQTT连接失败，退出连接")
		return
	}
	client.RegisterSensor(
		hamqtt.SensorEntity{
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
		hamqtt.SensorEntity{
			Name:              "disk_total",
			Description:       "Disk Total",
			DeviceClass:       "data_size",
			UnitOfMeasurement: "GB",
			ValueTemplate:     "value_json.total",
		},
		nil, // no command handler
		nil)
	client.RegisterSensor(
		hamqtt.SensorEntity{
			Name:              "disk_free",
			Description:       "Disk Free",
			DeviceClass:       "data_size",
			UnitOfMeasurement: "GB",
			ValueTemplate:     "value_json.free",
		},
		nil, // no command handler
		nil)
	defer client.Stop()
	fmt.Println("连接成功，开始上报系统信息...")

	// 阻塞主线程
	select {}
}
