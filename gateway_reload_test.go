/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-15 12:57:40
 * @FilePath: \go-rpc-gateway\gateway_reload_test.go
 * @Description: 测试Gateway的配置重新加载功能
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package gateway

import (
	"testing"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/stretchr/testify/assert"
)

func TestHTTPRuntimeChangedIgnoresEndpoint(t *testing.T) {
	oldConfig := gwconfig.Default()
	newConfig := gwconfig.Default()

	oldConfig.HTTPServer.Endpoint = "127.0.0.1:8080"
	newConfig.HTTPServer.Endpoint = "127.0.0.1:18080"

	// endpoint-only changes should not trigger HTTP reload
	assert.False(t, httpRuntimeChanged(oldConfig, newConfig), "endpoint-only changes should not trigger HTTP reload")
}

func TestHTTPRuntimeChangedDetectsPort(t *testing.T) {
	oldConfig := gwconfig.Default()
	newConfig := gwconfig.Default()
	newConfig.HTTPServer.Port = oldConfig.HTTPServer.Port + 1

	// HTTP port changes should trigger HTTP reload
	assert.True(t, httpRuntimeChanged(oldConfig, newConfig), "HTTP port changes should trigger HTTP reload")
}

func TestGRPCRuntimeChangedDetectsPort(t *testing.T) {
	oldConfig := gwconfig.Default()
	newConfig := gwconfig.Default()
	newConfig.GRPC.Server.Port = oldConfig.GRPC.Server.Port + 1

	// gRPC port changes should trigger gRPC reload
	assert.True(t, grpcRuntimeChanged(oldConfig, newConfig), "gRPC port changes should trigger gRPC reload")
}

func TestMergeGatewayConfigWithDefaultsRefreshesEndpoints(t *testing.T) {
	config := &gwconfig.Gateway{
		HTTPServer: &gwconfig.HTTPServer{
			Host: "0.0.0.0",
			Port: 28080,
		},
		GRPC: &gwconfig.GRPC{
			Server: &gwconfig.GRPCServer{
				Enable: true,
				Host:   "0.0.0.0",
				Port:   29090,
			},
		},
	}

	merged := mergeGatewayConfigWithDefaults(config)

	assert.Equal(t, "http://0.0.0.0:28080", merged.HTTPServer.GetEndpoint())
	assert.Equal(t, "0.0.0.0:29090", merged.GRPC.Server.GetEndpoint())
}

func TestSwaggerRuntimeChangedDetectsEnabled(t *testing.T) {
	oldConfig := gwconfig.Default()
	newConfig := gwconfig.Default()
	newConfig.Swagger.Enabled = !oldConfig.Swagger.Enabled

	// Swagger changes should trigger HTTP gateway reload
	assert.True(t, swaggerRuntimeChanged(oldConfig, newConfig), "Swagger changes should trigger HTTP gateway reload")
}

func TestPProfRuntimeChangedDetectsPort(t *testing.T) {
	oldConfig := gwconfig.Default()
	newConfig := gwconfig.Default()
	newConfig.Middleware.PProf.Port = oldConfig.Middleware.PProf.Port + 1

	// PProf port changes should trigger PProf reload
	assert.True(t, pprofRuntimeChanged(oldConfig, newConfig), "PProf port changes should trigger PProf reload")
}
