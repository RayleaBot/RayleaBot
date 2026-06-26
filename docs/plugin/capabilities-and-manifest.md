# Capabilities and Manifest

本页说明 RayleaBot 插件 manifest 的正式结构、能力声明和能力参数边界。

正式 schema 以 `contracts/plugin-info.schema.json` 为准。

## Manifest 核心字段

| 字段 | 含义 |
| --- | --- |
| `id` / `name` / `version` | 插件身份与版本 |
| `manifest_version` / `plugin_protocol_version` | manifest 与协议版本 |
| `type` | `managed_runtime` 或 `dev_source` |
| `runtime` | `python` 或 `nodejs` |
| `entry` | 插件入口 |
| `concurrency` | 事件并发度声明；省略时按 `1` 处理 |
| `role` | `builtin` / `user` / `example` / `dev` |
| `default_config` | 插件默认配置 |
| `management_ui` | 插件详情页内置管理页入口 |
| `capabilities` | 插件声明的平台能力集合 |
| `capability_parameters` | HTTP 主机、文件存储根和 Webhook 路由边界 |
| `commands` | 插件命令声明 |
| `dependencies` | 语言级依赖说明 |
| `icon` / `repo` / `homepage` / `keywords` / `screenshots` / `platforms` / `system_dependencies` | 展示、来源与平台约束元数据 |

## 正式 capability 集合

`capabilities` 使用同一套正式 capability 名称。机器可读的完整枚举以 `contracts/plugin-info.schema.json` 的 `capability_name` 定义为准。

`capabilities` 同时用于安装校验、兼容性判断、插件详情展示和运行时 local action 检查。插件调用未声明 capability，或超出 `capability_parameters` 边界时，平台返回 `plugin.capability_violation`。

local action 的请求结构和返回结构见 [Protocol](./protocol.md)，SDK helper 覆盖范围见 [Plugin SDK Docs](./sdk/README.md)。

## 插件开发者声明规则

插件代码、SDK helper、local action、OneBot 单动作、provider 扩展动作和高敏事件字段都会映射到正式 capability 名称。

声明规则：

| 插件行为 | capability | `capabilities` | `capability_parameters` |
| --- | --- | --- | --- |
| 接收平台分发的常规事件 | `event.subscribe` | 需要 | 无 |
| 读取原始事件载荷 | `event.raw_payload` | 需要 | 无 |
| 发送或回复消息 | `message.send` / `message.reply` | 需要 | 无 |
| 调用通用 local action | action kind，例如 `logger.write`、`storage.kv`、`config.read` | 需要 | 按 action 需要声明 |
| 读取三方账号 CK | `thirdparty.account.read` | 需要 | `third_party_account_platforms` |
| 调用 OneBot 单动作 | action kind，例如 `message.history.get`、`group.member.list` | 需要 | 无 |
| 调用 provider 扩展动作 | provider action kind | 需要 | 无 |
| 发起 HTTP 请求 | `http.request` | 需要 | `http_hosts` |
| 读写插件文件 | `storage.file` | 需要 | `storage_roots` |
| 暴露 Webhook 入口 | `event.expose_webhook` | 需要 | `webhooks` |
| 只声明命令、帮助、依赖、截图、管理页等元数据 | 无 | 无 | 无 |

示例：

```json
{
  "capabilities": [
    "event.subscribe",
    "message.send",
    "http.request",
    "storage.file"
  ],
  "capability_parameters": {
    "http_hosts": [
      "api.example.com"
    ],
    "storage_roots": [
      "cache"
    ]
  }
}
```

## 能力参数

`capability_parameters` 只表达运行边界参数，当前正式范围包括：

- `http_hosts`：`http.request` 可访问的主机名列表。平台仍执行全局 HTTP 超时、重试、DNS 预检、SSRF 防护和私网主机限制。
- `storage_roots`：`storage.file` 可访问的插件文件根目录列表。平台仍执行路径穿越、符号链接和插件工作目录配额校验。
- `third_party_account_platforms`：`thirdparty.account.read` 可读取的三方平台列表。平台只返回已保存、已启用且非 invalid 的账号，CK 以 secret 值标记返回。
- `webhooks`：`event.expose_webhook` 可暴露的路由列表。每个路由可声明签名要求、允许来源和重放窗口。

