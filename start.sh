#!/bin/bash

# go-rpc-gateway 快速启动脚本

set -e

APP_NAME="go-rpc-gateway"
VERSION="v1.0.0"

echo "🚀 $APP_NAME $VERSION 快速启动脚本"
echo "=================================="

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ 错误: 未找到Go环境，请先安装Go"
    exit 1
fi

# 检查当前目录
if [ ! -f "go.mod" ]; then
    echo "❌ 错误: 请在项目根目录下运行此脚本"
    exit 1
fi

# 下载依赖
echo "📦 下载依赖..."
go mod tidy

# 设置环境变量
export APP_ENV=${APP_ENV:-development}
echo "🌍 运行环境: $APP_ENV"

# 根据参数选择配置
CONFIG_PATH="./config"
if [ $# -gt 0 ]; then
    case $1 in
        dev|development)
            CONFIG_PATH="./config/gateway-dev.yaml"
            export APP_ENV="development"
            ;;
        prod|production)
            CONFIG_PATH="./config/gateway-prod.yaml"
            export APP_ENV="production"
            ;;
        test|testing)
            CONFIG_PATH="./config/gateway-test.yaml"
            export APP_ENV="test"
            ;;
        *)
            CONFIG_PATH="$1"
            ;;
    esac
fi

echo "📄 配置文件: $CONFIG_PATH"

# 编译并运行
echo "🏗️  编译并启动服务..."
go run cmd/gateway/main.go -config="$CONFIG_PATH"