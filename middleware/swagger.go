/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 22:15:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-15 11:15:50
 * @FilePath: \go-rpc-gateway\middleware\swagger.go
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
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	goswagger "github.com/kamalyes/go-config/pkg/swagger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonpb "github.com/kamalyes/go-rpc-gateway/proto"
	"github.com/kamalyes/go-toolbox/pkg/convert"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/safe"
	"github.com/kamalyes/go-toolbox/pkg/stringx"
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
		httpClient:      &http.Client{Timeout: constants.DefaultSwaggerTimeout},
		refreshInterval: constants.DefaultSwaggerRefreshInterval,
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
		s.config.UIPath + constants.SwaggerIndexHTML,
		s.config.UIPath + constants.SwaggerJSONPath,
	}

	// 添加聚合相关路径
	if s.config.IsAggregateEnabled() {
		aggregatedPaths := []string{
			s.config.UIPath + constants.SwaggerServicesPath,
			s.config.UIPath + constants.SwaggerAggregatePath,
			s.config.UIPath + constants.SwaggerDebugPath,
		}
		swaggerPaths = append(swaggerPaths, aggregatedPaths...)

		// 支持单个服务路径: /swagger/services/{serviceName}
		if strings.HasPrefix(path, s.config.UIPath+constants.SwaggerServicesPath+"/") {
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
		if strings.HasSuffix(path, constants.SwaggerAggregatePath) {
			s.handleAggregatedJSON(w, r)
			return
		}

		// 单个服务JSON
		if strings.HasPrefix(path, s.config.UIPath+constants.SwaggerServicesPath+"/") && strings.HasSuffix(path, constants.SwaggerJSONExt) {
			s.handleServiceJSON(w, r)
			return
		}

		// 单个服务UI
		if strings.HasPrefix(path, s.config.UIPath+constants.SwaggerServicesPath+"/") && !strings.HasSuffix(path, constants.SwaggerJSONExt) {
			s.handleServiceUI(w, r)
			return
		}

		// 服务列表
		if strings.HasSuffix(path, constants.SwaggerServicesPath) {
			s.handleServicesIndex(w, r)
			return
		}

		// 调试端点：显示所有可用服务名称
		if strings.HasSuffix(path, constants.SwaggerDebugPath) {
			s.handleServicesDebug(w, r)
			return
		}

		// 聚合模式下Swagger UI使用聚合JSON
		if strings.HasSuffix(path, constants.SwaggerJSONPath) {
			s.handleAggregatedJSON(w, r)
			return
		}
	} else {
		// 处理swagger.json请求
		// [EN] Handle swagger.json request
		if strings.HasSuffix(path, constants.SwaggerJSONPath) {
			s.handleSwaggerJSON(w, r)
			return
		}
	}

	// 处理Swagger UI请求
	// [EN] Handle Swagger UI request
	if path == s.config.UIPath || path == s.config.UIPath+"/" || strings.HasSuffix(path, constants.SwaggerIndexHTML) {
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
<html lang="` + constants.SwaggerHTMLLangEN + `">
<head>
    <meta charset="` + constants.SwaggerHTMLCharset + `">
    <title>{{.Title}}</title>
    <link rel="stylesheet" type="text/css" href="{{.CSSURL}}" />
    <link rel="icon" type="image/png" href="{{.Favicon32}}" sizes="` + constants.HTMLIconSizes32 + `" />
    <link rel="icon" type="image/png" href="{{.Favicon16}}" sizes="` + constants.HTMLIconSizes16 + `" />
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
    <div id="` + constants.SwaggerUIDomID[1:] + `"></div>
    <script src="{{.BundleJS}}" charset="` + constants.SwaggerHTMLCharset + `"></script>
    <script src="{{.PresetJS}}" charset="` + constants.SwaggerHTMLCharset + `"></script>
    <script>
    window.onload = function() {
        //<editor-fold desc="Changeable Configuration Block">
        
        // the following lines will be replaced by docker/configurator, when it runs in a docker-container
        window.ui = SwaggerUIBundle({
            url: '{{.UIPath}}/swagger.json',
            dom_id: '` + constants.SwaggerUIDomID + `',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIStandalonePreset
            ],
            plugins: [
                SwaggerUIBundle.plugins.DownloadUrl
            ],
            layout: "` + constants.SwaggerUILayout + `"
        });

        //</editor-fold>
    };
    </script>
</body>
</html>`

	tmpl := template.Must(template.New(constants.SwaggerUITemplateName).Parse(htmlTemplate))
	w.Header().Set(constants.HeaderContentType, constants.MimeTextHTMLCharset)

	data := struct {
		Title     string
		UIPath    string
		CSSURL    string
		Favicon32 string
		Favicon16 string
		BundleJS  string
		PresetJS  string
	}{
		Title:     s.config.Title,
		UIPath:    s.config.UIPath,
		CSSURL:    s.config.GetCDNCSSURL(),
		Favicon32: s.config.GetCDNFavicon32(),
		Favicon16: s.config.GetCDNFavicon16(),
		BundleJS:  s.config.GetCDNBundleJS(),
		PresetJS:  s.config.GetCDNPresetJS(),
	}

	if err := tmpl.Execute(w, data); err != nil {
		global.LOGGER.Error("渲染Swagger UI失败: %v", err)
		writeSwaggerError(w, http.StatusInternalServerError, commonpb.StatusCode_Internal, "Failed to render Swagger UI")
		return
	}
}

// handleSwaggerJSON 处理Swagger JSON请求
// [EN] Handle Swagger JSON request
func (s *SwaggerMiddleware) handleSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSONCharset)
	w.Header().Set(constants.HeaderAccessControlAllowOrigin, constants.CORSAllowAll)
	w.Header().Set(constants.HeaderAccessControlAllowMethods, constants.CORSDefaultMethods)
	w.Header().Set(constants.HeaderAccessControlAllowHeaders, constants.CORSDefaultHeaders)

	if r.Method == constants.HTTPMethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if s.swaggerJSON == nil {
		writeSwaggerError(w, http.StatusNotFound, commonpb.StatusCode_NotFound, "Swagger JSON not found")
		return
	}

	w.Write(s.swaggerJSON)
}

// writeSwaggerError 写入Swagger相关错误响应
func writeSwaggerError(w http.ResponseWriter, httpStatus int, statusCode commonpb.StatusCode, message string) {
	result := &commonpb.Result{
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
	case constants.FileExtYAML, constants.FileExtYML:
		// 解析YAML格式
		if err := yaml.Unmarshal(data, &swagger); err != nil {
			return err
		}
	case constants.FileExtJSON:
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
	s.swaggerJSON, err = json.MarshalIndent(swagger, constants.JSONIndentPrefix, constants.JSONIndentValue)
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
	s.swaggerJSON, err = json.MarshalIndent(swagger, constants.JSONIndentPrefix, constants.JSONIndentValue)
	return err
}

// ReloadSwaggerJSON 重新加载Swagger文件
// [EN] Reload Swagger file
func (s *SwaggerMiddleware) ReloadSwaggerJSON() error {
	return s.loadSwaggerSpec()
}

// handleAggregatedJSON 处理聚合的Swagger JSON请求
func (s *SwaggerMiddleware) handleAggregatedJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSONCharset)
	w.Header().Set(constants.HeaderAccessControlAllowOrigin, constants.CORSAllowAll)
	w.Header().Set(constants.HeaderAccessControlAllowMethods, constants.CORSDefaultMethods)
	w.Header().Set(constants.HeaderAccessControlAllowHeaders, constants.CORSDefaultHeaders)

	if r.Method == constants.HTTPMethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !s.IsAggregateEnabled() {
		writeSwaggerError(w, http.StatusNotFound, commonpb.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	jsonData, err := s.GetAggregatedSpec()
	if err != nil {
		global.LOGGER.Error("获取聚合Swagger规范失败: %v", err)
		writeSwaggerError(w, http.StatusInternalServerError, commonpb.StatusCode_Internal, "获取聚合规范失败")
		return
	}

	w.Write(jsonData)
}

// handleServiceJSON 处理单个服务的Swagger JSON请求
func (s *SwaggerMiddleware) handleServiceJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSONCharset)
	w.Header().Set(constants.HeaderAccessControlAllowOrigin, constants.CORSAllowAll)
	w.Header().Set(constants.HeaderAccessControlAllowMethods, constants.CORSDefaultMethods)
	w.Header().Set(constants.HeaderAccessControlAllowHeaders, constants.CORSDefaultHeaders)

	if r.Method == constants.HTTPMethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !s.IsAggregateEnabled() {
		writeSwaggerError(w, http.StatusNotFound, commonpb.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	// 从路径中提取服务名称
	path := r.URL.Path
	serviceName := strings.TrimPrefix(path, s.config.UIPath+constants.SwaggerPathServicePrefix)
	serviceName = strings.TrimSuffix(serviceName, constants.SwaggerJSONExt)

	if serviceName == "" {
		writeSwaggerError(w, http.StatusBadRequest, commonpb.StatusCode_InvalidArgument, "服务名称不能为空")
		return
	}

	jsonData, err := s.GetServiceSpec(serviceName)
	if err != nil {
		global.LOGGER.Error("获取服务 %s 的规范失败: %v", serviceName, err)
		writeSwaggerError(w, http.StatusNotFound, commonpb.StatusCode_NotFound, fmt.Sprintf("服务 %s 的规范不存在", serviceName))
		return
	}

	w.Write(jsonData)
}

// handleServiceUI 处理单个服务的Swagger UI请求
func (s *SwaggerMiddleware) handleServiceUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(constants.HeaderContentType, constants.MimeTextHTMLCharset)
	w.Header().Set(constants.HeaderAccessControlAllowOrigin, constants.CORSAllowAll)

	if r.Method == constants.HTTPMethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !s.IsAggregateEnabled() {
		http.Error(w, "聚合功能未启用", http.StatusNotFound)
		return
	}

	// 从路径中提取服务名称
	path := r.URL.Path
	serviceName := strings.TrimPrefix(path, s.config.UIPath+constants.SwaggerPathServicePrefix)

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
<html lang="`+constants.SwaggerHTMLLangEN+`">
<head>
    <meta charset="`+constants.SwaggerHTMLCharset+`">
    <meta name="viewport" content="`+constants.HTMLMetaViewport+`">
    <title>%s - API Documentation</title>
    <link rel="stylesheet" type="text/css" href="`+s.config.GetCDNCSSURL()+`" />
    <link rel="icon" type="image/png" href="`+s.config.GetCDNFavicon32()+`" sizes="`+constants.HTMLIconSizes32+`" />
    <link rel="icon" type="image/png" href="`+s.config.GetCDNFavicon16()+`" sizes="`+constants.HTMLIconSizes16+`" />
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
    
    <div id="`+constants.SwaggerUIDomID[1:]+`"></div>
    <script src="`+s.config.GetCDNBundleJS()+`" charset="`+constants.SwaggerHTMLCharset+`"></script>
    <script src="`+s.config.GetCDNPresetJS()+`" charset="`+constants.SwaggerHTMLCharset+`"></script>
    <script>
    window.onload = function() {
        //<editor-fold desc="Changeable Configuration Block">
        
        // the following lines will be replaced by docker/configurator, when it runs in a docker-container
        window.ui = SwaggerUIBundle({
            url: '%s/services/%s.json',
            dom_id: '`+constants.SwaggerUIDomID+`',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIStandalonePreset
            ],
            plugins: [
                SwaggerUIBundle.plugins.DownloadUrl
            ],
            layout: "`+constants.SwaggerUILayout+`"
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
		writeSwaggerError(w, http.StatusNotFound, commonpb.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	// 获取所有服务的信息
	aggregatedSpec, err := s.GetAggregatedSpec()
	if err != nil {
		writeSwaggerError(w, http.StatusInternalServerError, commonpb.StatusCode_Internal, "获取服务列表失败")
		return
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(aggregatedSpec, &spec); err != nil {
		writeSwaggerError(w, http.StatusInternalServerError, commonpb.StatusCode_Internal, "解析服务信息失败")
		return
	}

	// 构建服务列表HTML
	servicesHTML := s.buildServicesHTML(spec)

	w.Header().Set(constants.HeaderContentType, constants.MimeTextHTMLCharset)
	w.Write([]byte(servicesHTML))
}

// handleServicesDebug 处理服务调试信息
func (s *SwaggerMiddleware) handleServicesDebug(w http.ResponseWriter, r *http.Request) {
	if !s.IsAggregateEnabled() {
		writeSwaggerError(w, http.StatusNotFound, commonpb.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSONCharset)
	w.Header().Set(constants.HeaderAccessControlAllowOrigin, constants.CORSAllowAll)

	// 构建调试信息
	debugInfo := map[string]interface{}{
		constants.SwaggerFieldTotalServices:      len(s.serviceSpecs),
		constants.SwaggerFieldLoadedServices:     make([]map[string]interface{}, 0),
		constants.SwaggerFieldConfiguredServices: make([]map[string]interface{}, 0),
		constants.SwaggerFieldTimestamp:          time.Now().Format(time.RFC3339),
	}

	// 加载的服务规范
	for serviceName, _ := range s.serviceSpecs {
		debugInfo[constants.SwaggerFieldLoadedServices] = append(debugInfo[constants.SwaggerFieldLoadedServices].([]map[string]interface{}), map[string]interface{}{
			constants.SwaggerFieldName: serviceName,
			constants.SwaggerFieldURL:  fmt.Sprintf("%s/services/%s", s.config.UIPath, serviceName),
		})
	}

	// 配置的服务
	safeAggregate := safe.Safe(s.config.Aggregate)
	if safeAggregate.Field("Enabled").Bool(false) {
		servicesVal := safeAggregate.Field("Services").Value()
		if services, ok := servicesVal.([]*goswagger.ServiceSpec); ok {
			for _, service := range services {
				debugInfo[constants.SwaggerFieldConfiguredServices] = append(debugInfo[constants.SwaggerFieldConfiguredServices].([]map[string]interface{}), map[string]interface{}{
					constants.SwaggerFieldName:     service.Name,
					constants.SwaggerFieldEnabled:  service.Enabled,
					constants.SwaggerFieldSpecPath: service.SpecPath,
					constants.SwaggerFieldURL:      service.URL,
				})
			}
		}
	}

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(debugInfo, constants.JSONIndentPrefix, constants.JSONIndentValue)
	if err != nil {
		writeSwaggerError(w, http.StatusInternalServerError, commonpb.StatusCode_Internal, "序列化调试信息失败")
		return
	}

	w.Write(jsonData)
}

// buildServicesHTML 构建服务列表HTML页面
func (s *SwaggerMiddleware) buildServicesHTML(aggregatedSpec map[string]interface{}) string {
	var services []map[string]interface{}

	if aggregateInfo, ok := aggregatedSpec[constants.SwaggerFieldXAggregateInfo].(map[string]interface{}); ok {
		if servicesList, ok := aggregateInfo[constants.SwaggerFieldServices].([]interface{}); ok {
			for _, service := range servicesList {
				if serviceMap, ok := service.(map[string]interface{}); ok {
					services = append(services, serviceMap)
				}
			}
		}
	}

	html := `<!DOCTYPE html>
<html lang="` + constants.SwaggerHTMLLangZH + `">
<head>
    <meta charset="` + constants.SwaggerHTMLCharset + `">
    <meta name="viewport" content="` + constants.HTMLMetaViewport + `">
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
		name := getServiceStringField(service, constants.SwaggerFieldName)
		description := getServiceStringField(service, constants.SwaggerFieldDescription)
		version := getServiceStringField(service, constants.SwaggerFieldVersion)

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

		if tags, ok := service[constants.SwaggerFieldTags].([]interface{}); ok && len(tags) > 0 {
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
// loadAllServiceSpecs 加载所有服务的Swagger规范
func (s *SwaggerMiddleware) loadAllServiceSpecs() error {
	if s.config.Aggregate == nil || len(s.config.Aggregate.Services) == 0 {
		return fmt.Errorf("没有配置聚合服务")
	}

	global.LOGGER.Info("开始加载所有服务规范，总计 %d 个服务", len(s.config.Aggregate.Services))

	loadedServices := make(map[string]bool)

	for i, service := range s.config.Aggregate.Services {
		s.loadSingleService(i, service, loadedServices)
	}

	if err := s.aggregateSpecs(); err != nil {
		return fmt.Errorf("聚合规范失败: %v", err)
	}

	s.lastUpdated = time.Now()
	global.LOGGER.Info("✅ 所有服务规范加载完成，共 %d 个服务", len(s.serviceSpecs))
	return nil
}

// loadSingleService 加载单个服务规范
func (s *SwaggerMiddleware) loadSingleService(index int, service *goswagger.ServiceSpec, loadedServices map[string]bool) {
	global.LOGGER.Info("正在处理第 %d 个服务: %s (enabled: %t, spec_path: %s)",
		index+1, service.Name, service.Enabled, service.SpecPath)

	if !service.Enabled {
		global.LOGGER.Info("跳过已禁用的服务: %s", service.Name)
		return
	}

	if loadedServices[service.Name] {
		global.LOGGER.Warn("服务 %s 已存在，跳过重复加载", service.Name)
		return
	}

	spec := s.loadServiceSpec(service)
	if spec == nil {
		return
	}

	if err := s.processAndStoreSpec(service, spec); err != nil {
		global.LOGGER.Error("处理服务 %s 的规范失败: %v", service.Name, err)
		return
	}

	loadedServices[service.Name] = true
	global.LOGGER.Info("✅ 成功加载服务 %s 的规范", service.Name)
}

// loadServiceSpec 加载服务规范（尝试文件和URL）
func (s *SwaggerMiddleware) loadServiceSpec(service *goswagger.ServiceSpec) map[string]interface{} {
	// 尝试从文件加载
	if spec := s.tryLoadFromFile(service); spec != nil {
		return spec
	}

	// 尝试从URL加载
	if spec := s.tryLoadFromURL(service); spec != nil {
		return spec
	}

	global.LOGGER.Error("无法加载服务 %s 的规范：文件和URL都失败", service.Name)
	return nil
}

// tryLoadFromFile 尝试从文件加载服务规范
func (s *SwaggerMiddleware) tryLoadFromFile(service *goswagger.ServiceSpec) map[string]interface{} {
	if service.SpecPath == "" {
		return nil
	}

	global.LOGGER.Info("尝试从文件加载服务 %s 的规范: %s", service.Name, service.SpecPath)
	spec, err := s.loadSpecFromFile(service.SpecPath)
	if err != nil {
		global.LOGGER.Error("从文件加载服务 %s 的规范失败: %v", service.Name, err)
		return nil
	}

	global.LOGGER.Info("成功从文件加载服务 %s 的规范", service.Name)
	return spec
}

// tryLoadFromURL 尝试从URL加载服务规范
func (s *SwaggerMiddleware) tryLoadFromURL(service *goswagger.ServiceSpec) map[string]interface{} {
	if service.URL == "" {
		return nil
	}

	global.LOGGER.Info("尝试从URL加载服务 %s 的规范: %s", service.Name, service.URL)
	spec, err := s.loadSpecFromURL(service.URL)
	if err != nil {
		global.LOGGER.Error("从URL加载服务 %s 的规范失败: %v", service.Name, err)
		return nil
	}

	global.LOGGER.Info("成功从URL加载服务 %s 的规范", service.Name)
	return spec
}

// processAndStoreSpec 处理并存储服务规范
func (s *SwaggerMiddleware) processAndStoreSpec(service *goswagger.ServiceSpec, spec map[string]interface{}) error {
	// 预处理服务规范
	s.preprocessServiceSpec(spec, service)

	// 使用mathx.ConvertMapKeysToString确保所有键都是字符串
	convertedSpec := mathx.ConvertMapKeysToString(spec)
	if convertedMap, ok := convertedSpec.(map[string]interface{}); ok {
		s.serviceSpecs[service.Name] = convertedMap
	} else {
		return fmt.Errorf("转换服务规范失败: 无法转换为map[string]interface{}")
	}
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
	case constants.FileExtYAML, constants.FileExtYML:
		err = yaml.Unmarshal(data, &spec)
		if err != nil {
			return nil, fmt.Errorf("YAML解析失败: %v", err)
		}
	case constants.FileExtJSON:
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
	contentType := resp.Header.Get(constants.HeaderContentType)

	// 根据Content-Type或URL扩展名判断格式
	if stringx.ContainsAny(contentType, []string{constants.MimeYAML, constants.MimeYML}) ||
		stringx.EndWithAnyIgnoreCase(url, []string{constants.FileExtYAML, constants.FileExtYML}) {
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
	if _, ok := spec[constants.SwaggerFieldPaths].(map[string]interface{}); ok {
		spec[constants.SwaggerFieldBasePath] = basePath
		global.LOGGER.Debug("更新服务BasePath: %s", basePath)
	}
}

// addServiceTagsToOperations 为操作添加服务标签
// 只在配置了额外标签时才添加，否则保持原始标签不变
func (s *SwaggerMiddleware) addServiceTagsToOperations(spec map[string]interface{}, service *goswagger.ServiceSpec) {
	if len(service.Tags) == 0 {
		global.LOGGER.Debug("服务 %s 未配置额外标签，保持原始标签", service.Name)
		return
	}

	paths, ok := spec[constants.SwaggerFieldPaths].(map[string]interface{})
	if !ok {
		return
	}

	serviceTags := s.buildServiceTags(service)
	s.processAllPaths(paths, serviceTags)
}

// buildServiceTags 构建服务标签列表
func (s *SwaggerMiddleware) buildServiceTags(service *goswagger.ServiceSpec) []interface{} {
	if len(service.Tags) == 0 {
		return nil
	}
	serviceTags := make([]interface{}, len(service.Tags))
	for i, tag := range service.Tags {
		serviceTags[i] = tag
	}
	return serviceTags
}

// processAllPaths 处理所有路径，添加标签
func (s *SwaggerMiddleware) processAllPaths(paths map[string]interface{}, serviceTags []interface{}) {
	for pathName, pathData := range paths {
		pathMap, ok := pathData.(map[string]interface{})
		if !ok {
			continue
		}
		s.processPathMethods(pathName, pathMap, serviceTags)
	}
}

// processPathMethods 处理单个路径下的所有HTTP方法
func (s *SwaggerMiddleware) processPathMethods(pathName string, pathMap map[string]interface{}, serviceTags []interface{}) {
	for method, operation := range pathMap {
		opMap, ok := operation.(map[string]interface{})
		if !ok {
			continue
		}
		s.mergeOperationTags(pathName, method, opMap, serviceTags)
	}
}

// mergeOperationTags 合并操作的标签
func (s *SwaggerMiddleware) mergeOperationTags(pathName, method string, opMap map[string]interface{}, serviceTags []interface{}) {
	existingTags := s.extractExistingTags(opMap)
	mergedTags := s.mergeOperationTagsLists(existingTags, serviceTags)

	opMap[constants.SwaggerFieldTags] = mergedTags
	global.LOGGER.Debug("路径 %s %s: 原始标签%v + 额外标签%v → 最终%v",
		method, pathName, existingTags, serviceTags, mergedTags)
}

// extractExistingTags 提取现有标签
func (s *SwaggerMiddleware) extractExistingTags(opMap map[string]interface{}) []interface{} {
	if tags, exists := opMap[constants.SwaggerFieldTags]; exists {
		if tagList, ok := tags.([]interface{}); ok {
			return tagList
		}
	}
	return nil
}

// mergeOperationTagsLists 合并两个操作标签列表并去重
func (s *SwaggerMiddleware) mergeOperationTagsLists(existingTags, newTags []interface{}) []interface{} {
	if len(existingTags) == 0 {
		return newTags
	}
	if len(newTags) == 0 {
		return existingTags
	}

	// 合并标签
	allTags := make([]interface{}, 0, len(existingTags)+len(newTags))
	allTags = append(allTags, existingTags...)
	allTags = append(allTags, newTags...)

	// 使用map去重，保持顺序
	seen := make(map[string]bool, len(allTags))
	result := make([]interface{}, 0, len(allTags))

	for _, tag := range allTags {
		tagStr := convert.MustString(tag)
		if tagStr != "" && !seen[tagStr] {
			seen[tagStr] = true
			result = append(result, tag)
		}
	}

	return result
}

// aggregateSpecs 执行规范聚合
func (s *SwaggerMiddleware) aggregateSpecs() error {
	if len(s.serviceSpecs) == 0 {
		return fmt.Errorf("没有加载的服务规范")
	}

	switch strings.ToLower(s.config.Aggregate.Mode) {
	case constants.SwaggerAggregateModeMerge:
		return s.mergeAllSpecs()
	case constants.SwaggerAggregateModeSelector:
		return s.createSelectorSpec()
	default:
		return fmt.Errorf("不支持的聚合模式: %s", s.config.Aggregate.Mode)
	}
}

// mergeAllSpecs 合并所有服务规范
func (s *SwaggerMiddleware) mergeAllSpecs() error {
	s.initializeAggregatedSpec()

	// 按服务名排序，确保每次执行顺序一致
	serviceNames := s.getSortedServiceNames()

	allPaths := s.aggregatedSpec[constants.SwaggerFieldPaths].(map[string]interface{})
	allDefinitions := s.aggregatedSpec[constants.SwaggerFieldDefs].(map[string]interface{})
	allTags := s.aggregatedSpec[constants.SwaggerFieldTags].([]interface{})
	tagNames := make(map[string]bool)

	for _, serviceName := range serviceNames {
		if err := s.mergeServiceSpec(serviceName, allPaths, allDefinitions, &allTags, tagNames); err != nil {
			return err
		}
	}

	// 更新聚合规范中的 tags
	s.aggregatedSpec[constants.SwaggerFieldTags] = allTags

	// 修复所有引用路径
	if err := s.fixReferences(); err != nil {
		global.LOGGER.Error("修复引用路径失败: %v", err)
	}

	global.LOGGER.Info("✅ 规范合并完成，路径数: %d，定义数: %d", len(allPaths), len(allDefinitions))
	return nil
}

// initializeAggregatedSpec 初始化聚合规范
func (s *SwaggerMiddleware) initializeAggregatedSpec() {
	s.aggregatedSpec = map[string]interface{}{
		constants.SwaggerFieldSwagger:        constants.SwaggerVersion,
		constants.SwaggerFieldInfo:           s.buildAggregateInfo(),
		constants.SwaggerFieldConsumes:       []string{constants.MimeApplicationJSON},
		constants.SwaggerFieldProduces:       []string{constants.MimeApplicationJSON},
		constants.SwaggerFieldPaths:          make(map[string]interface{}),
		constants.SwaggerFieldDefs:           make(map[string]interface{}),
		constants.SwaggerFieldTags:           make([]interface{}, 0),
		constants.SwaggerFieldXAggregateInfo: s.buildServicesInfo(),
	}
}

// getSortedServiceNames 获取排序后的服务名列表
func (s *SwaggerMiddleware) getSortedServiceNames() []string {
	serviceNames := make([]string, 0, len(s.serviceSpecs))
	for name := range s.serviceSpecs {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)
	return serviceNames
}

// mergeServiceSpec 合并单个服务的规范
func (s *SwaggerMiddleware) mergeServiceSpec(serviceName string, allPaths, allDefinitions map[string]interface{}, allTags *[]interface{}, tagNames map[string]bool) error {
	spec := s.serviceSpecs[serviceName]
	global.LOGGER.Info("正在合并服务 %s 的规范", serviceName)

	// 使用mathx.ConvertMapKeysToString确保键为字符串
	convertedSpec := mathx.ConvertMapKeysToString(spec)
	specMap, ok := convertedSpec.(map[string]interface{})
	if !ok {
		return fmt.Errorf("转换服务 %s 规范失败: 无法转换为map[string]interface{}", serviceName)
	}

	// 合并路径
	s.mergePaths(specMap, serviceName, allPaths)

	// 合并定义
	s.mergeDefinitions(specMap, serviceName, allDefinitions)

	// 合并标签
	s.mergeServiceSpecTags(specMap, serviceName, allTags, tagNames)

	return nil
}

// mergePaths 合并路径
func (s *SwaggerMiddleware) mergePaths(specMap map[string]interface{}, serviceName string, allPaths map[string]interface{}) {
	paths, ok := specMap[constants.SwaggerFieldPaths].(map[string]interface{})
	if !ok {
		return
	}

	for path, operations := range paths {
		// 使用mathx.ConvertMapKeysToString确保操作的键为字符串
		convertedOps := mathx.ConvertMapKeysToString(operations)
		newOps, ok := convertedOps.(map[string]interface{})
		if !ok {
			global.LOGGER.Error("路径 %s 的操作格式不正确", path)
			continue
		}

		s.mergePathOperations(path, newOps, serviceName, allPaths)
	}
}

// mergePathOperations 合并单个路径的操作
func (s *SwaggerMiddleware) mergePathOperations(path string, newOps map[string]interface{}, serviceName string, allPaths map[string]interface{}) {
	existingPath, exists := allPaths[path]
	if !exists {
		allPaths[path] = newOps
		global.LOGGER.Debug("✅ 添加新路径: %s (来自: %s)", path, serviceName)
		return
	}

	existingOps, ok := existingPath.(map[string]interface{})
	if !ok {
		global.LOGGER.Error("现有路径 %s 的操作格式不正确", path)
		return
	}

	mergedAny := false
	for method, op := range newOps {
		if method == "parameters" || method == "$ref" {
			continue
		}

		if _, methodExists := existingOps[method]; methodExists {
			global.LOGGER.Warn("⚠️  路径 %s 的方法 %s 在多个服务中重复定义 (当前: %s)，保留首次加载的定义", path, method, serviceName)
		} else {
			existingOps[method] = op
			mergedAny = true
			global.LOGGER.Debug("✅ 添加方法 %s 到路径 %s (来自: %s)", method, path, serviceName)
		}
	}

	if !mergedAny {
		global.LOGGER.Debug("路径 %s 的所有方法已存在，无需合并 (来自: %s)", path, serviceName)
	}
}

// mergeDefinitions 合并定义
func (s *SwaggerMiddleware) mergeDefinitions(specMap map[string]interface{}, serviceName string, allDefinitions map[string]interface{}) {
	definitions, ok := specMap[constants.SwaggerFieldDefs].(map[string]interface{})
	if !ok {
		return
	}

	for finalDefName, definition := range definitions {
		// 使用mathx.ConvertMapKeysToString确保定义的键为字符串
		convertedDef := mathx.ConvertMapKeysToString(definition)

		if existingDef, exists := allDefinitions[finalDefName]; exists {
			s.checkDefinitionConsistency(finalDefName, existingDef, convertedDef, serviceName)
			continue
		}

		allDefinitions[finalDefName] = convertedDef
	}
}

// checkDefinitionConsistency 检查定义一致性
func (s *SwaggerMiddleware) checkDefinitionConsistency(defName string, existingDef, newDef interface{}, serviceName string) {
	existingJSON, _ := json.Marshal(existingDef)
	newJSON, _ := json.Marshal(newDef)
	if string(existingJSON) != string(newJSON) {
		global.LOGGER.Warn("⚠️  类型 %s 在不同服务中定义不一致！当前使用第一个定义 (来自排序后的首个服务)，忽略 %s 的定义", defName, serviceName)
	} else {
		global.LOGGER.Debug("类型 %s 已存在且定义一致，跳过 (来自: %s)", defName, serviceName)
	}
}

// mergeServiceSpecTags 合并服务规范的标签
func (s *SwaggerMiddleware) mergeServiceSpecTags(specMap map[string]interface{}, serviceName string, allTags *[]interface{}, tagNames map[string]bool) {
	tags, ok := specMap[constants.SwaggerFieldTags].([]interface{})
	if !ok {
		return
	}

	for _, tag := range tags {
		tagMap, ok := tag.(map[string]interface{})
		if !ok {
			continue
		}

		name, exists := tagMap[constants.SwaggerFieldName]
		if !exists {
			continue
		}

		nameStr := convert.MustString(name)
		if s.addUniqueTag(nameStr, tagMap, allTags, tagNames) {
			global.LOGGER.Debug("添加原始Swagger标签: %s (服务: %s)", nameStr, serviceName)
		}
	}
}

// addUniqueTag 添加唯一标签（通用去重逻辑）
func (s *SwaggerMiddleware) addUniqueTag(tagKey string, tag interface{}, allTags *[]interface{}, tagSet map[string]bool) bool {
	if tagKey == "" || tagSet[tagKey] {
		return false
	}
	tagSet[tagKey] = true
	*allTags = append(*allTags, tag)
	return true
}

// createSelectorSpec 创建选择器规范
func (s *SwaggerMiddleware) createSelectorSpec() error {
	s.aggregatedSpec = map[string]interface{}{
		constants.SwaggerFieldSwagger:        constants.SwaggerVersion,
		constants.SwaggerFieldInfo:           s.buildAggregateInfo(),
		constants.SwaggerFieldConsumes:       []string{constants.MimeApplicationJSON},
		constants.SwaggerFieldProduces:       []string{constants.MimeApplicationJSON},
		constants.SwaggerFieldPaths:          make(map[string]interface{}),
		constants.SwaggerFieldDefs:           make(map[string]interface{}),
		constants.SwaggerFieldXAggregateInfo: s.buildServicesInfo(),
		constants.SwaggerFieldXServiceSelector: map[string]interface{}{
			constants.SwaggerFieldEnabled:  true,
			constants.SwaggerFieldServices: s.buildServicesSummary(),
		},
	}

	global.LOGGER.Info("✅ 选择器规范创建完成")
	return nil
}

// buildAggregateInfo 构建聚合信息
func (s *SwaggerMiddleware) buildAggregateInfo() map[string]interface{} {
	info := map[string]interface{}{
		constants.SwaggerFieldTitle:       s.config.Title,
		constants.SwaggerFieldDescription: s.config.Description,
		constants.SwaggerFieldVersion:     s.config.Version,
	}

	// 只在配置存在时才添加 contact 字段
	if contact := s.buildContactInfo(); contact != nil {
		info[constants.SwaggerFieldContact] = contact
	}

	// 只在配置存在时才添加 license 字段
	if license := s.buildLicenseInfo(); license != nil {
		info[constants.SwaggerFieldLicense] = license
	}

	return info
}

// buildContactInfo 构建联系信息
func (s *SwaggerMiddleware) buildContactInfo() interface{} {
	safeContact := safe.Safe(s.config.Contact)
	contact := make(map[string]interface{})

	// 只添加非空字段
	if name := safeContact.Field("Name").String(""); name != "" {
		contact[constants.SwaggerFieldName] = name
	}
	if email := safeContact.Field("Email").String(""); email != "" {
		contact[constants.SwaggerFieldEmail] = email
	}
	if url := safeContact.Field("URL").String(""); url != "" {
		contact[constants.SwaggerFieldURL] = url
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
		license[constants.SwaggerFieldName] = name
	}
	if url := safeLicense.Field("URL").String(""); url != "" {
		license[constants.SwaggerFieldURL] = url
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
				constants.SwaggerFieldName:        service.Name,
				constants.SwaggerFieldDescription: service.Description,
				constants.SwaggerFieldVersion:     service.Version,
				constants.SwaggerFieldTags:        service.Tags,
				constants.SwaggerFieldEnabled:     service.Enabled,
			}
			services = append(services, serviceInfo)
		}
	}
	return services
}

// buildServicesInfo 构建服务信息
func (s *SwaggerMiddleware) buildServicesInfo() map[string]interface{} {
	return map[string]interface{}{
		constants.SwaggerFieldMode:     s.config.Aggregate.Mode,
		constants.SwaggerFieldServices: s.buildServicesSummary(),
		constants.SwaggerFieldUpdated:  s.lastUpdated.Format(time.RFC3339),
		constants.SwaggerFieldCount:    len(s.serviceSpecs),
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

	// 使用mathx.ConvertMapKeysToString确保所有键为字符串
	convertedSpec := mathx.ConvertMapKeysToString(s.aggregatedSpec)

	// 序列化为JSON
	jsonData, err := json.MarshalIndent(convertedSpec, constants.JSONIndentPrefix, constants.JSONIndentValue)
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

	spec, exists := s.findServiceSpec(serviceName)
	if !exists {
		return nil, s.serviceNotFoundError(serviceName)
	}

	return s.serializeServiceSpec(spec)
}

// findServiceSpec 查找服务规范（支持多种匹配策略）
func (s *SwaggerMiddleware) findServiceSpec(serviceName string) (map[string]interface{}, bool) {
	// 尝试直接匹配
	if spec, exists := s.serviceSpecs[serviceName]; exists {
		return spec, true
	}

	// 尝试灵活匹配
	return s.flexibleMatchService(serviceName)
}

// flexibleMatchService 灵活匹配服务（忽略大小写、标准化名称、包含匹配）
func (s *SwaggerMiddleware) flexibleMatchService(serviceName string) (map[string]interface{}, bool) {
	for actualServiceName, actualSpec := range s.serviceSpecs {
		if s.matchServiceByCaseInsensitive(serviceName, actualServiceName) {
			global.LOGGER.Info("通过忽略大小写匹配找到服务: %s -> %s", serviceName, actualServiceName)
			return actualSpec, true
		}

		if s.matchServiceByNormalized(serviceName, actualServiceName) {
			global.LOGGER.Info("通过标准化名称匹配找到服务: %s -> %s", serviceName, actualServiceName)
			return actualSpec, true
		}

		if s.matchServiceByContains(serviceName, actualServiceName) {
			global.LOGGER.Info("通过包含匹配找到服务: %s -> %s", serviceName, actualServiceName)
			return actualSpec, true
		}
	}

	return nil, false
}

// matchServiceByCaseInsensitive 忽略大小写匹配
func (s *SwaggerMiddleware) matchServiceByCaseInsensitive(requested, actual string) bool {
	return stringx.EqualsIgnoreCase(actual, requested)
}

// matchServiceByNormalized 标准化名称匹配（使用多种命名风格）
func (s *SwaggerMiddleware) matchServiceByNormalized(requested, actual string) bool {
	// 使用stringx.NormalizeFieldName获取所有可能的变体
	requestedVariants := stringx.NormalizeFieldName(requested)
	actualVariants := stringx.NormalizeFieldName(actual)

	// 检查是否有任何变体匹配
	for _, rv := range requestedVariants {
		for _, av := range actualVariants {
			if rv == av {
				return true
			}
		}
	}
	return false
}

// matchServiceByContains 包含匹配
func (s *SwaggerMiddleware) matchServiceByContains(requested, actual string) bool {
	return stringx.ContainsIgnoreCase(actual, requested)
}

// serviceNotFoundError 构建服务未找到错误
func (s *SwaggerMiddleware) serviceNotFoundError(serviceName string) error {
	availableServices := make([]string, 0, len(s.serviceSpecs))
	for name := range s.serviceSpecs {
		availableServices = append(availableServices, name)
	}
	sort.Strings(availableServices) // 排序以便阅读
	errMsg := fmt.Sprintf("服务 %s 不存在。可用服务: [%s]", serviceName, strings.Join(availableServices, ", "))
	global.LOGGER.Error(errMsg)
	return fmt.Errorf("%s", errMsg)
}

// serializeServiceSpec 序列化服务规范为JSON
func (s *SwaggerMiddleware) serializeServiceSpec(spec map[string]interface{}) ([]byte, error) {
	// 使用mathx.ConvertMapKeysToString确保所有键为字符串
	convertedSpec := mathx.ConvertMapKeysToString(spec)

	jsonData, err := json.MarshalIndent(convertedSpec, constants.JSONIndentPrefix, constants.JSONIndentValue)
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

// getServiceStringField 安全获取服务字段值
func getServiceStringField(service map[string]interface{}, field string) string {
	return safe.Safe(service).Field(field).String("")
}

// fixReferences 修复聚合规范中的所有引用路径
func (s *SwaggerMiddleware) fixReferences() error {
	return s.fixReferencesInObject(s.aggregatedSpec)
}

// fixReferencesInObject 递归修复对象中的引用
func (s *SwaggerMiddleware) fixReferencesInObject(obj interface{}) error {
	switch v := obj.(type) {
	case map[string]interface{}:
		return s.fixReferencesInMap(v)
	case []interface{}:
		return s.fixReferencesInSlice(v)
	}
	return nil
}

// fixReferencesInMap 修复map中的引用
func (s *SwaggerMiddleware) fixReferencesInMap(m map[string]interface{}) error {
	for _, value := range m {
		if err := s.fixReferencesInObject(value); err != nil {
			return err
		}
	}
	return nil
}

// fixReferencesInSlice 修复slice中的引用
func (s *SwaggerMiddleware) fixReferencesInSlice(slice []interface{}) error {
	for _, item := range slice {
		if err := s.fixReferencesInObject(item); err != nil {
			return err
		}
	}
	return nil
}
