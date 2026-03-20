# RayleaBot v0.1 执行计划

> 本文档根据 `docs/RayleaBot机器人项目规划.md`、`docs/engineering/implementation-order.md` 与当前仓库实际落地情况整理。
>
> 本文档在 `docs/engineering/implementation-order.md` 的 10 个顶层阶段之外，额外增加一个 `Pre-Phase / Foundation`，用于记录治理、基线与 CI 骨架。`Phase 1` 到 `Phase 10` 与 `implementation-order` 保持一一对应。后续较细的实现轮次只作为这些顶层阶段的落地进展，不单独改写阶段编号。
>
> 状态图例：✅ 已完成 · 🟡 进行中 · ❌ 未开始 · ⏭️ 暂不纳入 v0.1

---

## 一、总览

| 阶段 | 名称 | 状态 | 当前落地摘要 |
|------|------|------|--------------|
| Pre-Phase | Foundation / 基线 / 仓库治理 / CI 骨架 | 🟡 | 基线、治理、局部规则、repo-local skills 与 CI skeleton 已落库；`.deps/manifest.json` 的来源与哈希类字段仍待后续补全 |
| Phase 1 | 契约文件补全 | ✅ | 当前 formal contract 范围内的 7 份正式契约均已 fixture-ready |
| Phase 2 | Fixtures / Golden Cases | ✅ | config、web-api、websocket、plugin-info、plugin-protocol、release-manifest 的 golden fixtures 已落库 |
| Phase 3 | Server 内核骨架 | ✅ | 最小 server 壳、配置校验、日志、`/healthz`、`/readyz`、examples/plugins 与任务状态骨架已落地 |
| Phase 4 | Adapter（OneBot11） | 🟡 | 只读 reverse WebSocket adapter shell、状态机、intake、最小内部事件归一化与单一 `message.send -> send_msg` 出站 action slice 已落地；更广 action family 仍未实现 |
| Phase 5 | Plugin Protocol Bridge | 🟡 | 最小 runtime manager、`init -> init_ack`、`shutdown(stop)` 与单一 `event -> action(message.send) | result | error` bridge 已落地；完整 bridge 编排仍未实现 |
| Phase 6 | Config / Storage / Security | 🟡 | 配置解析、schema 校验、`auth.Manager`、SQLite 存储层（WAL / read-write split / migration runner）与 auth persistence（bootstrap state + admin sessions 跨重启存活）已落地；secret store、grants/RBAC storage、config hot reload 仍未落地 |
| Phase 7 | Web API & Tasks | 🟡 | `healthz` / `readyz`、只读插件查询、最小任务状态骨架已存在；`POST /api/setup/admin`、`POST /api/session/login` 已实现；`/ws/events` auth-gated aggregate-only WebSocket 已实现；统一 HTTP 鉴权中间件（`RequireAuth`）已落地，受保护路由组与公开路由组已分离；插件写操作 API（install / enable / disable）已实现；任务执行接口与其余管理路由仍未实现 |
| Phase 8 | Web UI | ❌ | `web/package.json` 与 baseline 已有，真实页面与前端交互尚未开始 |
| Phase 9 | Launcher | ❌ | .NET / Avalonia 版本与包基线已锁定，真实 Launcher 行为尚未开始 |
| Phase 10 | Render Service | ❌ | render service 尚未实现；`.deps/manifest.json` 仅为 baseline 资源占位，不代表渲染链路已落地 |

---

## 二、Pre-Phase / Foundation — 基线 / 仓库治理 / CI 骨架 🟡

| 任务项 | 状态 | 说明 |
|--------|------|------|
| 仓库目录结构 | ✅ | `contracts/`、`docs/`、`fixtures/`、`examples/`、`server/`、`web/`、`launcher/`、`.deps/` 已就位 |
| 根与局部 `AGENTS.md` | ✅ | 根、`server/`、`contracts/`、`fixtures/` 规则已落库 |
| repo-local skills | ✅ | `.agents/skills/phase-boundary-check`、`.agents/skills/contract-audit` 已落库 |
| `docs/engineering/baseline.md` | ✅ | 工具链版本、默认命令与工程基线已锁定 |
| `docs/engineering/implementation-order.md` | ✅ | 10 阶段实施顺序已定义 |
| `contracts/README.md` | ✅ | formal contract 范围、TODO 边界与通用规则已建立 |
| Server 基础依赖 | ✅ | `server/go.mod` 已锁定 Go、chi、coder/websocket、jsonschema、yaml 等基线依赖 |
| Web scaffold 基线 | ✅ | `web/package.json` 已锁 Node / pnpm 版本与 TODO scripts |
| Launcher 基线 | ✅ | `launcher/global.json`、`launcher/Directory.Packages.props` 已锁 .NET / Avalonia 版本 |
| `.deps/manifest.json` | 🟡 | 资源 ID 与版本占位已存在，来源与哈希类字段仍待后续补全 |
| CI skeleton | ✅ | `lint.yml` 与 `contracts.yml` 已落库，并实际校验 formal contracts 与 server smoke |

