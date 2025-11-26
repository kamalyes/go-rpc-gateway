# API 端点信息聚合工具

这是一个纯工具库，用于收集和管理 API 端点信息，不包含任何业务逻辑。

## 核心功能

- ✅ 端点信息收集和管理
- ✅ 从 Swagger YAML 文件加载端点信息
- ✅ 生成标准格式的 JSON 响应
- ✅ 提供 HTTP 处理器
- ✅ 线程安全操作

## 使用示例

### 1. 基本使用

```go
package main

import (
    "fmt"
    "github.com/kamalyes/go-rpc-gateway/server"
)

func main() {
    // 创建端点收集器
    collector := server.NewEndpointCollector()
    
    // 手动添加端点信息
    endpoint := server.GenerateEndpointInfo(
        "GET", 
        "/v1/users", 
        "获取用户列表.\n[EN] Get user list.",
        "UserService_GetUserList",
        []string{"Users"},
    )
    collector.AddEndpoint(endpoint)
    
    // 获取所有端点
    endpoints := collector.GetAllEndpoints()
    fmt.Printf("收集到 %d 个端点\n", len(endpoints))
}
```

### 2. 从 Swagger 文件加载

```go
func main() {
    collector := server.NewEndpointCollector()
    
    // 从单个 Swagger 文件加载
    err := collector.LoadEndpointsFromSwaggerFile("./proto/user/user_service.swagger.yaml")
    if err != nil {
        log.Printf("加载失败: %v", err)
    }
    
    // 批量加载目录下所有 Swagger 文件
    err = collector.LoadEndpointsFromSwaggerFiles("./proto")
    if err != nil {
        log.Printf("批量加载失败: %v", err)
    }
    
    // 生成 JSON 响应
    jsonData, err := collector.ToJSON()
    if err != nil {
        log.Printf("生成JSON失败: %v", err)
    }
    
    fmt.Println(string(jsonData))
}
```

### 3. 创建 HTTP 接口

```go
import (
    "net/http"
    "github.com/kamalyes/go-rpc-gateway/server"
)

func main() {
    collector := server.NewEndpointCollector()
    
    // 加载端点信息
    collector.LoadEndpointsFromSwaggerFiles("./proto")
    
    // 创建 HTTP 处理器
    handler := collector.CreateHTTPHandler()
    
    // 注册路由
    http.Handle("/_endpoints", handler)
    
    // 启动服务器
    log.Println("Server starting on :8080")
    http.ListenAndServe(":8080", nil)
}
```

### 4. 完整示例

```go
package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    
    "github.com/kamalyes/go-rpc-gateway/server"
)

func main() {
    // 创建收集器
    collector := server.NewEndpointCollector()
    
    // 加载 Swagger 文件
    if err := collector.LoadEndpointsFromSwaggerFiles("./proto"); err != nil {
        log.Printf("加载 Swagger 文件失败: %v", err)
    }
    
    // 手动添加一些端点
    customEndpoints := []server.EndpointInfo{
        server.GenerateEndpointInfo(
            "GET", 
            "/healthz", 
            "健康检查.\n[EN] health check.",
            "Server_Healthz",
            []string{},
        ),
        server.GenerateEndpointInfo(
            "GET", 
            "/version", 
            "版本信息.\n[EN] version information.",
            "Server_Version", 
            []string{},
        ),
    }
    
    for _, endpoint := range customEndpoints {
        collector.AddEndpoint(endpoint)
    }
    
    // 设置路由
    http.HandleFunc("/_endpoints", collector.CreateHTTPHandler())
    
    // 输出统计信息
    endpoints := collector.GetAllEndpoints()
    fmt.Printf("✅ 收集到 %d 个API端点\n", len(endpoints))
    
    // 启动服务器
    fmt.Println("🚀 服务器启动: http://localhost:8080/_endpoints")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## API 方法说明

### EndpointCollector 方法

- `NewEndpointCollector()` - 创建新的端点收集器
- `AddEndpoint(endpoint)` - 添加单个端点信息
- `GetAllEndpoints()` - 获取所有端点信息（已排序）
- `Clear()` - 清空所有端点
- `LoadEndpointsFromSwaggerFile(filePath)` - 从单个 Swagger 文件加载
- `LoadEndpointsFromSwaggerFiles(dir)` - 从目录批量加载 Swagger 文件
- `CollectFromSwagger(swaggerData)` - 从 Swagger 数据对象收集
- `ToJSON()` - 生成 JSON 格式的响应
- `CreateHTTPHandler()` - 创建 HTTP 处理器

### 工具方法

- `GenerateEndpointInfo(method, path, summary, operationID, tags)` - 生成端点信息

## 返回格式

```json
{
    "endpoint_infos": [
        {
            "method": "GET",
            "path": "/v1/users",
            "summary": "获取用户列表.\n[EN] Get user list.",
            "operation_id": "UserService_GetUserList",
            "tags": ["Users"]
        }
    ]
}
```

## 注意事项

1. **无业务逻辑**: 这是纯工具库，不包含任何业务相关的逻辑
2. **用户提供信息**: 所有描述、操作ID等信息需要用户明确提供
3. **线程安全**: 所有操作都是线程安全的
4. **Swagger 优先**: 建议优先使用 Swagger 文件加载，以获得准确的API描述