/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 23:59:38
 * @FilePath: \go-rpc-gateway\middleware\i18n\struct_tag_validator_test.go
 * @Description: 基于 struct tag 的 gRPC 校验拦截器测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package i18n

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	validator "github.com/kamalyes/go-toolbox/pkg/argus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type validRequest struct {
	Name  string `validate:"required,min=1"`
	Email string `validate:"required,email"`
	Age   int    `validate:"min=0,max=150"`
}

type noTagRequest struct {
	Name  string
	Email string
}

type partialTagRequest struct {
	Name  string `validate:"required"`
	Email string
}

type nestedStructRequest struct {
	Inner *innerStruct `validate:"required"`
}

type innerStruct struct {
	Value string `validate:"required,min=3"`
}

func TestGetStructTagValidator(t *testing.T) {
	v1 := getStructTagValidator()
	v2 := getStructTagValidator()
	assert.NotNil(t, v1)
	assert.Same(t, v1, v2, "should return the same singleton instance")
}

func TestStructTagValidatorUnaryInterceptor_NilRequest(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "ok", resp)
}

func TestStructTagValidatorUnaryInterceptor_ValidRequest(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	req := &validRequest{Name: "kronos", Email: "test@example.com", Age: 25}
	resp, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "ok", resp)
}

func TestStructTagValidatorUnaryInterceptor_InvalidRequest_MissingRequired(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	req := &validRequest{Name: "", Email: "test@example.com", Age: 25}
	_, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
	assert.Error(t, err)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "invalid argument")
}

