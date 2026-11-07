/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\cpool\internal\dsnx\dsnx_test.go
 * @Description: DSN 查询串拆分工具测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package dsnx

import (
	"testing"
	"time"
)

// TestSplitLeavesDSNUntouchedWhenNothingPopped 断言没摘到任何扩展参数时 DSN 原样保留
//
// 这是 Split 的一条硬要求：url.Values.Encode 会给 key 排序并重新转义，
// 一旦无条件重写 RawQuery，像 clickhouse 的 session_timezone=Asia/Shanghai 这种值
// 就会被转义成 Asia%2FShanghai——驱动能解析，但日志里跟用户配的那条对不上，排查时很误导
//
// [EN] A hard requirement on Split: url.Values.Encode reorders and re-escapes keys, so
// rewriting RawQuery unconditionally would turn a value like Asia/Shanghai into
// Asia%2FShanghai — parseable, but no longer matching what the user wrote, which misleads
// anyone reading it back out of a log
func TestSplitLeavesDSNUntouchedWhenNothingPopped(t *testing.T) {
	const dsn = "clickhouse://default:pwd@127.0.0.1:9000/metrics?session_timezone=Asia/Shanghai&dial_timeout=10s"

	u, ext, err := Split(dsn, "conn_max_idle_time")
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if got := u.String(); got != dsn {
		t.Errorf("Split(%q) = %q, want it unchanged", dsn, got)
	}
	if ext.Has("conn_max_idle_time") {
		t.Error("Has(conn_max_idle_time) = true, want false")
	}
}

// TestSplitPopsExtParams 断言扩展参数被摘走、其余参数留在 URL 上
//
// [EN] Asserts that the named parameters are removed and everything else stays on the URL
func TestSplitPopsExtParams(t *testing.T) {
	const dsn = "redis://127.0.0.1:6379/0?pool_size=200&cluster=true&read_buffer_size=65536"

	u, ext, err := Split(dsn, "cluster", "read_buffer_size")
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	query := u.Query()
	if query.Has("cluster") || query.Has("read_buffer_size") {
		t.Errorf("ext params left on url: %s", u.RawQuery)
	}
	if got := query.Get("pool_size"); got != "200" {
		t.Errorf("pool_size = %q, want 200 (driver params must survive)", got)
	}
	if !ext.Bool("cluster", false) {
		t.Error("ext cluster = false, want true")
	}
	if got := ext.Int("read_buffer_size", 0); got != 65536 {
		t.Errorf("ext read_buffer_size = %d, want 65536", got)
	}
	if err := ext.Err(); err != nil {
		t.Errorf("ext.Err() = %v, want nil", err)
	}
}

// TestSplitRepeatedParamsPreserved 断言可重复出现的参数（redis 集群的 addr）不会被摘丢
//
// [EN] Asserts that a parameter allowed to repeat, such as redis cluster's addr, survives
func TestSplitRepeatedParamsPreserved(t *testing.T) {
	const dsn = "redis://10.0.0.1:6379?addr=10.0.0.2:6379&addr=10.0.0.3:6379&cluster=true"

	u, _, err := Split(dsn, "cluster")
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if got := len(u.Query()["addr"]); got != 2 {
		t.Errorf("addr count = %d, want 2", got)
	}
}

// TestExtAccessorDefaults 断言未配置的扩展参数走调用方给的默认值，
// 而显式配成零值不会被默认值覆盖
//
// [EN] Asserts that an unset parameter falls back to the caller's default while a value
// explicitly set to zero is not overwritten by it
func TestExtAccessorDefaults(t *testing.T) {
	_, ext, err := Split("redis://127.0.0.1:6379?read_buffer_size=0&cluster=false", "read_buffer_size", "cluster", "write_buffer_size")
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	// 显式写了 0 / false，不该被默认值顶掉
	if got := ext.Int("read_buffer_size", 32768); got != 0 {
		t.Errorf("read_buffer_size = %d, want 0 (explicit zero must win over the default)", got)
	}
	if ext.Bool("cluster", true) {
		t.Error("cluster = true, want false (explicit false must win over the default)")
	}
	// 没写的走默认值
	if got := ext.Int("write_buffer_size", 32768); got != 32768 {
		t.Errorf("write_buffer_size = %d, want the 32768 default", got)
	}
}

// TestExtBoolBareKey 断言不带值的参数（?cluster）视为 true
//
// [EN] Asserts that a bare key with no value counts as true
func TestExtBoolBareKey(t *testing.T) {
	_, ext, err := Split("redis://127.0.0.1:6379?cluster", "cluster")
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}
	if !ext.Bool("cluster", false) {
		t.Error("cluster = false, want true for a bare key")
	}
}

// TestExtDuration 断言时长参数带单位与纯数字两种写法
// 纯数字按秒解释，与 go-redis 查询参数的约定一致
//
// [EN] Asserts both duration spellings: a value with a unit, and a bare integer read as
// seconds, matching go-redis's convention for duration query parameters
func TestExtDuration(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
	}{
		{raw: "30s", want: 30 * time.Second},
		{raw: "5m", want: 5 * time.Minute},
		{raw: "1h30m", want: 90 * time.Minute},
		{raw: "45", want: 45 * time.Second},
	}

	for _, tc := range cases {
		_, ext, err := Split("redis://127.0.0.1:6379?conn_max_idle_time="+tc.raw, "conn_max_idle_time")
		if err != nil {
			t.Fatalf("Split failed: %v", err)
		}
		if got := ext.Duration("conn_max_idle_time", 0); got != tc.want {
			t.Errorf("Duration(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

// TestExtErrAccumulates 断言格式错误累积到 Err()，且一次报全
// 取值方法逐个返回 error 会让调用方写一长串 if err != nil，也只能报出第一个问题
//
// [EN] Asserts that format errors accumulate into Err() and are all reported at once;
// returning them one accessor at a time would both clutter the caller and surface only the
// first problem
func TestExtErrAccumulates(t *testing.T) {
	_, ext, err := Split("redis://127.0.0.1:6379?a=xx&b=yy&c=zz", "a", "b", "c")
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	ext.Int("a", 0)
	ext.Bool("b", false)
	ext.Duration("c", 0)

	extErr := ext.Err()
	if extErr == nil {
		t.Fatal("Err() = nil, want an error")
	}
	for _, key := range []string{"a", "b", "c"} {
		if !contains(extErr.Error(), key) {
			t.Errorf("Err() = %q, want it to mention %q", extErr.Error(), key)
		}
	}
}

// TestSplitRejectsUnparsableDSN 断言无法解析的 DSN 直接报错
//
// [EN] Asserts that an unparsable DSN fails outright
func TestSplitRejectsUnparsableDSN(t *testing.T) {
	if _, _, err := Split("redis://u:p@[::1:6379", "cluster"); err == nil {
		t.Fatal("Split succeeded, want error")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
