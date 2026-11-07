/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\cpool\redis\redis_test.go
 * @Description: Redis DSN 解析与连接模式判定测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package redis

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

// TestNewClientDocumentedDSNExamples 把 config/cache.Redis 类型注释里列出的每一条 DSN 示例
// 都真正跑一遍 newClient，并断言判定出的连接模式与构建出的客户端类型
//
// 示例写在注释里就会随驱动升级慢慢失真——参数改名、未知参数从忽略变报错，
// 文档却没人动。这个用例的作用是让「注释里的示例」和「驱动的实际行为」绑在一起：
// 哪天 go-redis 不再认某个参数，这里先红，而不是等线上连不上才发现
//
// [EN] Every DSN example written in the cache.Redis doc comment is run through newClient
// here, with the resulting connection mode asserted. Examples in comments rot as the driver
// moves on — parameters get renamed, unknown ones go from ignored to fatal — and nobody
// updates the comment. This test ties the documented examples to the driver's real
// behaviour, so a parameter go-redis stops accepting fails here instead of at a deploy
func TestNewClientDocumentedDSNExamples(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string // 期望判定出的连接模式
	}{
		{
			name: "单机最简形式",
			dsn:  "redis://127.0.0.1:6379",
			want: modeSingle,
		},
		{
			name: "单机带用户名密码与库号",
			dsn:  "redis://default:redis123456@127.0.0.1:6379/3",
			want: modeSingle,
		},
		{
			name: "单机仅密码",
			dsn:  "redis://:redis123456@127.0.0.1:6379/0",
			want: modeSingle,
		},
		{
			name: "单机调连接池与超时",
			dsn:  "redis://default:pwd@127.0.0.1:6379/0?pool_size=200&min_idle_conns=10&dial_timeout=5s&read_timeout=3s&write_timeout=3s",
			want: modeSingle,
		},
		{
			name: "TLS 跳过证书校验",
			dsn:  "rediss://default:pwd@redis.example.com:6380/0?skip_verify=true",
			want: modeSingle,
		},
		{
			name: "集群多节点",
			dsn:  "redis://default:pwd@10.0.0.1:6379?addr=10.0.0.2:6379&addr=10.0.0.3:6379&route_by_latency=true",
			want: modeCluster,
		},
		{
			name: "集群单入口地址",
			dsn:  "redis://default:pwd@redis-cluster.example.com:6379?cluster=true&max_redirects=3",
			want: modeCluster,
		},
		{
			name: "哨兵",
			dsn:  "redis://sentinel_user:sentinel_pwd@10.0.0.1:26379/0?master_name=mymaster&addr=10.0.0.2:26379&username=default&password=data_pwd",
			want: modeSentinel,
		},
		{
			name: "网关扩展参数全开",
			dsn:  "redis://127.0.0.1:6379/0?read_buffer_size=65536&write_buffer_size=65536&context_timeout_enabled=true",
			want: modeSingle,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, mode, err := newClient(tc.dsn)
			if err != nil {
				t.Fatalf("newClient(%q) failed: %v", tc.dsn, err)
			}
			defer client.Close()

			if mode != tc.want {
				t.Fatalf("newClient(%q) mode = %q, want %q", tc.dsn, mode, tc.want)
			}

			// 模式名与实际构建出的客户端类型必须对得上：
			// 集群走 *redis.ClusterClient，单机与哨兵都是 *redis.Client
			// （NewFailoverClient 返回的也是 *redis.Client）
			if tc.want == modeCluster {
				if _, ok := client.(*redis.ClusterClient); !ok {
					t.Fatalf("mode %q built %T, want *redis.ClusterClient", mode, client)
				}
			} else if _, ok := client.(*redis.Client); !ok {
				t.Fatalf("mode %q built %T, want *redis.Client", mode, client)
			}
		})
	}
}

// TestNewClientExtParamsApplied 断言网关扩展参数被摘出并真正写回 go-redis 的 Options
// 这几项 go-redis 的 URL 解析器不认识，摘漏了会直接让解析报错，写回漏了会静默失效
//
// [EN] Asserts that the gateway's extension parameters are both stripped from the DSN and
// written back onto go-redis's options: a missed strip makes the parse fail outright, while
// a missed write-back fails silently
func TestNewClientExtParamsApplied(t *testing.T) {
	const dsn = "redis://127.0.0.1:6379/0?read_buffer_size=65536&write_buffer_size=32768&context_timeout_enabled=true&pool_size=42"

	client, _, err := newClient(dsn)
	if err != nil {
		t.Fatalf("newClient failed: %v", err)
	}
	defer client.Close()

	single, ok := client.(*redis.Client)
	if !ok {
		t.Fatalf("newClient = %T, want *redis.Client", client)
	}

	opts := single.Options()
	if opts.ReadBufferSize != 65536 {
		t.Errorf("ReadBufferSize = %d, want 65536", opts.ReadBufferSize)
	}
	if opts.WriteBufferSize != 32768 {
		t.Errorf("WriteBufferSize = %d, want 32768", opts.WriteBufferSize)
	}
	if !opts.ContextTimeoutEnabled {
		t.Error("ContextTimeoutEnabled = false, want true")
	}
	// 驱动原生参数不受扩展参数摘取影响
	if opts.PoolSize != 42 {
		t.Errorf("PoolSize = %d, want 42", opts.PoolSize)
	}
	// 禁用 CLIENT SETINFO，避免 maint_notifications 错误
	if !opts.DisableIdentity {
		t.Error("DisableIdentity = false, want true")
	}
}

// TestNewClientRejectsBadDSN 断言坏 DSN 在建客户端这一步就报错，而不是等到第一条命令
//
// [EN] Asserts that a bad DSN fails while building the client rather than on the first command
func TestNewClientRejectsBadDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{
		{name: "scheme 不对", dsn: "http://127.0.0.1:6379"},
		{name: "未知查询参数", dsn: "redis://127.0.0.1:6379?no_such_option=1"},
		{name: "扩展参数值非法", dsn: "redis://127.0.0.1:6379?read_buffer_size=abc"},
		{name: "库号非法", dsn: "redis://127.0.0.1:6379/not_a_number"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _, err := newClient(tc.dsn)
			if err == nil {
				client.Close()
				t.Fatalf("newClient(%q) succeeded, want error", tc.dsn)
			}
		})
	}
}

// TestRedactDSN 断言日志里不会出现 DSN 中的密码
//
// [EN] Asserts that a DSN password never reaches the log
func TestRedactDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "带用户名与密码",
			dsn:  "redis://default:redis123456@127.0.0.1:6379/0",
			want: "redis://default@127.0.0.1:6379/0",
		},
		{
			name: "仅密码",
			dsn:  "redis://:redis123456@127.0.0.1:6379/0",
			want: "redis://@127.0.0.1:6379/0",
		},
		{
			name: "无认证信息",
			dsn:  "redis://127.0.0.1:6379/0",
			want: "redis://127.0.0.1:6379/0",
		},
		{
			name: "无法解析",
			dsn:  "redis://user:pwd@[::1:6379",
			want: "<unparsable dsn>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactDSN(tc.dsn); got != tc.want {
				t.Errorf("redactDSN(%q) = %q, want %q", tc.dsn, got, tc.want)
			}
		})
	}
}
