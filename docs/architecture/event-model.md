# Event Model

本文档说明 RayleaBot 当前三条正式事件流：OneBot11 归一化事件、插件协议消息和管理面 WebSocket 事件。

正式 schema 见 `contracts/websocket-events.yaml`、`contracts/plugin-protocol.schema.json` 和 `contracts/web-api.openapi.yaml`。

## 一、OneBot11 接入边界

- v0.1 只支持 OneBot11 反向 WebSocket。
- 默认使用 Header 方式附加 `access_token`，并兼容查询参数方式。
- 未支持的传输模式在启动前直接拒绝，不进入运行态。
- `self_id` 会用于一致性检查；发现不一致时记录可观测告警。

## 二、OneBot11 事件归一化

### 正式支持的入站事件

| OneBot11 组合 | 统一事件类型 |
| --- | --- |
| `post_type=message, message_type=private` | `message.private` |
| `post_type=message, message_type=group` | `message.group` |
| `post_type=notice, notice_type=group_increase` | `notice.member_increase` |
| `post_type=notice, notice_type=group_decrease` | `notice.member_decrease` |
| `post_type=meta_event, meta_event_type=heartbeat` | `meta.heartbeat` |
| `post_type=meta_event, meta_event_type=lifecycle` | `meta.lifecycle` |

- 未进入正式范围的事件不会伪装成已支持能力。
- 归一化后事件进入 bridge，再由 dispatcher 投递到订阅该事件的插件 runtime。

### 归一化链路

```plain
OneBot11 WS 上报
  -> adapter 解析原始 JSON
  -> bridge 映射统一事件
  -> dispatcher 投递到插件 runtime
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
| `action_request` / `action_response` | 本地 action RPC |

### 当前正式 local action

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

### 当前正式消息段

- `text`
- `image`
- `at`
- `at_all`
- `face`
- `reply`

## 四、管理 WebSocket 事件

| 频道 | 事件 |
| --- | --- |
| `/ws/tasks` | `tasks.updated` |
| `/ws/logs` | `logs.appended` |
| `/ws/events` | `events.received` |
| `/ws/plugins/{id}/console` | `plugins.console` |

管理面 WebSocket 使用统一 envelope，承载任务更新、日志追加、平台观测事件和插件 console。
