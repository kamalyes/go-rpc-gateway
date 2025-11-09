/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 01:05:46
 * @FilePath: \go-rpc-gateway\server\lifecycle.go
 * @Description: 服务器生命周期管理模块，包括启动、停止等
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
)

// Start 启动服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logger := global.LOGGER

	if s.running {
		return fmt.Errorf("server is already running")
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

	s.running = true
	logger.InfoKV("🚀 Gateway启动成功!", 
		"http_host", s.config.Gateway.HTTP.Host,
		"http_port", s.config.Gateway.HTTP.Port,
		"grpc_host", s.config.Gateway.GRPC.Host,
		"grpc_port", s.config.Gateway.GRPC.Port)

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
		return fmt.Errorf("failed to stop server: %w", err)
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
