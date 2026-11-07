/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-26 12:05:06
 * @FilePath: \go-rpc-gateway\server\http.go
 * @Description: HTTP服务器和网关初始化模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/desensitize"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// buildServeMuxOptions 构建ServeMux选项，支持从配置文件读取JSON序列化配置
func (s *Server) buildServeMuxOptions() []runtime.ServeMuxOption {
	// 配置已通过 safe.MergeWithDefaults 合并默认值，直接使用
	useProtoNames := s.config.JSON.UseProtoNames
	emitUnpopulated := s.config.JSON.EmitUnpopulated
	discardUnknown := s.config.JSON.DiscardUnknown

	opts := []runtime.ServeMuxOption{
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   useProtoNames,   // 使用 proto 字段名（snake_case）
				EmitUnpopulated: emitUnpopulated, // 输出所有字段，包括零值
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: discardUnknown, // 忽略未知字段
			},
		}),
		// 🔑 将 HTTP Header 传递到 gRPC metadata（过滤 HTTP/2 禁止的头，避免 RST_STREAM PROTOCOL_ERROR）
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			// HTTP/2 规范禁止的头，转发这些头会导致 gRPC 服务端发送 RST_STREAM
			switch strings.ToLower(key) {
			case "connection", "keep-alive", "proxy-connection",
				"transfer-encoding", "upgrade", "te":
				return key, false
			case "authorization":
				// grpc-gateway 的 AnnotateContext 已对 Authorization 做无条件转发（向后兼容），
				// 此处再匹配会导致 metadata 中出现重复的 authorization 值，
				// 下游服务解析 token 失败 → 认证失败 → 无数据权限
				return key, false
			}
			return key, true
		}),
	}

	// 启用 Protobuf 响应支持（当 gRPC Server 配置了 EnableProtobufResp 时）
	if s.config.GRPC != nil && s.config.GRPC.Server != nil && s.config.GRPC.Server.EnableProtobufResp {
		opts = append(opts, runtime.WithMarshalerOption("application/x-protobuf", &protobufMarshaler{}))
		opts = append(opts, runtime.WithMarshalerOption("application/protobuf", &protobufMarshaler{}))
		global.LOGGER.InfoMsg("✅ Protobuf 响应格式已启用（支持 application/x-protobuf 和 application/protobuf）")
	}

	return opts
}

// protobufMarshaler 实现 runtime.Marshaler 接口，用于 protobuf 二进制序列化
type protobufMarshaler struct{}

func (m *protobufMarshaler) ContentType(v any) string {
	return "application/x-protobuf"
}

func (m *protobufMarshaler) Marshal(v any) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("protobufMarshaler: value is not a proto.Message, got %T", v)
	}
	return proto.Marshal(msg)
}

func (m *protobufMarshaler) Unmarshal(data []byte, v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("protobufMarshaler: value is not a proto.Message, got %T", v)
	}
	return proto.Unmarshal(data, msg)
}

func (m *protobufMarshaler) NewDecoder(r io.Reader) runtime.Decoder {
	return &protobufDecoder{r: r}
}

func (m *protobufMarshaler) NewEncoder(w io.Writer) runtime.Encoder {
	return &protobufEncoder{w: w}
}

type protobufDecoder struct {
	r io.Reader
}

func (d *protobufDecoder) Decode(v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("protobufDecoder: value is not a proto.Message, got %T", v)
	}
	data, err := io.ReadAll(d.r)
	if err != nil {
		return err
	}
	return proto.Unmarshal(data, msg)
}

type protobufEncoder struct {
	w io.Writer
}

func (e *protobufEncoder) Encode(v any) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("protobufEncoder: value is not a proto.Message, got %T", v)
	}
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = e.w.Write(data)
	return err
}

