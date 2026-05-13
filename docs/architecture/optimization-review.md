# 架构优化评审报告

本报告基于 RayleaBot 当前仓库实际落地情况（`server/internal/`、`web/src/`、`launcher/src/`、`contracts/`），针对架构设计的清晰度、可维护性、可演进性、性能与可观测性给出可执行的优化建议。每条建议都标注优先级、影响面、预估成本与是否需要先动 `contracts/`。

## 范围

评审对象为 `docs/architecture/` 中描述的平台架构、消息主链、状态模型、渲染服务与平台运行时，以及对应实现。接口边界的裁决仍以 `contracts/` 为准；本报告不定义新接口，只在需要时指向应该先进契约的方向。

## 优先级约定

- **P1**：收益明显且风险可控，建议近两个版本内排期。
- **P2**：长期值得做，需先搭好观测或基础设施。
- **P3**：有价值但不紧急，先留存跟踪。

## 评审结论总览

| 编号 | 主题 | 优先级 | 是否 contract-first | 当前状态摘要 |
| --- | --- | --- | --- | --- |
| 1 | `server/internal/app` 包组件化拆分 | P1 | 否 | 单包 174 个 Go 文件，`App` 结构体聚合了 30+ 子系统字段，阅读与修改成本偏高 |
| 2 | 修复 `allowedTaskTypes` 与契约枚举漂移 | P1 | 否 | `recovery.confirm` 不在服务端白名单内，但在 OpenAPI `TaskType` 枚举内 |
| 3 | Chromium 渲染进程池化 | P1 | 否 | 每次 `render.preview` / `render.image` 都新起一个 `chromedp` 浏览器实例 |
| 4 | 运行时可观测性：指标与分布式跟踪 | P1 | 否 | 零 Prometheus / OpenTelemetry 接入；排障只能靠结构化日志拼接 |
| 5 | 插件 webhook 防重放与幂等窗口 | P2 | 是 | 当前只做 token / HMAC 校验，没有时间戳窗口与请求 ID 去重 |
| 6 | Dispatcher 队列满行为的可观测性升级 | P2 | 是 | 丢弃事件仅写日志，管理面没有正式统计面 | 
| 7 | 事件去重与 echo 超时的二级回路 | P2 | 部分 | Adapter 层做 echo 等待，但 Bridge / Dispatcher 之间没有 `event_id` 级别的去重 |
| 8 | Web WebSocket 断线重连与状态一致性 | P2 | 否 | 定长重连延迟数组，没有指数退避与抖动；断线期间的服务端增量靠 HTTP 兜底 |
| 9 | Session 密钥轮换与绝对 TTL 边界 | P3 | 是 | 当前只有滑动续期与最大会话数，没有密钥轮换与绝对 TTL 上限 |
| 10 | 插件子系统中 `App` 字段二次收敛 | P3 | 否 | 已按职责拆了 `plugin*` 八个前缀包，但 `App` 仍直接持有 14 个插件侧字段 |
| 11 | Runtime 状态机长时间 `dead_letter` 回路 | P3 | 否 | 进入 `dead_letter` 后没有受控再尝试入口，除手动启用外无状态回流 |
| 12 | 插件 `bot_id` 生命周期与重连语义 | P3 | 部分 | `bot.identity.changed` 已有事件，但插件内的 `bot_id` 失效窗口没有统一文档化 |

---

## 1. `server/internal/app` 包组件化拆分（P1）

**现状**

- `server/internal/app` 单包 174 个 Go 文件、大量 `_http.go` / `_ws.go` / `lifecycle` / `service_status` 相关文件并排。
- `App` 结构体聚合了约 30+ 子系统字段（`storage`、`auth`、`tasks`、`plugins`、`adapter`、`bridge`、`dispatcher`、`renderer`、`localActions` 等），`New()` 入口承担配置校验、存储初始化、插件注册、HTTP 路由挂载、schedule 引擎启动等多线职责。
- 已按功能拆了 `app_build_http.go` / `app_build_platform.go` / `app_build_plugins.go` / `app_services.go`，但 `App` 仍然是跨 swimlane 的依赖承接点。

**问题**

- 新增子系统必须同时改 `App` 定义、`New()` 组装链、HTTP 挂载链三处，提高了跨领域改动的成本。
- 测试辅助 helpers 集中在 `app_test_helpers_test.go` 内，任何 fake 替换需要触发整包重新编译。
- 阅读门槛高：新贡献者进入 `app` 包需要同时理解组装顺序、接口形状与领域状态。

