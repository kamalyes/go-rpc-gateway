/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-30 08:22:08
 * @FilePath: \go-rpc-gateway\config\env.go
 * @Description: 重构后的环境管理器，提供线程安全的环境管理和别名支持
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// EnvironmentType 定义环境类型
type EnvironmentType string

// 定义有效的环境常量
const (
	// 标准环境
	EnvDevelopment EnvironmentType = "development" // 开发环境
	EnvTest        EnvironmentType = "test"        // 测试环境
	EnvStaging     EnvironmentType = "staging"     // 预发布环境
	EnvUAT         EnvironmentType = "uat"         // 用户验收测试环境
	EnvProduction  EnvironmentType = "production"  // 生产环境
	EnvLocal       EnvironmentType = "local"       // 本地环境
	EnvDebug       EnvironmentType = "debug"       // 调试环境
	EnvDemo        EnvironmentType = "demo"        // 演示环境
	EnvIntegration EnvironmentType = "integration" // 集成环境

	// 亚洲国家/地区
	EnvChina       EnvironmentType = "china"       // 中国
	EnvJapan       EnvironmentType = "japan"       // 日本
	EnvKorea       EnvironmentType = "korea"       // 韩国
	EnvIndia       EnvironmentType = "india"       // 印度
	EnvSingapore   EnvironmentType = "singapore"   // 新加坡
	EnvThailand    EnvironmentType = "thailand"    // 泰国
	EnvVietnam     EnvironmentType = "vietnam"     // 越南
	EnvMalaysia    EnvironmentType = "malaysia"    // 马来西亚
	EnvIndonesia   EnvironmentType = "indonesia"   // 印度尼西亚
	EnvPhilippines EnvironmentType = "philippines" // 菲律宾
	EnvMyanmar     EnvironmentType = "myanmar"     // 缅甸
	EnvLaos        EnvironmentType = "laos"        // 老挝
	EnvCambodia    EnvironmentType = "cambodia"    // 柬埔寨
	EnvPakistan    EnvironmentType = "pakistan"    // 巴基斯坦
	EnvBangladesh  EnvironmentType = "bangladesh"  // 孟加拉国
	EnvSriLanka    EnvironmentType = "srilanka"    // 斯里兰卡
	EnvNepal       EnvironmentType = "nepal"       // 尼泊尔
	EnvHongKong    EnvironmentType = "hongkong"    // 香港
	EnvTaiwan      EnvironmentType = "taiwan"      // 台湾
	EnvMacao       EnvironmentType = "macao"       // 澳门

	// 欧洲国家
	EnvUK          EnvironmentType = "uk"          // 英国
	EnvGermany     EnvironmentType = "germany"     // 德国
	EnvFrance      EnvironmentType = "france"      // 法国
	EnvItaly       EnvironmentType = "italy"       // 意大利
	EnvSpain       EnvironmentType = "spain"       // 西班牙
	EnvNetherlands EnvironmentType = "netherlands" // 荷兰
	EnvBelgium     EnvironmentType = "belgium"     // 比利时
	EnvSwitzerland EnvironmentType = "switzerland" // 瑞士
	EnvAustria     EnvironmentType = "austria"     // 奥地利
	EnvSweden      EnvironmentType = "sweden"      // 瑞典
	EnvNorway      EnvironmentType = "norway"      // 挪威
	EnvDenmark     EnvironmentType = "denmark"     // 丹麦
	EnvFinland     EnvironmentType = "finland"     // 芬兰
	EnvPoland      EnvironmentType = "poland"      // 波兰
	EnvRussia      EnvironmentType = "russia"      // 俄罗斯
	EnvTurkey      EnvironmentType = "turkey"      // 土耳其

	// 北美洲国家
	EnvUSA    EnvironmentType = "usa"    // 美国
	EnvCanada EnvironmentType = "canada" // 加拿大
	EnvMexico EnvironmentType = "mexico" // 墨西哥

	// 南美洲国家
	EnvBrazil    EnvironmentType = "brazil"    // 巴西
	EnvArgentina EnvironmentType = "argentina" // 阿根廷
	EnvChile     EnvironmentType = "chile"     // 智利
	EnvColombia  EnvironmentType = "colombia"  // 哥伦比亚
	EnvPeru      EnvironmentType = "peru"      // 秘鲁

	// 大洋洲国家
	EnvAustralia  EnvironmentType = "australia"  // 澳大利亚
	EnvNewZealand EnvironmentType = "newzealand" // 新西兰

	// 非洲国家
	EnvSouthAfrica EnvironmentType = "southafrica" // 南非
	EnvEgypt       EnvironmentType = "egypt"       // 埃及
	EnvNigeria     EnvironmentType = "nigeria"     // 尼日利亚
	EnvKenya       EnvironmentType = "kenya"       // 肯尼亚

	// 中东国家
	EnvUAE         EnvironmentType = "uae"         // 阿联酋
	EnvSaudiArabia EnvironmentType = "saudiarabia" // 沙特阿拉伯
	EnvIsrael      EnvironmentType = "israel"      // 以色列
	EnvQatar       EnvironmentType = "qatar"       // 卡塔尔

	// 向后兼容的别名
	Dev  = EnvDevelopment
	Sit  = EnvTest
	Fat  = EnvStaging
	Uat  = EnvUAT
	Prod = EnvProduction

	DefaultEnv EnvironmentType = EnvDevelopment
)

