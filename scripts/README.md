# 🔧 脚本使用说明

这个目录包含了 Go RPC Gateway 项目的所有自动化脚本和性能测试工具。

## 📄 脚本列表

### 核心开发脚本

| 脚本 | 功能 | 说明 |
|------|------|------|
| `generate.sh/bat` | 生成 Protobuf 代码 | 自动安装工具并生成 gRPC 代码 |
| `inject-tags.sh/bat` | 注入结构体标签 | 使用 protoc-go-inject-tag 为 Go 结构体注入标签 |
| `run.sh/bat` | 启动开发服务 | 自动检查依赖、生成代码并启动服务 |
| `build.sh/bat` | 构建项目 | 编译生成可执行文件，支持多平台构建 |
| `test.sh/bat` | 运行测试 | 单元测试、覆盖率统计、性能测试 |
| `clean.sh/bat` | 清理项目 | 删除构建文件、生成文件、临时文件 |

---

## 🚀 快速开始

### 1. 生成 Protobuf 代码

```bash
# Linux/Mac
./scripts/generate.sh

# Windows
scripts\generate.bat
```

### 2. 注入结构体标签

```bash
# Linux/Mac
./scripts/inject-tags.sh

# Windows
scripts\inject-tags.bat
```

### 3. 启动开发服务

```bash
# Linux/Mac
./scripts/run.sh

# Windows
scripts\run.bat
```

### 4. 构建项目

```bash
# 构建当前平台
./scripts/build.sh

# 构建所有平台
./scripts/build.sh --all
```

### 5. 运行测试

```bash
# 基础测试
./scripts/test.sh

# 包含覆盖率
./scripts/test.sh --coverage

# 性能测试
./scripts/test.sh --bench
```

### 6. 清理项目

```bash
./scripts/clean.sh
```

## 🔧 脚本特性

### 自动化检查

- ✅ 自动检查必需工具（protoc、go、git）
- ✅ 自动安装 protobuf 生成器
- ✅ 自动安装 protoc-go-inject-tag 标签注入工具
- ✅ 自动更新 Go 依赖
- ✅ 自动生成缺失的代码

### 智能检测

- 🔍 检测 proto 文件变化并自动重新生成
- 🔍 自动注入 GORM、JSON、Validator 标签
- 🔍 编译前检查代码语法
- 🔍 检测数据库文件并智能清理

### 跨平台支持

- 🌍 Linux、macOS、Windows 全支持
- 🌍 统一的命令接口
- 🌍 多架构构建支持

### 开发友好

- 📝 详细的错误信息和提示
- 📝 彩色输出和进度显示
- 📝 完整的使用说明

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
| `generate.sh` | `protoc --go_out=. --go-grpc_out=. proto/*.proto && protoc-go-inject-tag -input="proto/*.pb.go"` |
| `inject-tags.sh` | `protoc-go-inject-tag -input="proto/*.pb.go"` |
| `run.sh` | `go mod tidy && go run main.go` |
| `build.sh` | `go build -o build/app .` |
| `test.sh` | `go test -v ./...` |
| `clean.sh` | `rm -rf build/ && find . -name "*.pb.go" -delete` |

## 🎯 最佳实践

1. **开发时**: 使用 `run.sh` 启动服务，自动处理所有依赖
2. **标签注入**: 在 proto 文件中使用 `@gotags` 注释，运行 `inject-tags.sh` 注入
3. **测试时**: 使用 `test.sh --coverage` 确保代码质量
4. **构建时**: 使用 `build.sh --all` 生成多平台版本
5. **发布前**: 使用 `clean.sh` 清理并重新构建

## 🐛 故障排除

### 常见问题

1. **protoc 未安装**: 脚本会提示安装方法
2. **protoc-go-inject-tag 失败**: 检查 proto 文件中的 `@gotags` 注释格式
3. **标签注入无效**: 确保 `@gotags` 注释在字段定义的前一行
4. **权限问题**: 确保脚本有执行权限 `chmod +x scripts/*.sh`
5. **路径问题**: 脚本会自动切换到项目根目录

### 获取帮助

- 查看脚本源码了解详细功能
- 运行脚本查看具体错误信息
- 检查 Go 环境和网络连接
