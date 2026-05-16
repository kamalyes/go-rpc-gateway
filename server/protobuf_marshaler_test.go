/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-16 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-17 01:15:11
 * @FilePath: \go-rpc-gateway\server\protobuf_marshaler_test.go
 * @Description: Protobuf Marshaler 测试 + grpc-gateway 内容协商集成测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	gwRuntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestProtobufMarshaler_ContentType(t *testing.T) {
	m := &protobufMarshaler{}
	assert.Equal(t, "application/x-protobuf", m.ContentType(nil))
}

func TestProtobufMarshaler_MarshalNonProtoMessage(t *testing.T) {
	m := &protobufMarshaler{}
	_, err := m.Marshal("not a proto message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a proto.Message")
}

func TestProtobufMarshaler_UnmarshalNonProtoMessage(t *testing.T) {
	m := &protobufMarshaler{}
	err := m.Unmarshal([]byte{0x01, 0x02}, "not a proto message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a proto.Message")
}

func TestProtobufMarshaler_NewEncoder(t *testing.T) {
	m := &protobufMarshaler{}
	var buf bytes.Buffer
	enc := m.NewEncoder(&buf)
	assert.NotNil(t, enc)
}

func TestProtobufMarshaler_NewDecoder(t *testing.T) {
	m := &protobufMarshaler{}
	r := bytes.NewReader([]byte{0x01, 0x02})
	dec := m.NewDecoder(r)
	assert.NotNil(t, dec)
}

func TestProtobufEncoder_NonProtoMessage(t *testing.T) {
	var buf bytes.Buffer
	enc := &protobufEncoder{w: &buf}
	err := enc.Encode("not a proto message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a proto.Message")
}

func TestProtobufDecoder_NonProtoMessage(t *testing.T) {
	r := bytes.NewReader([]byte{0x01, 0x02})
	dec := &protobufDecoder{r: r}
	err := dec.Decode("not a proto message")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a proto.Message")
}

func TestProtobufMarshaler_RoundTrip(t *testing.T) {
	m := &protobufMarshaler{}

	original := wrapperspb.String("test-service")

	data, err := m.Marshal(original)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	received := &wrapperspb.StringValue{}
	err = m.Unmarshal(data, received)
	require.NoError(t, err)

	assert.Equal(t, original.GetValue(), received.GetValue())
}

func TestProtobufMarshaler_Int32RoundTrip(t *testing.T) {
	m := &protobufMarshaler{}

	original := wrapperspb.Int32(42)

	data, err := m.Marshal(original)
	require.NoError(t, err)

	received := &wrapperspb.Int32Value{}
	err = m.Unmarshal(data, received)
	require.NoError(t, err)

	assert.Equal(t, original.GetValue(), received.GetValue())
}

func TestProtobufMarshaler_EncoderDecoderRoundTrip(t *testing.T) {
	m := &protobufMarshaler{}

	original := wrapperspb.String("encoder-test")

	var buf bytes.Buffer
	enc := m.NewEncoder(&buf)
	err := enc.Encode(original)
	require.NoError(t, err)

	dec := m.NewDecoder(&buf)
	received := &wrapperspb.StringValue{}
	err = dec.Decode(received)
	require.NoError(t, err)

	assert.Equal(t, original.GetValue(), received.GetValue())
}

func TestProtobufMarshaler_CompatibleWithProtoMarshal(t *testing.T) {
	m := &protobufMarshaler{}

	original := wrapperspb.String("compatibility-test")

	protoData, err := proto.Marshal(original)
	require.NoError(t, err)

	received := &wrapperspb.StringValue{}
	err = m.Unmarshal(protoData, received)
	require.NoError(t, err)

	assert.Equal(t, original.GetValue(), received.GetValue())
}

func TestProtobufMarshaler_MarshalProducesProtoCompatible(t *testing.T) {
	m := &protobufMarshaler{}

	original := wrapperspb.String("reverse-compat")

	data, err := m.Marshal(original)
	require.NoError(t, err)

	received := &wrapperspb.StringValue{}
	err = proto.Unmarshal(data, received)
	require.NoError(t, err)

	assert.Equal(t, original.GetValue(), received.GetValue())
}

func TestProtobufDecoder_EmptyData(t *testing.T) {
	r := bytes.NewReader(nil)
	dec := &protobufDecoder{r: r}

	received := &wrapperspb.StringValue{}
	err := dec.Decode(received)
	require.NoError(t, err)
	assert.Equal(t, "", received.GetValue())
}

func TestProtobufMarshaler_LargeData(t *testing.T) {
	m := &protobufMarshaler{}

	largeStr := make([]byte, 10000)
	for i := range largeStr {
		largeStr[i] = byte('a' + i%26)
	}

	original := wrapperspb.String(string(largeStr))

	data, err := m.Marshal(original)
	require.NoError(t, err)

	received := &wrapperspb.StringValue{}
	err = m.Unmarshal(data, received)
	require.NoError(t, err)

	assert.Equal(t, original.GetValue(), received.GetValue())
}

