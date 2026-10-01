# Contracts

`contracts/` 是 RayleaBot 当前对外接口、schema、错误码和发行元数据的唯一正式来源。

## 当前状态

### Fixture-ready 正式契约

以下正式契约提供可执行 fixtures：

- `backup-manifest.schema.json`
- `config.user.schema.json`
- `deps-manifest.schema.json`
- `error-codes.yaml`
- `web-api.openapi.yaml`
- `websocket-events.yaml`
- `plugin-info.schema.json`
- `plugin-artifact.schema.json`
- `plugin-store-catalog.schema.json`
- `plugin-management-ui.yaml`
- `plugin-protocol.schema.json`
- `release-manifest.schema.json`
- `cli-commands.yaml`

这些文件都带有 `x-fixtures` 或等价引用，并接受 CI 的解析、存在性与最小覆盖校验。

## 文件职责

- `config.user.schema.json`
  - `config/user.yaml` 的正式机器可校验结构
- `backup-manifest.schema.json`
  - `backup-manifest.json` 的正式机器可校验结构
  - 恢复包版本、core / config / db schema 兼容性判断边界，以及插件库存摘要
  - `core_version` 从有效的安装产物 `build_info.json` 读取；缺失或无效时记为 `unknown`。未知版本不参与升降级排序；恢复仍按清单检查配置、数据库与插件合同版本。有最低 core 版本要求的插件须确认兼容后才能安装。
  - 本机 `plugin dev-sync` 和受控开发同步接口允许未标版本的源码构建接收 `development` artifact；此路径不声称已验证最低 core 版本，仍执行 manifest、artifact、平台与协议握手检查。普通安装和商店安装不使用此例外。
  - 配置与数据库 schema 版本从实际归档内容读取：配置为 `4`，备份契约接受数据库 `000001`、`000002`、`000003`；没有归档数据库时明确记录 `absent`。可前向迁移的旧结构在首次启动时迁移，迁移日志记录源版本与目标版本。初始化元数据、配置与业务数据一起恢复。
- `deps-manifest.schema.json`
  - `.deps/manifest.json` 的正式机器可校验结构
  - 图片渲染与插件浏览器会话共用 Chromium，以及受信本地插件共用 FFmpeg / FFprobe 的可信来源列表、SHA256、归档格式与相对入口
- `error-codes.yaml`
  - 统一错误码命名、默认消息资源键、HTTP 语义和适用范围
- `web-api.openapi.yaml`
  - 当前已固定的管理 HTTP 接口
  - 当前包含 setup / cookie 与 Bearer session、launcher control、config snapshot/update、protocol snapshot、OneBot target / identity resolution、plugin lifecycle、插件商店、可信代码确认与安装、自定义插件管理页、plugin settings / secrets、governance 管理面、logs / system、scheduler、recovery、runtime bootstrap、render templates 以及更新状态与检查入口
  - `POST /api/launcher/shutdown` 可选 `intent: stop | restart | update`，省略为 `stop`；首次优雅关闭请求固定停机意图。`POST /api/system/shutdown` 始终为 `stop`。
  - `PUT /api/config` response 固定返回 `apply_effects.applied_now`、`apply_effects.reloaded_now`、`apply_effects.restart_required_fields`
  - plugin lifecycle surface 统一使用正式 `state` 枚举与可选 `state_diagnosis`
  - 插件列表、详情及生命周期详情响应返回当前生效的 `command_prefixes` 与 `dedicated_command_prefixes`；用法示例使用前者的第一项，专属前缀标记使用后者。
  - 插件商店的 `PluginStoreReleaseSummary` 仅在 `compatible: false` 时携带 `incompatible_reason`：`core_version_unknown` 表示无法确认当前版本，`core_version_too_old` 表示已知版本低于 `min_core_version`；`asset_available` 独立表示当前平台有无产物。普通本地安装与商店安装的版本准入失败均返回 `plugin.core_version_incompatible`，其 `details` 包含相同原因和最低版本；客户端不解析消息判断原因。
  - 黑白名单条目必须携带 `scope`。`global` 只允许 `onebot11`，`source_adapter` 与 `bot_id` 均为空；`instance` 必须同时提供协议、实例 ID 和 bot ID。读取聚合所有作用域，写入与删除按完整作用域定位；实例规则与同协议的全局规则均可命中。白名单启用开关仍作用于整个服务。
  - `info.version` 是本文档的契约修订版本，独立于产品版本、包版本与运行时协议版本；破坏性契约变更递增 minor（0.x 阶段），兼容新增递增 patch。
  - `TaskStatusResponse.error_code` 等标注 `x-error-code-registry: contracts/error-codes.yaml` 的字段，取值必须是该目录已登记的 code；契约校验对 fixtures 与 examples 强制执行。
