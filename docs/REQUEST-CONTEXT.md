# 请求上下文

## 概述

`RequestCommonMeta` 实现 HTTP → gRPC → Service → Repository 全链路上下文传递，确保 trace\_id、tenant\_id、user\_id 等 30+ 个关键字段在整个调用链中不丢失。

> 源码：[middleware/request\_context.go](../middleware/request_context.go)

## 全链路传递流程

```mermaid
flowchart TD
    CLIENT["客户端, 发送 HTTP 请求, 携带 X-* Headers"]

    subgraph HTTP_LAYER["HTTP 层"]
        MW["RequestContextMiddleware, HTTP 中间件"]
        EXTRACT["提取 HTTP Headers, → 构建 RequestCommonMeta"]
        AUTO_GEN["自动生成 TraceID / RequestID, 如果不存在"]
        INJECT_CTX["注入 context.Context, WithRequestCommonMeta(ctx, meta), 单次缓存完整元信息"]
        SET_RESP["设置响应头, X-Trace-ID / X-Request-ID / X-Server-Node"]
    end

    subgraph GW_LAYER["gRPC-Gateway 层"]
        GW_MUX["runtime.ServeMux, WithIncomingHeaderMatcher, 传递所有 HTTP Header → gRPC Metadata"]
    end

    subgraph GRPC_SERVER["gRPC Server 层"]
        UNARY["UnaryServerRequestContextInterceptor, 一元调用拦截器"]
        STREAM["StreamServerRequestContextInterceptor, 流式调用拦截器"]
        ENRICH["enrichContextFromMetadata(), 从 gRPC Metadata 提取, → 注入 context.Context"]
        SET_MD["setResponseMetadata(), 设置 gRPC 响应 Metadata"]
    end

    subgraph SERVICE["Service 层"]
        GET_CTX["GetXxx(ctx), 读取上下文字段"]
        SCOPE["ContextScopeReader, 作用域适配器"]
    end

    subgraph GRPC_CLIENT["gRPC Client 层（跨服务调用）"]
        CLIENT_UNARY["UnaryClientRequestContextInterceptor, 客户端一元调用拦截器"]
        CLIENT_STREAM["StreamClientRequestContextInterceptor, 客户端流式调用拦截器"]
        INJECT_OUT["injectTraceToOutgoingContext(), context → outgoing Metadata"]
    end

    CLIENT --> MW
    MW --> EXTRACT
    EXTRACT --> AUTO_GEN
    AUTO_GEN --> INJECT_CTX
    INJECT_CTX --> SET_RESP
    INJECT_CTX --> GW_MUX
    GW_MUX --> UNARY
    GW_MUX --> STREAM
    UNARY --> ENRICH
    STREAM --> ENRICH
    ENRICH --> SET_MD
    ENRICH --> GET_CTX
    ENRICH --> SCOPE
    GET_CTX --> CLIENT_UNARY
    GET_CTX --> CLIENT_STREAM
    CLIENT_UNARY --> INJECT_OUT
    CLIENT_STREAM --> INJECT_OUT
    INJECT_OUT -->|传递到下游服务| CLIENT

    style CLIENT fill:#e1f5fe
    style HTTP_LAYER fill:#fff3e0
    style GW_LAYER fill:#f3e5f5
    style GRPC_SERVER fill:#e8f5e9
    style SERVICE fill:#fce4ec
    style GRPC_CLIENT fill:#e0f2f1
```

## RequestCommonMeta — 请求公共元信息

