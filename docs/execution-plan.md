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
| Phase 1 | 契约文件补全 | ✅ | 7 份正式契约均已 fixture-ready |
| Phase 2 | Fixtures / Golden Cases | ✅ | config、web-api、websocket、plugin-info、plugin-protocol、release-manifest 的 golden fixtures 已落库 |
| Phase 3 | Server 内核骨架 | ✅ | 最小 server 壳、配置校验、日志、`/healthz`、`/readyz`、examples/plugins 与任务状态骨架已落地 |
| Phase 4 | Adapter（OneBot11） | 🟡 | reverse WebSocket adapter、状态机、intake、最小事件归一化、三种出站 action（`message.send` / `message.reply` / `message.send_image`）已落地；更广 action family 与事件归一化仍未实现 |
| Phase 5 | Plugin Protocol Bridge | 🟡 | runtime manager、init/shutdown/ping-pong、三种 action bridge、supervisor crash-backoff/dead_letter、用户主动 reload 已落地；多插件调度、SDK 便利层与完整权限授予状态机仍未实现 |
| Phase 6 | Config / Storage / Security | 🟡 | 配置解析与校验、auth（session + bootstrap + persistence）、SQLite（WAL + migration）、plugin desired_state/packages 持久化、CLI 契约骨架已落地；secret store、scheduler persistence、grants/RBAC、config hot reload 与 CLI 子命令实现仍未落地 |
| Phase 7 | Web API & Tasks | 🟡 | 全部管理路由（setup/session/config/system/logs/tasks）、4 条管理 WebSocket、plugin install/enable/disable/reload/uninstall、统一鉴权与中断安装清理已落地；通用 task executor、远程安装源、配置热更新与日志持久化查询仍未实现 |
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
| `fixtures/web-api` | ✅ | health、ready、plugin、setup-admin、session-login、auth、config、logs、tasks、plugin install/enable/disable/reload/uninstall 相关样例已落库 |
| `fixtures/websocket` | ✅ | management WebSocket 消息样例已落库，包含 tasks/logs/console/events 的正向与边界样例 |
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
| `message.reply` outbound action slice | ✅ | 已支持 `message.reply -> send_msg(CQ:reply,id=<id>)` 请求构造与 `echo` 配对 |
| `message.send_image` outbound action slice | ✅ | 已支持 `message.send_image -> send_msg(CQ:image,file=<file>)` 请求构造与 `echo` 配对 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 更广 OneBot 出站 action / request-response path | ❌ | 当前实现范围为 `message.send`、`message.reply` 与 `message.send_image`；更广 OneBot API 调用与 action 执行链路仍未实现 |
| `media / richer API action` | ❌ | 文件发送、媒体发送与更广动作族仍未实现 |
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
| `event -> action(message.reply) \| result \| error` | ✅ | bridge 已支持 `message.reply` 动作路径，经 `SendReply` 映射到 `send_msg(CQ:reply)` |
| `event -> action(message.send_image) \| result \| error` | ✅ | bridge 已支持 `message.send_image` 动作路径，经 `SendImage` 映射到 `send_msg(CQ:image)` |
| lazy-start first valid plugin | ✅ | 首个可投递事件到达时可 lazy-start 单个有效插件 |
| bridge/runtime summary state | ✅ | 内存计数与最近摘要状态已落地 |
| runtime -> adapter outbound mapper | ✅ | plugin runtime 的 `action=message.send`、`action=message.reply` 与 `action=message.send_image` 均已可经 bridge 映射到 adapter 的最小 `send_msg` 执行链路 |
| `ping` / `pong` contract formalize | ✅ | `ping`/`pong` 已进入 `contracts/plugin-protocol.schema.json`、x-message-catalog 与 `fixtures/plugin-protocol/ok.ping-pong.yaml` |
| `ping` / `pong` runtime 实现 | ✅ | runtime manager 中的 `Ping()` / `awaitPong()` / `parsePongResponse()` 已实现，含超时停止与协议违规检测 |
| supervisor / crash-backoff / dead_letter | ✅ | runtime manager 支持 `crashed` / `backoff` / `dead_letter` 状态流转；lifecycle controller 驱动指数退避重启与最大重试次数后进入 `dead_letter`；配置消费 `crash_backoff_initial_seconds` / `crash_backoff_max_seconds` |
| 用户主动 reload | ✅ | `POST /api/plugins/{plugin_id}/reload` 停止当前 runtime 后重新启动，desired_state 保持 enabled 不变 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 更广 adapter 出站 action 执行 | ❌ | 当前实现范围为 `message.send`、`message.reply` 与 `message.send_image`；其余动作族与更丰富发送语义仍未落地 |
| 多插件调度 / fan-out | ❌ | 当前无多插件并发调度与分发引擎 |
| 完整权限授予状态机 | ❌ | 授权、重确认、撤销等流程尚未实现 |
| 不停机热重载 | ❌ | 当前 reload 通过 stop + start 实现；更精细的不停机代码更新仍未实现 |
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
| 插件状态迁移 `0002_plugin_instances.sql` | ✅ | `plugin_instances` 表已落地，当前只持久化 `plugin_id`、`desired_state` 与 `updated_at` |
| Plugin desired_state persistence — Repository 接口 | ✅ | `internal/plugins/repository.go` 已提供 `LoadDesiredStates` / `SaveDesiredState` / `DeleteDesiredState` 的 SQLite 实现 |
| Plugin desired_state persistence — Startup hydration | ✅ | `internal/app/app.go` 启动后会读取 `plugin_instances`，并在 discovery 结果上恢复已安装插件的 `desired_state` |
| Plugin desired_state persistence — 跨重启验证 | ✅ | `plugin_persistence_test.go` 覆盖 enable/disable 后重启仍保留 `desired_state`，`runtime_state` 保持进程内语义 |
| Plugin `plugin_packages` 元数据持久化 | ✅ | `internal/plugins/repository.go` 提供 `PackageRepository` 接口与 `SavePackageMetadata` / `DeletePackageMetadata` 的 SQLite upsert/delete 实现；install 执行链在写入正式目录后持久化 `source_type`、`source_ref`、`version`、`manifest_hash`、`package_hash`、`installed_at`；uninstall 执行链在卸载时清理对应记录 |
| App 集成 — Storage + Auth Repository | ✅ | `internal/app/app.go` 在启动时解析 `database.path`（支持相对路径基于 config 目录解析）、打开 `storage.Store`、构建 `auth.SQLiteRepository` 并注入 `auth.Manager`；关闭时按序释放 storage handle |
| Database config consumption | ✅ | `config.Config` 已包含 `DatabaseConfig{Engine, Path}`，`config.Summary` 已包含 `DatabaseEngine` 与 `DatabasePath`，启动日志已输出数据库引擎与路径 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| secret store | ❌ | 独立敏感凭据存储与注入尚未实现；当前 signing key 已持久化到 SQLite `auth_bootstrap_state` 表，但尚无独立 secret store 抽象 |
| scheduler persistence / recovery | ❌ | 调度持久化与恢复能力尚未实现 |
| grants / RBAC storage | ❌ | 授权记录与权限数据库尚未实现 |
| 聊天侧 Permission / 黑名单 / 冷却限流持久化基座 | ❌ | 当前既无相关规则执行面，也无与之配套的持久化结构；后续应建立在 grants / RBAC 与 richer event/runtime 能力之上 |
| config hot reload | ❌ | 配置热更新与局部重载尚未实现 |
| 受控运维工具链 CLI 子命令实现 | ❌ | `contracts/cli-commands.yaml` 正式契约骨架已落地（6 条子命令、在线/离线可用性矩阵、task 模型关联与可取消性）；CLI 子命令的实际执行逻辑、备份/恢复/诊断执行管线尚未建立 |

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
| Management status / session / system handlers | ✅ | `GET /api/setup/status`、`DELETE /api/session`、`POST /api/session/launcher-token`、`GET /api/system/status`、`POST /api/system/shutdown` 已按现有 contract 落地；当前 `launcher-token` 为进程内、单次使用、短 TTL 的最小 issuance shell |
| Config / logs management handlers | ✅ | `GET /api/config`、`PUT /api/config`、`GET /api/logs` 已落地；配置读取返回当前生效配置的可公开快照，敏感字段做基础掩码；配置更新按 formal schema 校验后原子写回 `config/user.yaml`，并显式返回 `restart_required`；日志查询复用 bounded in-memory summary stream 与既有 redaction 逻辑 |
| `/api/tasks` list / detail / cancel handlers | ✅ | `GET /api/tasks`、`GET /api/tasks/{task_id}`、`POST /api/tasks/{task_id}/cancel` 已落地；当前直接复用内存 `tasks.Registry`，并对运行中的 `plugin.install` 提供最小取消接线；其余不可取消状态继续返回既有 `platform.task_not_cancellable` 错误形状 |
| `POST /api/plugins/install` | ✅ | 异步 local-source install 执行链：支持 `local_directory` / `local_zip`、来源准备、manifest 校验、正式目录写入、catalog refresh、依赖安装（`preparePython` / `prepareNode`）、`plugin_packages` 元数据持久化、install scripts 授权（`AllowInstallScripts`）、task progress 更新与最小取消 |
| `POST /api/plugins/{plugin_id}/enable` / `disable` | ✅ | 已切换到 SQLite 持久化 `desired_state`，并在 `enable` 时触发 runtime 实际启动（含 capability gating）、在 `disable` 时触发 runtime 实际停止；runtime_state 保持进程内语义 |
| `POST /api/plugins/{plugin_id}/reload` | ✅ | 已 formalize（contract + fixtures）并实现：停止当前 runtime 后重新启动，desired_state 保持 enabled 不变；仅当 desired_state=enabled 时接受，否则返回 409 |
| `DELETE /api/plugins/{plugin_id}` | ✅ | 已 formalize（contract + fixtures）并实现异步 `plugin.uninstall` task：停止 runtime、清理 `plugin_instances` 与 `plugin_packages` 数据库记录、删除安装目录、刷新 catalog |
| 中断安装清理 | ✅ | 启动时自动扫描 `plugins/installed/` 中遗留的 `.plugin-install-*` 临时目录并清理，防止中断安装的孤立目录累积 |
| 共享 HTTP error / request context 写入路径 | ✅ | 路由级 request_id 注入、统一 JSON error envelope 写入与最小 panic recovery 已在 server router 上接通；management handlers 与 plugin handlers 当前共享同一写出路径 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 通用 task executor / progress writer | ❌ | 当前实现范围包括 `plugin.install` 与 `plugin.uninstall` 两种异步执行切片；backup/restore/migrate 等更广 task type 的统一执行编排、历史持久化与恢复仍未建立 |
| 远程来源 plugin install | ❌ | 当前 install 仅支持 `local_directory` / `local_zip`；远程来源（HTTP/HTTPS、artifact repository）仍未实现 |
| 配置热更新 / 局部重载 | ❌ | `PUT /api/config` 当前只完成 formal schema 校验、原子写盘与 `restart_required` 响应；热更新、局部重载与字段级即时生效尚未实现 |
| 日志历史检索 / 持久化查询 | ❌ | `GET /api/logs` 当前只查询 bounded in-memory summary stream，不提供日志文件检索、全量 attrs 或历史持久化查询 |

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
| `contracts.yml` / `validate-contracts` | push main / PR | 7 份 fixture-ready formal contracts 解析与结构校验、`cli-commands.yaml` 命令集与可用性矩阵校验、fixture 目录存在性、example manifests、fixture 引用可达性、web-api exact path set、websocket exact channel/event set、plugin-protocol 三种 action（`message.send` / `message.reply` / `message.send_image`）shape、CLI task_type 与 web-api TaskType enum 交叉校验 |
| `lint.yml` / `baseline` | push main / PR | baseline 版本锁定校验（Go、Node、pnpm、.NET、Avalonia）、必要目录与文件存在性、`.deps/manifest.json` baseline 资源项校验 |
| `lint.yml` / `server-smoke` | push main / PR | server `go test ./...` 与 `go build ./cmd/raylea-server` |

