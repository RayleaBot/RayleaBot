# Server Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `server/` 目录特有、长期有效的规则。

## Server Rules

- `server/` 实现优先消费当前 `contracts/`、`fixtures/` 与 `examples/`，不在实现中自造第二套接口或状态语义。
- 配置加载、错误响应、健康检查、插件只读视图、adapter 状态、治理裁决、日志详情和任务结果都必须与正式 contract 保持一致。
- 新增包时保持职责收敛；优先一包一职责，不为未来协议、未来 runtime 或未来持久化预埋大而全抽象。
- 不绕过 schema 校验直接消费用户配置、plugin manifest 或其他外部输入。
- Server 是正式状态源；不要把 Web、Launcher 或插件运行时做成第二状态源。

## Current Boundary Expectations

- 当前主链包含治理读写、插件设置、插件内置管理页静态资源、OneBot11 协议快照、命令拒绝日志、render management、recovery/runtime tasks 与 launcher bootstrap。
- 插件内置管理页资源只读取 `management_ui.entry` 所在目录下的文件，不扩成目录枚举或管理数据出口。
- 命令被白名单、黑名单、权限或冷却拒绝时，继续通过现有管理日志主链记录，不新增第二套拒绝语义。

## Config and Policy Reading

- 聊天策略优先读取正式配置字段：
  - `admin.super_admins`
  - `permission.default_level`
  - `user.command_rate_limit`
  - `group.command_rate_limit`
  - `user.cooldown_reply`
- 旧兼容字段只作为显式回退路径，不作为新的主读取口径。
- 白名单、黑名单、默认权限、冷却与 super admin 判断保持同一套正式语义。

## Testing Rules

- 任何会影响 HTTP shape、状态名、错误码、adapter 行为、plugin discovery、治理裁决、日志内容或配置读取的改动，都必须补最小回归测试。
- 变更对外边界时，同时检查四件套是否齐全：实现、契约、测试、示例。
- 优先复用 `fixtures/` 与 `examples/`，不要先写散乱的 ad-hoc 样例。

## Cross-Surface Checks

- Web API 或共享 schema 改动时，同时检查 Web 与 Launcher 的生成类型是否需要更新。
- 不在 `server/` 复制 contract 真相；实现消费它们，而不是再维护一套平行常量和文档。

## Consult Before Major Changes

- 当前服务端能力与入口：`server/README.md`
- 工程基线与默认命令：`docs/engineering/baseline.md`
- 正式 HTTP / WebSocket / errors / plugin contracts：`contracts/README.md`
