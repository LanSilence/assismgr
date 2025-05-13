package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var mqttClient mqtt.Client
var mqttTopic string
var mqttBroker string
var mqttUsername string
var mqttPassword string
var mqttClientID string
var deviceName string = "Hadev"
var deviceID string

func getDeviceID() string {
	var deviceID string
	idFilePath := "/data/deviceID"

	// 检查是否已有生成的deviceID文件
	if data, err := os.ReadFile(idFilePath); err == nil {
		return string(data[:len(data)-1]) // 删除换行符
	}

	// 如果没有，设置默认的deviceID并保存到文件
	deviceID = "0001"

	if err := os.MkdirAll(filepath.Dir(idFilePath), 0755); err != nil {
		panic(err)
	}

	if err := os.WriteFile(idFilePath, []byte(deviceID), 0644); err != nil {
		panic(err)
	}
	return deviceID
}

func initMQTT() {
	mqttBroker = "tcp://43.134.133.119:1883"
	mqttUsername = "admin"
	mqttPassword = "admin"
	mqttClientID = "mqtt_client_id"

	deviceID = getDeviceID()
	mqttTopicMem := "homeassistant/sensor/" + deviceName + deviceID + "mem/config"
	mqttTopicCpu := "homeassistant/sensor/" + deviceName + deviceID + "cpu/config"

	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttBroker)
	opts.SetClientID(mqttClientID)
	opts.SetUsername(mqttUsername)
	opts.SetPassword(mqttPassword)

	mqttClient = mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	payloadMem := `
       		{
  				"name": "Memory Usage",
				"device_class":"humidity",
  				"state_topic": "homeassistant/sensor/` + deviceName + deviceID + `/state",
  				"unit_of_measurement": "%",
				"unique_id":"HAdev001",
  				"value_template": "{{ value_json.mem_usage }}",
  				"device": {
  				  "identifiers":["` + deviceName + deviceID + `"],
  				  "name": "Hadev",
  				  "manufacturer": "Custom Device"
  				}
			}`

	payloadCpu := `
       		{
				"name": "CPU Usage",
   				"device_class":"power",
   				"state_topic":"homeassistant/sensor/` + deviceName + deviceID + `/state",
   				"unit_of_measurement":"%",
   				"value_template":"{{ value_json.cpu_usage }}",
   				"unique_id":"HAdev002",
   				"device":{
   				   "identifiers":["` + deviceName + deviceID + `"],
   				   "name":"Hadev",
				   "manufacturer": "Custom Device"
   				}
			}`

	token := mqttClient.Publish(mqttTopicMem, 1, false, payloadMem)
	fmt.Println(payloadCpu)
	fmt.Println(payloadMem)

	token.Wait()
	token = mqttClient.Publish(mqttTopicCpu, 1, false, payloadCpu)
	token.Wait()
	go publishServerStatus()
}

func publishServerStatus() {

	for {
		info := getSystemInfo()
		stateTopic := "homeassistant/sensor/" + deviceName + deviceID + "/state"

		// 将状态数据编码为JSON
		payload, err := json.Marshal(info)
		if err != nil {
			panic(err)
		}

		// 发布状态数据到Home Assistant
		token := mqttClient.Publish(stateTopic, 0, false, payload)
		token.Wait()
		time.Sleep(5 * time.Second) // 每5秒发布一次状态
	}
}
