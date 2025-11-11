/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 18:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:49:18
 * @FilePath: \go-rpc-gateway\middleware\pprof.go
 * @Description: pprof性能分析中间件 - 使用适配器模式包装go-config类型
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"path"
	"strings"
	"time"

	gopprof "github.com/kamalyes/go-config/pkg/pprof"
	gologger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/constants"
)

// DefaultPProfConfig 默认pprof配置
func DefaultPProfConfig() *gopprof.PProf {
	return gopprof.Default()
}

// PProfInfo pprof端点信息
type PProfInfo struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	Method      string `json:"method"`
}

// PProfConfigAdapter pprof配置适配器，包装外部配置类型并扩展额外字段
type PProfConfigAdapter struct {
	*gopprof.PProf
	scenarios      *PProfScenarios
	logger         *gologger.Logger
	
	// 扩展字段 - go-config 中没有的字段
	AllowedIPs     []string                   // IP白名单
	RequireAuth    bool                       // 是否需要认证
	AuthToken      string                     // 认证token
	EnableLogging  bool                       // 是否启用访问日志
	Timeout        int                        // 超时时间（秒）
	CustomHandlers map[string]http.HandlerFunc // 自定义处理器（运行时）
}

// NewPProfConfigAdapter 创建pprof配置适配器
func NewPProfConfigAdapter(config *gopprof.PProf) *PProfConfigAdapter {
	if config == nil {
		config = DefaultPProfConfig()
	}
	adapter := &PProfConfigAdapter{
		PProf:          config,
		scenarios:      NewPProfScenarios(),
		AllowedIPs:     []string{},     // 默认允许所有IP
		RequireAuth:    false,          // 默认不需要认证
		AuthToken:      "",             // 需要用户设置
		EnableLogging:  true,           // 默认启用日志
		Timeout:        30,             // 默认30秒超时
		CustomHandlers: make(map[string]http.HandlerFunc),
	}
	
	// 创建日志记录器
	logConfig := &gologger.LogConfig{Level: gologger.INFO}
	adapter.logger = gologger.NewLogger(logConfig)
	
	return adapter
}

// GetAvailableEndpoints 获取所有可用的pprof端点信息
func (a *PProfConfigAdapter) GetAvailableEndpoints() []PProfInfo {
	basePrefix := strings.TrimSuffix(a.PProf.PathPrefix, "/")
	
	endpoints := []PProfInfo{
		{
			Path:        basePrefix + "/",
			Description: "pprof索引页面，显示所有可用的性能分析处理器链接",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/allocs",
			Description: "显示内存分配采样信息",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/block",
			Description: "显示导致阻塞的同步原语的堆栈跟踪",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/cmdline",
			Description: "显示当前程序的命令行参数",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/goroutine",
			Description: "显示当前所有goroutine的堆栈跟踪",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/heap",
			Description: "显示活动对象的内存分配采样",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/mutex",
			Description: "显示导致互斥锁争用的堆栈跟踪",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/profile",
			Description: "CPU性能分析，可通过seconds参数指定采样时间",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/threadcreate",
			Description: "显示创建新OS线程的堆栈跟踪",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/trace",
			Description: "当前程序的执行跟踪，可通过seconds参数指定跟踪时间",
			Method:      "GET",
		},
		{
			Path:        basePrefix + "/symbol",
			Description: "查找程序计数器列表中的符号",
			Method:      "POST",
		},
	}
	
	// 添加自定义处理器
	for customPath := range a.CustomHandlers {
		endpoints = append(endpoints, PProfInfo{
			Path:        basePrefix + "/" + strings.TrimPrefix(customPath, "/"),
			Description: "自定义性能分析处理器",
			Method:      "GET",
		})
	}
	
	return endpoints
}

// RegisterScenarios 注册性能测试场景
func (a *PProfConfigAdapter) RegisterScenarios() {
	if a.scenarios != nil {
		a.scenarios.RegisterScenariosToAdapter(a)
	}
}

// RegisterCustomHandler 注册自定义处理器
func (a *PProfConfigAdapter) RegisterCustomHandler(path string, handler http.HandlerFunc) {
	if a.CustomHandlers == nil {
		a.CustomHandlers = make(map[string]http.HandlerFunc)
	}
	a.CustomHandlers[path] = handler
}

// isIPAllowed 检查IP是否被允许访问
func (a *PProfConfigAdapter) isIPAllowed(clientIP string) bool {
	// 如果没有配置允许的IP列表，则允许所有IP
	if len(a.AllowedIPs) == 0 {
		return true
	}
	
	for _, allowedIP := range a.AllowedIPs {
		if allowedIP == clientIP || allowedIP == "*" {
			return true
		}
		// 这里可以扩展支持CIDR格式
	}
	
	return false
}

// isAuthenticated 检查是否已认证
func (a *PProfConfigAdapter) isAuthenticated(r *http.Request) bool {
	if !a.RequireAuth {
		return true
	}
	
	// 检查Authorization头
	authHeader := r.Header.Get(constants.HeaderAuthorization)
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			return token == a.AuthToken
		}
	}
	
	// 检查查询参数中的token
	token := r.URL.Query().Get("token")
	if token != "" {
		return token == a.AuthToken
	}
	
	return false
}

