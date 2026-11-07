/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 15:06:55
 * @FilePath: \go-rpc-gateway\server\middleware_init.go
 * @Description: 中间件管理器初始化模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// initMiddleware 初始化中间件管理器
func (s *Server) initMiddleware() error {
	// 使用统一的配置系统创建中间件管理器
	manager, err := middleware.NewManager(s.config)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeMiddlewareInitFailed)
	}
	s.middlewareManager = manager

	// 初始化健康检查管理器
	if err := s.initHealthManager(); err != nil {
		return errors.Wrap(err, errors.ErrCodeHealthManagerFailed)
	}

	return nil
}

// initHealthManager 初始化健康检查管理器
func (s *Server) initHealthManager() error {
	// 配置已通过 safe.MergeWithDefaults 合并默认值
	healthManager := middleware.NewHealthManager()

	// 添加Redis健康检查
	if s.config.Health.Redis.Enabled {
		timeout := time.Duration(s.config.Health.Redis.Timeout) * time.Second
		redisChecker := middleware.NewRedisChecker(timeout)
		healthManager.RegisterChecker(redisChecker)
	}

	// 添加PostgreSQL健康检查（配置键沿用 health.mysql 保持配置兼容）
	if s.config.Health.MySQL.Enabled {
		timeout := time.Duration(s.config.Health.MySQL.Timeout) * time.Second
		pgChecker := middleware.NewPostgreSQLChecker(timeout)
		healthManager.RegisterChecker(pgChecker)
	}

	s.healthManager = healthManager
	return nil
}

// initServers 初始化服务器组件
func (s *Server) initServers() error {
	// 初始化gRPC服务器
	if err := s.initGRPCServer(); err != nil {
		return errors.Wrap(err, errors.ErrCodeGRPCServerInitFailed)
	}

	// 初始化HTTP网关
	if err := s.initHTTPGateway(); err != nil {
		return errors.Wrap(err, errors.ErrCodeHTTPGatewayInitFailed)
	}

	// 初始化命名监听器（多端口支持）
	if err := s.initNamedListeners(); err != nil {
		global.LOGGER.WithError(err).WarnMsg("命名监听器初始化失败，将跳过启动")
	}

	// 注入扩展指标采集函数（连接池/熔断器）
	// 在所有组件初始化完成后调用，实现按需采集业务指标
	s.injectMetricsCollectors()

	return nil
}

// injectMetricsCollectors 注入扩展指标采集函数到 MetricsManager
// 采集函数在 Prometheus scrape 时按需调用，避免后台 goroutine 轮询
func (s *Server) injectMetricsCollectors() {
	if s.middlewareManager == nil {
		return
	}
	mm := s.middlewareManager.GetMetricsManager()
	if mm == nil {
		return
	}

	// 注入连接池健康检查采集函数
	if s.poolManager != nil {
		poolMgr := s.poolManager
		mm.SetPoolHealthFn(func() map[string]bool {
			return poolMgr.HealthCheck()
		})
	}

	// 注入熔断器统计采集函数
	if bm := s.middlewareManager.GetBreakerManager(); bm != nil {
		breakerMgr := bm
		mm.SetBreakerStatsFn(func() []middleware.BreakerStat {
			snapshots := breakerMgr.GetAllBreakerSnapshots()
			stats := make([]middleware.BreakerStat, 0, len(snapshots))
			for path, snap := range snapshots {
				stats = append(stats, middleware.BreakerStat{
					Path:           path,
					State:          string(snap.State),
					TotalRequests:  snap.TotalRequests,
					FailedRequests: snap.FailedRequests,
					FailureCount:   snap.FailureCount,
					SuccessCount:   snap.SuccessCount,
				})
			}
			return stats
		})
	}

	// 注册降载指标采集器（启用降载时才注册，未启用时指标名不会出现在注册表里）
	if sm := s.middlewareManager.GetShedderManager(); sm != nil {
		shedderMgr := sm
		if err := mm.RegisterShedderCollector(func() *middleware.ShedderStat {
			snap := shedderMgr.Snapshot()
			return &middleware.ShedderStat{
				Shedding:        snap.Shedding,
				CPUUsage:        snap.CPUUsage,
				CPUSupported:    snap.CPUSupported,
				CPUThreshold:    snap.CPUThreshold,
				InFlight:        snap.InFlight,
				AvgFlying:       snap.AvgFlying,
				MaxFlight:       snap.MaxFlight,
				MaxPass:         snap.MaxPass,
				MinRTMicros:     snap.MinRTMicros,
				MaxConcurrency:  snap.MaxConcurrency,
				TotalRequests:   snap.TotalRequests,
				DroppedRequests: snap.DroppedRequests,
			}
		}); err != nil {
			// 重复注册（热加载重入）不影响服务，降级为告警
			global.LOGGER.WithError(err).WarnMsg("降载指标采集器注册失败")
		}
	}

	global.LOGGER.InfoMsg("✅ 扩展指标采集函数已注入（连接池/熔断器/降载器）")
}
