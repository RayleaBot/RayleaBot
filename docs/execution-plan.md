# RayleaBot v0.1 执行计划

> 本文档根据 `docs/RayleaBot机器人项目规划.md`、`docs/engineering/implementation-order.md` 与当前仓库实际落地情况整理。
>
> 本文档在 `docs/engineering/implementation-order.md` 的 10 个顶层阶段之外，额外增加一个 `Pre-Phase / Foundation`，用于记录治理、基线与 CI 骨架。`Phase 1` 到 `Phase 10` 与 `implementation-order` 保持一一对应。
>
> 状态图例：✅ 已完成 · 🟡 进行中 · ❌ 未开始

---

## 一、总览

| 阶段 | 名称 | 状态 | 当前落地摘要 |
|------|------|------|--------------|
| Pre-Phase | Foundation / 基线 / 仓库治理 / CI 骨架 | 🟡 | baseline、治理规则、repo-local skills、CI skeleton 已落库；`.deps/manifest.json` 仍是资源占位清单 |
| Phase 1 | 契约文件补全 | ✅ | 8 份 formal contracts 已全部进入 fixture-ready，并受 CI 引用与覆盖校验 |
| Phase 2 | Fixtures / Golden Cases | ✅ | config、web-api、websocket、plugin-info、plugin-protocol、release-manifest、CLI fixtures 已落库并进入 CI 校验 |
| Phase 3 | Server 内核骨架 | ✅ | server 入口、配置校验、日志、健康检查、SQLite、auth、tasks、plugin discovery 已接入主运行链路 |
| Phase 4 | Adapter（OneBot11） | 🟡 | reverse WebSocket、ready gating、重连、心跳、消息/notice 归一化、`message.send` / `message.reply` 已接入主链路；更广动作族与多 adapter 仍未实现 |
| Phase 5 | Plugin Protocol Bridge | 🟡 | 多 runtime mainline、dispatch fan-out、命令路由、scheduler trigger、zero-gap reload、builtin discovery、grant expiry runtime enforcement、rich message actions、`logger.write` / `storage.kv` / `storage.file` / `http.request` local action RPC 已接入；更广动作族仍未实现 |
| Phase 6 | Config / Storage / Security | ✅ | 配置、SQLite migration、auth persistence、plugin desired_state、grants、secret store、task persistence、scheduler persistence/trigger、聊天侧 command policy、temporal grants、plugin-scoped KV persistence、plugin_data 文件区与 scoped HTTP client 已落地 |
| Phase 7 | Web API & Tasks | 🟡 | 管理 HTTP / WebSocket、plugin lifecycle、grants 管理、task 历史持久化、配置热更新、日志历史持久化查询已可用；config snapshot 与 grants surface 已补齐 command/cooldown、storage/http 和 `expires_at`；更广管理面扩展仍未开始 |
| Phase 8 | Web UI | ✅ | Web 管理面已接入 `setup/login/session`、系统状态、4 条管理 WebSocket 连接壳，以及 `plugins/tasks/logs/config` 主流程与最小 E2E |
| Phase 9 | Launcher | ❌ | .NET / Avalonia 基线已锁定，真实 Launcher 行为尚未开始 |
| Phase 10 | Render Service | ❌ | render service 与 Chromium 调度尚未开始；`.deps/manifest.json` 仅为 baseline 占位 |

### 判定口径

- “已完成”只用于当前仓库里同时存在主链路实现、测试和可回指证据的能力。
- formal contract 已存在，不等于对应产品能力已经落地。
- 资源入库、示例入库，不等于 discovery、调度、生命周期已经自动接线。

---

## 二、Pre-Phase / Foundation — 基线 / 仓库治理 / CI 骨架 🟡

| 任务项 | 状态 | 说明 |
|--------|------|------|
| 仓库目录结构 | ✅ | `contracts/`、`docs/`、`fixtures/`、`examples/`、`server/`、`web/`、`launcher/`、`.deps/` 已就位 |
| 根与局部 `AGENTS.md` | ✅ | 根、`server/`、`contracts/`、`fixtures/` 规则已落库 |
| repo-local skills | ✅ | `.agents/skills/phase-boundary-check`、`.agents/skills/contract-audit` 已落库 |
| `docs/engineering/baseline.md` | ✅ | 工具链版本、默认命令与工程基线已锁定 |
| `docs/engineering/implementation-order.md` | ✅ | 10 阶段实施顺序已定义 |
| `contracts/README.md` | ✅ | formal contract 范围与当前 TODO 边界已收敛 |
| Server / Web / Launcher 基线文件 | ✅ | `server/go.mod`、`web/package.json`、`launcher/global.json`、`launcher/Directory.Packages.props` 已锁定基线 |
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

