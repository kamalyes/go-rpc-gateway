#!/bin/bash
# {{.ProjectName}} 开发工具脚本 v1.0
# 提供项目全生命周期管理功能

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 图标定义
ICON_SUCCESS="✅"
ICON_ERROR="❌"
ICON_WARNING="⚠️"
ICON_INFO="ℹ️"
ICON_ROCKET="🚀"
ICON_GEAR="⚙️"
ICON_CLEAN="🧹"
ICON_TEST="🧪"
ICON_BUILD="🏗️"

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 显示帮助信息
show_help() {
    echo -e "${BLUE}{{.ProjectName}} 开发工具脚本${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo "  ./scripts/dev.sh <命令> [选项]"
    echo ""
    echo -e "${CYAN}命令:${NC}"
    echo "  gen, generate     ${ICON_GEAR}  生成 Protobuf 代码"
    echo "  tags, inject      🏷️   注入结构体标签"
    echo "  setup, deps       📦  下载 Google APIs 依赖"
    echo "  run, start        ${ICON_ROCKET} 启动开发服务"
    echo "  build             ${ICON_BUILD} 构建项目"
    echo "  test              ${ICON_TEST}  运行测试"
    echo "  clean             ${ICON_CLEAN} 清理项目文件"
    echo "  help, h           ${ICON_INFO}  显示此帮助信息"
    echo ""
    echo -e "${CYAN}选项:${NC}"
    echo "  --verbose, -v     显示详细输出"
    echo "  --quiet, -q       静默模式"
    echo "  --force, -f       强制执行（跳过检查）"
    echo ""
    echo -e "${CYAN}示例:${NC}"
    echo "  ./scripts/dev.sh generate     # 生成 protobuf 代码"
    echo "  ./scripts/dev.sh tags         # 注入结构体标签"
    echo "  ./scripts/dev.sh run --verbose   # 启动服务并显示详细日志"
    echo "  ./scripts/dev.sh build --force   # 强制重新构建"
    echo "  ./scripts/dev.sh test --coverage # 运行测试并生成覆盖率"
    echo ""
    echo -e "${YELLOW}${ICON_INFO} 更多信息请查看 scripts/README.md${NC}"
}

# 解析命令行参数
VERBOSE=false
QUIET=false
FORCE=false
COMMAND=""

while [[ $# -gt 0 ]]; do
    case $1 in
        generate)
            COMMAND="generate"
            shift
            ;;
        tags|inject)
            COMMAND="inject"
            shift
            ;;
        setup|deps)
            COMMAND="setup"
            shift
            ;;
        run|start)
            COMMAND="run"
            shift
            ;;
        build)
            COMMAND="build"
            shift
            ;;
        test)
            COMMAND="test"
            shift
            ;;
        clean)
            COMMAND="clean"
            shift
            ;;
        help|h|-h|--help)
            show_help
            exit 0
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --quiet|-q)
            QUIET=true
            shift
            ;;
        --force|-f)
            FORCE=true
            shift
            ;;
        --coverage)
            COVERAGE=true
            shift
            ;;
        --bench)
            BENCH=true
            shift
            ;;
        --all)
            ALL=true
            shift
            ;;
        *)
            echo -e "${RED}${ICON_ERROR} 未知选项: $1${NC}"
            echo "使用 './scripts/dev.sh help' 查看帮助"
            exit 1
            ;;
    esac
done

# 如果没有指定命令，显示帮助
if [[ -z "$COMMAND" ]]; then
    show_help
    exit 0
fi

# 日志函数
log_info() {
    if [[ "$QUIET" != true ]]; then
        echo -e "${BLUE}${ICON_INFO} $1${NC}"
    fi
}

log_success() {
    if [[ "$QUIET" != true ]]; then
        echo -e "${GREEN}${ICON_SUCCESS} $1${NC}"
    fi
}

log_warning() {
    echo -e "${YELLOW}${ICON_WARNING} $1${NC}"
}

log_error() {
    echo -e "${RED}${ICON_ERROR} $1${NC}"
}

# 执行命令函数
run_command() {
    if [[ "$VERBOSE" == true ]]; then
        echo -e "${CYAN}执行: $1${NC}"
    fi
    
    if [[ "$VERBOSE" == true ]]; then
        eval "$1"
    else
        eval "$1" >/dev/null 2>&1
    fi
}

# 检查脚本是否存在
check_script() {
    local script_name="$1"
    local script_path="scripts/${script_name}.sh"
    
    if [[ ! -f "$script_path" ]]; then
        log_error "脚本 ${script_path} 不存在"
        return 1
    fi
    
    if [[ ! -x "$script_path" ]]; then
        log_info "设置脚本执行权限: ${script_path}"
        chmod +x "$script_path"
    fi
    
    return 0
}

# 执行脚本
execute_script() {
    local script_name="$1"
    shift
    
    if ! check_script "$script_name"; then
        return 1
    fi
    
    log_info "执行 ${script_name} 脚本..."
    
    # 构建命令行参数
    local args=""
    for arg in "$@"; do
        args="$args $arg"
    done
    
    if [[ "$VERBOSE" == true ]]; then
        args="$args --verbose"
    fi
    
    if [[ "$QUIET" == true ]]; then
        args="$args --quiet"
    fi
    
    if [[ "$FORCE" == true ]]; then
        args="$args --force"
    fi
    
    # 执行脚本
    if eval "./scripts/${script_name}.sh$args"; then
        log_success "${script_name} 执行成功"
        return 0
    else
        log_error "${script_name} 执行失败"
        return 1
    fi
}

# 主逻辑
case "$COMMAND" in
    generate)
        execute_script "generate"
        ;;
    inject)
        execute_script "inject-tags"
        ;;
    setup)
        execute_script "setup-googleapis"
        ;;
    run)
        execute_script "run"
        ;;
    build)
        local args=""
        if [[ "$ALL" == true ]]; then
            args="--all"
        fi
        execute_script "build" $args
        ;;
    test)
        local args=""
        if [[ "$COVERAGE" == true ]]; then
            args="$args --coverage"
        fi
        if [[ "$BENCH" == true ]]; then
            args="$args --bench"
        fi
        execute_script "test" $args
        ;;
    clean)
        execute_script "clean"
        ;;
    *)
        log_error "未知命令: $COMMAND"
        show_help
        exit 1
        ;;
esac

log_success "开发工具脚本执行完成 ${ICON_ROCKET}"