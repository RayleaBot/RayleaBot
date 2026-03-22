# Plugin Docs

本目录用于说明 RayleaBot 插件平台的当前边界、生命周期和开发语义。

## 当前插件平台

RayleaBot 当前已经接入以下插件主链路：

- `contracts/plugin-info.schema.json` 驱动的插件静态校验与 discovery
- `plugins/builtin`、`examples/plugins`、`plugins/installed` 三个 discovery roots
- builtin plugin 默认发现、默认启用、允许 enable / disable / reload，拒绝卸载
- per-plugin runtime manager、`init -> init_ack` 启动握手、`ping/pong` 保活、`shutdown` 优雅停止
- dispatcher 订阅 fan-out、命令定向投递与 scheduler `scheduler.trigger`
- rich `message.send` / `message.reply` 与 shared `message.segments` 出站模型

## 当前正式边界

- 插件 manifest 与 runtime JSONL 协议以 `contracts/plugin-info.schema.json` 和 `contracts/plugin-protocol.schema.json` 为准。
- 当前正式 `action` 集合包含 richer `message.send`、richer `message.reply` 与 legacy compatibility `message.send_image`。
- 当前正式 outbound segment 种类为 `text`、`image`、`at`、`at_all`、`face`、`reply`。
- grants storage、scope 校验、temporal grants 与 enable / reload / reconcile / restart 前权限门禁已接入正式行为。
- 聊天侧 blacklist、命令权限、cooldown 与可选 cooldown reply 已进入 live command path。

## 维护规则

- 本目录用于解释插件开发语义、能力边界与生命周期，不替代正式 contract。
- 插件不得绕过 Capability 校验、协议约束或跨层访问平台内部模块。
- 若协议、manifest 字段或动作集合发生变化，先更新 `contracts/`，再同步 SDK、fixtures、示例与本目录说明。
