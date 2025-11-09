/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:30:00
 * @FilePath: \go-rpc-gateway\examples\05-grpc\main.go
 * @Description: gRPC服务集成示例 - 展示如何集成gRPC服务到Gateway
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gateway "github.com/kamalyes/go-rpc-gateway"
	"google.golang.org/grpc"
)

// MockUserService 模拟的用户服务
type MockUserService struct{}

// MockProductService 模拟的产品服务  
type MockProductService struct{}

func main() {
	// 1. 创建Gateway实例
	gw, err := gateway.New()
	if err != nil {
		log.Fatalf("创建Gateway失败: %v", err)
	}

	// 2. 注册gRPC服务（模拟）
	registerGRPCServices(gw)

	// 3. 注册HTTP路由（模拟gRPC-Gateway生成的路由）
	registerHTTPRoutes(gw)

	log.Println("🚀 gRPC服务集成示例启动中...")
	log.Println("📡 服务端口:")
	log.Println("   - HTTP Gateway: http://localhost:8080")
	log.Println("   - gRPC Server:  localhost:9090")
	log.Println()
	log.Println("🔌 gRPC服务:")
	log.Println("   - UserService: 用户管理服务")
	log.Println("   - ProductService: 产品管理服务")
	log.Println()
	log.Println("🌐 HTTP API端点:")
	log.Println("   - GET  /api/v1/users")
	log.Println("   - POST /api/v1/users")
	log.Println("   - GET  /api/v1/users/{id}")
	log.Println("   - GET  /api/v1/products")
	log.Println("   - POST /api/v1/products")
	log.Println("   - GET  /api/v1/services/status")
	log.Println()
	log.Println("💡 测试命令:")
	log.Println("   curl http://localhost:8080/api/v1/users")
	log.Println(`   curl -X POST -H "Content-Type: application/json" -d '{"name":"Alice","email":"alice@example.com"}' http://localhost:8080/api/v1/users`)

	// 4. 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 5. 启动服务器
	go func() {
		if err := gw.Start(); err != nil {
			log.Printf("服务器启动失败: %v", err)
		}
	}()

	// 6. 等待关闭信号
	<-sigChan
	log.Println("🛑 正在优雅关闭服务器...")
	
	if err := gw.Stop(); err != nil {
		log.Printf("关闭服务器时出错: %v", err)
	}
	log.Println("✅ 服务器已成功关闭")
}

// registerGRPCServices 注册gRPC服务
func registerGRPCServices(gw *gateway.Gateway) {
	// 注册用户服务
	gw.RegisterService(func(s *grpc.Server) {
		// 这里通常会注册真正的gRPC服务
		// pb.RegisterUserServiceServer(s, &MockUserService{})
		log.Println("📝 已注册 UserService gRPC服务")
	})

	// 注册产品服务
	gw.RegisterService(func(s *grpc.Server) {
		// pb.RegisterProductServiceServer(s, &MockProductService{})
		log.Println("📝 已注册 ProductService gRPC服务")
	})

	log.Println("✅ 所有gRPC服务注册完成")
}

// registerHTTPRoutes 注册HTTP路由（模拟gRPC-Gateway生成的路由）
func registerHTTPRoutes(gw *gateway.Gateway) {
	// 用户服务路由
	registerUserRoutes(gw)
	
	// 产品服务路由
	registerProductRoutes(gw)
	
	// 服务状态路由
	registerStatusRoutes(gw)
}

