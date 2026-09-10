# Event Model

本文档说明 RayleaBot 当前的正式事件流：OneBot11 与 QQ 官方机器人的归一化事件、插件协议消息和管理面 WebSocket 事件。

正式 schema 见 `contracts/websocket-events.yaml`、`contracts/plugin-protocol.schema.json` 和 `contracts/web-api.openapi.yaml`。

## 一、OneBot11 接入边界

- 当前正式传输模式包括 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook`。
- `reverse_ws` 用于 OneBot 主动回连 RayleaBot；`forward_ws` 用于 RayleaBot 主动连接 OneBot；`http_api` 负责出站 API 调用；`webhook` 负责入站事件上报。
- 传输鉴权使用各连接方式的 `access_token`；协议快照对外暴露 `configured_transports`、`active_transports`、`transport_status` 和 `readiness_status`。
- `self_id` 会用于一致性检查；发现不一致时记录可观测告警。

## 二、OneBot11 事件归一化

### 正式支持的入站事件

| OneBot11 组合 | 统一事件类型 |
| --- | --- |
| `post_type=message, message_type=private` | `message.private` |
| `post_type=message, message_type=group` | `message.group` |
| `post_type=message_sent, message_type=private` | `message_sent.private` |
| `post_type=message_sent, message_type=group` | `message_sent.group` |
| `post_type=notice, notice_type=group_increase` | `notice.member_increase` |
| `post_type=notice, notice_type=group_decrease` | `notice.member_decrease` |
| `post_type=notice, notice_type=group_admin` | `notice.group_admin` |
| `post_type=notice, notice_type=group_ban` | `notice.group_ban` |
| `post_type=notice, notice_type=group_recall` | `notice.group_recall` |
| `post_type=notice, notice_type=group_upload` | `notice.group_upload` |
| `post_type=notice, notice_type=group_card` | `notice.group_card` |
| `post_type=notice, notice_type=group_title` | `notice.group_title` |
| `post_type=notice, notice_type=essence` | `notice.group_essence` |
| `post_type=notice, notice_type=friend_add` | `notice.friend_add` |
| `post_type=notice, notice_type=friend_recall` | `notice.friend_recall` |
| `post_type=notice, notice_type=flash_file` | `notice.flash_file` |
| `post_type=notice, notice_type=notify, sub_type=poke` | `notice.poke` |
| `post_type=notice, notice_type=notify, sub_type=poke_recall` | `notice.poke_recall` |
| `post_type=notice, notice_type=notify, sub_type=profile_like` | `notice.profile_like` |
| `post_type=notice, notice_type=notify, sub_type=input_status` | `notice.input_status` |
| `post_type=notice, notice_type=notify, sub_type=group_msg_emoji_like` | `notice.group_message_emoji_like` |
| `post_type=request, request_type=friend` | `request.friend` |
| `post_type=request, request_type=group` | `request.group` |
| `post_type=meta_event, meta_event_type=heartbeat` | `meta.heartbeat` |
| `post_type=meta_event, meta_event_type=lifecycle` | `meta.lifecycle` |

- 生命周期与心跳既作为 adapter 连接状态信号，也作为正式 `event` 投递进入插件主流程。
- 支持能力清单与已定义的事件范围一致。
- Bridge 负责事件形状校验、统一字段转换和桥接层观测；Dispatcher 负责选择可投递 runtime、按会话 lane 排队和执行插件返回的动作。
- `message_id` 表示单条消息编号，`conversation_id` 表示统一会话标识；群消息使用 `group_id`，私聊消息使用对端 `user_id`。
- OneBot 原生字段通过 `event.payload.onebot` 暴露给所有订阅插件，不需要额外 permission。它是形状固定的归一化投影（字段集由 `contracts/plugin-protocol.schema.json` 的 `payload.onebot` 闭合定义，`additionalProperties: false`），不是原始上报帧的透传，可读取 `group_id`、`user_id`、`time`、`real_id`、`message_seq`、`raw_message`、`sender`、`meta_event_type`、`interval` 和 `status` 等字段。
- `event.raw_payload` 是另一个字段，与 `payload.onebot` 无关：它承载已校验 webhook 请求的原始正文，且只在插件 manifest 声明 `event.raw_payload` permission 时出现。
- 管理面不接收上述任一原始 payload，只消费脱敏后的观测摘要和管理日志详情。
- `meta.*` 事件使用 `conversation_type=system`、`conversation_id=bot:<self_id>`、`sender_id=<self_id>`、`target.type=bot`、`target.id=<self_id>`；`event.message` 保持为空。

### 归一化链路

```plain
OneBot11 上报帧
  -> adapter 解析原始 JSON
  -> bridge 校验并映射统一事件
  -> dispatcher 选择可投递 runtime 并排队
  -> plugin runtime
  -> dispatcher 执行动作