// initGzipWriterPool 初始化 Gzip writer 对象池与跳过规则（从配置读取）
// resolveGzipLevel 定出 gzip 压缩级别：配置里给了合法值就用它，否则回落默认
//
// 单独成函数是为了能直接断言「级别被正确读出来」这件事
// 原先这段逻辑埋在 initGzipWriterPool 里，想验证只能去压一段数据再比字节数，
// 而那种断言跨 Go 版本与平台都不稳：实测同一段高重复载荷在 macOS 上 1 档与 9 档有差，
// 在 Linux / Go 1.27 上两档都是 91 字节，于是测试在 CI 上判红而代码其实是对的
// flate 各级别对某些输入收敛成同一结果是允许的，不该被当成配置没生效
//
// [EN] Extracted so that "the level is read correctly" can be asserted directly. While this
// logic sat inside the pool constructor the only way to check it was to compress something and
// compare byte counts, and such an assertion is not stable across Go versions or platforms:
// the same highly repetitive payload differed between levels on macOS and came out identical
// at 91 bytes on Linux with Go 1.27, failing in CI while the code was correct. flate is
// allowed to converge on the same output for some inputs, which is no sign of a lost setting
func resolveGzipLevel(cfg *gwconfig.HTTPServer) int {
	level := gwconfig.DefaultHTTPServer().GzipCompressionLevel

	// 原先这里套了一层 TLS != nil 才读压缩级别，但 GzipCompressionLevel 是 HTTPServer
	// 下与 TLS 平级的字段，跟 TLS 没有任何关系
	// 于是没配 TLS 的部署里，用户写的 gzip-compression-level 被静默忽略、一律回落默认，
	// 而且不报错不告警
	//
	// [EN] The level used to be read only when TLS was configured, but GzipCompressionLevel
	// sits beside TLS under HTTPServer and has nothing to do with it. Without TLS the
	// configured level was therefore ignored in silence
	if cfg != nil {
		if configured := cfg.GzipCompressionLevel; configured > 0 && configured <= 9 {
			level = configured
		}
	}
	return level
}

func (s *Server) initGzipWriterPool() {
	compressionLevel := resolveGzipLevel(s.config.HTTPServer)

	// 创建对象池（在 Server 初始化时创建一次，供所有请求复用）
	s.gzipWriterPool = &sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(io.Discard, compressionLevel)
			return w
		},
	}

	// 上面刚判过 HTTPServer 可能为 nil，这里就不能再无条件解引用：
	// 原来的实现判完 nil 之后紧接着就读 s.config.HTTPServer.GzipSkipPaths，
	// HTTPServer 真是 nil 时直接 panic 在启动路径上
	// [EN] HTTPServer was just tested for nil, so it cannot be dereferenced unconditionally
	// here: the previous code did exactly that one line later and panicked during startup
	if s.config.HTTPServer == nil {
		return
	}

	s.gzipSkipPrefixes = append([]string(nil), s.config.HTTPServer.GzipSkipPaths...)
	s.gzipSkipSuffixes = append([]string(nil), s.config.HTTPServer.GzipSkipExtensions...)

	// 每一项都是「单个点开头」时才建扩展名表：这种形状下「取最后一段扩展名再查一次表」
	// 与「逐个 HasSuffix」结果完全一致
	// 一旦出现 .min.js 这类带两个点的配置，取扩展名只会得到 .js，两者就不等价了，
	// 那时宁可慢也要保语义，整张表不建，走下面的后缀循环
	//
	// [EN] The extension table is built only when every entry is a single dot-prefixed
	// suffix, the shape in which extracting the final extension and looking it up once is
	// exactly equivalent to testing each suffix. An entry such as .min.js breaks that
	// equivalence, because extraction yields only .js, so the table is skipped and the
	// suffix loop keeps the original meaning
	extMap := make(map[string]struct{}, len(s.gzipSkipSuffixes))
	for _, ext := range s.gzipSkipSuffixes {
		if len(ext) < 2 || ext[0] != '.' || strings.Count(ext, ".") != 1 {
			extMap = nil
			break
		}
		extMap[ext] = struct{}{}
	}
	s.gzipSkipExtMap = extMap
}

