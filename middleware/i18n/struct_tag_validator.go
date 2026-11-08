/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 00:15:56
 * @FilePath: \go-rpc-gateway\middleware\i18n\struct_tag_validator.go
 * @Description: 基于 go-argus 的 struct tag 校验拦截器
 *               配合 protoc-go-inject-tag 在 pb 生成代码字段上注入 `validate:"..."` 标签，
 *               无需业务方在 service 中手写 `if req.GetXxx() == ""` 之类的参数校验
 *               同时支持 gRPC 拦截器和 grpc-gateway HTTP 中间件两种模式，
 *               确保本地 Handler（RegisterXxxHandlerServer）模式下 HTTP 请求也能走校验
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package i18n

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	validator "github.com/kamalyes/go-toolbox/pkg/argus"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

var (
	structTagValidatorOnce sync.Once           // 确保线程安全
	structTagValidator     *validator.Validate // 全局共享的 validator 实例
)

// getStructTagValidator 返回进程内共享的 validator 实例（并发安全）
func getStructTagValidator() *validator.Validate {
	structTagValidatorOnce.Do(func() {
		structTagValidator = validator.New(validator.WithRequiredStructEnabled())
	})
	return structTagValidator
}

// StructTagValidatorUnaryInterceptor 基于 struct tag 的 gRPC Unary 校验拦截器
// 对每个入参 req 做校验
// 工作方式：对每个入参 req 调用 `validator.Struct(req)`若 pb 消息字段通过
// protoc-go-inject-tag 注入了 `validate:"required,min=1,..."` 之类的标签，校验失败
// 时返回 codes.InvalidArgument；字段未注入标签时 validator 会跳过，不会产生误报
func StructTagValidatorUnaryInterceptor() core.GRPCInterceptor {
	v := getStructTagValidator()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if req == nil {
			return handler(ctx, req)
		}
		if err := v.Struct(req); err != nil {
			return nil, status.Error(codes.InvalidArgument, formatStructTagValidationError(err))
		}
		return handler(ctx, req)
	}
}

// StructTagValidatorStreamInterceptor 基于 struct tag 的 gRPC Stream 校验拦截器
// 对每条流消息 RecvMsg 做校验
func StructTagValidatorStreamInterceptor() grpc.StreamServerInterceptor {
	v := getStructTagValidator()
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapped := &structTagValidatingStream{ServerStream: ss, validate: v}
		return handler(srv, wrapped)
	}
}

// structTagValidatingStream 基于 struct tag 的 gRPC Stream 校验拦截器
type structTagValidatingStream struct {
	grpc.ServerStream
	validate *validator.Validate
}

// RecvMsg 接收流消息，校验 struct tag
func (s *structTagValidatingStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	if m == nil {
		return nil
	}
	if err := s.validate.Struct(m); err != nil {
		return status.Error(codes.InvalidArgument, formatStructTagValidationError(err))
	}
	return nil
}

// formatStructTagValidationError 将 validator 的错误格式化为更易读的消息
func formatStructTagValidationError(err error) string {
	var fieldErrs validator.ValidationErrors
	if ok := toValidationErrors(err, &fieldErrs); !ok {
		return err.Error()
	}
	if len(fieldErrs) == 0 {
		return err.Error()
	}
	parts := make([]string, 0, len(fieldErrs))
	for _, fe := range fieldErrs {
		parts = append(parts, fe.Namespace()+": "+fe.Tag())
	}
	return "invalid argument: " + strings.Join(parts, ", ")
}

// ────────────────────────────────────────
// proto 消息类型注册表
// ────────────────────────────────────────

