# Plugin SDK Docs

本目录用于说明 RayleaBot 官方 Python / Node.js SDK 的当前使用边界。

## 当前 SDK 角色

- SDK 为当前插件协议提供便利封装，服务于已落地的 runtime 主链路和示例插件。
- 文档范围以 `init` / `init_progress` / `init_ack`、事件接收、`ping/pong`、`shutdown` 与三种正式 `action` 为主。
- SDK 应与 builtin 资源、示例插件和当前 dispatcher / scheduler 投递模型保持一致。

## 当前适用范围

- Python / Node.js SDK 只覆盖当前正式协议与已落地 action。
- 更宽的调试流、复杂流式回传、批量消息和额外 action 仍未进入正式协议范围。
- SDK 说明需要与 `contracts/plugin-protocol.schema.json`、`docs/plugin/` 和 `examples/plugins/` 保持一致。

## 维护规则

- SDK 说明必须服从正式插件协议契约。
- 若 SDK 需要新增字段、消息类型或 action，先更新 `contracts/`，再补 fixtures、示例和 SDK 文档。
- SDK 是协议的实现便利层，不单独裁决对外语义。
