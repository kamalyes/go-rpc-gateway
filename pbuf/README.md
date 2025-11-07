# Protocol Buffers 生成工具使用指南

本目录包含自动生成Protocol Buffers代码的完整工具链。

## 目录结构
```
pbuf/
├── buf.gen.yaml        # buf 生成配置文件
├── generate.bat        # Windows 生成脚本
├── generate.sh         # Linux/Mac 生成脚本
├── Makefile           # Make 构建脚本
├── README.md          # 本说明文件
├── common/
│   └── common.proto   # 公共消息定义
```

## 使用方法
```bash
buf generate
```

每个 `.proto` 文件会生成以下文件：

1. **`*.pb.go`** - Protocol Buffers 消息定义
2. **`*_grpc.pb.go`** - gRPC 服务接口  
3. **`*.gw.go`** - grpc-gateway HTTP/JSON 代理
4. **`*.swagger.json`** - OpenAPI/Swagger 文档

## 配置说明

### buf.gen.yaml 配置

- **go**: 生成Go语言的protobuf消息
- **go-grpc**: 生成Go语言的gRPC服务接口
- **grpc-gateway**: 生成HTTP/JSON到gRPC的网关代码
- **openapiv2**: 生成OpenAPI/Swagger文档

### 输出选项

- `paths=source_relative`: 生成的文件与源文件保持相同的目录结构
- `logtostderr=true`: 错误日志输出到stderr
- `json_names_for_fields=false`: 使用原始字段名而非JSON名称
- `allow_merge=true`: 允许合并多个API文档

## 依赖工具

运行前请确保安装以下工具：

```bash
# 安装 buf
go install github.com/bufbuild/buf/cmd/buf@latest

# 安装 protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# 安装 protoc-gen-go-grpc
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 安装 protoc-gen-grpc-gateway
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

# 安装 protoc-gen-openapiv2
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
```

## 常见问题

### 1. 权限错误

**Windows**: 以管理员身份运行PowerShell  
**Linux/Mac**: 确保脚本有执行权限 `chmod +x generate.sh`

### 2. 找不到protoc-gen-* 插件

确保 `$GOPATH/bin` 或 `$GOBIN` 在系统 PATH 中。

### 3. buf 版本兼容性

建议使用最新版本的buf工具，部分旧版本可能存在兼容性问题。

### 4. 导入路径问题

确保 `.proto` 文件中的 `import` 语句使用正确的相对路径。

## 开发工作流

1. **修改proto文件**：在相应目录下编辑 `.proto` 文件
2. **生成代码**：运行 `make proto-gen`
3. **验证生成**：检查生成的Go文件是否正确
4. **提交代码**：将proto文件和生成的代码一起提交

## 性能优化

- 使用 `make watch` 开发时自动生成
- 大型项目可考虑分目录并行生成  
- CI/CD 中使用 `make verify` 确保代码同步

## 扩展配置

如需添加新的生成插件，编辑 `buf.gen.yaml` 文件：

```yaml
plugins:
  - name: your-custom-plugin
    out: .
    opt:
      - your-option=value
```

## 支持与维护

如有问题请查阅：

- [buf官方文档](https://docs.buf.build/)
- [gRPC-Gateway文档](https://grpc-ecosystem.github.io/grpc-gateway/)
- [Protocol Buffers官方文档](https://developers.google.com/protocol-buffers/)