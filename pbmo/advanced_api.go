// Package pbmo provides advanced high-level APIs for simplified usage
package pbmo

import (
	"context"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-toolbox/pkg/desensitize"
)

// AdvancedOption 高级转换器选项函数类型
type AdvancedOption func(*AdvancedConverter)

// PerformanceLevel 转换器性能级别
type PerformanceLevel int

const (
	BasicLevel     PerformanceLevel = iota // BidiConverter (基线)
	OptimizedLevel                         // OptimizedBidiConverter (推荐生产)
	UltraFastLevel                         // UltraFastConverter (超高性能)
)

// AdvancedConverter 提供高级封装的转换器
// 支持多种性能级别的转换器集成
type AdvancedConverter struct {
	performanceLevel   PerformanceLevel
	basicConverter     *BidiConverter          // 基础转换器
	optimizedConverter *OptimizedBidiConverter // 优化转换器
	ultraFastConverter *UltraFastConverter     // 超高性能转换器
	validationCfg      *ValidationConfig
	concurrencyCfg     *ConcurrencyConfig
	desensitizationCfg *DesensitizationConfig
	mu                 sync.RWMutex
}

// DesensitizationConfig 脱敏配置
type DesensitizationConfig struct {
	Enabled       bool                                `json:"enabled"`       // 是否启用脱敏
	AutoDiscover  bool                                `json:"autoDiscover"`  // 自动发现脱敏规则
	Rules         map[string][]DesensitizeRule        `json:"rules"`         // 手动配置的规则
	TagBased      bool                                `json:"tagBased"`      // 基于tag自动脱敏
	Desensitizers map[string]desensitize.Desensitizer `json:"desensitizers"` // 注册的脱敏器
	TypeMapping   map[string]string                   `json:"typeMapping"`   // 标签到脱敏类型的映射
	CustomParsers map[string]CustomRuleParser         `json:"-"`             // 自定义规则解析器
}

// CustomRuleParser 自定义规则解析器
type CustomRuleParser func(tag string, rule *DesensitizeRule) error

// DesensitizeRule 脱敏规则
type DesensitizeRule struct {
	Field      string              `json:"field"`      // 字段名
	Type       string              `json:"type"`       // 脱敏类型
	StartIndex int                 `json:"startIndex"` // 开始索引
	EndIndex   int                 `json:"endIndex"`   // 结束索引
	MaskChar   string              `json:"maskChar"`   // 掩码字符
	CustomFunc func(string) string `json:"-"`          // 自定义脱敏函数
	Config     map[string]string   `json:"config"`     // 额外配置参数
}

// ValidationConfig 校验配置
type ValidationConfig struct {
	Enabled      bool                   `json:"enabled"`      // 是否启用校验
	AutoDiscover bool                   `json:"autoDiscover"` // 自动发现校验规则
	Rules        map[string][]FieldRule `json:"rules"`        // 手动配置的规则
	TagBased     bool                   `json:"tagBased"`     // 基于tag自动校验
}

// ConcurrencyConfig 并发配置
type ConcurrencyConfig struct {
	MaxGoroutines int           `json:"maxGoroutines"` // 最大协程数
	BatchSize     int           `json:"batchSize"`     // 批处理大小
	Timeout       time.Duration `json:"timeout"`       // 超时时间
}

// ConversionResult 转换结果
type ConversionResult[T any] struct {
	Data    []T           `json:"data"`
	Errors  []error       `json:"errors"`
	Success int           `json:"success"`
	Failed  int           `json:"failed"`
	Elapsed time.Duration `json:"elapsed"`
}

// ValidationRule 校验规则标签
type ValidationRule struct {
	Required bool   `json:"required"`
	MinLen   int    `json:"minLen"`
	MaxLen   int    `json:"maxLen"`
	Pattern  string `json:"pattern"`
	Min      int64  `json:"min"`
	Max      int64  `json:"max"`
}

