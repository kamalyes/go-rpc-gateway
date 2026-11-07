/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:17:09
 * @FilePath: \go-rpc-gateway\config\database\database.go
 * @Description: 数据库配置（单 DSN 方案）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package database

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// DBType 数据库类型
type DBType string

// 支持的数据库类型
const (
	DBTypeMySQL       DBType = "mysql"
	DBTypePostgreSQL  DBType = "postgres"
	DBTypeSQLite      DBType = "sqlite"
	DBTypeCockroachDB DBType = "cockroachdb"
)

// Database 数据库统一配置（单 DSN）
// 连接信息只需一个 DSN，连接池参数与 GORM 行为开关可选配置
type Database struct {
	ModuleName string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名称
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`            // 是否启用数据库
	Type       DBType `mapstructure:"type" yaml:"type" json:"type"`                     // 数据库类型 (postgres/mysql/sqlite/cockroachdb)
	DSN        string `mapstructure:"dsn" yaml:"dsn" json:"dsn"`                        // 连接字符串，如 postgres://user:pass@host:5432/dbname?sslmode=disable

	// 连接池参数
	MaxIdleConns    int           `mapstructure:"max-idle-conns" yaml:"max-idle-conns" json:"maxIdleConns"`            // 最大空闲连接数
	MaxOpenConns    int           `mapstructure:"max-open-conns" yaml:"max-open-conns" json:"maxOpenConns"`            // 最大打开连接数
	ConnMaxIdleTime time.Duration `mapstructure:"conn-max-idle-time" yaml:"conn-max-idle-time" json:"connMaxIdleTime"` // 连接最大空闲时间
	ConnMaxLifetime time.Duration `mapstructure:"conn-max-lifetime" yaml:"conn-max-lifetime" json:"connMaxLifetime"`   // 连接最大存活时间

	// GORM 行为开关
	SkipDefaultTransaction    bool   `mapstructure:"skip-default-transaction" yaml:"skip-default-transaction" json:"skipDefaultTransaction"`              // 跳过默认事务
	PrepareStmt               bool   `mapstructure:"prepare-stmt" yaml:"prepare-stmt" json:"prepareStmt"`                                                 // 预编译语句
	IgnoreRecordNotFoundError bool   `mapstructure:"ignore-record-not-found-error" yaml:"ignore-record-not-found-error" json:"ignoreRecordNotFoundError"` // 忽略记录未找到错误
	SlowThreshold             int    `mapstructure:"slow-threshold" yaml:"slow-threshold" json:"slowThreshold"`                                           // 慢查询阈值（毫秒）
	LogLevel                  string `mapstructure:"log-level" yaml:"log-level" json:"logLevel"`                                                          // SQL日志等级 (silent/error/warn/info)
}

// DefaultDatabaseConfig 返回默认数据库配置
func DefaultDatabaseConfig() *Database {
	return &Database{
		ModuleName:      "database",
		Enabled:         false,
		Type:            DBTypePostgreSQL,
		DSN:             "postgres://postgres:postgres@127.0.0.1:5432/gateway?sslmode=disable",
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxIdleTime: 30 * time.Minute,
		ConnMaxLifetime: time.Hour,
		SlowThreshold:   200,
		LogLevel:        "warn",
	}
}

// Get 返回配置接口
func (d *Database) Get() any {
	return d
}

// Set 设置配置数据
func (d *Database) Set(data any) {
	if cfg, ok := data.(*Database); ok {
		*d = *cfg
	}
}

// Clone 返回配置的副本
func (d *Database) Clone() internal.Configurable {
	var cloned Database
	if err := syncx.DeepCopy(&cloned, d); err != nil {
		return &Database{}
	}
	return &cloned
}

// Validate 验证配置
func (d *Database) Validate() error {
	if !d.Enabled {
		return nil
	}
	if d.DSN == "" {
		d.DSN = "postgres://postgres:postgres@127.0.0.1:5432/gateway?sslmode=disable"
	}
	if d.Type == "" {
		d.Type = DBTypePostgreSQL
	}
	if d.MaxIdleConns <= 0 {
		d.MaxIdleConns = 10
	}
	if d.MaxOpenConns <= 0 {
		d.MaxOpenConns = 100
	}
	if d.ConnMaxIdleTime <= 0 {
		d.ConnMaxIdleTime = 30 * time.Minute
	}
	if d.ConnMaxLifetime <= 0 {
		d.ConnMaxLifetime = time.Hour
	}
	return internal.ValidateStruct(d)
}
