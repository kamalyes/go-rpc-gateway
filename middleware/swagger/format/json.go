/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\format\json.go
 * @Description: 规范序列化工具 - 将规范 map 序列化为 JSON
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package format

import (
	"encoding/json"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	serrors "github.com/kamalyes/go-rpc-gateway/middleware/swagger/errors"
	errorx "github.com/kamalyes/go-toolbox/pkg/errorx"
	mathx "github.com/kamalyes/go-toolbox/pkg/mathx"
)

// MarshalSpec 将规范序列化为 JSON
func MarshalSpec(spec map[string]interface{}) ([]byte, error) {
	convertedSpec := mathx.ConvertMapKeysToString(spec)
	jsonData, err := json.MarshalIndent(convertedSpec, constants.JSONIndentPrefix, constants.JSONIndentValue)
	if err != nil {
		return nil, errorx.NewError(serrors.ErrTypeSerializeFailed, err)
	}
	return jsonData, nil
}