// DefaultEnvPrefixes 全局环境前缀映射（统一定义，避免重复）
// 覆盖常见的环境命名约定，支持多种风格：短名、全名、连字符等
var DefaultEnvPrefixes = map[EnvironmentType][]string{
	// 标准环境
	EnvDevelopment: {"dev", "develop", "development"},
	EnvLocal:       {"local", "localhost"},
	EnvTest:        {"test", "testing", "qa", "sit"},
	EnvStaging:     {"staging", "stage", "stg", "pre", "preprod", "pre-prod", "fat", "gray", "grey", "canary"},
	EnvProduction:  {"prod", "production", "prd", "release", "live", "online", "master", "main"},
	EnvDebug:       {"debug", "debugging", "dbg"},
	EnvDemo:        {"demo", "demonstration", "showcase", "preview", "sandbox"},
	EnvUAT:         {"uat", "acceptance", "user-acceptance", "beta"},
	EnvIntegration: {"integration", "int", "ci", "integration-test", "integ"},

	// 亚洲国家/地区
	EnvChina:       {"china", "cn", "chn"},
	EnvJapan:       {"japan", "jp", "jpn"},
	EnvKorea:       {"korea", "kr", "kor", "south-korea", "southkorea"},
	EnvIndia:       {"india", "in", "ind"},
	EnvSingapore:   {"singapore", "sg", "sgp"},
	EnvThailand:    {"thailand", "th", "tha", "thai"},
	EnvVietnam:     {"vietnam", "vn", "vnm", "viet"},
	EnvMalaysia:    {"malaysia", "my", "mys"},
	EnvIndonesia:   {"indonesia", "id", "idn"},
	EnvPhilippines: {"philippines", "ph", "phl"},
	EnvMyanmar:     {"myanmar", "mm", "mmr", "burma"},
	EnvLaos:        {"laos", "la", "lao"},
	EnvCambodia:    {"cambodia", "kh", "khm"},
	EnvPakistan:    {"pakistan", "pk", "pak"},
	EnvBangladesh:  {"bangladesh", "bd", "bgd"},
	EnvSriLanka:    {"srilanka", "lk", "lka", "sri-lanka"},
	EnvNepal:       {"nepal", "np", "npl"},
	EnvHongKong:    {"hongkong", "hk", "hkg", "hong-kong"},
	EnvTaiwan:      {"taiwan", "tw", "twn"},
	EnvMacao:       {"macao", "mo", "mac", "macau"},

	// 欧洲国家
	EnvUK:          {"uk", "gb", "gbr", "united-kingdom", "britain", "england"},
	EnvGermany:     {"germany", "de", "deu", "deutschland"},
	EnvFrance:      {"france", "fr", "fra"},
	EnvItaly:       {"italy", "it", "ita", "italia"},
	EnvSpain:       {"spain", "es", "esp", "espana"},
	EnvNetherlands: {"netherlands", "nl", "nld", "holland"},
	EnvBelgium:     {"belgium", "be", "bel"},
	EnvSwitzerland: {"switzerland", "ch", "che"},
	EnvAustria:     {"austria", "at", "aut"},
	EnvSweden:      {"sweden", "se", "swe"},
	EnvNorway:      {"norway", "no", "nor"},
	EnvDenmark:     {"denmark", "dk", "dnk"},
	EnvFinland:     {"finland", "fi", "fin"},
	EnvPoland:      {"poland", "pl", "pol"},
	EnvRussia:      {"russia", "ru", "rus"},
	EnvTurkey:      {"turkey", "tr", "tur"},

	// 北美洲国家
	EnvUSA:    {"usa", "us", "united-states", "america"},
	EnvCanada: {"canada", "ca", "can"},
	EnvMexico: {"mexico", "mx", "mex"},

	// 南美洲国家
	EnvBrazil:    {"brazil", "br", "bra", "brasil"},
	EnvArgentina: {"argentina", "ar", "arg"},
	EnvChile:     {"chile", "cl", "chl"},
	EnvColombia:  {"colombia", "co", "col"},
	EnvPeru:      {"peru", "pe", "per"},

	// 大洋洲国家
	EnvAustralia:  {"australia", "au", "aus"},
	EnvNewZealand: {"newzealand", "nz", "nzl", "new-zealand"},

	// 非洲国家
	EnvSouthAfrica: {"southafrica", "za", "zaf", "south-africa"},
	EnvEgypt:       {"egypt", "eg", "egy"},
	EnvNigeria:     {"nigeria", "ng", "nga"},
	EnvKenya:       {"kenya", "ke", "ken"},

	// 中东国家
	EnvUAE:         {"uae", "ae", "are", "emirates", "dubai"},
	EnvSaudiArabia: {"saudiarabia", "sa", "sau", "saudi-arabia", "saudi"},
	EnvIsrael:      {"israel", "il", "isr"},
	EnvQatar:       {"qatar", "qa", "qat"},
}

