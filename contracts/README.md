# Contracts

`contracts/` 是 RayleaBot 当前对外接口、schema、错误码和发行元数据的唯一正式来源。

## 当前状态

### Fixture-ready 正式契约

当前已有 12 份 fixture-ready formal contracts：

- `backup-manifest.schema.json`
- `config.user.schema.json`
- `deps-manifest.schema.json`
- `error-codes.yaml`
- `web-api.openapi.yaml`
- `websocket-events.yaml`
- `plugin-info.schema.json`
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
  - 图片渲染 Chromium、Python / Node.js 运行环境资源的可信来源列表、SHA256、归档格式与相对入口
- `error-codes.yaml`
  - 统一错误码命名、默认消息资源键、HTTP 语义和适用范围
- `web-api.openapi.yaml`
  - 当前已冻结的管理 HTTP 接口
  - 当前包含 setup / session、loopback launcher bootstrap、config snapshot/update、protocol snapshot / compatibility、OneBot target / identity resolution、plugin lifecycle、plugin rich detail、plugin settings、plugin secrets、third-party accounts、third-party QR login、plugin management actions、governance 管理面、logs / system / metrics surfaces、scheduler 任务列表与手动触发、recovery recheck / confirm、runtime bootstrap、render templates、preview HTML 与模板资源读取面
  - `PUT /api/config` response 固定返回 `apply_effects.applied_now`、`apply_effects.reloaded_now`、`apply_effects.restart_required_fields`
  - plugin lifecycle surface 统一使用正式 `state` 枚举与可选 `state_diagnosis`
- `websocket-events.yaml`
  - 当前已冻结的管理 WebSocket envelope、事件名和 payload 约束
  - `events.received` 的通用 `event_type + summary` 分支当前包含 `governance.changed`
- `plugin-info.schema.json`
  - 插件 `info.json` 的安装前静态校验、兼容性门禁、能力声明、能力参数和迁移判断边界
  - 当前已冻结 `default_config`、`default_config_file`、`role`、`icon`、`repo`、`homepage`、`keywords`、`screenshots`、`system_dependencies`、`platforms`、`management_ui`、`render_templates`、`help` 与插件详情页投影所需 metadata
  - `capabilities` 使用正式 capability 集合，覆盖基础 local action、治理 local action、冻结的 OneBot 单动作能力与 3 个正式 provider 扩展动作
  - `capability_parameters` 表达 `http.request`、`storage.file` 与 `event.expose_webhook` 的边界参数
  - `concurrency` 省略时按 `1` 处理，声明值用于插件事件并发 opt-in
  - command `permission` 省略时使用 `permission.default_level`
- `plugin-management-ui.yaml`
  - 插件内置管理页的公共静态资源路由前缀
  - 当前固定为 `/plugin-ui/{plugin_id}/...`
- `plugin-management-ui-bridge.schema.json`
  - Web 宿主页与插件内置 iframe 的正式桥接消息结构
  - 当前固定 `page.ready`、`host.init`、`settings.reload`、`settings.save`、`settings.changed`、`secrets.reload`、`secrets.save`、`secrets.changed`、`scheduler.trigger`、`scheduler.triggered`、`render_template.open`、`protocol.targets.reload`、`protocol.targets.changed`、`protocol.identities.resolve`、`protocol.identities.resolved`、`plugin.action.invoke`、`plugin.action.result` 与 `error`
