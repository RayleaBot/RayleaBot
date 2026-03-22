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
| Phase 4 | Adapter（OneBot11） | 🟡 | reverse WebSocket adapter、状态机、intake、广义事件归一化、三种出站 action、内部 API 调用、identity cache 已落地；更广 media action family 与多 adapter 抽象仍未实现 |
| Phase 5 | Plugin Protocol Bridge | 🟡 | runtime manager、init/shutdown/ping-pong、三种 action bridge、supervisor crash-backoff/dead_letter、用户主动 reload、多插件并发调度 / fan-out、command parser / routing、零停机热重载、官方 Python / Node.js SDK、内置 help 插件已落地；temporal grants 仍未实现 |
| Phase 6 | Config / Storage / Security | 🟡 | 配置解析与校验、auth 全链路持久化、SQLite（WAL + migration）、plugin desired_state/packages 持久化、grants storage + scope 持久化 + 升级 re-grant、CLI 6 条子命令、secret store、config hot reload、scheduler persistence、聊天侧 Permission / 黑名单 / 冷却限流持久化基座已落地 |
| Phase 7 | Web API & Tasks | 🟡 | 全部管理路由、4 条管理 WebSocket、plugin install/enable/disable/reload/uninstall、per-plugin grants 管理端点、统一鉴权、通用 task executor、task 历史持久化与跨重启恢复、配置热更新与字段级即时生效已落地；日志持久化查询仍未实现 |
| Phase 8 | Web UI | ❌ | `web/package.json` 与 baseline 已有，真实页面与前端交互尚未开始 |
| Phase 9 | Launcher | ❌ | .NET / Avalonia 版本与包基线已锁定，真实 Launcher 行为尚未开始 |
| Phase 10 | Render Service | ❌ | render service 尚未实现；`.deps/manifest.json` 仅为 baseline 资源占位 |

### 判定口径

- "已完成"只用于当前仓库里同时存在实现、测试与可回指证据的能力，不把规划目标、README TODO 或 contract 预留项误记为已落地。
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

7 份正式契约均已进入 fixture-ready：`config.user.schema.json`、`error-codes.yaml`、`web-api.openapi.yaml`、`websocket-events.yaml`、`plugin-info.schema.json`、`plugin-protocol.schema.json`、`release-manifest.schema.json`。

说明：

- 本阶段的"已完成"仅表示当前 formal contract 范围已经冻结并有 fixtures 支撑。
- 规划文档中更广的 API、状态或载荷边界，若尚未进入 `contracts/`，仍应视为后续 formalization 工作。
- 当前正式 contract 以 `contracts/` 为准，不应再从规划正文、README 或实现代码反向推断契约状态。

---

