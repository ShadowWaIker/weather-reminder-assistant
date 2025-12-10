#!/bin/bash

# 天气提醒助手 - 快速启动脚本

echo "??  天气提醒助手 - 快速启动"
echo "================================"

# 自动检测Go安装路径
find_go_installation() {
    local go_paths=(
        "/Users/f/go/go1.20.3"
        "/usr/local/go"
        "/opt/go"
        "$HOME/go"
        "/usr/bin/go"
        "/usr/bin/local/go"
    )
    
    for go_path in "${go_paths[@]}"; do
        if [ -d "$go_path" ] && [ -x "$go_path/bin/go" ]; then
            echo "$go_path"
            return 0
        fi
    done
    return 1
}

# 如果go命令不存在，尝试自动检测和设置
if [ -z "$(command -v go)" ]; then
    echo "🔍 正在检测Go安装路径..."
    
    # 先尝试用户指定的路径
    if [ -d "/Users/f/go/go1.20.3" ] && [ -x "/Users/f/go/go1.20.3/bin/go" ]; then
        go_path="/Users/f/go/go1.20.3"
    elif go_path=$(find_go_installation); then
        # 使用函数检测其他路径
        :
    else
        echo "❌ 未找到Go安装，请:"
        echo "   1. 确保Go已正确安装"
        echo "   2. 或者手动指定Go路径"
        echo ""
        echo "🔧 常见安装路径:"
        echo "   - /Users/f/go/go1.20.3"
        echo "   - /usr/local/go"
        echo "   - $HOME/go"
        echo ""
        exit 1
    fi
    
    if [ -n "$go_path" ]; then
        echo "✅ 找到Go安装: $go_path"
        export GOROOT="$go_path"
        export PATH="$GOROOT/bin:$PATH"
        echo "🌍 GOROOT设置: $GOROOT"
    fi
fi

echo "? Go版本: $(go version)"
echo ""

# 检查依赖
echo ""
echo "📦 检查依赖包..."
export GOROOT="/Users/f/go/go1.20.3"
export PATH="$GOROOT/bin:$PATH"
export GOPROXY="https://goproxy.cn,direct"
go mod tidy

echo ""
echo "??  配置检查:"
if [ ! -f "config.yaml" ]; then
    echo "? config.yaml 文件不存在，请从 config.yaml.example 复制"
    echo "   cp config.yaml.example config.yaml"
    exit 1
fi

# 检查API密钥配置
if grep -q "YOUR_WEATHER_API_KEY" config.yaml; then
    echo "??  请先在 config.yaml 中配置和风天气API密钥"
fi

if grep -q "YOUR_BARK_DEVICE_KEY" config.yaml; then
    echo "??  请先在 config.yaml 中配置Bark设备密钥"
fi

echo ""
echo "🚀 编译程序..."
export GOPROXY="https://goproxy.cn,direct"
go build -o weather-reminder main.go

if [ $? -eq 0 ]; then
    echo "? 编译成功！"
    echo ""
    echo "? 使用说明:"
    echo "   ./weather-reminder          # 持续运行，间隔检查天气"
    echo "   ./weather-reminder --once   # 执行一次检查后退出"
    echo ""
    echo "??  使用前请确保:"
    echo "   1. 已在 config.yaml 中配置正确的API密钥"
    echo "   2. iPhone已安装Bark App并获取设备密钥"
    echo ""
else
    echo "? 编译失败，请检查代码"
fi