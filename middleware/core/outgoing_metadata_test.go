/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\core\outgoing_metadata_test.go
 * @Description: 出站 metadata 注入的语义测试 - 锁住「跳过空值」与「已有值优先」
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package core

import (
	"context"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/kamalyes/go-rpc-gateway/constants"
	goi18n "github.com/kamalyes/go-toolbox/pkg/i18n"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"google.golang.org/grpc/metadata"
)

// allMetadataKeys injectTraceToOutgoingContext 会写入的全部键
func allMetadataKeys() []string {
	return []string{
		constants.MetadataID, constants.MetadataTraceID, constants.MetadataRequestID,
		constants.MetadataAuthorization, constants.MetadataUserID, constants.MetadataUserName,
		constants.MetadataDomain, constants.MetadataRoleCode, constants.MetadataTenantID,
		constants.MetadataTenantCode, constants.MetadataSessionID, constants.MetadataTimezone,
		constants.MetadataIPAddress, constants.MetadataAppID, constants.MetadataDeviceID,
		constants.MetadataAppVersion, constants.MetadataPlatformID, constants.MetadataPlatformCode,
		constants.MetadataRegionID, constants.MetadataRegionCode, constants.MetadataAgentLineID,
		constants.MetadataNonce, constants.MetadataJti, constants.MetadataFamilyId,
		constants.MetadataXNsID, constants.MetadataUserAgent, constants.MetadataPushToken,
		constants.MetadataToken, constants.MetadataTimestamp, constants.MetadataSignature,
		constants.MetadataAccessKey, constants.MetadataAcceptLanguage,
		constants.MetadataForwardedHost, constants.MetadataServerNode,
	}
}

// TestMetadataKeysAreLowercase 锁住 injectTraceToOutgoingContext 绕过 ToLower 的前提
//
// 该函数把常量直接当 metadata 键用，不再经 strings.ToLower
// 一旦有人往 constants 里加了带大写的键，gRPC 侧的键会与下游读取用的常量不一致，
// 而且不会有任何报错 —— 所以前提必须由测试守住，而不是靠注释
//
// [EN] Locks the precondition under which injectTraceToOutgoingContext skips ToLower: it
// uses the constants directly as metadata keys. If someone adds a key with an uppercase
// letter, the key on the wire stops matching the constant the downstream reads with, and
// nothing reports an error -- so the precondition is held by a test, not by a comment
func TestMetadataKeysAreLowercase(t *testing.T) {
	for _, k := range allMetadataKeys() {
		if k != strings.ToLower(k) {
			t.Errorf("metadata 键 %q 含大写字母；injectTraceToOutgoingContext 直接用常量作键，"+
				"不会再过 ToLower，请改成全小写", k)
		}
	}
}

// TestInjectOutgoingSkipsEmptyValues 空值不进 metadata，非空值逐字进
func TestInjectOutgoingSkipsEmptyValues(t *testing.T) {
	meta := &RequestCommonMeta{
		TraceID:   "trace-abc",
		RequestID: "req-def",
		UserID:    "u-1",
	}
	ctx := WithRequestCommonMeta(context.Background(), meta)
	out := injectTraceToOutgoingContext(ctx)

	md, ok := metadata.FromOutgoingContext(out)
	if !ok {
		t.Fatal("注入后取不到 outgoing metadata")
	}

	// 非空的三项必须在，且值逐字相同
	for k, want := range map[string]string{
		constants.MetadataTraceID:   "trace-abc",
		constants.MetadataRequestID: "req-def",
		constants.MetadataUserID:    "u-1",
	} {
		if got := md.Get(k); len(got) != 1 || got[0] != want {
			t.Errorf("键 %s 期望 [%q]，实际 %v", k, want, got)
		}
	}

	// 空值字段一律不出现
	for _, k := range []string{
		constants.MetadataAuthorization, constants.MetadataToken,
		constants.MetadataSignature, constants.MetadataTenantID,
	} {
		if got := md.Get(k); len(got) != 0 {
			t.Errorf("键 %s 的值为空，本不该注入，实际 %v", k, got)
		}
	}

	// 对读取方而言「键不存在」与「键存在但值为空」等价，这是跳过空值的正当性所在
	// [EN] For a reader an absent key and a present-but-empty one are equivalent, which is
	// what makes skipping empty values sound
	if v := firstValueOf(md, constants.MetadataToken); v != "" {
		t.Errorf("缺失键的读取结果应为空串，实际 %q", v)
	}
}

