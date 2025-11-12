/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 13:38:16
 * @FilePath: \go-rpc-gateway\global\global.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package global

import (
	"net/smtp"

	"github.com/bwmarrin/snowflake"
	"github.com/casbin/casbin/v2"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	ut "github.com/go-playground/universal-translator"
	goconfig "github.com/kamalyes/go-config"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/redis/go-redis/v9"

	dysmsapi20170525 "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	aliyunoss "github.com/aliyun/aliyun-oss-go-sdk/oss"
	awss3oss "github.com/aws/aws-sdk-go-v2/service/s3"
	cachex "github.com/kamalyes/go-cachex"
	logger "github.com/kamalyes/go-logger"
	"github.com/minio/minio-go/v7"
	"github.com/spf13/viper"
	bbolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 单节点应用常用全局变量
var (

	// ENV 设置环境
	ENV goconfig.Environment

	// DB 数据库
	DB *gorm.DB

	// REDIS 默认客户端
	REDIS *redis.Client

	// Cachex 默认客户端
	CACHEX cachex.CtxCache

	// ALIYUNOSS 阿里云OSS客户端
	ALIYUNOSS *aliyunoss.Client

	// AWSS3OSS AWS S3客户端
	AWSS3OSS *awss3oss.Client

	// DYSMS 阿里云短信客户端
	DYSMS *dysmsapi20170525.Client

	// BOLTDB BoltDB客户端
	BOLTDB *bbolt.DB

	// MQTT 客户端
	MQTT *mqtt.Client

	// SMTP 客户端
	SMTP *smtp.Client

	// GATEWAY 全局系统配置
	GATEWAY *gwconfig.Gateway

	// VP 通过 viper 读取的yaml配置文件
	VP *viper.Viper

	// LOG 全局日志
	LOG *zap.Logger

	// LOGGER 高性能日志
	LOGGER *logger.Logger

	// CSBEF casbin实施者
	CSBEF casbin.IEnforcer

	// 雪花ID节点
	Node *snowflake.Node

	// MinIO客户端
	MinIO *minio.Client

	// Trans 全局validate翻译器
	Trans ut.Translator

	GPerFix = "goc_"
)
