package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// 配置结构体
type Config struct {
	WeatherAPI struct {
		APIKey  string `mapstructure:"api_key"`
		City    string `mapstructure:"city"`
		APIHost string `mapstructure:"api_host"`
	} `mapstructure:"weather_api"`
	Bark struct {
		DeviceKey string `mapstructure:"device_key"`
		ServerURL string `mapstructure:"server_url"`
		Sound     string `mapstructure:"sound"`
		Level     string `mapstructure:"level"`
		Category  string `mapstructure:"category"`
	} `mapstructure:"bark"`
	App struct {
		CheckInterval time.Duration `mapstructure:"check_interval"`
		MaxRetries    int           `mapstructure:"max_retries"`
		Verbose       bool          `mapstructure:"verbose"`
	} `mapstructure:"app"`
}

// 天气API响应结构
type WeatherResponse struct {
	Code       string `json:"code"`
	UpdateTime string `json:"updateTime"`
	FXLink     string `json:"fxLink"`
	Now        struct {
		ObsTime   string `json:"obsTime"`
		Temp      string `json:"temp"`
		FeelsLike string `json:"feelsLike"`
		Icon      string `json:"icon"`
		Text      string `json:"text"`
		Wind360   string `json:"wind360"`
		WindDir   string `json:"windDir"`
		WindScale string `json:"windScale"`
		WindSpeed string `json:"windSpeed"`
		Humidity  string `json:"humidity"`
		Precip    string `json:"precip"`
		Pressure  string `json:"pressure"`
		Cloud     string `json:"cloud"`
		Dew       string `json:"dew"`
	} `json:"now"`
	Hourly []struct {
		FxTime    string `json:"fxTime"`
		Temp      string `json:"temp"`
		Text      string `json:"text"`
		Precip    string `json:"precip"`
		Wind360   string `json:"wind360"`
		WindDir   string `json:"windDir"`
		WindScale string `json:"windScale"`
		WindSpeed string `json:"windSpeed"`
	} `json:"hourly"`
	HasCurrentPrecipitation bool `json:"-"` // 当前天气是否有降水（内部字段，不参与JSON序列化）
}

// 降水预报详情结构
type PrecipitationForecast struct {
	WillPrecipitate bool   // 是否会有降水
	StartTime       string // 开始时间
	EndTime         string // 结束时间
	WeatherType     string // 天气类型（雨/雪等）
	Intensity       string // 强度描述
	PrecipAmount    string // 降水量
}

// 城市查询响应结构
type CitySearchResponse struct {
	Code     string `json:"code"`
	Info     string `json:"info"`
	Count    int    `json:"count"`
	Location []struct {
		Id       string `json:"id"`
		Name     string `json:"name"`
		Country  string `json:"country"`
		Adm1     string `json:"adm1"`
		Adm2     string `json:"adm2"`
		Lat      string `json:"lat"`
		Lon      string `json:"lon"`
		Timezone string `json:"timezone"`
		Type     string `json:"type"`
		Rank     string `json:"rank"`
		FxLink   string `json:"fxLink"`
	} `json:"location"`
}

