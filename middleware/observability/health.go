/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-10 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 10:52:57
 * @FilePath: \go-rpc-gateway\middleware\observability\health.go
 * @Description: 健康检查模块 - 支持Redis和PostgreSQL健康检查
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// HealthChecker 健康检查器接口
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) HealthStatus
}

// HealthStatus 健康状态
type HealthStatus struct {
	Status    string        `json:"status"`            // "ok", "warning", "error"
	Message   string        `json:"message,omitempty"` // 状态描述
	Latency   time.Duration `json:"latency_ms"`        // 延迟(毫秒)
	Details   interface{}   `json:"details,omitempty"` // 详细信息
	CheckedAt time.Time     `json:"checked_at"`        // 检查时间
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status    string                  `json:"status"`    // 整体状态
	Service   string                  `json:"service"`   // 服务名称
	Version   string                  `json:"version"`   // 版本
	Timestamp int64                   `json:"timestamp"` // 时间戳
	Uptime    time.Duration           `json:"uptime"`    // 运行时间
	BuildTime string                  `json:"buildTime"` // 构建时间
	BuildUser string                  `json:"buildUser"` // 构建用户
	GoVersion string                  `json:"goVersion"` // Go版本
	GitCommit string                  `json:"gitCommit"` // Git提交哈希
	GitBranch string                  `json:"gitBranch"` // Git分支
	GitTag    string                  `json:"gitTag"`    // Git标签
	Checks    map[string]HealthStatus `json:"checks"`    // 各组件检查结果
}

// RedisChecker Redis健康检查器
type RedisChecker struct {
	client    redis.UniversalClient
	timeout   time.Duration
	useGlobal bool
}

// NewRedisChecker 创建Redis健康检查器（使用全局Redis客户端）
func NewRedisChecker(timeout time.Duration) *RedisChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &RedisChecker{
		timeout:   timeout,
		useGlobal: true,
	}
}

// NewRedisCheckerWithClient 使用指定Redis客户端创建检查器
func NewRedisCheckerWithClient(client redis.UniversalClient, timeout time.Duration) *RedisChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &RedisChecker{
		client:    client,
		useGlobal: false,
	}
}

func (r *RedisChecker) Name() string {
	return "redis"
}

func (r *RedisChecker) Check(ctx context.Context) HealthStatus {
	start := time.Now()

	var client redis.UniversalClient
	if r.useGlobal {
		client = global.REDIS
	} else {
		client = r.client
	}

	// 检查Redis客户端是否存在
	if client == nil {
		return HealthStatus{
			Status:    "error",
			Message:   "Redis client is not available",
			Latency:   time.Since(start),
			CheckedAt: start,
		}
	}

	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// 执行PING命令
	pong, err := client.Ping(timeoutCtx).Result()
	latency := time.Since(start)

	if err != nil {
		return HealthStatus{
			Status:    "error",
			Message:   fmt.Sprintf("Redis ping failed: %v", err),
			Latency:   latency,
			CheckedAt: start,
		}
	}

	if pong != "PONG" {
		return HealthStatus{
			Status:    "warning",
			Message:   fmt.Sprintf("Unexpected Redis response: %s", pong),
			Latency:   latency,
			CheckedAt: start,
		}
	}

	// 检查延迟是否过高
	status := "ok"
	message := "Redis is healthy"
	if latency > 100*time.Millisecond {
		status = "warning"
		message = fmt.Sprintf("Redis latency is high: %v", latency)
	}

	return HealthStatus{
		Status:    status,
		Message:   message,
		Latency:   latency,
		CheckedAt: start,
		Details: map[string]interface{}{
			"response": pong,
		},
	}
}

// PostgreSQLChecker PostgreSQL健康检查器
type PostgreSQLChecker struct {
	db        *gorm.DB
	timeout   time.Duration
	useGlobal bool
}

// NewPostgreSQLChecker 创建PostgreSQL健康检查器（使用全局DB）
func NewPostgreSQLChecker(timeout time.Duration) *PostgreSQLChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &PostgreSQLChecker{
		timeout:   timeout,
		useGlobal: true,
	}
}

// NewPostgreSQLCheckerWithDB 使用现有 gorm.DB 连接创建PostgreSQL健康检查器
func NewPostgreSQLCheckerWithDB(db *gorm.DB, timeout time.Duration) *PostgreSQLChecker {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &PostgreSQLChecker{
		db:        db,
		timeout:   timeout,
		useGlobal: false,
	}
}

func (m *PostgreSQLChecker) Name() string {
	return "postgres"
}

func (m *PostgreSQLChecker) Check(ctx context.Context) HealthStatus {
	start := time.Now()

	var db *gorm.DB

	// 获取数据库连接
	if m.useGlobal {
		db = global.DB
	} else {
		db = m.db
	}

	// 检查连接是否存在
	if db == nil {
		return HealthStatus{
			Status:    "error",
			Message:   "PostgreSQL connection is not available",
			Latency:   time.Since(start),
			CheckedAt: start,
		}
	}

	// 创建带超时的上下文
	timeoutCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	// 执行ping检查
	sqlDB, err := db.DB()
	if err != nil {
		return HealthStatus{
			Status:    "error",
			Message:   fmt.Sprintf("PostgreSQL ping failed: %v", err),
			Latency:   time.Since(start),
			CheckedAt: start,
		}
	}
	err = sqlDB.PingContext(timeoutCtx)
	latency := time.Since(start)

	if err != nil {
		return HealthStatus{
			Status:    "error",
			Message:   fmt.Sprintf("PostgreSQL ping failed: %v", err),
			Latency:   latency,
			CheckedAt: start,
		}
	}

	// 连接池统计
	stat := sqlDB.Stats()

	// 检查延迟是否过高
	status := "ok"
	message := "PostgreSQL is healthy"
	if latency > 100*time.Millisecond {
		status = "warning"
		message = fmt.Sprintf("PostgreSQL latency is high: %v", latency)
	}

	details := map[string]interface{}{
		"max_open_conns":     stat.MaxOpenConnections,
		"open_connections":   stat.OpenConnections,
		"idle_connections":   stat.Idle,
		"in_use_connections": stat.InUse,
	}

	// 检查连接池状态
	if stat.MaxOpenConnections > 0 && stat.OpenConnections >= stat.MaxOpenConnections {
		status = "warning"
		message = "PostgreSQL connection pool is at maximum capacity"
	}

	return HealthStatus{
		Status:    status,
		Message:   message,
		Latency:   latency,
		CheckedAt: start,
		Details:   details,
	}
}

