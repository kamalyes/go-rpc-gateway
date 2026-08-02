# Protocol Buffers 生成工具使用指南

本目录包含自动生成 Protocol Buffers 代码的完整工具链。

## 目录结构

```bash
proto/
├── buf.gen.yaml        # buf 生成配置文件
├── common.proto        # 公共消息与枚举定义
├── common.pb.go        # 生成的 Go 代码（包名 commonapis）
└── README.md           # 本说明文件
```

> 注：`common.proto` 中声明了 `option go_package = "github.com/kamalyes/go-rpc-gateway/pbuf/commonapis"`，生成的 Go 代码包名为 `commonapis`；同时声明了 `option java_package = "com.github.kamalyes.commonapis.api"`。

## 公共消息定义 (common.proto)

### StatusCode - 公共状态码枚举

```protobuf
enum StatusCode {
  OK = 0;
  Canceled = 1;
  Unknown = 2;
  InvalidArgument = 3;
  DeadlineExceeded = 4;
  NotFound = 5;
  AlreadyExists = 6;
  PermissionDenied = 7;
  ResourceExhausted = 8;
  FailedPrecondition = 9;
  Aborted = 10;
  OutOfRange = 11;
  Unimplemented = 12;
  Internal = 13;
  Unavailable = 14;
  DataLoss = 15;
  Unauthenticated = 16;
  reserved 17 to 100;

  // 通用系统错误
  SYSTEM_UNKNOWN_ERROR = 101;
  SYSTEM_NETWORK_ERROR = 102;
  SYSTEM_STORAGE_ERROR = 103;
  reserved 104 to 200;
}
```

### Result - 通用处理结果

通常在 Batch 类请求中返回每个子请求的错误状态。

```protobuf
message Result {
  int32 code = 1;           // 通用服务/系统错误码
  string error = 2;          // 错误详细描述
  StatusCode status = 3;     // 业务错误码
}
```

### Paging - 通用翻页结构

```protobuf
message Paging {
  int32 page = 1;     // 页码，可选，默认值为 1
  int32 size = 2;      // 数量，取值范围 [1,100]
  int32 total = 3;     // 总数，请求无须填写，响应由系统填写
}
```

### Sorting - 通用排序结构

```protobuf
message Sorting {
  string field = 1;     // 排序字段，由对应接口解析
  bool reversed = 2;     // 可选，是否倒序
}
```

### TimeRange - 时间区间

表示一段时间区间 `[start, end)`，时间戳为 UTC 时间，JSON 表示形式为 RFC 3339 格式。

```protobuf
message TimeRange {
  google.protobuf.Timestamp start = 1;   // 开始时间，区间包含
  google.protobuf.Timestamp end = 2;     // 结束时间，区间不包含
}
```

> `TimeRange` 依赖 `import "google/protobuf/timestamp.proto";`。

## 使用方法

使用 buf 工具链生成代码：

```bash
buf generate
```

或者使用项目提供的脚本（Windows，基于 `protoc` 直接调用，功能与 `buf.gen.yaml` 相当）：
```bash
scripts\generate.bat
```

每个 `.proto` 文件根据 `buf.gen.yaml` 配置可能生成以下文件：

1. **`*.pb.go`** - Protocol Buffers 消息定义（`go` 插件）
2. **`*_grpc.pb.go`** - gRPC 服务接口（`go-grpc` 插件，仅当 `.proto` 定义了 `service` 时生成）
3. **`*.gw.go`** - grpc-gateway HTTP/JSON 代理（`grpc-gateway` 插件，仅当 `.proto` 定义了 `service` 时生成）
4. **`api.swagger.json`** - OpenAPI/Swagger 文档（`openapiv2` 插件，输出到 `docs` 目录）

> 注：当前 `common.proto` 仅定义消息与枚举，未定义 `service`，因此实际只生成 `common.pb.go`，不会生成 `*_grpc.pb.go`、`*.gw.go` 与 swagger 文档。