// DefaultSupportedExtensions 默认支持的配置文件扩展名
var DefaultSupportedExtensions = []string{".yaml", ".yml", ".json", ".toml", ".properties"}

// DefaultConfigNames 默认配置文件名
var DefaultConfigNames = []string{"config", "application", "app", "gateway", "service"}

// envPrefixesMutex 保护 DefaultEnvPrefixes 的并发访问
var envPrefixesMutex sync.RWMutex

// RegisterEnvPrefixes 注册自定义环境前缀（包级别便捷函数）
// 内部调用 GetGlobalEnvManager().RegisterEnvironment()，保持逻辑统一
// 示例:
//
//	func init() {
//	    boot.RegisterEnvPrefixes("custom", "custom", "my-env", "myenv")
//	}
func RegisterEnvPrefixes(env EnvironmentType, prefixes ...string) {
	GetGlobalEnvManager().RegisterEnvironment(env, prefixes...)
}

// GetEnvPrefixes 获取指定环境的前缀列表
func GetEnvPrefixes(env EnvironmentType) []string {
	envPrefixesMutex.RLock()
	defer envPrefixesMutex.RUnlock()

	if prefixes, ok := DefaultEnvPrefixes[env]; ok {
		// 返回副本，避免外部修改
		result := make([]string, len(prefixes))
		copy(result, prefixes)
		return result
	}
	return nil
}

// ListAllEnvPrefixes 列出所有已注册的环境前缀
func ListAllEnvPrefixes() map[EnvironmentType][]string {
	envPrefixesMutex.RLock()
	defer envPrefixesMutex.RUnlock()

	result := make(map[EnvironmentType][]string)
	for env, prefixes := range DefaultEnvPrefixes {
		copied := make([]string, len(prefixes))
		copy(copied, prefixes)
		result[env] = copied
	}
	return result
}

// String 返回环境类型的字符串表示
func (e EnvironmentType) String() string {
	return string(e)
}

// IsValid 检查环境类型是否有效
func (e EnvironmentType) IsValid() bool {
	return defaultEnvManager.IsRegistered(e)
}

// 定义上下文键类型
type ContextKey string

// String 返回上下文键的字符串表示
func (c ContextKey) String() string {
	return string(c)
}

var (
	envContextKey   ContextKey = "APP_ENV" // 默认上下文键
	contextKeyMutex sync.Mutex             // 互斥锁，用于保护 envContextKey
)

// ContextKeyOptions 定义上下文键的选项
type ContextKeyOptions struct {
	Key   ContextKey
	Value EnvironmentType
}

// EnvironmentManager 环境管理器
type EnvironmentManager struct {
	environments map[EnvironmentType][]string // 环境类型到别名的映射
	mu           sync.RWMutex                 // 读写锁
}

