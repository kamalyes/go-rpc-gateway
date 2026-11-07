/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\middleware.go
 * @Description: Swagger 中间件核心 - 结构定义、构造函数、HTTP Handler 路由分发
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	aggregate "github.com/kamalyes/go-rpc-gateway/middleware/swagger/aggregate"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	documents "github.com/kamalyes/go-rpc-gateway/middleware/swagger/documents"
	serrors "github.com/kamalyes/go-rpc-gateway/middleware/swagger/errors"
	format "github.com/kamalyes/go-rpc-gateway/middleware/swagger/format"
	loader "github.com/kamalyes/go-rpc-gateway/middleware/swagger/loader"
	errorx "github.com/kamalyes/go-toolbox/pkg/errorx"
	logger "github.com/kamalyes/go-toolbox/pkg/logger"
)

// UIAction Swagger UI 页面顶部导航链接
type UIAction struct {
	Href  string
	Label string
}

// Middleware Swagger 文档中间件
// 支持单一模式和聚合模式，聚合模式下可按服务/文档维度查看 API 规范
type Middleware struct {
	config      *goswagger.Swagger
	swaggerJSON []byte

	aggregator *aggregate.Aggregator
	docBuilder *documents.Builder

	lastUpdated     time.Time
	refreshInterval time.Duration

	watcher *Watcher
	logger  logger.ILogger

	errorResponseFn ErrorResponseFunc
}

// NewMiddleware 创建 Swagger 中间件
// config: go-config 的 Swagger 配置
// opts: 可选参数，支持自定义日志器和错误响应函数
func NewMiddleware(config *goswagger.Swagger, opts ...Option) *Middleware {
	options := ApplyOptions(opts...)

	m := &Middleware{
		config:          config,
		refreshInterval: constants.DefaultRefreshInterval,
		logger:          options.Logger,
		errorResponseFn: options.ErrorResponseFn,
	}

	if m.logger == nil {
		m.logger = logger.New().WithPrefix("[SWAGGER]")
	}
	if m.errorResponseFn == nil {
		m.errorResponseFn = defaultErrorResponse
	}

	if config.IsAggregateEnabled() {
		m.aggregator = aggregate.NewAggregator(config, m.logger)
		m.logger.Info("启用Swagger聚合模式")
		if err := m.loadAllServiceSpecs(); err != nil {
			m.logger.Error("初始化聚合规范失败: %v", err)
		} else {
			m.logger.Info("聚合规范创建成功")
		}
	} else {
		m.logger.Info("使用单一Swagger模式")
		if config.Enabled {
			if err := m.loadSwaggerSpec(); err != nil {
				m.logger.Error("加载Swagger文件失败: %v", err)
			}
		}
	}

	if config.Enabled && config.HotReload {
		if err := m.EnableFileWatcher(); err != nil {
			m.logger.Error("启用Swagger文件热重载失败: %v", err)
		}
	}

	return m
}

