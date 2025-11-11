/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 00:00:00
 * @FilePath: \go-rpc-gateway\examples\01-simple-api\main.go
 * @Description: 简单API服务示例 - 展示基础HTTP API开发
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
	gateway "github.com/kamalyes/go-rpc-gateway"
)

// User 用户模型
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// APIResponse 统一响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 模拟数据存储
var users = []User{
	{ID: 1, Name: "Alice", Email: "alice@example.com", CreatedAt: time.Now().Add(-24 * time.Hour)},
	{ID: 2, Name: "Bob", Email: "bob@example.com", CreatedAt: time.Now().Add(-12 * time.Hour)},
	{ID: 3, Name: "Charlie", Email: "charlie@example.com", CreatedAt: time.Now().Add(-6 * time.Hour)},
}
var nextID = 4

func main() {
	// 创建网关实例
	gw, err := gateway.New()
	if err != nil {
		panic(err)
	}

	// 注册 API 路由
	gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
		"/api/users":      usersHandler,      // GET: 获取用户列表, POST: 创建用户
		"/api/users/{id}": userByIDHandler,   // GET: 获取单个用户, PUT: 更新用户, DELETE: 删除用户
		"/api/health":     healthHandler,     // GET: 健康检查
		"/api/stats":      statsHandler,      // GET: 统计信息
	})

	// 启用功能特性
	gw.EnablePProf()      // 性能分析
	gw.EnableMonitoring() // 监控指标
	gw.EnableHealth()     // 健康检查

	// 启动服务
	global.LOGGER.InfoMsg("🚀 启动简单API服务...")
	global.LOGGER.InfoKV("服务信息",
		"name", "simple-api-service",
		"version", "1.0.0",
		"http_port", 8080,
	)

	if err := gw.Start(); err != nil {
		panic(err)
	}
}

// usersHandler 处理用户列表相关请求
func usersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getUsersList(w, r)
	case http.MethodPost:
		createUser(w, r)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// userByIDHandler 处理单个用户相关请求
func userByIDHandler(w http.ResponseWriter, r *http.Request) {
	// 简单的ID提取（生产环境推荐使用路由库）
	idStr := r.URL.Path[len("/api/users/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "无效的用户ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		getUserByID(w, r, id)
	case http.MethodPut:
		updateUser(w, r, id)
	case http.MethodDelete:
		deleteUser(w, r, id)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// getUsersList 获取用户列表
func getUsersList(w http.ResponseWriter, r *http.Request) {
	global.LOGGER.InfoMsg("获取用户列表")

	writeSuccessResponse(w, map[string]interface{}{
		"users": users,
		"total": len(users),
	})
}

// createUser 创建新用户
func createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 验证必填字段
	if req.Name == "" || req.Email == "" {
		writeErrorResponse(w, http.StatusBadRequest, "姓名和邮箱不能为空")
		return
	}

	// 创建新用户
	user := User{
		ID:        nextID,
		Name:      req.Name,
		Email:     req.Email,
		CreatedAt: time.Now(),
	}
	nextID++
	users = append(users, user)

	global.LOGGER.InfoKV("创建用户成功",
		"user_id", user.ID,
		"name", user.Name,
		"email", user.Email,
	)

	writeSuccessResponse(w, user)
}

// getUserByID 根据ID获取用户
func getUserByID(w http.ResponseWriter, r *http.Request, id int) {
	for _, user := range users {
		if user.ID == id {
			writeSuccessResponse(w, user)
			return
		}
	}

	writeErrorResponse(w, http.StatusNotFound, "用户不存在")
}

// updateUser 更新用户信息
func updateUser(w http.ResponseWriter, r *http.Request, id int) {
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 查找并更新用户
	for i, user := range users {
		if user.ID == id {
			if req.Name != "" {
				users[i].Name = req.Name
			}
			if req.Email != "" {
				users[i].Email = req.Email
			}

			global.LOGGER.InfoKV("更新用户成功",
				"user_id", id,
				"name", users[i].Name,
				"email", users[i].Email,
			)

			writeSuccessResponse(w, users[i])
			return
		}
	}

	writeErrorResponse(w, http.StatusNotFound, "用户不存在")
}

// deleteUser 删除用户
func deleteUser(w http.ResponseWriter, r *http.Request, id int) {
	for i, user := range users {
		if user.ID == id {
			// 删除用户
			users = append(users[:i], users[i+1:]...)

			global.LOGGER.InfoKV("删除用户成功",
				"user_id", id,
				"name", user.Name,
			)

			writeSuccessResponse(w, map[string]interface{}{
				"message": "用户删除成功",
				"deleted_user": user,
			})
			return
		}
	}

	writeErrorResponse(w, http.StatusNotFound, "用户不存在")
}

// healthHandler 健康检查处理器
func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, map[string]interface{}{
		"status":    "ok",
		"service":   "simple-api-service",
		"version":   "1.0.0",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(time.Now().Add(-time.Hour)).String(), // 模拟运行时间
	})
}

// statsHandler 统计信息处理器
func statsHandler(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"total_users":  len(users),
		"service_name": "simple-api-service",
		"endpoints": []string{
			"GET /api/users",
			"POST /api/users",
			"GET /api/users/{id}",
			"PUT /api/users/{id}",
			"DELETE /api/users/{id}",
			"GET /api/health",
			"GET /api/stats",
		},
		"features": []string{
			"PProf性能分析",
			"Prometheus监控",
			"健康检查",
			"结构化日志",
		},
	}

	writeSuccessResponse(w, stats)
}

// 辅助函数

// writeSuccessResponse 写成功响应
func writeSuccessResponse(w http.ResponseWriter, data interface{}) {
	response := APIResponse{
		Code:    200,
		Message: "success",
		Data:    data,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

// writeErrorResponse 写错误响应
func writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := APIResponse{
		Code:    statusCode,
		Message: message,
	}
	writeJSONResponse(w, statusCode, response)
}

// writeJSONResponse 写JSON响应
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		global.LOGGER.WithError(err).ErrorMsg("写入JSON响应失败")
	}
}