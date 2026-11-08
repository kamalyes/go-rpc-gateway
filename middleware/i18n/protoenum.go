/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-08 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\i18n\protoenum.go
 * @Description: 引入 protobuf 枚举校验规则，使 validate:"protoenum" 对所有服务开箱可用
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package i18n

// 空导入：把 protoenum 规则注册进 argus 的包级规则表
//
// 为什么由网关来引入，而不是让每个服务自己写一遍：
//
// protobuf 枚举字段在 Go 里就是一个 int32，赋一个协议里没声明过的数字编译完全
// 合法。要在校验层拦住它，需要一条按 protoreflect 描述符判定的规则，而那条规则
// 必须注册进 argus 的包级表 —— 执行校验的是本包里一个 sync.Once 的包私有单例，
// 没有导出入口，实例级的 RegisterValidation 够不着它。
//
// 这件事此前是每个服务在自己的 validate/ 包里写一遍：十来行代码、N 份副本，
// 各自漂移，而漏写的那个服务不会报错，只是那些字段从此不校验。规则由网关引入
// 之后，凡是用到网关中间件的服务都自动具备 —— 而本包正是 middleware 包 import
// 的，所以不需要任何额外的空导入。
//
// 规则本身放在 go-toolbox 的子模块里而不是 argus 包内：argus 在 go-toolbox 的根
// 模块，而根模块不依赖 protobuf，写进去会把 protobuf 拖进每一个 go-toolbox 使用者。
// pkg/protox、pkg/pbmo 已经确立了「需要 protobuf 的东西各自成子模块」的做法。
//
// [EN] A blank import registering the protoenum rule into argus's package-level table.
//
// Why the gateway brings it in rather than each service writing it again:
//
// A protobuf enum field is an int32 in Go, and assigning a number the contract never
// declared compiles perfectly well. Catching that in the validation layer needs a rule that
// decides from the protoreflect descriptor, and that rule must go into argus's package-level
// table — the validator at work here is a sync.Once package-private singleton with no
// exported accessor, so instance-level RegisterValidation cannot reach it.
//
// Until now every service wrote this in its own validate/ package: a dozen lines copied N
// times, drifting apart, and the service that forgot raised no error — those fields simply
// stopped being validated. With the gateway importing it, every service using the gateway's
// middleware has it, and since the middleware package imports this one no extra blank import
// is needed anywhere.
//
// The rule itself lives in a go-toolbox submodule rather than inside argus: argus is in
// go-toolbox's root module, which does not depend on protobuf, and putting it there would
// pull protobuf into every consumer of go-toolbox. pkg/protox and pkg/pbmo already
// established that protobuf-dependent code is carved into submodules.
import _ "github.com/kamalyes/go-toolbox/pkg/argus/protoenum"