**建议**

- 拆出 `server/internal/appcore`（`App` 基础结构、`Options`、`Run` / `Close`、运行态字段）与 `server/internal/apphandlers`（按领域分的 HTTP / WS handlers）。
- 把 `eventIngressService`、`pluginLifecycleController`、`protocolService`、`systemService` 等"服务对象"正式提升为各自包的首类对象，并在 `appcore` 内只保留各自的接口契约。
- 把 handlers 的依赖改为构造器注入（`NewConfigHandlers(deps ConfigDeps)`），与 `App` 之间只交换接口，减少字段体。
- 维持 `app/*.go` 内对外 API 名字不变，避免影响 tests 与 `cmd/raylea-server/main.go`。

**影响面**：Server 内部大型重构，无对外契约改动。
**预估成本**：2-3 个版本周期，拆分可按子领域滚动进行。
**收益**：改动领域独立、测试加载面积缩小、代码导航成本显著下降。

## 2. 修复 `allowedTaskTypes` 与契约枚举漂移（P1）

**现状**

- `contracts/web-api.openapi.yaml` 中 `TaskType` 枚举包含 11 个类型，含 `recovery.confirm`。
- `server/internal/app/tasks_http.go` 的 `allowedTaskTypes` map 只登记了 10 个类型，遗漏 `recovery.confirm`。
- 管理面按 `task_type=recovery.confirm` 过滤 `GET /api/tasks` 时会被 `errors.platform.invalid_request` 拒绝。

**建议**

- 把 `allowedTaskTypes` 改为从 `tasks.KnownTaskTypes()` 等单一来源推导，避免再次漂移。
- 加一条回归测试：`openapi.TaskType.enum` 与 `allowedTaskTypes` 必须完全一致。
- 可进一步把任务类型上升为 `contracts/` 中正式的生成源，由 `web/src/types/generated.ts` 与 Server 共享。

**影响面**：一行枚举修复 + 一条守卫测试，近期能合并。
**预估成本**：0.5 天。

## 3. Chromium 渲染进程池化（P1）

**现状**

- `server/internal/render/chromium_runner.go#Render` 每次调用都执行 `chromedp.NewExecAllocator` + `chromedp.NewContext`，也就是"每次渲染启动一个浏览器进程"。
- 模板预览、`render.image` 与冷却提示等都会触发新进程冷启动；Chromium 冷启动在 Windows 上一般要 300-800ms。

**建议**

- 引入受控的浏览器生命周期管理：
  - 一个长驻 `ExecAllocator`（允许空闲超时回收），复用进程；
  - 在进程内按需新建 `NewContext`（tab）承担单次渲染，用完立即 `Cancel`；
  - 固定"单进程最大并发 tab 数"和"单 tab 最大活跃时长"两条守护线。
- 暴露正式配置项（例如 `render.browser_pool_max_tabs`、`render.browser_idle_timeout_seconds`），默认值保守即可。
- 在渲染失败或 tab 异常时回退到"销毁进程 + 重新拉起"策略，避免积累泄漏的页面。

**影响面**：`render` 包内部重构，`render.image` / `render.preview` 契约不变。
**预估成本**：3-5 天，包括并发保护、空闲回收与失败回退。
**收益**：单次渲染 P95 延迟显著下降，插件冷启动命中渲染时体验改善。

## 4. 运行时可观测性：指标与分布式跟踪（P1）

**现状**

- `server/` 目录没有引入 `prometheus`、`opentelemetry` 或任何 metrics 抽象。
- 事件丢弃、插件重启次数、调度任务延迟、渲染队列深度、SQLite 冷热路径延迟只能通过 `management_logs` 推断。
- 诊断导出包与 `readyz` 给出的是"状态快照"，没有时序视图。

**建议**

- 引入轻量 metrics 基座：`github.com/prometheus/client_golang`（当前冻结版本线允许）。
- 暴露正式 `/metrics` 端点（仅本机或经过 session token 的管理路径访问）。
- 优先铺设五组关键指标：
  1. `raylea_event_pipeline_stage_total{stage, outcome}`（Adapter / Ingress / Bridge / Dispatcher 各阶段的计数）
  2. `raylea_plugin_runtime_state_total{plugin, state}` 与 `raylea_plugin_dispatch_queue_depth{plugin}`
  3. `raylea_task_execution_seconds{task_type, status}` 直方图
  4. `raylea_render_queue_depth` 与 `raylea_render_latency_seconds`
  5. `raylea_outbound_send_latency_seconds{transport}` 与失败计数
