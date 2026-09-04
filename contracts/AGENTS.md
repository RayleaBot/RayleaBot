# Contracts Agent Guide

先遵守根 `AGENTS.md`，从 `contracts/README.md` 定位相关契约。

## Definitions

- 只描述已确定的对外语义，不写未来能力、猜测字段、临时别名或宽泛占位结构；未决定事项仅保留指向具体边界的窄 TODO。
- 字段、状态、错误码和事件保持单一命名；不因实现已有偏差而放宽契约。
- 优先明确 required、enum、const、pattern 和对象形状，避免依赖隐式语义。

## API and State

- 可能无界增长的 list API 明确分页默认值与上限、排序、过滤和空列表形状；有明确最大项数的 bounded snapshot 可以省略分页，但仍需定义顺序与空列表。
- 单资源查询明确不存在时的 HTTP 状态码和错误码；批量操作明确部分成功与全部失败的结构。请求和响应优先使用结构化对象。
- 已有 enum wire value 保持稳定，修改按 breaking change 处理；状态说明用户含义、终态或瞬态及预期迁移路径。
- 明确 unknown / future 策略：可扩展展示枚举允许兼容降级，安全、权限和状态机边界按契约严格拒绝。不给未来值预留占位名称。

## Errors

- 每个错误码明确触发条件、HTTP status 与 message 策略；不同接口触发条件不一致时拆分错误码或明确 scope。
- 程序分支依赖 code 和已定义的结构化 details，message 面向读者；details 只在契约声明时出现，优先固定对象形状。
- 新增错误码时提供必要错误样例，检查受影响客户端的消费点。

## Merge Readiness

- 按实际依赖检查实现、样例、测试、生成物与文档；生成来源和输出从现有工程脚本确认，不维护完整输出清单。
- fixture-ready 是合并验收条件。契约和样例可以先后编辑，合并前 `x-fixtures` 或等价引用须存在、可解析、可被 CI 枚举，并通过必要校验。