// NewEnvironmentManager 创建新的环境管理器
func NewEnvironmentManager() *EnvironmentManager {
	manager := &EnvironmentManager{
		environments: make(map[EnvironmentType][]string),
	}

	// 注册默认环境和它们的别名
	manager.registerDefaultEnvironments()
	return manager
}

// registerDefaultEnvironments 注册默认环境
// 直接从 DefaultEnvPrefixes 读取，保持单一数据源
func (em *EnvironmentManager) registerDefaultEnvironments() {
	for envType, prefixes := range DefaultEnvPrefixes {
		em.registerEnvironmentInternal(envType, prefixes...)
	}
	logger.GetGlobalLogger().DebugKV("注册环境类型", DefaultEnvPrefixes)
}

// registerEnvironmentInternal 内部注册方法，不会反向更新 DefaultEnvPrefixes
func (em *EnvironmentManager) registerEnvironmentInternal(envType EnvironmentType, aliases ...string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// 添加环境类型本身作为别名
	allAliases := []string{string(envType)}
	allAliases = append(allAliases, aliases...)

	// 标准化别名（转换为小写并去除空格）
	for i, alias := range allAliases {
		allAliases[i] = strings.ToLower(strings.TrimSpace(alias))
	}

	em.environments[envType] = allAliases
}

// RegisterEnvironment 注册环境类型及其别名
// 同时会更新 DefaultEnvPrefixes，保持环境别名和配置文件前缀的一致性
func (em *EnvironmentManager) RegisterEnvironment(envType EnvironmentType, aliases ...string) {
	// 先更新 DefaultEnvPrefixes
	envPrefixesMutex.Lock()
	DefaultEnvPrefixes[envType] = aliases
	envPrefixesMutex.Unlock()

	// 再注册到环境管理器
	em.registerEnvironmentInternal(envType, aliases...)
}

// IsRegistered 检查环境类型是否已注册
func (em *EnvironmentManager) IsRegistered(envType EnvironmentType) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()

	_, exists := em.environments[envType]
	return exists
}

// IsEnvironment 检查给定的环境字符串是否属于指定的环境类型
func (em *EnvironmentManager) IsEnvironment(envString string, envType EnvironmentType) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()

	envString = strings.ToLower(strings.TrimSpace(envString))

	if aliases, exists := em.environments[envType]; exists {
		for _, alias := range aliases {
			if alias == envString {
				return true
			}
		}
	}

	return false
}

// DetectEnvironmentType 检测环境字符串对应的环境类型
func (em *EnvironmentManager) DetectEnvironmentType(envString string) (EnvironmentType, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	envString = strings.ToLower(strings.TrimSpace(envString))

	for envType, aliases := range em.environments {
		for _, alias := range aliases {
			if alias == envString {
				return envType, true
			}
		}
	}

	return "", false
}

// 全局环境管理器实例
var defaultEnvManager = NewEnvironmentManager()

// init 包初始化时自动初始化全局环境实例
func init() {
	// 自动初始化全局环境实例
	initGlobalEnvironment()
	logger.GetGlobalLogger().DebugKV("go-config包已自动初始化", "environment", GetCurrentEnvironment())
}

// GetGlobalEnvManager 获取全局环境管理器
// 可用于注册自定义环境类型及其别名
func GetGlobalEnvManager() *EnvironmentManager {
	return defaultEnvManager
}

// EnvironmentCallback 环境变更回调函数类型
type EnvironmentCallback func(oldEnv, newEnv EnvironmentType) error

// EnvironmentCallbackInfo 环境回调信息
type EnvironmentCallbackInfo struct {
	ID       string              // 回调唯一标识
	Callback EnvironmentCallback // 回调函数
	Priority int                 // 优先级（数字越小优先级越高）
	Async    bool                // 是否异步执行
}

// 环境配置
type Environment struct {
	CheckFrequency time.Duration                       // 检查频率
	mu             sync.RWMutex                        // 读写互斥锁
	quit           chan struct{}                       // 用于停止监控的信道
	Value          EnvironmentType                     // 当前环境值
	registeredEnvs []EnvironmentType                   // 注册的环境类型
	callbacks      map[string]*EnvironmentCallbackInfo // 环境变更回调
}