// NewAdvancedConverter 创建高级转换器（基于不同性能级别的转换器）
func NewAdvancedConverter(pb, model interface{}, opts ...AdvancedOption) *AdvancedConverter {
	ac := &AdvancedConverter{
		performanceLevel: BasicLevel, // 默认使用基础转换器
		validationCfg: &ValidationConfig{
			Enabled:      true,
			AutoDiscover: true,
			Rules:        make(map[string][]FieldRule),
			TagBased:     true,
		},
		concurrencyCfg: &ConcurrencyConfig{
			MaxGoroutines: runtime.NumCPU(),
			BatchSize:     100,
			Timeout:       30 * time.Second,
		},
		desensitizationCfg: &DesensitizationConfig{
			Enabled:       false,
			AutoDiscover:  true,
			Rules:         make(map[string][]DesensitizeRule),
			TagBased:      true,
			Desensitizers: make(map[string]desensitize.Desensitizer),
			TypeMapping:   getDefaultTypeMapping(),
			CustomParsers: make(map[string]CustomRuleParser),
		},
	}

	// 初始化所有转换器
	ac.basicConverter = NewBidiConverter(pb, model)
	ac.optimizedConverter = NewOptimizedBidiConverter(pb, model)
	ac.ultraFastConverter = NewUltraFastConverter(pb, model)

	// 应用选项
	for _, opt := range opts {
		opt(ac)
	}

	// 自动发现校验规则（应用到当前选定的转换器）
	if ac.validationCfg.AutoDiscover {
		ac.autoDiscoverValidationRules(model)
		// 将规则注册到所有转换器
		for typeName, rules := range ac.validationCfg.Rules {
			if len(rules) > 0 {
				ac.basicConverter.RegisterValidationRules(typeName, rules...)
				// 优化转换器和超高性能转换器可能有不同的校验机制
				// 这里可以根据需要进行适配
			}
		}
	}

	// 自动发现脱敏规则（检查PB和Model类型）
	if ac.desensitizationCfg.AutoDiscover {
		ac.autoDiscoverDesensitizationRules(model) // 检查Model类型
		ac.autoDiscoverDesensitizationRules(pb)    // 检查PB类型
	}

	return ac
}

// WithPerformanceLevel 设置性能级别
func WithPerformanceLevel(level PerformanceLevel) AdvancedOption {
	return func(ac *AdvancedConverter) {
		ac.performanceLevel = level
	}
}

// ConvertPBToModel 统一的PB到Model转换接口
func (ac *AdvancedConverter) ConvertPBToModel(pb interface{}, model interface{}) error {
	switch ac.performanceLevel {
	case OptimizedLevel:
		return ac.optimizedConverter.ConvertPBToModel(pb, model)
	case UltraFastLevel:
		return ac.ultraFastConverter.ConvertPBToModel(pb, model)
	default:
		return ac.basicConverter.ConvertPBToModel(pb, model)
	}
}

// getModelType 获取模型类型
func (ac *AdvancedConverter) getModelType() reflect.Type {
	switch ac.performanceLevel {
	case OptimizedLevel:
		return ac.optimizedConverter.modelType
	case UltraFastLevel:
		return ac.ultraFastConverter.modelType
	default:
		return ac.basicConverter.modelType
	}
}

// WithValidation 配置校验
func WithValidation(enabled bool, autoDiscover bool) AdvancedOption {
	return func(ac *AdvancedConverter) {
		ac.validationCfg.Enabled = enabled
		ac.validationCfg.AutoDiscover = autoDiscover
	}
}

// WithDesensitization 配置脱敏
func WithDesensitization(enabled bool, autoDiscover bool) AdvancedOption {
	return func(ac *AdvancedConverter) {
		ac.desensitizationCfg.Enabled = enabled
		ac.desensitizationCfg.AutoDiscover = autoDiscover
	}
}

// WithEasyDesensitization 简易脱敏配置 - 傻瓜式使用
func WithEasyDesensitization(modelType string, rules ...EasyDesensitizeRule) AdvancedOption {
	return func(ac *AdvancedConverter) {
		desensitizeRules := make([]DesensitizeRule, 0, len(rules))
		for _, rule := range rules {
			desensitizeRules = append(desensitizeRules, rule.toDesensitizeRule())
		}
		ac.desensitizationCfg.Rules[modelType] = desensitizeRules
	}
}
func WithConcurrency(maxGoroutines, batchSize int, timeout time.Duration) AdvancedOption {
	return func(ac *AdvancedConverter) {
		ac.concurrencyCfg.MaxGoroutines = maxGoroutines
		ac.concurrencyCfg.BatchSize = batchSize
		ac.concurrencyCfg.Timeout = timeout
	}
}

