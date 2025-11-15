/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-15 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 12:00:00
 * @FilePath: \go-rpc-gateway\server\wsc_feature.go
 * @Description: WebSocket通信功能集成 - 桥接wsc包
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package server

import (
	wscconfig "github.com/kamalyes/go-config/pkg/wsc"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-rpc-gateway/wsc"
)

// wscMiddleware WSC中间件实例（全局单例）
var wscMiddleware *middleware.WSCMiddleware

// EnableWSC 启用WebSocket通信功能（使用配置中的设置）
func (s *Server) EnableWSC() error {
	if s.config.WSC != nil && s.config.WSC.Enabled {
		return s.EnableWSCWithConfig(s.config.WSC)
	}

	global.LOGGER.Info("ℹ️  WSC功能未在配置中启用")
	return nil
}

// EnableWSCWithConfig 使用自定义配置启用WebSocket通信功能
func (s *Server) EnableWSCWithConfig(config interface{}) error {
	wscConfig, ok := config.(*wscconfig.WSC)
	if !ok {
		return errors.ErrInvalidConfigType.WithDetails("expected *wscconfig.WSC")
	}

	if !wscConfig.Enabled {
		global.LOGGER.Info("ℹ️  WSC功能已禁用（wsc.enabled=false）")
		return nil
	}

	// 创建WSC中间件
	wscMiddleware = middleware.NewWSCMiddleware(&middleware.WSCConfig{
		Config: wscConfig,
	})

	if !wscMiddleware.IsEnabled() {
		return errors.ErrMiddlewareInitFailed.WithDetails("WSC middleware initialization failed")
	}

	// 注册路由到HTTP Mux
	if err := wscMiddleware.RegisterRoutes(s.httpMux); err != nil {
		return errors.Wrapf(err, errors.ErrCodeWSCRouteFailed, "failed to register WSC routes")
	}

	global.LOGGER.Info("✅ WSC功能已启用 [NodeID=%s]", wscMiddleware.GetAdapter().GetNodeID())
	return nil
}

// ==================== 类型别名 - 向后兼容 ====================

// WSCAdapter 类型别名，指向wsc包的WSCAdapter
type WSCAdapter = wsc.WSCAdapter

// HubMessage 类型别名，指向wsc包的HubMessage  
type HubMessage = wsc.HubMessage

// UserConnectionInfo 类型别名，指向wsc包的UserConnectionInfo
type UserConnectionInfo = wsc.UserConnectionInfo

// UserInfoExtractor 类型别名，指向wsc包的UserInfoExtractor
type UserInfoExtractor = wsc.UserInfoExtractor

// WSCBuiltinAPI 类型别名，指向wsc包的WSCBuiltinAPI
type WSCBuiltinAPI = wsc.WSCBuiltinAPI

// WSCBuiltinAPIConfig 类型别名，指向wsc包的WSCBuiltinAPIConfig
type WSCBuiltinAPIConfig = wsc.WSCBuiltinAPIConfig

// ==================== 工厂函数 - 向后兼容 ====================

// NewWSCAdapter 创建WSC适配器（向后兼容）
func NewWSCAdapter(config *wscconfig.WSC) *WSCAdapter {
	return wsc.NewWSCAdapter(config)
}

// NewUserInfoExtractor 创建用户信息提取器（向后兼容）
func NewUserInfoExtractor() *UserInfoExtractor {
	return wsc.NewUserInfoExtractor()
}

// NewWSCBuiltinAPI 创建WSC内置API（向后兼容）
func NewWSCBuiltinAPI(adapter *WSCAdapter, config *WSCBuiltinAPIConfig) *WSCBuiltinAPI {
	return wsc.NewWSCBuiltinAPI(adapter, config)
}

// DefaultWSCBuiltinAPIConfig 获取默认内置API配置（向后兼容）
func DefaultWSCBuiltinAPIConfig() *WSCBuiltinAPIConfig {
	return wsc.DefaultWSCBuiltinAPIConfig()
}

// EnableWSCWithCallbacks 使用自定义回调启用WebSocket通信功能
func (s *Server) EnableWSCWithCallbacks(callbacks *middleware.WSCCallbacks) error {
	if s.config.WSC == nil || !s.config.WSC.Enabled {
		return errors.ErrWSCNotEnabled
	}

	// 创建带回调的WSC中间件
	wscMiddleware = middleware.NewWSCMiddleware(&middleware.WSCConfig{
		Config:    s.config.WSC,
		Callbacks: callbacks,
	})

	if !wscMiddleware.IsEnabled() {
		return errors.ErrMiddlewareInitFailed.WithDetails("WSC middleware initialization failed with callbacks")
	}

	// 注册路由到HTTP Mux
	if err := wscMiddleware.RegisterRoutes(s.httpMux); err != nil {
		return errors.WrapWithContext(err, errors.ErrCodeWSCRouteFailed)
	}

	global.LOGGER.Info("✅ WSC功能已启用（带自定义回调）[NodeID=%s]", 
		wscMiddleware.GetAdapter().GetNodeID())
	return nil
}

// GetWSCMiddleware 获取WSC中间件实例
func (s *Server) GetWSCMiddleware() *middleware.WSCMiddleware {
	return wscMiddleware
}

// IsWSCEnabled 检查WSC是否已启用
func (s *Server) IsWSCEnabled() bool {
	return wscMiddleware != nil && wscMiddleware.IsEnabled()
}
