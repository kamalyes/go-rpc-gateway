/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-16 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-17 01:09:33
 * @FilePath: \go-rpc-gateway\cpool\grpc\compression_test.go
 * @Description: gRPC 压缩编解码器测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/encoding"
)

func TestGzipCompressor_Name(t *testing.T) {
	c := newGzipCompressor()
	assert.Equal(t, GRPCCompressGzip, c.Name())
}

func TestGzipCompressor_CompressDecompress(t *testing.T) {
	c := newGzipCompressor()
	original := []byte("hello gRPC compression test data with some longer content to ensure compression works properly")

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)

	_, err = wc.Write(original)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	rc, err := c.Decompress(&buf)
	require.NoError(t, err)

	decompressed, err := io.ReadAll(rc)
	require.NoError(t, err)
	if closer, ok := rc.(io.Closer); ok {
		require.NoError(t, closer.Close())
	}

	assert.Equal(t, original, decompressed, "decompressed data should match original")
}

func TestGzipCompressor_EmptyData(t *testing.T) {
	c := newGzipCompressor()
	original := []byte("")

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	rc, err := c.Decompress(&buf)
	require.NoError(t, err)

	decompressed, err := io.ReadAll(rc)
	require.NoError(t, err)
	if closer, ok := rc.(io.Closer); ok {
		require.NoError(t, closer.Close())
	}

	assert.Equal(t, original, decompressed)
}

func TestGzipCompressor_LargeData(t *testing.T) {
	c := newGzipCompressor()
	original := make([]byte, 1024*100)
	for i := range original {
		original[i] = byte(i % 256)
	}

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)

	_, err = wc.Write(original)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	rc, err := c.Decompress(&buf)
	require.NoError(t, err)

	decompressed, err := io.ReadAll(rc)
	require.NoError(t, err)
	if closer, ok := rc.(io.Closer); ok {
		require.NoError(t, closer.Close())
	}

	assert.Equal(t, original, decompressed)
}

func TestGzipCompressor_MultipleRounds(t *testing.T) {
	c := newGzipCompressor()

	for i := 0; i < 5; i++ {
		original := []byte("round test data compression cycle")
		var buf bytes.Buffer

		wc, err := c.Compress(&buf)
		require.NoError(t, err)

		_, err = wc.Write(original)
		require.NoError(t, err)
		require.NoError(t, wc.Close())

		rc, err := c.Decompress(&buf)
		require.NoError(t, err)

		decompressed, err := io.ReadAll(rc)
		require.NoError(t, err)
		if closer, ok := rc.(io.Closer); ok {
			require.NoError(t, closer.Close())
		}

		assert.Equal(t, original, decompressed, "round %d: decompressed data should match", i)
	}
}

func TestGzipCompressor_CompatibleWithStdGzip(t *testing.T) {
	c := newGzipCompressor()
	original := []byte("test compatibility with standard gzip")

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)

	_, err = wc.Write(original)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	gr, err := gzip.NewReader(&buf)
	require.NoError(t, err)
	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err)
	require.NoError(t, gr.Close())

	assert.Equal(t, original, decompressed, "should be compatible with standard gzip decompression")
}

func TestZstdCompressor_Name(t *testing.T) {
	c := newZstdCompressor()
	assert.Equal(t, GRPCCompressZstd, c.Name())
}

func TestZstdCompressor_CompressDecompress(t *testing.T) {
	c := newZstdCompressor()
	original := []byte("hello zstd compression test data with some longer content to ensure compression works properly")

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)

	_, err = wc.Write(original)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	rc, err := c.Decompress(&buf)
	require.NoError(t, err)

	decompressed, err := io.ReadAll(rc)
	require.NoError(t, err)
	if closer, ok := rc.(io.Closer); ok {
		require.NoError(t, closer.Close())
	}

	assert.Equal(t, original, decompressed)
}

func TestZstdCompressor_EmptyData(t *testing.T) {
	c := newZstdCompressor()
	original := []byte("")

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	rc, err := c.Decompress(&buf)
	require.NoError(t, err)

	decompressed, err := io.ReadAll(rc)
	require.NoError(t, err)
	if closer, ok := rc.(io.Closer); ok {
		require.NoError(t, closer.Close())
	}

	assert.Equal(t, original, decompressed)
}

