# 架构优化清单

本清单基于 RayleaBot 当前仓库的实际代码与契约，记录已经被仓库证据支撑的优化方向。接口、字段、状态名、错误码与协议结构以 `contracts/` 为准；本清单只描述实现侧风险、改动边界与验证方式。

## 优先级

- **P1**：已确认存在行为偏差或高频维护成本，建议近期处理。
- **P2**：价值明确，但需要先补观测、契约或基础设施。
- **P3**：长期治理项，建议跟随相关模块改动滚动收敛。

## 总览

| 编号 | 主题 | 优先级 | Contract-first | 状态 | 当前证据 |
| --- | --- | --- | --- | --- | --- |
| 1 | `server/internal/app` 组装边界收敛 | P1 | 否 | 无需改动 | 现有按领域 deps struct 已覆盖建议动作；`App` 字段聚合面与项 10 同源，落地以项 10 为准 |
| 2 | `allowedTaskTypes` 与 `TaskType` 枚举对齐 | P1 | 否 | 完成 | `recovery.confirm` 已进入服务端过滤白名单；服务端白名单与 OpenAPI `TaskType` 枚举有守卫测试 |
| 3 | Chromium 渲染浏览器生命周期复用 | P1 | 部分 | 完成 | `chromiumRunner` 复用长驻浏览器上下文，单次渲染创建独立 tab；`Service.Close` 与 `RefreshBrowserPath` 会释放默认 runner |
| 4 | 运行时指标与时序观测面 | P2 | 是 | 部分完成 | `server/internal/metrics` 注册 Prometheus 指标族；`GET /api/system/metrics` 走 admin session 鉴权；链路侧 instrumentation 仍逐步铺设 |
| 5 | 插件 webhook 防重放与幂等窗口 | P2 | 是 | 完成 | `pluginwebhook.Service` 强制 `replay_protection`，对客户端 timestamp 与 event_id 做 LRU 去重；HMAC 输入串含 timestamp/event_id/body；新错误码 `plugin.webhook_replay_rejected`/`plugin.webhook_timestamp_skew`；Python/Node SDK 同步 |
| 6 | Dispatcher 队列满行为可见化 | P2 | 是 | 完成 | dispatcher 维护 per-reason 窗口统计，10s 周期通过 bridge subscriber 推 `dispatcher_runtime` 帧；公开 `Stats()` / `FlushDispatcherWindow` 接入 metrics 与测试 |
| 7 | 跨层重复 / 丢弃事件统计 | P2 | 部分 | 完成 | bridge `ObservabilityData` 扩展 `adapter_dedup_drops_total`/`bridge_ignored_total`/`dispatcher_*_total`；adapter 暴露 `DedupDropsSnapshot`；dispatcher 暴露 `Stats` 并由 bridge 拉取 |
| 8 | Web WebSocket 重连退避 | P2 | 否 | 完成 | `web/src/lib/ws.ts` 走指数退避 + 抖动；`socket-controller` 每频道独立；`ConnectionStatusStrip` 展示重连倒计时与最后错误时间 |
| 9 | Session 绝对 TTL 与签名密钥轮换 | P3 | 是 | 待处理 | `auth.Config` 仅含 `SessionTTLDays` / `SlidingRenewal` / `MaxSessions`；签名密钥常驻 secret store |
| 10 | 插件子系统字段收敛 | P3 | 否 | 待处理 | `App` 仍直接持有 14 个插件侧字段（installer / uninstaller / repo / config / files / KV / grants / 黑白名单 / webhook / runtime registry 等） |
| 11 | `dead_letter` 状态恢复入口 | P3 | 部分 | 待处理 | runtime 流转 `crashed → backoff → dead_letter` 已实现，无自动清理与冷启动尝试入口 |
| 12 | 插件 `bot_id` 身份不可用语义 | P3 | 部分 | 待处理 | `init.bot`、`bot.identity.changed` 与 SDK 更新链已存在，未冻结身份不可用期间的出站降级口径 |

## 1. `server/internal/app` 组装边界收敛（P1）

**完成情况**

- 状态：无需改动。
- `server/internal/app/app_services.go` 已经按领域定义 `authHTTPDeps`、`configHTTPDeps`、`managementHTTPDeps`、`eventIngressDeps`、`pluginLifecycleDeps`、`systemServiceDeps`，每个 deps 只持有该领域真实使用的字段。
- handler 不直接接收 `*App`：`taskHTTPHandlers`、`authHTTPHandlers`、`configHTTPHandlers`、`managementHTTPHandlers` 等仅持有领域内的依赖。
- `app_test_helpers_test.go` 的 `setTestEventIngress`、`setTestLifecycle`、`setTestLocalActions`、`setTestSystem`、`setTestWebhookService` 已按领域装配，单领域测试改动不需要联动其他领域。
- 剩余的 `App` 47 字段聚合面与第 10 项（插件子系统字段收敛）同源；脱离 `pluginStack` 聚合的进一步收敛会沦为命名重排，无可证明收益。

