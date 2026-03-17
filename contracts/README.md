# Contracts

`contracts/` 是 RayleaBot v0.1 对外接口、schema、错误码和发行元数据的唯一正式来源。

## Phase 1 Fixture-Ready 范围

本轮进入 fixture-ready 的正式契约只有以下 4 个文件：

- `config.user.schema.json`
- `error-codes.yaml`
- `web-api.openapi.yaml`
- `websocket-events.yaml`

这 4 个文件都带有 `x-fixtures` 或等价示例引用，供 PR 级 CI 校验和后续 golden test 复用。

以下文件当前仍是 Phase 0 骨架，不在本轮 fixture-ready 范围内：

- `plugin-info.schema.json`
- `plugin-protocol.schema.json`
- `release-manifest.schema.json`

## 文件职责

- `config.user.schema.json`
  - 负责 `config/user.yaml` 的正式机器可校验结构。
  - Phase 1 只冻结首版闭环必需字段，不为未来版本预留空对象或宽泛占位。
- `error-codes.yaml`
  - 负责统一错误码命名、默认文案资源键、HTTP 语义和适用范围。
  - Phase 1 只冻结最核心错误，不保留冲突旧名并存。
- `web-api.openapi.yaml`
  - 负责本轮最小管理闭环接口。
  - Phase 1 只冻结 10 个 fixture-ready 路径，不把其他 planned route 先以 skeleton 形式写入 OpenAPI。
- `websocket-events.yaml`
  - 负责 4 个最小管理通道的 shared envelope、正式事件名和 payload 约束。
  - `events.received` 明确不是原始聊天事件广播。

## 当前未进入 OpenAPI 的 TODO

以下 HTTP 路由仍属于规划内能力，但不在本轮正式 OpenAPI 冻结范围内：

- `GET /api/setup/status`
- `POST /api/setup/admin`
- `POST /api/session/login`
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

这些接口的正式裁决要等后续 Phase 2 再进入 `contracts/web-api.openapi.yaml`，不能在实现中提前补字段。

## 当前未冻结的 WebSocket TODO

- WebSocket close reason 枚举
- 更细粒度的任务阶段事件
- 更细粒度的插件状态推送事件
- 非管理面调试事件流

这些内容在进入正式契约前，不得在前后端各自补命名。

## 文档同步 TODO

- `platform.config_error` 已被 Phase 1 收口为 `platform.invalid_config`。
- 旧示例中的 `task.updated` 已被 Phase 1 收口为 `tasks.updated`。
- Phase 0 中较宽的配置顶层分组已被 Phase 1 收口为 10 个正式对象。

## 通用规则

- 规划文档解释设计意图，`contracts/` 裁决最终接口。
- 若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明。
- 任一涉及 HTTP API、WebSocket、config schema、error codes 的改动，必须先更新这里，再更新实现代码、测试和示例。
- `contracts/` 不接受“实现里已经这样做了，所以 contract 先不改”的倒置流程。
- `fixtures/` 与 `examples/` 只能从这里派生，不能反向覆盖这里。
