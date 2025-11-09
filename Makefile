# Makefile for Go RPC Gateway
# 完整的构建、测试和部署自动化脚本

# 项目信息
PROJECT_NAME := go-rpc-gateway
MODULE_NAME := github.com/yourusername/go-rpc-gateway
VERSION := $(shell git describe --tags --always --dirty=-dev 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Go相关配置
GO_VERSION := 1.21
GO_OS := $(shell go env GOOS)
GO_ARCH := $(shell go env GOARCH)
GO_PROXY := https://goproxy.cn,direct
GOOS := linux
GOARCH := amd64

# Docker配置
DOCKER_REGISTRY := ghcr.io
DOCKER_REPO := $(DOCKER_REGISTRY)/yourusername
DOCKER_IMAGE := $(DOCKER_REPO)/$(PROJECT_NAME)
DOCKER_TAG := $(VERSION)

# Kubernetes配置
K8S_NAMESPACE := gateway
K8S_CONTEXT := $(shell kubectl config current-context)

# 目录配置
BIN_DIR := bin
BUILD_DIR := build
DIST_DIR := dist
DOCS_DIR := docs
EXAMPLES_DIR := examples
SCRIPTS_DIR := scripts

# 编译参数
CGO_ENABLED := 0
LDFLAGS := -s -w \
	-X '$(MODULE_NAME)/internal/constants.Version=$(VERSION)' \
	-X '$(MODULE_NAME)/internal/constants.BuildTime=$(BUILD_TIME)' \
	-X '$(MODULE_NAME)/internal/constants.GitCommit=$(GIT_COMMIT)' \
	-X '$(MODULE_NAME)/internal/constants.GitBranch=$(GIT_BRANCH)'

# 测试配置
TEST_TIMEOUT := 300s
COVERAGE_THRESHOLD := 80

.PHONY: help
help: ## 显示帮助信息
	@echo "Go RPC Gateway - $(PROJECT_NAME) v$(VERSION)"
	@echo "======================================================"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

## 🏗️ 构建相关
.PHONY: deps
deps: ## 安装依赖
	@echo "📦 Installing dependencies..."
	go mod download
	go mod verify
	go mod tidy

.PHONY: build
build: deps ## 构建主程序
	@echo "🔨 Building $(PROJECT_NAME)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/gateway ./cmd/gateway

.PHONY: build-all
build-all: deps ## 构建所有程序
	@echo "🔨 Building all binaries..."
	@mkdir -p $(BIN_DIR)
	@for cmd in gateway simple-gateway test-adapter; do \
		echo "Building $$cmd..."; \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$$cmd ./cmd/$$cmd; \
	done

.PHONY: build-cross
build-cross: deps ## 交叉编译
	@echo "🌐 Cross-compiling for multiple platforms..."
	@mkdir -p $(DIST_DIR)
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ "$$os" = "windows" ] && [ "$$arch" = "arm64" ]; then continue; fi; \
			echo "Building for $$os/$$arch..."; \
			ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -ldflags="$(LDFLAGS)" \
			-o $(DIST_DIR)/$(PROJECT_NAME)-$$os-$$arch$$ext ./cmd/gateway; \
		done \
	done

.PHONY: install
install: build ## 安装到GOPATH/bin
	@echo "📦 Installing $(PROJECT_NAME)..."
	go install -ldflags="$(LDFLAGS)" ./cmd/gateway

## 🧪 测试相关
.PHONY: test
test: ## 运行单元测试
	@echo "🧪 Running unit tests..."
	go test -v -race -timeout=$(TEST_TIMEOUT) ./...

.PHONY: test-coverage
test-coverage: ## 运行测试并生成覆盖率报告
	@echo "📊 Running tests with coverage..."
	@mkdir -p $(BUILD_DIR)
	go test -v -race -timeout=$(TEST_TIMEOUT) -coverprofile=$(BUILD_DIR)/coverage.out ./...
	go tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	go tool cover -func=$(BUILD_DIR)/coverage.out | tail -n 1

.PHONY: test-integration
test-integration: ## 运行集成测试
	@echo "🔗 Running integration tests..."
	go test -v -tags=integration -timeout=$(TEST_TIMEOUT) ./tests/integration/...

.PHONY: test-performance
test-performance: ## 运行性能测试
	@echo "⚡ Running performance tests..."
	go test -v -bench=. -benchmem -timeout=$(TEST_TIMEOUT) ./tests/performance/...

.PHONY: test-all
test-all: test test-integration test-performance ## 运行所有测试

## 📋 代码质量
.PHONY: lint
lint: ## 代码检查
	@echo "🔍 Running linters..."
	@which golangci-lint >/dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run --timeout=5m

.PHONY: fmt
fmt: ## 格式化代码
	@echo "🎨 Formatting code..."
	go fmt ./...
	goimports -w .

.PHONY: vet
vet: ## 代码静态分析
	@echo "🔬 Running go vet..."
	go vet ./...

.PHONY: sec
sec: ## 安全扫描
	@echo "🔒 Running security scan..."
	@which gosec >/dev/null || (echo "Installing gosec..." && go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest)
	gosec -fmt json -out $(BUILD_DIR)/security-report.json ./...

.PHONY: quality
quality: fmt vet lint sec test-coverage ## 完整代码质量检查

## 🐳 Docker相关
.PHONY: docker-build
docker-build: ## 构建Docker镜像
	@echo "🐳 Building Docker image..."
	docker build -f $(EXAMPLES_DIR)/docker/Dockerfile -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

