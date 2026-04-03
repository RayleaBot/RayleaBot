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
| Pre-Phase | Foundation / 基线 / 仓库治理 / CI 骨架 | 🟡 | baseline、治理规则、3 个 repo-local skills、CI skeleton、`deps-manifest` formal contract 与受控运行时 bootstrap 基线已落库；repo identity TODO 仍保留 |
| Phase 1 | 契约文件补全 | ✅ | 10 份 formal contracts 已全部进入 fixture-ready，并受 CI 引用与覆盖校验 |
| Phase 2 | Fixtures / Golden Cases | ✅ | config、web-api、websocket、plugin-info、plugin-protocol、release-manifest、CLI fixtures 已落库并进入 CI 校验 |
| Phase 3 | Server 内核骨架 | ✅ | server 入口、配置校验、日志、健康检查、SQLite、auth、tasks、plugin discovery 已接入主运行链路 |
| Phase 4 | Adapter（OneBot11） | 🟡 | reverse WebSocket、`idle/ready` 语义、重连、心跳、消息/notice 归一化、`message.send` / `message.reply` 已接入主链路；更广动作族与多 adapter 仍未实现 |
| Phase 5 | Plugin Protocol Bridge | ✅ | 多 runtime mainline、dispatch fan-out、命令路由、scheduler trigger、zero-gap reload、builtin discovery、grant expiry runtime enforcement、rich message actions、`logger.write` / `storage.kv` / `config.read` / `config.write` / `storage.file` / `http.request` / `scheduler.create` / `event.expose_webhook` / `render.image` local action RPC 与 gated `event.raw_payload` 已接入；完整 Chromium Render Service 继续后置到 Phase 10 |
| Phase 6 | Config / Storage / Security | ✅ | planning-aligned canonical config、`config/default.yaml` 基线、首份 `user.yaml` bootstrap、启动安全迁移、SQLite、auth persistence、grants、secret store、task/scheduler persistence、聊天侧 command policy、temporal grants、plugin-scoped KV / file / HTTP 已落地；`/readyz`、诊断包、`doctor`、Launcher 环境检查共享 `code` / `severity` / `summary` / `remediation` 统一诊断结构 |
| Phase 7 | Web API & Tasks | ✅ | 管理 HTTP / WebSocket、plugin lifecycle、grants、task 历史持久化、配置热更新、日志历史查询、在线备份提交、诊断导出、webhook ingress、插件来源/信任/命令冲突 metadata 与 render preview / artifact 管理面已进入正式主链 |
| Phase 8 | Web UI | ✅ | Web 管理面已覆盖 `setup/login/session`、系统状态、4 条管理 WebSocket、`plugins/tasks/logs/config` 主流程，以及 plugin install / uninstall / grants / console、`system/shutdown`、在线备份、诊断导出、命令冲突提示、来源信任标识、Launcher 自动登录失败短提示、错误恢复、响应式与可访问性回归 |
| Phase 9 | Launcher | ✅ | Loopback launcher token admission、首启配置预检与 server bootstrap 承接、Electron 主进程 / preload / renderer 分层、环境检查、server 启停 / 健康轮询 / 打开管理界面、托盘关闭语义、桌面设置持久化、版本检查、Windows / Linux / macOS CI 与 release feed 联动已落地；Launcher 已收口为本地服务壳与 Web 入口，初始化 / 登录流程判断集中在 Web；凭据丢失恢复入口、正式发行包根目录入口、安装根目录派生设置模型与已有工作区复用语义已对齐 |
| Phase 10 | Render Service | ✅ | 受控 Chromium 渲染、模板资源、artifact registry、`render.preview` 任务流、管理面预览入口、任务详情图片预览与统一资源诊断已接入主链 |

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
| `.deps/manifest.json` | ✅ | Chromium、Python 与 Node.js 资源的 version / source / SHA256 / archive_format / entrypoints / platform 已固定；Python `3.12.13` 当前记录 `python-build-standalone` 便携发行物来源，Node.js `24.14.0` 记录正式平台归档 |
| CI skeleton | ✅ | `contracts.yml` 与 `lint.yml` 已落库，并实际校验 contracts、baseline、server smoke |

