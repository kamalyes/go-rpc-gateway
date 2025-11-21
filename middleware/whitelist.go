/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 16:00:00
 * @FilePath: \go-rpc-gateway\middleware\whitelist.go
 * @Description: 通用白名单中间件 - 支持灵活的规则配置
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"net"
	"regexp"
	"strings"
	"sync"
)

// ============================================================================
// 白名单规则接口
// ============================================================================

// WhitelistRule 白名单规则接口
type WhitelistRule interface {
	// Match 检查请求是否匹配规则
	Match(method, path string) bool
	// Description 获取规则描述
	Description() string
	// Priority 获取规则优先级（数字越小优先级越高）
	Priority() int
}

// ============================================================================
// 白名单管理器
// ============================================================================

// WhitelistManager 白名单管理器
type WhitelistManager struct {
	rules   []WhitelistRule
	ipRules []IPWhitelistRule // IP 相关规则单独存储
	mu      sync.RWMutex
}

// NewWhitelistManager 创建白名单管理器
func NewWhitelistManager() *WhitelistManager {
	return &WhitelistManager{
		rules:   make([]WhitelistRule, 0),
		ipRules: make([]IPWhitelistRule, 0),
	}
}

// Register 注册白名单规则
func (m *WhitelistManager) Register(rule WhitelistRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否是 IP 规则
	if ipRule, ok := rule.(IPWhitelistRule); ok {
		// IP 规则单独存储
		inserted := false
		for i, r := range m.ipRules {
			if rule.Priority() < r.Priority() {
				m.ipRules = append(m.ipRules[:i], append([]IPWhitelistRule{ipRule}, m.ipRules[i:]...)...)
				inserted = true
				break
			}
		}
		if !inserted {
			m.ipRules = append(m.ipRules, ipRule)
		}
	}

	// 按优先级插入排序
	inserted := false
	for i, r := range m.rules {
		if rule.Priority() < r.Priority() {
			m.rules = append(m.rules[:i], append([]WhitelistRule{rule}, m.rules[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		m.rules = append(m.rules, rule)
	}
}

// IsWhitelisted 检查请求是否在白名单中
func (m *WhitelistManager) IsWhitelisted(method, path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rule := range m.rules {
		if rule.Match(method, path) {
			return true
		}
	}
	return false
}

// IsWhitelistedWithIP 检查请求是否在白名单中（包含 IP 检查）
func (m *WhitelistManager) IsWhitelistedWithIP(method, path, clientIP string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 先检查 IP 规则
	for _, ipRule := range m.ipRules {
		if ipRule.MatchWithIP(clientIP) {
			return true
		}
	}

	// 再检查普通规则
	for _, rule := range m.rules {
		if rule.Match(method, path) {
			return true
		}
	}
	return false
}

// GetRules 获取所有规则（用于调试）
func (m *WhitelistManager) GetRules() []WhitelistRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]WhitelistRule, len(m.rules))
	copy(rules, m.rules)
	return rules
}

// Clear 清空所有规则
func (m *WhitelistManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = make([]WhitelistRule, 0)
	m.ipRules = make([]IPWhitelistRule, 0)
}

// ============================================================================
// 内置规则实现
// ============================================================================

// PathPrefixRule 路径前缀匹配规则
type PathPrefixRule struct {
	prefix      string
	description string
	priority    int
}

func (r *PathPrefixRule) Match(method, path string) bool {
	return strings.HasPrefix(path, r.prefix)
}

func (r *PathPrefixRule) Description() string {
	return r.description
}

func (r *PathPrefixRule) Priority() int {
	return r.priority
}

// ExactPathRule 精确路径匹配规则
type ExactPathRule struct {
	method      string
	path        string
	description string
	priority    int
}

func (r *ExactPathRule) Match(method, path string) bool {
	return r.method == method && strings.EqualFold(path, r.path)
}

func (r *ExactPathRule) Description() string {
	return r.description
}

func (r *ExactPathRule) Priority() int {
	return r.priority
}

// PathSuffixRule 路径后缀匹配规则
type PathSuffixRule struct {
	suffix      string
	description string
	priority    int
}

func (r *PathSuffixRule) Match(method, path string) bool {
	return strings.HasSuffix(path, r.suffix)
}

func (r *PathSuffixRule) Description() string {
	return r.description
}

func (r *PathSuffixRule) Priority() int {
	return r.priority
}

// RegexRule 正则表达式匹配规则
type RegexRule struct {
	pattern     *regexp.Regexp
	description string
	priority    int
}

func (r *RegexRule) Match(method, path string) bool {
	return r.pattern.MatchString(path)
}

func (r *RegexRule) Description() string {
	return r.description
}

func (r *RegexRule) Priority() int {
	return r.priority
}

// MethodRule 仅匹配 HTTP 方法的规则
type MethodRule struct {
	methods     []string
	description string
	priority    int
}

func (r *MethodRule) Match(method, path string) bool {
	for _, m := range r.methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

func (r *MethodRule) Description() string {
	return r.description
}

func (r *MethodRule) Priority() int {
	return r.priority
}

// CustomRule 自定义匹配函数规则
type CustomRule struct {
	matchFunc   func(method, path string) bool
	description string
	priority    int
}

func (r *CustomRule) Match(method, path string) bool {
	return r.matchFunc(method, path)
}

func (r *CustomRule) Description() string {
	return r.description
}

func (r *CustomRule) Priority() int {
	return r.priority
}

// IPRule IP 地址匹配规则
type IPRule struct {
	allowedIPs  []string
	description string
	priority    int
}

func (r *IPRule) Match(method, path string) bool {
	// IP 规则需要从请求中获取，这里暂时返回 false
	// 实际使用时需要配合 MatchWithIP 方法
	return false
}

func (r *IPRule) MatchWithIP(clientIP string) bool {
	for _, ip := range r.allowedIPs {
		if ip == clientIP {
			return true
		}
	}
	return false
}

func (r *IPRule) Description() string {
	return r.description
}

func (r *IPRule) Priority() int {
	return r.priority
}

// CIDRRule CIDR 网段匹配规则
type CIDRRule struct {
	allowedNets []*net.IPNet
	description string
	priority    int
}

func (r *CIDRRule) Match(method, path string) bool {
	// CIDR 规则需要从请求中获取 IP，这里暂时返回 false
	// 实际使用时需要配合 MatchWithIP 方法
	return false
}

func (r *CIDRRule) MatchWithIP(clientIP string) bool {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	for _, ipNet := range r.allowedNets {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func (r *CIDRRule) Description() string {
	return r.description
}

func (r *CIDRRule) Priority() int {
	return r.priority
}

// ============================================================================
// 扩展的白名单规则接口（支持 IP 检查）
// ============================================================================

// IPWhitelistRule 支持 IP 检查的白名单规则接口
type IPWhitelistRule interface {
	WhitelistRule
	// MatchWithIP 检查 IP 是否匹配规则
	MatchWithIP(clientIP string) bool
}

// ============================================================================
// 规则构建器（Builder Pattern）
// ============================================================================

// RuleBuilder 规则构建器
type RuleBuilder struct {
	manager *WhitelistManager
}

// NewRuleBuilder 创建规则构建器
func NewRuleBuilder(manager *WhitelistManager) *RuleBuilder {
	return &RuleBuilder{manager: manager}
}

// AddPathPrefix 添加路径前缀规则
func (b *RuleBuilder) AddPathPrefix(prefix, description string) *RuleBuilder {
	b.manager.Register(&PathPrefixRule{
		prefix:      prefix,
		description: description,
		priority:    100,
	})
	return b
}

// AddPathPrefixWithPriority 添加路径前缀规则（指定优先级）
func (b *RuleBuilder) AddPathPrefixWithPriority(prefix, description string, priority int) *RuleBuilder {
	b.manager.Register(&PathPrefixRule{
		prefix:      prefix,
		description: description,
		priority:    priority,
	})
	return b
}

// AddExactPath 添加精确路径规则
func (b *RuleBuilder) AddExactPath(method, path, description string) *RuleBuilder {
	b.manager.Register(&ExactPathRule{
		method:      method,
		path:        path,
		description: description,
		priority:    50, // 精确匹配优先级更高
	})
	return b
}

// AddPathSuffix 添加路径后缀规则
func (b *RuleBuilder) AddPathSuffix(suffix, description string) *RuleBuilder {
	b.manager.Register(&PathSuffixRule{
		suffix:      suffix,
		description: description,
		priority:    150,
	})
	return b
}

// AddRegex 添加正则表达式规则
func (b *RuleBuilder) AddRegex(pattern, description string) *RuleBuilder {
	regex := regexp.MustCompile(pattern)
	b.manager.Register(&RegexRule{
		pattern:     regex,
		description: description,
		priority:    200,
	})
	return b
}

// AddMethods 添加 HTTP 方法规则
func (b *RuleBuilder) AddMethods(methods []string, description string) *RuleBuilder {
	b.manager.Register(&MethodRule{
		methods:     methods,
		description: description,
		priority:    300,
	})
	return b
}

// AddCustom 添加自定义规则
func (b *RuleBuilder) AddCustom(matchFunc func(method, path string) bool, description string, priority int) *RuleBuilder {
	b.manager.Register(&CustomRule{
		matchFunc:   matchFunc,
		description: description,
		priority:    priority,
	})
	return b
}

// AddIP 添加 IP 地址规则
func (b *RuleBuilder) AddIP(ips []string, description string) *RuleBuilder {
	b.manager.Register(&IPRule{
		allowedIPs:  ips,
		description: description,
		priority:    5, // IP 规则优先级很高
	})
	return b
}

// AddCIDR 添加 CIDR 网段规则
func (b *RuleBuilder) AddCIDR(cidrs []string, description string) *RuleBuilder {
	ipNets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			// 忽略无效的 CIDR
			continue
		}
		ipNets = append(ipNets, ipNet)
	}

	b.manager.Register(&CIDRRule{
		allowedNets: ipNets,
		description: description,
		priority:    5, // CIDR 规则优先级很高
	})
	return b
}

// Build 完成构建
func (b *RuleBuilder) Build() *WhitelistManager {
	return b.manager
}

// ============================================================================
// 预设规则集
// ============================================================================

// CommonRules 常用规则集
type CommonRules struct{}

// HealthCheck 健康检查规则
func (CommonRules) HealthCheck() WhitelistRule {
	return &PathPrefixRule{
		prefix:      "/health",
		description: "健康检查端点",
		priority:    10,
	}
}

// Metrics 监控指标规则
func (CommonRules) Metrics() WhitelistRule {
	return &PathPrefixRule{
		prefix:      "/metrics",
		description: "Prometheus 监控指标",
		priority:    10,
	}
}

// StaticFiles 静态文件规则
func (CommonRules) StaticFiles(prefix string) WhitelistRule {
	return &PathPrefixRule{
		prefix:      prefix,
		description: "静态文件资源",
		priority:    100,
	}
}

// PublicAPI 公开 API 规则
func (CommonRules) PublicAPI(prefix string) WhitelistRule {
	return &PathPrefixRule{
		prefix:      prefix,
		description: "公开 API",
		priority:    80,
	}
}

// Swagger Swagger 文档规则
func (CommonRules) Swagger() WhitelistRule {
	return &PathPrefixRule{
		prefix:      "/swagger",
		description: "Swagger API 文档",
		priority:    10,
	}
}

// Pprof 性能分析规则
func (CommonRules) Pprof() WhitelistRule {
	return &PathPrefixRule{
		prefix:      "/debug/pprof",
		description: "性能分析端点",
		priority:    10,
	}
}

// ============================================================================
// 全局默认管理器
// ============================================================================

var (
	defaultWhitelistManager     *WhitelistManager
	defaultWhitelistManagerOnce sync.Once
)

// DefaultWhitelistManager 获取全局默认白名单管理器
func DefaultWhitelistManager() *WhitelistManager {
	defaultWhitelistManagerOnce.Do(func() {
		defaultWhitelistManager = NewWhitelistManager()
	})
	return defaultWhitelistManager
}

// RegisterWhitelistRule 注册规则到默认管理器（便捷方法）
func RegisterWhitelistRule(rule WhitelistRule) {
	DefaultWhitelistManager().Register(rule)
}

// IsWhitelisted 检查是否在默认白名单中（便捷方法）
func IsWhitelisted(method, path string) bool {
	return DefaultWhitelistManager().IsWhitelisted(method, path)
}

// IsWhitelistedWithIP 检查是否在默认白名单中（包含 IP 检查）
func IsWhitelistedWithIP(method, path, clientIP string) bool {
	return DefaultWhitelistManager().IsWhitelistedWithIP(method, path, clientIP)
}
