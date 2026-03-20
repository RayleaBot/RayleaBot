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
| Phase 5 | Plugin Protocol Bridge | 🟡 | 最小 runtime manager、`init -> init_ack`、`shutdown(stop)` 与单一 `event -> action(message.send) \| result \| error` bridge 已落地；`ping/pong` contract 已 formalize，runtime 实现仍未落地；多插件调度、SDK 便利层与更完整 bridge 编排仍未实现 |
| Phase 6 | Config / Storage / Security | 🟡 | 配置解析、schema 校验、`auth.Manager`、SQLite 存储层（WAL / read-write split / migration runner）与 auth persistence（bootstrap state + admin sessions 跨重启存活）已落地；secret store、scheduler persistence、grants/RBAC、config hot reload 与运维工具链仍未落地 |
| Phase 7 | Web API & Tasks | 🟡 | `healthz` / `readyz`、只读插件查询、`POST /api/setup/admin`、`POST /api/session/login`、统一 `RequireAuth` 与 4 条管理 WebSocket 通道已落地；`setup/status`、`session logout`、`launcher-token`、`system/status`、`system/shutdown` 的 contract + fixtures 已落地，handler 仍未实现；真实 task/config/logs 查询面与更完整插件管理面仍未实现 |
| Phase 8 | Web UI | ❌ | `web/package.json` 与 baseline 已有，真实页面与前端交互尚未开始 |
| Phase 9 | Launcher | ❌ | .NET / Avalonia 版本与包基线已锁定，真实 Launcher 行为尚未开始 |
| Phase 10 | Render Service | ❌ | render service 尚未实现；`.deps/manifest.json` 仅为 baseline 资源占位，不代表渲染链路已落地 |

### 判定口径

- "已完成"只用于当前仓库里同时存在**实现、测试与可回指证据**的能力，不把规划目标、README TODO 或 contract 预留项误记为已落地。
- "已 formalize / 未实现"的能力写入对应 phase 的"仍未完成"或末尾路线图。
- 跨 phase 的产品化能力（如 CLI、SDK、官方内置插件体系）按真实依赖关系归并到对应 phase 和后续路线。

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

- 本阶段的"已完成"仅表示当前 formal contract 范围已经冻结并有 fixtures 支撑。
- 规划文档中更广的 API、状态或载荷边界，若尚未进入 `contracts/`，仍应视为后续 formalization 工作，不计入本阶段完成范围。
- 当前正式 contract 以 `contracts/` 为准，不应再从规划正文、README 或实现代码反向推断契约状态。

---