// logAccess 记录访问日志
func (a *PProfConfigAdapter) logAccess(r *http.Request, startTime time.Time, statusCode int, message string) {
	if !a.EnableLogging || a.logger == nil {
		return
	}
	
	duration := time.Since(startTime)
	clientIP := getClientIP(r)
	
	a.logger.InfoKV("pprof access",
		"method", r.Method,
		"path", r.URL.Path,
		"client_ip", clientIP,
		"status_code", statusCode,
		"duration", duration,
		"user_agent", r.UserAgent(),
		"message", message,
	)
}

// PProfMiddleware 创建pprof中间件
func PProfMiddleware(configAdapter *PProfConfigAdapter) HTTPMiddleware {
	if configAdapter == nil {
		config := DefaultPProfConfig()
		configAdapter = NewPProfConfigAdapter(config)
	}
	
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()
			
			// 检查是否启用pprof
			if !configAdapter.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			
			// 检查是否为pprof路径
			if !strings.HasPrefix(r.URL.Path, configAdapter.PathPrefix) {
				next.ServeHTTP(w, r)
				return
			}
			
			// 检查IP白名单
			clientIP := getClientIP(r)
			if !configAdapter.isIPAllowed(clientIP) {
				configAdapter.logAccess(r, startTime, http.StatusForbidden, "IP not allowed")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			
			// 检查认证
			if !configAdapter.isAuthenticated(r) {
				configAdapter.logAccess(r, startTime, http.StatusUnauthorized, "Authentication required")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			
			// 设置超时
			if configAdapter.Timeout > 0 {
				// 这里可以添加超时控制
			}
			
			// 处理pprof请求
			pprofPath := strings.TrimPrefix(r.URL.Path, configAdapter.PathPrefix)
			pprofPath = strings.TrimPrefix(pprofPath, "/")
			
			configAdapter.logAccess(r, startTime, http.StatusOK, fmt.Sprintf("Accessing pprof: %s", pprofPath))
			
			// 根据路径分发到对应的pprof处理器
			switch pprofPath {
			case "", "index":
				pprof.Index(w, r)
			case "allocs":
				pprof.Handler("allocs").ServeHTTP(w, r)
			case "block":
				pprof.Handler("block").ServeHTTP(w, r)
			case "cmdline":
				pprof.Cmdline(w, r)
			case "goroutine":
				pprof.Handler("goroutine").ServeHTTP(w, r)
			case "heap":
				pprof.Handler("heap").ServeHTTP(w, r)
			case "mutex":
				pprof.Handler("mutex").ServeHTTP(w, r)
			case "profile":
				pprof.Profile(w, r)
			case "threadcreate":
				pprof.Handler("threadcreate").ServeHTTP(w, r)
			case "trace":
				pprof.Trace(w, r)
			case "symbol":
				pprof.Symbol(w, r)
			default:
				// 检查自定义处理器
				if handler, exists := configAdapter.CustomHandlers[pprofPath]; exists {
					handler(w, r)
				} else {
					http.NotFound(w, r)
				}
			}
		})
	}
}

// CreatePProfHandler 创建独立的pprof处理器（不作为中间件使用）
func CreatePProfHandler(configAdapter *PProfConfigAdapter) http.Handler {
	if configAdapter == nil {
		config := DefaultPProfConfig()
		configAdapter = NewPProfConfigAdapter(config)
	}
	
	mux := http.NewServeMux()
	
	// 注册标准pprof处理器
	mux.HandleFunc(configAdapter.PathPrefix+"/", pprof.Index)
	mux.HandleFunc(configAdapter.PathPrefix+"/allocs", pprof.Handler("allocs").ServeHTTP)
	mux.HandleFunc(configAdapter.PathPrefix+"/block", pprof.Handler("block").ServeHTTP)
	mux.HandleFunc(configAdapter.PathPrefix+"/cmdline", pprof.Cmdline)
	mux.HandleFunc(configAdapter.PathPrefix+"/goroutine", pprof.Handler("goroutine").ServeHTTP)
	mux.HandleFunc(configAdapter.PathPrefix+"/heap", pprof.Handler("heap").ServeHTTP)
	mux.HandleFunc(configAdapter.PathPrefix+"/mutex", pprof.Handler("mutex").ServeHTTP)
	mux.HandleFunc(configAdapter.PathPrefix+"/profile", pprof.Profile)
	mux.HandleFunc(configAdapter.PathPrefix+"/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
	mux.HandleFunc(configAdapter.PathPrefix+"/trace", pprof.Trace)
	mux.HandleFunc(configAdapter.PathPrefix+"/symbol", pprof.Symbol)
	
	// 注册自定义处理器
	for customPath, handler := range configAdapter.CustomHandlers {
		fullPath := path.Join(configAdapter.PathPrefix, customPath)
		mux.HandleFunc(fullPath, handler)
	}
	
	return mux
}
