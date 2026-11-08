/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 11:45:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 11:45:00
 * @FilePath: \go-rpc-gateway\cpool\manager_pools.go
 * @Description: 连接池管理器的多实例与自定义池能力
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package cpool

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	dbconfig "github.com/kamalyes/go-rpc-gateway/config/database"
	kafkaconfig "github.com/kamalyes/go-rpc-gateway/config/kafka"
	otsconfig "github.com/kamalyes/go-rpc-gateway/config/ots"
	cpdatabase "github.com/kamalyes/go-rpc-gateway/cpool/database"
	kafkapool "github.com/kamalyes/go-rpc-gateway/cpool/kafka"
	otspool "github.com/kamalyes/go-rpc-gateway/cpool/ots"
	"gorm.io/gorm"
)

// DefaultName 默认实例名
// 数据库的默认实例仍由 Manager.GetDB 持有，其余资源的默认实例存放在命名表中
const DefaultName = "default"

const healthCheckTimeout = 2 * time.Second

// ManagedPool 可注册到 Manager 的自定义连接池
// Manager 统一负责关闭；实现 PoolHealthChecker 时还会参与健康检查
type ManagedPool interface {
	Close() error
}

// PoolHealthChecker 可选的自定义池健康检查
type PoolHealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// managerExtensions 命名实例与自定义池的存储
// 独立持锁，保证 Manager 既有字段与新能力之间没有锁顺序冲突：
// 统一按 Manager.mu -> managerExtensions.mu 的顺序获取
type managerExtensions struct {
	mu sync.RWMutex

	databases     map[string]*gorm.DB
	databaseOrder []string

	kafkaProducers     map[string]*kafkapool.Producer
	kafkaProducerOrder []string

	kafkaConsumers     map[string]*kafkapool.Consumer
	kafkaConsumerOrder []string

	otsClients map[string]*otspool.Client
	otsOrder   []string

	pools     map[string]ManagedPool
	poolOrder []string
}

// normalizeName 空名字统一落到默认实例
func normalizeName(name string) string {
	if name == "" {
		return DefaultName
	}
	return name
}

// storeDatabase 保存已打开的命名数据库实例
// 供 Manager.Initialize 在持有 Manager.mu 时使用，避免走 AddDatabase 造成锁重入
func (e *managerExtensions) storeDatabase(name string, db *gorm.DB) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.databases == nil {
		e.databases = make(map[string]*gorm.DB)
	}
	if _, ok := e.databases[name]; ok {
		return fmt.Errorf("数据库实例 %s 已存在", name)
	}
	e.databases[name] = db
	e.databaseOrder = append(e.databaseOrder, name)
	return nil
}

// AddDatabase 注册一个命名数据库实例
// 名字为 default（或空）时写入 Manager.GetDB 持有的默认实例
func (m *Manager) AddDatabase(ctx context.Context, name string, cfg *dbconfig.Database) error {
	name = normalizeName(name)
	if name == DefaultName {
		if m.GetDB() != nil {
			return fmt.Errorf("默认数据库实例已存在")
		}
		db, err := cpdatabase.Open(ctx, cfg, m.logger)
		if err != nil {
			return err
		}
		m.SetDB(db)
		return nil
	}

	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	if extensions.databases == nil {
		extensions.databases = make(map[string]*gorm.DB)
	}
	if _, ok := extensions.databases[name]; ok {
		return fmt.Errorf("数据库实例 %s 已存在", name)
	}
	db, err := cpdatabase.Open(ctx, cfg, m.logger)
	if err != nil {
		return err
	}
	extensions.databases[name] = db
	extensions.databaseOrder = append(extensions.databaseOrder, name)
	return nil
}

// GetDatabase 获取命名数据库实例
func (m *Manager) GetDatabase(name string) *gorm.DB {
	name = normalizeName(name)
	if name == DefaultName {
		return m.GetDB()
	}
	extensions := &m.extensions
	extensions.mu.RLock()
	defer extensions.mu.RUnlock()
	return extensions.databases[name]
}

