# Event Model

本文档说明 RayleaBot 中三条事件流的数据模型与生命周期。

正式 schema 见 `contracts/websocket-events.yaml`、`contracts/plugin-protocol.schema.json`、`contracts/web-api.openapi.yaml`。

---

## 一、OneBot11 事件归一化

OneBot11 上报的事件为 JSON 对象，经 adapter 最小归一化后进入 bridge。bridge 做平台到内部协议的映射。

### 归一化阶段

```
OneBot11 WS 上报
  ↓  adapter: 接收原始 JSON
  ↓  intake: 解析 post_type / message_type / notice_type / sub_type
  ↓  bridge: 映射为 plugin protocol event 消息
  ↓  dispatcher: 投递到订阅该事件的插件 runtime worker
```

消息以外的事件（notice、request、meta_event）也按同样路径流转。adapter 会记录 `valid_count`、`invalid_count`、`conflict_count` 指标。

---

## 二、Plugin Protocol 消息模型

插件子进程与 server runtime manager 之间通过 JSONL（per-line JSON）协议通信。每条消息为一个 JSON 对象，以换行结束。

正式 schema 见 `contracts/plugin-protocol.schema.json`。

### 生命周期时序

```
server → plugin: init          # 传递配置快照与 granted capabilities
plugin → server: init_progress  # 可选，0~100% 进度上报（用于长 init 场景）
plugin → server: init_ack       # 握手就绪，runtime 进入 running
─── running ───
server → plugin: event         # OneBot11 事件投递
plugin → server: result        # 投递成功响应
plugin → server: error         # 投递失败响应，含 error_code
─── local action RPC ───
plugin → server: action_request  # 调用 local action（如 message.send）
server → plugin: action_response # local action 响应
─── keepalive ───
server → plugin: ping
plugin → server: pong
─── shutdown ───
server → plugin: shutdown
plugin → server: result/error (可选，发完即退)
```

### 消息类型汇总

| 方向 | 类型 | 说明 |
|------|------|------|
| server→plugin | `init` | 启动握手，含 capabilities、config_snapshot |
| plugin→server | `init_progress` | 可选启动进度上报，含 `percent`（0–100）|
| plugin→server | `init_ack` | 握手完成 |
| server→plugin | `event` | OneBot11 事件投递，含 `request_id` |
| plugin→server | `result` | 事件/action 成功响应，含 `request_id` |
| plugin→server | `error` | 事件/action 失败响应，含 `request_id`、`error_code` |
| plugin→server | `action_request` | 调用 local action，含 `action`、`params` |
| server→plugin | `action_response` | local action 响应，含 `ok`、`data`/`error` |
| server→plugin | `ping` | keepalive 探测 |
| plugin→server | `pong` | keepalive 响应 |
| server→plugin | `shutdown` | 优雅关闭指令 |

### Local Actions

以下 local action 可通过 `action_request` 调用。每项均经过 capability 校验。

| action | 说明 | 所需 capability |
|--------|------|-----------------|
| `message.send` | 发送消息到指定会话 | `message.send` |
| `message.reply` | 引用回复 | `message.reply` |
| `storage.kv` | KV 读写（`get`/`set`/`delete`/`list` 子命令） | `storage.kv` |
| `storage.file` | 文件区读写（`read`/`write`/`delete`/`list` 子命令） | `storage.file` |
| `logger.write` | 向 server 日志写入一条插件日志 | 始终可用 |
| `http.request` | 发起受 `http_hosts` 限制的出站 HTTP 请求 | `http.request` |
| `config.read` | 读取插件自身配置快照 | `config.read` |
| `config.write` | 写入插件配置字段（须通过 schema 校验） | `config.write` |
| `scheduler.create` | 注册 cron 调度任务 | `scheduler.create` |
| `event.expose_webhook` | 注册 Webhook 端点并接收入站事件 | `event.expose_webhook` |
| `render.image` | 提交渲染任务，返回 artifact ID | `render.image` |

### 出站消息 Segment 类型

`message.send` / `message.reply` 的 `segments` 数组支持以下 segment 类型：

| 类型 | 说明 |
|------|------|
| `text` | 纯文本 |
| `image` | 图片（URL 或 base64） |
| `at` | @某人（user_id） |
| `at_all` | @全体成员 |
| `face` | QQ 表情（face_id） |
| `reply` | 引用回复（message_id） |

---

## 三、WebSocket 管理事件

管理面 WebSocket 频道用于向连接的客户端推送增量状态。

正式 schema 见 `contracts/websocket-events.yaml`。

### 频道与事件类型

| 频道 | 事件类型 | 说明 |
|------|----------|------|
| `/ws/tasks` | `tasks.updated` | 任务状态变更（status、progress、result、error） |
| `/ws/logs` | `logs.appended` | 新增 management log 条目 |
| `/ws/events` | `events.received` | OneBot11 事件 / 平台状态变化（含 adapter 连接状态、插件 runtime 状态） |
| `/ws/plugins/{id}/console` | `plugins.console` | 插件 stderr 实时日志行 |
| 所有频道（session 层） | `session_expired` | 当前 session 已被服务端失效，客户端须重新登录 |

### 统一信封格式

所有 WebSocket 消息均包裹在统一 envelope 中：

```json
{
  "type": "<event_type>",
  "seq": 42,
  "payload": { ... }
}
```

- `type`：事件类型字符串（见上表）
- `seq`：单频道单调递增序号，客户端可用于检测跳号
- `payload`：事件内容，具体结构见 `contracts/websocket-events.yaml`

### `events.received` payload 说明

`events.received` 事件承载多类子事件，通过 `kind` 字段区分：

| `kind` | 说明 |
|--------|------|
| `onebot_event` | 归一化后的 OneBot11 事件（仅 management 可见） |
| `plugin_state` | 插件 runtime 状态变更 |
| `connection_status` | adapter WebSocket 连接状态变更 |
| `bridge_runtime_observability` | bridge 层可观测性统计（valid/invalid/conflict 计数等） |
