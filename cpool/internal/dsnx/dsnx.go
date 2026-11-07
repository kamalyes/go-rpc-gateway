/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\cpool\internal\dsnx\dsnx.go
 * @Description: DSN 查询串拆分工具，把网关扩展参数从 DSN 里摘出来再交给底层驱动解析
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// cpool 各连接池统一走「单 DSN」配置：连接信息、认证、超时、池参数全写在一条 DSN 里，
// 配置结构体只保留驱动无法表达的字段。各驱动自带 DSN 解析器
// （go-redis ParseURL / clickhouse-go ParseDSN / nats.Connect），
// 但它们对「不认识的查询参数」的处理方式互不相同，且都不利于承载网关自己的扩展参数：
//
//   - go-redis ParseURL/ParseClusterURL/ParseFailoverURL：遇到未知参数直接返回
//     "redis: unexpected option: xxx" 错误（options.go setupConnParams 末尾的 q.remaining 检查），
//     连接直接建不起来
//   - clickhouse-go ParseDSN：未知参数被当成 ClickHouse 服务端 setting 原样下发
//     （clickhouse_options.go fromDSN 的 default 分支写进 o.Settings），
//     服务端不认识就报 "code: 115 Unknown setting"，同样连不上
//
// 所以网关扩展参数（驱动不支持、但网关这一层要用的）必须在交给驱动之前摘干净，
// 这就是 Split 的职责
//
// [EN] Every cpool client takes a single DSN: endpoint, credentials, timeouts and pool
// sizing all live in one string, and the config struct keeps only what the DSN cannot
// express. Each driver ships its own DSN parser, but they disagree on unknown query
// parameters — go-redis rejects them outright, clickhouse-go forwards them to the server
// as settings — and neither can carry gateway-level extras. Split pulls those extras out
// before the driver ever sees the DSN.
package dsnx

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Ext 承载从 DSN 查询串里摘出的网关扩展参数
// 取值方法遇到格式错误不会立刻返回 error，而是累积到 Err()，
// 调用方解析完所有参数后统一检查一次即可
//
// [EN] Ext holds the gateway-level parameters pulled out of a DSN query string. The
// accessors accumulate format errors into Err() instead of returning them one by one, so
// a caller reads every parameter first and checks once
type Ext struct {
	values url.Values
	errs   []string
}

// Split 把 DSN 拆成「交给驱动解析的 URL」与「网关自己消费的扩展参数」
// extKeys 列出要摘走的参数名；没有出现在 DSN 里的 key 不会报错，取值时走调用方给的默认值
//
// 返回解析后的 *url.URL 而不是字符串：调用方除了把它交给驱动（u.String()），
// 通常还要读剩下的查询参数来判定连接模式（如 redis 靠 master_name 区分哨兵），
// 返回字符串会让调用方再解析一遍同一条 DSN
//
// 只有确实摘走了参数才会重写 RawQuery：url.Values.Encode 会给 key 排序并重新转义，
// 没必要时不去动用户写的 DSN——不重写时 u.String() 原样复刻入参的查询串
//
// [EN] Split separates a DSN into the URL the driver parses and the gateway-only
// parameters named by extKeys; a key absent from the DSN is not an error and falls back to
// the caller's default at read time. It returns the parsed *url.URL rather than a string
// because a caller usually also reads the remaining query to pick a connection mode (redis
// tells sentinel apart by master_name), and a string would force it to parse the same DSN
// twice. RawQuery is rewritten only when something was actually removed, since
// url.Values.Encode reorders and re-escapes keys; left alone, u.String() reproduces the
// caller's query string verbatim
func Split(rawURL string, extKeys ...string) (*url.URL, *Ext, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse dsn failed: %w", err)
	}

	query := u.Query()
	ext := &Ext{values: url.Values{}}

	popped := false
	for _, key := range extKeys {
		if vs, ok := query[key]; ok {
			ext.values[key] = vs
			query.Del(key)
			popped = true
		}
	}

	if popped {
		u.RawQuery = query.Encode()
	}

	return u, ext, nil
}

// Has 判断扩展参数是否在 DSN 里显式出现过
// 用于区分「没配」和「配成了零值」——后者是调用方的明确意图，不该被默认值覆盖
//
// [EN] Has reports whether the parameter was written in the DSN at all, which separates
// "unset" from "explicitly set to a zero value"; the latter is a deliberate choice and
// must not be overwritten by a default
func (e *Ext) Has(key string) bool {
	_, ok := e.values[key]
	return ok
}

// String 取字符串型扩展参数，未配置返回 def
//
// [EN] String returns a string parameter, or def when unset
func (e *Ext) String(key, def string) string {
	if v := e.values.Get(key); v != "" {
		return v
	}
	return def
}

// Bool 取布尔型扩展参数，未配置返回 def
// 空值（如 ?cluster）视为 true，与 clickhouse-go 对 secure/skip_verify 的处理保持一致
//
// [EN] Bool returns a boolean parameter, or def when unset. A bare key with no value (as
// in ?cluster) counts as true, matching how clickhouse-go treats secure and skip_verify
func (e *Ext) Bool(key string, def bool) bool {
	if !e.Has(key) {
		return def
	}
	raw := e.values.Get(key)
	if raw == "" {
		return true
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		e.errs = append(e.errs, fmt.Sprintf("%s: invalid bool %q", key, raw))
		return def
	}
	return v
}

// Int 取整型扩展参数，未配置返回 def
//
// [EN] Int returns an integer parameter, or def when unset
func (e *Ext) Int(key string, def int) int {
	if !e.Has(key) {
		return def
	}
	raw := e.values.Get(key)
	v, err := strconv.Atoi(raw)
	if err != nil {
		e.errs = append(e.errs, fmt.Sprintf("%s: invalid int %q", key, raw))
		return def
	}
	return v
}

// Duration 取时长型扩展参数，未配置返回 def
// 带单位走 time.ParseDuration（如 30s、5m），纯数字按秒解释，
// 与 go-redis 查询参数的时长约定一致（options.go queryOptions.duration）
//
// [EN] Duration returns a duration parameter, or def when unset. A value with a unit goes
// through time.ParseDuration (30s, 5m); a bare integer is read as seconds, matching the
// convention go-redis uses for duration query parameters
func (e *Ext) Duration(key string, def time.Duration) time.Duration {
	if !e.Has(key) {
		return def
	}
	raw := e.values.Get(key)
	if secs, err := strconv.Atoi(raw); err == nil {
		return time.Duration(secs) * time.Second
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		e.errs = append(e.errs, fmt.Sprintf("%s: invalid duration %q", key, raw))
		return def
	}
	return v
}

// Err 返回取值过程中累积的全部格式错误，没有错误返回 nil
//
// [EN] Err returns every format error accumulated while reading parameters, or nil
func (e *Ext) Err() error {
	if len(e.errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid dsn parameter: %s", strings.Join(e.errs, "; "))
}
