# 🚀 部署指南

## 📖 概述

本文档提供了 go-rpc-gateway 在不同环境下的部署指南，包括本地开发、Docker 容器化、Kubernetes 集群以及云平台部署等多种部署方式。

## 📋 部署准备

### 系统要求

| 组件 | 最低要求 | 推荐配置 |
|------|----------|----------|
| **CPU** | 1 核 | 2 核以上 |
| **内存** | 512 MB | 1 GB 以上 |
| **磁盘** | 1 GB | 5 GB 以上 |
| **操作系统** | Linux/Windows/macOS | Linux (推荐) |
| **Go 版本** | Go 1.23+ | Go 1.23+ |

### 依赖服务 (可选)

| 服务 | 用途 | 是否必需 |
|------|------|----------|
| **Redis** | 缓存/限流 | 否 |
| **MySQL** | 数据存储 | 否 |
| **MinIO** | 对象存储 | 否 |
| **RabbitMQ** | 消息队列 | 否 |
| **Jaeger** | 链路追踪 | 否 |
| **Prometheus** | 监控指标 | 否 |

## 🏠 本地开发部署

### 1. 源码编译部署

```bash
# 1. 克隆项目
git clone https://github.com/kamalyes/go-rpc-gateway.git
cd go-rpc-gateway

# 2. 安装依赖
go mod download

# 3. 构建应用
go build -o bin/gateway cmd/gateway/main.go

# 4. 创建配置文件
cp config/config.example.yaml config.yaml

# 5. 启动服务
./bin/gateway -config config.yaml
```

### 2. 开发模式启动

```bash
# 直接运行 Go 源码
go run cmd/gateway/main.go -config config-dev.yaml -log-level debug

# 或使用构建脚本
./build.sh && ./start.sh
```

### 3. 配置文件示例

```yaml
# config-dev.yaml
server:
  name: go-rpc-gateway-dev
  version: v1.0.0
  environment: development

gateway:
  name: go-rpc-gateway
  debug: true
  
  http:
    host: localhost
    port: 8080
    read_timeout: 30
    write_timeout: 30
    
  grpc:
    host: localhost
    port: 9090
    enable_reflection: true

middleware:
  rate_limit:
    enabled: true
    algorithm: token_bucket
    rate: 100
    burst: 10
    
  access_log:
    enabled: true
    format: json
    
# 开发环境可选组件
components:
  database:
    enabled: false
  redis:
    enabled: false
  minio:
    enabled: false
```

## 🐳 Docker 容器化部署

### 1. 构建 Docker 镜像

```dockerfile
# Dockerfile
FROM golang:1.23-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git ca-certificates

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.Version=$(cat VERSION) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o gateway cmd/gateway/main.go

# 运行时镜像
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 创建用户
RUN addgroup -g 1001 -S gateway && \
    adduser -u 1001 -S gateway -G gateway

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/gateway ./
COPY --from=builder /app/configs ./configs/
COPY --from=builder /app/locales ./locales/

# 修改权限
RUN chown -R gateway:gateway /app

# 切换用户
USER gateway

# 暴露端口
EXPOSE 8080 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# 启动命令
CMD ["./gateway", "-config", "configs/config.yaml"]
```

### 2. 构建和运行

```bash
# 构建镜像
docker build -t go-rpc-gateway:latest .

# 运行容器
docker run -d \
  --name go-rpc-gateway \
  -p 8080:8080 \
  -p 9090:9090 \
  -v $(pwd)/config:/app/config \
  -v $(pwd)/logs:/app/logs \
  go-rpc-gateway:latest
```

### 3. Docker Compose 部署

