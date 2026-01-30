/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-01-30 18:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-01-30 21:30:00
 * @FilePath: \go-rpc-gateway\middleware\pprof_test.go
 * @Description: pprof独立服务器测试
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gopprof "github.com/kamalyes/go-config/pkg/pprof"
	"github.com/stretchr/testify/assert"
)

// TestStartPProfServer_WithAuth 测试带认证的服务器
func TestStartPProfServer_WithAuth(t *testing.T) {
	cfg := gopprof.Default().
		Enable().
		WithPort(16061).
		WithPathPrefix("/debug/pprof").
		WithAuthToken("test-token")

	// 确保认证配置正确设置
	if cfg.Authentication == nil {
		cfg.Authentication = &gopprof.AuthConfig{}
	}
	cfg.Authentication.Enabled = true
	cfg.Authentication.RequireAuth = true
	cfg.Authentication.AuthToken = "test-token"

	// 在goroutine中启动服务器
	go func() {
		_ = StartPProfServer(cfg)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 测试无token访问
	resp, err := http.Get("http://localhost:16061/debug/pprof/")
	if err == nil {
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "无token应返回401")
	}

	// 测试有效token访问（Header）
	req, _ := http.NewRequest("GET", "http://localhost:16061/debug/pprof/", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	client := &http.Client{}
	resp, err = client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "有效token应返回200")
	}

	// 测试有效token访问（Query）
	resp, err = http.Get("http://localhost:16061/debug/pprof/?token=test-token")
	if err == nil {
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "query参数token应有效")
	}
}

// TestStartPProfServer_WithIPWhitelist 测试IP白名单
func TestStartPProfServer_WithIPWhitelist(t *testing.T) {
	cfg := gopprof.Default().
		Enable().
		WithPort(16062).
		WithPathPrefix("/debug/pprof").
		WithAllowedIPs([]string{"127.0.0.1", "::1", "192.168.1.0/24"})

	// 确保认证配置启用
	if cfg.Authentication != nil {
		cfg.Authentication.Enabled = true
	}

	// 在goroutine中启动服务器
	go func() {
		_ = StartPProfServer(cfg)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 测试本地访问（应该允许）
	resp, err := http.Get("http://localhost:16062/debug/pprof/")
	if err == nil {
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "本地IP应允许访问")
	}
}

// TestStartPProfServer_ProfilesEnabled 测试Profile启用控制
func TestStartPProfServer_ProfilesEnabled(t *testing.T) {
	cfg := gopprof.Default().
		Enable().
		WithPort(16063).
		WithPathPrefix("/debug/pprof").
		EnableCPUProfile()

	// 确保配置正确设置
	if cfg.EnableProfiles == nil {
		cfg.EnableProfiles = &gopprof.ProfilesConfig{}
	}
	cfg.EnableProfiles.CPU = true
	cfg.EnableProfiles.Heap = false

	// 在goroutine中启动服务器
	go func() {
		_ = StartPProfServer(cfg)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		desc       string
	}{
		{"启用的CPU profile", "/debug/pprof/profile?seconds=1", http.StatusOK, "CPU profile已启用"},
		{"禁用的Heap profile", "/debug/pprof/heap", http.StatusForbidden, "Heap profile未启用"},
		{"索引页面", "/debug/pprof/", http.StatusOK, "索引页面始终可访问"},
		{"cmdline", "/debug/pprof/cmdline", http.StatusOK, "cmdline始终可访问"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("http://localhost:16063%s", tt.path)
			resp, err := http.Get(url)
			if err != nil {
				t.Logf("访问 %s 失败: %v", url, err)
				return
			}
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode, tt.desc)
		})
	}
}

