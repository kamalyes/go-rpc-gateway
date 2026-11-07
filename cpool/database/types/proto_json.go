/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-09 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-11 12:57:50
 * @FilePath: \go-rpc-gateway\cpool\database\types\proto_json.go
 * @Description: ProtoJSON - 泛型 protobuf JSON 类型，支持 protobuf 消息的数据库存储
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	validator "github.com/kamalyes/go-toolbox/pkg/argus"
	"github.com/kamalyes/go-toolbox/pkg/protox"
	tbtypes "github.com/kamalyes/go-toolbox/pkg/types"
)

const (
	errFieldFormat = "field %s: %w"
	errScanFormat  = "ProtoJSON scan: %w"
)

// ProtoToMap 将 protobuf 消息转换为 MapAny
func ProtoToMap(message proto.Message) MapAny {
	if message == nil {
		return nil
	}
	if timestamp, ok := message.(*timestamppb.Timestamp); ok {
		return MapAny{
			"seconds": timestamp.GetSeconds(),
			"nanos":   timestamp.GetNanos(),
		}
	}
	data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(message)
	if err != nil {
		return MapAny{"marshal_error": err.Error()}
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return MapAny{"marshal_error": err.Error()}
	}
	return MapAny(payload)
}

// ProtoJSON 泛型 protobuf JSON 类型，支持 protobuf 消息的数据库存储
type ProtoJSON[T any] struct {
	Data T
}

// Scan - 从数据库值扫描 protobuf JSON 类型
func (p *ProtoJSON[T]) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, err := toBytes(value)
	if err != nil {
		return fmt.Errorf(errScanFormat, err)
	}
	if len(b) == 0 || string(b) == "null" {
		return nil
	}

	v := reflect.ValueOf(&p.Data).Elem()
	return p.scanValue(b, v)
}

// Value - 将 protobuf JSON 类型转换为数据库值
func (p *ProtoJSON[T]) scanValue(b []byte, v reflect.Value) error {
	if v.Kind() == reflect.Ptr && v.Type().Implements(protox.ProtoMessageType) {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return protojson.Unmarshal(b, v.Interface().(proto.Message))
	}

	if v.Kind() == reflect.Struct {
		return p.scanStruct(b, v)
	}

	return fmt.Errorf("ProtoJSON: unsupported type %T", p.Data)
}

// scanStruct - 递归扫描结构体字段
func (p *ProtoJSON[T]) scanStruct(b []byte, v reflect.Value) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	tbtypes.EnsureStructDefaults(v)
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		jsonKey := tbtypes.ExtractJSONKey(fieldType)
		if jsonKey == "" {
			continue
		}

		rawVal, ok := raw[jsonKey]
		if !ok || len(rawVal) == 0 || string(rawVal) == "null" {
			continue
		}

		if err := p.scanField(field, rawVal, jsonKey); err != nil {
			return err
		}
	}

	return nil
}

// scanField - 递归扫描结构体字段
func (p *ProtoJSON[T]) scanField(field reflect.Value, rawVal json.RawMessage, jsonKey string) error {
	switch {
	case field.Kind() == reflect.Ptr && field.Type().Implements(protox.ProtoMessageType):
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		if err := protojson.Unmarshal(rawVal, field.Interface().(proto.Message)); err != nil {
			return fmt.Errorf(errFieldFormat, jsonKey, err)
		}

	case field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct:
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return p.scanStruct(rawVal, field.Elem())

	case field.Kind() == reflect.Struct:
		return p.scanStruct(rawVal, field)
	}

	return nil
}

// Value - 将 protobuf JSON 类型转换为数据库值
func (p *ProtoJSON[T]) Value() (driver.Value, error) {
	v := reflect.ValueOf(&p.Data).Elem()
	return p.valueOf(v)
}

// valueOf - 递归转换结构体字段为 JSON 字符串
func (p *ProtoJSON[T]) valueOf(v reflect.Value) (driver.Value, error) {
	if v.Kind() == reflect.Ptr && v.Type().Implements(protox.ProtoMessageType) {
		if validator.IsEmptyValue(v) {
			return nil, nil
		}
		b, err := protojson.Marshal(v.Interface().(proto.Message))
		if err != nil {
			return nil, err
		}
		if string(b) == "{}" {
			return nil, nil
		}
		return string(b), nil
	}

	if v.Kind() == reflect.Struct {
		return p.valueStruct(v, true)
	}

	return nil, fmt.Errorf("ProtoJSON: unsupported type %T", p.Data)
}

// valueStruct - 递归转换结构体字段为 JSON 字符串
func (p *ProtoJSON[T]) valueStruct(v reflect.Value, emitEmpty bool) (driver.Value, error) {
	tbtypes.EnsureStructDefaults(v)
	t := v.Type()
	result := make(map[string]json.RawMessage)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		jsonKey := tbtypes.ExtractJSONKey(fieldType)
		if jsonKey == "" {
			continue
		}

		fieldVal, err := p.valueField(field, jsonKey)
		if err != nil {
			return nil, err
		}
		if fieldVal != nil {
			result[jsonKey] = fieldVal
		}
	}

	if len(result) == 0 {
		if emitEmpty {
			return "{}", nil
		}
		return nil, nil
	}

	b, _ := json.Marshal(result)
	return string(b), nil
}

