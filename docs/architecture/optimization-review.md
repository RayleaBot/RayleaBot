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
| 4 | 运行时指标与时序观测面 | P2 | 是 | 完成 | `server/internal/metrics` 注册 Prometheus 指标族；`GET /api/system/metrics` 走 admin session 鉴权；adapter / bridge / dispatcher / runtime / task / render / outbound / webhook 主链已接入 |
| 5 | 插件 webhook 防重放与幂等窗口 | P2 | 是 | 完成 | `pluginwebhook.Service` 强制 `replay_protection`，对客户端 timestamp 与 event_id 做 LRU 去重；HMAC 输入串含 timestamp/event_id/body；新错误码 `plugin.webhook_replay_rejected`/`plugin.webhook_timestamp_skew`；Python/Node SDK 同步 |
| 6 | Dispatcher 队列满行为可见化 | P2 | 是 | 完成 | dispatcher 维护 per-reason 窗口统计，10s 周期通过 bridge subscriber 推 `dispatcher_runtime` 帧；公开 `Stats()` / `FlushDispatcherWindow` 接入 metrics 与测试 |
| 7 | 跨层重复 / 丢弃事件统计 | P2 | 部分 | 完成 | bridge `ObservabilityData` 扩展 `adapter_dedup_drops_total`/`bridge_ignored_total`/`dispatcher_*_total`；adapter 暴露 `DedupDropsSnapshot`；dispatcher 暴露 `Stats` 并由 bridge 拉取 |
| 8 | Web WebSocket 重连退避 | P2 | 否 | 完成 | `web/src/lib/ws.ts` 走指数退避 + 抖动；`socket-controller` 每频道独立；`ConnectionStatusStrip` 展示重连倒计时与最后错误时间 |
| 9 | Session 绝对 TTL 与签名密钥轮换 | P3 | 是 | 待处理 | `auth.Config` 仅含 `SessionTTLDays` / `SlidingRenewal` / `MaxSessions`；签名密钥常驻 secret store |
| 10 | 插件子系统字段收敛 | P3 | 否 | 无需改动 | 插件子系统已按 `plugin*` / `protocolcap` 包拆分，handler deps 走领域 struct；引入 `pluginStack` 聚合沦为命名重排，结论与项 1 同 |
| 11 | `dead_letter` 状态恢复入口 | P3 | 部分 | 部分完成 | runtime 进入 `dead_letter` 时记录 `EnteredDeadLetterAt`、自动清理插件 webhook 路由；管理面摘要字段与冷启动重试入口仍属 contract-first 待处理 |
| 12 | 插件 `bot_id` 身份不可用语义 | P3 | 部分 | 完成 | `init.bot` 缺省与空 `bot.identity.changed` 表示身份不可用；Python / Node.js SDK 提供等待身份就绪 helper |

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

**完成情况**

- 状态：完成。
- `docs/engineering/baseline.md` 固定 `github.com/prometheus/client_golang 1.23.2` 作为指标依赖。
- `contracts/web-api.openapi.yaml` 冻结 `GET /api/system/metrics`，响应为 Prometheus text exposition format，并受 admin session 保护。
- `server/internal/metrics` 预注册正式指标族，调用方只使用已声明的 metric handle。
- 指标面覆盖事件主链阶段计数、插件 runtime 状态分布、任务执行延迟、render 队列深度与渲染时长、outbound 发送计数与延迟、dispatcher drop、adapter dedup drop、bridge ignored、plugin webhook replay 观测。
- adapter / bridge / dispatcher 通过窄 observer 接口接入指标，不直接依赖 Prometheus client。
- 插件 runtime 状态 gauge 由 catalog 订阅和周期刷新共同维护，`App.Close` 会停止刷新 goroutine。

**契约边界**

- `GET /api/system/metrics` 属于正式管理 HTTP contract。
- 新增指标族需先在 `server/internal/metrics` 注册，并保持 label 基数受控。

**验证方式**

- `cd server && go test ./internal/metrics ./internal/app ./internal/bridge ./internal/dispatch`
- `cd server && go test ./...`

## 5. 插件 webhook 防重放与幂等窗口（P2）

**完成情况**

- 状态：完成。
- `contracts/plugin-protocol.schema.json` 冻结 `replay_protection` 配置（必填 `enforce`、可选 `timestamp_skew_seconds`、`event_id_header`、`timestamp_header`）。
- `webhook.received` 事件 payload 暴露 `client_timestamp` 与 `client_event_id`，两者只在 grace 模式下且客户端缺省时才省略。
- `server/internal/pluginwebhook.Service` 把 timestamp、event_id 与 body 串入 HMAC 签名输入，并按 `(plugin_id, route, event_id)` 做窗口化 LRU 去重。
- 新错误码：超出容忍窗口返回 `plugin.webhook_timestamp_skew`，重复 event_id 返回 `plugin.webhook_replay_rejected`。
- Python / Node.js SDK helper 同步生成正式 timestamp 与 event id 头。

