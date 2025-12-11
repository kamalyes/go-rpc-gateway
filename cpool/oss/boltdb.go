/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-05 23:05:02
 * @FilePath: \go-rpc-gateway\cpool\oss\boltdb.go
 * @Description: BoltDB对象存储实现 - 本地文件存储
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package oss

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	ossconfig "github.com/kamalyes/go-config/pkg/oss"
	"github.com/kamalyes/go-logger"
	bolt "go.etcd.io/bbolt"
	"io"
	"net/http"
	"time"
)

const (
	MetadataBucketKey = "metadata/buckets"
	MetadataObjectKey = "metadata/objects"
	BlobObjectKey     = "blob/objects"
)

// BoltDBStorage BoltDB存储实现
type BoltDBStorage struct {
	db     *bolt.DB
	logger logger.ILogger
}

// NewBoltDBStorage 创建BoltDB存储
func NewBoltDBStorage(ctx context.Context, cfg *ossconfig.BoltDB, log logger.ILogger) (*BoltDBStorage, error) {
	// 打开BoltDB数据库
	db, err := bolt.Open(cfg.Path, 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		log.ErrorContextKV(ctx, "Failed to open BoltDB", "path", cfg.Path, "error", err)
		return nil, ErrBoltDBOpenFailed
	}

	// 创建元数据bucket
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(MetadataBucketKey))
		return err
	})
	if err != nil {
		db.Close()
		log.ErrorContextKV(ctx, "Failed to create metadata bucket", "error", err)
		return nil, ErrBoltDBCreateBucketFailed
	}

	log.InfoContextKV(ctx, "BoltDB storage initialized successfully", "path", cfg.Path)

	return &BoltDBStorage{
		db:     db,
		logger: log,
	}, nil
}

// ListBuckets 列出所有存储桶
func (b *BoltDBStorage) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	var buckets []BucketInfo

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(MetadataBucketKey))
		if bucket == nil {
			return nil
		}

		return bucket.ForEach(func(k, v []byte) error {
			var info BucketInfo
			if err := jsoniter.Unmarshal(v, &info); err != nil {
				return err
			}
			buckets = append(buckets, info)
			return nil
		})
	})

	return buckets, err
}

// BucketExists 检查存储桶是否存在
func (b *BoltDBStorage) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	exists := false

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(MetadataBucketKey))
		if bucket == nil {
			return nil
		}

		v := bucket.Get([]byte(bucketName))
		if len(v) > 0 {
			exists = true
		}
		return nil
	})

	return exists, err
}

// CreateBucket 创建存储桶
func (b *BoltDBStorage) CreateBucket(ctx context.Context, bucketName string) error {
	info := BucketInfo{
		Name:         bucketName,
		CreationDate: time.Now(),
	}

	val, err := jsoniter.Marshal(info)
	if err != nil {
		return err
	}

	dbObjBucket := fmt.Sprintf("%s/%s", MetadataObjectKey, bucketName)
	dbBlobBucket := fmt.Sprintf("%s/%s", BlobObjectKey, bucketName)

	return b.db.Update(func(tx *bolt.Tx) error {
		// 创建元数据bucket
		metaBucket := tx.Bucket([]byte(MetadataBucketKey))
		if metaBucket == nil {
			return ErrBucketNotFound
		}

		// 创建对象元数据bucket
		if _, err := tx.CreateBucket([]byte(dbObjBucket)); err != nil {
			return err
		}

		// 创建对象数据bucket
		if _, err := tx.CreateBucket([]byte(dbBlobBucket)); err != nil {
			return err
		}

		// 保存bucket信息
		return metaBucket.Put([]byte(bucketName), val)
	})
}

// DeleteBucket 删除存储桶
func (b *BoltDBStorage) DeleteBucket(ctx context.Context, bucketName string) error {
	dbObjBucket := fmt.Sprintf("%s/%s", MetadataObjectKey, bucketName)
	dbBlobBucket := fmt.Sprintf("%s/%s", BlobObjectKey, bucketName)

	return b.db.Update(func(tx *bolt.Tx) error {
		// 删除对象元数据bucket
		if err := tx.DeleteBucket([]byte(dbObjBucket)); err != nil {
			return err
		}

		// 删除对象数据bucket
		if err := tx.DeleteBucket([]byte(dbBlobBucket)); err != nil {
			return err
		}

		// 删除bucket元数据
		metaBucket := tx.Bucket([]byte(MetadataBucketKey))
		if metaBucket == nil {
			return ErrBucketNotFound
		}

		return metaBucket.Delete([]byte(bucketName))
	})
}

// ListObjects 列出对象
func (b *BoltDBStorage) ListObjects(ctx context.Context, bucketName, prefix string, limit int) ([]ObjectInfo, string, error) {
	var objects []ObjectInfo
	dbBucket := fmt.Sprintf("%s/%s", MetadataObjectKey, bucketName)
	count := 0
	nextMarker := ""

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(dbBucket))
		if bucket == nil {
			return ErrBucketNotFound
		}

		c := bucket.Cursor()
		for k, v := c.Seek([]byte(prefix)); k != nil && count < limit; k, v = c.Next() {
			if bytes.Compare(k, []byte(prefix)) < 0 {
				continue
			}

			var obj ObjectInfo
			if err := jsoniter.Unmarshal(v, &obj); err != nil {
				return err
			}

			objects = append(objects, obj)
			nextMarker = obj.Key
			count++
		}

		return nil
	})

	return objects, nextMarker, err
}

