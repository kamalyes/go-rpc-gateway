/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2024-11-03 22:15:56
 * @FilePath: \go-rpc-gateway\jwt\jwt.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package jwt

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/golang-jwt/jwt/v4"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"gorm.io/gorm"
)

// 定义一些常量
var (
	TokenExpired     = errors.ErrTokenExpired
	TokenNotValidYet = errors.ErrTokenNotValidYet
	TokenMalformed   = errors.ErrTokenMalformed
	TokenInvalid     = errors.ErrInvalidToken
	jwtSignKey       = "82011FC650590620FEFAC6500ADAB0F77" // 默认签名用的key
)

// JWT jwt签名结构
type JWT struct {
	SigningKey []byte
}

// SetJWTSignKey 动态设置JWT签名密钥
func SetJWTSignKey(key string) {
	jwtSignKey = key
}

// GetJWTSignKey 获取JWT签名密钥
func GetJWTSignKey() string {
	return jwtSignKey
}

// NewJWT 新建一个 jwt 实例
func NewJWT() *JWT {
	return &JWT{[]byte(GetJWTSignKey())}
}

// RegisteredClaims expiresAt 过期时间单位秒
func RegisteredClaims(issuer string, expiresAt int64) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Unix(expiresAt, 0)),
	}
}

// CreateToken 生成 token
func (j *JWT) CreateToken(claims CustomClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// 判断多点登录拦截是否开启
	if global.GATEWAY != nil && global.GATEWAY.JWT != nil && global.GATEWAY.JWT.UseMultipoint {
		// 拦截
		if global.REDIS != nil {
			// 优先存入到 redis
			jsonData, _ := json.Marshal(claims)
			toJson := string(jsonData)
			// 此处过期时间等于jwt过期时间
			timer := time.Duration(global.GATEWAY.JWT.ExpiresTime) * time.Second
			err := global.REDIS.Set(context.Background(), claims.UserId, toJson, timer).Err()
			if err != nil {
				return "", err
			}
			return token.SignedString(j.SigningKey)
		}
		// 没有redis存入到 数据库
		err := global.DB.Save(&claims).Error
		if err != nil {
			return "", err
		} else {
			return token.SignedString(j.SigningKey)
		}
	}
	// 不拦截
	return token.SignedString(j.SigningKey)
}

// DeleteToken 强制删除Token记录，用途--用户账号被盗后，强制下线
func DeleteToken(userId string) (err error) {
	if global.REDIS != nil {
		err = global.REDIS.Del(context.Background(), userId).Err()
		return err
	}
	err = global.DB.Where("user_id = ?", userId).Delete(&CustomClaims{}).Error
	return err
}

// ResolveToken 解析token
func (j *JWT) ResolveToken(tokenString string) (*CustomClaims, error) {
	token, parseErr := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.SigningKey, nil
	})
	if parseErr != nil {
		return handleTokenParseError(parseErr)
	}

	if token != nil && token.Valid {
		claims, ok := token.Claims.(*CustomClaims)
		if !ok {
			return nil, TokenInvalid
		}

		if !j.isMultipointAuthEnabled() {
			return claims, nil
		}

		if err := j.checkMultipointAuth(claims); err != nil {
			return nil, err
		}

		return claims, nil
	}

	return nil, TokenInvalid
}

// handleTokenParseError 处理token解析错误
func handleTokenParseError(err error) (*CustomClaims, error) {
	if ve, ok := err.(*jwt.ValidationError); ok {
		switch {
		case ve.Errors&jwt.ValidationErrorMalformed != 0:
			return nil, TokenMalformed
		case ve.Errors&jwt.ValidationErrorExpired != 0:
			return nil, TokenExpired
		case ve.Errors&jwt.ValidationErrorNotValidYet != 0:
			return nil, TokenNotValidYet
		default:
			return nil, TokenInvalid
		}
	}
	return nil, err
}

