# Contracts

`contracts/` 是 RayleaBot 当前对外接口、schema、错误码和发行元数据的唯一正式来源。

## 当前状态

### Fixture-ready 正式契约

当前已有 15 份 fixture-ready formal contracts：

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
- `websocket-events.yaml`
  - 当前已固定的管理 WebSocket envelope、事件名和 payload 约束
  - `events.received` 的通用 `event_type + summary` 分支当前包含 `governance.changed` 与 `third_party.account.changed`
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
  - 插件 Runtime JSONL protocol v2
  - 当前固定 `init`、`init_progress`、`init_ack`、`event`、`result`、`error`、`ping`、`pong`、`shutdown`
  - `error` 帧由插件终态失败与平台 local action 失败共用，固定包含 `code`、`message`，可选 `details`
  - 只有 init 携带协议版本和插件身份；后续帧使用最小 envelope
  - `message.send` 统一发送与回复；非终态动作通过独立 `request_id` 和当前事件 `parent_request_id` 关联
  - `init.bot` 在协议身份可用时出现，`bot.identity.changed` 用于向运行中插件同步当前 bot 身份
  - 协议身份不可用时 `init.bot` 缺省或 `bot.identity.changed` 携带空身份；依赖 `self_id` 的出站 OneBot 动作返回正式 `error` 帧，不依赖身份的 local action 保持可用
  - `logger.write`、`storage.kv`、`storage.file` 和 `config.write` 是隐式插件私有动作；HTTP、消息、secret、三方账号、治理、调度、渲染、OneBot 与 provider 动作使用显式权限。
    - `scheduler.create.log_label` 用于定时任务管理日志展示。
    - `secret.read` 只读取调用插件自己的 secret 命名空间。
    - `thirdparty.account.read` 只读取插件 manifest 声明平台的已启用有效三方账号，并把 CK 按 secret 值处理。
    - `thirdparty.account.validate` 只提交受限异常观察并请求 Server 权威复检，不接受 CK 状态、凭据、响应正文或自由文本错误。
    - `thirdparty.resolve` 请求宿主用已登录浏览器环境解析三方平台用户，当前仅支持 douyin；返回的 `uid` 是稳定绑定标识，`unique_id` 是平台可修改标识，仅用于展示。
    - `render.image` 支持系统模板 ID、调用插件自动发现的模板短 ID，以及平台经统一 HTTPS、DNS/重定向复查、SSRF/私网和资源限制预取后交给 Chromium 的请求级临时图片资源
  - local action `action` 帧使用 `parent_request_id` 归属到对应事件；并发插件必须提供该字段
  - 当前已固定 OneBot 单动作能力，provider 扩展 action 固定为 `provider.napcat.message_emoji.like.set`、`provider.napcat.group.sign.set` 与 `provider.luckylillia.friend_groups.get`
  - 正式 `event.event_type` 固定包含 `scheduler.trigger`、`plugin.started`、`management.action`、`config.changed`、`webhook.received`、`bot.identity.changed` 以及 OneBot `message.*`、`message_sent.*`、`notice.*`、`request.*`、`meta.*`
  - `event.payload.onebot` 是形状闭合的 OneBot11 归一化投影（`additionalProperties: false`），正式暴露 `post_type`、`meta_event_type`、`message_type`、`request_type`、`notice_type`、`sub_type`、`self_id`、`user_id`、`group_id`、`target_id`、`time`、`interval`、`message_id`、`real_id`、`message_seq`、`raw_message`、`font`、`message_format`、`sender`、`comment`、`flag`、`status`；不需要 permission，与 permission-gated 的 `event.raw_payload` 无关
  - 正式 inbound / outbound segment 种类当前为 `text`、`image`、`at`、`at_all`、`face`、`reply`、`record`、`video`、`file`、`flash_file`、`json`、`xml`、`markdown`、`music`、`contact`、`forward`、`node`、`poke`、`dice`、`rps`、`mface`、`keyboard`、`shake`；该集合随正式接入的适配器增长，宿主不会发出集合外的种类
  - 会话种类词表 `conversation_target_type` 当前为 `group`、`private`。出站 `message.send` / `message.reply` 严格校验并对未知值 fail-closed；入站 `event.target.type` 有意保持开放，另含 `system`、`bot` 等宿主内部种类，插件忽略不认识的种类。两个方向的 unknown 策略不同
  - 治理词表 `governance_entry_type`（`user`、`group`）与 `conversation_target_type` 是两套词表，`user` 不是 `private` 的别名
  - `event.actor.id`、`event.target.id` 与 `init.super_admins` 的标识符属于 `source_protocol` 的身份命名空间，并限定于接收事件的 bot 身份；不跨协议、不跨 bot 身份可移植，不能作为全局关联键
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

