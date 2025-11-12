#!/bin/bash
set -e

echo "🧪 运行 {{.ProjectName}} 测试..."

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 检查是否需要生成 protobuf 文件
if [ -d "proto" ] && [ "$(ls -A proto/*.proto 2>/dev/null)" ]; then
    if [ ! "$(ls -A proto/*.pb.go 2>/dev/null)" ]; then
        echo "🔧 生成 protobuf 文件..."
        ./scripts/generate.sh
    fi
fi

echo "📦 更新依赖..."
go mod tidy

echo "🔍 运行 go vet 检查..."
go vet ./...

echo "🧪 运行单元测试..."
if [ "$1" = "--coverage" ]; then
    echo "📊 包含覆盖率统计..."
    go test -v -race -coverprofile=coverage.out ./...
    
    if command -v go &> /dev/null; then
        echo "📋 生成覆盖率报告..."
        go tool cover -html=coverage.out -o coverage.html
        echo "✅ 覆盖率报告已生成: coverage.html"
        
        echo "📊 覆盖率统计："
        go tool cover -func=coverage.out | tail -1
    fi
elif [ "$1" = "--bench" ]; then
    echo "⚡ 运行性能测试..."
    go test -v -bench=. -benchmem ./...
else
    go test -v -race ./...
fi

echo ""
echo "✅ 测试完成！"

# 提供一些有用的测试命令提示
echo ""
echo "💡 其他测试命令："
echo "   ./scripts/test.sh --coverage    # 生成覆盖率报告"
echo "   ./scripts/test.sh --bench       # 运行性能测试"
echo "   go test -short ./...            # 跳过长时间运行的测试"
echo "   go test -run TestSpecific       # 运行特定测试"