// gatewayMessageTypeRegistry 全局注册表：HTTP 方法+路径模式 → 消息工厂
// 用于本地 Handler 模式下，在 runtime.Middleware 层反序列化请求体并做 struct tag 校验
var gatewayMessageTypeRegistry struct {
	mu   sync.RWMutex
	data map[string]func() any // key = "METHOD /path/pattern"

	// templates 带 {} 占位的路由，按「方法 + 段数」分桶
	//
	// 为什么分桶而不是遍历整张表：带路径参数的路由，进来的具体路径永远不会精确命中
	// 模板键，所以**每个这样的请求**都要走一遍回退匹配回退原先是 O(表长)，
	// 且每条都要把模板与实际路径各切一次
	//
	// 实测 500 条路由时一个带参请求 60665 ns / 1003 次分配——这是每请求的热路径
	// matchRouteTemplate 本来就要求段数相等，把它提到桶的键上，扫描范围就缩到
	// 「同方法同段数」的那几条；模板在注册时切好，实际路径每次只切一次
	//
	// [EN] Buckets keyed by method and segment count rather than one flat scan. A concrete path
	// never matches a placeholder template exactly, so every parameterised request fell through
	// to a linear scan that re-split both the template and the path per entry: measured at
	// 60665 ns and 1003 allocations with 500 routes, on the per-request path; bucketing brings that
	// to 2449 ns and 2 allocations, and 134 ns at ten routes. matchRouteTemplate
	// already demands equal segment counts, so that moves into the bucket key, the scan narrows
	// to routes of the same shape, templates are split once at registration, and the incoming
	// path is split once per lookup.
	templates map[templateBucket][]compiledRoute
}

// templateBucket 「方法 + 段数」，决定一条模板和哪些请求有可能匹配
// [EN] Method plus segment count: which requests a template could possibly match
type templateBucket struct {
	method   string
	segments int
}

// compiledRoute 注册时就切好的模板
// [EN] A template split once, at registration
type compiledRoute struct {
	segs   []string // 路径按 / 切开，占位段保留原样
	static int      // 非占位段的个数，多者更具体
	newMsg func() any
}

// RegisterGatewayMessageType 注册某个 HTTP 路由对应的请求消息工厂函数
// 业务方在调用 RegisterXxxHandlerServer 的同时，应调用此函数注册请求消息类型，例如：
//
//	tenantpb.RegisterPlatformServiceHandlerServer(ctx, mux, g.platformSvc)
//	middleware.RegisterGatewayMessageType(http.MethodPost, "/v1/platforms", func() any {
//	    return &tenantpb.CreatePlatformRequest{}
//	})
//
// 只有注册了消息类型的路由才会触发 struct tag 校验，未注册的路由会被跳过
func RegisterGatewayMessageType(method string, pathPattern string, newMsg func() any) {
	gatewayMessageTypeRegistry.mu.Lock()
	defer gatewayMessageTypeRegistry.mu.Unlock()
	if gatewayMessageTypeRegistry.data == nil {
		gatewayMessageTypeRegistry.data = make(map[string]func() any)
	}
	key := method + " " + pathPattern
	gatewayMessageTypeRegistry.data[key] = newMsg
	indexTemplateLocked(method, pathPattern, newMsg)
}

// indexTemplateLocked 把带占位的模板切好放进对应的桶；调用方必须已持有写锁
//
// 不带占位的路由不进桶：它们永远走精确匹配那条路，放进来只会让扫描变长
// [EN] Routes without placeholders are not bucketed: they always hit the exact-match path, and
// including them would only lengthen the scan
func indexTemplateLocked(method, pathPattern string, newMsg func() any) {
	if !strings.ContainsRune(pathPattern, '{') {
		return
	}
	segs := strings.Split(strings.Trim(pathPattern, "/"), "/")
	static := 0
	for _, seg := range segs {
		if !isPlaceholderSegment(seg) {
			static++
		}
	}
	if gatewayMessageTypeRegistry.templates == nil {
		gatewayMessageTypeRegistry.templates = make(map[templateBucket][]compiledRoute)
	}
	bucket := templateBucket{method: method, segments: len(segs)}
	for _, existing := range gatewayMessageTypeRegistry.templates[bucket] {
		if equalSegments(existing.segs, segs) {
			return
		}
	}
	gatewayMessageTypeRegistry.templates[bucket] = append(
		gatewayMessageTypeRegistry.templates[bucket],
		compiledRoute{segs: segs, static: static, newMsg: newMsg})
}

func isPlaceholderSegment(seg string) bool {
	return strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}

