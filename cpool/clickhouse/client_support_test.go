//go:build clickhouse

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\cpool\clickhouse\client_support_test.go
 * @Description: ClickHouse DSN 解析测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package clickhouse

import (
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// TestBuildClickHouseOptionsDocumentedDSNExamples 把 config/tsdb.ClickHouse 类型注释里
// 列出的每一条 DSN 示例都真正跑一遍解析，并断言关键字段落位
//
// 示例写在注释里就会随驱动升级慢慢失真，这个用例把「注释里的示例」和
// 「clickhouse-go 的实际解析行为」绑在一起
//
// [EN] Every DSN example in the tsdb.ClickHouse doc comment is parsed here with its key
// fields asserted, tying the documented examples to clickhouse-go's real behaviour
func TestBuildClickHouseOptionsDocumentedDSNExamples(t *testing.T) {
	cases := []struct {
		name   string
		dsn    string
		assert func(t *testing.T, opts *clickhouse.Options, connMaxIdleTime time.Duration)
	}{
		{
			name: "本地开发最简形式",
			dsn:  "clickhouse://127.0.0.1:9000/default",
			assert: func(t *testing.T, opts *clickhouse.Options, _ time.Duration) {
				if got := opts.Addr; len(got) != 1 || got[0] != "127.0.0.1:9000" {
					t.Errorf("Addr = %v, want [127.0.0.1:9000]", got)
				}
				if opts.Auth.Database != "default" {
					t.Errorf("Database = %q, want default", opts.Auth.Database)
				}
				if opts.Protocol != clickhouse.Native {
					t.Errorf("Protocol = %v, want Native", opts.Protocol)
				}
			},
		},
		{
			name: "带认证与超时和压缩",
			dsn:  "clickhouse://default:ch_pwd@127.0.0.1:9000/metrics?dial_timeout=10s&read_timeout=20s&compress=lz4",
			assert: func(t *testing.T, opts *clickhouse.Options, _ time.Duration) {
				if opts.Auth.Username != "default" || opts.Auth.Password != "ch_pwd" {
					t.Errorf("Auth = %q/%q, want default/ch_pwd", opts.Auth.Username, opts.Auth.Password)
				}
				if opts.Auth.Database != "metrics" {
					t.Errorf("Database = %q, want metrics", opts.Auth.Database)
				}
				if opts.DialTimeout != 10*time.Second {
					t.Errorf("DialTimeout = %v, want 10s", opts.DialTimeout)
				}
				if opts.ReadTimeout != 20*time.Second {
					t.Errorf("ReadTimeout = %v, want 20s", opts.ReadTimeout)
				}
				if opts.Compression == nil || opts.Compression.Method != clickhouse.CompressionLZ4 {
					t.Errorf("Compression = %+v, want LZ4", opts.Compression)
				}
			},
		},
		{
			name: "多节点轮询",
			dsn:  "clickhouse://default:ch_pwd@ch-1:9000/metrics?alt_hosts=ch-2:9000,ch-3:9000&connection_open_strategy=round_robin",
			assert: func(t *testing.T, opts *clickhouse.Options, _ time.Duration) {
				if len(opts.Addr) != 3 {
					t.Errorf("Addr = %v, want 3 nodes", opts.Addr)
				}
				if opts.ConnOpenStrategy != clickhouse.ConnOpenRoundRobin {
					t.Errorf("ConnOpenStrategy = %v, want round_robin", opts.ConnOpenStrategy)
				}
			},
		},
		{
			name: "连接池与生命周期",
			dsn:  "clickhouse://default:ch_pwd@127.0.0.1:9000/metrics?max_idle_conns=5&max_open_conns=20&conn_max_lifetime=1h&conn_max_idle_time=5m",
			assert: func(t *testing.T, opts *clickhouse.Options, connMaxIdleTime time.Duration) {
				if opts.MaxIdleConns != 5 {
					t.Errorf("MaxIdleConns = %d, want 5", opts.MaxIdleConns)
				}
				if opts.MaxOpenConns != 20 {
					t.Errorf("MaxOpenConns = %d, want 20", opts.MaxOpenConns)
				}
				if opts.ConnMaxLifetime != time.Hour {
					t.Errorf("ConnMaxLifetime = %v, want 1h", opts.ConnMaxLifetime)
				}
				// 网关扩展参数被摘出来单独返回，而不是留在 Settings 里下发给服务端
				if connMaxIdleTime != 5*time.Minute {
					t.Errorf("connMaxIdleTime = %v, want 5m", connMaxIdleTime)
				}
				if _, leaked := opts.Settings[extConnMaxIdleTime]; leaked {
					t.Errorf("%s leaked into Settings, server would reject it as an unknown setting", extConnMaxIdleTime)
				}
			},
		},
		{
			name: "TLS 自签证书跳过校验",
			dsn:  "clickhouse://default:ch_pwd@ch.example.com:9440/metrics?secure=true&skip_verify=true",
			assert: func(t *testing.T, opts *clickhouse.Options, _ time.Duration) {
				if opts.TLS == nil {
					t.Fatal("TLS = nil, want configured")
				}
				if !opts.TLS.InsecureSkipVerify {
					t.Error("InsecureSkipVerify = false, want true")
				}
			},
		},
		{
			name: "HTTP 协议",
			dsn:  "http://default:ch_pwd@127.0.0.1:8123/metrics?dial_timeout=10s",
			assert: func(t *testing.T, opts *clickhouse.Options, _ time.Duration) {
				if opts.Protocol != clickhouse.HTTP {
					t.Errorf("Protocol = %v, want HTTP", opts.Protocol)
				}
			},
		},
		{
			name: "ClickHouse Cloud HTTPS",
			dsn:  "https://default:ch_pwd@xxx.clickhouse.cloud:8443/metrics?secure=true",
			assert: func(t *testing.T, opts *clickhouse.Options, _ time.Duration) {
				if opts.Protocol != clickhouse.HTTP {
					t.Errorf("Protocol = %v, want HTTP", opts.Protocol)
				}
				if opts.TLS == nil {
					t.Error("TLS = nil, want configured")
				}
			},
		},
		{
			name: "覆盖会话 setting",
			dsn:  "clickhouse://default:ch_pwd@127.0.0.1:9000/metrics?session_timezone=Asia/Shanghai&max_execution_time=60",
			assert: func(t *testing.T, opts *clickhouse.Options, _ time.Duration) {
				if got := opts.Settings[sessionTimezoneSetting]; got != "Asia/Shanghai" {
					t.Errorf("session_timezone = %v, want Asia/Shanghai", got)
				}
				if got := opts.Settings["max_execution_time"]; got != 60 {
					t.Errorf("max_execution_time = %v (%T), want 60", got, got)
				}
			},
		},
		{
			name: "默认 DSN（config.DefaultClickHouse 用的那条）",
			dsn:  "clickhouse://default@127.0.0.1:9000/default?dial_timeout=10s&read_timeout=20s&compress=lz4&max_idle_conns=5&max_open_conns=20&conn_max_lifetime=1h&conn_max_idle_time=5m",
			assert: func(t *testing.T, opts *clickhouse.Options, connMaxIdleTime time.Duration) {
				if opts.Auth.Username != "default" {
					t.Errorf("Username = %q, want default", opts.Auth.Username)
				}
				if connMaxIdleTime != 5*time.Minute {
					t.Errorf("connMaxIdleTime = %v, want 5m", connMaxIdleTime)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, connMaxIdleTime, err := buildClickHouseOptions(tc.dsn)
			if err != nil {
				t.Fatalf("buildClickHouseOptions(%q) failed: %v", tc.dsn, err)
			}
			tc.assert(t, opts, connMaxIdleTime)
		})
	}
}

// TestSessionTimezoneDefault 断言 DSN 没写 session_timezone 时补 UTC，写了就不动
//
// [EN] Asserts that session_timezone defaults to UTC when the DSN omits it and is left
// alone when the DSN states it
func TestSessionTimezoneDefault(t *testing.T) {
	t.Run("未写时补 UTC", func(t *testing.T) {
		opts, _, err := buildClickHouseOptions("clickhouse://127.0.0.1:9000/metrics")
		if err != nil {
			t.Fatalf("buildClickHouseOptions failed: %v", err)
		}
		if got := opts.Settings[sessionTimezoneSetting]; got != defaultSessionTimezone {
			t.Errorf("session_timezone = %v, want %s", got, defaultSessionTimezone)
		}
	})

	t.Run("写了就不覆盖", func(t *testing.T) {
		opts, _, err := buildClickHouseOptions("clickhouse://127.0.0.1:9000/metrics?session_timezone=Europe/Berlin")
		if err != nil {
			t.Fatalf("buildClickHouseOptions failed: %v", err)
		}
		if got := opts.Settings[sessionTimezoneSetting]; got != "Europe/Berlin" {
			t.Errorf("session_timezone = %v, want Europe/Berlin", got)
		}
	})
}

// TestSettingValueCasePreserved 断言 DSN 查询参数的值在进 Settings 时保持大小写
//
// 这是条历史故障的回归用例：clickhouse-go 早期版本的 fromDSN 把未知参数的值
// 统一小写化后再写进 Settings，于是 session_timezone=UTC 变成 utc，
// 服务端报 code: 36 Invalid time zone: utc，连接直接建不起来。
// v2.49.0 的 default 分支已改为写回原值（strings.ToLower 只用于判断 true/false），
// 所以现在可以放心把 DSN 交给 ParseDSN——这个用例盯着它别再退回去
//
// [EN] A regression test for a past failure: older clickhouse-go versions lowercased
// unknown parameter values before storing them in Settings, turning session_timezone=UTC
// into utc, which the server rejects with code: 36 Invalid time zone. In v2.49.0 the
// default branch stores the raw value and only lowercases for the true/false check, which
// is what makes handing the DSN to ParseDSN safe. This test watches for a regression
func TestSettingValueCasePreserved(t *testing.T) {
	opts, _, err := buildClickHouseOptions("clickhouse://127.0.0.1:9000/metrics?session_timezone=UTC&some_setting=MixedCase")
	if err != nil {
		t.Fatalf("buildClickHouseOptions failed: %v", err)
	}

	if got := opts.Settings[sessionTimezoneSetting]; got != "UTC" {
		t.Errorf("session_timezone = %v, want UTC (lowercased value is rejected by the server)", got)
	}
	if got := opts.Settings["some_setting"]; got != "MixedCase" {
		t.Errorf("some_setting = %v, want MixedCase", got)
	}
}

// TestBuildClickHouseOptionsRejectsBadDSN 断言坏 DSN 在解析这一步就报错
//
// [EN] Asserts that a bad DSN fails at parse time
func TestBuildClickHouseOptionsRejectsBadDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{
		{name: "没有主机", dsn: "clickhouse:///metrics"},
		{name: "超时值非法", dsn: "clickhouse://127.0.0.1:9000/metrics?dial_timeout=ten_seconds"},
		{name: "扩展参数值非法", dsn: "clickhouse://127.0.0.1:9000/metrics?conn_max_idle_time=abc"},
		{name: "https 缺 secure", dsn: "https://127.0.0.1:8443/metrics"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := buildClickHouseOptions(tc.dsn); err == nil {
				t.Fatalf("buildClickHouseOptions(%q) succeeded, want error", tc.dsn)
			}
		})
	}
}

// TestRedactDSN 断言日志里不会出现 DSN 中的密码
//
// [EN] Asserts that a DSN password never reaches the log
func TestRedactDSN(t *testing.T) {
	cases := []struct {
		dsn  string
		want string
	}{
		{dsn: "clickhouse://default:ch_pwd@127.0.0.1:9000/metrics", want: "clickhouse://default@127.0.0.1:9000/metrics"},
		{dsn: "clickhouse://127.0.0.1:9000/metrics", want: "clickhouse://127.0.0.1:9000/metrics"},
		{dsn: "clickhouse://u:p@[::1:9000", want: "<unparsable dsn>"},
	}

	for _, tc := range cases {
		if got := redactDSN(tc.dsn); got != tc.want {
			t.Errorf("redactDSN(%q) = %q, want %q", tc.dsn, got, tc.want)
		}
	}
}
