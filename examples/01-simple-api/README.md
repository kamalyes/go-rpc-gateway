# Simple API Service Demo

这是一个基础的HTTP API服务示例，展示如何使用 go-rpc-gateway 快速构建RESTful API。

## 功能特性

- ✅ **RESTful API** - 完整的CRUD操作
- ✅ **统一响应格式** - 标准化的JSON响应
- ✅ **错误处理** - 完善的错误处理机制
- ✅ **结构化日志** - 基于go-logger的日志记录
- ✅ **性能分析** - 内置PProf支持
- ✅ **监控指标** - Prometheus指标收集
- ✅ **健康检查** - 服务健康状态监控

## API 端点

### 用户管理

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/users` | 获取用户列表 |
| POST | `/api/users` | 创建新用户 |
| GET | `/api/users/{id}` | 获取指定用户 |
| PUT | `/api/users/{id}` | 更新用户信息 |
| DELETE | `/api/users/{id}` | 删除用户 |

### 系统接口

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/health` | 健康检查 |
| GET | `/api/stats` | 服务统计 |

## 快速开始

### 1. 运行服务

```bash
cd examples/01-simple-api
go run main.go
```

### 2. 测试API

**获取用户列表：**
```bash
curl http://localhost:8080/api/users
```

**创建用户：**
```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"name":"David","email":"david@example.com"}'
```

**获取单个用户：**
```bash
curl http://localhost:8080/api/users/1
```

**更新用户：**
```bash
curl -X PUT http://localhost:8080/api/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice Updated","email":"alice.new@example.com"}'
```

**删除用户：**
```bash
curl -X DELETE http://localhost:8080/api/users/1
```

**健康检查：**
```bash
curl http://localhost:8080/api/health
```

**服务统计：**
```bash
curl http://localhost:8080/api/stats
```

## 响应格式

所有API响应都遵循统一的JSON格式：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    // 具体数据
  }
}
```

## 监控和调试

### 健康检查
```bash
curl http://localhost:8080/health
```

### Prometheus 指标
```bash
curl http://localhost:8080/metrics
```

### 性能分析 (PProf)
```bash
# 查看所有可用的性能分析
curl http://localhost:8080/debug/pprof/

# CPU 性能分析
curl http://localhost:8080/debug/pprof/profile

# 内存分析
curl http://localhost:8080/debug/pprof/heap

# Goroutine 分析
curl http://localhost:8080/debug/pprof/goroutine
```

## 示例响应

### 获取用户列表响应
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "users": [
      {
        "id": 1,
        "name": "Alice",
        "email": "alice@example.com",
        "created_at": "2025-11-11T10:00:00Z"
      }
    ],
    "total": 1
  }
}
```

### 创建用户响应
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 4,
    "name": "David",
    "email": "david@example.com",
    "created_at": "2025-11-12T10:00:00Z"
  }
}
```

### 错误响应
```json
{
  "code": 404,
  "message": "用户不存在"
}
```

## 技术特点

- **轻量级**: 基于 go-rpc-gateway 框架，启动快速
- **生产就绪**: 内置监控、日志、健康检查等生产级特性
- **易扩展**: 模块化设计，易于添加新功能
- **标准化**: 遵循RESTful API设计原则

这个示例展示了如何使用 go-rpc-gateway 快速构建生产级的API服务。