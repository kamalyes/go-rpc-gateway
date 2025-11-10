/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:15:15
 * @FilePath: \go-rpc-gateway\config\defaults.go
 * @Description: 默认配置和配置管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

import (
	goconfig "github.com/kamalyes/go-config"
	"github.com/kamalyes/go-config/pkg/cache"
	"github.com/kamalyes/go-config/pkg/cors"
	"github.com/kamalyes/go-config/pkg/jwt"
	"github.com/kamalyes/go-config/pkg/register"
	zapconfig "github.com/kamalyes/go-config/pkg/zap"
	"github.com/kamalyes/go-rpc-gateway/constants"
)

// DefaultGatewayConfig 返回默认Gateway配置，使用go-config的链式调用风格
func DefaultGatewayConfig() *GatewayConfig {
	// 创建go-config的默认单例配置，使用优雅的链式调用
	defaultSingleConfig := &goconfig.SingleConfig{
		// Server配置 - 使用链式调用
		Server: *register.Default().
			WithModuleName(constants.DefaultServiceName).
			WithEndpoint(":8080").
			WithServerName(constants.DefaultServiceName).
			WithDataDriver("memory").
			WithContextPath("/").
			WithLanguage("zh-cn"),

		// CORS配置 - 使用链式调用
		Cors: *cors.Default().
			WithModuleName("cors").
			WithAllowedOrigins([]string{"*"}).
			WithAllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}).
			WithAllowedHeaders([]string{"*"}).
			WithMaxAge("86400").
			WithAllowedAllOrigins(true).
			WithAllowCredentials(true).
			WithOptionsResponseCode(200),

		// JWT配置 - 使用链式调用
		JWT: *jwt.Default().
			WithModuleName("jwt").
			WithSigningKey("go-rpc-gateway-default-key").
			WithExpiresTime(3600).
			WithBufferTime(300).
			WithUseMultipoint(false),

		// Cache配置 - 使用链式调用
		Cache: *cache.Default().
			WithModuleName("cache").
			WithType(cache.TypeMemory).
			WithEnabled(true).
			WithKeyPrefix("gateway:").
			WithSerializer("json"),

		// PProf配置 - 使用链式调用
		Pprof: *register.DefaultPProfConfig().
			WithEnabled(true).
			WithPathPrefix("/debug/pprof").
			WithRequireAuth(false),

		// Jaeger配置 - 使用链式调用
		Jaeger: *register.DefaultJaegerConfig().
			WithType("const").
			WithParam(1).
			WithLogSpans(false).
			WithEndpoint("http://localhost:14268/api/traces").
			WithService(constants.DefaultServiceName).
			WithModuleName("jaeger"),

		// Zap日志配置 - 使用链式调用
		Zap: *zapconfig.Default().
			WithModuleName("zap").
			WithLevel("info").
			WithFormat("console").
			WithPrefix("[GO-RPC-GATEWAY]").
			WithDirector("logs").
			WithMaxSize(100).
			WithMaxAge(7).
			WithMaxBackups(5).
			WithCompress(true).
			WithShowLine(true).
			WithEncodeLevel("LowercaseColorLevelEncoder").
			WithStacktraceKey("stacktrace").
			WithLogInConsole(true).
			WithDevelopment(true),
	}

	return &GatewayConfig{
		SingleConfig: defaultSingleConfig,
		Gateway: GatewaySettings{
			Name:        constants.DefaultServiceName,
			Version:     "v1.0.0",
			Environment: "development",
			Debug:       true,
			HTTP: HTTPConfig{
				Host:               "0.0.0.0",
				Port:               8080,
				ReadTimeout:        30,
				WriteTimeout:       30,
				IdleTimeout:        120,
				MaxHeaderBytes:     1048576,
				EnableGzipCompress: true,
			},
			GRPC: GRPCConfig{
				Host:              "0.0.0.0",
				Port:              9090,
				Network:           "tcp",
				MaxRecvMsgSize:    4194304,
				MaxSendMsgSize:    4194304,
				ConnectionTimeout: 30,
				KeepaliveTime:     60,
				KeepaliveTimeout:  30,
				EnableReflection:  true,
			},
			HealthCheck: HealthCheckConfig{
				Enabled:  true,
				Path:     constants.DefaultHealthPath,
				Interval: 30,
			},
		},
		Middleware: MiddlewareConfig{
			RateLimit: RateLimitConfig{
				Enabled:    true,
				Algorithm:  "token_bucket",
				Rate:       100,
				Burst:      200,
				WindowSize: 60,
			},
			AccessLog: AccessLogConfig{
				Enabled:        true,
				Format:         "json",
				IncludeBody:    false,
				IncludeHeaders: false,
			},
			Signature: SignatureConfig{
				Enabled:   false,
				Algorithm: "hmac-sha256",
				SkipPaths: []string{constants.DefaultHealthPath, constants.DefaultMetricsPath},
				TTL:       300,
			},
		},
		Monitoring: MonitoringConfig{
			Metrics: MetricsConfig{
				Enabled:   true,
				Path:      constants.DefaultMetricsPath,
				Port:      9100,
				Namespace: "gateway",
				Subsystem: "rpc",
			},
			Tracing: TracingConfig{
				Enabled: false,
				Exporter: TracingExporterConfig{
					Type:     "jaeger",
					Endpoint: "http://localhost:14268/api/traces",
				},
				Sampler: TracingSamplerConfig{
					Type:        "probability",
					Probability: 0.1,
				},
				Resource: TracingResourceConfig{
					ServiceName:    constants.DefaultServiceName,
					ServiceVersion: "v1.0.0",
					Environment:    "development",
				},
			},
		},
		Security: SecurityConfig{
			TLS: TLSConfig{
				Enabled: false,
			},
			Security: SecurityPolicy{
				EnableCSRFProtection: false,
				EnableXSSProtection:  true,
				ContentTypeNoSniff:   true,
				FrameOptions:         "DENY",
			},
		},
	}
}