.PHONY: docker-push
docker-push: docker-build ## 推送Docker镜像
	@echo "📤 Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

.PHONY: docker-run
docker-run: docker-build ## 运行Docker容器
	@echo "🚀 Running Docker container..."
	docker run --rm -p 8080:8080 -p 9090:9090 $(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-compose-up
docker-compose-up: ## 启动Docker Compose环境
	@echo "🐳 Starting Docker Compose environment..."
	cd $(EXAMPLES_DIR)/docker && docker-compose up -d

.PHONY: docker-compose-down
docker-compose-down: ## 停止Docker Compose环境
	@echo "🛑 Stopping Docker Compose environment..."
	cd $(EXAMPLES_DIR)/docker && docker-compose down -v

.PHONY: docker-logs
docker-logs: ## 查看Docker容器日志
	cd $(EXAMPLES_DIR)/docker && docker-compose logs -f gateway

## ☸️ Kubernetes相关
.PHONY: k8s-deploy
k8s-deploy: ## 部署到Kubernetes
	@echo "☸️  Deploying to Kubernetes ($(K8S_CONTEXT))..."
	kubectl apply -f $(EXAMPLES_DIR)/k8s/

.PHONY: k8s-delete
k8s-delete: ## 从Kubernetes删除
	@echo "🗑️  Deleting from Kubernetes..."
	kubectl delete -f $(EXAMPLES_DIR)/k8s/ --ignore-not-found

.PHONY: k8s-status
k8s-status: ## 查看Kubernetes状态
	@echo "📊 Kubernetes Status:"
	kubectl get pods,svc,ingress -n $(K8S_NAMESPACE)

.PHONY: k8s-logs
k8s-logs: ## 查看Kubernetes日志
	kubectl logs -f deployment/gateway-deployment -n $(K8S_NAMESPACE)

.PHONY: k8s-port-forward
k8s-port-forward: ## Kubernetes端口转发
	@echo "🔗 Port forwarding 8080:8080..."
	kubectl port-forward service/gateway-service 8080:80 -n $(K8S_NAMESPACE)

## 🚀 部署相关
.PHONY: deploy-local
deploy-local: build ## 本地部署
	@echo "🏠 Deploying locally..."
	./$(BIN_DIR)/gateway

.PHONY: deploy-staging
deploy-staging: docker-push ## 部署到staging环境
	@echo "🎭 Deploying to staging..."
	@echo "Update staging deployment with image: $(DOCKER_IMAGE):$(DOCKER_TAG)"

.PHONY: deploy-prod
deploy-prod: docker-push ## 部署到生产环境
	@echo "🏭 Deploying to production..."
	@echo "Update production deployment with image: $(DOCKER_IMAGE):$(DOCKER_TAG)"

## 📚 文档相关
.PHONY: docs
docs: ## 生成文档
	@echo "📚 Generating documentation..."
	@mkdir -p $(DOCS_DIR)/api
	@which godoc >/dev/null || go install golang.org/x/tools/cmd/godoc@latest
	godoc -http=:6060 &
	@echo "Documentation available at http://localhost:6060"

.PHONY: docs-swagger
docs-swagger: ## 生成Swagger文档
	@echo "📝 Generating Swagger documentation..."
	@which swag >/dev/null || go install github.com/swaggo/swag/cmd/swag@latest
	swag init -g gateway.go -o $(DOCS_DIR)/swagger

## 🔧 开发工具
.PHONY: dev-setup
dev-setup: ## 安装开发工具
	@echo "🛠️  Setting up development environment..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install golang.org/x/tools/cmd/godoc@latest

.PHONY: dev-run
dev-run: build ## 开发模式运行
	@echo "🔄 Running in development mode..."
	./$(BIN_DIR)/gateway -config ./examples/configs/development.yaml

.PHONY: dev-watch
dev-watch: ## 文件变化时自动重新构建
	@echo "👀 Watching for file changes..."
	@which air >/dev/null || go install github.com/cosmtrek/air@latest
	air

## 🧹 清理相关
.PHONY: clean
clean: ## 清理构建文件
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BIN_DIR) $(BUILD_DIR) $(DIST_DIR)
	go clean -cache -modcache -testcache

.PHONY: clean-docker
clean-docker: ## 清理Docker资源
	@echo "🐳 Cleaning Docker resources..."
	docker system prune -f
	docker volume prune -f

## 🎯 快捷命令
.PHONY: all
all: quality build-all test-all ## 完整构建流程

.PHONY: quick
quick: build test ## 快速构建和测试

.PHONY: release
release: quality build-cross docker-push ## 发布流程

.PHONY: ci
ci: quality test-all build ## CI流程

## 📋 项目信息
.PHONY: version
version: ## 显示版本信息
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Git Branch: $(GIT_BRANCH)"
	@echo "Go Version: $(shell go version)"
	@echo "OS/Arch: $(GO_OS)/$(GO_ARCH)"

.PHONY: info
info: version ## 显示项目信息
	@echo ""
	@echo "📁 Directories:"
	@echo "  BIN_DIR: $(BIN_DIR)"
	@echo "  BUILD_DIR: $(BUILD_DIR)"
	@echo "  DIST_DIR: $(DIST_DIR)"
	@echo ""
	@echo "🐳 Docker:"
	@echo "  Registry: $(DOCKER_REGISTRY)"
	@echo "  Image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@echo ""
	@echo "☸️  Kubernetes:"
	@echo "  Namespace: $(K8S_NAMESPACE)"
	@echo "  Context: $(K8S_CONTEXT)"

# 默认目标
.DEFAULT_GOAL := help