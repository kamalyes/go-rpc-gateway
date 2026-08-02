# Response Package

HTTP 响应标准化工具模块，提供统一的响应格式和便捷的响应写入函数

## 文件结构

```
response/
├── writer.go      # 核心写入函数和编码器池
├── types.go       # 响应类型定义和常量
├── error.go       # 错误响应相关函数
├── success.go     # 成功响应相关函数
├── health.go      # 健康检查相关函数
├── server.go      # 服务器响应工具函数（避免循环导入）
```

## 公共 API

所有响应统一输出 `*commonapis.Result`（字段：`Code int32`、`Error string`、`Status commonapis.StatusCode`），自定义结构体则直接输出 JSON。

### writer.go — 核心写入

```go
// 写入标准化 Result 响应（使用 sync.Pool 优化编码）
func WriteResult(w http.ResponseWriter, httpStatus int, result *commonapis.Result)

// 写入自定义 JSON 响应（使用 sync.Pool 优化编码）
func WriteJSONResponse(w http.ResponseWriter, httpStatus int, data any)
```

### server.go — Server 包工具（避免循环导入）

```go
// 写入 AppError 标准化错误响应（不使用对象池）
func WriteErrorResponse(w http.ResponseWriter, appErr *errors.AppError)

// 写入 Result 响应（不使用对象池）
func WriteResultResponse(w http.ResponseWriter, httpStatus int, result *commonapis.Result)

// 写入简单错误响应（不依赖 errors 包）
func WriteSimpleError(w http.ResponseWriter, httpStatus int, statusCode commonapis.StatusCode, message string)
```

### success.go — 成功响应

```go
func WriteSuccessResult(w http.ResponseWriter, message string)
func WriteVersionResponse(w http.ResponseWriter, version, gitBranch, gitHash, buildTime string)
func WriteCSRFTokenResponse(w http.ResponseWriter, token string)
```

### error.go — 错误响应

```go
// 通用错误响应（自定义 HTTP 状态码与 StatusCode）
func WriteErrorResult(w http.ResponseWriter, httpStatus int, errorMsg string, statusCode commonapis.StatusCode)

// 便捷错误函数（自动设置 HTTP 状态码与 StatusCode）
func WriteBadRequestResult(w http.ResponseWriter, errorMsg string)              // 400 InvalidArgument
func WriteUnauthorizedResult(w http.ResponseWriter, errorMsg string)            // 401 Unauthenticated
func WriteForbiddenResult(w http.ResponseWriter, errorMsg string)               // 403 PermissionDenied
func WriteNotFoundResult(w http.ResponseWriter, errorMsg string)                // 404 NotFound
func WriteTooManyRequestsResult(w http.ResponseWriter, errorMsg string)         // 429 ResourceExhausted
func WriteInternalServerErrorResult(w http.ResponseWriter, errorMsg string)     // 500 Internal
func WriteServiceUnavailableResult(w http.ResponseWriter, errorMsg string)      // 503 Unavailable

// 写入 AppError 响应（使用对象池）
func WriteAppError(w http.ResponseWriter, appErr *errors.AppError)
func WriteAppErrorf(w http.ResponseWriter, code errors.ErrorCode, format string, args ...any)

// 根据 HTTP 状态码选择 StatusCode，输出格式为 "{errorCode}: {message}"
func WriteErrorResponseWithCode(w http.ResponseWriter, statusCode int, errorCode, message string)
```

### health.go — 健康检查响应

```go
// isHealthy 为 true 返回 200/OK，否则返回 503/Unavailable
func WriteHealthCheckResult(w http.ResponseWriter, isHealthy bool, component string, message string, details map[string]any)
```

### types.go — 类型定义

```go
type VersionInfo struct {
    Version   string `json:"version"`
    GitBranch string `json:"git_branch"`
    GitHash   string `json:"git_hash"`
    BuildTime string `json:"build_time"`
}

type CSRFTokenResponse struct {
    CSRFToken string `json:"csrf_token"`
}
```
