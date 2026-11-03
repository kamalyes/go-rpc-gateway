/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-03 18:02:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-03 18:22:00
 * @FilePath: \\go-rpc-gateway\\middleware\\security\\cors_test.go
 * @Description: CORS 中间件测试 - 非 CORS 请求零开销、预检与真实请求的头部划分、Vary 声明
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kamalyes/go-config/pkg/cors"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/stretchr/testify/assert"
)

func corsHandler(t *testing.T, cfg *cors.Cors) http.Handler {
	t.Helper()
	return CORSMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func corsConfig(origins []string, credentials bool) *cors.Cors {
	c := cors.Default()
	c.AllowedOrigins = origins
	c.AllowCredentials = credentials
	c.MaxAge = "600"
	return c
}

// TestCORS_NoOriginWritesNothing 不带 Origin 的请求不是 CORS 请求，不该写任何 CORS 响应头
// 这是把 263 ns 与 4 次分配降到零的前提，回归了就会悄悄涨回去
//
// [EN] A request without Origin is not a CORS request and must get no CORS response header
// This is what takes the middleware to zero cost, and a regression would silently restore it
func TestCORS_NoOriginWritesNothing(t *testing.T) {
	h := corsHandler(t, corsConfig([]string{"https://a.example"}, true))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/x", nil))

	assert.Equal(t, http.StatusOK, rr.Code)
	for _, k := range []string{
		constants.HeaderAccessControlAllowOrigin,
		constants.HeaderAccessControlAllowMethods,
		constants.HeaderAccessControlAllowHeaders,
		constants.HeaderAccessControlAllowCredentials,
		constants.HeaderAccessControlMaxAge,
		constants.HeaderVary,
	} {
		assert.Empty(t, rr.Header().Get(k), "非 CORS 请求不该带 %s", k)
	}
}

// TestCORS_OptionsWithoutOriginStill204 不带 Origin 的 OPTIONS 仍答 204，行为与改造前一致
// [EN] An OPTIONS without Origin still answers 204, unchanged from before
func TestCORS_OptionsWithoutOriginStill204(t *testing.T) {
	h := corsHandler(t, corsConfig([]string{"*"}, false))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodOptions, "/x", nil))
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

// TestCORS_ActualRequestSetsOriginAndVary 真实 CORS 请求要有 Allow-Origin 与 Vary，
// 但不该带仅预检才有意义的三个头
//
// [EN] An actual CORS request gets Allow-Origin and Vary but not the preflight-only headers
func TestCORS_ActualRequestSetsOriginAndVary(t *testing.T) {
	h := corsHandler(t, corsConfig([]string{"https://a.example"}, true))
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set(constants.HeaderOrigin, "https://a.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "https://a.example", rr.Header().Get(constants.HeaderAccessControlAllowOrigin))
	assert.Equal(t, constants.HeaderOrigin, rr.Header().Get(constants.HeaderVary),
		"回显 Origin 必须声明 Vary，否则共享缓存会跨源串响应")
	assert.Equal(t, constants.CORSCredentialsTrue, rr.Header().Get(constants.HeaderAccessControlAllowCredentials))
	assert.Empty(t, rr.Header().Get(constants.HeaderAccessControlAllowMethods), "仅预检才需要 Allow-Methods")
	assert.Empty(t, rr.Header().Get(constants.HeaderAccessControlAllowHeaders), "仅预检才需要 Allow-Headers")
	assert.Empty(t, rr.Header().Get(constants.HeaderAccessControlMaxAge), "仅预检才需要 Max-Age")
}

// TestCORS_PreflightSetsAllHeaders 预检响应要带齐
// [EN] A preflight response carries the full set
func TestCORS_PreflightSetsAllHeaders(t *testing.T) {
	h := corsHandler(t, corsConfig([]string{"https://a.example"}, true))
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set(constants.HeaderOrigin, "https://a.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.Equal(t, "https://a.example", rr.Header().Get(constants.HeaderAccessControlAllowOrigin))
	assert.Equal(t, constants.HeaderOrigin, rr.Header().Get(constants.HeaderVary))
	assert.NotEmpty(t, rr.Header().Get(constants.HeaderAccessControlAllowMethods))
	assert.NotEmpty(t, rr.Header().Get(constants.HeaderAccessControlAllowHeaders))
	assert.Equal(t, "600", rr.Header().Get(constants.HeaderAccessControlMaxAge))
}

// TestCORS_DisallowedOriginGetsNoCredentials 不在白名单的 origin 不发 Allow-Origin，
// 也不该发 Allow-Credentials —— 发了是自相矛盾的响应
//
// [EN] A disallowed origin gets neither Allow-Origin nor Allow-Credentials, since
// sending credentials for an origin that was not allowed is a contradictory response
func TestCORS_DisallowedOriginGetsNoCredentials(t *testing.T) {
	h := corsHandler(t, corsConfig([]string{"https://a.example"}, true))
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set(constants.HeaderOrigin, "https://evil.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Empty(t, rr.Header().Get(constants.HeaderAccessControlAllowOrigin))
	assert.Empty(t, rr.Header().Get(constants.HeaderAccessControlAllowCredentials),
		"未放行的 origin 不该收到 Allow-Credentials")
	assert.Equal(t, constants.HeaderOrigin, rr.Header().Get(constants.HeaderVary),
		"即便未放行也要声明 Vary，响应仍然随 Origin 变化")
}

// TestCORS_WildcardEchoesOrigin 通配符模式回显请求的 origin
// [EN] The wildcard mode echoes the request's origin
func TestCORS_WildcardEchoesOrigin(t *testing.T) {
	h := corsHandler(t, corsConfig([]string{"*"}, false))
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set(constants.HeaderOrigin, "https://anything.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, "https://anything.example", rr.Header().Get(constants.HeaderAccessControlAllowOrigin))
	assert.Equal(t, constants.HeaderOrigin, rr.Header().Get(constants.HeaderVary))
}
