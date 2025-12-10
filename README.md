# 天气提醒助手

一个基于Golang开发的智能天气提醒程序，可以自动检查天气并通过Bark App向iPhone发送天气提醒通知。

## 功能特点

- ?? **智能天气检测**: 使用和风天气API获取准确的实时天气数据
- ? **定时检查**: 可配置的时间间隔自动检查天气
- ? **Bark推送**: 通过Bark App发送高优先级通知到iPhone
- ? **重试机制**: 内置网络错误重试机制，提高稳定性
- ?? **灵活配置**: 支持YAML配置文件和环境变量

## 环境要求

- Go 1.21+
- iOS设备安装Bark App

## 安装配置

### 1. 获取API密钥

#### 和风天气API
1. 访问 [和风天气开发者平台](https://dev.qweather.com/)
2. 注册账号并创建应用
3. 获取API Key

#### Bark App
1. 在iPhone上安装[Bark App](https://apps.apple.com/app/bark-custom-notifications/id1593753179)
2. 打开App，点击右上角的"+"按钮
3. 在新页面中，点击"复制设备密钥"
4. 记录设备密钥（格式类似：`abcdef123456789`）

### 2. 配置程序

1. 复制配置文件：
```bash
cp config.yaml.example config.yaml
```

2. 编辑配置文件 `config.yaml`：
```yaml
weather_api:
  api_key: "你的和风天气API密钥"
  city: "你的城市名称"

bark:
  device_key: "你的Bark设备密钥"
```

3. 设置环境变量（可选，更安全）：
```bash
export WEATHER_API_KEY="你的和风天气API密钥"
export BARK_DEVICE_KEY="你的Bark设备密钥"
```

### 3. 运行程序

```bash
# 初始化Go模块
go mod tidy

# 编译并运行
go run main.go

# 或者编译后运行
go build -o weather-reminder main.go
./weather-reminder
```

## 配置说明

### config.yaml 参数说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `weather_api.api_key` | string | - | 和风天气API密钥（必需） |
| `weather_api.city` | string | "北京" | 监控的城市名称 |
| `weather_api.api_host` | string | "devapi.qweather.com" | 和风天气API Host（推荐使用） |
| `bark.device_key` | string | - | Bark设备密钥（必需） |
| `bark.server_url` | string | "https://api.day.app" | Bark服务器地址 |
| `bark.sound` | string | "alarm" | 通知声音 |
| `bark.level` | string | "timeSensitive" | 通知优先级 |
| `bark.category` | string | "weather" | 通知分类 |
| `app.check_interval` | duration | "1h" | 检查间隔时间 |
| `app.max_retries` | int | 3 | 最大重试次数 |

### ?? 和风天气API Host配置

**重要通知**: 和风天气已发布API Host功能，公共API域名将于2026年停止服务：

- `devapi.qweather.com` - 2026年1月1日停止服务
- `api.qweather.com` - 2026年6月1日停止服务  
- `geoapi.qweather.com` - 2026年6月1日停止服务

**建议**: 请尽快迁移到API Host以确保程序长期可用。

**获取API Host步骤**：
1. 登录 [和风天气控制台](https://console.qweather.com/)
2. 进入 **设置** → **API Host**
3. 复制你的API Host（如：`abc.def.qweatherapi.com`）
4. 在 `config.yaml` 中设置：
```yaml
weather_api:
  api_host: "abc.def.qweatherapi.com"  # 使用你的API Host
```

API Host提供更高的安全等级，建议所有用户都进行配置。

### 通知优先级说明

Bark支持以下优先级：
- `passive`: 被动通知
- `active`: 主动通知  
- `timeSensitive`: 时间敏感通知（最高优先级）

建议使用 `timeSensitive` 确保通知能够及时送达。

## 运行模式

程序支持两种运行模式：

### 1. 守护进程模式
程序会按照配置的间隔持续运行，适合长时间监控：
```bash
./weather-reminder
```

### 2. 手动检查模式
程序执行一次检查后退出，适合脚本调用：
```bash
go run main.go --once
```

## 日志输出

程序会输出详细的运行日志，包括：
- 程序启动信息
- 天气检查结果
- 网络请求状态
- 通知发送状态
- 错误信息

## 注意事项

1. **API调用限制**: 和风天气免费版每天限制1000次请求
2. **Bark密钥安全**: 请勿将Bark设备密钥分享给他人
3. **网络连接**: 确保程序运行环境中网络连接正常
4. **时间精度**: 天气检查基于小时级别，适合日常天气监控

## 故障排除

### 天气数据获取失败
- 检查API密钥是否正确
- 确认网络连接正常
- 验证城市名称是否正确

### 通知发送失败
- 确认Bark设备密钥正确
- 检查Bark App是否正常运行
- 验证通知权限设置

### 程序启动失败
- 检查配置文件格式是否正确
- 确认必需的环境变量已设置
- 验证Go版本兼容性

## 扩展功能

程序可以进一步扩展的功能：
- 支持多个城市监控
- 添加温度阈值提醒
- 集成更多通知方式（邮件、短信等）
- 添加历史记录和统计
- 支持自定义天气条件

## 开源协议

MIT License