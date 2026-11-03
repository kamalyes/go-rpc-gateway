/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-30 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-30 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\core\response_writer.go
 * @Description: 统一的 ResponseWriter 包装器 - 供所有中间件共享使用
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package core

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"sync"
)

// ResponseWriter 统一的响应写入器包装器
// 用于捕获状态码、响应大小等信息，供多个中间件共享使用
type ResponseWriter struct {
	http.ResponseWriter
	statusCode   int           // HTTP 状态码
	bytesWritten int64         // 写入的字节数
	wroteHeader  bool          // 是否已写入头部
	hijacked     bool          // 是否被劫持（WebSocket等）
	body         *bytes.Buffer // 响应体缓存
	captureBody  bool          // 是否捕获响应体
	// bodyLimit 捕获上限，单位字节 捕获满了之后不再往 body 里写
	// [EN] Capture ceiling in bytes; writes to body stop once it is reached
	bodyLimit int
}

// DefaultBodyCaptureLimit 未指定上限时的默认捕获字节数
//
// 取 2048 与 go-config 的 logging.MaxBodySize 默认值一致 ——
// 捕获的唯一消费者是访问日志，而它本来就只取前 MaxBodySize 字节，
// 捕获得比这更多的部分一定会被丢弃
//
// [EN] The default capture size when no ceiling is given
// 2048 matches go-config's logging.MaxBodySize default: the only consumer of the
// captured body is the access log, which already keeps just the first MaxBodySize
// bytes, so anything captured beyond that is certain to be discarded
const DefaultBodyCaptureLimit = 2048

// maxPooledBodyCapacity 归还对象池时允许保留的 buffer 容量上限
//
// bytes.Buffer.Reset 只把长度归零、容量原样保留，所以一次大响应会让池里那个
// writer 永久占着对应容量 捕获已有上限后这里通常用不到，留作兜底
//
// [EN] The buffer capacity still allowed back into the pool
// bytes.Buffer.Reset zeroes the length and keeps the capacity, so one large response
// would otherwise pin that capacity in the pool forever; with capture bounded this is
// a backstop
const maxPooledBodyCapacity = 64 << 10

// responseWriterPool 对象池 - 减少内存分配，提升性能
var responseWriterPool = sync.Pool{
	New: func() interface{} {
		return &ResponseWriter{
			statusCode: http.StatusOK,
			body:       bytes.NewBuffer(make([]byte, 0, 1024)),
		}
	},
}

// NewResponseWriter 从对象池获取 ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	rw := responseWriterPool.Get().(*ResponseWriter)
	rw.ResponseWriter = w
	rw.statusCode = http.StatusOK
	rw.bytesWritten = 0
	rw.wroteHeader = false
	rw.hijacked = false
	rw.captureBody = false
	rw.bodyLimit = 0
	rw.body.Reset()
	return rw
}

// Release 归还 ResponseWriter 到对象池
// buffer 长得过大就不再归还，换一个新的 —— 否则它的容量会一直留在池里
//
// [EN] A buffer grown too large is not returned, so its capacity does not stay in the pool
func (rw *ResponseWriter) Release() {
	rw.ResponseWriter = nil
	if rw.body.Cap() > maxPooledBodyCapacity {
		rw.body = bytes.NewBuffer(make([]byte, 0, 1024))
	} else {
		rw.body.Reset()
	}
	responseWriterPool.Put(rw)
}

// EnableBodyCapture 启用响应体捕获，按 DefaultBodyCaptureLimit 设上限
// [EN] Enables response body capture, bounded by DefaultBodyCaptureLimit
func (rw *ResponseWriter) EnableBodyCapture() {
	rw.EnableBodyCaptureLimit(DefaultBodyCaptureLimit)
}

// EnableBodyCaptureLimit 启用响应体捕获并指定上限，limit <= 0 时用默认值
//
// 加上限的理由：改造前 Write 把每一片响应都无条件写进 buffer，
// 一个流式下载的大响应会被整段缓冲进内存，而访问日志只用得到前 MaxBodySize 字节
//
// [EN] Enables capture with an explicit ceiling, falling back to the default when limit <= 0
// Write used to append every chunk unconditionally, so a large streaming response was
// buffered whole while the access log could only ever use the first MaxBodySize bytes
func (rw *ResponseWriter) EnableBodyCaptureLimit(limit int) {
	if limit <= 0 {
		limit = DefaultBodyCaptureLimit
	}
	rw.captureBody = true
	rw.bodyLimit = limit
}

// GetBody 获取捕获的响应体
func (rw *ResponseWriter) GetBody() []byte {
	if rw.body == nil {
		return nil
	}
	return rw.body.Bytes()
}

// WriteHeader 实现 http.ResponseWriter 接口
func (rw *ResponseWriter) WriteHeader(statusCode int) {
	if !rw.wroteHeader {
		rw.statusCode = statusCode
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(statusCode)
	}
}

// Write 实现 http.ResponseWriter 接口
func (rw *ResponseWriter) Write(data []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	if rw.captureBody {
		// 只补到上限为止，超出的部分直接不进 buffer
		// [EN] Only fill up to the ceiling; anything beyond never enters the buffer
		if room := rw.bodyLimit - rw.body.Len(); room > 0 {
			if len(data) <= room {
				rw.body.Write(data)
			} else {
				rw.body.Write(data[:room])
			}
		}
	}
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += int64(n)
	return n, err
}

// StatusCode 获取 HTTP 状态码
func (rw *ResponseWriter) StatusCode() int {
	return rw.statusCode
}

// BytesWritten 获取已写入的字节数
func (rw *ResponseWriter) BytesWritten() int64 {
	return rw.bytesWritten
}

// WroteHeader 检查是否已写入头部
func (rw *ResponseWriter) WroteHeader() bool {
	return rw.wroteHeader
}

// Hijack 实现 http.Hijacker 接口（支持 WebSocket）
func (rw *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		rw.hijacked = true
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Flush 实现 http.Flusher 接口（支持流式响应）
func (rw *ResponseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Push 实现 http.Pusher 接口（支持 HTTP/2 Server Push）
func (rw *ResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Unwrap 返回底层的 http.ResponseWriter
func (rw *ResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// IsHijacked 检查连接是否被劫持
func (rw *ResponseWriter) IsHijacked() bool {
	return rw.hijacked
}

// IsSuccess 判断是否为成功响应 (2xx)
func (rw *ResponseWriter) IsSuccess() bool {
	return rw.statusCode >= 200 && rw.statusCode < 300
}

// IsClientError 判断是否为客户端错误 (4xx)
func (rw *ResponseWriter) IsClientError() bool {
	return rw.statusCode >= 400 && rw.statusCode < 500
}

// IsServerError 判断是否为服务器错误 (5xx)
func (rw *ResponseWriter) IsServerError() bool {
	return rw.statusCode >= 500
}

// IsError 判断是否为错误响应 (4xx 或 5xx)
func (rw *ResponseWriter) IsError() bool {
	return rw.statusCode >= 400
}