// HealthManager 健康检查管理器
type HealthManager struct {
	// mu 保护 checkers：RegisterChecker 通常在启动期调用，而 Check 由
	// HTTP 健康端点并发触发（负载均衡器会同时打多个副本/多次探测）。
	// 无锁 append 与遍历并发执行时，轻则读到正在扩容的旧底层数组，
	// 重则触发 runtime 的 slice 越界/数据竞争 —— 而这个端点正是探活路径，
	// 崩在这里会让整个实例被摘除
	// [EN] Guards checkers. Registration happens during startup while Check runs
	// concurrently from the HTTP health endpoint (load balancers probe repeatedly), so an
	// unlocked append can race with the range: at best a stale backing array, at worst a
	// data race on the probe path that gets the whole instance pulled out of rotation
	mu        sync.RWMutex
	checkers  []HealthChecker
	startTime time.Time
}

// NewHealthManager 创建健康检查管理器
func NewHealthManager() *HealthManager {
	return &HealthManager{
		checkers:  make([]HealthChecker, 0),
		startTime: time.Now(),
	}
}

// RegisterChecker 注册健康检查器
// nil 直接忽略：否则 Check 在遍历时会对它调用 Check/Name，
// 把一个可选的注册失误变成健康端点的空指针崩溃
// [EN] nil is ignored; otherwise Check would call Check/Name on it and turn an optional
// registration mistake into a nil dereference on the health endpoint
func (h *HealthManager) RegisterChecker(checker HealthChecker) {
	if h == nil || checker == nil {
		return
	}
	h.mu.Lock()
	h.checkers = append(h.checkers, checker)
	h.mu.Unlock()
}

// snapshotCheckers 返回当前检查器列表的副本
// 不在执行检查时持锁：checker.Check 可能走网络（Redis/PostgreSQL），
// 持锁会让一次慢检查阻塞后续注册，也会把并发探测串行化
// [EN] Returns a copy instead of holding the lock while checking: Check may do network I/O
// (Redis/PostgreSQL), and holding the lock would block later registrations and serialize
// concurrent probes
func (h *HealthManager) snapshotCheckers() []HealthChecker {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.checkers) == 0 {
		return nil
	}
	out := make([]HealthChecker, len(h.checkers))
	copy(out, h.checkers)
	return out
}

// Check 执行健康检查
func (h *HealthManager) Check(ctx context.Context, detailed bool) HealthCheckResult {
	// 使用全局配置
	//
	// 走 GetGatewayConfig 并补 nil 保护：原先直接 `cfg := global.GATEWAY` 再读 cfg.Name，
	// 关停流程把配置置空之后，这个端点就会 panic —— 而它恰好是负载均衡在探的那个
	//
	// [EN] GetGatewayConfig plus a nil guard: this used to read cfg.Name straight off
	// global.GATEWAY, so once shutdown cleared the config this endpoint panicked — and it is
	// precisely the one the load balancer is probing.
	cfg := global.GetGatewayConfig()
	if cfg == nil {
		cfg = &gwconfig.Gateway{}
	}
	result := HealthCheckResult{
		Service:   cfg.Name,
		Version:   cfg.Version,
		Timestamp: time.Now().Unix(),
		Uptime:    time.Since(h.startTime),
		BuildTime: cfg.BuildTime,
		BuildUser: cfg.BuildUser,
		GoVersion: cfg.GoVersion,
		GitCommit: cfg.GitCommit,
		GitBranch: cfg.GitBranch,
		GitTag:    cfg.GitTag,
		Checks:    make(map[string]HealthStatus),
	}

	if !detailed {
		result.Status = "ok"
		return result
	}

	// 执行所有检查器
	overallStatus := "ok"

	for _, checker := range h.snapshotCheckers() {
		status := checker.Check(ctx)
		result.Checks[checker.Name()] = status

		// 更新整体状态
		switch status.Status {
		case "error":
			overallStatus = "error"
		case "warning":
			if overallStatus == "ok" {
				overallStatus = "warning"
			}
		}
	}

	result.Status = overallStatus
	return result
}

// HTTPHandler 创建HTTP健康检查处理器
func (h *HealthManager) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		detailed := r.URL.Query().Get("detail") == "true"

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result := h.Check(ctx, detailed)

		w.Header().Set("Content-Type", "application/json")

		// 根据整体状态设置HTTP状态码
		switch result.Status {
		case "ok", "warning":
			w.WriteHeader(http.StatusOK)
		case "error":
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		if err := json.NewEncoder(w).Encode(result); err != nil {
			global.LOGGER.ErrorKV("Failed to encode health check response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}