**契约边界**

- `replay_protection` 配置、payload 字段、错误码与 HMAC 输入串属于 contract-first，已在 `contracts/plugin-protocol.schema.json` 与 `contracts/error-codes.yaml` 内冻结。

**验证方式**

- `cd server && go test ./internal/pluginwebhook ./internal/app`
- `cd sdk/python && python -m unittest discover -s tests`
- `cd sdk/nodejs && node --test tests/*.test.mjs`

## 6. Dispatcher 队列满行为可见化（P2）

**完成情况**

- 状态：完成。
- `dispatch.Dispatcher` 维护按 `(plugin_id, reason, event_type)` 聚合的窗口化 drop 统计，公开 `Stats()` 与 `FlushDispatcherWindow(windowSeconds)`。
- `StartObservabilityFlush` 由 bridge 启动定时器，10 秒周期把窗口快照通过 `DispatcherRuntimePublisher` 转发到管理 WebSocket 的正式 `dispatcher_runtime` 帧。
- `contracts/websocket-events.yaml` 冻结 `dispatcher_runtime` payload 分支；`server/internal/metrics` 同步导出 `raylea_dispatcher_outcome_total` 与 `raylea_dispatcher_window_drops_total`。
- 管理面（仪表盘 / 协议中心 / 插件详情）按帧消费聚合摘要。

**契约边界**

- 新 WebSocket payload 分支与 metrics 名称属于 contract-first，已落地。

**验证方式**

- `cd server && go test ./internal/dispatch ./internal/app`
- `cd web && pnpm test`

## 7. 跨层重复 / 丢弃事件统计（P2）

**完成情况**

- 状态：完成。
- Adapter 暴露 `DedupDropsSnapshot()`，把 `recentEventIDs` 命中拒绝的累计计数对外公开。
- Bridge `ObservabilityData` 新增 `adapter_dedup_drops_total`、`bridge_ignored_total`、`dispatcher_delivered_total`、`dispatcher_dropped_total` 维度，由 bridge 周期拉取上述统计并合入正式 `bridge_runtime` 摘要。
- Dispatcher 通过 `Stats()` 提供 cumulative outcome counts，bridge 与 metrics 模块共用。
- `contracts/websocket-events.yaml` 冻结上述字段；管理 WebSocket `events.received` 已能在同一帧内反映四类事件结果。

**契约边界**

- 拓宽后的 `events.received` payload 字段属于 contract-first，已落地。

**验证方式**

- `cd server && go test ./internal/adapter ./internal/bridge ./internal/dispatch ./internal/app`

## 8. Web WebSocket 重连退避（P2）

**完成情况**

- 状态：完成。
- `web/src/lib/ws.ts` 引入 `BackoffOptions`（默认 `baseMs=500`、`capMs=30_000`、`jitterRatio=0.25`）与 `computeBackoffMs`，按指数退避 + 抖动计算下一次重连延迟。
- 每个 `socket-controller` 实例独立维护 `reconnectAttempts` / `nextBackoffMs`，多个频道不会同步重连。
- `ConnectionStatusStrip.vue` 展示重连倒计时、最后错误时间与累计断线时长。
- 单测覆盖退避上限、jitter 范围、`session_expired` 后停止重连等路径。

**契约边界**

- Web 内部行为，无 contract 改动。

**验证方式**

- `cd web && pnpm test`

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

**完成情况**

- 状态：无需改动。
- 插件子系统已经按职责拆为 `plugins`、`pluginconfig`、`pluginfile`、`pluginhttp`、`pluginkv`、`pluginui`、`pluginwebhook`，OneBot provider 扩展能力注册在 `protocolcap`。
- handler 装配通过 `app_services.go` 中按领域定义的 deps struct（`pluginLifecycleDeps`、`systemServiceDeps`、`managementHTTPDeps` 等）完成；测试入口经 `setTestEventIngress` / `setTestLifecycle` 等领域 helper 替换。
- `App` 结构体保留对插件侧组件的直接引用是为了维持单一关闭顺序与 boot 阶段的明确依赖图；再引入 `pluginStack` 聚合不会减少字段总数，只会多一层间接命名，无可证明收益。
- 与第 1 项同源，结论一致：进一步拆分留给具体新能力 / 新依赖触发，不主动重排。