// isMultipointAuthEnabled 检查是否启用多点登录拦截
func (j *JWT) isMultipointAuthEnabled() bool {
	if global.GATEWAY != nil && global.GATEWAY.JWT != nil {
		return global.GATEWAY.JWT.UseMultipoint
	}
	return false
}

// checkMultipointAuth 检查多点登录验证
func (j *JWT) checkMultipointAuth(claims *CustomClaims) error {
	if global.REDIS != nil {
		if jsonStr, err := global.REDIS.Get(context.Background(), claims.UserId).Result(); err == redis.Nil || jsonStr == "" {
			return nil
		} else {
			return j.checkRedisMultipointAuth(claims, jsonStr)
		}
	}
	return j.checkDBMultipointAuth(claims)
}

// checkRedisMultipointAuth 检查Redis中的多点登录验证
func (j *JWT) checkRedisMultipointAuth(claims *CustomClaims, jsonStr string) error {
	var clis CustomClaims
	if err := json.Unmarshal([]byte(jsonStr), &clis); err != nil {
		return errors.WrapWithContext(err, errors.ErrCodeRedisParseError)
	}

	if clis.TokenId != "" && claims.TokenId != clis.TokenId {
		return errors.ErrAccountLoginElsewhere
	}

	return nil
}

// checkDBMultipointAuth 检查数据库中的多点登录验证
func (j *JWT) checkDBMultipointAuth(claims *CustomClaims) error {
	var clis CustomClaims
	if err := global.DB.Where("user_id = ?", claims.UserId).First(&clis).Error; err != nil && err != gorm.ErrRecordNotFound {
		return errors.WrapWithContext(err, errors.ErrCodeDBQueryError)
	}

	if claims.TokenId != clis.TokenId {
		return errors.ErrAccountLoginElsewhere
	}

	return nil
}

// RefreshToken 更新token
func (j *JWT) RefreshToken(tokenString string) (string, error) {
	jwt.TimeFunc = func() time.Time {
		return time.Unix(0, 0)
	}
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.SigningKey, nil
	})
	if err != nil {
		return "", err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		jwt.TimeFunc = time.Now
		var expiresTime int64 = 3600 * 24 * 7 // 默认7天
		if global.GATEWAY != nil && global.GATEWAY.JWT != nil {
			expiresTime = global.GATEWAY.JWT.ExpiresTime
		}
		claims.RegisteredClaims.ExpiresAt = jwt.NewNumericDate(time.Unix(time.Now().Unix()+expiresTime, 0))
		return j.CreateToken(*claims)
	}
	return "", TokenInvalid
}

// GetClaims 从 context.Context 中获取 Claims
func GetClaims(ctx context.Context) (*CustomClaims, error) {
	if claims := ctx.Value("claims"); claims != nil {
		if token, ok := claims.(*CustomClaims); ok {
			return token, nil
		}
	}
	global.LOGGER.Error("从 Context 中获取从 jwt 解析出来的用户 claims 失败, 请检查是否已设置 claims")
	return nil, errors.ErrClaimsParseFailed
}

// ClaimHandlerFunc 定义处理声明的函数
type ClaimHandlerFunc func(*CustomClaims) interface{}

// ClaimHandlers 存储不同类型Claim的处理函数
var ClaimHandlers = map[string]ClaimHandlerFunc{
	"TokenId":      func(claims *CustomClaims) interface{} { return claims.TokenId },
	"UserId":       func(claims *CustomClaims) interface{} { return claims.UserId },
	"UserName":     func(claims *CustomClaims) interface{} { return claims.UserName },
	"UserType":     func(claims *CustomClaims) interface{} { return claims.UserType },
	"NickName":     func(claims *CustomClaims) interface{} { return claims.NickName },
	"PhoneNumber":  func(claims *CustomClaims) interface{} { return claims.PhoneNumber },
	"MerchantNo":   func(claims *CustomClaims) interface{} { return claims.MerchantNo },
	"AuthorityId":  func(claims *CustomClaims) interface{} { return claims.AuthorityId },
	"AppProductId": func(claims *CustomClaims) interface{} { return claims.AppProductId },
	"PlatformType": func(claims *CustomClaims) interface{} { return claims.PlatformType },
	"BufferTime":   func(claims *CustomClaims) interface{} { return claims.BufferTime },
	"Extend":       func(claims *CustomClaims) interface{} { return claims.Extend },
}

