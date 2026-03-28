# RayleaBot v0.1 执行计划

> 本文档根据 `docs/RayleaBot机器人项目规划.md`、`docs/engineering/implementation-order.md` 与当前仓库实际落地情况整理。
>
> 本文档在 `docs/engineering/implementation-order.md` 的 10 个顶层阶段之外，额外增加一个 `Pre-Phase / Foundation`，用于记录治理、基线与 CI 骨架。`Phase 1` 到 `Phase 10` 与 `implementation-order` 保持一一对应。
>
> 状态图例：✅ 已完成 · 🟡 进行中 / 部分落地 · ❌ 未开始 · 🔷 超前完成 · ⚠️ 口径漂移

---

## 一、总览

| 阶段 | 名称 | 状态 | 当前落地摘要 |
|------|------|------|--------------|
| Pre-Phase | Foundation / 基线 / 仓库治理 / CI 骨架 | 🟡 | baseline、治理规则、3 个 repo-local skills、CI skeleton 已落库；`.deps/manifest.json` 仍是资源占位清单 |
| Phase 1 | 契约文件补全 | ✅ | 8 份 formal contracts 已全部进入 fixture-ready，并受 CI 引用与覆盖校验 |
| Phase 2 | Fixtures / Golden Cases | ✅ | config、web-api、websocket、plugin-info、plugin-protocol、release-manifest、CLI fixtures 已落库并进入 CI 校验 |
| Phase 3 | Server 内核骨架 | ✅ | server 入口、配置校验、日志、健康检查、SQLite、auth、tasks、plugin discovery 已接入主运行链路 |
| Phase 4 | Adapter（OneBot11） | 🟡 | reverse WebSocket、ready gating、重连、心跳、消息/notice 归一化、`message.send` / `message.reply` 已接入主链路；更广动作族与多 adapter 仍未实现 |
| Phase 5 | Plugin Protocol Bridge | ✅ | 多 runtime mainline、dispatch fan-out、命令路由、scheduler trigger、zero-gap reload、builtin discovery、grant expiry runtime enforcement、rich message actions、`logger.write` / `storage.kv` / `config.read` / `config.write` / `storage.file` / `http.request` / `scheduler.create` / `event.expose_webhook` / `render.image` local action RPC 与 gated `event.raw_payload` 已接入；完整 Chromium Render Service 继续后置到 Phase 10 |
| Phase 6 | Config / Storage / Security | 🟡 | planning-aligned canonical config、`config/default.yaml` 基线、首份 `user.yaml` bootstrap、启动安全迁移、SQLite、auth persistence、grants、secret store、task/scheduler persistence、聊天侧 command policy、temporal grants、plugin-scoped KV / file / HTTP 已落地；共享 degraded / remediation 结构仍未完全统一到全部入口 |
| Phase 7 | Web API & Tasks | 🟡 | 管理 HTTP / WebSocket、plugin lifecycle、grants、task 历史持久化、配置热更新、日志历史查询、在线备份提交、诊断导出、webhook ingress 与插件来源/信任/命令冲突 metadata 已可用；插件安装来源和 lifecycle 路由形状与规划正文仍有口径待收口 |
| Phase 8 | Web UI | ✅ | Web 管理面已覆盖 `setup/login/session`、系统状态、4 条管理 WebSocket、`plugins/tasks/logs/config` 主流程，以及 plugin install / uninstall / grants / console、`system/shutdown`、在线备份、诊断导出、命令冲突提示、来源信任标识、Launcher 自动登录失败短提示、错误恢复、响应式与可访问性回归 |
| Phase 9 | Launcher | 🟡 | Loopback launcher token admission、首启配置 bootstrap、Electron 主进程 / preload / renderer 分层、环境检查、server 启停 / 健康轮询 / 打开管理界面、托盘关闭语义、桌面设置持久化、版本检查、Windows / Linux / macOS CI 与 release feed 联动已落地；Launcher 已收口为本地服务壳与 Web 入口，初始化 / 登录流程判断集中在 Web；凭据丢失恢复入口与正式安装体验仍待收口 |
| Phase 10 | Render Service | 🟡 | `render.image` 最小占位渲染、产物输出与资源检查已接线；受控 Chromium 队列、模板版本 / 缓存、preview 与正式 Render Service 调度仍未完成 |

