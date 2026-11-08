/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\observability\accesslog_headers_diff_test.go
 * @Description: 请求头脱敏的差分测试 - 直接遍历实现必须与原 JSON 往返实现逐项等价
 *
 * maskHeaders 原先的实现是 json.Marshal(header) → masker.Mask（内部再 Unmarshal→
 * 脱敏→Marshal）→ json.Unmarshal → 重建 map，每请求两次 Marshal 加两次 Unmarshal。
 * 现在改为直接遍历 header，按键名取动作。
 *
 * 脱敏路径的改动只能靠差分来保证：漏擦不会报错，只会让凭据静默进入审计库 ——
 * 而审计库恰恰是留存最久、导出最多的那份数据。所以这里把原实现原样保留为
 * legacyMaskHeadersViaJSON，对一大批键名形态逐项比对。
 *
 * [EN] maskHeaders used to be json.Marshal(header) → masker.Mask (which internally
 * unmarshals, masks and marshals again) → json.Unmarshal → rebuild, i.e. two marshals and two
 * unmarshals per request. It now walks the header map directly, taking the action per key name.
 *
 * A change to a redaction path can only be made safe by differential testing: a missed
 * redaction raises no error and merely lets credentials land silently in the audit store —
 * precisely the data retained longest and exported most. The original implementation is
 * therefore kept verbatim as legacyMaskHeadersViaJSON and compared item by item across a wide
 * range of key shapes.
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/desensitize"
)

// legacyMaskHeadersViaJSON 改造前的实现，原样保留供差分比对
func legacyMaskHeadersViaJSON(header http.Header) map[string][]string {
	if len(header) == 0 {
		return nil
	}

	raw, err := json.Marshal(map[string][]string(header))
	if err != nil {
		return nil
	}

	var masked map[string]any
	if err := json.Unmarshal([]byte(activeHeaderMasker().Mask(raw)), &masked); err != nil {
		return nil
	}

	out := make(map[string][]string, len(masked))
	for name, value := range masked {
		switch v := value.(type) {
		case string:
			out[name] = []string{v}
		case []any:
			values := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					values = append(values, s)
				}
			}
			out[name] = values
		}
	}
	return out
}

// diffHeaderCases 覆盖各种键名形态
//
// 刻意把「长得像敏感键但不该被擦」的运维字段也列进来：脱敏最怕的不只是漏擦，
// 过度擦除会让日志失去排障价值，而且同样不会报错
//
// [EN] Ops fields that look sensitive but must stay visible are deliberately included: the
// risk is not only missing a redaction — over-redaction destroys the log's diagnostic value
// and is equally silent
func diffHeaderCases() []http.Header {
	return []http.Header{
		// 典型凭据，HTTP 的连字符拼法
		{
			"Authorization": {"Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig"},
			"Cookie":        {"session=abc; csrf=def"},
			"X-Token":       {"tok-secret"},
			"X-Access-Key":  {"AKIAsecret"},
			"X-Signature":   {"sig-secret"},
		},
		// 运维字段，必须原样保留
		{
			"X-Trace-Id":      {"4bf92f3577b34da6a3ce929d0e0e4736"},
			"X-Request-Id":    {"req-0123456789abcdef"},
			"X-Forwarded-For": {"203.0.113.10"},
			"User-Agent":      {"Mozilla/5.0"},
			"Content-Type":    {"application/json"},
			"X-Server-Node":   {"pod-abc-123"},
		},
		// 大小写与分隔符变体：归一化会去掉 - _ . 并转小写
		{
			"AUTHORIZATION":       {"Bearer x"},
			"x_token":             {"y"},
			"Set-Cookie":          {"a=b"},
			"Proxy-Authorization": {"Basic z"},
			"WWW-Authenticate":    {"Bearer realm=x"},
		},
		// 多值
		{
			"Cookie":     {"a=1", "b=2", "c=3"},
			"X-Trace-Id": {"t1", "t2"},
			"Accept":     {"application/json", "text/plain"},
		},
		// 手机号 / 邮箱类键名
		// 注意：JSON 路径里 header 的值是数组，而 maskTextValue 对非字符串一律返回
		// 完整掩码，所以这类键在请求头上的实际结果是整体擦除而非部分掩码
		// [EN] Header values are arrays on the JSON path and maskTextValue returns the full
		// mask for anything that is not a string, so these keys are wiped outright on headers
		// rather than partially masked
		{
			"X-Phone": {"13800138000"},
			"Email":   {"someone@example.com"},
			"Mail":    {"a@b.c"},
		},
		// 空值与奇怪的键
		{
			"X-Empty":    {""},
			"X":          {"v"},
			"X-Weird--_": {"v"},
		},
		// 单个键
		{"Authorization": {"Bearer only"}},
	}
}

