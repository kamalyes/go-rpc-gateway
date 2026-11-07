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
	"time"

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
func (h *HealthManager) RegisterChecker(checker HealthChecker) {
	h.checkers = append(h.checkers, checker)
}

// Check 执行健康检查
func (h *HealthManager) Check(ctx context.Context, detailed bool) HealthCheckResult {
	// 使用全局配置
	cfg := global.GATEWAY
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

	for _, checker := range h.checkers {
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