### 判定口径

- “已完成”只用于当前仓库里同时存在主链路实现、测试和可回指证据的能力。
- “部分落地”用于已有主干，但仍未覆盖规划正文全部要求的能力。
- “超前完成”只用于超出 v0.1 但在规划文档后续阶段已明确存在、因此无需回退的能力；当前高置信复核结果里，主差异仍以“部分落地 / 未完成 / 口径漂移”为主。
- “口径漂移”用于规划正文、formal contract 与当前实现存在边界不一致，且暂不能直接视为后续规划前置落地的能力。
- formal contract 已存在，不等于对应产品能力已经落地。
- 资源入库、示例入库，不等于 discovery、调度、生命周期已经自动接线。

---

## 二、Pre-Phase / Foundation — 基线 / 仓库治理 / CI 骨架 🟡

| 任务项 | 状态 | 说明 |
|--------|------|------|
| 仓库目录结构 | ✅ | `contracts/`、`docs/`、`fixtures/`、`examples/`、`server/`、`web/`、`launcher/`、`.deps/` 已就位 |
| 根与局部 `AGENTS.md` | ✅ | 根、`server/`、`contracts/`、`fixtures/` 规则已落库 |
| repo-local skills | ✅ | `.agents/skills/phase-boundary-check`、`.agents/skills/contract-audit`、`.agents/skills/editing-final-state-content` 已落库 |
| `docs/engineering/baseline.md` | ✅ | 工具链版本、默认命令与工程基线已锁定 |
| `docs/engineering/implementation-order.md` | ✅ | 10 阶段实施顺序已定义 |
| `contracts/README.md` | ✅ | formal contract 范围与当前 TODO 边界已收敛 |
| Server / Web / Launcher 基线文件 | ✅ | `server/go.mod`、`web/package.json`、`launcher/package.json`、`launcher/pnpm-lock.yaml` 已锁定基线 |
| `.deps/manifest.json` | 🟡 | 资源 ID 与版本线已存在，来源与 SHA256 仍待补齐 |
| CI skeleton | ✅ | `contracts.yml` 与 `lint.yml` 已落库，并实际校验 contracts、baseline、server smoke |

---

## 三、Phase 1 — 契约文件补全 ✅

当前 formal contract 已形成以下正式文件：

- `config.user.schema.json`
- `error-codes.yaml`
- `web-api.openapi.yaml`
- `websocket-events.yaml`
- `plugin-info.schema.json`
- `plugin-protocol.schema.json`
- `release-manifest.schema.json`
- `cli-commands.yaml`

说明：

- 8 份 formal contract 均已进入 fixture-ready。
- 当前正式 contract 以 `contracts/` 为准，不再从规划正文、README 或实现代码反向推断接口。

---

## 四、Phase 2 — Fixtures / Golden Cases 🟡

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `fixtures/config` | ✅ | `ok` / `invalid` / `edge` 配置样例已落库 |
| `fixtures/web-api` | ✅ | health、ready、setup、session、config、logs、tasks、plugin lifecycle 相关样例已落库 |
| `fixtures/websocket` | ✅ | tasks/logs/events/console 的管理 WebSocket 样例已落库 |
| `fixtures/plugin-info` | ✅ | plugin manifest 的正反与边界样例已落库 |
| `fixtures/plugin-protocol` | ✅ | init / progress / ack / ping / pong / action / result / error 样例已落库 |
| `fixtures/release-manifest` | ✅ | release manifest 的正反与边界样例已落库 |
| Golden 命名与结构 | ✅ | `ok` / `invalid` / `edge` 命名与目录约束已落库 |
| CLI fixtures / golden cases | ✅ | 6 条正式 CLI 命令均已配套 `ok` / `invalid` / `edge` fixtures，并进入 CI 最小覆盖校验 |

