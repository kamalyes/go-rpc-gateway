/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\loader\url.go
 * @Description: URL 规范加载器 - 从远程 HTTP/HTTPS 地址加载 Swagger 规范
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package loader

import (
	"encoding/json"
	"io"
	"net/http"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	serrors "github.com/kamalyes/go-rpc-gateway/middleware/swagger/errors"
	errorx "github.com/kamalyes/go-toolbox/pkg/errorx"
	stringx "github.com/kamalyes/go-toolbox/pkg/stringx"
	"gopkg.in/yaml.v3"
)

// LoadSpecFromURL 从远程 URL 加载 Swagger 规范
// 根据 Content-Type 或 URL 扩展名自动判断 JSON/YAML 格式
func LoadSpecFromURL(client *http.Client, url string) (map[string]interface{}, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, errorx.NewError(serrors.ErrTypeLoaderHTTPFailed, url)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorx.NewError(serrors.ErrTypeLoaderHTTPFailed, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errorx.NewError(serrors.ErrTypeLoaderReadFailed, url)
	}

	var spec map[string]interface{}
	contentType := resp.Header.Get(constants.HeaderContentType)

	if stringx.ContainsAny(contentType, []string{constants.MimeYAML, constants.MimeYML}) ||
		stringx.EndWithAnyIgnoreCase(url, []string{constants.FileExtYAML, constants.FileExtYML}) {
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return nil, errorx.NewError(serrors.ErrTypeLoaderParseFailed, url)
		}
	} else {
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, errorx.NewError(serrors.ErrTypeLoaderParseFailed, url)
		}
	}

	return spec, nil
}