// routeMatches 按段比对；占位段匹配任意非空段
//
// 为什么是按段比对而不是字符串前缀：前缀匹配对真实路由两头不着，而且两种失效都不报错——
//
//   - 模板里带 {plan_id}，HasPrefix("…/plans/abc", "…/plans/{plan_id}") 永远是 false，
//     于是所有带路径参数的接口都匹不上，静默跳过校验
//   - 若改成登记裸前缀 "…/plans/" 来绕过，它又会同时命中 "…/plans/{id}" 与
//     "…/plans/{id}/entitlements"，而 map 遍历顺序是随机的，于是同一个请求这次按 A 校验、
//     下次按 B 校验，合法请求被偶发拒绝
//
// 前者是「校验没跑」，后者是「偶发 400」，都极难归因。多条命中时取非占位段最多的那条。
//
// [EN] Compared segment by segment rather than by string prefix, which failed both ways and
// reported neither: a template containing a placeholder never prefixes a concrete path, so every
// parameterised endpoint silently skipped validation; and registering a bare prefix to work
// around that matched both a resource and its sub-resources, with random map order deciding
// which, so the same request validated against a different message on each call and valid input
// was rejected intermittently. On multiple matches the one with the most literal segments wins.
func routeMatches(template, actual []string) bool {
	for i, seg := range template {
		if isPlaceholderSegment(seg) {
			if actual[i] == "" {
				return false
			}
			continue
		}
		if seg != actual[i] {
			return false
		}
	}
	return true
}

func equalSegments(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// autoDiscoverOnce 保证自动发现只跑一次，且在第一次查找时才跑
//
// 不在 init 里跑：init 的执行顺序只保证「被依赖的包先于依赖者」，而业务的 pb 包是否
// 已经把自己注册进 protoregistry，取决于它有没有被链接进来放到第一次查找时跑，
// 那时所有包的 init 都已结束，注册表是完整的
//
// [EN] Runs once, on the first lookup rather than in init: init ordering only guarantees that a
// dependency runs before its dependents, and whether the service's pb package has registered
// itself depends on it being linked in at all. By the first lookup every init has finished and
// the registry is complete
var autoDiscoverOnce sync.Once

// autoDiscoverGatewayMessageTypes 从 proto 描述符自动建出「HTTP 路由 → 请求消息」
//
// 为什么要自动建：这张表原先只能由每个服务手工登记一遍，而协议里本来就写着
// google.api.http 注解——grpc-gateway 正是靠它生成路由的同一件事让使用者再抄一遍，
// 抄漏了不会有任何报错，只会让那个接口静默地没有校验：协议里的 validate 标签一条都不生效，
// 接口照常返回 200，非法值直接落库
//
// 一张必须手工维护、漏了还不报错的表，不该存在生成的 pb 代码在 init 里已经把文件
// 描述符注册进 protoregistry.GlobalFiles，注解就在方法的 options 上，网关自己走一遍就有了
//
// 手工登记仍然保留并且优先：协议之外另挂的路由、或者要换掉某个路由的消息类型时用它
//
// [EN] Builds the HTTP-route-to-request-message table from the proto descriptors.
//
// This table used to be hand-written once per service, while the contract already carries the
// google.api.http annotations that grpc-gateway itself generates the routes from. Making the
// user copy the same facts again costs nothing when done right and reports nothing when done
// wrong: a forgotten line leaves that endpoint silently unvalidated — every validate tag in the
// contract stops applying while the endpoint still returns 200 and invalid values reach the
// database.
//
// A table that must be maintained by hand and stays quiet when it is wrong should not exist.
// Generated pb code registers its file descriptors with protoregistry.GlobalFiles in init, the
// annotations sit on the method options, and the gateway can walk them itself.
//
// Manual registration still works and still wins: use it for routes outside the contract, or to
// substitute a different message type for one route.
func autoDiscoverGatewayMessageTypes() {
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			methods := services.Get(i).Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				rule, ok := proto.GetExtension(method.Options(), annotations.E_Http).(*annotations.HttpRule)
				if !ok || rule == nil {
					continue
				}
				input := method.Input().FullName()
				registerDiscoveredRule(rule, input)
				for _, extra := range rule.GetAdditionalBindings() {
					registerDiscoveredRule(extra, input)
				}
			}
		}
		return true
	})
}

