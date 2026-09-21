# Plugin Protocol v4

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

只有 `init` 携带 `protocol_version: "4"` 和 `plugin_id`。后续帧不得重复协议版本、插件 ID、envelope 时间戳或事件订阅。

`init` 同时提供：

- 完整配置快照 `config`。
- 身份列表 `bots`，元素包含 `source_adapter`、`source_protocol`、`id` 与可选 `nickname`；没有已知身份时为 `[]`。
- OneBot11 QQ 超级管理员列表；列表不适用于 QQ 官方 openid。
- `command_prefixes`：对本插件生效的命令前缀，专属前缀在前，接受通用前缀时再列出通用前缀。
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
- `plugin.request`（定向服务请求）
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

协议 v2 的单一 `init.bot` 和 `bot.identity.changed` 不再使用。SDK 的协议版本校验会拒绝 v2 宿主，v2 SDK 也不能处理 v3 握手；插件须用当前 SDK 重新构建，其他语言实现按 `contracts/plugin-protocol.schema.json` 更新。

## Action RPC

插件用 `action` 帧调用宿主能力；action 使用独立 `request_id`，并通过 `parent_request_id` 归属当前事件。宿主返回 `result` 或 `error`。

同一事件可以有多个并发 action，但插件必须等待它们完成后再发送事件终态。每个插件进程同时未完成的 action 最多 256 个，超出的 action 立即返回可重试的 `platform.rate_limited`，不进入执行。

普通宿主动作取消本地等待不代表动作已取消；`plugin.call` 的取消传播见下方服务调用说明。SDK 保留已发出动作的响应关联，接收迟到响应；终态最多等待一个 `ActionTimeout`，仍有未结算动作时不发送终态。事件开始收尾后禁止新 action。未知或非法协议帧按协议违规处理。

`plugin.event_canceled` 表示请求或生命周期取消，调度统计计入 `other`；`plugin.event_timeout` 表示确实超过事件处理时限。两者不能通过重放消息动作自动恢复。

### 消息传播

消息按插件的 `priority` 降序分层，同层并发；成功终态的 `propagation: stop|continue` 覆盖静态 `block`。正优先级消息订阅者先于命令声明者接收匹配的命令消息，零优先级普通订阅者仍不接收已定向的命令。同名命令权限、名单、菜单与冷却保持现有规则。低层在上层完成前已占据原 FIFO 位置，发送失败不改写终态传播决定。

### 插件服务调用

`plugin.call` 必须携带 `parent_request_id`。参数为 `target_plugin_id`、`service`、`service_version`、`method`、对象类型的 `params`；宿主不解释业务参数。

宿主向已运行的目标投递 `plugin.request`，`payload.service_request` 包含服务/版本/方法/参数、`caller_plugin_id`、`deadline_at_ms` 和 `origin`。origin 保存实际父事件的来源、可用的 bot/actor/target 及调度任务标识，不包含聊天正文或任意原始上报。调用者不能在 action 参数中自报 caller。

提供者使用该新事件自己的上下文，可调用自身存储和其他普通宿主动作，以 `result` 或 `error` 完成。宿主将结果关联回原调用；提供者的结构化错误保留 code、message 和 details，路由不按业务消息文案分支。首版只支持一跳：自调用和服务处理器继续发起 `plugin.call` 返回 `plugin.call_chain_rejected`。

调用期限为 30 秒、父事件剩余期限与目标事件期限中的最小值，包含排队时间；每个目标最多 64 个未完成服务请求。目标并发度仍按其 manifest 生效。发送给目标或返回调用者的完整帧超过对应上限时，调用失败而不因转发大对象回收对端进程。

协议没有取消帧。调用方的父事件结束、超时，或调用方进程停止、重载时，宿主结束所属的未完成 `plugin.call` 并不再等待；提供者不会收到通知，它的 `plugin.request` 持续到 `deadline_at_ms`，迟到的终态帧被宿主忽略，旧运行代次的结果不会交给新实例。Go SDK 为服务处理器的 context 设置该期限；仍在排队的请求到期后不再执行。处理器应在 context 结束后尽快返回，否则会一直占用提供者的并发槽直到期限。已开始的外部副作用不能视为已回滚，宿主不自动重试调用。

Go SDK 的 `CallService` 在本地 context 结束时返回错误，并保留响应关联直到宿主结算。

只有通用路由和生命周期属于宿主；服务业务、允许调用者与数据保护仍由插件负责。路由不默认记录参数或结果正文。

### 插件私有动作

以下动作按插件命名空间隔离：

- `logger.write`
- `config.write`
- `storage.kv`
- `session.wait` / `session.finish`

宿主使用 init 建立的插件身份选择命名空间。持久化文件由插件直接写入 `RAYLEABOT_PLUGIN_DATA_DIR`。配置读取不使用 action；插件读取当前 `EventContext.Config`。