---

## 三、Phase 1 — 契约文件补全 ✅

当前 formal contract 范围内，以下 7 份正式契约已全部进入 fixture-ready：

| 契约文件 | 状态 | 当前结论 |
|----------|------|----------|
| `config.user.schema.json` | ✅ | 当前用户配置 schema 已 fixture-ready |
| `error-codes.yaml` | ✅ | 当前正式错误码目录已 fixture-ready |
| `web-api.openapi.yaml` | ✅ | 当前正式 HTTP 管理接口路径集已 fixture-ready |
| `websocket-events.yaml` | ✅ | 当前正式管理 WebSocket 通道与消息已 fixture-ready |
| `plugin-info.schema.json` | ✅ | 当前正式插件 manifest schema 已 fixture-ready |
| `plugin-protocol.schema.json` | ✅ | 当前正式插件 JSONL 协议 schema 已 fixture-ready |
| `release-manifest.schema.json` | ✅ | 当前正式发行元数据 schema 已 fixture-ready |

说明：

- 本阶段的“已完成”仅表示当前 formal contract 范围已经冻结并有 fixtures 支撑。
- 规划文档中更广的 API、状态或载荷边界，若尚未进入 `contracts/`，仍应视为后续 formalization 工作，而不是本阶段未完成项。
- 当前正式 contract 以 `contracts/` 为准，不应再从规划正文、README 或实现代码反向推断契约状态。

---

## 四、Phase 2 — Fixtures / Golden Cases ✅

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `fixtures/config` | ✅ | `ok` / `invalid` / `edge` 配置样例已落库 |
| `fixtures/web-api` | ✅ | health、ready、plugin、setup-admin、session-login、auth 相关响应样例已落库（14 份） |
| `fixtures/websocket` | ✅ | management WebSocket 消息样例已落库 |
| `fixtures/plugin-info` | ✅ | plugin manifest 的正反与边界样例已落库 |
| `fixtures/plugin-protocol` | ✅ | plugin protocol 的 init / progress / ack 等样例已落库 |
| `fixtures/release-manifest` | ✅ | release manifest 的正反与边界样例已落库 |
| Golden 命名与结构 | ✅ | `ok` / `invalid` / `edge` 命名与 `input/expect`、`request/response/expect`、`frames/expect` 约束已落库 |
| bridge/runtime observability fixtures | ✅ | `events.received` 的 `bridge_runtime` aggregate-only 样例已落库 |

说明：

- 当前 fixtures 已覆盖 config、web-api、websocket、plugin-info、plugin-protocol、release-manifest 六类 formal contract。
- `bridge_runtime` 相关 websocket fixtures 已用于约束 aggregate-only observability 语义，禁止 raw 内容泄漏。

---

## 五、Phase 3 — Server 内核骨架 ✅

| 任务项 | 状态 | 说明 |
|--------|------|------|
| contract-aligned example plugins | ✅ | `examples/plugins/hello-python` 与 `hello-node` 已落库 |
| 配置加载与 schema 校验 | ✅ | 启动前读取 YAML 并消费 `contracts/config.user.schema.json` |
| 统一日志基线 | ✅ | `slog` 已接入 server 最小壳 |
| `GET /healthz` | ✅ | 基础进程存活检查已实现 |
| `GET /readyz` | ✅ | 最小 readiness 报告已实现，并接入保守状态映射 |
| 最小任务状态模型 | ✅ | 任务状态枚举与只读内存模型骨架已存在 |
| server 最小 HTTP 壳 | ✅ | `cmd/raylea-server` 与最小 router/app 装配已落地 |

---

