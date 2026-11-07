/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-10-15 17:30:08
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-10-17 19:05:08
 * @FilePath: \go-rpc-gateway\middleware\i18n\main_test.go
 * @Description: 测试入口 - 初始化本子包测试所需的全局状态
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package i18n

import (
	"os"
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// TestMain 测试入口
//
// 拆包前整个 middleware 测试包共用 ratelimit_test.go 里的 TestMain 初始化 global.LOGGER 与
// global.GATEWAY，拆包后每个子包都是独立的测试二进制，需要各自初始化同一份全局状态，
// 否则读取 global.GATEWAY 的用例会拿到 nil
//
// [EN] Test entry point. Before the split, the whole middleware test package shared the
// TestMain in ratelimit_test.go to initialize global.LOGGER and global.GATEWAY. After the
// split each subpackage is its own test binary and has to initialize the same global state
// itself, otherwise cases reading global.GATEWAY would observe nil
func TestMain(m *testing.M) {
	global.LOGGER = logger.New()
	global.GATEWAY = gwconfig.Default()

	os.Exit(m.Run())
}
