/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-01-01 17:00:19
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 19:08:26
 * @FilePath: \go-rpc-gateway\middleware\swagger\auth.go
 * @Description: Swagger 文档访问认证 - 凭据校验、401 挑战、凭据 Cookie 换发
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"crypto/sha256"
	"crypto/subtle"
	"html"
	"net/http"
	"strings"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
)

// authResult 凭据校验结果
// [EN] authResult is the outcome of credential verification
type authResult int

const (
	// authDeny 凭据缺失或不匹配
	// [EN] authDeny means the credential is missing or does not match
	authDeny authResult = iota
	// authAllow 凭据有效，直接放行
	// [EN] authAllow means the credential is valid and the request passes through
	authAllow
	// authAllowViaQuery 凭据有效且来自查询参数，需换发 Cookie 后重定向
	// [EN] authAllowViaQuery means the credential is valid but came from a query parameter,
	// so it must be exchanged for a cookie followed by a redirect
	authAllowViaQuery
)

// ==================== 鉴权入口 ====================

// authorize Swagger 路由统一鉴权
// 返回 true 表示放行；返回 false 表示响应已写出（401 挑战或换发 Cookie 的重定向），调用方应立即结束处理
//
// [EN] authorize is the single authentication gate for every Swagger route. True means the request
// may proceed; false means the response has already been written (a 401 challenge or the redirect
// that exchanges a query credential for a cookie) and the caller must stop handling immediately
func (m *Middleware) authorize(w http.ResponseWriter, r *http.Request) bool {
	if !m.config.IsAuthEnabled() {
		return true
	}

	// CORS 预检请求按规范不携带凭据，放行后交由各 handler 的 OPTIONS 分支处理
	// [EN] CORS preflight carries no credentials by specification, so let it through and leave it
	// to each handler's OPTIONS branch
	if r.Method == constants.HTTPMethodOptions {
		return true
	}

	switch m.checkCredentials(r) {
	case authAllow:
		return true

	case authAllowViaQuery:
		// 凭据从查询参数换发为 Cookie，再重定向到去掉 token 的地址：
		// 既让页面内 Swagger UI 发起的 XHR 能继续携带凭据，
		// 也避免凭据长期留存在地址栏、浏览器历史与 Referer 中
		//
		// [EN] Exchange the query credential for a cookie, then redirect to the same address with
		// token stripped: the cookie keeps the XHRs that Swagger UI issues from the page
		// authenticated, and the redirect keeps the credential out of the address bar, browser
		// history and Referer
		m.issueAuthCookie(w, r)
		http.Redirect(w, r, urlWithoutToken(r), http.StatusFound)
		return false

	default:
		m.writeAuthChallenge(w, r)
		return false
	}
}

// checkCredentials 按配置的认证类型校验请求凭据
// [EN] checkCredentials verifies the request credential according to the configured auth type
func (m *Middleware) checkCredentials(r *http.Request) authResult {
	auth := m.config.Auth

	switch m.config.GetAuthType() {
	case goswagger.AuthBasic:
		return m.checkBasicAuth(r, auth)
	case goswagger.AuthBearer:
		return m.checkTokenAuth(r, auth.Token, "")
	case goswagger.AuthCustom:
		return m.checkTokenAuth(r, auth.HeaderValue, auth.HeaderName)
	default:
		// 认证已启用但类型无法识别（多为配置拼写错误）时一律拒绝：
		// 宁可访问不了文档，也不能因为一个错别字让文档彻底裸奔
		//
		// [EN] Authentication is on but the type is unrecognizable, almost always a config typo.
		// Deny everything: better that the docs are unreachable than wide open because of a
		// misspelling
		m.logger.Warn("无法识别的 Swagger 认证类型 %q，已拒绝访问", m.config.GetAuthType())
		return authDeny
	}
}

// checkBasicAuth 校验 Basic 认证
// 浏览器在首次 401 挑战后会对同源请求（含 XHR）自动重放 Authorization 头，故无需 Cookie 兜底
//
// [EN] checkBasicAuth verifies Basic authentication. After the first 401 challenge the browser
// replays the Authorization header on every same-origin request, XHRs included, so no cookie
// fallback is needed here
func (m *Middleware) checkBasicAuth(r *http.Request, auth *goswagger.AuthConfig) authResult {
	username, password, ok := r.BasicAuth()
	if !ok {
		return authDeny
	}
	// 两项都要比较，不能因为用户名先不匹配就短路，否则又把时序差异漏出去
	// [EN] Compare both fields; short-circuiting on a username mismatch would leak timing again
	userMatch := secretEqual(username, auth.Username)
	passMatch := secretEqual(password, auth.Password)
	if userMatch && passMatch {
		return authAllow
	}
	return authDeny
}