// gzipResponseWriter 包装ResponseWriter以支持gzip压缩
//
// Flusher / Hijacker / Pusher 必须一个个显式实现，不能指望嵌入带过来
// 嵌进来的是 http.ResponseWriter 接口，提升上来的方法集只有 Header / Write / WriteHeader，
// 所以包装之后 w.(http.Flusher) 断言不再成立 —— 而这正是 grpc-gateway 的
// ForwardResponseStream 的第一步，断言失败时它直接回 500 "unexpected type of web server"
// 也就是说开了 gzip，所有 server-streaming 接口对默认带 Accept-Encoding: gzip 的客户端
// （浏览器与多数 HTTP 客户端）一律 500，而不是「慢一点」
//
// 同一个仓库里的 core.ResponseWriter 就是显式实现这几个接口的，这里照它补齐
//
// [EN] Flusher, Hijacker and Pusher have to be implemented explicitly rather than inherited:
// what is embedded is the http.ResponseWriter interface, so the promoted method set is only
// Header, Write and WriteHeader, and w.(http.Flusher) stops asserting once wrapped. That
// assertion is the first thing grpc-gateway's ForwardResponseStream does, and on failure it
// answers 500 "unexpected type of web server" — so with gzip enabled every server-streaming
// endpoint returns 500 to any client sending Accept-Encoding: gzip, which is browsers and most
// HTTP clients, rather than merely running slower.
//
// core.ResponseWriter in this same repository implements them explicitly; this follows it.
type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter *gzip.Writer
	// hijacked 连接已被劫持，不要再通过原 ResponseWriter 写 gzip 尾块
	// [EN] The connection was hijacked; no gzip trailer through the original writer
	hijacked bool
}

// Write 写入压缩数据
func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.gzipWriter.Write(b)
}

// Close 关闭 gzip writer
//
// 劫持过的连接直接短路：此时原 ResponseWriter 已经交出控制权，
// 再写尾块只会拿到 http.ErrHijacked，并把一个已 Close 的 writer 放回池子
//
// [EN] A hijacked connection short-circuits: the original writer has handed over control, so
// writing the trailer only yields http.ErrHijacked and returns a closed writer to the pool
func (w *gzipResponseWriter) Close() error {
	if w.hijacked {
		return nil
	}
	return w.gzipWriter.Close()
}

// Flush 实现 http.Flusher 接口（流式响应）
//
// 顺序不能反：gzip 把数据攒在自己的 flate 缓冲里，只刷底层 writer 等于什么都没发出去，
// 流式响应会一路攒到 Close 才一次性到达客户端
//
// [EN] The order matters: gzip holds data in its own flate buffer, so flushing only the
// underlying writer sends nothing and a streaming response piles up until Close
func (w *gzipResponseWriter) Flush() {
	_ = w.gzipWriter.Flush()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack 实现 http.Hijacker 接口（WebSocket 升级需要裸连接）
func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	conn, buf, err := hijacker.Hijack()
	if err == nil {
		w.hijacked = true
	}
	return conn, buf, err
}

