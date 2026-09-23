# Plugin Manifest

本页说明 RayleaBot 插件 manifest v4 与静态声明。正式结构以 `contracts/plugin-info.schema.json` 与 `contracts/plugin-artifact.schema.json` 为准。

## Manifest v4

必填字段：

| 字段 | 含义 |
| --- | --- |
| `id`、`name`、`version` | 稳定插件 ID、展示名称和语义版本 |
| `manifest_version` | 固定为 `"4"` |
| `license` | 许可证标识 |
| `min_core_version` | 能运行该插件的最低 RayleaBot 版本；manifest v4 插件声明 `0.7.0` 或更高，不接受 `0.7.0` 预发布版本；构建器与 Server 均会拒绝更低的声明 |

常用可选字段：

| 字段 | 含义 |
| --- | --- |
| `metadata` | 作者、描述、图标、仓库、主页、关键词和截图 |
| `concurrency` | 插件事件并发度，默认 `1` |
| `priority` | 消息优先级，整数 -1000..1000，默认 `0`；值越大越早执行 |
| `block` | 成功处理消息后默认阻断后续插件层，默认 `false` |
| `events` | 静态事件订阅；省略或空数组表示不接收普通事件 |
| `default_config` | 内联默认配置 |
| `commands`、`command_groups`、`help` | 命令、真实命令分组，帮助标题/摘要与插件自带的帮助命令 |
| `management_ui` | 单一 UI 入口及页面 ID/标签 |
| `webhooks` | 宿主启动时注册的静态 webhook |

插件运行时只要求 artifact 提供当前平台原生可执行文件，不绑定实现语言。插件角色不写入 manifest；Server 根据安装来源判定为 `official`、`community` 或 `development`。

## 宿主能力

manifest 不声明宿主权限。插件进程是管理员确认安装的完全可信本地代码，全部宿主动作对每个插件可用，宿主不提供操作系统沙箱。

- 私有日志、配置、KV 与会话动作始终按调用插件 ID 隔离，插件不能选择其他插件的命名空间。持久化文件写入宿主传入的 `RAYLEABOT_PLUGIN_DATA_DIR`。
- 消息、治理、调度、渲染、浏览器会话、插件目录与密钥等宿主动作只做参数与领域规则校验。
- OneBot 单动作与 provider 扩展动作承载 OneBot11 的语义、可用性和参数形状，不跨聊天协议可移植；provider 扩展还取决于所连的 OneBot11 实现。

插件使用自己的 HTTP 客户端；插件自行发起的网络、子进程和临时文件操作不经过宿主 action 校验。HTTPS 目录地址和归档摘要用于核对来源记录与包完整性，不构成代码安全证明。

## 统一命令声明

每条 `commands` 声明必须有稳定 `id`、展示 `name`、说明、用法和一个触发器：

- `exact`：静态 `names`，首项是主触发词，其余项是别名。
- `pattern`：Go regexp 规则。`fallback: true` 表示兜底命令：只有所有插件的普通命令和内置帮助菜单都没有命中时才参与匹配，前缀与层级规则不变，适合“#角色名”这类宽泛写法。
- `setting`：从 `default_config` 与保存配置的 `settings_key` 推导实际触发词。

`command_groups[].commands` 只能引用存在的命令 ID。帮助菜单从命令和分组生成；`help` 只提供标题与摘要，不能声明没有对应命令的任意项目。插件自己画帮助时，`help.command` 指向该命令（须为 `exact` 触发），内置菜单对这个插件的页面（如“/插件名帮助”）改为以该命令的主触发词投递给插件，总菜单仍由宿主生成。

同名有效触发词会在管理面标记冲突。命令权限由 `command.permission`、全局默认权限、黑白名单、冷却和超级管理员共同决定。

### 专属命令前缀

宿主的通用命令前缀由用户配置，对所有插件等价。插件可以用 `command_prefixes` 再声明自己的专属前缀，让同名命令按前缀区分归属：

```json
{
  "command_prefixes": {
    "dedicated": ["*", "＊", "星铁"],
    "settings_key": "command_prefixes",
    "accept_global": false
  }
}
```

- `dedicated` 是默认的专属前缀，可以单独出现，也可以紧跟在通用前缀之后：`*体力`、`星铁体力`、`/星铁体力` 都解析为该插件的 `体力`。
- `settings_key` 指向插件配置里的一个字符串或字符串数组，管理员保存后替换 `dedicated`；缺少该键时使用 `dedicated`。`accept_global` 为 `false` 且配置值里没有任何有效前缀时同样使用 `dedicated`，插件不会因此变得无法触发。
- `accept_global` 默认 `true`。设为 `false` 后，该插件的命令不再响应通用前缀。

