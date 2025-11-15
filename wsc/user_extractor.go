/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-15 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 00:20:36
 * @FilePath: \go-rpc-gateway\wsc\user_extractor.go
 * @Description: WebSocket用户信息提取器 - 从HTTP请求中提取详细的用户连接信息
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package wsc

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kamalyes/go-rpc-gateway/errors"
	gowsc "github.com/kamalyes/go-wsc"
)

// UserConnectionInfo 用户连接的详细信息（生产级别字段）
// 这些信息会在用户连接时被提取并存储
type UserConnectionInfo struct {
	// ========== 基础身份信息 ==========
	ClientID string       `json:"client_id"` // 客户端唯一ID
	UserID   string       `json:"user_id"`   // 用户ID
	UserType gowsc.UserType `json:"user_type"` // 用户类型 (customer/agent/admin)
	Role     gowsc.UserRole `json:"role"`      // 角色
	TicketID string       `json:"ticket_id"` // 工单ID（如果有）

	// ========== 网络信息 ==========
	RemoteAddr    string `json:"remote_addr"`     // 客户端IP:Port
	RemoteIP      string `json:"remote_ip"`       // 客户端IP（解析后）
	RemotePort    string `json:"remote_port"`     // 客户端端口
	RealIP        string `json:"real_ip"`         // 真实IP（来自X-Real-IP/X-Forwarded-For）
	ForwardedFor  string `json:"forwarded_for"`   // X-Forwarded-For原始值
	ProxyChain    string `json:"proxy_chain"`     // 代理链
	LocalAddr     string `json:"local_addr"`      // 服务端地址
	ConnectionID  string `json:"connection_id"`   // 连接ID（唯一标识）
	Protocol      string `json:"protocol"`        // 协议 (ws/wss/sse)
	TLSVersion    string `json:"tls_version"`     // TLS版本（如果是WSS）
	
	// ========== HTTP请求信息 ==========
	UserAgent     string            `json:"user_agent"`      // User-Agent
	Origin        string            `json:"origin"`          // Origin头
	Referer       string            `json:"referer"`         // Referer
	AcceptLang    string            `json:"accept_language"` // 接受的语言
	RequestURI    string            `json:"request_uri"`     // 请求URI
	QueryString   string            `json:"query_string"`    // 查询字符串
	Headers       map[string]string `json:"headers"`         // 关键请求头

	// ========== 客户端信息 ==========
	ClientType    gowsc.ClientType `json:"client_type"`    // 客户端类型 (web/mobile/desktop)
	Platform      string         `json:"platform"`       // 平台 (iOS/Android/Windows/Mac/Linux)
	Browser       string         `json:"browser"`        // 浏览器类型
	BrowserVer    string         `json:"browser_version"` // 浏览器版本
	OSName        string         `json:"os_name"`        // 操作系统
	OSVersion     string         `json:"os_version"`     // 系统版本
	DeviceType    string         `json:"device_type"`    // 设备类型 (mobile/tablet/desktop)
	DeviceModel   string         `json:"device_model"`   // 设备型号
	AppVersion    string         `json:"app_version"`    // 应用版本（如果有）

	// ========== 地理位置信息 ==========
	Country    string  `json:"country"`     // 国家
	Region     string  `json:"region"`      // 省/州
	City       string  `json:"city"`        // 城市
	ISP        string  `json:"isp"`         // ISP运营商
	Timezone   string  `json:"timezone"`    // 时区
	Latitude   float64 `json:"latitude"`    // 纬度
	Longitude  float64 `json:"longitude"`   // 经度

	// ========== 认证信息 ==========
	Token         string    `json:"token"`          // 认证Token
	SessionID     string    `json:"session_id"`     // 会话ID
	AuthMethod    string    `json:"auth_method"`    // 认证方式 (jwt/oauth/basic)
	AuthTime      time.Time `json:"auth_time"`      // 认证时间
	TokenExpireAt time.Time `json:"token_expire_at"` // Token过期时间

	// ========== 业务信息 ==========
	Department   gowsc.Department `json:"department"`    // 部门（客服专用）
	Skills       []gowsc.Skill    `json:"skills"`        // 技能标签（客服专用）
	MaxTickets   int            `json:"max_tickets"`   // 最大工单数（客服专用）
	Tags         []string       `json:"tags"`          // 用户标签
	CustomFields map[string]interface{} `json:"custom_fields"` // 自定义字段

	// ========== 连接状态信息 ==========
	ConnectedAt     time.Time `json:"connected_at"`      // 连接时间
	LastActiveAt    time.Time `json:"last_active_at"`    // 最后活跃时间
	ConnectionCount int       `json:"connection_count"`  // 连接次数（本次会话）
	ReconnectCount  int       `json:"reconnect_count"`   // 重连次数
	Status          string    `json:"status"`            // 状态 (online/away/busy/offline)

	// ========== 性能信息 ==========
	Latency         int64  `json:"latency_ms"`        // 连接延迟(毫秒)
	ConnectionSpeed string `json:"connection_speed"`  // 网络速度估算
	Quality         string `json:"quality"`           // 连接质量 (good/fair/poor)

	// ========== 扩展元数据 ==========
	Metadata map[string]interface{} `json:"metadata"` // 额外的元数据
	Context  context.Context        `json:"-"`        // 上下文（不序列化）
}