- `plugin-protocol.schema.json`
  - 插件 Runtime JSONL 协议
  - 当前冻结 `init`、`init_progress`、`init_ack`、`event`、`result`、`error`、`ping`、`pong`、`shutdown`
  - `error` 帧由插件终态失败与平台 local action 失败共用，固定包含 `code`、`message`，可选 `details`
  - `message.send`、`message.reply` 使用 shared `message.segments` payload
  - `init.bot` 在协议身份可用时出现，`bot.identity.changed` 用于向运行中插件同步当前 bot 身份
  - 协议身份不可用时 `init.bot` 缺省或 `bot.identity.changed` 携带空身份；依赖 `self_id` 的出站 OneBot 动作返回正式 `error` 帧，不依赖身份的 local action 保持可用
  - `logger.write`、`storage.kv`、`storage.file`、`http.request`、`config.read`、`config.write`、`plugin.list`、`secret.read`、`thirdparty.account.read`、`governance.blacklist.read`、`governance.blacklist.write`、`governance.whitelist.read`、`governance.whitelist.write`、`governance.command_policy.read`、`scheduler.create`、`event.expose_webhook`、`render.image` 已进入正式 local action RPC surface；`scheduler.create.log_label` 用于定时任务管理日志展示；`secret.read` 只读取调用插件自己的 secret 命名空间；`thirdparty.account.read` 只读取插件 manifest 声明平台的已启用有效三方账号，并把 CK 标记为 secret 值；`render.image` 支持系统模板 ID 和调用插件声明的模板短 ID
  - local action `action` 帧使用 `parent_request_id` 归属到对应事件；并发插件必须提供该字段
  - 当前已冻结 OneBot 单动作 surface，provider 扩展 action 固定为 `provider.napcat.message_emoji.like.set`、`provider.napcat.group.sign.set` 与 `provider.luckylillia.friend_groups.get`
  - 正式 `event.event_type` 固定包含 `scheduler.trigger`、`plugin.started`、`management.action`、`config.changed`、`webhook.received`、`bot.identity.changed` 以及 OneBot `message.*`、`message_sent.*`、`notice.*`、`request.*`、`meta.*`
  - `event.payload.onebot` 正式暴露 `meta_event_type`、`interval`、`status`
  - 正式 inbound / outbound segment 种类当前为 `text`、`image`、`at`、`at_all`、`face`、`reply`、`record`、`video`、`file`、`flash_file`、`json`、`xml`、`markdown`、`music`、`contact`、`forward`、`node`、`poke`、`dice`、`rps`、`mface`、`keyboard`、`shake`
- `release-manifest.schema.json`
  - `release_manifest.json` 与 `build_info.json` 的正式字段结构
  - `SHA256SUMS.txt` 继续由 release tool 的生成与校验规则裁决，不作为独立 schema
- `cli-commands.yaml`
  - `reset-admin`、`backup`、`restore`、`doctor`、`cleanup` 的正式命令模型

## 当前延后到后续版本的边界

### Plugin Protocol

- 调试流
- 批量消息
- 复杂流式回传

### Release Metadata

- 签名服务
- 增量升级
- 发布流水线策略

## OpenAPI 已冻结范围

当前没有额外的管理 HTTP 路由保留在正式 OpenAPI 冻结范围之外。

当前已进入 OpenAPI 冻结范围的 protocol management surface：

- `GET /api/protocols/onebot11/compatibility`
- `GET /api/protocols/onebot11/targets`
- `POST /api/protocols/onebot11/identities/resolve`

其中 compatibility response 固定返回 `events`、`message_segments`、`read_capabilities`、`provider_extensions` 四类能力矩阵；provider 支持状态固定为 `supported` 或 `unsupported`。targets response 固定返回 `groups`、`private_users` 与可展示的 `issues`。identities resolve response 固定返回每个请求项的展示身份与失败原因。

当前已进入 OpenAPI 冻结范围的 scheduler surface：

- `GET /api/system/scheduler/jobs`
- `POST /api/system/scheduler/jobs/{job_id}/trigger`

当前已进入 OpenAPI 冻结范围的 metrics surface：

- `GET /api/system/metrics`

其中 response 为 Prometheus text exposition format，并受 admin session 保护。

当前已进入 OpenAPI 冻结范围的 third-party account surface：

- `GET /api/third-party/accounts`
- `PUT /api/third-party/accounts/{platform}/{account_id}`
- `DELETE /api/third-party/accounts/{platform}/{account_id}`
- `POST /api/third-party/accounts/{platform}/login/qrcode`
- `GET /api/third-party/accounts/{platform}/login/qrcode/{login_id}`

