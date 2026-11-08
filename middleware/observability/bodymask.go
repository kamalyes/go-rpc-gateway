/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\observability\bodymask.go
 * @Description: 报文脱敏的快路径 - 整份报文没有敏感键时跳过 JSON 往返
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"encoding/json"

	"github.com/kamalyes/go-toolbox/pkg/desensitize"
)

// defaultMaskerMaxBodySize 与 go-toolbox desensitize 的默认 MaxBodySize 一致
//
// 该常量在库里未导出，这里取同值，由 TestDefaultMaskerMaxBodySizeMatchesLibrary
// 用脱敏器的实际截断行为闸住 —— 与 defaultSensitiveMask 同样的做法
//
// [EN] Mirrors desensitize's unexported default MaxBodySize; the library's actual truncation
// behaviour locks it down in TestDefaultMaskerMaxBodySizeMatchesLibrary, the same approach used
// for defaultSensitiveMask
const defaultMaskerMaxBodySize = 10240

// maskBody 脱敏报文，并在「这份报文里没有任何键需要脱敏」时跳过那次 JSON 往返
//
// 为什么值得专门做这条快路径：DataMasker.Mask 的 JSON 路径是
// json.Unmarshal 到 any → 递归遍历 → json.Marshal 回去的完整往返，memprofile 里
// 一次往返约 14 次分配（含 reflect.MapIter 的 copyVal 4 次），请求体加响应体就是 28 次 ——
// 这是 audit 档比 production 档多出来的那 28 次的几乎全部
//
// 而脱敏本身是**按键名**驱动的（DataMasker.maskJSONFields 对每个键查 actionFor，
// 值本身从不参与判定），所以「没有任何键命中规则」时那次往返的输出与输入是同一份
// JSON 文档，只差 encoding/json 的归一化（键序、空白、数字格式、< > & 的转义）
// 跳过它等于把报文原样留存，对审计库而言是更高的保真度：
// 键序保持原样，int64 不会被 float64 往返打掉精度
//
// 这是行为变化，不是纯优化 —— 不再脱敏的报文从「归一化后的 JSON」变成「原始字节」
// TestMaskBodyMatchesMaskerSemantically 按 JSON 文档等价（而非字节等价）闸住它
//
// [EN] Masks a body, skipping the JSON round trip when nothing in it needs masking.
//
// Why this fast path earns its keep: DataMasker.Mask's JSON path is a full
// json.Unmarshal-to-any, recursive walk and json.Marshal back, measured at roughly fourteen
// allocations per round trip (including four reflect.MapIter copyVals) — request plus response
// is twenty-eight, which is virtually all of the twenty-eight-allocation gap between the audit
// and production tiers.
//
// Masking is driven entirely by key names (maskJSONFields consults actionFor per key and never
// inspects the value), so when no key matches a rule the round trip's output is the same JSON
// document as its input, differing only in encoding/json's normalisation: key order, whitespace,
// number formatting and the escaping of <, > and &. Skipping it stores the body verbatim, which
// is higher fidelity for an audit store: key order survives and an int64 is not run through a
// float64.
//
// This is a behaviour change rather than a pure optimisation — an unmasked body goes from
// "normalised JSON" to "the original bytes". TestMaskBodyMatchesMaskerSemantically locks it down
// as JSON-document equivalence rather than byte equality
func maskBody(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if jsonBodyNeedsNoMasking(raw) {
		return string(raw)
	}
	return activeMasker().Mask(raw)
}