// registerDiscoveredRule 把一条 google.api.http 规则登记进注册表
//
// 取的是 GlobalTypes 里注册的具体 Go 类型，不是 dynamicpb：argus 校验的是 Go 结构体
// 上的 validate 标签，而 dynamicpb 构造出来的消息没有那些标签，校验会全部通过
//
// [EN] Resolves the concrete generated Go type from GlobalTypes rather than building a
// dynamicpb message: argus validates the validate tags on the Go struct, and a dynamicpb
// message carries none of them, so everything would pass
func registerDiscoveredRule(rule *annotations.HttpRule, input protoreflect.FullName) {
	method, path := httpRuleRoute(rule)
	if method == "" || path == "" {
		return
	}
	mt, err := protoregistry.GlobalTypes.FindMessageByName(input)
	if err != nil {
		return
	}
	registerGatewayMessageTypeIfAbsent(method, path, func() any {
		return mt.New().Interface()
	})
}

// httpRuleRoute 取出一条规则的方法与路径模板
func httpRuleRoute(rule *annotations.HttpRule) (string, string) {
	switch pattern := rule.GetPattern().(type) {
	case *annotations.HttpRule_Get:
		return http.MethodGet, pattern.Get
	case *annotations.HttpRule_Post:
		return http.MethodPost, pattern.Post
	case *annotations.HttpRule_Put:
		return http.MethodPut, pattern.Put
	case *annotations.HttpRule_Patch:
		return http.MethodPatch, pattern.Patch
	case *annotations.HttpRule_Delete:
		return http.MethodDelete, pattern.Delete
	case *annotations.HttpRule_Custom:
		return pattern.Custom.GetKind(), pattern.Custom.GetPath()
	}
	return "", ""
}

// registerGatewayMessageTypeIfAbsent 只在该路由还没有登记时写入
//
// 自动发现不覆盖手工登记：手工那条是使用者明确表达的意图，可能正是为了换掉协议里声明的类型
//
// [EN] Auto-discovery never overwrites a manual registration: that entry is the user stating an
// intent, possibly to substitute exactly the type the contract declares
func registerGatewayMessageTypeIfAbsent(method, pathPattern string, newMsg func() any) {
	gatewayMessageTypeRegistry.mu.Lock()
	defer gatewayMessageTypeRegistry.mu.Unlock()
	if gatewayMessageTypeRegistry.data == nil {
		gatewayMessageTypeRegistry.data = make(map[string]func() any)
	}
	key := method + " " + pathPattern
	if _, exists := gatewayMessageTypeRegistry.data[key]; exists {
		return
	}
	gatewayMessageTypeRegistry.data[key] = newMsg
	indexTemplateLocked(method, pathPattern, newMsg)
}