// checkTokenAuth 校验 Bearer / Custom 这类单值凭据
// headerName 为空时按 Bearer 方案从 Authorization 头取值，否则从该自定义头取值；
// 两者都支持 Cookie 与查询参数兜底，以便浏览器地址栏直接访问
//
// [EN] checkTokenAuth verifies single-value credentials such as Bearer and Custom. An empty
// headerName reads the Bearer scheme from the Authorization header, otherwise it reads the given
// custom header; both fall back to a cookie and a query parameter so the docs stay reachable by
// typing the address into a browser
func (m *Middleware) checkTokenAuth(r *http.Request, expected, headerName string) authResult {
	if secretEqual(headerCredential(r, headerName), expected) {
		return authAllow
	}

	if c, err := r.Cookie(constants.AuthCookieName); err == nil && secretEqual(c.Value, expected) {
		return authAllow
	}

	if secretEqual(queryToken(r), expected) {
		return authAllowViaQuery
	}

	return authDeny
}

// headerCredential 从请求头提取凭据
// headerName 为空时解析 Authorization 头的 Bearer 方案，否则直接读取该自定义头
//
// [EN] headerCredential extracts the credential from the request headers. An empty headerName
// parses the Bearer scheme out of the Authorization header, otherwise the custom header is read
// directly
func headerCredential(r *http.Request, headerName string) string {
	if headerName != "" {
		return strings.TrimSpace(r.Header.Get(headerName))
	}

	raw := r.Header.Get(constants.HeaderAuthorization)
	if raw == "" {
		return ""
	}

	parts := strings.SplitN(raw, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], constants.AuthSchemeBearer) {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// queryToken 读取查询参数中的凭据
// [EN] queryToken reads the credential carried in the query parameter
func queryToken(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get(constants.AuthQueryToken))
}

// secretEqual 以恒定时间比较凭据，避免逐字节提前返回形成时序侧信道
// 先各自取 SHA-256 再比对定长摘要：ConstantTimeCompare 在长度不等时会立刻返回，
// 直接比原文仍会把凭据长度漏出去，摘要后长度恒定，这点差异也一并抹掉
// 任一侧为空都视为不匹配：配置项缺省时不能被一个空请求绕过
//
// [EN] secretEqual compares credentials in constant time so that an early byte-wise return cannot
// form a timing side channel. Each side is hashed with SHA-256 first and the fixed-length digests
// are compared: ConstantTimeCompare returns immediately when the lengths differ, so comparing the
// raw strings would still leak the credential length, while digests are always the same size and
// erase that difference too. An empty value on either side never matches, so a missing config
// entry cannot be bypassed by an empty request
func secretEqual(got, want string) bool {
	if got == "" || want == "" {
		return false
	}
	gotSum := sha256.Sum256([]byte(got))
	wantSum := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotSum[:], wantSum[:]) == 1
}

// ==================== 凭据 Cookie ====================

// issueAuthCookie 将查询参数中的凭据换发为会话 Cookie
// 不设 MaxAge，浏览器关闭即失效
//
// [EN] issueAuthCookie exchanges the credential from the query parameter for a session cookie.
// MaxAge is left unset so it expires when the browser closes
func (m *Middleware) issueAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     constants.AuthCookieName,
		Value:    queryToken(r),
		Path:     cookiePath(m.config.UIPath),
		HttpOnly: true,
		Secure:   isTLSRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// clearAuthCookie 清除凭据 Cookie
// [EN] clearAuthCookie removes the credential cookie
func (m *Middleware) clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     constants.AuthCookieName,
		Value:    "",
		Path:     cookiePath(m.config.UIPath),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isTLSRequest(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// cookiePath 将 Cookie 作用域收敛到 Swagger 路由前缀，避免凭据被带到业务接口上
// [EN] cookiePath narrows the cookie scope to the Swagger route prefix so the credential is never
// attached to business endpoints
func cookiePath(uiPath string) string {
	if uiPath == "" {
		return constants.PathSeparator
	}
	return uiPath
}

// isTLSRequest 判断请求是否经由 HTTPS，决定 Cookie 是否带 Secure
// 兼容在网关前终止 TLS 的部署：此时只能依赖 X-Forwarded-Proto
//
// [EN] isTLSRequest reports whether the request arrived over HTTPS, which decides if the cookie
// gets the Secure attribute. It also covers deployments that terminate TLS in front of the
// gateway, where X-Forwarded-Proto is the only signal available
func isTLSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get(constants.HeaderXForwardedProto), constants.SchemeHTTPS)
}