### Server 根级测试文件

| 测试文件 | 覆盖范围 |
|----------|----------|
| `config_fixture_test.go` | 配置 fixture golden case 校验 |
| `example_manifests_test.go` | 示例插件 manifest 合法性校验 |
| `http_health_test.go` | `/healthz` 与 `/readyz` 端点 |
| `plugin_discovery_test.go` | 插件发现与 catalog 构建 |
| `plugin_http_test.go` | 插件 HTTP API（列表、详情、404、reload 成功/拒绝/404、uninstall 成功/404） |
| `tasks_test.go` | 任务注册表只读操作 |
| `setup_admin_test.go` | Bootstrap 管理员（创建 token、凭证不泄漏、重复初始化拒绝） |
| `session_login_test.go` | 管理员登录（token 签发、错误凭证拒绝、session 上限） |
| `auth_surface_test.go` | Auth 路由攻击面审计（无内部路由暴露） |
| `auth_middleware_test.go` | HTTP 鉴权中间件（token 提取、统一拒绝、request_id 唯一性、Claims 上下文传递、管理 WebSocket 查询参数备用、头优先级、未鉴权零值、公开/受保护路由分类） |
| `management_http_test.go` | `setup/status`、`session logout`、`launcher-token`、`system/status`、`system/shutdown` handlers |
| `config_http_test.go` | `/api/config` 读取、原子写盘、敏感字段保留/掩码与 `restart_required` 语义 |
| `logs_http_test.go` | `/api/logs` 过滤、空结果、无效查询拒绝与 redaction/字段白名单 |
| `tasks_http_test.go` | `/api/tasks` list / detail / cancel handlers（过滤、404、取消接受与不可取消拒绝） |
| `events_ws_test.go` | `/ws/events` WebSocket（鉴权、observability 帧投递） |
| `tasks_ws_test.go` | `/ws/tasks` WebSocket（鉴权、snapshot replay、live task updates） |
| `logs_ws_test.go` | `/ws/logs` WebSocket（鉴权、bounded summary replay、live append、payload whitelist、基础敏感字面值掩码） |
| `console_ws_test.go` | `/ws/plugins/{id}/console` WebSocket（鉴权、ring-buffer replay、live plugin console frames） |
| `auth_persistence_test.go` | 端到端 auth 持久化（bootstrap state 跨重启存活、bootstrap token 跨重启校验、login token 跨重启 WebSocket 接入、重复初始化跨重启拒绝） |
| `plugin_persistence_test.go` | plugin `desired_state` 跨重启持久化与启动 hydration |
| `plugin_install_flow_test.go` | 带鉴权的 `/api/plugins/install` 端到端 local-source 安装执行、task 成功收敛与 catalog refresh |