```yaml
# docker-compose.yml
version: '3.8'

services:
  gateway:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
      - "9090:9090"
    volumes:
      - ./configs:/app/configs:ro
      - ./logs:/app/logs
    environment:
      - GATEWAY_ENVIRONMENT=production
      - GATEWAY_DEBUG=false
    depends_on:
      - redis
      - mysql
      - jaeger
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes
    restart: unless-stopped

  mysql:
    image: mysql:8.0
    ports:
      - "3306:3306"
    environment:
      MYSQL_ROOT_PASSWORD: rootpassword
      MYSQL_DATABASE: gateway
      MYSQL_USER: gateway
      MYSQL_PASSWORD: gatewaypassword
    volumes:
      - mysql_data:/var/lib/mysql
      - ./scripts/mysql:/docker-entrypoint-initdb.d
    restart: unless-stopped

  jaeger:
    image: jaegertracing/all-in-one:1.50
    ports:
      - "16686:16686"
      - "14268:14268"
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    restart: unless-stopped

  prometheus:
    image: prom/prometheus:v2.45.0
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
    restart: unless-stopped

  grafana:
    image: grafana/grafana:10.1.0
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/grafana:/etc/grafana/provisioning
    restart: unless-stopped

volumes:
  redis_data:
  mysql_data:
  prometheus_data:
  grafana_data:

networks:
  default:
    name: gateway-network
```

### 4. 启动完整环境

```bash
# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f gateway

# 停止服务
docker-compose down
```

## ☸️ Kubernetes 部署

### 1. Namespace 和 ConfigMap

```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: go-rpc-gateway
  labels:
    name: go-rpc-gateway
    
---
# k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-config
  namespace: go-rpc-gateway
data:
  config.yaml: |
    server:
      name: go-rpc-gateway
      version: v1.0.0
      environment: production
    
    gateway:
      name: go-rpc-gateway
      debug: false
      
      http:
        host: 0.0.0.0
        port: 8080
        read_timeout: 30
        write_timeout: 30
        
      grpc:
        host: 0.0.0.0
        port: 9090
        enable_reflection: false
    
    middleware:
      rate_limit:
        enabled: true
        algorithm: token_bucket
        rate: 1000
        burst: 100
        
      access_log:
        enabled: true
        format: json
        
      signature:
        enabled: true
        algorithm: hmac-sha256
        secret_key: "production-secret-key-32-chars!"
    
    components:
      redis:
        enabled: true
        host: redis-service
        port: 6379
      database:
        enabled: true
        host: mysql-service
        port: 3306
        username: gateway
        password: gatewaypassword
        database: gateway
```

### 2. Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-rpc-gateway
  namespace: go-rpc-gateway
  labels:
    app: go-rpc-gateway
    version: v1.0.0
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: go-rpc-gateway
  template:
    metadata:
      labels:
        app: go-rpc-gateway
        version: v1.0.0
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: go-rpc-gateway
      containers:
      - name: gateway
        image: go-rpc-gateway:v1.0.0
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
          name: http
          protocol: TCP
        - containerPort: 9090
          name: grpc
          protocol: TCP
        env:
        - name: GATEWAY_ENVIRONMENT
          value: "production"
        - name: GATEWAY_DEBUG
          value: "false"
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true
        - name: logs
          mountPath: /app/logs
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 3
        securityContext:
          runAsNonRoot: true
          runAsUser: 1001
          runAsGroup: 1001
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
      volumes:
      - name: config
        configMap:
          name: gateway-config
      - name: logs
        emptyDir: {}
      terminationGracePeriodSeconds: 30
      restartPolicy: Always
```

### 3. Service 和 Ingress

```yaml
# k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: go-rpc-gateway-service
  namespace: go-rpc-gateway
  labels:
    app: go-rpc-gateway
    service: go-rpc-gateway
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 80
    targetPort: 8080
    protocol: TCP
  - name: grpc
    port: 9090
    targetPort: 9090
    protocol: TCP
  selector:
    app: go-rpc-gateway

---
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: go-rpc-gateway-ingress
  namespace: go-rpc-gateway
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/rewrite-target: /
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/backend-protocol: "HTTP"
    # gRPC 支持
    nginx.ingress.kubernetes.io/grpc-backend: "true"
    # 限流配置
    nginx.ingress.kubernetes.io/rate-limit-rps: "100"
    nginx.ingress.kubernetes.io/rate-limit-connections: "20"
spec:
  tls:
  - hosts:
    - api.example.com
    secretName: api-tls-secret
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: go-rpc-gateway-service
            port:
              number: 80
  - host: grpc.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: go-rpc-gateway-service
            port:
              number: 9090
