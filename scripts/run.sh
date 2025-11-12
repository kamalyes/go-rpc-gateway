#!/bin/bash
set -e

echo "🚀 启动 {{.ProjectName}} 服务..."

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 检查 go.mod 文件
if [ ! -f go.mod ]; then
    echo "❌ 未找到 go.mod 文件"
    echo "请先运行: go mod init github.com/Divine-Dragon-Voyage/engine-im-push-service"
    exit 1
fi

# 检查依赖
echo "📦 检查并更新依赖..."
go mod tidy
if [ $? -ne 0 ]; then
    echo "❌ 依赖更新失败"
    exit 1
fi

# 检查是否需要生成 protobuf 文件
if [ -d "proto" ] && [ "$(ls -A proto/*.proto 2>/dev/null)" ]; then
    if [ ! "$(ls -A proto/*.pb.go 2>/dev/null)" ]; then
        echo "🔧 检测到 proto 文件，自动生成 gRPC 代码..."
        ./scripts/generate.sh
        if [ $? -ne 0 ]; then
            echo "❌ protobuf 代码生成失败"
            exit 1
        fi
    fi
fi

# 编译检查
echo "🔍 编译检查..."
go build -o /dev/null .
if [ $? -ne 0 ]; then
    echo "❌ 编译失败，请检查代码错误"
    exit 1
fi

# 启动服务
echo "🌟 启动服务中..."
echo "按 Ctrl+C 停止服务"
echo "----------------------------------------"
go run main.go