/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\ui_test.go
 * @Description: Swagger UI 页面渲染测试 - 样式表完整性与格式串转义
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"strings"
	"testing"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
)

// TestGenerateScopedSwaggerUIEscapesPercent 页面模板整体是 fmt.Sprintf 的格式串，
// CSS 里的百分号漏写成单个 % 会被当成动词，渲染出 %!;(MISSING) 之类的碎片，
// 同时把后面所有占位符的实参顺序一起错开
//
// [EN] The page template is one fmt.Sprintf format string, so a single % in the CSS is read as
// a verb: it renders fragments like %!;(MISSING) and shifts every later placeholder's argument.
func TestGenerateScopedSwaggerUIEscapesPercent(t *testing.T) {
	m := &Middleware{config: goswagger.Default()}

	html := m.generateScopedSwaggerUI("标题", "大标题", "描述", "/swagger/doc.json?view=ui", nil)

	if strings.Contains(html, "%!") {
		t.Errorf("页面含 fmt 转换错误标记:\n%s", html)
	}
	for _, want := range []string{
		"<title>标题 - API Documentation</title>",
		"<h1>大标题</h1>",
		"<p>描述</p>",
		"url: '/swagger/doc.json?view=ui'",
		"width: 100%;",
		"max-width: 100%;",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("页面缺少 %q", want)
		}
	}
}

// TestGenerateScopedSwaggerUIStylesDescriptionMarkdown 描述区表格与行内代码需要自带样式，
// Swagger UI 原生 CSS 不覆盖 .renderedMarkdown 里的内容
func TestGenerateScopedSwaggerUIStylesDescriptionMarkdown(t *testing.T) {
	m := &Middleware{config: goswagger.Default()}

	html := m.generateScopedSwaggerUI("t", "h", "d", "/spec.json", nil)

	for _, want := range []string{
		// Swagger 2.0 的描述用 .markdown，OAS3 用 .renderedMarkdown，两套都要覆盖
		// [EN] Swagger 2.0 descriptions use .markdown, OAS3 uses .renderedMarkdown; cover both
		".swagger-ui .markdown table,",
		".swagger-ui .renderedMarkdown table",
		".swagger-ui .markdown code,",
		".swagger-ui .renderedMarkdown code",
		".swagger-ui .markdown pre > code,",
		// 第一列要压过 swagger-ui 的 td:first-of-type，选择器深度不能少
		// [EN] The first column must outrank swagger-ui's td:first-of-type, so keep the depth
		".swagger-ui .markdown table tbody tr td:first-child,",
		// 短列的宽度下限：没有它，位置/类型/必填会被说明列压成紧贴的一团
		// [EN] The floor for the short columns; without it 位置/类型/必填 collapse into a clump
		"min-width: 76px;",
		"min-width: 220px;",
		"overflow-wrap: break-word;",
		"overflow-wrap: anywhere;",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("样式表缺少 %q", want)
		}
	}
}

// TestDescriptionCSSCoversBothMarkdownClasses 每条规则都必须同时覆盖两套 class
//
// 回归点：只写 .renderedMarkdown 时，Swagger 2.0 的规范一条规则都不生效——表格没边框、
// 单元格横向内边距为 0，而官方 .markdown code 的紫色样式照常命中，看上去像只有代码高亮生效
//
// [EN] Regression: with only .renderedMarkdown, a Swagger 2.0 spec matched nothing — no borders
// and zero horizontal cell padding — while the stock .markdown code rule still applied, which
// reads as if only the code highlighting had taken effect.
func TestDescriptionCSSCoversBothMarkdownClasses(t *testing.T) {
	markdown := strings.Count(descriptionMarkdownCSS, ".swagger-ui .markdown")
	rendered := strings.Count(descriptionMarkdownCSS, ".swagger-ui .renderedMarkdown")

	if markdown == 0 || markdown != rendered {
		t.Errorf("两套 class 的规则数不一致: .markdown=%d, .renderedMarkdown=%d", markdown, rendered)
	}
}

// TestDescriptionCSSNeedsNoPercentEscaping CSS 走 %s 实参传入，里面的百分号应保持字面量
//
// 常量拼接救不了这个坑：Go 在编译期把常量合成同一个字符串字面量，拼进模板后还是格式串
// [EN] Concatenation cannot fix this: Go folds constants into one literal at compile time, so a
// spliced constant is still part of the format string. Only an argument keeps the percent signs.
func TestDescriptionCSSNeedsNoPercentEscaping(t *testing.T) {
	if strings.Contains(descriptionMarkdownCSS, "%%") {
		t.Error("CSS 作为实参传入，不应再写 %% 转义")
	}
	if !strings.Contains(descriptionMarkdownCSS, "width: 100%;") {
		t.Error("CSS 缺少字面量百分号宽度")
	}
}
