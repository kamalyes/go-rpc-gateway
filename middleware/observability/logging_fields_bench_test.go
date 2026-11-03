package observability

import (
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := make([]any, 0, 32)
		f = append(f, constants.LogFieldTraceID, v)
		sink = f
	}
}

func BenchmarkFields_ConstKeyConstValue(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := make([]any, 0, 32)
		f = append(f, constants.LogFieldTraceID, "literal")
		sink = f
	}
}

// IfEmpty(value, "") 是不是空操作，以及它自己分不分配
func BenchmarkFields_IfEmptyOverhead(b *testing.B) {
	v := "dynamic-value-x"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkStr = mathx.IfEmpty(v, "")
	}
}

// isBlank 对照 IfEmpty：两者判定等价（见 logging_fields_test.go），成本不等价
func BenchmarkFields_IsBlank(b *testing.B) {
	v := "dynamic-value-x"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBool = isBlank(v)
	}
}

func BenchmarkFields_NoIfEmpty(b *testing.B) {
	v := "dynamic-value-x"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkStr = v
	}
}

// 整个 LogFields 组装：14 个字段，与访问日志的真实规模一致
func BenchmarkFields_Assemble14(b *testing.B) {
	v := "dynamic-value-x"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lf := NewLogFields()
		for j := 0; j < 14; j++ {
			lf.Add(constants.LogFieldTraceID, v)
		}
		sink = lf.fields
	}
}