## 六、Phase 4 — Adapter（OneBot11）🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| OneBot11 reverse WebSocket shell | ✅ | 最小反向 WebSocket adapter shell 已落地 |
| 保守连接状态机 | ✅ | `idle` / `connecting` / `connected` / `auth_failed` / `reconnecting` / `stopped` |
| ready-frame gating | ✅ | 仅在看到最小 ready frame 后进入 `connected` |
| backoff reconnect | ✅ | 窄指数退避与抖动重连已实现 |
| 心跳感知与超时处理 | ✅ | 已接入心跳观测与超时回退逻辑 |
| read-only intake 分类 | ✅ | 已对接收到的 OneBot 帧做最小只读 intake 分类 |
| 最小内部事件归一化 | ✅ | 当前仅支持 `onebot11.message_text` 这一内部事件形状 |
| 单一 outbound request-response path | ✅ | 已支持最小 `message.send -> send_msg` 请求构造、`echo` 配对与窄成功/失败观察 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 更广 OneBot 出站 action / request-response path | ❌ | 当前只实现单一 `message.send -> send_msg`；更广 OneBot API 调用与 action 执行链路仍未实现 |
| `message.reply` / media / richer API action | ❌ | 仍未实现 `message.reply`、图片/文件/媒体发送与更广动作族 |
| 广义事件归一化 | ❌ | 尚未扩展到更完整的消息段、通知、请求与其他事件类别 |
| 多 adapter / 多 bot 抽象 | ❌ | 仍为单协议、单实例、单 adapter 的最小壳 |

---

## 七、Phase 5 — Plugin Protocol Bridge 🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| runtime spec creation | ✅ | 已可从有效 discovered plugin 快照构建最小 runtime spec |
| subprocess spawn | ✅ | 最小子进程拉起已落地 |
| `init -> init_ack` | ✅ | 最小启动握手已打通 |
| `shutdown(stop)` | ✅ | 最小优雅停止路径已实现 |
| 最小 lifecycle tracking | ✅ | runtime 最小生命周期状态已在内存中维护 |
| 单一 adapter -> runtime read-only bridge | ✅ | 已支持最小只读事件投递 |
| `event -> action(message.send) | result | error` | ✅ | 当前最小 bridge 已支持单一动作、`result` 与 `error` 三种回收路径 |
| lazy-start first valid plugin | ✅ | 首个可投递事件到达时可 lazy-start 单个有效插件 |
| bridge/runtime summary state | ✅ | 内存计数与最近摘要状态已落地 |
| runtime -> adapter outbound mapper | ✅ | plugin runtime 的单一 `action=message.send` 已可经 bridge 映射到 adapter 的最小 `send_msg` 执行链路 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 更广 adapter 出站 action 执行 | ❌ | 当前只实现单一 `action=message.send`；其余动作族与更丰富发送语义仍未落地 |
| 多插件调度 / fan-out | ❌ | 当前无多插件并发调度与分发引擎 |
| supervisor / backoff / dead_letter 扩展 | ❌ | 尚未建立完整 supervisor 与恢复策略 |
| 完整权限授予状态机 | ❌ | 授权、重确认、撤销等流程尚未实现 |
| 热重载 / restart loop | ❌ | 尚未实现 runtime 热重载与自动重启循环 |

---

