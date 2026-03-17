# Contracts Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `contracts/` 目录特有、长期有效的规则。

## Contract Rules

- `contracts/` 是唯一正式来源；实现、README、fixtures、examples 都不能反向裁决这里。
- contract 只表达已经决定的正式边界；不要把未来能力、猜测字段、临时兼容名、占位 skeleton 直接写进正式 contract。
- schema / OpenAPI / event catalog / error code 命名必须单一；不要并存旧名和新名。
- 能收窄就收窄：优先明确 `required`、`enum`、`const`、`pattern`、`description`、固定字段形状。
- TODO 只能保留为窄 TODO，且必须指向具体未冻结边界，不能把整块接口故意写宽。

## Companion Updates

- 改 contract 时，必须同步检查对应 fixtures、examples、tests、README、CI 校验。
- `x-fixtures` 或等价示例引用必须保持存在且可解析。
- 如果某个接口或消息还没有进入 fixture-ready，就不要把它抢先塞进正式 contract。

## Review Heuristics

- 不要因为“实现已经这么写了”就反向放宽 contract。
- 不要在没有正式来源支撑的情况下新增状态、错误码、事件名、字段名。
- 若 contract 与文档冲突，以 contract 为准，并在同一变更中修正文档说明。

## Consult Before Editing

- 契约范围与 TODO：`contracts/README.md`
- 工程基线与固定选型：`docs/engineering/baseline.md`
- 实施顺序与阶段边界：`docs/engineering/implementation-order.md`
