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
	"net/http"
	"net/http/pprof"
	"strings"

	gopprof "github.com/kamalyes/go-config/pkg/pprof"
)

// PProfMiddleware 创建pprof中间件
func PProfMiddleware(cfg *gopprof.PProf) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查是否启用pprof
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 检查是否为pprof路径
			if !strings.HasPrefix(r.URL.Path, cfg.PathPrefix) {
				next.ServeHTTP(w, r)
				return
			}

			// 处理pprof请求
			pprofPath := strings.TrimPrefix(r.URL.Path, cfg.PathPrefix)
			pprofPath = strings.TrimPrefix(pprofPath, "/")

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
				http.NotFound(w, r)
			}
		})
	}
}

// CreatePProfHandler 创建独立的pprof处理器（不作为中间件使用）
func CreatePProfHandler(cfg *gopprof.PProf) http.Handler {
	mux := http.NewServeMux()

	// 注册标准pprof处理器
	mux.HandleFunc(cfg.PathPrefix+"/", pprof.Index)
	mux.HandleFunc(cfg.PathPrefix+"/allocs", pprof.Handler("allocs").ServeHTTP)
	mux.HandleFunc(cfg.PathPrefix+"/block", pprof.Handler("block").ServeHTTP)
	mux.HandleFunc(cfg.PathPrefix+"/cmdline", pprof.Cmdline)
	mux.HandleFunc(cfg.PathPrefix+"/goroutine", pprof.Handler("goroutine").ServeHTTP)
	mux.HandleFunc(cfg.PathPrefix+"/heap", pprof.Handler("heap").ServeHTTP)
	mux.HandleFunc(cfg.PathPrefix+"/mutex", pprof.Handler("mutex").ServeHTTP)
	mux.HandleFunc(cfg.PathPrefix+"/profile", pprof.Profile)
	mux.HandleFunc(cfg.PathPrefix+"/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
	mux.HandleFunc(cfg.PathPrefix+"/trace", pprof.Trace)
	mux.HandleFunc(cfg.PathPrefix+"/symbol", pprof.Symbol)

	return mux
}
