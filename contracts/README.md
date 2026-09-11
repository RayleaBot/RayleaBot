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
- `plugin-development-workspace.schema.json`
- `plugin-management-ui.yaml`
- `plugin-management-ui-bridge.schema.json`
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
  - `core_version` 从有效的安装产物 `build_info.json` 读取；缺失或无效时记为 `unknown`。未知版本不参与升降级排序，恢复操作标为 `restore`，仍检查 schema 与协议版本；有最低 core 版本要求的插件须确认兼容后才能自动启用或通过商店安装。
  - 本机 `plugin dev-sync` 和受控开发同步接口允许未标版本的源码构建接收 `development` artifact；此路径不声称已验证最低 core 版本，仍执行 manifest、artifact、平台、权限与协议握手检查。普通安装和商店安装不使用此例外。
  - 配置与数据库 schema 版本从实际归档内容读取：当前配置为 `4`，数据库为 `000001`；没有归档数据库时明确记录 `absent`。恢复只处理当前格式，初始化元数据、配置与业务数据在本版备份恢复中保持一致。
- `deps-manifest.schema.json`
  - `.deps/manifest.json` 的正式机器可校验结构
  - 图片渲染与抖音扫码登录（浏览器兜底）共用 Chromium，以及受信本地插件共用 FFmpeg / FFprobe 的可信来源列表、SHA256、归档格式与相对入口
- `error-codes.yaml`
  - 统一错误码命名、默认消息资源键、HTTP 语义和适用范围
- `web-api.openapi.yaml`
  - 当前已固定的管理 HTTP 接口
  - 当前包含 setup / cookie 与 Bearer session、launcher control、config snapshot/update、protocol snapshot / compatibility、OneBot target / identity resolution、plugin lifecycle、插件商店、安装检查与可信代码确认、自定义插件管理页、plugin settings / secrets、third-party accounts、governance 管理面、logs / system / metrics、scheduler、recovery、runtime bootstrap、render templates 以及受信更新状态与检查入口
  - `PUT /api/config` response 固定返回 `apply_effects.applied_now`、`apply_effects.reloaded_now`、`apply_effects.restart_required_fields`
  - plugin lifecycle surface 统一使用正式 `state` 枚举与可选 `state_diagnosis`
  - 黑白名单条目必须携带 `scope`。`global` 只允许 `onebot11`，`source_adapter` 与 `bot_id` 均为空；`instance` 必须同时提供协议、实例 ID 和 bot ID。读取聚合所有作用域，写入与删除按完整作用域定位；实例规则与同协议的全局规则均可命中。白名单启用开关仍作用于整个服务。
  - `info.version` 是本文档的契约修订版本，独立于产品版本、包版本与运行时协议版本；破坏性契约变更递增 minor（0.x 阶段），兼容新增递增 patch。
  - `TaskStatusResponse.error_code` 等标注 `x-error-code-registry: contracts/error-codes.yaml` 的字段，取值必须是该目录已登记的 code；契约校验对 fixtures 与 examples 强制执行。
- `websocket-events.yaml`
  - 当前已固定的管理 WebSocket envelope、事件名和 payload 约束
  - `events.received` 的通用 `event_type + summary` 分支当前包含 `governance.changed` 与 `third_party.account.changed`
  - 插件状态、诊断及命令运行态投影引用 OpenAPI 的同一 schema；命令触发器、权限级别、帮助与分组等声明字段引用 `plugin-info.schema.json` 的定义。静态 manifest 与含有效命令名的运行态投影保持各自的 required 字段。
- `plugin-info.schema.json`
  - 插件 `info.json` v3 的安装前静态校验、最低 Core 版本、事件、权限、命令、管理页与 webhook 边界
  - 固定 `manifest_version: "3"`；运行语言、入口和目标平台由 artifact 提供
  - `events` 静态声明普通事件订阅；`permissions` 只声明高权限或跨系统宿主能力
  - 当前已固定内联 `default_config`、metadata、统一 `commands`、真实 `command_groups`、帮助标题/摘要、单入口 `management_ui` 和静态 `webhooks`
  - `concurrency` 省略时按 `1` 处理，声明值用于插件事件并发 opt-in
  - command `permission` 省略时使用 `permission.default_level`
