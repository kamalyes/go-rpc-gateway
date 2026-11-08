/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:56:02
 * @FilePath: \go-rpc-gateway\cpool\kafka\kafka.go
 * @Description: Kafka 生产者与消费者封装
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package kafka 提供 Kafka 生产者与消费者连接池
//
// 只封装网关需要的能力：发送、拉取、提交与关闭，SASL/TLS 由配置驱动
// 业务语义（Topic 编排、重试阶段、DLQ）留在业务服务里
//
// [EN] Package kafka wraps Kafka producers and consumers for the gateway: send, fetch,
// commit and close, with SASL/TLS driven by configuration. Business semantics such as topic
// orchestration, retry stages and DLQs stay in the services.
package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	kafkaconfig "github.com/kamalyes/go-rpc-gateway/config/kafka"
	segmentio "github.com/segmentio/kafka-go"
)

// Producer Kafka 生产者
//
// 一条连接可发往多个 Topic：Topic 由每次 Send 显式传入，不写进配置
// [EN] One producer connection serves multiple topics; the topic is passed to each Send call
// and is not part of the configuration.
type Producer struct {
	writer *segmentio.Writer
}

// NewProducer 创建生产者
func NewProducer(cfg *kafkaconfig.Producer) (*Producer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Kafka 生产者配置为空")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	transport, err := buildTransport(cfg.ClientID, secondsDuration(cfg.DialTimeout), cfg.TLS, cfg.SASLMechanism, cfg.Username, cfg.Password)
	if err != nil {
		return nil, err
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	batchTimeout := millisDuration(cfg.BatchTimeout)
	if batchTimeout <= 0 {
		batchTimeout = defaultBatchTimeout
	}
	writeTimeout := secondsDuration(cfg.WriteTimeout)
	if writeTimeout <= 0 {
		writeTimeout = defaultWriteTimeout
	}
	writer := &segmentio.Writer{
		Addr:         segmentio.TCP(cfg.BrokersList()...),
		Transport:    transport,
		RequiredAcks: requiredAcks(cfg.RequiredAcks),
		Compression:  compression(cfg.Compression),
		BatchSize:    batchSize,
		BatchTimeout: batchTimeout,
		WriteTimeout: writeTimeout,
		Async:        cfg.Async,
	}
	return &Producer{writer: writer}, nil
}

// Send 发送一条消息到指定主题
func (p *Producer) Send(ctx context.Context, topic string, key, value []byte, headers map[string]string) error {
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("Kafka 发送主题不能为空")
	}
	message := segmentio.Message{
		Topic:   topic,
		Key:     key,
		Value:   value,
		Headers: toKafkaHeaders(headers),
	}
	if err := p.writer.WriteMessages(ctx, message); err != nil {
		return fmt.Errorf("Kafka 发送消息到 %s: %w", topic, err)
	}
	return nil
}

// SendJSON 序列化 payload 后发送到指定主题
func (p *Producer) SendJSON(ctx context.Context, topic, key string, payload any, headers map[string]string) error {
	value, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Kafka 序列化消息: %w", err)
	}
	return p.Send(ctx, topic, []byte(key), value, headers)
}

// Close 关闭生产者
func (p *Producer) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

// Stats 返回写出统计
func (p *Producer) Stats() segmentio.WriterStats {
	if p == nil || p.writer == nil {
		return segmentio.WriterStats{}
	}
	return p.writer.Stats()
}

// HealthCheck 检查生产者是否可用
// kafka-go 没有轻量级 ping，真正的连通性由首次写入暴露，这里只拦截未初始化
func (p *Producer) HealthCheck(_ context.Context) error {
	if p == nil || p.writer == nil {
		return errors.New("Kafka 生产者未初始化")
	}
	return nil
}

// Message 消费到的消息
type Message struct {
	Topic     string            `json:"topic"`
	Partition int               `json:"partition"`
	Offset    int64             `json:"offset"`
	Key       []byte            `json:"key"`
	Value     []byte            `json:"value"`
	Headers   map[string]string `json:"headers"`
	Time      time.Time         `json:"time"`
}

// DecodeJSON 把消息体按 JSON 解码到 target
// 不实现 json.Unmarshaler：Message 包装的是原始消息元数据，消息体需要显式解码
func (m Message) DecodeJSON(target any) error {
	return json.Unmarshal(m.Value, target)
}

// Header 读取指定消息头
func (m Message) Header(key string) string {
	return m.Headers[strings.ToLower(key)]
}

