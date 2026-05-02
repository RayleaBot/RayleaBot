# Plugin Examples

本目录承载与 `contracts/plugin-info.schema.json` 和
`contracts/plugin-protocol.schema.json` 对齐的示例插件。

规则：

- 示例用于理解 `info.json`、插件协议、SDK 入口和常用 local action。
- Python 示例使用 `RayleaBotPlugin` 子类、`@command(...)` 与 `@event_handler(...)`。
- Node.js 示例使用 `RayleaBotPlugin` 子类与 `PluginEventContext`。
- 示例不包含真实 secrets、token 或凭据。
- 若示例需要新增字段、状态或消息类型，必须先更新对应 contract。
