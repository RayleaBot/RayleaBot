# Examples

本目录承载 RayleaBot 的示例内容，包括：

- 示例插件。
- 示例配置。
- 示例请求 / 响应。

当前已收录的 HTTP 请求 / 响应示例包含：

- `examples/http/recovery-confirm.request.json`
- `examples/http/recovery-confirm.accepted.json`
- `examples/http/recovery-confirm.task-detail.json`
- `examples/http/logs-current-session.request.json`
- `examples/http/logs-current-session.response.json`
- `examples/http/logs-history-range.request.json`
- `examples/http/logs-history-range.response.json`
- `examples/http/recovery-recheck.accepted.json`
- `examples/http/runtime-bootstrap.request.json`
- `examples/http/runtime-bootstrap.accepted.json`

当前已收录的资源清单示例包含：

- `examples/deps-manifest.sample.json`

规则：

- 示例只能演示已被 `contracts/` 确认的结构。
- 示例不是正式裁决来源。
- 若示例需要新增字段或消息类型，必须先更新对应 contract。
- `examples/plugins/` 下的示例插件服务于 manifest、plugin protocol、SDK 入口和常用 local action 理解。
