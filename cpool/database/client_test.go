/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 12:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 07:28:09
 * @FilePath: \go-rpc-gateway\cpool\database\client_test.go
 * @Description: client 数据库连接测试
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package database

import (
	"testing"
	"time"

	"github.com/kamalyes/go-config/pkg/database"
	"github.com/stretchr/testify/assert"
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
	expected := "user:pass@tcp(localhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
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
	expected = "host=localhost user=user password=pass dbname=testdb port=5432 sslmode=disable"
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
		name                                     string
		slowThreshold                            int
		ignoreRecordNotFoundError                bool
		skipDefaultTransaction                   bool
		prepareStmt                              bool
		disableForeignKeyConstraintWhenMigrating bool
		allowGlobalUpdate                        bool
		createBatchSize                          int
		singularTable                            bool
	}{
		{"默认配置", 100, false, false, true, true, false, 100, true},
		{"高性能配置", 50, true, true, true, false, false, 200, true},
		{"安全配置", 200, false, false, true, true, false, 50, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试用的MySQL配置
			mysqlConfig := &database.MySQL{
				Host:                                     "localhost",
				Port:                                     "3306",
				Username:                                 "user",
				Password:                                 "pass",
				Dbname:                                   "testdb",
				Config:                                   "charset=utf8mb4",
				SlowThreshold:                            tt.slowThreshold,
				IgnoreRecordNotFoundError:                tt.ignoreRecordNotFoundError,
				SkipDefaultTransaction:                   tt.skipDefaultTransaction,
				PrepareStmt:                              tt.prepareStmt,
				DisableForeignKeyConstraintWhenMigrating: tt.disableForeignKeyConstraintWhenMigrating,
				AllowGlobalUpdate:                        tt.allowGlobalUpdate,
				CreateBatchSize:                          tt.createBatchSize,
				SingularTable:                            tt.singularTable,
				QueryFields:                              true,
				DisableNestedTransaction:                 false,
			}

			config := gormConfig(mysqlConfig)
			assert.NotNil(t, config)

			// 验证 GORM 配置
			assert.Equal(t, tt.skipDefaultTransaction, config.SkipDefaultTransaction)
			assert.Equal(t, tt.prepareStmt, config.PrepareStmt)
			assert.Equal(t, tt.disableForeignKeyConstraintWhenMigrating, config.DisableForeignKeyConstraintWhenMigrating)
			assert.Equal(t, tt.allowGlobalUpdate, config.AllowGlobalUpdate)
			assert.Equal(t, tt.createBatchSize, config.CreateBatchSize)
			assert.NotNil(t, config.NamingStrategy)

			// 验证logger配置
			assert.NotNil(t, config.Logger)
			gormLogger, ok := config.Logger.(*GormLogger)
			assert.True(t, ok)
			assert.NotNil(t, gormLogger)
			assert.Equal(t, time.Duration(tt.slowThreshold)*time.Millisecond, gormLogger.Config.SlowThreshold)
			assert.Equal(t, tt.ignoreRecordNotFoundError, gormLogger.Config.IgnoreRecordNotFoundError)
		})
	}
}
