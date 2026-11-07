/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\cpool\smtp\smtp_test.go
 * @Description: SMTP DSN 解析测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package smtp

import (
	"testing"

	smtpconfig "github.com/kamalyes/go-rpc-gateway/config/smtp"
)

// TestParseDSNDocumentedExamples 把 config/smtp.Smtp 类型注释里列出的每一条 DSN 示例
// 都真正跑一遍解析，断言地址、是否带认证、传输加密方式
//
// [EN] Every DSN example in the smtp.Smtp doc comment is parsed here, with the resulting
// address, presence of credentials and transport mode asserted
func TestParseDSNDocumentedExamples(t *testing.T) {
	cases := []struct {
		name      string
		dsn       string
		wantAddr  string
		wantAuth  bool
		wantTrans string
	}{
		{
			name:      "标准 587 自动 STARTTLS",
			dsn:       "smtp://noreply%40example.com:app_password@smtp.example.com:587",
			wantAddr:  "smtp.example.com:587",
			wantAuth:  true,
			wantTrans: "plain",
		},
		{
			name:      "465 隐式 TLS",
			dsn:       "smtps://noreply%40example.com:app_password@smtp.example.com:465",
			wantAddr:  "smtp.example.com:465",
			wantAuth:  true,
			wantTrans: "implicit-tls",
		},
		{
			name:      "强制 STARTTLS 并跳过证书校验",
			dsn:       "smtp://noreply%40corp.lan:pwd@mail.corp.lan:587?starttls=true&skip_verify=true",
			wantAddr:  "mail.corp.lan:587",
			wantAuth:  true,
			wantTrans: "starttls",
		},
		{
			name:      "本地 25 中继无需认证",
			dsn:       "smtp://127.0.0.1:25",
			wantAddr:  "127.0.0.1:25",
			wantAuth:  false,
			wantTrans: "plain",
		},
		{
			name:      "Gmail 应用专用密码",
			dsn:       "smtp://someone%40gmail.com:abcd_efgh_ijkl_mnop@smtp.gmail.com:587",
			wantAddr:  "smtp.gmail.com:587",
			wantAuth:  true,
			wantTrans: "plain",
		},
		{
			name:      "省略端口走 smtp 默认 587",
			dsn:       "smtp://user:pwd@smtp.example.com",
			wantAddr:  "smtp.example.com:587",
			wantAuth:  true,
			wantTrans: "plain",
		},
		{
			name:      "省略端口走 smtps 默认 465",
			dsn:       "smtps://user:pwd@smtp.example.com",
			wantAddr:  "smtp.example.com:465",
			wantAuth:  true,
			wantTrans: "implicit-tls",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewSmtpClient(&smtpconfig.Smtp{DSN: tc.dsn}, nil)
			if err != nil {
				t.Fatalf("NewSmtpClient(%q) failed: %v", tc.dsn, err)
			}

			if client.addr != tc.wantAddr {
				t.Errorf("addr = %q, want %q", client.addr, tc.wantAddr)
			}
			if gotAuth := client.auth != nil; gotAuth != tc.wantAuth {
				t.Errorf("auth present = %v, want %v", gotAuth, tc.wantAuth)
			}
			if got := client.transportName(); got != tc.wantTrans {
				t.Errorf("transport = %q, want %q", got, tc.wantTrans)
			}
		})
	}
}

