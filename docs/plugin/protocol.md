# Plugin Protocol v3

RayleaBot 与插件进程使用 JSONL 通信。正式消息结构以 `contracts/plugin-protocol.schema.json` 为准。

## 传输约束

- `stdout` 只输出一行一个 JSON 协议帧。
- `stderr` 用于插件调试输出，由宿主接入插件 console。
- 单帧大小、待处理 action 数、action 突发率、事件超时和关闭宽限由宿主配置限制。
- 插件后端只需要是当前平台原生可执行文件，协议不依赖实现语言。

## 生命周期

| 方向 | 帧 | 作用 |
| --- | --- | --- |
| Server → plugin | `init` | 建立协议版本、插件身份和初始上下文 |
| plugin → Server | `init_progress` | 可选启动进度 |
| plugin → Server | `init_ack` | 宣告可运行 |
| Server → plugin | `ping` | 保活请求 |
| plugin → Server | `pong` | 保活响应 |
| Server → plugin | `shutdown` | 受控退出 |

只有 `init` 携带 `protocol_version: "3"` 和 `plugin_id`。后续帧不得重复协议版本、插件 ID、envelope 时间戳或事件订阅。

`init` 同时提供：

- 完整配置快照 `config`。
- 生效权限 `effective_permissions`。
- 身份列表 `bots`，元素包含 `source_adapter`、`source_protocol`、`id` 与可选 `nickname`；没有已知身份时为 `[]`。
- OneBot11 QQ 超级管理员列表和命令前缀；列表不适用于 QQ 官方 openid。
- 生效并发度。
- 必填的 IANA 时区 `timezone`，对应宿主当前生效的时区；保存后待重启的时区不提前下发。

SDK 从 init 建立插件 ID、并发限制和原子配置快照，插件不手工配置这些值。

Go SDK 通过 `EventContext.Location` 和 `Actions.TimeLocation()` 提供该时区，生命周期内保持不变，不修改进程的 `time.Local`。插件展示时间使用这个位置；无时区的平台日期按平台约定解析，投递时效、超时和去重仍比较绝对时间。

## 事件

manifest 的 `events` 是唯一普通事件订阅来源。省略或空数组表示不接收普通 fan-out；定向控制事件仍按宿主生命周期语义投递。

`event` 帧携带统一事件。宿主在进程会话中生成唯一 `request_id`；正常完成的事件用该 ID 返回一个终态 `result` 或 `error`。事件过期、运行时关闭或动作无法在收尾时限内结算时，插件停止发送该事件的后续帧，由宿主结束事件。

重要平台事件：

- `plugin.started`
- `scheduler.trigger`
- `management.action`
- `config.changed`
- `webhook.received`
- `bot.identities.changed`

OneBot 消息、notice、request 与 meta 事件继续使用正式 `event_type` 枚举。消息文本位于 `event.message.plain_text`，结构化段位于 `event.message.segments`。

平台原生字段位于 `event.payload.onebot`，它是形状闭合的归一化投影而非原始上报帧透传，只在 `event.source_protocol` 为 `onebot11` 时出现；读取前先判断 `source_protocol`。`event.actor.id` 与 `event.target.id` 属于该协议的身份命名空间，并限定于接收事件的 bot 身份，不能当作跨协议的全局标识。

### 配置变更

`config.changed` 提供：

- `config`：变更后的完整配置快照。
- `changed_keys`：本次变化的顶层键。

SDK 在调用事件 handler 前原子替换配置快照。每个 `EventContext.Config` 都是隔离副本，插件修改该 map 不影响后续事件。

### 身份变更

`bot.identities.changed` 是来源为 `platform` / `adapters.internal` 的控制事件，`payload.bots` 是完整身份列表。SDK 在执行 handler 前替换列表；空数组清除旧身份。列表按 `source_adapter` 排序，每个适配器实例最多一个身份，停用实例从列表移除。暂时重连与已确认身份是不同状态，连接失败仍通过对应动作结果表达。

Go SDK 的 `EventContext.Bots` 是隔离的列表副本，`EventContext.Bot` 根据聊天事件的 `source_adapter` 和 `source_protocol` 选择对应身份。平台内部事件仅在列表恰好有一个身份时提供该便利值，多实例时为空；插件应明确选择目标实例。相同字符串 ID 在不同实例中属于不同身份。

协议 v2 的单一 `init.bot` 和 `bot.identity.changed` 不再使用；升级与重建步骤见[协议 v3 升级](../release/plugin-protocol-v3-upgrade.md)。

## Action RPC

插件用 `action` 帧调用宿主能力；action 使用独立 `request_id`，并通过 `parent_request_id` 归属当前事件。宿主返回 `result` 或 `error`。

同一事件可以有多个并发 action，但插件必须等待它们完成后再发送事件终态。

取消本地等待不代表宿主动作已取消。SDK 保留已发出动作的响应关联，接收迟到响应；终态最多等待一个 `ActionTimeout`，仍有未结算动作时不发送终态。事件开始收尾后禁止新 action。未知或非法协议帧按协议违规处理。

`plugin.event_canceled` 表示请求或生命周期取消，调度统计计入 `other`；`plugin.event_timeout` 表示确实超过事件处理时限。两者不能通过重放消息动作自动恢复。

### 隐式插件私有动作

