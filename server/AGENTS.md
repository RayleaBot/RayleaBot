# Server Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `server/` 目录特有、长期有效的规则。

## Server-Specific Rules

- `server/` 的实现必须优先消费现有 `contracts/`、`fixtures/` 与 `examples/`，不能自行发明第二套接口或状态语义。
- 配置加载、错误响应、健康检查、插件只读视图、adapter 状态等对外行为，都必须与对应 contract 保持一致。
- 新增包时保持职责收敛；优先一包一职责，不要为了未来协议、未来 runtime、未来持久化预埋大而全抽象。
- 若当前 contract 只支持最小只读或 shell 语义，实现也必须保持最小，不要抢跑写操作、运行时编排或跨子系统桥接。
- 不要绕过 schema 校验直接消费用户配置、plugin manifest 或其他外部输入。
- 不要在 `server/` 中复制 contract 真相；实现应引用和消费它们，而不是重新定义一套常量和文档。

## Testing Rules

- 变更优先复用 `fixtures/` 与 `examples/`，不要优先写散乱的 ad-hoc 样例。
- 任何会影响 HTTP shape、状态名、错误码、adapter 行为、plugin discovery 行为的改动，都必须补最小回归测试。
- 若改动触及对外边界，同时检查是否满足四件套门禁：实现、契约、测试、示例。

## Consult Before Major Changes

- 配置与工程基线：`docs/engineering/baseline.md`
- HTTP / WebSocket / errors / plugin contracts：`contracts/README.md`
- 服务器当前实现边界：`server/README.md`