**契约边界**

- 纯 server 内部状态，无 contract。

**验证方式**

- `cd server && go test ./internal/app ./internal/plugins ./internal/runtime ./internal/dispatch`

## 2. `allowedTaskTypes` 与 `TaskType` 枚举对齐（P1）

**完成情况**

- 状态：完成。
- `server/internal/app/tasks_http.go` 的 `allowedTaskTypes` 包含 `recovery.confirm`。
- `server/internal/app/tasks_http_test.go` 覆盖 `GET /api/tasks?task_type=recovery.confirm`。
- `server/internal/app/tasks_http_test.go` 校验服务端允许的 task type 与 OpenAPI `TaskType` 枚举一致。

**契约边界**

- 当前 contract 已包含该类型，本项只涉及服务端实现与测试。

**验证方式**

- `cd server && go test ./internal/app`

## 3. Chromium 渲染浏览器生命周期复用（P1）

**完成情况**

- 状态：完成。
- `server/internal/render/chromium_runner.go` 复用长驻 `ExecAllocator` 与浏览器根 context。
- 单次 `Render` 创建独立 `chromedp.NewContext` 作为 tab，渲染结束后关闭 tab。
- `Service.Close` 会关闭可释放的 runner。
- `RefreshBrowserPath` 替换默认 Chromium runner 时等待当前 worker 结束，并关闭旧 runner。
- 渲染失败会重置浏览器上下文，后续渲染可重新初始化。

**契约边界**

- 内部复用：render 包内部优化，无 contract。
- 暴露配置：先动 `contracts/config.user.schema.json` 与生成类型。

**验证方式**

- `cd server && go test ./internal/render ./internal/app`

## 4. 运行时指标与时序观测面（P2）

**现状证据**

- 正式诊断入口为 `/healthz`、`/readyz`、`/api/system/diagnostics/export`、结构化日志和 `recovery_summary`。
- `server/go.mod` 没有 Prometheus / OpenTelemetry 依赖。
- 当前没有 `/metrics` 或等价时序指标接口。

**风险**

- 事件丢弃、插件崩溃、调度延迟、render 排队、SQLite 写入压力只能从日志和快照间接判断。
- 性能与稳定性问题缺少时间序列证据，难以做容量与回归比较。

**建议动作**

- 定义最小指标面：事件主链阶段计数、插件 runtime 状态分布、任务执行延迟、render 队列深度与等待 / 渲染时长、outbound 发送延迟与失败计数。
- 引入新依赖前同步更新 `docs/engineering/baseline.md`、`server/go.mod` / `server/go.sum`。
- `/metrics` 若作为正式管理接口，先进入 `contracts/web-api.openapi.yaml`；若只在本机暴露，固定访问边界写入 `docs/dev/diagnostics.md`。

**契约边界**

- 新增 HTTP 端点属于 contract-first。
- 新增依赖需要同步基线与 server 锁文件。

**验证方式**

- contract fixtures 覆盖 `/metrics` 或诊断入口说明。
- `cd server && go test ./...`
- 指标名、label 基数、访问权限通过单测或集成测试覆盖。

## 5. 插件 webhook 防重放与幂等窗口（P2）

**现状证据**

- `server/internal/pluginwebhook.Service` 当前校验 route、HTTP method、插件状态、`SourceIPs` 白名单、`fixed_token` 或 `hmac_sha256` 签名。
- 投递时 `EventID = "webhook-<route>-<UnixNano>"`，即每次重新生成。
- webhook 入站没有正式 timestamp、event id 或容忍窗口字段；`Adapter.recentEventIDs` 不覆盖 webhook 路径。

**风险**

- 合法签名请求可被重复发送并重复触发 `webhook.received`。
- 插件需要自行处理幂等，平台没有统一拒绝语义。

**建议动作**

- 在 plugin webhook contract 中冻结 timestamp、event id、签名覆盖范围与 replay tolerance。
- 服务端按 `(plugin_id, route, event_id)` 做短期 LRU 去重；过期 timestamp 与重复 event id 走 `errors.permission.denied` 或新增正式错误码。
- fixtures 同步覆盖正常、过期、重复、签名不匹配。

**契约边界**

- timestamp、event id、错误语义和 HMAC 覆盖范围属于 contract-first。

**验证方式**

- `cd server && go test ./internal/pluginwebhook ./internal/app`
- contract fixtures 覆盖三类异常路径。

## 6. Dispatcher 队列满行为可见化（P2）

**现状证据**