// TestMaskHeadersMatchesLegacyJSONPath 直接遍历实现与原 JSON 往返实现逐项等价
func TestMaskHeadersMatchesLegacyJSONPath(t *testing.T) {
	for _, sensitiveKeys := range [][]string{
		nil,
		{"custom-secret"},
		{"weird", "phone"},
	} {
		cfg := gwconfig.Default()
		cfg.Middleware.Logging.SensitiveKeys = sensitiveKeys
		global.SetGatewayConfig(cfg)
		headerMaskerCache.Store(nil)
		t.Cleanup(func() {
			global.SetGatewayConfig(gwconfig.Default())
			headerMaskerCache.Store(nil)
		})

		for i, h := range diffHeaderCases() {
			got := map[string][]string(maskHeaders(h))
			want := legacyMaskHeadersViaJSON(h)

			if !reflect.DeepEqual(got, want) {
				t.Errorf("sensitive-keys=%v 用例 %d 不等价\n现实现: %v\n原实现: %v",
					sensitiveKeys, i, sortedDump(got), sortedDump(want))
			}
		}
	}
}

// sortedDump 稳定输出 map，便于比对差异
func sortedDump(m map[string][]string) string {
	if m == nil {
		return "<nil>"
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		b.WriteString(n)
		b.WriteString("=")
		b.WriteString(strings.Join(m[n], "|"))
		b.WriteString(" ")
	}
	return b.String()
}

// TestHeaderMaskTextMatchesMasker 本地掩码常量必须与脱敏器实际输出一致
//
// defaultSensitiveMask 在 go-toolbox 里未导出，这里只能取同值；
// 用脱敏器的实际输出闸住它，库里改了默认掩码这条测试就会失败
//
// [EN] The library's defaultSensitiveMask is unexported so the same value is restated here;
// this locks it against the masker's actual output, failing if the library's default changes
func TestHeaderMaskTextMatchesMasker(t *testing.T) {
	cfg := gwconfig.Default()
	cfg.Middleware.Logging.SensitiveMask = "" // 不配，走默认
	global.SetGatewayConfig(cfg)
	headerMaskerCache.Store(nil)
	t.Cleanup(func() {
		global.SetGatewayConfig(gwconfig.Default())
		headerMaskerCache.Store(nil)
	})

	// 让脱敏器自己吐出一次默认掩码
	masked := activeHeaderMasker().Mask([]byte(`{"authorization":"Bearer x"}`))
	var m map[string]any
	if err := json.Unmarshal([]byte(masked), &m); err != nil {
		t.Fatalf("脱敏器输出不是 JSON: %v (%q)", err, masked)
	}
	fromMasker, ok := m["authorization"].(string)
	if !ok {
		t.Fatalf("脱敏器未把 authorization 替换成字符串，实际 %#v", m["authorization"])
	}

	if got := headerMaskText(); got != fromMasker {
		t.Errorf("本地掩码 %q 与脱敏器输出 %q 不一致；"+
			"go-toolbox 的 defaultSensitiveMask 多半改了，请同步 defaultSensitiveMask 常量",
			got, fromMasker)
	}
}

// TestHeaderKeyActionMatchesMaskerActionFor 键名判定与脱敏器的动作判定等价
//
// 等价关系要分别在「内置规则命中」与「配置追加键命中」两条分支上都成立
// [EN] The equivalence must hold on both branches: a built-in rule hit and a configured
// extra-key hit
func TestHeaderKeyActionMatchesMaskerActionFor(t *testing.T) {
	extra := []string{"custom-secret", "mycred"}

	for _, name := range []string{
		"Authorization", "AUTHORIZATION", "authorization",
		"Cookie", "Set-Cookie", "Proxy-Authorization", "WWW-Authenticate",
		"X-Token", "x_token", "X.Token",
		"X-Access-Key", "X-Signature", "Password", "Passwd",
		"X-Trace-Id", "X-Request-Id", "User-Agent", "Content-Type",
		"X-Forwarded-For", "X-Server-Node", "Accept", "Accept-Language",
		"X-Custom-Secret", "my-MyCred-field", "X-Phone", "Email",
		"X", "", "--__..",
	} {
		// 期望值按 DataMasker.actionFor 的两段逻辑独立算一遍：
		// 先内置规则表，再配置追加键（命中即整体擦除）
		// [EN] The expectation is derived independently from DataMasker.actionFor's two stages
		want := desensitize.ActionForKeyLoose(name)
		if want == desensitize.KeepAsIs {
			for _, k := range extra {
				if containsFold(name, k) {
					want = desensitize.WipeValue
					break
				}
			}
		}

		if got := keyMaskAction(name, extra); got != want {
			t.Errorf("键 %q：keyMaskAction = %v，期望 %v", name, got, want)
		}
	}
}