// WithEasyValidation 简易校验配置 - 傻瓜式使用
func WithEasyValidation(modelType string, rules ...EasyRule) AdvancedOption {
	return func(ac *AdvancedConverter) {
		fieldRules := make([]FieldRule, 0, len(rules))
		for _, rule := range rules {
			fieldRules = append(fieldRules, rule.toFieldRule())
		}
		ac.validationCfg.Rules[modelType] = fieldRules
	}
}

// EasyDesensitizeRule 简易脱敏规则定义
type EasyDesensitizeRule struct {
	Field      string
	Type       string // "email", "phone", "name", "idCard", "address", "bankCard" 等
	StartIndex int
	EndIndex   int
	MaskChar   string
	Custom     func(string) string // 自定义脱敏函数
}

func (r EasyDesensitizeRule) toDesensitizeRule() DesensitizeRule {
	rule := DesensitizeRule{
		Field:      r.Field,
		Type:       r.Type,
		StartIndex: r.StartIndex,
		EndIndex:   r.EndIndex,
		MaskChar:   r.MaskChar,
		CustomFunc: r.Custom,
	}

	// 设置默认掩码字符
	if rule.MaskChar == "" {
		rule.MaskChar = "*"
	}

	return rule
}

type EasyRule struct {
	Field    string
	Required bool
	MinLen   int
	MaxLen   int
	Email    bool
	Pattern  string
	Min      int64
	Max      int64
}

func (r EasyRule) toFieldRule() FieldRule {
	rule := FieldRule{
		Name:     r.Field,
		Required: r.Required,
		MinLen:   r.MinLen,
		MaxLen:   r.MaxLen,
		Min:      float64(r.Min),
		Max:      float64(r.Max),
		Pattern:  r.Pattern,
	}

	// 邮箱规则
	if r.Email {
		rule.Pattern = `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	}

	return rule
}

// autoDiscoverDesensitizationRules 自动发现脱敏规则（基于struct tag）
func (ac *AdvancedConverter) autoDiscoverDesensitizationRules(model interface{}) {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	if modelType.Kind() != reflect.Struct {
		return
	}

	typeName := modelType.Name()
	var rules []DesensitizeRule

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		// 跳过非导出字段
		if !field.IsExported() {
			continue
		}

		rule := ac.parseDesensitizationTag(field)
		if rule != nil {
			rule.Field = field.Name
			rules = append(rules, *rule)
		}
	}

	if len(rules) > 0 {
		ac.desensitizationCfg.Rules[typeName] = rules
	}
}

// parseDesensitizationTag 解析脱敏标签
func (ac *AdvancedConverter) parseDesensitizationTag(field reflect.StructField) *DesensitizeRule {
	desensitizeTag := field.Tag.Get("desensitize")
	if desensitizeTag == "" {
		return nil
	}

	rule := &DesensitizeRule{
		MaskChar: "*", // 默认掩码字符
	}

	tags := strings.Split(desensitizeTag, ",")
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)

		// 检查预定义类型映射
		if mappedType, exists := ac.desensitizationCfg.TypeMapping[tag]; exists {
			rule.Type = mappedType
			continue
		}

		// 处理自定义规则
		if strings.HasPrefix(tag, "custom:") {
			rule.Type = "custom"
			// 提取自定义规则参数
			customRule := strings.TrimPrefix(tag, "custom:")
			if err := ac.parseCustomRule(customRule, rule); err != nil {
				// 记录错误但不中断处理
				continue
			}
			continue
		}

		// 检查是否有自定义解析器
		for prefix, parser := range ac.desensitizationCfg.CustomParsers {
			if strings.HasPrefix(tag, prefix+":") {
				if err := parser(tag, rule); err == nil {
					break
				}
			}
		}
	}

	if rule.Type == "" {
		return nil
	}

	return rule
}
func (ac *AdvancedConverter) autoDiscoverValidationRules(model interface{}) {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	if modelType.Kind() != reflect.Struct {
		return
	}

	typeName := modelType.Name()
	var rules []FieldRule

	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		// 跳过非导出字段
		if !field.IsExported() {
			continue
		}

		rule := ac.parseValidationTag(field)
		if rule != nil {
			rule.Name = field.Name
			rules = append(rules, *rule)
		}
	}

	if len(rules) > 0 {
		ac.validationCfg.Rules[typeName] = append(ac.validationCfg.Rules[typeName], rules...)
	}
}

// parseValidationTag 解析校验标签
func (ac *AdvancedConverter) parseValidationTag(field reflect.StructField) *FieldRule {
	validate := field.Tag.Get("validate")
	if validate == "" {
		return nil
	}

	rule := &FieldRule{}
	tags := strings.Split(validate, ",")

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)

		switch {
		case tag == "required":
			rule.Required = true
		case strings.HasPrefix(tag, "min="):
			// 处理 min 规则
			if minStr := strings.TrimPrefix(tag, "min="); minStr != "" {
				if minVal := parseIntValue(minStr); minVal >= 0 {
					rule.MinLen = minVal
				}
			}
		case strings.HasPrefix(tag, "max="):
			// 处理 max 规则
			if maxStr := strings.TrimPrefix(tag, "max="); maxStr != "" {
				if maxVal := parseIntValue(maxStr); maxVal >= 0 {
					rule.MaxLen = maxVal
				}
			}
		case tag == "email":
			rule.Pattern = `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
		}
	}

	return rule
}

