# Event Model

本文档说明 RayleaBot 当前三条正式事件流：OneBot11 归一化事件、插件协议消息和管理面 WebSocket 事件。

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

- 生命周期与心跳既作为 adapter 连接状态信号，也作为正式 `event` 投递进入插件主链。
- 未进入正式范围的事件不会伪装成已支持能力。
- Bridge 负责事件形状校验、统一字段转换和桥接层观测；Dispatcher 负责选择可投递 runtime、按会话 lane 排队和执行插件返回的动作。
- `message_id` 表示单条消息编号，`conversation_id` 表示统一会话标识；群消息使用 `group_id`，私聊消息使用对端 `user_id`。
- OneBot 原生字段通过 `event.payload.onebot` 正式暴露，插件和管理面都可以直接读取 `group_id`、`user_id`、`time`、`real_id`、`message_seq`、`raw_message`、`sender`、`meta_event_type`、`interval` 和 `status` 等字段。
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

## 三、插件协议消息

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

### 当前正式 local action

- 平台基础动作包括 `message.send`、`message.reply`、`logger.write`、`storage.kv`、`storage.file`、`http.request`、`config.read`、`config.write`、`secret.read`、`plugin.list`、`scheduler.create`、`event.expose_webhook`、`render.image` 与 `governance.*`。
- OneBot generic action 覆盖消息读取与管理、好友与用户、群治理、文件、reaction 与 poke 等家族。
- Provider 扩展动作固定为 `provider.napcat.message_emoji.like.set`、`provider.napcat.group.sign.set` 和 `provider.luckylillia.friend_groups.get`。
- 平台内部事件（不经 Bridge，直接进入 Dispatcher）：`scheduler.trigger`、`config.changed`、`webhook.received`、`bot.identity.changed`、`management.action`。

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

## 四、管理 WebSocket 事件

| 频道 | 路径 | 事件 |
| --- | --- | --- |
| `tasks` | `/ws/tasks` | `tasks.updated` |
| `logs` | `/ws/logs` | `logs.appended` |
| `events` | `/ws/events` | `events.received` |
| `plugin_console` | `/ws/plugins/{id}/console` | `plugins.console` |

管理面 WebSocket 使用统一 envelope（`channel` / `type` / `timestamp` / `data`），承载任务更新、日志追加、平台观测事件和插件 console。`/ws/events` 的 `events.received` 复用同一个事件名，通过 payload 分支表达不同观测语义：

- `service_status`：服务总体状态变化摘要
- `plugin_id` + `state` + `commands` + `command_conflicts` + 可选 `state_diagnosis`：插件生命周期状态投影
- `connection_status`：OneBot 连接状态摘要
- `event_type` + `summary`：通用管理事件（当前包括 `governance.changed`）
- `protocol` + `protocol_snapshot`：OneBot11 协议快照推送
- `observability_scope` = `bridge_runtime` 时的聚合观测摘要
- `observability_scope` = `dispatcher_runtime` 时的 dispatcher 窗口统计摘要

会话失效时，连接会下发 `session_expired` session event，客户端必须换新 token 重连。
