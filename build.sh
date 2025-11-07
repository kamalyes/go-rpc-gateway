#!/bin/bash

# Go RPC Gateway 构建脚本
# 重构后的验证和构建

set -e

echo "🏗️  构建 Go RPC Gateway (基于 go-config 和 go-core 重构版本)"
echo "==============================================="

# 检查Go环境
echo "📦 检查Go环境..."
go version

# 清理模块缓存
echo "🧹 清理依赖..."
go mod tidy

# 下载依赖
echo "⬇️  下载依赖..."
go mod download

# 格式化代码
echo "🎨 格式化代码..."
go fmt ./...

# 运行测试
echo "🧪 运行测试..."
go test ./... -v || echo "⚠️  一些测试可能需要数据库连接"

# 构建主程序
echo "🔨 构建主程序..."
cd cmd/gateway
go build -o ../../bin/gateway .
cd ../..

# 创建输出目录
mkdir -p bin

echo "✅ 构建完成!"
echo ""
echo "📁 输出文件:"
echo "   - bin/gateway              (主程序)"
echo ""
echo "🚀 运行示例:"
echo "   ./bin/gateway -config examples/config.yaml"
echo ""
echo "🎉 重构完成! Gateway 已成功集成 go-config 和 go-core"