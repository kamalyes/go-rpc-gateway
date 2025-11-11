/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 00:00:00
 * @FilePath: \go-rpc-gateway\examples\02-ecommerce-service\main.go
 * @Description: 电商微服务示例 - 使用现有的访问控制服务API
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"context"
	"fmt"
	"time"

	gateway "github.com/kamalyes/go-rpc-gateway"
	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/config"
	pb "github.com/kamalyes/go-rpc-gateway/examples/02-ecommerce-service/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	ServiceName    = "ecommerce-service"
	ServiceVersion = "1.0.0"
)

// EcommerceService 电商服务实现 - 基于访问控制服务的API结构
type EcommerceService struct {
	pb.UnimplementedAccessControlServiceServer
	healthServer *health.Server
}

// HealthService 健康检查服务
type HealthService struct {
	healthpb.UnimplementedHealthServer
}

func main() {
	// 初始化配置
	cfg := &config.GatewayConfig{
		Name:     ServiceName,
		Version:  ServiceVersion,
		HTTPPort: 8080,
		GRPCPort: 9090,
	}

	// 创建网关
	gw, err := gateway.New(cfg)
	if err != nil {
		panic(fmt.Sprintf("创建网关失败: %v", err))
	}

	// 创建服务实例
	ecommerceSvc := &EcommerceService{
		healthServer: health.NewServer(),
	}
	healthSvc := &HealthService{}

	// 注册 gRPC 服务
	gw.RegisterService(func(s *grpc.Server) {
		// 注册电商服务（使用访问控制服务的API结构）
		pb.RegisterAccessControlServiceServer(s, ecommerceSvc)
		
		// 注册健康检查服务
		healthpb.RegisterHealthServer(s, healthSvc)
		
		// 设置服务健康状态
		ecommerceSvc.healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
		ecommerceSvc.healthServer.SetServingStatus(ServiceName, healthpb.HealthCheckResponse_SERVING)
	})

	// 注册 HTTP 路由 - 使用访问控制API的路由结构作为示例
	gw.RegisterHTTPRoute("/api/v1/products", "GET", func(ctx *gateway.Context) {
		// 模拟商品列表
		products := []map[string]interface{}{
			{
				"id":          1,
				"name":        "iPhone 15 Pro",
				"description": "最新款苹果手机",
				"price":       9999.00,
				"stock":       100,
				"category":    "手机",
			},
			{
				"id":          2,
				"name":        "MacBook Pro 16\"",
				"description": "高性能笔记本电脑",
				"price":       25999.00,
				"stock":       50,
				"category":    "电脑",
			},
		}

		ctx.JSON(200, map[string]interface{}{
			"code":    0,
			"message": "success",
			"data": map[string]interface{}{
				"products": products,
				"total":    len(products),
			},
		})
	})

	gw.RegisterHTTPRoute("/api/v1/orders", "POST", func(ctx *gateway.Context) {
		// 模拟订单创建
		order := map[string]interface{}{
			"id":           12345,
			"order_no":     fmt.Sprintf("ORDER%d", time.Now().Unix()),
			"user_id":      ctx.GetString("user_id"),
			"status":       "pending",
			"total_amount": 9999.00,
			"created_at":   time.Now().Format(time.RFC3339),
		}

		ctx.JSON(200, map[string]interface{}{
			"code":    0,
			"message": "订单创建成功",
			"data":    order,
		})
	})

	// 启用功能特性
	gw.EnablePProf()      // 性能分析
	gw.EnableMonitoring() // 监控指标
	gw.EnableTracing()    // 链路追踪
	gw.EnableHealth()     // 健康检查

	// 启动服务
	global.LOGGER.InfoMsg("🚀 启动电商微服务...")
	global.LOGGER.InfoKV("服务信息",
		"name", ServiceName,
		"version", ServiceVersion,
		"http_port", 8080,
		"grpc_port", 9090,
	)

	global.LOGGER.InfoMsg("📋 已注册的服务:")
	global.LOGGER.InfoMsg("  - AccessControlService (基于现有API)")
	global.LOGGER.InfoMsg("  - HealthService (健康检查)")
	
	global.LOGGER.InfoMsg("🔗 HTTP API路由:")
	global.LOGGER.InfoMsg("  - GET  /api/v1/products (获取商品列表)")
	global.LOGGER.InfoMsg("  - POST /api/v1/orders (创建订单)")

	if err := gw.Start(); err != nil {
		panic(fmt.Sprintf("启动失败: %v", err))
	}
}

