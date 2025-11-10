/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 13:05:35
 * @FilePath: \go-rpc-gateway\middleware\pprof_gateway.go
 * @Description: pprof网关集成功能 - Gateway的pprof相关便捷方法和Web界面
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"fmt"
	"net/http"
	"os"

	"github.com/kamalyes/go-config/pkg/register"
	"github.com/kamalyes/go-rpc-gateway/constants"
)

// PProfOptions pprof配置选项
type PProfOptions struct {
	Enabled       bool     `json:"enabled"`        // 是否启用pprof
	AuthToken     string   `json:"auth_token"`     // 认证令牌
	AllowedIPs    []string `json:"allowed_ips"`    // 允许的IP列表
	PathPrefix    string   `json:"path_prefix"`    // 路径前缀
	DevModeOnly   bool     `json:"dev_mode_only"`  // 是否只在开发模式启用
	EnableLogging bool     `json:"enable_logging"` // 是否启用日志
	Timeout       int      `json:"timeout"`        // 超时时间(秒)
}

// PProfGatewayConfig Gateway的pprof配置
type PProfGatewayConfig struct {
	adapter                *PProfConfigAdapter
	scenarios              *PProfScenarios
	enabled                bool
	webInterfaceRegistered bool
}

// NewPProfGatewayConfig 创建Gateway pprof配置
func NewPProfGatewayConfig() *PProfGatewayConfig {
	defaultConfig := DefaultPProfConfig()
	return &PProfGatewayConfig{
		adapter:                NewPProfConfigAdapter(defaultConfig),
		scenarios:              NewPProfScenarios(),
		enabled:                false,
		webInterfaceRegistered: false,
	}
}

// getEnvOrDefault 获取环境变量或返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// EnablePProf 启用pprof性能分析功能
// 这是一个简化的API，使用默认配置启用pprof
func (cfg *PProfGatewayConfig) EnablePProf() *PProfGatewayConfig {
	return cfg.EnablePProfWithOptions(PProfOptions{
		Enabled:       true,
		AuthToken:     getEnvOrDefault("PPROF_TOKEN", constants.PProfDefaultAuthToken),
		PathPrefix:    constants.PProfBasePath,
		DevModeOnly:   false,
		AllowedIPs:    []string{}, // 默认允许所有IP
		EnableLogging: true,
		Timeout:       30,
	})
}

// EnablePProfWithOptions 使用自定义选项启用pprof
func (cfg *PProfGatewayConfig) EnablePProfWithOptions(options PProfOptions) *PProfGatewayConfig {
	// 更新pprof配置
	cfg.adapter.PProf.Enabled = options.Enabled
	cfg.adapter.PProf.AuthToken = options.AuthToken
	cfg.adapter.PProf.AllowedIPs = options.AllowedIPs
	cfg.adapter.PProf.RequireAuth = options.AuthToken != ""
	cfg.adapter.PProf.EnableLogging = options.EnableLogging
	cfg.adapter.PProf.Timeout = options.Timeout

	if options.PathPrefix != "" {
		cfg.adapter.PProf.PathPrefix = options.PathPrefix
	}

	cfg.enabled = options.Enabled

	// 注册性能测试场景
	if cfg.scenarios != nil && options.Enabled {
		cfg.scenarios.RegisterScenariosToAdapter(cfg.adapter)
	}

	return cfg
}

// EnablePProfWithToken 使用指定token启用pprof (便捷方法)
func (cfg *PProfGatewayConfig) EnablePProfWithToken(token string) *PProfGatewayConfig {
	return cfg.EnablePProfWithOptions(PProfOptions{
		Enabled:       true,
		AuthToken:     token,
		PathPrefix:    constants.PProfBasePath,
		AllowedIPs:    []string{},
		EnableLogging: true,
		Timeout:       30,
	})
}

// EnablePProfForDevelopment 启用开发环境pprof (便捷方法)
func (cfg *PProfGatewayConfig) EnablePProfForDevelopment() *PProfGatewayConfig {
	return cfg.EnablePProfWithOptions(PProfOptions{
		Enabled:       true,
		AuthToken:     "dev-debug-token",
		PathPrefix:    constants.PProfBasePath,
		DevModeOnly:   true,
		AllowedIPs:    []string{"127.0.0.1", "::1"},
		EnableLogging: true,
		Timeout:       30,
	})
}

// GetPProfConfig 获取pprof配置
func (cfg *PProfGatewayConfig) GetPProfConfig() *register.PProf {
	if cfg.adapter != nil {
		return cfg.adapter.PProf
	}
	return nil
}

// GetPProfAdapter 获取pprof适配器
func (cfg *PProfGatewayConfig) GetPProfAdapter() *PProfConfigAdapter {
	return cfg.adapter
}

