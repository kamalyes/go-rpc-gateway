/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 17:00:00
 * @FilePath: \go-rpc-gateway\cpool\smtp\smtp.go
 * @Description: SMTP邮件发送客户端（单 DSN）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"net/url"

	"github.com/jordan-wright/email"
	smtpconfig "github.com/kamalyes/go-rpc-gateway/config/smtp"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// 传输加密方式，由 DSN 的 scheme 与 starttls 参数共同决定
//
// [EN] The transport encryption mode, decided by the DSN's scheme together with its
// starttls parameter
type transportMode int

const (
	// transportPlain smtp:// 且未带 starttls：走 net/smtp 的默认投递，
	// 服务端声明支持 STARTTLS 就自动升级，不支持就明文
	//
	// [EN] Plain smtp:// with no starttls parameter: net/smtp's default delivery, which
	// upgrades opportunistically when the server advertises STARTTLS and stays plaintext
	// when it does not
	transportPlain transportMode = iota

	// transportStartTLS smtp:// 且 starttls=true：强制 STARTTLS，使用网关提供的 TLS 配置
	// （这条路径才吃得到 skip_verify）
	//
	// [EN] smtp:// with starttls=true: STARTTLS is required and uses the TLS config the
	// gateway supplies, which is the only path where skip_verify has any effect
	transportStartTLS

	// transportImplicitTLS smtps://：连上即 TLS，端口通常 465
	//
	// [EN] smtps://: TLS from the first byte, conventionally on port 465
	transportImplicitTLS
)

// MailHandler SMTP邮件处理接口
type MailHandler interface {
	SendEmail(ctx context.Context, to []string, subject, body string) error
	SendEmailWithHTML(ctx context.Context, to []string, subject, htmlBody string) error
	Close() error
}

// SmtpClient SMTP客户端实现
//
// DSN 在构造时解析一次，解析结果（地址、认证、传输方式、TLS 配置）存在字段上，
// 不在每次发信时重新解析
//
// [EN] The DSN is parsed once at construction and the result — address, auth, transport
// mode and TLS config — is kept on the struct rather than re-parsed on every send
type SmtpClient struct {
	config *smtpconfig.Smtp
	logger logger.ILogger

	addr      string
	auth      smtp.Auth
	transport transportMode
	tlsConfig *tls.Config
}

// NewSmtpClient 创建SMTP客户端
func NewSmtpClient(cfg *smtpconfig.Smtp, log logger.ILogger) (*SmtpClient, error) {
	if cfg == nil {
		return nil, ErrSMTPConfigNil
	}
	if cfg.DSN == "" {
		return nil, ErrSMTPDSNEmpty
	}

	client := &SmtpClient{config: cfg, logger: log}
	if err := client.parseDSN(cfg.DSN); err != nil {
		return nil, err
	}

	if log != nil {
		ctx := context.Background()
		log.InfoContextKV(ctx, "SMTP client initialized", "addr", client.addr, "transport", client.transportName())
	}

	return client, nil
}

// parseDSN 解析 DSN 为地址、认证与传输加密方式
func (s *SmtpClient) parseDSN(dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrSMTPDSNInvalid, err)
	}

	host := u.Hostname()
	if host == "" {
		return ErrSMTPHostEmpty
	}

	// 端口按 scheme 取默认值：smtp 提交端口 587，隐式 TLS 端口 465
	port := u.Port()
	if port == "" {
		if u.Scheme == "smtps" {
			port = "465"
		} else {
			port = "587"
		}
	}
	s.addr = host + ":" + port

	// 认证信息来自 userinfo；没写用户名就不带认证（本地中继的常见形态）
	// 用户名里的 @ 要在 DSN 里写成 %40，url.Parse 会还原回来
	//
	// [EN] Credentials come from the userinfo; with no username the client authenticates
	// not at all, which is how a local relay is usually reached. An @ inside the username
	// is written %40 in the DSN and url.Parse restores it
	if u.User != nil && u.User.Username() != "" {
		password, _ := u.User.Password()
		s.auth = smtp.PlainAuth("", u.User.Username(), password, host)
	}

	query := u.Query()
	skipVerify := query.Get("skip_verify") == "true"
	forceStartTLS := query.Get("starttls") == "true"

	switch {
	case u.Scheme == "smtps":
		s.transport = transportImplicitTLS
	case u.Scheme == "smtp" && forceStartTLS:
		s.transport = transportStartTLS
	case u.Scheme == "smtp":
		s.transport = transportPlain
	default:
		return fmt.Errorf("%w: scheme %q", ErrSMTPDSNInvalid, u.Scheme)
	}

	// TLS 的 ServerName 必须是 DSN 里的主机名，不能留空：留空时 crypto/tls 会拿
	// 连接地址去校验，自建邮件服务器的证书常常只签了对外域名，会校验失败
	//
	// [EN] The TLS ServerName must be the host from the DSN; left empty, crypto/tls
	// verifies against the dialed address, which fails on self-hosted mail servers whose
	// certificate only carries the public domain name
	if s.transport != transportPlain {
		s.tlsConfig = &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: skipVerify,
		}
	}

	return nil
}

