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
- Go 目录就是包边界；同一生命周期、同一调用路径、没有独立复用价值的 helper 优先放在同一包内，用文件名区分职责。
- 不把同一领域内部细节拆成 `model`、`spec`、`process`、`protocol`、`repository` 等薄包，除非能减少循环依赖或形成真实复用边界。
- 开发辅助工具不得通过 `server/go.mod` 的 `tool` 指令引入与 server 运行无关的大型依赖图；需要热重载或生成能力时，优先使用仓库脚本或独立工具边界，并同步工程基线文档。

## Concurrency and State

- 运行期可变共享状态（配置快照、策略引擎、订阅表）必须用原子快照（`atomic.Pointer`）或锁保护；热更新写路径不得与事件读路径共享裸字段。
- 配置热更新的读-改-写必须串行化；策略相关派生对象整体替换，不逐字段就地修改。
- 订阅/广播一律复用 `server/internal/pubsub` 的泛型 Hub，不再手写订阅表。

## Nil Defense and Assembly

- 只在系统边界（外部输入、协议入口）和真正可选协作者（logger、observer、menu 之类）处判 nil；禁止方法级"接收者 == nil 就静默返回"样板。
- 必选依赖缺失属于装配错误：在 `app` 装配层 fail-fast 返回 error，不在业务方法里静默降级或假成功。
- 可选依赖装配到接口字段时，先判断具体指针是否为 nil 再赋值，避免 typed nil 接口绕过 `== nil` 判断。

## Projection and Semantics

- 同一领域视图只在领域包构建一次（如 `plugins.BuildSummaryView`）；`management` 层只做 HTTP/WS 序列化标注，不复制投影逻辑。
- 语义分支依赖稳定的 code/枚举字段（如 `Verdict.ErrorCode`、`Verdict.Scope`），禁止比对用户可见文案字符串。

## Config and Policy Reading

- 聊天策略优先读取正式配置字段：
  - `admin.super_admins`
  - `permission.default_level`
  - `user.command_rate_limit`
  - `group.command_rate_limit`
  - `user.cooldown_reply`
- 白名单、黑名单、默认权限、冷却与 super admin 判断保持同一套正式语义。

## Testing Rules

- 只有影响 HTTP shape、状态名、错误码、adapter 行为、plugin discovery、治理裁决、日志语义、配置读取或历史 bug 路径的改动，才补最小回归测试。
- 纯搬移、包合并、等价改名或普通文案调整不新增测试；优先复用现有行为测试和构建验证。
- 变更对外边界时，同时检查四件套是否齐全：实现、契约、测试、示例。
- 优先复用 `fixtures/` 与 `examples/`，不要先写散乱的 ad-hoc 样例。
- 插件 runtime helper 测试发出预期协议违规 frame 后，不要立刻退出进程；应等待 stdin 关闭或管理器终止进程，避免 CI 因进程退出竞态把协议违规误判为 `plugin.internal_error`。
- 测试替身通过 `app.Options` 构造期注入；禁止为测试在 App 或服务上新增运行期 setter。
- 测试分层：包内单测验证包内行为；`server/tests/services` 验证服务装配与配置链路；`server/tests/integration` 验证跨包端到端流程；`server/tests/ws` 验证 WebSocket 事件契约；`server/tests/architecture` 验证包依赖边界。同一行为不跨层重复覆盖。
- 需要区分 race 构建时使用 `server/internal/testenv` 的 `RaceEnabled` 常量，不再复制 build tag stub。

## Cross-Surface Checks

- Web API 或共享 schema 改动时，同时检查 Web 与 Launcher 的生成类型是否需要更新。
- 修改 `server/internal/storage/migrations/*.sql` 或 `server/internal/sqlcqueries/*.sql` 时，运行 `sqlc generate`，提交 `server/internal/sqlcgen/` 生成结果，并用 `sqlc diff` 确认无漂移。
- 不在 `server/` 复制 contract 真相；实现消费它们，而不是再维护一套平行常量和文档。

## Consult Before Major Changes

- 当前服务端能力与入口：`server/README.md`
- 工程基线与默认命令：`docs/engineering/baseline.md`
- 正式 HTTP / WebSocket / errors / plugin contracts：`contracts/README.md`
- 架构、状态模型、事件模型：`docs/architecture/`
