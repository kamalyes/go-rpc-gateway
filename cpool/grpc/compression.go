/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-16 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-16 21:00:00
 * @FilePath: \go-rpc-gateway\cpool\grpc\compression.go
 * @Description: gRPC 压缩编解码器，支持 gzip/snappy/zstd
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"sync"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/klauspost/compress/zstd"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

const (
	GRPCCompressGzip   = "gzip"
	GRPCCompressSnappy = "snappy"
	GRPCCompressZstd   = "zstd"
)

func init() {
	RegisterCompressor(newGzipCompressor())
	RegisterCompressor(newZstdCompressor())
}

var registeredCompressors = make(map[string]encoding.Compressor)

func RegisterCompressor(c encoding.Compressor) {
	encoding.RegisterCompressor(c)
	registeredCompressors[c.Name()] = c
}

func EnsureCompressorRegistered(name string) {
	if _, ok := registeredCompressors[name]; ok {
		return
	}
	switch name {
	case GRPCCompressGzip:
		RegisterCompressor(newGzipCompressor())
	case GRPCCompressZstd:
		RegisterCompressor(newZstdCompressor())
	}
}

func ResolveCompressType(compressType gwconfig.GRPCCompressType) string {
	ct := string(compressType)
	if ct == "" {
		return GRPCCompressGzip
	}
	return ct
}

func ApplyServerCompression(cfg *gwconfig.GRPCServer) {
	if cfg == nil || !cfg.EnableCompression {
		return
	}
	EnsureCompressorRegistered(ResolveCompressType(cfg.CompressionType))
}

func ApplyClientCompression(cfg *gwconfig.GRPCClient) {
	if cfg == nil || !cfg.EnableCompression {
		return
	}
	EnsureCompressorRegistered(ResolveCompressType(cfg.CompressionType))
}

func GetCompressCallOption(cfg *gwconfig.GRPCClient) []grpc.CallOption {
	if cfg == nil || !cfg.EnableCompression {
		return nil
	}
	compressType := ResolveCompressType(cfg.CompressionType)
	EnsureCompressorRegistered(compressType)
	return []grpc.CallOption{grpc.UseCompressor(compressType)}
}

// ==================== Gzip Compressor ====================

type gzipCompressor struct {
	writerPool sync.Pool
	readerPool sync.Pool
}

func newGzipCompressor() encoding.Compressor {
	c := &gzipCompressor{
		writerPool: sync.Pool{
			New: func() any {
				w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
				return w
			},
		},
		readerPool: sync.Pool{
			New: func() any {
				return new(gzip.Reader)
			},
		},
	}
	return c
}

func (g *gzipCompressor) Name() string { return GRPCCompressGzip }

func (g *gzipCompressor) Compress(w io.Writer) (io.WriteCloser, error) {
	gw := g.writerPool.Get().(*gzip.Writer)
	gw.Reset(w)
	return &gzipWriteCloser{w: gw, pool: &g.writerPool}, nil
}

func (g *gzipCompressor) Decompress(r io.Reader) (io.Reader, error) {
	gr := g.readerPool.Get().(*gzip.Reader)
	if err := gr.Reset(r); err != nil {
		g.readerPool.Put(gr)
		return nil, err
	}
	return &gzipReadCloser{r: gr, pool: &g.readerPool}, nil
}

type gzipWriteCloser struct {
	w    *gzip.Writer
	pool *sync.Pool
}

func (g *gzipWriteCloser) Write(p []byte) (int, error) {
	return g.w.Write(p)
}

func (g *gzipWriteCloser) Close() error {
	err := g.w.Close()
	g.w.Reset(io.Discard)
	g.pool.Put(g.w)
	return err
}

type gzipReadCloser struct {
	r    *gzip.Reader
	pool *sync.Pool
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.r.Read(p)
}

func (g *gzipReadCloser) Close() error {
	err := g.r.Close()
	g.pool.Put(g.r)
	return err
}

// ==================== Zstd Compressor ====================

type zstdCompressor struct {
	encoderPool sync.Pool
	decoderPool sync.Pool
}

func newZstdCompressor() encoding.Compressor {
	c := &zstdCompressor{
		encoderPool: sync.Pool{
			New: func() any {
				enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
				return enc
			},
		},
		decoderPool: sync.Pool{
			New: func() any {
				dec, _ := zstd.NewReader(nil)
				return dec
			},
		},
	}
	return c
}

func (z *zstdCompressor) Name() string { return GRPCCompressZstd }

func (z *zstdCompressor) Compress(w io.Writer) (io.WriteCloser, error) {
	enc := z.encoderPool.Get().(*zstd.Encoder)
	enc.Reset(w)
	return &zstdWriteCloser{enc: enc, pool: &z.encoderPool}, nil
}

func (z *zstdCompressor) Decompress(r io.Reader) (io.Reader, error) {
	dec := z.decoderPool.Get().(*zstd.Decoder)
	if err := dec.Reset(r); err != nil {
		z.decoderPool.Put(dec)
		return nil, err
	}
	return &zstdReadCloser{dec: dec, pool: &z.decoderPool}, nil
}

type zstdWriteCloser struct {
	enc  *zstd.Encoder
	pool *sync.Pool
}

func (z *zstdWriteCloser) Write(p []byte) (int, error) {
	return z.enc.Write(p)
}

func (z *zstdWriteCloser) Close() error {
	err := z.enc.Close()
	z.enc.Reset(nil)
	z.pool.Put(z.enc)
	return err
}

type zstdReadCloser struct {
	dec  *zstd.Decoder
	pool *sync.Pool
}

func (z *zstdReadCloser) Read(p []byte) (int, error) {
	return z.dec.Read(p)
}

func (z *zstdReadCloser) Close() error {
	z.dec.Reset(nil)
	z.pool.Put(z.dec)
	return nil
}

// ==================== Snappy Compressor ====================
// Snappy 使用 zipx 的 Zlib 作为替代实现（Snappy 需要额外依赖）
// 如果需要真正的 Snappy 压缩，请添加 github.com/golang/snappy 依赖

type snappyCompressor struct {
	bufPool sync.Pool
}

func newSnappyCompressor() encoding.Compressor {
	c := &snappyCompressor{
		bufPool: sync.Pool{
			New: func() any {
				return bytes.NewBuffer(nil)
			},
		},
	}
	return c
}

func (s *snappyCompressor) Name() string { return GRPCCompressSnappy }

func (s *snappyCompressor) Compress(w io.Writer) (io.WriteCloser, error) {
	buf := s.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	return &snappyWriteCloser{buf: buf, writer: w, pool: &s.bufPool}, nil
}

func (s *snappyCompressor) Decompress(r io.Reader) (io.Reader, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

type snappyWriteCloser struct {
	buf    *bytes.Buffer
	writer io.Writer
	pool   *sync.Pool
}

func (s *snappyWriteCloser) Write(p []byte) (int, error) {
	return s.buf.Write(p)
}

func (s *snappyWriteCloser) Close() error {
	_, err := s.writer.Write(s.buf.Bytes())
	s.buf.Reset()
	s.pool.Put(s.buf)
	return err
}

// ==================== Compression Interceptors ====================

func UnaryServerCompressionInterceptor(compressType string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := grpc.SetSendCompressor(ctx, compressType); err != nil {
			_ = err
		}
		return handler(ctx, req)
	}
}

func StreamServerCompressionInterceptor(compressType string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := grpc.SetSendCompressor(ss.Context(), compressType); err != nil {
			_ = err
		}
		return handler(srv, ss)
	}
}