## 基础 capability

- `event.subscribe`
- `event.raw_payload`
- `message.send`
- `message.reply`
- `logger.write`
- `storage.kv`
- `storage.file`
- `http.request`
- `config.read`
- `config.write`
- `secret.read`
- `thirdparty.account.read`
- `governance.blacklist.read`
- `governance.blacklist.write`
- `governance.whitelist.read`
- `governance.whitelist.write`
- `governance.command_policy.read`
- `scheduler.create`
- `event.expose_webhook`
- `render.image`
- `plugin.list`

## OneBot 单动作 capability

能力名称直接等于正式 action kind：

- 消息：`message.get`、`message.delete`、`message.history.get`、`message.forward.get`、`message.forward.send`、`message.read.mark`
- 好友与用户：`friend.request.handle`、`friend.list`、`friend.remark.set`、`user.info.get`、`user.like.send`
- 群：`group.list`、`group.info.get`、`group.member.get`、`group.member.list`、`group.request.handle`、`group.leave`、`group.admin.set`、`group.ban.set`、`group.card.set`、`group.title.set`、`group.name.set`
- 群扩展：`group.announcement.list`、`group.announcement.create`、`group.announcement.delete`、`group.essence.list`、`group.essence.set`、`group.essence.unset`、`group.honor.get`、`group.todo.set`
- 文件：`file.get`、`file.download`、`file.group.upload`、`file.private.upload`、`file.group.url.get`、`file.private.url.get`、`file.group.fs.info`、`file.group.fs.list`、`file.group.fs.mkdir`、`file.group.fs.delete`
- 互动：`reaction.set`、`reaction.list`、`poke.send`

## provider 扩展 capability

- `provider.napcat.message_emoji.like.set`
- `provider.napcat.group.sign.set`
- `provider.luckylillia.friend_groups.get`

## 命令声明

- 插件可通过 `commands` 声明命令名、别名、说明、示例和权限级别；静态命令名和别名使用 UTF-8 非空文本，不能包含空白字符。
- 平台保留 `raylea:*` 命名空间给官方内置插件。
- 同名命令默认保持 fan-out；管理面负责提示冲突。
- 聊天命令权限治理使用 `command.permission`、`permission.default_level`、黑白名单、冷却和超级管理员配置。

## 并发声明

- `concurrency` 只定义事件处理并发度。
- 插件有效并发度取 `min(manifest.concurrency, runtime.max_concurrent_tasks_per_plugin)`，最小值为 `1`。
- 同一插件内按 `event.target.type + ":" + event.target.id` 保持同会话顺序；不同会话可并发。
- 没有稳定 `event.target` 的事件使用独立 fallback lane。
- 插件详情页显示 `concurrency`、`default_config`、`declared_capabilities`、`dependencies` 和 `capability_parameters`。

## 依赖与发布边界

- `dependencies` 只覆盖语言级依赖，不覆盖插件间依赖。
- manifest 字段和语义变化先进入 contract，再同步 SDK、fixtures、示例和管理面。

## 插件内置管理页

- `management_ui.pages` 声明插件详情页内的管理页签，至少包含一个页面。
- `pages[].id` 是稳定页签标识，`pages[].label` 是页签标题，`pages[].entry` 是插件包内的 HTML 文件路径。
- 同一插件的所有 `pages[].entry` 必须位于同一目录。
- 插件详情页在概览之外提供同一插件的内置管理页工作区。
- 插件内置页面通过 `/plugin-ui/{plugin_id}/...` 读取自身静态资源。
- 插件内置页面只通过正式桥接消息读取和保存插件自己的设置，不直接持有管理会话。
- 当前设置由 `default_config` 叠加已保存配置得到；保存成功后宿主会同步刷新插件详情中的配置预览。
- 未验证来源插件首次打开内置管理页需要人工确认；确认记录会随插件版本或来源变化失效。

## 相关文档

- [Plugin Lifecycle](./lifecycle.md)
- [Management UI](./management-ui.md)
- [Protocol](./protocol.md)
- [Plugin SDK Docs](./sdk/README.md)
