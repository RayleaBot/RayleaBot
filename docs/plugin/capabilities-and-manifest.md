# Capabilities and Manifest

本页说明 RayleaBot 插件 manifest 的正式结构、能力声明和授权边界。

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
| `permissions` | capability 与作用域声明 |
| `commands` | 插件命令声明 |
| `dependencies` | 语言级依赖说明 |
| `icon` / `repo` / `homepage` / `keywords` / `screenshots` / `platforms` / `system_dependencies` | 展示、来源与平台约束元数据 |

## 正式 capability 集合

`capabilities`、`permissions.required` 和 `permissions.optional` 共用同一套正式 capability 名称。

### 基础 capability

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
- `scheduler.create`
- `event.expose_webhook`
- `render.image`
- `plugin.list`

### OneBot 单动作 capability

能力名称直接等于正式 action kind：

- 消息：`message.get`、`message.delete`、`message.history.get`、`message.forward.get`、`message.forward.send`、`message.read.mark`
- 好友与用户：`friend.request.handle`、`friend.list`、`friend.remark.set`、`user.info.get`、`user.like.send`
- 群：`group.list`、`group.info.get`、`group.member.get`、`group.member.list`、`group.request.handle`、`group.leave`、`group.admin.set`、`group.ban.set`、`group.card.set`、`group.title.set`、`group.name.set`
- 群扩展：`group.announcement.list`、`group.announcement.create`、`group.announcement.delete`、`group.essence.list`、`group.essence.set`、`group.essence.unset`、`group.honor.get`、`group.todo.set`
- 文件：`file.get`、`file.download`、`file.group.upload`、`file.private.upload`、`file.group.url.get`、`file.private.url.get`、`file.group.fs.info`、`file.group.fs.list`、`file.group.fs.mkdir`、`file.group.fs.delete`
- 互动：`reaction.set`、`reaction.list`、`poke.send`

### provider 扩展 capability

- `provider.napcat.message_emoji.like.set`
- `provider.napcat.group.sign.set`
- `provider.luckylillia.friend_groups.get`

## 授权与作用域

- 插件通过 `permissions.required` 和 `permissions.optional` 声明 capability 需求
- 官方内置插件的声明权限会自动生效，并在授权来源中显示为 `builtin_auto`
- 平台按 capability grant 模型保存授权结果，并在启用、重载、恢复和崩溃恢复前重新过滤时效窗口
- 当前授权列表会合并内置自动授权、配置自动授权和持久化授权
- manifest 作用域与已保存授权不一致时，启用和重载返回 `plugin.permission_pending`；管理面可对 `persisted` 授权重新确认后继续启用
- 作用域约束通过 `permissions.scopes` 声明，当前正式范围包括 `http_hosts`、`storage_roots` 和 `webhooks`
- `storage_roots` 正式范围固定为 `plugin_data`
- OneBot 单动作 capability 与当前 provider capability 只做 capability 校验，不新增额外 scope 类型

## 命令声明

- 插件可通过 `commands` 声明命令名、别名、说明、示例和权限级别
- 平台保留 `raylea:*` 命名空间给官方内置插件
- 同名命令默认保持 fan-out；管理面负责提示冲突

## 并发声明

- `concurrency` 只定义事件处理并发度
- 插件有效并发度取 `min(manifest.concurrency, runtime.max_concurrent_tasks_per_plugin)`，最小值为 `1`
- 同一插件内按 `event.target.type + ":" + event.target.id` 保持同会话顺序；不同会话可并发
- 没有稳定 `event.target` 的事件使用独立 fallback lane
- 插件详情页显示 `concurrency`、`default_config`、`declared_capabilities`、`dependencies` 和 `scopes`

## 依赖与发布边界

- `dependencies` 只覆盖语言级依赖，不覆盖插件间依赖
- manifest 字段和语义变化先进入 contract，再同步 SDK、fixtures、示例和管理面

## 相关文档

- [Plugin Lifecycle](./lifecycle.md)
- [Protocol](./protocol.md)
- [Plugin SDK Docs](./sdk/README.md)