### 对话等待

消息事件中的 `session.wait` 登记下一条输入，返回对话 ID、scope 和 Unix 毫秒期限。发起事件成功终态后才等待；只有 waiting 状态的回复定向交给登记进程，沿用普通消息的名单准入，跳过普通命令、菜单和订阅分发。发起中或处理中的消息继续普通流程，不缓冲或回放。

当前回复带有 `payload.session`，再次等待传入同一个 session_id；业务步骤由插件维护。同插件在同一路由新建会覆盖自己的旧项，新 ID 产生新绝对期限。插件停止或重载时清理其进程的对话。

`notify_on_expire: true` 请求尽力投递的 `session.expired`；只有该新事件上下文可用于发送超时提示，仍受平台发送能力限制。主动结束、覆盖和进程退出不通知。`session.finish` 可在同一插件进程的其他活动父事件中调用。

### KV 期限

`storage.kv` 的 `set` 可携带整数 `ttl_seconds`（1..31536000）。使用 TTL 的插件声明 `min_core_version >= 0.6.0`。省略 TTL 会永久覆盖并清除旧期限；成功结果中的 `expires_at_ms` 是实际 Unix 毫秒截止时间，永久值省略该字段。

在 `now >= expires_at_ms` 时，get 返回 `exists=false` 且不包含 value/expiry，list 不列出该键，delete 返回 `deleted=false`。全局逻辑配额排除过期键，写入仍在事务内校验单值与总容量。空列表返回 `keys: []`。

需要进程内条件写入时，由插件自行同步。宿主每 60 秒分批删除过期行，每批 1000 行；逻辑配额释放不表示 SQLite 文件立即缩小。

### 宿主动作

常用动作：

- `message.send`
- `plugin.list`
- `plugin.call`
- `secret.read` / `write` / `delete`
- `browser.launch` / `browser.close`
- `governance.blacklist.read` / `write`
- `governance.whitelist.read` / `write`
- `governance.command_policy.read`
- `scheduler.create`
- `render.image`
- OneBot family actions
- provider 扩展动作

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

### 渲染资源

`render.image.resources` 中的每项图片由宿主在渲染前解析，模板通过 `data-render-resource` 引用：

- `url` 项由宿主预取，最多 16 项，不拦截私网地址。插件需要其他 HTTP 请求时使用自己的 HTTP 客户端。
- `path` 项是插件数据目录（`RAYLEABOT_PLUGIN_DATA_DIR`）内的相对斜杠路径，用于插件自行下载或生成的图片。宿主只在该目录内打开文件，不跟随指向目录外的链接，并在渲染前复制一份，插件随后改写文件不影响本次渲染。

两类资源共用图片格式、单项大小、总量和处理期限限制，单次请求最多 256 项。取图失败、文件缺失或不是可接受的图片时该项保持未解析，模板原有的回退内容继续显示。

### Webhook

Webhook 路由由 manifest 静态声明。协议没有运行时暴露 webhook 的 action。宿主只按路由转发，请求头与原始正文随 `webhook.received` 投递，插件自行验签。

### 浏览器会话

- `browser.launch` 为调用插件启动或附着宿主托管的浏览器会话，宿主按插件隔离本地 profile、应用启动硬化，并限制同一 profile 同时只有一个会话；返回的 `debugger_url` 是该会话的浏览器级 CDP WebSocket 端点。
- `lifetime_seconds` 设置 1～1800 秒的会话期限，默认 1800 秒。会话在期限到期、所属插件进程退出或收到 `browser.close` 时关闭；宿主启动的浏览器进程回收完成后才释放 profile。
- 关闭会话会释放占用并保留 profile 文件；关闭失败返回错误，继续保留占用以供重试。卸载插件在会话关闭成功后删除其宿主管理 profile。
- `remote_cdp` 模式只接受无凭据的本机回环 HTTP(S) 或 WS(S) 端点。

## 终态和错误

`error` 固定包含 `code` 和 `message`，可选 `details`。插件 handler panic、重复终态、未知 action、移除字段或错误 envelope 都会被转换为正式插件错误并记录脱敏诊断。

Go SDK 提供 `event.SendText`、`event.Send`、`event.Reply`、`event.Result` 和 `event.Fail` 终态 helper，以及 `event.Actions()` 非终态 action helper。每个事件只能成功发送一次终态。

## 并发与顺序

- 生效并发度为 `min(manifest.concurrency, runtime.max_concurrent_tasks_per_plugin)`，最小为 `1`。
- 同插件、同 `event.target.type + ":" + event.target.id` 保持顺序。
- 不同会话可以并发。
- 无稳定 target 的事件进入独立 fallback lane。

## 相关文档

- [Architecture Overview](../architecture/README.md)
- [Plugin Manifest](./manifest.md)
- [Plugin SDK](./sdk/README.md)
