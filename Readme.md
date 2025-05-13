# AssistMgr

AssistMgr 是一个基于 Go 语言开发的服务器监控和管理工具，提供了多种功能，包括系统信息监控、网络状态检测、日志管理以及与 Home Assistant 的集成。

## 功能

- **系统信息监控**：实时获取 CPU 使用率、内存使用率和磁盘使用率。
- **网络状态检测**：检测网络连接状态，并显示上传和下载速度。
- **日志管理**：提供服务器日志和系统日志的查看功能。
- **WebSocket 支持**：通过 WebSocket 实时推送系统信息。
- **Home Assistant 集成**：通过 MQTT 协议与 Home Assistant 集成，实现设备自动发现和状态更新。

## 安装

1. 克隆项目代码：
   ```bash
   git clone <repository-url>
   cd AssisMgr
   ```

2. 安装依赖：
   ```bash
   go mod tidy
   ```

3. 编译项目：
   ```bash
   go build -o assistmgr
   ```

4. 运行程序：
   ```bash
   ./assistmgr
   ```

## 配置

- **MQTT 配置**：
  在 `mqtt.go` 文件中配置 MQTT Broker 地址、用户名和密码。

- **设备 ID**：
  设备 ID 存储在 `/data/deviceID` 文件中。如果文件不存在，程序会自动生成一个默认的设备 ID（`0001`）。

## API 路由

- `/ws`：WebSocket 接口，用于实时推送系统信息。
- `/serverlogs`：获取服务器日志。
- `/systemlogs`：获取系统日志。
- `/netstatus`：获取网络状态。
- `/ledstatus`：控制 LED 状态。

## 静态文件

静态文件存储在 `public/` 目录下，包括 HTML、CSS 和 JavaScript 文件。

## 开发

1. 启动开发环境：
   ```bash
   go run main.go
   ```

2. 修改代码后重新编译运行。

## 依赖

- [gorilla/websocket](https://github.com/gorilla/websocket)：用于 WebSocket 通信。
- [gopsutil](https://github.com/shirou/gopsutil)：用于获取系统信息。
- [paho.mqtt.golang](https://github.com/eclipse/paho.mqtt.golang)：用于 MQTT 通信。

## 贡献

欢迎提交 Issue 和 Pull Request 来改进本项目。

## 许可证

本项目使用 MIT 许可证。