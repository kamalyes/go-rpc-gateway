/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\observability\accesslog_types.go
 * @Description: 访问日志列类型 - 大字段落库编解码器（接口 + 注入）与两个列类型
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package observability

import (
	"database/sql/driver"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/kamalyes/go-toolbox/pkg/serializer"
	"github.com/kamalyes/go-toolbox/pkg/zipx"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// AccessLogCodec 访问日志大字段（请求头 / 请求体 / 响应体）的落库编解码器
//
// 这三列**存成什么样是调用方的决定**，不是网关的：审计库可能是 ClickHouse 也可能是
// MySQL，可能要 gzip、可能要换一种压缩、也可能要先过一层加密。原先这三个字段直接写死
// 成 cpool/database/types 里的 CompressedText / CompressedTextMap，于是网关为了一个
// 日志结构体背上了一整套数据库列类型的维护责任 —— 收成一个接口之后，网关只负责把快照
// 交出去，编解码由调用方注入
//
// 实现必须是并发安全的：Value / Scan 会被多个请求的落库路径并发调用
//
// [EN] Codec for the access log's large columns (headers, request body, response body).
//
// How these three columns are stored is the *caller's* decision, not the gateway's: the audit
// store may be ClickHouse or MySQL, may want gzip, a different compression, or a layer of
// encryption first. These fields used to be hard-wired to CompressedText/CompressedTextMap from
// cpool/database/types, which made the gateway own a whole set of database column types for the
// sake of one log struct. Behind an interface the gateway only hands out the snapshot and the
// caller injects the codec.
//
// Implementations must be safe for concurrent use: Value and Scan are called concurrently from
// the persistence paths of many requests
type AccessLogCodec interface {
	// Encode 落库前编码
	// [EN] Encodes a value before it is written
	Encode(data []byte) ([]byte, error)

	// Decode 读取后解码；必须兼容未编码的历史数据——同一张审计表里很可能既有
	// 改造前写入的明文，也有编码过的新数据
	// [EN] Decodes a value after it is read; must tolerate unencoded legacy data, since one
	// audit table may well hold both plain values written earlier and encoded ones
	Decode(data []byte) ([]byte, error)
}

// GzipAccessLogCodec 默认编解码器：gzip 压缩并带 GZIP: 前缀，读取时按前缀智能解压
//
// 默认值刻意选它而不是明文透传：已有审计库里的数据正是这个格式（改造前
// sqltypes.CompressedText 走的就是同样两个 zipx 调用），默认明文会让升级后读到的
// 历史行变成一堆压缩字节
//
// 两个方法都直接转给 go-toolbox/pkg/zipx，这里不含任何压缩逻辑
//
// [EN] The default codec: gzip with a GZIP: prefix, prefix-aware decompression on read.
//
// It is the default rather than a plain passthrough because that is exactly the format already
// in the audit stores — the previous sqltypes.CompressedText made the same two zipx calls — and
// defaulting to plain text would turn legacy rows into compressed bytes on read.
//
// Both methods delegate straight to go-toolbox/pkg/zipx; no compression logic lives here
type GzipAccessLogCodec struct{}

// Encode gzip 压缩并添加 GZIP: 前缀
func (GzipAccessLogCodec) Encode(data []byte) ([]byte, error) {
	return zipx.GzipCompressWithPrefix(data)
}

// Decode 按 GZIP: 前缀智能解压，无前缀的历史明文原样返回
func (GzipAccessLogCodec) Decode(data []byte) ([]byte, error) {
	return zipx.GzipSmartDecompress(data)
}

// PlainAccessLogCodec 原样透传：明文落库，不做任何编解码
//
// 给「库里的审计数据要能直接用 SQL 看 / LIKE」的场景，以及测试
// [EN] A passthrough codec that stores values verbatim, for audit data meant to be readable or
// LIKE-able directly in SQL, and for tests
type PlainAccessLogCodec struct{}

// Encode 原样返回
func (PlainAccessLogCodec) Encode(data []byte) ([]byte, error) { return data, nil }

// Decode 原样返回
func (PlainAccessLogCodec) Decode(data []byte) ([]byte, error) { return data, nil }

// 当前注入的编解码器，nil 表示用默认的 GzipAccessLogCodec
//
// 用 atomic.Pointer 而不是普通变量：注入一般发生在启动期、读取发生在落库路径，
// 但「一般」不是「保证」——调用方完全可以在运行期换掉它（比如按配置热加载），
// 普通变量的这组读写在 -race 下就是数据竞争
//
// 存的是 *AccessLogCodec 而不是 AccessLogCodec：接口值是两个字宽，原子指针只能
// 搬一个字宽的指针
//
// [EN] The injected codec; nil means the default GzipAccessLogCodec.
//
// An atomic.Pointer rather than a plain variable: injection usually happens at startup and
// reads happen on the persistence path, but "usually" is not "always" — a caller may swap it at
// runtime, say on a config reload — and that pair of accesses on a plain variable is a data
// race under -race. It holds a *AccessLogCodec because an interface value is two words wide
// while an atomic pointer can only carry one
var accessLogCodec atomic.Pointer[AccessLogCodec]

// SetAccessLogCodec 注入访问日志大字段的落库编解码器，返回恢复函数
//
// 传 nil 恢复默认（GzipAccessLogCodec）。恢复函数把编解码器还原成本次调用前的值，
// 重复调用只生效一次 —— 与 RegisterAccessLog 的反注册同一个用法
//
// [EN] Injects the codec for the access log's large columns and returns a restore function.
// Passing nil restores the default. The restore function puts back whatever was in effect
// before this call and takes effect only once, the same shape as RegisterAccessLog's
// unregister
func SetAccessLogCodec(codec AccessLogCodec) (restore func()) {
	prev := accessLogCodec.Load()
	if codec == nil {
		accessLogCodec.Store(nil)
	} else {
		accessLogCodec.Store(&codec)
	}

	var once sync.Once
	return func() {
		once.Do(func() { accessLogCodec.Store(prev) })
	}
}

// activeAccessLogCodec 当前生效的编解码器，未注入时给默认 gzip
//
// 与 activeMasker 同样的理由：中间件是导出的、可以被单独使用，不能假定调用方跑过
// 完整初始化链 —— 兜底给默认实现，而不是让 Value / Scan 在未注入时 panic
//
// [EN] The codec in effect, falling back to gzip when none was injected. Same reasoning as
// activeMasker: the middlewares are exported and usable on their own, so the caller cannot be
// assumed to have run a full init chain; Value and Scan fall back instead of panicking
func activeAccessLogCodec() AccessLogCodec {
	if codec := accessLogCodec.Load(); codec != nil && *codec != nil {
		return *codec
	}
	return GzipAccessLogCodec{}
}

// =================== AccessText（大文本列） ===================

// AccessText 访问日志里的大文本列：请求体与响应体
//
// 落库走注入的编解码器（默认 gzip），空串写 NULL
// [EN] A large text column in the access log — the request and response bodies. It goes through
// the injected codec (gzip by default) and an empty string is written as NULL
type AccessText string

// Value 实现 driver.Valuer：空值返回 nil，非空值交给编解码器
func (t AccessText) Value() (driver.Value, error) {
	if t == "" {
		return nil, nil
	}
	return activeAccessLogCodec().Encode([]byte(t))
}

// Scan 实现 sql.Scanner：读出的值交给编解码器解码
func (t *AccessText) Scan(value any) error {
	if value == nil {
		*t = ""
		return nil
	}
	b, err := accessLogScanBytes(value)
	if err != nil {
		return fmt.Errorf("access log text scan: %w", err)
	}
	if len(b) == 0 {
		*t = ""
		return nil
	}
	data, err := activeAccessLogCodec().Decode(b)
	if err != nil {
		return fmt.Errorf("access log text decode: %w", err)
	}
	*t = AccessText(data)
	return nil
}

// GormDataType 实现 schema.GormDataTypeInterface：声明逻辑数据类型为 string
//
// 必须实现：否则 gorm schema 解析时 DataType 为空，字段会被误判为关联关系（parseRelation 报错）
// [EN] Required: without it gorm's schema parse leaves DataType empty and the field is
// mistaken for a relation, failing in parseRelation
func (AccessText) GormDataType() string {
	return "string"
}

// GormDBDataType 按方言返回列类型：ClickHouse→String，MySQL→LONGTEXT，其他→TEXT
//
// 不能省掉它交给 gorm 按 "string" 自己推：gorm mysql 驱动在 DefaultStringSize 被配置过
// 时会把无 size 的 string 列推成 varchar(DefaultStringSize)，而这两列装的是整个请求体/
// 响应体 —— 那是静默截断
//
// [EN] Not left to gorm's own mapping of "string": with DefaultStringSize configured, the gorm
// mysql driver maps a size-less string column to varchar(DefaultStringSize), and these columns
// hold entire request and response bodies — that is silent truncation
func (AccessText) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	return accessLogTextColumnType(db)
}