// valueField - 递归转换结构体字段为 JSON 字符串
func (p *ProtoJSON[T]) valueField(field reflect.Value, jsonKey string) (json.RawMessage, error) {
	switch {
	case field.Kind() == reflect.Ptr && field.Type().Implements(protox.ProtoMessageType):
		if validator.IsEmptyValue(field) {
			return nil, nil
		}
		b, err := protojson.Marshal(field.Interface().(proto.Message))
		if err != nil {
			return nil, fmt.Errorf(errFieldFormat, jsonKey, err)
		}
		if string(b) != "{}" {
			return json.RawMessage(b), nil
		}

	case field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct:
		if field.IsNil() {
			return nil, nil
		}
		val, err := p.valueStruct(field.Elem(), false)
		if err != nil {
			return nil, err
		}
		if val != nil {
			return json.RawMessage(val.(string)), nil
		}

	case field.Kind() == reflect.Struct:
		val, err := p.valueStruct(field, false)
		if err != nil {
			return nil, err
		}
		if val != nil {
			return json.RawMessage(val.(string)), nil
		}
	}

	return nil, nil
}

// Get - 获取结构体字段值
func (p *ProtoJSON[T]) Get() T {
	return p.Data
}

// Set - 设置 protobuf JSON 类型值
func (p *ProtoJSON[T]) Set(data T) {
	p.Data = data
}

// IsEmpty 判断是否为空
func (p *ProtoJSON[T]) IsEmpty() bool {
	return validator.IsEmptyValue(reflect.ValueOf(p.Data))
}

// Clone 克隆数据
func (p *ProtoJSON[T]) Clone() ProtoJSON[T] {
	rv := reflect.ValueOf(p.Data)

	// 处理 proto 消息
	if rv.Kind() == reflect.Ptr && rv.Type().Implements(protox.ProtoMessageType) {
		if !rv.IsNil() {
			cloned := proto.Clone(rv.Interface().(proto.Message))
			return ProtoJSON[T]{Data: cloned.(T)}
		}
		return ProtoJSON[T]{}
	}

	// 处理普通结构体
	clone := reflect.New(rv.Type()).Elem()
	clone.Set(rv)
	return ProtoJSON[T]{Data: clone.Interface().(T)}
}

// ProtoJSONMap - 泛型 protobuf JSON 类型，支持 protobuf 消息的数据库存储
type ProtoJSONMap[T proto.Message] struct {
	Fields map[string]ProtoJSON[T]
}

// NewProtoJSONMap - 创建一个新的 protobuf JSON 类型映射
func NewProtoJSONMap[T proto.Message]() *ProtoJSONMap[T] {
	return &ProtoJSONMap[T]{Fields: make(map[string]ProtoJSON[T])}
}

func (p *ProtoJSONMap[T]) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, err := toBytes(value)
	if err != nil {
		return fmt.Errorf("ProtoJSONMap scan: %w", err)
	}
	if len(b) == 0 || string(b) == "null" {
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	p.Fields = make(map[string]ProtoJSON[T], len(raw))
	for key, rawVal := range raw {
		if len(rawVal) == 0 || string(rawVal) == "null" {
			continue
		}
		var pj ProtoJSON[T]
		pj.Data = protox.NewProtoMessage[T]()
		if err := protojson.Unmarshal(rawVal, pj.Data); err != nil {
			return fmt.Errorf(errFieldFormat, key, err)
		}
		p.Fields[key] = pj
	}
	return nil
}

// Value - 将 protobuf JSON 类型转换为数据库值
func (p ProtoJSONMap[T]) Value() (driver.Value, error) {
	if len(p.Fields) == 0 {
		return nil, nil
	}
	result := make(map[string]string, len(p.Fields))
	for key, val := range p.Fields {
		if validator.IsEmptyValue(reflect.ValueOf(val.Data)) {
			continue
		}
		b, err := protojson.Marshal(val.Data)
		if err != nil {
			return nil, fmt.Errorf(errFieldFormat, key, err)
		}
		if string(b) != "{}" {
			result[key] = string(b)
		}
	}
	if len(result) == 0 {
		return nil, nil
	}
	return json.Marshal(result)
}

// Get - 获取指定键的 protobuf JSON 类型值
func (p *ProtoJSONMap[T]) Get(key string) T {
	if pj, ok := p.Fields[key]; ok {
		return pj.Data
	}
	var zero T
	return zero
}

// Set - 设置指定键的 protobuf JSON 类型值
func (p *ProtoJSONMap[T]) Set(key string, data T) {
	if p.Fields == nil {
		p.Fields = make(map[string]ProtoJSON[T])
	}
	p.Fields[key] = ProtoJSON[T]{Data: data}
}

// Keys - 获取所有键
func (p *ProtoJSONMap[T]) Keys() []string {
	keys := make([]string, 0, len(p.Fields))
	for k := range p.Fields {
		keys = append(keys, k)
	}
	return keys
}

// Len 获取键值对数量
func (p *ProtoJSONMap[T]) Len() int {
	return len(p.Fields)
}

// IsEmpty 判断是否为空
func (p *ProtoJSONMap[T]) IsEmpty() bool {
	return len(p.Fields) == 0
}

// Has 检查键是否存在
func (p *ProtoJSONMap[T]) Has(key string) bool {
	_, ok := p.Fields[key]
	return ok
}

// Delete 删除指定键
func (p *ProtoJSONMap[T]) Delete(key string) {
	delete(p.Fields, key)
}

// Clear 清空所有键值对
func (p *ProtoJSONMap[T]) Clear() {
	p.Fields = make(map[string]ProtoJSON[T])
}

// Clone 克隆一个新的映射
func (p *ProtoJSONMap[T]) Clone() ProtoJSONMap[T] {
	clone := ProtoJSONMap[T]{Fields: make(map[string]ProtoJSON[T], len(p.Fields))}
	for k, v := range p.Fields {
		clone.Fields[k] = v
	}
	return clone
}
