# Contracts Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `contracts/` 目录特有、长期有效的规则。

## Contract Rules

- `contracts/` 是唯一正式来源；实现、README、fixtures、examples 和生成类型都不能反向裁决这里。
- contract 只表达已经冻结的正式边界，不写未来能力、猜测字段、临时兼容名或宽泛占位结构。
- 命名保持单一：状态名、错误码、事件名、字段名、配置键名不并存旧名和新名。
- 能收窄就收窄，优先明确 `required`、`enum`、`const`、`pattern`、固定对象形状和稳定说明。
- TODO 只能保留为窄 TODO，并指向具体尚未冻结的边界。

## API Shape Rules

- 可能无界增长的 list API 必须明确 pagination（默认页大小、最大页大小、游标或偏移语义）、sort（默认排序字段与方向）、filter（支持的字段与操作符）和 empty state。由 contract 明确限制最大项数的 bounded snapshot 可以省略 pagination，但仍须定义顺序与空列表形状。
- 单个资源查询必须明确不存在时的 HTTP 状态码和错误码，不依赖通用 404 的隐式语义。
- 批量操作必须定义部分成功与全部失败的响应形状，不返回裸数组或裸状态字符串。
- 请求体与响应体优先使用结构化对象，避免顶级标量或裸数组。

## Enum and Status Rules

- enum 的 wire value 必须稳定，变更视为 breaking change。
- 每个状态值必须附带用户可见含义说明，不只在内部注释中解释。
- 状态必须区分终态（terminal）与瞬态（transient），瞬态必须说明预期迁移路径。
- 必须定义 unknown / future 的处理策略：扩展安全的展示枚举使用兼容降级；安全、权限或状态机边界可以按 contract 严格拒绝并 fail closed。
- 不在 enum 中预留占位值（如 `reserved_1`），未来扩展通过新增值实现。

## Error Code Rules

- 每个错误码必须定义触发条件、对应 HTTP status 和 message 策略；需要携带 details 时再由 contract 明确定义其形状。
- message 面向人类可读，不用于程序分支；程序分支依赖稳定的 `code` 和 contract 已定义的结构化 `details`。
- details 只在正式 contract 声明时出现，优先使用固定字段对象；自由文本只用于 contract 明确允许的人类可读补充。
- 同一错误码在不同接口中触发条件不一致时，拆分为独立错误码或加接口级 scope。
- 新增错误码必须同步更新 fixtures 中的错误示例和 Web/Launcher 生成类型的消费点。

## Companion Updates

- 改 contract 时，必须同步检查 fixtures、examples、tests、README、生成类型和 CI 校验。
- `x-fixtures` 或等价示例引用必须存在、可解析、可被 CI 枚举。
- OpenAPI 或共享 schema 改动后，同步检查完整 generated companion：Web 的 `generated.ts` 与 `websocket.generated.ts`、Launcher 的 `web-api.generated.ts` 与 Wails bindings、Server 的嵌入 config contracts / schema bytes、Vue SDK 的 `contract.generated.ts`。
- 尚未进入 fixture-ready 的结构，不写进正式 contract。

## Review Heuristics

- 不因实现已经存在就反向放宽 contract。
- 不在没有正式来源支撑的情况下新增状态、错误码、事件名、字段名或 release metadata。
- 若 contract 与文档冲突，以 contract 为准，并在同一变更中修正文档说明。

## Consult Before Editing

- 契约范围与说明：`contracts/README.md`
- 工程基线与默认命令：`docs/engineering/baseline.md`
- 长期依赖顺序与实现边界：`docs/engineering/implementation-order.md`
