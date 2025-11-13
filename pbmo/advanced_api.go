// Package pbmo provides advanced high-level APIs for simplified usage
package pbmo

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

// AdvancedConverter 提供高级封装的转换器
type AdvancedConverter struct {
	converter     *BidiConverter
	validationCfg *ValidationConfig
	concurrencyCfg *ConcurrencyConfig
	mu            sync.RWMutex
}

// ValidationConfig 校验配置
type ValidationConfig struct {
	Enabled      bool                     `json:"enabled"`       // 是否启用校验
	AutoDiscover bool                     `json:"autoDiscover"`  // 自动发现校验规则
	Rules        map[string][]FieldRule   `json:"rules"`         // 手动配置的规则
	TagBased     bool                     `json:"tagBased"`      // 基于tag自动校验
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

// NewAdvancedConverter 创建高级转换器
func NewAdvancedConverter(pb, model interface{}, opts ...AdvancedOption) *AdvancedConverter {
	ac := &AdvancedConverter{
		converter: NewBidiConverter(pb, model),
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
	}

	// 应用选项
	for _, opt := range opts {
		opt(ac)
	}

	// 自动发现校验规则
	if ac.validationCfg.AutoDiscover {
		ac.autoDiscoverValidationRules(model)
	}

	return ac
}

// AdvancedOption 高级选项
type AdvancedOption func(*AdvancedConverter)

// WithValidation 配置校验
func WithValidation(enabled bool, autoDiscover bool) AdvancedOption {
	return func(ac *AdvancedConverter) {
		ac.validationCfg.Enabled = enabled
		ac.validationCfg.AutoDiscover = autoDiscover
	}
}

// WithConcurrency 配置并发
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

// EasyRule 简易规则定义
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
		Min:      r.Min,
		Max:      r.Max,
		Pattern:  r.Pattern,
	}

	// 邮箱规则
	if r.Email {
		rule.Pattern = `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	}

	return rule
}

// autoDiscoverValidationRules 自动发现校验规则（基于struct tag）
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
		ac.validationCfg.Rules[typeName] = rules
		// 注册到底层转换器
		ac.converter.RegisterValidationRules(typeName, rules...)
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
		case strings.HasPrefix(tag, "max="):
			// 处理 max 规则
		case tag == "email":
			rule.Pattern = `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
		}
	}

	return rule
}

// SuperEasyBatchConvert 超级简易的批量转换 - 傻瓜式使用
func SuperEasyBatchConvert[TPB, TModel any](
	pbSlice []TPB,
	opts ...SuperEasyOption,
) *ConversionResult[TModel] {
	start := time.Now()
	
	// 默认配置
	config := &SuperEasyConfig{
		MaxGoroutines: runtime.NumCPU(),
		BatchSize:     100,
		Timeout:       30 * time.Second,
		Validation:    true,
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
	)

	// 执行转换
	result := converter.BatchConvertWithConcurrency(pbSlice)
	result.Elapsed = time.Since(start)

	return result
}

// SuperEasyConfig 超级简易配置
type SuperEasyConfig struct {
	MaxGoroutines int
	BatchSize     int
	Timeout       time.Duration
	Validation    bool
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

// BatchConvertWithConcurrency 带并发的批量转换
func (ac *AdvancedConverter) BatchConvertWithConcurrency(pbSlice interface{}) *ConversionResult[interface{}] {
	ctx, cancel := context.WithTimeout(context.Background(), ac.concurrencyCfg.Timeout)
	defer cancel()

	// 转换为 reflect.Value 处理
	pbValue := reflect.ValueOf(pbSlice)
	if pbValue.Kind() != reflect.Slice {
		return &ConversionResult[interface{}]{
			Errors: []error{fmt.Errorf("输入必须是切片类型")},
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
	data   []interface{}
	err    error
	start  int
	count  int
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
	batchInterface := batchSlice.Interface()

	// 执行转换
	var results []interface{}
	err := ac.converter.BatchConvertPBToModel(batchInterface, &results)

	resultCh <- batchResult{
		data:  results,
		err:   err,
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
		"validation_enabled":   ac.validationCfg.Enabled,
		"validation_rules":     len(ac.validationCfg.Rules),
		"max_goroutines":       ac.concurrencyCfg.MaxGoroutines,
		"batch_size":          ac.concurrencyCfg.BatchSize,
		"timeout_seconds":     ac.concurrencyCfg.Timeout.Seconds(),
	}
}