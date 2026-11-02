/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 11:52:56
 * @FilePath: \go-rpc-gateway\server\grpc.go
 * @Description: gRPC服务器初始化和启动模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	stderrors "errors"
	"net"
	"time"

	grpcpool "github.com/kamalyes/go-rpc-gateway/cpool/grpc"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// initGRPCServer 初始化gRPC服务器
// go-config 的 Default() 已经设置了所有默认值，无需再次设置
func (s *Server) initGRPCServer() error {
	// 配置已通过 safe.MergeWithDefaults 合并默认值
	grpcServer := s.config.GRPC.Server

	// 检查是否启用 gRPC 服务
	if !grpcServer.Enable {
		global.LOGGER.InfoMsg("gRPC服务未启用,跳过初始化")
		return nil
	}

	// 监控消息大小配置（仅警告，不限制）
	const (
		minMsgSize = 1024 * 1024       // 1MB
		maxMsgSize = 100 * 1024 * 1024 // 100MB
	)

	recvMsgSize := grpcServer.MaxRecvMsgSize
	if recvMsgSize < minMsgSize || recvMsgSize > maxMsgSize {
		global.LOGGER.WarnKV("gRPC MaxRecvMsgSize 超出推荐范围",
			"current", recvMsgSize,
			"min_recommended", minMsgSize,
			"max_recommended", maxMsgSize)
	}

	sendMsgSize := grpcServer.MaxSendMsgSize
	if sendMsgSize < minMsgSize || sendMsgSize > maxMsgSize {
		global.LOGGER.WarnKV("gRPC MaxSendMsgSize 超出推荐范围",
			"current", sendMsgSize,
			"min_recommended", minMsgSize,
			"max_recommended", maxMsgSize)
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(recvMsgSize),
		grpc.MaxSendMsgSize(sendMsgSize),
		// 注册 otelgrpc StatsHandler，在 gRPC 层自动提取/注入 OTel trace context
		// （grpc-trace-bin），使 OTel trace 跨 gRPC 调用连续传播，
		// 与 HTTP Tracing 中间件统一 trace_id，实现全链路打通
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}

	// 启用压缩编码器注册（必须在 grpc.NewServer 之前）
	if grpcServer.EnableCompression {
		grpcpool.ApplyServerCompression(grpcServer)
	}

	// 添加Keepalive配置
	if grpcServer.KeepaliveTime > 0 {
		keepalivePolicy := keepalive.ServerParameters{
			Time:    time.Duration(grpcServer.KeepaliveTime) * time.Second,
			Timeout: time.Duration(grpcServer.KeepaliveTimeout) * time.Second,
		}
		opts = append(opts, grpc.KeepaliveParams(keepalivePolicy))

		global.LOGGER.InfoKV("gRPC Keepalive配置已启用",
			"keepalive_time", grpcServer.KeepaliveTime,
			"keepalive_timeout", grpcServer.KeepaliveTimeout)
	}

	// 添加连接超时配置
	if grpcServer.ConnectionTimeout > 0 {
		keepaliveEnforcement := keepalive.EnforcementPolicy{
			MinTime:             time.Duration(grpcServer.ConnectionTimeout) * time.Second,
			PermitWithoutStream: true,
		}
		opts = append(opts, grpc.KeepaliveEnforcementPolicy(keepaliveEnforcement))

		global.LOGGER.InfoKV("gRPC连接超时配置已启用",
			"connection_timeout", grpcServer.ConnectionTimeout)
	}

	// 添加中间件拦截器链（按执行顺序）
	if s.middlewareManager != nil {
		// 构建 Unary 拦截器链
		var unaryInterceptors []grpc.UnaryServerInterceptor

		// 链路追踪拦截器（必须在 RequestContext 之前，与 HTTP 中间件顺序一致）
		// 原因：先创建 OTel span，后续 RequestContextInterceptor 的 extractOrGenerateTraceID
		// 才能从 span context 中提取 OTel trace_id，确保日志、gRPC metadata、OTel 后端
		// 使用同一个 trace_id，实现全链路打通
		if tracingInterceptor := s.middlewareManager.GRPCTracingInterceptor(); tracingInterceptor != nil {
			unaryInterceptors = append(unaryInterceptors, tracingInterceptor)
		}

		// RequestContext 注入（在 Tracing 之后，可从 OTel span 提取 trace_id）
		unaryInterceptors = append(unaryInterceptors, middleware.UnaryServerRequestContextInterceptor())

		// 日志记录
		unaryInterceptors = append(unaryInterceptors, middleware.UnaryServerLoggingInterceptor())

		// 添加 i18n 拦截器（如果启用国际化，在 RequestContext 之后注入 i18n context）
		if i18nInterceptor := s.middlewareManager.GRPCUnaryI18nInterceptor(); i18nInterceptor != nil {
			unaryInterceptors = append(unaryInterceptors, i18nInterceptor)
		}

		// 添加监控拦截器（如果启用）
		if metricsInterceptor := s.middlewareManager.GRPCMetricsInterceptor(); metricsInterceptor != nil {
			unaryInterceptors = append(unaryInterceptors, metricsInterceptor)
		}

		// 添加限流拦截器（如果启用，放在校验之前以尽早丢弃过载请求）
		if rlInterceptor := s.middlewareManager.GRPCRateLimitUnaryInterceptor(); rlInterceptor != nil {
			unaryInterceptors = append(unaryInterceptors, rlInterceptor)
		}

		// 添加自适应降载拦截器（如果启用，夹在限流与熔断之间）
		// 位置理由：
		//  1. 必须在限流之后——限流是确定性的配额判定，被它拒绝的请求根本不会触达 handler；
		//     放在降载之前，这些请求既不占用降载的飞行计数，也不会把「拒绝得极快」的
		//     响应时间掺进降载的 RT 滑窗，否则最小平均 RT 被压低、承载上限被低估，
		//     降载会在系统其实还有余量时就开始丢请求
		//  2. 必须在熔断之前（熔断在降载内层）——降载拒绝返回 codes.Unavailable，
		//     而 Unavailable 恰好在 IsBreakerFailureCode 的失败集合里；若降载挂在熔断内层，
		//     熔断器就会把降载的自我保护拒绝当成下游故障，形成
		//     「过载→降载拒绝→熔断打开」的正反馈：负载退潮后仍要等熔断超时才恢复，
		//     而且熔断是按路径的，一次全局过载会把所有路径的断路器一起打开
		//     放在外层后，本进程的熔断器永远观测不到降载拒绝
		//     （这与熔断自己放在限流之后、避免把 ResourceExhausted 当成下游故障，是同一类考虑）
		//  3. 语义上降载比熔断更该早拒绝：降载回答「本进程还扛不扛得住」，是全局自保；
		//     熔断回答「下游还活着吗」，需要尽量贴近 handler 才能观测到真实健康度
		//  4. 降载拒绝的 codes.Unavailable 仍会传给调用方，被调用方的熔断器观测到——
		//     那是期望的负反馈（上游退避 → 本进程过载缓解），与第 2 点要避免的
		//     「自己的拒绝被自己的熔断器计为下游故障」不是一回事
		if shedderInterceptor := s.middlewareManager.GRPCShedderUnaryInterceptor(); shedderInterceptor != nil {
			unaryInterceptors = append(unaryInterceptors, shedderInterceptor)
		}

		// 添加熔断拦截器（如果启用，紧跟限流之后、校验之前）
		// 位置理由：
		//  1. 必须在限流之后——限流拒绝返回 ResourceExhausted，若熔断在外层就会把
		//     这类自我保护的拒绝当成下游响应观测到；放在内层后熔断器永远看不到限流拒绝，
		//     不会出现「流量高峰→限流拒绝→熔断打开」的正反馈
		//  2. 必须在校验之前——熔断打开时的全部价值就是 fail fast，不该再为注定被拒绝的
		//     请求付反射校验的代价
		//  3. 必须尽量贴近 handler——熔断统计要反映 handler 的真实健康度，中间夹的层越少越好；
		//     这里唯一夹在中间的 struct tag 校验返回 InvalidArgument，已被判为客户端问题不计入失败
		if breakerInterceptor := s.middlewareManager.GRPCBreakerUnaryInterceptor(); breakerInterceptor != nil {
			unaryInterceptors = append(unaryInterceptors, breakerInterceptor)
		}

		// 添加 struct tag 参数校验拦截器（配合 protoc-go-inject-tag 生效）
		unaryInterceptors = append(unaryInterceptors, s.middlewareManager.GRPCStructTagValidatorInterceptor())

		// 添加压缩拦截器（如果启用压缩，在拦截器链末尾设置响应压缩）
		if grpcServer.EnableCompression {
			unaryInterceptors = append(unaryInterceptors, grpcpool.UnaryServerCompressionInterceptor(grpcpool.ResolveCompressType(grpcServer.CompressionType)))
		}

		opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))

		// 构建 Stream 拦截器链
		var streamInterceptors []grpc.StreamServerInterceptor

		// 链路追踪拦截器（必须在 RequestContext 之前，与 Unary 链和 HTTP 链顺序一致）
		if tracingStreamInterceptor := s.middlewareManager.GRPCTracingStreamInterceptor(); tracingStreamInterceptor != nil {
			streamInterceptors = append(streamInterceptors, tracingStreamInterceptor)
		}

		// RequestContext 注入（在 Tracing 之后）
		streamInterceptors = append(streamInterceptors, middleware.StreamServerRequestContextInterceptor())

		// 日志记录
		streamInterceptors = append(streamInterceptors, middleware.StreamServerLoggingInterceptor())

		// 添加限流拦截器（如果启用，放在校验之前以尽早丢弃过载请求）
		if rlStreamInterceptor := s.middlewareManager.GRPCRateLimitStreamInterceptor(); rlStreamInterceptor != nil {
			streamInterceptors = append(streamInterceptors, rlStreamInterceptor)
		}

		// 添加自适应降载拦截器（位置与理由同 Unary 链：限流之后、熔断之前）
		// 流只在建流时判定一次，且不占用飞行计数（见 middleware.Shedder.AllowStream）
		if shedderStreamInterceptor := s.middlewareManager.GRPCShedderStreamInterceptor(); shedderStreamInterceptor != nil {
			streamInterceptors = append(streamInterceptors, shedderStreamInterceptor)
		}

		// 添加熔断拦截器（位置与理由同 Unary 链：限流之后、校验之前、尽量贴近 handler）
		if breakerStreamInterceptor := s.middlewareManager.GRPCBreakerStreamInterceptor(); breakerStreamInterceptor != nil {
			streamInterceptors = append(streamInterceptors, breakerStreamInterceptor)
		}

		streamInterceptors = append(streamInterceptors, s.middlewareManager.GRPCStructTagValidatorStreamInterceptor())

		// 添加 i18n Stream 拦截器（如果启用国际化）
		if i18nStreamInterceptor := s.middlewareManager.GRPCStreamI18nInterceptor(); i18nStreamInterceptor != nil {
			streamInterceptors = append(streamInterceptors, i18nStreamInterceptor)
		}

		// 添加 Stream 压缩拦截器
		if grpcServer.EnableCompression {
			streamInterceptors = append(streamInterceptors, grpcpool.StreamServerCompressionInterceptor(grpcpool.ResolveCompressType(grpcServer.CompressionType)))
		}

		opts = append(opts, grpc.ChainStreamInterceptor(streamInterceptors...))
	}

	s.grpcServer = grpc.NewServer(opts...)

	// 启用反射
	if grpcServer.EnableReflection {
		reflection.Register(s.grpcServer)
		global.LOGGER.InfoMsg("gRPC反射服务已启用")
	}

	// 启用压缩
	if grpcServer.EnableCompression {
		grpcpool.ApplyServerCompression(grpcServer)
		compressType := grpcpool.ResolveCompressType(grpcServer.CompressionType)
		global.LOGGER.InfoKV("gRPC压缩已启用",
			"compression_type", compressType,
			"compression_level", grpcServer.CompressionLevel,
			"min_compress_size", grpcServer.MinCompressSize)
	}

	global.LOGGER.InfoKV("gRPC服务器初始化完成",
		"max_recv_size", grpcServer.MaxRecvMsgSize,
		"max_send_size", grpcServer.MaxSendMsgSize,
		"reflection_enabled", grpcServer.EnableReflection,
		"compression_enabled", grpcServer.EnableCompression)

	return nil
}