// NewEnvironment 创建一个新的 Environment 实例，并将环境变量写入上下文
func NewEnvironment() *Environment {
	envInstance := &Environment{
		Value:          GetEnvironment(),
		CheckFrequency: 2 * time.Second, // 默认检查频率
		quit:           make(chan struct{}),
		registeredEnvs: []EnvironmentType{Dev, Sit, Fat, Uat, Prod}, // 默认注册的环境类型
		callbacks:      make(map[string]*EnvironmentCallbackInfo),   // 初始化回调映射
	}
	if err := setEnv(GetContextKey(), envInstance.Value); err != nil {
		logger.GetGlobalLogger().FatalKV("初始化环境失败", "error", err)
	}

	go envInstance.watchEnv() // 启动监控环境变量的 goroutine

	return envInstance
}

// RegisterCallback 注册环境变更回调
func (e *Environment) RegisterCallback(id string, callback EnvironmentCallback, priority int, async bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if id == "" {
		return ErrCallbackIDEmpty
	}

	if _, exists := e.callbacks[id]; exists {
		return ErrCallbackIDExists(id)
	}

	e.callbacks[id] = &EnvironmentCallbackInfo{
		ID:       id,
		Callback: callback,
		Priority: priority,
		Async:    async,
	}

	logger.GetGlobalLogger().DebugKV("注册环境变更回调", "id", id, "priority", priority, "async", async)
	return nil
}

// UnregisterCallback 注销环境变更回调
func (e *Environment) UnregisterCallback(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.callbacks[id]; !exists {
		return ErrCallbackIDNotFound(id)
	}

	delete(e.callbacks, id)
	logger.GetGlobalLogger().DebugKV("注销环境变更回调", "id", id)
	return nil
}

// triggerCallbacks 触发环境变更回调
func (e *Environment) triggerCallbacks(oldEnv, newEnv EnvironmentType) {
	e.mu.RLock()
	// 复制回调列表避免死锁
	callbacks := make([]*EnvironmentCallbackInfo, 0, len(e.callbacks))
	for _, cb := range e.callbacks {
		callbacks = append(callbacks, cb)
	}
	e.mu.RUnlock()

	if len(callbacks) == 0 {
		return
	}

	// 按优先级排序
	for i := 0; i < len(callbacks)-1; i++ {
		for j := i + 1; j < len(callbacks); j++ {
			if callbacks[i].Priority > callbacks[j].Priority {
				callbacks[i], callbacks[j] = callbacks[j], callbacks[i]
			}
		}
	}

	logger.GetGlobalLogger().DebugKV("触发环境变更回调", "count", len(callbacks), "oldEnv", oldEnv, "newEnv", newEnv)

	// 执行回调
	for _, cb := range callbacks {
		if cb.Async {
			go e.executeCallback(cb, oldEnv, newEnv)
		} else {
			e.executeCallback(cb, oldEnv, newEnv)
		}
	}
}

// executeCallback 执行单个回调
func (e *Environment) executeCallback(cb *EnvironmentCallbackInfo, oldEnv, newEnv EnvironmentType) {
	defer func() {
		if r := recover(); r != nil {
			logger.GetGlobalLogger().ErrorKV("环境变更回调执行时发生panic", "id", cb.ID, "panic", r)
		}
	}()

	if err := cb.Callback(oldEnv, newEnv); err != nil {
		logger.GetGlobalLogger().ErrorKV("环境变更回调执行失败", "id", cb.ID, "error", err)
	} else {
		logger.GetGlobalLogger().DebugKV("环境变更回调执行成功", "id", cb.ID)
	}
}

// ListCallbacks 列出所有环境变更回调
func (e *Environment) ListCallbacks() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]string, 0, len(e.callbacks))
	for id := range e.callbacks {
		ids = append(ids, id)
	}
	return ids
}

// ClearCallbacks 清除所有环境变更回调
func (e *Environment) ClearCallbacks() {
	e.mu.Lock()
	defer e.mu.Unlock()

	count := len(e.callbacks)
	e.callbacks = make(map[string]*EnvironmentCallbackInfo)
	logger.GetGlobalLogger().DebugKV("已清除环境变更回调", "count", count)
}

// RegisterEnvironment 注册新的环境类型
func (e *Environment) RegisterEnvironment(env EnvironmentType) error {
	e.mu.Lock()         // 获取写锁
	defer e.mu.Unlock() // 确保在函数结束时释放锁

	// 检查是否已经存在该环境类型
	for _, registeredEnv := range e.registeredEnvs {
		if registeredEnv == env {
			return ErrEnvTypeRegistered(env)
		}
	}

	// 注册新的环境类型
	e.registeredEnvs = append(e.registeredEnvs, env)
	logger.GetGlobalLogger().DebugKV("环境类型注册成功", "type", env)
	return nil
}