```

## 三、QQ 官方机器人事件归一化

QQ 开放平台适配器与 OneBot11 共用同一套归一化事件与插件协议，事件来源由 `event.source_protocol=qqofficial` 与 `event.source_adapter` 标识。`source_protocol` 决定如何读取事件，`source_adapter` 是产生该事件的适配器实例 id（配置中的 `adapters[].id`）；出站回复据此路由回同一条连接，因此同一协议的多个实例不会互相串消息。

| 网关 dispatch | 统一事件类型 | 会话 |
| --- | --- | --- |
| `C2C_MESSAGE_CREATE` | `message.private` | 无独立会话标识，对端 openid 即会话 |
| `GROUP_AT_MESSAGE_CREATE` | `message.group` | `group_openid` |
| `GROUP_ADD_ROBOT` / `FRIEND_ADD` | `notice.bot_added` | `group_openid` / 对端 openid |
| `GROUP_DEL_ROBOT` / `FRIEND_DEL` | `notice.bot_removed` | 同上 |
| `GROUP_MSG_RECEIVE` / `C2C_MSG_RECEIVE` | `notice.push_enabled` | 同上 |
| `GROUP_MSG_REJECT` / `C2C_MSG_REJECT` | `notice.push_disabled` | 同上 |

消息复用现有 `message.private` 与 `message.group`，不新增事件类型：语义一致，来源由 `source_protocol` 承载。

管理类 dispatch 归一为四个中立事件类型：平台的群与单聊两套拼写描述的是同一件事，差别只在发生于哪种会话，因此八个 dispatch 收敛为四个类型，由 `conversation_type` 区分。`notice.push_disabled` 表示该会话不再接受主动推送，插件收到该事件后停止向该会话主动推送。连接生命周期 dispatch（`READY`、`RESUMED`）不投递。

平台侧与 OneBot11 的实质差异：

- 标识符是 per-bot **openid**，不是 QQ 号；同一个人在不同机器人下标识不同，不跨协议、不跨 bot 身份可移植。
- 群 @ 消息的 at 由平台剥离，只 @ 不带正文时 `content` 为空白；附件消息的 `content` 是客户端标记，真实媒体在 `attachments` 数组。两者都不作为消息文本。
- 没有消息段数组，也没有 CQ 码：纯文本加平行附件列表，附件按 `content_type` 映射到既有段类型。
- `timestamp` 有两种拼写：消息 dispatch 为 RFC3339 字符串，成员变更通知为 Unix 整数。
- 原生字段位于 `event.payload.qq_official`，形状闭合，含 dispatch 名、消息 id 与相关 openid。

出站以被动回复为主：回复引用收到的消息 id，并按 `msg_seq` 递增；主动推送有配额限制。回复目标保留来源协议、adapter 和 bot 身份，自动路由到对应实例。主动推送通过 `message.send.source_adapter` 指定实例；只提供 `source_protocol` 时，该协议须恰好有一个已启用实例；二者都省略时，必须全局只有一个已启用实例。路由不明确时拒绝发送，避免把同名目标投递到其他连接。目标配额和熔断也按完整来源身份隔离。

富媒体分两步：先上传到 `/v2/groups/{group_openid}/files` 或 `/v2/users/{openid}/files` 取得 `file_info`，再以 `msg_type=7` 的 `media` 字段发送。本机文件（含渲染服务产出的 `file://` 地址）以 base64 通过 `file_data` 直传，不需要公网可达地址；`http(s)` 地址交由平台自行拉取。群与单聊的上传通道彼此独立，上传时不置 `srv_send_msg`，避免在发送前消耗主动推送额度。平台每条消息只承载一个媒体，因此图文消息会拆成先文字后媒体的连续发送。