// transportName 返回传输加密方式的名称，用于日志
func (s *SmtpClient) transportName() string {
	switch s.transport {
	case transportImplicitTLS:
		return "implicit-tls"
	case transportStartTLS:
		return "starttls"
	default:
		return "plain"
	}
}

// send 通用发送邮件方法，按 DSN 解析出的传输方式选择投递路径
func (s *SmtpClient) send(em *email.Email) error {
	switch s.transport {
	case transportImplicitTLS:
		return em.SendWithTLS(s.addr, s.auth, s.tlsConfig)
	case transportStartTLS:
		return em.SendWithStartTLS(s.addr, s.auth, s.tlsConfig)
	default:
		return em.Send(s.addr, s.auth)
	}
}

// newEmail 按配置构造邮件骨架，统一处理发件人与自定义头部
// 发件人留空时回落到 DSN 里的用户名——多数邮件服务商要求发件人与认证账号一致
//
// [EN] newEmail builds the message skeleton, applying the sender and the configured custom
// headers. An empty sender falls back to the DSN's username, since most providers require
// the From address to match the authenticated account
func (s *SmtpClient) newEmail(to []string, subject string) *email.Email {
	em := email.NewEmail()
	em.From = s.config.FromAddress
	if em.From == "" {
		if u, err := url.Parse(s.config.DSN); err == nil && u.User != nil {
			em.From = u.User.Username()
		}
	}
	em.To = to
	em.Subject = subject

	for key, value := range s.config.Headers {
		em.Headers.Set(key, value)
	}

	return em
}

// SendEmail 发送纯文本邮件
func (s *SmtpClient) SendEmail(ctx context.Context, to []string, subject, body string) error {
	em := s.newEmail(to, subject)
	em.Text = []byte(body)

	if s.logger != nil {
		s.logger.InfoContextKV(ctx, "Sending email", "to", to, "subject", subject)
	}

	if err := s.send(em); err != nil {
		if s.logger != nil {
			s.logger.ErrorContextKV(ctx, "Send email failed", "error", err, "to", to)
		}
		return err
	}
	return nil
}

// SendEmailWithHTML 发送HTML邮件
func (s *SmtpClient) SendEmailWithHTML(ctx context.Context, to []string, subject, htmlBody string) error {
	em := s.newEmail(to, subject)
	em.HTML = []byte(htmlBody)

	if s.logger != nil {
		s.logger.InfoContextKV(ctx, "Sending HTML email", "to", to, "subject", subject)
	}

	if err := s.send(em); err != nil {
		if s.logger != nil {
			s.logger.ErrorContextKV(ctx, "Send HTML email failed", "error", err, "to", to)
		}
		return err
	}
	return nil
}

// Close 关闭连接
// net/smtp 每次发信自建一条连接、发完即关，客户端本身不持有长连接，
// 所以这里只记一行日志，留着 Close 是为了满足 MailHandler 接口
//
// [EN] net/smtp opens a connection per message and closes it on completion, so the client
// holds nothing to release; Close only logs, and exists to satisfy MailHandler
func (s *SmtpClient) Close() error {
	if s.logger != nil {
		ctx := context.Background()
		s.logger.InfoContext(ctx, "SMTP client closed")
	}
	return nil
}

// NewMail 根据配置创建邮件客户端
func NewMail(cfg *smtpconfig.Smtp, log logger.ILogger) (MailHandler, error) {
	return NewSmtpClient(cfg, log)
}