以下动作不要求 manifest 权限：

- `logger.write`
- `config.write`
- `storage.kv`
- `storage.file`

宿主使用 init 建立的插件身份选择命名空间。`storage.file` 请求只传相对 `path`，不能选择文件根或其他插件空间。配置读取不使用 action；插件读取当前 `EventContext.Config`。

### 显式权限动作

常用动作：

- `message.send`
- `http.request`
- `plugin.list`
- `secret.read`
- `thirdparty.account.read`
- `thirdparty.account.validate`
- `thirdparty.resolve`
- `governance.blacklist.read` / `write`
- `governance.whitelist.read` / `write`
- `governance.command_policy.read`
- `scheduler.create`
- `render.image`
- OneBot family actions
- provider 扩展动作

动作未在 manifest `permissions` 声明，或请求平台超出权限范围时，宿主返回 `plugin.permission_denied`。

### OneBot 与 provider 实例选择

OneBot 和 provider 扩展动作从 OneBot 父事件继承 `source_adapter`。主动动作可以在 `data` 中指定 `source_adapter`，可选的 `source_protocol` 只允许 `onebot11`；这些字段只供宿主选路，不转发给 provider。

平台内部事件不指定聊天实例。没有实例选择信息时，必须恰好有一个已启用的 OneBot11 实例。实例未知、停用、已移除、协议不匹配、选择存在歧义，或显式选择与聊天父事件冲突时，宿主返回 `plugin.protocol_violation`，不会发出 API 请求。其他聊天协议的父事件不能调用 OneBot 动作。

### 消息发送与回复

插件只发送 `message.send` action：

- 普通发送提供目标和 segments；`source_adapter` 指定配置中的适配器实例 ID。
- 只提供 `source_protocol` 时，该协议必须恰好有一个已启用实例；两个字段都不提供时，必须全局恰好有一个已启用实例。两个字段同时提供时，实例与协议必须匹配。
- 回复当前事件时原样提供宿主的 `event_id` 作为 `reply_to_event_id`，宿主据此定位实例。事件 ID 是不透明标识，不应从平台消息 ID 构造或解析。
- 回复指定消息时使用首个 `reply` segment 或相应回复字段。

宿主内部可以根据适配器能力转换为回复或普通发送；协议仅暴露 `message.send` action。

QQ 官方机器人事件的原生投影位于 `event.payload.qq_official`，包含分发类型、消息标识和 openid 等字段；仅在 `source_protocol=qqofficial` 时读取。私聊和群聊的被动回复都引用入站消息，避免作为主动推送消耗额度。

`adapter.send_unconfirmed` 表示请求可能已发出，但尚未收到确定回执，消息可能继续送达。调用方不得自动重发，也不能立即删除适配器尚可能读取的媒体文件。明确拒绝的发送使用 `adapter.send_failed` 等相应错误码。

### 黑白名单

`governance.blacklist.write` 与 `governance.whitelist.write` 的增删操作必须提供 `scope`，与管理 API 共用相同规则：

- OneBot 全局规则为 `{"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""}`。
- 实例规则使用 `kind: "instance"`，填写 `source_protocol`、`source_adapter` 和 `bot_id`。Go SDK 可从 `EventContext.Bot` 取得实例身份，使用 `GovernanceScope` 传入写请求；身份未知时不能猜测 bot ID。
- 读取返回所有作用域的条目；删除需要原样提供目标条目的 scope，相同目标在其他作用域的规则不受影响。
- 白名单 `set_enabled` 不要求 scope，它修改服务级开关。

### HTTP

`http.request` 需要显式权限，但不声明主机白名单。宿主仍执行 HTTPS、DNS、重定向复查、SSRF/私网拦截、超时和响应体限制。

`render.image.resources` 复用同一 HTTP 安全边界，并叠加图片格式、单项大小、总量、数量和处理期限限制。

### Webhook

Webhook 路由由 manifest 静态声明。协议没有运行时暴露 webhook 的 action。请求通过宿主鉴权与重放检查后以 `webhook.received` 投递。

### 三方账号

- `thirdparty.account.read` 只返回已保存、启用且可用的账号凭据。
- `thirdparty.account.validate` 只提交受限异常观察，由 Server 决定凭据状态。
- `thirdparty.resolve` 当前用抖音登录环境解析用户候选。

## 终态和错误

`error` 固定包含 `code` 和 `message`，可选 `details`。插件 handler panic、重复终态、未知 action、移除字段或错误 envelope 都会被转换为正式插件错误并记录脱敏诊断。

Go SDK 提供 `event.SendText`、`event.Send`、`event.Reply`、`event.Result` 和 `event.Fail` 终态 helper，以及 `event.Actions()` 非终态 action helper。每个事件只能成功发送一次终态。

## 并发与顺序

- 生效并发度为 `min(manifest.concurrency, runtime.max_concurrent_tasks_per_plugin)`，最小为 `1`。
- 同插件、同 `event.target.type + ":" + event.target.id` 保持顺序。
- 不同会话可以并发。
- 无稳定 target 的事件进入独立 fallback lane。

## 相关文档

- [Event Model](../architecture/event-model.md)
- [Plugin Manifest and Permissions](./permissions-and-manifest.md)
- [Plugin SDK](./sdk/README.md)