- `websocket-events.yaml`
  - 当前已固定的管理 WebSocket envelope、事件名和 payload 约束
  - `events.received` 服务状态快照仅在 `stopping` 时携带可选 `stop_intent`；`restart` / `update` 的断线为临时中断，客户端继续重连，缺省按 `stop` 处理。
  - `events.received` 的通用 `event_type + summary` 分支当前包含 `governance.changed`
  - 插件状态、诊断及命令运行态投影引用 OpenAPI 的同一 schema；命令触发器、权限级别、帮助与分组等声明字段引用 `plugin-info.schema.json` 的定义。静态 manifest 与含有效命令名的运行态投影保持各自的 required 字段。
- `plugin-info.schema.json`
  - 插件 `info.json` v4 的安装前静态校验、最低 Core 版本、事件、命令、管理页与 webhook 边界
  - 固定 `manifest_version: "4"`；运行语言、入口和目标平台由 artifact 提供
  - `events` 静态声明普通事件订阅；manifest 不声明宿主权限，全部宿主动作对可信插件进程可用
  - 当前已固定内联 `default_config`、metadata、统一 `commands`、真实 `command_groups`、帮助标题/摘要、单入口 `management_ui` 和静态 `webhooks`
  - `services` 静态声明服务名称、精确版本和方法，同名同版本不可重复
  - `command_prefixes` 声明插件的专属命令前缀、可选的配置键与是否接受通用前缀；前缀是匹配条件而非所有权，宿主按插件分别解析，专属前缀命中的候选遮蔽通用前缀命中的候选；命令只按消息中的文字段解析，@ 等非文字段不参与匹配，插件从 `message.segments` 读取被 @ 的用户
  - `concurrency` 省略时按 `1` 处理，声明值用于插件事件并发 opt-in
  - `priority`（默认 0）与 `block`（默认 false）定义消息分层与阻断。正优先级消息订阅者可先于命令声明者接收命令消息；其他事件保留既有投递。成功终态的显式 propagation 覆盖 block，同名命令授权与冷却保持既有语义。
  - command `permission` 省略时使用 `permission.default_level`
- `plugin-artifact.schema.json`
  - artifact v2 的目标平台与原生入口边界
  - `artifact.json` 不重复插件身份或文件清单；安装器扫描实际内容并检查路径与入口
- `plugin-store-catalog.schema.json`
  - 官方或自定义静态商店目录结构，固定当前版本、最低核心版本和可用平台的资产 URL 与归档摘要
  - 官方身份只能由默认官方来源和安装元数据授予，不能由插件 manifest、目录名或仓库名推断
- `plugin-management-ui.yaml`
  - 插件内置管理页在管理面同源路径 `/plugin-ui/{plugin_id}/` 下的只读静态资源与 CSP 边界