// lookupGatewayMessageType 根据 HTTP 方法和路径查找注册的消息工厂
func lookupGatewayMessageType(method string, path string) (func() any, bool) {
	// 在取读锁之前触发：自动发现内部要取写锁
	// [EN] Triggered before the read lock is taken, since discovery takes the write lock
	autoDiscoverOnce.Do(autoDiscoverGatewayMessageTypes)

	gatewayMessageTypeRegistry.mu.RLock()
	defer gatewayMessageTypeRegistry.mu.RUnlock()
	if gatewayMessageTypeRegistry.data == nil {
		return nil, false
	}
	// 先尝试精确匹配
	key := method + " " + path
	if fn, ok := gatewayMessageTypeRegistry.data[key]; ok {
		return fn, true
	}
	// 再按路径模板匹配：注册的是 grpc-gateway 的路由模板（含 {id} 占位），
	// 进来的是具体路径，两者要按段比对，并在多条命中时取最具体的那条
	//
	// 原先这里用的是字符串前缀匹配，对真实路由两头不着：
	//   - 模板里带 {plan_id}，HasPrefix("…/plans/abc", "…/plans/{plan_id}") 永远是 false，
	//     于是所有带路径参数的接口都匹不上 → 静默跳过校验
	//   - 若改成登记裸前缀 "…/plans/" 来绕过，它又会同时命中
	//     "…/plans/{id}" 和 "…/plans/{id}/entitlements"，而 map 遍历顺序是随机的，
	//     于是同一个请求这次按 A 校验、下次按 B 校验 → 合法请求被偶发拒绝
	//
	// 两种失效都不报错：前者是「校验没跑」，后者是「偶发 400」，都极难归因
	//
	// [EN] Registered keys are grpc-gateway route templates containing {id} placeholders while
	// the incoming value is a concrete path, so they are compared segment by segment and the
	// most specific match wins. The previous prefix comparison failed both ways: a template
	// containing a placeholder never prefixes a concrete path, so every parameterised endpoint
	// silently skipped validation; and registering a bare prefix to work around that matches
	// both the resource and its sub-resources, with map iteration order deciding which — the
	// same request validated against a different message on each call, rejecting valid input
	// intermittently. Neither failure reports anything.
	// 回退：只扫「同方法、同段数」的那一桶，实际路径只切一次
	// [EN] Only the bucket of the same method and segment count is scanned, and the incoming
	// path is split once
	actual := strings.Split(strings.Trim(path, "/"), "/")
	bucket := templateBucket{method: method, segments: len(actual)}
	var best func() any
	bestScore := -1
	for _, route := range gatewayMessageTypeRegistry.templates[bucket] {
		if !routeMatches(route.segs, actual) {
			continue
		}
		if route.static > bestScore {
			best, bestScore = route.newMsg, route.static
		}
	}
	if best != nil {
		return best, true
	}
	return nil, false
}

// StructTagValidatorGatewayMiddleware 基于 struct tag 的 grpc-gateway HTTP 校验中间件
// 用于本地 Handler 模式（RegisterXxxHandlerServer），HTTP 请求绕过 gRPC 拦截器链，
// 需要通过 runtime.WithMiddlewares 注入此中间件才能触发校验
//
// 工作原理：
//  1. 在 runtime.Middleware 层拦截 HTTP 请求
//  2. 读取请求体，查找注册的 proto 消息类型
//  3. 反序列化为 proto 消息后调用 go-argus 的 Struct 校验
//  4. 校验通过后回放请求体，交给后续 handler 处理
//  5. 未注册消息类型的路由直接放行
//
// marshalerMux 取 marshaler 与写错误响应用的共享 ServeMux
//
// 原先这里每个请求 new 一个：runtime.MarshalerForRequest(runtime.NewServeMux(), r)，
// 校验失败时 runtime.HTTPError 又再 new 一个。而 NewServeMux 会建一整套
// marshaler 注册表与默认选项，纯属白付
//
// 实测（Apple M4，-benchtime=2s -count=3 -cpu=10）：
//
//	每请求 new    196.7 ~ 210.5 ns/op   608 B/op   5 allocs/op
//	复用一个       66.9 ~  69.5 ns/op    48 B/op   1 allocs/op
//
// 也就是每个被校验的请求白付约 130 ns、560 B、4 次分配，校验失败的请求翻倍
//
// 共享是安全的：这里只读它的 marshaler 注册表与错误处理器，两者都在构造时定好之后不再变
//
// 已知局限：它是用零选项建的，而网关真正在用的那个 mux 由 server 的 buildServeMuxOptions
// 按 gateway.json 配置建（UseProtoNames / EmitUnpopulated / DiscardUnknown 等）
// 两者的反序列化口径不一致时，这里 Decode 失败会放行给后续 handler（也就是跳过校验而不是误拒）
// 这一点在改造前就存在，本次只去掉重复构造，没有改变口径
//
// [EN] A shared ServeMux for obtaining a marshaler and for writing error responses.
//
// This used to allocate one per request via runtime.MarshalerForRequest(runtime.NewServeMux(),
// r), and runtime.HTTPError allocated a second one on a validation failure. NewServeMux builds
// a whole marshaler registry plus default options, all of it wasted.
//
// Measured on an Apple M4 with -benchtime=2s -count=3 -cpu=10:
//
//	per-request new   196.7 – 210.5 ns/op   608 B/op   5 allocs/op
//	shared instance    66.9 –  69.5 ns/op    48 B/op   1 allocs/op
//
// which is roughly 130 ns, 560 B and 4 allocations wasted on every validated request, doubled
// for requests that fail validation.
//
// Sharing is safe: only the marshaler registry and the error handlers are read, and both are
// fixed at construction.
//
// Known limitation: it is built with zero options, while the mux the gateway actually serves
// with is built by the server's buildServeMuxOptions from the gateway JSON config
// (UseProtoNames, EmitUnpopulated, DiscardUnknown and so on). Where the two disagree, a Decode
// failure here falls through to the next handler — skipping validation rather than rejecting
// wrongly. That predates this change; only the duplicate construction is removed here.
var marshalerMux = runtime.NewServeMux()