// Push 实现 http.Pusher 接口（HTTP/2 Server Push）
func (w *gzipResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Unwrap 返回底层的 http.ResponseWriter（供 http.ResponseController 使用）
func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// gzipEnabled 判断是否启用 gzip 压缩
//
// 判定自带 nil 保护：HTTPServer 是指针，整棵子树可能缺失（程序化构造配置时很容易不填），
// 而 gzipMiddleware 原先每个请求都裸解引用 s.config.HTTPServer.EnableGzipCompress
// 同一个文件里的 initGzipWriterPool 专门为这件事加过 nil 判断并写了注释 ——
// 同一个判定散在三处，就必然出现一处有保护、另两处没有的情况，所以这里收口成一个方法
//
// [EN] Nil-safe by construction: HTTPServer is a pointer and the whole subtree may be absent,
// which is easy to leave unset when the config is built programmatically, while gzipMiddleware
// used to dereference s.config.HTTPServer.EnableGzipCompress on every request.
// initGzipWriterPool in this same file already guards exactly that; one decision spelled out in
// three places is how two of them end up without the guard, so it is funnelled through here.
func (s *Server) gzipEnabled() bool {
	return s.config != nil && s.config.HTTPServer != nil && s.config.HTTPServer.EnableGzipCompress
}

// http2Enabled 判断是否启用 HTTP/2，与 gzipEnabled 同样自带 nil 保护
// [EN] Whether HTTP/2 is enabled, nil-safe like gzipEnabled
func (s *Server) http2Enabled() bool {
	return s.config != nil && s.config.HTTPServer != nil && s.config.HTTPServer.EnableHTTP2
}

// shouldSkipGzip 判断是否跳过 gzip 压缩
//
// 不命中才是所有 API 请求的常态，所以这条路径要按「全部比完」来算成本，
// 而不是按命中时提前返回来算
// [EN] A miss is the normal outcome for every API request, so this path has to be costed as
// the full set of comparisons rather than as an early return on a hit
func (s *Server) shouldSkipGzip(r *http.Request) bool {
	reqPath := r.URL.Path

	for _, prefix := range s.gzipSkipPrefixes {
		if strings.HasPrefix(reqPath, prefix) {
			return true
		}
	}

	if s.gzipSkipExtMap != nil {
		if ext := pathExt(reqPath); ext != "" {
			_, skip := s.gzipSkipExtMap[ext]
			return skip
		}
		return false
	}

	for _, suffix := range s.gzipSkipSuffixes {
		if strings.HasSuffix(reqPath, suffix) {
			return true
		}
	}

	return false
}

// pathExt 取路径最后一段的扩展名，没有则返回空串
//
// 自己写而不用 path.Ext：行为相同，但省掉一次包级函数调用，且不必为这一处引入 path 包
// 只看最后一个 / 之后的部分，所以 /a.b/c 这种目录名带点的路径不会被误判
// [EN] Written out rather than taken from path.Ext: same behaviour, one fewer package-level
// call, and no import for a single use. Only the segment after the final slash is examined,
// so a path such as /a.b/c with a dot in a directory name is not misread
func pathExt(p string) string {
	for i := len(p) - 1; i >= 0 && p[i] != '/'; i-- {
		if p[i] == '.' {
			return p[i:]
		}
	}
	return ""
}

// gzipMiddleware HTTP Gzip压缩中间件
func (s *Server) gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查是否启用压缩
		if !s.gzipEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		// 检查客户端是否支持gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// 检查是否应该跳过压缩
		if s.shouldSkipGzip(r) {
			next.ServeHTTP(w, r)
			return
		}

		// 从对象池获取 gzip writer
		gzipWriter := s.gzipWriterPool.Get().(*gzip.Writer)
		defer s.gzipWriterPool.Put(gzipWriter)

		// 设置响应头（必须在 WriteHeader 之前）
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length") // 删除原始长度，因为压缩后长度会变

		// 使用标准 gzip writer
		gzipWriter.Reset(w)
		// 包装ResponseWriter
		gzw := &gzipResponseWriter{ResponseWriter: w, gzipWriter: gzipWriter}
		defer gzw.Close()

		next.ServeHTTP(gzw, r)
	})
}

// initDataMasker 初始化数据脱敏器（从配置读取敏感字段）
func (s *Server) initDataMasker() {
	config := &desensitize.MaskerConfig{
		SensitiveKeys: s.config.Middleware.Logging.SensitiveKeys,
		SensitiveMask: s.config.Middleware.Logging.SensitiveMask,
		MaxBodySize:   s.config.Middleware.Logging.MaxBodySize,
	}
	// 创建脱敏器（在 Server 初始化时创建一次，供所有请求复用）
	s.dataMasker = desensitize.NewMasker(config)
	global.DATAMASKER = s.dataMasker
}

