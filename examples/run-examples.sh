#!/bin/bash

# Go RPC Gateway Examples Runner
# 批量运行和测试所有示例

set -e

EXAMPLES_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$EXAMPLES_DIR/.." && pwd)"

echo "🚀 Go RPC Gateway Examples Runner"
echo "=================================="
echo

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ Go not found. Please install Go 1.21 or later."
    exit 1
fi

echo "📦 Go version: $(go version)"
echo "📁 Examples directory: $EXAMPLES_DIR"
echo "📁 Root directory: $ROOT_DIR"
echo

# 构建主程序
echo "🔨 Building main gateway..."
cd "$ROOT_DIR"
if [ ! -d "bin" ]; then
    mkdir bin
fi

cd cmd/gateway
go build -o ../../bin/gateway .
cd "$ROOT_DIR"

if [ -f "bin/gateway" ]; then
    echo "✅ Main gateway built successfully"
else
    echo "❌ Failed to build main gateway"
    exit 1
fi
echo

# 函数：运行单个示例
run_example() {
    local example_name="$1"
    local example_dir="$EXAMPLES_DIR/$example_name"
    
    if [ ! -d "$example_dir" ]; then
        echo "❌ Example directory not found: $example_dir"
        return 1
    fi
    
    echo "🔄 Running example: $example_name"
    echo "   Directory: $example_dir"
    
    cd "$example_dir"
    
    # 检查是否有main.go文件
    if [ ! -f "main.go" ]; then
        echo "❌ main.go not found in $example_dir"
        return 1
    fi
    
    # 尝试构建
    echo "   🔨 Building..."
    if go build -o "example_$example_name" main.go; then
        echo "   ✅ Build successful"
        
        # 可选：运行测试（这里只是构建测试）
        echo "   🧪 Build test passed"
        
        # 清理构建文件
        rm -f "example_$example_name"
        
        echo "   ✅ Example $example_name verified successfully"
        return 0
    else
        echo "   ❌ Build failed for $example_name"
        return 1
    fi
}

# 运行特定示例
run_specific_example() {
    local example_name="$1"
    local example_dir="$EXAMPLES_DIR/$example_name"
    
    echo "🎯 Running specific example: $example_name"
    echo "=================================="
    
    if [ ! -d "$example_dir" ]; then
        echo "❌ Example not found: $example_name"
        echo "Available examples:"
        ls -1 "$EXAMPLES_DIR" | grep -E "^[0-9]" | sort
        exit 1
    fi
    
    cd "$example_dir"
    
    echo "📍 Current directory: $(pwd)"
    echo "📦 Building and running..."
    echo
    
    # 运行示例
    go run main.go
}

# 测试所有示例
test_all_examples() {
    echo "🧪 Testing all examples..."
    echo "========================="
    echo
    
    local success_count=0
    local total_count=0
    local failed_examples=()
    
    # 查找所有示例目录
    for example_dir in "$EXAMPLES_DIR"/*/; do
        if [ -d "$example_dir" ]; then
            local example_name=$(basename "$example_dir")
            
            # 跳过非示例目录
            if [[ ! "$example_name" =~ ^[0-9] ]]; then
                continue
            fi
            
            ((total_count++))
            
            if run_example "$example_name"; then
                ((success_count++))
                echo
            else
                failed_examples+=("$example_name")
                echo
            fi
        fi
    done
    
    # 输出测试结果
    echo "📊 Test Results"
    echo "==============="
    echo "✅ Successful: $success_count/$total_count"
    
    if [ ${#failed_examples[@]} -gt 0 ]; then
        echo "❌ Failed examples:"
        for failed in "${failed_examples[@]}"; do
            echo "   - $failed"
        done
        exit 1
    else
        echo "🎉 All examples passed!"
    fi
}

# 显示帮助信息
show_help() {
    echo "Usage: $0 [OPTION] [EXAMPLE_NAME]"
    echo
    echo "Options:"
    echo "  test              Test all examples (build verification)"
    echo "  run <example>     Run a specific example"
    echo "  list              List all available examples"
    echo "  help              Show this help message"
    echo
    echo "Examples:"
    echo "  $0 test                           # Test all examples"
    echo "  $0 run 01-quickstart            # Run quickstart example"
    echo "  $0 run 04-pprof                 # Run pprof example"
    echo "  $0 list                          # List all examples"
    echo
    echo "Available examples:"
    ls -1 "$EXAMPLES_DIR" | grep -E "^[0-9]" | sort | while read example; do
        echo "  - $example"
    done
}

# 列出所有示例
list_examples() {
    echo "📚 Available Examples"
    echo "===================="
    echo
    
    for example_dir in "$EXAMPLES_DIR"/*/; do
        if [ -d "$example_dir" ]; then
            local example_name=$(basename "$example_dir")
            
            # 跳过非示例目录
            if [[ ! "$example_name" =~ ^[0-9] ]]; then
                continue
            fi
            
            echo "📁 $example_name"
            
            # 尝试读取描述（从main.go的注释中）
            local main_file="$example_dir/main.go"
            if [ -f "$main_file" ]; then
                local description=$(grep -E "Description:|@Description:" "$main_file" | head -1 | sed 's/.*Description: *//' | sed 's/ \*//')
                if [ -n "$description" ]; then
                    echo "   📝 $description"
                fi
            fi
            
            echo
        fi
    done
}

# 主逻辑
case "${1:-help}" in
    "test")
        test_all_examples
        ;;
    "run")
        if [ -z "$2" ]; then
            echo "❌ Please specify an example name"
            echo "Use '$0 list' to see available examples"
            exit 1
        fi
        run_specific_example "$2"
        ;;
    "list")
        list_examples
        ;;
    "help"|"--help"|"-h")
        show_help
        ;;
    *)
        echo "❌ Unknown option: $1"
        echo
        show_help
        exit 1
        ;;
esac