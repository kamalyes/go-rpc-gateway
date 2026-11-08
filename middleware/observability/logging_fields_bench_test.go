package observability

import (
	"fmt"
	"testing"

	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// sink 防止编译器把整段消掉 —— 不逃逸时 make+append 会被整体优化掉，
// 测出 0.5 ns / 0 allocs 这种不可能的数
//
// [EN] A sink so the compiler cannot eliminate the whole thing: without escape the
// make and appends are optimised away, yielding impossible figures
var (
	sink     []any
	sinkStr  string
	sinkBool bool
)

// 常量键 vs 动态键，确认编译器对常量字符串装箱是否免费
func BenchmarkFields_ConstKeyDynValue(b *testing.B) {
	v := "dynamic-value-x"
	b.ReportAllocs()
	for b.Loop() {
		f := make([]any, 0, 32)
		f = append(f, constants.LogFieldTraceID, v)
		sink = f
	}
}

func BenchmarkFields_ConstKeyConstValue(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		f := make([]any, 0, 32)
		f = append(f, constants.LogFieldTraceID, "literal")
		sink = f
	}
}

// IfEmpty(value, "") 是不是空操作，以及它自己分不分配
func BenchmarkFields_IfEmptyOverhead(b *testing.B) {
	v := "dynamic-value-x"
	b.ReportAllocs()
	for b.Loop() {
		sinkStr = mathx.IfEmpty(v, "")
	}
}

// isBlank 对照 IfEmpty：两者判定等价（见 logging_fields_test.go），成本不等价
func BenchmarkFields_IsBlank(b *testing.B) {
	v := "dynamic-value-x"
	b.ReportAllocs()
	for b.Loop() {
		sinkBool = isBlank(v)
	}
}

func BenchmarkFields_NoIfEmpty(b *testing.B) {
	v := "dynamic-value-x"
	b.ReportAllocs()
	for b.Loop() {
		sinkStr = v
	}
}

// 整个 LogFields 组装：14 个字段，与访问日志的真实规模一致
func BenchmarkFields_Assemble14(b *testing.B) {
	v := "dynamic-value-x"
	b.ReportAllocs()
	for b.Loop() {
		lf := NewLogFields()
		for j := 0; j < 14; j++ {
			lf.Add(constants.LogFieldTraceID, v)
		}
		sink = lf.fields
	}
}

// ============================================================================
// LogFields 初始容量
//
// NewLogFields 用 make([]any, 0, 32)，也就是 16 个字段的位置。
// 但一条 HTTP 访问日志最多有 31 个字段（7 基础 + 1 慢标记 + 20 上下文 + query/req/resp），
// 也就是 62 个元素 —— 超过 32 就要重新分配一次（32→64）。
// Add 是内联的，所以 profile 会把这次重新分配算在 Add 的 flat 分配里。
//
// 按真实字段数分档量，而不是只量一种：字段数取决于请求带了多少属性头，
// 空值会被 Add 跳过，所以常态请求远不到上界。
// 只量上界会夸大收益，只量常态会看不见上界的代价。
//
// [EN] NewLogFields uses cap 32, room for 16 fields, while a full HTTP access-log line has up
// to 31 fields (62 elements) and so regrows once. Add is inlined, so the profile attributes
// that regrowth to Add's flat allocations. Measured at several field counts because the count
// depends on how many attribute headers the request carries, and Add skips blank values, so a
// common request is far below the upper bound
// ============================================================================

// buildFields 按给定字段数构造字段列表，模拟 Add 链的实际形状
func buildFields(capHint, nFields int, value string) []any {
	f := make([]any, 0, capHint)
	for i := 0; i < nFields; i++ {
		f = append(f, logBenchKeys[i%len(logBenchKeys)], value)
	}
	return f
}

// logBenchKeys 取真实的日志字段键常量
var logBenchKeys = []string{
	constants.LogFieldTraceID, constants.LogFieldRequestID, constants.LogFieldUserID,
	constants.LogFieldTenantID, constants.LogFieldMethod, constants.LogFieldPath,
	constants.LogFieldIP, constants.LogFieldUserAgent, constants.LogFieldDomain,
	constants.LogFieldRoleCode, constants.LogFieldSessionID, constants.LogFieldTimezone,
	constants.LogFieldAppID, constants.LogFieldDeviceID, constants.LogFieldAppVersion,
	constants.LogFieldPlatformID, constants.LogFieldRegionID, constants.LogFieldXNsID,
}

// 实测结论（Apple M4，-benchtime=2s -count=3）：
//
//	字段数   cap 32                  cap 64
//	 8       213 ns /  640 B / 9     300 ns / 1280 B / 9
//	14       282 ns /  736 B / 15    395 ns / 1376 B / 15
//	31       650 ns / 2160 B / 33    513 ns / 1648 B / 32
//
// 32 是对的，不要改成 64：常态访问日志只有 10 到 14 个字段（空值被 Add 跳过），
// 这一档 cap 64 白占一倍内存还更慢；只有「请求带满 20 个上下文属性头」的上界
// 才轮到 cap 64 划算，而那不是常态
//
// [EN] Measured conclusion: 32 is correct and should not be raised to 64. A common access-log
// line carries only 10 to 14 fields because Add skips blank values, and at that size cap 64
// costs twice the memory and more time; cap 64 only pays off at the upper bound where the
// request carries all 20 context attribute headers, which is not the common case
func BenchmarkLogFieldsCapacity(b *testing.B) {
	v := "dynamic-value-x"
	for _, nFields := range []int{8, 14, 22, 31} {
		for _, capHint := range []int{32, 64} {
			b.Run(fmt.Sprintf("fields%d/cap%d", nFields, capHint), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					sink = buildFields(capHint, nFields, v)
				}
			})
		}
	}
}