```

### 4. HPA 和 PDB

```yaml
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: go-rpc-gateway-hpa
  namespace: go-rpc-gateway
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: go-rpc-gateway
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60

---
# k8s/pdb.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: go-rpc-gateway-pdb
  namespace: go-rpc-gateway
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: go-rpc-gateway
```

### 5. ServiceAccount 和 RBAC

```yaml
# k8s/rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: go-rpc-gateway
  namespace: go-rpc-gateway

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: go-rpc-gateway-role
  namespace: go-rpc-gateway
rules:
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: go-rpc-gateway-binding
  namespace: go-rpc-gateway
subjects:
- kind: ServiceAccount
  name: go-rpc-gateway
  namespace: go-rpc-gateway
roleRef:
  kind: Role
  name: go-rpc-gateway-role
  apiGroup: rbac.authorization.k8s.io
```

### 6. 部署脚本

```bash
#!/bin/bash
# scripts/k8s-deploy.sh

set -e

NAMESPACE="go-rpc-gateway"
VERSION=${1:-"v1.0.0"}

echo "🚀 Deploying go-rpc-gateway v$VERSION to Kubernetes..."

# 创建命名空间
echo "📦 Creating namespace..."
kubectl apply -f k8s/namespace.yaml

# 部署 ConfigMap
echo "⚙️ Deploying ConfigMap..."
kubectl apply -f k8s/configmap.yaml

# 部署 RBAC
echo "🔐 Deploying RBAC..."
kubectl apply -f k8s/rbac.yaml

# 部署应用
echo "🏗️ Deploying application..."
kubectl apply -f k8s/deployment.yaml

# 部署服务
echo "🌐 Deploying services..."
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml

# 部署自动伸缩
echo "📈 Deploying autoscaling..."
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/pdb.yaml

# 等待部署完成
echo "⏳ Waiting for deployment to be ready..."
kubectl wait --for=condition=available --timeout=300s \
  deployment/go-rpc-gateway -n $NAMESPACE

echo "✅ Deployment completed successfully!"

# 显示状态
echo "📊 Deployment status:"
kubectl get pods,svc,ingress -n $NAMESPACE
```

## ☁️ 云平台部署

### 1. AWS EKS 部署

```yaml
# aws/eks-cluster.yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: go-rpc-gateway-cluster
  region: us-west-2
  version: "1.28"

iam:
  withOIDC: true

vpc:
  cidr: "10.0.0.0/16"
  nat:
    gateway: Single

nodeGroups:
  - name: gateway-nodes
    instanceType: t3.medium
    minSize: 2
    maxSize: 6
    desiredCapacity: 3
    volumeSize: 50
    volumeType: gp3
    amiFamily: AmazonLinux2
    iam:
      attachPolicyARNs:
        - arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy
        - arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy
        - arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly
    tags:
      Environment: production
      Application: go-rpc-gateway

addons:
  - name: vpc-cni
  - name: coredns  
  - name: kube-proxy
  - name: aws-load-balancer-controller

cloudWatch:
  clusterLogging:
    enableTypes: ["*"]
```

### 2. 部署到 EKS

```bash
#!/bin/bash
# aws/deploy-eks.sh

# 创建 EKS 集群
eksctl create cluster -f aws/eks-cluster.yaml

# 配置 kubectl
aws eks update-kubeconfig --region us-west-2 --name go-rpc-gateway-cluster

# 安装 AWS Load Balancer Controller
kubectl apply -k "github.com/aws/eks-charts/stable/aws-load-balancer-controller/crds?ref=master"

helm repo add eks https://aws.github.io/eks-charts
helm repo update

helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=go-rpc-gateway-cluster \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller

# 部署应用
kubectl apply -f k8s/
```

### 3. Google Cloud GKE 部署

```bash
#!/bin/bash
# gcp/deploy-gke.sh

PROJECT_ID="your-project-id"
CLUSTER_NAME="go-rpc-gateway-cluster"
REGION="us-central1"

