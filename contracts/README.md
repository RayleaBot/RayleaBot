# Contracts

`contracts/` 是 RayleaBot v0.1 对外接口、schema、错误码和发行元数据的唯一正式来源。

## 当前状态

### Fixture-ready 正式契约

当前已有 8 份 fixture-ready formal contracts：

- `config.user.schema.json`
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
- `error-codes.yaml`
  - 统一错误码命名、默认消息资源键、HTTP 语义和适用范围
- `web-api.openapi.yaml`
  - 当前已冻结的管理 HTTP 接口
  - 当前包含 setup / session、loopback launcher bootstrap、config snapshot/update、plugin lifecycle、plugin grants 与 tasks / logs / system surfaces
- `websocket-events.yaml`
  - 当前已冻结的管理 WebSocket envelope、事件名和 payload 约束
- `plugin-info.schema.json`
  - 插件 `info.json` 的安装前静态校验、兼容性门禁、权限声明和迁移判断边界
  - command `permission` 省略时回落到 `auth.default_level`
- `plugin-protocol.schema.json`
  - 插件 Runtime JSONL 协议
  - 当前冻结 `init`、`init_progress`、`init_ack`、`event`、`result`、`error`、`ping`、`pong`、`shutdown`
  - `message.send`、`message.reply` 使用 shared `message.segments` payload
  - `logger.write`、`storage.kv`、`storage.file` 与 `http.request` 已进入正式 local action RPC surface
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
- `render.image`
- `message.send`、`message.reply`、`logger.write`、`storage.kv`、`storage.file`、`http.request` 之外的其他 `action`

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

### CLI

- CLI 与 HTTP task 模型的共享执行路径验证

## 当前未进入 OpenAPI 的 TODO

以下 HTTP 路由仍未进入正式 OpenAPI 冻结范围：

- `POST /api/webhooks/{plugin_id}/{route}`

当前已进入 OpenAPI 冻结范围的 plugin grants surface：

- `GET /api/plugins/{plugin_id}/grants`
- `POST /api/plugins/{plugin_id}/grants`
- `DELETE /api/plugins/{plugin_id}/grants/{capability}`

其中 grant request / response / list item 支持可选 `expires_at`，用于表达当前生效授权的时效窗口。

当前已进入 OpenAPI 冻结范围的 launcher bootstrap surface：

- `POST /api/session/launcher-token`
- `POST /api/session/launcher-admission`

其中 `launcher-token` 用于本机回环的一次性短时 bootstrap，`launcher-admission` 负责把一次性 token 换成正常管理 session。

## 通用规则

- 规划文档解释设计意图，`contracts/` 裁决最终接口
- 若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明
- 任一涉及 HTTP API、WebSocket、plugin manifest、plugin protocol、release metadata、config schema、error codes 的改动，必须先更新这里，再更新实现代码、测试和示例
- `fixtures/` 与 `examples/` 只能从这里派生，不能反向覆盖这里