// firstValueOf 复刻 enrichContextFromMetadata 里 firstMetadataValue 的读取方式
func firstValueOf(md metadata.MD, key string) string {
	if values := md.Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

// TestInjectOutgoingPreservesExistingPrecedence 已有 outgoing metadata 的值仍排在前面
//
// 这一条锁住的是从 Join(existingMD, md) 换成 AppendToOutgoingContext 之后的顺序语义：
// 下游 firstMetadataValue 取 values[0]，所以谁在前决定了谁生效
//
// [EN] Locks the ordering semantics after Join(existingMD, md) was replaced by
// AppendToOutgoingContext: the downstream takes values[0], so whichever comes first wins
func TestInjectOutgoingPreservesExistingPrecedence(t *testing.T) {
	meta := &RequestCommonMeta{TraceID: "trace-from-meta"}
	ctx := WithRequestCommonMeta(context.Background(), meta)
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		constants.MetadataTraceID, "trace-already-outgoing",
	))

	out := injectTraceToOutgoingContext(ctx)
	md, ok := metadata.FromOutgoingContext(out)
	if !ok {
		t.Fatal("注入后取不到 outgoing metadata")
	}

	got := md.Get(constants.MetadataTraceID)
	want := []string{"trace-already-outgoing", "trace-from-meta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("trace-id 的值序列期望 %v，实际 %v；已有值必须排在本层注入之前", want, got)
	}
	if v := firstValueOf(md, constants.MetadataTraceID); v != "trace-already-outgoing" {
		t.Errorf("下游读到的应是已有值 trace-already-outgoing，实际 %q", v)
	}
}

// TestInjectOutgoingMatchesLegacyForNonEmpty 与改造前实现做差分：非空字段的结果必须完全一致
//
// legacyInject 是改造前的实现原样保留，用来证明这次改动只去掉了空值，
// 没有动任何非空字段的键、值或顺序
//
// [EN] A differential test against the pre-change implementation, kept verbatim as
// legacyInject, proving this change only removed empty values and altered no key, value or
// ordering of any non-empty field
func TestInjectOutgoingMatchesLegacyForNonEmpty(t *testing.T) {
	for _, shape := range benchMetaShapes() {
		t.Run(shape.name, func(t *testing.T) {
			base := WithRequestCommonMeta(context.Background(), shape.meta)

			newMD, _ := metadata.FromOutgoingContext(injectTraceToOutgoingContext(base))
			oldMD, _ := metadata.FromOutgoingContext(legacyInject(base))

			// 旧实现里每个非空值，新实现必须给出完全相同的值序列
			for k, oldVals := range oldMD {
				nonEmpty := make([]string, 0, len(oldVals))
				for _, v := range oldVals {
					if v != "" {
						nonEmpty = append(nonEmpty, v)
					}
				}
				newVals := newMD.Get(k)
				if len(nonEmpty) == 0 {
					if len(newVals) != 0 {
						t.Errorf("键 %s 旧实现只有空值，新实现却给出 %v", k, newVals)
					}
					continue
				}
				if !reflect.DeepEqual(newVals, nonEmpty) {
					t.Errorf("键 %s 期望 %v，实际 %v", k, nonEmpty, newVals)
				}
			}

			// 反向检查：新实现不得凭空多出键
			for k := range newMD {
				if _, ok := oldMD[k]; !ok {
					t.Errorf("新实现多出了旧实现没有的键 %s", k)
				}
			}
		})
	}
}