### 内部包级测试

- `internal/adapter/`: backoff_test、shell_test、intake_test — 覆盖退避算法、连接状态机、帧分类、`message.send -> send_msg`、`message.reply -> send_msg(CQ:reply)` 与 `message.send_image -> send_msg(CQ:image)` 出站 request-response
- `internal/auth/`: manager_test、persistence_test — 覆盖 token 签发/校验/过期、sliding renewal、session 上限、Bootstrap 幂等；persistence_test 覆盖跨重启 bootstrap state 存活、跨重启 token 校验、跨重启过期 session 清理、跨重启 sliding renewal 续期
- `internal/bridge/`: bridge_test — 覆盖事件投递、`message.send`、`message.reply` 与 `message.send_image` 映射、outcome 统计、observability 订阅
- `internal/runtime/`: manager_test、backoff_test、console_test、spec_test — 覆盖子进程生命周期、`ping/pong`、`event -> action(message.send | message.reply | message.send_image) | result | error`、crash 检测与 `CrashCallback` 调用、crash count 跨重启累积、`ResetCrashCount` / `SetBackoffState` / `SetDeadLetterState` 状态流转、指数退避计算（含零值与负值边界）、受控 `stderr` console capture / redaction / rate limiting、spec 校验
- `internal/logging/`: stream_test — 覆盖结构化日志在进入管理面摘要流前的基础敏感字面值掩码
- `internal/plugins/`: catalog_test、http_test、repository_test、install_test — 覆盖 `SetDesiredState` 状态更新（启用/禁用/冲突/未找到）、并发安全、install handler（round-trip / 无效请求拒绝）、enable/disable handler（成功/404/409 + 持久化写入）、SQLite desired_state repository 读写与 startup hydration，以及最小 local-source install 执行链（目录/压缩包安装、catalog refresh、重复 `plugin_id` 拒绝、运行中任务取消）
- `internal/tasks/`: tasks_test — 覆盖 `Registry.Create` task_id 唯一性属性测试、task_id 格式校验、创建后 Get/List 可查
- `internal/storage/`: store_test — 覆盖 SQLite 打开、WAL pragma、read/write handle 分离、migration 幂等性、重复 migration ID 拒绝、表结构验证（`schema_migrations` / `auth_bootstrap_state` / `admin_sessions` / `plugin_instances`）