// String 返回底层字符串值
func (t AccessText) String() string {
	return string(t)
}

// =================== AccessHeaders（请求头列） ===================

// AccessHeaders 访问日志里的请求头列
//
// 形状就是 http.Header（map[string][]string）：先 JSON 序列化再交给编解码器
// [EN] The access log's header column, shaped exactly like http.Header: JSON-marshalled first,
// then handed to the codec
type AccessHeaders map[string][]string

// Value 实现 driver.Valuer：nil 返回 nil；非 nil 先 JSON 序列化再交给编解码器
func (h AccessHeaders) Value() (driver.Value, error) {
	if h == nil {
		return nil, nil
	}
	data, err := serializer.JSONMarshal(h)
	if err != nil {
		return nil, fmt.Errorf("access log headers marshal: %w", err)
	}
	return activeAccessLogCodec().Encode(data)
}

// Scan 实现 sql.Scanner：解码后 JSON 反序列化
func (h *AccessHeaders) Scan(value any) error {
	if value == nil {
		*h = nil
		return nil
	}
	b, err := accessLogScanBytes(value)
	if err != nil {
		return fmt.Errorf("access log headers scan: %w", err)
	}
	if len(b) == 0 {
		*h = nil
		return nil
	}
	data, err := activeAccessLogCodec().Decode(b)
	if err != nil {
		return fmt.Errorf("access log headers decode: %w", err)
	}
	if len(data) == 0 {
		*h = nil
		return nil
	}
	return serializer.JSONUnmarshal(data, h)
}