// Consumer Kafka 消费者
type Consumer struct {
	reader    *segmentio.Reader
	topic     string
	manualRun bool
}

// NewConsumer 创建消费者
func NewConsumer(cfg *kafkaconfig.Consumer) (*Consumer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("Kafka 消费者配置为空")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	transport, err := buildTransport(cfg.ClientID, secondsDuration(cfg.DialTimeout), cfg.TLS, cfg.SASLMechanism, cfg.Username, cfg.Password)
	if err != nil {
		return nil, err
	}
	minBytes := cfg.MinBytes
	if minBytes <= 0 {
		minBytes = defaultMinBytes
	}
	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	maxWait := millisDuration(cfg.MaxWait)
	if maxWait <= 0 {
		maxWait = defaultMaxWait
	}
	queueCapacity := cfg.QueueCapacity
	if queueCapacity <= 0 {
		queueCapacity = defaultQueueSize
	}
	// GroupID 与 Partition 二选一：配了消费组就由组内分配分区
	partition := 0
	if cfg.GroupID == "" {
		partition = cfg.Partition
	}
	readerConfig := segmentio.ReaderConfig{
		Brokers:        cfg.BrokersList(),
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		Partition:      partition,
		MinBytes:       minBytes,
		MaxBytes:       maxBytes,
		MaxWait:        maxWait,
		CommitInterval: millisDuration(cfg.CommitInterval),
		StartOffset:    startOffset(cfg.Offset),
		QueueCapacity:  queueCapacity,
		Dialer: &segmentio.Dialer{
			Timeout:       transport.DialTimeout,
			ClientID:      cfg.ClientID,
			TLS:           transport.TLS,
			SASLMechanism: transport.SASL,
		},
	}
	return &Consumer{reader: segmentio.NewReader(readerConfig), topic: cfg.Topic, manualRun: cfg.CommitInterval == 0}, nil
}

// Fetch 拉取一条消息
func (c *Consumer) Fetch(ctx context.Context) (Message, error) {
	raw, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return Message{}, fmt.Errorf("Kafka 拉取 %s: %w", c.topic, err)
	}
	return Message{
		Topic:     raw.Topic,
		Partition: raw.Partition,
		Offset:    raw.Offset,
		Key:       raw.Key,
		Value:     raw.Value,
		Headers:   fromKafkaHeaders(raw.Headers),
		Time:      raw.Time,
	}, nil
}

// Commit 提交消息位点
// CommitInterval 为 0 时必须显式提交，否则重启后会重复消费
func (c *Consumer) Commit(ctx context.Context, message Message) error {
	raw := segmentio.Message{
		Topic:     message.Topic,
		Partition: message.Partition,
		Offset:    message.Offset,
		Key:       message.Key,
		Value:     message.Value,
		Time:      message.Time,
	}
	if err := c.reader.CommitMessages(ctx, raw); err != nil {
		return fmt.Errorf("Kafka 提交 %s/%d/%d: %w", message.Topic, message.Partition, message.Offset, err)
	}
	return nil
}

// ManualCommit 判断是否需要手动提交位点
func (c *Consumer) ManualCommit() bool { return c.manualRun }

// Topic 返回消费主题
func (c *Consumer) Topic() string { return c.topic }

// Close 关闭消费者
func (c *Consumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}
	return c.reader.Close()
}

// Stats 返回消费统计
func (c *Consumer) Stats() segmentio.ReaderStats {
	if c == nil || c.reader == nil {
		return segmentio.ReaderStats{}
	}
	return c.reader.Stats()
}

// HealthCheck 检查消费者是否可用
// kafka-go 没有轻量级 ping，真正的连通性由首次拉取暴露，这里只拦截未初始化
func (c *Consumer) HealthCheck(_ context.Context) error {
	if c == nil || c.reader == nil {
		return errors.New("Kafka 消费者未初始化")
	}
	return nil
}

// toKafkaHeaders 转换消息头
func toKafkaHeaders(headers map[string]string) []segmentio.Header {
	if len(headers) == 0 {
		return nil
	}
	result := make([]segmentio.Header, 0, len(headers))
	for key, value := range headers {
		result = append(result, segmentio.Header{Key: key, Value: []byte(value)})
	}
	return result
}

// fromKafkaHeaders 转换消息头
func fromKafkaHeaders(headers []segmentio.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string, len(headers))
	for _, header := range headers {
		result[strings.ToLower(header.Key)] = string(header.Value)
	}
	return result
}
