/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\violations.go
 * @Description: 逐字段违规项收集器，供标签表达不了的业务规则校验使用
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package errors

import (
	"context"
	"strconv"
	"sync/atomic"
)

// ViolationLogger 记录违规明细所需的最小日志接口
//
// 为什么不直接调 global.GetLogger()：那一行会把 global 拖进来，而 global 依赖 cpool，
// 于是一个只做错误码映射与违规收集的包要连带编译 MySQL 驱动、Redis、NATS、MQTT 与
// ClickHouse —— 实测传递依赖 404 个包。错误码定义是最底层的契约，应该是谁都能引用它，
// 而不是它反过来引用连接池；这个方向错了，上游想单独用错误码就得背上整个网关
//
// 所以这里只声明实际用到的那一个方法。go-toolbox 的 logger.ILogger 天然满足它，
// 由 global 在给 LOGGER 赋值时反向注入，依赖方向就正过来了
//
// [EN] The minimal logging interface this package needs for violation details.
//
// Calling global.GetLogger() would pull in global, which depends on cpool, so a package whose
// job is error-code mapping would compile the MySQL driver, Redis, NATS, MQTT and ClickHouse —
// 404 transitive packages, measured. Error-code definitions are the lowest-layer contract:
// everything should be able to reference them, not the reverse. Only the one method actually
// used is declared here; go-toolbox's logger.ILogger satisfies it structurally and global
// injects it when assigning LOGGER, which puts the dependency direction the right way round
type ViolationLogger interface {
	WarnContextKV(ctx context.Context, msg string, keysAndValues ...interface{})
}

// violationLogger 存指针而不是接口值：atomic.Value 不接受 nil 接口，
// 而热重载会把日志器换掉，读写必须是原子的
// [EN] A pointer, not an interface value: atomic.Value rejects a nil interface, and hot
// reload swaps the logger, so reads and writes have to be atomic
var violationLogger atomic.Pointer[ViolationLogger]

// SetViolationLogger 注入违规明细的日志出口；传 nil 表示不记录
//
// 网关在初始化与热重载时调用它。未注入时明细不进日志，但错误照常返回 ——
// 校验结果不依赖日志是否接好
//
// [EN] Injects the sink for violation details; nil means do not log. The gateway calls this on
// init and on hot reload. Without it the details are not logged but the error is still
// returned: validation does not depend on logging being wired
func SetViolationLogger(l ViolationLogger) {
	if l == nil {
		violationLogger.Store(nil)
		return
	}
	violationLogger.Store(&l)
}

// currentViolationLogger 取当前注入的日志器，未注入返回 nil
// [EN] Returns the injected logger, or nil when none is wired
func currentViolationLogger() ViolationLogger {
	if p := violationLogger.Load(); p != nil {
		return *p
	}
	return nil
}

// 这里收的是「struct tag 表达不了的」那部分校验结果
//
// 分工：能用标签说清的（必填、长度、区间、格式、枚举合法性）交给 proto 上的
// `validate:"..."`，由本仓库的校验拦截器统一执行，业务代码里不该再写一遍
// `if req.GetXxx() == ""`。留给调用方自己判的是跨字段、跨记录、要查库、或者
// 一条规则牵涉整个集合的那些：阶梯价要逐档递增、同一资源不能重复配价。
//
// 为什么放在本仓库而不是每个服务各写一份：这段代码此前是模板的一部分，于是
// 每个生成出来的服务里都有一份一模一样的收集器。十几行代码乘以 N 个服务，
// 各自漂移，而「哪一份是对的」没人说得清。它依赖的 ToGRPCError 就在本包，
// 住在一起才是它该在的位置。
//
// [EN] What is collected here is the part of validation a struct tag cannot express.
//
// Anything a tag can state — required, length, range, format, enum validity — belongs on the
// proto as `validate:"..."` and is enforced by this repository's interceptor, never rewritten
// as an if in the service. What is left to the caller is cross-field, cross-record,
// database-dependent, or spans a whole collection.
//
// Why it lives here rather than once per service: this used to be part of the scaffold, so
// every generated service carried an identical copy of the collector. A dozen lines times N
// services, drifting apart, with nobody able to say which copy is right. The ToGRPCError it
// depends on is in this very package, which is where it belongs.

