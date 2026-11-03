package observability

import (
	"strings"
	"testing"

	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// TestIsBlankMatchesIfEmpty isBlank 必须与 mathx.IfEmpty(v,"")=="" 判定逐项一致
//
// 这是用 isBlank 替换 IfEmpty 的依据：后者走 reflect.ValueOf，每次调用一次反射加一次分配
// 同时也锁住「不能改用 strings.TrimSpace」—— 它按 unicode.IsSpace 判，
// 会把 U+00A0、U+3000、U+2028、\v、\f 也算空白，与原语义不符
// 用例里刻意包含这五个字符，换成 TrimSpace 实现会让本测试判红
//
// [EN] isBlank must agree with mathx.IfEmpty(v, "") == "" on every case
// This is the basis for the replacement, and it also locks out strings.TrimSpace, which
// judges by unicode.IsSpace and would treat U+00A0, U+3000, U+2028, \v and \f as blank
// Those five are in the table on purpose, so a TrimSpace-based implementation fails here
func TestIsBlankMatchesIfEmpty(t *testing.T) {
	cases := []string{
		"", " ", "  ", "\t", "\n", "\r\n", " \t\n ",
		"x", " x", "x ", " x ", "有值", " 有值 ",
		" ",        // 不换行空格
		" x",       // 不换行空格加内容
		"　",        // 全角空格
		" ",        // 行分隔符
		"\v", "\f", // 垂直制表、换页
		"0", "false", "null",
		"\x00", // NUL
	}
	for _, v := range cases {
		want := mathx.IfEmpty(v, "") == ""
		got := isBlank(v)
		if got != want {
			t.Errorf("判定不一致 %q: isBlank=%v  IfEmpty空=%v", v, got, want)
		}
		// 顺带记录 TrimSpace 在哪些输入上与原语义分歧，防止有人日后换成它
		// [EN] Also records where TrimSpace diverges, so nobody swaps it in later
		if strings.TrimSpace(v) == "" && !want {
			t.Logf("  注意 %q: TrimSpace 当空而原语义不当空 —— 不能换 TrimSpace", v)
		}
	}
}