# 创建 GKE 集群
gcloud container clusters create $CLUSTER_NAME \
  --project=$PROJECT_ID \
  --region=$REGION \
  --machine-type=e2-standard-2 \
  --num-nodes=3 \
  --enable-autoscaling \
  --min-nodes=2 \
  --max-nodes=10 \
  --enable-autorepair \
  --enable-autoupgrade \
  --network=default \
  --subnetwork=default

# 获取集群凭证
gcloud container clusters get-credentials $CLUSTER_NAME \
  --region=$REGION \
  --project=$PROJECT_ID

# 部署应用
kubectl apply -f k8s/
```

## 🔧 配置优化

### 1. 生产环境配置

```yaml
# config/production.yaml
server:
  name: go-rpc-gateway
  version: v1.0.0
  environment: production

gateway:
  name: go-rpc-gateway
  debug: false
  
  http:
    host: 0.0.0.0
    port: 8080
    read_timeout: 60
    write_timeout: 60
    idle_timeout: 120
    max_header_bytes: 1048576  # 1MB
    enable_gzip_compress: true
    
  grpc:
    host: 0.0.0.0
    port: 9090
    max_recv_msg_size: 4194304  # 4MB
    max_send_msg_size: 4194304  # 4MB
    connection_timeout: 30
    keepalive_time: 30
    keepalive_timeout: 5
    enable_reflection: false

middleware:
  rate_limit:
    enabled: true
    algorithm: token_bucket
    rate: 1000
    burst: 100
    
  access_log:
    enabled: true
    format: json
    outputs:
      - type: file
        path: /var/log/gateway/access.log
        max_size: 100
        max_backups: 7
        max_age: 30
    
  signature:
    enabled: true
    algorithm: hmac-sha256
    secret_key: ${GATEWAY_SECRET_KEY}
    ttl: 300
    
  security:
    enabled: true
    headers:
      x_frame_options: "DENY"
      x_content_type_options: "nosniff"
      x_xss_protection: "1; mode=block"
      strict_transport_security: "max-age=31536000; includeSubDomains"
    
monitoring:
  metrics:
    enabled: true
    path: /metrics
    namespace: gateway
    
  tracing:
    enabled: true
    exporter:
      type: jaeger
      endpoint: http://jaeger:14268/api/traces
    sampler:
      type: probability
      probability: 0.1
      
  pprof:
    enabled: false  # 生产环境关闭
```

### 2. 性能调优

```yaml
# 性能优化配置
server:
  # Go runtime 配置
  go_max_procs: 0  # 使用所有可用 CPU
  
gateway:
  http:
    # 连接配置
    read_timeout: 30
    write_timeout: 30
    idle_timeout: 60
    max_header_bytes: 1048576
    
    # 性能配置
    enable_gzip_compress: true
    gzip_level: 6
    
  grpc:
    # 消息大小限制
    max_recv_msg_size: 4194304  # 4MB
    max_send_msg_size: 4194304  # 4MB
    
    # 连接保活
    keepalive_time: 30
    keepalive_timeout: 5
    keepalive_enforcement:
      min_time: 5
      permit_without_stream: true
    
    # 连接限制
    max_connection_idle: 300
    max_connection_age: 600
    max_connection_age_grace: 30

# 中间件性能配置
middleware:
  rate_limit:
    # 使用内存存储提高性能
    storage: memory
    cleanup_interval: 60
    
  logging:
    # 异步写入
    async: true
    buffer_size: 1000
    flush_interval: 5
```

## 📊 监控配置

### 1. Prometheus 配置

```yaml
# monitoring/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "gateway_rules.yml"

scrape_configs:
  - job_name: 'go-rpc-gateway'
    static_configs:
      - targets: ['go-rpc-gateway-service:8080']
    metrics_path: /metrics
    scrape_interval: 15s
    
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
        action: replace
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
        target_label: __address__

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

### 2. Grafana 仪表板

```json
{
  "dashboard": {
    "id": null,
    "title": "Go RPC Gateway Dashboard",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(gateway_http_requests_total[5m])",
            "legendFormat": "{{method}} {{status}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph", 
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(gateway_http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          },
          {
            "expr": "histogram_quantile(0.50, rate(gateway_http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "50th percentile"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "stat",
        "targets": [
          {
            "expr": "rate(gateway_http_requests_total{status=~\"5..\"}[5m]) / rate(gateway_http_requests_total[5m])",
            "legendFormat": "Error Rate"
          }
        ]
      }
    ]
  }
}
```

