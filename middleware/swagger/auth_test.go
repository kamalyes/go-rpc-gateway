/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-01-01 17:00:19
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 19:08:26
 * @FilePath: \go-rpc-gateway\middleware\swagger\auth_test.go
 * @Description: Swagger 文档访问认证测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
)

// 锁标识元素的 class 组合，用于区分「渲染出来的徽标」与「样式表里的规则」
// [EN] The badge element's class pair, used to tell the rendered badge apart from the rule that
// merely declares it in the stylesheet
const (
	badgeClassLocked = `class="swagger-auth-badge swagger-auth-badge--locked"`
	badgeClassOpen   = `class="swagger-auth-badge swagger-auth-badge--open"`
)

// newAuthTestMiddleware 构造一个仅用于鉴权测试的中间件，不依赖磁盘上的规范文件
// [EN] newAuthTestMiddleware builds a middleware purely for auth tests, with no dependency on a
// spec file on disk
func newAuthTestMiddleware(auth *goswagger.AuthConfig) *Middleware {
	config := &goswagger.Swagger{
		Enabled:     true,
		UIPath:      "/swagger",
		Title:       "Test API",
		Description: "鉴权测试",
		Version:     "1.0.0",
		Auth:        auth,
	}
	return NewMiddleware(config)
}

// getUIPage 以浏览器身份请求 Swagger UI 页面
// [EN] getUIPage requests a Swagger UI page as a browser would
func getUIPage(m *Middleware, path string, decorate func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set(constants.HeaderAccept, "text/html,application/xhtml+xml")
	if decorate != nil {
		decorate(req)
	}
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)
	return rec
}

func TestAuth_DisabledAllowsAnonymous(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{Type: goswagger.AuthNone})

	rec := getUIPage(m, "/swagger/", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("未启用认证时应放行，got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "公开访问") {
		t.Error("未启用认证时右上角应展示「公开访问」标识")
	}
	// 样式表里两种修饰类都会出现，这里只能断言实际渲染出来的那个元素
	// [EN] Both modifier classes appear in the stylesheet, so only the actually rendered element
	// can be asserted on
	if !strings.Contains(rec.Body.String(), badgeClassOpen) {
		t.Error("未启用认证时锁标识应为未加锁样式")
	}
	if strings.Contains(rec.Body.String(), badgeClassLocked) {
		t.Error("未启用认证时不应渲染加锁标识")
	}
}

func TestAuth_BasicChallengeWithoutCredentials(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{
		Type: goswagger.AuthBasic, Username: "admin", Password: "s3cret",
	})

	rec := getUIPage(m, "/swagger/", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("缺少凭据应返回 401，got %d", rec.Code)
	}
	challenge := rec.Header().Get(constants.HeaderWWWAuthenticate)
	if !strings.HasPrefix(challenge, constants.AuthSchemeBasic) {
		t.Errorf("应带 Basic 挑战头以触发浏览器登录弹窗，got %q", challenge)
	}
	if rec.Header().Get(constants.HeaderCacheControl) != constants.NoStoreCacheControl {
		t.Error("401 响应应禁止缓存")
	}
}

func TestAuth_BasicRejectsWrongCredentials(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{
		Type: goswagger.AuthBasic, Username: "admin", Password: "s3cret",
	})

	cases := []struct{ user, pass string }{
		{"admin", "wrong"},
		{"wrong", "s3cret"},
		{"", ""},
	}

	for _, c := range cases {
		rec := getUIPage(m, "/swagger/", func(r *http.Request) {
			r.SetBasicAuth(c.user, c.pass)
		})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("凭据 %q/%q 应被拒绝，got %d", c.user, c.pass, rec.Code)
		}
	}
}

