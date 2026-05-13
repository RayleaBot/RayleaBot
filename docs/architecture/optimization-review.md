# 架构优化清单

本文档记录 RayleaBot 当前架构中已由仓库证据支撑的优化项。接口、字段、状态名、错误码与协议结构以 `contracts/` 为准；本清单只说明当前实现中的风险、改动边界和验证方式。

## 优先级

- **P1**：已确认存在行为偏差或高频维护成本，适合近期处理。
- **P2**：价值明确，但需要补足观测、契约或基础设施。
- **P3**：长期治理项，适合跟随相关模块改动收敛。

## 总览

| 编号 | 主题 | 优先级 | Contract-first | 当前证据 |
| --- | --- | --- | --- | --- |
| 1 | `server/internal/app` 组装边界收敛 | P1 | 否 | app 包当前 46 个非测试 Go 文件、71 个 Go 文件总量，`App` 仍聚合跨领域运行态 |
| 2 | `allowedTaskTypes` 与 `TaskType` 枚举对齐 | P1 | 否 | `TaskType` 包含 `recovery.confirm`，服务端过滤白名单缺少该值 |
| 3 | Chromium 渲染浏览器生命周期复用 | P1 | 部分 | 每次渲染都会创建 `NewExecAllocator` 与 `NewContext` |
| 4 | 运行时指标与时序观测面 | P2 | 是 | 正式诊断面以 `/readyz`、diagnostics 与日志为主，缺少时序指标出口 |
| 5 | 插件 webhook 防重放与幂等窗口 | P2 | 是 | webhook 当前校验 token / HMAC / source IP，缺少 timestamp 与 event id 去重语义 |
| 6 | Dispatcher 队列满行为可见化 | P2 | 是 | `OutcomeDropped` 只写结构化日志，管理面没有正式聚合面 |
| 7 | 重复事件穿透风险观测 | P2 | 部分 | Adapter 已有 `recentEventIDs` 去重，Bridge / Dispatcher 缺少重复事件统计 |
| 8 | Web WebSocket 重连退避 | P2 | 否 | Web 使用 `[500, 1000, 2000, 4000]` 固定延迟数组，缺少 jitter |
| 9 | Session 绝对 TTL 与签名密钥轮换 | P3 | 是 | 当前支持 TTL、滑动续期、最大会话数和持久化签名密钥 |
| 10 | 插件子系统字段收敛 | P3 | 否 | 插件子系统已拆包，`App` 仍直接持有多个插件侧服务与仓库 |
| 11 | `dead_letter` 状态恢复入口 | P3 | 否 | runtime 支持 crash backoff 与 `dead_letter`，恢复入口依赖人工管理动作 |
| 12 | 插件 `bot_id` 身份不可用语义 | P3 | 部分 | `init.bot`、`bot.identity.changed` 与 SDK 更新逻辑已存在，出站降级口径仍需明确 |

## 1. `server/internal/app` 组装边界收敛（P1）

**现状证据**

- `server/internal/app` 当前包含 46 个非测试 Go 文件、71 个 Go 文件总量。
- `App` 持有 storage、auth、tasks、adapter、dispatcher、render、local action、plugin lifecycle、plugin webhook、system handler 等跨领域运行态。
- `app_build_http.go`、`app_build_platform.go`、`app_build_plugins.go`、`app_services.go` 已经把部分组装逻辑分段，但 `App` 仍是跨 swimlane 的依赖承接点。

**风险**

- 新增子系统容易同时牵动 `App` 字段、构造链、HTTP / WebSocket handler 和测试 helper。
- `app_test_helpers_test.go` 聚合大量 fake 注入入口，局部测试改动容易扩大编译与理解范围。

**建议动作**

- 把 App 运行态、HTTP / WebSocket handler 依赖和插件子系统组装边界进一步分层。
- 优先抽出按领域构造的 handler deps，使 handler 只依赖当前领域接口。
- 保持 `app.New(...)` 与 `cmd/raylea-server/main.go` 入口稳定，避免对外启动路径变化。

**契约边界**

- 纯 server 内部重构，不改 `contracts/`。

**验证方式**

- `cd server && go test ./internal/app ./internal/plugins ./internal/runtime ./internal/dispatch`
- 重点覆盖 setup/session、tasks、plugins、protocol、render、recovery、WebSocket handler。

## 2. `allowedTaskTypes` 与 `TaskType` 枚举对齐（P1）

