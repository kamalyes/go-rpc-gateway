/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 18:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 17:55:28
 * @FilePath: \go-rpc-gateway\middleware\pprof.go
 * @Description: pprof性能分析中间件 - 直接使用 go-config 配置
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strings"

	gopprof "github.com/kamalyes/go-config/pkg/pprof"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"github.com/kamalyes/go-toolbox/pkg/validator"
)

// initSamplingConfig 初始化采样配置
func initSamplingConfig(cfg *gopprof.PProf) {
	if cfg.Sampling == nil {
		return
	}

	// 设置内存采样率
	if cfg.Sampling.MemoryRate > 0 {
		runtime.MemProfileRate = cfg.Sampling.MemoryRate
	}

	// 设置阻塞采样率
	if cfg.EnableProfiles != nil && cfg.EnableProfiles.Block && cfg.Sampling.BlockRate > 0 {
		runtime.SetBlockProfileRate(cfg.Sampling.BlockRate)
	}

	// 设置互斥锁采样比例
	if cfg.EnableProfiles != nil && cfg.EnableProfiles.Mutex && cfg.Sampling.MutexFraction > 0 {
		runtime.SetMutexProfileFraction(cfg.Sampling.MutexFraction)
	}
}

// authenticateRequest 认证请求
func authenticateRequest(cfg *gopprof.PProf, r *http.Request) bool {
	if cfg.Authentication == nil || !cfg.Authentication.Enabled {
		return true
	}

	if !cfg.Authentication.RequireAuth {
		return true
	}

	// 检查认证令牌
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.URL.Query().Get("token")
	}

	// 移除 "Bearer " 前缀
	token = strings.TrimPrefix(token, "Bearer ")

	return token == cfg.Authentication.AuthToken
}

// checkPProfIPWhitelist 检查pprof IP白名单
func checkPProfIPWhitelist(cfg *gopprof.PProf, r *http.Request) bool {
	if cfg.Authentication == nil || len(cfg.Authentication.AllowedIPs) == 0 {
		return true
	}

	clientIP := netx.GetClientIP(r)
	return validator.IsIPAllowed(clientIP, cfg.Authentication.AllowedIPs)
}

// logAccess 记录访问日志
func logAccess(cfg *gopprof.PProf, r *http.Request) {
	if cfg.Gateway == nil || !cfg.Gateway.EnableLogging {
		return
	}

	global.LOGGER.InfoContextKV(r.Context(), "🔍 PProf访问",
		"ip", netx.GetClientIP(r),
		"path", r.URL.Path,
		"method", r.Method)
}

// isProfileEnabled 检查是否启用了对应的性能分析
func isProfileEnabled(cfg *gopprof.PProf, pprofPath string) bool {
	if cfg.EnableProfiles == nil {
		return true
	}

	switch pprofPath {
	case "", "index", "cmdline", "symbol":
		return true
	case "profile":
		return cfg.EnableProfiles.CPU
	case "heap":
		return cfg.EnableProfiles.Heap
	case "allocs":
		return cfg.EnableProfiles.Allocs
	case "goroutine":
		return cfg.EnableProfiles.Goroutine
	case "block":
		return cfg.EnableProfiles.Block
	case "mutex":
		return cfg.EnableProfiles.Mutex
	case "threadcreate":
		return cfg.EnableProfiles.ThreadCreate
	case "trace":
		return cfg.EnableProfiles.Trace
	default:
		return false
	}
}

// createAuthMiddleware 创建认证中间件
func createAuthMiddleware(cfg *gopprof.PProf) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 认证检查
			if !authenticateRequest(cfg, r) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// IP白名单检查
			if !checkPProfIPWhitelist(cfg, r) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// 记录访问日志
			logAccess(cfg, r)

			// 检查Profile是否启用
			pprofPath := strings.TrimPrefix(r.URL.Path, cfg.PathPrefix)
			pprofPath = strings.TrimPrefix(pprofPath, "/")

			if !isProfileEnabled(cfg, pprofPath) {
				http.Error(w, "Profile not enabled", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// registerPProfHandlers 注册pprof处理器到mux
func registerPProfHandlers(mux *http.ServeMux, pathPrefix string) {
	mux.HandleFunc(pathPrefix+"/", pprof.Index)
	mux.HandleFunc(pathPrefix+"/allocs", pprof.Handler("allocs").ServeHTTP)
	mux.HandleFunc(pathPrefix+"/block", pprof.Handler("block").ServeHTTP)
	mux.HandleFunc(pathPrefix+"/cmdline", pprof.Cmdline)
	mux.HandleFunc(pathPrefix+"/goroutine", pprof.Handler("goroutine").ServeHTTP)
	mux.HandleFunc(pathPrefix+"/heap", pprof.Handler("heap").ServeHTTP)
	mux.HandleFunc(pathPrefix+"/mutex", pprof.Handler("mutex").ServeHTTP)
	mux.HandleFunc(pathPrefix+"/profile", pprof.Profile)
	mux.HandleFunc(pathPrefix+"/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
	mux.HandleFunc(pathPrefix+"/trace", pprof.Trace)
	mux.HandleFunc(pathPrefix+"/symbol", pprof.Symbol)
}

// StartPProfServer 启动独立的pprof服务器（在单独的端口）
// 这个函数应该在 goroutine 中调用
func StartPProfServer(cfg *gopprof.PProf) error {
	if !cfg.Enabled {
		return nil
	}

	cfg.Port = mathx.IfNotZero(cfg.Port, 6060)

	initSamplingConfig(cfg)

	mux := http.NewServeMux()
	registerPProfHandlers(mux, cfg.PathPrefix)

	// 包装认证中间件
	authMiddleware := createAuthMiddleware(cfg)
	handler := authMiddleware(mux)

	addr := fmt.Sprintf(":%d", cfg.Port)

	global.LOGGER.InfoKV("🔍 PProf服务器启动",
		"地址", addr,
		"路径", cfg.PathPrefix)

	return http.ListenAndServe(addr, handler)
}
