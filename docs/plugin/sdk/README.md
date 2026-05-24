# Plugin SDK Docs

本目录说明 RayleaBot 官方 Python / Node.js SDK 的正式能力范围。

正式协议与字段以 `contracts/plugin-protocol.schema.json` 为准。

## 当前覆盖范围

- 生命周期握手：`init`、`init_progress`、`init_ack`、`ping`、`pong`、`shutdown`
- 插件结构：
  - Python：继承 `RayleaBotPlugin`，使用 `@command(...)` 与 `@event_handler(...)` 注册类方法
  - Node.js：继承 `RayleaBotPlugin`，在构造函数中使用 `this.onCommand(...)` 与 `this.onEvent(...)` 注册实例方法
- 事件上下文：
  - Python：`EventContext` 提供 `event`、`request_id`、`target`、`actor`、`payload`、`args`、`plain_text`、`bot_id` 和 request-bound helper
  - Node.js：`PluginEventContext` 提供 `event`、`requestId`、`target`、`actor`、`payload`、`args`、`plainText`、`botId` 和 request-bound helper
- 启动上下文 helper：
  - Python：`bot_id`、`capabilities`、`command_prefixes`、`primary_command_prefix`
  - Node.js：`botId`、`capabilities`、`commandPrefixes`、`primaryCommandPrefix`
- `bot_id` / `botId` 在协议身份不可用时为空字符串；SDK 收到 `bot.identity.changed` 后更新为当前 bot 身份。当 `bot.identity.changed` 不携带可用身份时（`target` 与 `event.payload.onebot.self_id` 均为空），SDK 将 `bot_id` / `botId` 重置为空，下次 `bot.identity.changed` 再恢复。
- 等待身份就绪 helper：
  - Python：`RayleaBotPlugin.await_bot_identity(timeout_seconds=30)`；身份已知时立即返回当前 `bot_id`，否则阻塞至身份就绪或超时，超时返回空字符串。线程安全，可在事件处理线程内调用。
  - Node.js：`plugin.awaitBotIdentity(timeoutMs=30000)` 与 `EventContext.awaitBotIdentity(timeoutMs)`；Promise 在身份就绪或超时时 resolve 当前 `botId`。
  - 调用方在身份不可用期间不应忙等：handler 内 `await` SDK helper 或直接 `return` 让出线程，避免阻塞 dispatcher。
- 事件接收与结果回传
- 通用 local action helper：`message.send`、`message.reply`、`logger.write`、`storage.kv`、`storage.file`、`http.request`、`config.read`、`config.write`、`governance.blacklist.read`、`governance.blacklist.write`、`governance.whitelist.read`、`governance.whitelist.write`、`governance.command_policy.read`、`scheduler.create`、`event.expose_webhook`、`render.image`、`plugin.list`
- 定时任务 helper 支持中文日志说明：Python 使用 `scheduler_create(..., log_label="每日早报")`，Node.js 使用 `schedulerCreate(..., { logLabel: "每日早报" })`。
- `secret.read` helper 当前只在 Python SDK 提供（`secret_read` / `secretRead`）；Node.js SDK 可通过通用回退入口 `onebotAction("secret.read", { key })` 调用。
- OneBot 单动作 helper：正式 capability 名称与 action kind 一一对应，helper 直接复用同一组动作名
- provider helper：`provider.napcat.message_emoji.like.set`、`provider.napcat.group.sign.set`、`provider.luckylillia.friend_groups.get`
- 通用回退入口：`onebot_action` / `onebotAction` 与 `provider_action` / `providerAction`
- 结构化错误：`ActionError.code`、`ActionError.message`、`ActionError.details`

## 推荐入口

Python：

```python
from rayleabot import RayleaBotPlugin, command


class EchoPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private")

    @command("echo", aliases=["repeat"])
    def handle_echo(self, ctx):
        ctx.send_text(" ".join(ctx.args) or ctx.plain_text or "(空消息)")


if __name__ == "__main__":
    EchoPlugin().run()
```

Node.js：

```js
import { RayleaBotPlugin } from '@rayleabot/sdk'

class EchoPlugin extends RayleaBotPlugin {
  constructor() {
    super()
    this.subscribe('message.group', 'message.private')
    this.onCommand('echo', this.handleEcho, ['repeat'])
  }

  handleEcho(ctx) {
    ctx.sendText(ctx.args.join(' ') || ctx.plainText || '(空消息)')
  }
}

await new EchoPlugin().run()
```

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