- 指标引入前先更新 `contracts/web-api.openapi.yaml` 里对 `/metrics` 路径的描述，避免代码先行。

**影响面**：新增一个只读 HTTP 端点；对现有 handler 的调用开销可忽略。
**预估成本**：5-7 天（含面板设计）。

## 5. 插件 webhook 防重放与幂等窗口（P2）

**现状**

- `server/internal/pluginwebhook` 做 token 与 HMAC 校验，但没有时间戳窗口与请求 ID 去重。
- 同一条合法签名的请求被重放时，`webhook.received` 会被重复投递给插件。

**建议**

- 先在 `contracts/` 中冻结 webhook 帧的 `timestamp` 与 `event_id`（或复用插件现有 `request_id`）。
- 在服务端引入：
  - HMAC 校验后对 `(plugin_id, route, event_id)` 做短期 LRU 去重；
  - 超过 `webhook_tolerance_seconds`（默认 5 分钟）的请求直接拒绝。
- 去重表可以复用 SQLite `secret_store` 的轻量键值，或仅维持进程内 LRU（按插件每 1000 条）。

**影响面**：新增字段需要先过 contract；内部服务与 fixtures 同轮更新。
**预估成本**：2-3 天。

## 6. Dispatcher 队列满行为的可观测性升级（P2）

**现状**

- `dispatch.OutcomeDropped` 只写结构化日志，管理面没有正式统计面。
- 用户只能从日志里尝试理解"为什么消息没被处理"。

**建议**

- 在 Dispatcher 内维护窗口化丢弃统计（例如过去 5 分钟、1 小时），按 `plugin_id` + 丢弃原因聚合。
- 通过管理 WebSocket 的 `events.received` 新增 `observability_scope=dispatcher_drop` 分支推送摘要，供系统状态页直接展示。
- 与建议 #4 结合：把 `raylea_dispatcher_dropped_total{plugin, reason}` 导出到 `/metrics`，保留原始事件计数与聚合展示两条路径。

**影响面**：新增 WebSocket payload 分支属于 contract 扩展。
**预估成本**：3-4 天（含 fixtures、Web 展示调整）。

## 7. 事件去重与 echo 超时的二级回路（P2）

**现状**

- Adapter 层做 echo 等待和事件去重（基于 OneBot `message_id`）。
- Bridge / Dispatcher 之间对 `event_id` 没有二级去重；当底层网络抖动导致两条传输都把同一事件推入时，只能靠 Adapter 的去重粗粒度拦截。
- OneBot 某些实现的 echo 缺失会记录 warning，但没有阶段性聚合。

**建议**

- Bridge 入口增加 `(event_id, source_adapter)` LRU 去重（例如 8192 条），并在丢弃时上报 `OutcomeIgnored` 分支。
- `contracts/plugin-protocol.schema.json` 对 `event_id` 已有 `minLength: 1` 约束；进一步要求 `event_id` 的跨 `source_adapter` 全局唯一性规范化规则。
- Echo 缺失告警按 `(transport, action_kind)` 聚合，与建议 #6 的聚合通道共享。

**影响面**：需要小幅扩展现有事件字段语义描述，但不新增字段。
**预估成本**：2 天。

## 8. Web WebSocket 断线重连与状态一致性（P2）

**现状**

- `web/src/lib/ws.ts` 使用 `[500, 1000, 2000, 4000]` 的定长延迟数组。
- 断线超过 4 步后保持 4s 固定节奏重连，没有抖动；多条频道同时重连会对本机服务端造成脉冲压力。
- `service_status` 事件断开期间的业务状态靠 `/api/system/status` HTTP 主拉兜底。

**建议**

- 把重连策略改为指数退避 + 随机抖动：`base=500ms`、`cap=30s`、`jitter=±25%`。
- 每条频道独立维护 `last_seen_token`，重连成功后通过 HTTP 补拉最近窗口内的关键事件（管理接口已提供查询面），再切回 WebSocket 增量。
- 在 UI 层暴露"重连中"与"断线累计时长"的视觉反馈，减少用户误以为服务异常。

**影响面**：`web/src/lib/ws.ts` 内部实现，契约不变。
**预估成本**：3 天。

## 9. Session 密钥轮换与绝对 TTL 边界（P3）

**现状**

