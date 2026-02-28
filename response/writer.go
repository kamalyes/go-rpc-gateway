/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 10:11:54
 * @FilePath: \go-rpc-gateway\response\writer.go
 * @Description: HTTP响应写入核心函数
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package response

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
)

// jsonEncoderPool JSON 编码器对象池
var jsonEncoderPool = sync.Pool{
	New: func() any {
		return json.NewEncoder(io.Discard)
	},
}

// WriteResult 写入标准化Result响应
func WriteResult(w http.ResponseWriter, httpStatus int, result *commonapis.Result) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(httpStatus)

	encoder := jsonEncoderPool.Get().(*json.Encoder)
	defer jsonEncoderPool.Put(encoder)

	// 创建新的 encoder 指向当前 writer
	*encoder = *json.NewEncoder(w)

	if err := encoder.Encode(result); err != nil {
		global.LOGGER.WithError(err).ErrorMsg("Failed to encode Result response")
	}
}

// WriteJSONResponse 写入自定义JSON响应
func WriteJSONResponse(w http.ResponseWriter, httpStatus int, data any) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(httpStatus)

	encoder := jsonEncoderPool.Get().(*json.Encoder)
	defer jsonEncoderPool.Put(encoder)

	// 创建新的 encoder 指向当前 writer
	*encoder = *json.NewEncoder(w)

	if err := encoder.Encode(data); err != nil {
		global.LOGGER.WithError(err).ErrorMsg("Failed to encode JSON response")
	}
}
