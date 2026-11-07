/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 17:00:00
 * @FilePath: \go-rpc-gateway\cpool\smtp\errors.go
 * @Description: SMTP错误定义
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package smtp

import "errors"

var (
	// 配置错误
	ErrSMTPConfigNil  = errors.New("smtp configuration is nil")
	ErrSMTPDSNEmpty   = errors.New("smtp dsn is empty")
	ErrSMTPDSNInvalid = errors.New("smtp dsn is invalid")
	ErrSMTPHostEmpty  = errors.New("smtp dsn has no host")
	// DSN 不带 userinfo 不再是错误：本地中继（smtp://127.0.0.1:25）本就无需认证，
	// 原先的 ErrSMTPUserEmpty 会把这种合法配置挡在初始化之外，已随单 DSN 改造移除
	//
	// [EN] A DSN without userinfo is no longer an error: a local relay needs no
	// authentication, and the former ErrSMTPUserEmpty rejected that legitimate setup

	// 发送错误
	ErrSendFailed     = errors.New("failed to send email")
	ErrInvalidTo      = errors.New("invalid recipient address")
	ErrInvalidSubject = errors.New("invalid email subject")
	ErrInvalidBody    = errors.New("invalid email body")
)
