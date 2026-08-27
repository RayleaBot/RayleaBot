# Capabilities and Manifest

本页说明 RayleaBot 插件 manifest 的正式结构、能力声明和能力参数边界。

正式 schema 以 `contracts/plugin-info.schema.json` 和 `contracts/plugin-artifact.schema.json` 为准。

## Manifest 字段

必填字段：

| 字段 | 含义 |
| --- | --- |
| `id` / `name` / `version` | 插件身份、展示名称与版本 |
| `manifest_version` / `plugin_protocol_version` | 固定为 `"2"` / `"1"` |
| `runtime` | 固定为 `"go"` |
| `entry` | `bin/` 下无扩展名的相对逻辑路径；Windows 安装时解析为 `.exe` |
| `platforms` | `windows-x64`、`linux-x64`、`macos-arm64` 的非空子集 |
| `license` | 插件开源许可证标识 |

可选字段：

| 字段 | 含义 |
| --- | --- |
| `description` / `author` | 描述与作者信息 |
| `min_core_version` / `data_schema_version` / `concurrency` | 最低核心版本、插件数据 schema 版本与事件并发度；`concurrency` 省略时按 `1` 处理 |
| `icon` / `repo` / `homepage` / `keywords` / `screenshots` | 展示、来源和截图元数据 |
| `management_ui` / `render_templates` / `help` | 插件详情页、渲染模板与帮助菜单声明 |
| `capabilities` / `capability_parameters` | 平台能力及 HTTP 主机、文件根、Webhook、三方账号平台边界 |
| `default_config` / `default_config_file` | 首次启用时使用的内联默认配置和包内 JSON 默认配置；两者并用时内联字段覆盖文件中的同名字段 |
| `commands` / `command_patterns` / `dynamic_commands` | 精确命令、正则命令族与设置驱动命令声明 |

插件角色不属于 manifest。Server 根据已验证商店目录、本地安装来源或开发同步来源投影 `official`、`community`、`development`。

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
| 请求复检三方账号 CK | `thirdparty.account.validate` | 需要 | `third_party_account_platforms` |
| 调用 OneBot 单动作 | action kind，例如 `message.history.get`、`group.member.list` | 需要 | 无 |
| 调用 provider 扩展动作 | provider action kind | 需要 | 无 |
| 发起 HTTP 请求 | `http.request` | 需要 | `http_hosts` |
| 读写插件文件 | `storage.file` | 需要 | `storage_roots` |
| 暴露 Webhook 入口 | `event.expose_webhook` | 需要 | `webhooks` |
| 只声明命令、帮助、截图、管理页等元数据 | 无 | 无 | 无 |

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
      "plugin_data"
    ]
  }
}
```

## 能力参数

`capability_parameters` 只表达运行边界参数，当前正式范围包括：

- `http_hosts`：`http.request` 与 `render.image.resources` 可访问的主机名列表。声明的主机名匹配该主机及其子域，例如 `douyinpic.com` 覆盖 `p3-pc-sign.douyinpic.com`，但不覆盖 `evil-douyinpic.com`。平台仍执行 HTTP 超时、DNS 预检、SSRF 防护、私网主机限制和重定向目标复检；`render.image.resources` 使用独立的图片资源上限与请求级期限。
- `storage_roots`：`storage.file` 可访问的插件文件根目录列表；当前唯一合法值是 `plugin_data`。平台仍执行路径穿越、符号链接和插件工作目录配额校验。
- `third_party_account_platforms`：`thirdparty.account.read` 可读取、`thirdparty.account.validate` 可请求复检的平台列表；冻结值为 `bilibili`、`weibo`、`douyin`、`netease_music`。读取只返回已保存、已启用且非 invalid 的账号，CK 以 secret 值标记返回；复检动作只能提交账号 ID、受限异常观察和可选 HTTP 状态，最终凭据状态由 Server 校验器决定。
- `webhooks`：`event.expose_webhook` 可暴露的路由列表。每项必填 `route`、`auth_strategy`、`header`、`secret_ref`，可选 `source_ips`。重放保护不在 manifest 中声明，而是每次注册 `event.expose_webhook` action 时通过必填 `replay_protection` 提交。

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
- `thirdparty.account.validate`
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
- `dynamic_commands` 从插件设置字段投影命令名，适合由用户配置触发词的插件。
- `command_patterns` 使用 Go regexp 匹配去掉全局前缀后的命令名，适合 `<角色名>攻略` 这类命令族；`name` 只作为展示名，不作为精确触发词。
- 命令用法中的 `<参数>` 与命令名后的裸参数表示必填项，`[参数]` 表示可选项；`command_patterns` 中未加括号的文本表示固定指令文本。
- 平台保留 `raylea.` ID 前缀给已验证目录中的官方插件；插件移出主仓库不改变该命名空间规则。
- 同名命令默认保持 fan-out；管理面负责提示冲突。
- 聊天命令权限治理使用 `command.permission`、`permission.default_level`、黑白名单、冷却和超级管理员配置。

## 并发声明

- `concurrency` 只定义事件处理并发度。
- 插件有效并发度取 `min(manifest.concurrency, runtime.max_concurrent_tasks_per_plugin)`，最小值为 `1`。
- 同一插件内按 `event.target.type + ":" + event.target.id` 保持同会话顺序；不同会话可并发。
- 没有稳定 `event.target` 的事件使用独立 fallback lane。
- 插件详情页显示 `concurrency`、`default_config`、`declared_capabilities` 和 `capability_parameters`。

## Artifact 与发布边界

- 使用 `sdk/go/pluginbuild` 正式构建器生成的平台包包含 manifest v2、artifact v1、一个 Go 后端、可选 UI/模板/数据、许可证、第三方 notices 和 SPDX SBOM。通用安装 contract 只要求满足 `plugin-artifact.schema.json`；合法的外部 artifact 不因缺少构建器附加的供应链文件而被拒绝。
- `artifact.json` 固定插件 ID、版本、目标平台和 `info.json` SHA-256，并列出除自身外所有文件的路径、角色、大小和 SHA-256。
- 包必须且只能有一个 `backend` 文件；`management_ui.pages[].entry` 必须属于 `ui` 文件集合。ZIP 只有一个插件根目录。
- 服务端只安装编译产物，不读取源码依赖声明、不运行安装脚本、不准备语言运行时，也不解析插件间依赖。
- manifest 或 artifact 字段变化先进入 contract，再同步 SDK、fixtures、示例、管理面与发布校验。

## 插件内置管理页

- `management_ui.pages` 声明插件详情页内的管理页签，至少包含一个页面。
- `pages[].id` 是稳定页签标识，`pages[].label` 是页签标题，`pages[].entry` 是插件包内的 HTML 文件路径。
- 插件详情页在概览之外提供同一插件的内置管理页工作区。
- 插件内置页面从插件专属 origin 读取 `ui/` 静态资源，不共享管理 cookie，也没有 API 路由或管理端 CORS。
- 插件内置页面只通过 bridge v2 的 nonce-bound `MessageChannel` 读取和保存插件自己的设置，不直接持有管理会话。
- 密钥只暴露 configured-state；写入只支持覆盖和显式删除，宿主从不回显已有明文。
- 当前设置由 `default_config` 叠加已保存配置得到；保存成功后宿主会同步刷新插件详情中的配置预览。
- 未验证来源插件首次打开内置管理页需要人工确认；确认记录会随插件版本或来源变化失效。

## 相关文档

- [Plugin Lifecycle](./lifecycle.md)
- [Management UI](./management-ui.md)
- [Protocol](./protocol.md)
- [Plugin SDK Docs](./sdk/README.md)