// UserInfoExtractor 用户信息提取器
type UserInfoExtractor struct {
	// 可选：GeoIP查询器
	geoIPLookup func(ip string) (country, region, city, isp string, lat, lon float64)
	
	// 可选：设备指纹提取器
	deviceExtractor func(userAgent string) (platform, browser, os, device string)
	
	// 可选：认证Token验证器
	tokenValidator func(token string) (userID string, expireAt time.Time, err error)
}

// NewUserInfoExtractor 创建用户信息提取器
func NewUserInfoExtractor() *UserInfoExtractor {
	return &UserInfoExtractor{}
}

// WithGeoIPLookup 设置GeoIP查询器
func (e *UserInfoExtractor) WithGeoIPLookup(
	lookup func(ip string) (country, region, city, isp string, lat, lon float64),
) *UserInfoExtractor {
	e.geoIPLookup = lookup
	return e
}

// WithDeviceExtractor 设置设备信息提取器
func (e *UserInfoExtractor) WithDeviceExtractor(
	extractor func(userAgent string) (platform, browser, os, device string),
) *UserInfoExtractor {
	e.deviceExtractor = extractor
	return e
}

// WithTokenValidator 设置Token验证器
func (e *UserInfoExtractor) WithTokenValidator(
	validator func(token string) (userID string, expireAt time.Time, err error),
) *UserInfoExtractor {
	e.tokenValidator = validator
	return e
}

// ExtractUserInfo 从HTTP请求中提取完整的用户信息
func (e *UserInfoExtractor) ExtractUserInfo(r *http.Request) (*UserConnectionInfo, error) {
	ctx := r.Context()
	now := time.Now()

	info := &UserConnectionInfo{
		ConnectedAt:  now,
		LastActiveAt: now,
		Status:       "online",
		Headers:      make(map[string]string),
		Metadata:     make(map[string]interface{}),
		CustomFields: make(map[string]interface{}),
		Context:      ctx,
	}

	// ========== 提取基础身份信息 ==========
	info.UserID = e.extractUserID(ctx, r)
	if info.UserID == "" {
		return nil, errors.ErrUserIDMissing
	}

	info.UserType = e.extractUserType(ctx, r)
	info.Role = e.extractUserRole(ctx, r)
	info.TicketID = r.URL.Query().Get("ticket_id")
	info.ClientID = e.generateClientID(info.UserID, r)

	// ========== 提取网络信息 ==========
	e.extractNetworkInfo(r, info)

	// ========== 提取HTTP请求信息 ==========
	e.extractRequestInfo(r, info)

	// ========== 提取客户端信息 ==========
	e.extractClientInfo(r, info)

	// ========== 提取地理位置信息（如果有GeoIP查询器） ==========
	if e.geoIPLookup != nil && info.RealIP != "" {
		info.Country, info.Region, info.City, info.ISP, info.Latitude, info.Longitude = 
			e.geoIPLookup(info.RealIP)
	}

	// ========== 提取认证信息 ==========
	e.extractAuthInfo(ctx, r, info)

	// ========== 提取业务信息 ==========
	e.extractBusinessInfo(ctx, r, info)

	// ========== 确定协议类型 ==========
	if strings.Contains(r.URL.Path, "/sse") {
		info.Protocol = "sse"
	} else if r.TLS != nil {
		info.Protocol = "wss"
		info.TLSVersion = fmt.Sprintf("TLS %d.%d", r.TLS.Version>>8, r.TLS.Version&0xff)
	} else {
		info.Protocol = "ws"
	}

	return info, nil
}

// extractUserID 提取用户ID
func (e *UserInfoExtractor) extractUserID(ctx context.Context, r *http.Request) string {
	// 优先级: Context > Header > Query > Cookie
	if userID, ok := ctx.Value(gowsc.ContextKeyUserID).(string); ok && userID != "" {
		return userID
	}
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}
	if userID := r.URL.Query().Get("user_id"); userID != "" {
		return userID
	}
	if cookie, err := r.Cookie("user_id"); err == nil {
		return cookie.Value
	}
	return ""
}