// TestContainsFoldMatchesStringsLower containsFold 与「两边转小写再 Contains」等价
//
// 本地这份是 go-toolbox 未导出函数的复制品，必须有东西闸住它
// [EN] This is a copy of go-toolbox's unexported function and needs something to lock it down
func TestContainsFoldMatchesStringsLower(t *testing.T) {
	cases := []struct{ s, sub string }{
		{"Authorization", "auth"}, {"Authorization", "AUTH"},
		{"X-Token", "token"}, {"X-Token", "TOKEN"},
		{"abc", ""}, {"", "abc"}, {"", ""},
		{"abc", "abcd"}, {"aAbB", "abb"}, {"aAbB", "AbB"},
		{"X-Trace-Id", "trace"}, {"X-Trace-Id", "race-i"},
		{"UPPER", "upper"}, {"lower", "LOWER"},
		{"a1_b2.c3", "B2.C"},
	}
	for _, c := range cases {
		want := strings.Contains(strings.ToLower(c.s), strings.ToLower(c.sub))
		if got := containsFold(c.s, c.sub); got != want {
			t.Errorf("containsFold(%q, %q) = %v，期望 %v", c.s, c.sub, got, want)
		}
	}
}

// BenchmarkMaskHeaders 请求头脱敏，直接遍历 vs 原 JSON 往返
//
// 同一个二进制里对比，避免跨次运行的噪声
// [EN] Compared in one binary to avoid cross-run noise
// legacyMaskHeadersPerSlice 平坦数组改造之前的实现：每个头各自分配一个切片
//
// 原样保留供两件事用：差分断言（下面的 TestMaskHeadersSharesOneBackingArray）
// 与同一个二进制内的 A/B（BenchmarkMaskHeaders 的 perSlice 档）
// 机器漂移对两档的影响相同，所以这种 A/B 才是可信的时间证据
//
// [EN] The implementation before the flat-array change, one slice per header, kept for the
// differential assertion below and for the same-binary A/B in BenchmarkMaskHeaders: both arms
// see the same machine drift, which is what makes that comparison trustworthy
func legacyMaskHeadersPerSlice(header http.Header) map[string][]string {
	if len(header) == 0 {
		return nil
	}

	mask := headerMaskText()
	extra := headerExtraSensitiveKeys()

	out := make(map[string][]string, len(header))
	for name, values := range header {
		switch keyMaskAction(name, extra) {
		case desensitize.KeepAsIs:
			out[name] = append(make([]string, 0, len(values)), values...)
		default:
			out[name] = []string{mask}
		}
	}
	return out
}