// SetContextKey 设置上下文键
// options: 指定的上下文键选项，如果为 nil，将使用默认值
func SetContextKey(options *ContextKeyOptions) {
	contextKeyMutex.Lock()         // 获取锁
	defer contextKeyMutex.Unlock() // 确保在函数结束时释放锁

	// 如果没有提供 options，则创建一个默认的 ContextKeyOptions
	if options == nil {
		options = &ContextKeyOptions{}
	}

	// 如果没有提供 Key，则使用默认值
	if options.Key == "" {
		options.Key = envContextKey // 使用当前的全局上下文键
	}

	// 如果没有提供 Value 则使用默认环境值
	if options.Value == "" {
		options.Value = DefaultEnv // 使用默认环境值
	}

	envContextKey = options.Key

	// 设置环境变量
	if err := setEnv(envContextKey, options.Value); err != nil {
		logger.GetGlobalLogger().ErrorKV("设置环境变量失败", "error", err)
	}
}

// GetContextKey 获取当前的上下文键
func GetContextKey() ContextKey {
	contextKeyMutex.Lock()         // 获取锁
	defer contextKeyMutex.Unlock() // 确保在函数结束时释放锁

	return envContextKey // 返回当前的上下文键
}

// SetEnvironment 设置环境变量
func (e *Environment) SetEnvironment(value EnvironmentType) *Environment {
	// 先在 e.mu 之外读取上下文键，避免 contextKeyMutex 与 e.mu 嵌套造成锁顺序倒置
	ctxKey := GetContextKey()

	e.mu.Lock() // 获取写锁

	oldValue := e.Value
	e.Value = value // 更新环境值

	if err := setEnv(ctxKey, value); err != nil {
		logger.GetGlobalLogger().ErrorKV("设置环境失败", "value", value, "error", err)
	}

	shouldTrigger := oldValue != value
	e.mu.Unlock() // 释放锁

	// 如果环境值发生变化，在锁外触发回调
	if shouldTrigger {
		e.triggerCallbacks(oldValue, value)
	}

	return e // 返回当前环境实例以支持链式调用
}

// CheckAndUpdateEnv 检查并更新环境变量
func (e *Environment) CheckAndUpdateEnv() {
	// 先在 e.mu 之外读取上下文键，避免 contextKeyMutex 与 e.mu 嵌套造成锁顺序倒置
	ctxKey := GetContextKey()

	e.mu.RLock() // 获取读锁
	currentOsEnv := os.Getenv(ctxKey.String())
	oldValue := e.Value
	e.mu.RUnlock() // 释放读锁

	if currentOsEnv == "" {
		logger.GetGlobalLogger().WarnKV("环境变量当前为空", "key", ctxKey)
		return
	}

	newEnv := EnvironmentType(currentOsEnv)
	if newEnv != oldValue {
		e.mu.Lock()
		e.Value = newEnv // 更新环境值
		e.mu.Unlock()

		e.SetEnvironment(newEnv) // 更新环境
		logger.GetGlobalLogger().DebugKV("环境变量已更新", "key", ctxKey, "value", newEnv)

		// 触发环境变更回调
		e.triggerCallbacks(oldValue, newEnv)
	}
}

// watchEnv 监控环境变量变化，支持动态调整检查频率
func (e *Environment) watchEnv() {
	e.mu.RLock()
	currentFrequency := e.CheckFrequency
	e.mu.RUnlock()

	// 使用 DynamicTicker 替代 time.Ticker，支持运行时动态调整频率
	ticker := syncx.NewDynamicTicker(currentFrequency)
	ticker.Start()
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.CheckAndUpdateEnv() // 检查并更新环境变量

			// 检查频率是否变化，变化时动态调整
			e.mu.RLock()
			newFreq := e.CheckFrequency
			e.mu.RUnlock()

			// DynamicTicker 内部会检查频率是否相同，相同则不做操作
			ticker.UpdateInterval(newFreq)
		case <-e.quit:
			logger.GetGlobalLogger().Info("手动终止取消监控环境变量")
			return // 退出监控
		}
	}
}

