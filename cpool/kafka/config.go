/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 10:40:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 11:10:00
 * @FilePath: \go-rpc-gateway\cpool\kafka\config.go
 * @Description: Kafka 配置到 kafka-go 的接线
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	kafkaconfig "github.com/kamalyes/go-rpc-gateway/config/kafka"
	segmentio "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// 默认值
const (
	defaultDialTimeout  = 5 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultBatchTimeout = 100 * time.Millisecond
	defaultBatchSize    = 100
	defaultMinBytes     = 1
	defaultMaxBytes     = 10 << 20
	defaultMaxWait      = 500 * time.Millisecond
	defaultQueueSize    = 100
)

// buildTransport 构建带 SASL/TLS 的传输层
func buildTransport(clientID string, dialTimeout time.Duration, tlsConfig kafkaconfig.TLS, saslMechanism, username, password string) (*segmentio.Transport, error) {
	if dialTimeout <= 0 {
		dialTimeout = defaultDialTimeout
	}
	transport := &segmentio.Transport{
		DialTimeout: dialTimeout,
		ClientID:    clientID,
	}
	if tlsConfig.Enabled || tlsConfig.CAFile != "" || tlsConfig.CertFile != "" {
		resolved, err := buildTLSConfig(tlsConfig)
		if err != nil {
			return nil, err
		}
		transport.TLS = resolved
	}
	if username != "" {
		mechanism, err := buildSASLMechanism(saslMechanism, username, password)
		if err != nil {
			return nil, err
		}
		transport.SASL = mechanism
	}
	return transport, nil
}

// buildTLSConfig 构建 TLS 配置
func buildTLSConfig(tlsConfig kafkaconfig.TLS) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: tlsConfig.InsecureSkipVerify, // nolint:gosec // 由配置显式开启
	}
	if tlsConfig.CAFile != "" {
		data, err := os.ReadFile(tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("读取 Kafka CA 证书 %s: %w", tlsConfig.CAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("Kafka CA 证书 %s 不含有效证书", tlsConfig.CAFile)
		}
		config.RootCAs = pool
	}
	if tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" {
		certificate, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载 Kafka 客户端证书: %w", err)
		}
		config.Certificates = []tls.Certificate{certificate}
	}
	return config, nil
}

// buildSASLMechanism 构建 SASL 机制
func buildSASLMechanism(name, username, password string) (sasl.Mechanism, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", kafkaconfig.SASLMechanismPlain:
		return plain.Mechanism{Username: username, Password: password}, nil
	case kafkaconfig.SASLMechanismScramSHA256:
		return scram.Mechanism(scram.SHA256, username, password)
	case kafkaconfig.SASLMechanismScramSHA512:
		return scram.Mechanism(scram.SHA512, username, password)
	default:
		return nil, fmt.Errorf("不支持的 Kafka SASL 机制: %s", name)
	}
}

// requiredAcks 把配置值映射为 kafka-go 的确认级别
func requiredAcks(value int) segmentio.RequiredAcks {
	switch value {
	case 0:
		return segmentio.RequireNone
	case 1:
		return segmentio.RequireOne
	default:
		return segmentio.RequireAll
	}
}

// compression 把配置值映射为压缩算法
func compression(value string) segmentio.Compression {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case kafkaconfig.CompressionGzip:
		return segmentio.Gzip
	case kafkaconfig.CompressionSnappy:
		return segmentio.Snappy
	case kafkaconfig.CompressionLz4:
		return segmentio.Lz4
	case kafkaconfig.CompressionZstd:
		return segmentio.Zstd
	default:
		return 0
	}
}

// startOffset 把配置值映射为起始位点
func startOffset(value string) int64 {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case kafkaconfig.OffsetEarliest:
		return segmentio.FirstOffset
	case kafkaconfig.OffsetLatest:
		return segmentio.LastOffset
	default:
		return 0
	}
}

// secondsDuration 把秒转换为时长
func secondsDuration(seconds int) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// millisDuration 把毫秒转换为时长
func millisDuration(millis int) time.Duration {
	if millis <= 0 {
		return 0
	}
	return time.Duration(millis) * time.Millisecond
}
