/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\internal\constants\constants.go
 * @Description: 项目常量定义 - 统一管理所有字符串字面量
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package constants

// 路径常量
const (
	// 系统路径
	PathHealth   = "/health"
	PathMetrics  = "/metrics"
	PathDebug    = "/debug"
	PathPprof    = "/debug/pprof"
	
	// API 路径前缀
	PathAPIV1    = "/api/v1"
	PathAPIV2    = "/api/v2"
	PathGraphQL  = "/graphql"
	PathWebSocket = "/ws"
)

// 日志相关常量
const (
	// 日志级别
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
	LogLevelPanic = "panic"
	
	// 日志格式
	LogFormatJSON = "json"
	LogFormatText = "text"
)

// 环境常量
const (
	EnvDevelopment = "development"
	EnvTesting     = "testing"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// 算法常量
const (
	// 限流算法
	RateLimitTokenBucket   = "token_bucket"
	RateLimitSlidingWindow = "sliding_window"
	RateLimitFixedWindow   = "fixed_window"
	
	// 签名算法
	SignatureHMACSHA256 = "hmac-sha256"
	SignatureHMACSHA512 = "hmac-sha512"
	SignatureRS256      = "rs256"
	
	// 哈希算法
	HashMD5    = "md5"
	HashSHA1   = "sha1"
	HashSHA256 = "sha256"
	HashSHA512 = "sha512"
)

// 认证类型常量
const (
	AuthTypeJWT    = "jwt"
	AuthTypeBasic  = "basic"
	AuthTypeOAuth2 = "oauth2"
	AuthTypeAPIKey = "apikey"
	AuthTypeBearer = "bearer"
)

// 缓存相关常量
const (
	CacheKeyPrefix    = "gateway:"
	CacheKeyUser      = "user:"
	CacheKeySession   = "session:"
	CacheKeyRateLimit = "ratelimit:"
	CacheKeySignature = "signature:"
)

// 数据库常量
const (
	DBDriverMySQL      = "mysql"
	DBDriverPostgreSQL = "postgres"
	DBDriverSQLite     = "sqlite3"
	DBDriverMongoDB    = "mongodb"
	DBDriverRedis      = "redis"
)

// 字符集常量
const (
	CharsetUTF8     = "utf8"
	CharsetUTF8MB4  = "utf8mb4"
	CharsetLatin1   = "latin1"
)

// 时间格式常量
const (
	TimeFormatISO8601    = "2006-01-02T15:04:05Z07:00"
	TimeFormatRFC3339    = "2006-01-02T15:04:05Z07:00"
	TimeFormatDateTime   = "2006-01-02 15:04:05"
	TimeFormatDate       = "2006-01-02"
	TimeFormatTime       = "15:04:05"
	TimeFormatUnix       = "1136239445"
)

// 文件大小常量
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
	TB = 1024 * GB
)

// 网络地址常量
const (
	DefaultLocalhost = "127.0.0.1"
	DefaultAnyHost   = "0.0.0.0"
)

// 默认配置常量
const (
	DefaultHost           = DefaultLocalhost
	DefaultHTTPPort       = 8080
	DefaultGRPCPort       = 9090
	DefaultReadTimeout    = 30  // 秒
	DefaultWriteTimeout   = 30  // 秒
	DefaultIdleTimeout    = 120 // 秒
	DefaultMaxHeaderBytes = 1 * MB
	
	// 数据库默认配置
	DefaultDBHost         = DefaultLocalhost
	DefaultDBPort         = 3306
	DefaultDBMaxIdleConns = 10
	DefaultDBMaxOpenConns = 100
	DefaultDBConnMaxLifetime = 300 // 秒
	
	// Redis 默认配置
	DefaultRedisHost = DefaultLocalhost
	DefaultRedisPort = 6379
	DefaultRedisDB   = 0
	
	// 限流默认配置
	DefaultRateLimit     = 100  // 每秒请求数
	DefaultBurstLimit    = 200  // 突发限制
	DefaultWindowSize    = 60   // 窗口大小（秒）
)

// 正则表达式常量
const (
	RegexEmail    = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	RegexPhone    = `^1[3-9]\d{9}$`
	RegexUsername = `^[a-zA-Z0-9_]{3,50}$`
	RegexPassword = `^.{6,128}$`
	RegexUUID     = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	RegexIPv4     = `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`
	RegexURL      = `^https?://[^\s/$.?#].[^\s]*$`
)