## 四、Phase 2 — Fixtures / Golden Cases ✅

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `fixtures/config` | ✅ | `ok` / `invalid` / `edge` 配置样例已落库 |
| `fixtures/web-api` | ✅ | health、ready、plugin、setup-admin、session-login、auth、config、logs、tasks、plugin install/enable/disable/reload/uninstall 相关样例已落库 |
| `fixtures/websocket` | ✅ | management WebSocket 消息样例已落库，包含 tasks/logs/console/events 的正向与边界样例 |
| `fixtures/plugin-info` | ✅ | plugin manifest 的正反与边界样例已落库 |
| `fixtures/plugin-protocol` | ✅ | plugin protocol 的 init / progress / ack / ping / pong 等样例已落库 |
| `fixtures/release-manifest` | ✅ | release manifest 的正反与边界样例已落库 |
| Golden 命名与结构 | ✅ | `ok` / `invalid` / `edge` 命名与 `input/expect`、`request/response/expect`、`frames/expect` 约束已落库 |
| bridge/runtime observability fixtures | ✅ | `events.received` 的 `bridge_runtime` aggregate-only 样例已落库 |

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
| 广义事件归一化 | ✅ | `onebot11.message`（含消息段解析：text/image/at/at_all/face/reply）与 `onebot11.notice`（`notice.member_increase` / `notice.member_decrease`）已落地；CQ code 解析与 JSON 消息数组解析均已实现 |
| 三种出站 action | ✅ | `message.send`、`message.reply`、`message.send_image` 均已支持请求构造、`echo` 配对与窄成功/失败观察 |
| 内部 OneBot API 调用 | ✅ | `get_login_info`、`get_group_member_info`、`get_group_info`、`get_stranger_info` 的 echo-based API 调用与 identity cache（TTL 缓存）已落地 |
| actor/target 上下文补全 | ✅ | 事件归一化自动从 sender 对象提取 nickname/card/role 回填 `ActorNickname`、`ActorRole`；group name 回填 `TargetName` |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| `media / richer API action` | ❌ | 文件发送、媒体发送与更广动作族仍未实现 |
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
| adapter -> runtime read-only bridge | ✅ | 已支持最小只读事件投递 |
| 三种 action bridge | ✅ | `message.send`、`message.reply`、`message.send_image` 均已支持动作路径、映射到 adapter `send_msg` 执行链路、`result` 与 `error` 回收 |
| lazy-start first valid plugin | ✅ | 首个可投递事件到达时可 lazy-start 单个有效插件 |
| bridge/runtime summary state | ✅ | 内存计数与最近摘要状态已落地 |
| `ping` / `pong` | ✅ | 已进入 formal contract、fixtures 与 runtime 实现，含超时停止与协议违规检测 |
| supervisor / crash-backoff / dead_letter | ✅ | `crashed` / `backoff` / `dead_letter` 状态流转、指数退避重启与最大重试次数后进入 `dead_letter`；配置消费 `crash_backoff_initial_seconds` / `crash_backoff_max_seconds` |
| 用户主动 reload | ✅ | `POST /api/plugins/{plugin_id}/reload` 停止当前 runtime 后重新启动，desired_state 保持 enabled 不变 |
| 多插件调度 / fan-out | ✅ | `internal/dispatch` EventBus fan-out 分发引擎已落地：per-plugin 异步队列（bounded channel）、subscription 过滤、并发上限与丢弃策略 |
| Command Parser / routing | ✅ | `internal/command` 命令前缀解析器已落地：longest-prefix-first 匹配、command/args 提取、directed delivery 定向投递 |
| 不停机热重载 | ✅ | `dispatch.ReloadPlugin` start-before-stop 零间隙热重载已落地：新进程 init_ack 成功后原子切换注册，旧进程随后停止 |
| 官方 SDK 便利层 | ✅ | `sdk/python/rayleabot` 与 `sdk/nodejs/@rayleabot/sdk` 已落地：JSONL 协议、on_event/on_command 注册、action 便利方法 |
| 官方内置插件与示例插件体系 | ✅ | `plugins/builtin/help`、`examples/plugins/echo-python`、`examples/plugins/notice-logger` 已落地 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 更广 adapter 出站 action 执行 | ❌ | 当前仅 `message.send`、`message.reply` 与 `message.send_image`；更广插件侧动作族仍未落地 |
| 完整权限授予状态机 | 🟡 | per-plugin grants storage、GrantRepository、lifecycle 集成、管理 HTTP 端点、grants scope validation 与升级 re-grant 检测均已落地；temporal grants 仍未实现 |

---