// initHTTPGateway 初始化HTTP网关
func (s *Server) initHTTPGateway() error {
	// 创建gRPC-Gateway多路复用器，配置JSON序列化选项
	opts := s.buildServeMuxOptions()

	// 收集所有中间件（静态 + 动态提供）并去重
	const middlewareWarnThreshold = 100
	middlewareSet := make(map[string]bool)
	var allMiddlewares []runtime.Middleware

	// 添加静态中间件
	for i, mw := range s.grpcGatewayMiddlewares {
		key := fmt.Sprintf("static_%d", i)
		if middlewareSet[key] {
			continue
		}
		allMiddlewares = append(allMiddlewares, mw)
		middlewareSet[key] = true
	}

	// 添加动态中间件
	for providerIdx, provider := range s.grpcGatewayMiddlewareProviders {
		mws := provider()
		if len(mws) == 0 {
			continue
		}

		for mwIdx, mw := range mws {
			key := fmt.Sprintf("provider_%d_%d", providerIdx, mwIdx)
			if middlewareSet[key] {
				continue
			}
			allMiddlewares = append(allMiddlewares, mw)
			middlewareSet[key] = true
		}
	}

	// 自动注入 struct tag 校验中间件（本地 Handler 模式下 HTTP 请求绕过 gRPC 拦截器，
	// 需要在 gateway 层补充校验，配合 protoc-go-inject-tag 生效）
	if s.middlewareManager != nil {
		validatorMW := s.middlewareManager.GRPCGatewayStructTagValidatorMiddleware()
		allMiddlewares = append(allMiddlewares, validatorMW)
	}

	// 中间件数量超过阈值时警告（warn-only 模式，不硬限制）
	if len(allMiddlewares) > middlewareWarnThreshold {
		global.LOGGER.WarnContext(s.ctx, "⚠️  中间件数量超过建议值",
			"count", len(allMiddlewares),
			"threshold", middlewareWarnThreshold)
	}

	// 添加所有中间件
	if len(allMiddlewares) > 0 {
		opts = append(opts, runtime.WithMiddlewares(allMiddlewares...))
		global.LOGGER.InfoContext(s.ctx, "✅ 已注册 %d 个 gRPC-Gateway 中间件", len(allMiddlewares))
	}

	s.gwMux = runtime.NewServeMux(opts...)

	// 创建HTTP多路复用器
	s.httpMux = http.NewServeMux()
	s.httpRoutePatterns = make(map[string]struct{})

	// 注册网关路由（默认路由到gwMux）
	s.httpMux.Handle("/", s.gwMux)
	s.httpRoutePatterns["/"] = struct{}{}

	httpEndpoint := netx.JoinHostPort(s.config.HTTPServer.Host, s.config.HTTPServer.Port)

	// 注册健康检查
	if s.config.Health.Enabled {
		healthPath := s.config.Health.Path
		s.httpMux.HandleFunc(healthPath, s.healthCheckHandler)
		s.httpRoutePatterns[healthPath] = struct{}{}

		global.LOGGER.InfoKV("❤️  健康检查已启用", "url", "http://"+httpEndpoint+healthPath)

		// 注册组件级健康检查端点
		s.registerComponentHealthChecks()
	}

	// 注册监控指标端点（使用 MetricsManager 的独立 registry，包含业务指标）
	if s.config.Monitoring.Metrics.Enabled {
		prometheusPath := s.config.Monitoring.Metrics.Endpoint
		var metricsHandler http.Handler
		if s.middlewareManager != nil {
			metricsHandler = s.middlewareManager.MetricsHandler()
		} else {
			metricsHandler = promhttp.Handler()
		}
		s.httpMux.Handle(prometheusPath, metricsHandler)
		s.httpRoutePatterns[prometheusPath] = struct{}{}

		global.LOGGER.InfoKV("📊 监控指标服务可用", "url", "http://"+httpEndpoint+prometheusPath)
	}

	// 应用中间件
	var handler http.Handler = s.httpMux

	if s.middlewareManager != nil {
		var middlewares []middleware.MiddlewareFunc
		middlewares = s.middlewareManager.GetMiddlewares()
		handler = middleware.ApplyMiddlewares(handler, middlewares...)
	}

	// 最后应用Gzip压缩中间件（如果启用）
	// 注意：Gzip 应该在日志中间件之后执行，否则日志记录的是压缩后的乱码
	if s.gzipEnabled() {
		handler = s.gzipMiddleware(handler)
		global.LOGGER.InfoMsg("✅ HTTP Gzip压缩已启用")
	}

	// 根据配置决定是否启用 HTTP/2
	if s.config.HTTPServer.EnableHTTP2 {
		h2s := s.buildHTTP2Server()
		handler = h2c.NewHandler(handler, h2s)
		global.LOGGER.InfoMsg("✅ HTTP/2 多路复用已启用 (h2c)")
	}

	// 创建 HTTP 服务器
	s.httpServer = &http.Server{
		Addr:              httpEndpoint,
		Handler:           handler,
		ReadTimeout:       time.Duration(s.config.HTTPServer.ReadTimeout) * time.Second,
		ReadHeaderTimeout: time.Duration(s.config.HTTPServer.ReadHeaderTimeout) * time.Second,
		WriteTimeout:      time.Duration(s.config.HTTPServer.WriteTimeout) * time.Second,
		IdleTimeout:       time.Duration(s.config.HTTPServer.IdleTimeout) * time.Second,
		MaxHeaderBytes:    s.config.HTTPServer.MaxHeaderBytes,
		TLSConfig:         s.buildTLSConfig(),
	}

	return nil
}