## 八、Phase 6 — Config / Storage / Security 🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| YAML parsing | ✅ | 已实现配置文件读取与解析 |
| schema validation | ✅ | 启动前执行严格 schema 校验 |
| 既有 `onebot.*` / server config consumption | ✅ | 当前最小 server、adapter、runtime 会消费已有配置字段 |
| 启动前失败阻断 | ✅ | 配置校验失败时阻止进入正常运行态 |
| `auth.Manager` | ✅ | Session token 管理（HMAC-SHA256 签名、TTL、sliding renewal、max sessions）已实现 |
| Bootstrap & Login | ✅ | 首次管理员初始化与登录已实现（SHA256 摘要 + 常量时间比较） |
| Token Validate | ✅ | Session token 格式校验、HMAC 签名校验、过期检查、自动续期已实现 |
| SQLite 存储层 | ✅ | `internal/storage` 已落地：`modernc.org/sqlite` 纯 Go 驱动、WAL 模式、read/write handle 分离（write max 1 conn、read max 4 conn）、`busy_timeout` 配置、`foreign_keys = ON`、自动创建父目录 |
| Migration runner | ✅ | `internal/storage/migrations.go` 已落地：`schema_migrations` 版本表、嵌入式 `embed.FS` 迁移文件、SHA256 checksum 校验（防止已应用迁移被篡改）、事务性逐条应用、重复 ID 检测、空迁移拒绝 |
| 初始迁移 `0001_auth_core.sql` | ✅ | 创建 `auth_bootstrap_state`（singleton 约束）与 `admin_sessions`（含 `expires_at` 索引）两张表 |
| Auth persistence — Repository 接口 | ✅ | `internal/auth/repository.go` 定义 `Repository` 接口（`LoadBootstrap` / `LoadSessions` / `SaveBootstrap` / `SaveSession` / `DeleteSessions`）与 `SQLiteRepository` 实现 |
| Auth persistence — Bootstrap 持久化 | ✅ | `SaveBootstrap` 在事务中原子写入 bootstrap state + 首个 session；singleton 约束防止重复初始化；signing key 持久化到 SQLite，重启后恢复 |
| Auth persistence — Session 持久化 | ✅ | session 的创建、sliding renewal 续期、过期清理均已持久化到 SQLite；`hydrate()` 在 `NewManager` 时从 SQLite 恢复 bootstrap state、signing key 与未过期 sessions |
| Auth persistence — 跨重启验证 | ✅ | `persistence_test.go` 覆盖：跨重启 bootstrap state 存活、跨重启 token 校验、跨重启过期 session 清理、跨重启 sliding renewal 续期存活 |
| App 集成 — Storage + Auth Repository | ✅ | `internal/app/app.go` 在启动时解析 `database.path`（支持相对路径基于 config 目录解析）、打开 `storage.Store`、构建 `auth.SQLiteRepository` 并注入 `auth.Manager`；关闭时按序释放 storage handle |
| Database config consumption | ✅ | `config.Config` 已包含 `DatabaseConfig{Engine, Path}`，`config.Summary` 已包含 `DatabaseEngine` 与 `DatabasePath`，启动日志已输出数据库引擎与路径 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| secret store | ❌ | 独立敏感凭据存储与注入尚未实现；当前 signing key 已持久化到 SQLite `auth_bootstrap_state` 表，但尚无独立 secret store 抽象 |
| scheduler persistence / recovery | ❌ | 调度持久化与恢复能力尚未实现 |
| grants / RBAC storage | ❌ | 授权记录与权限数据库尚未实现 |
| config hot reload | ❌ | 配置热更新与局部重载尚未实现 |

---

## 九、Phase 7 — Web API & Tasks 🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| `GET /healthz` | ✅ | 基础 liveness 已实现 |
| `GET /readyz` | ✅ | 最小 readiness 已实现 |
| `GET /api/plugins` | ✅ | 只读插件列表查询已实现 |
| `GET /api/plugins/{plugin_id}` | ✅ | 只读插件详情查询已实现 |
| 最小任务状态模型 skeleton | ✅ | 任务状态枚举与最小内存模型已存在 |
| contract-backed `events.received` aggregate payload variant | ✅ | `bridge_runtime` aggregate-only 变体已进入 formal contract 与 fixtures |
| 内部 aggregate-only events emitter | ✅ | server 内部已经可以从 bridge/runtime 内存摘要状态发射 `events.received` 的 aggregate-only `bridge_runtime` 载荷 |
| `POST /api/setup/admin` | ✅ | 首次管理员 Bootstrap 接口已实现：接收 `{identifier, secret}`，返回 `{session_token}`；已阻止重复初始化（403） |
| `POST /api/session/login` | ✅ | 管理员登录接口已实现：凭证校验 + token 签发；错误凭证返回 403 |
| `/ws/events` WebSocket | ✅ | auth-gated aggregate-only observability WebSocket 已实现：统一中间件鉴权（`Authorization: Bearer` 头优先，`session_token` 查询参数向后兼容），订阅 bridge observability 流，自动在 token 过期或断连时关闭 |
| HTTP 鉴权中间件 | ✅ | 统一 `RequireAuth` chi 中间件已落地：从 `Authorization: Bearer <token>` 头提取 token，调用 `auth.Manager.Validate` 校验，Claims 存入 request context；公开路由（`/healthz`、`/readyz`、`/api/setup/admin`、`/api/session/login`）与受保护路由组已分离；`/ws/events` 额外支持 `session_token` 查询参数向后兼容；鉴权失败统一返回 401 ErrorEnvelope（`permission.denied`）；契约已补充 `BearerAuth` 安全方案与 401 响应；鉴权失败 fixtures 已落库 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| system routes | ❌ | `/api/system/shutdown`、`/api/system/info` 尚未实现 |
| 写操作插件 API | ✅ | `POST /api/plugins/install`（异步安装，返回 202 + task_id）、`POST /api/plugins/{plugin_id}/enable`（同步启用）、`POST /api/plugins/{plugin_id}/disable`（同步禁用）已实现；`tasks.Registry.Create` 与 `plugins.Catalog.SetDesiredState` 已落地（含并发安全）；8 个正确性属性测试 + 12 个单元测试已通过 |
| `/api/tasks` 执行型接口 | ❌ | 任务执行、取消与进度接口尚未落地 |
| `/api/config` 配置管理接口 | ❌ | 获取/更新配置尚未实现 |
| `/api/logs` 日志查询接口 | ❌ | 日志查询尚未实现 |
| 其余 WebSocket 通道 | ❌ | `/ws/logs`、`/ws/tasks`、`/ws/plugins/{id}/console` 等尚未实现；当前仅有 `/ws/events` aggregate-only 通道 |
| 全局错误中间件 | ❌ | 统一错误响应中间件仍未完善 |

