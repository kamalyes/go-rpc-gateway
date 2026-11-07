/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\ui.go
 * @Description: Swagger UI 渲染 - 根页面、通用作用域 UI、导航链接
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"fmt"
	"strings"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
)

// generateRootSwaggerUI 生成根 Swagger UI 页面
func (m *Middleware) generateRootSwaggerUI() string {
	return m.generateScopedSwaggerUI(
		m.config.Title,
		m.config.Title,
		m.config.Description,
		m.config.UIPath+constants.JSONPath+"?view=ui",
		m.commonSwaggerUIActions(),
	)
}

// generateScopedSwaggerUI 生成通用作用域 Swagger UI 页面
// title: 页面标题, heading: 页面大标题, description: 描述, specURL: 规范文件URL, links: 导航链接
func (m *Middleware) generateScopedSwaggerUI(title, heading, description, specURL string, links []UIAction) string {
	var linksHTML strings.Builder
	for _, link := range links {
		linksHTML.WriteString(fmt.Sprintf(`
        <a href="%s">%s</a>`, link.Href, link.Label))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="`+constants.HTMLLangEN+`">
<head>
    <meta charset="`+constants.HTMLCharset+`">
    <meta name="viewport" content="`+constants.HTMLMetaViewport+`">
    <title>%s - API Documentation</title>
    <link rel="stylesheet" type="text/css" href="`+m.config.GetCDNCSSURL()+`" />
    <link rel="icon" type="image/png" href="`+m.config.GetCDNFavicon32()+`" sizes="`+constants.HTMLIconSizes32+`" />
    <link rel="icon" type="image/png" href="`+m.config.GetCDNFavicon16()+`" sizes="`+constants.HTMLIconSizes16+`" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin: 0;
            background: #fafafa;
        }
        .scoped-header {
            background: #fff;
            border-bottom: 1px solid #e8e8e8;
            padding: 20px;
            text-align: center;
        }
        .scoped-header h1 {
            margin: 0 0 10px 0;
            font-size: 1.8em;
            color: #3b4151;
        }
        .scoped-header p {
            margin: 5px 0 15px 0;
            color: #666;
        }
        .scoped-header a {
            display: inline-block;
            margin: 0 5px;
            padding: 8px 16px;
            background: #4990e2;
            color: white;
            text-decoration: none;
            border-radius: 4px;
            font-size: 14px;
        }
        .scoped-header a:hover {
            background: #3b7bbf;
        }
    </style>
</head>
<body>
    <div class="scoped-header">
        <h1>%s</h1>
        <p>%s</p>%s
    </div>

    <div id="swagger-ui"></div>
    <script src="`+m.config.GetCDNBundleJS()+`" charset="`+constants.HTMLCharset+`"></script>
    <script src="`+m.config.GetCDNPresetJS()+`" charset="`+constants.HTMLCharset+`"></script>
    <script>
    window.onload = function() {
        window.ui = SwaggerUIBundle({
            url: '%s',
            dom_id: '`+constants.UIDomID+`',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIStandalonePreset
            ],
            plugins: [
                SwaggerUIBundle.plugins.DownloadUrl
            ],
            layout: "`+constants.UILayout+`"
        });
    };
    </script>
</body>
</html>`, title, heading, description, linksHTML.String(), specURL)
}

// commonSwaggerUIActions 返回聚合模式下的通用导航链接
func (m *Middleware) commonSwaggerUIActions() []UIAction {
	if !m.IsAggregateEnabled() {
		return nil
	}

	return []UIAction{
		{Href: m.config.UIPath + "/documents", Label: "返回文档列表"},
		{Href: m.config.UIPath + "/services", Label: "查看服务列表"},
		{Href: m.config.UIPath, Label: "查看聚合文档"},
	}
}