**现状证据**

- `contracts/web-api.openapi.yaml` 的 `TaskType` 枚举包含 `recovery.confirm`。
- `server/internal/app/tasks_http.go` 的 `allowedTaskTypes` 未包含 `recovery.confirm`。
- `GET /api/tasks?task_type=recovery.confirm` 会命中过滤白名单并返回 `errors.platform.invalid_request`。

**风险**

- OpenAPI、Web/Launcher generated types 和服务端查询行为不一致。
- 用户无法通过正式任务列表接口筛选 `recovery.confirm` 任务。

**建议动作**

- 给 `allowedTaskTypes` 补齐 `recovery.confirm`。
- 添加守卫测试，断言服务端允许的 task type 与 OpenAPI `TaskType` 枚举一致。
- 可在后续重构中把 task type 白名单收敛到单一 server 常量源。

**契约边界**

- 当前 contract 已包含该类型；本项只修服务端实现与测试。

**验证方式**

- `cd server && go test ./internal/app`
- 增加 `GET /api/tasks?task_type=recovery.confirm` 的回归断言。

## 3. Chromium 渲染浏览器生命周期复用（P1）

**现状证据**

- `server/internal/render/chromium_runner.go` 的 `Render` 每次调用都会执行 `chromedp.NewExecAllocator` 与 `chromedp.NewContext`。
- `render.preview`、`render.image` 和使用模板渲染的内部回复都会经过同一 Runner。
- No measurements found：仓库内未发现正式 benchmark、trace、profile 或 flamegraph，可量化延迟结论不能直接写入。

**风险**

- 每次渲染都创建浏览器执行上下文，存在冷启动和资源抖动风险。
- 高并发渲染下，进程创建成本和失败恢复路径更难观测。

**建议动作**

- 在不新增配置项的版本中，先评估内部复用 `ExecAllocator` 的可行性。
- 若需要 `render.browser_pool_max_tabs`、`render.browser_idle_timeout_seconds` 等配置，必须先更新 `contracts/config.user.schema.json`、fixtures、生成类型和文档。
- 增加小基准或 trace，记录模板预览、插件 `render.image`、失败重试三类路径的 P50/P95。

**契约边界**

- 无新增配置项时是 render 包内部优化。
- 新增配置项属于 contract-first。

**验证方式**

- `cd server && go test ./internal/render ./internal/app`
- 补充 `templateResourceDigest` 或 render Runner 层 benchmark，记录优化前后的命令和结果。

## 4. 运行时指标与时序观测面（P2）

**现状证据**

- 正式诊断入口包括 `/healthz`、`/readyz`、`/api/system/diagnostics/export`、结构化日志和恢复摘要。
- `server/go.mod` 当前没有 Prometheus 或 OpenTelemetry server 依赖。
- 当前缺少正式 `/metrics` 或等价时序指标接口。

**风险**

- 事件丢弃、插件崩溃、调度延迟、渲染排队、SQLite 写入压力只能从日志和快照中间接判断。
- 性能和稳定性问题缺少统一时间序列证据。

**建议动作**

- 定义最小指标面，覆盖事件主链、插件 runtime、任务执行、render 队列、outbound 发送。
- 新增依赖前同步更新工程基线、server 依赖锁定和质量门禁说明。
- `/metrics` 若作为正式管理接口暴露，先进入 `contracts/web-api.openapi.yaml`；若只作为本机调试端点，也要在 `docs/dev/diagnostics.md` 固定访问边界。

**契约边界**

- 新 HTTP 端点属于 contract-first。
- 新依赖需要同步 `docs/engineering/baseline.md` 和 `server/go.mod` / `server/go.sum`。

**验证方式**

- contract fixtures 覆盖 `/metrics` 或诊断入口说明。
- `cd server && go test ./...`
- 指标名称、label 基数和访问权限需要单测或集成测试覆盖。

## 5. 插件 webhook 防重放与幂等窗口（P2）

**现状证据**

- `server/internal/pluginwebhook.Service` 当前校验 route、method、插件状态、source IP、fixed token 或 HMAC。
- `webhook.received` 的 `EventID` 当前由 route 和 `time.Now().UnixNano()` 生成。
- webhook 请求协议中没有正式 timestamp、event id 或 replay tolerance 字段。

**风险**

- 合法签名请求可以被重复发送并重复投递给插件。
- 插件需要自行处理幂等，平台没有统一拒绝语义。