func TestAuth_BasicAcceptsValidCredentials(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{
		Type: goswagger.AuthBasic, Username: "admin", Password: "s3cret",
	})

	rec := getUIPage(m, "/swagger/", func(r *http.Request) {
		r.SetBasicAuth("admin", "s3cret")
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("凭据正确应放行，got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, badgeClassLocked) {
		t.Error("已认证页面右上角应展示加锁标识")
	}
	if strings.Contains(body, badgeClassOpen) {
		t.Error("已认证页面不应渲染未加锁标识")
	}
	if !strings.Contains(body, "已认证 · Basic") {
		t.Error("锁标识应带出认证类型")
	}
	if !strings.Contains(body, "/swagger"+constants.LogoutPath) {
		t.Error("加锁状态下应提供退出入口")
	}
}

func TestAuth_BearerFromHeaderCookieAndQuery(t *testing.T) {
	const token = "tok-abc-123"
	m := newAuthTestMiddleware(&goswagger.AuthConfig{Type: goswagger.AuthBearer, Token: token})

	t.Run("请求头", func(t *testing.T) {
		rec := getUIPage(m, "/swagger/", func(r *http.Request) {
			r.Header.Set(constants.HeaderAuthorization, constants.AuthSchemeBearer+" "+token)
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("Authorization 头携带有效 token 应放行，got %d", rec.Code)
		}
	})

	t.Run("Cookie", func(t *testing.T) {
		rec := getUIPage(m, "/swagger/", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: constants.AuthCookieName, Value: token})
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("Cookie 携带有效 token 应放行，got %d", rec.Code)
		}
	})

	t.Run("查询参数换发Cookie", func(t *testing.T) {
		rec := getUIPage(m, "/swagger/?"+constants.AuthQueryToken+"="+token+"&view=ui", nil)

		if rec.Code != http.StatusFound {
			t.Fatalf("查询参数命中后应重定向以剥离凭据，got %d", rec.Code)
		}

		location := rec.Header().Get("Location")
		if strings.Contains(location, constants.AuthQueryToken+"=") {
			t.Errorf("重定向地址不应继续携带凭据，got %q", location)
		}
		if !strings.Contains(location, "view=ui") {
			t.Errorf("重定向应保留其余查询参数，got %q", location)
		}

		cookies := rec.Result().Cookies()
		var issued *http.Cookie
		for _, c := range cookies {
			if c.Name == constants.AuthCookieName {
				issued = c
			}
		}
		if issued == nil {
			t.Fatal("应换发凭据 Cookie")
		}
		if issued.Value != token {
			t.Errorf("Cookie 值应为凭据本身，got %q", issued.Value)
		}
		if !issued.HttpOnly {
			t.Error("凭据 Cookie 应为 HttpOnly")
		}
		if issued.Path != "/swagger" {
			t.Errorf("凭据 Cookie 作用域应收敛到 Swagger 前缀，got %q", issued.Path)
		}
	})

	t.Run("无效token", func(t *testing.T) {
		rec := getUIPage(m, "/swagger/", func(r *http.Request) {
			r.Header.Set(constants.HeaderAuthorization, constants.AuthSchemeBearer+" nope")
		})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("无效 token 应拒绝，got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "文档已加锁") {
			t.Error("Bearer 认证失败时应返回可输入凭据的登录页")
		}
	})
}

