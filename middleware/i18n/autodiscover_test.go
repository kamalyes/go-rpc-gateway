/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-09-08 23:16:57 +0800
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-09-11 19:56:15 +0800
 * @FilePath: \go-rpc-gateway\middleware\i18n\autodiscover_test.go
 * @Description: 路由→请求消息自动发现的守卫
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package i18n

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	gwproto "github.com/kamalyes/go-rpc-gateway/proto"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// registerSyntheticRoute 合成一个带 google.api.http 注解的服务并注册进全局描述符表
//
// 用合成描述符而不是真实 proto：这条测试要验的是「注解在，表就自动建得出来」，
// 而不是某个具体协议长什么样。输入消息借用网关自己已注册的类型，
// 这样 GlobalTypes 里查得到对应的 Go 类型——自动发现取的正是它
//
// [EN] A synthesized descriptor rather than a real proto: what is under test is that an
// annotation alone is enough to build the table, not what any particular contract looks like.
// The input message borrows a type the gateway already registers, so GlobalTypes can resolve
// the concrete Go type — which is what discovery looks up
func registerSyntheticRoute(t *testing.T, path string) {
	t.Helper()

	slug := strings.NewReplacer("/", "_", "-", "_").Replace(path)
	input := string((&gwproto.Paging{}).ProtoReflect().Descriptor().FullName())
	fdp := &descriptorpb.FileDescriptorProto{
		// 文件名与包名都按路径取唯一：同一个测试二进制里，重复的文件名或重复的
		// 服务全名都会让 protoregistry panic
		// [EN] Both the file name and the package are unique per route: within one test binary a
		// duplicate file name or a duplicate service full name makes protoregistry panic
		Name:    proto.String("gwtest/" + slug + ".proto"),
		Package: proto.String("gwtest" + slug),
		Syntax:  proto.String("proto3"),
		Dependency: []string{
			(&gwproto.Paging{}).ProtoReflect().Descriptor().ParentFile().Path(),
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name: proto.String("AutoDiscoverService"),
			Method: []*descriptorpb.MethodDescriptorProto{{
				Name:       proto.String("Do"),
				InputType:  proto.String("." + input),
				OutputType: proto.String("." + input),
				Options:    &descriptorpb.MethodOptions{},
			}},
		}},
	}
	proto.SetExtension(fdp.Service[0].Method[0].Options, annotations.E_Http, &annotations.HttpRule{
		Pattern: &annotations.HttpRule_Post{Post: path},
	})

	fd, err := protodesc.NewFile(fdp, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("合成描述符失败 | building the descriptor failed: %v", err)
	}
	if err := protoregistry.GlobalFiles.RegisterFile(fd); err != nil {
		t.Fatalf("注册描述符失败 | registering it failed: %v", err)
	}
}

// 协议里写了 google.api.http，就不该再要求使用者手工登记一遍
//
// 这是这次改动的全部意义：那张表原先必须每个服务自己抄一遍，抄漏了不报错，
// 只会让那个接口静默地没有校验
//
// [EN] An annotation in the contract is enough; the user should not have to restate it. That is
// the whole point of the change: the table used to be copied by hand per service, and a missing
// line reported nothing while leaving that endpoint unvalidated
func TestRoutesAreDiscoveredFromHTTPAnnotations(t *testing.T) {
	const path = "/v1/gwtest/discovered"
	registerSyntheticRoute(t, path)

	// 让自动发现重跑一次：合成的描述符是在它跑过之后才注册的
	// [EN] Let discovery run again: the synthetic descriptor was registered after it first ran
	autoDiscoverOnce = sync.Once{}

	newMsg, found := lookupGatewayMessageType(http.MethodPost, path)
	if !found {
		t.Fatal("注解写了却没被发现：这条路由会静默跳过校验 | annotated yet undiscovered — "+
			"this route would silently skip validation", path)
	}
	if _, ok := newMsg().(*gwproto.Paging); !ok {
		t.Errorf("拿到的不是生成的 Go 类型 %T —— argus 校验的是 Go 结构体上的标签，"+
			"dynamicpb 没有那些标签，校验会全部通过 | not the generated Go type", newMsg())
	}
}

// 手工登记优先于自动发现
//
// 自动发现不该剥夺使用者换掉某个路由消息类型的能力：协议之外另挂的路由、
// 或者要用一个更严的类型去校验，都靠这条
//
// [EN] A manual registration wins. Discovery must not take away the ability to substitute a
// type for one route — for routes outside the contract, or to validate against a stricter type
func TestManualRegistrationWinsOverDiscovery(t *testing.T) {
	const path = "/v1/gwtest/manual-wins"
	sentinel := &gwproto.Sorting{}
	RegisterGatewayMessageType(http.MethodPost, path, func() any { return sentinel })

	registerSyntheticRoute(t, path)
	autoDiscoverOnce = sync.Once{}

	newMsg, found := lookupGatewayMessageType(http.MethodPost, path)
	if !found {
		t.Fatal("手工登记的路由反而查不到了 | the manually registered route disappeared")
	}
	if newMsg() != any(sentinel) {
		t.Errorf("自动发现覆盖了手工登记 | discovery overwrote the manual registration: %T", newMsg())
	}
}
