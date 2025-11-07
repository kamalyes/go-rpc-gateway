#!/bin/bash

echo "========================================"
echo "    Go RPC Gateway 启动脚本"
echo "========================================"

# 检查是否存在配置目录
mkdir -p logs
mkdir -p config

# 检查配置文件
if [ ! -f "config/example.yaml" ]; then
    echo "警告: 配置文件 config/example.yaml 不存在"
    echo "将使用默认配置启动..."
    echo
fi

# 编译项目
echo "正在编译 Gateway..."
go build -o bin/gateway ./cmd/gateway
if [ $? -ne 0 ]; then
    echo "编译失败!"
    exit 1
fi

echo "编译完成!"
echo

# 启动网关
echo "启动 Go RPC Gateway..."
echo "使用配置文件: config/example.yaml"
echo "日志目录: logs/"
echo

./bin/gateway -config config/example.yaml -log-level debug -log-dir logs