## 🛡️ 安全最佳实践

### 1. 容器安全

```dockerfile
# 多阶段构建，减小镜像大小
FROM golang:1.23-alpine AS builder
# ... 构建步骤

FROM scratch
# 只包含必要的文件
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/gateway /gateway
USER 65534:65534
ENTRYPOINT ["/gateway"]
```

### 2. Kubernetes 安全

```yaml
# 安全上下文
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  runAsGroup: 65534
  allowPrivilegeEscalation: false
  capabilities:
    drop:
    - ALL
  readOnlyRootFilesystem: true
  seccompProfile:
    type: RuntimeDefault

# 网络策略
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: go-rpc-gateway-netpol
spec:
  podSelector:
    matchLabels:
      app: go-rpc-gateway
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
```

## 📋 故障排查

### 1. 常见问题

| 问题 | 可能原因 | 解决方案 |
|------|----------|----------|
| 服务启动失败 | 配置文件错误 | 检查配置文件语法 |
| 健康检查失败 | 依赖服务不可用 | 检查 Redis/MySQL 连接 |
| 内存占用过高 | 内存泄漏 | 使用 pprof 分析内存使用 |
| 请求延迟高 | 数据库连接池不足 | 增加连接池大小 |
| 限流触发 | 请求频率过高 | 调整限流配置 |

### 2. 日志分析

```bash
# 查看应用日志
kubectl logs -f deployment/go-rpc-gateway -n go-rpc-gateway

# 查看错误日志
kubectl logs deployment/go-rpc-gateway -n go-rpc-gateway | grep ERROR

# 实时监控资源使用
kubectl top pods -n go-rpc-gateway

# 查看事件
kubectl get events -n go-rpc-gateway --sort-by='.lastTimestamp'
```

### 3. 性能分析

```bash
# CPU 分析
kubectl port-forward svc/go-rpc-gateway-service 8080:8080 -n go-rpc-gateway
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# 内存分析
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# 查看 Goroutine 数量
curl http://localhost:8080/debug/pprof/goroutine?debug=1
```

## 🔄 CI/CD 集成

### 1. GitHub Actions

```yaml
# .github/workflows/deploy.yml
name: Deploy to Kubernetes

on:
  push:
    branches: [main]
    tags: ['v*']

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    
    steps:
    - name: Checkout
      uses: actions/checkout@v4
      
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
        
    - name: Test
      run: go test -v ./...
      
    - name: Build
      run: go build -o gateway cmd/gateway/main.go
      
    - name: Login to Container Registry
      uses: docker/login-action@v3
      with:
        registry: ${{ env.REGISTRY }}
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}
        
    - name: Extract metadata
      id: meta
      uses: docker/metadata-action@v5
      with:
        images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
        
    - name: Build and push Docker image
      uses: docker/build-push-action@v5
      with:
        context: .
        push: true
        tags: ${{ steps.meta.outputs.tags }}
        labels: ${{ steps.meta.outputs.labels }}
        
    - name: Configure kubectl
      uses: azure/k8s-set-context@v3
      with:
        method: kubeconfig
        kubeconfig: ${{ secrets.KUBE_CONFIG }}
        
    - name: Deploy to Kubernetes
      run: |
        kubectl set image deployment/go-rpc-gateway \
          gateway=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ github.sha }} \
          -n go-rpc-gateway
        kubectl rollout status deployment/go-rpc-gateway -n go-rpc-gateway
```

这个部署指南涵盖了从本地开发到生产环境的完整部署流程。根据你的具体需求，可以选择适合的部署方式。建议在生产环境中使用 Kubernetes 部署，并配置完整的监控和告警系统。

---

更多部署相关问题，请查看 [故障排查文档](TROUBLESHOOTING.md) 或提交 [GitHub Issues](https://github.com/kamalyes/go-rpc-gateway/issues)。