// RemoveDatabase 关闭并移除命名数据库实例
func (m *Manager) RemoveDatabase(name string) error {
	name = normalizeName(name)
	if name == DefaultName {
		db := m.GetDB()
		if db == nil {
			return nil
		}
		m.SetDB(nil)
		return closeGormDB(db)
	}

	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	db, ok := extensions.databases[name]
	if !ok {
		return fmt.Errorf("数据库实例 %s 不存在", name)
	}
	delete(extensions.databases, name)
	extensions.databaseOrder = removeName(extensions.databaseOrder, name)
	return closeGormDB(db)
}

// DatabaseNames 返回所有数据库实例名，包含默认实例
func (m *Manager) DatabaseNames() []string {
	names := make([]string, 0, 4)
	if m.GetDB() != nil {
		names = append(names, DefaultName)
	}
	extensions := &m.extensions
	extensions.mu.RLock()
	for name := range extensions.databases {
		names = append(names, name)
	}
	extensions.mu.RUnlock()
	sort.Strings(names)
	return names
}

// AddKafkaProducer 注册命名 Kafka 生产者
func (m *Manager) AddKafkaProducer(name string, cfg *kafkaconfig.Producer) error {
	name = normalizeName(name)
	producer, err := kafkapool.NewProducer(cfg)
	if err != nil {
		return err
	}
	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	if extensions.kafkaProducers == nil {
		extensions.kafkaProducers = make(map[string]*kafkapool.Producer)
	}
	if _, ok := extensions.kafkaProducers[name]; ok {
		_ = producer.Close()
		return fmt.Errorf("Kafka 生产者 %s 已存在", name)
	}
	extensions.kafkaProducers[name] = producer
	extensions.kafkaProducerOrder = append(extensions.kafkaProducerOrder, name)
	return nil
}

// GetKafkaProducer 获取命名 Kafka 生产者
func (m *Manager) GetKafkaProducer(name string) *kafkapool.Producer {
	extensions := &m.extensions
	extensions.mu.RLock()
	defer extensions.mu.RUnlock()
	return extensions.kafkaProducers[normalizeName(name)]
}

// RemoveKafkaProducer 关闭并移除命名 Kafka 生产者
func (m *Manager) RemoveKafkaProducer(name string) error {
	name = normalizeName(name)
	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	producer, ok := extensions.kafkaProducers[name]
	if !ok {
		return fmt.Errorf("Kafka 生产者 %s 不存在", name)
	}
	delete(extensions.kafkaProducers, name)
	extensions.kafkaProducerOrder = removeName(extensions.kafkaProducerOrder, name)
	return producer.Close()
}

// KafkaProducerNames 返回所有 Kafka 生产者名
func (m *Manager) KafkaProducerNames() []string {
	extensions := &m.extensions
	extensions.mu.RLock()
	defer extensions.mu.RUnlock()
	return sortedNames(extensions.kafkaProducers)
}

// AddKafkaConsumer 注册命名 Kafka 消费者
func (m *Manager) AddKafkaConsumer(name string, cfg *kafkaconfig.Consumer) error {
	name = normalizeName(name)
	consumer, err := kafkapool.NewConsumer(cfg)
	if err != nil {
		return err
	}
	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	if extensions.kafkaConsumers == nil {
		extensions.kafkaConsumers = make(map[string]*kafkapool.Consumer)
	}
	if _, ok := extensions.kafkaConsumers[name]; ok {
		_ = consumer.Close()
		return fmt.Errorf("Kafka 消费者 %s 已存在", name)
	}
	extensions.kafkaConsumers[name] = consumer
	extensions.kafkaConsumerOrder = append(extensions.kafkaConsumerOrder, name)
	return nil
}