## 四、Phase 2 — Fixtures / Golden Cases ✅

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `fixtures/config` | ✅ | `ok` / `invalid` / `edge` 配置样例已落库 |
| `fixtures/web-api` | ✅ | health、ready、plugin、setup-admin、session-login、auth 与 task-cancel 相关响应样例已落库 |
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
| 更广 OneBot 出站 action / request-response path | ❌ | 当前实现范围为单一 `message.send -> send_msg`；更广 OneBot API 调用与 action 执行链路仍未实现 |
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
| `event -> action(message.send) \| result \| error` | ✅ | 当前最小 bridge 已支持单一动作、`result` 与 `error` 三种回收路径 |
| lazy-start first valid plugin | ✅ | 首个可投递事件到达时可 lazy-start 单个有效插件 |
| bridge/runtime summary state | ✅ | 内存计数与最近摘要状态已落地 |
| runtime -> adapter outbound mapper | ✅ | plugin runtime 的单一 `action=message.send` 已可经 bridge 映射到 adapter 的最小 `send_msg` 执行链路 |
| `ping` / `pong` contract formalize | ✅ | `ping`/`pong` 已进入 `contracts/plugin-protocol.schema.json`、x-message-catalog 与 `fixtures/plugin-protocol/ok.ping-pong.yaml` |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 更广 adapter 出站 action 执行 | ❌ | 当前实现范围为单一 `action=message.send`；其余动作族与更丰富发送语义仍未落地 |
| 多插件调度 / fan-out | ❌ | 当前无多插件并发调度与分发引擎 |
| protocol-level `ping` / `pong` 实现 | ❌ | contract + fixtures 已落地；runtime manager 中的实际收发处理链路仍未实现 |
| supervisor / backoff / dead_letter 扩展 | ❌ | 尚未建立完整 supervisor 与恢复策略 |
| 完整权限授予状态机 | ❌ | 授权、重确认、撤销等流程尚未实现 |
| 热重载 / restart loop | ❌ | 尚未实现 runtime 热重载与自动重启循环 |
| 官方 SDK 便利层 | ❌ | 当前仅有 `docs/plugin/sdk/` 文档骨架，官方 Python / Node.js SDK 尚未进入实现 |
| 官方内置插件与更正式示例插件体系 | ❌ | 当前仍只有最小 `hello-python` / `hello-node` examples，未建立官方内置插件与 richer examples 体系 |
| Command Parser / routing | ❌ | 当前尚未建立基于更丰富事件模型与 runtime bridge 的命令路由层 |

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
| plugin desired_state persistence | ❌ | `POST /api/plugins/{plugin_id}/enable` / `disable` 当前会修改 catalog 内存 `desired_state`，跨重启持久化仍未建立 |
| grants / RBAC storage | ❌ | 授权记录与权限数据库尚未实现 |
| 聊天侧 Permission / 黑名单 / 冷却限流持久化基座 | ❌ | 当前既无相关规则执行面，也无与之配套的持久化结构；后续应建立在 grants / RBAC 与 richer event/runtime 能力之上 |
| config hot reload | ❌ | 配置热更新与局部重载尚未实现 |
| 受控运维工具链（`reset-admin` / `backup` / `restore` / `doctor` / `migrate`） | ❌ | 当前实现范围包括内部 migration runner 与最小 auth persistence；CLI surface、备份/恢复/诊断执行管线尚未建立 |

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
| `/ws/events` WebSocket | ✅ | auth-gated aggregate-only observability WebSocket 已实现：统一中间件鉴权（`Authorization: Bearer` 头优先，`session_token` 查询参数向后兼容），订阅 bridge observability 流；当前为 connect-time admission，不提供已建立连接上的 session_expired push / 强制关断 |
| `/ws/tasks` WebSocket | ✅ | auth-gated tasks 通道已实现：连接建立时回放当前内存 `tasks.Registry` 的最新 snapshots，后续推送 live `tasks.updated` |
| `/ws/logs` WebSocket | ✅ | auth-gated logs 通道已实现：连接建立时回放 bounded in-memory log summaries，后续推送 live `logs.appended`；当前暴露 contract 允许的白名单字段，并对已知敏感字面值做基础掩码 |
| `plugin console` WebSocket | ✅ | `/ws/plugins/{id}/console` 已实现：连接建立时回放每插件 bounded in-memory ring buffer，后续推送经 platform-side redaction + rate limiting 的 runtime `stderr` / `system` console frames；当前不提供历史持久化，也不暴露原始协议 `stdout` |
| HTTP 鉴权中间件 | ✅ | 统一 `RequireAuth` chi 中间件已落地：从 `Authorization: Bearer <token>` 头提取 token，调用 `auth.Manager.Validate` 校验，Claims 存入 request context；公开路由（`/healthz`、`/readyz`、`/api/setup/admin`、`/api/session/login`）与受保护路由组已分离；当前所有已实现的 management WebSocket 路径（`/ws/events`、`/ws/tasks`、`/ws/logs`、`/ws/plugins/{id}/console`）都支持 `session_token` 查询参数向后兼容；鉴权失败统一返回 401 ErrorEnvelope（`permission.denied`）；契约已补充 `BearerAuth` 安全方案与 401 响应；鉴权失败 fixtures 已落库 |
| 最小插件写操作入口 | ✅ | `POST /api/plugins/install` 当前仅做请求校验并创建 `plugin.install` 任务接受回执；`POST /api/plugins/{plugin_id}/enable` / `disable` 当前仅切换 catalog 内存 `desired_state`，尚未进入完整安装/卸载/重载与持久化编排 |
| `GET /api/setup/status` contract + fixtures | ✅ | 接口已进入正式 OpenAPI 与 fixtures；server handler 仍未实现（路由返回 404） |
| `DELETE /api/session` contract + fixtures | ✅ | 接口已进入正式 OpenAPI 与 fixtures；server handler 仍未实现（路由返回 404） |
| `POST /api/session/launcher-token` contract + fixtures | ✅ | 接口已进入正式 OpenAPI 与 fixtures；server handler 仍未实现（路由返回 404） |
| `GET /api/system/status` contract + fixtures | ✅ | 接口已进入正式 OpenAPI 与 fixtures；server handler 仍未实现（路由返回 404） |
| `POST /api/system/shutdown` contract + fixtures | ✅ | 接口已进入正式 OpenAPI 与 fixtures；server handler 仍未实现（路由返回 404） |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| `/api/tasks` 查询与取消 handlers | ❌ | OpenAPI 已冻结 `GET /api/tasks`、`GET /api/tasks/{task_id}`、`POST /api/tasks/{task_id}/cancel`，但 server 仍未实现对应 handlers；现有基础为内存 Registry 与 `/ws/tasks` 推送 |
| 真实 task executor / progress writer | ❌ | 当前实现范围为任务创建与只读 snapshot；持续进度写入、取消驱动和任务执行编排尚未建立 |
| 真实 plugin install pipeline | ❌ | `POST /api/plugins/install` 目前只返回 202 + task_id，并未执行解包、校验、落库或目录安装 |
| plugin reload / uninstall 管理面 | ❌ | `POST /api/plugins/{plugin_id}/reload` 与 `DELETE /api/plugins/{plugin_id}` 仍未 formalize / implement |
| `GET /api/setup/status` handler | ❌ | contract + fixtures 已落地，server handler 仍未实现 |
| `DELETE /api/session` handler | ❌ | contract + fixtures 已落地，server handler 仍未实现 |
| `POST /api/session/launcher-token` handler | ❌ | contract + fixtures 已落地，server handler 仍未实现 |
| `GET /api/system/status` handler | ❌ | contract + fixtures 已落地，server handler 仍未实现 |
| `POST /api/system/shutdown` handler | ❌ | contract + fixtures 已落地，server handler 仍未实现 |
| `/api/config` 配置管理接口 | ❌ | `GET /api/config` / `PUT /api/config` 仍未 formalize / implement |
| `/api/logs` 日志查询接口 | ❌ | 日志检索/查询面仍未 formalize / implement |
| 全局错误中间件 | ❌ | 统一错误响应中间件仍未完善 |