// IsPProfEnabled 检查pprof是否启用
func (cfg *PProfGatewayConfig) IsPProfEnabled() bool {
	return cfg.enabled && cfg.adapter != nil && cfg.adapter.PProf.Enabled
}

// GetPProfEndpoints 获取所有可用的pprof端点信息
func (cfg *PProfGatewayConfig) GetPProfEndpoints() []PProfInfo {
	if !cfg.IsPProfEnabled() {
		return []PProfInfo{}
	}
	return cfg.adapter.GetAvailableEndpoints()
}

// CreatePProfStatusAPIHandler 创建pprof状态API处理器
func (cfg *PProfGatewayConfig) CreatePProfStatusAPIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		statusJSON := fmt.Sprintf(`{
			"pprof_enabled": %t,
			"pprof_path": "%s",
			"auth_required": %t,
			"endpoints_count": %d
		}`,
			cfg.IsPProfEnabled(),
			cfg.adapter.PProf.PathPrefix,
			cfg.adapter.PProf.RequireAuth,
			len(cfg.GetPProfEndpoints()))

		w.Write([]byte(statusJSON))
	}
}

// CreatePProfWebInterface 创建pprof Web界面处理器
func (cfg *PProfGatewayConfig) CreatePProfWebInterface() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !cfg.IsPProfEnabled() {
			http.Error(w, "PProf is not enabled", http.StatusNotFound)
			return
		}

		endpoints := cfg.GetPProfEndpoints()
		authInfo := ""
		if cfg.adapter.PProf.RequireAuth {
			authInfo = fmt.Sprintf(`
			<div class="auth-info">
				<h3>🔐 认证信息</h3>
				<p>访问pprof端点需要认证，使用以下方式之一：</p>
				<ul>
					<li><strong>Header:</strong> <code>Authorization: Bearer %s</code></li>
					<li><strong>Query:</strong> <code>?token=%s</code></li>
				</ul>
			</div>`, cfg.adapter.PProf.AuthToken, cfg.adapter.PProf.AuthToken)
		}

		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Go RPC Gateway - PProf Dashboard</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; background: #f5f7fa; }
		.container { max-width: 1200px; margin: 0 auto; padding: 20px; }
		.header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; border-radius: 10px; margin-bottom: 30px; }
		.header h1 { margin: 0; font-size: 2.5em; }
		.header p { margin: 10px 0 0; opacity: 0.9; }
		.auth-info { background: #fff3cd; padding: 20px; border-radius: 8px; margin: 20px 0; border-left: 4px solid #ffc107; }
		.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
		.card { background: white; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); overflow: hidden; }
		.card-header { background: #f8f9fa; padding: 20px; border-bottom: 1px solid #e9ecef; }
		.card-header h3 { margin: 0; color: #495057; }
		.card-body { padding: 20px; }
		.endpoint { margin: 10px 0; padding: 15px; background: #f8f9fa; border-radius: 5px; border-left: 4px solid #007bff; }
		.endpoint strong { color: #007bff; }
		.scenario { margin: 10px 0; padding: 12px; background: #e8f5e8; border-radius: 5px; border-left: 4px solid #28a745; }
		.scenario a { color: #28a745; text-decoration: none; font-weight: 500; }
		.scenario a:hover { text-decoration: underline; }
		code { background: #e9ecef; padding: 4px 8px; border-radius: 4px; font-family: 'Monaco', 'Courier New', monospace; }
		.usage { background: #f8f9fa; padding: 20px; border-radius: 8px; margin: 20px 0; }
		.usage pre { background: #2d3748; color: #e2e8f0; padding: 15px; border-radius: 5px; overflow-x: auto; }
		.footer { text-align: center; margin: 40px 0 20px; color: #6c757d; }
		.status-badge { display: inline-block; padding: 4px 12px; border-radius: 20px; font-size: 0.8em; font-weight: bold; }
		.status-enabled { background: #d4edda; color: #155724; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>🚀 Go RPC Gateway</h1>
			<p>集成了pprof性能分析的微服务网关 <span class="status-badge status-enabled">PProf Enabled</span></p>
		</div>
		
		%s
		
		<div class="grid">
			<div class="card">
				<div class="card-header">
					<h3>📊 标准PProf端点</h3>
				</div>
				<div class="card-body">`, authInfo)

		for _, endpoint := range endpoints {
			if endpoint.Path == cfg.adapter.PProf.PathPrefix+"/" {
				tokenParam := ""
				if cfg.adapter.PProf.RequireAuth {
					tokenParam = "?token=" + cfg.adapter.PProf.AuthToken
				}
				html += fmt.Sprintf(`<div class="endpoint">
					<strong>%s <a href="%s%s">%s</a></strong><br>
					<em>%s</em>
				</div>`, endpoint.Method, endpoint.Path, tokenParam, endpoint.Path, endpoint.Description)
			} else {
				html += fmt.Sprintf(`<div class="endpoint">
					<strong>%s %s</strong><br>
					<em>%s</em>
				</div>`, endpoint.Method, endpoint.Path, endpoint.Description)
			}
		}

		// 如果启用认证，生成带token的测试场景链接
		tokenParam := ""
		if cfg.adapter.PProf.RequireAuth {
			tokenParam = "?token=" + cfg.adapter.PProf.AuthToken
		}

		html += fmt.Sprintf(`
				</div>
			</div>
			
			<div class="card">
				<div class="card-header">
					<h3>🧪 GC 测试场景</h3>
				</div>
				<div class="card-body">
					<div class="scenario">📦 <a href="%s/gc/small-objects%s">小对象GC测试</a> - 创建10万个小对象</div>
					<div class="scenario">📦 <a href="%s/gc/large-objects%s">大对象GC测试</a> - 创建1000个1MB对象</div>
					<div class="scenario">⚡ <a href="%s/gc/high-cpu%s">高CPU使用GC测试</a> - 4个goroutine密集计算</div>
					<div class="scenario">🔄 <a href="%s/gc/cyclic-objects%s">循环对象GC测试</a> - 创建循环引用对象</div>
					<div class="scenario">⏰ <a href="%s/gc/short-lived-objects%s">短生命周期对象GC测试</a></div>
					<div class="scenario">🏠 <a href="%s/gc/long-lived-objects%s">长生命周期对象GC测试</a></div>
					<div class="scenario">🌳 <a href="%s/gc/complex-structure%s">复杂结构GC测试</a> - 二叉树结构</div>
					<div class="scenario">🔀 <a href="%s/gc/concurrent%s">并发GC测试</a> - 10个并发goroutine</div>
				</div>
			</div>
			
			<div class="card">
				<div class="card-header">
					<h3>🔧 其他测试场景</h3>
				</div>
				<div class="card-body">
					<div class="scenario">💾 <a href="%s/memory/allocate%s">内存分配测试</a></div>
					<div class="scenario">🔋 <a href="%s/cpu/intensive%s">CPU密集测试</a></div>
					<div class="scenario">♻️ <a href="%s/cpu/recursive%s">递归计算测试</a></div>
					<div class="scenario">🧵 <a href="%s/goroutine/spawn%s">Goroutine创建测试</a></div>
					<div class="scenario">🔒 <a href="%s/mutex/contention%s">互斥锁竞争测试</a></div>
					<div class="scenario">🧹 <a href="%s/cleanup/all%s">清理所有对象</a></div>
				</div>
			</div>
		</div>
		
		<div class="usage">
			<h3>🛠️ 快速使用指南</h3>
			<h4>命令行分析工具</h4>
			<pre><code># CPU性能分析 (30秒采样)
curl -H "Authorization: Bearer %s" "http://localhost:8080%s/profile?seconds=30" -o cpu.prof
go tool pprof cpu.prof

# 内存分析  
curl -H "Authorization: Bearer %s" "http://localhost:8080%s/heap" -o heap.prof
go tool pprof heap.prof

# Web界面分析
go tool pprof -http=:8081 cpu.prof</code></pre>
		</div>
		
		<div class="footer">
			<p>🔍 访问 <a href="%s/%s">PProf 索引页面</a> 查看更多选项</p>
			<p>💡 内置到 go-rpc-gateway，一键启用性能分析</p>
		</div>
	</div>
</body>
</html>`,
			// GC测试场景链接
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			// 其他测试场景链接
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			cfg.adapter.PProf.PathPrefix, tokenParam,
			// 使用指南
			cfg.adapter.PProf.AuthToken, cfg.adapter.PProf.PathPrefix,
			cfg.adapter.PProf.AuthToken, cfg.adapter.PProf.PathPrefix,
			// footer
			cfg.adapter.PProf.PathPrefix, tokenParam)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}
}

// GetMiddleware 获取pprof中间件
func (cfg *PProfGatewayConfig) GetMiddleware() HTTPMiddleware {
	if cfg.adapter != nil {
		return PProfMiddleware(cfg.adapter)
	}
	return func(next http.Handler) http.Handler {
		return next
	}
}

// GetHandler 获取独立的pprof处理器
func (cfg *PProfGatewayConfig) GetHandler() http.Handler {
	if cfg.adapter != nil {
		return CreatePProfHandler(cfg.adapter)
	}
	return http.NotFoundHandler()
}
