/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 15:02:34
 * @FilePath: \go-rpc-gateway\server\lifecycle.go
 * @Description: 服务器生命周期管理模块，包括启动、停止等
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Start 启动服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger := global.LOGGER

	if s.running {
		return errors.NewError(errors.ErrCodeServiceUnavailable, "server is already running")
	}

	// 启动gRPC服务器
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.startGRPCServer(); err != nil {
			logger.WithError(err).ErrorMsg("gRPC server failed")
		}
	}()

	// 等待gRPC服务器启动
	time.Sleep(100 * time.Millisecond)

	// 启动HTTP服务器
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.startHTTPServer(); err != nil {
			logger.WithError(err).ErrorMsg("HTTP server failed")
		}
	}()

	// 启动 WebSocket 服务（如果已初始化）
	if s.webSocketService != nil {
		if err := s.webSocketService.Start(); err != nil {
			logger.WithError(err).WarnMsg("WebSocket service failed to start")
			// 不中断整个系统启动
		}
	}

	s.running = true

	// 使用安全访问获取端点信息
	httpHost := s.configSafe.Field("HTTPServer").Field("Host").String("0.0.0.0")
	httpPort := s.configSafe.Field("HTTPServer").Field("Port").Int(8080)
	grpcHost := s.configSafe.Field("GRPC").Field("Server").Field("Host").String("0.0.0.0")
	grpcPort := s.configSafe.Field("GRPC").Field("Server").Field("Port").Int(9090)

	endpointMsg := fmt.Sprintf("http://%s:%d, grpc://%s:%d", httpHost, httpPort, grpcHost, grpcPort)
	if s.webSocketService != nil && s.webSocketService.IsRunning() {
		wsHost := s.webSocketService.GetConfig().NodeIP
		wsPort := s.webSocketService.GetConfig().NodePort
		endpointMsg += fmt.Sprintf(", ws://%s:%d", wsHost, wsPort)
	}

	logger.InfoKV("🚀 Gateway启动成功!",
		"endpoints", endpointMsg)

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger := global.LOGGER

	if !s.running {
		return nil
	}

	logger.InfoMsg("Stopping Gateway server...")

	// 取消上下文
	s.cancel()

	// 停止 WebSocket 服务
	if s.webSocketService != nil {
		if err := s.webSocketService.Stop(); err != nil {
			logger.WithError(err).WarnMsg("Failed to stop WebSocket service")
		}
	}

	// 停止HTTP服务器
	if err := s.stopHTTPServer(); err != nil {
		logger.WithError(err).ErrorMsg("Failed to stop HTTP server")
	}

	// 停止gRPC服务器
	s.stopGRPCServer()

	// 等待所有goroutine结束
	s.wg.Wait()

	s.running = false
	logger.InfoMsg("Gateway server stopped")

	return nil
}

// Restart 重启服务器
func (s *Server) Restart() error {
	if err := s.Stop(); err != nil {
		return errors.NewErrorf(errors.ErrCodeInternalServerError, "failed to stop server: %v", err)
	}

	// 等待完全停止
	time.Sleep(1 * time.Second)

	return s.Start()
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown() error {
	return s.Stop()
}

// IsRunning 检查服务器是否运行中
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Wait 等待服务器运行
func (s *Server) Wait() {
	s.wg.Wait()
}

// WaitForShutdown 等待关闭信号并优雅关闭服务器
func (s *Server) WaitForShutdown() error {
	logger := global.LOGGER

	// 等待系统信号进行优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger.InfoMsg("🎯 服务器运行中，按 Ctrl+C 优雅关闭")
	<-quit

	logger.InfoMsg("🛑 收到关闭信号，开始优雅关闭...")

	// 优雅关闭
	if err := s.Shutdown(); err != nil {
		logger.WithError(err).ErrorMsg("Failed to shutdown server gracefully")
		return err
	}

	logger.InfoMsg("✅ 服务器已优雅关闭")
	return nil
}

// Run 启动服务器并等待信号进行优雅关闭（一键启动）
// 这是最简单的启动方式，使用者只需要调用这一个方法即可
func (s *Server) Run() error {
	logger := global.LOGGER

	// 启动服务器
	if err := s.Start(); err != nil {
		logger.WithError(err).ErrorMsg("Failed to start server")
		return err
	}

	// 等待关闭信号
	return s.WaitForShutdown()
}
