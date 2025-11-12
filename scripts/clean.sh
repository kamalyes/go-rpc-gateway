#!/bin/bash
set -e

echo "🧹 清理 {{.ProjectName}} 项目..."

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "🗑️  删除构建文件..."
if [ -d "build" ]; then
    rm -rf build/
    echo "✅ 已删除 build/ 目录"
fi

echo "🗑️  删除生成的 protobuf 文件..."
if [ -d "proto" ]; then
    find proto -name "*.pb.go" -delete 2>/dev/null || true
    find proto -name "*_grpc.pb.go" -delete 2>/dev/null || true
    echo "✅ 已删除生成的 .pb.go 文件"
fi

echo "🗑️  删除数据库文件..."
find . -name "*.db" -maxdepth 1 -delete 2>/dev/null || true
find . -name "*.sqlite" -maxdepth 1 -delete 2>/dev/null || true
find . -name "*.sqlite3" -maxdepth 1 -delete 2>/dev/null || true
echo "✅ 已删除数据库文件"

echo "🗑️  删除临时文件..."
find . -name "*.tmp" -delete 2>/dev/null || true
find . -name "*.log" -delete 2>/dev/null || true
find . -name ".DS_Store" -delete 2>/dev/null || true
echo "✅ 已删除临时文件"

echo "🗑️  清理 Go 模块缓存..."
go clean -cache 2>/dev/null || true
go clean -modcache 2>/dev/null || echo "需要 sudo 权限清理全局模块缓存"

echo "🗑️  删除测试覆盖率文件..."
find . -name "coverage.out" -delete 2>/dev/null || true
find . -name "*.cover" -delete 2>/dev/null || true

echo ""
echo "✅ 清理完成！"
echo ""
echo "保留的文件："
echo "  - 源代码文件 (*.go)"
echo "  - 配置文件 (config.yaml)"
echo "  - Proto 定义文件 (*.proto)"
echo "  - 脚本文件 (scripts/)"
echo "  - 文档文件 (README.md, *.md)"