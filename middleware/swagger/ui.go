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

        /* 类型列里的定义名是指向 Models 区块的锚点，得让人看出来能点
         *
         * 为什么挂在 href 前缀上而不是 class：描述过了一遍 DOMPurify，默认配置的
         * FORBID_ATTR 里有 class 和 style，标签上写什么都会被摘掉，只有 href 留得下来
         *
         * [EN] The definition names in the type column are anchors into the Models section and
         * have to read as clickable. The hook is the href prefix rather than a class because
         * descriptions pass through DOMPurify, whose default FORBID_ATTR covers class and style:
         * anything written on the tag is stripped and only the href survives. */
        .swagger-ui .markdown a[href^="#model-"],
        .swagger-ui .renderedMarkdown a[href^="#model-"] {
            font-family: Menlo, Monaco, Consolas, "Courier New", monospace;
            font-size: 12px;
            font-weight: 500;
            color: #3b7bbf;
            text-decoration: none;
            border-bottom: 1px dashed #9cc2ea;
            cursor: pointer;
            white-space: nowrap;
        }
        .swagger-ui .markdown a[href^="#model-"]:hover,
        .swagger-ui .renderedMarkdown a[href^="#model-"]:hover {
            color: #1f5c99;
            border-bottom-style: solid;
            background: #eef5fd;
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

// modelJumpCSS 类型链接跳过去之后的落点高亮与返回按钮样式
//
// 这两条作用在 Swagger UI 自己的 DOM 上（section.models 下的 .model-container）而不是描述区的
// Markdown，所以跟 descriptionMarkdownCSS 分开放；选择器写到 section.models 这一层是为了压过
// 官方的 `.swagger-ui .model-container:hover`，少一层就在 hover 时被盖掉
//
// [EN] These two target Swagger UI's own DOM — the .model-container elements under
// section.models — rather than the description markdown, so they live apart from
// descriptionMarkdownCSS. The selector reaches down to section.models to outrank the stock
// `.swagger-ui .model-container:hover`; one level shallower and hovering takes it back.
const modelJumpCSS = `
        .swagger-ui section.models .model-container.model-jump-target {
            background: #eef5fd;
            box-shadow: 0 0 0 2px #4990e2;
            border-radius: 4px;
        }
        #swagger-model-jump-back {
            display: none;
            position: fixed;
            right: 24px;
            bottom: 28px;
            z-index: 9999;
            padding: 9px 16px;
            border: 1px solid #3b7bbf;
            border-radius: 20px;
            background: #4990e2;
            color: #fff;
            font-size: 13px;
            line-height: 1.4;
            cursor: pointer;
            box-shadow: 0 2px 8px rgba(0, 0, 0, 0.18);
        }
        #swagger-model-jump-back:hover {
            background: #3b7bbf;
        }
`

// modelJumpScript 类型链接的跳转行为：展开 Models 区块与目标定义、滚过去、高亮落点，并留一个返回按钮
//
// 为什么不能只靠原生锚点：Models 区块折叠时，Swagger UI 的 Collapse 不是把子树藏起来而是整段
// 移出 DOM（渲染成 <noscript/>），#model-Xxx 这时根本不存在，浏览器无处可跳；就算区块是展开的，
// 单条定义默认仍是折叠的，跳过去只看到一行标题，字段一个都没有
//
// 为什么用事件委托：描述里的链接由 React 随 operation 的展开/收起反复创建销毁，
// 逐个 addEventListener 只能绑到当下已渲染的那些，后面展开的接口全漏掉
//
// 为什么按 href 前缀识别：描述过 DOMPurify 时 class 和 data-* 都会被摘掉（FORBID_ATTR
// 含 class/style，ALLOW_DATA_ATTR 默认关），href 是唯一能从 Go 那边带过来的标记
//
// [EN] A plain anchor is not enough. When the Models section is collapsed, Swagger UI's Collapse
// does not hide the subtree, it drops it from the DOM entirely (rendering <noscript/>), so
// #model-Xxx does not exist and the browser has nowhere to jump. Even with the section open each
// definition is still collapsed, so the jump lands on a bare title row with no fields in sight.
//
// The listener is delegated because React creates and destroys these links as operations expand
// and collapse: binding them one by one would only ever cover the ones rendered at that moment.
//
// Recognition goes by href prefix because DOMPurify strips class and data-* attributes from
// descriptions (FORBID_ATTR covers class and style, ALLOW_DATA_ATTR is off by default). The href
// is the only marker that survives the trip from the Go side.
const modelJumpScript = `
    (function () {
        var HREF_PREFIX = '#model-';
        var HIGHLIGHT_CLASS = 'model-jump-target';
        var BACK_BUTTON_ID = 'swagger-model-jump-back';
        var HIGHLIGHT_MS = 1800;
        var MOUNT_WAIT_MS = 3000;
        var backTo = null;

        function openModelsSection(section) {
            if (section.classList.contains('is-open')) {
                return;
            }
            // 5.x 的标题是 <h4><button class="models-control">，老版本是 <h4 onClick>
            var toggle = section.querySelector('button.models-control') || section.querySelector('h4');
            if (toggle) {
                toggle.click();
            }
        }

        function expandModel(container) {
            // 第一个 model-box-control 就是这一条自己的，嵌套字段的开关排在它后面
            // 展开态下 ModelCollapse 带着 hideSelfOnExpand，这个按钮会被整个摘掉，
            // 所以「查不到按钮」本身就说明它已经展开了，不必再点
            var control = container.querySelector('button.model-box-control');
            if (control && control.getAttribute('aria-expanded') !== 'true') {
                control.click();
            }
        }

        function highlight(container) {
            container.classList.add(HIGHLIGHT_CLASS);
            window.setTimeout(function () {
                container.classList.remove(HIGHLIGHT_CLASS);
            }, HIGHLIGHT_MS);
        }

        function backButton() {
            var button = document.getElementById(BACK_BUTTON_ID);
            if (button) {
                return button;
            }
            button = document.createElement('button');
            button.id = BACK_BUTTON_ID;
            button.type = 'button';
            button.textContent = '返回接口';
            button.addEventListener('click', function () {
                button.style.display = 'none';
                if (backTo !== null) {
                    window.scrollTo({ top: backTo, behavior: 'smooth' });
                }
            });
            document.body.appendChild(button);
            return button;
        }

        // 等目标定义真正挂到 DOM 上
        //
        // 不能数帧重试：Models 区块刚展开时这个节点还不存在，而 React 18 的提交由它自己的
        // 调度器安排，跟 requestAnimationFrame 不对齐——帧预算有可能在 React 提交之前就花完，
        // 于是跳转静悄悄地什么也不做改成盯 DOM 变化，节点什么时候出现就什么时候接着走
        //
        // [EN] A frame-count retry does not work here: the node does not exist yet when the
        // Models section has only just been expanded, and React 18 commits on its own scheduler,
        // which is not aligned with requestAnimationFrame — the frame budget can run out before
        // React commits, leaving the jump to do nothing at all and say nothing about it. Watching
        // for DOM changes instead picks the node up whenever it actually arrives.
        function whenModelMounted(name, done) {
            var id = HREF_PREFIX.slice(1) + name;
            var mounted = document.getElementById(id);
            if (mounted) {
                done(mounted);
                return;
            }
            var timer = null;
            var observer = new MutationObserver(function () {
                var node = document.getElementById(id);
                if (!node) {
                    return;
                }
                observer.disconnect();
                window.clearTimeout(timer);
                done(node);
            });
            observer.observe(document.body, { childList: true, subtree: true });
            timer = window.setTimeout(function () {
                observer.disconnect();
            }, MOUNT_WAIT_MS);
        }

        function revealModel(container) {
            expandModel(container);
            container.scrollIntoView({ behavior: 'smooth', block: 'start' });
            highlight(container);
            backButton().style.display = 'block';
        }

        document.addEventListener('click', function (event) {
            var target = event.target;
            var link = target && target.closest ? target.closest('a[href]') : null;
            if (!link) {
                return;
            }
            var href = link.getAttribute('href') || '';
            if (href.lastIndexOf(HREF_PREFIX, 0) !== 0) {
                return;
            }
            var name = href.slice(HREF_PREFIX.length);
            var section = document.querySelector('section.models');
            // 规范没有 definitions 时整个 Models 区块都不渲染，这时不接手，交回浏览器按原生锚点处理
            if (!name || !section) {
                return;
            }
            event.preventDefault();
            backTo = window.pageYOffset;
            openModelsSection(section);
            whenModelMounted(name, revealModel);
        });
    }());
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
    <script>%s</script>
</body>
</html>`, title, descriptionMarkdownCSS, modelJumpCSS, heading, description, linksHTML.String(), specURL, modelJumpScript)
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