// SetCheckFrequency 设置检查频率并支持链式调用
func (e *Environment) SetCheckFrequency(frequency time.Duration) *Environment {
	e.mu.Lock()         // 获取写锁
	defer e.mu.Unlock() // 确保在函数结束时释放锁

	e.CheckFrequency = frequency // 更新检查频率
	return e                     // 返回当前环境实例以支持链式调用
}

// StopWatch 停止监控环境变量
func (e *Environment) StopWatch() {
	close(e.quit) // 关闭 quit 信道以停止监控
}

// GetEnvironment 从上下文中获取环境变量并解析为正确的环境类型
// 上下文键经 GetContextKey 一次性取出再复用，不直接读 envContextKey：
// SetContextKey 是加锁写的，这里裸读会与它构成数据竞争
// hotReloadManager 的事件循环在后台 goroutine 里调本函数，调用方只要在运行期
// 调过一次 SetContextKey 就会触发，-race 下已实测到
//
// [EN] The context key is read once through GetContextKey rather than touching
// envContextKey directly: SetContextKey writes it under a mutex, so an unguarded
// read here races with it
// hotReloadManager's event loop calls this from a background goroutine, so any
// runtime call to SetContextKey trips it, as observed under -race
func GetEnvironment() EnvironmentType {
	contextKey := GetContextKey()

	// 尝试获取环境变量
	currentOsEnv, err := getEnv(contextKey)
	if err != nil {
		logger.GetGlobalLogger().ErrorKV("获取环境变量失败", "key", contextKey, "error", err)
		return DefaultEnv
	}

	// 尝试检测环境类型
	if detectedEnv, found := defaultEnvManager.DetectEnvironmentType(currentOsEnv.String()); found {
		return detectedEnv
	}

	// 如果检测失败，返回原始值
	logger.GetGlobalLogger().WarnKV("环境变量无法映射到已知环境类型，使用原始值", "key", contextKey, "value", currentOsEnv)
	return currentOsEnv
}

// setEnv 设置环境变量并记录日志
func setEnv(key ContextKey, value EnvironmentType) error {
	if err := os.Setenv(key.String(), value.String()); err != nil {
		return err // 返回错误
	}
	logger.GetGlobalLogger().DebugKV("环境变量设置成功", "key", key, "value", value)
	return nil
}

// getEnv 获取环境变量
func getEnv(key ContextKey) (EnvironmentType, error) {
	value := os.Getenv(key.String())
	if value == "" {
		return "", ErrEnvVarNotSet(key.String())
	}
	return EnvironmentType(value), nil
}

// ClearEnv 清除环境变量并等待锁
// 上下文键在取 e.mu 之前先读出来：与 SetEnvironment 同一个理由，
// contextKeyMutex 嵌在 e.mu 里会造成锁顺序倒置
//
// [EN] The context key is read before acquiring e.mu, for the same reason as in
// SetEnvironment: nesting contextKeyMutex inside e.mu inverts the lock order
func (e *Environment) ClearEnv() {
	contextKey := GetContextKey()

	e.mu.Lock()         // 获取写锁
	defer e.mu.Unlock() // 确保在函数结束时释放锁

	if err := os.Unsetenv(contextKey.String()); err != nil {
		logger.GetGlobalLogger().ErrorKV("清除环境变量失败", "key", contextKey.String(), "error", err)
	} else {
		logger.GetGlobalLogger().DebugKV("环境变量已清除", "key", contextKey.String())
	}
}

// 定义一个内部接口，隐藏实现细节
type EnvironmentInterface interface {
	SetEnvironment(value EnvironmentType) *Environment
	CheckAndUpdateEnv()
	SetCheckFrequency(frequency time.Duration) *Environment
	StopWatch()
	ClearEnv()
	RegisterCallback(id string, callback EnvironmentCallback, priority int, async bool) error
	UnregisterCallback(id string) error
	ListCallbacks() []string
	ClearCallbacks()
}

// Ensure Environment 实现了 EnvironmentInterface
var _ EnvironmentInterface = (*Environment)(nil)

// Default 返回默认的 Environment 指针，支持链式调用
func Default() *Environment {
	config := DefaultEnvironment()
	return &config
}

// DefaultEnvironment 返回默认的 Environment 值
func DefaultEnvironment() Environment {
	return Environment{
		CheckFrequency: 2 * time.Second,
		Value:          DefaultEnv,
		quit:           make(chan struct{}),
		callbacks:      make(map[string]*EnvironmentCallbackInfo),
	}
}

// 全局环境实例，自动初始化
var globalEnvironment *Environment
var globalEnvOnce sync.Once

