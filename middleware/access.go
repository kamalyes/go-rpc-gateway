/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 18:03:45
 * @FilePath: \go-rpc-gateway\middleware\access.go
 * @Description: 访问记录中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/kamalyes/go-rpc-gateway/internal/constants"
	"github.com/mssola/user_agent"
)

// AccessRecordConfig 访问记录配置
type AccessRecordConfig struct {
	Enabled         bool     `json:"enabled" yaml:"enabled"`                 // 是否启用
	ServiceName     string   `json:"serviceName" yaml:"serviceName"`         // 服务名称
	RetentionDays   int      `json:"retentionDays" yaml:"retentionDays"`     // 保留天数
	IncludeBody     bool     `json:"includeBody" yaml:"includeBody"`         // 是否记录请求体
	IncludeResponse bool     `json:"includeResponse" yaml:"includeResponse"` // 是否记录响应体
	IncludeHeaders  []string `json:"includeHeaders" yaml:"includeHeaders"`   // 要记录的头部
	ExcludePaths    []string `json:"excludePaths" yaml:"excludePaths"`       // 排除的路径
	MaxBodySize     int64    `json:"maxBodySize" yaml:"maxBodySize"`         // 最大请求体大小
	MaxResponseSize int64    `json:"maxResponseSize" yaml:"maxResponseSize"` // 最大响应体大小
}

// DefaultAccessRecordConfig 默认访问记录配置
func DefaultAccessRecordConfig() *AccessRecordConfig {
	return &AccessRecordConfig{
		Enabled:         true,
		ServiceName:     "rpc-gateway",
		RetentionDays:   60,
		IncludeBody:     true,
		IncludeResponse: true,
		IncludeHeaders:  []string{constants.HeaderUserAgent, constants.HeaderXRequestID, constants.HeaderXTraceID, constants.HeaderAuthorization, constants.HeaderContentType},
		ExcludePaths:    []string{constants.PathHealth, constants.PathMetrics, constants.PathDebug},
		MaxBodySize:     1024 * 1024,     // 1MB
		MaxResponseSize: 1024 * 1024 * 5, // 5MB
	}
}

// AccessRecord 访问记录结构
type AccessRecord struct {
	ServiceName   string            `json:"serviceName"`           // 服务名称
	Timestamp     time.Time         `json:"timestamp"`             // 时间戳
	IP            string            `json:"ip"`                    // 客户端 IP
	Method        string            `json:"method"`                // HTTP 方法
	Path          string            `json:"path"`                  // 请求路径
	Query         string            `json:"query"`                 // 查询参数
	Headers       map[string]string `json:"headers"`               // 请求头
	Body          string            `json:"body"`                  // 请求体
	Response      string            `json:"response"`              // 响应体
	Status        int               `json:"status"`                // 响应状态码
	Latency       int64             `json:"latency"`               // 延迟（毫秒）
	Error         string            `json:"error,omitempty"`       // 错误信息
	UserAgent     UserAgentInfo     `json:"userAgent"`             // User Agent 信息
	BusinessID    string            `json:"businessId,omitempty"`  // 业务 ID
	UserID        string            `json:"userId,omitempty"`      // 用户 ID
	TraceID       string            `json:"traceId,omitempty"`     // 链路追踪 ID
	RequestID     string            `json:"requestId,omitempty"`   // 请求 ID
	Referer       string            `json:"referer,omitempty"`     // 引用页
	ContentType   string            `json:"contentType,omitempty"` // 内容类型
	RetentionDays int               `json:"retentionDays"`         // 保留天数
}

// UserAgentInfo User Agent 信息
type UserAgentInfo struct {
	Raw            string `json:"raw"`                      // 原始 User Agent
	Platform       string `json:"platform,omitempty"`       // 平台
	OS             string `json:"os,omitempty"`             // 操作系统
	Engine         string `json:"engine,omitempty"`         // 引擎
	BrowserName    string `json:"browserName,omitempty"`    // 浏览器名称
	BrowserVersion string `json:"browserVersion,omitempty"` // 浏览器版本
	ProductModel   string `json:"productModel,omitempty"`   // 产品型号
}

// AccessRecordHandler 访问记录处理器接口
type AccessRecordHandler interface {
	Handle(ctx context.Context, record *AccessRecord) error
}

// LogAccessRecordHandler 日志访问记录处理器
type LogAccessRecordHandler struct{}

func (h *LogAccessRecordHandler) Handle(ctx context.Context, record *AccessRecord) error {
	recordJSON, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal access record: %w", err)
	}

	fmt.Printf("ACCESS_RECORD: %s\n", string(recordJSON))
	return nil
}