- `auth.Config` 支持 `SessionTTLDays`、`SlidingRenewal`、`MaxSessions`。
- 未设置绝对 TTL 上限，理论上一个会话可以通过持续滑动续期存在任意长时间。
- 签名密钥目前随 bootstrap 一次性生成，没有轮换入口。

**建议**

- 在 `config.user.schema.json` 中补 `admin.session_absolute_ttl_days`（默认 30）。
- 提供 `signing_key` 轮换入口（例如 `raylea rotate-signing-key`），轮换期内保留旧 key 的校验能力，新签发走新 key。
- 这些变更必须先过 `contracts/` + `cli-commands.yaml`。

**影响面**：跨契约、CLI、server、fixtures、docs 五件套。
**预估成本**：5-7 天。

## 10. 插件子系统中 `App` 字段二次收敛（P3）

**现状**

- 已经按职责拆出 `plugins`、`pluginconfig`、`pluginfile`、`pluginkv`、`pluginhttp`、`pluginui`、`pluginwebhook`、`protocolcap` 八个包。
- `App` 结构体仍直接持有 14 个插件侧字段（`pluginInstaller`、`pluginUninstaller`、`pluginRepository`、`pluginConfig`、`pluginFiles`、`pluginKV` 等）。

**建议**

- 把这些字段合并成 `pluginStack`（或类似名字）的子对象，`App` 只保留一个指针。
- `pluginStack` 负责所有插件相关子系统的装配、关闭顺序与只读访问。
- 配合建议 #1 一起推进，拆包收益翻倍。

**影响面**：与 #1 合并；纯重构，不改契约。
**预估成本**：1-2 天（在 #1 完成后）。

## 11. Runtime 状态机长时间 `dead_letter` 回路（P3）

**现状**

- 插件崩溃进入 backoff；超过阈值后落入 `dead_letter`，只能通过管理面重新启用。
- `dead_letter` 状态的插件不会自动回收资源（比如已分配的配置 / 存储引用），除非手动卸载。

**建议**

- 提供定时健康检查：处于 `dead_letter` 超过 N 小时后自动执行受控回收（释放本插件的本地连接、订阅），并在管理面给出清理摘要。
- 提供"冷启动尝试"路径：管理员可在不重启整个服务的前提下触发一次轻量重试。
- 上述逻辑进入后再同步到 `docs/architecture/state-model.md` 与 bot-core.md。

**影响面**：server 内部逻辑 + 管理面展示。
**预估成本**：3-5 天。

## 12. 插件 `bot_id` 生命周期与重连语义（P3）

**现状**

- `init.bot` 与 `bot.identity.changed` 已冻结在 `contracts/plugin-protocol.schema.json`。
- SDK 在协议身份不可用时返回空字符串，但"空字符串期间插件应如何处理出站动作"没有统一口径。
- 没有记录 `bot_id` 失效后的重发策略（例如先 buffer 再 flush vs. 直接失败）。

**建议**

- 在 `contracts/plugin-protocol.schema.json` 与 `docs/plugin/protocol.md` 内补充：
  - 身份不可用期间允许插件做什么、不允许做什么；
  - 插件开发者如何订阅 `bot.identity.changed` 重新初始化会话；
  - 运行时对"身份不可用期间"的出站动作的降级策略。
- 在 SDK 中提供一个显式的 `awaitBotIdentity(timeoutMs)` helper，让插件开发者不需要自己 busy-wait。

**影响面**：contract 文本扩展 + SDK 小升级 + 文档同步。
**预估成本**：2-3 天。

---

## 长期跟踪

以下方向当前不在可执行建议内，但值得持续跟踪：

- **多实例 / 高可用**：只做文档化决议；真正推进前先决定是否引入外部消息队列。
- **插件市场与远程分发**：与 v0.3 延后边界一致；先完成可信来源校验闭环。
- **强沙盒**：当前通过 runtime 隔离与 Local Action Service 网关做软隔离。若引入进程级强沙盒，需要在 `contracts/` 中先冻结能力授权的 scope 扩展。
- **非 OneBot 多协议**：adapter 架构已可扩展，但真正新增前须先冻结 `protocol` 维度的 ID 与归一化事件命名空间。

## 收益预估

按上述优先级推进，预计在当前 v0.3 范围内可合入 #2、#3、#4、#8 四条，覆盖最明显的"性能—可观测性—契约对齐"三角。结构性建议（#1、#10）建议滚动分版本进行，以控制一次性大重构的风险。
