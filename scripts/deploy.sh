#!/bin/bash
# {{.ProjectName}} 部署脚本
# 用于生产环境部署和服务管理

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 图标定义
ICON_SUCCESS="✅"
ICON_ERROR="❌"
ICON_WARNING="⚠️"
ICON_INFO="ℹ️"
ICON_ROCKET="🚀"
ICON_DEPLOY="🔧"
ICON_STOP="🛑"
ICON_STATUS="📊"

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 配置变量
APP_NAME="{{.ProjectName}}"
SERVICE_PORT="${SERVICE_PORT:-8080}"
BUILD_DIR="build"
DEPLOY_DIR="${DEPLOY_DIR:-/opt/$APP_NAME}"
LOG_DIR="${LOG_DIR:-/var/log/$APP_NAME}"
PID_FILE="/var/run/$APP_NAME.pid"
SERVICE_FILE="/etc/systemd/system/$APP_NAME.service"

# 显示帮助信息
show_help() {
    echo -e "${BLUE}$APP_NAME 部署工具${NC}"
    echo ""
    echo -e "${CYAN}用法:${NC}"
    echo "  ./scripts/deploy.sh <命令> [选项]"
    echo ""
    echo -e "${CYAN}命令:${NC}"
    echo "  install           ${ICON_DEPLOY} 安装服务到系统"
    echo "  start             ${ICON_ROCKET} 启动服务"
    echo "  stop              ${ICON_STOP}  停止服务"
    echo "  restart           🔄 重启服务"
    echo "  status            ${ICON_STATUS} 查看服务状态"
    echo "  logs              📝 查看服务日志"
    echo "  uninstall         🗑️  卸载服务"
    echo "  help              ${ICON_INFO}  显示帮助信息"
    echo ""
    echo -e "${CYAN}选项:${NC}"
    echo "  --port PORT       指定服务端口 (默认: 8080)"
    echo "  --deploy-dir DIR  指定部署目录 (默认: /opt/$APP_NAME)"
    echo "  --log-dir DIR     指定日志目录 (默认: /var/log/$APP_NAME)"
    echo "  --user USER       指定运行用户 (默认: 当前用户)"
    echo "  --force           强制执行"
    echo ""
    echo -e "${CYAN}环境变量:${NC}"
    echo "  SERVICE_PORT      服务端口"
    echo "  DEPLOY_DIR        部署目录"
    echo "  LOG_DIR           日志目录"
}

# 日志函数
log_info() {
    echo -e "${BLUE}${ICON_INFO} $1${NC}"
}

log_success() {
    echo -e "${GREEN}${ICON_SUCCESS} $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}${ICON_WARNING} $1${NC}"
}

log_error() {
    echo -e "${RED}${ICON_ERROR} $1${NC}"
}

# 检查权限
check_privileges() {
    if [[ $EUID -ne 0 ]]; then
        log_error "此操作需要 root 权限，请使用 sudo 运行"
        exit 1
    fi
}

# 检查构建文件
check_build() {
    if [[ ! -f "$BUILD_DIR/$APP_NAME" ]]; then
        log_error "构建文件不存在，请先运行 ./scripts/build.sh"
        exit 1
    fi
}

# 创建用户（如果不存在）
create_user() {
    local username="$1"
    if ! id "$username" &>/dev/null; then
        log_info "创建用户: $username"
        useradd -r -s /bin/false -d "$DEPLOY_DIR" "$username"
    fi
}

# 创建目录
create_directories() {
    log_info "创建必要目录..."
    
    mkdir -p "$DEPLOY_DIR"
    mkdir -p "$LOG_DIR"
    mkdir -p "$(dirname "$PID_FILE")"
    
    if [[ -n "$RUN_USER" ]]; then
        chown -R "$RUN_USER:$RUN_USER" "$DEPLOY_DIR"
        chown -R "$RUN_USER:$RUN_USER" "$LOG_DIR"
    fi
}

# 安装服务
install_service() {
    log_info "安装 $APP_NAME 服务..."
    
    check_build
    
    # 创建目录
    create_directories
    
    # 创建用户（如果指定）
    if [[ -n "$RUN_USER" ]] && [[ "$RUN_USER" != "root" ]]; then
        create_user "$RUN_USER"
    fi
    
    # 复制二进制文件
    log_info "复制应用程序到 $DEPLOY_DIR"
    cp "$BUILD_DIR/$APP_NAME" "$DEPLOY_DIR/"
    chmod +x "$DEPLOY_DIR/$APP_NAME"
    
    # 复制配置文件
    if [[ -f "config.yaml" ]]; then
        log_info "复制配置文件"
        cp "config.yaml" "$DEPLOY_DIR/"
    fi
    
    # 创建 systemd 服务文件
    create_systemd_service
    
    # 重新加载 systemd
    log_info "重新加载 systemd..."
    systemctl daemon-reload
    systemctl enable "$APP_NAME"
    
    log_success "$APP_NAME 服务安装完成"
    log_info "使用以下命令管理服务:"
    log_info "  启动: systemctl start $APP_NAME"
    log_info "  停止: systemctl stop $APP_NAME"
    log_info "  状态: systemctl status $APP_NAME"
}

