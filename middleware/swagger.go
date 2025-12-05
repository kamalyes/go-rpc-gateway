/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 22:15:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 15:52:42
 * @FilePath: \im-share-proto\go-rpc-gateway\middleware\swagger.go
 * @Description: Swagger文档中间件 - 提供API文档在线查看
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	goswagger "github.com/kamalyes/go-config/pkg/swagger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
	"github.com/kamalyes/go-toolbox/pkg/safe"
)

// SwaggerMiddleware Swagger文档中间件 (支持单服务和聚合模式)
// [EN] Swagger documentation middleware (supports single service and aggregation modes)
type SwaggerMiddleware struct {
	config      *goswagger.Swagger
	swaggerJSON []byte

	// 聚合功能相关字段
	aggregatedSpec  map[string]interface{}
	serviceSpecs    map[string]map[string]interface{}
	lastUpdated     time.Time
	httpClient      *http.Client
	refreshInterval time.Duration
}

// NewSwaggerMiddleware 创建Swagger中间件 (支持单服务和聚合模式)
// [EN] Create Swagger middleware (supports single service and aggregation modes)
func NewSwaggerMiddleware(config *goswagger.Swagger) *SwaggerMiddleware {
	middleware := &SwaggerMiddleware{
		config:          config,
		serviceSpecs:    make(map[string]map[string]interface{}),
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		refreshInterval: 5 * time.Minute, // 默认5分钟刷新一次
	}

	// 添加调试信息
	global.LOGGER.Debug("🔧 Swagger配置调试信息:")
	global.LOGGER.Debug("  - Enabled: %v", config.Enabled)
	global.LOGGER.Debug("  - Aggregate != nil: %v", config.Aggregate != nil)
	if config.Aggregate != nil {
		global.LOGGER.Debug("  - Aggregate.Enabled: %v", config.Aggregate.Enabled)
		global.LOGGER.Debug("  - Services count: %d", len(config.Aggregate.Services))
	}
	global.LOGGER.Debug("  - IsAggregateEnabled(): %v", config.IsAggregateEnabled())

	// 根据是否启用聚合模式进行不同的初始化
	if config.IsAggregateEnabled() {
		global.LOGGER.Info("✅ 启用Swagger聚合模式")
		// 立即加载所有服务的规范
		if err := middleware.loadAllServiceSpecs(); err != nil {
			global.LOGGER.Error("❌ 初始化聚合规范失败: %v", err)
		} else {
			global.LOGGER.Info("✅ 聚合规范创建成功")
		}
	} else {
		global.LOGGER.Info("📄 使用单一Swagger模式")
		// 如果未启用聚合，尝试加载Swagger文件
		// [EN] If aggregation is not enabled, try to load Swagger file
		if config.Enabled {
			if err := middleware.loadSwaggerSpec(); err != nil {
				global.LOGGER.Error("加载Swagger文件失败: %v", err)
			}
		}
	}

	return middleware
}

