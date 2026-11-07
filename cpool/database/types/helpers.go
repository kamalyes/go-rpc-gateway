/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-08 15:00:00
 * @FilePath: \go-rpc-gateway\cpool\database\types\helpers.go
 * @Description: 内部辅助函数
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package types

import "fmt"

// toBytes 将数据库值统一转换为 []byte
func toBytes(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unsupported scan type: %T", value)
	}
}