**建议动作**

- 在 plugin webhook contract 中冻结 timestamp、event id、签名覆盖范围和容忍窗口。
- 服务端按 `(plugin_id, route, event_id)` 做短期去重。
- 对过期 timestamp 和重复 event id 使用正式错误码或现有权限错误语义，并补 fixtures。

**契约边界**

- timestamp、event id、错误语义和 HMAC 规范都属于 contract-first。

**验证方式**

- contract fixtures 覆盖正常、过期、重复、签名不匹配。
- `cd server && go test ./internal/pluginwebhook ./internal/app`

## 6. Dispatcher 队列满行为可见化（P2）

**现状证据**

- `dispatch.OutcomeDropped` 在队列满时返回，并写入结构化日志。
- 管理 WebSocket `events.received` 当前没有 dispatcher drop 分支。
- 管理面没有按插件和原因聚合的队列满统计。

**风险**

- 用户只能从日志判断消息未处理原因。
- 插件队列容量和并发度调参缺少可见反馈。

**建议动作**

- 在 Dispatcher 内维护窗口化 drop 统计，按 plugin、reason、event_type 聚合。
- 管理面展示前，先在 `contracts/websocket-events.yaml` 增加正式 payload 分支。
- 若 `/metrics` 已存在，同步导出 drop counter；若未存在，保持 WebSocket / HTTP 管理面聚合即可。

**契约边界**

- 新 WebSocket payload 分支属于 contract-first。
- 指标导出依赖第 4 项的指标面决策。

**验证方式**

- `cd server && go test ./internal/dispatch ./internal/app`
- WebSocket contract 生成物和 Web socket-router 测试同步更新。

## 7. 重复事件穿透风险观测（P2）

**现状证据**

- Adapter `Shell` 使用 `recentEventIDs` 和 `isDuplicateEvent` 处理近期重复事件。
- Bridge 校验事件形状并输出 bridge runtime 观测摘要。
- 当前没有证据证明重复事件会穿透到 Dispatcher，也没有跨层重复事件统计。

**风险**

- 多 transport 或异常 OneBot 实现下，重复事件是否被完全拦截缺少可量化证据。
- Echo 缺失、重复事件、ignored 事件之间缺少统一聚合视图。

**建议动作**

- 不直接增加 Bridge / Dispatcher 二级去重。
- 先补观测：记录 Adapter duplicate drop、Bridge ignored、Dispatcher delivered/dropped 的聚合计数。
- 只有在观测显示重复事件穿透时，再设计 Bridge / Dispatcher 层的去重策略。

**契约边界**

- 仅补内部日志或 diagnostics 摘要时不需要 contract。
- 新增管理 WebSocket 分支或插件协议语义说明时需要 contract-first。

**验证方式**

- Adapter duplicate 测试覆盖多 transport 重复上报。
- Bridge / Dispatcher 测试确认观测摘要不会改变插件投递语义。

## 8. Web WebSocket 重连退避（P2）

**现状证据**

- `web/src/lib/ws.ts` 使用固定延迟数组 `[500, 1000, 2000, 4000]`。
- 超过数组长度后保持 4 秒重连间隔。
- 当前没有随机抖动，多个频道可能同时重连。

**风险**

- 本机服务恢复时，多条管理 WebSocket 容易形成同步重连脉冲。
- 用户只能看到连接状态，缺少断线累计时长等更细反馈。

**建议动作**

- 改为指数退避加随机抖动，例如 `base=500ms`、`cap=30s`、`jitter=25%`。
- 每条频道独立计算退避，避免同步重连。
- UI 可展示重连中和最后错误时间；不承诺断线期间的事件补回，除非对应 HTTP 查询面已正式存在。

**契约边界**

- Web 内部行为，不改 contract。

**验证方式**

- `cd web && pnpm test -- ws`
- 单测覆盖退避上限、jitter 范围、认证失败后停止重连。

## 9. Session 绝对 TTL 与签名密钥轮换（P3）

**现状证据**

- `auth.Config` 支持 `SessionTTLDays`、`SlidingRenewal`、`MaxSessions`。
- session signing key 通过 secret store 持久化。
- 当前配置 schema 没有 absolute TTL 字段，CLI contract 没有签名密钥轮换命令。

**风险**

- 开启滑动续期时，长期活跃会话缺少绝对生命周期边界。
- 签名密钥轮换缺少正式操作入口和兼容窗口语义。

