# Plugin Examples

本目录承载与 `contracts/plugin-info.schema.json` 和
`contracts/plugin-protocol.schema.json` 对齐的最小静态示例插件。

规则：

- 这些示例用于帮助 AI 与人工实现者理解 `info.json` 和最小 JSONL 协议骨架。
- 这些示例不是生产模板，不代表完整 SDK 形态，也不代表推荐的目录布局终稿。
- 示例入口文件只覆盖 `init -> init_ack` 和 `event -> result` 的最小交互；`init.bot` 可缺省，最小握手仍返回 `init_ack`。
- 示例入口文件不覆盖 OneBot、插件进程管理、IPC、shutdown 流程或错误恢复。
- 若示例需要新增字段、状态或消息类型，必须先更新对应 contract。