// startGRPCServer 启动gRPC服务器
func (s *Server) startGRPCServer() error {
	grpcServer := s.config.GRPC.Server

	// 检查是否启用 gRPC 服务
	if !grpcServer.Enable {
		global.LOGGER.InfoMsg("gRPC服务未启用,跳过启动")
		return nil
	}

	// 获取网络和地址配置
	grpcServerInstance := s.grpcServer
	if grpcServerInstance == nil {
		return nil
	}

	address := netx.JoinHostPort(grpcServer.Host, grpcServer.Port)

	listener, err := net.Listen(grpcServer.Network, address)
	if err != nil {
		return errors.NewErrorf(errors.ErrCodeGRPCConnectionFailed, "failed to listen on %s: %v", address, err)
	}

	global.LOGGER.InfoKV("Starting gRPC server", "address", address)
	if err := grpcServerInstance.Serve(listener); err != nil && !stderrors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

// stopGRPCServer 停止gRPC服务器
func (s *Server) stopGRPCServer() {
	grpcServerInstance := s.grpcServer
	if grpcServerInstance != nil {
		global.LOGGER.InfoContext(s.ctx, "Stopping gRPC server...")
		grpcServerInstance.GracefulStop()
		global.LOGGER.InfoContext(s.ctx, "gRPC server stopped")
	}
}