// Handler 返回Swagger处理中间件
// [EN] Return Swagger handler middleware
func (s *SwaggerMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果未启用，跳过
			// [EN] If not enabled, skip
			if !s.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 检查是否是Swagger相关路径
			// [EN] Check if it's Swagger related path
			if s.isSwaggerPath(r.URL.Path) {
				s.handleSwagger(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isSwaggerPath 检查是否是Swagger路径
// [EN] Check if it's Swagger path
func (s *SwaggerMiddleware) isSwaggerPath(path string) bool {
	swaggerPaths := []string{
		s.config.UIPath,
		s.config.UIPath + "/",
		s.config.UIPath + "/index.html",
		s.config.UIPath + "/swagger.json",
	}

	// 添加聚合相关路径
	if s.config.IsAggregateEnabled() {
		aggregatedPaths := []string{
			s.config.UIPath + "/services",
			s.config.UIPath + "/aggregate.json",
			s.config.UIPath + "/debug/services",
		}
		swaggerPaths = append(swaggerPaths, aggregatedPaths...)

		// 支持单个服务路径: /swagger/services/{serviceName}
		if strings.HasPrefix(path, s.config.UIPath+"/services/") {
			return true
		}
	}

	for _, sp := range swaggerPaths {
		if path == sp {
			return true
		}
	}

	return false
}

// handleSwagger 处理Swagger请求
// [EN] Handle Swagger requests
func (s *SwaggerMiddleware) handleSwagger(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 处理聚合相关请求
	if s.config.IsAggregateEnabled() {
		// 聚合JSON
		if strings.HasSuffix(path, "/aggregate.json") {
			s.handleAggregatedJSON(w, r)
			return
		}

		// 单个服务JSON
		if strings.HasPrefix(path, s.config.UIPath+"/services/") && strings.HasSuffix(path, ".json") {
			s.handleServiceJSON(w, r)
			return
		}

		// 单个服务UI
		if strings.HasPrefix(path, s.config.UIPath+"/services/") && !strings.HasSuffix(path, ".json") {
			s.handleServiceUI(w, r)
			return
		}

		// 服务列表
		if strings.HasSuffix(path, "/services") {
			s.handleServicesIndex(w, r)
			return
		}

		// 调试端点：显示所有可用服务名称
		if strings.HasSuffix(path, "/debug/services") {
			s.handleServicesDebug(w, r)
			return
		}

		// 聚合模式下Swagger UI使用聚合JSON
		if strings.HasSuffix(path, "/swagger.json") {
			s.handleAggregatedJSON(w, r)
			return
		}
	} else {
		// 处理swagger.json请求
		// [EN] Handle swagger.json request
		if strings.HasSuffix(path, "/swagger.json") {
			s.handleSwaggerJSON(w, r)
			return
		}
	}

	// 处理Swagger UI请求
	// [EN] Handle Swagger UI request
	if path == s.config.UIPath || path == s.config.UIPath+"/" || strings.HasSuffix(path, "/index.html") {
		s.handleSwaggerUI(w, r)
		return
	}

	// 默认重定向到Swagger UI
	// [EN] Default redirect to Swagger UI
	http.Redirect(w, r, s.config.UIPath+"/", http.StatusTemporaryRedirect)
}

// handleSwaggerUI 处理Swagger UI页面
// [EN] Handle Swagger UI page
func (s *SwaggerMiddleware) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	htmlTemplate := `<!-- HTML for static distribution bundle build -->
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.30.2/swagger-ui.css" />
    <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5.30.2/favicon-32x32.png" sizes="32x32" />
    <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5.30.2/favicon-16x16.png" sizes="16x16" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin: 0;
            background: #fafafa;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.30.2/swagger-ui-bundle.js" charset="UTF-8"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.30.2/swagger-ui-standalone-preset.js" charset="UTF-8"></script>
    <script>
    window.onload = function() {
        //<editor-fold desc="Changeable Configuration Block">
        
        // the following lines will be replaced by docker/configurator, when it runs in a docker-container
        window.ui = SwaggerUIBundle({
            url: '{{.UIPath}}/swagger.json',
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIStandalonePreset
            ],
            plugins: [
                SwaggerUIBundle.plugins.DownloadUrl
            ],
            layout: "StandaloneLayout"
        });

        //</editor-fold>
    };
    </script>
</body>
</html>`

	tmpl := template.Must(template.New("swagger").Parse(htmlTemplate))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := struct {
		Title  string
		UIPath string
	}{
		Title:  s.config.Title,
		UIPath: s.config.UIPath,
	}

	if err := tmpl.Execute(w, data); err != nil {
		global.LOGGER.Error("渲染Swagger UI失败: %v", err)
		writeSwaggerError(w, http.StatusInternalServerError, commonapis.StatusCode_Internal, "Failed to render Swagger UI")
		return
	}
}

// handleSwaggerJSON 处理Swagger JSON请求
// [EN] Handle Swagger JSON request
func (s *SwaggerMiddleware) handleSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if s.swaggerJSON == nil {
		writeSwaggerError(w, http.StatusNotFound, commonapis.StatusCode_NotFound, "Swagger JSON not found")
		return
	}

	w.Write(s.swaggerJSON)
}

// writeSwaggerError 写入Swagger相关错误响应
func writeSwaggerError(w http.ResponseWriter, httpStatus int, statusCode commonapis.StatusCode, message string) {
	result := &commonapis.Result{
		Code:   int32(httpStatus),
		Error:  message,
		Status: statusCode,
	}

	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(httpStatus)

	if err := json.NewEncoder(w).Encode(result); err != nil && global.LOGGER != nil {
		global.LOGGER.WithError(err).ErrorMsg("Failed to encode Swagger error response")
	}
}

// loadSwaggerSpec 加载Swagger规范文件（支持JSON和YAML格式）
// [EN] Load Swagger specification file (supports JSON and YAML formats)
func (s *SwaggerMiddleware) loadSwaggerSpec() error {
	// 优先尝试使用SpecPath（支持自动格式检测）
	if s.config.SpecPath != "" {
		return s.loadSpecFromPath(s.config.SpecPath)
	}

	// 如果有YamlPath，尝试加载YAML文件
	if s.config.YamlPath != "" {
		return s.loadSpecFromPath(s.config.YamlPath)
	}

	// 最后尝试JSONPath
	if s.config.JSONPath != "" {
		return s.loadSpecFromPath(s.config.JSONPath)
	}

	return nil
}

// loadSpecFromPath 从指定路径加载规范文件
func (s *SwaggerMiddleware) loadSpecFromPath(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// 根据文件扩展名判断格式
	ext := strings.ToLower(filepath.Ext(path))
	var swagger map[string]interface{}

	switch ext {
	case ".yaml", ".yml":
		// 解析YAML格式
		if err := yaml.Unmarshal(data, &swagger); err != nil {
			return err
		}
	case ".json":
		// 解析JSON格式
		if err := json.Unmarshal(data, &swagger); err != nil {
			return err
		}
	default:
		// 默认尝试JSON格式
		if err := json.Unmarshal(data, &swagger); err != nil {
			// 如果JSON失败，尝试YAML
			if yamlErr := yaml.Unmarshal(data, &swagger); yamlErr != nil {
				return err // 返回JSON错误
			}
		}
	}

	// 美化JSON输出
	s.swaggerJSON, err = json.MarshalIndent(swagger, "", "  ")
	return err
}

// [EN] Load Swagger JSON file
func (s *SwaggerMiddleware) loadSwaggerJSON() error {
	data, err := os.ReadFile(s.config.JSONPath)
	if err != nil {
		return err
	}

	// 验证JSON格式
	// [EN] Validate JSON format
	var swagger map[string]interface{}
	if err := json.Unmarshal(data, &swagger); err != nil {
		return err
	}

	// 美化JSON输出
	// [EN] Prettify JSON output
	s.swaggerJSON, err = json.MarshalIndent(swagger, "", "  ")
	return err
}

// SetSwaggerJSON 设置Swagger JSON数据
// [EN] Set Swagger JSON data
func (s *SwaggerMiddleware) SetSwaggerJSON(jsonData []byte) error {
	// 验证JSON格式
	// [EN] Validate JSON format
	var swagger map[string]interface{}
	if err := json.Unmarshal(jsonData, &swagger); err != nil {
		return err
	}

	// 美化JSON输出
	// [EN] Prettify JSON output
	var err error
	s.swaggerJSON, err = json.MarshalIndent(swagger, "", "  ")
	return err
}

// ReloadSwaggerJSON 重新加载Swagger文件
// [EN] Reload Swagger file
func (s *SwaggerMiddleware) ReloadSwaggerJSON() error {
	return s.loadSwaggerSpec()
}

// handleAggregatedJSON 处理聚合的Swagger JSON请求
func (s *SwaggerMiddleware) handleAggregatedJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !s.IsAggregateEnabled() {
		writeSwaggerError(w, http.StatusNotFound, commonapis.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	jsonData, err := s.GetAggregatedSpec()
	if err != nil {
		global.LOGGER.Error("获取聚合Swagger规范失败: %v", err)
		writeSwaggerError(w, http.StatusInternalServerError, commonapis.StatusCode_Internal, "获取聚合规范失败")
		return
	}

	w.Write(jsonData)
}

// handleServiceJSON 处理单个服务的Swagger JSON请求
func (s *SwaggerMiddleware) handleServiceJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !s.IsAggregateEnabled() {
		writeSwaggerError(w, http.StatusNotFound, commonapis.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	// 从路径中提取服务名称
	path := r.URL.Path
	serviceName := strings.TrimPrefix(path, s.config.UIPath+"/services/")
	serviceName = strings.TrimSuffix(serviceName, ".json")

	if serviceName == "" {
		writeSwaggerError(w, http.StatusBadRequest, commonapis.StatusCode_InvalidArgument, "服务名称不能为空")
		return
	}

	jsonData, err := s.GetServiceSpec(serviceName)
	if err != nil {
		global.LOGGER.Error("获取服务 %s 的规范失败: %v", serviceName, err)
		writeSwaggerError(w, http.StatusNotFound, commonapis.StatusCode_NotFound, fmt.Sprintf("服务 %s 的规范不存在", serviceName))
		return
	}

	w.Write(jsonData)
}

// handleServiceUI 处理单个服务的Swagger UI请求
func (s *SwaggerMiddleware) handleServiceUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !s.IsAggregateEnabled() {
		http.Error(w, "聚合功能未启用", http.StatusNotFound)
		return
	}

	// 从路径中提取服务名称
	path := r.URL.Path
	serviceName := strings.TrimPrefix(path, s.config.UIPath+"/services/")

	if serviceName == "" {
		http.Error(w, "服务名称不能为空", http.StatusBadRequest)
		return
	}

	// 检查服务是否存在
	_, err := s.GetServiceSpec(serviceName)
	if err != nil {
		http.Error(w, fmt.Sprintf("服务 %s 不存在", serviceName), http.StatusNotFound)
		return
	}

	// 生成单个服务的Swagger UI HTML
	html := s.generateServiceSwaggerUI(serviceName)
	w.Write([]byte(html))
}

// generateServiceSwaggerUI 生成单个服务的Swagger UI HTML页面
func (s *SwaggerMiddleware) generateServiceSwaggerUI(serviceName string) string {
	return fmt.Sprintf(`<!-- HTML for static distribution bundle build -->
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.30.2/swagger-ui.css" />
    <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5.30.2/favicon-32x32.png" sizes="32x32" />
    <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5.30.2/favicon-16x16.png" sizes="16x16" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin: 0;
            background: #fafafa;
        }
        .service-header {
            background: #fff;
            border-bottom: 1px solid #e8e8e8;
            padding: 20px;
            text-align: center;
        }
        .service-header h1 {
            margin: 0 0 10px 0;
            font-size: 1.8em;
            color: #3b4151;
        }
        .service-header p {
            margin: 5px 0 15px 0;
            color: #666;
        }
        .service-header a {
            display: inline-block;
            margin: 0 5px;
            padding: 8px 16px;
            background: #4990e2;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            font-size: 14px;
        }
        .service-header a:hover {
            background: #3b7bbf;
        }
    </style>
</head>
<body>
    <div class="service-header">
        <h1>📚 %s API</h1>
        <p>单独服务的 API 文档</p>
        <a href="%s/services">← 返回服务列表</a>
        <a href="%s">📖 查看聚合文档</a>
    </div>
    
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.30.2/swagger-ui-bundle.js" charset="UTF-8"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.30.2/swagger-ui-standalone-preset.js" charset="UTF-8"></script>
    <script>
    window.onload = function() {
        //<editor-fold desc="Changeable Configuration Block">
        
        // the following lines will be replaced by docker/configurator, when it runs in a docker-container
        window.ui = SwaggerUIBundle({
            url: '%s/services/%s.json',
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIStandalonePreset
            ],
            plugins: [
                SwaggerUIBundle.plugins.DownloadUrl
            ],
            layout: "StandaloneLayout"
        });

        //</editor-fold>
    };
    </script>
</body>
</html>`, serviceName, serviceName, s.config.UIPath, s.config.UIPath, s.config.UIPath, serviceName)
}

// handleServicesIndex 处理服务列表页面
func (s *SwaggerMiddleware) handleServicesIndex(w http.ResponseWriter, _ *http.Request) {
	if !s.IsAggregateEnabled() {
		writeSwaggerError(w, http.StatusNotFound, commonapis.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	// 获取所有服务的信息
	aggregatedSpec, err := s.GetAggregatedSpec()
	if err != nil {
		writeSwaggerError(w, http.StatusInternalServerError, commonapis.StatusCode_Internal, "获取服务列表失败")
		return
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(aggregatedSpec, &spec); err != nil {
		writeSwaggerError(w, http.StatusInternalServerError, commonapis.StatusCode_Internal, "解析服务信息失败")
		return
	}

	// 构建服务列表HTML
	servicesHTML := s.buildServicesHTML(spec)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(servicesHTML))
}

// handleServicesDebug 处理服务调试信息
func (s *SwaggerMiddleware) handleServicesDebug(w http.ResponseWriter, r *http.Request) {
	if !s.IsAggregateEnabled() {
		writeSwaggerError(w, http.StatusNotFound, commonapis.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 构建调试信息
	debugInfo := map[string]interface{}{
		"total_services":      len(s.serviceSpecs),
		"loaded_services":     make([]map[string]interface{}, 0),
		"configured_services": make([]map[string]interface{}, 0),
		"timestamp":           time.Now().Format(time.RFC3339),
	}

	// 加载的服务规范
	for serviceName, _ := range s.serviceSpecs {
		debugInfo["loaded_services"] = append(debugInfo["loaded_services"].([]map[string]interface{}), map[string]interface{}{
			"name": serviceName,
			"url":  fmt.Sprintf("%s/services/%s", s.config.UIPath, serviceName),
		})
	}

	// 配置的服务
	safeAggregate := safe.Safe(s.config.Aggregate)
	if safeAggregate.Field("Enabled").Bool(false) {
		servicesVal := safeAggregate.Field("Services").Value()
		if services, ok := servicesVal.([]*goswagger.ServiceSpec); ok {
			for _, service := range services {
				debugInfo["configured_services"] = append(debugInfo["configured_services"].([]map[string]interface{}), map[string]interface{}{
					"name":      service.Name,
					"enabled":   service.Enabled,
					"spec_path": service.SpecPath,
					"url":       service.URL,
				})
			}
		}
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(debugInfo, "", "  ")
	if err != nil {
		writeSwaggerError(w, http.StatusInternalServerError, commonapis.StatusCode_Internal, "序列化调试信息失败")
		return
	}

	w.Write(jsonData)
}

// buildServicesHTML 构建服务列表HTML页面
func (s *SwaggerMiddleware) buildServicesHTML(aggregatedSpec map[string]interface{}) string {
	var services []map[string]interface{}

	if aggregateInfo, ok := aggregatedSpec["x-aggregate-info"].(map[string]interface{}); ok {
		if servicesList, ok := aggregateInfo["services"].([]interface{}); ok {
			for _, service := range servicesList {
				if serviceMap, ok := service.(map[string]interface{}); ok {
					services = append(services, serviceMap)
				}
			}
		}
	}

	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + s.config.Title + ` - 服务列表</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 1200px; 
            margin: 0 auto; 
            padding: 20px;
            background-color: #f5f5f5;
        }
        .header { 
            text-align: center; 
            margin-bottom: 40px; 
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .services-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
            gap: 20px;
        }
        .service-card {
            background: white;
            padding: 25px;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .service-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 16px rgba(0,0,0,0.15);
        }
        .service-name { 
            font-size: 1.4em; 
            font-weight: 600; 
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .service-desc { 
            color: #666; 
            margin-bottom: 15px;
            line-height: 1.5;
        }
        .service-version {
            display: inline-block;
            background: #e3f2fd;
            color: #1565c0;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 0.85em;
            font-weight: 500;
            margin-bottom: 15px;
        }
        .service-actions {
            display: flex;
            gap: 10px;
        }
        .btn {
            padding: 8px 16px;
            border: none;
            border-radius: 4px;
            text-decoration: none;
            font-size: 0.9em;
            font-weight: 500;
            cursor: pointer;
            transition: background-color 0.2s;
        }
        .btn-primary {
            background-color: #1976d2;
            color: white;
        }
        .btn-primary:hover {
            background-color: #1565c0;
        }
        .btn-secondary {
            background-color: #f5f5f5;
            color: #555;
            border: 1px solid #ddd;
        }
        .btn-secondary:hover {
            background-color: #e0e0e0;
        }
        .aggregate-actions {
            text-align: center;
            margin: 30px 0;
            padding: 20px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .tags {
            margin-top: 10px;
        }
        .tag {
            display: inline-block;
            background: #f0f0f0;
            color: #666;
            padding: 2px 8px;
            border-radius: 10px;
            font-size: 0.75em;
            margin-right: 5px;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>` + s.config.Title + `</h1>
        <p>` + s.config.Description + `</p>
    </div>
    
    <div class="aggregate-actions">
        <h3>聚合文档</h3>
        <p>查看所有服务的聚合API文档</p>
        <a href="` + s.config.UIPath + `" class="btn btn-primary">查看聚合文档</a>
        <a href="` + s.config.UIPath + `/aggregate.json" class="btn btn-secondary">下载聚合JSON</a>
    </div>
    
    <div class="services-grid">`

	for _, service := range services {
		name := getServiceStringField(service, "name")
		description := getServiceStringField(service, "description")
		version := getServiceStringField(service, "version")

		if name == "" {
			continue
		}

		html += `
        <div class="service-card">
            <div class="service-name">` + name + `</div>`

		if description != "" {
			html += `<div class="service-desc">` + description + `</div>`
		}

		if version != "" {
			html += `<div class="service-version">v` + version + `</div>`
		}

		html += `
            <div class="service-actions">
                <a href="` + s.config.UIPath + `/services/` + name + `" class="btn btn-primary">查看文档</a>
                <a href="` + s.config.UIPath + `/services/` + name + `.json" class="btn btn-secondary">下载JSON</a>
            </div>`

		if tags, ok := service["tags"].([]interface{}); ok && len(tags) > 0 {
			html += `<div class="tags">`
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok {
					html += `<span class="tag">` + tagStr + `</span>`
				}
			}
			html += `</div>`
		}

		html += `</div>`
	}

	html += `
    </div>
</body>
</html>`

	return html
}

// ==================== 聚合功能方法 ====================
// 以下方法整合自swagger_aggregator.go

// loadAllServiceSpecs 加载所有服务的Swagger规范
func (s *SwaggerMiddleware) loadAllServiceSpecs() error {
	if s.config.Aggregate == nil || len(s.config.Aggregate.Services) == 0 {
		return fmt.Errorf("没有配置聚合服务")
	}

	global.LOGGER.Info("开始加载所有服务规范，总计 %d 个服务", len(s.config.Aggregate.Services))

	// 用于去重的服务名称集合
	loadedServices := make(map[string]bool)

	for i, service := range s.config.Aggregate.Services {
		global.LOGGER.Info("正在处理第 %d 个服务: %s (enabled: %t, spec_path: %s)",
			i+1, service.Name, service.Enabled, service.SpecPath)

		if !service.Enabled {
			global.LOGGER.Info("跳过已禁用的服务: %s", service.Name)
			continue
		}

		// 检查服务是否已经加载过
		if loadedServices[service.Name] {
			global.LOGGER.Warn("服务 %s 已存在，跳过重复加载", service.Name)
			continue
		}

		var spec map[string]interface{}
		var err error

		// 优先尝试从本地文件加载
		if service.SpecPath != "" {
			global.LOGGER.Info("尝试从文件加载服务 %s 的规范: %s", service.Name, service.SpecPath)
			spec, err = s.loadSpecFromFile(service.SpecPath)
			if err != nil {
				global.LOGGER.Error("从文件加载服务 %s 的规范失败: %v", service.Name, err)
			} else {
				global.LOGGER.Info("成功从文件加载服务 %s 的规范", service.Name)
			}
		}

		// 如果本地文件失败，尝试从远程URL加载
		if spec == nil && service.URL != "" {
			global.LOGGER.Info("尝试从URL加载服务 %s 的规范: %s", service.Name, service.URL)
			spec, err = s.loadSpecFromURL(service.URL)
			if err != nil {
				global.LOGGER.Error("从URL加载服务 %s 的规范失败: %v", service.Name, err)
				continue
			} else {
				global.LOGGER.Info("成功从URL加载服务 %s 的规范", service.Name)
			}
		}

		if spec == nil {
			global.LOGGER.Error("无法加载服务 %s 的规范：文件和URL都失败", service.Name)
			continue
		}

		// 预处理服务规范
		s.preprocessServiceSpec(spec, service)

		// 转换为JSON兼容格式
		convertedSpec, err := s.convertToJSONCompatible(spec)
		if err != nil {
			global.LOGGER.Error("转换服务 %s 的规范为JSON兼容格式失败: %v", service.Name, err)
			continue
		}

		s.serviceSpecs[service.Name] = convertedSpec.(map[string]interface{})
		loadedServices[service.Name] = true // 标记为已加载
		global.LOGGER.Info("✅ 成功加载服务 %s 的规范", service.Name)
	}

	// 执行聚合
	if err := s.aggregateSpecs(); err != nil {
		return fmt.Errorf("聚合规范失败: %v", err)
	}

	s.lastUpdated = time.Now()
	global.LOGGER.Info("✅ 所有服务规范加载完成，共 %d 个服务", len(s.serviceSpecs))
	return nil
}

// loadSpecFromFile 从文件加载Swagger规范
func (s *SwaggerMiddleware) loadSpecFromFile(filePath string) (map[string]interface{}, error) {
	// 如果是相对路径，转换为绝对路径
	if !filepath.IsAbs(filePath) {
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return nil, fmt.Errorf("无法解析文件路径 %s: %v", filePath, err)
		}
		filePath = absPath
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("文件不存在: %s", filePath)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	var spec map[string]interface{}
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &spec)
		if err != nil {
			return nil, fmt.Errorf("YAML解析失败: %v", err)
		}
	case ".json":
		err = json.Unmarshal(data, &spec)
		if err != nil {
			return nil, fmt.Errorf("JSON解析失败: %v", err)
		}
	default:
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
	}

	return spec, nil
}

// loadSpecFromURL 从远程URL加载Swagger规范
func (s *SwaggerMiddleware) loadSpecFromURL(url string) (map[string]interface{}, error) {
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP错误: %d %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var spec map[string]interface{}
	contentType := resp.Header.Get("Content-Type")

	// 根据Content-Type或URL扩展名判断格式
	if strings.Contains(contentType, "yaml") ||
		strings.Contains(contentType, "yml") ||
		strings.HasSuffix(strings.ToLower(url), ".yaml") ||
		strings.HasSuffix(strings.ToLower(url), ".yml") {
		err = yaml.Unmarshal(data, &spec)
		if err != nil {
			return nil, fmt.Errorf("YAML解析失败: %v", err)
		}
	} else {
		err = json.Unmarshal(data, &spec)
		if err != nil {
			return nil, fmt.Errorf("JSON解析失败: %v", err)
		}
	}

	return spec, nil
}

// convertToJSONCompatible 转换YAML加载的数据为JSON兼容格式
// 使用JSON序列化/反序列化的方式强制转换类型
func (s *SwaggerMiddleware) convertToJSONCompatible(input interface{}) (interface{}, error) {
	// 先序列化为JSON
	jsonData, err := json.Marshal(input)
	if err != nil {
		// 如果直接序列化失败，说明有不兼容的类型，需要递归转换
		return s.recursiveConvert(input)
	}

	// 再反序列化为map[string]interface{}
	var result interface{}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		return nil, fmt.Errorf("JSON反序列化失败: %v", err)
	}

	return result, nil
}

// recursiveConvert 递归转换不兼容的类型
func (s *SwaggerMiddleware) recursiveConvert(input interface{}) (interface{}, error) {
	switch v := input.(type) {
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			var keyStr string
			switch k := key.(type) {
			case string:
				keyStr = k
			case int:
				keyStr = fmt.Sprintf("%d", k)
			case int64:
				keyStr = fmt.Sprintf("%d", k)
			case float64:
				keyStr = fmt.Sprintf("%.0f", k)
			default:
				keyStr = fmt.Sprintf("%v", k)
			}

			convertedValue, err := s.recursiveConvert(value)
			if err != nil {
				return nil, err
			}
			result[keyStr] = convertedValue
		}
		return result, nil
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			convertedItem, err := s.recursiveConvert(item)
			if err != nil {
				return nil, err
			}
			result[i] = convertedItem
		}
		return result, nil
	case map[string]interface{}:
		// 已经是正确类型，但需要递归检查值
		result := make(map[string]interface{})
		for key, value := range v {
			convertedValue, err := s.recursiveConvert(value)
			if err != nil {
				return nil, err
			}
			result[key] = convertedValue
		}
		return result, nil
	default:
		// 对于基本类型（string, int, bool等），直接返回
		return v, nil
	}
}

// preprocessServiceSpec 预处理服务规范
func (s *SwaggerMiddleware) preprocessServiceSpec(spec map[string]interface{}, service *goswagger.ServiceSpec) {
	// 更新BasePath
	if service.BasePath != "" {
		s.updatePathsWithBasePath(spec, service.BasePath)
	}

	// 为操作添加服务标签
	s.addServiceTagsToOperations(spec, service)
}

// updatePathsWithBasePath 更新路径的BasePath
func (s *SwaggerMiddleware) updatePathsWithBasePath(spec map[string]interface{}, basePath string) {
	if _, ok := spec["paths"].(map[string]interface{}); ok {
		spec["basePath"] = basePath
		global.LOGGER.Debug("更新服务BasePath: %s", basePath)
	}
}

// addServiceTagsToOperations 为操作添加服务标签
func (s *SwaggerMiddleware) addServiceTagsToOperations(spec map[string]interface{}, service *goswagger.ServiceSpec) {
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		return
	}

	serviceTags := []interface{}{service.Name}
	if len(service.Tags) > 0 {
		for _, tag := range service.Tags {
			serviceTags = append(serviceTags, tag)
		}
	}

	for _, pathData := range paths {
		if pathMap, ok := pathData.(map[string]interface{}); ok {
			for method, operation := range pathMap {
				if opMap, ok := operation.(map[string]interface{}); ok {
					// 获取现有标签
					var existingTags []interface{}
					if tags, exists := opMap["tags"]; exists {
						if tagList, ok := tags.([]interface{}); ok {
							existingTags = tagList
						}
					}

					// 合并标签
					allTags := make([]interface{}, 0)
					allTags = append(allTags, serviceTags...)
					allTags = append(allTags, existingTags...)

					opMap["tags"] = allTags
					global.LOGGER.Debug("为操作 %s 添加服务标签: %v", method, serviceTags)
				}
			}
		}
	}
}