// RebuildHTTPGateway 重建 HTTP网关（用于在添加中间件后重新初始化）
func (s *Server) RebuildHTTPGateway() error {
	global.LOGGER.InfoContext(s.ctx, "🔄 重建 HTTP Gateway...")
	return s.initHTTPGateway()
}

// registerComponentHealthChecks 注册组件级健康检查端点
func (s *Server) registerComponentHealthChecks() {
	baseURL := fmt.Sprintf("http://%s", netx.JoinHostPort(s.config.HTTPServer.Host, s.config.HTTPServer.Port))

	// 注册Redis健康检查
	if s.config.Health.Redis.Enabled {
		s.httpMux.HandleFunc(s.config.Health.Redis.Path, s.redisHealthCheckHandler)
		s.httpRoutePatterns[s.config.Health.Redis.Path] = struct{}{}
		global.LOGGER.InfoKV("🔴 Redis健康检查已启用",
			"url", baseURL+s.config.Health.Redis.Path)
	}

	// 注册MySQL健康检查
	if s.config.Health.MySQL.Enabled {
		s.httpMux.HandleFunc(s.config.Health.MySQL.Path, s.mysqlHealthCheckHandler)
		s.httpRoutePatterns[s.config.Health.MySQL.Path] = struct{}{}
		global.LOGGER.InfoKV("🗃️  MySQL健康检查已启用",
			"url", baseURL+s.config.Health.MySQL.Path)
	}

	// 后续可以在这里继续添加其他组件的健康检查
	// 如: Elasticsearch, MongoDB, Kafka 等
}

// startHTTPServer 启动HTTP服务器
// 当 HTTPServer.Port == 0 时跳过启动（适用于仅使用命名监听器的场景）
func (s *Server) startHTTPServer() error {
	httpServer := s.httpServer
	if httpServer == nil {
		return nil
	}

	// port=0 表示禁用主 HTTP 服务器
	if s.config.HTTPServer.Port == 0 {
		global.LOGGER.InfoMsg("主 HTTP 服务器已禁用（port=0），流量由命名监听器接管")
		return nil
	}

	address := httpServer.Addr

	// TLS 支持待实现（需要在 go-config/pkg/security 中添加 TLS 配置）
	// if s.config.Security.TLS.Enabled {
	// 	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
	// }

	global.LOGGER.InfoKV("Starting HTTP server", "address", address)

	// 从配置中获取网络类型
	listener, err := net.Listen(s.config.HTTPServer.Network, address)
	if err != nil {
		return fmt.Errorf("failed to create %s listener: %w", s.config.HTTPServer.Network, err)
	}
	defer listener.Close() // Fix 确保 listener 关闭，防止连接泄漏

	if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// stopHTTPServer 停止HTTP服务器
func (s *Server) stopHTTPServer() error {
	if s.httpServer == nil {
		return nil
	}

	// 创建30秒超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	global.LOGGER.InfoContext(ctx, "Stopping HTTP server...")

	httpServer := s.httpServer
	if httpServer == nil {
		return nil
	}

	if err := httpServer.Shutdown(ctx); err != nil {
		global.LOGGER.WithError(err).ErrorContext(ctx, "Failed to shutdown HTTP server")
		return err
	}

	global.LOGGER.InfoContext(ctx, "HTTP server stopped")
	return nil
}

// healthCheckHandler 健康检查处理器
func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if s.healthManager != nil {
		// 使用健康检查管理器处理请求
		handler := s.healthManager.HTTPHandler()
		handler(w, r)
	} else {
		// 降级为基础健康检查
		response.WriteSuccessResult(w, "go-rpc-gateway service is healthy")
	}
}