// GetKafkaConsumer 获取命名 Kafka 消费者
func (m *Manager) GetKafkaConsumer(name string) *kafkapool.Consumer {
	extensions := &m.extensions
	extensions.mu.RLock()
	defer extensions.mu.RUnlock()
	return extensions.kafkaConsumers[normalizeName(name)]
}

// RemoveKafkaConsumer 关闭并移除命名 Kafka 消费者
func (m *Manager) RemoveKafkaConsumer(name string) error {
	name = normalizeName(name)
	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	consumer, ok := extensions.kafkaConsumers[name]
	if !ok {
		return fmt.Errorf("Kafka 消费者 %s 不存在", name)
	}
	delete(extensions.kafkaConsumers, name)
	extensions.kafkaConsumerOrder = removeName(extensions.kafkaConsumerOrder, name)
	return consumer.Close()
}

// KafkaConsumerNames 返回所有 Kafka 消费者名
func (m *Manager) KafkaConsumerNames() []string {
	extensions := &m.extensions
	extensions.mu.RLock()
	defer extensions.mu.RUnlock()
	return sortedNames(extensions.kafkaConsumers)
}

// AddOTS 注册命名 Tablestore 客户端
func (m *Manager) AddOTS(name string, cfg *otsconfig.OTS) error {
	name = normalizeName(name)
	client, err := otspool.NewClient(cfg)
	if err != nil {
		return err
	}
	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	if extensions.otsClients == nil {
		extensions.otsClients = make(map[string]*otspool.Client)
	}
	if _, ok := extensions.otsClients[name]; ok {
		_ = client.Close()
		return fmt.Errorf("Tablestore 客户端 %s 已存在", name)
	}
	extensions.otsClients[name] = client
	extensions.otsOrder = append(extensions.otsOrder, name)
	return nil
}

// GetOTS 获取命名 Tablestore 客户端
func (m *Manager) GetOTS(name string) *otspool.Client {
	extensions := &m.extensions
	extensions.mu.RLock()
	defer extensions.mu.RUnlock()
	return extensions.otsClients[normalizeName(name)]
}

// RemoveOTS 关闭并移除命名 Tablestore 客户端
func (m *Manager) RemoveOTS(name string) error {
	name = normalizeName(name)
	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	client, ok := extensions.otsClients[name]
	if !ok {
		return fmt.Errorf("Tablestore 客户端 %s 不存在", name)
	}
	delete(extensions.otsClients, name)
	extensions.otsOrder = removeName(extensions.otsOrder, name)
	return client.Close()
}

// OTSNames 返回所有 Tablestore 客户端名
func (m *Manager) OTSNames() []string {
	extensions := &m.extensions
	extensions.mu.RLock()
	defer extensions.mu.RUnlock()
	return sortedNames(extensions.otsClients)
}

// RegisterPool 注册自定义连接池
// 名称与内置资源共享命名空间，但健康检查与关闭顺序独立维护
func (m *Manager) RegisterPool(name string, pool ManagedPool) error {
	if name == "" {
		return fmt.Errorf("自定义池名称不能为空")
	}
	if pool == nil {
		return fmt.Errorf("自定义池不能为空")
	}
	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	if extensions.pools == nil {
		extensions.pools = make(map[string]ManagedPool)
	}
	if _, ok := extensions.pools[name]; ok {
		return fmt.Errorf("自定义池 %s 已存在", name)
	}
	extensions.pools[name] = pool
	extensions.poolOrder = append(extensions.poolOrder, name)
	return nil
}

// GetPool 获取自定义连接池
func (m *Manager) GetPool(name string) (ManagedPool, bool) {
	extensions := &m.extensions
	extensions.mu.RLock()
	defer extensions.mu.RUnlock()
	pool, ok := extensions.pools[name]
	return pool, ok
}

// UnregisterPool 关闭并移除自定义连接池
func (m *Manager) UnregisterPool(name string) error {
	extensions := &m.extensions
	extensions.mu.Lock()
	defer extensions.mu.Unlock()
	pool, ok := extensions.pools[name]
	if !ok {
		return fmt.Errorf("自定义池 %s 不存在", name)
	}
	delete(extensions.pools, name)
	extensions.poolOrder = removeName(extensions.poolOrder, name)
	return pool.Close()
}

