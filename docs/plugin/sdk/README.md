# Plugin SDK Docs

本目录说明 RayleaBot 官方 Python / Node.js SDK 的当前使用边界。

## 当前角色

- SDK 为正式插件协议提供便利封装。
- SDK 服务于已落地的 runtime 主链路和示例插件。
- SDK 不单独裁决协议语义，所有正式字段仍以 `contracts/plugin-protocol.schema.json` 为准。

## 当前覆盖范围

- 生命周期握手：`init`、`init_progress`、`init_ack`、`ping/pong`、`shutdown`
- 事件接收与结果回传
- 消息能力：`sendMessage` / `sendReply` 与 Python 对应 helper
- 本地 action helper：日志、KV、文件、HTTP、配置、调度、Webhook、渲染
- OneBot family helper：history、group manage、file、reaction / poke
- provider helper：`provider.napcat.*` 与 `provider.luckylillia.*`
- 扩展消息段 helper：`markdown`、`file`、`keyboard` 与通用 passthrough segment builder

## 并发与请求归属

- 本地 action helper 会自动生成独立 `request_id` 并附带 `parent_request_id`。
- SDK 按 `request_id` 路由等待结果，不依赖“下一帧就是我的返回”。
- Node.js SDK 的 `run()` 会持续收帧，并允许不同事件处理器并发执行。
- Python SDK 的 `run()` 使用线程并发处理事件。
- 使用 SDK 时，事件处理函数需要满足可重入要求。

## 相关文档

- [Plugin Lifecycle](../lifecycle.md)
- [Capabilities and Manifest](../capabilities-and-manifest.md)
- [Protocol](../protocol.md)

## 当前边界

- SDK 只覆盖当前已冻结协议与已落地 action。
- 更宽的调试流、复杂流式回传和未冻结 action 不属于当前正式范围。
