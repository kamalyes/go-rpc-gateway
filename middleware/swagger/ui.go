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

// descriptionMarkdownCSS 描述区 Markdown 的渲染样式
//
// 为什么两套 class 都要写：Swagger UI 的 Markdown 组件有两份实现，OAS3 那份输出
// class="renderedMarkdown"，Swagger 2.0 那份输出 class="markdown"本仓库的规范是
// 2.0，只写 renderedMarkdown 等于一条规则都不生效——表格没边框、单元格没横向内边距，
// 而官方 .markdown code 的紫色样式照常命中，看上去像「只有代码高亮生效了」
//
// 为什么选择器层级这么深：Swagger UI 自带 `.swagger-ui table tbody tr td{padding:10px 0 0}`
// 和 `td:first-of-type{min-width:6em;padding:10px 0}`，横向内边距是 0，列与列直接贴在一起
// 要压过它们就得在特指度上赢：带伪类的那条算两个类 + 四个元素，所以第一列的规则必须写到
// `.swagger-ui .markdown table tbody tr td:first-child` 这个深度，少一层就被官方样式盖掉
//
// 为什么作为 %s 实参传入而不是拼进模板：页面模板整体是 fmt.Sprintf 的格式串，CSS 里的
// width: 100% 会被当成动词，渲染出 %!;(MISSING) 并把后面所有占位符的实参顺序一起错开
// 常量拼接救不了——Go 在编译期就把它们合成同一个字符串字面量，拼出来的还是那个格式串
// 只有走实参，这里的百分号才真正是字面量
//
// [EN] Both class names are needed: Swagger UI ships two Markdown implementations, and the OAS3
// one emits class="renderedMarkdown" while the Swagger 2.0 one emits class="markdown". These
// specs are 2.0, so targeting only renderedMarkdown matches nothing — no borders, no horizontal
// cell padding — while the stock .markdown code rule still applies, which reads as "only the
// code highlighting took effect".
//
// The selectors are deep on purpose. Swagger UI ships `.swagger-ui table tbody tr td` with
// `padding:10px 0 0` and a `td:first-of-type` rule carrying `min-width:6em`, both with zero
// horizontal padding, which is what glues the columns together. Winning means winning on
// specificity: the rule with the pseudo-class counts as two classes and four elements, so the
// first-column rule has to reach `.swagger-ui .markdown table tbody tr td:first-child`. One
// level shallower and the stock style takes it back.
//
// It is passed in as a %s argument rather than spliced into the template because the page
// template as a whole is a fmt.Sprintf format string, where `width: 100%` is read as a verb: it
// renders %!;(MISSING) and shifts every later placeholder's argument. Concatenation does not
// help — Go folds constants into one string literal at compile time, so the result is still that
// same format string. Only an argument makes these percent signs literal.
const descriptionMarkdownCSS = `
        .swagger-ui .markdown table,
        .swagger-ui .renderedMarkdown table {
            width: 100%;
            max-width: 100%;
            border-collapse: collapse;
            margin: 10px 0 16px;
            font-size: 13px;
        }
        .swagger-ui .markdown table thead tr th,
        .swagger-ui .markdown table tbody tr td,
        .swagger-ui .renderedMarkdown table thead tr th,
        .swagger-ui .renderedMarkdown table tbody tr td {
            border: 1px solid #e3e8ee;
            padding: 9px 14px;
            text-align: left;
            vertical-align: top;
            font-size: 13px;
            line-height: 1.75;
            min-width: 76px;
            /* 这里只用 break-word：它在「整个词都放不下」时才断，不影响列的最小内容宽度
             * anywhere 会把最小内容宽度直接压到一个字符，类型列的 array<commonLocalizedText>
             * 就会被竖着劈成五行真正需要兜底的只有末列，单独给
             *
             * [EN] break-word only: it breaks when a whole word cannot fit and does not shrink
             * the column's min-content width. anywhere drops that width to a single character,
             * which slices array<commonLocalizedText> in the type column into five stacked
             * fragments. Only the last column needs the stronger rule, and it gets it below. */
            word-break: normal;
            overflow-wrap: break-word;
        }
        .swagger-ui .markdown table thead tr th,
        .swagger-ui .renderedMarkdown table thead tr th {
            background: #f4f6f8;
            color: #3b4151;
            font-weight: 600;
            white-space: nowrap;
        }
        /* 第一列是字段名，短且固定，不跟着说明一起换行
         * [EN] The first column holds field names: short and fixed, so keep it off the wrap */
        .swagger-ui .markdown table tbody tr td:first-child,
        .swagger-ui .renderedMarkdown table tbody tr td:first-child {
            padding: 9px 14px;
            min-width: 76px;
            white-space: nowrap;
        }
        /* 末列（说明）吃掉剩余宽度，其余列靠上面的下限各自留白
         * [EN] The last column absorbs the remaining width; the floor above keeps the rest apart */
        .swagger-ui .markdown table tbody tr td:last-child,
        .swagger-ui .renderedMarkdown table tbody tr td:last-child {
            width: 100%;
            min-width: 220px;
            white-space: normal;
            /* 末列有 width:100%，不怕被压窄，这里才用得起最强的断行兜底
             * [EN] The last column carries width:100%, so it cannot be squeezed; this is where
             * the strongest break rule belongs */
            overflow-wrap: anywhere;
        }
        .swagger-ui .markdown table tbody tr:nth-child(even) td,
        .swagger-ui .renderedMarkdown table tbody tr:nth-child(even) td {
            background: #fbfcfd;
        }
        .swagger-ui .markdown table tbody tr:hover td,
        .swagger-ui .renderedMarkdown table tbody tr:hover td {
            background: #f2f7fd;
        }

        .swagger-ui .markdown code,
        .swagger-ui .renderedMarkdown code {
            font-family: Menlo, Monaco, Consolas, "Courier New", monospace;
            font-size: 12px;
            font-weight: 500;
            color: #c7254e;
            background: #fdf2f5;
            border: 1px solid #f4dde4;
            border-radius: 3px;
            padding: 1px 5px;
        }
        /* 代码块里的 code 不要再套一层行内样式 | Keep the inline chrome out of fenced blocks */
        .swagger-ui .markdown pre > code,
        .swagger-ui .renderedMarkdown pre > code {
            display: block;
            color: #3b4151;
            background: none;
            border: 0;
            padding: 0;
            font-size: 12px;
            font-weight: 400;
        }
        .swagger-ui .markdown pre,
        .swagger-ui .renderedMarkdown pre {
            background: #f6f8fa;
            border: 1px solid #e3e8ee;
            border-radius: 4px;
            padding: 12px;
            overflow-x: auto;
            white-space: pre;
        }

        .swagger-ui .markdown ul,
        .swagger-ui .markdown ol,
        .swagger-ui .renderedMarkdown ul,
        .swagger-ui .renderedMarkdown ol {
            margin: 6px 0;
            padding-left: 22px;
        }
        .swagger-ui .markdown li,
        .swagger-ui .renderedMarkdown li {
            margin: 3px 0;
            line-height: 1.75;
        }
        .swagger-ui .markdown hr,
        .swagger-ui .renderedMarkdown hr {
            border: 0;
            border-top: 1px dashed #dfe4ea;
            margin: 16px 0;
        }
        /* 区块小标题（**参数速览** 之类）单独成段时给一条色条，便于扫读
         * [EN] A section label on a line of its own gets an accent bar so it is easy to scan */
        .swagger-ui .markdown p > strong:only-child,
        .swagger-ui .renderedMarkdown p > strong:only-child {
            display: inline-block;
            border-left: 3px solid #4990e2;
            padding-left: 8px;
            font-size: 14px;
            color: #3b4151;
        }
`

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

%s
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
</html>`, title, descriptionMarkdownCSS, heading, description, linksHTML.String(), specURL)
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