// parseIntValue 解析整数值
func parseIntValue(s string) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return -1
}

// SuperEasyBatchConvert 超级简易的批量转换 - 傻瓜式使用
func SuperEasyBatchConvert[TPB, TModel any](
	pbSlice []TPB,
	opts ...SuperEasyOption,
) *ConversionResult[TModel] {
	start := time.Now()

	// 默认配置
	config := &SuperEasyConfig{
		MaxGoroutines:   runtime.NumCPU(),
		BatchSize:       100,
		Timeout:         30 * time.Second,
		Validation:      true,
		Desensitization: false, // 默认禁用脱敏
	}

	// 应用选项
	for _, opt := range opts {
		opt(config)
	}

	// 创建高级转换器
	var pbExample TPB
	var modelExample TModel

	converter := NewAdvancedConverter(&pbExample, &modelExample,
		WithConcurrency(config.MaxGoroutines, config.BatchSize, config.Timeout),
		WithValidation(config.Validation, true),
		WithDesensitization(config.Desensitization, true),
	)

	// 执行转换
	untypedResult := converter.BatchConvertWithConcurrency(pbSlice)

	// 类型安全转换
	typedResult := &ConversionResult[TModel]{
		Data:    make([]TModel, 0, len(untypedResult.Data)),
		Errors:  untypedResult.Errors,
		Success: untypedResult.Success,
		Failed:  untypedResult.Failed,
		Elapsed: time.Since(start),
	}

	// 安全类型转换
	for _, item := range untypedResult.Data {
		if typedItem, ok := item.(TModel); ok {
			typedResult.Data = append(typedResult.Data, typedItem)
		}
	}

	return typedResult
}

// SuperEasyConfig 超级简易配置
type SuperEasyConfig struct {
	MaxGoroutines   int
	BatchSize       int
	Timeout         time.Duration
	Validation      bool
	Desensitization bool
}

// SuperEasyOption 超级简易选项
type SuperEasyOption func(*SuperEasyConfig)

// FastMode 快速模式（更大的批次，更多协程）
func FastMode() SuperEasyOption {
	return func(c *SuperEasyConfig) {
		c.MaxGoroutines = runtime.NumCPU() * 2
		c.BatchSize = 500
		c.Validation = false
	}
}

// WithDesensitizationMode 启用脱敏模式
func WithDesensitizationMode() SuperEasyOption {
	return func(c *SuperEasyConfig) {
		c.Desensitization = true
	}
}

// SecureMode 安全模式（启用校验和脱敏）
func SecureMode() SuperEasyOption {
	return func(c *SuperEasyConfig) {
		c.MaxGoroutines = runtime.NumCPU()
		c.BatchSize = 100
		c.Validation = true
		c.Desensitization = true
	}
}

// NoDesensitization 禁用脱敏
func NoDesensitization() SuperEasyOption {
	return func(c *SuperEasyConfig) {
		c.Desensitization = false
	}
}

// SafeMode 安全模式（较小批次，带校验）
func SafeMode() SuperEasyOption {
	return func(c *SuperEasyConfig) {
		c.MaxGoroutines = runtime.NumCPU() / 2
		c.BatchSize = 50
		c.Validation = true
	}
}