## 四、插件协议消息

### 生命周期消息

| 方向 | 类型 | 作用 |
| --- | --- | --- |
| server -> plugin | `init` | 启动握手 |
| plugin -> server | `init_progress` | 可选启动进度 |
| plugin -> server | `init_ack` | 握手完成 |
| server -> plugin | `ping` | 保活探测 |
| plugin -> server | `pong` | 保活响应 |
| server -> plugin | `shutdown` | 优雅退出指令 |

### 事件与结果

| 类型 | 说明 |
| --- | --- |
| `event` | 平台向插件投递统一事件 |
| `result` | 插件对事件或 action 的成功响应 |
| `error` | 插件对事件或 action 的失败响应 |
| `action` | 插件发起本地 action 请求；平台返回 `result` 或 `error` |

- 本地 action 使用独立 `request_id`，并通过 `parent_request_id` 归属到对应事件。
- manifest 省略 `concurrency` 时，插件按串行事件处理；显式声明后，同一 `event.target` 保持顺序，不同 `event.target` 可并发。

### Local action 能力族

- 平台能力族包括消息、日志、存储、HTTP、配置、secret、插件目录、三方账号、治理、调度、Webhook 和渲染。
- OneBot 动作族覆盖消息读取与管理、好友与用户、群治理、文件、reaction 与 poke；provider 另有受控扩展动作。
- 人类可读的完整 action 名称与参数清单只维护在[插件协议](../plugin/protocol.md#action-rpc)，机器可读结构以 `contracts/plugin-protocol.schema.json` 为准。
- 平台内部事件（不经 Bridge，直接进入 Dispatcher）：`scheduler.trigger`、`plugin.started`、`config.changed`、`webhook.received`、`bot.identities.changed`、`management.action`。

### 当前正式消息段

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

## 五、管理 WebSocket 事件

| 频道 | 路径 | 事件 |
| --- | --- | --- |
| `logs` | `/ws/logs` | `logs.appended` |
| `events` | `/ws/events` | `events.received` |
| `plugin_console` | `/ws/plugins/{id}/console` | `plugins.console` |

- 管理面 WebSocket 使用统一 envelope（`channel` / `type` / `timestamp` / `data`），负责日志追加、平台观测事件和插件 console。
- 异步任务更新通过 `logs.appended` 的 `source=tasks` 日志呈现。
- `/ws/events` 的 `events.received` 复用同一个事件名，通过 payload 分支表达不同观测语义：

- `service_status`：服务总体状态变化摘要
- `plugin_id` + `state` + `commands` + `command_conflicts` + 可选 `state_diagnosis`：插件生命周期状态展示
- `connection_status`：OneBot 连接状态摘要
- `event_type` + `summary`：通用管理事件（当前包括 `governance.changed` 与 `third_party.account.changed`）
- `adapters`：按配置顺序推送完整适配器实例集合；OneBot11 实例的协议快照位于该实例的 `onebot11` 字段
- `observability_scope` = `bridge_runtime` 时的聚合观测摘要
- `observability_scope` = `dispatcher_runtime` 时的 dispatcher 窗口统计摘要

会话失效时，连接会下发 `session_expired` session event，客户端必须换新 token 重连。
