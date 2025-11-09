#!/bin/bash

# 构建并运行带pprof的Gateway示例

echo "🚀 Building Gateway with PProf integration..."

# 设置环境变量
export PPROF_TOKEN="gateway-debug-2024"

# 构建项目
go mod tidy

echo "📦 Building gateway-pprof example..."
cd cmd/gateway-pprof
go build -o ../../bin/gateway-pprof .

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo ""
    echo "🔧 To run the example:"
    echo "   ./bin/gateway-pprof"
    echo ""
    echo "📊 Then access:"
    echo "   Web UI: http://localhost:8080/"
    echo "   Health: http://localhost:8080/health"
    echo "   PProf:  http://localhost:8080/debug/pprof/?token=$PPROF_TOKEN"
    echo ""
    echo "💡 Authentication token: $PPROF_TOKEN"
else
    echo "❌ Build failed!"
    exit 1
fi