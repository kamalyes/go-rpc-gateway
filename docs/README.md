# 📚 文档目录

欢迎来到 go-rpc-gateway 文档中心！这里包含了框架的完整文档，帮助您快速上手和深入使用。

## 📋 文档导航

### 🚀 快速开始

- [主要文档](../README.md) - 项目概述和快速开始
- [安装指南](../README.md#-快速安装) - 各种安装方式
- [基础使用](../README.md#-快速开始) - 零配置启动示例

### 🏗️ 架构设计

- [架构设计文档](ARCHITECTURE.md) - 详细的系统架构和组件关系
  - 设计理念与原则
  - 整体架构分层
  - 核心组件详解
  - 数据流向分析
  - 扩展机制说明
  - 性能优化指南

### 🛡️ 中间件系统

- [中间件使用指南](MIDDLEWARE_GUIDE.md) - 15+ 内置中间件详细说明
  - 🛡️ 安全类：Security、CORS、Signature
  - 🚦 控制类：RateLimit、Recovery、RequestID  
  - 📊 监控类：Metrics、Logging、Tracing、Health
  - 🌍 体验类：I18n、Access
  - 🔧 开发类：PProf、Banner
  - 自定义中间件开发指南

### 🚀 部署运维

- [部署指南](DEPLOYMENT.md) - 生产环境部署最佳实践
  - 本地开发部署
  - Docker 容器化部署
  - Kubernetes 集群部署
  - 云平台部署 (AWS/GCP)
  - 配置优化与安全实践
  - 监控配置与故障排查

### 🔧 专项功能

- [PProf 使用指南](PPROF_MIDDLEWARE.md) - 性能分析工具
- [国际化配置示例](I18N_CONFIG_EXAMPLE.md) - 多语言支持
- [内置 PProf 使用](BUILTIN_PPROF_USAGE.md) - 内置性能分析

## 🎯 按使用场景导航

### 新手入门

1. 阅读 [主要文档](../README.md) 了解项目概述
2. 按照 [快速安装](../README.md#-快速安装) 安装框架
3. 运行 [零配置启动示例](../README.md#-零配置启动)
4. 查看 [中间件使用指南](MIDDLEWARE_GUIDE.md) 了解内置功能

### 开发阶段

1. 学习 [架构设计文档](ARCHITECTURE.md) 理解系统设计
2. 参考 [中间件使用指南](MIDDLEWARE_GUIDE.md) 配置所需功能
3. 使用 [PProf 指南](PPROF_MIDDLEWARE.md) 进行性能调优
4. 配置 [国际化支持](I18N_CONFIG_EXAMPLE.md) 实现多语言

### 生产部署

1. 阅读 [部署指南](DEPLOYMENT.md) 了解部署选项
2. 选择适合的部署方式（Docker/K8s/云平台）
3. 配置监控和告警系统
4. 参考安全最佳实践加固系统

### 运维监控

1. 配置 [监控指标收集](MIDDLEWARE_GUIDE.md#1-metrics-中间件)
2. 设置 [健康检查](MIDDLEWARE_GUIDE.md#4-health-中间件)
3. 使用 [性能分析工具](BUILTIN_PPROF_USAGE.md)
4. 参考 [故障排查](DEPLOYMENT.md#-故障排查) 解决问题

## 🔗 相关资源

### 官方资源

- [GitHub 仓库](https://github.com/kamalyes/go-rpc-gateway) - 源代码和 Issues
- [发布版本](https://github.com/kamalyes/go-rpc-gateway/releases) - 版本更新记录
- [讨论区](https://github.com/kamalyes/go-rpc-gateway/discussions) - 社区讨论

### 依赖项目

- [go-config](https://github.com/kamalyes/go-config) - 统一配置管理
- [go-sqlbuilder](https://github.com/kamalyes/go-sqlbuilder) - SQL构建器
- [go-wsc](https://github.com/kamalyes/go-wsc) - WebSocket客户端
- [go-logger](https://github.com/kamalyes/go-logger) - 日志组件

### 技术栈文档

- [gRPC](https://grpc.io/docs/) - RPC 框架文档
- [grpc-gateway](https://grpc-ecosystem.github.io/grpc-gateway/) - HTTP/gRPC 网关
- [Prometheus](https://prometheus.io/docs/) - 监控指标
- [OpenTelemetry](https://opentelemetry.io/docs/) - 可观测性

## 🤝 贡献指南

### 文档贡献

我们欢迎对文档的改进和补充：

1. **发现错误** - 提交 [Issue](https://github.com/kamalyes/go-rpc-gateway/issues) 报告文档错误
2. **改进建议** - 在 [讨论区](https://github.com/kamalyes/go-rpc-gateway/discussions) 提出改进建议
3. **直接贡献** - Fork 项目并提交 Pull Request

### 文档规范

- 使用 Markdown 格式编写
- 遵循现有的文档结构和风格
- 包含实际可运行的代码示例
- 添加适当的表情符号和格式化

## 📞 获取帮助

如果您在使用过程中遇到问题：

1. **查阅文档** - 首先查看相关文档章节
2. **搜索 Issues** - 在 GitHub Issues 中搜索类似问题
3. **提交 Issue** - 如果没找到解决方案，创建新的 Issue
4. **参与讨论** - 在 Discussions 中与社区交流

## 📄 许可证

本文档采用 [MIT 许可证](../LICENSE)，与项目代码使用相同许可证。

---

**🎉 感谢使用 go-rpc-gateway！**

如果这个项目对您有帮助，请给我们一个 ⭐️ Star！
