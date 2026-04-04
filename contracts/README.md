# Contracts

`contracts/` 是 RayleaBot v0.1 对外接口、schema、错误码和发行元数据的唯一正式来源。

## 当前状态

### Fixture-ready 正式契约

当前已有 10 份 fixture-ready formal contracts：

- `backup-manifest.schema.json`
- `config.user.schema.json`
- `deps-manifest.schema.json`
- `error-codes.yaml`
- `web-api.openapi.yaml`
- `websocket-events.yaml`
- `plugin-info.schema.json`
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
  - Chromium、Python / Node.js 运行环境资源的有序来源列表、SHA256、归档格式与相对入口
- `error-codes.yaml`
  - 统一错误码命名、默认消息资源键、HTTP 语义和适用范围
- `web-api.openapi.yaml`
  - 当前已冻结的管理 HTTP 接口
  - 当前包含 setup / session、loopback launcher bootstrap、config snapshot/update、plugin lifecycle、plugin grants、tasks / logs / system surfaces、recovery recheck / confirm、runtime bootstrap、render preview 与 render artifact 读取面
- `websocket-events.yaml`
  - 当前已冻结的管理 WebSocket envelope、事件名和 payload 约束
- `plugin-info.schema.json`
  - 插件 `info.json` 的安装前静态校验、兼容性门禁、权限声明和迁移判断边界
  - command `permission` 省略时回落到 `auth.default_level`
- `plugin-protocol.schema.json`
  - 插件 Runtime JSONL 协议
  - 当前冻结 `init`、`init_progress`、`init_ack`、`event`、`result`、`error`、`ping`、`pong`、`shutdown`
  - `message.send`、`message.reply` 使用 shared `message.segments` payload
  - `logger.write`、`storage.kv`、`storage.file`、`http.request`、`config.read`、`config.write`、`scheduler.create`、`event.expose_webhook`、`render.image` 已进入正式 local action RPC surface
  - 正式 outbound segment 种类当前为 `text`、`image`、`at`、`at_all`、`face`、`reply`
- `release-manifest.schema.json`
  - `release_manifest.json` 与 `build_info.json` 的正式字段结构
- `cli-commands.yaml`
  - `reset-admin`、`backup`、`restore`、`doctor`、`migrate`、`cleanup` 的正式命令模型

## 当前仍保留为 TODO 的边界

### Plugin Manifest

- `default_config`
- `concurrency`
- `role`
- `icon`
- `repo`
- `homepage`
- `keywords`
- `screenshots`
- `system_dependencies`
- `binary` 插件形态和 `binary` 运行时

### Plugin Protocol

- 调试流
- 批量消息
- 复杂流式回传
- 更宽的平台到插件方向 `error` 语义
- `message.send`、`message.reply`、`logger.write`、`storage.kv`、`storage.file`、`http.request`、`config.read`、`config.write`、`scheduler.create`、`event.expose_webhook`、`render.image` 之外的其他 `action`

### Release Metadata

- `manifest_version`
- `project_name`
- 独立 `target_platform` / `target_arch`
- 显式 `bundled_runtimes` 列表
- `compatible_core_range`
- 签名服务
- 增量升级
- 发布流水线策略
- `SHA256SUMS.txt` 文件内容结构

### Deps Manifest

- 更宽的资源种类矩阵
- 资源签名与附加校验文件
- 显式解压目标目录覆盖

### CLI

- CLI 与 HTTP task 模型的共享执行路径验证

### Recovery Summary

- 更宽的跨版本恢复自动修复策略
- 更细的插件数据迁移动作族

## 当前未进入 OpenAPI 的 TODO

当前没有额外的管理 HTTP 路由保留在正式 OpenAPI 冻结范围之外。

当前已进入 OpenAPI 冻结范围的 plugin grants surface：

- `GET /api/plugins/{plugin_id}/grants`
- `POST /api/plugins/{plugin_id}/grants`
- `DELETE /api/plugins/{plugin_id}/grants/{capability}`

其中 grant request / response / list item 支持可选 `expires_at`，用于表达当前生效授权的时效窗口。

当前已进入 OpenAPI 冻结范围的 launcher bootstrap surface：

- `POST /api/session/launcher-token`
- `POST /api/session/launcher-admission`

其中 `launcher-token` 用于本机回环的一次性短时 bootstrap，`launcher-admission` 负责把一次性 token 换成正常管理 session。

当前已进入 OpenAPI 冻结范围的 render management surface：

- `POST /api/system/render/preview`
- `GET /api/system/render/artifacts/{artifact_id}`

其中 `render.preview` 任务详情会在 `result.details` 中暴露 `artifact_id`、`image_url`、`mime`、`cache_key`、`template`、`theme`、`from_cache`。

当前已进入 OpenAPI 冻结范围的 recovery / runtime task surface：

- `POST /api/system/recovery/recheck`
- `POST /api/system/recovery/confirm`
- `POST /api/system/runtime/bootstrap`

其中 `recovery.confirm` request 支持 `review_ids` 与可选 `note`；任务详情会在 `result.details` 中暴露 `confirmed_review_ids`、`operator_id`、`note` 与更新后的 `recovery_summary`。`runtime.bootstrap` request 支持可选 `resources` 列表；任务详情会在 `result.details.resources` 中暴露每类资源的缓存归档、展开目录、已尝试来源列表与命中来源。

## 通用规则

- 规划文档解释设计意图，`contracts/` 裁决最终接口
- 若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明
- 任一涉及 HTTP API、WebSocket、plugin manifest、plugin protocol、release metadata、config schema、error codes 的改动，必须先更新这里，再更新实现代码、测试和示例
- `fixtures/` 与 `examples/` 只能从这里派生，不能反向覆盖这里
