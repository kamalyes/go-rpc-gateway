/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-30 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-30 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\response_writer.go
 * @Description: 统一的 ResponseWriter 包装器 - 供所有中间件共享使用
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

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
}

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
	rw.body.Reset()
	return rw
}

// Release 归还 ResponseWriter 到对象池
func (rw *ResponseWriter) Release() {
	rw.ResponseWriter = nil
	rw.body.Reset()
	responseWriterPool.Put(rw)
}

// EnableBodyCapture 启用响应体捕获
func (rw *ResponseWriter) EnableBodyCapture() {
	rw.captureBody = true
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
		rw.body.Write(data)
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
