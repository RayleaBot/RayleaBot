# Server Agent Guide

先遵守根 `AGENTS.md`，本文件补充服务端约束。

## State and Input

- Server 是正式业务状态源；Web、Launcher 和插件运行时不能成为同一业务状态的第二来源。
- 用户配置、plugin manifest 等外部输入必须通过对应 schema 校验。
- Cookie / CK 等敏感凭据只写入 secret store，不写入配置文件、日志或管理响应。

## Architecture

- Go 目录就是包边界；同一生命周期、调用路径且无独立复用价值的 helper 优先留在同一包，以文件区分职责。
- 新包应减少耦合或形成真实复用边界，不为未来能力预埋抽象，也不把领域内部细节拆成大量薄包。
- 开发工具使用仓库脚本或独立工具边界，不通过 `server/go.mod` 的 `tool` 指令引入与 Server 运行无关的大型依赖图。
- 领域视图在对应领域包构建；`management` 层负责 HTTP/WS 序列化，不复制领域逻辑。
- 语义分支依赖稳定的 code 或枚举，不比对用户可见文案。

## Concurrency and Assembly

- 配置热更新的读-改-写必须串行化；策略派生对象整体替换，避免热更新写路径与事件读路径共享未保护字段。
- 订阅与广播复用 `server/internal/platform/pubsub` 的 Hub。
- 必选依赖缺失在装配层返回 error；不在业务方法中通过接收者 nil 检查静默降级或假成功。
- 可选协作者装配到接口字段前检查具体指针，避免 typed nil 绕过空值判断。

## Testing and Generation

- 测试替身通过构造期注入，不为测试给 App 或服务新增运行期 setter。
- 包内单测验证包内行为；装配、跨包流程、WebSocket 与包依赖测试沿用 `server/tests/` 的对应目录，共享替身放 `server/tests/testutil`。按风险分层，避免重复覆盖。
- 修改 `server/internal/storage/schema.sql` 或 `server/internal/sqlcqueries/*.sql` 时，运行 `sqlc generate`，提交 `server/internal/sqlcgen/` 生成结果，并用 `sqlc diff` 确认无漂移。