前缀是匹配条件，不是所有权：专属前缀可以与通用前缀相同，也可以与其他插件的专属前缀相同。宿主用每个插件自己的生效前缀分别解析消息，按最长前缀优先，再把解析出的命令名与该插件的触发器匹配。经专属前缀命中的插件优先于经通用前缀命中的插件，存在前者时不再投递后者，内置帮助菜单归入后者；同一层内继续按消息优先级、`block` 与 `propagation` 处理。每个命中的插件收到的 `command` 与 `args` 来自它自己的解析结果。

## 消息优先级与会话

声明 `priority` 或 `block` 的插件要求 `min_core_version >= 0.6.0`。普通消息按优先级降序分层，同层并发；同一目标的消息在每个插件内保持接收顺序。命令声明者之外，正优先级且订阅该消息的插件也会先收到命令消息。现有同名命令取最严格权限、名单与冷却规则继续适用。

成功消息终态可用 `propagation: stop|continue` 覆盖静态 `block`；未处理、异常、超时与队列拒绝继续后续层。终态动作发送完成后推进层次，发送失败不改变终态指定的传播结果。详情页的“消息优先级”和“默认传播”展示 manifest 声明。

多轮输入使用 `session.wait`，或使用 [Go SDK 的回调式会话](./sdk/README.md#回调式会话)。这类插件同样要求 Core 0.6.0。只有当前事件成功结束后的等待阶段接收回复，回复定向交给登记进程；业务状态由插件保存。完整三轮流程与 KV TTL 见[会话示例](../../examples/plugins/example-conversation/README.md)。

## 静态 Webhook

`webhooks` 的每项声明包含稳定 `id`、路由，以及可选的来源 CIDR 与正文上限。宿主从有效 manifest 自动注册 `POST /api/webhooks/{plugin_id}/{route}`，只按路由转发：检查来源与正文上限后投递 `webhook.received`，不做鉴权与重放检查。

`webhook.received` 事件的 `raw_payload` 携带请求的路由、方法、请求头、查询参数和原始正文（有效 UTF-8 为 `body_text`，否则为 `body_base64`）；插件按对方平台规则自行验签并处理重复投递。运行时不能新增或修改 webhook 路由。

## 模板与管理页

- 渲染模板由宿主自动发现 `templates/*/template.json`，无需 manifest 清单。
- `template.json` 必须提供非空 `name`（模板名称），可提供 `description`（用途说明）；建议使用便于用户理解的中文。缺少名称的模板无效，不再按 ID 补名称。模板预览页直接展示声明的名称，内部 ID 仍用于路由和渲染调用。
- 出图宽度取 `template.json` 的 `width`，高度按页面内所有元素的盒子测量，被祖先裁剪的部分同样计入；`body` 的 `overflow` 为 `hidden` 或 `clip` 时，高度取 `body` 自身的盒子，超出部分不显示。
- `management_ui.entry` 是所有页面共用的 `ui/*.html` 入口。
- `management_ui.pages` 只包含稳定 `id` 和展示 `label`；当前页面 ID 通过 iframe 地址的 `page` 参数传递。
- 插件页面与管理面同源加载，响应 CSP 只允许加载插件 UI 路径下的脚本；密钥只暴露 configured-state，不回显明文。

## Artifact v2

`artifact.json` 只包含：

- `artifact_version: "2"`
- `target_platform`
- `entry`

插件身份、版本和管理面来自 `info.json`，不在 artifact 重复。入口必须是目标平台可执行格式；安装器直接扫描管理 UI、模板、资源、许可证和 notices 等实际文件。

统一工具 `raylea-plugin inspect/pack/build-go` 分别负责检查、通用原生打包和 Go 构建打包。

## 相关文档

- [Plugin Protocol](./protocol.md)
- [Plugin Lifecycle](./lifecycle.md)
- [Management UI](./management-ui.md)
- [Plugin SDK](./sdk/README.md)

## 静态插件服务

服务调用是 manifest 与 JSONL v4 的兼容扩展，不另设最低 Core 版本；不支持该能力的 Core 会因未知的 `services` 字段拒绝清单。提供者通过 `services` 声明公开的方法：

```json
{
  "services": [
    {"name": "resource", "version": 1, "methods": ["query"]}
  ]
}
```

每个 `(name, version)` 组合唯一，方法名在同一服务版本内唯一。最多声明 32 个服务，每项最多 64 个方法；省略或空数组表示不提供服务。服务和方法使用小写字母开头的字母、数字、点、下划线或连字符名称，长度最多 64。

Go 构建器同步校验服务标识、重复声明、方法数量与最低 Core 版本，防止生成声明不一致的服务包。

服务只接受定向的 `plugin.request`，不依赖普通 `events` 订阅。宿主只允许调用当前运行实例声明的服务、精确版本和方法；不自动启动被停用的提供者，也不把服务声明作为全局权限授予。提供者按实际 caller 和自己的业务配置判断调用许可。

调用与取消见 [协议](./protocol.md#插件服务调用)，SDK 注册方式见 [服务示例](../../examples/plugins/example-service-provider/README.md)。