// PoolNames 返回所有自定义连接池名
func (m *Manager) PoolNames() []string {
	extensions := &m.extensions
	extensions.mu.RLock()
	defer extensions.mu.RUnlock()
	return sortedNames(extensions.pools)
}

// healthCheck 汇总命名实例与自定义池的健康状态
func (e *managerExtensions) healthCheck(ctx context.Context) map[string]bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := make(map[string]bool)
	for name, db := range e.databases {
		status["database."+name] = pingGormDB(ctx, db)
	}
	for name, producer := range e.kafkaProducers {
		status["kafka.producer."+name] = checkHealth(ctx, producer)
	}
	for name, consumer := range e.kafkaConsumers {
		status["kafka.consumer."+name] = checkHealth(ctx, consumer)
	}
	for name, client := range e.otsClients {
		status["ots."+name] = checkHealth(ctx, client)
	}
	for name, pool := range e.pools {
		status["pool."+name] = checkHealth(ctx, pool)
	}
	return status
}

// closeAll 按注册顺序关闭所有命名实例与自定义池
// 自定义池最先关闭：它们可能依赖数据库或其他客户端
func (e *managerExtensions) closeAll() []error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var errs []error
	for _, name := range e.poolOrder {
		if pool := e.pools[name]; pool != nil {
			if err := pool.Close(); err != nil {
				errs = append(errs, fmt.Errorf("关闭自定义池 %s: %w", name, err))
			}
		}
	}
	for _, name := range e.kafkaProducerOrder {
		if producer := e.kafkaProducers[name]; producer != nil {
			if err := producer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("关闭 Kafka 生产者 %s: %w", name, err))
			}
		}
	}
	for _, name := range e.kafkaConsumerOrder {
		if consumer := e.kafkaConsumers[name]; consumer != nil {
			if err := consumer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("关闭 Kafka 消费者 %s: %w", name, err))
			}
		}
	}
	for _, name := range e.otsOrder {
		if client := e.otsClients[name]; client != nil {
			if err := client.Close(); err != nil {
				errs = append(errs, fmt.Errorf("关闭 Tablestore 客户端 %s: %w", name, err))
			}
		}
	}
	for _, name := range e.databaseOrder {
		if db := e.databases[name]; db != nil {
			if err := closeGormDB(db); err != nil {
				errs = append(errs, fmt.Errorf("关闭数据库实例 %s: %w", name, err))
			}
		}
	}

	e.databases = nil
	e.databaseOrder = nil
	e.kafkaProducers = nil
	e.kafkaProducerOrder = nil
	e.kafkaConsumers = nil
	e.kafkaConsumerOrder = nil
	e.otsClients = nil
	e.otsOrder = nil
	e.pools = nil
	e.poolOrder = nil
	return errs
}

// closeGormDB 关闭 GORM 底层的 sql.DB
func closeGormDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// pingGormDB 带超时的数据库连通性检查
func pingGormDB(ctx context.Context, db *gorm.DB) bool {
	if db == nil {
		return false
	}
	sqlDB, err := db.DB()
	if err != nil {
		return false
	}
	pingCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	return sqlDB.PingContext(pingCtx) == nil
}

// checkHealth 通用健康检查
func checkHealth(ctx context.Context, target any) bool {
	checker, ok := target.(PoolHealthChecker)
	if !ok {
		return true
	}
	return checker.HealthCheck(ctx) == nil
}

// removeName 从有序名称表里移除一项
func removeName(names []string, name string) []string {
	for index, item := range names {
		if item == name {
			return append(names[:index], names[index+1:]...)
		}
	}
	return names
}

// sortedNames 返回排序后的命名实例列表
func sortedNames[T any](items map[string]T) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
