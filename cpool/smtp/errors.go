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
	ErrSMTPConfigNil = errors.New("smtp configuration is nil")
	ErrSMTPHostEmpty = errors.New("smtp host is empty")
	ErrSMTPUserEmpty = errors.New("smtp user is empty")

	// 发送错误
	ErrSendFailed     = errors.New("failed to send email")
	ErrInvalidTo      = errors.New("invalid recipient address")
	ErrInvalidSubject = errors.New("invalid email subject")
	ErrInvalidBody    = errors.New("invalid email body")
)