// WithTimeout 设置超时
func WithTimeout(timeout time.Duration) SuperEasyOption {
	return func(c *SuperEasyConfig) {
		c.Timeout = timeout
	}
}

// NoValidation 禁用校验
func NoValidation() SuperEasyOption {
	return func(c *SuperEasyConfig) {
		c.Validation = false
	}
}

// ConvertWithDesensitization 带脱敏的转换
func (ac *AdvancedConverter) ConvertWithDesensitization(pb interface{}, model interface{}) error {
	// 使用统一的转换接口
	err := ac.ConvertPBToModel(pb, model)
	if err != nil {
		return err
	}

	// 如果启用脱敏，则应用脱敏规则
	if ac.desensitizationCfg.Enabled {
		return desensitize.Desensitization(model)
	}

	return nil
}

// BatchConvertWithConcurrency 带并发的批量转换
func (ac *AdvancedConverter) BatchConvertWithConcurrency(pbSlice interface{}) *ConversionResult[interface{}] {
	ctx, cancel := context.WithTimeout(context.Background(), ac.concurrencyCfg.Timeout)
	defer cancel()

	// 转换为 reflect.Value 处理
	pbValue := reflect.ValueOf(pbSlice)
	if pbValue.Kind() != reflect.Slice {
		return &ConversionResult[interface{}]{
			Errors: []error{errors.ErrMustBeSlice},
			Failed: 1,
		}
	}

	total := pbValue.Len()
	if total == 0 {
		return &ConversionResult[interface{}]{Success: 0}
	}

	batchSize := ac.concurrencyCfg.BatchSize
	maxGoroutines := ac.concurrencyCfg.MaxGoroutines

	// 计算批次数量
	numBatches := (total + batchSize - 1) / batchSize

	// 结果通道
	resultCh := make(chan batchResult, numBatches)
	semaphore := make(chan struct{}, maxGoroutines)

	// 启动协程处理每个批次
	var wg sync.WaitGroup
	for i := 0; i < numBatches; i++ {
		wg.Add(1)
		go ac.processBatch(ctx, pbValue, i, batchSize, semaphore, resultCh, &wg)
	}

	// 等待所有协程完成
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集结果
	return ac.collectResults(resultCh, total)
}

// batchResult 批次结果
type batchResult struct {
	data  []interface{}
	err   error
	start int
	count int
}

// processBatch 处理单个批次
func (ac *AdvancedConverter) processBatch(
	ctx context.Context,
	pbValue reflect.Value,
	batchIndex int,
	batchSize int,
	semaphore chan struct{},
	resultCh chan<- batchResult,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	// 获取信号量
	select {
	case semaphore <- struct{}{}:
		defer func() { <-semaphore }()
	case <-ctx.Done():
		resultCh <- batchResult{err: ctx.Err()}
		return
	}

	start := batchIndex * batchSize
	end := start + batchSize
	total := pbValue.Len()
	if end > total {
		end = total
	}

	// 提取批次数据
	batchSlice := pbValue.Slice(start, end)

	// 获取源类型和目标类型
	if batchSlice.Len() == 0 {
		resultCh <- batchResult{
			data:  []interface{}{},
			start: start,
			count: 0,
		}
		return
	}

	// 使用 AdvancedConverter 的内部转换器进行单个转换
	var results []interface{}
	for i := 0; i < batchSlice.Len(); i++ {
		pbItem := batchSlice.Index(i).Interface()

		// 创建目标类型的新实例
		modelType := ac.getModelType()
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}
		modelPtr := reflect.New(modelType)

		// 执行转换
		err := ac.ConvertPBToModel(pbItem, modelPtr.Interface())
		if err != nil {
			resultCh <- batchResult{
				err:   errors.NewErrorf(errors.ErrCodeElementConversion, "element %d: %v", i, err),
				start: start,
				count: batchSlice.Len(),
			}
			return
		}

		// 获取转换后的值
		convertedValue := modelPtr.Elem().Interface()

		// 如果启用脱敏，对结果应用脱敏
		if ac.desensitizationCfg.Enabled {
			if desensitizeErr := desensitize.Desensitization(convertedValue); desensitizeErr != nil {
				// 记录脱敏错误但不中断处理
			}
		}

		results = append(results, convertedValue)
	}

	resultCh <- batchResult{
		data:  results,
		err:   nil,
		start: start,
		count: end - start,
	}
}