// GormDataType 实现 schema.GormDataTypeInterface：声明逻辑数据类型为 string
//
// 必须实现：map 底层类型无法被 gorm 推导 DataType，空 DataType 会被误判为关联关系导致解析失败
// [EN] Required: gorm cannot derive a DataType for a map, and an empty DataType is mistaken for
// a relation, failing the schema parse
func (AccessHeaders) GormDataType() string {
	return "string"
}

// GormDBDataType 按方言返回列类型，与 AccessText 一致（压缩后的 JSON 字符串）
func (AccessHeaders) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	return accessLogTextColumnType(db)
}

// ToMap 转为原生 map[string][]string（零拷贝，底层类型相同）
func (h AccessHeaders) ToMap() map[string][]string {
	return map[string][]string(h)
}

// accessLogScanBytes 把数据库值统一转换为 []byte
func accessLogScanBytes(value any) ([]byte, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unsupported scan type: %T", value)
	}
}

// accessLogTextColumnType 大文本列在各方言下的列类型
func accessLogTextColumnType(db *gorm.DB) string {
	switch db.Dialector.Name() {
	case "mysql":
		return "LONGTEXT"
	case "sqlite", "postgres":
		return "TEXT"
	default:
		return "String" // ClickHouse（gorm clickhouse driver 默认映射）
	}
}