**建议动作**

- 在 `contracts/config.user.schema.json` 增加 absolute TTL 字段并定义默认值。
- 在 `contracts/cli-commands.yaml` 增加签名密钥轮换命令。
- 服务端支持新旧 key 兼容窗口，新签发 token 使用新 key。

**契约边界**

- 跨 config、CLI、server、fixtures、docs 的 contract-first 改动。

**验证方式**

- config schema fixtures 覆盖默认值、非法值、滑动续期组合。
- CLI 与 auth manager 测试覆盖 key 轮换、旧 token 兼容、窗口结束后失效。

## 10. 插件子系统字段收敛（P3）

**现状证据**

- 插件相关职责已经拆到 `plugins`、`pluginconfig`、`pluginfile`、`pluginkv`、`pluginhttp`、`pluginui`、`pluginwebhook`、`protocolcap` 等包。
- `App` 仍直接持有 installer、uninstaller、repository、config、file、KV、webhook、local action、runtime registry 等插件侧对象。

**风险**

- 插件能力新增时容易继续扩大 `App` 字段面。
- 插件侧关闭顺序、测试替换和只读访问入口分散。

**建议动作**

- 引入内部 `pluginStack` 或等价聚合对象，集中插件侧装配、关闭顺序和 handler deps。
- `App` 只保留聚合对象和少量跨域接口。
- 与第 1 项一起推进，避免两次触碰同一构造链。

**契约边界**

- 纯 server 内部重构，不改 `contracts/`。

**验证方式**

- `cd server && go test ./internal/app ./internal/plugins ./internal/pluginwebhook ./internal/localaction`

## 11. `dead_letter` 状态恢复入口（P3）

**现状证据**

- runtime 支持 `crashed`、`backoff`、`dead_letter`。
- 插件生命周期文档说明超过阈值后进入 `dead_letter` 并等待人工干预。
- 当前缺少自动恢复、清理策略和管理面可见摘要的统一入口。

**风险**

- 管理员需要从状态和日志判断是否重试、禁用或卸载。
- 长时间 `dead_letter` 插件缺少统一治理策略。

**建议动作**

- 增加管理面上的 `dead_letter` 摘要：持续时间、最近错误、建议动作。
- 提供一次性冷启动尝试入口，复用现有 enable/reload 权限模型。
- 自动清理策略只处理运行时连接和订阅，不删除插件配置、KV 或文件数据。

**契约边界**

- 新管理动作或新状态摘要字段属于 contract-first。
- 仅补文档或内部日志不需要 contract。

**验证方式**

- runtime crash/backoff/dead_letter 测试覆盖状态流转。
- Web plugin detail 测试覆盖 `dead_letter` 摘要展示。

## 12. 插件 `bot_id` 身份不可用语义（P3）

**现状证据**

- `contracts/plugin-protocol.schema.json` 已冻结 `init.bot` 与 `bot.identity.changed`。
- Python / Node.js SDK 会在收到 `bot.identity.changed` 后更新 `bot_id` / `botId`。
- 协议身份不可用时 SDK 返回空字符串。

**风险**

- 插件作者对身份不可用期间能否发送消息、是否等待身份、如何处理重连缺少统一口径。
- SDK 没有显式等待 helper，插件可能自行 busy-wait 或实现不一致。

**建议动作**

- 在 plugin protocol 文档中定义身份不可用期间的出站动作语义。
- SDK 增加 `awaitBotIdentity(timeoutMs)` / `await_bot_identity(timeout_seconds)` helper。
- 插件示例展示订阅 `bot.identity.changed` 后刷新本地会话上下文。

**契约边界**

- 协议语义说明和 SDK helper 属于 contract / SDK / docs 联动改动。

**验证方式**

- plugin protocol fixture 覆盖 `bot.identity.changed`。
- Python / Node.js SDK 测试覆盖 init 无 bot、身份变更、等待超时。

## 长期跟踪

- **多实例 / 高可用**：当前仍是单实例自托管模型；引入外部队列前需要冻结部署和状态一致性边界。
- **插件市场与远程分发**：可信来源、签名和远程索引属于独立 contract 面。
- **强沙盒**：当前通过 runtime 进程隔离与 Local Action Service 做能力约束；进程级强沙盒需要扩展 capability scope。
- **非 OneBot 多协议**：adapter 结构可扩展；新增协议前需要冻结 protocol id、事件命名空间和兼容矩阵。