# 创建 systemd 服务文件
create_systemd_service() {
    log_info "创建 systemd 服务文件..."
    
    cat > "$SERVICE_FILE" << EOF
[Unit]
Description={{.ProjectName}} gRPC Gateway Service
After=network.target
Wants=network.target

[Service]
Type=simple
User=${RUN_USER:-root}
WorkingDirectory=$DEPLOY_DIR
ExecStart=$DEPLOY_DIR/$APP_NAME
ExecReload=/bin/kill -HUP \$MAINPID
KillMode=mixed
Restart=on-failure
RestartSec=5s

# 环境变量
Environment=SERVICE_PORT=$SERVICE_PORT
Environment=LOG_LEVEL=info

# 日志
StandardOutput=append:$LOG_DIR/$APP_NAME.log
StandardError=append:$LOG_DIR/$APP_NAME.error.log

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$LOG_DIR

[Install]
WantedBy=multi-user.target
EOF
    
    log_success "systemd 服务文件已创建: $SERVICE_FILE"
}

# 启动服务
start_service() {
    log_info "启动 $APP_NAME 服务..."
    
    if systemctl is-active --quiet "$APP_NAME"; then
        log_warning "服务已在运行"
        return 0
    fi
    
    systemctl start "$APP_NAME"
    
    # 检查启动状态
    sleep 2
    if systemctl is-active --quiet "$APP_NAME"; then
        log_success "$APP_NAME 服务启动成功"
        show_service_status
    else
        log_error "$APP_NAME 服务启动失败"
        systemctl status "$APP_NAME"
        exit 1
    fi
}

# 停止服务
stop_service() {
    log_info "停止 $APP_NAME 服务..."
    
    if ! systemctl is-active --quiet "$APP_NAME"; then
        log_warning "服务未运行"
        return 0
    fi
    
    systemctl stop "$APP_NAME"
    log_success "$APP_NAME 服务已停止"
}

# 重启服务
restart_service() {
    log_info "重启 $APP_NAME 服务..."
    systemctl restart "$APP_NAME"
    
    sleep 2
    if systemctl is-active --quiet "$APP_NAME"; then
        log_success "$APP_NAME 服务重启成功"
        show_service_status
    else
        log_error "$APP_NAME 服务重启失败"
        systemctl status "$APP_NAME"
        exit 1
    fi
}

# 显示服务状态
show_service_status() {
    echo ""
    systemctl status "$APP_NAME" --no-pager
}

# 查看日志
show_logs() {
    if [[ -f "$LOG_DIR/$APP_NAME.log" ]]; then
        echo -e "${CYAN}应用日志:${NC}"
        tail -50 "$LOG_DIR/$APP_NAME.log"
    fi
    
    if [[ -f "$LOG_DIR/$APP_NAME.error.log" ]]; then
        echo -e "${RED}错误日志:${NC}"
        tail -20 "$LOG_DIR/$APP_NAME.error.log"
    fi
    
    echo ""
    echo -e "${CYAN}systemd 日志:${NC}"
    journalctl -u "$APP_NAME" -n 20 --no-pager
}

# 卸载服务
uninstall_service() {
    log_info "卸载 $APP_NAME 服务..."
    
    # 停止服务
    if systemctl is-active --quiet "$APP_NAME"; then
        systemctl stop "$APP_NAME"
    fi
    
    # 禁用服务
    if systemctl is-enabled --quiet "$APP_NAME"; then
        systemctl disable "$APP_NAME"
    fi
    
    # 删除服务文件
    if [[ -f "$SERVICE_FILE" ]]; then
        rm -f "$SERVICE_FILE"
        systemctl daemon-reload
    fi
    
    # 删除文件（可选）
    if [[ "$FORCE" == true ]]; then
        log_warning "删除部署文件..."
        rm -rf "$DEPLOY_DIR"
        rm -rf "$LOG_DIR"
    else
        log_info "部署文件保留在: $DEPLOY_DIR"
        log_info "日志文件保留在: $LOG_DIR"
        log_info "使用 --force 选项可完全删除"
    fi
    
    log_success "$APP_NAME 服务已卸载"
}

# 解析参数
COMMAND=""
RUN_USER=""
FORCE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        install|start|stop|restart|status|logs|uninstall|help)
            COMMAND="$1"
            shift
            ;;
        --port)
            SERVICE_PORT="$2"
            shift 2
            ;;
        --deploy-dir)
            DEPLOY_DIR="$2"
            shift 2
            ;;
        --log-dir)
            LOG_DIR="$2"
            shift 2
            ;;
        --user)
            RUN_USER="$2"
            shift 2
            ;;
        --force)
            FORCE=true
            shift
            ;;
        *)
            log_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 检查命令
if [[ -z "$COMMAND" ]]; then
    show_help
    exit 0
fi

# 需要 root 权限的命令
case "$COMMAND" in
    install|start|stop|restart|uninstall)
        check_privileges
        ;;
esac

# 执行命令
case "$COMMAND" in
    install)
        install_service
        ;;
    start)
        start_service
        ;;
    stop)
        stop_service
        ;;
    restart)
        restart_service
        ;;
    status)
        show_service_status
        ;;
    logs)
        show_logs
        ;;
    uninstall)
        uninstall_service
        ;;
    help)
        show_help
        ;;
    *)
        log_error "未知命令: $COMMAND"
        show_help
        exit 1
        ;;
esac