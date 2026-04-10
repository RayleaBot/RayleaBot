# Capabilities and Manifest

本页说明 RayleaBot 插件 manifest 的当前结构、能力声明和授权边界。

正式 schema 以 `contracts/plugin-info.schema.json` 为准。

## Manifest 当前核心字段

| 字段 | 含义 |
| --- | --- |
| `id` / `name` / `version` | 插件身份与版本 |
| `manifest_version` / `plugin_protocol_version` | manifest 与协议版本 |
| `type` | 当前正式值为 `managed_runtime` 或 `dev_source` |
| `runtime` | 当前正式值为 `python` 或 `nodejs` |
| `entry` | 插件入口 |
| `concurrency` | 插件事件并发度声明；省略时按 `1` 处理 |
| `license` | 插件许可信息 |
| `role` | `builtin` / `user` / `example` / `dev` |
| `default_config` | 插件默认配置 |
| `permissions` | 插件能力和作用域声明 |
| `commands` | 插件命令声明 |
| `dependencies` | 语言级依赖说明 |

## 当前正式能力集合

| 能力 | 说明 |
| --- | --- |
| `message.send` | 发送消息 |
| `message.reply` | 引用回复 |
| `logger.write` | 写入平台管理日志 |
| `storage.kv` | 访问插件隔离 KV |
| `storage.file` | 访问 `plugin_data` 文件区 |
| `http.request` | 发起受控出站 HTTP 请求 |
| `config.read` | 读取插件配置 |
| `config.write` | 写入插件配置 |
| `scheduler.create` | 注册调度任务 |
| `event.expose_webhook` | 注册 Webhook 路由 |
| `render.image` | 使用平台渲染能力 |
| `plugin.list` | 读取当前插件目录与命令列表 |

## 授权与作用域

- 插件通过 `permissions.required` 和 `permissions.optional` 声明能力需求。
- 官方内置插件的声明权限会自动生效，并在授权来源中显示为 `builtin_auto`。
- 平台按 capability grant 模型保存授权结果，并在启用、重载、恢复和崩溃恢复前重新过滤时效窗口。
- 当前授权列表会合并内置自动授权、配置自动授权和持久化授权。
- 作用域约束通过 `permissions.scopes` 声明，当前重点包括 `http_hosts`、`storage_roots` 和 `webhooks`。
- `storage_roots` 当前正式范围固定在 `plugin_data`。
- 新增高敏权限时，平台需要显式重新确认，不静默沿用历史授权。

## 命令声明

- 插件可通过 `commands` 声明命令名、别名、说明、示例和权限级别。
- 平台保留 `raylea:*` 命名空间给官方内置插件。
- 同名命令默认保持 fan-out；管理面负责提示冲突，不自动做互斥仲裁。

## 并发声明

- `concurrency` 只定义事件处理并发度。
- 插件有效并发度取 `min(manifest.concurrency, runtime.max_concurrent_tasks_per_plugin)`，最小值为 `1`。
- 同一插件内按 `event.target.type + ":" + event.target.id` 保持同会话顺序；不同会话可并发。
- 没有稳定 `event.target` 的事件使用独立 fallback lane，不参与同会话串行。

## 依赖与发布边界

- 当前 `dependencies` 只覆盖语言级依赖，不覆盖插件间依赖。
- `keywords`、`screenshots`、`platforms` 等元数据可用于检索、展示和平台约束，但不替代正式能力裁决。
- manifest 字段和语义变化必须先进入 contract，再同步 SDK、fixtures、示例和管理面。

## 相关文档

- [Plugin Lifecycle](./lifecycle.md)
- [Protocol](./protocol.md)
- [Plugin SDK Docs](./sdk/README.md)