// accessRecordResponseWriter 响应写入器包装
type accessRecordResponseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	maxSize    int64
}

func (w *accessRecordResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *accessRecordResponseWriter) Write(data []byte) (int, error) {
	// 限制响应体大小
	if w.body != nil && int64(w.body.Len()+len(data)) <= w.maxSize {
		w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

// AccessRecordMiddleware 访问记录中间件
func AccessRecordMiddleware(config *AccessRecordConfig, handler AccessRecordHandler) HTTPMiddleware {
	if config == nil {
		config = DefaultAccessRecordConfig()
	}

	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	if handler == nil {
		handler = &LogAccessRecordHandler{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查是否在排除路径中
			for _, excludePath := range config.ExcludePaths {
				if strings.HasPrefix(r.URL.Path, excludePath) {
					next.ServeHTTP(w, r)
					return
				}
			}

			startTime := time.Now()

			// 创建访问记录
			record := &AccessRecord{
				ServiceName:   config.ServiceName,
				Timestamp:     startTime,
				IP:            getClientIP(r),
				Method:        r.Method,
				Path:          r.URL.Path,
				Query:         r.URL.RawQuery,
				Headers:       make(map[string]string),
				Status:        200, // 默认状态码
				Referer:       r.Referer(),
				ContentType:   r.Header.Get("Content-Type"),
				RetentionDays: config.RetentionDays,
				RequestID:     r.Header.Get("X-Request-Id"),
				TraceID:       r.Header.Get("X-Trace-Id"),
			}

			// 记录指定的请求头
			for _, headerName := range config.IncludeHeaders {
				if value := r.Header.Get(headerName); value != "" {
					record.Headers[headerName] = value
				}
			}

			// 解析 User Agent
			record.UserAgent = parseUserAgent(r.UserAgent())

			// 记录请求体
			if config.IncludeBody && r.Body != nil {
				body, err := io.ReadAll(io.LimitReader(r.Body, config.MaxBodySize))
				if err == nil {
					r.Body = io.NopCloser(bytes.NewBuffer(body))
					if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
						record.Body = "文件上传"
					} else {
						record.Body = compactJSON(body)
					}
				}
			}

			// 包装响应写入器
			var responseWriter *accessRecordResponseWriter
			if config.IncludeResponse {
				responseWriter = &accessRecordResponseWriter{
					ResponseWriter: w,
					body:           &bytes.Buffer{},
					statusCode:     200,
					maxSize:        config.MaxResponseSize,
				}
			} else {
				responseWriter = &accessRecordResponseWriter{
					ResponseWriter: w,
					statusCode:     200,
				}
			}

			// 处理请求
			next.ServeHTTP(responseWriter, r)

			// 完成记录
			record.Status = responseWriter.statusCode
			record.Latency = time.Since(startTime).Milliseconds()

			if config.IncludeResponse && responseWriter.body != nil {
				record.Response = responseWriter.body.String()
			}

			// 异步处理访问记录
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := handler.Handle(ctx, record); err != nil {
					fmt.Printf("Failed to handle access record: %v\n", err)
				}
			}()
		})
	}
}

// parseUserAgent 解析 User Agent
func parseUserAgent(userAgentStr string) UserAgentInfo {
	info := UserAgentInfo{
		Raw: userAgentStr,
	}

	if userAgentStr == "" {
		return info
	}

	ua := user_agent.New(userAgentStr)

	// 解析基本信息
	info.Platform = ua.Platform()
	info.OS = ua.OS()

	// 解析引擎信息
	engineName, engineVersion := ua.Engine()
	if engineName != "" {
		info.Engine = fmt.Sprintf("%s/%s", engineName, engineVersion)
	}

	// 解析浏览器信息
	info.BrowserName, info.BrowserVersion = ua.Browser()

	// 解析设备型号（从括号中提取）
	rex := regexp.MustCompile(`\(([^)]+)\)`)
	params := rex.FindAllStringSubmatch(userAgentStr, -1)
	if len(params) > 0 {
		param := strings.Replace(params[0][0], ")", "", 1)
		uaInfo := strings.Split(param, ";")
		if len(uaInfo) > 2 {
			info.ProductModel = strings.TrimSpace(uaInfo[2])
		}
	}

	return info
}

// compactJSON 压缩 JSON
func compactJSON(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var compacted bytes.Buffer
	if err := json.Compact(&compacted, data); err != nil {
		// 如果不是有效的 JSON，直接返回字符串
		return string(data)
	}

	return compacted.String()
}
