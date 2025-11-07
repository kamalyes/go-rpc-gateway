#!/bin/bash

# Gateway日志测试脚本

echo "🏗️  构建Gateway主程序..."

# 构建主程序
cd cmd/gateway
go build -o ../../bin/gateway .
cd ../..

# 创建日志目录
mkdir -p logs

echo "🚀 启动Gateway (日志将保存到 logs/ 目录)"
echo "按 Ctrl+C 退出"

# 运行Gateway
./bin/gateway -log-dir=logs -log-level=info

echo "✅ Gateway已停止"
echo ""
echo "📁 查看日志文件:"
echo "   tail -f logs/gateway.log"
echo "   或者直接查看 logs/ 目录下的日志文件"