// TestAuthenticateRequest 测试认证函数
func TestAuthenticateRequest(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *gopprof.PProf
		token    string
		useQuery bool
		want     bool
	}{
		{
			name: "认证未启用",
			cfg: &gopprof.PProf{
				Authentication: &gopprof.AuthConfig{
					Enabled: false,
				},
			},
			token: "",
			want:  true,
		},
		{
			name: "不需要认证",
			cfg: &gopprof.PProf{
				Authentication: &gopprof.AuthConfig{
					Enabled:     true,
					RequireAuth: false,
				},
			},
			token: "",
			want:  true,
		},
		{
			name: "有效token（Header）",
			cfg: &gopprof.PProf{
				Authentication: &gopprof.AuthConfig{
					Enabled:     true,
					RequireAuth: true,
					AuthToken:   "test-token",
				},
			},
			token: "Bearer test-token",
			want:  true,
		},
		{
			name: "有效token（Query）",
			cfg: &gopprof.PProf{
				Authentication: &gopprof.AuthConfig{
					Enabled:     true,
					RequireAuth: true,
					AuthToken:   "test-token",
				},
			},
			token:    "test-token",
			useQuery: true,
			want:     true,
		},
		{
			name: "无效token",
			cfg: &gopprof.PProf{
				Authentication: &gopprof.AuthConfig{
					Enabled:     true,
					RequireAuth: true,
					AuthToken:   "test-token",
				},
			},
			token: "wrong-token",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.useQuery {
				req = httptest.NewRequest("GET", "/?token="+tt.token, nil)
			} else {
				req = httptest.NewRequest("GET", "/", nil)
				if tt.token != "" {
					req.Header.Set("Authorization", tt.token)
				}
			}

			got := authenticateRequest(tt.cfg, req)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestCheckPProfIPWhitelist 测试IP白名单函数
func TestCheckPProfIPWhitelist(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *gopprof.PProf
		remoteAddr string
		want       bool
	}{
		{
			name: "白名单为空",
			cfg: &gopprof.PProf{
				Authentication: &gopprof.AuthConfig{
					AllowedIPs: []string{},
				},
			},
			remoteAddr: "10.0.0.1:12345",
			want:       true,
		},
		{
			name: "精确匹配",
			cfg: &gopprof.PProf{
				Authentication: &gopprof.AuthConfig{
					AllowedIPs: []string{"127.0.0.1"},
				},
			},
			remoteAddr: "127.0.0.1:12345",
			want:       true,
		},
		{
			name: "CIDR匹配",
			cfg: &gopprof.PProf{
				Authentication: &gopprof.AuthConfig{
					AllowedIPs: []string{"192.168.1.0/24"},
				},
			},
			remoteAddr: "192.168.1.100:12345",
			want:       true,
		},
		{
			name: "不在白名单",
			cfg: &gopprof.PProf{
				Authentication: &gopprof.AuthConfig{
					AllowedIPs: []string{"127.0.0.1", "192.168.1.0/24"},
				},
			},
			remoteAddr: "10.0.0.1:12345",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr

			got := checkPProfIPWhitelist(tt.cfg, req)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestIsProfileEnabled 测试Profile启用检查
func TestIsProfileEnabled(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *gopprof.PProf
		pprofPath string
		want      bool
	}{
		{
			name:      "配置为nil，全部启用",
			cfg:       &gopprof.PProf{},
			pprofPath: "heap",
			want:      true,
		},
		{
			name: "CPU已启用",
			cfg: &gopprof.PProf{
				EnableProfiles: &gopprof.ProfilesConfig{
					CPU: true,
				},
			},
			pprofPath: "profile",
			want:      true,
		},
		{
			name: "Heap未启用",
			cfg: &gopprof.PProf{
				EnableProfiles: &gopprof.ProfilesConfig{
					Heap: false,
				},
			},
			pprofPath: "heap",
			want:      false,
		},
		{
			name: "索引页面始终可访问",
			cfg: &gopprof.PProf{
				EnableProfiles: &gopprof.ProfilesConfig{
					CPU: false,
				},
			},
			pprofPath: "",
			want:      true,
		},
		{
			name: "cmdline始终可访问",
			cfg: &gopprof.PProf{
				EnableProfiles: &gopprof.ProfilesConfig{
					CPU: false,
				},
			},
			pprofPath: "cmdline",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isProfileEnabled(tt.cfg, tt.pprofPath)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestInitSamplingConfig 测试采样配置初始化
func TestInitSamplingConfig(t *testing.T) {
	cfg := gopprof.Default()

	// 确保配置正确设置
	if cfg.Sampling == nil {
		cfg.Sampling = &gopprof.SamplingConfig{}
	}
	cfg.Sampling.MemoryRate = 4096
	cfg.Sampling.BlockRate = 1
	cfg.Sampling.MutexFraction = 1

	if cfg.EnableProfiles == nil {
		cfg.EnableProfiles = &gopprof.ProfilesConfig{}
	}
	cfg.EnableProfiles.Block = true
	cfg.EnableProfiles.Mutex = true

	// 调用初始化函数
	initSamplingConfig(cfg)

	// 验证采样配置已设置（无法直接验证runtime设置，只能确保不panic）
	assert.NotNil(t, cfg.Sampling)
}

// TestRegisterPProfHandlers 测试注册pprof处理器
func TestRegisterPProfHandlers(t *testing.T) {
	mux := http.NewServeMux()
	pathPrefix := "/debug/pprof"

	registerPProfHandlers(mux, pathPrefix)

	// 验证处理器已注册（通过创建测试请求）
	paths := []string{
		"/debug/pprof/",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile?seconds=1",
		"/debug/pprof/trace?seconds=1",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			// 只要不是404就说明路由已注册
			assert.NotEqual(t, http.StatusNotFound, w.Code, "路由应已注册")
		})
	}
}
