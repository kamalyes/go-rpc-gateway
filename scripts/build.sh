#!/bin/bash
set -e

echo "🔨 构建 {{.ProjectName}} 项目..."

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 设置构建变量
APP_NAME="{{.ProjectName}}"
VERSION="1.0.0"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=""

# 尝试获取 Git 提交信息
if command -v git &> /dev/null && [ -d ".git" ]; then
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
fi

# 构建标志
LDFLAGS="-w -s"
LDFLAGS="$LDFLAGS -X main.Version=$VERSION"
LDFLAGS="$LDFLAGS -X main.BuildTime=$BUILD_TIME"
if [ -n "$GIT_COMMIT" ]; then
    LDFLAGS="$LDFLAGS -X main.GitCommit=$GIT_COMMIT"
fi

echo "📦 更新依赖..."
go mod tidy

# 检查是否需要生成 protobuf 文件
if [ -d "proto" ] && [ "$(ls -A proto/*.proto 2>/dev/null)" ]; then
    echo "🔧 生成 protobuf 文件..."
    ./scripts/generate.sh
fi

echo "🏗️  编译项目..."
echo "   应用名称: $APP_NAME"
echo "   版本: $VERSION"
echo "   构建时间: $BUILD_TIME"
echo "   Git 提交: ${GIT_COMMIT:-unknown}"

# 构建不同平台的二进制文件
build_binary() {
    local os=$1
    local arch=$2
    local ext=$3
    local output="build/${APP_NAME}-${os}-${arch}${ext}"
    
    echo "构建 ${os}/${arch}..."
    GOOS=$os GOARCH=$arch go build -ldflags "$LDFLAGS" -o "$output" .
    
    if [ $? -eq 0 ]; then
        echo "✅ 构建成功: $output"
        # 显示文件大小
        if command -v du &> /dev/null; then
            size=$(du -h "$output" | cut -f1)
            echo "   文件大小: $size"
        fi
    else
        echo "❌ 构建失败: ${os}/${arch}"
        return 1
    fi
}

# 创建构建目录
mkdir -p build

# 构建当前平台
echo "🎯 构建当前平台..."
go build -ldflags "$LDFLAGS" -o "build/$APP_NAME" .

# 构建多平台（可选）
if [ "$1" = "--all" ]; then
    echo "🌍 构建多平台版本..."
    build_binary "linux" "amd64" ""
    build_binary "windows" "amd64" ".exe"
    build_binary "darwin" "amd64" ""
    build_binary "darwin" "arm64" ""
fi

echo ""
echo "✅ 构建完成！"
echo "构建文件位于 build/ 目录："
ls -la build/

echo ""
echo "🚀 运行方式："
echo "   ./build/$APP_NAME"