// legacyInject 改造前的 injectTraceToOutgoingContext，原样保留供差分测试使用
func legacyInject(ctx context.Context) context.Context {
	m := GetRequestCommonMeta(ctx)
	acceptLanguage := mathx.IfEmpty(m.AcceptLanguage, goi18n.GetLanguage(ctx))
	md := metadata.Pairs(
		constants.MetadataID, m.ID,
		constants.MetadataTraceID, m.TraceID,
		constants.MetadataRequestID, m.RequestID,
		constants.MetadataAuthorization, m.Authorization,
		constants.MetadataUserID, m.UserID,
		constants.MetadataUserName, url.QueryEscape(m.UserName),
		constants.MetadataDomain, m.Domain,
		constants.MetadataRoleCode, m.RoleCode,
		constants.MetadataTenantID, m.TenantID,
		constants.MetadataTenantCode, m.TenantCode,
		constants.MetadataSessionID, m.SessionID,
		constants.MetadataTimezone, m.Timezone,
		constants.MetadataIPAddress, m.IPAddress,
		constants.MetadataAppID, m.AppID,
		constants.MetadataDeviceID, m.DeviceID,
		constants.MetadataAppVersion, m.AppVersion,
		constants.MetadataPlatformID, m.PlatformID,
		constants.MetadataPlatformCode, m.PlatformCode,
		constants.MetadataRegionID, m.RegionID,
		constants.MetadataRegionCode, m.RegionCode,
		constants.MetadataAgentLineID, m.AgentLineID,
		constants.MetadataNonce, m.Nonce,
		constants.MetadataJti, m.Jti,
		constants.MetadataFamilyId, m.FamilyId,
		constants.MetadataXNsID, m.XNsID,
		constants.MetadataUserAgent, m.UserAgent,
		constants.MetadataPushToken, m.PushToken,
		constants.MetadataToken, m.Token,
		constants.MetadataTimestamp, m.Timestamp,
		constants.MetadataSignature, m.Signature,
		constants.MetadataAccessKey, m.AccessKey,
		constants.MetadataAcceptLanguage, acceptLanguage,
		constants.MetadataForwardedHost, m.ForwardedHost,
	)
	if existingMD, ok := metadata.FromOutgoingContext(ctx); ok {
		md = metadata.Join(existingMD, md)
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// TestResponseMetadataWithholdsCredentials 响应 metadata 不得带回凭据
//
// 出站方向走 grpc-gateway 的 defaultOutgoingHeaderMatcher，它对每个键都返回
// (Grpc-Metadata-<键>, true)，所以这里放进去的东西会变成 HTTP 响应头直达客户端
//
// [EN] The outgoing direction uses grpc-gateway's defaultOutgoingHeaderMatcher, which returns
// (Grpc-Metadata-<key>, true) for every key, so whatever is put here becomes an HTTP response
// header reaching the client
func TestResponseMetadataWithholdsCredentials(t *testing.T) {
	meta := &RequestCommonMeta{
		TraceID:       "trace-abc",
		UserID:        "u-1",
		Authorization: "Bearer eyJhbGciOiJIUzI1NiJ9.payload.signature",
		Token:         "tok-secret",
		AccessKey:     "AKIAsecret",
		Signature:     "sig-secret",
		PushToken:     "push-secret",
	}
	ctx := WithRequestCommonMeta(context.Background(), meta)

	md := buildResponseMetadata(ctx)

	for _, k := range responseMetadataWithheldKeys {
		if got := md.Get(k); len(got) != 0 {
			t.Errorf("凭据键 %s 不该出现在响应 metadata 中（会经 Grpc-Metadata-%s 回显给客户端），实际 %v", k, k, got)
		}
	}

	// 非凭据字段仍要正常回显，不能过度摘除
	// [EN] Non-credential fields must still be echoed; withholding must not overreach
	if got := md.Get(constants.MetadataTraceID); len(got) != 1 || got[0] != "trace-abc" {
		t.Errorf("trace-id 应正常回显，实际 %v", got)
	}
	if got := md.Get(constants.MetadataUserID); len(got) != 1 || got[0] != "u-1" {
		t.Errorf("user-id 应正常回显，实际 %v", got)
	}

	// 空值不占响应头
	if got := md.Get(constants.MetadataTenantID); len(got) != 0 {
		t.Errorf("空值字段不该出现，实际 %v", got)
	}
}