// redisHealthCheckHandler Redis健康检查处理器
func (s *Server) redisHealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	s.componentHealthCheck(w, r, "redis")
}

// mysqlHealthCheckHandler MySQL健康检查处理器
func (s *Server) mysqlHealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	s.componentHealthCheck(w, r, "mysql")
}

// componentHealthCheck 组件健康检查通用处理器
func (s *Server) componentHealthCheck(w http.ResponseWriter, r *http.Request, component string) {
	w.Header().Set(constants.HeaderContentType, httpx.ContentTypeApplicationJSON)

	if s.healthManager == nil {
		response.WriteServiceUnavailableResult(w, fmt.Sprintf("%s health checker not configured", component))
		return
	}

	// 使用健康检查管理器进行组件检查
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result := s.healthManager.Check(ctx, true)

	// 返回指定组件的检查结果
	if status, ok := result.Checks[component]; ok {
		isHealthy := status.Status != "error"
		message := fmt.Sprintf("%s: %s (latency: %dms, checked at: %v)",
			status.Status, status.Message, status.Latency.Milliseconds(), status.CheckedAt)

		// 安全地处理 details 类型转换
		var details map[string]any
		if status.Details != nil {
			if d, ok := status.Details.(map[string]any); ok {
				details = d
			}
		}

		response.WriteHealthCheckResult(w, isHealthy, component, message, details)
	} else {
		response.WriteServiceUnavailableResult(w, fmt.Sprintf("%s health checker not registered", component))
	}
}

// RegisterHTTPRoute 注册HTTP路由
func (s *Server) RegisterHTTPRoute(pattern string, handler http.Handler) {
	if s.httpMux == nil {
		global.LOGGER.ErrorMsg("HTTP multiplexer not initialized")
		return
	}

	if s.httpRoutePatterns == nil {
		s.httpRoutePatterns = make(map[string]struct{})
	}
	if _, exists := s.httpRoutePatterns[pattern]; exists {
		global.LOGGER.DebugKV("HTTP route already registered, skip duplicate",
			"pattern", pattern,
			"handler_type", fmt.Sprintf("%T", handler))
		return
	}

	s.httpMux.Handle(pattern, handler)
	s.httpRoutePatterns[pattern] = struct{}{}
	global.LOGGER.InfoKV("✅ 注册HTTP路由成功",
		"pattern", pattern,
		"handler_type", fmt.Sprintf("%T", handler))
}

// RegisterHTTPHandlerFunc 注册HTTP处理函数
func (s *Server) RegisterHTTPHandlerFunc(pattern string, handlerFunc http.HandlerFunc) {
	if s.httpMux == nil {
		global.LOGGER.ErrorMsg("HTTP multiplexer not initialized")
		return
	}

	if s.httpRoutePatterns == nil {
		s.httpRoutePatterns = make(map[string]struct{})
	}
	if _, exists := s.httpRoutePatterns[pattern]; exists {
		global.LOGGER.DebugKV("HTTP handler func already registered, skip duplicate", "pattern", pattern)
		return
	}

	s.httpMux.HandleFunc(pattern, handlerFunc)
	s.httpRoutePatterns[pattern] = struct{}{}
	global.LOGGER.InfoKV("✅ 注册HTTP处理函数成功", "pattern", pattern)
}

// buildTLSConfig 构建 TLS 配置（从配置文件读取）
func (s *Server) buildTLSConfig() *tls.Config {
	if s.config.HTTPServer.TLS == nil {
		return nil
	}

	tlsCfg := s.config.HTTPServer.TLS

	// 构建 TLS 配置（使用枚举类型的转换方法）
	config := &tls.Config{
		MinVersion:               tlsCfg.MinVersion.ToUint16(),
		PreferServerCipherSuites: tlsCfg.PreferServerCiphers,
		InsecureSkipVerify:       tlsCfg.InsecureSkipVerify,
		ClientAuth:               tlsCfg.ClientAuth.ToTLSClientAuth(),
	}

	// 设置 ALPN 协议（用于 HTTP/2 协商）
	if len(tlsCfg.NextProtos) > 0 {
		config.NextProtos = tlsCfg.NextProtos
	}

	return config
}

