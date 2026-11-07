/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\documents_ui.go
 * @Description: 独立子文档 UI - 文档列表页、文档 Swagger UI 页面
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"fmt"
	"net/http"
	"strings"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	serrors "github.com/kamalyes/go-rpc-gateway/middleware/swagger/errors"
	errorx "github.com/kamalyes/go-toolbox/pkg/errorx"
)

// handleDocumentUI 处理独立文档 UI 请求
func (m *Middleware) handleDocumentUI(w http.ResponseWriter, r *http.Request) {
	writeHTMLHeaders(w)
	if handleOptions(w, r) {
		return
	}

	if !m.IsAggregateEnabled() {
		http.Error(w, "聚合功能未启用", http.StatusNotFound)
		return
	}

	documentName := m.extractEntityName(r.URL.Path, constants.PathDocumentPrefix)
	if documentName == "" {
		http.Error(w, "文档名称不能为空", http.StatusBadRequest)
		return
	}

	documentSpecs := m.docBuilder.GetDocumentSpecs()
	spec, exists := documentSpecs[documentName]
	if !exists {
		http.Error(w, fmt.Sprintf("文档 %s 不存在", documentName), http.StatusNotFound)
		return
	}

	title := m.docBuilder.ResolveSpecTitle(spec, documentName)
	w.Write([]byte(m.generateDocumentSwaggerUI(documentName, title)))
}

// handleDocumentsIndex 处理独立文档列表页
func (m *Middleware) handleDocumentsIndex(w http.ResponseWriter, _ *http.Request) {
	if !m.IsAggregateEnabled() {
		m.errorResponseFn(w, http.StatusNotFound, "聚合功能未启用")
		return
	}

	writeHTMLHeaders(w)
	w.Write([]byte(m.buildDocumentsHTML()))
}

// handleDocumentJSON 处理独立文档 JSON 请求
func (m *Middleware) handleDocumentJSON(w http.ResponseWriter, r *http.Request) {
	writeJSONHeaders(w)
	if handleOptions(w, r) {
		return
	}

	if !m.IsAggregateEnabled() {
		m.errorResponseFn(w, http.StatusNotFound, "聚合功能未启用")
		return
	}

	documentName := m.extractEntityName(r.URL.Path, constants.PathDocumentPrefix)
	if documentName == "" {
		m.errorResponseFn(w, http.StatusBadRequest, "文档名称不能为空")
		return
	}

	jsonData, err := m.docBuilder.GetDocumentSpec(documentName)
	if err != nil {
		m.logger.Error("获取独立文档 %s 的规范失败: %v", documentName, err)
		m.errorResponseFn(w, http.StatusNotFound, fmt.Sprintf("文档 %s 的规范不存在", documentName))
		return
	}

	m.serveSpecJSON(w, r, jsonData)
}

// GetDocumentSpec 获取独立文档规范 JSON
func (m *Middleware) GetDocumentSpec(documentName string) ([]byte, error) {
	if !m.config.IsAggregateEnabled() {
		return nil, errorx.NewError(serrors.ErrTypeAggregateDisabled)
	}
	return m.docBuilder.GetDocumentSpec(documentName)
}

// generateDocumentSwaggerUI 生成独立文档 Swagger UI 页面
func (m *Middleware) generateDocumentSwaggerUI(documentName, title string) string {
	return m.generateScopedSwaggerUI(
		title,
		title,
		"独立子文档 API 视图",
		fmt.Sprintf("%s/documents/%s.json?view=ui", m.config.UIPath, documentName),
		m.commonSwaggerUIActions(),
	)
}

// buildDocumentsHTML 构建独立文档列表 HTML 页面
func (m *Middleware) buildDocumentsHTML() string {
	summaries := m.docBuilder.GetDocumentSummaries()

	html := `<!DOCTYPE html>
<html lang="` + constants.HTMLLangZH + `">
<head>
    <meta charset="` + constants.HTMLCharset + `">
    <meta name="viewport" content="` + constants.HTMLMetaViewport + `">
    <title>` + m.config.Title + ` - 独立文档列表</title>
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
            margin-bottom: 30px;
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .top-actions {
            text-align: center;
            margin: 30px 0;
            padding: 20px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .documents-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
            gap: 20px;
        }
        .document-card {
            background: white;
            padding: 24px;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .document-name {
            font-size: 1.35em;
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .document-desc {
            color: #666;
            margin-bottom: 15px;
            line-height: 1.5;
        }
        .document-version {
            display: inline-block;
            background: #e8f5e9;
            color: #2e7d32;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 0.85em;
            font-weight: 500;
            margin-bottom: 12px;
        }
        .document-services {
            color: #555;
            margin-bottom: 16px;
            line-height: 1.5;
        }
        .document-actions {
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
        }
        .btn-primary {
            background-color: #1976d2;
            color: white;
        }
        .btn-secondary {
            background-color: #f5f5f5;
            color: #555;
            border: 1px solid #ddd;
        }
        .empty {
            background: white;
            padding: 32px;
            border-radius: 8px;
            text-align: center;
            color: #666;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>` + m.config.Title + `</h1>
        <p>按 path + method 组合出来的独立 Swagger 子文档</p>
    </div>

    <div class="top-actions">
        <a href="` + m.config.UIPath + `" class="btn btn-primary">查看聚合文档</a>
        <a href="` + m.config.UIPath + `/services" class="btn btn-secondary">查看服务列表</a>
        <a href="` + m.config.UIPath + `/aggregate.json" class="btn btn-secondary">下载聚合 JSON</a>
    </div>`

	if len(summaries) == 0 {
		html += `
    <div class="empty">
        当前没有可用的独立文档，请检查 swagger.aggregate.documents 配置。
    </div>
</body>
</html>`
		return html
	}

	html += `
    <div class="documents-grid">`

	for _, summary := range summaries {
		html += `
        <div class="document-card">
            <div class="document-name">` + summary.Title + `</div>`

		if summary.Description != "" {
			html += `<div class="document-desc">` + summary.Description + `</div>`
		}

		if summary.Version != "" {
			html += `<div class="document-version">v` + summary.Version + `</div>`
		}

		if len(summary.Services) > 0 {
			html += `<div class="document-services">来源服务: ` + strings.Join(summary.Services, ", ") + `</div>`
		}

		html += `
            <div class="document-actions">
                <a href="` + m.config.UIPath + `/documents/` + summary.Name + `" class="btn btn-primary">查看文档</a>
                <a href="` + m.config.UIPath + `/documents/` + summary.Name + `.json" class="btn btn-secondary">下载 JSON</a>
            </div>
        </div>`
	}

	html += `
    </div>
</body>
</html>`

	return html
}