---

## 五、Phase 3 — Server 内核骨架 ✅

| 任务项 | 状态 | 说明 |
|--------|------|------|
| server 最小入口 | ✅ | `cmd/raylea-server` 与 `-config` / `-config-schema` flags 已落地 |
| 配置加载与 schema 校验 | ✅ | 启动前读取 YAML 并消费 `contracts/config.user.schema.json` |
| 统一日志基线 | ✅ | `slog` 与日志 summary stream 已接入 |
| `GET /healthz` | ✅ | 基础进程存活检查已实现 |
| `GET /readyz` | ✅ | readiness 与保守 adapter 状态映射已实现 |
| SQLite foundation | ✅ | WAL、migration runner、读写句柄分离、自动建库已落地 |
| Auth / Task / Plugin 基础装配 | ✅ | auth、tasks、plugin catalog、storage、secret store 已随 app 启动装配 |
| plugin discovery | ✅ | 当前扫描 `plugins/builtin`、`examples/plugins` 与 `plugins/installed` |

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
| 广义事件归一化 | ✅ | `message.group` / `message.private` 与 `notice.member_increase` / `notice.member_decrease` 已进入 bridge 可消费形态 |
| rich message actions | ✅ | `message.send`、`message.reply` 已支持 shared `message.segments` |
| 内部 OneBot API 调用 | ✅ | `get_login_info`、`get_group_member_info`、`get_group_info`、`get_stranger_info` 与 identity cache 已落地 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 更广 media / richer API action | ❌ | 文件发送与更广动作族仍未 formalize / 实现 |
| 多 adapter / 多 bot 抽象 | ❌ | 当前仍为单协议、单实例、单 adapter |

---

## 七、Phase 5 — Plugin Protocol Bridge ✅

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| runtime spec creation | ✅ | 已可从有效 discovered plugin 构建 runtime spec |
| subprocess spawn | ✅ | 最小子进程拉起已落地 |
| `init -> init_ack` | ✅ | 最小启动握手已打通，并保留 `init_ack.subscriptions` |
| `shutdown(stop)` | ✅ | 最小优雅停止路径已实现 |
| `ping` / `pong` | ✅ | keepalive 已进入 formal contract、fixtures 与 runtime 实现 |
| 多 runtime 主链路 | ✅ | `app` 已切换到每插件一个 `runtime.Manager` 的编排方式 |
| dispatcher fan-out / directed delivery | ✅ | adapter 事件已通过 dispatcher 进入订阅 fan-out 与命令定向投递 |
| Command Parser / routing | ✅ | `internal/command` 已接入主链路，消息事件会附带 `command` / `args` payload |
| scheduler trigger | ✅ | scheduler job 已按 `plugin_id` 直投 `scheduler.trigger` 到目标插件 |
| zero-gap reload | ✅ | reload 已走 start-before-stop 的 dispatcher swap 语义 |
| builtin discovery / lifecycle | ✅ | `plugins/builtin` 已纳入默认 discovery roots，默认 `desired_state=enabled`，支持 enable / disable / reload，拒绝卸载 |
| rich message action bridge | ✅ | `message.send`、`message.reply` 已按 shared `message.segments` 进入 runtime / dispatch / adapter 主链 |
| local action RPC | ✅ | runtime 事件处理中已支持 `logger.write`、`storage.kv`、`storage.file`、`http.request` 的 request/response 循环，terminal `message.*` 继续保持既有语义 |
| planning-aligned local actions | ✅ | `config.read`、`config.write`、`scheduler.create`、`event.expose_webhook`、`render.image` 已进入 formal contract、runtime parser、app executor 与 tests |
| gated `event.raw_payload` | ✅ | 仅在 manifest 声明且授予 `event.raw_payload` 后，`webhook.received` 事件才附带高敏原始载荷 |
| temporal grants runtime enforcement | ✅ | `expires_at` 已进入 grants 管理面、存储层与 runtime 启停 / reload / reconcile / crash restart 判定 |
| crash-backoff / dead_letter | ✅ | runtime crash 后的 `crashed` / `backoff` / `dead_letter` 状态流转已接入 app 生命周期 |
| SDK 与示例插件 | ✅ | Python / Node.js SDK 已补 `logger.write` / `storage.kv` / `config.read` / `config.write` / `storage.file` / `http.request` / `scheduler.create` / `event.expose_webhook` / `render.image` helper，`notice-logger`、`example-permission-scope`、`example-config-panel`、`example-render-card`、`example-scheduler`、`example-webhook` 已演示对应能力 |