func TestZstdCompressor_LargeData(t *testing.T) {
	c := newZstdCompressor()
	original := make([]byte, 1024*100)
	for i := range original {
		original[i] = byte(i % 256)
	}

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)

	_, err = wc.Write(original)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	rc, err := c.Decompress(&buf)
	require.NoError(t, err)

	decompressed, err := io.ReadAll(rc)
	require.NoError(t, err)
	if closer, ok := rc.(io.Closer); ok {
		require.NoError(t, closer.Close())
	}

	assert.Equal(t, original, decompressed)
}

func TestZstdCompressor_CompatibleWithKlauspostZstd(t *testing.T) {
	c := newZstdCompressor()
	original := []byte("test compatibility with klauspost zstd library")

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)

	_, err = wc.Write(original)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	dec, err := zstd.NewReader(&buf)
	require.NoError(t, err)
	decompressed, err := io.ReadAll(dec)
	require.NoError(t, err)

	assert.Equal(t, original, decompressed, "should be compatible with klauspost zstd decompression")
}

func TestZstdCompressor_MultipleRounds(t *testing.T) {
	c := newZstdCompressor()

	for i := 0; i < 5; i++ {
		original := []byte("round zstd test data compression cycle")
		var buf bytes.Buffer

		wc, err := c.Compress(&buf)
		require.NoError(t, err)

		_, err = wc.Write(original)
		require.NoError(t, err)
		require.NoError(t, wc.Close())

		rc, err := c.Decompress(&buf)
		require.NoError(t, err)

		decompressed, err := io.ReadAll(rc)
		require.NoError(t, err)
		if closer, ok := rc.(io.Closer); ok {
			require.NoError(t, closer.Close())
		}

		assert.Equal(t, original, decompressed, "round %d: decompressed data should match", i)
	}
}

func TestSnappyCompressor_Name(t *testing.T) {
	c := newSnappyCompressor()
	assert.Equal(t, GRPCCompressSnappy, c.Name())
}

func TestSnappyCompressor_CompressDecompress(t *testing.T) {
	c := newSnappyCompressor()
	original := []byte("hello snappy passthrough test")

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)

	_, err = wc.Write(original)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	rc, err := c.Decompress(&buf)
	require.NoError(t, err)

	decompressed, err := io.ReadAll(rc)
	require.NoError(t, err)

	assert.Equal(t, original, decompressed, "snappy passthrough should preserve data")
}

func TestRegisterCompressor(t *testing.T) {
	assert.NotNil(t, registeredCompressors[GRPCCompressGzip], "gzip should be registered in init()")
	assert.NotNil(t, registeredCompressors[GRPCCompressZstd], "zstd should be registered in init()")
}

func TestEnsureCompressorRegistered_AlreadyRegistered(t *testing.T) {
	EnsureCompressorRegistered(GRPCCompressGzip)
	_, ok := registeredCompressors[GRPCCompressGzip]
	assert.True(t, ok, "gzip should still be registered")
}

func TestApplyServerCompression_Disabled(t *testing.T) {
	cfg := &gwconfig.GRPCServer{EnableCompression: false}
	ApplyServerCompression(cfg)
}

func TestApplyServerCompression_Enabled(t *testing.T) {
	cfg := &gwconfig.GRPCServer{
		EnableCompression: true,
		CompressionType:   gwconfig.GRPCCompressGzip,
	}
	ApplyServerCompression(cfg)
	_, ok := registeredCompressors[GRPCCompressGzip]
	assert.True(t, ok)
}

func TestApplyServerCompression_Zstd(t *testing.T) {
	cfg := &gwconfig.GRPCServer{
		EnableCompression: true,
		CompressionType:   gwconfig.GRPCCompressZstd,
	}
	ApplyServerCompression(cfg)
	_, ok := registeredCompressors[GRPCCompressZstd]
	assert.True(t, ok)
}

func TestApplyServerCompression_Nil(t *testing.T) {
	ApplyServerCompression(nil)
}

func TestApplyServerCompression_DefaultType(t *testing.T) {
	cfg := &gwconfig.GRPCServer{
		EnableCompression: true,
		CompressionType:   "",
	}
	ApplyServerCompression(cfg)
	_, ok := registeredCompressors[GRPCCompressGzip]
	assert.True(t, ok, "empty compression type should default to gzip")
}

func TestApplyClientCompression_Disabled(t *testing.T) {
	cfg := &gwconfig.GRPCClient{EnableCompression: false}
	ApplyClientCompression(cfg)
}