## 七、Phase 5 — Plugin Protocol Bridge 🟡

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
| temporal grants runtime enforcement | ✅ | `expires_at` 已进入 grants 管理面、存储层与 runtime 启停 / reload / reconcile / crash restart 判定 |
| crash-backoff / dead_letter | ✅ | runtime crash 后的 `crashed` / `backoff` / `dead_letter` 状态流转已接入 app 生命周期 |
| SDK 与示例插件 | ✅ | Python / Node.js SDK 已补 `logger.write` / `storage.kv` / `storage.file` / `http.request` helper，`notice-logger` 与 `example-permission-scope` 已演示本地 action 调用，bundled manifests 已通过 contract 校验 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 更广插件动作族 | ❌ | 三种 action 之外的动作仍未进入正式链路 |

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
| `storage.file` formalize | ✅ | 已进入 plugin protocol、plugin_data 文件区服务、config limits、SDK、fixtures、示例与 tests |
| `http.request` formalize | ✅ | 已进入 plugin protocol、scoped HTTP client、config allowlist / timeout / retry、SDK、fixtures、示例与 tests |

---

## 九、Phase 7 — Web API & Tasks 🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| Health endpoints | ✅ | `GET /healthz` 与 `GET /readyz` |
| Setup & Session | ✅ | `setup/admin`、`setup/status`、`session/login`、`session logout`、`launcher-token` 已落地 |
| System management | ✅ | `GET /api/system/status`、`POST /api/system/shutdown` |
| Config management | ✅ | `GET /api/config`、`PUT /api/config` 已包含 `command` / `cooldown` / `storage` / `http`，并支持对应热更新 |
| Logs query | ✅ | `GET /api/logs` 与 `/ws/logs` 已提供跨重启的持久化 summary 查询与历史回放 |
| Tasks management | ✅ | `GET /api/tasks`、`GET /api/tasks/{task_id}`、`POST /api/tasks/{task_id}/cancel` |
| Plugin install | ✅ | `local_directory`、`local_zip`、`remote_url` 安装路径已进入真实路由 |
| Plugin lifecycle | ✅ | `enable` / `disable` / `reload` / `DELETE` 已接入真实路由 |
| Plugin grants 管理 | ✅ | `GET/POST/DELETE /api/plugins/{plugin_id}/grants...` 已落地，并支持可选 `expires_at` |
| 4 条管理 WebSocket | ✅ | `/ws/events`、`/ws/tasks`、`/ws/logs`、`/ws/plugins/{id}/console` 已落地 |
| HTTP 鉴权中间件 | ✅ | `RequireAuth`、公开/受保护路由分离、WebSocket `session_token` 兼容已落地 |

---

## 十、Phase 8 — Web UI ✅

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `web/package.json` 与 baseline | ✅ | Vite 8 + Vue 3 + Element Plus + Vue Router + Pinia + TypeScript 工程已落地 |
| auth/session shell | ✅ | `setup/login/session`、路由守卫、`sessionStorage` token 与未授权回退已落地 |
| 真实页面与布局 | ✅ | 受保护布局壳、状态页、插件页、任务页、日志页、配置页已落地 |
| HTTP / WebSocket 消费 | ✅ | 已消费 `setup/status`、`setup/admin`、`session/login`、`config`、`system/status`、`plugins`、`tasks`、`logs` 与 4 条管理 WebSocket |
| 运维交互流 | ✅ | 插件 lifecycle、任务详情/取消、日志查询/追加、配置保存与 `restart_required` 提示已接入 |
| 最小前端测试 | ✅ | Vitest 单测与 fixture-backed Playwright E2E 已落地 |

---

## 十一、Phase 9 — Launcher ❌