// HealthService 实现

func (h *HealthService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	global.LOGGER.InfoKV("健康检查", "service", req.Service)
	
	return &healthpb.HealthCheckResponse{
		Status: healthpb.HealthCheckResponse_SERVING,
	}, nil
}

func (h *HealthService) Watch(req *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	global.LOGGER.InfoKV("监听健康状态", "service", req.Service)
	
	// 定期发送健康状态
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			if err := stream.Send(&healthpb.HealthCheckResponse{
				Status: healthpb.HealthCheckResponse_SERVING,
			}); err != nil {
				return err
			}
		}
	}
}

// EcommerceService 实现 - 基于访问控制服务API的示例实现

func (s *EcommerceService) UserInfo(ctx context.Context, req *pb.UserInfoRequest) (*pb.UserInfoResponse, error) {
	global.LOGGER.InfoKV("获取用户信息", "request", req)

	// 模拟用户信息
	user := &pb.User{
		UserId:      1001,
		Username:    "demo_user",
		Nickname:    "演示用户",
		Email:       "demo@example.com",
		Phone:       "13800138000",
		Status:      pb.Status_STAT_NORMAL,
		CreatedTime: timestamppb.New(time.Now().Add(-30*24*time.Hour)), // 30天前创建
		UpdatedTime: timestamppb.New(time.Now()),
	}

	return &pb.UserInfoResponse{
		Code:    0,
		Message: "获取用户信息成功",
		Data:    user,
	}, nil
}