func TestApplyClientCompression_Enabled(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		EnableCompression: true,
		CompressionType:   gwconfig.GRPCCompressGzip,
	}
	ApplyClientCompression(cfg)
}

func TestApplyClientCompression_Nil(t *testing.T) {
	ApplyClientCompression(nil)
}

func TestGetCompressCallOption_Disabled(t *testing.T) {
	cfg := &gwconfig.GRPCClient{EnableCompression: false}
	opts := GetCompressCallOption(cfg)
	assert.Nil(t, opts)
}

func TestGetCompressCallOption_Nil(t *testing.T) {
	opts := GetCompressCallOption(nil)
	assert.Nil(t, opts)
}

func TestGetCompressCallOption_Enabled(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		EnableCompression: true,
		CompressionType:   gwconfig.GRPCCompressGzip,
	}
	opts := GetCompressCallOption(cfg)
	assert.Len(t, opts, 1)
}

func TestCompressorRegisteredInGrpcEncoding(t *testing.T) {
	gzipCompressor := encoding.GetCompressor(GRPCCompressGzip)
	assert.NotNil(t, gzipCompressor, "gzip compressor should be registered in grpc encoding")

	zstdCompressor := encoding.GetCompressor(GRPCCompressZstd)
	assert.NotNil(t, zstdCompressor, "zstd compressor should be registered in grpc encoding")
}

func TestCompressionConstants(t *testing.T) {
	assert.Equal(t, "gzip", GRPCCompressGzip)
	assert.Equal(t, "snappy", GRPCCompressSnappy)
	assert.Equal(t, "zstd", GRPCCompressZstd)
}

func TestResolveCompressType(t *testing.T) {
	assert.Equal(t, GRPCCompressGzip, ResolveCompressType(""), "empty should default to gzip")
	assert.Equal(t, GRPCCompressGzip, ResolveCompressType(gwconfig.GRPCCompressGzip))
	assert.Equal(t, GRPCCompressZstd, ResolveCompressType(gwconfig.GRPCCompressZstd))
	assert.Equal(t, GRPCCompressSnappy, ResolveCompressType(gwconfig.GRPCCompressSnappy))
}

func TestGzipCompressor_CompressRatio(t *testing.T) {
	c := newGzipCompressor()
	original := make([]byte, 10*1024)
	for i := range original {
		original[i] = byte('a' + i%3)
	}

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)

	_, err = wc.Write(original)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	compressed := buf.Bytes()
	assert.Less(t, len(compressed), len(original), "highly repetitive data should compress well with gzip")
}

func TestZstdCompressor_CompressRatio(t *testing.T) {
	c := newZstdCompressor()
	original := make([]byte, 10*1024)
	for i := range original {
		original[i] = byte('a' + i%3)
	}

	var buf bytes.Buffer
	wc, err := c.Compress(&buf)
	require.NoError(t, err)

	_, err = wc.Write(original)
	require.NoError(t, err)
	require.NoError(t, wc.Close())

	compressed := buf.Bytes()
	assert.Less(t, len(compressed), len(original), "highly repetitive data should compress well with zstd")
}

func BenchmarkGzipCompress(b *testing.B) {
	c := newGzipCompressor()
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		wc, _ := c.Compress(&buf)
		wc.Write(data)
		wc.Close()
	}
}

func BenchmarkGzipDecompress(b *testing.B) {
	c := newGzipCompressor()
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	var compressed bytes.Buffer
	wc, _ := c.Compress(&compressed)
	wc.Write(data)
	wc.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := compressed.Bytes()
		rc, _ := c.Decompress(bytes.NewReader(buf))
		io.ReadAll(rc)
		if closer, ok := rc.(io.Closer); ok {
			closer.Close()
		}
	}
}

func BenchmarkZstdCompress(b *testing.B) {
	c := newZstdCompressor()
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		wc, _ := c.Compress(&buf)
		wc.Write(data)
		wc.Close()
	}
}

func BenchmarkZstdDecompress(b *testing.B) {
	c := newZstdCompressor()
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	var compressed bytes.Buffer
	wc, _ := c.Compress(&compressed)
	wc.Write(data)
	wc.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := compressed.Bytes()
		rc, _ := c.Decompress(bytes.NewReader(buf))
		io.ReadAll(rc)
		if closer, ok := rc.(io.Closer); ok {
			closer.Close()
		}
	}
}