// registerUserRoutes 注册用户服务路由
func registerUserRoutes(gw *gateway.Gateway) {
	// 获取用户列表
	gw.RegisterHTTPRoute("/api/v1/users", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			users := []map[string]interface{}{
				{"id": 1, "name": "Alice", "email": "alice@example.com", "created_at": "2024-01-01T00:00:00Z"},
				{"id": 2, "name": "Bob", "email": "bob@example.com", "created_at": "2024-01-02T00:00:00Z"},
				{"id": 3, "name": "Charlie", "email": "charlie@example.com", "created_at": "2024-01-03T00:00:00Z"},
			}
			
			response := map[string]interface{}{
				"success": true,
				"data":    users,
				"total":   len(users),
				"note":    "Data from gRPC UserService (mocked)",
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.Method == "POST" {
			// 创建用户
			var reqData map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqData)
			
			newUser := map[string]interface{}{
				"id":         len(mockUsers) + 1,
				"name":       reqData["name"],
				"email":      reqData["email"],
				"created_at": time.Now().Format(time.RFC3339),
			}
			
			response := map[string]interface{}{
				"success": true,
				"data":    newUser,
				"message": "User created successfully",
				"note":    "Created via gRPC UserService (mocked)",
			}
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
		}
	}))

	// 获取特定用户
	gw.RegisterHTTPRoute("/api/v1/users/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		
		// 简单解析ID（实际项目中会使用路由参数）
		userID := r.URL.Path[len("/api/v1/users/"):]
		
		user := map[string]interface{}{
			"id":         userID,
			"name":       "User " + userID,
			"email":      "user" + userID + "@example.com",
			"created_at": time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		}
		
		response := map[string]interface{}{
			"success": true,
			"data":    user,
			"note":    "User data from gRPC UserService (mocked)",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

// registerProductRoutes 注册产品服务路由
func registerProductRoutes(gw *gateway.Gateway) {
	// 获取产品列表
	gw.RegisterHTTPRoute("/api/v1/products", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			products := []map[string]interface{}{
				{"id": 1, "name": "iPhone 15", "price": 999.99, "category": "smartphones"},
				{"id": 2, "name": "MacBook Pro", "price": 1999.99, "category": "laptops"},
				{"id": 3, "name": "AirPods Pro", "price": 249.99, "category": "accessories"},
			}
			
			response := map[string]interface{}{
				"success": true,
				"data":    products,
				"total":   len(products),
				"note":    "Data from gRPC ProductService (mocked)",
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.Method == "POST" {
			// 创建产品
			var reqData map[string]interface{}
			json.NewDecoder(r.Body).Decode(&reqData)
			
			newProduct := map[string]interface{}{
				"id":       len(mockProducts) + 1,
				"name":     reqData["name"],
				"price":    reqData["price"],
				"category": reqData["category"],
			}
			
			response := map[string]interface{}{
				"success": true,
				"data":    newProduct,
				"message": "Product created successfully",
				"note":    "Created via gRPC ProductService (mocked)",
			}
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
		}
	}))
}

// registerStatusRoutes 注册状态路由
func registerStatusRoutes(gw *gateway.Gateway) {
	gw.RegisterHTTPRoute("/api/v1/services/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"gateway": map[string]interface{}{
				"status":    "running",
				"version":   "1.0.0",
				"framework": "go-rpc-gateway",
			},
			"grpc_services": map[string]interface{}{
				"user_service": map[string]interface{}{
					"status":      "healthy",
					"endpoints":   []string{"/api/v1/users", "/api/v1/users/{id}"},
					"description": "用户管理服务",
				},
				"product_service": map[string]interface{}{
					"status":      "healthy", 
					"endpoints":   []string{"/api/v1/products"},
					"description": "产品管理服务",
				},
			},
			"gateway_features": []string{
				"gRPC to HTTP translation",
				"Request/Response logging",
				"Rate limiting",
				"CORS support",
				"Security middleware",
				"Health checks",
				"Metrics collection",
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}))
}

// 模拟数据
var mockUsers = []map[string]interface{}{
	{"id": 1, "name": "Alice", "email": "alice@example.com"},
	{"id": 2, "name": "Bob", "email": "bob@example.com"},
}

var mockProducts = []map[string]interface{}{
	{"id": 1, "name": "iPhone", "price": 999.99},
	{"id": 2, "name": "MacBook", "price": 1999.99},
}