// collectResults 收集结果
func (ac *AdvancedConverter) collectResults(
	resultCh <-chan batchResult,
	total int,
) *ConversionResult[interface{}] {
	result := &ConversionResult[interface{}]{
		Data:   make([]interface{}, 0, total),
		Errors: make([]error, 0),
	}

	for batchRes := range resultCh {
		if batchRes.err != nil {
			result.Errors = append(result.Errors, batchRes.err)
			result.Failed += batchRes.count
		} else {
			result.Data = append(result.Data, batchRes.data...)
			result.Success += batchRes.count
		}
	}

	return result
}

// TemporaryDisableValidation 临时禁用校验
func (ac *AdvancedConverter) TemporaryDisableValidation() func() {
	ac.mu.Lock()
	originalState := ac.validationCfg.Enabled
	ac.validationCfg.Enabled = false
	ac.mu.Unlock()

	// 返回恢复函数
	return func() {
		ac.mu.Lock()
		ac.validationCfg.Enabled = originalState
		ac.mu.Unlock()
	}
}

// IsValidationEnabled 检查校验是否启用
func (ac *AdvancedConverter) IsValidationEnabled() bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.validationCfg.Enabled
}

// GetValidationRules 获取校验规则
func (ac *AdvancedConverter) GetValidationRules(typeName string) []FieldRule {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.validationCfg.Rules[typeName]
}

// TemporaryDisableDesensitization 临时禁用脱敏
func (ac *AdvancedConverter) TemporaryDisableDesensitization() func() {
	ac.mu.Lock()
	originalState := ac.desensitizationCfg.Enabled
	ac.desensitizationCfg.Enabled = false
	ac.mu.Unlock()

	// 返回恢复函数
	return func() {
		ac.mu.Lock()
		ac.desensitizationCfg.Enabled = originalState
		ac.mu.Unlock()
	}
}

// IsDesensitizationEnabled 检查脱敏是否启用
func (ac *AdvancedConverter) IsDesensitizationEnabled() bool {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.desensitizationCfg.Enabled
}

// GetDesensitizationRules 获取脱敏规则
func (ac *AdvancedConverter) GetDesensitizationRules(typeName string) []DesensitizeRule {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.desensitizationCfg.Rules[typeName]
}

// RegisterDesensitizer 注册自定义脱敏器
func (ac *AdvancedConverter) RegisterDesensitizer(fieldName string, desensitizer desensitize.Desensitizer) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.desensitizationCfg.Desensitizers[fieldName] = desensitizer
}

// UpdateConcurrencyConfig 更新并发配置
func (ac *AdvancedConverter) UpdateConcurrencyConfig(maxGoroutines, batchSize int, timeout time.Duration) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	ac.concurrencyCfg.MaxGoroutines = maxGoroutines
	ac.concurrencyCfg.BatchSize = batchSize
	ac.concurrencyCfg.Timeout = timeout
}

// GetStats 获取转换器统计信息
func (ac *AdvancedConverter) GetStats() map[string]interface{} {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	return map[string]interface{}{
		"performance_level":           ac.performanceLevel,
		"validation_enabled":          ac.validationCfg.Enabled,
		"validation_rules_count":      len(ac.validationCfg.Rules),
		"desensitization_enabled":     ac.desensitizationCfg.Enabled,
		"desensitization_rules_count": len(ac.desensitizationCfg.Rules),
		"desensitizers_count":         len(ac.desensitizationCfg.Desensitizers),
		"type_mappings_count":         len(ac.desensitizationCfg.TypeMapping),
		"custom_parsers_count":        len(ac.desensitizationCfg.CustomParsers),
		"max_goroutines":              ac.concurrencyCfg.MaxGoroutines,
		"batch_size":                  ac.concurrencyCfg.BatchSize,
		"timeout_seconds":             ac.concurrencyCfg.Timeout.Seconds(),
	}
}

// getDefaultTypeMapping 获取默认的类型映射
func getDefaultTypeMapping() map[string]string {
	return map[string]string{
		"email":    "email",
		"phone":    "phoneNumber",
		"name":     "name",
		"idCard":   "identityCard",
		"address":  "address",
		"bankCard": "bankCard",
		"password": "password",
	}
}