---

## 十、Phase 8 — Web UI ❌

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `web/package.json` 与 baseline | ✅ | Node / pnpm 基线已锁定，scripts 已占位 |
| auth/session shell | ❌ | 登录、session 生命周期与受保护管理面的前端壳尚未开始 |
| 真实页面与布局 | ❌ | 路由、页面结构、状态管理与基础布局尚未开始 |
| HTTP / WebSocket 消费 | ❌ | 插件、任务、日志、events、console 等管理面接口消费尚未开始 |
| 运维交互流 | ❌ | 插件管理、任务查看/取消、配置编辑、system/logs 视图等前端流程尚未实现 |

---

## 十一、Phase 9 — Launcher ❌

| 任务项 | 状态 | 说明 |
|--------|------|------|
| .NET / Avalonia 基线 | ✅ | 版本与包基线已锁定 |
| 环境检查 / 本机诊断壳 | ❌ | 启动前环境检查、资源存在性检查与诊断入口尚未开始 |
| 真实 Launcher 行为 | ❌ | 启停、打开 Web UI、最小托盘/窗口行为尚未开始 |
| 与 server 管理面联动 | ❌ | 尚未接入 `launcher-token`、`system/status`、`system/shutdown` 等最小受控联动面 |
| 发布与安装体验 | ❌ | 安装、升级、版本检查与分发体验尚未开始 |

---

## 十二、Phase 10 — Render Service ❌

| 任务项 | 状态 | 说明 |
|--------|------|------|
| render contract / API surface | ❌ | 当前 formal contract 仍未进入 render service 的公开实现阶段 |
| 渲染队列与 Chromium 调度 | ❌ | 队列、并发控制、超时、重试与浏览器调度尚未实现 |
| 模板校验 / 缓存 / 结果管理 | ❌ | 模板输入校验、缓存、失败回收与产物管理尚未实现 |
| `.deps/manifest.json` baseline | 🟡 | 仅存在资源清单占位，不代表 render 运行链路已落地 |
| 受控运行时资源接线 | ❌ | Chromium / 运行时资源解析、下载校验与 render service 的真实接线尚未实现 |

---

## 十三、测试 & CI 现状

### CI 工作流

| 工作流 / Job | 触发 | 覆盖 |
|--------|------|------|
| `contracts.yml` / `validate-contracts` | push main / PR | 7 份 formal contracts 解析与结构校验、fixture 目录存在性、example manifests、fixture 引用可达性、web-api exact path set、websocket exact channel/event set、plugin-protocol 单一 `message.send` shape |
| `lint.yml` / `baseline` | push main / PR | baseline 版本锁定校验（Go、Node、pnpm、.NET、Avalonia）、必要目录与文件存在性、`.deps/manifest.json` baseline 资源项校验 |
| `lint.yml` / `server-smoke` | push main / PR | server `go test ./...` 与 `go build ./cmd/raylea-server` |