// extractUserType 提取用户类型
func (e *UserInfoExtractor) extractUserType(ctx context.Context, r *http.Request) gowsc.UserType {
	if userType, ok := ctx.Value("user_type").(string); ok && userType != "" {
		return gowsc.UserType(userType)
	}
	if userType := r.URL.Query().Get("user_type"); userType != "" {
		return gowsc.UserType(userType)
	}
	return gowsc.UserTypeCustomer
}

// extractUserRole 提取用户角色
func (e *UserInfoExtractor) extractUserRole(ctx context.Context, r *http.Request) gowsc.UserRole {
	if role, ok := ctx.Value("role").(string); ok && role != "" {
		return gowsc.UserRole(role)
	}
	if role := r.URL.Query().Get("role"); role != "" {
		return gowsc.UserRole(role)
	}
	return gowsc.UserRoleCustomer
}

// extractNetworkInfo 提取网络信息
func (e *UserInfoExtractor) extractNetworkInfo(r *http.Request, info *UserConnectionInfo) {
	// 远程地址
	info.RemoteAddr = r.RemoteAddr
	
	// 解析IP和端口
	if host, port, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		info.RemoteIP = host
		info.RemotePort = port
	} else {
		info.RemoteIP = r.RemoteAddr
	}

	// 获取真实IP（考虑代理）
	info.RealIP = e.getRealIP(r)
	info.ForwardedFor = r.Header.Get("X-Forwarded-For")
	info.ProxyChain = r.Header.Get("X-Forwarded-Chain")
	
	// 本地地址
	if r.TLS != nil && r.TLS.ServerName != "" {
		info.LocalAddr = r.TLS.ServerName
	} else {
		info.LocalAddr = r.Host
	}

	// 生成连接ID
	info.ConnectionID = fmt.Sprintf("%s-%d", info.UserID, time.Now().UnixNano())
}

// getRealIP 获取真实IP（处理代理）
func (e *UserInfoExtractor) getRealIP(r *http.Request) string {
	// 优先级: X-Real-IP > X-Forwarded-For > RemoteAddr
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For可能包含多个IP，取第一个
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	
	return r.RemoteAddr
}

// extractRequestInfo 提取HTTP请求信息
func (e *UserInfoExtractor) extractRequestInfo(r *http.Request, info *UserConnectionInfo) {
	info.UserAgent = r.Header.Get("User-Agent")
	info.Origin = r.Header.Get("Origin")
	info.Referer = r.Header.Get("Referer")
	info.AcceptLang = r.Header.Get("Accept-Language")
	info.RequestURI = r.RequestURI
	info.QueryString = r.URL.RawQuery

	// 保存关键请求头
	importantHeaders := []string{
		"Authorization", "X-Request-ID", "X-Session-ID",
		"X-Device-ID", "X-App-Version", "X-Platform",
		"Accept", "Accept-Encoding", "Connection",
	}
	for _, header := range importantHeaders {
		if value := r.Header.Get(header); value != "" {
			info.Headers[header] = value
		}
	}
}

// extractClientInfo 提取客户端信息
func (e *UserInfoExtractor) extractClientInfo(r *http.Request, info *UserConnectionInfo) {
	// 从请求头直接获取
	info.Platform = r.Header.Get("X-Platform")
	info.AppVersion = r.Header.Get("X-App-Version")
	info.DeviceModel = r.Header.Get("X-Device-Model")
	
	// 从URL参数获取
	if platform := r.URL.Query().Get("platform"); platform != "" {
		info.Platform = platform
	}
	if appVer := r.URL.Query().Get("app_version"); appVer != "" {
		info.AppVersion = appVer
	}

	// 解析User-Agent
	ua := info.UserAgent
	if ua != "" {
		if e.deviceExtractor != nil {
			info.Platform, info.Browser, info.OSName, info.DeviceType = 
				e.deviceExtractor(ua)
		} else {
			// 简单的User-Agent解析
			e.parseUserAgent(ua, info)
		}
	}

	// 确定客户端类型
	info.ClientType = e.determineClientType(info)
}