## 八、Phase 6 — Config / Storage / Security 🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| YAML parsing + schema validation | ✅ | 配置文件读取、解析与启动前严格 schema 校验 |
| 既有配置消费 | ✅ | 当前最小 server、adapter、runtime 消费已有 `onebot.*` / server 配置字段 |
| 启动前失败阻断 | ✅ | 配置校验失败时阻止进入正常运行态 |
| Auth 全链路 | ✅ | `auth.Manager`（HMAC-SHA256 签名、TTL、sliding renewal、max sessions）、Bootstrap & Login（SHA256 摘要 + 常量时间比较）、Token Validate（格式校验、签名校验、过期检查、自动续期） |
| SQLite 存储层 | ✅ | `modernc.org/sqlite` 纯 Go 驱动、WAL 模式、read/write handle 分离、`busy_timeout`、`foreign_keys = ON`、自动创建父目录 |
| Migration runner | ✅ | `schema_migrations` 版本表、嵌入式 `embed.FS` 迁移文件、SHA256 checksum 校验、事务性逐条应用 |
| Auth persistence | ✅ | `0001_auth_core.sql` 迁移、`Repository` 接口与 `SQLiteRepository` 实现、Bootstrap/Session 持久化、跨重启恢复与验证 |
| Plugin desired_state persistence | ✅ | `0002_plugin_instances.sql` 迁移、`LoadDesiredStates` / `SaveDesiredState` / `DeleteDesiredState` 实现、启动 hydration 与跨重启验证 |
| Plugin packages 元数据持久化 | ✅ | `PackageRepository` 接口、`SavePackageMetadata` / `DeletePackageMetadata` 的 SQLite upsert/delete 实现；install/uninstall 执行链集成 |
| App 集成 — Storage + Auth | ✅ | 启动时解析 `database.path`、打开 `storage.Store`、构建 `auth.SQLiteRepository` 并注入 `auth.Manager`；关闭时按序释放 |
| Database config consumption | ✅ | `DatabaseConfig{Engine, Path}` 与启动日志输出 |
| Grants storage | ✅ | `0005_plugin_grants.sql` 迁移（`plugin_id` + `capability` 复合主键）、`GrantRepository` 接口与 CRUD 实现 |
| Grants scope 持久化 | ✅ | `0006_plugin_grants_scope.sql`（`scope_json` 列）、授予 capability 时自动从 manifest `permissions.scopes` 构建 scope JSON |
| Grants lifecycle 集成 | ✅ | `grantedCapabilities()` 合并 `auto_grant_capabilities` 与 per-plugin 显式 grants；Enable、startPluginAsync、reconcileRuntime 均消费合并后的授权列表 |
| CLI 子命令框架 | ✅ | `internal/cli/` 包与 `main.go` 子命令分发；`reset-admin`、`doctor`、`cleanup`、`migrate`、`backup`、`restore` 6 条子命令均已实现 |
| Secret store | ✅ | `0008_secret_store.sql` 迁移、`secrets.Store` 接口与 `SQLiteStore` 实现（Get/Set/Delete/List）、App 集成与启动注入 |
| Config hot reload | ✅ | `LevelController` 动态日志级别控制、`PUT /api/config` 按字段分类即时生效（`logging.level`）或标记 `restart_required`（`server.*`、`database.*`、`onebot.*` 等）、内存配置同步更新 |
| Scheduler persistence / recovery | ✅ | `0009_scheduler.sql` 迁移、`scheduler.Engine` 含 cron 解析与 tick 循环、`SQLiteRepository` 持久化、启动 hydration 与跨重启恢复、App 集成与生命周期管理 |
| 聊天侧 Permission / 黑名单 / 冷却限流 | ✅ | `0010_blacklists.sql` 迁移、`internal/permission` 包（Checker 四步检查：super_admin bypass → blacklist → command permission level → cooldown）、`BlacklistRepository` SQLite 实现、`CooldownTracker` 内存滑动窗口限流 |
| Command / cooldown 配置 | ✅ | `command.prefixes` 与 `cooldown.user_command_rate_limit` / `group_command_rate_limit` / `cooldown_reply` 已进入 `config.user.schema.json` 与 Go config struct |

---

## 九、Phase 7 — Web API & Tasks 🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| Health endpoints | ✅ | `GET /healthz`（liveness）与 `GET /readyz`（readiness + 保守状态映射） |
| Plugin query | ✅ | `GET /api/plugins`（列表）与 `GET /api/plugins/{plugin_id}`（详情） |
| Setup & Session | ✅ | `POST /api/setup/admin`（Bootstrap）、`GET /api/setup/status`、`POST /api/session/login`、`DELETE /api/session`、`POST /api/session/launcher-token` |
| System management | ✅ | `GET /api/system/status`、`POST /api/system/shutdown` |
| Config management | ✅ | `GET /api/config`（敏感字段掩码）、`PUT /api/config`（schema 校验 + 原子写盘 + `restart_required`） |
| Logs query | ✅ | `GET /api/logs`（bounded in-memory summary stream + redaction） |
| Tasks management | ✅ | `GET /api/tasks`、`GET /api/tasks/{task_id}`、`POST /api/tasks/{task_id}/cancel` |
| Plugin install | ✅ | `POST /api/plugins/install`（`local_directory` / `local_zip` / `remote_url`、manifest 校验、依赖安装、`plugin_packages` 持久化、install scripts 授权、task progress、最小取消） |
| Plugin lifecycle | ✅ | `enable` / `disable`（SQLite 持久化 + runtime 启停）、`reload`（stop + start，409 if disabled）、`DELETE`（异步 uninstall task） |
| Plugin grants 管理 | ✅ | `GET/POST /api/plugins/{plugin_id}/grants`、`DELETE /api/plugins/{plugin_id}/grants/{capability}`（含 scope validation 与升级 re-grant） |
| 4 条管理 WebSocket | ✅ | `/ws/events`（aggregate-only observability）、`/ws/tasks`（snapshot replay + live updates）、`/ws/logs`（bounded summary + live append + redaction）、`/ws/plugins/{id}/console`（ring-buffer replay + live frames） |
| HTTP 鉴权中间件 | ✅ | 统一 `RequireAuth` 中间件、`BearerAuth` 安全方案、公开/受保护路由分离、WebSocket `session_token` 查询参数兼容 |
| 共享 HTTP 基础设施 | ✅ | request_id 注入、统一 JSON error envelope、最小 panic recovery |
| 通用 task executor | ✅ | `internal/tasks/executor.go`：`Submit`/`Cancel`/`Close`、`ExecuteFunc` 签名、自动状态驱动（pending → running → succeeded/failed/cancelled） |
| 中断安装清理 | ✅ | 启动时自动扫描并清理 `.plugin-install-*` 临时目录 |
| Task 历史持久化与跨重启恢复 | ✅ | `0007_tasks.sql` 迁移、`TaskRepository` 接口与 `SQLiteRepository` 实现（SaveTask/LoadTasks/DeleteTask）、Registry `SetRepository` + `Hydrate` + 异步持久化、App 集成与启动 hydration |
| 配置热更新与字段级即时生效 | ✅ | `LevelController` 动态日志级别控制、`PUT /api/config` 按字段分类即时生效（`logging.level`）或标记 `restart_required`（`server.*`、`database.*`、`onebot.*` 等）、内存配置同步更新 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 日志历史检索 / 持久化查询 | ❌ | `GET /api/logs` 当前只查询 bounded in-memory summary stream |

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
| render contract / API surface | ❌ | formal contract 仍未进入 render service 的公开实现阶段 |
| 渲染队列与 Chromium 调度 | ❌ | 队列、并发控制、超时、重试与浏览器调度尚未实现 |
| 模板校验 / 缓存 / 结果管理 | ❌ | 模板输入校验、缓存、失败回收与产物管理尚未实现 |
| `.deps/manifest.json` baseline | 🟡 | 仅存在资源清单占位 |
| 受控运行时资源接线 | ❌ | Chromium / 运行时资源解析、下载校验与 render service 的真实接线尚未实现 |