// GetClaimValue 从 context.Context 中获取特定类型的Claim值，通过ClaimHandlers映射来获取
func GetClaimValue(ctx context.Context, key string) interface{} {
	claims := ctx.Value("claims")
	if claims == nil {
		return nil
	}

	customClaims, ok := claims.(*CustomClaims)
	if !ok {
		return nil
	}

	handler, found := ClaimHandlers[key]
	if !found {
		return nil
	}

	return handler(customClaims)
}

// GetStringClaimValue 从 context.Context 中获取字符串类型的Claim值
func GetStringClaimValue(ctx context.Context, key string) string {
	value := GetClaimValue(ctx, key)
	if strValue, ok := value.(string); ok {
		return strValue
	}
	return ""
}

// GetInt32ClaimValue 从 context.Context 中获取Int32类型的Claim值
func GetInt32ClaimValue(ctx context.Context, key string) int32 {
	value := GetClaimValue(ctx, key)
	if intValue, ok := value.(int32); ok {
		return intValue
	}
	return 0
}

// GetInt64ClaimValue 从 context.Context 中获取Int64类型的Claim值
func GetInt64ClaimValue(ctx context.Context, key string) int64 {
	value := GetClaimValue(ctx, key)
	if intValue, ok := value.(int64); ok {
		return intValue
	}
	return 0
}

// GetTokenId 从 context.Context 中获取Token Id
func GetTokenId(ctx context.Context) string {
	return GetStringClaimValue(ctx, "TokenId")
}

// GetUserId 从 context.Context 中获取用户Id
func GetUserId(ctx context.Context) string {
	return GetStringClaimValue(ctx, "UserId")
}

// GetUserName 从 context.Context 中获取用户名
func GetUserName(ctx context.Context) string {
	return GetStringClaimValue(ctx, "UserName")
}

// GetUserType 从 context.Context 中获取用户类型
func GetUserType(ctx context.Context) string {
	return GetStringClaimValue(ctx, "UserType")
}

// GetNickName 从 context.Context 中获取用户昵称
func GetNickName(ctx context.Context) string {
	return GetStringClaimValue(ctx, "NickName")
}

// GetPhoneNumber 从 context.Context 中获取用户手机号
func GetPhoneNumber(ctx context.Context) string {
	return GetStringClaimValue(ctx, "PhoneNumber")
}

// GetMerchantNo 从 context.Context 中获取商户号
func GetMerchantNo(ctx context.Context) string {
	return GetStringClaimValue(ctx, "MerchantNo")
}

// GetUserAuthorityId 从 context.Context 中获取用户角色Id
func GetUserAuthorityId(ctx context.Context) string {
	return GetStringClaimValue(ctx, "AuthorityId")
}

// GetAppProductId 从 context.Context 中获取AppProduct Id
func GetAppProductId(ctx context.Context) int32 {
	return GetInt32ClaimValue(ctx, "AppProductId")
}

// GetPlatformType 从 context.Context 中获取Platform Type
func GetPlatformType(ctx context.Context) int32 {
	return GetInt32ClaimValue(ctx, "PlatformType")
}

// GetBufferTime 从 context.Context 中获取BufferTime
func GetBufferTime(ctx context.Context) int64 {
	return GetInt64ClaimValue(ctx, "BufferTime")
}

// GetExtend 从 context.Context 中获取Extend
func GetExtend(ctx context.Context) string {
	return GetStringClaimValue(ctx, "Extend")
}
