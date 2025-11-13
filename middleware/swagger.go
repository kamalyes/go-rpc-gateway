/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 22:15:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 06:55:12
 * @FilePath: \go-rpc-gateway\middleware\swagger.go
 * @Description: Swagger文档中间件 - 提供API文档在线查看
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strings"

	goswagger "github.com/kamalyes/go-config/pkg/swagger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
)

// SwaggerMiddleware Swagger文档中间件
// [EN] Swagger documentation middleware
type SwaggerMiddleware struct {
	config      *goswagger.Swagger
	swaggerJSON []byte
}

// NewSwaggerMiddleware 创建Swagger中间件
// [EN] Create Swagger middleware
func NewSwaggerMiddleware(config *goswagger.Swagger) *SwaggerMiddleware {
	if config == nil {
		config = goswagger.Default()
	}

	middleware := &SwaggerMiddleware{
		config: config,
	}

	// 如果启用且有JSON路径，加载Swagger JSON
	// [EN] If enabled and has JSON path, load Swagger JSON
	if config.Enabled && config.JSONPath != "" {
		if err := middleware.loadSwaggerJSON(); err != nil {
			global.LOGGER.Error("加载Swagger JSON失败: %v, path: %s", err, config.JSONPath)
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

	// 处理swagger.json请求
	// [EN] Handle swagger.json request
	if strings.HasSuffix(path, "/swagger.json") {
		s.handleSwaggerJSON(w, r)
		return
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
	htmlTemplate := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>{{.Title}}</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.0.0/swagger-ui.css" />
    <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5.0.0/favicon-32x32.png" sizes="32x32" />
    <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5.0.0/favicon-16x16.png" sizes="16x16" />
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
            margin:0;
            background: #fafafa;
        }
        .swagger-ui .topbar {
            background-color: #89CFF0;
            border-bottom: 1px solid #bfbfbf;
        }
        .swagger-ui .topbar .download-url-wrapper {
            display: none;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.0.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.0.0/swagger-ui-standalone-preset.js"></script>
    <script>
    window.onload = function() {
        const ui = SwaggerUIBundle({
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
            layout: "StandaloneLayout",
            validatorUrl: null,
            docExpansion: "none",
            operationsSorter: "alpha",
            tagsSorter: "alpha",
            filter: true,
            showExtensions: true,
            showCommonExtensions: true
        });

        document.title = '{{.Title}}';
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

// ReloadSwaggerJSON 重新加载Swagger JSON文件
// [EN] Reload Swagger JSON file
func (s *SwaggerMiddleware) ReloadSwaggerJSON() error {
	if s.config.JSONPath == "" {
		return nil
	}
	return s.loadSwaggerJSON()
}