func TestProtobufEncoder_WriteError(t *testing.T) {
	enc := &protobufEncoder{w: &errorWriter{}}
	err := enc.Encode(wrapperspb.String("test"))
	assert.Error(t, err)
}

type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (int, error) {
	return 0, io.ErrShortWrite
}

func newTestServeMux(withProtobuf bool) *gwRuntime.ServeMux {
	opts := []gwRuntime.ServeMuxOption{
		gwRuntime.WithMarshalerOption(gwRuntime.MIMEWildcard, &gwRuntime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   true,
				EmitUnpopulated: true,
			},
		}),
	}
	if withProtobuf {
		opts = append(opts, gwRuntime.WithMarshalerOption("application/x-protobuf", &protobufMarshaler{}))
		opts = append(opts, gwRuntime.WithMarshalerOption("application/protobuf", &protobufMarshaler{}))
	}
	return gwRuntime.NewServeMux(opts...)
}

func TestContentNegotiation_JSON(t *testing.T) {
	mux := newTestServeMux(true)

	err := mux.HandlePath("GET", "/test", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		resp := wrapperspb.String("hello-json")
		data, _ := protojson.Marshal(resp)
		w.Write(data)
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, rec.Body.String(), "hello-json")
}

func TestContentNegotiation_Protobuf(t *testing.T) {
	mux := newTestServeMux(true)

	err := mux.HandlePath("GET", "/test", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		resp := wrapperspb.String("hello-protobuf")
		data, _ := proto.Marshal(resp)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.Write(data)
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/x-protobuf")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	received := &wrapperspb.StringValue{}
	err = proto.Unmarshal(rec.Body.Bytes(), received)
	require.NoError(t, err)
	assert.Equal(t, "hello-protobuf", received.GetValue())
}

func TestContentNegotiation_DefaultIsJSON(t *testing.T) {
	mux := newTestServeMux(true)

	err := mux.HandlePath("GET", "/test", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		resp := wrapperspb.String("default-json")
		data, _ := protojson.Marshal(resp)
		w.Write(data)
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "default-json")
}

func TestContentNegotiation_ProtobufAndJSON_ProduceDifferentOutput(t *testing.T) {
	mux := newTestServeMux(true)
	msg := wrapperspb.String("same-data")

	err := mux.HandlePath("GET", "/test", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		accept := r.Header.Get("Accept")
		switch accept {
		case "application/x-protobuf":
			data, _ := proto.Marshal(msg)
			w.Header().Set("Content-Type", "application/x-protobuf")
			w.Write(data)
		default:
			data, _ := protojson.Marshal(msg)
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
		}
	})
	require.NoError(t, err)

	reqJSON := httptest.NewRequest("GET", "/test", nil)
	reqJSON.Header.Set("Accept", "application/json")
	recJSON := httptest.NewRecorder()
	mux.ServeHTTP(recJSON, reqJSON)

	reqPB := httptest.NewRequest("GET", "/test", nil)
	reqPB.Header.Set("Accept", "application/x-protobuf")
	recPB := httptest.NewRecorder()
	mux.ServeHTTP(recPB, reqPB)

	assert.NotEqual(t, recJSON.Body.Bytes(), recPB.Body.Bytes(), "JSON 和 Protobuf 输出应该不同")

	var jsonResult wrapperspb.StringValue
	err = protojson.Unmarshal(recJSON.Body.Bytes(), &jsonResult)
	require.NoError(t, err)
	assert.Equal(t, "same-data", jsonResult.GetValue())

	var pbResult wrapperspb.StringValue
	err = proto.Unmarshal(recPB.Body.Bytes(), &pbResult)
	require.NoError(t, err)
	assert.Equal(t, "same-data", pbResult.GetValue())
}

func TestContentNegotiation_ProtobufSizeSmallerThanJSON(t *testing.T) {
	msg := wrapperspb.String("size-comparison-test-data-with-longer-string")

	jsonData, err := protojson.Marshal(msg)
	require.NoError(t, err)

	pbData, err := proto.Marshal(msg)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(pbData), len(jsonData), "Protobuf 二进制应该不大于 JSON")
}

func TestContentNegotiation_WithoutProtobufMarshaler_AcceptProtobufFallsBackToJSON(t *testing.T) {
	mux := newTestServeMux(false)

	err := mux.HandlePath("GET", "/test", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		resp := wrapperspb.String("fallback")
		data, _ := protojson.Marshal(resp)
		w.Write(data)
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/x-protobuf")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "fallback")
}