### 剩余边界

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 完整 Render Service | 🟡 | `render.image` 已有最小占位产物输出，但 Chromium 调度、模板版本化、缓存、preview 与正式渲染队列仍在 Phase 10 |
| 更广 future action families | ❌ | v0.1 之外的更广动作族仍未 formalize / 实现 |

---

## 八、Phase 6 — Config / Storage / Security 🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| YAML parsing + schema validation | ✅ | 配置文件读取、解析与启动前严格 schema 校验 |
| 启动前失败阻断 | ✅ | 配置校验失败时阻止进入正常运行态 |
| Auth 全链路 | ✅ | Bootstrap、Login、签名、TTL、sliding renewal、session 上限、持久化均已落地 |
| SQLite 存储层 | ✅ | `modernc.org/sqlite`、WAL、migration runner、checksum 校验已落地 |
| Plugin desired_state persistence | ✅ | desired_state 持久化、启动 hydration 与跨重启恢复已落地 |
| Plugin packages / grants / secret store | ✅ | package metadata、grant storage、scope JSON、secret store 已落地 |
| Task 历史持久化 | ✅ | tasks repository、hydration、异步持久化已接入 app |
| Config hot reload | ✅ | `PUT /api/config` 已支持字段级即时生效与 `restart_required` |
| CLI 子命令框架 | ✅ | `reset-admin`、`backup`、`restore`、`doctor`、`migrate`、`cleanup` 均已有实现 |
| Scheduler persistence / recovery | ✅ | repository、hydration、tick loop 与 plugin runtime trigger 已进入 app |
| 聊天侧 Permission / 黑名单 / 冷却限流 | ✅ | blacklist、命令权限、cooldown 与可选 cooldown reply 已进入 live command path |
| Temporal grants | ✅ | `plugin_grants.expires_at`、生效授权过滤、enable / reload / reconcile / restart 过期判定已接入 |
| `logger.write` formalize | ✅ | 已进入 plugin protocol、runtime local action executor、SDK、fixtures、示例与 tests |
| `storage.kv` formalize | ✅ | 已进入 plugin protocol、SQLite migration / repository、config limits、SDK、fixtures、示例与 tests |
| canonical config realign | ✅ | `contracts/config.user.schema.json`、typed config 与 `/api/config` 已对齐规划正文命名；旧口径作为迁移输入保留 |
| `config/default.yaml` 默认基线 | ✅ | repo-tracked 默认模板与运行时 baseline 已落库 |
| 首份 `user.yaml` 自动生成 | ✅ | server 与 Launcher 在 `config/user.yaml` 缺失时会基于 `default.yaml` bootstrap 首份用户配置 |
| `default.yaml` + `user.yaml` 覆盖语义 | ✅ | 运行时固定按 `default.yaml` -> `user.yaml` 覆盖生成有效配置，并在保存时输出 canonical 新形状 |
| `storage.file` formalize | ✅ | 已进入 plugin protocol、plugin_data 文件区服务、config limits、SDK、fixtures、示例与 tests |
| `http.request` formalize | ✅ | 已进入 plugin protocol、scoped HTTP client、config allowlist / timeout / retry、SDK、fixtures、示例与 tests |

