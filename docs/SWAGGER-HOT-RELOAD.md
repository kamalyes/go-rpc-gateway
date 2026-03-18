# Swagger 文件热重载功能

## 功能说明

Swagger 文件热重载功能允许在开发过程中自动检测 Swagger 文件的变动，并实时重新加载，无需重启服务

## 特性

- ✅ 自动监听 Swagger 文件变动（支持 `.yaml`、`.yml`、`.json` 格式）
- ✅ 支持单服务模式和聚合模式
- ✅ 防抖机制，避免频繁重载
- ✅ 文件写入完成后自动延迟加载
- ✅ 详细的日志输出

## 配置方式

### 1. 单服务模式

```yaml
swagger:
  enabled: true
  hot-reload: true  # 启用文件热重载
  spec-path: "./docs/swagger.yaml"
  ui-path: "/swagger"
  title: "API Documentation"
  description: "API Documentation with Hot Reload"
  version: "1.0.0"
```

### 2. 聚合模式

```yaml
swagger:
  enabled: true
  hot-reload: true  # 启用文件热重载
  ui-path: "/swagger"
  title: "Aggregated API Documentation"
  description: "Multiple Services API Documentation"
  version: "1.0.0"
  aggregate:
    enabled: true
    mode: "merge"
    services:
      - name: "user-service"
        enabled: true
        spec-path: "./docs/user-service.swagger.yaml"
        description: "User Management Service"
        version: "1.0.0"
      - name: "order-service"
        enabled: true
        spec-path: "./docs/order-service.swagger.yaml"
        description: "Order Management Service"
        version: "1.0.0"
```

## 使用方法

### 启动服务

```bash
# 使用配置文件启动
go run main.go --config=config.yaml

# 或者使用环境变量
export SWAGGER_HOT_RELOAD=true
go run main.go
```

### 修改 Swagger 文件

当你修改 Swagger 文件并保存后，服务会自动检测变动并重新加载：

```bash
# 编辑 Swagger 文件
vim docs/swagger.yaml

# 保存后，日志会显示：
# 🔄 检测到 Swagger 文件变动: /path/to/docs/swagger.yaml
# ✅ Swagger 文件已重新加载
```

### 查看效果

刷新浏览器中的 Swagger UI 页面，即可看到最新的 API 文档

## 工作原理

1. **文件监听**：使用 `fsnotify` 库监听配置的 Swagger 文件路径
2. **事件过滤**：只处理文件写入（Write）和创建（Create）事件
3. **防抖处理**：2 秒内的重复事件会被忽略，避免频繁重载
4. **延迟加载**：检测到变动后延迟 100ms 加载，确保文件写入完成
5. **自动重载**：
   - 单服务模式：重新加载单个 Swagger 文件
   - 聚合模式：重新加载所有服务的 Swagger 文件并重新聚合

## 日志输出

### 启动时

```
✅ 开始监听 Swagger 文件: /path/to/docs/swagger.yaml
✅ Swagger 文件监听器已启动，监听 1 个文件
✅ Swagger 文件热重载已启用
```

### 文件变动时

```
🔄 检测到 Swagger 文件变动: /path/to/docs/swagger.yaml
✅ Swagger 文件已重新加载
```

### 错误情况

```
❌ 重新加载 Swagger 失败: 解析YAML失败: yaml: line 10: mapping values are not allowed in this context
```

## 注意事项

1. **文件路径**：确保配置的文件路径正确且文件存在
2. **文件格式**：Swagger 文件必须是有效的 YAML 或 JSON 格式
3. **性能影响**：热重载会占用少量系统资源，建议仅在开发环境启用
4. **生产环境**：生产环境建议关闭热重载（`hot-reload: false`）

## 手动控制

如果需要手动控制热重载，可以通过代码调用：

```go
// 启用热重载
if err := swaggerMiddleware.EnableFileWatcher(); err != nil {
    log.Errorf("启用热重载失败: %v", err)
}

// 停用热重载
if err := swaggerMiddleware.DisableFileWatcher(); err != nil {
    log.Errorf("停用热重载失败: %v", err)
}

// 手动重新加载
if err := swaggerMiddleware.ReloadSwaggerJSON(); err != nil {
    log.Errorf("重新加载失败: %v", err)
}
```

## 故障排查

### 问题 1：文件变动未触发重载

**可能原因**：
- 文件路径配置错误
- 文件不在监听列表中
- 防抖时间内的重复事件被忽略

**解决方法**：
- 检查配置文件中的 `spec-path` 是否正确
- 查看启动日志，确认文件已被监听
- 等待 2 秒后再次修改文件

### 问题 2：重载失败

**可能原因**：
- Swagger 文件格式错误
- 文件权限问题
- 文件正在被其他程序占用

**解决方法**：
- 使用 YAML/JSON 验证工具检查文件格式
- 检查文件读取权限
- 确保文件未被锁定

### 问题 3：监听器未启动

**可能原因**：
- `hot-reload` 配置为 `false`
- `enabled` 配置为 `false`
- 没有配置 Swagger 文件路径

**解决方法**：
- 确认配置文件中 `hot-reload: true`
- 确认配置文件中 `enabled: true`
- 确认配置了 `spec-path` 或服务的 `spec-path`

## 性能优化

1. **防抖时间**：默认 2 秒，可根据需要调整
2. **延迟加载**：默认 100ms，确保文件写入完成
3. **监听范围**：只监听配置的文件，不监听整个目录

## 与其他功能的集成

### 与配置热重载的区别

- **配置热重载**：监听配置文件（如 `config.yaml`）的变动
- **Swagger 热重载**：监听 Swagger 文档文件的变动
- 两者独立工作，互不影响

### 与聚合模式的配合

在聚合模式下，热重载会：
1. 监听所有启用服务的 Swagger 文件
2. 任一文件变动时，重新加载所有服务规范
3. 重新执行聚合逻辑
4. 更新聚合后的 Swagger 文档

## 示例场景

### 场景 1：开发新接口

1. 在 Swagger 文件中添加新接口定义
2. 保存文件
3. 刷新 Swagger UI，立即看到新接口

### 场景 2：修改接口参数

1. 修改 Swagger 文件中的参数定义
2. 保存文件
3. 刷新 Swagger UI，参数变更立即生效

### 场景 3：聚合模式下更新服务

1. 修改某个服务的 Swagger 文件
2. 保存文件
3. 系统自动重新聚合所有服务
4. 刷新 Swagger UI，看到更新后的聚合文档

## 总结

Swagger 文件热重载功能大大提升了 API 文档的开发体验，让你可以实时预览文档变更，无需频繁重启服务建议在开发环境启用此功能，在生产环境关闭以节省资源