> 源码：[request\_context.go:RequestCommonMeta](../middleware/request_context.go#L33)

```go
type RequestCommonMeta struct {
    ID             string `json:"id" header:"X-ID"`
    TraceID        string `json:"traceID" header:"X-Trace-ID"`
    RequestID      string `json:"requestID" header:"X-Request-ID"`
    UserID         string `json:"userID" header:"X-User-ID"`
    UserName       string `json:"userName" header:"X-User-Name"`
    Domain         string `json:"domain" header:"X-Domain"`
    RoleCode       string `json:"roleCode" header:"X-Role-Code"`
    TenantID       string `json:"tenantID" header:"X-Tenant-ID"`
    TenantCode     string `json:"tenantCode" header:"X-Tenant-Code"`
    SessionID      string `json:"sessionID" header:"X-Session-ID"`
    Timezone       string `json:"timezone" header:"X-Timezone"`
    Timestamp      string `json:"timestamp" header:"X-Timestamp"`
    Signature      string `json:"signature" header:"X-Signature"`
    Authorization  string `json:"authorization" header:"Authorization"`
    AccessKey      string `json:"accessKey" header:"X-Access-Key"`
    AppID          string `json:"appID" header:"X-App-ID"`
    DeviceID       string `json:"deviceID" header:"X-Device-ID"`
    AppVersion     string `json:"appVersion" header:"X-App-Version"`
    IPAddress      string `json:"ipAddress" header:"X-Forwarded-For"`
    PlatformID     string `json:"platformID" header:"X-Platform-ID"`
    PlatformCode   string `json:"platformCode" header:"X-Platform-Code"`
    RegionID       string `json:"regionID" header:"X-Region-ID"`
    RegionCode     string `json:"regionCode" header:"X-Region-Code"`
    AgentLineID    string `json:"agentLineID" header:"X-Agent-Line-ID"`
    Nonce          string `json:"nonce" header:"X-Nonce"`
    Jti            string `json:"jti" header:"X-Jti"`
    FamilyId       string `json:"familyId" header:"X-Family-ID"`
    XNsID          string `json:"xNsID" header:"X-Ns-ID"`
    UserAgent      string `json:"userAgent" header:"User-Agent"`
    PushToken      string `json:"pushToken" header:"X-Push-Token"`
    Token          string `json:"token" header:"X-Token"`
    AcceptLanguage string `json:"acceptLanguage" header:"Accept-Language"`
    ForwardedHost  string `json:"forwardedHost" header:"X-Forwarded-Host"`
}
```

## 三层映射关系

每个字段在 HTTP Header、gRPC Metadata、Context Key 之间有固定的映射关系：

```mermaid
flowchart LR
    subgraph HTTP["HTTP Header"]
        H1["X-Trace-ID"]
        H2["X-User-ID"]
        H3["X-Tenant-ID"]
        H4["Authorization"]
        H5["X-Request-ID"]
        H6["X-Device-ID"]
        H7["X-Platform-Code"]
    end

    subgraph GRPC["gRPC Metadata"]
        G1["x-trace-id"]
        G2["x-user-id"]
        G3["x-tenant-id"]
        G4["authorization"]
        G5["x-request-id"]
        G6["x-device-id"]
        G7["x-platform-code"]
    end

    subgraph CTX["Context Key"]
        C1["x-trace-id"]
        C2["x-user-id"]
        C3["x-tenant-id"]
        C4["authorization"]
        C5["x-request-id"]
        C6["x-device-id"]
        C7["x-platform-code"]
    end

    H1 -->|Header → Metadata| G1
    H2 -->|Header → Metadata| G2
    H3 -->|Header → Metadata| G3
    H4 -->|Header → Metadata| G4
    H5 -->|Header → Metadata| G5
    H6 -->|Header → Metadata| G6
    H7 -->|Header → Metadata| G7

    G1 -->|Metadata → Context| C1
    G2 -->|Metadata → Context| C2
    G3 -->|Metadata → Context| C3
    G4 -->|Metadata → Context| C4
    G5 -->|Metadata → Context| C5
    G6 -->|Metadata → Context| C6
    G7 -->|Metadata → Context| C7
```

### 完整映射表

| 字段             | HTTP Header             | gRPC Metadata           | Context Key             | 便捷读取函数                       | 源码                                              |
| ---------------- | ----------------------- | ----------------------- | ----------------------- | ---------------------------- | ----------------------------------------------- |
| ID               | `X-ID`                  | `x-id`                  | `x-id`                  | `GetID(ctx)`                 | [metadata.go:L22](../constants/metadata.go#L22) |
| TraceID          | `X-Trace-ID`            | `x-trace-id`            | `x-trace-id`            | `GetTraceID(ctx)`            | [metadata.go:L20](../constants/metadata.go#L20) |
| RequestID        | `X-Request-ID`          | `x-request-id`          | `x-request-id`          | `GetRequestID(ctx)`          | [metadata.go:L21](../constants/metadata.go#L21) |
| UserID           | `X-User-ID`             | `x-user-id`             | `x-user-id`             | `GetUserID(ctx)`             | [metadata.go:L23](../constants/metadata.go#L23) |
| UserName         | `X-User-Name`           | `x-user-name`           | `x-user-name`           | `GetUserName(ctx)`           | [metadata.go:L24](../constants/metadata.go#L24) |
| Domain           | `X-Domain`              | `x-domain`              | `x-domain`              | `GetDomain(ctx)`             | [metadata.go:L25](../constants/metadata.go#L25) |
| RoleCode         | `X-Role-Code`           | `x-role-code`           | `x-role-code`           | `GetRoleCode(ctx)`           | [metadata.go:L26](../constants/metadata.go#L26) |
| TenantID         | `X-Tenant-ID`           | `x-tenant-id`           | `x-tenant-id`           | `GetTenantID(ctx)`           | [metadata.go:L27](../constants/metadata.go#L27) |
| TenantCode       | `X-Tenant-Code`         | `x-tenant-code`         | `x-tenant-code`         | `GetTenantCode(ctx)`         | [metadata.go:L28](../constants/metadata.go#L28) |
| SessionID        | `X-Session-ID`          | `x-session-id`          | `x-session-id`          | `GetSessionID(ctx)`          | [metadata.go:L29](../constants/metadata.go#L29) |
| Timezone         | `X-Timezone`            | `x-timezone`            | `x-timezone`            | `GetTimezone(ctx)`           | [metadata.go:L30](../constants/metadata.go#L30) |
| IPAddress        | `X-Forwarded-For`       | `x-ip-address`          | `x-ip-address`          | `GetIPAddress(ctx)`          | [metadata.go:L31](../constants/metadata.go#L31) |
| ForwardedHost    | `X-Forwarded-Host`      | `x-forwarded-host`      | `x-forwarded-host`      | `GetForwardedHost(ctx)`      | [metadata.go:L32](../constants/metadata.go#L32) |
| AppID            | `X-App-ID`              | `x-app-id`              | `x-app-id`              | `GetAppID(ctx)`              | [metadata.go:L33](../constants/metadata.go#L33) |
| DeviceID         | `X-Device-ID`           | `x-device-id`           | `x-device-id`           | `GetDeviceID(ctx)`           | [metadata.go:L34](../constants/metadata.go#L34) |
| AppVersion       | `X-App-Version`         | `x-app-version`         | `x-app-version`         | `GetAppVersion(ctx)`         | [metadata.go:L35](../constants/metadata.go#L35) |
| PlatformID       | `X-Platform-ID`         | `x-platform-id`         | `x-platform-id`         | `GetPlatformID(ctx)`         | [metadata.go:L36](../constants/metadata.go#L36) |
| PlatformCode     | `X-Platform-Code`       | `x-platform-code`       | `x-platform-code`       | `GetPlatformCode(ctx)`       | [metadata.go:L37](../constants/metadata.go#L37) |
| RegionID         | `X-Region-ID`           | `x-region-id`           | `x-region-id`           | `GetRegionID(ctx)`           | [metadata.go:L38](../constants/metadata.go#L38) |
| RegionCode       | `X-Region-Code`         | `x-region-code`         | `x-region-code`         | `GetRegionCode(ctx)`         | [metadata.go:L39](../constants/metadata.go#L39) |
| AgentLineID      | `X-Agent-Line-ID`       | `x-agent-line-id`       | `x-agent-line-id`       | `GetAgentLineID(ctx)`        | [metadata.go:L40](../constants/metadata.go#L40) |
| Nonce            | `X-Nonce`               | `x-nonce`               | `x-nonce`               | `GetNonce(ctx)`              | [metadata.go:L41](../constants/metadata.go#L41) |
| XNsID            | `X-Ns-ID`               | `x-ns-id`               | `x-ns-id`               | `GetXNsID(ctx)`              | [metadata.go:L42](../constants/metadata.go#L42) |
| Authorization    | `Authorization`         | `authorization`         | `authorization`         | `GetAuthorization(ctx)`      | [metadata.go:L43](../constants/metadata.go#L43) |
| Timestamp        | `X-Timestamp`           | `x-timestamp`           | `x-timestamp`           | `GetTimestamp(ctx)`          | [metadata.go:L45](../constants/metadata.go#L45) |
| Signature        | `X-Signature`           | `x-signature`           | `x-signature`           | `GetSignature(ctx)`          | [metadata.go:L46](../constants/metadata.go#L46) |
| AccessKey        | `X-Access-Key`          | `x-access-key`          | `x-access-key`          | `GetAccessKey(ctx)`          | [metadata.go:L47](../constants/metadata.go#L47) |
| Jti              | `X-Jti`                 | `x-jti`                 | `x-jti`                 | `GetJti(ctx)`                | [metadata.go:L48](../constants/metadata.go#L48) |
| FamilyId         | `X-Family-ID`           | `x-family-id`           | `x-family-id`           | `GetFamilyId(ctx)`           | [metadata.go:L49](../constants/metadata.go#L49) |
| UserAgent        | `User-Agent`            | `x-user-agent`          | `x-user-agent`          | `GetUserAgent(ctx)`          | [metadata.go:L50](../constants/metadata.go#L50) |
| PushToken        | `X-Push-Token`          | `x-push-token`          | `x-push-token`          | `GetPushToken(ctx)`          | [metadata.go:L51](../constants/metadata.go#L51) |
| Token            | `X-Token`               | `x-token`               | `x-token`               | `GetToken(ctx)`              | [metadata.go:L52](../constants/metadata.go#L52) |
| AcceptLanguage   | `Accept-Language`       | `accept-language`       | `accept-language`       | `GetAcceptLanguageFromMeta(ctx)` | [metadata.go:L53](../constants/metadata.go#L53) |

> 注意：gRPC Metadata 键必须小写，这是 gRPC 协议规范。HTTP Header 使用大写 `X-` 前缀，gRPC-Gateway 的 `WithIncomingHeaderMatcher` 自动完成大小写转换。`Authorization` 与 `Accept-Language` 因属标准 Header，键名不带 `x-` 前缀。

## HTTP 中间件详解

> 源码：[request\_context.go:RequestContextMiddleware()](../middleware/request_context.go#L82)

```mermaid
flowchart TD
    REQ["HTTP Request"] --> MW["RequestContextMiddleware()"]

    MW --> STEP1["1. 从 HTTP Header 提取字段, gccommon.ExtractAttribute(r, sources)"]
    STEP1 --> STEP2["2. 自动生成 TraceID, extractOrGenerateTraceID(), 优先 Header → OpenTelemetry Span → osx.HashUnixMicroCipherText()"]
    STEP2 --> STEP3["3. 自动生成 RequestID, extractOrGenerateRequestID(), 优先 Header → osx.HashUnixMicroCipherText()"]
    STEP3 --> STEP4["4. 提取客户端 IP, netx.GetClientIP(r)"]
    STEP4 --> STEP5["5. 单次 WithRequestCommonMeta(ctx, meta), 缓存完整 RequestCommonMeta, 避免 30+ 层 context 嵌套"]
    STEP5 --> STEP6["6. 设置响应头, X-Trace-ID / X-Request-ID / X-Server-Node"]
    STEP6 --> NEXT["next.ServeHTTP(w, r.WithContext(ctx))"]

    style STEP2 fill:#fff9c4
    style STEP3 fill:#fff9c4
```

### TraceID 生成策略

> 源码：[request\_context.go:extractOrGenerateTraceID()](../middleware/request_context.go#L426)

1. 优先使用请求头中的 `X-Trace-ID`
2. 如果存在 OpenTelemetry Span，使用 Span 的 TraceID
3. 以上都不存在时，使用 `osx.HashUnixMicroCipherText()` 生成

### RequestID 生成策略

> 源码：[request\_context.go:extractOrGenerateRequestID()](../middleware/request_context.go#L440)

1. 优先使用请求头中的 `X-Request-ID`
2. 不存在时，使用 `osx.HashUnixMicroCipherText()` 生成

### 可配置的数据源

每个字段的提取源可通过 `Gateway.RequestContext` 配置：

```yaml
request-context:
  trace-id-sources:
    - header: "X-Trace-ID"
    - query: "trace_id"
  user-id-sources:
    - header: "X-User-ID"
  tenant-id-sources:
    - header: "X-Tenant-ID"
```

`gccommon.ExtractAttribute(r, sources)` 按配置顺序依次尝试，取第一个非空值。

## gRPC Server 拦截器详解

> 源码：[request\_context.go:UnaryServerRequestContextInterceptor()](../middleware/request_context.go#L188)

```mermaid
flowchart TD
    REQ["gRPC 请求到达"] --> UNARY["UnaryServerRequestContextInterceptor()"]

    UNARY --> STEP1["1. metadata.FromIncomingContext(ctx), 获取 incoming gRPC Metadata"]
    STEP1 --> STEP2["2. firstMetadataValue(key), 逐字段提取 Metadata 值"]
    STEP2 --> STEP3["3. extractOrGenerateTraceID(), 提取或生成 TraceID"]
    STEP3 --> STEP4["4. extractOrGenerateRequestID(), 提取或生成 RequestID"]
    STEP4 --> STEP5["5. WithRequestCommonMeta(ctx, meta), 单次 WithValue 缓存完整元信息, 避免 30+ 层 context 嵌套"]
    STEP5 --> STEP6["6. setResponseMetadata(ctx), 设置 gRPC 响应 Metadata, grpc.SetHeader(ctx, md)"]
    STEP6 --> HANDLER["handler(ctx, req)"]

    style STEP1 fill:#e8f5e9
    style STEP6 fill:#e8f5e9
```

### enrichContextFromMetadata — 核心提取逻辑

> 源码：[request\_context.go:enrichContextFromMetadata()](../middleware/request_context.go#L223)

```go
func enrichContextFromMetadata(ctx context.Context) context.Context {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        md = metadata.MD{}
    }

    firstMetadataValue := func(key string) string {
        if values := md.Get(key); len(values) > 0 {
            return values[0]
        }
        return ""
    }

    // 提取所有字段，构建 RequestCommonMeta
    meta := &RequestCommonMeta{
        ID:             firstMetadataValue(constants.MetadataID),
        TraceID:        extractOrGenerateTraceID(ctx, firstMetadataValue(constants.MetadataTraceID)),
        RequestID:      extractOrGenerateRequestID(firstMetadataValue(constants.MetadataRequestID)),
        // ... 其余字段均通过 firstMetadataValue 提取
    }

    // 单次 WithValue，避免 30+ 层 context 嵌套
    return WithRequestCommonMeta(ctx, meta)
}
```

### setResponseMetadata — 响应回传

> 源码：[request\_context.go:setResponseMetadata()](../middleware/request_context.go#L281)

将 `RequestCommonMeta` 中的所有字段通过 `grpc.SetHeader()` 写回 gRPC 响应 Metadata，确保下游服务或客户端可以获取到完整的上下文信息。同时会追加 `x-server-node`（来自 `global.GetServerNode()`）以便在 K8s 等环境中定位处理请求的具体 Pod。

## gRPC Client 拦截器详解

> 源码：[request\_context.go:UnaryClientRequestContextInterceptor()](../middleware/request_context.go#L348)

```mermaid
flowchart TD
    CALL["Service 调用下游 gRPC 服务"] --> CLIENT["UnaryClientRequestContextInterceptor()"]
    CLIENT --> INJECT["injectTraceToOutgoingContext(ctx)"]
    INJECT --> GET["GetRequestCommonMeta(ctx), 获取当前请求的完整元信息"]
    GET --> PAIRS["metadata.Pairs(), 构建所有字段的 Metadata"]
    PAIRS --> JOIN["metadata.Join(existingMD, md), 合并已有 outgoing Metadata"]
    JOIN --> NEW["metadata.NewOutgoingContext(ctx, md), 写入 outgoing Metadata"]
    NEW --> INVOKE["invoker(ctx, method, req, reply, cc, opts...)"]

    style INJECT fill:#e0f2f1
    style NEW fill:#e0f2f1
```

### injectTraceToOutgoingContext — 核心注入逻辑

> 源码：[request\_context.go:injectTraceToOutgoingContext()](../middleware/request_context.go#L366)

```go
func injectTraceToOutgoingContext(ctx context.Context) context.Context {
    requestCommonMeta := GetRequestCommonMeta(ctx)

    // 优先使用 RequestCommonMeta 中的 Accept Language，若为空则从 i18n context 获取
    acceptLanguage := mathx.IfEmpty(requestCommonMeta.AcceptLanguage, goi18n.GetLanguage(ctx))

    md := metadata.Pairs(
        constants.MetadataID, requestCommonMeta.ID,
        constants.MetadataTraceID, requestCommonMeta.TraceID,
        constants.MetadataRequestID, requestCommonMeta.RequestID,
        // ... 所有字段
        constants.MetadataAcceptLanguage, acceptLanguage,
    )

    // 合并已有的 outgoing metadata
    if existingMD, ok := metadata.FromOutgoingContext(ctx); ok {
        md = metadata.Join(existingMD, md)
    }

    // 透传服务节点标识到下游服务
    if serverNode := global.GetServerNode(); serverNode != "" {
        md.Append(constants.MetadataServerNode, serverNode)
    }

    return metadata.NewOutgoingContext(ctx, md)
}
```

> 此外 `ForwardOutgoingContext(r *http.Request) context.Context`（[request\_context.go:L448](../middleware/request_context.go#L448)）可将 HTTP 请求的所有 Header（除 `connection` 外）原样转发为 gRPC outgoing Metadata，适用于需要全量透传 Header 的场景。

## 跨服务传递完整链路

```mermaid
sequenceDiagram
    participant Client as 客户端
    participant HTTP as HTTP 中间件
    participant GW as gRPC-Gateway
    participant ServerA as 服务 A (gRPC Server)
    participant ServiceA as Service A
    participant ClientA as 服务 A (gRPC Client)
    participant ServerB as 服务 B (gRPC Server)
    participant ServiceB as Service B

    Client->>HTTP: HTTP Request X-Trace-ID: abc123 X-User-ID: user-001 X-Tenant-ID: tenant-001
    HTTP->>HTTP: 提取 Headers → RequestCommonMeta
    HTTP->>HTTP: 注入 context + 设置响应头
    HTTP->>GW: context 传递
    GW->>GW: WithIncomingHeaderMatcher, HTTP Headers → gRPC Metadata
    GW->>ServerA: gRPC 请求 (Metadata 携带)

    ServerA->>ServerA: enrichContextFromMetadata(), Metadata → context
    ServerA->>ServerA: setResponseMetadata(), 设置响应 Metadata
    ServerA->>ServiceA: handler(ctx, req)

    ServiceA->>ClientA: 调用下游服务
    ClientA->>ClientA: injectTraceToOutgoingContext(), context → outgoing Metadata
    ClientA->>ServerB: gRPC 请求 (Metadata 携带)

    ServerB->>ServerB: enrichContextFromMetadata(), Metadata → context
    ServerB->>ServiceB: handler(ctx, req)

    Note over Client,ServiceB: 全链路 TraceID/RequestID/UserID/TenantID 一致
```

## 便捷读取函数

> 源码：[request\_context.go:GetXxx()](../middleware/request_context.go#L466)

所有读取函数内部调用 `GetRequestCommonMeta(ctx)`，该函数有二级缓存机制：

1. **优先**：从 `context.Value(requestCommonMetaKey{})` 获取缓存的 `RequestCommonMeta`
2. **回退**：从 `context.Value(constants.MetadataXxx)` 逐字段提取，组装新的 `RequestCommonMeta`

| 函数                            | 返回字段                 | 源码                                                                |
| ------------------------------- | ---------------------- | ------------------------------------------------------------------- |
| `GetID(ctx)`                    | ID                     | [request\_context.go:L532](../middleware/request_context.go#L532)   |
| `GetTraceID(ctx)`               | TraceID                | [request\_context.go:L466](../middleware/request_context.go#L466)   |
| `GetRequestID(ctx)`             | RequestID              | [request\_context.go:L472](../middleware/request_context.go#L472)   |
| `GetUserID(ctx)`                | UserID                 | [request\_context.go:L478](../middleware/request_context.go#L478)   |
| `GetUserName(ctx)`              | UserName               | [request\_context.go:L484](../middleware/request_context.go#L484)   |
| `GetDomain(ctx)`                | Domain                 | [request\_context.go:L490](../middleware/request_context.go#L490)   |
| `GetRoleCode(ctx)`              | RoleCode               | [request\_context.go:L496](../middleware/request_context.go#L496)   |
| `GetTenantID(ctx)`              | TenantID               | [request\_context.go:L502](../middleware/request_context.go#L502)   |
| `GetTenantCode(ctx)`            | TenantCode             | [request\_context.go:L538](../middleware/request_context.go#L538)   |
| `GetSessionID(ctx)`             | SessionID              | [request\_context.go:L508](../middleware/request_context.go#L508)   |
| `GetTimezone(ctx)`              | Timezone               | [request\_context.go:L514](../middleware/request_context.go#L514)   |
| `GetIPAddress(ctx)`             | IPAddress              | [request\_context.go:L526](../middleware/request_context.go#L526)   |
| `GetAppID(ctx)`                 | AppID                  | [request\_context.go:L580](../middleware/request_context.go#L580)   |
| `GetDeviceID(ctx)`              | DeviceID               | [request\_context.go:L586](../middleware/request_context.go#L586)   |
| `GetAppVersion(ctx)`            | AppVersion             | [request\_context.go:L592](../middleware/request_context.go#L592)   |
| `GetPlatformID(ctx)`            | PlatformID             | [request\_context.go:L544](../middleware/request_context.go#L544)   |
| `GetPlatformCode(ctx)`          | PlatformCode           | [request\_context.go:L550](../middleware/request_context.go#L550)   |
| `GetRegionID(ctx)`              | RegionID               | [request\_context.go:L556](../middleware/request_context.go#L556)   |
| `GetRegionCode(ctx)`            | RegionCode             | [request\_context.go:L562](../middleware/request_context.go#L562)   |
| `GetAgentLineID(ctx)`           | AgentLineID            | [request\_context.go:L568](../middleware/request_context.go#L568)   |
| `GetXNsID(ctx)`                 | XNsID (命名空间)        | [request\_context.go:L574](../middleware/request_context.go#L574)   |
| `GetAuthorization(ctx)`         | Authorization          | [request\_context.go:L520](../middleware/request_context.go#L520)   |
| `GetTimestamp(ctx)`             | Timestamp              | [request\_context.go:L622](../middleware/request_context.go#L622)   |
| `GetSignature(ctx)`             | Signature              | [request\_context.go:L628](../middleware/request_context.go#L628)   |
| `GetAccessKey(ctx)`             | AccessKey              | [request\_context.go:L634](../middleware/request_context.go#L634)   |
| `GetNonce(ctx)`                 | Nonce                  | [request\_context.go:L598](../middleware/request_context.go#L598)   |
| `GetJti(ctx)`                   | Jti (JWT ID)           | [request\_context.go:L604](../middleware/request_context.go#L604)   |
| `GetFamilyId(ctx)`              | FamilyId (Token 家族)   | [request\_context.go:L610](../middleware/request_context.go#L610)   |
| `GetUserAgent(ctx)`             | UserAgent              | [request\_context.go:L616](../middleware/request_context.go#L616)   |
| `GetPushToken(ctx)`             | PushToken              | [request\_context.go:L640](../middleware/request_context.go#L640)   |
| `GetToken(ctx)`                 | Token                  | [request\_context.go:L646](../middleware/request_context.go#L646)   |
| `GetAcceptLanguageFromMeta(ctx)`| AcceptLanguage         | [request\_context.go:L883](../middleware/request_context.go#L883)   |
| `GetForwardedHost(ctx)`         | ForwardedHost          | [request\_context.go:L898](../middleware/request_context.go#L898)   |

### WithXxx — 写入函数

每个字段都有对应的 `WithXxx(ctx, value)` 函数，使用 `contextx.WithValue()` 写入 context，并同步更新已缓存的 `RequestCommonMeta`（通过 `updateRequestCommonMetaField`）：

```go
ctx = middleware.WithTraceID(ctx, "abc123")
ctx = middleware.WithUserID(ctx, "user-001")
ctx = middleware.WithTenantID(ctx, "tenant-001")
```

> 源码：[request\_context.go:WithXxx()](../middleware/request_context.go#L659)（`WithTraceID` 起，后续每个字段一个 `WithXxx`）

### ContextBuilder — 链式构建器

> 源码：[request\_context.go:ContextBuilder](../middleware/request_context.go#L906)

当需要一次性设置多个字段时，使用 `ContextBuilder` 链式调用更清晰：

```go
ctx = middleware.NewContextBuilder(ctx).
    WithTraceID("trace-123").
    WithUserID("user-456").
    WithTenantID("tenant-789").
    WithAuthorization("Bearer token").
    Build()
```

`ContextBuilder` 内部依次调用对应的 `WithXxx`，因此同样会同步更新 `RequestCommonMeta`，后续 Client 拦截器可自动传播到 outgoing metadata。

## 使用示例

### Service 层读取上下文

```go
func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
    tenantID := middleware.GetTenantID(ctx)
    userID := middleware.GetUserID(ctx)
    traceID := middleware.GetTraceID(ctx)
    domain := middleware.GetDomain(ctx)

    s.logger.InfoContext(ctx, "GetUser called",
        "tenant_id", tenantID,
        "user_id", userID,
        "trace_id", traceID,
        "domain", domain)

    return &pb.GetUserResponse{}, nil
}
```

### 获取完整元信息

```go
func (s *OrderService) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
    meta := middleware.GetRequestCommonMeta(ctx)

    s.logger.InfoContext(ctx, "CreateOrder",
        "tenant", meta.TenantID,
        "user", meta.UserID,
        "platform", meta.PlatformCode,
        "region", meta.RegionCode,
        "app_version", meta.AppVersion,
        "device", meta.DeviceID,
        "ip", meta.IPAddress)

    return &pb.CreateOrderResponse{}, nil
}
```

### Repository 层数据隔离

```go
func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    tenantID := middleware.GetTenantID(ctx)

    var user User
    err := r.db.WithContext(ctx).
        Where("id = ? AND tenant_id = ?", id, tenantID).
        First(&user).Error

    return &user, err
}
```

## ContextScopeReader — 作用域适配器

> 源码：[middleware/scope\_reader.go](../middleware/scope_reader.go)

将请求上下文适配为外部作用域读取接口，供限流、签名等中间件获取租户/角色等维度信息：

```go
type ContextScopeReader struct{}

func (ContextScopeReader) GetDomain(ctx context.Context) string   // 委托 GetDomain(ctx)
func (ContextScopeReader) GetTenantID(ctx context.Context) string // 委托 GetTenantID(ctx)
func (ContextScopeReader) GetRoleCode(ctx context.Context) string // 委托 GetRoleCode(ctx)
```

> 源码：[scope\_reader.go:L16-L31](../middleware/scope_reader.go#L16)

## 下一步

- [中间件系统](./MIDDLEWARE.md) — 了解所有中间件
- [gRPC 客户端](./GRPC-CLIENT.md) — 了解客户端拦截器如何传播上下文
- [常量参考](./CONSTANTS.md) — 查看完整的 Metadata 键和日志字段定义