// urlWithoutToken 返回剥掉 token 查询参数后的当前地址，其余参数原样保留
// [EN] urlWithoutToken returns the current address with the token query parameter stripped,
// leaving every other parameter untouched
func urlWithoutToken(r *http.Request) string {
	cloned := *r.URL
	query := cloned.Query()
	query.Del(constants.AuthQueryToken)
	cloned.RawQuery = query.Encode()

	target := cloned.RequestURI()
	if target == "" {
		return constants.PathSeparator
	}
	return target
}

// ==================== 401 挑战 ====================

// handleAuthLogout 退出登录：清除凭据 Cookie 并重新发起 401 挑战
// 注意 Basic 认证的凭据缓存在浏览器内，各家清理时机不一致，
// 可靠的退出方式仍是关闭浏览器；此处的 401 只负责让当前标签页立刻失去访问权
//
// [EN] handleAuthLogout clears the credential cookie and re-issues the 401 challenge. Note that
// Basic credentials are cached inside the browser and each vendor drops them at a different time,
// so closing the browser is still the only reliable logout; the 401 here only makes the current
// tab lose access right away
func (m *Middleware) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	m.clearAuthCookie(w, r)
	m.writeAuthChallenge(w, r)
}

// writeAuthChallenge 写出 401 响应
// Basic 认证附带 WWW-Authenticate 以触发浏览器原生登录弹窗；
// Bearer/Custom 无法由原生弹窗承接，对页面请求返回一个仅含凭据输入框的登录页
//
// [EN] writeAuthChallenge writes the 401 response. Basic authentication carries a WWW-Authenticate
// header to trigger the browser's native login dialog; Bearer and Custom cannot be served by that
// dialog, so page requests get a minimal login page with a single credential field instead
func (m *Middleware) writeAuthChallenge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(constants.HeaderCacheControl, constants.NoStoreCacheControl)

	authType := m.config.GetAuthType()
	switch authType {
	case goswagger.AuthBasic:
		w.Header().Set(constants.HeaderWWWAuthenticate,
			constants.AuthSchemeBasic+` realm="`+constants.AuthRealm+`", charset="UTF-8"`)
	case goswagger.AuthBearer:
		w.Header().Set(constants.HeaderWWWAuthenticate,
			constants.AuthSchemeBearer+` realm="`+constants.AuthRealm+`"`)
	}

	// 机器客户端与规范请求走 JSON，交由调用方注入的错误响应函数统一格式
	// [EN] Machine clients and spec requests get JSON, formatted by the error response function
	// the caller injected
	if !wantsHTML(r) {
		m.errorResponseFn(w, http.StatusUnauthorized, "访问 Swagger 文档需要认证")
		return
	}

	if authType == goswagger.AuthBasic {
		// 弹窗由响应头驱动，正文仅在用户取消弹窗后展示
		// [EN] The dialog is driven by the response header; this body only shows after the user
		// dismisses it
		writeHTMLHeaders(w)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(m.buildAuthNoticeHTML()))
		return
	}

	writeHTMLHeaders(w)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(m.buildAuthLoginHTML(r)))
}

// wantsHTML 判断是否应当返回页面而非 JSON
// 规范类请求（.json 结尾）始终返回 JSON，其余看 Accept 是否接受 HTML
//
// [EN] wantsHTML reports whether a page should be returned instead of JSON. Spec requests (ending
// in .json) always get JSON; everything else depends on whether Accept allows HTML
func wantsHTML(r *http.Request) bool {
	if strings.HasSuffix(r.URL.Path, constants.JSONExt) {
		return false
	}
	return strings.Contains(r.Header.Get(constants.HeaderAccept), constants.MimeTextHTML)
}

// ==================== 登录页 ====================

