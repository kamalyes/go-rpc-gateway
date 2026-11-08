/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\observability\bodymask_test.go
 * @Description: 报文脱敏快路径的差分测试 - 快路径只能省掉工作，不能少擦一个字节
 *
 * 脱敏路径上的性能改动只有差分测试能保证：漏擦不报错，只会让凭据静默进入审计库。
 * 所以这里对每个用例同时跑 maskBody 与 activeMasker().Mask，并断言三件事：
 *   1. 走慢路径时逐字节相同
 *   2. 走快路径时作为 JSON 文档相同（允许键序、空白、数字格式的归一化差异）
 *   3. 「某个秘密值是否出现在输出里」两条路径完全一致 —— 这是漏擦的直接断言
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/desensitize"
)

// bodySecret 用例里统一的秘密值，便于断言「它有没有漏到输出里」
const bodySecret = "SUPERSECRET-ZZZ"

// setupBodyMaskConfig 按 server/http.go:initDataMasker 的同一套参数装配脱敏器
//
// 必须与网关自己的构造方式一致，否则这组差分测的是两份不同配置，证不了等价
// [EN] The masker must be built exactly as initDataMasker builds it, otherwise this
// differential compares two different configurations and proves nothing
func setupBodyMaskConfig(t *testing.T) {
	t.Helper()
	cfg := gwconfig.Default()
	global.SetGatewayConfig(cfg)
	global.DATAMASKER = desensitize.NewMasker(&desensitize.MaskerConfig{
		SensitiveKeys: cfg.Middleware.Logging.SensitiveKeys,
		SensitiveMask: cfg.Middleware.Logging.SensitiveMask,
		MaxBodySize:   cfg.Middleware.Logging.MaxBodySize,
	})
	headerMaskerCache.Store(nil)
	t.Cleanup(func() {
		global.DATAMASKER = nil
		global.SetGatewayConfig(gwconfig.Default())
		headerMaskerCache.Store(nil)
	})
}

// bodyMaskCases fast 标记的是「期望走快路径」，由 jsonBodyNeedsNoMasking 直接断言
func bodyMaskCases() []struct {
	name string
	raw  string
	fast bool
} {
	return []struct {
		name string
		raw  string
		fast bool
	}{
		{"干净对象", `{"message":"hello benchmark","count":1}`, true},
		{"干净数组", `[{"id":1},{"id":2}]`, true},
		{"嵌套干净", `{"data":{"items":[{"id":1,"name":"x"}],"total":2}}`, true},
		{"带空白与换行", "{\n  \"message\" : \"hi\" ,\n  \"n\" : 1\n}", true},
		{"空对象", `{}`, true},
		{"顶层标量数组", `[1,2,3]`, true},

		// 值里出现敏感词但键名干净：脱敏是按键名驱动的，两条路径都不擦
		// [EN] A sensitive word in a value under a clean key: masking is key-driven and
		// neither path redacts it
		{"敏感词只出现在值里", `{"note":"my token is ` + bodySecret + `"}`, true},
		// 值是一段转义后的 JSON 文本：里面的 "token" 不是本文档的键
		// [EN] The value is escaped JSON text; the "token" inside is not a key of this document
		{"值里嵌着转义的 JSON", `{"note":"{\"token\":\"` + bodySecret + `\"}"}`, true},
		{"值串里有冒号", `{"note":"a:b","n":1}`, true},

		{"顶层敏感键", `{"token":"` + bodySecret + `","count":1}`, false},
		{"嵌套敏感键", `{"data":{"authorization":"Bearer ` + bodySecret + `"}}`, false},
		{"数组里的敏感键", `[{"password":"` + bodySecret + `"}]`, false},
		{"大小写与分隔符变体", `{"X_Access-Key":"` + bodySecret + `"}`, false},
		// 键名带转义：Unmarshal 会解成 token，这里不解转义，于是交给 Mask
		// [EN] Unmarshal would decode this to token; the scan does not decode escapes and
		// hands the verdict to Mask
		{"键名带 unicode 转义", `{"toke\u006e":"` + bodySecret + `"}`, false},

		{"非法 JSON", `{"a":}`, false},
		{"未加引号的键", `{authorization: ` + bodySecret + `}`, false},
		{"截断的 JSON", `{"a":"b"`, false},
		{"纯文本表单", `token=` + bodySecret + `&a=1`, false},
		{"纯文本", `plain text body`, false},
	}
}