// Violation 一条违规项
//
// Rule 是机器可读的稳定串，前端按它定位到具体输入框；Msg 可本地化，Rule 不变。
// 两者分开是因为文案会改、会翻译，而前端的定位逻辑不该跟着文案走
//
// [EN] Rule is a stable machine-readable id the frontend locates the input by, while Msg is
// localizable. They are separate because copy changes and gets translated, and the frontend's
// targeting must not follow the copy
type Violation struct {
	Field string // 字段路径，如 tiers[2].minAmount | [EN] field path
	Rule  string // 规则标识，前端据此定位 | [EN] rule id the frontend targets by
	Msg   string // 可本地化的说明 | [EN] localizable message
}

// Violations 违规项收集器
//
// 收集而不是遇错即返：一次提交里有三处填错时，逐条返回会让使用者改一处、提交一次、
// 再被拒一次。把一整轮的问题一次说完，是校验层对使用者最基本的礼貌
//
// [EN] Violations collects rather than returning on the first error.
//
// With three mistakes in one submission, returning one at a time makes the user fix one,
// submit, and be rejected again. Stating a whole round's problems at once is the most basic
// courtesy a validation layer owes
type Violations struct {
	items []Violation
}

// NewViolations 新建一个收集器 | [EN] NewViolations creates a collector
func NewViolations() *Violations { return &Violations{} }

// Add 记一条违规 | [EN] Add records one violation
func (v *Violations) Add(field, rule, msg string) {
	v.items = append(v.items, Violation{Field: field, Rule: rule, Msg: msg})
}

// Addf 记一条集合内某一项的违规，字段路径自动带上下标
// [EN] Addf records a violation inside a collection, indexing the field path automatically
func (v *Violations) Addf(prefix string, i int, field, rule, msg string) {
	v.Add(IndexedField(prefix, i, field), rule, msg)
}

// Has 是否有违规 | [EN] Has reports whether anything was collected
func (v *Violations) Has() bool { return len(v.items) > 0 }

// Items 取全部违规项 | [EN] Items returns every violation
func (v *Violations) Items() []Violation { return v.items }

// Err 把收集到的违规项转成 gRPC 错误
//
// 明细先进日志：gRPC 的错误详情要走 status.WithDetails，而本仓库的错误统一由
// ToGRPCError 构造、由网关包络成 {code,msg,data}。两套机制混用会让同一个接口有
// 两种错误形状，前端得各写一套解析。要把明细带到前端（界面按字段高亮），正确做法
// 是给错误包络加一个 data 透传口子，而不是在这里绕过 ToGRPCError 自己拼 status
// —— 那样做出来的错误不会被 i18n 与脱敏中间件处理到
//
// [EN] Err turns the collected violations into a gRPC error.
//
// The details go to the log first: gRPC error details travel through status.WithDetails while
// this repository's errors are built by ToGRPCError and enveloped by the gateway as
// {code, msg, data}. Mixing the two gives one endpoint two error shapes and forces the
// frontend to parse both. Carrying them to the UI properly means adding a data passthrough to
// the envelope, not hand-rolling a status here — one built that way would bypass the i18n and
// redaction middleware
func (v *Violations) Err(ctx context.Context, bizCode string) error {
	if logger := currentViolationLogger(); logger != nil {
		for _, item := range v.items {
			logger.WarnContextKV(ctx, "业务规则校验未通过 | business rule validation failed",
				"field", item.Field, "rule", item.Rule, "msg", item.Msg)
		}
	}
	return ToGRPCError(ctx, bizCode)
}

// IndexedField 拼出带下标的字段路径，如 tiers[2].minAmount
//
// 前端要靠这个路径定位到具体那一行的那个输入框，所以格式必须稳定
//
// [EN] IndexedField builds an indexed field path such as tiers[2].minAmount.
//
// The frontend locates one input in one row by this path, so the format has to stay stable
func IndexedField(prefix string, i int, field string) string {
	return prefix + "[" + strconv.Itoa(i) + "]." + field
}