| 任务项 | 状态 | 说明 |
|--------|------|------|
| .NET / Avalonia 基线 | ✅ | 版本与包基线已锁定 |
| 环境检查 / 本机诊断壳 | ❌ | 资源存在性检查与诊断入口尚未开始 |
| 真实 Launcher 行为 | ❌ | 启停、打开 Web UI、最小托盘/窗口行为尚未开始 |
| 与 server 管理面联动 | ❌ | 尚未接入 `launcher-token`、`system/status`、`system/shutdown` 等最小联动面 |
| 发布与安装体验 | ❌ | 安装、升级、版本检查与分发体验尚未开始 |

---

## 十二、Phase 10 — Render Service ❌

| 任务项 | 状态 | 说明 |
|--------|------|------|
| render contract / API surface | ❌ | render service 仍未进入公开实现阶段 |
| 渲染队列与 Chromium 调度 | ❌ | 队列、并发控制、超时、重试与浏览器调度尚未实现 |
| 模板校验 / 缓存 / 结果管理 | ❌ | 模板输入校验、缓存、失败回收与产物管理尚未实现 |
| `.deps/manifest.json` baseline | 🟡 | 仅存在资源清单占位 |
| 受控运行时资源接线 | ❌ | Chromium / 运行时资源解析、下载校验与 render service 的真实接线尚未实现 |

---

## 十三、测试 & CI 现状

### CI 工作流

| 工作流 / Job | 触发 | 覆盖 |
|--------|------|------|
| `contracts.yml` / `validate-contracts` | push main / PR | formal contracts、fixture 引用、example manifests、OpenAPI frozen path set、WebSocket frozen event set、plugin-protocol action shape、CLI fixtures 结构/覆盖校验、CLI contract 与 TaskType enum 交叉校验 |
| `lint.yml` / `baseline` | push main / PR | baseline 版本锁定、必要目录与文件存在性、`.deps/manifest.json` baseline 校验 |
| `lint.yml` / `server-smoke` | push main / PR | `go test ./...` 与 `go build ./cmd/raylea-server` |

### 当前验证结论

- `go test ./...` 当前通过。
- `go build ./cmd/raylea-server` 当前通过。
- `pnpm build`、`pnpm test`、`pnpm test:e2e` 已在 `web/` 本地通过。
- bundled plugin manifests 当前已与 `contracts/plugin-info.schema.json` 对齐。
- 根包 discovery 测试当前覆盖 `echo-python`、`hello-node`、`hello-python`、`notice-logger`。
- `raylea.help` builtin plugin 已进入默认 discovery，并受安装/卸载边界测试覆盖。
- 聊天侧 command policy 与 temporal grants 当前已受 app / plugins / storage / http tests 覆盖。
- rich message contract、runtime parser、dispatch / bridge sender、OneBot11 adapter 映射与 reply fallback 当前已受 tests 覆盖。
- `logger.write` / `storage.kv` / `storage.file` / `http.request` 当前已受 contract fixtures、runtime parser、app executor、pluginfile / pluginhttp 单测、SDK 编译与示例 smoke 覆盖。
- 当前主要风险已转向 Web 管理面剩余能力补齐、Launcher 尚未启动，以及 Render 仍未开始带来的外层交付断层。

---

## 十四、下一步行动建议

当前 Web UI 基座、主管理流程与最小 E2E 已进入稳定闭环，下一步建议转向以下事项：

### 1. 补齐 Web 管理面剩余已冻结能力

1. 接入 `plugin install / uninstall / grants / console` 的完整管理流
2. 接入 `system/shutdown` 交互与确认流程
3. 细化错误恢复、初始加载失败与 WebSocket 重连反馈

### 2. 扩大 Web UI 回归面

1. 补齐响应式布局、移动端可读性与可访问性交互细节
2. 为异常路径补更多 Vitest / Playwright 回归
3. 补齐配置编辑、日志筛选、插件状态流转的边界场景覆盖

### 3. 在 Web 状态语义稳定后启动 Launcher 最小闭环

1. 基于 `launcher-token`、`system/status`、`system/shutdown` 完成最小启停闭环
2. 接入环境检查、打开 Web UI 与基础诊断入口
3. 保持 `render.image` 与更广运行时资源管理面后置