---

## 十三、测试 & CI 现状

### CI 工作流

| 工作流 / Job | 触发 | 覆盖 |
|--------|------|------|
| `contracts.yml` / `validate-contracts` | push main / PR | 7 份 fixture-ready formal contracts 解析与结构校验、`cli-commands.yaml` 命令集与可用性矩阵校验、fixture 目录存在性、example manifests、fixture 引用可达性、web-api exact path set、websocket exact channel/event set、plugin-protocol 三种 action shape、CLI task_type 与 web-api TaskType enum 交叉校验 |
| `lint.yml` / `baseline` | push main / PR | baseline 版本锁定校验（Go、Node、pnpm、.NET、Avalonia）、必要目录与文件存在性、`.deps/manifest.json` baseline 资源项校验 |
| `lint.yml` / `server-smoke` | push main / PR | server `go test ./...` 与 `go build ./cmd/raylea-server` |

### Server 测试覆盖

根级集成测试：`config_fixture_test`、`example_manifests_test`、`http_health_test`、`plugin_discovery_test`、`plugin_http_test`、`tasks_test`、`setup_admin_test`、`session_login_test`、`auth_surface_test`、`auth_middleware_test`、`management_http_test`、`config_http_test`、`logs_http_test`、`tasks_http_test`、`events_ws_test`、`tasks_ws_test`、`logs_ws_test`、`console_ws_test`、`auth_persistence_test`、`plugin_persistence_test`、`plugin_install_flow_test`。

内部包级测试：

- `internal/adapter/`：backoff、shell、intake — 退避算法、连接状态机、帧分类、三种出站 request-response
- `internal/auth/`：manager、persistence — token 签发/校验/过期、sliding renewal、session 上限、Bootstrap 幂等、跨重启恢复
- `internal/bridge/`：bridge — 事件投递、三种 action 映射、outcome 统计、observability 订阅
- `internal/runtime/`：manager、backoff、console、spec — 子进程生命周期、ping/pong、三种 action 路径、crash 检测与回调、crash count 累积、状态流转、指数退避、console capture/redaction/rate limiting、spec 校验
- `internal/logging/`：stream — 结构化日志敏感字面值掩码
- `internal/plugins/`：catalog、http、repository、install — desired_state 更新、并发安全、install/enable/disable handler、grants scope validation、SQLite repository、local-source install 执行链
- `internal/tasks/`：tasks、executor、repository — task_id 唯一性与格式校验、executor submit/fail/cancel/close、SQLite repository CRUD 与 upsert、Registry hydrate 跨重启恢复
- `internal/secrets/`：secrets — Get/Set/Delete/List、upsert 覆盖、ErrNotFound 语义、key 排序
- `internal/scheduler/`：scheduler、cron、repository — SQLite repository CRUD 与 DeleteByPlugin、5-field cron 解析（步长/范围/列表/通配）、Engine register/hydrate/unregister、tick 触发与 TriggerFunc 回调
- `internal/cli/`：cli — backup/restore 归档创建与恢复、manifest 校验、路径遍历防护
- `internal/storage/`：store — SQLite 打开、WAL pragma、handle 分离、migration 幂等性、表结构验证（含 tasks、secret_store、scheduler_jobs、blacklist_entries）
- `internal/command/`：parser — 单前缀/多前缀匹配、longest-prefix-first、Unicode 前缀、空文本/仅前缀边界
- `internal/dispatch/`：dispatch — 多插件 fan-out、directed command delivery、alias 匹配、subscription 过滤、queue overflow/drop、deregister、action 执行、zero-gap reload
- `internal/permission/`：checker — super_admin bypass、blacklist user/group、permission level 判定、cooldown 滑动窗口限流

