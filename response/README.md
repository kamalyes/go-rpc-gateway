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
