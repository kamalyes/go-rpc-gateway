/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-15 11:37:08
 * @FilePath: \go-rpc-gateway\cpool\redis\redis.go
 * @Description: Redis连接客户端（单 DSN），兼容Gateway结构
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package redis

import (
	"context"
	"fmt"
	"net/url"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/cpool/internal/dsnx"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// 网关扩展查询参数名
// go-redis 的 URL 解析器不认识这几项（遇到未知参数会直接报错），
// 所以交给驱动之前必须先摘出来，再手工写回 Options
//
// [EN] Gateway-level query parameters. go-redis's URL parsers reject unknown parameters
// outright, so these are pulled out before the driver sees the DSN and written back onto
// the parsed options by hand
const (
	extCluster               = "cluster"                 // 单地址也按集群模式连接
	extReadBufferSize        = "read_buffer_size"        // 每连接读缓冲区字节数
	extWriteBufferSize       = "write_buffer_size"       // 每连接写缓冲区字节数
	extContextTimeoutEnabled = "context_timeout_enabled" // 用 context 截止时间控制命令超时
)

// extKeys 全部网关扩展参数，按此列表从 DSN 查询串里摘取
var extKeys = []string{extCluster, extReadBufferSize, extWriteBufferSize, extContextTimeoutEnabled}

// 连接模式，由 DSN 判定得出，连接成功后写进日志
// 模式是自动判定的，判错（例如忘了给单地址集群加 cluster=true，被当成单机连上去，
// 然后在第一条跨槽命令上报 MOVED）排查起来很绕，所以把判定结果直接摆到启动日志里
//
// [EN] The connection mode inferred from the DSN, reported in the startup log. Inference
// that lands on the wrong mode is awkward to diagnose — a single-address cluster missing
// cluster=true connects happily as a single node and only fails on the first cross-slot
// command with MOVED — so the inferred mode goes straight into the log
const (
	modeSingle   = "single"   // 单机
	modeCluster  = "cluster"  // 集群
	modeSentinel = "sentinel" // 哨兵
)

// Redis 初始化 redis 客户端（单 DSN）
//
// 连接模式由 DSN 自行表达，不再需要额外的配置开关：
//   - master_name 查询参数非空 → 哨兵模式，ParseFailoverURL + NewFailoverClient
//   - 出现重复的 addr 查询参数，或带网关扩展参数 cluster=true → 集群模式，
//     ParseClusterURL + NewClusterClient
//   - 其余 → 单机模式，ParseURL + NewClient
//
// 三种模式都直接用 go-redis 官方解析器，而不是先填 UniversalOptions 再让
// NewUniversalClient 去判定：UniversalOptions 没有对应的 URL 解析器，走它就得把
// DSN 查询参数再手工搬一遍到结构体字段上——那正是这次要消掉的那份映射表。
// 三个官方解析器各自认得本模式专属的参数（集群的 max_redirects、哨兵的 replica_only 等），
// 填错还会直接报错，比自己维护白名单更准
//
// [EN] The DSN alone selects the connection mode, so no extra config switch is needed:
// a master_name parameter means sentinel, repeated addr parameters or the gateway's
// cluster=true means cluster, and anything else is a single node. Each mode uses go-redis's
// own parser rather than filling a UniversalOptions and letting NewUniversalClient decide,
// because UniversalOptions has no URL parser — going through it would mean hand-copying
// every query parameter onto a struct field again, which is exactly the mapping table this
// change removes. The three upstream parsers also know their mode's own parameters
// (max_redirects for cluster, replica_only for sentinel) and reject typos outright
//
// 返回 redis.UniversalClient 接口，统一单机、集群与哨兵的调用方式；
// 配置缺失或连接失败返回 nil，由调用方决定降级还是失败退出
func Redis(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) redis.UniversalClient {
	// 先检查配置是否存在，再检查是否启用，避免 cfg.Cache 为 nil 时 panic
	if cfg == nil || cfg.Cache == nil {
		if log != nil {
			log.WarnContext(ctx, "Redis configuration not found")
		}
		return nil
	}

	if !cfg.Cache.Enabled {
		return nil
	}

	rawDSN := cfg.Cache.Redis.URL
	if rawDSN == "" {
		if log != nil {
			log.WarnContext(ctx, "Redis url not configured")
		}
		return nil
	}

	client, mode, err := newClient(rawDSN)
	if err != nil {
		if log != nil {
			log.ErrorContextKV(ctx, "Redis client build failed", "err", err)
		}
		return nil
	}

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		if log != nil {
			// DSN 里带密码，日志只打脱敏后的地址，不打整条 DSN
			log.ErrorContextKV(ctx, "Redis connection failed", "mode", mode, "addr", redactDSN(rawDSN), "err", err)
		}
		_ = client.Close()
		return nil
	}

	if log != nil {
		log.InfoContextKV(ctx, "Redis connected", "mode", mode, "addr", redactDSN(rawDSN))
	}

	return client
}

