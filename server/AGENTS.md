# Server Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `server/` 目录特有、长期有效的规则。

## Server Rules

- `server/` 实现优先消费当前 `contracts/`、`fixtures/` 与 `examples/`，不在实现中自造第二套接口或状态语义。
- 配置加载、错误响应、健康检查、插件只读视图、adapter 状态、治理裁决、日志详情和任务结果都必须与正式 contract 保持一致。
- 不绕过 schema 校验直接消费用户配置、plugin manifest 或其他外部输入。
- Server 是正式状态源；不要把 Web、Launcher 或插件运行时做成第二状态源。

## Security Rules

- 管理 API、配置 API、日志、fixtures、examples、测试快照不得返回或记录明文凭据。
- Cookie / CK 等敏感凭据只写入 secret store，不写入配置文件、日志或管理响应。

## Architecture Constraint

- 不新增平行配置读取链路、日志栈、路由栈或状态源。
- 新增包需职责收敛，优先一包一职责，不为未来协议、未来 runtime 或未来持久化预埋大而全抽象。
- 开发辅助工具不得通过 `server/go.mod` 的 `tool` 指令引入与 server 运行无关的大型依赖图；需要热重载或生成能力时，优先使用仓库脚本或独立工具边界，并同步工程基线文档。

## Config and Policy Reading

- 聊天策略优先读取正式配置字段：
  - `admin.super_admins`
  - `permission.default_level`
  - `user.command_rate_limit`
  - `group.command_rate_limit`
  - `user.cooldown_reply`
- 白名单、黑名单、默认权限、冷却与 super admin 判断保持同一套正式语义。

## Testing Rules

- 任何会影响 HTTP shape、状态名、错误码、adapter 行为、plugin discovery、治理裁决、日志内容或配置读取的改动，都必须补最小回归测试。
- 变更对外边界时，同时检查四件套是否齐全：实现、契约、测试、示例。
- 优先复用 `fixtures/` 与 `examples/`，不要先写散乱的 ad-hoc 样例。
- 插件 runtime helper 测试发出预期协议违规 frame 后，不要立刻退出进程；应等待 stdin 关闭或管理器终止进程，避免 CI 因进程退出竞态把协议违规误判为 `plugin.internal_error`。

## Cross-Surface Checks

- Web API 或共享 schema 改动时，同时检查 Web 与 Launcher 的生成类型是否需要更新。
- 修改 `server/internal/storage/migrations/*.sql` 或 `server/internal/sqlcqueries/*.sql` 时，运行 `sqlc generate`，提交 `server/internal/sqlcgen/` 生成结果，并用 `sqlc diff` 确认无漂移。
- 不在 `server/` 复制 contract 真相；实现消费它们，而不是再维护一套平行常量和文档。

## Consult Before Major Changes

- 当前服务端能力与入口：`server/README.md`
- 工程基线与默认命令：`docs/engineering/baseline.md`
- 正式 HTTP / WebSocket / errors / plugin contracts：`contracts/README.md`
- 架构、状态模型、事件模型：`docs/architecture/`
