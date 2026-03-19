# Contracts

`contracts/` 是 RayleaBot v0.1 对外接口、schema、错误码和发行元数据的唯一正式来源。

## Fixture-Ready 范围

当前进入 fixture-ready 的正式契约共有 7 个文件：

- `config.user.schema.json`
- `error-codes.yaml`
- `web-api.openapi.yaml`
- `websocket-events.yaml`
- `plugin-info.schema.json`
- `plugin-protocol.schema.json`
- `release-manifest.schema.json`

这些文件都必须带 `x-fixtures` 或等价示例引用，并接受 PR 级 CI 的存在性、可解析性和最低契约覆盖校验。

## 文件职责

- `config.user.schema.json`
  - 负责 `config/user.yaml` 的正式机器可校验结构。
  - 冻结 v0.1 首版闭环配置，不预留宽泛占位。
- `error-codes.yaml`
  - 负责统一错误码命名、默认消息资源键、HTTP 语义和适用范围。
- `web-api.openapi.yaml`
  - 负责当前已冻结的最小管理 HTTP 接口，包括最小 bootstrap/admin credential-source 与 management session issuance surfaces。
- `websocket-events.yaml`
  - 负责 Phase 1 最小管理通道的 shared envelope、事件名和 payload 约束。
- `plugin-info.schema.json`
  - 负责插件 `info.json` 的安装前静态校验、兼容性门禁、权限声明和数据迁移判断边界。
  - 依赖 Phase 1 已冻结的配置、错误码和管理面语义。
- `plugin-protocol.schema.json`
  - 负责插件 Runtime 最小 JSONL 协议。
  - 本轮只冻结 `init`、`init_progress`、`init_ack`、`event`、`result`、`error`、`shutdown`。
  - 依赖 Phase 1 已冻结的错误码、任务状态和运行时边界。
- `release-manifest.schema.json`
  - 负责 `release_manifest.json` 与 `build_info.json` 的正式字段结构。
  - 只表达发行物整体元数据，不重复展开 `.deps/manifest.json` 中的受控运行时资源清单。

## Phase 1 与 Phase 2 的关系

- Phase 1 冻结了平台配置、错误码、管理面 HTTP API 和管理面 WebSocket 事件。
- Phase 2 在此基础上补齐：
  - 插件包边界：`plugin-info.schema.json`
  - 插件 Runtime 协议边界：`plugin-protocol.schema.json`
  - 发行物边界：`release-manifest.schema.json`
- 进入后续 server 最小空壳时，应该只消费这 7 份正式 contract，而不是从 README 或规划正文重新猜字段。

## 当前仍保留为 TODO 的边界

### Plugin Manifest

- `default_config`
- `commands`
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

- `action`
- `ping`
- `pong`
- 调试流
- 文件传输
- 批量消息
- 复杂流式回传
- 平台到插件方向的扩展 `error` 语义

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

## 当前未进入 OpenAPI 的 TODO

以下 HTTP 路由仍属于规划内能力，但不在正式 OpenAPI 冻结范围内：

- `GET /api/setup/status`
- `DELETE /api/session`
- `POST /api/session/launcher-token`
- `POST /api/system/shutdown`
- `GET /api/system/status`
- `POST /api/plugins/{plugin_id}/reload`
- `DELETE /api/plugins/{plugin_id}`
- `GET /api/config`
- `PUT /api/config`
- `GET /api/logs`
- `POST /api/webhooks/{plugin_id}/{route}`

这些接口在进入 fixture-ready 前，不得以 skeleton 形式提前回写到 `contracts/web-api.openapi.yaml`。

## 文档同步 TODO

- `platform.config_error` 已收口为 `platform.invalid_config`。
- 旧示例中的 `task.updated` 已收口为 `tasks.updated`。
- Phase 0 中较宽的配置顶层分组已收口为 10 个正式对象。
- Phase 2 中的 `plugin-info` / `plugin-protocol` / `release-manifest` 已进入 fixture-ready，相关规划正文的“建议字段”表需要同步为“正式 contract 以 contracts 为准”。

## 通用规则

- 规划文档解释设计意图，`contracts/` 裁决最终接口。
- 若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明。
- 任一涉及 HTTP API、WebSocket、plugin manifest、plugin protocol、release metadata、config schema、error codes 的改动，必须先更新这里，再更新实现代码、测试和示例。
- `contracts/` 不接受“实现里已经这样做了，所以 contract 先不改”的倒置流程。
- `fixtures/` 与 `examples/` 只能从这里派生，不能反向覆盖这里。
