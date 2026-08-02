# 🔧 脚本使用说明

这个目录包含了 Go RPC Gateway 项目的所有自动化脚本和性能测试工具。

## 📄 脚本列表

### 核心开发脚本

| 脚本 | 功能 | 说明 |
|------|------|------|
| `generate.bat` | 生成 Protobuf 代码 | 检查并安装 protoc 插件，使用 `protoc` 生成 gRPC/gateway/OpenAPI 代码并注入标签（仅 Windows） |
| `inject-tags.sh/bat` | 注入结构体标签 | 使用 protoc-go-inject-tag 为生成的 Go 结构体注入 JSON/GORM/Validator 等标签 |
| `dev.sh/bat` | 开发工具入口 | 统一入口，委派到各子脚本（generate/inject/build/test/clean 等） |
| `build.sh/bat` | 构建项目 | 编译生成可执行文件，注入版本信息，支持多平台构建 |
| `test.sh/bat` | 运行测试 | 单元测试、覆盖率统计、性能测试 |
| `clean.sh/bat` | 清理项目 | 删除构建文件、生成文件、临时文件 |
| `deploy.sh/bat` | 部署服务 | 安装/启停/卸载系统服务（Linux 使用 systemd，Windows 使用 `sc`/`net`） |
| `setup-dependencies.bat` | 下载依赖 | 通过 git 下载 Google APIs 和 gRPC-Gateway 的 proto 文件到 GOPATH（仅 Windows） |
| `setup-protobuf-includes.bat` | 设置 protobuf include | 下载标准 protobuf `.proto` 文件到 protoc 的 include 目录（仅 Windows） |

> 注：`generate`、`setup-dependencies`、`setup-protobuf-includes` 仅提供 Windows（`.bat`）版本。跨平台生成代码也可使用 `buf generate`（配置见 `proto/buf.gen.yaml`）。

---

## 🚀 快速开始

### 1. 生成 Protobuf 代码

```bash
# Windows（基于 protoc 直接调用，自动安装缺失插件并注入标签）
scripts\generate.bat

# 跨平台（使用 buf，依赖 proto/buf.gen.yaml）
buf generate
```

### 2. 注入结构体标签

```bash
# Linux/Mac
./scripts/inject-tags.sh

# Windows
scripts\inject-tags.bat

# 常用选项
./scripts/inject-tags.sh --verbose        # 详细输出
./scripts/inject-tags.sh --input=proto    # 指定输入目录（默认 proto）
./scripts/inject-tags.sh --force          # 强制执行（忽略检查）
./scripts/inject-tags.sh --help           # 查看帮助
```

### 3. 使用开发工具入口

`dev.sh`/`dev.bat` 是统一入口，可调用其他子脚本：

```bash
# Linux/Mac
./scripts/dev.sh <命令> [选项]

# Windows
scripts\dev.bat <命令> [选项]

# 支持的命令
./scripts/dev.sh generate       # 生成 protobuf 代码
./scripts/dev.sh tags           # 注入结构体标签
./scripts/dev.sh build          # 构建项目
./scripts/dev.sh build --all    # 多平台构建
./scripts/dev.sh test           # 运行测试
./scripts/dev.sh test --coverage
./scripts/dev.sh clean          # 清理项目
./scripts/dev.sh help           # 查看帮助

# 通用选项：--verbose/-v、--quiet/-q、--force/-f
```

> 注：`dev.sh/bat` 的 `run`、`setup` 命令分别委派到 `run.sh/bat`、`setup-googleapis.sh/bat`，这两个子脚本当前不存在。

### 4. 构建项目

```bash
# 构建当前平台
./scripts/build.sh          # Linux/Mac
scripts\build.bat            # Windows

# 构建所有平台（linux/amd64、windows/amd64、darwin/amd64、darwin/arm64）
./scripts/build.sh --all
```

构建时会通过 `-ldflags` 注入版本信息（`main.Version`、`main.BuildTime`、`main.GitCommit`），产物输出到 `build/` 目录。

### 5. 运行测试

```bash
# 基础测试（go test -v -race ./...）
./scripts/test.sh

# 包含覆盖率（生成 coverage.out 与 coverage.html）
./scripts/test.sh --coverage

# 性能测试（go test -bench=. -benchmem ./...）
./scripts/test.sh --bench
```

测试前会自动执行 `go mod tidy` 和 `go vet ./...`，必要时会先调用 `generate` 补齐 protobuf 代码。

### 6. 清理项目

```bash
./scripts/clean.sh
```

清理 `build/` 目录、`proto/*.pb.go` 与 `proto/*_grpc.pb.go`、数据库文件（`*.db`/`*.sqlite`/`*.sqlite3`）、临时文件（`*.tmp`/`*.log`）、覆盖率文件，并执行 `go clean -cache` 与 `go clean -modcache`。

### 7. 部署服务（生产环境）

```bash
# Linux（systemd，需 sudo）
sudo ./scripts/deploy.sh install       # 安装为 systemd 服务
sudo ./scripts/deploy.sh start         # 启动
sudo ./scripts/deploy.sh stop          # 停止
sudo ./scripts/deploy.sh restart       # 重启
./scripts/deploy.sh status             # 查看服务状态
./scripts/deploy.sh logs               # 查看日志
sudo ./scripts/deploy.sh uninstall     # 卸载（加 --force 同时删除部署/日志目录）

# Windows（sc/net，需管理员权限）
scripts\deploy.bat install
scripts\deploy.bat start
scripts\deploy.bat stop
scripts\deploy.bat restart
scripts\deploy.bat status
scripts\deploy.bat uninstall
```

