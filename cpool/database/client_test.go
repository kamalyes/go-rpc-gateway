/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-22 15:27:05
 * @FilePath: \go-rpc-gateway\cpool\database\client_test.go
 * @Description: 数据库连接测试（单 DSN 方案）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package database

import (
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/config/database"
)

func TestGormDisabled(t *testing.T) {
	cfg := &gwconfig.Gateway{
		Database: &database.Database{Enabled: false, DSN: "postgres://u:p@127.0.0.1:5432/db"},
	}
	if db := Gorm(t.Context(), cfg, nil); db != nil {
		t.Fatal("expected nil when database disabled")
	}
}

func TestGormEmptyDSN(t *testing.T) {
	cfg := &gwconfig.Gateway{
		Database: &database.Database{Enabled: true},
	}
	if db := Gorm(t.Context(), cfg, nil); db != nil {
		t.Fatal("expected nil when DSN empty")
	}
}

func TestGormConfig(t *testing.T) {
	c := database.DefaultDatabaseConfig()
	gc := gormConfig(c)
	if gc.SkipDefaultTransaction != c.SkipDefaultTransaction {
		t.Fatal("SkipDefaultTransaction not applied")
	}
	if gc.PrepareStmt != c.PrepareStmt {
		t.Fatal("PrepareStmt not applied")
	}
	if gc.Logger == nil {
		t.Fatal("custom gorm logger not set")
	}
}