### 总体状况

- fixture / golden 回归已覆盖 config、web-api、websocket、plugin-info、plugin-protocol、release-manifest。
- web / launcher 仍主要停留在 baseline scaffold，尚无真实功能测试面。

---

## 十四、下一步行动建议

以下路线聚焦当前**仍未完成**的工作，并按"近期主线 -> 状态化基础 -> 生态与产品化外层"排列。

### 1. 近期主线（优先继续补平台闭环）

1. **远程来源 plugin install**
   - 当前 install 仅支持 `local_directory` / `local_zip`。
   - 远程来源（HTTP/HTTPS、artifact repository）是进入真实分发体验前的必要能力。

2. **Grants / RBAC 与完整权限授予状态机**
   - 当前 capability gating 仅基于 `config.auth.auto_grant_capabilities` 的静态匹配。
   - 需要建立 grants storage、grant manager 状态机（授权/重确认/撤销）与 per-plugin 权限跟踪，这是多插件生态与聊天侧权限控制的前置依赖。

3. **运维工具链 CLI 子命令实现**
   - `contracts/cli-commands.yaml` 正式契约骨架已落地（6 条子命令、在线/离线可用性矩阵、task 模型关联与可取消性）。
   - CI 已校验 CLI 命令集完整性与 task_type 与 web-api `TaskType` enum 的交叉一致性。
   - CLI 子命令的实际执行逻辑仍需在后续轮次中实现。