// buildAuthLoginHTML 构建 Bearer/Custom 认证的凭据输入页
// 使用 GET 表单直接把凭据提交回当前地址的 token 参数，命中后由 authorize 换发 Cookie，无需 JS
//
// [EN] buildAuthLoginHTML renders the credential entry page for Bearer and Custom authentication.
// A plain GET form submits the credential back to the token parameter of the current address and
// authorize then exchanges it for a cookie, so no JavaScript is involved
func (m *Middleware) buildAuthLoginHTML(r *http.Request) string {
	action := html.EscapeString(urlWithoutToken(r))
	title := html.EscapeString(m.config.Title)

	return `<!DOCTYPE html>
<html lang="` + constants.HTMLLangZH + `">
<head>
    <meta charset="` + constants.HTMLCharset + `">
    <meta name="viewport" content="` + constants.HTMLMetaViewport + `">
    <title>` + title + ` - 需要认证</title>
    <style>` + authPageCSS + `</style>
</head>
<body>
    <div class="auth-card">
        <div class="auth-icon">🔒</div>
        <h1>文档已加锁</h1>
        <p>该 Swagger 文档启用了 ` + html.EscapeString(string(m.config.GetAuthType())) + ` 认证，请输入访问凭据。</p>
        <form method="get" action="` + action + `">
            <input type="password" name="` + constants.AuthQueryToken + `" placeholder="访问凭据" autofocus autocomplete="current-password" />
            <button type="submit">进入文档</button>
        </form>
        <p class="auth-hint">接口调用方请改用请求头携带凭据。</p>
    </div>
</body>
</html>`
}

// buildAuthNoticeHTML 构建 Basic 认证取消弹窗后的提示页
// [EN] buildAuthNoticeHTML renders the notice page shown after the user dismisses the Basic
// authentication dialog
func (m *Middleware) buildAuthNoticeHTML() string {
	title := html.EscapeString(m.config.Title)

	return `<!DOCTYPE html>
<html lang="` + constants.HTMLLangZH + `">
<head>
    <meta charset="` + constants.HTMLCharset + `">
    <meta name="viewport" content="` + constants.HTMLMetaViewport + `">
    <title>` + title + ` - 需要认证</title>
    <style>` + authPageCSS + `</style>
</head>
<body>
    <div class="auth-card">
        <div class="auth-icon">🔒</div>
        <h1>文档已加锁</h1>
        <p>该 Swagger 文档启用了 Basic 认证，请刷新页面并在浏览器弹窗中输入用户名与密码。</p>
        <p class="auth-hint">接口调用方请在 Authorization 头中携带 Basic 凭据。</p>
    </div>
</body>
</html>`
}

// authPageCSS 认证相关页面样式
// [EN] authPageCSS holds the styles shared by the authentication pages
const authPageCSS = `
        body {
            margin: 0;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #fafafa;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            color: #3b4151;
        }
        .auth-card {
            width: 100%;
            max-width: 360px;
            margin: 16px;
            padding: 32px 28px;
            background: #fff;
            border: 1px solid #e8e8e8;
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);
            text-align: center;
        }
        .auth-icon {
            font-size: 36px;
            line-height: 1;
        }
        .auth-card h1 {
            margin: 16px 0 8px;
            font-size: 1.35em;
        }
        .auth-card p {
            margin: 0 0 20px;
            color: #666;
            font-size: 14px;
            line-height: 1.6;
        }
        .auth-card form {
            display: flex;
            flex-direction: column;
            gap: 12px;
        }
        .auth-card input {
            padding: 10px 12px;
            font-size: 14px;
            border: 1px solid #d9d9d9;
            border-radius: 4px;
            outline: none;
        }
        .auth-card input:focus {
            border-color: #4990e2;
        }
        .auth-card button {
            padding: 10px 16px;
            font-size: 14px;
            color: #fff;
            background: #4990e2;
            border: none;
            border-radius: 4px;
            cursor: pointer;
        }
        .auth-card button:hover {
            background: #3b7bbf;
        }
        .auth-hint {
            margin: 20px 0 0;
            font-size: 12px;
            color: #999;
        }
`

// ==================== 配置校验 ====================

// validateAuthConfig 校验认证配置，仅在启用认证时生效
// [EN] validateAuthConfig checks the auth configuration, and only applies when auth is enabled
func validateAuthConfig(config *goswagger.Swagger) error {
	if config == nil || !config.IsAuthEnabled() {
		return nil
	}
	return config.ValidateAuth()
}