**契约边界**

- 纯 server 内部状态，无 contract。

**验证方式**

- `cd server && go test ./internal/app ./internal/plugins ./internal/pluginwebhook ./internal/localaction`

## 11. `dead_letter` 状态恢复入口（P3）

**当前进展**

- 状态：部分完成。
- `runtime.Manager` 在 `SetDeadLetterState` 时记录 `EnteredDeadLetterAt`，`SetStopped` 与 `ResetCrashCount` 都会清空该时间戳，确保字段反映当前 dwell time。
- `pluginLifecycleController.handleCrash` 进入 `dead_letter` 时除了 `dispatcher.Deregister` 与 `clearBotIdentity`，会同时 `webhooks.DeletePlugin`，避免插件已停止重启时 webhook 路由仍在受理外部请求。
- 单测 `TestHandleCrashDeadLetterCleansUpWebhooks` 与 `TestManagerSetDeadLetterState` 锁定上述行为。

**剩余工作**

- 管理面对 `dead_letter` 的展示停留在状态枚举，没有"持续时间 / 最近错误 / 建议动作"的统一摘要。
- 进入 `dead_letter` 后的冷启动尝试入口尚未存在，管理员只能通过 disable + enable 走完整生命周期。
- `EnteredDeadLetterAt`、`crash_count`、`last_error_code` 等字段尚未通过 `GET /api/plugins/{plugin_id}` 暴露给 Web。

**剩余建议动作**

- 在 `contracts/web-api.openapi.yaml` 的 `PluginDetailResponse` 内冻结 `dead_letter` 摘要对象（`entered_at`、`crash_count`、`last_error_code`、`last_error_message`）。
- 新增 `POST /api/plugins/{plugin_id}/dead_letter/recover` 受保护路由：仅在当前 `runtime_state=dead_letter` 时受理，否则 `409`；成功时重置 crash count 并按现有 enable / reload 路径冷启动。
- Web `PluginDetailView` 展示摘要与"恢复尝试"入口，复用 enable / reload 权限校验。

**契约边界**

- 自动清理与时间戳记录属于纯 server 内部行为，已落地无需 contract。
- 摘要字段、冷启动入口与错误码属于 contract-first，待后续版本统一冻结。

**验证方式**

- `cd server && go test ./internal/app ./internal/runtime`

## 12. 插件 `bot_id` 身份不可用语义（P3）

**完成情况**

- 状态：完成。
- `contracts/plugin-protocol.schema.json` 冻结 `init.bot` 与 `bot.identity.changed`。
- `docs/plugin/protocol.md` 说明身份不可用期间的出站语义：依赖 `self_id` 的 `message.*`、`reaction.set` 与 `onebot.*` action 返回正式 `error` 帧，不依赖身份的 local action 保持可用。
- Python SDK 在 `bot.identity.changed` 时更新或清空 `bot_id`，并提供 `RayleaBotPlugin.await_bot_identity(timeout_seconds=30)`。
- Node.js SDK 在 `bot.identity.changed` 时更新或清空 `botId`，并提供 `plugin.awaitBotIdentity(timeoutMs=30000)` 与 `EventContext.awaitBotIdentity(timeoutMs)`。
- SDK helper 在身份已知时立即返回；身份不可用时等待到身份恢复或超时，超时返回空字符串。
- Node.js SDK 的等待项在超时和身份提前恢复时都会释放，避免长期不可用时堆积等待闭包。

**契约边界**

- `init.bot` 与 `bot.identity.changed` 的字段结构由 `contracts/plugin-protocol.schema.json` 裁决。
- 身份不可用期间的出站语义由 `docs/plugin/protocol.md` 和正式错误码口径表达。

**验证方式**

- plugin protocol fixture 覆盖 `bot.identity.changed`。
- `cd sdk/python && python -m unittest discover -s tests`
- `cd sdk/nodejs && node --test tests/*.test.mjs`
- `cd sdk/nodejs && npm run typecheck`

## 长期跟踪

- **多实例 / 高可用**：当前为单实例自托管模型；引入外部队列前需要冻结部署与状态一致性边界。
- **插件市场与远程分发**：可信来源、签名与远程索引属于独立 contract 面。
- **强沙盒**：当前通过 runtime 进程隔离与 Local Action Service 做能力约束；进程级强沙盒需要扩展 capability scope。
- **非 OneBot 多协议**：adapter 结构可扩展；新增协议前需要冻结 protocol id、事件命名空间与兼容矩阵。