// parseUserAgent 简单的User-Agent解析
func (e *UserInfoExtractor) parseUserAgent(ua string, info *UserConnectionInfo) {
	ua = strings.ToLower(ua)
	
	// 浏览器检测
	if strings.Contains(ua, "chrome") {
		info.Browser = "Chrome"
	} else if strings.Contains(ua, "firefox") {
		info.Browser = "Firefox"
	} else if strings.Contains(ua, "safari") {
		info.Browser = "Safari"
	} else if strings.Contains(ua, "edge") {
		info.Browser = "Edge"
	}

	// 操作系统检测
	if strings.Contains(ua, "windows") {
		info.OSName = "Windows"
		info.DeviceType = "desktop"
	} else if strings.Contains(ua, "mac os") {
		info.OSName = "macOS"
		info.DeviceType = "desktop"
	} else if strings.Contains(ua, "linux") {
		info.OSName = "Linux"
		info.DeviceType = "desktop"
	} else if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		info.OSName = "iOS"
		info.DeviceType = "mobile"
	} else if strings.Contains(ua, "android") {
		info.OSName = "Android"
		info.DeviceType = "mobile"
	}
}

// determineClientType 确定客户端类型
func (e *UserInfoExtractor) determineClientType(info *UserConnectionInfo) gowsc.ClientType {
	if info.Platform != "" {
		platform := strings.ToLower(info.Platform)
		if strings.Contains(platform, "ios") || strings.Contains(platform, "android") {
			return gowsc.ClientTypeMobile
		}
		if strings.Contains(platform, "desktop") || strings.Contains(platform, "electron") {
			return gowsc.ClientTypeDesktop
		}
	}
	
	if info.DeviceType == "mobile" {
		return gowsc.ClientTypeMobile
	}
	
	return gowsc.ClientTypeWeb
}

// extractAuthInfo 提取认证信息
func (e *UserInfoExtractor) extractAuthInfo(ctx context.Context, r *http.Request, info *UserConnectionInfo) {
	// Token
	info.Token = e.extractToken(r)
	
	// Session ID
	info.SessionID = r.Header.Get("X-Session-ID")
	if info.SessionID == "" {
		if cookie, err := r.Cookie("session_id"); err == nil {
			info.SessionID = cookie.Value
		}
	}

	// 认证方式
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			info.AuthMethod = "jwt"
		} else if strings.HasPrefix(authHeader, "Basic ") {
			info.AuthMethod = "basic"
		} else {
			info.AuthMethod = "custom"
		}
	}

	// 使用Token验证器获取更多信息
	if e.tokenValidator != nil && info.Token != "" {
		if userID, expireAt, err := e.tokenValidator(info.Token); err == nil {
			if info.UserID == "" {
				info.UserID = userID
			}
			info.TokenExpireAt = expireAt
		}
	}

	info.AuthTime = time.Now()
}

// extractToken 提取Token
func (e *UserInfoExtractor) extractToken(r *http.Request) string {
	// 优先级: Header > Query > Cookie
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	if cookie, err := r.Cookie("token"); err == nil {
		return cookie.Value
	}
	return ""
}

// extractBusinessInfo 提取业务信息
func (e *UserInfoExtractor) extractBusinessInfo(ctx context.Context, r *http.Request, info *UserConnectionInfo) {
	// 部门（客服）
	if dept, ok := ctx.Value("department").(string); ok {
		info.Department = gowsc.Department(dept)
	} else if dept := r.URL.Query().Get("department"); dept != "" {
		info.Department = gowsc.Department(dept)
	}

	// 技能标签（客服）
	if skills, ok := ctx.Value("skills").([]gowsc.Skill); ok {
		info.Skills = skills
	}

	// 最大工单数（客服）
	if maxTickets, ok := ctx.Value("max_tickets").(int); ok {
		info.MaxTickets = maxTickets
	}

	// 用户标签
	if tags, ok := ctx.Value("tags").([]string); ok {
		info.Tags = tags
	}

	// 时区
	info.Timezone = r.Header.Get("X-Timezone")
	if info.Timezone == "" {
		info.Timezone = "UTC"
	}
}

// generateClientID 生成客户端ID
func (e *UserInfoExtractor) generateClientID(userID string, r *http.Request) string {
	return fmt.Sprintf("client_%s_%d", userID, time.Now().UnixNano())
}

// ToWSCClient 转换为go-wsc的Client对象
func (info *UserConnectionInfo) ToWSCClient() *gowsc.Client {
	return &gowsc.Client{
		ID:         info.ClientID,
		UserID:     info.UserID,
		UserType:   info.UserType,
		Role:       info.Role,
		TicketID:   info.TicketID,
		LastSeen:   info.LastActiveAt,
		Status:     gowsc.UserStatus(info.Status),
		Department: info.Department,
		Skills:     info.Skills,
		ClientType: info.ClientType,
		Context:    info.Context,
		Metadata: map[string]interface{}{
			"connection_info": info,
			"real_ip":         info.RealIP,
			"user_agent":      info.UserAgent,
			"platform":        info.Platform,
			"app_version":     info.AppVersion,
		},
	}
}

