# locales

**这个目录是空的，而且应该是空的**

网关不需要任何文案文件就能正常工作，这是设计如此，不是缺失

## 文案是怎么解析的

三层兜底，优先级从高到低：

| 层 | 来源 | 谁提供 |
|---|---|---|
| 1 | 服务自己的 locales | 你的服务，路径由你传给 loader |
| 2 | 包内置通用文案 | 本仓库 `errors/builtin_messages.go`，编译进二进制的 Go map（中英两套） |
| 3 | 错误码本身 | 兜底，保证 Code 始终可读 |

所以一个新服务**零配置**就有可读的错误消息 —— 走第 2 层 配了自己的 locales 就覆盖第 2 层，同名 key 优先

实测过「一个文案文件都没有」的情况：全量测试通过

## 为什么把原来的 19 个文件删了

原先这里有 19 个语言文件、76 KB，内容是同一套 22 个演示键（`welcome`、`hello`、`order.total` 这类），而：

- 网关代码里没有任何地方读这个目录 —— 唯一相关的入口是 `NewFileMessageLoader(localesPath)`，路径由调用方传入
- 没有任何测试加载这些文件
- 它们不是网关的消息，是演示数据

既不被代码引用也不被测试覆盖，却跟着模块分发给每一个下游，所以删掉

## 你的服务要怎么用

把自己的文案目录路径传给 loader，和这个目录无关：

```go
loader := middleware.NewFileMessageLoader("resources/locales")
```

注意 go-i18n 只加载 `SupportedLanguages` 里列出的语言，配置里没列的语言不会被读盘：

```yaml
middleware:
  i18n:
    enabled: true
    default-language: zh
    supported-languages: ["zh", "en"]
    messages-path: resources/locales
```

文件名就是语言码，`zh.json`、`en.json`、`pt-br.json` 这样 键名自定，值里的 `{{.name}}` 是模板参数：

```json
{
  "user.created": "用户 {{.name}} 创建成功",
  "order.total": "订单总额：{{.amount}}"
}
```

## 为什么不在首次启动时自动生成

考虑过，不做，理由是没有收益而有代价：

- **没有收益** 零文件已经能正常工作（第 2 层兜底），自动生成的只能是空壳 —— 网关没法替你翻译 而 go-i18n 把空值与缺失键同等对待（两者都返回键名，实测确认），所以空壳文件与没有文件的行为完全一致
- **有代价** 启动时写文件在只读文件系统上会失败（容器、K8s 的常见配置），并且让启动不再幂等

给新服务搭脚手架是代码生成的职责，应当是一条显式命令，而不是服务进程启动时的副作用