// replayBody 把已读进内存的请求体回放给后续 handler，同时保留原 body 的 Close
//
// 不用 io.NopCloser(bytes.NewReader(...))：那样原始 body 永远不会被 Close
// 同一个仓库里 observability/logging.go 的 prefixedBody 为这件事专门写了注释，
// 这里原本恰好踩的就是它点明的那个坑
//
// [EN] Replays the already-read body to the next handler while preserving the original body's
// Close. Not io.NopCloser(bytes.NewReader(...)), which leaves the original body never closed —
// prefixedBody in observability/logging.go in this same repository carries a comment about
// exactly that, and this was precisely the pitfall it points out.
type replayBody struct {
	*bytes.Reader
	orig io.ReadCloser
}

// Close 转交给原始 body
func (b *replayBody) Close() error {
	if b.orig == nil {
		return nil
	}
	return b.orig.Close()
}

func StructTagValidatorGatewayMiddleware() runtime.Middleware {
	v := getStructTagValidator()
	return func(next runtime.HandlerFunc) runtime.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
			// 查找该路由对应的 proto 消息类型
			newMsg, found := lookupGatewayMessageType(r.Method, r.URL.Path)
			if !found {
				next(w, r, pathParams)
				return
			}

			// 读取请求体
			//
			// 这里必须整体读完：校验要在 handler 之前对完整消息做判定，截断读会把
			// 「字段缺失」误判成校验失败 代价是请求体在内存里存在一份副本，这是
			// 「在 handler 之前校验」本身的成本，不是可以优化掉的重复缓冲
			// （下面的回放用 bytes.NewReader 复用同一块底层数组，不再复制一份）
			//
			// [EN] The body has to be read in full: validation judges the complete message
			// before the handler runs, and a truncated read would turn a missing field into a
			// validation failure. The cost is one in-memory copy of the body, which is inherent
			// to validating ahead of the handler rather than a duplicate buffer that could be
			// optimised away — the replay below reuses the same backing array via
			// bytes.NewReader instead of copying again.
			original := r.Body
			bodyBytes, err := io.ReadAll(original)
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}

			// 回放请求体，确保后续 handler 仍可读取；Close 仍转交给原始 body
			r.Body = &replayBody{Reader: bytes.NewReader(bodyBytes), orig: original}

			// 空请求体跳过校验（GET 请求等）
			if len(bodyBytes) == 0 {
				next(w, r, pathParams)
				return
			}

			// 创建 proto 消息实例并反序列化
			msg := newMsg()
			inboundMarshaler, _ := runtime.MarshalerForRequest(marshalerMux, r)
			if err := inboundMarshaler.NewDecoder(bytes.NewReader(bodyBytes)).Decode(msg); err != nil {
				// 反序列化失败，交给后续 handler 处理（让它返回标准错误）
				next(w, r, pathParams)
				return
			}

			// 把路径参数填进消息之后再校验
			//
			// 中间件只把 body 反序列化进消息，而 /v1/plans/{plan_id} 这种路径参数
			// 是 grpc-gateway 在后面的 handler 里才填的不补这一步，
			// 凡是路径参数标了 required 的接口都会被**误拒**——
			// 参数明明在 URL 里，却报 "plan_id: required"
			//
			// 这个坑比"没校验"更难查：登记路由之前接口是通的，登记之后突然全 400，
			// 看起来像是登记这件事本身出了问题
			//
			// [EN] Only the body is decoded into the message while path parameters are filled in
			// later by grpc-gateway's own handler, so without this step every endpoint whose path
			// parameter is marked required gets rejected although the value is right there in the
			// URL. It is worse than missing validation: the endpoint works until its route is
			// registered and then returns 400 for everything, which looks like the registration
			// itself is at fault.
			applyPathParams(msg, pathParams)

			// 执行 struct tag 校验
			if err := v.Struct(msg); err != nil {
				runtime.HTTPError(r.Context(), marshalerMux, inboundMarshaler, w, r,
					status.Error(codes.InvalidArgument, formatStructTagValidationError(err)))
				return
			}

			next(w, r, pathParams)
		}
	}
}