## OpenAPI 已固定范围

`web-api.openapi.yaml#paths` 是完整 method/path 集合的唯一正式来源。以下索引按能力族镜像当前 71 个 path template；仓库没有保留在 OpenAPI 之外的管理 HTTP 路由。

### Method / path 索引

#### 健康与初始化

- `GET /healthz`
- `GET /readyz`
- `POST /api/setup/admin`
- `GET /api/setup/status`
- `POST /api/session/login`
- `DELETE /api/session`

#### Launcher 与配置

- `GET /api/launcher/status`
- `POST /api/launcher/shutdown`
- `GET /api/development/status`
- `POST /api/development/plugins/sync`
- `GET /api/development/plugins/sync/{task_id}`
- `GET /api/config`、`PUT /api/config`

#### 治理

- `GET /api/governance/blacklist`
- `POST /api/governance/blacklist/entries`
- `DELETE /api/governance/blacklist/entries/{entry_type}/{target_id}`
- `GET /api/governance/whitelist`
- `PUT /api/governance/whitelist/state`
- `POST /api/governance/whitelist/entries`
- `DELETE /api/governance/whitelist/entries/{entry_type}/{target_id}`
- `GET /api/governance/command-policy`

#### 系统控制与备份

- `GET /api/system/status`
- `POST /api/system/shutdown`
- `POST /api/system/backup`

#### OneBot11 协议

- `GET /api/protocols/onebot11`
- `GET /api/protocols/onebot11/targets`
- `POST /api/protocols/onebot11/identities/resolve`
- `GET /api/protocols/onebot11/compatibility`
- `GET /api/protocols/onebot11/reverse-ws`
- `POST /api/protocols/onebot11/webhook`

#### 恢复与运行环境准备

- `POST /api/system/recovery/recheck`
- `POST /api/system/recovery/confirm`
- `POST /api/system/runtime/bootstrap`

#### 渲染

- `GET /api/system/render/templates`
- `GET /api/system/render/templates/{template_id}`
- `POST /api/system/render/templates/{template_id}/preview-html`
- `GET /api/system/render/templates/{template_id}/asset`

#### 调度、诊断、指标与日志

- `GET /api/system/scheduler/jobs`
- `POST /api/system/scheduler/jobs/{job_id}/trigger`
- `GET /api/system/diagnostics`
- `GET /api/system/diagnostics/export`
- `GET /api/system/metrics`
- `GET /api/logs`
- `GET /api/logs/{log_id}`

#### 插件生命周期、设置与安装

- `GET /api/plugins`
- `GET /api/plugins/{plugin_id}/icon`
- `POST /api/plugins/{plugin_id}/enable`
- `POST /api/plugins/{plugin_id}/disable`
- `POST /api/plugins/{plugin_id}/reload`
- `POST /api/plugins/{plugin_id}/recover`
- `GET /api/plugins/{plugin_id}/settings`、`PUT /api/plugins/{plugin_id}/settings`
- `GET /api/plugins/{plugin_id}/secrets`、`PUT /api/plugins/{plugin_id}/secrets`、`DELETE /api/plugins/{plugin_id}/secrets`
- `POST /api/plugins/{plugin_id}/management/actions`
- `GET /api/plugins/{plugin_id}`、`DELETE /api/plugins/{plugin_id}`
- `POST /api/plugins/install/inspect`
- `POST /api/plugins/install`

#### 插件商店

- `GET /api/plugin-store/sources`、`POST /api/plugin-store/sources`
- `PUT /api/plugin-store/sources/{source_id}`、`DELETE /api/plugin-store/sources/{source_id}`
- `POST /api/plugin-store/sources/{source_id}/refresh`
- `GET /api/plugin-store/plugins`
- `GET /api/plugin-store/plugins/{plugin_id}`
- `POST /api/plugin-store/plugins/{plugin_id}/inspect`
- `POST /api/plugin-store/plugins/{plugin_id}/install`

#### 三方账号

- `GET /api/third-party/accounts`
- `PUT /api/third-party/accounts/{platform}/{account_id}`、`DELETE /api/third-party/accounts/{platform}/{account_id}`
- `POST /api/third-party/accounts/{platform}/login/qrcode`
- `GET /api/third-party/accounts/{platform}/login/qrcode/{login_id}`、`DELETE /api/third-party/accounts/{platform}/login/qrcode/{login_id}`
- `POST /api/third-party/accounts/{platform}/{account_id}/validate`
- `GET /api/third-party/accounts/{platform}/{account_id}/avatar`

