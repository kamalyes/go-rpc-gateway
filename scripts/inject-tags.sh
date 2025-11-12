#!/bin/bash
# {{.ProjectName}} 标签注入脚本
# 使用 protoc-go-inject-tag 为生成的 Go 结构体注入标签

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 检查参数
VERBOSE=false
FORCE=false
INPUT_DIR="proto"

while [[ $# -gt 0 ]]; do
    case $1 in
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --force|-f)
            FORCE=true
            shift
            ;;
        --input|-i)
            INPUT_DIR="$2"
            shift 2
            ;;
        --help|-h)
            echo -e "${BLUE}标签注入脚本使用说明${NC}"
            echo ""
            echo -e "${BLUE}用法:${NC}"
            echo "  ./scripts/inject-tags.sh [选项]"
            echo ""
            echo -e "${BLUE}选项:${NC}"
            echo "  --verbose, -v     显示详细输出"
            echo "  --force, -f       强制执行（忽略检查）"
            echo "  --input, -i DIR   指定输入目录 (默认: proto)"
            echo "  --help, -h        显示帮助信息"
            echo ""
            echo -e "${BLUE}功能:${NC}"
            echo "  - 自动安装 protoc-go-inject-tag 工具"
            echo "  - 为生成的 .pb.go 文件注入结构体标签"
            echo "  - 支持 JSON、GORM、Validator 等标签"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ 未知选项: $1${NC}"
            echo "使用 --help 查看帮助信息"
            exit 1
            ;;
    esac
done

# 日志函数
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_verbose() {
    if [[ "$VERBOSE" == true ]]; then
        echo -e "${BLUE}🔍 $1${NC}"
    fi
}

# 检查 Go 环境
check_go() {
    if ! command -v go &> /dev/null; then
        log_error "Go 未安装，请先安装 Go 环境"
        exit 1
    fi
    
    log_verbose "Go 版本: $(go version)"
}

# 检查并安装 protoc-go-inject-tag
install_protoc_go_inject_tag() {
    if ! command -v protoc-go-inject-tag &> /dev/null; then
        log_info "安装 protoc-go-inject-tag..."
        
        if go install github.com/favadi/protoc-go-inject-tag@latest; then
            log_success "protoc-go-inject-tag 安装成功"
        else
            log_error "protoc-go-inject-tag 安装失败"
            log_error "请检查网络连接和 Go 环境配置"
            exit 1
        fi
    else
        log_verbose "protoc-go-inject-tag 已安装"
    fi
}

# 检查输入目录
check_input_dir() {
    if [[ ! -d "$INPUT_DIR" ]]; then
        log_error "输入目录不存在: $INPUT_DIR"
        log_error "请先运行生成脚本或指定正确的目录"
        exit 1
    fi
    
    log_verbose "检查输入目录: $INPUT_DIR"
}

# 查找 .pb.go 文件
find_pb_files() {
    local pb_files=($(find "$INPUT_DIR" -name "*.pb.go" -not -name "*_grpc.pb.go" -not -name "*.gw.go"))
    
    if [[ ${#pb_files[@]} -eq 0 ]]; then
        log_warning "在 $INPUT_DIR 目录中没有找到 .pb.go 文件"
        log_warning "请先运行 ./scripts/generate.sh 生成 protobuf 代码"
        
        if [[ "$FORCE" != true ]]; then
            exit 1
        fi
        
        return 1
    fi
    
    log_info "找到 ${#pb_files[@]} 个 .pb.go 文件"
    
    if [[ "$VERBOSE" == true ]]; then
        for file in "${pb_files[@]}"; do
            log_verbose "  - $file"
        done
    fi
    
    return 0
}

# 备份原文件
backup_files() {
    log_info "备份原始文件..."
    
    local backup_dir="${INPUT_DIR}/backup_$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$backup_dir"
    
    find "$INPUT_DIR" -name "*.pb.go" -not -name "*_grpc.pb.go" -not -name "*.gw.go" -exec cp {} "$backup_dir/" \;
    
    log_success "文件备份到: $backup_dir"
    echo "$backup_dir" > .inject_tags_backup_path
}

# 注入标签
inject_tags() {
    log_info "开始注入结构体标签..."
    
    local input_pattern="${INPUT_DIR}/*.pb.go"
    
    if [[ "$VERBOSE" == true ]]; then
        log_verbose "执行命令: protoc-go-inject-tag -input=\"$input_pattern\""
    fi
    
    if protoc-go-inject-tag -input="$input_pattern"; then
        log_success "标签注入完成"
        return 0
    else
        log_error "标签注入失败"
        return 1
    fi
}

# 验证注入结果
verify_injection() {
    log_info "验证标签注入结果..."
    
    local has_tags=false
    
    while IFS= read -r -d '' file; do
        if grep -q 'json:\|gorm:\|validate:\|form:\|query:\|uri:' "$file"; then
            has_tags=true
            log_verbose "文件 $file 包含注入的标签"
        fi
    done < <(find "$INPUT_DIR" -name "*.pb.go" -not -name "*_grpc.pb.go" -not -name "*.gw.go" -print0)
    
    if [[ "$has_tags" == true ]]; then
        log_success "标签注入验证通过"
    else
        log_warning "未发现注入的标签，请检查 proto 文件中的 @gotags 注释"
    fi
}

# 显示使用提示
show_usage_tips() {
    echo ""
    log_info "使用提示："
    echo "  1. 在 proto 文件中使用 @gotags 注释定义标签"
    echo "  2. 运行 ./scripts/generate.sh 生成 protobuf 代码"
    echo "  3. 运行此脚本注入结构体标签"
    echo ""
    log_info "标签示例："
    echo '  // @gotags: json:"username" gorm:"uniqueIndex" validate:"required"'
    echo '  string username = 1;'
    echo ""
    log_info "更多信息请查看: proto/README.md"
}

# 主函数
main() {
    log_info "{{.ProjectName}} 标签注入工具启动..."
    echo ""
    
    # 检查环境
    check_go
    install_protoc_go_inject_tag
    check_input_dir
    
    # 查找文件
    if ! find_pb_files; then
        if [[ "$FORCE" != true ]]; then
            show_usage_tips
            exit 1
        fi
    fi
    
    # 备份和注入
    backup_files
    
    if inject_tags; then
        verify_injection
        log_success "标签注入流程完成 🎉"
        
        # 清理备份路径文件
        rm -f .inject_tags_backup_path
    else
        # 失败时提供恢复选项
        if [[ -f .inject_tags_backup_path ]]; then
            local backup_path=$(cat .inject_tags_backup_path)
            log_error "注入失败，可以使用以下命令恢复原文件："
            log_error "  cp $backup_path/*.pb.go $INPUT_DIR/"
        fi
        exit 1
    fi
    
    show_usage_tips
}

# 执行主函数
main "$@"