/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 16:30:00
 * @FilePath: \go-rpc-gateway\examples\07-i18n\main.go
 * @Description: i18n国际化中间件使用示例
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// Response 响应结构
type Response struct {
	Message  string `json:"message"`
	Language string `json:"language"`
}

// helloHandler 示例处理器
func helloHandler(w http.ResponseWriter, r *http.Request) {
	// 从上下文获取i18n
	i18nCtx := middleware.I18nFromContext(r.Context())
	if i18nCtx == nil {
		http.Error(w, "i18n context not found", http.StatusInternalServerError)
		return
	}

	// 使用翻译函数
	message := i18nCtx.T(constants.MessageKeyWelcome)
	language := i18nCtx.GetLanguage()

	// 构建响应
	resp := Response{
		Message:  message,
		Language: language,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// errorHandler 错误处理示例
func errorHandler(w http.ResponseWriter, r *http.Request) {
	// 使用全局翻译函数
	errorMsg := middleware.T(r.Context(), constants.MessageKeyErrorNotFound)
	language := middleware.GetLanguage(r.Context())

	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "application/json")
	
	resp := map[string]interface{}{
		"error":    true,
		"message":  errorMsg,
		"language": language,
	}
	json.NewEncoder(w).Encode(resp)
}

// validationHandler 验证消息示例
func validationHandler(w http.ResponseWriter, r *http.Request) {
	i18nCtx := middleware.I18nFromContext(r.Context())
	if i18nCtx == nil {
		http.Error(w, "i18n context not found", http.StatusInternalServerError)
		return
	}

	// 使用带参数的翻译
	minLengthMsg := i18nCtx.T("validation.min_length", 8)
	emailMsg := i18nCtx.T("validation.email")

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"validation_messages": map[string]string{
			"min_length": minLengthMsg,
			"email":      emailMsg,
		},
		"language": i18nCtx.GetLanguage(),
	}
	json.NewEncoder(w).Encode(resp)
}

// customMessageLoader 自定义消息加载器示例
type customMessageLoader struct {
	messages map[string]map[string]string
}

func (c *customMessageLoader) LoadMessages(language string) (map[string]string, error) {
	if msgs, exists := c.messages[language]; exists {
		return msgs, nil
	}
	return nil, nil
}

func main() {
	// 创建自定义配置
	config := &middleware.I18nConfig{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "zh", "ja"},
		DetectionOrder:     []string{"query", "header", "cookie", "default"},
		LanguageParam:      "lang",
		LanguageHeader:     constants.HeaderAcceptLanguage,
		MessagesPath:       "./locales",
		EnableFallback:     true,
	}

	// 创建HTTP服务器
	mux := http.NewServeMux()

	// 应用i18n中间件
	i18nMiddleware := middleware.I18nWithConfig(config)

	// 注册路由
	mux.Handle("/hello", i18nMiddleware(http.HandlerFunc(helloHandler)))
	mux.Handle("/error", i18nMiddleware(http.HandlerFunc(errorHandler)))
	mux.Handle("/validation", i18nMiddleware(http.HandlerFunc(validationHandler)))

	// 添加说明页面
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <title>i18n 中间件示例</title>
    <meta charset="utf-8">
</head>
<body>
    <h1>i18n 国际化中间件示例</h1>
    <h2>测试链接：</h2>
    <ul>
        <li><a href="/hello">默认语言 (Hello)</a></li>
        <li><a href="/hello?lang=en">英语 (English)</a></li>
        <li><a href="/hello?lang=zh">中文 (Chinese)</a></li>
        <li><a href="/hello?lang=ja">日语 (Japanese)</a></li>
        <li><a href="/error?lang=zh">错误消息 (Error - Chinese)</a></li>
        <li><a href="/validation?lang=ja">验证消息 (Validation - Japanese)</a></li>
    </ul>
    
    <h2>设置Accept-Language头测试：</h2>
    <p>使用curl测试：</p>
    <pre>
curl -H "Accept-Language: zh-CN,zh;q=0.9,en;q=0.8" http://localhost:8080/hello
curl -H "Accept-Language: ja,en;q=0.9" http://localhost:8080/hello
    </pre>
    
    <h2>Cookie测试：</h2>
    <p>设置cookie:</p>
    <pre>
curl -b "lang=zh" http://localhost:8080/hello
    </pre>
</body>
</html>
		`))
	})

	log.Println("Server starting on :8080")
	log.Println("访问 http://localhost:8080 查看示例")
	log.Fatal(http.ListenAndServe(":8080", mux))
}