// RegisterDesensitizationType 注册新的脱敏类型
func (ac *AdvancedConverter) RegisterDesensitizationType(tag, desensitizeType string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.desensitizationCfg.TypeMapping[tag] = desensitizeType
}

// RegisterCustomParser 注册自定义规则解析器
func (ac *AdvancedConverter) RegisterCustomParser(prefix string, parser CustomRuleParser) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.desensitizationCfg.CustomParsers[prefix] = parser
}

// parseCustomRule 解析自定义脱敏规则
func (ac *AdvancedConverter) parseCustomRule(customRule string, rule *DesensitizeRule) error {
	// 解析如: mask(1,3,*) 或 func(myCustomFunc)
	if strings.HasPrefix(customRule, "mask(") && strings.HasSuffix(customRule, ")") {
		// 解析 mask(start,end,char) 格式
		params := strings.TrimSuffix(strings.TrimPrefix(customRule, "mask("), ")")
		parts := strings.Split(params, ",")

		if len(parts) >= 3 {
			if start, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
				rule.StartIndex = start
			}
			if end, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				rule.EndIndex = end
			}
			rule.MaskChar = strings.TrimSpace(parts[2])
		}
		return nil
	}

	if strings.HasPrefix(customRule, "func(") && strings.HasSuffix(customRule, ")") {
		// 解析 func(functionName) 格式
		funcName := strings.TrimSuffix(strings.TrimPrefix(customRule, "func("), ")")
		rule.Config = map[string]string{"function": funcName}
		return nil
	}

	// 其他格式的解析...
	return errors.NewErrorf(errors.ErrCodeInvalidParameter, "unsupported custom rule format: %s", customRule)
}

// GetDesensitizationTypeMapping 获取脱敏类型映射
func (ac *AdvancedConverter) GetDesensitizationTypeMapping() map[string]string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	// 返回副本避免并发修改
	result := make(map[string]string)
	for k, v := range ac.desensitizationCfg.TypeMapping {
		result[k] = v
	}
	return result
}

// NewBasicAdvancedConverter 创建基础性能级别的高级转换器
func NewBasicAdvancedConverter(pb, model interface{}, opts ...AdvancedOption) *AdvancedConverter {
	opts = append([]AdvancedOption{WithPerformanceLevel(BasicLevel)}, opts...)
	return NewAdvancedConverter(pb, model, opts...)
}

// NewOptimizedAdvancedConverter 创建优化性能级别的高级转换器
func NewOptimizedAdvancedConverter(pb, model interface{}, opts ...AdvancedOption) *AdvancedConverter {
	opts = append([]AdvancedOption{WithPerformanceLevel(OptimizedLevel)}, opts...)
	return NewAdvancedConverter(pb, model, opts...)
}

// NewUltraFastAdvancedConverter 创建超高性能级别的高级转换器
func NewUltraFastAdvancedConverter(pb, model interface{}, opts ...AdvancedOption) *AdvancedConverter {
	opts = append([]AdvancedOption{WithPerformanceLevel(UltraFastLevel)}, opts...)
	return NewAdvancedConverter(pb, model, opts...)
}

// GetPerformanceInfo 获取性能级别信息
func (ac *AdvancedConverter) GetPerformanceInfo() map[string]interface{} {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	levelName := "Basic"
	switch ac.performanceLevel {
	case OptimizedLevel:
		levelName = "Optimized"
	case UltraFastLevel:
		levelName = "UltraFast"
	}

	return map[string]interface{}{
		"level":       ac.performanceLevel,
		"level_name":  levelName,
		"description": getPerformanceLevelDescription(ac.performanceLevel),
	}
}

// getPerformanceLevelDescription 获取性能级别描述
func getPerformanceLevelDescription(level PerformanceLevel) string {
	switch level {
	case BasicLevel:
		return "BidiConverter (基线) - 基础反射转换，功能完整"
	case OptimizedLevel:
		return "OptimizedBidiConverter (推荐生产) - 16x性能提升，推荐生产使用"
	case UltraFastLevel:
		return "UltraFastConverter (超高性能) - 超高性能转换，极速处理"
	default:
		return "Unknown performance level"
	}
}
