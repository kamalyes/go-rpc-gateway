/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2024-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\gateway.go
 * @Description: Gateway主入口，基于go-config和go-core重构
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

// Package gateway 提供一个轻量级的gRPC-Gateway框架
// 集成了数据库、Redis和对象存储等组件
// 基于go-config和go-core架构
package gateway

import (
	"context"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	goconfig "github.com/kamalyes/go-config"
	"github.com/kamalyes/go-config/pkg/database"
	"github.com/kamalyes/go-config/pkg/oss"
	"github.com/kamalyes/go-config/pkg/redis"
	"github.com/kamalyes/go-rpc-gateway/internal/config"
	"github.com/kamalyes/go-rpc-gateway/internal/server"
	"google.golang.org/grpc"
)

// Gateway 是主要的网关服务器
type Gateway struct {
	*server.Server
}

// Config 是网关配置的别名
type Config = config.GatewayConfig

// SingleConfig 是go-config单例配置的别名
type SingleConfig = goconfig.SingleConfig

// DatabaseConfig 是数据库配置的别名
type DatabaseConfig = database.MySQL

// RedisConfig 是Redis配置的别名
type RedisConfig = redis.Redis

// OSSConfig 是对象存储配置的别名
type OSSConfig = oss.Minio

// ServiceRegisterFunc gRPC服务注册函数类型
type ServiceRegisterFunc func(*grpc.Server)

// HandlerRegisterFunc HTTP处理器注册函数类型
type HandlerRegisterFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

// New 创建新的网关实例
func New(cfg ...*Config) (*Gateway, error) {
	var gatewayConfig *Config
	if len(cfg) > 0 && cfg[0] != nil {
		gatewayConfig = cfg[0]
	} else {
		gatewayConfig = DefaultConfig()
	}

	srv, err := server.NewServer(gatewayConfig)
	if err != nil {
		return nil, err
	}

	return &Gateway{
		Server: srv,
	}, nil
}

// NewWithConfigFile 使用配置文件创建Gateway实例
func NewWithConfigFile(configPath string) (*Gateway, error) {
	// 创建配置管理器
	configManager, err := config.NewConfigManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	// 使用配置管理器创建服务器
	srv, err := server.NewServerWithConfigManager(configManager)
	if err != nil {
		return nil, err
	}

	return &Gateway{
		Server: srv,
	}, nil
}

// RegisterService 注册gRPC服务
func (g *Gateway) RegisterService(registerFunc ServiceRegisterFunc) {
	g.Server.RegisterGRPCService(registerFunc)
}

// RegisterHandler 注册HTTP处理器
func (g *Gateway) RegisterHandler(pattern string, handler http.Handler) {
	g.Server.RegisterHTTPRoute(pattern, handler)
}

// RegisterHTTPRoute 注册HTTP路由 (便捷方法)
func (g *Gateway) RegisterHTTPRoute(pattern string, handlerFunc http.HandlerFunc) {
	g.Server.RegisterHTTPRoute(pattern, handlerFunc)
}

// RegisterHTTPRoutes 批量注册HTTP路由
func (g *Gateway) RegisterHTTPRoutes(routes map[string]http.HandlerFunc) {
	for pattern, handler := range routes {
		g.RegisterHTTPRoute(pattern, handler)
	}
}

// GetConfig 获取网关配置
func (g *Gateway) GetConfig() *config.GatewayConfig {
	return g.Server.GetConfig()
}

// Start 启动网关服务
func (g *Gateway) Start() error {
	return g.Server.Start()
}

// Stop 停止网关服务
func (g *Gateway) Stop() error {
	return g.Server.Stop()
}

// DefaultConfig 返回默认网关配置
func DefaultConfig() *Config {
	return config.DefaultGatewayConfig()
}

// DefaultDatabaseConfig 返回默认数据库配置
func DefaultDatabaseConfig() *DatabaseConfig {
	return &database.MySQL{
		Host:         "127.0.0.1",
		Port:         "3306",
		Config:       "charset=utf8mb4&parseTime=True&loc=Local",
		Dbname:       "gateway",
		Username:     "root",
		Password:     "",
		MaxIdleConns: 10,
		MaxOpenConns: 100,
		LogLevel:     "info",
	}
}

// DefaultRedisConfig 返回默认Redis配置
func DefaultRedisConfig() *RedisConfig {
	return &redis.Redis{
		DB:       0,
		Addr:     "127.0.0.1:6379",
		Password: "",
	}
}

// DefaultOSSConfig 返回默认OSS配置
func DefaultOSSConfig() *OSSConfig {
	return &oss.Minio{
		Host:      "127.0.0.1",
		Port:      9000,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
	}
}

// NewWithSingleConfig 使用单一配置创建Gateway实例
func NewWithSingleConfig(cfg *SingleConfig) (*Gateway, error) {
	gatewayConfig := DefaultConfig()
	gatewayConfig.SingleConfig = cfg

	return New(gatewayConfig)
}
