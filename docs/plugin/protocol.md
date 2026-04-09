# Plugin Protocol

本页说明 RayleaBot 插件与平台之间的正式通信方式和消息语义。

正式 schema 以 `contracts/plugin-protocol.schema.json` 为准。

## 通信形态

- 插件进程与平台通过 JSONL 协议通信。
- `stdout` 保留给协议帧，普通文本不得混入。
- `stderr` 用于调试输出和故障摘要，由平台接入插件 console。

## 生命周期握手

| 方向 | 消息 | 作用 |
| --- | --- | --- |
| server -> plugin | `init` | 传递配置快照、授权结果和启动上下文 |
| plugin -> server | `init_progress` | 可选启动进度上报 |
| plugin -> server | `init_ack` | 宣告握手完成并进入可运行态 |
| server -> plugin | `shutdown` | 要求插件按受控窗口退出 |

- 启动后平台会发送 `ping`，插件返回 `pong` 做保活。
- 插件异常退出会进入崩溃恢复路径，而不是默默消失。

## 事件与结果

| 消息 | 说明 |
| --- | --- |
| `event` | 平台向插件投递统一事件 |
| `result` | 插件对事件或 action 的成功响应 |
| `error` | 插件对事件或 action 的失败响应 |

- 每次事件投递和本地 action 调用都通过 `request_id` 关联。
- 事件方向和 action 方向使用同一套结果 / 错误语义，避免双套协议。

### 事件字段

- `event.message.plain_text` 提供统一纯文本摘要。
- `event.message.segments` 保留结构化消息段。
- `event.payload.message_id` 表示单条消息编号。
- `event.target.id` 与 `event.payload.onebot.group_id` / `event.payload.onebot.user_id` 一起用于定位会话。
- `event.payload.onebot` 保留 OneBot11 原生字段，包括 `post_type`、`message_type`、`group_id`、`user_id`、`time`、`real_id`、`message_seq`、`raw_message`、`message_format`、`font` 和 `sender`。
- `message_sent.private` 与 `message_sent.group` 作为独立事件类型进入插件协议，不并入普通 `message.*`。

## Local Action RPC

当前正式 local action 集合：

- `message.send`
- `message.reply`
- `logger.write`
- `storage.kv`
- `storage.file`
- `http.request`
- `config.read`
- `config.write`
- `scheduler.create`
- `event.expose_webhook`
- `render.image`
- OneBot family actions:
  - `message.get`
  - `message.delete`
  - `message.history.get`
  - `message.forward.get`
  - `message.forward.send`
  - `message.read.mark`
  - `friend.request.handle`
  - `friend.list`
  - `friend.remark.set`
  - `user.info.get`
  - `user.like.send`
  - `group.list`
  - `group.info.get`
  - `group.member.get`
  - `group.member.list`
  - `group.request.handle`
  - `group.leave`
  - `group.admin.set`
  - `group.ban.set`
  - `group.card.set`
  - `group.title.set`
  - `group.name.set`
  - `group.announcement.list`
  - `group.announcement.create`
  - `group.announcement.delete`
  - `group.essence.list`
  - `group.essence.set`
  - `group.essence.unset`
  - `group.honor.get`
  - `group.todo.set`
  - `file.get`
  - `file.download`
  - `file.group.upload`
  - `file.private.upload`
  - `file.group.url.get`
  - `file.private.url.get`
  - `file.group.fs.info`
  - `file.group.fs.list`
  - `file.group.fs.mkdir`
  - `file.group.fs.delete`
  - `reaction.set`
  - `reaction.list`
  - `poke.send`
- Provider extension actions:
  - `provider.napcat.*`
  - `provider.luckylillia.*`

所有 action 都走正式 capability 校验、scope 校验和结构化错误返回。

## 出站消息结构

当前正式消息段类型：

- `text`
- `image`
- `at`
- `at_all`
- `face`
- `reply`
- `record`
- `video`
- `file`
- `flash_file`
- `json`
- `xml`
- `markdown`
- `music`
- `contact`
- `forward`
- `node`
- `poke`
- `dice`
- `rps`
- `mface`
- `keyboard`
- `shake`

平台负责把 shared `message.segments` 投影到当前适配器支持的消息格式。

## 当前边界

- 当前协议不包含复杂流式回传、额外调试流或未冻结 action。
- 协议扩展先更新 contract，再更新 SDK、fixtures、示例和运行时实现。

## 相关文档

- [Event Model](../architecture/event-model.md)
- [Capabilities and Manifest](./capabilities-and-manifest.md)
- [Plugin SDK Docs](./sdk/README.md)