- `dispatch.OutcomeDropped` 在 per-plugin 队列满时返回，只写结构化日志。
- Dispatcher 默认 `queueSize=16`，由 `New(logger, sender, resolver, queueSize)` 入口决定。
- 管理 WebSocket `events.received` 当前没有 dispatcher drop 分支；管理面没有按 plugin / 原因 / event_type 的聚合视图。

**风险**

- 用户只能从日志判断消息未处理原因。
- 插件队列容量与并发度调参缺少可见反馈。

**建议动作**

- 在 Dispatcher 内维护窗口化 drop 统计（按 plugin / reason / event_type），并通过现有 `events.received` 推送聚合摘要。
- 管理面展示前先在 `contracts/websocket-events.yaml` 增加正式 payload 分支。
- 第 4 项落地后同步导出对应 `_total` counter；未落地前先用 WebSocket 聚合或 `/api/system/diagnostics/export` 暴露。

**契约边界**

- 新 WebSocket payload 分支属于 contract-first。

**验证方式**

- `cd server && go test ./internal/dispatch ./internal/app`
- WebSocket contract 生成物与 Web socket-router 测试同步更新。

## 7. 跨层重复 / 丢弃事件统计（P2）

**现状证据**

- Adapter 通过 `recentEventIDs` + `isDuplicateEvent` 在 `recentEventDedupRetention=2m` 内去重，但没有对外暴露 dedup 计数。
- Bridge 已实现 `ObservabilityData`（`observability_scope=bridge_runtime`、`delivered_count` / `result_count` / `error_count`），通过 `events.received` 暴露聚合摘要。
- Dispatcher 的 `OutcomeIgnored` / `OutcomeDropped` 与 Adapter 的 dedup 不在同一份聚合面里。

**风险**

- 多 transport 或异常 OneBot 实现下，重复事件是否被完全拦截缺少端到端证据。
- echo 缺失、duplicate drop、ignored、dropped 之间缺少统一聚合视图。

**建议动作**

- 不直接增加 Bridge / Dispatcher 二级去重。
- 在现有 `bridge_runtime` 摘要基础上拓宽口径：补 Adapter dedup drop、Bridge ignored、Dispatcher delivered / dropped 的聚合计数。
- 仅在观测显示重复事件穿透时再设计跨层去重。

**契约边界**

- 仅补内部日志或 diagnostics 摘要时不需要 contract。
- 拓宽 `events.received` payload 字段属于 contract-first。

**验证方式**

- Adapter dedup 测试覆盖多 transport 重复上报。
- Bridge / Dispatcher 测试确认观测摘要不影响插件投递语义。

## 8. Web WebSocket 重连退避（P2）

**现状证据**

- `web/src/lib/ws.ts` 使用固定数组 `[500, 1000, 2000, 4000]`，`reconnectDelays[Math.min(attempts-1, 3)]`。
- 超过数组长度后保持 4 秒固定间隔，没有 jitter。
- 多条管理 WebSocket（`events`、`tasks`、`logs`、`plugin_console`）在断线恢复时容易同步重连。

**风险**

- 服务端短暂抖动恢复时，多频道同步重连形成脉冲。
- 用户只能看到当前连接状态，缺少断线累计时长 / 最近错误等更细反馈。

**建议动作**

- 改为指数退避 + 随机抖动，例如 `base=500ms`、`cap=30s`、`jitter=±25%`。
- 每条频道独立计算退避，避免同步重连。
- UI 展示重连中状态与最后错误时间；不承诺断线期间的事件补回，除非对应 HTTP 查询面已正式存在。

**契约边界**

- Web 内部行为，不改 contract。

**验证方式**

- `cd web && pnpm test -- ws`
- 单测覆盖退避上限、jitter 范围、`session_expired` 后停止重连。

## 9. Session 绝对 TTL 与签名密钥轮换（P3）

**现状证据**

- `auth.Config` 只暴露 `SessionTTLDays`、`SlidingRenewal`、`MaxSessions`。
- session signing key 通过 secret store 持久化，但没有正式轮换接口。
- `contracts/config.user.schema.json` 当前没有 absolute TTL；`contracts/cli-commands.yaml` 当前没有签名密钥轮换命令。

**风险**

- 开启滑动续期时，长期活跃会话缺少绝对生命周期边界。
- 签名密钥轮换缺少正式入口与新旧 key 兼容窗口。

**建议动作**

- 在 `contracts/config.user.schema.json` 增加 `admin.session_absolute_ttl_days` 与默认值。
- 在 `contracts/cli-commands.yaml` 增加签名密钥轮换命令；服务端支持新旧 key 兼容窗口，新签发使用新 key。
- fixtures、SDK、docs 同轮更新。

**契约边界**