---

## 十、Phase 8 — Web UI ❌

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `web/package.json` 与 baseline | ✅ | Node / pnpm 基线已锁定，scripts 已占位 |
| 真实页面与布局 | ❌ | 真实页面、路由、状态管理尚未开始 |
| HTTP / WebSocket 消费 | ❌ | 管理面接口消费与实时推送消费尚未开始 |
| UI 交互与管理流程 | ❌ | 插件管理、日志、状态面板等前端交互尚未实现 |

---

## 十一、Phase 9 — Launcher ❌

| 任务项 | 状态 | 说明 |
|--------|------|------|
| .NET / Avalonia 基线 | ✅ | 版本与包基线已锁定 |
| 真实 Launcher 行为 | ❌ | 启停、环境检查、打开 Web UI 等行为尚未开始 |
| 与 server 管理面联动 | ❌ | 尚未接入健康检查、状态展示或管理入口 |

---

## 十二、Phase 10 — Render Service ❌

| 任务项 | 状态 | 说明 |
|--------|------|------|
| render service 实现 | ❌ | 渲染引擎、队列、模板与缓存尚未实现 |
| `.deps/manifest.json` baseline | 🟡 | 仅存在资源清单占位，不代表 render 运行链路已落地 |
| render contract / API surface | ❌ | 当前 formal contract 仍未进入 render service 的公开实现阶段 |

---

## 十三、当前仍未开始但属于 v0.1 路线图的关键能力

- CLI 工具链尚未实现：`reset-admin`、`backup`、`restore`、`doctor`、`migrate`。
- 官方 Python / Node.js SDK 尚未实现；当前仅有 `docs/plugin/sdk/` 文档骨架。
- 官方内置插件体系与更正式的示例插件体系尚未建立；当前仅有最小 contract-aligned examples。
- Command Parser / routing 尚未实现。
- 聊天侧 Permission System、黑名单与冷却限流尚未实现。
- Capabilities / grant manager 的真实授权状态机尚未实现。
- 热重载、`backoff` / `dead_letter` 等更完整插件生命周期流转尚未实现。
- Secret store 独立抽象尚未实现；当前 signing key 已持久化到 SQLite，但无独立 secret store 层。
- Adapter 更广出站动作族尚未实现；当前仅有单一 `message.send -> send_msg` 切片，`message.reply`、media/file/image 与更丰富发送语义仍未落地。
- 多插件并发调度与 fan-out 机制尚未实现。
- Grants / RBAC 存储尚未实现。
- 调度持久化与恢复能力尚未实现。
- 配置热更新与局部重载尚未实现。

这些能力属于 v0.1 路线图的一部分，但当前仓库尚未进入真实实现阶段，不能因为已有 contract、README 或规划正文而误记为“已落地”。

---

## 十四、测试 & CI 现状

### CI 工作流

| 工作流 | 触发 | 覆盖 |
|--------|------|------|
| `contracts.yml` | push main / PR | 7 份 formal contracts 校验、fixture 目录结构、example manifests、server `go test` + `go build` |
| `lint.yml` | push main / PR | baseline 版本锁定校验（Go、Node、pnpm、.NET、Avalonia）、必要目录与文件存在性 |

### Server 根级测试文件（12 个）

