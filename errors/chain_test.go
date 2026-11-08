/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\chain_test.go
 * @Description: 具名构造器与链式构造器测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package errors

import (
	"context"
	stderrors "errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 具名构造器必须覆盖码表里的每一个码，否则调用方碰到漏掉的那个就又会自己包一层，
// 而「自己包一层」正是这批构造器要消灭的东西
//
// 常量无法反射枚举，所以直接读源码：码表是事实，测试对着事实比，而不是对着手写清单比
//
// [EN] Every code in the table needs a constructor: a missing one sends the caller back to
// hand-wrapping, which is exactly what these constructors exist to remove. Constants are not
// reflectable, so the table is read from source — the test compares against the fact, not a list
func TestEveryBizErrCodeHasANamedConstructor(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	require.NoError(t, err)
	pkg, ok := pkgs["errors"]
	require.True(t, ok, "没有解析到 errors 包")

	const prefix = "BizErrCode"
	codeNames := map[string]string{} // 构造器名 -> 常量名
	funcs := map[string]bool{}

	for path, file := range pkg.Files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok != token.CONST {
					continue
				}
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, name := range vs.Names {
						if strings.HasPrefix(name.Name, prefix) {
							codeNames[strings.TrimPrefix(name.Name, prefix)] = name.Name
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					funcs[d.Name.Name] = true
				}
			}
		}
	}

	require.NotEmpty(t, codeNames, "码表为空，解析逻辑坏了")
	for ctor, constName := range codeNames {
		assert.True(t, funcs[ctor], "常量 %s 没有对应的具名构造器 func %s(ctx context.Context) error", constName, ctor)
	}
}

// 码表里的每个码都必须有默认映射与中英内置文案，缺一个就会在线上变成
// 「msg 是 error.xxx」或「gRPC 码兜底成 Internal」
// [EN] Every code needs a default mapping and builtin copy in both languages; a gap shows up
// live as a msg reading error.xxx, or a status silently falling back to Internal
func TestEveryBizErrCodeHasMappingAndBuiltinCopy(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	require.NoError(t, err)
	pkg := pkgs["errors"]
	require.NotNil(t, pkg)

	var codes1 []string
	for path, file := range pkg.Files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			d, ok := decl.(*ast.GenDecl)
			if !ok || d.Tok != token.CONST {
				continue
			}
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if !strings.HasPrefix(name.Name, "BizErrCode") || i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if ok {
						codes1 = append(codes1, strings.Trim(lit.Value, `"`))
					}
				}
			}
		}
	}

	require.NotEmpty(t, codes1)
	for _, code := range codes1 {
		t.Run(code, func(t *testing.T) {
			mappingMu.RLock()
			_, registered := mappings[code]
			mappingMu.RUnlock()
			assert.True(t, registered, "缺少默认映射")
			assert.NotEmpty(t, BuiltinMessage("zh", code), "缺少中文内置文案")
			assert.NotEmpty(t, BuiltinMessage("en", code), "缺少英文内置文案")
		})
	}
}

func TestNamedConstructorsCarryCodeAndMessage(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		got  error
		code codes.Code
	}{
		{"Unauthenticated", Unauthenticated(ctx), codes.Unauthenticated},
		{"NotFound", NotFound(ctx), codes.NotFound},
		{"PermissionDenied", PermissionDenied(ctx), codes.PermissionDenied},
		{"InvalidArgument", InvalidArgument(ctx), codes.InvalidArgument},
		{"DatabaseOperationFailed", DatabaseOperationFailed(ctx), MapBizCodeToGRPCCode(BizErrCodeDatabaseOperationFailed)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, ok := status.FromError(tt.got)
			require.True(t, ok, "不是 gRPC status 错误")
			assert.Equal(t, tt.code, st.Code())
			assert.NotEmpty(t, st.Message(), "消息不能为空，否则调用方拿不到任何可读信息")
		})
	}
}

// 具名构造器与两段式写法必须完全等价，否则换过去会悄悄改变线上行为
// [EN] The named form must be equivalent to the two-step call, or migrating to it would
// silently change what callers already return
func TestNamedConstructorEqualsTwoStepForm(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, ToGRPCError(ctx, BizErrCodeUnauthenticated).Error(), Unauthenticated(ctx).Error())
	assert.Equal(t, ToGRPCError(ctx, BizErrCodeNotFound).Error(), NotFound(ctx).Error())
}

func TestChainWithoutTemplateEqualsPlainConversion(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, ToGRPCError(ctx, BizErrCodeNotFound).Error(), Of(BizErrCodeNotFound).Err(ctx).Error())
}

func TestChainCauseFillsReasonPlaceholder(t *testing.T) {
	ctx := context.Background()

	withCause := Of(BizErrCodeInvalidParameter).Cause(stderrors.New("port out of range")).Err(ctx)
	st, ok := status.FromError(withCause)
	require.True(t, ok)
	assert.Contains(t, st.Message(), "port out of range")
	assert.NotContains(t, st.Message(), "{reason}", "内置文案兜底时模板必须套上")
}

// nil cause 不改变链，调用方才能无条件 .Cause(err)
// [EN] A nil cause must leave the chain untouched so callers never branch on nil
func TestChainNilCauseDegradesToPlainError(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t,
		Of(BizErrCodeDatabaseError).Err(ctx).Error(),
		Of(BizErrCodeDatabaseError).Cause(nil).Err(ctx).Error(),
	)
}

func TestChainWithMapMergesAndOverrides(t *testing.T) {
	chain := Of(BizErrCodeInvalidParameter).
		With("reason", "first").
		WithMap(map[string]interface{}{"reason": "second", "field": "port"})

	assert.Equal(t, "second", chain.template["reason"])
	assert.Equal(t, "port", chain.template["field"])
}

func TestChainMessageSkipsGRPCWrapping(t *testing.T) {
	ctx := context.Background()

	plain := Of(BizErrCodeNotFound).Message(ctx)
	assert.NotEmpty(t, plain)

	st, _ := status.FromError(NotFound(ctx))
	assert.Equal(t, st.Message(), plain, "Message 与 Err 必须解析出同一条文案")

	templated := Of(BizErrCodeInvalidParameter).Cause(stderrors.New("boom")).Message(ctx)
	assert.Contains(t, templated, "boom")
}

// 服务自注册的码也能走链式，不需要在服务里再包一层
// [EN] Service-registered codes work through the chain too, so services need no wrapper
func TestChainWorksForServiceRegisteredCode(t *testing.T) {
	const quotaExceeded = "error.quota_exceeded"
	Register(quotaExceeded, CodeMapping{codes.ResourceExhausted, ErrCodeTooManyRequests})

	st, ok := status.FromError(Of(quotaExceeded).With("limit", 10).Err(context.Background()))
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	assert.Equal(t, ErrCodeTooManyRequests, MapBizCodeToGatewayCode(quotaExceeded))
}

func TestNilChainIsSafe(t *testing.T) {
	var chain *Chain
	assert.Nil(t, chain.With("k", "v"))
	assert.Nil(t, chain.WithMap(map[string]interface{}{"k": "v"}))
	assert.Nil(t, chain.Cause(stderrors.New("x")))
	assert.NoError(t, chain.Err(context.Background()))
	assert.Empty(t, chain.Message(context.Background()))
}