// aggregateSpecs 执行规范聚合
func (s *SwaggerMiddleware) aggregateSpecs() error {
	if len(s.serviceSpecs) == 0 {
		return fmt.Errorf("没有加载的服务规范")
	}

	switch strings.ToLower(s.config.Aggregate.Mode) {
	case "merge":
		return s.mergeAllSpecs()
	case "selector":
		return s.createSelectorSpec()
	default:
		return fmt.Errorf("不支持的聚合模式: %s", s.config.Aggregate.Mode)
	}
}

// mergeAllSpecs 合并所有服务规范
func (s *SwaggerMiddleware) mergeAllSpecs() error {
	s.aggregatedSpec = map[string]interface{}{
		"swagger":          "2.0",
		"info":             s.buildAggregateInfo(),
		"consumes":         []string{"application/json"},
		"produces":         []string{"application/json"},
		"paths":            make(map[string]interface{}),
		"definitions":      make(map[string]interface{}),
		"tags":             make([]interface{}, 0),
		"x-aggregate-info": s.buildServicesInfo(),
	}

	allPaths := s.aggregatedSpec["paths"].(map[string]interface{})
	allDefinitions := s.aggregatedSpec["definitions"].(map[string]interface{})
	allTags := s.aggregatedSpec["tags"].([]interface{})
	tagNames := make(map[string]bool) // 用于去重

	for serviceName, spec := range s.serviceSpecs {
		global.LOGGER.Info("正在合并服务 %s 的规范", serviceName)

		convertedSpec, err := s.convertToJSONCompatible(spec)
		if err != nil {
			return fmt.Errorf("转换服务 %s 规范失败: %v", serviceName, err)
		}

		specMap := convertedSpec.(map[string]interface{})

		// 合并路径 - 改进去重逻辑
		if paths, ok := specMap["paths"].(map[string]interface{}); ok {
			for path, operations := range paths {
				if existingPath, exists := allPaths[path]; exists {
					// 路径已存在，检查是否来自不同服务
					global.LOGGER.Debug("路径 %s 已存在，来自服务 %s，检查操作合并", path, serviceName)
					if existingOps, ok := existingPath.(map[string]interface{}); ok {
						if _, ok := operations.(map[string]interface{}); ok {
							convertedOps, err := s.convertToJSONCompatible(operations)
							if err != nil {
								global.LOGGER.Error("转换路径操作失败: %v", err)
								continue
							}

							shouldMerge := false
							for method, op := range convertedOps.(map[string]interface{}) {
								// 只在操作不存在时才添加，避免重复
								if _, methodExists := existingOps[method]; !methodExists {
									// 清理新操作中的重复标签
									if opMap, ok := op.(map[string]interface{}); ok {
										if tags, exists := opMap["tags"]; exists {
											if tagSlice, ok := tags.([]interface{}); ok {
												// 去重标签
												uniqueTags := make([]interface{}, 0)
												tagSet := make(map[string]bool)

												for _, tag := range tagSlice {
													tagStr := fmt.Sprintf("%v", tag)
													if !tagSet[tagStr] {
														tagSet[tagStr] = true
														uniqueTags = append(uniqueTags, tag)
													}
												}

												// 更新标签
												opMap["tags"] = uniqueTags
												if len(tagSlice) != len(uniqueTags) {
													global.LOGGER.Debug("清理方法 %s 的重复标签，原始: %d，清理后: %d", method, len(tagSlice), len(uniqueTags))
												}
											}
										}
									}
									existingOps[method] = op
									shouldMerge = true
									global.LOGGER.Debug("添加方法 %s 到路径 %s (来自 %s)", method, path, serviceName)
								} else {
									global.LOGGER.Debug("方法 %s 在路径 %s 中已存在，跳过重复添加 (来自 %s)", method, path, serviceName)
								}
							}

							if !shouldMerge {
								global.LOGGER.Warn("服务 %s 的路径 %s 与现有路径完全重复，可能存在配置问题", serviceName, path)
							}
						}
					}
				} else {
					// 新路径，直接添加，但需要清理标签
					cleanedOperations, err := s.convertToJSONCompatible(operations)
					if err != nil {
						global.LOGGER.Error("转换路径操作失败: %v", err)
						continue
					}

					// 清理路径操作中的重复标签
					s.cleanPathOperationTags(cleanedOperations)
					allPaths[path] = cleanedOperations
					global.LOGGER.Debug("添加新路径: %s (来自服务: %s)", path, serviceName)
				}
			}
		} // 合并定义，添加服务前缀避免冲突
		if definitions, ok := specMap["definitions"].(map[string]interface{}); ok {
			for defName, definition := range definitions {
				// 智能前缀处理 - 避免重复前缀
				var finalDefName string
				if s.isCommonType(defName) || strings.HasPrefix(defName, serviceName+"_") {
					finalDefName = defName // 保持原名或已有前缀
				} else {
					finalDefName = fmt.Sprintf("%s_%s", serviceName, defName)
				}

				convertedDef, err := s.convertToJSONCompatible(definition)
				if err != nil {
					global.LOGGER.Error("转换定义失败: %v", err)
					continue
				}

				// 检查是否已存在，避免重复
				if _, exists := allDefinitions[finalDefName]; !exists {
					allDefinitions[finalDefName] = convertedDef
					global.LOGGER.Debug("添加定义: %s -> %s", defName, finalDefName)
				}
			}
		}

		// 直接使用Swagger文件中的原始标签，不进行覆盖
		// 这样可以保持与protobuf定义的完全一致性
		if tags, ok := specMap["tags"].([]interface{}); ok {
			for _, tag := range tags {
				if tagMap, ok := tag.(map[string]interface{}); ok {
					if name, exists := tagMap["name"]; exists {
						nameStr := fmt.Sprintf("%v", name)
						if !tagNames[nameStr] {
							tagNames[nameStr] = true
							allTags = append(allTags, tagMap)
							global.LOGGER.Debug("添加原始Swagger标签: %s (服务: %s)", nameStr, serviceName)
						}
					}
				}
			}
		}
	}

	// 更新聚合规范中的 tags
	s.aggregatedSpec["tags"] = allTags

	// 修复所有引用路径
	if err := s.fixReferences(); err != nil {
		global.LOGGER.Error("修复引用路径失败: %v", err)
	}

	global.LOGGER.Info("✅ 规范合并完成，路径数: %d，定义数: %d",
		len(allPaths), len(allDefinitions))
	return nil
}

