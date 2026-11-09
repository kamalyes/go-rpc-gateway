/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-01-01 17:00:19
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 19:08:26
 * @FilePath: \go-rpc-gateway\middleware\swagger\auth_ui.go
 * @Description: Swagger UI 认证状态标识 - 右上角锁徽标的状态计算与渲染
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"html"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
)

// authBadgeState Swagger UI 右上角锁标识的状态
// 页面能被渲染出来即说明已通过鉴权，故此处只需区分「是否加锁」而无需再判断当前请求
//
// [EN] authBadgeState is the state of the lock badge in the top-right corner of the Swagger UI.
// A page can only be rendered once the request has passed authentication, so the badge only has
// to distinguish "locked or not" without inspecting the current request again
type authBadgeState struct {
	// Locked 是否启用了认证
	// [EN] Locked reports whether authentication is enabled
	Locked bool
	// Icon 锁图标
	// [EN] Icon is the lock glyph
	Icon string
	// Label 状态文案
	// [EN] Label is the status text
	Label string
	// Tooltip 悬浮说明
	// [EN] Tooltip is the hover explanation
	Tooltip string
	// LogoutURL 退出链接，未加锁时为空
	// [EN] LogoutURL is the logout link, empty when the docs are not locked
	LogoutURL string
}

// authBadgeState 计算当前配置下的锁标识状态
// [EN] authBadgeState derives the badge state from the current configuration
func (m *Middleware) authBadgeState() authBadgeState {
	if !m.config.IsAuthEnabled() {
		return authBadgeState{
			Locked:  false,
			Icon:    "🔓",
			Label:   "公开访问",
			Tooltip: "未启用认证，任何能访问该端口的人都可以查看全部接口文档",
		}
	}

	return authBadgeState{
		Locked:    true,
		Icon:      "🔒",
		Label:     "已认证 · " + authTypeLabel(m.config.GetAuthType()),
		Tooltip:   "该文档已启用访问认证，当前会话凭据校验通过",
		LogoutURL: m.config.UIPath + constants.LogoutPath,
	}
}

// authTypeLabel 认证类型的展示文案
// [EN] authTypeLabel is the display text for an auth type
func authTypeLabel(authType goswagger.AuthType) string {
	switch authType {
	case goswagger.AuthBasic:
		return "Basic"
	case goswagger.AuthBearer:
		return "Bearer"
	case goswagger.AuthCustom:
		return "自定义头"
	default:
		return string(authType)
	}
}

// authBadgeHTML 渲染右上角锁标识
// [EN] authBadgeHTML renders the lock badge for the top-right corner
func (m *Middleware) authBadgeHTML() string {
	state := m.authBadgeState()

	modifier := "swagger-auth-badge--open"
	if state.Locked {
		modifier = "swagger-auth-badge--locked"
	}

	badge := `
    <div class="swagger-auth-badge ` + modifier + `" title="` + html.EscapeString(state.Tooltip) + `">
        <span class="swagger-auth-badge__icon">` + state.Icon + `</span>
        <span class="swagger-auth-badge__text">` + html.EscapeString(state.Label) + `</span>`

	if state.LogoutURL != "" {
		badge += `
        <a class="swagger-auth-badge__action" href="` + html.EscapeString(state.LogoutURL) + `">退出</a>`
	}

	return badge + `
    </div>`
}

// authBadgeCSS 右上角锁标识样式
// 固定定位在视口右上角，层级高于 Swagger UI 自身的顶栏
//
// [EN] authBadgeCSS styles the lock badge. It is fixed to the top-right of the viewport and sits
// above Swagger UI's own top bar
const authBadgeCSS = `
        .swagger-auth-badge {
            position: fixed;
            top: 12px;
            right: 12px;
            z-index: 1000;
            display: inline-flex;
            align-items: center;
            gap: 6px;
            padding: 6px 10px;
            border-radius: 14px;
            font-size: 12px;
            font-weight: 600;
            line-height: 1;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            box-shadow: 0 1px 3px rgba(0, 0, 0, 0.12);
            cursor: default;
            user-select: none;
        }
        .swagger-auth-badge--locked {
            background: #e8f5e9;
            border: 1px solid #a5d6a7;
            color: #2e7d32;
        }
        .swagger-auth-badge--open {
            background: #fff4e5;
            border: 1px solid #ffcc80;
            color: #b26500;
        }
        .swagger-auth-badge__icon {
            font-size: 13px;
        }
        .swagger-auth-badge__action {
            margin-left: 4px;
            padding-left: 8px;
            border-left: 1px solid currentColor;
            color: inherit;
            text-decoration: none;
            font-weight: 600;
            opacity: 0.85;
        }
        .swagger-auth-badge__action:hover {
            opacity: 1;
            text-decoration: underline;
        }
        @media (max-width: 480px) {
            .swagger-auth-badge__text {
                display: none;
            }
        }
`
