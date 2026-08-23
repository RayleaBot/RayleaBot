# Plugin Examples

本目录承载与 `contracts/plugin-info.schema.json` 和
`contracts/plugin-protocol.schema.json` 对齐的示例插件。

规则：

- 示例用于理解 `info.json`、插件协议、SDK 入口和常用 local action。
- 每个示例都是独立 Go module：`cmd/<name>/main.go` 是进程入口，示例资源放在顶层目录（如 `templates/`、`ui/`、`assets/`），并通过 `sdk/go` 消费协议客户端与 local action helper；示例保持精简布局，不代表独立插件仓库的完整目录约定。
- 插件运行时固定为 Go；Python、Node.js 等其他语言运行时不被接受。
- 示例不包含真实 secrets、token 或凭据。
- 若示例需要新增字段、状态或消息类型，必须先更新对应 contract。