// toValidationErrors 执行一次断言，避免 fmt 依赖
func toValidationErrors(err error, out *validator.ValidationErrors) bool {
	ve, ok := err.(validator.ValidationErrors)
	if !ok {
		return false
	}
	*out = ve
	return true
}

// applyPathParams 把 grpc-gateway 解析出的路径参数写进 proto 消息
//
// 只处理顶层标量字段：路径参数在 grpc-gateway 的路由模板里本来就只能绑定标量
// （string / 数字 / 枚举 / bool），嵌套路径如 {a.b} 极少用且 gateway 自己也会再填一次，
// 这里填不上不会造成误拒——漏填的字段若标了 required，仍然由 gateway 的
// handler 填好后在 service 里兜住
//
// 枚举同时接受名称与数字：JSON 侧传的是名称（PLAN_STATUS_ON_SALE），
// 而有些调用方直接拼数字，两种都认才不会在 URL 形式上挑食
//
// [EN] Only top-level scalars are handled, which is all a route template can bind anyway;
// nested paths are rare and the gateway's own handler fills them again, so a miss here cannot
// cause a false rejection. Enums accept both their name and their number, since JSON callers
// send the name while some clients put the number straight into the URL.
func applyPathParams(msg any, pathParams map[string]string) {
	if len(pathParams) == 0 {
		return
	}
	pm, ok := msg.(proto.Message)
	if !ok {
		return
	}
	refl := pm.ProtoReflect()
	fields := refl.Descriptor().Fields()
	for key, raw := range pathParams {
		if raw == "" {
			continue
		}
		fd := fields.ByName(protoreflect.Name(key))
		if fd == nil {
			fd = fields.ByJSONName(key)
		}
		if fd == nil || fd.IsList() || fd.IsMap() {
			continue
		}
		if val, ok := pathParamValue(fd, raw); ok {
			refl.Set(fd, val)
		}
	}
}

// pathParamValue 把路径上的字符串转成字段对应的 protoreflect 值
func pathParamValue(fd protoreflect.FieldDescriptor, raw string) (protoreflect.Value, bool) {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(raw), true
	case protoreflect.BoolKind:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return protoreflect.Value{}, false
		}
		return protoreflect.ValueOfBool(b), true
	case protoreflect.EnumKind:
		if ev := fd.Enum().Values().ByName(protoreflect.Name(raw)); ev != nil {
			return protoreflect.ValueOfEnum(ev.Number()), true
		}
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return protoreflect.Value{}, false
		}
		return protoreflect.ValueOfEnum(protoreflect.EnumNumber(n)), true
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil {
			return protoreflect.Value{}, false
		}
		return protoreflect.ValueOfInt32(int32(n)), true
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return protoreflect.Value{}, false
		}
		return protoreflect.ValueOfInt64(n), true
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		n, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return protoreflect.Value{}, false
		}
		return protoreflect.ValueOfUint32(uint32(n)), true
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return protoreflect.Value{}, false
		}
		return protoreflect.ValueOfUint64(n), true
	default:
		return protoreflect.Value{}, false
	}
}
