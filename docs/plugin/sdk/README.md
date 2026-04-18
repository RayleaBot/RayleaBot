# Plugin SDK Docs

本目录说明 RayleaBot 官方 Python / Node.js SDK 的正式能力范围。

正式协议与字段以 `contracts/plugin-protocol.schema.json` 为准。

## 当前覆盖范围

- 生命周期握手：`init`、`init_progress`、`init_ack`、`ping`、`pong`、`shutdown`
- 启动上下文 helper：
  - Python：`bot_id`、`capabilities`、`command_prefixes`、`primary_command_prefix`
  - Node.js：`botId`、`capabilities`、`commandPrefixes`、`primaryCommandPrefix`
- 事件接收与结果回传
- 通用 local action helper：`message.send`、`message.reply`、`logger.write`、`storage.kv`、`storage.file`、`http.request`、`config.read`、`config.write`、`scheduler.create`、`event.expose_webhook`、`render.image`、`plugin.list`
- OneBot 单动作 helper：正式 capability 名称与 action kind 一一对应，helper 直接复用同一组动作名
- provider helper：`provider.napcat.message_emoji.like.set`、`provider.napcat.group.sign.set`、`provider.luckylillia.friend_groups.get`
- 通用回退入口：`onebot_action` / `onebotAction` 与 `provider_action` / `providerAction`
- 结构化错误：`ActionError.code`、`ActionError.message`、`ActionError.details`

## 消息段 builder

官方 builder 覆盖当前正式消息段集合：

- 基础段：`text`、`image`、`at`、`at_all`、`face`、`reply`
- 媒体与文件：`record`、`video`、`file`、`flash_file`
- 富文本与卡片：`json`、`xml`、`markdown`、`music`、`contact`
- 组合与转发：`forward`、`node`
- 互动段：`poke`、`dice`、`rps`
- provider 扩展段：`mface`、`keyboard`、`shake`

Python 使用 snake_case builder，例如 `flash_file_segment()`、`keyboard_segment()`；Node.js 使用 camelCase builder，例如 `flashFileSegment()`、`keyboardSegment()`。两套 SDK 都保留 `passthrough_segment()` / `passthroughSegment()` 作为通用构造入口。

## 并发与请求归属

- 本地 action helper 会自动生成独立 `request_id`，并附带 `parent_request_id`
- SDK 按 `request_id` 路由返回结果，不依赖帧到达顺序
- Node.js SDK 的 `run()` 允许不同事件处理器并发执行
- Python SDK 的 `run()` 使用线程并发处理事件
- 事件处理函数需要满足可重入要求

## 相关文档

- [Plugin Lifecycle](../lifecycle.md)
- [Capabilities and Manifest](../capabilities-and-manifest.md)
- [Protocol](../protocol.md)