### Server 根级测试文件

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
| `auth_middleware_test.go` | HTTP 鉴权中间件（token 提取、统一拒绝、request_id 唯一性、Claims 上下文传递、管理 WebSocket 查询参数备用、头优先级、未鉴权零值、公开/受保护路由分类） |
| `events_ws_test.go` | `/ws/events` WebSocket（鉴权、observability 帧投递） |
| `tasks_ws_test.go` | `/ws/tasks` WebSocket（鉴权、snapshot replay、live task updates） |
| `logs_ws_test.go` | `/ws/logs` WebSocket（鉴权、bounded summary replay、live append、payload whitelist、基础敏感字面值掩码） |
| `console_ws_test.go` | `/ws/plugins/{id}/console` WebSocket（鉴权、ring-buffer replay、live plugin console frames） |
| `auth_persistence_test.go` | 端到端 auth 持久化（bootstrap state 跨重启存活、bootstrap token 跨重启校验、login token 跨重启 WebSocket 接入、重复初始化跨重启拒绝） |

### 内部包级测试

- `internal/adapter/`: backoff_test、shell_test、intake_test — 覆盖退避算法、连接状态机、帧分类与最小 `send_msg` 出站 request-response
- `internal/auth/`: manager_test、persistence_test — 覆盖 token 签发/校验/过期、sliding renewal、session 上限、Bootstrap 幂等；persistence_test 覆盖跨重启 bootstrap state 存活、跨重启 token 校验、跨重启过期 session 清理、跨重启 sliding renewal 续期
- `internal/bridge/`: bridge_test — 覆盖事件投递、单一 `message.send` 映射、outcome 统计、observability 订阅
- `internal/runtime/`: manager_test、console_test、spec_test — 覆盖子进程生命周期、`event -> action(message.send) | result | error`、受控 `stderr` console capture / redaction / rate limiting、spec 校验
- `internal/logging/`: stream_test — 覆盖结构化日志在进入管理面摘要流前的基础敏感字面值掩码
- `internal/plugins/`: catalog_test、http_test — 覆盖 `SetDesiredState` 状态更新（启用/禁用/冲突/未找到）、并发安全、install handler（round-trip / 无效请求拒绝）、enable/disable handler（成功/404/409）、错误响应 schema 一致性；4 个属性测试 + 4 个单元测试（catalog）、4 个属性测试 + 8 个单元测试（http）
- `internal/tasks/`: tasks_test — 覆盖 `Registry.Create` task_id 唯一性属性测试、task_id 格式校验、创建后 Get/List 可查
- `internal/storage/`: store_test — 覆盖 SQLite 打开、WAL pragma、read/write handle 分离、migration 幂等性、重复 migration ID 拒绝、表结构验证（`schema_migrations` / `auth_bootstrap_state` / `admin_sessions`）

### 总体状况

- fixture / golden 回归已覆盖 config、web-api、websocket、plugin-info、plugin-protocol、release-manifest。
- web / launcher 仍主要停留在 baseline scaffold，尚无真实功能测试面。

---

## 十四、下一步行动建议

以下路线聚焦当前**仍未完成**的工作，并按"近期主线 -> 状态化基础 -> 生态与产品化外层"排列。

### 1. 近期主线（优先继续补平台闭环）

1. **补全 Phase 7 已 formalize 但仍缺实现的接口 handlers**
   - `GET /api/setup/status`、`DELETE /api/session`、`POST /api/session/launcher-token`、`GET /api/system/status`、`POST /api/system/shutdown` 的 contract + fixtures 均已落地，server handler 仍未实现。
   - 优先补齐这批已有 contract 支撑的 handler，使管理面闭环。

2. **实现 `/api/tasks` list / detail / cancel 三个已冻结接口**
   - 再补真实 task executor / progress writer，使当前 install acceptance 与 future backup/restore/migrate task type 有统一执行落点。

3. **补 config/logs 查询与统一错误中间件**
   - `/api/config`、`/api/logs` 的 handler 实现，以及全局错误中间件，避免后续 handler 与 WebSocket 入口继续各自散落错误落点。

4. **把当前"入口级插件写操作"推进成"真实执行链路"**
   - `POST /api/plugins/install` 目前只做到 task acceptance。
   - `enable` / `disable` 目前只做到内存 `desired_state` 切换，后续应逐步接到更真实的 runtime / persistence / grants 流程。