func TestAuth_CustomHeader(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{
		Type: goswagger.AuthCustom, HeaderName: "X-Docs-Key", HeaderValue: "open-sesame",
	})

	rec := getUIPage(m, "/swagger/", func(r *http.Request) {
		r.Header.Set("X-Docs-Key", "open-sesame")
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("自定义头匹配应放行，got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "已认证 · 自定义头") {
		t.Error("锁标识应展示自定义头认证类型")
	}

	rec = getUIPage(m, "/swagger/", func(r *http.Request) {
		r.Header.Set("X-Docs-Key", "wrong")
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("自定义头不匹配应拒绝，got %d", rec.Code)
	}
}

// TestAuth_UnknownTypeFailsClosed 认证类型拼错时必须锁死而非放行
// [EN] A misspelled auth type must lock everything down rather than let requests through
func TestAuth_UnknownTypeFailsClosed(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{
		Type: goswagger.AuthType("BASCI"), Username: "admin", Password: "s3cret",
	})

	rec := getUIPage(m, "/swagger/", func(r *http.Request) {
		r.SetBasicAuth("admin", "s3cret")
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("无法识别的认证类型应拒绝访问，got %d", rec.Code)
	}
}

// TestAuth_EmptyConfiguredSecretFailsClosed 凭据漏配时不能被空请求绕过
// [EN] A missing credential in the config must not be bypassable by an empty request
func TestAuth_EmptyConfiguredSecretFailsClosed(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{Type: goswagger.AuthBearer})

	rec := getUIPage(m, "/swagger/", func(r *http.Request) {
		r.Header.Set(constants.HeaderAuthorization, constants.AuthSchemeBearer+" ")
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未配置 token 时应拒绝全部访问，got %d", rec.Code)
	}
}

// TestAuth_SpecRequestGetsJSONChallenge 规范类请求应拿到 JSON 而非 HTML 登录页
// [EN] A spec request must get JSON back, not the HTML login page
func TestAuth_SpecRequestGetsJSONChallenge(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{
		Type: goswagger.AuthBearer, Token: "tok",
	})

	req := httptest.NewRequest(http.MethodGet, "/swagger/swagger.json", nil)
	req.Header.Set(constants.HeaderAccept, "text/html")
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未认证的规范请求应返回 401，got %d", rec.Code)
	}
	if ct := rec.Header().Get(constants.HeaderContentType); !strings.Contains(ct, constants.MimeApplicationJSON) {
		t.Errorf(".json 请求应返回 JSON 错误，got Content-Type %q", ct)
	}
	if !strings.Contains(rec.Header().Get(constants.HeaderWWWAuthenticate), constants.AuthSchemeBearer) {
		t.Error("Bearer 的 JSON 挑战应带 WWW-Authenticate 头")
	}
}

// TestAuth_OptionsPreflightBypasses CORS 预检不携带凭据，不能被鉴权拦掉
// [EN] CORS preflight carries no credentials and must not be blocked by the auth gate
func TestAuth_OptionsPreflightBypasses(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{
		Type: goswagger.AuthBearer, Token: "tok",
	})

	req := httptest.NewRequest(http.MethodOptions, "/swagger/swagger.json", nil)
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("OPTIONS 预检应放行，got %d", rec.Code)
	}
}

func TestAuth_LogoutClearsCookieAndChallenges(t *testing.T) {
	const token = "tok-xyz"
	m := newAuthTestMiddleware(&goswagger.AuthConfig{Type: goswagger.AuthBearer, Token: token})

	rec := getUIPage(m, "/swagger"+constants.LogoutPath, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: constants.AuthCookieName, Value: token})
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("退出后应立即失去访问权，got %d", rec.Code)
	}

	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == constants.AuthCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("退出应清除凭据 Cookie")
	}
}

// TestAuth_LogoutPathOnlyRoutedWhenAuthEnabled 未加锁时不应凭空多出一个退出路由
// [EN] An unlocked setup must not grow a logout route out of nowhere
func TestAuth_LogoutPathOnlyRoutedWhenAuthEnabled(t *testing.T) {
	open := newAuthTestMiddleware(&goswagger.AuthConfig{Type: goswagger.AuthNone})
	if open.isSwaggerPath("/swagger" + constants.LogoutPath) {
		t.Error("未启用认证时不应注册退出路由")
	}

	locked := newAuthTestMiddleware(&goswagger.AuthConfig{Type: goswagger.AuthBearer, Token: "t"})
	if !locked.isSwaggerPath("/swagger" + constants.LogoutPath) {
		t.Error("启用认证时应注册退出路由")
	}
	if !contains(locked.GetSwaggerPaths(), "/swagger"+constants.LogoutPath) {
		t.Error("GetSwaggerPaths 应包含退出路由，否则宿主路由注册时会漏掉")
	}
}

