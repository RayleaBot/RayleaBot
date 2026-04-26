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
- `init.command_prefixes` 提供当前生效的命令前缀列表，至少包含一项。
- `init.bot` 在 OneBot 身份可用时提供当前 bot 身份；协议身份不可用时该字段缺省。

## 事件与结果

| 消息 | 说明 |
| --- | --- |
| `event` | 平台向插件投递统一事件 |
| `result` | 插件对事件或 action 的成功响应 |
| `error` | 插件对事件或 action 的失败响应 |
| `action` | 插件发起本地 action 请求；平台返回 `result` 或 `error` |

- 事件投递使用独立 `request_id`。
- 本地 action 使用自己的 `request_id`，并通过 `parent_request_id` 归属到对应事件。
- manifest 省略 `concurrency` 时，插件按串行事件处理；显式声明后，不同 `event.target` 可并发，同一 `event.target` 保持顺序。
- 并发插件发起本地 action 时必须提供 `parent_request_id`。
- 事件方向和 action 方向共用 `result` / `error` 语义。
- `error` 固定返回 `code`、`message`，可选 `details` 用于补充结构化失败上下文。

### 事件字段

- 当前正式 `event_type` 集合包括：
  - 平台事件：`scheduler.trigger`、`config.changed`、`webhook.received`、`bot.identity.changed`
  - OneBot 消息事件：`message.private`、`message.group`、`message_sent.private`、`message_sent.group`
  - OneBot notice 事件：`notice.member_increase`、`notice.member_decrease`、`notice.group_admin`、`notice.group_ban`、`notice.group_recall`、`notice.group_upload`、`notice.group_card`、`notice.group_title`、`notice.group_essence`、`notice.friend_add`、`notice.friend_recall`、`notice.flash_file`、`notice.poke`、`notice.poke_recall`、`notice.profile_like`、`notice.input_status`、`notice.group_message_emoji_like`
  - OneBot request 事件：`request.friend`、`request.group`
  - OneBot meta 事件：`meta.heartbeat`、`meta.lifecycle`
- `event.message.plain_text` 提供统一纯文本摘要。
- `event.message.segments` 保留结构化消息段。
- `event.message.segments[].type` 正式类型为 `text`、`image`、`at`、`at_all`、`face`、`reply`、`record`、`video`、`file`、`flash_file`、`json`、`xml`、`markdown`、`music`、`contact`、`forward`、`node`、`poke`、`dice`、`rps`、`mface`、`keyboard`、`shake`。
- `event.payload.message_id` 表示单条消息编号。
- `event.target.id` 与 `event.payload.onebot.group_id` / `event.payload.onebot.user_id` 一起用于定位会话。
- `event.payload.onebot` 保留 OneBot11 原生字段，包括 `post_type`、`message_type`、`group_id`、`user_id`、`time`、`real_id`、`message_seq`、`raw_message`、`message_format`、`font`、`sender`、`meta_event_type`、`interval` 和 `status`。
- `message_sent.private` 与 `message_sent.group` 作为独立事件类型进入插件协议，不并入普通 `message.*`。
- `meta.*` 事件使用系统会话：`conversation_type=system`、`conversation_id=bot:<self_id>`、`sender_id=<self_id>`、`target.type=bot`、`target.id=<self_id>`；`event.message` 保持为空。
- `bot.identity.changed` 使用 `target.type=bot`、`target.id=<self_id>`，并在 `event.payload.onebot.self_id` 中提供同一身份。

## Local Action RPC

当前正式 local action 集合：

- `message.send`
- `message.reply`
- `logger.write`
- `storage.kv`
- `storage.file`
- `http.request`
- `config.read`
- `plugin.list`
- `config.write`
- `governance.blacklist.read`
- `governance.blacklist.write`
- `governance.whitelist.read`
- `governance.whitelist.write`
- `governance.command_policy.read`
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
  - `provider.napcat.message_emoji.like.set`
  - `provider.napcat.group.sign.set`
  - `provider.luckylillia.friend_groups.get`

所有 action 都走正式 capability 校验、scope 校验和结构化错误返回。

`message.send`、`message.reply`、OneBot family actions 与 provider extension actions 需要可用的 OneBot adapter 连接；连接不可用时返回 adapter 类错误，插件进程保持运行。

OneBot 单动作 capability 名称与 action kind 保持一致，provider capability 只包含上面三项正式扩展动作。

- `plugin.list` 返回当前已发现插件的只读目录，包括插件状态、命令列表和命令冲突信息。
- `governance.blacklist.read` 与 `governance.whitelist.read` 返回当前治理快照。
- `governance.blacklist.write` 支持单条黑名单 `upsert` 与 `delete`。
- `governance.whitelist.write` 支持白名单开关 `set_enabled`，以及单条白名单 `upsert` 与 `delete`。
- `governance.command_policy.read` 返回当前生效的默认权限、冷却配置和命令级权限投影。

- 同一事件内允许多个 local action 同时在途。
- 插件在本地 action 尚未完成时返回事件级 `result` 或 `error`，属于协议违规。
- 处理器需要满足可重入要求，避免把会话外状态写成单线程假设。

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

管理面兼容矩阵通过 `GET /api/protocols/onebot11/compatibility` 提供正式读取面，固定覆盖 `events`、`message_segments`、`read_capabilities` 和 `provider_extensions` 四类能力。

## 当前边界

- 当前协议不包含批量消息、复杂流式回传、额外调试流或未冻结 action。
- 协议扩展先更新 contract，再更新 SDK、fixtures、示例和运行时实现。

## 相关文档

- [Event Model](../architecture/event-model.md)
- [Capabilities and Manifest](./capabilities-and-manifest.md)
- [Plugin SDK Docs](./sdk/README.md)