### 规划对齐缺口

| 子任务 | 状态 | 说明 |
|--------|------|------|
| Web / Launcher / `doctor` / 诊断包共享降级口径 | 🟡 | `/readyz` 的 `reason_codes` / `checks`、Launcher remediation 与诊断导出已存在，但统一 `code / severity / summary / remediation` 结构尚未完全推广到全部入口 |

---

## 九、Phase 7 — Web API & Tasks 🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| Health endpoints | ✅ | `GET /healthz` 与 `GET /readyz` |
| Setup & Session | ✅ | `setup/admin`、`setup/status`、`session/login`、`session logout`、loopback-only `launcher-token` 与 `launcher-admission` 已落地 |
| System management | ✅ | `GET /api/system/status`、`POST /api/system/shutdown` |
| Config management | ✅ | `GET /api/config`、`PUT /api/config` 已包含 `command` / `cooldown` / `storage` / `http`，并支持对应热更新 |
| Logs query | ✅ | `GET /api/logs` 与 `/ws/logs` 已提供跨重启的持久化 summary 查询与历史回放 |
| Tasks management | ✅ | `GET /api/tasks`、`GET /api/tasks/{task_id}`、`POST /api/tasks/{task_id}/cancel` |
| Plugin install | ✅ | `local_directory`、`local_zip`、`remote_url` 安装路径已进入真实路由 |
| Plugin lifecycle | ✅ | `enable` / `disable` / `reload` / `DELETE` 已接入真实路由 |
| Plugin grants 管理 | ✅ | `GET/POST/DELETE /api/plugins/{plugin_id}/grants...` 已落地，并支持可选 `expires_at` |
| System backup / diagnostics | ✅ | `POST /api/system/backup` 与 `GET /api/system/diagnostics/export` 已进入 formal API、任务流与 tests |
| Webhook ingress | ✅ | `POST /api/webhooks/{plugin_id}/{route}` 已进入 formal contract 与主链路 |
| Plugin metadata surface | ✅ | 插件 list/detail 已暴露 `name`、`role`、`source`、`trust` 与 `command_conflicts`，足以支撑 Web 管理面展示 |
| 4 条管理 WebSocket | ✅ | `/ws/events`、`/ws/tasks`、`/ws/logs`、`/ws/plugins/{id}/console` 已落地 |
| HTTP 鉴权中间件 | ✅ | `RequireAuth`、公开/受保护路由分离、WebSocket `session_token` 兼容已落地 |

### 规划对齐与口径收口

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 插件安装来源边界 | ✅ | 规划正文 3.9.6 已更新，与当前 OpenAPI、fixtures 与 Web 统一支持 `remote_url` 作为 v0.1 正式能力 |
| 插件 lifecycle 路由形状 | ✅ | 规划正文已更新，消除原 `PATCH` 语义，与当前 formal contract 的 `enable` / `disable` / `reload` 独立路由对齐 |

---

## 十、Phase 8 — Web UI ✅

### 已落地

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `web/package.json` 与 baseline | ✅ | Vite 8 + Vue 3 + Element Plus + Vue Router + Pinia + TypeScript 工程已落地 |
| auth/session shell | ✅ | `setup/login/session`、路由守卫、`sessionStorage` token 与未授权回退已落地 |
| 真实页面与布局 | ✅ | 受保护布局壳、状态页、插件页、任务页、日志页、配置页，以及移动端导航抽屉和卡片化布局已落地 |
| HTTP / WebSocket 消费 | ✅ | 已消费 `setup/status`、`setup/admin`、`session/login`、`config`、`system/status`、`plugins`、`tasks`、`logs` 与 4 条管理 WebSocket |
| 运维交互流 | ✅ | plugin install / uninstall / grants / console、插件 lifecycle、任务详情/取消、日志查询/追加、shutdown 确认、配置保存与 `restart_required` 提示已接入 |
| 规划内 companion flows | ✅ | 在线备份入口、诊断导出入口、命令冲突提示、插件来源 / 信任等级标签、Launcher 自动登录失败短提示已接入 |
| 前端质量与回归 | ✅ | Vitest 单测、fixture-backed Playwright E2E、异常路径、响应式与可访问性交互回归已落地 |