// TestMaskHeadersSharesOneBackingArray 平坦底层数组的三个前提
//
//  1. 与「每头一个切片」的实现逐项等价 —— 这同时就是「cap 上界没算错」的测试：
//     一旦 append 扩容，先前交出去的子切片会继续指向旧数组、拿不到后续写入，
//     表现为某些头的值是空的或过期的，而不会有任何报错
//  2. 每个子切片的 cap 等于 len —— 否则钩子对某个头的值 append 一下，
//     写进去的东西会落到相邻头的值上
//  3. 分配次数不随头数线性增长 —— 这正是这次改动的目的，不闸住就会被改回去
//
// [EN] The three preconditions of the flat backing array: item-by-item equivalence with the
// one-slice-per-header implementation (which doubles as the test that the capacity bound is
// right, since a reallocation silently strands the sub-slices already handed out), cap equal
// to len on every sub-slice (or a hook appending to one header's values would overwrite the
// next header's), and an allocation count that no longer grows with the header count
func TestMaskHeadersSharesOneBackingArray(t *testing.T) {
	cfg := gwconfig.Default()
	global.SetGatewayConfig(cfg)
	headerMaskerCache.Store(nil)
	t.Cleanup(func() {
		global.SetGatewayConfig(gwconfig.Default())
		headerMaskerCache.Store(nil)
	})

	cases := append(diffHeaderCases(),
		// 值为空切片：擦除分支仍要写一个掩码，上界必须给它留位置
		// [EN] An empty value slice still gets one mask written, so the bound must leave room
		http.Header{"Authorization": {}, "Accept": {}, "X-Trace-Id": {"t1"}},
		// 全部命中敏感键：每个头只写 1 个掩码，上界会宽出很多
		// [EN] All sensitive: one mask each, so the bound is much looser than the writes
		http.Header{
			"Authorization": {"a", "b", "c"},
			"Cookie":        {"x", "y"},
			"X-Token":       {"z"},
		},
		// 全部保留且多值：上界刚好等于写入量，这是最容易算错的一档
		// [EN] All kept and multi-valued: the bound exactly equals what is written, the case
		// most easily miscounted
		http.Header{
			"Accept":          {"application/json", "text/plain", "*/*"},
			"Accept-Language": {"zh-CN", "en"},
			"X-Trace-Id":      {"t1", "t2", "t3", "t4"},
		},
	)

	for i, h := range cases {
		got := map[string][]string(maskHeaders(h))
		want := legacyMaskHeadersPerSlice(h)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("用例 %d 与每头一个切片的实现不等价\n现实现: %v\n旧实现: %v",
				i, sortedDump(got), sortedDump(want))
		}

		for name, values := range got {
			if cap(values) != len(values) {
				t.Errorf("用例 %d 的头 %s：cap=%d len=%d，cap 必须掐到 len，"+
					"否则钩子 append 会改写相邻头的值", i, name, cap(values), len(values))
			}
		}
	}

	// 分配次数与头数无关：20 个头与 3 个头相差不应超过 map 自身的桶增长
	// 改动前这里是每个头 1 次，20 个头就是 21 次
	// [EN] The allocation count is independent of the header count: the gap between 20 headers
	// and 3 must not exceed the map's own bucket growth. Before the change it was one per
	// header, so twenty headers cost twenty-one allocations
	small := http.Header{"Accept": {"application/json"}, "X-Trace-Id": {"t1"}, "Authorization": {"Bearer x"}}
	large := http.Header{}
	for _, name := range []string{
		"Accept", "Accept-Language", "Accept-Encoding", "Cache-Control", "Connection",
		"Content-Type", "Content-Length", "User-Agent", "Referer", "Origin",
		"X-Trace-Id", "X-Request-Id", "X-Forwarded-For", "X-Forwarded-Proto", "X-Real-Ip",
		"Authorization", "Cookie", "X-Token", "X-Access-Key", "X-Signature",
	} {
		large[name] = []string{"v1", "v2"}
	}

	allocsSmall := int(testing.AllocsPerRun(200, func() { _ = maskHeaders(small) }))
	allocsLarge := int(testing.AllocsPerRun(200, func() { _ = maskHeaders(large) }))
	if allocsLarge > allocsSmall+2 {
		t.Errorf("3 个头 %d 次分配，20 个头 %d 次：分配次数仍随头数增长，"+
			"平坦数组没生效（允许的差额只有 map 桶增长的 2 次）", allocsSmall, allocsLarge)
	}
}

func BenchmarkMaskHeaders(b *testing.B) {
	cfg := gwconfig.Default()
	global.SetGatewayConfig(cfg)
	headerMaskerCache.Store(nil)

	// 典型浏览器请求头规模
	h := http.Header{
		"Authorization":   {"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig"},
		"Cookie":          {"session=abc123; csrf=def456; theme=dark"},
		"User-Agent":      {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)"},
		"Accept":          {"application/json"},
		"Accept-Language": {"zh-CN,zh;q=0.9,en;q=0.8"},
		"Content-Type":    {"application/json"},
		"X-Trace-Id":      {"4bf92f3577b34da6a3ce929d0e0e4736"},
		"X-Request-Id":    {"req-0123456789abcdef"},
		"X-Forwarded-For": {"203.0.113.10"},
		"X-User-Id":       {"u-100286"},
	}

	b.Run("direct", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = maskHeaders(h)
		}
	})

	b.Run("perSlice", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = legacyMaskHeadersPerSlice(h)
		}
	})

	b.Run("legacyJSON", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = legacyMaskHeadersViaJSON(h)
		}
	})
}
