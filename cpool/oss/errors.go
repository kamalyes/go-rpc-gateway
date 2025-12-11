/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-05 23:00:15
 * @FilePath: \go-rpc-gateway\cpool\oss\errors.go
 * @Description: OSS错误定义
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package oss

import "errors"

var (
	// 配置错误
	ErrOSSConfigNotFound   = errors.New("oss configuration not found")
	ErrMinIOConfigNotFound = errors.New("minio configuration not found")
	ErrS3ConfigNotFound    = errors.New("s3 configuration not found")
	ErrNoOSSConfigured     = errors.New("no oss provider configured")

	// 初始化错误
	ErrMinIOInitFailed          = errors.New("failed to initialize minio client")
	ErrS3InitFailed             = errors.New("failed to initialize s3 client")
	ErrBoltDBPathEmpty          = errors.New("boltdb path is empty")
	ErrBoltDBOpenFailed         = errors.New("failed to open boltdb")
	ErrBoltDBCreateBucketFailed = errors.New("failed to create metadata bucket")
	ErrDisabledConfiguration    = errors.New("disabled configuration")
	ErrUnsupportedOSSType       = errors.New("unsupported oss storage type")

	// Bucket错误
	ErrBucketNotFound      = errors.New("bucket not found")
	ErrBucketAlreadyExists = errors.New("bucket already exists")
	ErrBucketNotEmpty      = errors.New("bucket is not empty")

	// Object错误
	ErrObjectNotFound      = errors.New("object not found")
	ErrObjectAlreadyExists = errors.New("object already exists")
	ErrInvalidObjectKey    = errors.New("invalid object key")

	// 操作错误
	ErrInvalidOperation         = errors.New("invalid operation")
	ErrPermissionDenied         = errors.New("permission denied")
	ErrNetworkError             = errors.New("network error")
	ErrPresignedURLNotSupported = errors.New("presigned url not supported for this storage type")
)
