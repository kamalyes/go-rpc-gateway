/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 15:56:59
 * @FilePath: \im-access-control-service\go-rpc-gateway\cpool\oss\storage.go
 * @Description: 对象存储服务统一接口 - 支持S3/MinIO/阿里云OSS
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package oss

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	ossconfig "github.com/kamalyes/go-config/pkg/oss"
	"github.com/kamalyes/go-logger"
	"github.com/minio/minio-go/v7"
	"io"
	"time"
)

// ObjectInfo 对象信息
type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	ContentType  string
	LastModified time.Time
	Metadata     map[string]string
}

// BucketInfo 存储桶信息
type BucketInfo struct {
	Name         string
	CreationDate time.Time
}

// StorageHandler 对象存储统一接口
type StorageHandler interface {
	// Bucket操作
	ListBuckets(ctx context.Context) ([]BucketInfo, error)
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	CreateBucket(ctx context.Context, bucketName string) error
	DeleteBucket(ctx context.Context, bucketName string) error

	// Object操作
	ListObjects(ctx context.Context, bucketName, prefix string, limit int) ([]ObjectInfo, string, error)
	GetObject(ctx context.Context, bucketName, objectKey string) (*ObjectInfo, error)
	GetObjectBlob(ctx context.Context, bucketName, objectKey string) ([]byte, error)
	PutObject(ctx context.Context, bucketName, objectKey string, data io.Reader, size int64, contentType string) (*ObjectInfo, error)
	DeleteObject(ctx context.Context, bucketName, objectKey string) error

	// 预签名URL
	GetPresignedDownloadURL(ctx context.Context, bucketName, objectKey string, expiry time.Duration) (string, error)
	GetPresignedUploadURL(ctx context.Context, bucketName, objectKey string, expiry time.Duration) (string, error)

	// 关闭连接
	Close() error
}

// MinIOStorage MinIO存储实现
type MinIOStorage struct {
	client *minio.Client
	logger logger.ILogger
}

// NewMinIOStorage 创建MinIO存储
func NewMinIOStorage(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) (*MinIOStorage, error) {
	client := Minio(ctx, cfg, log)
	if client == nil {
		return nil, ErrMinIOInitFailed
	}

	return &MinIOStorage{
		client: client,
		logger: log,
	}, nil
}

// ListBuckets 列出所有存储桶
func (m *MinIOStorage) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	buckets, err := m.client.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]BucketInfo, len(buckets))
	for i, bucket := range buckets {
		result[i] = BucketInfo{
			Name:         bucket.Name,
			CreationDate: bucket.CreationDate,
		}
	}

	return result, nil
}

// BucketExists 检查存储桶是否存在
func (m *MinIOStorage) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	return m.client.BucketExists(ctx, bucketName)
}

// CreateBucket 创建存储桶
func (m *MinIOStorage) CreateBucket(ctx context.Context, bucketName string) error {
	return m.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
}

// DeleteBucket 删除存储桶
func (m *MinIOStorage) DeleteBucket(ctx context.Context, bucketName string) error {
	return m.client.RemoveBucket(ctx, bucketName)
}

// ListObjects 列出对象
func (m *MinIOStorage) ListObjects(ctx context.Context, bucketName, prefix string, limit int) ([]ObjectInfo, string, error) {
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
		MaxKeys:   limit,
	}

	var objects []ObjectInfo
	var nextMarker string

	for object := range m.client.ListObjects(ctx, bucketName, opts) {
		if object.Err != nil {
			return nil, "", object.Err
		}

		objects = append(objects, ObjectInfo{
			Key:          object.Key,
			Size:         object.Size,
			ETag:         object.ETag,
			ContentType:  object.ContentType,
			LastModified: object.LastModified,
		})

		if len(objects) >= limit {
			nextMarker = object.Key
			break
		}
	}

	return objects, nextMarker, nil
}

// GetObject 获取对象信息
func (m *MinIOStorage) GetObject(ctx context.Context, bucketName, objectKey string) (*ObjectInfo, error) {
	stat, err := m.client.StatObject(ctx, bucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Key:          stat.Key,
		Size:         stat.Size,
		ETag:         stat.ETag,
		ContentType:  stat.ContentType,
		LastModified: stat.LastModified,
		Metadata:     stat.UserMetadata,
	}, nil
}

// GetObjectBlob 获取对象数据
func (m *MinIOStorage) GetObjectBlob(ctx context.Context, bucketName, objectKey string) ([]byte, error) {
	object, err := m.client.GetObject(ctx, bucketName, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	return io.ReadAll(object)
}

// PutObject 上传对象
func (m *MinIOStorage) PutObject(ctx context.Context, bucketName, objectKey string, data io.Reader, size int64, contentType string) (*ObjectInfo, error) {
	opts := minio.PutObjectOptions{
		ContentType: contentType,
	}

	info, err := m.client.PutObject(ctx, bucketName, objectKey, data, size, opts)
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Key:         info.Key,
		Size:        info.Size,
		ETag:        info.ETag,
		ContentType: contentType,
	}, nil
}

// DeleteObject 删除对象
func (m *MinIOStorage) DeleteObject(ctx context.Context, bucketName, objectKey string) error {
	return m.client.RemoveObject(ctx, bucketName, objectKey, minio.RemoveObjectOptions{})
}