// Handler 返回 Swagger 处理中间件
// 当请求路径匹配 Swagger 路由时由中间件处理，否则传递给下一个 Handler
func (m *Middleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !m.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			if m.isSwaggerPath(r.URL.Path) {
				m.handleSwagger(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ServeHTTP 实现 http.Handler 接口，独立提供 Swagger 服务
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.config.Enabled {
		http.NotFound(w, r)
		return
	}
	m.handleSwagger(w, r)
}

// ==================== 路由匹配 ====================

// isSwaggerPath 检查请求路径是否属于 Swagger 路由
func (m *Middleware) isSwaggerPath(path string) bool {
	swaggerPaths := []string{
		m.config.UIPath,
		m.config.UIPath + "/",
		m.config.UIPath + constants.IndexHTML,
		m.config.UIPath + constants.JSONPath,
	}

	if m.config.IsAggregateEnabled() {
		swaggerPaths = append(swaggerPaths,
			m.config.UIPath+constants.ServicesPath,
			m.config.UIPath+constants.DocumentsPath,
			m.config.UIPath+constants.AggregatePath,
			m.config.UIPath+constants.DebugPath,
		)
		if strings.HasPrefix(path, m.config.UIPath+constants.PathServicePrefix) {
			return true
		}
		if strings.HasPrefix(path, m.config.UIPath+constants.PathDocumentPrefix) {
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

// ==================== 请求分发 ====================

// handleSwagger 根据请求路径分发到对应的处理方法
func (m *Middleware) handleSwagger(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if m.config.IsAggregateEnabled() {
		if m.tryHandleDocumentJSON(w, r, path) ||
			m.tryHandleDocumentUI(w, r, path) ||
			m.tryHandleAggregatedJSON(w, r, path) ||
			m.tryHandleServiceJSON(w, r, path) ||
			m.tryHandleServiceUI(w, r, path) ||
			m.tryHandleServicesIndex(w, r, path) ||
			m.tryHandleDocumentsIndex(w, r, path) ||
			m.tryHandleServicesDebug(w, r, path) ||
			m.tryHandleAggregatedAsJSON(w, r, path) {
			return
		}
	} else {
		if strings.HasSuffix(path, constants.JSONPath) {
			m.handleSwaggerJSON(w, r)
			return
		}
	}

	if path == m.config.UIPath || path == m.config.UIPath+"/" || strings.HasSuffix(path, constants.IndexHTML) {
		m.handleSwaggerUI(w, r)
		return
	}

	http.Redirect(w, r, m.config.UIPath+"/", http.StatusTemporaryRedirect)
}

// ==================== 聚合模式路由尝试 ====================

func (m *Middleware) tryHandleDocumentJSON(w http.ResponseWriter, r *http.Request, path string) bool {
	if strings.HasPrefix(path, m.config.UIPath+constants.PathDocumentPrefix) && strings.HasSuffix(path, constants.JSONExt) {
		m.handleDocumentJSON(w, r)
		return true
	}
	return false
}

func (m *Middleware) tryHandleDocumentUI(w http.ResponseWriter, r *http.Request, path string) bool {
	if strings.HasPrefix(path, m.config.UIPath+constants.PathDocumentPrefix) && !strings.HasSuffix(path, constants.JSONExt) {
		m.handleDocumentUI(w, r)
		return true
	}
	return false
}

func (m *Middleware) tryHandleAggregatedJSON(w http.ResponseWriter, r *http.Request, path string) bool {
	if strings.HasSuffix(path, constants.AggregatePath) {
		m.handleAggregatedJSON(w, r)
		return true
	}
	return false
}

func (m *Middleware) tryHandleServiceJSON(w http.ResponseWriter, r *http.Request, path string) bool {
	if strings.HasPrefix(path, m.config.UIPath+constants.PathServicePrefix) && strings.HasSuffix(path, constants.JSONExt) {
		m.handleServiceJSON(w, r)
		return true
	}
	return false
}

func (m *Middleware) tryHandleServiceUI(w http.ResponseWriter, r *http.Request, path string) bool {
	if strings.HasPrefix(path, m.config.UIPath+constants.PathServicePrefix) && !strings.HasSuffix(path, constants.JSONExt) {
		m.handleServiceUI(w, r)
		return true
	}
	return false
}

func (m *Middleware) tryHandleServicesIndex(w http.ResponseWriter, r *http.Request, path string) bool {
	if strings.HasSuffix(path, constants.ServicesPath) {
		m.handleServicesIndex(w, r)
		return true
	}
	return false
}

func (m *Middleware) tryHandleDocumentsIndex(w http.ResponseWriter, r *http.Request, path string) bool {
	if strings.HasSuffix(path, constants.DocumentsPath) {
		m.handleDocumentsIndex(w, r)
		return true
	}
	return false
}

func (m *Middleware) tryHandleServicesDebug(w http.ResponseWriter, r *http.Request, path string) bool {
	if strings.HasSuffix(path, constants.DebugPath) {
		m.handleServicesDebug(w, r)
		return true
	}
	return false
}

func (m *Middleware) tryHandleAggregatedAsJSON(w http.ResponseWriter, r *http.Request, path string) bool {
	if strings.HasSuffix(path, constants.JSONPath) {
		m.handleAggregatedJSON(w, r)
		return true
	}
	return false
}

// ==================== 单一模式处理 ====================

// handleSwaggerUI 处理 Swagger UI 页面请求
func (m *Middleware) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	writeHTMLHeaders(w)
	_, _ = w.Write([]byte(m.generateRootSwaggerUI()))
}

// handleSwaggerJSON 处理单一模式下的 swagger.json 请求
func (m *Middleware) handleSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	writeJSONHeaders(w)
	if handleOptions(w, r) {
		return
	}
	if m.swaggerJSON == nil {
		m.errorResponseFn(w, http.StatusNotFound, "Swagger JSON not found")
		return
	}
	m.serveSpecJSON(w, r, m.swaggerJSON)
}

// ==================== 配置更新 ====================

// UpdateConfig 更新 Swagger 配置，并根据最新配置重建文档与监听器
func (m *Middleware) UpdateConfig(config *goswagger.Swagger) error {
	if config == nil {
		return errorx.NewError(serrors.ErrTypeConfigNil)
	}

	if m.watcher != nil {
		if err := m.DisableFileWatcher(); err != nil {
			return err
		}
	}

	m.config = config

	if !config.Enabled {
		m.swaggerJSON = nil
		m.aggregator = nil
		m.docBuilder = nil
		m.lastUpdated = time.Now()
		m.logger.Info("Swagger 已禁用，已清空内存中的文档缓存")
		return nil
	}

	if config.IsAggregateEnabled() {
		if err := m.loadAllServiceSpecs(); err != nil {
			return errorx.NewError(serrors.ErrTypeAggregateFailed, err)
		}
	} else {
		if err := m.loadSwaggerSpec(); err != nil {
			return errorx.NewError(serrors.ErrTypeLoadFailed, err)
		}
	}

	if config.HotReload {
		if err := m.EnableFileWatcher(); err != nil {
			return errorx.NewError(serrors.ErrTypeWatcherStartFailed, err)
		}
	}

	m.logger.Info("Swagger 配置已更新: enabled=%v, hot_reload=%v, aggregate=%v",
		config.Enabled, config.HotReload, config.IsAggregateEnabled())
	return nil
}

// ==================== 公共访问方法 ====================

// IsAggregateEnabled 检查聚合功能是否启用
func (m *Middleware) IsAggregateEnabled() bool {
	return m.config.IsAggregateEnabled()
}

// GetLastUpdated 获取最后更新时间
func (m *Middleware) GetLastUpdated() time.Time {
	return m.lastUpdated
}

// RefreshSpecs 刷新所有服务规范
func (m *Middleware) RefreshSpecs() error {
	return m.loadAllServiceSpecs()
}

// GetConfig 获取当前配置
func (m *Middleware) GetConfig() *goswagger.Swagger {
	return m.config
}

// GetSwaggerPaths 获取所有 Swagger 路由路径
func (m *Middleware) GetSwaggerPaths() []string {
	if !m.config.Enabled {
		return nil
	}

	paths := []string{
		m.config.UIPath,
		m.config.UIPath + "/",
		m.config.UIPath + constants.IndexHTML,
		m.config.UIPath + constants.JSONPath,
	}

	if m.config.IsAggregateEnabled() {
		paths = append(paths,
			m.config.UIPath+constants.ServicesPath,
			m.config.UIPath+constants.DocumentsPath,
			m.config.UIPath+constants.AggregatePath,
			m.config.UIPath+constants.DebugPath,
		)
	}

	return paths
}

// ReloadSwaggerJSON 重新加载 Swagger 文件
func (m *Middleware) ReloadSwaggerJSON() error {
	return m.loadSwaggerSpec()
}

// SetSwaggerJSON 设置 Swagger JSON 数据
func (m *Middleware) SetSwaggerJSON(jsonData []byte) error {
	var swagger map[string]interface{}
	if err := json.Unmarshal(jsonData, &swagger); err != nil {
		return err
	}
	var err error
	m.swaggerJSON, err = json.MarshalIndent(swagger, constants.JSONIndentPrefix, constants.JSONIndentValue)
	return err
}

// ==================== 聚合加载入口 ====================

// loadAllServiceSpecs 加载所有服务的 Swagger 规范并执行聚合
func (m *Middleware) loadAllServiceSpecs() error {
	if m.aggregator == nil {
		m.aggregator = aggregate.NewAggregator(m.config, m.logger)
	}

	if err := m.aggregator.LoadAll(); err != nil {
		return err
	}

	m.lastUpdated = m.aggregator.GetLastUpdated()

	// 构建独立子文档
	m.docBuilder = documents.NewBuilder(m.config, m.aggregator.GetServiceSpecs(), m.lastUpdated, m.logger)
	if err := m.docBuilder.BuildAll(); err != nil {
		return err
	}

	return nil
}

// loadSwaggerSpec 加载单一模式 Swagger 规范文件
func (m *Middleware) loadSwaggerSpec() error {
	var path string
	if m.config.SpecPath != "" {
		path = m.config.SpecPath
	} else if m.config.YamlPath != "" {
		path = m.config.YamlPath
	} else if m.config.JSONPath != "" {
		path = m.config.JSONPath
	} else {
		return nil
	}

	spec, err := loader.LoadSpecFromPath(path)
	if err != nil {
		return err
	}

	m.swaggerJSON, err = format.MarshalSpec(spec)
	return err
}

// ==================== 名称匹配工具 ====================

// extractEntityName 从请求路径中提取实体名称（服务名或文档名）
func (m *Middleware) extractEntityName(path, entityPrefix string) string {
	entityName := strings.TrimPrefix(path, m.config.UIPath+entityPrefix)
	entityName = strings.TrimSuffix(entityName, constants.JSONExt)
	return strings.Trim(entityName, constants.PathSeparator)
}

// ==================== HTTP 响应工具 ====================

// writeJSONHeaders 写入 JSON 响应头和 CORS 头
func writeJSONHeaders(w http.ResponseWriter) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSONCharset)
	w.Header().Set(constants.HeaderAccessControlAllowOrigin, constants.CORSAllowAll)
	w.Header().Set(constants.HeaderAccessControlAllowMethods, constants.CORSDefaultMethods)
	w.Header().Set(constants.HeaderAccessControlAllowHeaders, constants.CORSDefaultHeaders)
}

// writeHTMLHeaders 写入 HTML 响应头和 CORS 头
func writeHTMLHeaders(w http.ResponseWriter) {
	w.Header().Set(constants.HeaderContentType, constants.MimeTextHTMLCharset)
	w.Header().Set(constants.HeaderAccessControlAllowOrigin, constants.CORSAllowAll)
}

// handleOptions 处理 OPTIONS 预检请求
func handleOptions(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != constants.HTTPMethodOptions {
		return false
	}
	w.WriteHeader(http.StatusOK)
	return true
}

// defaultErrorResponse 默认 JSON 错误响应
func defaultErrorResponse(w http.ResponseWriter, httpStatus int, message string) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSONCharset)
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":   httpStatus,
		"error":  message,
		"status": httpStatus,
	})
}
