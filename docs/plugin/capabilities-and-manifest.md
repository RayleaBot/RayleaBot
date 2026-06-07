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
| `management_ui` | 插件详情页内置管理页入口 |
| `capabilities` | 插件声明的完整平台能力画像 |
| `permissions` | capability 与作用域声明 |
| `commands` | 插件命令声明 |
| `dependencies` | 语言级依赖说明 |
| `icon` / `repo` / `homepage` / `keywords` / `screenshots` / `platforms` / `system_dependencies` | 展示、来源与平台约束元数据 |

## 正式 capability 集合

`capabilities`、`permissions.required` 和 `permissions.optional` 共用同一套正式 capability 名称。

机器可读的完整枚举以 `contracts/plugin-info.schema.json` 的 `capability_name` 定义为准。本页列出面向插件开发者的常用分类；local action 的请求结构和返回结构见 [Protocol](./protocol.md)，SDK helper 覆盖范围见 [Plugin SDK Docs](./sdk/README.md)。

## 声明能力与授权口径

`capabilities` 表示插件可能使用的平台能力集合，用于安装校验、兼容性判断和管理面展示。

`permissions.required` 和 `permissions.optional` 表示进入授权模型的 capability 需求。管理面“权限与授权”列表来自这两个字段，显示必需/可选、已授权/未授权、授权来源和有效期。

两者数量可以不同：

- `capabilities` 可以包含不需要单独授权决策的能力，例如 `event.subscribe`
- 只写入 `capabilities` 不会生成授权记录，也不会进入管理面的“权限与授权”列表
- 需要平台授予、运行时拦截、撤销或作用域限制的能力应放入 `permissions.required` 或 `permissions.optional`
- `permissions.required` 缺少有效授权时，插件启用、重载、恢复和崩溃恢复会返回 `plugin.permission_pending`
- `permissions.optional` 默认不阻止插件启用，插件使用对应能力前仍需要有效授权
- `permissions.scopes` 只约束进入授权模型的能力，当前覆盖 `http.request`、`storage.file` 和 `event.expose_webhook`

管理面中的“声明能力”显示 `capabilities`；“权限与授权”显示 `permissions.required` 与 `permissions.optional` 的授权摘要。

## 插件开发者声明规则

`capabilities` 写入插件会使用的完整平台能力集合。插件代码、SDK helper、local action、OneBot 单动作、provider 扩展动作和高敏事件字段都会映射到正式 capability 名称。

`permissions.required` 和 `permissions.optional` 只写入需要授权的能力。当前正式能力里，`event.subscribe` 只用于事件订阅画像，通常只写入 `capabilities`；其余能力一旦要在运行时真正使用，都应进入 `permissions.required` 或 `permissions.optional`：

- 插件核心功能离不开的能力写入 `permissions.required`
- 缺少后仍可运行、只影响增强功能或可延后开启的能力写入 `permissions.optional`
- 仅用于说明插件参与平台事件流、但没有独立授权决策的能力只写入 `capabilities`
- 需要授权的能力如果只写入 `capabilities`，插件可通过 manifest 校验，但运行时使用该能力会被 `permission.scope_violation` 拒绝，除非该能力通过配置自动授权或手动 grant 获得有效授权
- `permissions.required` 和 `permissions.optional` 中的能力同时保留在 `capabilities`，让插件详情页展示完整能力画像

声明规则：

| 插件行为 | capability | `capabilities` | `permissions.required / optional` |
| --- | --- | --- | --- |
| 接收平台分发的常规事件 | `event.subscribe` | 需要 | 通常不需要 |
| 读取原始事件载荷 | `event.raw_payload` | 需要 | 需要；按功能重要性放入 `required` 或 `optional` |
| 发送或回复消息 | `message.send` / `message.reply` | 需要 | 需要；核心回复能力通常放入 `required` |
| 调用通用 local action | action kind，例如 `logger.write`、`storage.kv`、`config.read` | 需要 | 需要；按功能重要性放入 `required` 或 `optional` |
| 调用 OneBot 单动作 | action kind，例如 `message.history.get`、`group.member.list` | 需要 | 需要；按功能重要性放入 `required` 或 `optional` |
| 调用 provider 扩展动作 | provider action kind | 需要 | 需要；按功能重要性放入 `required` 或 `optional` |
| 发起 HTTP 请求 | `http.request` | 需要 | 需要，并声明 `permissions.scopes.http_hosts` |
| 读写插件文件 | `storage.file` | 需要 | 需要，并声明 `permissions.scopes.storage_roots` |
| 暴露 Webhook 入口 | `event.expose_webhook` | 需要 | 需要，并声明 `permissions.scopes.webhooks` |
| 只声明命令、帮助、依赖、截图、管理页等元数据 | 无 | 不需要 | 不需要 |

示例：

```json
{
  "capabilities": [
    "event.subscribe",
    "message.send",
    "http.request"
  ],
  "permissions": {
    "required": [
      "message.send"
    ],
    "optional": [
      "http.request"
    ],
    "scopes": {
      "http_hosts": [
        "api.example.com"
      ]
    }
  }
}
```

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
- `secret.read`
- `governance.blacklist.read`
- `governance.blacklist.write`
- `governance.whitelist.read`
- `governance.whitelist.write`
- `governance.command_policy.read`
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

- 插件可通过 `commands` 声明命令名、别名、说明、示例和权限级别；静态命令名和别名使用 UTF-8 非空文本，不能包含空白字符
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

## 插件内置管理页

- `management_ui.pages` 声明插件详情页内的管理页签，至少包含一个页面。
- `pages[].id` 是稳定页签标识，`pages[].label` 是页签标题，`pages[].entry` 是插件包内的 HTML 文件路径。
- 同一插件的所有 `pages[].entry` 必须位于同一目录。
- 插件详情页在概览之外提供同一插件的内置管理页工作区，不提供第二个插件一级页面。
- 插件内置页面通过 `/plugin-ui/{plugin_id}/...` 读取自身静态资源。
- 插件内置页面只通过正式桥接消息读取和保存插件自己的设置，不直接持有管理会话。
- 当前设置由 `default_config` 叠加已保存配置得到；保存成功后宿主会同步刷新插件详情中的配置预览。
- 未验证来源插件首次打开内置管理页需要人工确认；确认记录会随插件版本或来源变化失效。

## 相关文档

- [Plugin Lifecycle](./lifecycle.md)
- [Management UI](./management-ui.md)
- [Protocol](./protocol.md)
- [Plugin SDK Docs](./sdk/README.md)