// jsonBodyNeedsNoMasking 报告 raw 是否「合法 JSON，且没有任何键命中脱敏规则」
//
// 四道门都通过才返回 true，每一道都对应 Mask 的一个分支，缺一道快路径就不等价：
//
//  1. 首字节必须是 { 或 [ —— 与 Mask 的入口判定逐字一致 其它形态（表单、纯文本）
//     走的是 maskText，那条路径按 key=value / key: value 的文本形态擦除，
//     判定规则与 JSON 键完全不同，这里不替它做判断
//  2. 长度不得超过上限 —— 超了 Mask 会先截断，半截 JSON 反序列化失败后退回 maskText，
//     于是输出既不是原文也不是合法 JSON，没有等价的快路径
//  3. 每个键查一遍动作 —— 键名带反斜杠转义时直接判为需要脱敏：
//     Unmarshal 会把 "\u0061uth" 解成 auth，而这里不解转义，于是交给 Mask 去判
//  4. 必须是合法 JSON —— json.Valid 与 json.Unmarshal 的接受集一致（后者的目标是 any，
//     不会有类型错误）不合法时 Mask 同样退回 maskText，而 maskText 能擦到的东西
//     这里的扫描看不见（比如未加引号的键 {authorization: x}）—— 漏擦不会报错，
//     所以这一道必须在
//
// 3 在 4 之前是实测定下来的顺序：键扫描命中敏感键就立刻返回，而 json.Valid 要走完整份报文
// 先 Valid 再扫描的话，「报文里真有敏感键」这个最常见的慢路径场景要白付一次完整扫描 ——
// 实测 940 ns 的报文变成 1210 ns，+29%
// 放到后面不影响正确性：扫描命中就退，没命中才需要 Valid 来保证「Mask 会走 JSON 路径」
//
// 非对称是刻意的：拿不准一律走慢路径 快路径只在能证明输出等价时才生效
//
// [EN] Reports whether raw is valid JSON in which no key matches a redaction rule. All four
// gates must pass, each mirroring one branch of Mask:
//
//  1. The first byte must be { or [, exactly as Mask decides. Other shapes (form bodies, plain
//     text) go through maskText, whose key detection works on key=value text and differs
//     entirely from JSON keys; this function does not judge for it.
//  2. The length must not exceed the limit: beyond it Mask truncates first, the half document
//     fails to unmarshal and falls back to maskText, so the output is neither the original nor
//     valid JSON and no equivalent fast path exists.
//  3. Every key is looked up. A key containing a backslash escape counts as needing masking:
//     Unmarshal would decode "\u0061uth" to auth while this scan does not decode escapes, so
//     the verdict is handed to Mask.
//  4. It must be valid JSON. json.Valid accepts exactly what json.Unmarshal into any accepts.
//     When it is invalid Mask again falls back to maskText, which can redact things this scan
//     cannot see — an unquoted key such as {authorization: x} — and a missed redaction raises
//     no error, so this gate is mandatory.
//
// Gate 3 precedes gate 4 on measured grounds: the key scan returns the moment it hits a
// sensitive key, while json.Valid always walks the whole body. With Valid first, the most common
// slow-path case — a body that really does carry a sensitive key — pays for a full scan for
// nothing, measured as 940 ns becoming 1210 ns. Putting it last costs no correctness: a hit
// returns immediately, and only a miss needs Valid to guarantee Mask takes the JSON path.
//
// The asymmetry is deliberate: anything uncertain takes the slow path, and the fast path only
// applies where the output can be shown to be equivalent
func jsonBodyNeedsNoMasking(raw []byte) bool {
	if raw[0] != '{' && raw[0] != '[' {
		return false
	}
	if len(raw) > bodyMaskLimit() {
		return false
	}

	extra := headerExtraSensitiveKeys()
	for i := 0; i < len(raw); {
		if raw[i] != '"' {
			i++
			continue
		}

		// 找这个字符串的收尾引号，反斜杠转义整对跳过
		// [EN] Find the closing quote, skipping backslash escapes as pairs
		j := i + 1
		escaped := false
		for j < len(raw) {
			switch raw[j] {
			case '\\':
				escaped = true
				j += 2
			case '"':
				goto closed
			default:
				j++
			}
		}
	closed:
		if j >= len(raw) {
			// 字符串没有收尾引号：报文本身不合法，交给 Mask（它会退回文本脱敏）
			// json.Valid 排在扫描之后，所以这条分支是真会走到的
			// [EN] An unterminated string means the body is not valid JSON, so it goes to Mask,
			// which falls back to text masking. json.Valid runs after this scan, so this branch
			// is genuinely reachable
			return false
		}

		// 字符串后面紧跟 ':' 的才是键 合法 JSON 里值串后面只能是 , } ] 或结束，
		// 所以这个判定不会把值误当成键
		// [EN] Only a string followed by ':' is a key; in valid JSON a value string can only be
		// followed by , } ] or the end, so a value is never mistaken for a key
		k := j + 1
		for k < len(raw) && isJSONSpace(raw[k]) {
			k++
		}
		if k < len(raw) && raw[k] == ':' {
			if escaped {
				return false
			}
			if keyMaskAction(string(raw[i+1:j]), extra) != desensitize.KeepAsIs {
				return false
			}
		}

		i = j + 1
	}

	// 扫描没找到需要脱敏的键 最后确认报文合法：不合法的话 Mask 会退回 maskText，
	// 而 maskText 的判定规则与这里的 JSON 键扫描不同，等价性不成立
	// [EN] The scan found nothing to mask; the last gate confirms the body is valid, because an
	// invalid one sends Mask to maskText whose rules differ from this JSON key scan
	return json.Valid(raw)
}