---

## 三、Phase 1 — 契约文件补全 ✅

当前 formal contract 已形成以下正式文件：

- `backup-manifest.schema.json`
- `config.user.schema.json`
- `deps-manifest.schema.json`
- `error-codes.yaml`
- `web-api.openapi.yaml`
- `websocket-events.yaml`
- `plugin-info.schema.json`
- `plugin-protocol.schema.json`
- `release-manifest.schema.json`
- `cli-commands.yaml`

说明：

- 10 份 formal contract 均已进入 fixture-ready。
- 当前正式 contract 以 `contracts/` 为准；规划正文、README 与实现代码只作派生说明。

---

## 四、Phase 2 — Fixtures / Golden Cases ✅

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
| 更广 future action families | ❌ | v0.1 之外的更广动作族仍未 formalize / 实现 |

---

## 八、Phase 6 — Config / Storage / Security ✅

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
| 首份 `user.yaml` 自动生成 | ✅ | server 在 `config/user.yaml` 缺失时会基于 `default.yaml` 生成首份用户配置；Launcher 负责预检提示与启动链路承接 |
| `default.yaml` + `user.yaml` 覆盖语义 | ✅ | 运行时固定按 `default.yaml` -> `user.yaml` 覆盖生成有效配置，并在保存时输出 canonical 新形状 |
| `storage.file` formalize | ✅ | 已进入 plugin protocol、plugin_data 文件区服务、config limits、SDK、fixtures、示例与 tests |
| `http.request` formalize | ✅ | 已进入 plugin protocol、scoped HTTP client、config allowlist / timeout / retry、SDK、fixtures、示例与 tests |
| `/readyz`、诊断包、`doctor`、Launcher 共享诊断结构 | ✅ | `/readyz` 同时输出 `checks` 与 `issues`，`doctor` 输出结构化 `DoctorReport`，诊断包包含 `doctor.json`，Launcher 环境检查共享 `code` / `severity` / `summary` / `remediation` 结构 |

---

## 九、Phase 7 — Web API & Tasks ✅

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| Health endpoints | ✅ | `GET /healthz` 与 `GET /readyz` |
| Setup & Session | ✅ | `setup/admin`、`setup/status`、`session/login`、`session logout`、loopback-only `launcher-token` 与 `launcher-admission` 已落地 |
| System management | ✅ | `GET /api/system/status`、`POST /api/system/shutdown` |
| Recovery / runtime task surface | ✅ | `POST /api/system/recovery/recheck` 与 `POST /api/system/runtime/bootstrap` 已进入 formal API、任务流、fixtures、examples 与 tests |
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
| 插件安装来源边界 | ✅ | 规划正文 3.9.6 已更新，与当前 OpenAPI、fixtures 与 Web 统一支持 `remote_url` 作为 v0.1 正式能力 |
| 插件 lifecycle 路由形状 | ✅ | 规划正文已更新，消除原 `PATCH` 语义，与当前 formal contract 的 `enable` / `disable` / `reload` 独立路由对齐 |

---

## 十、Phase 8 — Web UI ✅

### 已落地

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `web/package.json` 与 baseline | ✅ | Vite 8 + Vue 3 + Element Plus + Vue Router + Pinia + TypeScript 工程已落地 |
| auth/session shell | ✅ | `setup/login/session`、路由守卫、`sessionStorage` token 与未授权回退已落地 |
| 真实页面与布局 | ✅ | 受保护布局壳、状态页、插件页、任务页、日志页、配置页，以及固定侧栏、内容区内部滚动摘要视图与响应式布局已落地 |
| HTTP / WebSocket 消费 | ✅ | 已消费 `setup/status`、`setup/admin`、`session/login`、`config`、`system/status`、`plugins`、`tasks`、`logs` 与 4 条管理 WebSocket |
| 运维交互流 | ✅ | plugin install / uninstall / grants / console、插件 lifecycle、恢复摘要操作入口、任务详情/取消、日志查询/追加、shutdown 确认、配置保存与 `restart_required` 提示已接入 |
| 规划内 companion flows | ✅ | 在线备份入口、诊断导出入口、命令冲突提示、插件来源 / 信任等级标签、Launcher 自动登录失败短提示已接入 |
| 前端质量与回归 | ✅ | Vitest 单测、fixture-backed Playwright E2E、异常路径、响应式与可访问性交互回归已落地 |

