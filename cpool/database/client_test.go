/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 12:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 22:56:15
 * @FilePath: \go-rpc-gateway\cpool\database\client_test.go
 * @Description: client 数据库连接测试
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package database

import (
	"testing"

	"github.com/kamalyes/go-config/pkg/database"
	gologger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestBuildDSN 测试DSN构建
func TestBuildDSN(t *testing.T) {
	// 创建MySQL配置
	mysqlConfig := &database.MySQL{
			Host:     "localhost",
			Port:     "3306",
			Username: "user",
			Password: "pass",
			Dbname:   "testdb",
			Config:   "charset=utf8mb4&parseTime=True&loc=Local",
	}

	// 测试MySQL DSN
	mysqlDSN := buildDSN(mysqlConfig, database.DBTypeMySQL)
	expected := "user:pass@tcp(localhost:3306)/testdb?charset%3Dutf8mb4%26parseTime%3DTrue%26loc%3DLocal"
	assert.Equal(t, expected, mysqlDSN)

	// 创建PostgreSQL配置
	postgresConfig := &database.PostgreSQL{
			Host:     "localhost",
			Port:     "5432",
			Username: "user",
			Password: "pass",
			Dbname:   "testdb",
			Config:   "sslmode=disable",
	}

	// 测试PostgreSQL DSN
	postgresDSN := buildDSN(postgresConfig, database.DBTypePostgreSQL)
	expected = "host=localhost user=user password=pass dbname=testdb port=5432 sslmode%3Ddisable"
	assert.Equal(t, expected, postgresDSN)

	// 创建SQLite配置
	sqliteConfig := &database.SQLite{
		DbPath: "/tmp/test.db",
	}

	// 测试SQLite DSN
	sqliteDSN := buildDSN(sqliteConfig, database.DBTypeSQLite)
	assert.Equal(t, "/tmp/test.db", sqliteDSN)
}

// TestGormConfig 测试GORM配置
func TestGormConfig(t *testing.T) {
	tests := []struct {
		logLevel       string
		expectedLogger logger.LogLevel
	}{
		{"silent", logger.Silent},
		{"Silent", logger.Silent},
		{"error", logger.Error},
		{"Error", logger.Error},
		{"warn", logger.Warn},
		{"Warn", logger.Warn},
		{"info", logger.Info},
		{"Info", logger.Info},
		{"unknown", logger.Error}, // 默认值
		{"", logger.Error},        // 空值
	}

	for _, tt := range tests {
		t.Run(tt.logLevel, func(t *testing.T) {
			config := gormConfig(tt.logLevel)
			assert.NotNil(t, config)
			assert.True(t, config.DisableForeignKeyConstraintWhenMigrating)
			assert.NotNil(t, config.NamingStrategy)
			assert.NotNil(t, config.Logger)
		})
	}
}

// TestGormFunctionsWithoutConfig 测试在没有配置时的行为
func TestGormFunctionsWithoutConfig(t *testing.T) {
	// 备份原始配置
	originalGateway := global.GATEWAY
	originalLogger := global.LOGGER
	
	// 设置为nil以模拟无配置状态
	global.GATEWAY = nil
	// 保持logger不为nil以避免panic
	global.LOGGER = gologger.NewLogger(&gologger.LogConfig{Level: gologger.ERROR})
	
	defer func() {
		global.GATEWAY = originalGateway
		global.LOGGER = originalLogger
	}()

	// 这些函数在没有正确配置时应该返回nil
	assert.Nil(t, Gorm())
	assert.Nil(t, GormMySQL())
	assert.Nil(t, GormPostgreSQL())
	assert.Nil(t, GormSQLite())
}

// TestInitDBWithEmptyHost 测试空主机配置
func TestInitDBWithEmptyHost(t *testing.T) {
	// 备份原始配置
	originalLog := global.LOGGER
	global.LOGGER = gologger.NewLogger(&gologger.LogConfig{Level: gologger.INFO})

	defer func() {
		global.LOGGER = originalLog
	}()

	// 创建空主机的MySQL配置
	mysqlConfig := &database.MySQL{
			Host: "", // 空主机
	}

	// 测试空主机应该返回nil
	db := initDB(mysqlConfig, database.DBTypeMySQL, func(dsn string) (*gorm.DB, error) {
		return nil, nil
	})

	assert.Nil(t, db)
}
