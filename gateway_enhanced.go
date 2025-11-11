/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 00:00:00
 * @FilePath: \go-rpc-gateway\gateway_enhanced.go
 * @Description: Gateway 增强 API
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package gateway

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

// RegisterCustomHandler 注册自定义处理器（支持路径参数）
// pattern 示例: "/v1/buckets/{bucket_name}/objects/upload"
func (g *Gateway) RegisterCustomHandler(method, pattern string, handler runtime.HandlerFunc) {
	if g.Server != nil && g.Server.GetGatewayMux() != nil {
		g.Server.GetGatewayMux().HandlePath(method, pattern, handler)
	}
}

// RegisterCustomHandlers 批量注册自定义处理器
// routes 格式: map[pattern]map[method]handler
// 示例:
//
//	routes := map[string]map[string]runtime.HandlerFunc{
//	    "/v1/buckets/{bucket_name}/objects/upload": {
//	        http.MethodPost: uploadHandler,
//	    },
//	    "/v1/buckets/{bucket_name}/objects/{object_key}": {
//	        http.MethodGet: downloadHandler,
//	        http.MethodDelete: deleteHandler,
//	    },
//	}
func (g *Gateway) RegisterCustomHandlers(routes map[string]map[string]runtime.HandlerFunc) {
	for pattern, methods := range routes {
		for method, handler := range methods {
			g.RegisterCustomHandler(method, pattern, handler)
		}
	}
}

// RegisterHealthCheck 注册健康检查端点
func (g *Gateway) RegisterHealthCheck(path string, handler http.HandlerFunc) {
	g.RegisterHTTPRoute(path, handler)
}

// RegisterVersionEndpoint 注册版本信息端点
func (g *Gateway) RegisterVersionEndpoint(path string, version, gitBranch, gitHash, buildTime string) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]string{
			"version":    version,
			"git_branch": gitBranch,
			"git_hash":   gitHash,
			"build_time": buildTime,
		}
		// 简单的 JSON 编码
		w.Write([]byte("{"))
		w.Write([]byte("\"version\":\"" + response["version"] + "\","))
		w.Write([]byte("\"git_branch\":\"" + response["git_branch"] + "\","))
		w.Write([]byte("\"git_hash\":\"" + response["git_hash"] + "\","))
		w.Write([]byte("\"build_time\":\"" + response["build_time"] + "\""))
		w.Write([]byte("}"))
	}
	g.RegisterHTTPRoute(path, handler)
}

// GetGatewayMux 获取 Gateway Mux（用于高级自定义）
func (g *Gateway) GetGatewayMux() *runtime.ServeMux {
	if g.Server != nil {
		return g.Server.GetGatewayMux()
	}
	return nil
}