// GetObject 获取对象信息
func (b *BoltDBStorage) GetObject(ctx context.Context, bucketName, objectKey string) (*ObjectInfo, error) {
	var obj ObjectInfo
	dbBucket := fmt.Sprintf("%s/%s", MetadataObjectKey, bucketName)

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(dbBucket))
		if bucket == nil {
			return ErrBucketNotFound
		}

		val := bucket.Get([]byte(objectKey))
		if len(val) == 0 {
			return ErrObjectNotFound
		}

		return jsoniter.Unmarshal(val, &obj)
	})

	if err != nil {
		return nil, err
	}

	return &obj, nil
}

// GetObjectBlob 获取对象数据
func (b *BoltDBStorage) GetObjectBlob(ctx context.Context, bucketName, objectKey string) ([]byte, error) {
	var blob []byte
	dbBucket := fmt.Sprintf("%s/%s", BlobObjectKey, bucketName)

	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(dbBucket))
		if bucket == nil {
			return ErrBucketNotFound
		}

		data := bucket.Get([]byte(objectKey))
		if len(data) == 0 {
			return ErrObjectNotFound
		}

		// 复制数据,因为bolt的数据在事务外无效
		blob = make([]byte, len(data))
		copy(blob, data)

		return nil
	})

	return blob, err
}

// PutObject 上传对象
func (b *BoltDBStorage) PutObject(ctx context.Context, bucketName, objectKey string, data io.Reader, size int64, contentType string) (*ObjectInfo, error) {
	// 读取数据
	blob, err := io.ReadAll(data)
	if err != nil {
		return nil, err
	}

	// 检测Content-Type
	if contentType == "" {
		contentType = http.DetectContentType(blob)
	}

	// 计算MD5
	md5Hash := md5.New()
	md5Hash.Write(blob)
	etag := hex.EncodeToString(md5Hash.Sum(nil))

	// 创建对象信息
	obj := ObjectInfo{
		Key:          objectKey,
		Size:         int64(len(blob)),
		ETag:         etag,
		ContentType:  contentType,
		LastModified: time.Now(),
		Metadata:     make(map[string]string),
	}

	// 保存到数据库
	objVal, err := jsoniter.Marshal(obj)
	if err != nil {
		return nil, err
	}

	dbObjBucket := fmt.Sprintf("%s/%s", MetadataObjectKey, bucketName)
	dbBlobBucket := fmt.Sprintf("%s/%s", BlobObjectKey, bucketName)

	err = b.db.Update(func(tx *bolt.Tx) error {
		// 保存元数据
		objBucket := tx.Bucket([]byte(dbObjBucket))
		if objBucket == nil {
			return ErrBucketNotFound
		}

		if err := objBucket.Put([]byte(objectKey), objVal); err != nil {
			return err
		}

		// 保存数据
		blobBucket := tx.Bucket([]byte(dbBlobBucket))
		if blobBucket == nil {
			return ErrBucketNotFound
		}

		return blobBucket.Put([]byte(objectKey), blob)
	})

	if err != nil {
		return nil, err
	}

	return &obj, nil
}

// DeleteObject 删除对象
func (b *BoltDBStorage) DeleteObject(ctx context.Context, bucketName, objectKey string) error {
	dbObjBucket := fmt.Sprintf("%s/%s", MetadataObjectKey, bucketName)
	dbBlobBucket := fmt.Sprintf("%s/%s", BlobObjectKey, bucketName)

	return b.db.Update(func(tx *bolt.Tx) error {
		// 删除元数据
		objBucket := tx.Bucket([]byte(dbObjBucket))
		if objBucket == nil {
			return ErrBucketNotFound
		}

		if err := objBucket.Delete([]byte(objectKey)); err != nil {
			return err
		}

		// 删除数据
		blobBucket := tx.Bucket([]byte(dbBlobBucket))
		if blobBucket == nil {
			return ErrBucketNotFound
		}

		return blobBucket.Delete([]byte(objectKey))
	})
}

// GetPresignedDownloadURL 获取下载预签名URL (BoltDB不支持)
func (b *BoltDBStorage) GetPresignedDownloadURL(ctx context.Context, bucketName, objectKey string, expiry time.Duration) (string, error) {
	return "", ErrPresignedURLNotSupported
}

// GetPresignedUploadURL 获取上传预签名URL (BoltDB不支持)
func (b *BoltDBStorage) GetPresignedUploadURL(ctx context.Context, bucketName, objectKey string, expiry time.Duration) (string, error) {
	return "", ErrPresignedURLNotSupported
}

// Close 关闭数据库
func (b *BoltDBStorage) Close() error {
	if b.db != nil {
		ctx := context.Background()
		if err := b.db.Close(); err != nil {
			if b.logger != nil {
				b.logger.ErrorContextKV(ctx, "Failed to close BoltDB", "error", err)
			}
			return err
		}
		if b.logger != nil {
			b.logger.InfoContext(ctx, "BoltDB storage closed successfully")
		}
	}
	return nil
}