// TestMaskBodyMatchesMaskerSemantically 快路径与 Mask 的三项等价
func TestMaskBodyMatchesMaskerSemantically(t *testing.T) {
	setupBodyMaskConfig(t)
	masker := activeMasker()

	for _, c := range bodyMaskCases() {
		t.Run(c.name, func(t *testing.T) {
			raw := []byte(c.raw)

			if got := jsonBodyNeedsNoMasking(raw); got != c.fast {
				t.Fatalf("jsonBodyNeedsNoMasking = %v，期望 %v（走错路径的后果："+
					"快路径本不该生效时生效，就是一次漏擦）", got, c.fast)
			}

			fast := maskBody(raw)
			slow := masker.Mask(raw)

			// 3. 漏擦的直接断言：秘密值在两条路径下的去留必须一致
			if strings.Contains(fast, bodySecret) != strings.Contains(slow, bodySecret) {
				t.Errorf("秘密值去留不一致：maskBody 含=%v，Mask 含=%v\nmaskBody: %s\nMask: %s",
					strings.Contains(fast, bodySecret), strings.Contains(slow, bodySecret),
					fast, slow)
			}

			if !c.fast {
				// 1. 慢路径逐字节相同
				if fast != slow {
					t.Errorf("慢路径必须与 Mask 逐字节相同\nmaskBody: %s\nMask: %s", fast, slow)
				}
				return
			}

			// 2. 快路径：输出就是原文，且作为 JSON 文档与 Mask 的输出相同
			if fast != c.raw {
				t.Errorf("快路径应原样返回\n实际: %s\n原文: %s", fast, c.raw)
			}
			var a, b any
			if err := json.Unmarshal([]byte(fast), &a); err != nil {
				t.Fatalf("快路径输出不是合法 JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(slow), &b); err != nil {
				t.Fatalf("Mask 输出不是合法 JSON: %v", err)
			}
			if !reflect.DeepEqual(a, b) {
				t.Errorf("两条路径的 JSON 文档不等价\nmaskBody: %s\nMask: %s", fast, slow)
			}
		})
	}
}

// TestMaskBodyFastPathIsSingleAllocation 快路径只该有「复制成 string」这一次分配
//
// 这是这次改动的全部目的 不闸住的话，哪天在扫描里加一次 string(key) 的逃逸、
// 或者把 extra 清单复制一份，收益就悄悄没了，而功能测试全都照过
//
// [EN] The one allocation of the fast path is the copy into a string, which is the entire point
// of the change. Without this gate an escaping string(key) in the scan, or a copy of the extra
// key list, would quietly erase the gain while every functional test still passed
func TestMaskBodyFastPathIsSingleAllocation(t *testing.T) {
	setupBodyMaskConfig(t)

	clean := []byte(`{"message":"hello benchmark","count":1,"nested":{"a":"b","c":2}}`)
	if got := jsonBodyNeedsNoMasking(clean); !got {
		t.Fatalf("用例本身应走快路径")
	}

	allocs := int(testing.AllocsPerRun(500, func() { _ = maskBody(clean) }))
	if allocs > 1 {
		t.Errorf("快路径 %d 次分配，期望 1 次（只有 string(raw) 那一次）；"+
			"多出来的通常是扫描里某个 string(...) 逃逸了", allocs)
	}

	sensitive := []byte(`{"token":"` + bodySecret + `","count":1}`)
	slowAllocs := int(testing.AllocsPerRun(500, func() { _ = maskBody(sensitive) }))
	if slowAllocs <= allocs {
		t.Errorf("慢路径 %d 次分配、快路径 %d 次：慢路径本该贵得多，"+
			"这说明用例没真的走慢路径", slowAllocs, allocs)
	}
}

// TestDefaultMaskerMaxBodySizeMatchesLibrary 本地镜像的默认上限必须与库一致
//
// desensitize 的默认 MaxBodySize 未导出，这里用它的实际截断行为反推：
// 不超限时 JSON 路径成功、输出是合法 JSON；超限一个字节就会被截成半截 JSON，
// 反序列化失败后退回文本脱敏，输出不再是合法 JSON
//
// 库里改了默认值，这条测试会失败 —— 而 bodyMaskLimit 正是靠这个常量
// 保证「快路径生效时 Mask 不会截断」
//
// [EN] desensitize's default MaxBodySize is unexported, so it is inferred from the actual
// truncation behaviour: under the limit the JSON path succeeds and the output is valid JSON,
// one byte over it the body is cut into half a document, fails to unmarshal and falls back to
// text masking, so the output is no longer valid JSON. The test fails if the library's default
// changes, which is what lets bodyMaskLimit guarantee the fast path never races a truncation
func TestDefaultMaskerMaxBodySizeMatchesLibrary(t *testing.T) {
	masker := desensitize.NewMasker() // 默认配置

	// {"a":"<pad>"} 共 n 字节
	body := func(n int) []byte {
		const head, tail = `{"a":"`, `"}`
		return []byte(head + strings.Repeat("x", n-len(head)-len(tail)) + tail)
	}

	atLimit := masker.Mask(body(defaultMaskerMaxBodySize))
	if !json.Valid([]byte(atLimit)) {
		t.Errorf("正好 %d 字节时输出应是合法 JSON，实际被截断了 —— "+
			"说明库的默认上限小于 %d", defaultMaskerMaxBodySize, defaultMaskerMaxBodySize)
	}

	overLimit := masker.Mask(body(defaultMaskerMaxBodySize + 1))
	if json.Valid([]byte(overLimit)) {
		t.Errorf("超出 %d 字节一个字节时输出仍是合法 JSON —— "+
			"说明库的默认上限大于 %d，bodyMaskLimit 的上界算小了",
			defaultMaskerMaxBodySize, defaultMaskerMaxBodySize)
	}
}

// TestBodyMaskLimitFollowsConfig 配置里的 max-body-size 更小时，上限跟着它走
func TestBodyMaskLimitFollowsConfig(t *testing.T) {
	setupBodyMaskConfig(t)

	if got, want := bodyMaskLimit(), gwconfig.Default().Middleware.Logging.MaxBodySize; got != want {
		t.Errorf("bodyMaskLimit() = %d，期望跟随配置的 %d", got, want)
	}

	// 超过配置上限的干净报文必须走慢路径：Mask 会先截断，快路径无法等价
	// [EN] A clean body over the configured limit must take the slow path, since Mask truncates
	limit := bodyMaskLimit()
	big := []byte(`{"a":"` + strings.Repeat("x", limit) + `"}`)
	if jsonBodyNeedsNoMasking(big) {
		t.Errorf("%d 字节的报文超过上限 %d，不该走快路径", len(big), limit)
	}
}

// BenchmarkMaskBody 同一个二进制内的 A/B：快路径 vs 直接调 Mask
func BenchmarkMaskBody(b *testing.B) {
	cfg := gwconfig.Default()
	global.SetGatewayConfig(cfg)
	global.DATAMASKER = desensitize.NewMasker(&desensitize.MaskerConfig{
		SensitiveKeys: cfg.Middleware.Logging.SensitiveKeys,
		SensitiveMask: cfg.Middleware.Logging.SensitiveMask,
		MaxBodySize:   cfg.Middleware.Logging.MaxBodySize,
	})
	b.Cleanup(func() {
		global.DATAMASKER = nil
		global.SetGatewayConfig(gwconfig.Default())
	})
	masker := activeMasker()

	clean := []byte(`{"message":"hello benchmark","count":1}`)
	sensitive := []byte(`{"message":"hello benchmark","token":"` + bodySecret + `"}`)

	b.Run("clean/fastPath", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = maskBody(clean)
		}
	})
	b.Run("clean/alwaysMask", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = masker.Mask(clean)
		}
	})
	b.Run("sensitive/fastPath", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = maskBody(sensitive)
		}
	})
	b.Run("sensitive/alwaysMask", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = masker.Mask(sensitive)
		}
	})
}