func (s *EcommerceService) UserList(ctx context.Context, req *pb.UserListRequest) (*pb.UserListResponse, error) {
	global.LOGGER.InfoKV("获取用户列表", 
		"page", req.Page,
		"page_size", req.PageSize,
	)

	// 模拟用户列表数据
	users := []*pb.User{
		{
			UserId:      1001,
			Username:    "user1",
			Nickname:    "用户1",
			Email:       "user1@example.com",
			Phone:       "13800138001",
			Status:      pb.Status_STAT_NORMAL,
			CreatedTime: timestamppb.New(time.Now().Add(-30*24*time.Hour)),
			UpdatedTime: timestamppb.New(time.Now()),
		},
		{
			UserId:      1002,
			Username:    "user2",
			Nickname:    "用户2",
			Email:       "user2@example.com", 
			Phone:       "13800138002",
			Status:      pb.Status_STAT_NORMAL,
			CreatedTime: timestamppb.New(time.Now().Add(-25*24*time.Hour)),
			UpdatedTime: timestamppb.New(time.Now()),
		},
	}

	// 计算分页
	total := int64(len(users))
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	return &pb.UserListResponse{
		Code:    0,
		Message: "获取用户列表成功",
		Data: &pb.UserListData{
			List:     users,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	}, nil
}

func (s *EcommerceService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	global.LOGGER.InfoKV("用户登录", 
		"username", req.Username,
		"login_type", req.LoginType,
	)

	// 模拟登录验证
	if req.Username == "" || req.Password == "" {
		return &pb.LoginResponse{
			Code:    400,
			Message: "用户名和密码不能为空",
		}, nil
	}

	// 模拟成功登录
	loginData := &pb.LoginData{
		Token: fmt.Sprintf("demo_token_%d", time.Now().Unix()),
		User: &pb.User{
			UserId:      1001,
			Username:    req.Username,
			Nickname:    "演示用户",
			Email:       "demo@example.com",
			Phone:       "13800138000",
			Status:      pb.Status_STAT_NORMAL,
			CreatedTime: timestamppb.New(time.Now().Add(-30*24*time.Hour)),
			UpdatedTime: timestamppb.New(time.Now()),
		},
	}

	return &pb.LoginResponse{
		Code:    0,
		Message: "登录成功",
		Data:    loginData,
	}, nil
}

func (s *EcommerceService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	global.LOGGER.InfoKV("用户登出", "token", req.Token)

	return &pb.LogoutResponse{
		Code:    0,
		Message: "登出成功",
	}, nil
}

func (s *EcommerceService) Endpoints(ctx context.Context, req *pb.EndpointsRequest) (*pb.EndpointsResponse, error) {
	global.LOGGER.InfoKV("获取端点信息", "request", req)

	// 模拟API端点信息
	endpoints := []*structpb.Struct{
		{
			Fields: map[string]*structpb.Value{
				"path":        structpb.NewStringValue("/api/v1/products"),
				"method":      structpb.NewStringValue("GET"),
				"description": structpb.NewStringValue("获取商品列表"),
			},
		},
		{
			Fields: map[string]*structpb.Value{
				"path":        structpb.NewStringValue("/api/v1/orders"),
				"method":      structpb.NewStringValue("POST"),
				"description": structpb.NewStringValue("创建订单"),
			},
		},
	}

	return &pb.EndpointsResponse{
		Code:    0,
		Message: "获取端点信息成功",
		Data:    endpoints,
	}, nil
}

// 其他必需的方法实现（返回未实现错误或空实现）

func (s *EcommerceService) DictNew(ctx context.Context, req *pb.DictNewRequest) (*pb.DictNewResponse, error) {
	return &pb.DictNewResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) DictList(ctx context.Context, req *pb.DictListRequest) (*pb.DictListResponse, error) {
	return &pb.DictListResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) DictGet(ctx context.Context, req *pb.DictGetRequest) (*pb.DictGetResponse, error) {
	return &pb.DictGetResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) DictUpdate(ctx context.Context, req *pb.DictUpdateRequest) (*pb.DictUpdateResponse, error) {
	return &pb.DictUpdateResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) DictDelete(ctx context.Context, req *pb.DictDeleteRequest) (*pb.DictDeleteResponse, error) {
	return &pb.DictDeleteResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) OplogNew(ctx context.Context, req *pb.OplogNewRequest) (*pb.OplogNewResponse, error) {
	return &pb.OplogNewResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) OplogList(ctx context.Context, req *pb.OplogListRequest) (*pb.OplogListResponse, error) {
	return &pb.OplogListResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) OplogGet(ctx context.Context, req *pb.OplogGetRequest) (*pb.OplogGetResponse, error) {
	return &pb.OplogGetResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) OplogDelete(ctx context.Context, req *pb.OplogDeleteRequest) (*pb.OplogDeleteResponse, error) {
	return &pb.OplogDeleteResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) BucketList(ctx context.Context, req *pb.BucketListRequest) (*pb.BucketListResponse, error) {
	return &pb.BucketListResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) BucketNew(ctx context.Context, req *pb.BucketNewRequest) (*pb.BucketNewResponse, error) {
	return &pb.BucketNewResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) BucketDelete(ctx context.Context, req *pb.BucketDeleteRequest) (*pb.BucketDeleteResponse, error) {
	return &pb.BucketDeleteResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) ObjectList(ctx context.Context, req *pb.ObjectListRequest) (*pb.ObjectListResponse, error) {
	return &pb.ObjectListResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) ObjectGet(ctx context.Context, req *pb.ObjectGetRequest) (*pb.ObjectGetResponse, error) {
	return &pb.ObjectGetResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) ObjectPut(ctx context.Context, req *pb.ObjectPutRequest) (*pb.ObjectPutResponse, error) {
	return &pb.ObjectPutResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) ObjectDelete(ctx context.Context, req *pb.ObjectDeleteRequest) (*pb.ObjectDeleteResponse, error) {
	return &pb.ObjectDeleteResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) ObjectPresignedDownloadUrl(ctx context.Context, req *pb.ObjectPresignedDownloadUrlRequest) (*pb.ObjectPresignedDownloadUrlResponse, error) {
	return &pb.ObjectPresignedDownloadUrlResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) ObjectPresignedUploadUrl(ctx context.Context, req *pb.ObjectPresignedUploadUrlRequest) (*pb.ObjectPresignedUploadUrlResponse, error) {
	return &pb.ObjectPresignedUploadUrlResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) Install(ctx context.Context, req *pb.InstallRequest) (*pb.InstallResponse, error) {
	return &pb.InstallResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) SendCode(ctx context.Context, req *pb.SendCodeRequest) (*pb.SendCodeResponse, error) {
	return &pb.SendCodeResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) RegisterCheck(ctx context.Context, req *pb.RegisterCheckRequest) (*pb.RegisterCheckResponse, error) {
	return &pb.RegisterCheckResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return &pb.RegisterResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	return &pb.RefreshTokenResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) Authorities(ctx context.Context, req *pb.AuthoritiesRequest) (*pb.AuthoritiesResponse, error) {
	return &pb.AuthoritiesResponse{Code: 501, Message: "Not implemented"}, nil
}

func (s *EcommerceService) UserMenusTree(ctx context.Context, req *pb.UserMenusTreeRequest) (*pb.UserMenusTreeResponse, error) {
	return &pb.UserMenusTreeResponse{Code: 501, Message: "Not implemented"}, nil
}