---

## 十一、Phase 9 — Launcher 🟡

| 任务项 | 状态 | 说明 |
|--------|------|------|
| Electron / pnpm 基线 | ✅ | Node / pnpm / Electron / Vite / Vue 启动器基线已锁定 |
| Loopback bootstrap auth | ✅ | `launcher-token`、`launcher-admission` 与 Web `?token=` 自动登录已打通，并已收口为打开 Web 时的 best-effort 增强能力 |
| 环境检查 / 本机诊断壳 | ✅ | server 可执行文件、配置文件、workdir、`LongPathsEnabled`、`.deps/manifest.json` 检查与诊断摘要已落地 |
| 真实 Launcher 行为 | ✅ | 单窗口桌面壳、启动 / 停止 / 打开管理界面 / 重试健康检查、错误输出 ring buffer 与 `logs/launcher.log` 已落地 |
| 与 server 管理面联动 | ✅ | 已接入 `healthz`、`readyz`、`setup/status`、`system/status`、`system/shutdown` 与打开 Web 时的本机自动登录增强 |
| Launcher 测试与 CI | ✅ | `pnpm test`、`pnpm build` 与 Windows / Linux / macOS `ci-launcher` job 已落地 |
| 首启配置 bootstrap | ✅ | Launcher preflight 与 server 启动链已对齐 `default.yaml` -> `user.yaml` bootstrap 语义 |
| 凭据丢失恢复入口 | ❌ | 规划要求停服务后可通过 Launcher 或本地 CLI 触发重置向导；当前 Launcher 仍未提供 `reset-admin` / 恢复入口 |
| Launcher 设计系统与布局重构 | ✅ | 左侧导航、紧凑页头、统一 tokens / card / badge / log panel patterns、状态页单主操作层级、环境问题列表化、纵向诊断工具页、紧凑关闭策略设置、托盘短文案与统一弹窗表面已落地，整体视觉已收敛为更克制的深色 Fluent 工具壳 |
| 启动前状态建模与误导性报错修复 | ✅ | preflight、进程状态与 health 已按 launcher-local 语义建模；初始化、登录和管理会话问题已从 Launcher 主界面剥离，adapter / OneBot 连接状态不再污染启动完成语义与主状态文案；健康端口已存在但不是当前 Launcher 子进程时，主界面会显式标为“检测到现有服务” |
| 桌面交互反馈、禁用态与诊断引导 | ✅ | 全量中文文案、按钮 gating、首页问题提示条、路径复制与打开目录快捷动作、暗色对比度、文案去技术化、结构化诊断摘要与设置编辑态提示已系统化接入 |
| 托盘最小化与关闭语义 | ✅ | 托盘左键直接恢复窗口，右键使用原生菜单承载状态头、动态服务动作、日志目录与完全退出；tooltip 与菜单可用态会随运行状态和环境风险联动 |
| 关闭确认与托盘引导 | ✅ | 关闭行为已收口为 `AskEveryTime / HideToTray / ExitApplication` 三态策略；设置页、关闭确认弹窗与实际关闭路径共用同一模型，弹窗支持把本次选择设为默认行为 |
| Chromium / 模板资源完整性检查 | ✅ | Launcher preflight 已覆盖 Chromium 与模板资源完整性，并给出 remediation |
| 发布目录布局与正式发行包 | 🟡 | packaging tooling 与 release workflow 已产出 `windows-x64-full`、`linux-x64-full`、`macos-arm64-full`、`linux-x64-server`，但正式安装体验仍需继续打磨 |
| 发布元数据与交付 gate | ✅ | `release_manifest.json`、`build_info.json`、`SHA256SUMS.txt`、`windows_full_smoke` / `linux_server_smoke` 与 release workflow 已接入 |
| 版本检查 | ✅ | Launcher 已通过 GitHub Releases + `release_manifest.json` 做独立版本检查与发布页跳转 |

