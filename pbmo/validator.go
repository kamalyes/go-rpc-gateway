/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:49:58
 * @FilePath: \go-rpc-gateway\pbmo\validator.go
 * @Description: 参数校验模块
 * 职责：数据验证规则定义、执行、错误收集
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"reflect"
	"regexp"
)

// Validator 参数校验接口
type Validator interface {
	Validate() error
}

// FieldRule 字段校验规则
type FieldRule struct {
	Name      string                        // 字段名
	Required  bool                          // 是否必填
	MinLen    int                           // 最小长度
	MaxLen    int                           // 最大长度
	Min       float64                       // 最小值
	Max       float64                       // 最大值
	Pattern   string                        // 正则表达式
	Custom    func(interface{}) error       // 自定义校验函数
	Transform func(interface{}) interface{} // 转换函数
}

// ValidationError 校验错误
type ValidationError struct {
	Field   string
	Message string
}

// ValidationErrors 多个校验错误集合
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return "validation passed"
	}
	msg := "validation errors:\n"
	for _, e := range ve {
		msg += fmt.Sprintf("  - %s: %s\n", e.Field, e.Message)
	}
	return msg
}

// FieldValidator 参数校验器
type FieldValidator struct {
	rules map[string][]FieldRule
}

// NewFieldValidator 创建参数校验器
func NewFieldValidator() *FieldValidator {
	return &FieldValidator{
		rules: make(map[string][]FieldRule),
	}
}

// RegisterRules 注册字段校验规则
func (fv *FieldValidator) RegisterRules(structName string, rules ...FieldRule) {
	fv.rules[structName] = append(fv.rules[structName], rules...)
}

// Validate 校验数据
func (fv *FieldValidator) Validate(data interface{}) error {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	structName := val.Type().Name()
	rules, ok := fv.rules[structName]
	if !ok {
		return nil // 没有注册规则
	}

	var errors ValidationErrors

	for _, rule := range rules {
		field := val.FieldByName(rule.Name)
		if !field.IsValid() {
			continue
		}

		// 必填校验
		if rule.Required && isZeroValue(field) {
			errors = append(errors, ValidationError{
				Field:   rule.Name,
				Message: "required field",
			})
			continue
		}

		// 跳过零值的其他校验
		if isZeroValue(field) {
			continue
		}

		// 字符串长度校验
		if field.Kind() == reflect.String {
			str := field.String()
			if rule.MinLen > 0 && len(str) < rule.MinLen {
				errors = append(errors, ValidationError{
					Field:   rule.Name,
					Message: fmt.Sprintf("minimum length is %d", rule.MinLen),
				})
			}
			if rule.MaxLen > 0 && len(str) > rule.MaxLen {
				errors = append(errors, ValidationError{
					Field:   rule.Name,
					Message: fmt.Sprintf("maximum length is %d", rule.MaxLen),
				})
			}

			// 正则表达式校验
			if rule.Pattern != "" {
				if match, _ := regexp.MatchString(rule.Pattern, str); !match {
					errors = append(errors, ValidationError{
						Field:   rule.Name,
						Message: "format invalid",
					})
				}
			}
		}

		// 数值范围校验
		if isNumeric(field) {
			num := getNumericValue(field)
			// 检查最小值（当Min被设置时，包括0）
			if (rule.Min != 0 || rule.Max != 0) && num < rule.Min {
				errors = append(errors, ValidationError{
					Field:   rule.Name,
					Message: fmt.Sprintf("minimum value is %.2f", rule.Min),
				})
			}
			// 检查最大值
			if rule.Max > 0 && num > rule.Max {
				errors = append(errors, ValidationError{
					Field:   rule.Name,
					Message: fmt.Sprintf("maximum value is %.2f", rule.Max),
				})
			}
		}

		// 自定义校验
		if rule.Custom != nil {
			if err := rule.Custom(field.Interface()); err != nil {
				errors = append(errors, ValidationError{
					Field:   rule.Name,
					Message: err.Error(),
				})
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// ValidateWithTransform 校验并转换数据
func (fv *FieldValidator) ValidateWithTransform(data interface{}) (interface{}, error) {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	structName := val.Type().Name()
	rules, ok := fv.rules[structName]
	if !ok {
		return data, nil
	}

	// 先校验
	if err := fv.Validate(data); err != nil {
		return nil, err
	}

	// 再转换
	for _, rule := range rules {
		if rule.Transform == nil {
			continue
		}

		field := val.FieldByName(rule.Name)
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		transformed := rule.Transform(field.Interface())
		field.Set(reflect.ValueOf(transformed))
	}

	return data, nil
}
