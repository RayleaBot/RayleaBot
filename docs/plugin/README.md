# Plugin Docs

本目录用于说明 RayleaBot 插件平台的当前边界、生命周期和开发语义。

## 当前插件平台

RayleaBot 当前已经接入以下插件主链路：

- `contracts/plugin-info.schema.json` 驱动的插件静态校验与 discovery
- `plugins/builtin`、`examples/plugins`、`plugins/installed` 三个 discovery roots
- builtin plugin 默认发现、默认启用、允许 enable / disable / reload，拒绝卸载
- per-plugin runtime manager、`init -> init_progress -> init_ack` 启动握手（`init_progress` 为可选进度上报）、`ping/pong` 保活、`shutdown` 优雅停止
- dispatcher 订阅 fan-out、命令定向投递与 scheduler `scheduler.trigger`
- `message.send` / `message.reply` 与 shared `message.segments` 出站模型
- local action RPC：`logger.write`、`storage.kv`、`storage.file`、`http.request`、`config.read`、`config.write`、`scheduler.create`、`event.expose_webhook`、`render.image`
- plugin-scoped KV persistence、plugin_data 文件区与 management log integration

## 当前正式边界

- 插件 manifest 与 runtime JSONL 协议以 `contracts/plugin-info.schema.json` 和 `contracts/plugin-protocol.schema.json` 为准。
- 当前正式 `action` 集合包含 `message.send`、`message.reply`、`logger.write`、`storage.kv`、`storage.file`、`http.request`、`config.read`、`config.write`、`scheduler.create`、`event.expose_webhook`与 `render.image`。
- 当前正式 outbound segment 种类为 `text`、`image`、`at`、`at_all`、`face`、`reply`。
- grants storage、scope 校验、temporal grants 与 enable / reload / reconcile / restart 前权限门禁已接入正式行为。
- 聊天侧 blacklist、命令权限、cooldown 与可选 cooldown reply 已进入 live command path。
- `storage.kv` 当前正式支持 `get` / `set` / `delete` / `list(prefix)`，并按插件隔离命名空间。
- `storage.file` 当前正式支持 `read` / `write` / `delete` / `list(prefix)`，范围固定在 `plugin_data` root，并拒绝绝对路径、逃逸路径与 symlink 穿透。
- `http.request` 当前正式支持 `GET` / `HEAD` / `POST` / `PUT` / `PATCH` / `DELETE`，目标 host 必须同时满足 `http_hosts` scope、DNS 解析校验与受控私网例外规则。
- `logger.write` 当前进入 management log surface，并复用现有脱敏、持久化与查询链路。

## 维护规则

- 本目录用于解释插件开发语义、能力边界与生命周期，不替代正式 contract。
- 插件不得绕过 Capability 校验、协议约束或跨层访问平台内部模块。
- 若协议、manifest 字段或动作集合发生变化，先更新 `contracts/`，再同步 SDK、fixtures、示例与本目录说明。
