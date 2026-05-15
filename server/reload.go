/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-15 13:13:21
 * @FilePath: \go-rpc-gateway\server\reload.go
 * @Description: 服务器配置重新加载功能模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package server

import (
	"context"
	"time"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"google.golang.org/grpc"
)

// ApplyConfig 更新服务器内存中的网关配置
func (s *Server) ApplyConfig(cfg *gwconfig.Gateway) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = cfg
	if s.bannerManager != nil {
		s.bannerManager = NewBannerManager(cfg).WithContext(s.ctx)
	}
}

// ReloadHTTPGateway 重新构建并可选重启HTTP网关运行时
func (s *Server) ReloadHTTPGateway(cfg *gwconfig.Gateway, replay func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasRunning := s.running
	if wasRunning {
		if err := s.stopHTTPServer(); err != nil {
			return err
		}
	}

	s.config = cfg
	if s.middlewareManager != nil {
		if err := s.middlewareManager.UpdateConfig(cfg); err != nil {
			return err
		}
	}
	s.initGzipWriterPool()
	s.initDataMasker()

	if err := s.initHTTPGateway(); err != nil {
		return err
	}
	if replay != nil {
		if err := replay(); err != nil {
			return err
		}
	}

	if wasRunning {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := s.startHTTPServer(); err != nil {
				global.LOGGER.WithError(err).ErrorMsg("HTTP server failed")
			}
		}()
	}

	return nil
}

// ReloadGRPCServer 重新构建并可选重启gRPC服务器运行时
func (s *Server) ReloadGRPCServer(cfg *gwconfig.Gateway, registrars []func(*grpc.Server)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasRunning := s.running
	if wasRunning {
		s.stopGRPCServer()
	}

	s.config = cfg
	if s.middlewareManager != nil {
		if err := s.middlewareManager.UpdateConfig(cfg); err != nil {
			return err
		}
	}

	s.grpcServer = nil
	if err := s.initGRPCServer(); err != nil {
		return err
	}

	for _, register := range registrars {
		if register != nil {
			s.RegisterGRPCService(register)
		}
	}

	if wasRunning && cfg.GRPC != nil && cfg.GRPC.Server != nil && cfg.GRPC.Server.Enable {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := s.startGRPCServer(); err != nil {
				global.LOGGER.WithError(err).ErrorMsg("gRPC server failed")
			}
		}()
	}

	return nil
}

// ReloadPProfServer 重新构建并可选重启独立的pprof服务器
func (s *Server) ReloadPProfServer(cfg *gwconfig.Gateway) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pprofServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := s.pprofServer.Shutdown(ctx)
		cancel()
		if err != nil {
			return err
		}
		s.pprofServer = nil
	}

	s.config = cfg
	if !s.running || cfg.Middleware == nil || cfg.Middleware.PProf == nil || !cfg.Middleware.PProf.Enabled {
		return nil
	}

	s.pprofServer = middleware.NewPProfServer(cfg.Middleware.PProf)
	s.wg.Add(1)
	go func(pprofServer *middleware.PProfServer) {
		defer s.wg.Done()
		if err := pprofServer.Start(); err != nil {
			global.LOGGER.WithError(err).WarnMsg("PProf server failed to start")
		}
	}(s.pprofServer)

	return nil
}