// createSelectorSpec 创建选择器规范
func (s *SwaggerMiddleware) createSelectorSpec() error {
	s.aggregatedSpec = map[string]interface{}{
		"swagger":          "2.0",
		"info":             s.buildAggregateInfo(),
		"consumes":         []string{"application/json"},
		"produces":         []string{"application/json"},
		"paths":            make(map[string]interface{}),
		"definitions":      make(map[string]interface{}),
		"x-aggregate-info": s.buildServicesInfo(),
		"x-service-selector": map[string]interface{}{
			"enabled":  true,
			"services": s.buildServicesSummary(),
		},
	}

	global.LOGGER.Info("✅ 选择器规范创建完成")
	return nil
}

// buildAggregateInfo 构建聚合信息
func (s *SwaggerMiddleware) buildAggregateInfo() map[string]interface{} {
	info := map[string]interface{}{
		"title":       s.config.Title,
		"description": s.config.Description,
		"version":     s.config.Version,
	}

	// 只在配置存在时才添加 contact 字段
	if contact := s.buildContactInfo(); contact != nil {
		info["contact"] = contact
	}

	// 只在配置存在时才添加 license 字段
	if license := s.buildLicenseInfo(); license != nil {
		info["license"] = license
	}

	return info
}

// buildContactInfo 构建联系信息
func (s *SwaggerMiddleware) buildContactInfo() interface{} {
	safeContact := safe.Safe(s.config.Contact)
	contact := make(map[string]interface{})

	// 只添加非空字段
	if name := safeContact.Field("Name").String(""); name != "" {
		contact["name"] = name
	}
	if email := safeContact.Field("Email").String(""); email != "" {
		contact["email"] = email
	}
	if url := safeContact.Field("URL").String(""); url != "" {
		contact["url"] = url
	}

	// 如果有任何字段，返回联系信息对象
	if len(contact) > 0 {
		return contact
	}
	return nil
}

