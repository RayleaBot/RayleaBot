# Contracts

`contracts/` 是 RayleaBot v0.1 对外接口、schema、错误码和发行元数据的唯一正式来源。

规则：

- 规划文档解释设计意图，`contracts/` 裁决最终接口。
- 若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明。
- 任一涉及 HTTP API、WebSocket、plugin manifest、plugin protocol、config schema、error codes、release manifest 的改动，必须先更新这里，再更新实现代码、测试和示例。
- 若本轮无法一次写完完整契约，也必须先保留合法骨架与显式 `TODO`，不能跳过 contract 直接写实现。

文件清单：

- `plugin-info.schema.json`：插件 `info.json` schema
- `plugin-protocol.schema.json`：插件 JSONL 协议 schema
- `web-api.openapi.yaml`：HTTP API OpenAPI
- `websocket-events.yaml`：WebSocket 通道与 envelope 规范
- `config.user.schema.json`：`config/user.yaml` schema
- `error-codes.yaml`：统一错误码目录
- `release-manifest.schema.json`：`release_manifest.json` 与 `build_info.json` schema

补充约束：

- `contracts/` 不接受“实现里已经这样做了，所以 contract 先不改”的倒置流程。
- `fixtures/` 与 `examples/` 只能从这里派生，不能反向覆盖这里。