### 当前主要问题

- Launcher 的主流程、首启配置、左侧导航桌面壳、自定义标题栏、第二轮液态玻璃视觉层、托盘原生菜单、版本检查和交付 metadata 已进入可验证主链；初始化、登录和自动登录失败提示已回收到 Web 侧处理，Web 登录/初始化页也已补齐可见错误反馈。
- 当前仍未收口的 Launcher 欠账主要集中在凭据丢失后的本地恢复入口，以及正式安装体验与长期自托管打磨。

---

## 十二、Phase 10 — Render Service 🟡

| 任务项 | 状态 | 说明 |
|--------|------|------|
| render contract / API surface | ✅ | `render.image` 已进入 formal contract、fixtures、SDK、examples 与主运行链 |
| 最小 render artifact 输出 | ✅ | server 已能生成占位渲染产物并把 `image_path` / `mime` / `cache_key` 返回给插件 |
| 渲染队列与 Chromium 调度 | ❌ | 队列、并发控制、超时、重试与浏览器调度尚未实现 |
| 模板校验 / 缓存 / 结果管理 | 🟡 | 最小 artifact path 已存在，但模板版本化、缓存、失败回收与产物管理仍未实现 |
| `.deps/manifest.json` baseline | 🟡 | 仅存在资源清单占位 |
| 受控运行时资源接线 | 🟡 | Launcher preflight 已覆盖 Chromium / 模板资源检查，真正的 render worker 资源接线仍未完成 |

---

## 十三、测试 & CI 现状

### CI 工作流

| 工作流 / Job | 触发 | 覆盖 |
|--------|------|------|
| `contracts.yml` / `validate-contracts` | push main / PR | formal contracts、fixture 引用、example manifests、OpenAPI frozen path set、WebSocket frozen event set、plugin-protocol action shape、CLI fixtures 结构/覆盖校验、CLI contract 与 TaskType enum 交叉校验 |
| `lint.yml` / `baseline` | push main / PR | baseline 版本锁定、必要目录与文件存在性、`.deps/manifest.json` baseline 校验 |
| `lint.yml` / `server-smoke` | push main / PR | `go test ./...` 与 `go build ./cmd/raylea-server` |
| `lint.yml` / `ci-web` | push main / PR | `pnpm install --frozen-lockfile`、`pnpm test`、`pnpm build` |
| `lint.yml` / `smoke-pr` | push main / PR | mocked Web E2E、release helper tests、linux packaging smoke 与 metadata verify |
| `lint.yml` / `ci-launcher` | push main / PR | Windows / Linux / macOS 上的 `pnpm test` 与 `pnpm build` |
| `nightly.yml` | schedule / manual | server tests、web tests / E2E、release helper tests、launcher tests / publish |
| `release.yml` | tag push | `windows-x64-full`、`linux-x64-full`、`macos-arm64-full`、`linux-x64-server` 打包、smoke、`release_manifest.json` / `SHA256SUMS.txt` 校验与发布 |

### 规划对齐缺口

| 交付门禁 | 状态 | 说明 |
|--------|------|------|
| 发布后升级 / 回滚 drills | ❌ | 规划要求交付后持续验证升级、回滚和恢复路径，当前 workflow 仍以 build/smoke 为主 |
| 长期自托管 smoke | ❌ | 规划要求更长时间窗的安装、运行、诊断闭环回归，当前 CI 仍未覆盖 |

### 当前验证结论