// TestParseDSNTLSConfig 断言 TLS 路径上的 ServerName 与 skip_verify 落位，
// 以及明文路径不构造 TLS 配置
//
// [EN] Asserts that ServerName and skip_verify land on the TLS paths, and that the
// plaintext path builds no TLS config at all
func TestParseDSNTLSConfig(t *testing.T) {
	t.Run("隐式 TLS 带 ServerName", func(t *testing.T) {
		client, err := NewSmtpClient(&smtpconfig.Smtp{DSN: "smtps://u:p@mail.example.com:465"}, nil)
		if err != nil {
			t.Fatalf("NewSmtpClient failed: %v", err)
		}
		if client.tlsConfig == nil {
			t.Fatal("tlsConfig = nil, want configured")
		}
		if client.tlsConfig.ServerName != "mail.example.com" {
			t.Errorf("ServerName = %q, want mail.example.com", client.tlsConfig.ServerName)
		}
		if client.tlsConfig.InsecureSkipVerify {
			t.Error("InsecureSkipVerify = true, want false")
		}
	})

	t.Run("STARTTLS 跳过校验", func(t *testing.T) {
		client, err := NewSmtpClient(&smtpconfig.Smtp{DSN: "smtp://u:p@mail.corp.lan:587?starttls=true&skip_verify=true"}, nil)
		if err != nil {
			t.Fatalf("NewSmtpClient failed: %v", err)
		}
		if client.tlsConfig == nil {
			t.Fatal("tlsConfig = nil, want configured")
		}
		if !client.tlsConfig.InsecureSkipVerify {
			t.Error("InsecureSkipVerify = false, want true")
		}
	})

	t.Run("明文不构造 TLS 配置", func(t *testing.T) {
		client, err := NewSmtpClient(&smtpconfig.Smtp{DSN: "smtp://127.0.0.1:25"}, nil)
		if err != nil {
			t.Fatalf("NewSmtpClient failed: %v", err)
		}
		if client.tlsConfig != nil {
			t.Errorf("tlsConfig = %+v, want nil", client.tlsConfig)
		}
	})
}

// TestNewSmtpClientRejectsBadConfig 断言坏配置在构造这一步就报错，而不是等到第一次发信
//
// [EN] Asserts that a bad config fails at construction rather than on the first send
func TestNewSmtpClientRejectsBadConfig(t *testing.T) {
	cases := []struct {
		name string
		cfg  *smtpconfig.Smtp
	}{
		{name: "配置为空", cfg: nil},
		{name: "DSN 为空", cfg: &smtpconfig.Smtp{DSN: ""}},
		{name: "scheme 不对", cfg: &smtpconfig.Smtp{DSN: "http://smtp.example.com:587"}},
		{name: "没有主机", cfg: &smtpconfig.Smtp{DSN: "smtp://"}},
		{name: "无法解析", cfg: &smtpconfig.Smtp{DSN: "smtp://u:p@[::1:587"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSmtpClient(tc.cfg, nil); err == nil {
				t.Fatal("NewSmtpClient succeeded, want error")
			}
		})
	}
}

// TestNewEmailAppliesFromAndHeaders 断言发件人回落与自定义头部真正写进邮件
// 这两项原先声明在配置里却没人读，改造时一并接上
//
// [EN] Asserts that the sender fallback and the configured custom headers actually reach
// the message; both were declared in the config but never read before this change
func TestNewEmailAppliesFromAndHeaders(t *testing.T) {
	t.Run("显式发件人优先", func(t *testing.T) {
		client, err := NewSmtpClient(&smtpconfig.Smtp{
			DSN:         "smtp://login%40example.com:pwd@smtp.example.com:587",
			FromAddress: "noreply@example.com",
		}, nil)
		if err != nil {
			t.Fatalf("NewSmtpClient failed: %v", err)
		}
		if em := client.newEmail([]string{"a@b.com"}, "s"); em.From != "noreply@example.com" {
			t.Errorf("From = %q, want noreply@example.com", em.From)
		}
	})

	t.Run("发件人留空回落到 DSN 用户名", func(t *testing.T) {
		client, err := NewSmtpClient(&smtpconfig.Smtp{
			DSN: "smtp://login%40example.com:pwd@smtp.example.com:587",
		}, nil)
		if err != nil {
			t.Fatalf("NewSmtpClient failed: %v", err)
		}
		if em := client.newEmail([]string{"a@b.com"}, "s"); em.From != "login@example.com" {
			t.Errorf("From = %q, want login@example.com", em.From)
		}
	})

	t.Run("自定义头部写进邮件", func(t *testing.T) {
		client, err := NewSmtpClient(&smtpconfig.Smtp{
			DSN:     "smtp://127.0.0.1:25",
			Headers: map[string]string{"X-Gateway": "go-rpc-gateway"},
		}, nil)
		if err != nil {
			t.Fatalf("NewSmtpClient failed: %v", err)
		}
		if got := client.newEmail([]string{"a@b.com"}, "s").Headers.Get("X-Gateway"); got != "go-rpc-gateway" {
			t.Errorf("X-Gateway = %q, want go-rpc-gateway", got)
		}
	})
}