// isJSONSpace JSON 允许的四种空白
func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// bodyMaskLimit 允许走快路径的报文长度上限
//
// 取「配置里的 max-body-size」与「库的默认值」中更小的那个，因为快路径必须保证
// Mask 不会截断，而脱敏器的实际上限只有三种来源：
//   - 网关自己建的那个（server/http.go:initDataMasker）—— 用的就是 logging.max-body-size
//   - activeMasker 的兜底实例 —— 用库的默认值
//   - 调用方手工赋给 global.DATAMASKER 的 —— 未知，而 MaxBodySize 没有 getter 能读出来
//     它的上限若比上面两个都小，这道门就比它的真实上限松：干净报文走快路径原样留存，
//     而 Mask 会先截断，半截 JSON 反序列化失败后退回 maskText，于是按 key=value
//     的文本形态擦到值里面去 这是快路径与 Mask 唯一不等价的情形，
//     且只在「Mask 因截断而退化成文本脱敏」时出现 —— 网关自建的脱敏器落不到这里，
//     因为它与这道门用的是同一个 logging.max-body-size
//
// 取更小值的方向是「宁可多算一次」，与整个脱敏路径的取向一致
//
// 另外上游早就把报文截到 logging.max-body-size 了（LoggingMiddleware 里的
// io.LimitReader 与 EnableBodyCaptureLimit），所以这道门在常态下根本不会挡住谁 ——
// 它的作用是让「Mask 不截断」这个前提可证，而不是真去拦什么
//
// [EN] The smaller of the configured max-body-size and the library default, because the fast
// path must guarantee Mask will not truncate. A masker's real limit has only three origins: the
// one the gateway builds (from logging.max-body-size), activeMasker's fallback (the library
// default), or one assigned to global.DATAMASKER by hand -- unknown, since MaxBodySize has no
// getter. Were that one's limit below both, this gate would be looser than the real thing: a
// clean body takes the fast path verbatim while Mask truncates, fails to unmarshal half a
// document and falls back to maskText, which redacts into values by their key=value text shape.
// That is the one case where the fast path is not equivalent to Mask, and it arises only when
// truncation has degraded Mask into text masking; the masker the gateway builds cannot reach it,
// because it and this gate share the same logging.max-body-size.
//
// Upstream has already capped the body at logging.max-body-size anyway (the io.LimitReader and
// EnableBodyCaptureLimit in LoggingMiddleware), so in practice this gate stops nobody; its job
// is to make "Mask will not truncate" provable
func bodyMaskLimit() int {
	limit := defaultMaskerMaxBodySize
	if cfg := getLoggingConfig(); cfg != nil && cfg.MaxBodySize > 0 && cfg.MaxBodySize < limit {
		limit = cfg.MaxBodySize
	}
	return limit
}