// TestAuth_UpdateConfigRejectsInvalidAuth 认证配置残缺时不应被接受
// [EN] An incomplete auth config must be rejected
func TestAuth_UpdateConfigRejectsInvalidAuth(t *testing.T) {
	m := newAuthTestMiddleware(&goswagger.AuthConfig{Type: goswagger.AuthNone})

	broken := &goswagger.Swagger{
		Enabled: true,
		UIPath:  "/swagger",
		Title:   "Test API",
		Auth:    &goswagger.AuthConfig{Type: goswagger.AuthBasic, Username: "admin"},
	}

	if err := m.UpdateConfig(broken); err == nil {
		t.Fatal("Basic 认证缺少密码时 UpdateConfig 应返回错误")
	}
	if m.config.IsAuthEnabled() {
		t.Error("非法认证配置不应生效")
	}
}

// TestAuth_ConfigAuthTypeIsCaseInsensitive YAML 里写小写 type 必须能生效
// 归一化缺失时的表现极具欺骗性：type: basic 既不等于 AuthNone（于是判定为已启用），
// 又匹配不上任何分支（于是拒绝所有请求），文档整个打不开而日志只说「无法识别的认证类型」
//
// [EN] A lowercase type in YAML must work. Without normalization the symptom is deceptive:
// `type: basic` is neither equal to AuthNone (so auth counts as enabled) nor matches any branch
// (so every request is denied), leaving the docs entirely unreachable while the log only mentions
// an unrecognizable auth type
func TestAuth_ConfigAuthTypeIsCaseInsensitive(t *testing.T) {
	for _, raw := range []string{"basic", "Basic", "BASIC", "BaSiC", "  basic  "} {
		m := newAuthTestMiddleware(&goswagger.AuthConfig{
			Type: goswagger.AuthType(raw), Username: "admin", Password: "s3cret",
		})

		if got := m.config.GetAuthType(); got != goswagger.AuthBasic {
			t.Errorf("type=%q 应归一化为 BASIC，got %q", raw, got)
		}
		if err := m.config.ValidateAuth(); err != nil {
			t.Errorf("type=%q 归一化后 ValidateAuth 不应报错: %v", raw, err)
		}

		rec := getUIPage(m, "/swagger/", func(r *http.Request) {
			r.SetBasicAuth("admin", "s3cret")
		})
		if rec.Code != http.StatusOK {
			t.Errorf("type=%q 凭据正确应放行，got %d", raw, rec.Code)
		}

		if rec := getUIPage(m, "/swagger/", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("type=%q 无凭据应拒绝，got %d", raw, rec.Code)
		}
	}
}

// TestAuth_ConfigAuthTypeNormalization 其余类型与边界取值的归一化
// [EN] Normalization of the remaining types and edge-case values
func TestAuth_ConfigAuthTypeNormalization(t *testing.T) {
	cases := []struct {
		raw     string
		want    goswagger.AuthType
		enabled bool
	}{
		{"bearer", goswagger.AuthBearer, true},
		{"BeArEr", goswagger.AuthBearer, true},
		{"custom", goswagger.AuthCustom, true},
		{"none", goswagger.AuthNone, false},
		{"NONE", goswagger.AuthNone, false},
		// 空值视为未配置，而不是「启用了一种叫空字符串的认证」
		// [EN] An empty value means unconfigured, not "an auth type named empty string"
		{"", goswagger.AuthNone, false},
		{"   ", goswagger.AuthNone, false},
		// 无法识别的取值原样保留，由鉴权入口按「失败即锁死」拒绝
		// [EN] An unrecognizable value is kept as-is and denied by the gate's fail-closed policy
		{"bascis", goswagger.AuthType("bascis"), true},
	}

	for _, c := range cases {
		config := &goswagger.Swagger{Auth: &goswagger.AuthConfig{Type: goswagger.AuthType(c.raw)}}
		if got := config.GetAuthType(); got != c.want {
			t.Errorf("type=%q 应归一化为 %q，got %q", c.raw, c.want, got)
		}
		if got := config.IsAuthEnabled(); got != c.enabled {
			t.Errorf("type=%q IsAuthEnabled 应为 %v，got %v", c.raw, c.enabled, got)
		}
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