// initGlobalEnvironment 初始化全局环境实例
func initGlobalEnvironment() {
	globalEnvOnce.Do(func() {
		globalEnvironment = NewEnvironment()
		logger.GetGlobalLogger().DebugKV("全局环境实例已自动初始化", "environment", globalEnvironment.Value)
	})
}

// GetGlobalEnvironment 获取全局环境实例，如果未初始化则自动初始化
func GetGlobalEnvironment() *Environment {
	initGlobalEnvironment()
	return globalEnvironment
}

// GetCurrentEnvironment 获取当前环境类型（便捷函数）
func GetCurrentEnvironment() EnvironmentType {
	globalEnv := GetGlobalEnvironment()
	globalEnv.mu.RLock()
	defer globalEnv.mu.RUnlock()
	return globalEnv.Value
}

// SetCurrentEnvironment 设置当前环境类型（便捷函数）
func SetCurrentEnvironment(env EnvironmentType) {
	globalEnv := GetGlobalEnvironment()
	globalEnv.SetEnvironment(env)
	// 立即更新全局环境实例的值
	globalEnv.mu.Lock()
	globalEnv.Value = env
	globalEnv.mu.Unlock()
}

// 环境类型判断函数

// IsDev 判断当前环境是否为开发环境
func IsDev() bool {
	env := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(env.String(), EnvDevelopment)
}

// IsTest 判断当前环境是否为测试环境
func IsTest() bool {
	env := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(env.String(), EnvTest)
}

// IsStaging 判断当前环境是否为预发布环境
func IsStaging() bool {
	env := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(env.String(), EnvStaging)
}

// IsUAT 判断当前环境是否为用户验收测试环境
func IsUAT() bool {
	env := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(env.String(), EnvUAT)
}

// IsProduction 判断当前环境是否为生产环境
func IsProduction() bool {
	env := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(env.String(), EnvProduction)
}

// IsLocal 判断当前环境是否为本地环境
func IsLocal() bool {
	env := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(env.String(), EnvLocal)
}

// IsDebug 判断当前环境是否为调试环境
func IsDebug() bool {
	env := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(env.String(), EnvDebug)
}

// IsDemo 判断当前环境是否为演示环境
func IsDemo() bool {
	env := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(env.String(), EnvDemo)
}

// IsIntegration 判断当前环境是否为集成环境
func IsIntegration() bool {
	env := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(env.String(), EnvIntegration)
}

// IsEnvironment 通用环境判断函数
// env: 要检查的环境类型
func IsEnvironment(env EnvironmentType) bool {
	currentEnv := GetCurrentEnvironment()
	return defaultEnvManager.IsEnvironment(currentEnv.String(), env)
}

// IsAnyOf 判断当前环境是否为指定环境类型中的任意一种
// envs: 要检查的环境类型列表
func IsAnyOf(envs ...EnvironmentType) bool {
	currentEnv := GetCurrentEnvironment()
	for _, env := range envs {
		if defaultEnvManager.IsEnvironment(currentEnv.String(), env) {
			return true
		}
	}
	return false
}

// GetEnvironmentLevel 获取环境级别（用于比较环境的重要程度）
// 数字越小表示环境级别越高（生产环境最高）
func GetEnvironmentLevel(env EnvironmentType) int {
	switch env {
	case EnvLocal, EnvDebug:
		return 1
	case EnvDevelopment:
		return 2
	case EnvTest:
		return 3
	case EnvIntegration:
		return 4
	case EnvStaging, EnvUAT:
		return 5
	case EnvDemo:
		return 6
	case EnvProduction:
		return 10
	default:
		return 0
	}
}

// GetCurrentEnvironmentLevel 获取当前环境级别
func GetCurrentEnvironmentLevel() int {
	return GetEnvironmentLevel(GetCurrentEnvironment())
}

// IsProductionLevel 判断当前环境是否为生产级别
func IsProductionLevel() bool {
	return GetCurrentEnvironmentLevel() >= 10
}

// IsTestingLevel 判断当前环境是否为测试级别
func IsTestingLevel() bool {
	level := GetCurrentEnvironmentLevel()
	return level >= 3 && level < 10
}

// IsDevelopmentLevel 判断当前环境是否为开发级别
func IsDevelopmentLevel() bool {
	return GetCurrentEnvironmentLevel() <= 2
}
