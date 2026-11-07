/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 11:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 11:00:00
 * @FilePath: \go-rpc-gateway\config\kafka\common.go
 * @Description: Kafka 连接级公共配置
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package kafka

import (
	"errors"
	"fmt"
	"strings"
)

// SASL 鉴权机制
const (
	SASLMechanismPlain       = "plain"         // PLAIN，用户名密码明文校验，需配合 TLS 使用
	SASLMechanismScramSHA256 = "scram-sha-256" // SCRAM-SHA-256
	SASLMechanismScramSHA512 = "scram-sha-512" // SCRAM-SHA-512
)

// 消息压缩算法
const (
	CompressionNone   = "none"   // 不压缩
	CompressionGzip   = "gzip"   // gzip，兼容性最好
	CompressionSnappy = "snappy" // snappy，吞吐与 CPU 平衡
	CompressionLz4    = "lz4"    // lz4，低延迟
	CompressionZstd   = "zstd"   // zstd，高压缩比
)

// 消费起始位点
const (
	OffsetEarliest = "earliest" // 从最早可用的位点开始
	OffsetLatest   = "latest"   // 从最新位点开始
)

// TLS Kafka TLS 配置
// 双向 TLS 时同时配置 cert-file 与 key-file，只配其中一个会在校验阶段失败
type TLS struct {
	Enabled            bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                      // 是否启用 TLS
	InsecureSkipVerify bool   `mapstructure:"insecure-skip-verify" yaml:"insecure-skip-verify" json:"insecureSkipVerify"` // 跳过证书校验，仅限测试环境
	CAFile             string `mapstructure:"ca-file" yaml:"ca-file" json:"caFile"`                                       // CA 证书路径
	CertFile           string `mapstructure:"cert-file" yaml:"cert-file" json:"certFile"`                                 // 客户端证书路径
	KeyFile            string `mapstructure:"key-file" yaml:"key-file" json:"keyFile"`                                    // 客户端私钥路径
}

// Validate 校验 TLS 配置
func (t TLS) Validate() error {
	if (t.CertFile == "") != (t.KeyFile == "") {
		return errors.New("Kafka 客户端证书与私钥必须同时配置")
	}
	return nil
}

// validateConnection 校验连接级配置
func validateConnection(brokers []string, saslMechanism, username, password string, tlsConfig TLS) error {
	if len(brokers) == 0 {
		return errors.New("Kafka brokers 不能为空")
	}
	if (username == "") != (password == "") {
		return errors.New("Kafka SASL 用户名与密码必须同时配置")
	}
	if username != "" && !isSupportedSASLMechanism(saslMechanism) {
		return fmt.Errorf("Kafka SASL 机制不支持: %s", saslMechanism)
	}
	return tlsConfig.Validate()
}

// isSupportedSASLMechanism 判断 SASL 机制是否受支持
// 空值按 PLAIN 处理，与 cpool/kafka 的默认行为一致
func isSupportedSASLMechanism(mechanism string) bool {
	switch strings.ToLower(strings.TrimSpace(mechanism)) {
	case "", SASLMechanismPlain, SASLMechanismScramSHA256, SASLMechanismScramSHA512:
		return true
	default:
		return false
	}
}

// isSupportedCompression 判断压缩算法是否受支持
func isSupportedCompression(compression string) bool {
	switch strings.ToLower(strings.TrimSpace(compression)) {
	case "", CompressionNone, CompressionGzip, CompressionSnappy, CompressionLz4, CompressionZstd:
		return true
	default:
		return false
	}
}

// normalizeBrokers 清洗 broker 地址列表
func normalizeBrokers(brokers []string) []string {
	result := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		if broker = strings.TrimSpace(broker); broker != "" {
			result = append(result, broker)
		}
	}
	return result
}