// Bark推送请求结构
type BarkRequest struct {
	DeviceKey string `json:"device_key"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Category  string `json:"category"`
	Sound     string `json:"sound"`
	Level     string `json:"level"`
	Url       string `json:"url"`
}

// 全局配置
var config Config

// 测试模式标志（用于模拟降水天气）
var testMode = false

func main() {
	// 解析命令行参数
	once := flag.Bool("once", false, "执行一次检查后退出")
	test := flag.Bool("test", false, "测试模式，模拟有降水天气")
	flag.Parse()

	// 设置测试模式
	if *test {
		testMode = true
		log.Printf("启用测试模式，将模拟降水天气")
	}

	// 初始化配置
	if err := initConfig(); err != nil {
		log.Fatal("配置初始化失败:", err)
	}

	log.Printf("天气提醒程序启动")
	log.Printf("监控城市: %s", config.WeatherAPI.City)

	if *once {
		log.Printf("执行单次天气检查")
		checkWeatherAndNotify()
		return
	}

	log.Printf("检查间隔: %v", config.App.CheckInterval)

	// 启动定时检查
	ticker := time.NewTicker(config.App.CheckInterval)
	defer ticker.Stop()

	// 立即执行一次检查
	checkWeatherAndNotify()

	for range ticker.C {
		checkWeatherAndNotify()
	}
}

// 初始化配置
func initConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// 设置默认值
	viper.SetDefault("app.check_interval", time.Hour)
	viper.SetDefault("app.max_retries", 3)
	viper.SetDefault("app.verbose", true)
	viper.SetDefault("weather_api.api_host", "devapi.qweather.com")
	viper.SetDefault("bark.server_url", "https://api.day.app")
	viper.SetDefault("bark.sound", "alarm")
	viper.SetDefault("bark.level", "timeSensitive")
	viper.SetDefault("bark.category", "weather")

	// 从环境变量读取敏感信息
	viper.BindEnv("weather_api.api_key", "WEATHER_API_KEY")
	viper.BindEnv("bark.device_key", "BARK_DEVICE_KEY")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return fmt.Errorf("配置文件不存在: %v", err)
		}
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("解析配置失败: %v", err)
	}

	// 验证必需的配置
	if config.WeatherAPI.APIKey == "" {
		return fmt.Errorf("天气API密钥未设置 WEATHER_API_KEY，请设置环境变量")
	}
	if config.Bark.DeviceKey == "" {
		return fmt.Errorf("Bark设备密钥未设置，请设置环境变量 BARK_DEVICE_KEY")
	}
	if config.WeatherAPI.City == "" {
		config.WeatherAPI.City = "北京" // 默认城市
	}
	if config.WeatherAPI.APIHost == "" {
		config.WeatherAPI.APIHost = "devapi.qweather.com" // 默认API Host
	}

	return nil
}

// 检查天气
func checkWeatherAndNotify() {
	log.Println("正在检查天气...")

	weather, err := fetchWeather(config.WeatherAPI.City)
	if err != nil {
		log.Printf("获取天气数据失败: %v", err)
		return
	}

	// 获取详细降水预报信息
	forecast := getPrecipitationForecast(weather)
	if forecast.WillPrecipitate {
		log.Printf("检测到降水预报: %s，%s 到 %s，%s，%s",
			forecast.WeatherType, forecast.StartTime, forecast.EndTime,
			forecast.Intensity, forecast.PrecipAmount)
		if err := sendNotification(weather, forecast); err != nil {
			log.Printf("发送通知失败: %v", err)
		} else {
			log.Println("通知发送成功")
		}
	} else {
		log.Println("未来3小时内无降水")
	}
}

// 城市ID映射表（常用城市）
var cityIdMap = map[string]string{
	"北京":   "101010100",
	"上海":   "101020100",
	"广州":   "101280101",
	"深圳":   "101280601",
	"杭州":   "101210101",
	"南京":   "101190101",
	"武汉":   "101200101",
	"成都":   "101270101",
	"重庆":   "101040100",
	"西安":   "101110101",
	"天津":   "101030100",
	"苏州":   "101190401",
	"长沙":   "101250101",
	"郑州":   "101180101",
	"济南":   "101120101",
	"长春":   "101060101",
	"哈尔滨":  "101050101",
	"沈阳":   "101070101",
	"大连":   "101070201",
	"青岛":   "101120201",
	"昆明":   "101290101",
	"南宁":   "101280101",
	"贵阳":   "101260101",
	"太原":   "101100101",
	"合肥":   "101220101",
	"南昌":   "101240101",
	"福州":   "101230101",
	"厦门":   "101230201",
	"石家庄":  "101090101",
	"呼和浩特": "101080101",
	"银川":   "101170101",
	"西宁":   "101150101",
	"拉萨":   "101140101",
	"乌鲁木齐": "101130101",
	"兰州":   "101160101",
	"海口":   "101310101",
	"三亚":   "101310201",
	"台北":   "101340101",
	"香港":   "101320101",
	"澳门":   "101330101",
}

// 获取城市ID（先查本地映射，再查API）
func getCityID(cityName string) (string, error) {
	// 首先尝试从预定义映射中查找
	if cityId, exists := cityIdMap[cityName]; exists {
		log.Printf("从本地映射表中找到城市ID: %s -> %s", cityName, cityId)
		return cityId, nil
	}

	// 如果本地映射表中没有，尝试从API获取
	log.Printf("本地映射表中未找到城市 %s，尝试从API查询", cityName)

	// 根据官方文档，使用请求参数形式的API Key认证
	// 格式：https://api_host/geo/v2/city/lookup?location=城市名&key=API_KEY
	url := fmt.Sprintf("https://%s/geo/v2/city/lookup?location=%s&key=%s",
		config.WeatherAPI.APIHost, url.QueryEscape(cityName), config.WeatherAPI.APIKey)

	client := &http.Client{Timeout: 10 * time.Second}

	// 创建HTTP请求并添加gzip支持
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 添加Accept-Encoding头支持gzip压缩
	req.Header.Add("Accept-Encoding", "gzip")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("查询城市ID失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("城市查询API返回错误状态码: %d", resp.StatusCode)
	}

	// 处理gzip压缩的响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 如果响应被gzip压缩，则进行解压缩
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("创建gzip读取器失败: %v", err)
		}
		defer gzipReader.Close()

		body, err = io.ReadAll(gzipReader)
		if err != nil {
			return "", fmt.Errorf("gzip解压缩失败: %v", err)
		}
	}

	var cityResponse CitySearchResponse
	if err := json.Unmarshal(body, &cityResponse); err != nil {
		return "", fmt.Errorf("解析城市响应失败: %v", err)
	}

	if cityResponse.Code != "200" {
		return "", fmt.Errorf("城市查询失败: %s", cityResponse.Info)
	}

	if len(cityResponse.Location) == 0 {
		return "", fmt.Errorf("未找到城市: %s", cityName)
	}

	return cityResponse.Location[0].Id, nil
}

// 获取天气数据
// 当前天气响应结构
type CurrentWeatherResponse struct {
	Code       string `json:"code"`
	UpdateTime string `json:"updateTime"`
	FXLink     string `json:"fxLink"`
	Now        struct {
		ObsTime   string `json:"obsTime"`
		Temp      string `json:"temp"`
		FeelsLike string `json:"feelsLike"`
		Icon      string `json:"icon"`
		Text      string `json:"text"`
		Wind360   string `json:"wind360"`
		WindDir   string `json:"windDir"`
		WindScale string `json:"windScale"`
		WindSpeed string `json:"windSpeed"`
		Humidity  string `json:"humidity"`
		Precip    string `json:"precip"`
		Pressure  string `json:"pressure"`
		Cloud     string `json:"cloud"`
		Dew       string `json:"dew"`
	} `json:"now"`
}

func fetchWeather(cityName string) (*WeatherResponse, error) {
	// 首先获取城市ID
	cityId, err := getCityID(config.WeatherAPI.City)
	if err != nil {
		return nil, fmt.Errorf("获取城市ID失败: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// 1. 获取当前天气
	currentWeatherURL := fmt.Sprintf("https://%s/v7/weather/now?location=%s&key=%s",
		config.WeatherAPI.APIHost, cityId, config.WeatherAPI.APIKey)

	var currentWeather CurrentWeatherResponse
	if err := fetchWeatherData(client, currentWeatherURL, &currentWeather); err != nil {
		return nil, fmt.Errorf("获取当前天气失败: %v", err)
	}

	// 2. 检查当前天气是否有降水
	// hasCurrentPrecipitation := checkWeatherPrecipitation(currentWeather.Now.Text, currentWeather.Now.Precip)
	// log.Printf("检查当前天气: %s, 温度: %s°C, 降水量: %s, 有降水: %v",
	// 	currentWeather.Now.Text, currentWeather.Now.Temp, currentWeather.Now.Precip, hasCurrentPrecipitation)

	// if hasCurrentPrecipitation {
	// 	log.Printf("当前天气已有降水: %s，跳过获取逐小时预报", currentWeather.Now.Text)

	// 	// 直接返回当前天气信息，无需获取逐小时预报
	// 	weather := &WeatherResponse{
	// 		Code:                    currentWeather.Code,
	// 		Now:                     currentWeather.Now,
	// 		HasCurrentPrecipitation: true,
	// 	}
	// 	return weather, nil
	// }

	// 3. 如果当前没有降水，获取24小时逐小时预报进行未来预测
	hourlyURL := fmt.Sprintf("https://%s/v7/weather/24h?location=%s&key=%s",
		config.WeatherAPI.APIHost, cityId, config.WeatherAPI.APIKey)

	var hourlyWeather WeatherResponse
	if err := fetchWeatherData(client, hourlyURL, &hourlyWeather); err != nil {
		return nil, fmt.Errorf("获取24小时天气数据失败: %v", err)
	}

	// 4. 合并数据：将当前天气信息合并到24小时预报响应中
	weather := &hourlyWeather
	weather.Now = currentWeather.Now
	weather.HasCurrentPrecipitation = false // 当前无降水，需要检查未来

	return weather, nil
}

// 统一的气象现象检查函数
func checkWeatherPrecipitation(text, precip string) bool {
	rainWeather := []string{"雨", "雪", "阵雨", "雷阵雨", "毛毛雨", "小雪", "中雪", "大雪", "暴雪", "雨夹雪"}

	for _, weatherType := range rainWeather {
		if strings.Contains(text, weatherType) {
			return true
		}
	}

	// 检查降水量
	if precip != "0.0" && precip != "0" {
		return true
	}

	return false
}

// 通用API数据获取函数
func fetchWeatherData(client *http.Client, url string, result interface{}) error {
	for i := 0; i < config.App.MaxRetries; i++ {
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("第%d次请求失败: %v", i+1, err)
			if i == config.App.MaxRetries-1 {
				return fmt.Errorf("获取天气数据失败: %v", err)
			}
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("天气API返回错误状态码: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("读取响应失败: %v", err)
		}

		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("解析天气数据失败: %v", err)
		}

		// 检查API状态码
		switch v := result.(type) {
		case *WeatherResponse:
			if v.Code != "200" {
				return fmt.Errorf("天气API返回错误代码: %s", v.Code)
			}
		case *CurrentWeatherResponse:
			if v.Code != "200" {
				return fmt.Errorf("天气API返回错误代码: %s", v.Code)
			}
		}

		return nil
	}

	return fmt.Errorf("达到最大重试次数")
}

// 获取详细的降水预报信息
func getPrecipitationForecast(weather *WeatherResponse) PrecipitationForecast {
	// 测试模式下返回模拟数据
	if testMode {
		return PrecipitationForecast{
			WillPrecipitate: true,
			StartTime:       "15:30",
			EndTime:         "18:30",
			WeatherType:     "小雨",
			Intensity:       "轻到中雨",
			PrecipAmount:    "5-15mm",
		}
	}

	// 如果当前天气已经有降水
	if weather.HasCurrentPrecipitation {
		return PrecipitationForecast{
			WillPrecipitate: true,
			StartTime:       "当前",
			EndTime:         "正在降水",
			WeatherType:     weather.Now.Text,
			Intensity:       "当前降水",
			PrecipAmount:    weather.Now.Precip + "mm",
		}
	}

	// 当前无降水，检查逐小时预报
	now := time.Now()
	var precipitationPeriods []struct {
		startTime string
		endTime   string
		weather   string
		precip    string
	}

	for _, hour := range weather.Hourly {
		hourTime, err := time.Parse("2006-01-02T15:04-07:00", hour.FxTime)
		if err != nil {
			log.Printf("解析时间失败: %v", err)
			continue
		}

		timeDiff := hourTime.Sub(now)
		log.Printf("检查时间: %s, 相差: %v", hour.FxTime, timeDiff)

		// 如果在未来3小时内
		if timeDiff > 0 && timeDiff <= 3*time.Hour {
			log.Printf("进入检查区间: %s, 天气: %s, 降水量: %s", hour.FxTime, hour.Text, hour.Precip)

			// 使用统一检查函数
			if checkWeatherPrecipitation(hour.Text, hour.Precip) {
				log.Printf("检测到降水: %s, 降水量: %smm", hour.Text, hour.Precip)
				precipitationPeriods = append(precipitationPeriods, struct {
					startTime string
					endTime   string
					weather   string
					precip    string
				}{
					startTime: hourTime.Format("15:04"),
					endTime:   hourTime.Format("15:04"),
					weather:   hour.Text,
					precip:    hour.Precip,
				})
			}
		}
	}

	// 如果没有找到降水预报
	if len(precipitationPeriods) == 0 {
		return PrecipitationForecast{
			WillPrecipitate: false,
		}
	}

	// 计算降水时间范围和类型
	startTime := precipitationPeriods[0].startTime
	endTime := precipitationPeriods[len(precipitationPeriods)-1].endTime

	// 确定主要的天气类型和强度
	weatherTypes := make(map[string]int)
	var totalPrecip float64
	for _, period := range precipitationPeriods {
		weatherTypes[period.weather]++
		if precip, err := strconv.ParseFloat(period.precip, 64); err == nil {
			totalPrecip += precip
		}
	}

	// 获取最常见的天气类型
	mainWeather := ""
	maxCount := 0
	for weather, count := range weatherTypes {
		if count > maxCount {
			maxCount = count
			mainWeather = weather
		}
	}

	// 计算平均降水量
	avgPrecip := ""
	if totalPrecip > 0 {
		avgPrecip = fmt.Sprintf("%.1f", totalPrecip/float64(len(precipitationPeriods)))
	}

	// 确定降水强度
	intensity := ""
	if precipFloat, err := strconv.ParseFloat(avgPrecip, 64); err == nil {
		if precipFloat < 2.5 {
			intensity = "小雨"
		} else if precipFloat < 10 {
			intensity = "中雨"
		} else if precipFloat < 25 {
			intensity = "大雨"
		} else {
			intensity = "暴雨"
		}
	}

	return PrecipitationForecast{
		WillPrecipitate: true,
		StartTime:       startTime,
		EndTime:         endTime,
		WeatherType:     mainWeather,
		Intensity:       intensity,
		PrecipAmount:    avgPrecip + "mm",
	}
}

// 发送通知
func sendNotification(weather *WeatherResponse, forecast PrecipitationForecast) error {
	// 获取当前天气信息
	currentWeather := weather.Now.Text
	currentTemp := weather.Now.Temp

	// 构造详细的通知内容
	title := "☔️ 降水提示"

	var body string
	if forecast.StartTime == "当前" {
		body = fmt.Sprintf("%s %s，温度：%s°C，%s（%s）。请注意携带雨具！",
			config.WeatherAPI.City, currentWeather, currentTemp,
			forecast.Intensity, forecast.PrecipAmount)
	} else {
		body = fmt.Sprintf("%s 预计%s开始%s，%s（%s）。当前天气：%s，温度：%s°C。请提前准备雨具！",
			config.WeatherAPI.City, forecast.StartTime, forecast.WeatherType,
			forecast.Intensity, forecast.PrecipAmount, currentWeather, currentTemp)
	}

	// 构造Bark请求
	request := BarkRequest{
		DeviceKey: config.Bark.DeviceKey,
		Title:     title,
		Body:      body,
		Category:  config.Bark.Category,
		Sound:     config.Bark.Sound,
		Level:     config.Bark.Level,
		Url:       "https://www.qweather.com/",
	}

	if config.App.Verbose {
		log.Printf("准备发送Bark通知:")
		log.Printf("  标题: %s", request.Title)
		log.Printf("  内容: %s", request.Body)
		log.Printf("  分类: %s", request.Category)
		log.Printf("  声音: %s", request.Sound)
		log.Printf("  优先级: %s", request.Level)
	}

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("序列化请求数据失败: %v", err)
	}

	url := fmt.Sprintf("%s/%s", config.Bark.ServerURL, config.Bark.DeviceKey)

	if config.App.Verbose {
		log.Printf("发送请求到: %s", url)
	}

	resp, err := client.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("发送通知请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Bark服务器返回错误: %d, %s", resp.StatusCode, string(body))
	}

	if config.App.Verbose {
		log.Printf("Bark通知发送成功")
	}

	return nil
}