---

## 十一、Phase 9 — Launcher ✅

| 任务项 | 状态 | 说明 |
|--------|------|------|
| Electron / pnpm 基线 | ✅ | Node / pnpm / Electron / Vite / Vue 启动器基线已锁定 |
| Loopback bootstrap auth | ✅ | `launcher-token`、`launcher-admission` 与 Web `?token=` 自动登录已打通，并已收口为打开 Web 时的 best-effort 增强能力 |
| 环境检查 / 本机诊断壳 | ✅ | server 可执行文件、配置文件、workdir、`LongPathsEnabled`、`.deps/manifest.json` 检查与诊断摘要已落地 |
| 真实 Launcher 行为 | ✅ | 单窗口桌面壳、启动 / 停止 / 打开管理界面 / 重试健康检查、错误输出 ring buffer 与 `logs/launcher.log` 已落地 |
| 与 server 管理面联动 | ✅ | 已接入 `healthz`、`readyz`、`setup/status`、`system/status`、`system/shutdown` 与打开 Web 时的本机自动登录增强 |
| Launcher 测试与 CI | ✅ | `pnpm test`、`pnpm build` 与 Windows / Linux / macOS `ci-launcher` job 已落地 |
| 首启配置 bootstrap | ✅ | Launcher preflight 会提示缺失配置并继续拉起服务；首份 `user.yaml` 由 server 按 `default.yaml` 基线生成 |
| 凭据丢失恢复入口 | ✅ | Launcher 偏好设置页提供"重置管理员凭据"入口，执行时停止服务、调用 `reset-admin` CLI、重启服务并打开 Web 初始化页面；coordinator、IPC、preload、renderer 全链路已接入并受测试覆盖 |
| Launcher 设计系统与布局重构 | ✅ | 左侧导航、紧凑页头、统一 tokens / card / badge / log panel patterns、状态页单主操作层级、环境问题列表化、纵向诊断工具页、紧凑关闭策略设置、托盘短文案与统一弹窗表面已落地，整体视觉已收敛为更克制的深色 Fluent 工具壳 |
| 启动前状态建模与误导性报错修复 | ✅ | preflight、进程状态与 health 已按正式服务状态枚举建模；初始化、登录和管理会话问题已从 Launcher 主界面剥离，adapter / OneBot 连接状态与启动完成语义分离；健康端口已存在但不是当前 Launcher 子进程时，主界面会显式标为“检测到现有服务” |
| 桌面交互反馈、禁用态与诊断引导 | ✅ | 全量中文文案、按钮 gating、首页问题提示条、路径复制与打开目录快捷动作、暗色对比度、文案去技术化、结构化诊断摘要与设置编辑态提示已系统化接入 |
| 托盘最小化与关闭语义 | ✅ | 托盘左键直接恢复窗口，右键使用原生菜单承载状态头、动态服务动作、日志目录与完全退出；tooltip 与菜单可用态会随运行状态和环境风险联动 |
| 关闭确认与托盘引导 | ✅ | 关闭行为已收口为 `AskEveryTime / HideToTray / ExitApplication` 三态策略；设置页、关闭确认弹窗与实际关闭路径共用同一模型，弹窗支持把本次选择设为默认行为 |
| Chromium / 模板资源完整性检查 | ✅ | Launcher preflight 已覆盖 Chromium 与模板资源完整性，并给出 remediation |
| 恢复摘要与运行时动作深链 | ✅ | Launcher 已支持恢复摘要本地 fallback、打开 Web 插件详情、触发 `recovery.recheck` / `runtime.bootstrap` 任务并直接深链到对应任务页 |
| 安装根目录派生设置模型 | ✅ | Launcher 偏好设置已收口为安装目录主模型，服务端路径、配置文件路径与运行目录默认从安装目录派生；高级覆盖仅用于排障与特殊复用场景 |
| 发布目录布局与正式发行包 | ✅ | full artifact 已统一为根目录 Launcher 入口，`windows-x64-full`、`linux-x64-full`、`macos-arm64-full`、`linux-x64-server` 的目录真相、smoke、packaged recovery drill 与用户 / release 文档已对齐 |
| 发布元数据与交付 gate | ✅ | `release_manifest.json`、`build_info.json`、`SHA256SUMS.txt`、`windows_full_smoke` / `linux_server_smoke` 与 release workflow 已接入 |
| 版本检查 | ✅ | Launcher 已通过 GitHub Releases + `release_manifest.json` 做独立版本检查与发布页跳转 |