- `plugin-artifact.schema.json`
  - artifact v2 的目标平台与原生入口边界
  - `artifact.json` 不重复插件身份或文件清单；安装器扫描实际内容并检查路径、入口与二进制格式
- `plugin-store-catalog.schema.json`
  - 官方或自定义静态商店目录结构，固定当前版本、最低核心版本和可用平台的资产 URL 与归档摘要
  - 官方身份只能由默认官方来源和安装元数据授予，不能由插件 manifest、目录名或仓库名推断
- `plugin-development-workspace.schema.json`
  - workspace v2 的本地插件仓库路径和启用状态；插件 ID 从 `info.json` 推导
- `plugin-management-ui.yaml`
  - 插件内置管理页的独立来源、只读静态资源、CSP、cookie、CORS 和管理 API 隔离边界
  - 本机模式默认派生 `p-<id-hash>.plugins.localhost`；LAN 与反向代理模式要求显式配置 `web.plugin_ui_origin_template`
- `plugin-management-ui-bridge.schema.json`
  - Web 宿主页与插件内置 iframe 的 bridge v3 消息结构
  - `page.ready` / `host.connect` 只用于校验窗口、来源和一次性 nonce 并转交一个 `MessagePort`，后续消息仅允许通过绑定端口
  - secret 只暴露是否已配置，写操作仅支持覆盖与显式删除；`ui.resize` 的宿主有效范围为 320–1600px
- `plugin-protocol.schema.json`
  - 插件 Runtime JSONL protocol v3
  - 当前固定 `init`、`init_progress`、`init_ack`、`event`、`result`、`error`、`ping`、`pong`、`shutdown`
  - `error` 帧由插件终态失败与平台 local action 失败共用，固定包含 `code`、`message`，可选 `details`
  - 只有 init 携带协议版本和插件身份；后续帧使用最小 envelope
  - `message.send` 统一发送与回复；非终态动作通过独立 `request_id` 和当前事件 `parent_request_id` 关联
  - `init.bots` 提供按适配器实例区分的身份列表；`bot.identities.changed` 通过 `payload.bots` 替换整个列表
  - 未知或已停用实例不出现在身份列表中；空列表清除旧身份。身份包含 `source_adapter`、`source_protocol`、`id`，不跨实例合并。连接可用性仍由 adapter 动作的正式结果表达
  - `logger.write`、`storage.kv`、`storage.file` 和 `config.write` 是隐式插件私有动作；HTTP、消息、secret、三方账号、治理、调度、渲染、OneBot 与 provider 动作使用显式权限。
    - `scheduler.create.log_label` 用于定时任务管理日志展示。
    - `secret.read` 只读取调用插件自己的 secret 命名空间。
    - `thirdparty.account.read` 只读取插件 manifest 声明平台的已启用有效三方账号，并把 CK 按 secret 值处理。
    - `thirdparty.account.validate` 只提交受限异常观察并请求 Server 权威复检，不接受 CK 状态、凭据、响应正文或自由文本错误。
    - `thirdparty.resolve` 请求宿主用已登录浏览器环境解析三方平台用户，当前仅支持 douyin；返回的 `uid` 是稳定绑定标识，`unique_id` 是平台可修改标识，仅用于展示。
    - `render.image` 支持系统模板 ID、调用插件自动发现的模板短 ID，以及平台经统一 HTTPS、DNS/重定向复查、SSRF/私网和资源限制预取后交给 Chromium 的请求级临时图片资源
  - local action `action` 帧使用 `parent_request_id` 归属到对应事件；并发插件必须提供该字段
  - 当前已固定 OneBot 单动作能力，provider 扩展 action 固定为 `provider.napcat.message_emoji.like.set`、`provider.napcat.group.sign.set` 与 `provider.luckylillia.friend_groups.get`
  - 正式 `event.event_type` 固定包含 `scheduler.trigger`、`plugin.started`、`management.action`、`config.changed`、`webhook.received`、`bot.identities.changed` 以及 OneBot `message.*`、`message_sent.*`、`notice.*`、`request.*`、`meta.*`
  - `event.payload.onebot` 是形状闭合的 OneBot11 归一化投影（`additionalProperties: false`），正式暴露 `post_type`、`meta_event_type`、`message_type`、`request_type`、`notice_type`、`sub_type`、`self_id`、`user_id`、`group_id`、`target_id`、`time`、`interval`、`message_id`、`real_id`、`message_seq`、`raw_message`、`font`、`message_format`、`sender`、`comment`、`flag`、`status`；不需要 permission，与 permission-gated 的 `event.raw_payload` 无关
  - 正式 inbound / outbound segment 种类当前为 `text`、`image`、`at`、`at_all`、`face`、`reply`、`record`、`video`、`file`、`flash_file`、`json`、`xml`、`markdown`、`music`、`contact`、`forward`、`node`、`poke`、`dice`、`rps`、`mface`、`keyboard`、`shake`；该集合随正式接入的适配器增长，宿主不会发出集合外的种类
  - 会话种类词表 `conversation_target_type` 当前为 `group`、`private`。出站 `message.send` / `message.reply` 严格校验并对未知值 fail-closed；入站 `event.target.type` 有意保持开放，另含 `system`、`bot` 等宿主内部种类，插件忽略不认识的种类。两个方向的 unknown 策略不同
  - 治理词表 `governance_entry_type`（`user`、`group`）与 `conversation_target_type` 是两套词表，`user` 不是 `private` 的别名
  - `governance.blacklist.write` 与 `governance.whitelist.write` 的条目增删要求与管理 API 相同的 `scope`；`set_enabled` 仅修改服务级白名单开关。
  - `event.actor.id` 与 `event.target.id` 属于 `source_protocol` 的身份命名空间，并限定于接收事件的 bot 身份；不能作为跨协议、跨 bot 的全局关联键。`init.super_admins` 固定为 `admin.super_admins` 的 OneBot11 QQ 账号列表，不授予 QQ 官方 openid 管理权限。
