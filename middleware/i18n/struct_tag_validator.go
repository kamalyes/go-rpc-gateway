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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
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
}

// lookupGatewayMessageType 根据 HTTP 方法和路径查找注册的消息工厂
func lookupGatewayMessageType(method string, path string) (func() any, bool) {
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
	var best func() any
	bestScore := -1
	for k, fn := range gatewayMessageTypeRegistry.data {
		score, ok := matchRouteTemplate(k, key)
		if ok && score > bestScore {
			best, bestScore = fn, score
		}
	}
	if best != nil {
		return best, true
	}
	return nil, false
}

// matchRouteTemplate 比对「METHOD /a/{x}/b」模板与「METHOD /a/1/b」实际路径
//
// 返回的分数是模板里字面段的数量：段数相同的候选中，字面段越多越具体
// 例如 /plans/{id}/entitlements 比 /plans/{id} 更具体，前者得分更高
//
// [EN] The score counts literal segments, so among candidates with the same segment count the
// more literal one wins: /plans/{id}/entitlements outranks /plans/{id}.
func matchRouteTemplate(template, actual string) (int, bool) {
	tm, tp, ok := splitMethodPath(template)
	if !ok {
		return 0, false
	}
	am, ap, ok := splitMethodPath(actual)
	if !ok || tm != am {
		return 0, false
	}
	ts := strings.Split(strings.Trim(tp, "/"), "/")
	as := strings.Split(strings.Trim(ap, "/"), "/")
	if len(ts) != len(as) {
		return 0, false
	}
	score := 0
	for i, seg := range ts {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			// 占位段匹配任意非空实际段：空段说明路径里少了一级
			// [EN] A placeholder matches any non-empty segment; an empty one means a missing level
			if as[i] == "" {
				return 0, false
			}
			continue
		}
		if seg != as[i] {
			return 0, false
		}
		score++
	}
	return score, true
}

// splitMethodPath 拆出 "METHOD /path" 里的方法与路径
func splitMethodPath(s string) (string, string, bool) {
	idx := strings.IndexByte(s, ' ')
	if idx <= 0 || idx+1 >= len(s) {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
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
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}

			// 回放请求体，确保后续 handler 仍可读取
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// 空请求体跳过校验（GET 请求等）
			if len(bodyBytes) == 0 {
				next(w, r, pathParams)
				return
			}

			// 创建 proto 消息实例并反序列化
			msg := newMsg()
			inboundMarshaler, _ := runtime.MarshalerForRequest(runtime.NewServeMux(), r)
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
				runtime.HTTPError(r.Context(), runtime.NewServeMux(), inboundMarshaler, w, r,
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