### 2. 状态化基础（为恢复、运维与长期运行铺路）

1. **Scheduler persistence / recovery**
   - 当前 SQLite foundation 与 auth persistence 已就位，但调度持久化与恢复仍未开始。
   - 这是把"进程内状态"推进为"可恢复平台状态"的下一层关键基础。

2. **Secret store 独立抽象**
   - 当前 signing key 已持久化到 SQLite，但还没有独立 secret store 层。
   - 后续若要扩展更多鉴权、外部连接、render 或 CLI 恢复能力，这一层需要先收口。

3. **Config hot reload / 局部重载**
   - 当前配置仍是启动时加载模式。
   - 热更新和局部重载是后续 scheduler、logging、runtime 限流、render 队列以及长运行服务调优的基础。

### 3. Runtime / Adapter / Plugin 扩展路线

1. **更广 OneBot 事件归一化**
   - 当前进入 runtime bridge 的内部事件形状为 `onebot11.message_text`。
   - 通知、请求、更多消息段和 richer event shapes 仍未进入实现。

2. **多插件并发调度与 fan-out**
   - 当前仍是"单 runtime、单插件、lazy-start first valid plugin"的最小切片。
   - 进入更真实的插件生态前，这一层必须先被 formalize 并最小落地。

3. **不停机热重载**
   - 当前 reload 通过 stop + start 实现，存在短暂中断窗口。
   - 更精细的不停机代码更新仍未实现。

4. **官方 SDK、内置插件体系与 richer examples**
   - 当前仅有协议文档骨架和最小 examples。
   - 在 plugin protocol 与 runtime boundary 更稳定后，再推进官方 Python / Node.js SDK、内置插件体系和更正式的示例插件矩阵会更稳妥。

5. **Command Parser / routing、聊天侧 Permission / 黑名单 / 冷却限流**
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
   - `contracts/cli-commands.yaml` 已定义 6 条子命令的正式执行模型、在线/离线可用性矩阵与 task 模型关联。
   - CLI 子命令的实际执行逻辑、停服窗口检测、诊断输出格式和与 Web/Launcher 的共享后端路径仍需在后续轮次中实现。