选项：`--port`（默认 8080）、`--deploy-dir`、`--log-dir`、`--force`；Linux 版本还支持 `--user` 及环境变量 `SERVICE_PORT`、`DEPLOY_DIR`、`LOG_DIR`。

### 8. 下载 gRPC-Gateway 依赖（仅 Windows）

```bash
scripts\setup-dependencies.bat
```

通过 `git clone --depth=1` 下载 `googleapis`（master）与 `grpc-gateway`（v2.19.0）到 `%GOPATH%\src\github.com\`，并校验 `annotations.proto`、`http.proto` 等关键文件，供 `generate.bat` 使用。

### 9. 设置 Protobuf Include（仅 Windows）

```bash
scripts\setup-protobuf-includes.bat
```

将 `descriptor.proto`、`timestamp.proto`、`wrappers.proto`、`struct.proto`、`any.proto`、`empty.proto`、`duration.proto`、`field_mask.proto` 等标准文件下载到 protoc 的 include 目录（`<protoc>\..\include`）。

## 🔧 脚本特性

### 自动化检查

- ✅ 自动检查必需工具（protoc、go、git）
- ✅ 自动安装缺失的 protoc-gen-* 插件与 protoc-go-inject-tag
- ✅ 自动更新 Go 依赖（`go mod tidy`）
- ✅ 自动生成缺失的 protobuf 代码（build/test 时）
- ✅ 自动下载 Google APIs / gRPC-Gateway 依赖与标准 protobuf include（`generate.bat` 触发）

### 智能检测

- 🔍 检测 proto 文件并自动重新生成
- 🔍 自动注入 GORM、JSON、Validator 标签
- 🔍 编译前运行 `go vet` 检查
- 🔍 检测数据库文件（`*.db`/`*.sqlite*`）并智能清理

### 跨平台支持

- 🌍 Linux、macOS、Windows 全支持
- 🌍 统一的命令接口（`dev.sh`/`dev.bat`）
- 🌍 多架构构建支持（linux/amd64、windows/amd64、darwin/amd64、darwin/arm64）

### 开发友好

- 📝 详细的错误信息和提示
- 📝 彩色输出和进度显示
- 📝 完整的使用说明（`--help`/`-h`）

## 🛠️ 自定义脚本

### 添加新脚本

1. 创建 `.sh` 和 `.bat` 两个版本
2. 添加适当的错误检查
3. 更新此 README 文档

### 脚本模板

```bash
#!/bin/bash
set -e

echo "🔧 脚本功能描述..."

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 脚本逻辑...

echo "✅ 操作完成！"
```

## 🔗 相关命令

### 手动命令对照表

| 脚本 | 等效手动命令 |
|------|-------------|
| `generate.bat` | `protoc -I. --go_out=. --go-grpc_out=. --grpc-gateway_out=. --openapiv2_out=docs proto/*.proto` 然后 `protoc-go-inject-tag -input="proto/*.pb.go"` |
| `inject-tags.sh/bat` | `protoc-go-inject-tag -input="proto/*.pb.go"` |
| `dev.sh/bat` | 调用对应子脚本（如 `./scripts/build.sh`） |
| `build.sh/bat` | `go mod tidy && go build -ldflags "-w -s -X main.Version=..." -o build/<app> .` |
| `test.sh/bat` | `go vet ./... && go test -v -race ./...` |
| `clean.sh/bat` | `rm -rf build/ && find proto -name "*.pb.go" -delete && go clean -cache` |
| `deploy.sh` | `systemctl start/stop/restart/status <service>` |
| `deploy.bat` | `sc create/delete`、`net start/stop` |

## 🎯 最佳实践

1. **开发时**：使用 `dev.sh`/`dev.bat <命令>` 作为统一入口，自动处理依赖
2. **标签注入**：在 proto 文件中使用 `@gotags` 注释，运行 `inject-tags.sh/bat` 注入
3. **测试时**：使用 `test.sh --coverage` 确保代码质量
4. **构建时**：使用 `build.sh --all` 生成多平台版本
5. **发布前**：使用 `clean.sh` 清理并重新构建
6. **部署**：Linux 用 `deploy.sh`（systemd），Windows 用 `deploy.bat`（Windows 服务）

## 🐛 故障排除

### 常见问题

1. **protoc 未安装**：`generate.bat` 会提示安装方法（`choco install protoc` 或从 GitHub releases 下载）
2. **protoc-gen-* 插件缺失**：`generate.bat` 会自动通过 `go install` 安装
3. **protoc-go-inject-tag 失败**：检查 proto 文件中的 `@gotags` 注释格式
4. **标签注入无效**：确保 `@gotags` 注释在字段定义的前一行
5. **权限问题**：`deploy.sh` 需 sudo、`deploy.bat` 需管理员权限；Linux/Mac 脚本需 `chmod +x scripts/*.sh`
6. **路径问题**：脚本会自动切换到项目根目录

### 获取帮助

- 运行 `scripts/dev.sh help` 或 `scripts/dev.bat help` 查看开发工具帮助
- 运行各脚本的 `--help`/`-h` 选项查看详细用法
- 查看脚本源码了解详细功能
- 检查 Go 环境和网络连接