#### 更新与插件 Webhook

- `GET /api/update/status`
- `POST /api/update/check`
- `POST /api/webhooks/{plugin_id}/{route}`

### 关键固定语义

#### OneBot11 协议管理

Compatibility response 固定返回 `events`、`message_segments`、`read_capabilities`、`provider_extensions` 四类能力矩阵，provider 支持状态固定为 `supported` 或 `unsupported`。Targets response 固定返回 `groups`、`private_users` 与可展示的 `issues`；identities resolve response 固定返回每个请求项的展示身份与失败原因。

#### 指标

Metrics response 使用 Prometheus text exposition format，并受 admin session 保护。

#### 三方账号

- 正式平台为 `bilibili`、`weibo`、`douyin`、`netease_music`；三方账号响应只暴露账号摘要、凭据状态和保存状态，不暴露 Cookie / CK 明文。
- 账号头像接口只读取已保存的头像地址，通过对应平台的受控图片来源返回内容。
- 凭据检查以 `valid`、`invalid` 或 `unknown` 作为正常的 `200` 结果；账号不存在或尚未配置凭据时返回 `platform.third_party_account_not_found`。
- 扫码登录的瞬态为 `pending_scan`、`pending_confirm`、`verification_required`，终态为 `expired`、`failed`、`succeeded`；取消接口释放对应 provider 资源。
- 插件使用 CK 时可通过 `thirdparty.account.validate` 报告 `auth_rejected` 或 `session_blocked`，Server 去重并执行权威复检，状态写回后通过 `third_party.account.changed` 通知管理面刷新。
- 订阅、用户解析、内容检查和状态展示由订阅中心插件通过三方账号 local action 与插件管理动作完成。

#### 插件设置与敏感值

插件详情 response 暴露只读 `management_ui` 元数据；插件设置接口只读写插件自己的当前生效配置；插件管理动作只投递给所属插件 runtime。插件 secrets 接口只读写所属插件的敏感值命名空间，供受保护插件管理页配置 token、webhook secret 和 API key；插件 runtime 通过 `secret.read` 读取自身命名空间内的单个值。

#### Launcher 与管理会话

Launcher 本机接口只接受本机直连请求和独立 launcher control token，带代理转发头、来自非本机地址或缺少凭据的请求统一拒绝。浏览器管理面通过 Host-only HttpOnly cookie 与 CSRF 建立会话；Bearer transport 保留给非浏览器客户端。

#### 更新

Web 只读取状态并触发受信元数据检查，不下载或安装更新。Windows 自动安装由 Launcher 和外置 updater 执行；正式 Authenticode 与真实签名 packaged E2E 未满足时，发布元数据保持 `guided`。

#### 渲染管理

模板预览工作区使用同步 HTML 预览接口展示当前模板文档；模板资源接口只读取受控模板资源。模板列表和详情返回 `source`，用于区分系统模板与插件携带模板；模板目录可提供 `preview.json` 作为预览示例数据。

#### 治理

黑白名单条目使用单条 upsert 与单条删除；白名单状态通过独立开关接口切换。`GET /api/governance/command-policy` 返回当前生效的默认权限、冷却配置和命令级权限设置，供指令中心直接展示。

#### 配置与插件生命周期

- `PUT /api/config` response 使用 `apply_effects.applied_now`、`apply_effects.reloaded_now`、`apply_effects.restart_required_fields`
- `restart_required` 与 `apply_effects.restart_required_fields` 保持一致
- `/api/plugins`、`/api/plugins/{plugin_id}`、enable / disable / reload / recover 响应与 `/ws/events` 插件生命周期分支统一使用正式 `state` 枚举与可选 `state_diagnosis`

#### 恢复与运行环境任务

`recovery.confirm` request 支持 `review_ids` 与可选 `note`；`runtime.bootstrap` request 支持可选 `resources` 列表。异步任务的创建、运行和完成结果通过管理日志 `source=tasks` 暴露，不单独提供任务查询接口。

## 通用规则

- 规划文档解释设计意图，对外接口以 `contracts/` 为准
- 若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明
- 新增或改变对外接口、协议、schema、状态、错误码、事件、CLI 或发布元数据的正式语义时，先更新对应契约，再同步受影响的实现、测试、样例、生成物和文档；修复实现以符合现有契约时，直接修实现并按风险验证
- `fixtures/` 与 `examples/` 只能从这里派生，不能反向覆盖这里