---

## 十二、Phase 10 — Render Service ✅

| 任务项 | 状态 | 说明 |
|--------|------|------|
| render contract / API surface | ✅ | `render.image`、`POST /api/system/render/preview`、`GET /api/system/render/artifacts/{artifact_id}` 与 `render.preview` task detail shape 已进入 formal contract、fixtures、tests 与 Web 主链 |
| 最小 render artifact 输出 | ✅ | `render.image` 继续返回插件可消费的 `file:// image_path`，同时生成稳定 `artifact_id`、`mime` 与 `cache_key`，供管理面同源读取 |
| 渲染队列与 Chromium 调度 | ✅ | bounded queue、并发 worker、排队超时、执行超时、Chromium 渲染与错误映射已落地 |
| 模板校验 / 缓存 / 结果管理 | ✅ | `templates/` 已提供 `help.menu`、`status.panel`、input schema、模板版本、data hash cache key 与 artifact registry |
| Chromium 与托管运行时资源基线 | ✅ | `.deps/manifest.json` 已固定 Chromium、Python 与 Node.js 资源的 version / source / SHA256 / archive_format / entrypoints；doctor、Launcher、`/readyz`、recovery finalization 与 baseline 门禁已复用同一份清单校验受控运行时 metadata 完整性 |
| 受控运行时资源接线 | ✅ | 启动时、插件依赖安装、render 诊断、CLI `doctor`、Launcher preflight、release smoke、recovery drill 与长期自托管 smoke 已共享 `.deps/manifest.json` bootstrap 语义；受控运行时按需下载到 `cache/downloads/runtime/`，并展开到 `.deps/store/<resource-id>/<version>/` |
| 恢复摘要收敛与离线 bootstrap 动作 | ✅ | `recovery.recheck` 与 `runtime.bootstrap` 已进入 server / Web / Launcher 主链；恢复后人工处理完成后可重新检查并收敛到 `compatible`，离线或受限网络场景已给出缓存归档与预展开目录两条正式回退路径 |

---

## 十三、测试 & CI 现状

### CI 工作流