- `plugin-protocol.schema.json`
  - 插件 Runtime JSONL protocol v4
  - 当前固定 `init`、`init_progress`、`init_ack`、`event`、`result`、`error`、`ping`、`pong`、`shutdown`
  - `error` 帧由插件终态失败与平台 local action 失败共用，固定包含 `code`、`message`，可选 `details`
  - 只有 init 携带协议版本和插件身份；后续帧使用最小 envelope
  - `plugin.call` 定向调用已运行插件的静态服务，提供者接收 `plugin.request` 并使用自己的事件上下文；没有取消帧，调用方放弃后提供者处理到 `deadline_at_ms`，迟到终态被忽略；首版禁止自调用和嵌套服务调用
  - `message.send` 统一发送与回复；非终态动作通过独立 `request_id` 和当前事件 `parent_request_id` 关联
  - 每个 `event` 帧携带 `deadline_at_ms`，即该事件在宿主处的处理期限：投递时刻加 `runtime.plugin_event_timeout_seconds`，`plugin.request` 等于服务请求的期限
  - `event.detach` 把 `message.private`、`message.group`、`scheduler.trigger` 或 `management.action` 事件转入后台：宿主以给定结果完成投递并释放会话队列与并发槽，事件保留原 `request_id` 与来源，在返回的 `deadline_at_ms` 前继续接受动作，直到插件发送终态、到期（`plugin.event_timeout`）或插件停止、重载（`plugin.event_canceled`）。转入规则、拒绝条件与收尾语义由 `x-detached-events` 定义
  - `scheduler.trigger` 的 `payload.task_id` 必填，等于 `scheduler.create` 时的任务 ID，其他事件不携带该字段
  - `init.bots` 提供按适配器实例区分的身份列表；`bot.identities.changed` 通过 `payload.bots` 替换整个列表
  - 未知或已停用实例不出现在身份列表中；空列表清除旧身份。身份包含 `source_adapter`、`source_protocol`、`id`，不跨实例合并。连接可用性仍由 adapter 动作的正式结果表达
  - `logger.write`、`storage.kv` 和 `config.write` 是按插件命名空间隔离的私有动作；插件数据目录经环境变量 `RAYLEABOT_PLUGIN_DATA_DIR` 传入，由插件直接读写；插件包目录经 `RAYLEABOT_PLUGIN_PACKAGE_DIR` 传入，只读
    - `storage.kv set` 的 `ttl_seconds` 定义有效期限；省略表示永久覆盖并清除旧期限。写入在事务内检查有效全局配额，返回可选的 `expires_at_ms`。`x-action-result-schemas` 中的 KV 结果按请求 operation 关联校验。
    - `scheduler.create.log_label` 用于定时任务管理日志展示。
    - `secret.read`、`secret.write` 和 `secret.delete` 只在调用插件自己的 secret 命名空间内读取、覆盖或删除；值保存在宿主本地 secret store，读取结果仅返回调用插件。
    - `browser.launch` 启动或附着插件专属的宿主托管浏览器会话，宿主按插件隔离 profile、应用启动硬化、限制同一 profile 同时只有一个会话，并在生命周期到期或 `browser.close` 时关闭；返回的 `debugger_url` 是该会话的浏览器级 CDP WebSocket 端点。
    - `browser.close` 关闭调用插件自己启动的会话并释放其持久 profile。
    - `render.image` 支持系统模板 ID、调用插件自动发现的模板短 ID，以及平台按 HTTPS、超时和资源上限预取后交给 Chromium 的请求级临时图片资源
  - local action `action` 帧使用 `parent_request_id` 归属到对应事件；并发插件必须提供该字段
  - `session.wait` 和 `session.finish` 固定为必须携带父事件的私有动作；新建和本轮再次等待使用互斥形状，由对话 ID 和当前父事件识别归属。`payload.session` 只包含对话 ID、scope 和期限，业务状态由插件维护。`session.expired` 是显式请求后的尽力超时通知，不参与普通订阅广播。
  - 当前已固定 OneBot 单动作能力，provider 扩展 action 固定为 `provider.napcat.message_emoji.like.set`、`provider.napcat.group.sign.set` 与 `provider.luckylillia.friend_groups.get`
  - 正式 `event.event_type` 以 schema 枚举为准，包含平台内部事件与 OneBot `message.*`、`message_sent.*`、`notice.*`、`request.*`、`meta.*`
  - `event.payload.onebot` 是形状闭合的 OneBot11 归一化投影（`additionalProperties: false`），字段集以 schema 为准；投递给所有订阅插件，与只出现在 webhook 事件中的 `event.raw_payload` 无关
  - inbound / outbound 消息段种类以 schema 枚举为准，随正式接入的适配器增长；宿主不会发出集合外的种类
  - 会话种类词表 `conversation_target_type` 当前为 `group`、`private`。出站 `message.send` / `message.reply` 严格校验并对未知值 fail-closed；入站 `event.target.type` 有意保持开放，另含 `system`、`bot` 等宿主内部种类，插件忽略不认识的种类。两个方向的 unknown 策略不同
  - 治理词表 `governance_entry_type`（`user`、`group`）与 `conversation_target_type` 是两套词表，`user` 不是 `private` 的别名
  - `governance.blacklist.write` 与 `governance.whitelist.write` 的条目增删要求与管理 API 相同的 `scope`；`set_enabled` 仅修改服务级白名单开关。
  - `event.actor.id` 与 `event.target.id` 属于 `source_protocol` 的身份命名空间，并限定于接收事件的 bot 身份；不能作为跨协议、跨 bot 的全局关联键。`init.super_admins` 固定为 `admin.super_admins` 的 OneBot11 QQ 账号列表，不授予 QQ 官方 openid 管理权限。
- `release-manifest.schema.json`
  - `release_manifest.v2.json` 与 `build_info.json` 的正式字段结构
  - 发布脚本按 schema 严格生成与校验；读取端忽略未知字段，只校验实际使用的字段，插件格式版本仅供展示
- `cli-commands.yaml`
  - `config init / normalize / validate`、`reset-admin`、`backup`、`restore <backup-path>`、`doctor`、`cleanup`、`plugin dev-sync`、`version --json`、`update check --json`、`update download`、`update apply` 的正式命令模型

## 当前延后到后续版本的边界

### Plugin Protocol

- 调试流
- 批量消息
- 复杂流式回传

### Release Metadata

- 增量或差分更新

## HTTP 与 WebSocket 入口

[`web-api.openapi.yaml`](./web-api.openapi.yaml) 的 `paths` 定义完整 HTTP 操作集合；[`websocket-events.yaml`](./websocket-events.yaml) 定义 WebSocket 频道、事件与载荷。

异步任务通过 `GET /api/system/tasks/{task_id}` 查询状态与失败码。配置应用结果和插件生命周期状态分别由对应 operation/schema 定义，README 不维护第二份完整接口列表。

## 通用规则

- 规划文档解释设计意图，对外接口以 `contracts/` 为准
- 若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明
- `fixtures/` 与 `examples/` 只能从这里派生，不能反向覆盖这里