// buildLicenseInfo 构建许可证信息
func (s *SwaggerMiddleware) buildLicenseInfo() interface{} {
	safeLicense := safe.Safe(s.config.License)
	license := make(map[string]interface{})

	// 只添加非空字段
	if name := safeLicense.Field("Name").String(""); name != "" {
		license["name"] = name
	}
	if url := safeLicense.Field("URL").String(""); url != "" {
		license["url"] = url
	}

	// 如果有任何字段，返回许可证信息对象
	if len(license) > 0 {
		return license
	}
	return nil
}

// buildServicesSummary 构建服务摘要
func (s *SwaggerMiddleware) buildServicesSummary() []interface{} {
	var services []interface{}
	for _, service := range s.config.Aggregate.Services {
		if service.Enabled {
			serviceInfo := map[string]interface{}{
				"name":        service.Name,
				"description": service.Description,
				"version":     service.Version,
				"tags":        service.Tags,
				"enabled":     service.Enabled,
			}
			services = append(services, serviceInfo)
		}
	}
	return services
}

// buildServicesInfo 构建服务信息
func (s *SwaggerMiddleware) buildServicesInfo() map[string]interface{} {
	return map[string]interface{}{
		"mode":     s.config.Aggregate.Mode,
		"services": s.buildServicesSummary(),
		"updated":  s.lastUpdated.Format(time.RFC3339),
		"count":    len(s.serviceSpecs),
	}
}