| 工作流 / Job | 触发 | 覆盖 |
|--------|------|------|
| `contracts.yml` / `validate-contracts` | push main / PR | formal contracts、fixture 引用、example manifests、OpenAPI frozen path set、WebSocket frozen event set、plugin-protocol action shape、CLI fixtures 结构/覆盖校验、CLI contract 与 TaskType enum 交叉校验 |
| `lint.yml` / `baseline` | push main / PR | baseline 版本锁定、必要目录与文件存在性、`.deps/manifest.json` v2 baseline 校验 |
| `lint.yml` / `server-smoke` | push main / PR | `go test ./...` 与 `go build ./cmd/raylea-server` |
| `lint.yml` / `ci-web` | push main / PR | `pnpm install --frozen-lockfile`、`pnpm test`、`pnpm build` |
| `lint.yml` / `smoke-pr` | push main / PR | mocked Web E2E、release helper tests、`linux-x64-full` / `linux-x64-server` packaging smoke、runtime bootstrap 前置条件校验、跨版本 packaged recovery drill（60 秒观察窗口）与 metadata verify |
| `lint.yml` / `ci-launcher` | push main / PR | Windows / Linux / macOS 上的 `pnpm test` 与 `pnpm build`；Windows / macOS full artifact 打包、smoke、runtime bootstrap 前置条件校验与跨版本 packaged recovery drill（60 秒观察窗口） |
| `release.yml` | tag push | `windows-x64-full`、`linux-x64-full`、`macos-arm64-full`、`linux-x64-server` 打包、smoke、runtime bootstrap 前置条件校验、跨版本 packaged recovery drill（300 秒观察窗口）、长期自托管 smoke、`release_manifest.json` / `SHA256SUMS.txt` 校验与发布 |
| `self-host-smoke.yml` | workflow_dispatch | 复用正式打包路径，对选定 artifact 执行长期自托管 smoke 手动回归 |

### 规划对齐缺口

| 交付门禁 | 状态 | 说明 |
|--------|------|------|
| 跨版本 upgrade / rollback drills | ✅ | `lint.yml` 与 `release.yml` 已对 4 个正式 artifact 接入跨版本 upgrade / rollback-style packaged recovery drill，并显式处理 previous-release bootstrap skip |
| 长期自托管 smoke | ✅ | `release.yml` 已对 4 个正式 artifact 接入长期自托管 smoke，`self-host-smoke.yml` 提供按 artifact 子集执行的手动回归入口 |

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
- `ci-web`、`smoke-pr`、`ci-launcher`、`release` 与 `self-host-smoke` 已进入仓库工作流，release metadata / checksum 校验、交付矩阵 smoke、runtime bootstrap 前置条件校验、跨版本 packaged recovery drill、长期自托管 smoke 与恢复摘要长周期观测已有门禁。
- 当前主要风险集中在多镜像源 / 内网镜像分发与恢复后批量处理体验层面：共享 `recovery_summary`、`recovery.recheck` 与 `runtime.bootstrap` 已覆盖 API、本地文件、diagnostics、Web、Launcher、packaged drill 与长期自托管 smoke，兼容通过 / 需要人工处理 / 修复后收敛三类路径都已进入回归矩阵。

---

## 十四、下一轮规划

### 主工作包

1. 评估多镜像源或内网镜像是否进入 `deps-manifest` 正式契约。
   当前 `.deps/manifest.json` 已固定单一正式来源、SHA256 与 entrypoints；下一轮重点是判断镜像 URL、镜像优先级或企业内网分发信息是否需要进入 formal contract，而不是继续停留在脚本或部署约定层。

2. 评估恢复后批量处理与审计追踪是否进入正式任务面。
   当前恢复闭环已覆盖人工处理建议、再次检查、运行时准备和单插件深链；下一轮重点是判断批量处理、处理结果审计与 operator 级确认是否需要进入共享任务模型。

### 下一轮边界

- 不在下一轮回头扩张第二套跨版本恢复状态语义或发布元数据口径。
- 不在下一轮把镜像策略提前写成平行配置入口。
- 不在下一轮推进多 adapter / 多 bot 抽象。
- 不在下一轮扩展更宽 future action families。
- 不新增平行安装入口、平行状态语义或新的独立发布 metadata。

### 下一轮验收口径

- 长时间窗 smoke、packaged recovery drill 与 diagnostics 校验必须继续复用默认构建命令、现有 artifact matrix 与现有 release metadata。
- 新增镜像或内网分发能力如需落地，必须继续复用 `.deps/manifest.json` 与既有 bootstrap 目录语义，不新增第二套运行时元数据面。
- 新增恢复批量处理能力必须继续复用共享 `recovery_summary`、现有任务模型与现有 diagnostics 投影，不新增 Web / Launcher / CLI 各自独立状态口径。