其中正式平台为 `bilibili`、`weibo`、`douyin`、`netease_music`；三方账号响应只暴露账号摘要、凭据状态和保存状态，不暴露 Cookie / CK 明文。扫码登录统一使用通用三方账号扫码接口。订阅、用户解析、内容检查和状态展示由订阅中心插件通过 `thirdparty.account.read` 与插件管理动作承接。

当前已进入 OpenAPI 冻结范围的 plugin settings surface：

- `GET /api/plugins/{plugin_id}/settings`
- `PUT /api/plugins/{plugin_id}/settings`
- `POST /api/plugins/{plugin_id}/management/actions`

其中插件详情 response 会暴露只读 `management_ui` 元数据；插件设置接口只读写插件自己的当前生效配置；插件管理动作接口只把插件管理页动作转给插件 runtime 处理。

当前已进入 OpenAPI 冻结范围的 plugin secrets surface：

- `GET /api/plugins/{plugin_id}/secrets`
- `PUT /api/plugins/{plugin_id}/secrets`

其中插件 secrets 接口只读写插件自己的敏感值命名空间，供受保护插件管理页配置 token、webhook secret 和 API key 等敏感值；插件 runtime 通过 `secret.read` 读取自身命名空间内的单个值。

当前已进入 OpenAPI 冻结范围的 launcher bootstrap surface：

- `GET /api/launcher/status`
- `POST /api/launcher/shutdown`

其中 launcher surface 只接受本机直连请求，带代理转发头或来自非本机地址的请求返回 `403`。Web 管理面会话仍通过初始化和登录接口建立。

当前已进入 OpenAPI 冻结范围的 render management surface：

- `GET /api/system/render/templates`
- `GET /api/system/render/templates/{template_id}`
- `POST /api/system/render/templates/{template_id}/preview-html`
- `GET /api/system/render/templates/{template_id}/asset`

其中模板预览工作区使用同步 HTML 预览接口展示当前模板文档；模板资源接口只读取受控模板资源。模板列表和详情返回 `source`，用于区分系统模板与插件携带模板；模板目录可提供 `preview.json` 作为预览示例数据。

当前已进入 OpenAPI 冻结范围的 governance surface：

- `GET /api/governance/blacklist`
- `POST /api/governance/blacklist/entries`
- `DELETE /api/governance/blacklist/entries/{entry_type}/{target_id}`
- `GET /api/governance/whitelist`
- `PUT /api/governance/whitelist/state`
- `POST /api/governance/whitelist/entries`
- `DELETE /api/governance/whitelist/entries/{entry_type}/{target_id}`
- `GET /api/governance/command-policy`

其中黑白名单条目使用单条 upsert 与单条删除；白名单状态通过独立开关接口表达。`GET /api/governance/command-policy` 继续返回当前生效的默认权限、冷却配置和命令级权限投影，供指令中心直接展示。

当前已进入正式边界的 config / lifecycle semantics：

- `PUT /api/config` response 使用 `apply_effects.applied_now`、`apply_effects.reloaded_now`、`apply_effects.restart_required_fields`
- `restart_required` 与 `apply_effects.restart_required_fields` 保持一致
- `/api/plugins`、`/api/plugins/{plugin_id}`、enable / disable / reload / recover 响应与 `/ws/events` 插件生命周期分支统一使用正式 `state` 枚举与可选 `state_diagnosis`

当前已进入 OpenAPI 冻结范围的 recovery / runtime task surface：

- `POST /api/system/recovery/recheck`
- `POST /api/system/recovery/confirm`
- `POST /api/system/runtime/bootstrap`

其中 `recovery.confirm` request 支持 `review_ids` 与可选 `note`；`runtime.bootstrap` request 支持可选 `resources` 列表。异步任务的创建、运行和完成结果通过管理日志 `source=tasks` 暴露，不提供单独任务查询面。

## 通用规则

- 规划文档解释设计意图，`contracts/` 裁决最终接口
- 若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明
- 任一涉及 HTTP API、WebSocket、plugin manifest、plugin protocol、release metadata、config schema、error codes 的改动，必须先更新这里，再更新实现代码、测试和示例
- `fixtures/` 与 `examples/` 只能从这里派生，不能反向覆盖这里
