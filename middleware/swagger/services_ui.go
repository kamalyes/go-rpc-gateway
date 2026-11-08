/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\services_ui.go
 * @Description: 服务列表 UI - 服务卡片列表页、单服务 Swagger UI 页面
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"fmt"
	"net/http"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	convert "github.com/kamalyes/go-toolbox/pkg/convert"
)

// handleServiceUI 处理单个服务的 Swagger UI 请求
func (m *Middleware) handleServiceUI(w http.ResponseWriter, r *http.Request) {
	writeHTMLHeaders(w)
	if handleOptions(w, r) {
		return
	}

	if !m.IsAggregateEnabled() {
		http.Error(w, "聚合功能未启用", http.StatusNotFound)
		return
	}

	serviceName := m.extractEntityName(r.URL.Path, constants.PathServicePrefix)
	if serviceName == "" {
		http.Error(w, "服务名称不能为空", http.StatusBadRequest)
		return
	}

	_, err := m.aggregator.GetServiceSpec(serviceName)
	if err != nil {
		http.Error(w, fmt.Sprintf("服务 %s 不存在", serviceName), http.StatusNotFound)
		return
	}

	html := m.generateServiceSwaggerUI(serviceName)
	w.Write([]byte(html))
}

// generateServiceSwaggerUI 生成单个服务的 Swagger UI HTML 页面
func (m *Middleware) generateServiceSwaggerUI(serviceName string) string {
	return m.generateScopedSwaggerUI(
		serviceName,
		fmt.Sprintf("%s API", serviceName),
		"单独服务的 API 文档",
		fmt.Sprintf("%s/services/%s.json?view=ui", m.config.UIPath, serviceName),
		m.commonSwaggerUIActions(),
	)
}

// buildServicesHTML 构建服务列表 HTML 页面
func (m *Middleware) buildServicesHTML(aggregatedSpec map[string]interface{}) string {
	var services []map[string]interface{}

	if aggregateInfo, ok := aggregatedSpec[constants.FieldXAggregateInfo].(map[string]interface{}); ok {
		if servicesList, ok := aggregateInfo[constants.FieldServices].([]interface{}); ok {
			for _, service := range servicesList {
				if serviceMap, ok := service.(map[string]interface{}); ok {
					services = append(services, serviceMap)
				}
			}
		}
	}

	html := `<!DOCTYPE html>
<html lang="` + constants.HTMLLangZH + `">
<head>
    <meta charset="` + constants.HTMLCharset + `">
    <meta name="viewport" content="` + constants.HTMLMetaViewport + `">
    <title>` + m.config.Title + ` - 服务列表</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .header {
            text-align: center;
            margin-bottom: 40px;
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .services-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
            gap: 20px;
        }
        .service-card {
            background: white;
            padding: 25px;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .service-card:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 16px rgba(0,0,0,0.15);
        }
        .service-name {
            font-size: 1.4em;
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .service-desc {
            color: #666;
            margin-bottom: 15px;
            line-height: 1.5;
        }
        .service-version {
            display: inline-block;
            background: #e3f2fd;
            color: #1565c0;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 0.85em;
            font-weight: 500;
            margin-bottom: 15px;
        }
        .service-actions {
            display: flex;
            gap: 10px;
        }
        .btn {
            padding: 8px 16px;
            border: none;
            border-radius: 4px;
            text-decoration: none;
            font-size: 0.9em;
            font-weight: 500;
            cursor: pointer;
            transition: background-color 0.2s;
        }
        .btn-primary {
            background-color: #1976d2;
            color: white;
        }
        .btn-primary:hover {
            background-color: #1565c0;
        }
        .btn-secondary {
            background-color: #f5f5f5;
            color: #555;
            border: 1px solid #ddd;
        }
        .btn-secondary:hover {
            background-color: #e0e0e0;
        }
        .aggregate-actions {
            text-align: center;
            margin: 30px 0;
            padding: 20px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .tags {
            margin-top: 10px;
        }
        .tag {
            display: inline-block;
            background: #f0f0f0;
            color: #666;
            padding: 2px 8px;
            border-radius: 10px;
            font-size: 0.75em;
            margin-right: 5px;
        }
` + authBadgeCSS + `
    </style>
</head>
<body>` + m.authBadgeHTML() + `
    <div class="header">
        <h1>` + m.config.Title + `</h1>
        <p>` + m.config.Description + `</p>
    </div>

    <div class="aggregate-actions">
        <h3>聚合文档</h3>
        <p>查看所有服务的聚合API文档</p>
        <a href="` + m.config.UIPath + `" class="btn btn-primary">查看聚合文档</a>
        <a href="` + m.config.UIPath + `/aggregate.json" class="btn btn-secondary">下载聚合JSON</a>
    </div>

    <div class="services-grid">`

	for _, service := range services {
		name := convert.MustString(service[constants.FieldName])
		description := convert.MustString(service[constants.FieldDescription])
		version := convert.MustString(service[constants.FieldVersion])

		if name == "" {
			continue
		}

		html += `
        <div class="service-card">
            <div class="service-name">` + name + `</div>`

		if description != "" {
			html += `<div class="service-desc">` + description + `</div>`
		}

		if version != "" {
			html += `<div class="service-version">v` + version + `</div>`
		}

		html += `
            <div class="service-actions">
                <a href="` + m.config.UIPath + `/services/` + name + `" class="btn btn-primary">查看文档</a>
                <a href="` + m.config.UIPath + `/services/` + name + `.json" class="btn btn-secondary">下载JSON</a>
            </div>`

		if tags, ok := service[constants.FieldTags].([]interface{}); ok && len(tags) > 0 {
			html += `<div class="tags">`
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok {
					html += `<span class="tag">` + tagStr + `</span>`
				}
			}
			html += `</div>`
		}

		html += `</div>`
	}

	html += `
    </div>
</body>
</html>`

	return html
}