5. **在下一条 action contract 落定后，补第二个最小 outbound action slice**
   - 继续保持"一次只落一个动作种类"的节奏，不直接扩成通用 action 平台。
   - 优先考虑 `message.reply` 这类最贴近现有聊天闭环的单动作切片，再做更广 send semantics。
   - 新 action 种类必须先进入 `contracts/plugin-protocol.schema.json`、fixtures、examples、tests，再能进入实现。

6. **运维工具链的 contract / execution model 收口**
   - `reset-admin`、`backup`、`restore`、`doctor`、`migrate` 仍无正式 CLI/后端执行面。
   - 在进入实现前，需要先把哪些能力通过 HTTP、哪些能力只保留 CLI、本地/停服窗口要求、任务模型与恢复语义进一步收口。

### 2. 状态化基础（为恢复、运维与长期运行铺路）

1. **Scheduler persistence / recovery**
   - 当前 SQLite foundation 与 auth persistence 已就位，但调度持久化与恢复仍未开始。
   - 这是把"进程内状态"推进为"可恢复平台状态"的下一层关键基础。

2. **Secret store 独立抽象**
   - 当前 signing key 已持久化到 SQLite，但还没有独立 secret store 层。
   - 后续若要扩展更多鉴权、外部连接、render 或 CLI 恢复能力，这一层需要先收口。

3. **Grants / RBAC storage 与真实授权状态机**
   - 当前实现范围为最小 management auth/session；真实授权记录、grants 存储和 grant manager 状态机尚未进入实现。
   - 这会直接影响插件能力授予、升级 re-grant、聊天侧权限控制和后续 Web UI / CLI 运维面。

4. **Config hot reload / 局部重载**
   - 当前配置仍是启动时加载模式。
   - 热更新和局部重载是后续 scheduler、logging、runtime 限流、render 队列以及长运行服务调优的基础。

### 3. Runtime / Adapter / Plugin 扩展路线

1. **protocol-level `ping` / `pong` 实现**
   - contract + fixtures 已落地；runtime manager 中的实际收发处理链路仍未实现。
   - 在进入更高层 SDK 与 richer runtime behavior 前，应先把最小心跳/保活语义补齐。

2. **更广 OneBot 事件归一化**
   - 当前进入 runtime bridge 的内部事件形状为 `onebot11.message_text`。
   - 通知、请求、更多消息段和 richer event shapes 仍未进入实现。

3. **多插件并发调度与 fan-out**
   - 当前仍是"单 runtime、单插件、lazy-start first valid plugin"的最小切片。
   - 进入更真实的插件生态前，这一层必须先被 formalize 并最小落地。

4. **热重载、restart loop、`backoff` / `dead_letter`**
   - 当前 runtime 生命周期仍停留在最小 shell。
   - 更完整的恢复策略和生命周期流转仍是 Phase 5 的主缺口。

5. **官方 SDK、内置插件体系与 richer examples**
   - 当前仅有协议文档骨架和最小 examples。
   - 在 plugin protocol 与 runtime boundary 更稳定后，再推进官方 Python / Node.js SDK、内置插件体系和更正式的示例插件矩阵会更稳妥。

6. **Command Parser / routing、聊天侧 Permission / 黑名单 / 冷却限流**
   - 这些仍属于 v0.1 路线图，但还没有进入真实实现阶段。
   - 其依赖包括 richer event model、grants/storage、multi-plugin dispatch 与更完整 runtime 生命周期。

### 4. 产品化外层路线（在核心平台更稳定后推进）

1. **Web UI**（Phase 8）
   - 当前仍停留在 scaffold / baseline。
   - 在更多管理 API、任务面、config/logs 查询面与 session lifecycle 稳定后，再进入真实页面与交互流会更顺畅。

2. **Launcher**（Phase 9）
   - 当前已落定 .NET / Avalonia baseline。
   - `launcher-token`、`system/status`、`system/shutdown` 的 contract + fixtures 均已就位，Launcher 真实能力实现可在此基础上推进。

3. **Render Service**（Phase 10）
   - 当前 `.deps/manifest.json` 仍只是 baseline 资源占位。
   - render queue、browser scheduling、cache、模板输入校验与 render contract 仍未进入实现。

4. **CLI / 本地运维体验**
   - `reset-admin`、`backup`、`restore`、`doctor`、`migrate` 仍需要单独设计本地执行体验、停服窗口、诊断输出和与 Web/Launcher 的职责边界。