func TestStructTagValidatorUnaryInterceptor_InvalidRequest_BadEmail(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	req := &validRequest{Name: "kronos", Email: "not-an-email", Age: 25}
	_, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
	assert.Error(t, err)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestStructTagValidatorUnaryInterceptor_InvalidRequest_AgeOutOfRange(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	req := &validRequest{Name: "kronos", Email: "test@example.com", Age: 200}
	_, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
	assert.Error(t, err)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestStructTagValidatorUnaryInterceptor_NoTagRequest(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	req := &noTagRequest{Name: "", Email: ""}
	resp, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "ok", resp)
}

func TestStructTagValidatorUnaryInterceptor_PartialTagRequest_Valid(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	req := &partialTagRequest{Name: "kronos", Email: ""}
	_, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestStructTagValidatorUnaryInterceptor_PartialTagRequest_Invalid(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	req := &partialTagRequest{Name: "", Email: "anything"}
	_, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
	assert.Error(t, err)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestStructTagValidatorUnaryInterceptor_NestedStruct(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	t.Run("valid nested", func(t *testing.T) {
		req := &nestedStructRequest{Inner: &innerStruct{Value: "hello"}}
		_, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
		assert.NoError(t, err)
	})

	t.Run("nil nested required", func(t *testing.T) {
		req := &nestedStructRequest{Inner: nil}
		_, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
		assert.Error(t, err)
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("nested field too short", func(t *testing.T) {
		req := &nestedStructRequest{Inner: &innerStruct{Value: "ab"}}
		_, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
		assert.Error(t, err)
	})
}

type mockServerStream struct {
	grpc.ServerStream
	recvMsgs []interface{}
	recvIdx  int
	recvErr  error
}

func (m *mockServerStream) RecvMsg(msg interface{}) error {
	if m.recvErr != nil {
		return m.recvErr
	}
	if m.recvIdx >= len(m.recvMsgs) {
		return errors.New("EOF")
	}
	src := reflect.ValueOf(m.recvMsgs[m.recvIdx])
	dst := reflect.ValueOf(msg)
	if src.Kind() == reflect.Ptr && dst.Kind() == reflect.Ptr {
		dst.Elem().Set(src.Elem())
	}
	m.recvIdx++
	return nil
}

func TestStructTagValidatorStreamInterceptor_DelegatesToHandler(t *testing.T) {
	interceptor := StructTagValidatorStreamInterceptor()
	handlerCalled := false
	handler := func(srv interface{}, ss grpc.ServerStream) error {
		handlerCalled = true
		return nil
	}

	mock := &mockServerStream{
		recvMsgs: []interface{}{&validRequest{Name: "kronos", Email: "test@example.com", Age: 25}},
	}
	err := interceptor(nil, mock, &grpc.StreamServerInfo{}, handler)
	assert.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestStructTagValidatingStream_RecvMsg_Valid(t *testing.T) {
	v := getStructTagValidator()
	mock := &mockServerStream{
		recvMsgs: []interface{}{&validRequest{Name: "kronos", Email: "test@example.com", Age: 25}},
	}
	stream := &structTagValidatingStream{ServerStream: mock, validate: v}

	msg := &validRequest{}
	err := stream.RecvMsg(msg)
	assert.NoError(t, err)
	assert.Equal(t, "kronos", msg.Name)
}

func TestStructTagValidatingStream_RecvMsg_Invalid(t *testing.T) {
	v := getStructTagValidator()
	mock := &mockServerStream{
		recvMsgs: []interface{}{&validRequest{Name: "", Email: "bad", Age: 25}},
	}
	stream := &structTagValidatingStream{ServerStream: mock, validate: v}

	msg := &validRequest{}
	err := stream.RecvMsg(msg)
	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestStructTagValidatingStream_RecvMsg_Nil(t *testing.T) {
	v := getStructTagValidator()
	mock := &mockServerStream{
		recvMsgs: []interface{}{nil},
	}
	stream := &structTagValidatingStream{ServerStream: mock, validate: v}

	err := stream.RecvMsg(nil)
	assert.NoError(t, err)
}

func TestStructTagValidatingStream_RecvMsg_RecvError(t *testing.T) {
	v := getStructTagValidator()
	recvErr := errors.New("connection reset")
	mock := &mockServerStream{recvErr: recvErr}
	stream := &structTagValidatingStream{ServerStream: mock, validate: v}

	err := stream.RecvMsg(&validRequest{})
	assert.Equal(t, recvErr, err)
}

func TestFormatStructTagValidationError_WithValidationErrors(t *testing.T) {
	v := validator.New(validator.WithRequiredStructEnabled())
	req := &validRequest{Name: "", Email: "not-email", Age: 999}
	err := v.Struct(req)
	assert.Error(t, err)

	msg := formatStructTagValidationError(err)
	assert.Contains(t, msg, "invalid argument:")
}

func TestFormatStructTagValidationError_NonValidationError(t *testing.T) {
	err := errors.New("some random error")
	msg := formatStructTagValidationError(err)
	assert.Equal(t, "some random error", msg)
}

func TestToValidationErrors_Success(t *testing.T) {
	v := validator.New(validator.WithRequiredStructEnabled())
	req := &validRequest{Name: ""}
	err := v.Struct(req)
	assert.Error(t, err)

	var fieldErrs validator.ValidationErrors
	ok := toValidationErrors(err, &fieldErrs)
	assert.True(t, ok)
	assert.True(t, len(fieldErrs) > 0)
}

func TestToValidationErrors_NonValidationError(t *testing.T) {
	err := errors.New("not a validation error")
	var fieldErrs validator.ValidationErrors
	ok := toValidationErrors(err, &fieldErrs)
	assert.False(t, ok)
}

func TestStructTagValidatorUnaryInterceptor_MultipleErrors(t *testing.T) {
	interceptor := StructTagValidatorUnaryInterceptor()
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}

	req := &validRequest{Name: "", Email: "bad", Age: -1}
	_, err := interceptor(context.Background(), req, &grpc.UnaryServerInfo{}, handler)
	assert.Error(t, err)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), "invalid argument:")
}

// ────────────────────────────────────────
// Gateway middleware 测试
// ────────────────────────────────────────

// testGatewayMsg 用于 Gateway middleware 测试的结构体
type testGatewayMsg struct {
	Name  string `validate:"required,min=1" json:"name"`
	Email string `validate:"required,email" json:"email"`
}

func TestRegisterGatewayMessageType(t *testing.T) {
	// 清空注册表
	gatewayMessageTypeRegistry.mu.Lock()
	gatewayMessageTypeRegistry.data = nil
	gatewayMessageTypeRegistry.mu.Unlock()

	// 注册消息类型
	RegisterGatewayMessageType(http.MethodPost, "/v1/test", func() any {
		return &testGatewayMsg{}
	})

	// 验证注册成功
	fn, found := lookupGatewayMessageType(http.MethodPost, "/v1/test")
	assert.True(t, found)
	assert.NotNil(t, fn)

	// 验证未注册的路径
	_, found = lookupGatewayMessageType(http.MethodGet, "/v1/other")
	assert.False(t, found)

	// 清理
	gatewayMessageTypeRegistry.mu.Lock()
	gatewayMessageTypeRegistry.data = nil
	gatewayMessageTypeRegistry.mu.Unlock()
}

func TestStructTagValidatorGatewayMiddleware_UnregisteredPath(t *testing.T) {
	// 清空注册表
	gatewayMessageTypeRegistry.mu.Lock()
	gatewayMessageTypeRegistry.data = nil
	gatewayMessageTypeRegistry.mu.Unlock()

	mw := StructTagValidatorGatewayMiddleware()
	called := false
	next := func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		called = true
	}

	handler := mw(next)

	req := httptest.NewRequest(http.MethodPost, "/v1/unregistered", bytes.NewReader([]byte(`{"name":"test"}`)))
	w := httptest.NewRecorder()

	handler(w, req, nil)
	assert.True(t, called, "unregistered path should pass through to next handler")
}

func TestStructTagValidatorGatewayMiddleware_EmptyBody(t *testing.T) {
	// 清空并注册
	gatewayMessageTypeRegistry.mu.Lock()
	gatewayMessageTypeRegistry.data = nil
	gatewayMessageTypeRegistry.mu.Unlock()
	RegisterGatewayMessageType(http.MethodPost, "/v1/test", func() any {
		return &testGatewayMsg{}
	})

	mw := StructTagValidatorGatewayMiddleware()
	called := false
	next := func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		called = true
	}

	handler := mw(next)

	req := httptest.NewRequest(http.MethodPost, "/v1/test", nil)
	w := httptest.NewRecorder()

	handler(w, req, nil)
	assert.True(t, called, "empty body should pass through to next handler")
}

func TestStructTagValidatorGatewayMiddleware_ValidBody(t *testing.T) {
	// 清空并注册
	gatewayMessageTypeRegistry.mu.Lock()
	gatewayMessageTypeRegistry.data = nil
	gatewayMessageTypeRegistry.mu.Unlock()
	RegisterGatewayMessageType(http.MethodPost, "/v1/test", func() any {
		return &testGatewayMsg{}
	})

	mw := StructTagValidatorGatewayMiddleware()
	called := false
	next := func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		called = true
		// 验证 body 仍然可读
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, `{"name":"kronos","email":"test@example.com"}`, string(body))
	}

	handler := mw(next)

	body := `{"name":"kronos","email":"test@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/test", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler(w, req, nil)
	assert.True(t, called, "valid body should pass through to next handler")
}

func TestStructTagValidatorGatewayMiddleware_InvalidBody(t *testing.T) {
	// 清空并注册
	gatewayMessageTypeRegistry.mu.Lock()
	gatewayMessageTypeRegistry.data = nil
	gatewayMessageTypeRegistry.mu.Unlock()
	RegisterGatewayMessageType(http.MethodPost, "/v1/test", func() any {
		return &testGatewayMsg{}
	})

	mw := StructTagValidatorGatewayMiddleware()
	called := false
	next := func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		called = true
	}

	handler := mw(next)

	// name 为空，email 格式错误
	body := `{"name":"","email":"bad"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/test", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler(w, req, nil)
	assert.False(t, called, "invalid body should NOT call next handler")
	// 应该返回了错误响应
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestStructTagValidatorGatewayMiddleware_BodyRestored(t *testing.T) {
	// 清空并注册
	gatewayMessageTypeRegistry.mu.Lock()
	gatewayMessageTypeRegistry.data = nil
	gatewayMessageTypeRegistry.mu.Unlock()
	RegisterGatewayMessageType(http.MethodPost, "/v1/test", func() any {
		return &testGatewayMsg{}
	})

	mw := StructTagValidatorGatewayMiddleware()
	var receivedBody string
	next := func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		b, _ := io.ReadAll(r.Body)
		receivedBody = string(b)
	}

	handler := mw(next)

	originalBody := `{"name":"kronos","email":"test@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/test", bytes.NewReader([]byte(originalBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler(w, req, nil)
	assert.Equal(t, originalBody, receivedBody, "body should be restored after validation")
}

// TestMarshalerMuxIsShared marshaler 用的 ServeMux 必须是共享的一份
//
// 回归点：原先每个请求 runtime.MarshalerForRequest(runtime.NewServeMux(), r)，校验失败时
// runtime.HTTPError 又再 new 一个。NewServeMux 会建一整套 marshaler 注册表与默认选项，
// 实测每个被校验的请求白付约 130 ns、560 B、4 次分配，失败路径翻倍
//
// [EN] Regression: every request built a ServeMux for the marshaler and a validation failure
// built a second one. NewServeMux creates a whole marshaler registry and default options;
// measured at roughly 130 ns, 560 B and 4 allocations wasted per validated request, doubled on
// the failure path.
func TestMarshalerMuxIsShared(t *testing.T) {
	if marshalerMux == nil {
		t.Fatal("共享 ServeMux 未初始化")
	}

	r := httptest.NewRequest("POST", "/v1/x", nil)
	r.Header.Set("Content-Type", "application/json")

	first, _ := runtime.MarshalerForRequest(marshalerMux, r)
	second, _ := runtime.MarshalerForRequest(marshalerMux, r)
	if first == nil || second == nil {
		t.Fatal("取不到 marshaler")
	}
	// 同一个 mux 对同样的请求必须给出同一个 marshaler 实例
	if first != second {
		t.Error("共享 mux 每次给出不同的 marshaler，说明没有真正复用")
	}
}

// BenchmarkMarshalerLookup 共享与每请求新建的对比，数字写在 marshalerMux 的注释里
func BenchmarkMarshalerLookup(b *testing.B) {
	r := httptest.NewRequest("POST", "/v1/x", nil)
	r.Header.Set("Content-Type", "application/json")

	b.Run("Shared", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m, _ := runtime.MarshalerForRequest(marshalerMux, r)
			_ = m
		}
	})
	b.Run("PerRequestNewServeMux", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			m, _ := runtime.MarshalerForRequest(runtime.NewServeMux(), r)
			_ = m
		}
	})
}

// closeTrackingBody 记录自己是否被 Close 过
type closeTrackingBody struct {
	*bytes.Reader
	closed bool
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
}

// TestReplayBodyClosesOriginal 回放请求体时必须把 Close 转交给原始 body
//
// 回归点：原先是 io.NopCloser(bytes.NewReader(bodyBytes))，原始 body 从此永远不会被 Close
// 同一个仓库里 observability/logging.go 的 prefixedBody 为这件事专门写了注释
// （「不用 io.NopCloser 包一层：那样原始 body 永远不会被 Close」），这里原本恰好踩的
// 就是它点明的那个坑
//
// [EN] Regression: it used to be io.NopCloser(bytes.NewReader(bodyBytes)), leaving the original
// body never closed. prefixedBody in observability/logging.go in this same repository carries a
// comment about exactly that, and this was precisely the pitfall it points out.
func TestReplayBodyClosesOriginal(t *testing.T) {
	payload := []byte(`{"id":"x"}`)
	orig := &closeTrackingBody{Reader: bytes.NewReader(payload)}

	replay := &replayBody{Reader: bytes.NewReader(payload), orig: orig}

	got, err := io.ReadAll(replay)
	if err != nil {
		t.Fatalf("回放读取失败: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("回放内容 = %q, want %q", got, payload)
	}

	if err := replay.Close(); err != nil {
		t.Fatalf("Close 返回错误: %v", err)
	}
	if !orig.closed {
		t.Error("原始 body 没有被 Close：HTTP/1.1 的 keep-alive 连接会泄漏")
	}
}

// TestReplayBodyNilOriginal orig 为 nil 时 Close 不得 panic
func TestReplayBodyNilOriginal(t *testing.T) {
	replay := &replayBody{Reader: bytes.NewReader(nil)}
	if err := replay.Close(); err != nil {
		t.Errorf("orig 为 nil 时 Close 返回错误: %v", err)
	}
}
