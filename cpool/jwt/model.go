/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 21:41:41
 * @FilePath: \go-rpc-gateway\cpool\jwt\model.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package jwt

import (
	"fmt"

	"github.com/golang-jwt/jwt/v4"
	"github.com/kamalyes/go-rpc-gateway/global"
	"go.uber.org/zap"
)

// 自动建表
func AutoCreateTables() {
	if global.DB != nil {
		err := global.DB.AutoMigrate(
			CustomClaims{},
		)
		if err != nil {
			errMsgs := fmt.Sprintf("自动创建%s表失败", string(CustomClaims{}.TableName()))
			global.LOGGER.Error(errMsgs, zap.Any("err", err))
		}
	}
}

// TableName 自定义表名
func (CustomClaims) TableName() string {
	return global.GPerFix + "custom_claims"
}

// CustomClaims 基础 claims
type CustomClaims struct {

	/** 单次登陆产生的tokenId */
	TokenId string `json:"tokenId"           gorm:"column:token_id;index;comment:;type:varchar(36);"`

	/** 用户账号id */
	UserId string `json:"userId"            gorm:"column:user_id;primary_key;comment:;type:varchar(36);"`

	/** 用户名 */
	UserName string `json:"userName"          gorm:"column:user_name;comment:用户名;type:varchar(128);index;"`

	/** 用户类型 */
	UserType string `json:"userType"          gorm:"column:user_type;comment:用户类型;type:varchar(8)"`

	/** 用户昵称 */
	NickName string `json:"nickName"          gorm:"column:nick_name;comment:用户昵称;type:varchar(128);"`

	/** 手机号 */
	PhoneNumber string `json:"phoneNumber"             gorm:"column:phone_number;comment:手机号;type:varchar(11)"`

	/** 角色id兼容多个角色，用逗号分割 */
	AuthorityId string `json:"authorityId"       gorm:"column:authority_id;comment:角色ID;type:text;"`

	/** 用户所属商户的商户号 */
	MerchantNo string `json:"merchantNo"        gorm:"column:merchant_no;comment:商户号;type:varchar(32);"`

	/** 平台类型 */
	PlatformType int32 `json:"platformType"           gorm:"column:platform_type;comment:平台类型;type:int(3);"`

	/** 应用Id*/
	AppProductId int32 `json:"appProductId"           gorm:"column:app_product_id;comment:应用Id;type:int(3);"`

	/** 自定义扩展，可以为格式化后的json */
	Extend string `json:"extend"            gorm:"column:extend;comment:自定义扩展，可以为格式化后的json;type:text;"`

	/** 有效时间 */
	BufferTime int64 `json:"bufferTime"`

	/** 系统标准Claims */
	jwt.RegisteredClaims `json:"-"        gorm:"-"`
}