- 跨 config、CLI、server、fixtures、docs 的 contract-first 改动。

**验证方式**

- config schema fixtures 覆盖默认值、非法值、滑动续期组合。
- CLI 与 auth manager 测试覆盖 key 轮换、旧 token 兼容、窗口结束后失效。

## 10. 插件子系统字段收敛（P3）

**现状证据**

- 插件相关包已拆为 `plugins`、`pluginconfig`、`pluginfile`、`pluginhttp`、`pluginkv`、`pluginui`、`pluginwebhook`，OneBot provider 扩展能力注册在 `protocolcap`。
- `App` 结构体在插件相关字段上仍保留 14 个直接持有项：`pluginInstaller`、`pluginUninstaller`、`pluginRepository`、`pluginConfig`、`pluginFiles`、`pluginKV`、`grantRepository`、`blacklistRepo`、`whitelistRepo`、`whitelistState`、`webhookRegistry`、`pluginLogLimiter`、`outboundLimiter`、`pluginLifecycle`。
- 插件侧 handler、关闭顺序、测试替换通过 `App` 字段直接访问。

**风险**

- 插件能力新增时容易继续扩大 `App` 字段面。
- 插件侧关闭顺序与只读访问入口分散。

**建议动作**

- 引入内部 `pluginStack`（或等价聚合对象）集中插件侧装配、关闭顺序与 handler deps。
- `App` 只保留聚合对象与少量跨域接口。
- 与第 1 项一同推进，避免两次触碰同一构造链。

**契约边界**

- 纯 server 内部重构，不改 `contracts/`。

**验证方式**

- `cd server && go test ./internal/app ./internal/plugins ./internal/pluginwebhook ./internal/localaction`

## 11. `dead_letter` 状态恢复入口（P3）

**现状证据**

- `runtime` 状态机覆盖 `stopped / starting / running / stopping / crashed / backoff / dead_letter`；`docs/architecture/state-model.md` 已记录流转。
- 进入 `dead_letter` 后只能由人工触发 enable / reload，没有自动清理与冷启动尝试入口。
- 管理面对 `dead_letter` 的展示停留在状态枚举，没有"持续时间 / 最近错误 / 建议动作"的统一摘要。

**风险**

- 长时间 `dead_letter` 插件可能仍持有运行时连接、订阅、调度 job。
- 用户需要自行翻日志判断重试 / 禁用 / 卸载。

**建议动作**

- 管理面展示 `dead_letter` 摘要：持续时间、最近错误、建议动作。
- 提供一次性冷启动尝试入口，复用 enable / reload 权限模型。
- 自动清理只处理运行时连接和订阅，不删除插件配置、KV 或文件数据。

**契约边界**

- 新管理动作或新摘要字段属于 contract-first。
- 仅补内部日志或文档不需要 contract。

**验证方式**

- runtime crash / backoff / dead_letter 测试覆盖状态流转。
- Web plugin detail 测试覆盖 `dead_letter` 摘要展示。

## 12. 插件 `bot_id` 身份不可用语义（P3）

**现状证据**

- `contracts/plugin-protocol.schema.json` 已冻结 `init.bot` 与 `bot.identity.changed`。
- Python / Node.js SDK 在 `bot.identity.changed` 时更新 `bot_id` / `botId`；身份不可用时返回空字符串。
- 协议与 SDK 没有冻结身份不可用期间的出站降级口径与显式 `awaitBotIdentity` helper。

**风险**

- 插件作者对身份不可用期间能否发送消息、是否等待身份、如何处理重连缺少统一口径。
- 不同插件可能自行 busy-wait 或实现不一致。

**建议动作**

- 在 plugin protocol 文档与 SDK README 中冻结身份不可用期间的出站语义。
- SDK 增加 `awaitBotIdentity(timeoutMs)` / `await_bot_identity(timeout_seconds)` helper。
- 插件示例展示订阅 `bot.identity.changed` 后刷新本地会话上下文。

**契约边界**

- 协议语义说明与 SDK helper 属于 contract / SDK / docs 联动改动。

**验证方式**

- plugin protocol fixture 覆盖 `bot.identity.changed`。
- Python / Node.js SDK 测试覆盖 init 无 bot、身份变更、等待超时。

## 长期跟踪

- **多实例 / 高可用**：当前为单实例自托管模型；引入外部队列前需要冻结部署与状态一致性边界。
- **插件市场与远程分发**：可信来源、签名与远程索引属于独立 contract 面。
- **强沙盒**：当前通过 runtime 进程隔离与 Local Action Service 做能力约束；进程级强沙盒需要扩展 capability scope。
- **非 OneBot 多协议**：adapter 结构可扩展；新增协议前需要冻结 protocol id、事件命名空间与兼容矩阵。