### 总体状况

- fixture / golden 回归已覆盖 config、web-api、websocket、plugin-info、plugin-protocol、release-manifest。
- web / launcher 仍主要停留在 baseline scaffold，尚无真实功能测试面。

---

## 十四、下一步行动建议

server 端核心管理面、存储层、CLI 工具链、调度器、事件模型与插件生态的基础能力均已落地。adapter → bridge → runtime 全链路已扩展至广义事件归一化、多插件 fan-out、command routing、聊天侧权限/黑名单/冷却限流、官方 SDK 与内置插件体系。以下路线聚焦当前仍未完成的工作，按"收尾 server 侧缺口 → 产品化外层"排列。

### 1. Server 侧收尾

1. **日志持久化查询**
   - `GET /api/logs` 当前只查询 bounded in-memory summary stream。
   - 持久化日志存储与历史检索可在 Web UI logs viewer 需求明确后一并推进，也可独立先行落地 SQLite 或文件级日志归档。

2. **temporal grants**
   - per-plugin grants 的 storage、scope validation、lifecycle 集成与管理端点均已落地，仅 temporal grants（有效期限制）仍未实现。
   - 优先级较低，可在 v0.1 后续迭代中补齐。

3. **更广 adapter 出站 action 与多 adapter 抽象**
   - 当前仅冻结 `message.send`、`message.reply`、`message.send_image` 三种出站 action。
   - 文件发送、媒体发送等更广动作族需先在 `contracts/plugin-protocol.schema.json` 中 formalize 新 action 种类，再落地 adapter 与 bridge 实现。
   - 多 adapter / 多 bot 抽象属于架构层扩展，建议在 v0.1 核心闭环稳定后评估。

4. **CLI 契约落地与 fixture 覆盖**
   - `cli-commands.yaml` 6 条子命令的实现代码已存在，但 contract 中仍标记为 TODO。
   - 需补齐 CLI 契约的正式冻结、CLI 专用 fixture 与 golden case，以及 CLI 与 HTTP task 模型的共享执行路径验证。

### 2. Web UI（Phase 8）

管理 API、任务面、config/logs 查询面与 session lifecycle 均已稳定，具备进入真实前端开发的条件。

建议起步路径：
1. auth shell — 登录页、session 管理、受保护路由守卫
2. dashboard — system status + adapter status 概览
3. plugin list/detail — 插件列表、详情、enable/disable/reload/uninstall 操作
4. task list — 任务列表、详情、取消操作
5. logs viewer — 日志流实时查看（WebSocket `/ws/logs`）
6. config editor — 配置查看与编辑（`GET/PUT /api/config`）

### 3. Launcher（Phase 9）

`launcher-token`、`system/status`、`system/shutdown` 的 contract + fixtures 均已就位。

建议起步路径：
1. 环境检查 — `.deps/` 完整性、运行时资源存在性
2. server 启停 — 子进程管理与健康检查
3. 打开 Web UI — 默认浏览器跳转
4. 最小托盘行为 — 系统托盘图标与基础菜单

### 4. Render Service（Phase 10）

render queue、browser scheduling、cache、模板输入校验与 render contract 仍未进入实现。依赖 `.deps/manifest.json` 的来源与哈希字段补全、Chromium 资源准备策略确定。

### 5. 基线收尾

- `.deps/manifest.json` 的来源 URL 与 SHA256 哈希字段仍为占位，需在 Render Service 或 Launcher 环境检查推进前补全。
- CLI / 本地运维体验：停服窗口检测、诊断输出格式和与 Web/Launcher 的共享后端路径仍需后续推进。