## 配置说明

### buf.gen.yaml 配置

`version: v1`，包含以下插件：

- **go**: 生成 Go 语言的 protobuf 消息，输出到当前目录（`.`）
- **go-grpc**: 生成 Go 语言的 gRPC 服务接口，输出到当前目录（`.`）
- **grpc-gateway**: 生成 HTTP/JSON 到 gRPC 的网关代码，输出到当前目录（`.`）
- **openapiv2**: 生成 OpenAPI/Swagger 文档，输出到 `docs` 目录

### 输出选项

- `paths=source_relative`: 生成的文件与源文件保持相同的目录结构（go、go-grpc、grpc-gateway）
- `generate_unbound_methods=true`: 为未绑定方法生成代理（grpc-gateway）
- `logtostderr=true`: 错误日志输出到 stderr（openapiv2）
- `json_names_for_fields=false`: 使用原始字段名而非 JSON 名称（openapiv2）
- `allow_merge=true`: 允许合并多个 API 文档（openapiv2）
- `merge_file_name=api`: 合并后的文档文件名为 `api.swagger.json`（openapiv2）
- `use_go_templates=true`: 启用 Go 模板（openapiv2）
- `repeated_path_param_separator=ssv`: 重复路径参数使用空格分隔（openapiv2）
- `openapi_naming_strategy=fqn`: 使用完全限定名作为命名策略（openapiv2）
- `simple_operation_ids=true`: 使用简单的 operation ID（openapiv2）

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

# 安装 protoc-go-inject-tag（用于注入结构体标签）
go install github.com/ kamalyes/protoc-go-inject-tag@latest
```

> 使用 `scripts/generate.bat` 时还会用到 `protoc` 本体；该脚本会自动检查并安装上述缺失的插件，并按需调用 `scripts/setup-protobuf-includes.bat` 与 `scripts/setup-dependencies.bat` 准备依赖文件。

## 常见问题

### 1. 权限错误

**Windows**: 以管理员身份运行 PowerShell
**Linux/Mac**: 确保脚本有执行权限 `chmod +x scripts/*.sh`

### 2. 找不到 protoc-gen-* 插件

确保 `$GOPATH/bin` 或 `$GOBIN` 在系统 PATH 中。`scripts/generate.bat` 会自动检测并通过 `go install` 安装缺失的插件。

### 3. buf 版本兼容性

建议使用最新版本的 buf 工具，部分旧版本可能存在兼容性问题。

### 4. 导入路径问题

确保 `.proto` 文件中的 `import` 语句使用正确的相对路径。`common.proto` 依赖 `google/protobuf/timestamp.proto`，若本地 protoc include 目录缺失该文件，可运行 `scripts/setup-protobuf-includes.bat`（Windows）下载标准 protobuf 文件。

## 开发工作流

1. **修改 proto 文件**：在 `proto/` 目录下编辑 `.proto` 文件
2. **生成代码**：运行 `buf generate` 或 `scripts/generate.bat`
3. **注入标签**（可选）：运行 `scripts/inject-tags.sh` 或 `scripts/inject-tags.bat`
4. **验证生成**：检查生成的 Go 文件是否正确
5. **提交代码**：将 proto 文件和生成的代码一起提交

## 性能优化

- 大型项目可考虑分目录并行生成
- CI/CD 中可运行 `buf generate` 或 `scripts/generate.bat` 后，通过 `git diff` 校验生成代码是否与 proto 定义同步

## 扩展配置

如需添加新的生成插件，编辑 `buf.gen.yaml` 文件：

```yaml
version: v1
plugins:
  - name: your-custom-plugin
    out: .
    opt:
      - your-option=value
```

## 支持与维护

如有问题请查阅：

- [buf 官方文档](https://docs.buf.build/)
- [gRPC-Gateway 文档](https://grpc-ecosystem.github.io/grpc-gateway/)
- [Protocol Buffers 官方文档](https://developers.google.com/protocol-buffers/)
