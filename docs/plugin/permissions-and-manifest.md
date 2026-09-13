# Plugin Manifest and Permissions

本页说明 RayleaBot 插件 manifest v3、权限边界和静态声明。正式结构以 `contracts/plugin-info.schema.json` 与 `contracts/plugin-artifact.schema.json` 为准。

## Manifest v3

必填字段：

| 字段 | 含义 |
| --- | --- |
| `id`、`name`、`version` | 稳定插件 ID、展示名称和语义版本 |
| `manifest_version` | 固定为 `"3"` |
| `license` | 许可证标识 |
| `min_core_version` | 能运行该插件的最低 RayleaBot 版本 |

常用可选字段：

| 字段 | 含义 |
| --- | --- |
| `metadata` | 作者、描述、图标、仓库、主页、关键词和截图 |
| `concurrency` | 插件事件并发度，默认 `1` |
| `priority` | 消息优先级，整数 -1000..1000，默认 `0`；值越大越早执行 |
| `block` | 成功处理消息后默认阻断后续插件层，默认 `false` |
| `events` | 静态事件订阅；省略或空数组表示不接收普通事件 |
| `permissions` | 需要宿主授权的高权限或跨系统能力 |
| `default_config` | 内联默认配置 |
| `commands`、`command_groups`、`help` | 命令、真实命令分组和帮助标题/摘要 |
| `management_ui` | 单一 UI 入口及页面 ID/标签 |
| `webhooks` | 宿主启动时注册的静态 webhook |

插件运行时只要求 artifact 提供当前平台原生可执行文件，不绑定实现语言。插件角色不写入 manifest；Server 根据安装来源判定为 `official`、`community` 或 `development`。

## 权限模型

`permissions` 只声明显式宿主权限。完整名称集合以 schema 的 `permission_name` 为准。

插件私有能力默认可用，不写入 `permissions`：

- `logger.write`
- `config.write`，以及 init/config.changed 提供的配置快照
- `storage.kv`
- `storage.file`
- `session.wait`、`session.finish`

这些能力始终按调用插件 ID 隔离。插件不能选择其他插件命名空间，也不能通过文件根参数扩大访问范围。

需要显式权限的能力包括：

- 消息、治理、调度、渲染、浏览器会话和插件目录等宿主动作。这类基础权限与聊天协议无关。
- OneBot 单动作与 provider 扩展动作。这些名字承载 OneBot11 的语义、可用性和参数形状，不跨聊天协议可移植；provider 扩展还取决于所连的 OneBot11 实现。
- `http.request`。
- `secret.read`、`secret.write` 和 `secret.delete`。
- `event.raw_payload`。

未声明权限时，宿主返回 `plugin.permission_denied`。

## HTTP 安全边界

`http.request` 不包含插件级主机白名单。宿主统一执行：

- 只接受 HTTPS 正式请求。
- DNS 解析与每次重定向目标复查。
- SSRF、环回、链路本地和私网地址拦截。
- 超时、重试、响应体和渲染资源总量限制。

插件进程是管理员信任的本地代码，不是操作系统沙箱；插件自行发起的网络、子进程和临时文件操作不经过宿主 action 校验。HTTPS 目录地址和归档摘要用于核对来源记录与包完整性，不构成代码安全证明。

## 统一命令声明

每条 `commands` 声明必须有稳定 `id`、展示 `name`、说明、用法和一个触发器：

- `exact`：静态 `names`，首项是主触发词，其余项是别名。
- `pattern`：Go regexp 规则。
- `setting`：从 `default_config` 与保存配置的 `settings_key` 推导实际触发词。

`command_groups[].commands` 只能引用存在的命令 ID。帮助菜单从命令和分组生成；`help` 只提供标题与摘要，不能声明没有对应命令的任意项目。

同名有效触发词会在管理面标记冲突。命令权限由 `command.permission`、全局默认权限、黑白名单、冷却和超级管理员共同决定。

## 消息优先级与会话

声明 `priority` 或 `block` 的插件要求 `min_core_version >= 0.6.0`。普通消息按优先级降序分层，同层并发；同一目标的消息在每个插件内保持接收顺序。命令声明者之外，正优先级且订阅该消息的插件也会先收到命令消息。现有同名命令取最严格权限、名单与冷却规则继续适用。

成功消息终态可用 `propagation: stop|continue` 覆盖静态 `block`；未处理、异常、超时与队列拒绝继续后续层。终态动作发送完成后推进层次，发送失败不改变终态指定的传播结果。详情页的“消息优先级”和“默认传播”展示 manifest 声明。

多轮输入使用 `session.wait`，或使用 [Go SDK 的回调式会话](./sdk/README.md#回调式会话)。这类插件同样要求 Core 0.6.0。只有当前事件成功结束后的等待阶段接收回复，回复定向交给登记进程；业务状态由插件保存。完整三轮流程与 KV TTL 见[会话示例](../../examples/plugins/example-conversation/README.md)。

## 静态 Webhook

`webhooks` 的每项声明包含稳定 `id`、路由、鉴权策略、请求头、secret 引用、正文上限和重放保护。宿主从有效 manifest 自动注册 `POST /api/webhooks/{plugin_id}/{route}`，完成来源、鉴权和重放检查后投递 `webhook.received`。

插件通过 `event.raw_payload` 决定是否接收已校验请求的原始正文。运行时不能新增或修改 webhook 路由。

## 模板与管理页

- 渲染模板由宿主自动发现 `templates/*/template.json`，无需 manifest 清单。
- `template.json` 必须提供非空 `name`（模板名称），可提供 `description`（用途说明）；建议使用便于用户理解的中文。缺少名称的模板无效，不再按 ID 补名称。模板预览页直接展示声明的名称，内部 ID 仍用于路由和渲染调用。
- `management_ui.entry` 是所有页面共用的 `ui/*.html` 入口。
- `management_ui.pages` 只包含稳定 `id` 和展示 `label`；当前页面 ID 通过 bridge 上下文传递。
- 插件页面使用隔离 origin、CSP、nonce 和 MessagePort；密钥只暴露 configured-state，不回显明文。

## Artifact v2

`artifact.json` 只包含：

- `artifact_version: "2"`
- `target_platform`
- `entry`

插件身份、版本和管理面来自 `info.json`，不在 artifact 重复。入口必须是目标平台可执行格式；安装器直接扫描管理 UI、模板、资源、许可证、notices 和 SBOM 等实际文件。

统一工具 `raylea-plugin inspect/pack/build-go` 分别负责检查、通用原生打包和 Go 构建打包。

## 相关文档

- [Plugin Protocol](./protocol.md)
- [Plugin Lifecycle](./lifecycle.md)
- [Management UI](./management-ui.md)
- [Plugin SDK](./sdk/README.md)
