# Contracts Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `contracts/` 目录特有、长期有效的规则。

## Contract Rules

- `contracts/` 是唯一正式来源；实现、README、fixtures、examples 和生成类型都不能反向裁决这里。
- contract 只表达已经冻结的正式边界，不写未来能力、猜测字段、临时兼容名或宽泛占位结构。
- 命名保持单一：状态名、错误码、事件名、字段名、配置键名不并存旧名和新名。
- 能收窄就收窄，优先明确 `required`、`enum`、`const`、`pattern`、固定对象形状和稳定说明。
- TODO 只能保留为窄 TODO，并指向具体尚未冻结的边界。

## Current Surface Expectations

- 当前正式 surface 已覆盖管理 HTTP / WebSocket、插件 manifest、插件协议、插件内置管理页静态路由与桥接、用户配置、错误码、release metadata、backup manifest、deps manifest、CLI、三方账号、三方监控、三方媒体代理、Bilibili source 状态和 Bilibili 扫码登录。
- governance、plugin settings、plugin rich detail、protocol compatibility、render management、recovery/runtime tasks、launcher bootstrap 都属于已冻结范围。
- `plugin-management-ui.yaml` 与 `plugin-management-ui-bridge.schema.json` 继续表达插件内置管理页的正式边界，不把这类结构塞回 OpenAPI。

## Companion Updates

- 改 contract 时，必须同步检查 fixtures、examples、tests、README、生成类型和 CI 校验。
- `x-fixtures` 或等价示例引用必须存在、可解析、可被 CI 枚举。
- OpenAPI 或共享 schema 改动后，同步检查 `web/src/types/generated.ts` 和 `launcher/src/shared/web-api.generated.ts` 的一致性门禁。
- 尚未进入 fixture-ready 的结构，不写进正式 contract。

## Review Heuristics

- 不因实现已经存在就反向放宽 contract。
- 不在没有正式来源支撑的情况下新增状态、错误码、事件名、字段名或 release metadata。
- 若 contract 与文档冲突，以 contract 为准，并在同一变更中修正文档说明。

## Consult Before Editing

- 契约范围与说明：`contracts/README.md`
- 工程基线与默认命令：`docs/engineering/baseline.md`
- 实施顺序与阶段边界：`docs/engineering/implementation-order.md`