// GetAggregatedSpec 获取聚合后的Swagger规范
func (s *SwaggerMiddleware) GetAggregatedSpec() ([]byte, error) {
	if !s.config.IsAggregateEnabled() {
		return nil, fmt.Errorf("聚合模式未启用")
	}

	if s.aggregatedSpec == nil {
		return nil, fmt.Errorf("聚合规范未初始化")
	}

	// 转换为JSON兼容格式
	convertedSpec, err := s.convertToJSONCompatible(s.aggregatedSpec)
	if err != nil {
		return nil, fmt.Errorf("转换聚合规范失败: %v", err)
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(convertedSpec, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化JSON失败: %v", err)
	}

	return jsonData, nil
}

// GetServiceSpec 获取单个服务的规范
func (s *SwaggerMiddleware) GetServiceSpec(serviceName string) ([]byte, error) {
	if !s.config.IsAggregateEnabled() {
		return nil, fmt.Errorf("聚合模式未启用")
	}

	// 尝试直接匹配
	spec, exists := s.serviceSpecs[serviceName]

	// 如果直接匹配失败，尝试更灵活的匹配
	if !exists {
		// 尝试不同的匹配策略
		for actualServiceName, actualSpec := range s.serviceSpecs {
			// 1. 忽略大小写匹配
			if strings.EqualFold(actualServiceName, serviceName) {
				spec = actualSpec
				exists = true
				global.LOGGER.Info("通过忽略大小写匹配找到服务: %s -> %s", serviceName, actualServiceName)
				break
			}

			// 2. 去掉连字符/下划线匹配
			normalizedRequested := strings.ReplaceAll(strings.ToLower(serviceName), "-", "")
			normalizedRequested = strings.ReplaceAll(normalizedRequested, "_", "")
			normalizedActual := strings.ReplaceAll(strings.ToLower(actualServiceName), "-", "")
			normalizedActual = strings.ReplaceAll(normalizedActual, "_", "")

			if normalizedRequested == normalizedActual {
				spec = actualSpec
				exists = true
				global.LOGGER.Info("通过标准化名称匹配找到服务: %s -> %s", serviceName, actualServiceName)
				break
			}

			// 3. 包含匹配（服务名包含请求的名称）
			if strings.Contains(strings.ToLower(actualServiceName), strings.ToLower(serviceName)) {
				spec = actualSpec
				exists = true
				global.LOGGER.Info("通过包含匹配找到服务: %s -> %s", serviceName, actualServiceName)
				break
			}
		}
	}

	if !exists {
		// 记录可用的服务名称以便调试
		var availableServices []string
		for name := range s.serviceSpecs {
			availableServices = append(availableServices, name)
		}
		global.LOGGER.Error("服务 %s 不存在。可用服务: [%s]", serviceName, strings.Join(availableServices, ", "))
		return nil, fmt.Errorf("服务 %s 不存在。可用服务: [%s]", serviceName, strings.Join(availableServices, ", "))
	}

	// 转换为JSON兼容格式
	convertedSpec, err := s.convertToJSONCompatible(spec)
	if err != nil {
		return nil, fmt.Errorf("转换服务规范失败: %v", err)
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(convertedSpec, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化JSON失败: %v", err)
	}

	return jsonData, nil
}

// RefreshSpecs 刷新所有服务规范
func (s *SwaggerMiddleware) RefreshSpecs() error {
	return s.loadAllServiceSpecs()
}

// GetLastUpdated 获取最后更新时间
func (s *SwaggerMiddleware) GetLastUpdated() time.Time {
	return s.lastUpdated
}

// IsAggregateEnabled 检查是否启用聚合模式
func (s *SwaggerMiddleware) IsAggregateEnabled() bool {
	return s.config.IsAggregateEnabled()
}

func getServiceStringField(service map[string]interface{}, field string) string {
	if val, ok := service[field].(string); ok {
		return val
	}
	return ""
}

// fixReferences 修复聚合规范中的所有引用路径
func (s *SwaggerMiddleware) fixReferences() error {
	return s.fixReferencesInObject(s.aggregatedSpec)
}

// fixReferencesInObject 递归修复对象中的引用
func (s *SwaggerMiddleware) fixReferencesInObject(obj interface{}) error {
	switch v := obj.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if key == "$ref" {
				if refStr, ok := value.(string); ok {
					// 修复引用路径
					if newRef := s.fixReference(refStr); newRef != refStr {
						v[key] = newRef
						global.LOGGER.Debug("修复引用: %s -> %s", refStr, newRef)
					}
				}
			} else {
				// 递归处理嵌套对象
				if err := s.fixReferencesInObject(value); err != nil {
					return err
				}
			}
		}
	case []interface{}:
		for _, item := range v {
			if err := s.fixReferencesInObject(item); err != nil {
				return err
			}
		}
	}
	return nil
}