- `go test ./...` 当前通过。
- `go build ./cmd/raylea-server` 当前通过。
- `pnpm build`、`pnpm test`、`pnpm test:e2e` 已在 `web/` 本地通过。
- `pnpm test`、`pnpm typecheck` 与 `pnpm build` 已在 `launcher/` 本地通过。
- bundled plugin manifests 当前已与 `contracts/plugin-info.schema.json` 对齐。
- 根包 discovery 测试当前覆盖 `echo-python`、`hello-node`、`hello-python`、`notice-logger`、`example-config-panel`、`example-render-card`、`example-scheduler`、`example-webhook`。
- `raylea.help` builtin plugin 已进入默认 discovery，并受安装/卸载边界测试覆盖。
- 聊天侧 command policy 与 temporal grants 当前已受 app / plugins / storage / http tests 覆盖。
- rich message contract、runtime parser、dispatch / bridge sender、OneBot11 adapter 映射与 reply fallback 当前已受 tests 覆盖。
- `logger.write` / `storage.kv` / `config.read` / `config.write` / `storage.file` / `http.request` / `scheduler.create` / `event.expose_webhook` / `render.image` 当前已受 contract fixtures、runtime parser、app executor、SDK 编译与示例 smoke 覆盖。
- 在线备份、诊断导出、webhook ingress、插件来源 / 信任 / 命令冲突 metadata 已受 API、Web 单测 / E2E 与 management tests 覆盖。
- `ci-web`、`smoke-pr`、`nightly`、`release` 已进入仓库工作流，release metadata / checksum 校验与交付矩阵 smoke 已有门禁。
- 当前主要风险集中在四个层面：共享 degraded / remediation 结构尚未完全统一到 `/readyz`、Launcher、`doctor` 与诊断包；规划与 formal surface 仍存在 install source narrative 与 plugin lifecycle route shape 漂移；Launcher 仍缺凭据丢失恢复入口与更完整的安装体验；Render Service 仍停留在最小占位产物输出，Chromium 队列与模板 / 缓存体系尚未完成。

---

## 十四、下一步行动建议

当前执行计划中的 1-4 号主线已完成，下一步从“补主链能力”切换为“收口剩余漂移与交付稳定性”。

### 1. 已收口的规划 / contract 漂移

1. ✅ 统一插件 lifecycle 口径：已在规划正文中对齐 formal contract，明确采用 `enable` / `disable` / `reload` 独立路由语义。
2. ✅ 收口插件安装来源叙事：已在规划正文中明确 `remote_url` 属于 v0.1 正式支持能力。
3. 把“超前完成”能力与当前阶段能力的分层说明继续同步进规划相关文档，避免再次出现执行计划与规划脱节。

### 2. 扩大发布后回归与长期自托管验证

1. 增加 upgrade / rollback drills，验证 release metadata、数据库 / 配置 schema 与 launcher build info 的回滚判断链。
2. 增加 diagnostic bundle drills，验证 Web / Launcher / CLI 产出的诊断信息在支持场景下可交叉使用。
3. 增加更长时间窗的自托管 smoke，覆盖正式安装、启动、发布后升级和恢复流程。

### 3. v0.1 交付面稳定后进入 v0.2+ 运行时完善项

1. 完成真正的 Render Service：受控 Chromium、模板版本、缓存、preview 与任务编排。
2. 在 v0.1 交付边界稳定后，再推进更广运行时与平台能力，而不是继续在 v0.1 范围内补洞。
3. 保持 Web、Launcher、CLI 继续复用同一套状态语义与 release metadata，不再新增平行口径。

### 后续实施验收口径

- 规划 / contract 漂移收口的验收应满足：当前 install source narrative 与 plugin lifecycle route shape 已完成收口并在各文档中得到单一、一致解释；超前完成能力还需继续在规划中分层说明。
- 发布后回归扩面的验收应满足：upgrade / rollback、diagnostic bundle、正式安装与长期自托管 smoke 进入稳定门禁，且不会与既有 `release_manifest.json` / `build_info.json` / `SHA256SUMS.txt` 语义冲突。
- v0.2+ 运行时完善项的验收应满足：Render Service 从占位产物输出升级到真正的 Chromium 渲染链路，并继续保持 contract-first、四件套同步更新与单一状态语义。