// newClient 按 DSN 判定连接模式并构建对应客户端，返回客户端与判定出的模式名
//
// 三个官方解析器返回三种互不相同的 Options 结构体，Go 里没法用一个变量统一承接，
// 所以扩展参数先在 switch 之前取成局部变量，三个分支各自写回自己的结构体字段——
// 字段名在三个结构体里是一致的（ReadBufferSize/WriteBufferSize/ContextTimeoutEnabled/
// DisableIdentity），只是类型不同
//
// [EN] The three upstream parsers return three unrelated option structs that no single
// variable can hold, so the extension parameters are read into locals before the switch
// and each branch writes them onto its own struct; the field names are identical across
// the three types, only the types differ
func newClient(rawDSN string) (redis.UniversalClient, string, error) {
	// 摘掉网关扩展参数，剩下的查询串全部交给 go-redis 解析
	u, ext, err := dsnx.Split(rawDSN, extKeys...)
	if err != nil {
		return nil, "", err
	}

	// 扩展参数的默认值全部取零值：go-redis 自己会把零值补成官方默认
	// （ReadBufferSize/WriteBufferSize 补 32KB，见 options.go init），
	// 这里再写一份默认值只会和驱动的默认值两头漂
	//
	// [EN] Every extension parameter defaults to its zero value, which go-redis then
	// fills in with the upstream default (32KB for the buffer sizes, see options.go
	// init); duplicating those defaults here would only drift against the driver
	var (
		readBufferSize        = ext.Int(extReadBufferSize, 0)
		writeBufferSize       = ext.Int(extWriteBufferSize, 0)
		contextTimeoutEnabled = ext.Bool(extContextTimeoutEnabled, false)
		forceCluster          = ext.Bool(extCluster, false)
	)
	if err := ext.Err(); err != nil {
		return nil, "", err
	}

	driverDSN := u.String()
	query := u.Query()

	switch {
	// 哨兵模式：master_name 是哨兵模式的唯一标识，必须排在集群判定之前——
	// 哨兵 DSN 同样用重复的 addr 参数追加哨兵地址，顺序颠倒会把哨兵集群错认成数据集群
	//
	// [EN] Sentinel first: master_name is what identifies the mode, and a sentinel DSN
	// lists its extra sentinel addresses with the same repeated addr parameter, so
	// checking cluster first would mistake a sentinel set for a data cluster
	case query.Get("master_name") != "":
		opts, err := redis.ParseFailoverURL(driverDSN)
		if err != nil {
			return nil, "", fmt.Errorf("parse redis sentinel url failed: %w", err)
		}
		opts.DisableIdentity = true
		opts.ReadBufferSize = readBufferSize
		opts.WriteBufferSize = writeBufferSize
		opts.ContextTimeoutEnabled = contextTimeoutEnabled
		return redis.NewFailoverClient(opts), modeSentinel, nil

	// 集群模式：多地址自动判定，单地址（云厂商的集群代理端点）需显式 cluster=true
	case len(query["addr"]) > 0 || forceCluster:
		opts, err := redis.ParseClusterURL(driverDSN)
		if err != nil {
			return nil, "", fmt.Errorf("parse redis cluster url failed: %w", err)
		}
		opts.DisableIdentity = true
		opts.ReadBufferSize = readBufferSize
		opts.WriteBufferSize = writeBufferSize
		opts.ContextTimeoutEnabled = contextTimeoutEnabled
		return redis.NewClusterClient(opts), modeCluster, nil

	// 单机模式
	default:
		opts, err := redis.ParseURL(driverDSN)
		if err != nil {
			return nil, "", fmt.Errorf("parse redis url failed: %w", err)
		}
		// 禁用 CLIENT SETINFO，避免 maint_notifications 错误
		opts.DisableIdentity = true
		opts.ReadBufferSize = readBufferSize
		opts.WriteBufferSize = writeBufferSize
		opts.ContextTimeoutEnabled = contextTimeoutEnabled
		return redis.NewClient(opts), modeSingle, nil
	}
}

// redactDSN 抹掉 DSN 里的密码，只保留可用于定位问题的 scheme、用户名与地址
// 解析失败时返回固定占位串而不是原串，避免把密码写进日志
//
// [EN] redactDSN strips the password, keeping the scheme, username and address that are
// useful for diagnosis. A DSN that fails to parse yields a fixed placeholder rather than
// the original string, so a password never reaches the log
func redactDSN(rawDSN string) string {
	u, err := url.Parse(rawDSN)
	if err != nil {
		return "<unparsable dsn>"
	}
	if u.User != nil {
		if _, hasPwd := u.User.Password(); hasPwd {
			u.User = url.User(u.User.Username())
		}
	}
	return u.Redacted()
}