// fixReference 修复单个引用路径
func (s *SwaggerMiddleware) fixReference(ref string) string {
	// 如果引用指向 definitions，检查是否需要添加服务前缀
	if strings.HasPrefix(ref, "#/definitions/") {
		defName := strings.TrimPrefix(ref, "#/definitions/")

		// 检查是否存在带服务前缀的定义
		definitions := s.aggregatedSpec["definitions"].(map[string]interface{})

		// 如果直接引用的定义不存在，尝试查找带前缀的定义
		if _, exists := definitions[defName]; !exists {
			// 查找可能的服务前缀版本
			for actualDefName := range definitions {
				// 检查是否是某个服务的前缀版本
				if strings.HasSuffix(actualDefName, "_"+defName) {
					return "#/definitions/" + actualDefName
				}
			}

			// 如果是常见的通用类型，尝试添加默认前缀
			if s.isCommonType(defName) {
				// 为常见类型添加 commonapis 前缀
				prefixedName := "commonapis_" + defName
				if _, exists := definitions[prefixedName]; exists {
					return "#/definitions/" + prefixedName
				}
			}
		}
	}

	return ref
}

// isCommonType 检查是否是通用类型
func (s *SwaggerMiddleware) isCommonType(typeName string) bool {
	commonTypes := []string{
		"rpcStatus",
		"GeneralEmptyResponse",
		"GeneralEmptyRequest",
		"AgentSettings",
		"settingsAgentSettings",
	}

	for _, commonType := range commonTypes {
		if typeName == commonType || strings.HasSuffix(typeName, commonType) {
			return true
		}
	}

	return false
}

// cleanPathOperationTags 清理路径操作中的重复标签
func (s *SwaggerMiddleware) cleanPathOperationTags(pathOperations interface{}) {
	if operationsMap, ok := pathOperations.(map[string]interface{}); ok {
		for method, operation := range operationsMap {
			if operationMap, ok := operation.(map[string]interface{}); ok {
				if tags, exists := operationMap["tags"]; exists {
					if tagSlice, ok := tags.([]interface{}); ok {
						// 去重标签
						uniqueTags := make([]interface{}, 0)
						tagSet := make(map[string]bool)

						for _, tag := range tagSlice {
							tagStr := fmt.Sprintf("%v", tag)
							if !tagSet[tagStr] {
								tagSet[tagStr] = true
								uniqueTags = append(uniqueTags, tag)
							}
						}

						// 更新标签
						operationMap["tags"] = uniqueTags
						global.LOGGER.Debug("清理方法 %s 的重复标签，原始: %d，清理后: %d", method, len(tagSlice), len(uniqueTags))
					}
				}
			}
		}
	}
}