// GetPresignedDownloadURL 获取下载预签名URL
func (m *MinIOStorage) GetPresignedDownloadURL(ctx context.Context, bucketName, objectKey string, expiry time.Duration) (string, error) {
	url, err := m.client.PresignedGetObject(ctx, bucketName, objectKey, expiry, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// GetPresignedUploadURL 获取上传预签名URL
func (m *MinIOStorage) GetPresignedUploadURL(ctx context.Context, bucketName, objectKey string, expiry time.Duration) (string, error) {
	url, err := m.client.PresignedPutObject(ctx, bucketName, objectKey, expiry)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// Close 关闭连接
func (m *MinIOStorage) Close() error {
	// MinIO客户端不需要显式关闭
	return nil
}

// S3Storage AWS S3存储实现
type S3Storage struct {
	client *s3.Client
	logger logger.ILogger
}

// NewS3Storage 创建S3存储
func NewS3Storage(ctx context.Context, cfg *ossconfig.S3, log logger.ILogger) (*S3Storage, error) {
	// 创建AWS配置
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			cfg.SessionToken,
		)),
	)
	if err != nil {
		return nil, err
	}

	// 创建S3客户端
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.PathStyle
	})

	return &S3Storage{
		client: client,
		logger: log,
	}, nil
}

// ListBuckets 列出所有存储桶
func (s *S3Storage) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	result, err := s.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	buckets := make([]BucketInfo, len(result.Buckets))
	for i, bucket := range result.Buckets {
		buckets[i] = BucketInfo{
			Name:         aws.ToString(bucket.Name),
			CreationDate: aws.ToTime(bucket.CreationDate),
		}
	}

	return buckets, nil
}

// BucketExists 检查存储桶是否存在
func (s *S3Storage) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return false, nil
	}
	return true, nil
}

// CreateBucket 创建存储桶
func (s *S3Storage) CreateBucket(ctx context.Context, bucketName string) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	return err
}

// DeleteBucket 删除存储桶
func (s *S3Storage) DeleteBucket(ctx context.Context, bucketName string) error {
	_, err := s.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	return err
}

// ListObjects 列出对象
func (s *S3Storage) ListObjects(ctx context.Context, bucketName, prefix string, limit int) ([]ObjectInfo, string, error) {
	maxKeys := int32(limit)
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: &maxKeys,
	})
	if err != nil {
		return nil, "", err
	}

	objects := make([]ObjectInfo, len(result.Contents))
	for i, obj := range result.Contents {
		objects[i] = ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			ETag:         aws.ToString(obj.ETag),
			LastModified: aws.ToTime(obj.LastModified),
		}
	}

	nextMarker := ""
	if result.NextContinuationToken != nil {
		nextMarker = aws.ToString(result.NextContinuationToken)
	}

	return objects, nextMarker, nil
}

// GetObject 获取对象信息
func (s *S3Storage) GetObject(ctx context.Context, bucketName, objectKey string) (*ObjectInfo, error) {
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, err
	}

	metadata := make(map[string]string)
	for k, v := range result.Metadata {
		metadata[k] = v
	}

	return &ObjectInfo{
		Key:          objectKey,
		Size:         aws.ToInt64(result.ContentLength),
		ETag:         aws.ToString(result.ETag),
		ContentType:  aws.ToString(result.ContentType),
		LastModified: aws.ToTime(result.LastModified),
		Metadata:     metadata,
	}, nil
}

// GetObjectBlob 获取对象数据
func (s *S3Storage) GetObjectBlob(ctx context.Context, bucketName, objectKey string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()

	return io.ReadAll(result.Body)
}

// PutObject 上传对象
func (s *S3Storage) PutObject(ctx context.Context, bucketName, objectKey string, data io.Reader, size int64, contentType string) (*ObjectInfo, error) {
	result, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucketName),
		Key:           aws.String(objectKey),
		Body:          data,
		ContentLength: &size,
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return nil, err
	}

	return &ObjectInfo{
		Key:         objectKey,
		Size:        size,
		ETag:        aws.ToString(result.ETag),
		ContentType: contentType,
	}, nil
}

// DeleteObject 删除对象
func (s *S3Storage) DeleteObject(ctx context.Context, bucketName, objectKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	return err
}

// GetPresignedDownloadURL 获取下载预签名URL
func (s *S3Storage) GetPresignedDownloadURL(ctx context.Context, bucketName, objectKey string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)
	result, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// GetPresignedUploadURL 获取上传预签名URL
func (s *S3Storage) GetPresignedUploadURL(ctx context.Context, bucketName, objectKey string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)
	result, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// Close 关闭S3连接
func (s *S3Storage) Close() error {
	return nil
}

// NewStorage 根据配置创建存储实例
func NewStorage(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) (StorageHandler, error) {
	if cfg.OSS == nil {
		return nil, ErrOSSConfigNotFound
	}

	// 优先使用MinIO配置
	if cfg.OSS.Minio != nil && cfg.OSS.Minio.Endpoint != "" {
		return NewMinIOStorage(ctx, cfg, log)
	}

	// 其次使用S3配置
	if cfg.OSS.S3 != nil && cfg.OSS.S3.Endpoint != "" {
		return NewS3Storage(ctx, cfg.OSS.S3, log)
	}

	// 最后使用BoltDB本地存储
	if cfg.OSS.BoltDB != nil && cfg.OSS.BoltDB.Path != "" {
		return NewBoltDBStorage(ctx, cfg.OSS.BoltDB, log)
	}

	return nil, ErrNoOSSConfigured
}
