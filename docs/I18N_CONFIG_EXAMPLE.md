# I18n中间件使用示例

## 基本使用

```go
// 使用默认配置
i18nMiddleware := middleware.I18n()
app.Use(i18nMiddleware)

// 使用自定义配置
config := &middleware.I18nConfig{
    DefaultLanguage:    "en",
    SupportedLanguages: []string{"en", "zh", "zh-tw", "fr", "fr-fr"},
    DetectionOrder:     []string{"query", "header", "cookie", "default"},
    LanguageParam:      "lang",
    LanguageHeader:     "Accept-Language",
    MessagesPath:       "./locales",
    EnableFallback:     true,
}
i18nMiddleware := middleware.I18nWithConfig(config)
```

## 语言映射配置

支持地区变体到基础语言的映射：

```go
config := &middleware.I18nConfig{
    DefaultLanguage:    "en",
    SupportedLanguages: []string{"en", "zh", "zh-tw", "fr", "fr-fr", "pt", "pt-br"},
    MessagesPath:       "./locales",
    EnableFallback:     true,
    
    // 语言映射：地区变体 -> 目标语言
    LanguageMapping: map[string]string{
        "zh-cn": "zh",           // 简体中文映射到zh
        "zh-tw": "zh-tw",        // 繁体中文映射到zh-tw
        "zh-hk": "zh-tw",        // 香港中文映射到繁体
        "zh-sg": "zh",           // 新加坡中文映射到简体
        "en-us": "en",           // 美国英语映射到en
        "en-gb": "en",           // 英国英语映射到en
        "fr-fr": "fr-fr",        // 法国法语映射到fr-fr
        "fr-ca": "fr-fr",        // 加拿大法语映射到法国法语
        "pt-br": "pt-br",        // 巴西葡萄牙语映射到pt-br
        "pt-pt": "pt",           // 葡萄牙葡萄牙语映射到pt
    },
    
    // 自定义消息文件路径（可选）
    CustomMessagePaths: map[string]string{
        "zh-tw": "locales/traditional",  // 繁体中文使用特定目录
        "fr-fr": "locales/france",       // 法国法语使用特定目录
    },
}
```

## 运行时配置修改

支持运行时修改支持的语言列表和消息路径：

```go
// 创建配置
config := middleware.DefaultI18nConfig()

// 动态修改支持的语言
config.SupportedLanguages = []string{"en", "zh", "ja"}

// 动态修改消息路径
config.MessagesPath = "./custom_locales"

// 添加新的语言映射
config.LanguageMapping["custom-lang"] = "en"

// 添加自定义路径
config.CustomMessagePaths["special"] = "./special_locales"

// 创建管理器
manager, err := middleware.NewI18nManager(config)
```

## 业务层API使用

```go
// 在处理函数中使用
func handleRequest(c echo.Context) error {
    ctx := c.Request().Context()
    
    // 获取翻译消息
    welcomeMsg := middleware.GetMsgByKey(ctx, "welcome")
    
    // 带参数的翻译
    userMsg := middleware.GetMsgWithMap(ctx, "user.created", map[string]interface{}{
        "name": "张三",
    })
    
    return c.JSON(200, map[string]string{
        "welcome": welcomeMsg,
        "user":    userMsg,
    })
}
```

## 文件结构示例

```text
项目根目录/
├── locales/
│   ├── en.json      # 英语
│   ├── zh.json      # 简体中文
│   ├── zh-tw.json   # 繁体中文
│   ├── fr.json      # 法语
│   ├── fr-fr.json   # 法国法语
│   ├── pt.json      # 葡萄牙语
│   └── pt-br.json   # 巴西葡萄牙语
├── locales/traditional/  # 自定义路径示例
│   └── zh-tw.json
└── locales/france/       # 自定义路径示例
    └── fr-fr.json
```

## 消息文件格式

```json
{
  "welcome": "Welcome",
  "error.not_found": "Resource not found", 
  "error.bad_request": "Bad request",
  "user.created": "User {{.name}} created successfully",
  "user.info": "User {{.name}} is {{.age}} years old"
}
```

## 支持的功能

1. **灵活的语言映射**：支持地区变体映射到基础语言或其他变体
2. **自定义文件路径**：不同语言可以使用不同的消息文件目录
3. **运行时配置**：支持动态修改语言列表和路径
4. **16种语言支持**：内置支持16种国际主流语言
5. **回退机制**：当指定语言不存在时自动回退到默认语言
6. **模板变量**：支持在翻译消息中使用变量替换