| 测试文件 | 覆盖范围 |
|----------|----------|
| `config_fixture_test.go` | 配置 fixture golden case 校验 |
| `example_manifests_test.go` | 示例插件 manifest 合法性校验 |
| `http_health_test.go` | `/healthz` 与 `/readyz` 端点 |
| `plugin_discovery_test.go` | 插件发现与 catalog 构建 |
| `plugin_http_test.go` | 插件 HTTP API（列表、详情、404） |
| `tasks_test.go` | 任务注册表只读操作 |
| `setup_admin_test.go` | Bootstrap 管理员（创建 token、凭证不泄漏、重复初始化拒绝） |
| `session_login_test.go` | 管理员登录（token 签发、错误凭证拒绝、session 上限） |
| `auth_surface_test.go` | Auth 路由攻击面审计（无内部路由暴露） |
| `auth_middleware_test.go` | HTTP 鉴权中间件（7 个属性测试 + 4 个路由分类单元测试：token 提取、统一拒绝、request_id 唯一性、Claims 上下文传递、WebSocket 查询参数备用、头优先级、未鉴权零值、公开/受保护路由分类） |
| `events_ws_test.go` | `/ws/events` WebSocket（鉴权、observability 帧投递、断连清理） |
| `auth_persistence_test.go` | 端到端 auth 持久化（bootstrap state 跨重启存活、bootstrap token 跨重启校验、login token 跨重启 WebSocket 接入、重复初始化跨重启拒绝） |

### 内部包级测试

- `internal/adapter/`: backoff_test、shell_test、intake_test — 覆盖退避算法、连接状态机、帧分类与最小 `send_msg` 出站 request-response
- `internal/auth/`: manager_test、persistence_test — 覆盖 token 签发/校验/过期、sliding renewal、session 上限、Bootstrap 幂等；persistence_test 覆盖跨重启 bootstrap state 存活、跨重启 token 校验、跨重启过期 session 清理、跨重启 sliding renewal 续期
- `internal/bridge/`: bridge_test — 覆盖事件投递、单一 `message.send` 映射、outcome 统计、observability 订阅
- `internal/runtime/`: manager_test、spec_test — 覆盖子进程生命周期、`event -> action(message.send) | result | error`、spec 校验
- `internal/plugins/`: catalog_test、http_test — 覆盖 `SetDesiredState` 状态更新（启用/禁用/冲突/未找到）、并发安全、install handler（round-trip / 无效请求拒绝）、enable/disable handler（成功/404/409）、错误响应 schema 一致性；4 个属性测试 + 4 个单元测试（catalog）、4 个属性测试 + 8 个单元测试（http）
- `internal/tasks/`: tasks_test — 覆盖 `Registry.Create` task_id 唯一性属性测试、task_id 格式校验、创建后 Get/List 可查
- `internal/storage/`: store_test — 覆盖 SQLite 打开、WAL pragma、read/write handle 分离、migration 幂等性、重复 migration ID 拒绝、表结构验证（`schema_migrations` / `auth_bootstrap_state` / `admin_sessions`）

### 总体状况

- fixture / golden 回归已覆盖 config、web-api（14 份）、websocket、plugin-info、plugin-protocol、release-manifest。
- web / launcher 仍主要停留在 baseline scaffold，尚无真实功能测试面。

---

## 十五、下一步行动建议

按当前主线缺口，下一批最小推进建议为：

1. ~~**SQLite 存储层 & Migration**（Phase 6）~~：✅ 已落地。SQLite 打开 / WAL 模式 / read-write handle split / migration runner / `0001_auth_core.sql` 已完成。
2. ~~**Auth persistence**（Phase 6）~~：✅ 已落地。Bootstrap state、admin sessions、signing key 已持久化到 SQLite，跨重启存活已验证。
3. ~~**HTTP 鉴权中间件**（Phase 7）~~：✅ 已落地。统一 `RequireAuth` chi 中间件、公开/受保护路由组分离、`/ws/events` 向后兼容迁移、契约 `BearerAuth` 安全方案与鉴权失败 fixtures 已完成。
4. ~~**插件写操作 API**（Phase 7）~~：✅ 已落地。`POST /api/plugins/install`（异步安装，202 + task_id）、`POST /api/plugins/{plugin_id}/enable`（同步启用）、`POST /api/plugins/{plugin_id}/disable`（同步禁用）已实现；`tasks.Registry.Create`（含 Mutex）与 `plugins.Catalog.SetDesiredState`（含 RWMutex）已落地；`RegisterRoutes` 已扩展并在 `app.go` 中接线；8 个正确性属性测试 + 12 个单元测试已通过。
5. **其余 WebSocket 通道**（Phase 7）：`/ws/logs`、`/ws/tasks`、`/ws/plugins/{id}/console` — 在 `/ws/events` 模式基础上扩展。
6. **更广 outbound action family**（Phase 4 / Phase 5）：在已完成的单一 `message.send` slice 之上，再逐步扩到 `message.reply`、更丰富发送语义与更完整 adapter/runtime action path。