- `release-manifest.schema.json`
  - `release_manifest.v2.json`、`release_manifest.v2.sig.json` 与 `build_info.json` 的正式字段结构
  - Ed25519 双签轮换、artifact 摘要与资源上限、更新协议、平台模式和 Windows signer 摘要
  - `SHA256SUMS.txt` 继续由 release tool 的生成与校验规则决定，不作为独立 schema
- `cli-commands.yaml`
  - `config init / normalize / validate`、`reset-admin`、`backup`、`restore <backup-path>`、`doctor`、`cleanup`、`plugin dev-sync`、`version --json`、`update check --json` 与 `update verify` 的正式命令模型

## 当前延后到后续版本的边界

### Plugin Protocol

- 调试流
- 批量消息
- 复杂流式回传

### Release Metadata

- 增量或差分更新

## HTTP 与 WebSocket 入口

[`web-api.openapi.yaml`](./web-api.openapi.yaml) 的 `paths` 定义完整 HTTP 操作集合；[`websocket-events.yaml`](./websocket-events.yaml) 定义 WebSocket 频道、事件与载荷。

异步任务通过 `GET /api/system/tasks/{task_id}` 查询状态与失败码。配置应用结果、插件生命周期状态和恢复确认参数分别由对应 operation/schema 定义，README 不维护第二份完整接口列表。

## 通用规则

- 规划文档解释设计意图，对外接口以 `contracts/` 为准
- 若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明
- 新增或改变对外接口、协议、schema、状态、错误码、事件、CLI 或发布元数据的正式语义时，先更新对应契约，再同步受影响的实现、测试、样例、生成物和文档；修复实现以符合现有契约时，直接修实现并按风险验证
- `fixtures/` 与 `examples/` 只能从这里派生，不能反向覆盖这里
