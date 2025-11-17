/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 17:00:00
 * @FilePath: \go-rpc-gateway\cpool\smtp\smtp.go
 * @Description: SMTP邮件发送客户端
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package smtp

import (
	"context"
	"fmt"
	"net/smtp"

	"github.com/jordan-wright/email"
	smtpconfig "github.com/kamalyes/go-config/pkg/smtp"
	"github.com/kamalyes/go-logger"
)

// MailHandler SMTP邮件处理接口
type MailHandler interface {
	SendEmail(ctx context.Context, to []string, subject, body string) error
	SendEmailWithHTML(ctx context.Context, to []string, subject, htmlBody string) error
	Close() error
}

// SmtpClient SMTP客户端实现
type SmtpClient struct {
	config *smtpconfig.Smtp
	logger logger.ILogger
}

// NewSmtpClient 创建SMTP客户端
func NewSmtpClient(cfg *smtpconfig.Smtp, log logger.ILogger) (*SmtpClient, error) {
	if cfg == nil {
		return nil, ErrSMTPConfigNil
	}
	if cfg.SMTPHost == "" {
		return nil, ErrSMTPHostEmpty
	}
	if cfg.Username == "" {
		return nil, ErrSMTPUserEmpty
	}

	if log != nil {
		log.Info("SMTP client initialized: %s:%d", cfg.SMTPHost, cfg.SMTPPort)
	}

	return &SmtpClient{config: cfg, logger: log}, nil
}

// send 通用发送邮件方法
func (s *SmtpClient) send(em *email.Email) error {
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)
	auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.SMTPHost)
	return em.Send(addr, auth)
}

// SendEmail 发送纯文本邮件
func (s *SmtpClient) SendEmail(ctx context.Context, to []string, subject, body string) error {
	em := email.NewEmail()
	em.From = s.config.FromAddress
	if em.From == "" {
		em.From = s.config.Username
	}
	em.To = to
	em.Subject = subject
	em.Text = []byte(body)

	if s.logger != nil {
		s.logger.InfoKV("Sending email", "to", to, "subject", subject)
	}

	if err := s.send(em); err != nil {
		if s.logger != nil {
			s.logger.ErrorKV("Send email failed", "error", err, "to", to)
		}
		return err
	}
	return nil
}

// SendEmailWithHTML 发送HTML邮件
func (s *SmtpClient) SendEmailWithHTML(ctx context.Context, to []string, subject, htmlBody string) error {
	em := email.NewEmail()
	em.From = s.config.FromAddress
	if em.From == "" {
		em.From = s.config.Username
	}
	em.To = to
	em.Subject = subject
	em.HTML = []byte(htmlBody)

	if s.logger != nil {
		s.logger.InfoKV("Sending HTML email", "to", to, "subject", subject)
	}

	if err := s.send(em); err != nil {
		if s.logger != nil {
			s.logger.ErrorKV("Send HTML email failed", "error", err, "to", to)
		}
		return err
	}
	return nil
}

// Close 关闭连接
func (s *SmtpClient) Close() error {
	if s.logger != nil {
		s.logger.Info("SMTP client closed")
	}
	return nil
}

// NewMail 根据配置创建邮件客户端
func NewMail(cfg *smtpconfig.Smtp, log logger.ILogger) (MailHandler, error) {
	return NewSmtpClient(cfg, log)
}
