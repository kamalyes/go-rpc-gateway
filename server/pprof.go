/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 15:02:09
 * @FilePath: \go-rpc-gateway\server\pprof.go
 * @Description: PProf 功能实现
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"net/http"
	"net/http/pprof"

	"github.com/kamalyes/go-rpc-gateway/global"
)

// EnablePProf 启用性能分析功能（使用配置文件）
func (s *Server) EnablePProf() error {
	if s.config.Middleware == nil || s.config.Middleware.PProf == nil || !s.config.Middleware.PProf.Enabled {
		return nil
	}
	return s.EnablePProfWithConfig()
}

// EnablePProfWithConfig 使用自定义配置启用性能分析
func (s *Server) EnablePProfWithConfig() error {
	if s.config.Middleware == nil || s.config.Middleware.PProf == nil || !s.config.Middleware.PProf.Enabled {
		return nil
	}

	// 获取路径前缀
	prefix := s.config.Middleware.PProf.PathPrefix

	global.LOGGER.InfoContext(s.ctx, "✅ PProf 性能分析已启用: prefix=%s", prefix)

	// 注册 pprof 路由
	s.RegisterHTTPRoute(prefix+"/", http.HandlerFunc(pprof.Index))
	s.RegisterHTTPRoute(prefix+"/cmdline", http.HandlerFunc(pprof.Cmdline))
	s.RegisterHTTPRoute(prefix+"/profile", http.HandlerFunc(pprof.Profile))
	s.RegisterHTTPRoute(prefix+"/symbol", http.HandlerFunc(pprof.Symbol))
	s.RegisterHTTPRoute(prefix+"/trace", http.HandlerFunc(pprof.Trace))

	// 注册其他 pprof 处理器
	s.RegisterHTTPRoute(prefix+"/allocs", pprof.Handler("allocs"))
	s.RegisterHTTPRoute(prefix+"/block", pprof.Handler("block"))
	s.RegisterHTTPRoute(prefix+"/goroutine", pprof.Handler("goroutine"))
	s.RegisterHTTPRoute(prefix+"/heap", pprof.Handler("heap"))
	s.RegisterHTTPRoute(prefix+"/mutex", pprof.Handler("mutex"))
	s.RegisterHTTPRoute(prefix+"/threadcreate", pprof.Handler("threadcreate"))

	return nil
}