// buildHTTP2Server 构建 HTTP/2 服务器配置（从配置文件读取）
func (s *Server) buildHTTP2Server() *http2.Server {
	h2cfg := s.config.HTTPServer.HTTP2

	// 从配置读取所有参数
	return &http2.Server{
		MaxConcurrentStreams: h2cfg.MaxConcurrentStreams,
		MaxReadFrameSize:     h2cfg.MaxReadFrameSize,
		IdleTimeout:          time.Duration(s.config.HTTPServer.IdleTimeout) * time.Second,
	}
}

// namedListener 命名监听器，绑定独立的 http.Server 和 handler
type namedListener struct {
	name   string
	config *gwconfig.Listener
	server *http.Server
}

// initNamedListeners 初始化命名监听器
// 每个命名监听器复用主 HTTP 网关的 gwMux 和中间件链，
// 但监听独立的 Host:Port，适用于 Ops/Tenant 端口分离等场景
func (s *Server) initNamedListeners() error {
	if len(s.config.Listeners) == 0 {
		return nil
	}

	s.namedListeners = make(map[string]*namedListener, len(s.config.Listeners))

	for _, l := range s.config.Listeners {
		if l == nil || l.Name == "" {
			continue
		}

		// 复用主 HTTP 网关的 handler（包含中间件链和 gwMux）
		var handler http.Handler = s.httpMux
		if s.middlewareManager != nil {
			handler = middleware.ApplyMiddlewares(handler, s.middlewareManager.GetMiddlewares()...)
		}
		if s.gzipEnabled() {
			handler = s.gzipMiddleware(handler)
		}
		if s.http2Enabled() {
			h2s := s.buildHTTP2Server()
			handler = h2c.NewHandler(handler, h2s)
		}

		addr := netx.JoinHostPort(l.Host, l.Port)
		srv := &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadTimeout:       time.Duration(s.config.HTTPServer.ReadTimeout) * time.Second,
			ReadHeaderTimeout: time.Duration(s.config.HTTPServer.ReadHeaderTimeout) * time.Second,
			WriteTimeout:      time.Duration(s.config.HTTPServer.WriteTimeout) * time.Second,
			IdleTimeout:       time.Duration(s.config.HTTPServer.IdleTimeout) * time.Second,
			MaxHeaderBytes:    s.config.HTTPServer.MaxHeaderBytes,
		}

		s.namedListeners[l.Name] = &namedListener{
			name:   l.Name,
			config: l,
			server: srv,
		}

		global.LOGGER.InfoKV("命名监听器已初始化", "name", l.Name, "address", addr)
	}

	return nil
}

// startNamedListeners 启动所有命名监听器
func (s *Server) startNamedListeners() {
	for _, nl := range s.namedListeners {
		nl := nl
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			addr := nl.server.Addr
			network := mathx.IfEmpty(nl.config.Network, "tcp4")
			listener, err := net.Listen(network, addr)
			if err != nil {
				global.LOGGER.WithError(err).ErrorKV("命名监听器启动失败", "name", nl.name, "address", addr)
				return
			}
			defer listener.Close()

			global.LOGGER.InfoKV("命名监听器已启动", "name", nl.name, "address", addr)
			if err := nl.server.Serve(listener); err != nil && err != http.ErrServerClosed {
				global.LOGGER.WithError(err).ErrorKV("命名监听器异常退出", "name", nl.name)
			}
		}()
	}
}

// stopNamedListeners 停止所有命名监听器
func (s *Server) stopNamedListeners() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, nl := range s.namedListeners {
		if nl.server != nil {
			global.LOGGER.InfoKV("停止命名监听器", "name", nl.name)
			if err := nl.server.Shutdown(ctx); err != nil {
				global.LOGGER.WithError(err).WarnKV("命名监听器关闭失败", "name", nl.name)
			}
		}
	}
}
