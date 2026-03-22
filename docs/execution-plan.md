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
| Phase 1 | 契约文件补全 | ✅ | 7 份核心 formal contracts 已 fixture-ready，`cli-commands.yaml` 也已 formalize |
| Phase 2 | Fixtures / Golden Cases | 🟡 | config、web-api、websocket、plugin-info、plugin-protocol、release-manifest fixtures 已落库；CLI fixtures 仍未补齐 |
| Phase 3 | Server 内核骨架 | ✅ | server 入口、配置校验、日志、健康检查、SQLite、auth、tasks、plugin discovery 已接入主运行链路 |
| Phase 4 | Adapter（OneBot11） | 🟡 | reverse WebSocket、ready gating、重连、心跳、消息/notice 归一化、三种出站 action 已接入主链路；更广动作族与多 adapter 仍未实现 |
| Phase 5 | Plugin Protocol Bridge | 🟡 | 单 runtime manager、bridge、reload/install/uninstall 基础链路可用；多插件 fan-out、命令路由、zero-gap reload 仍停留在库级实现 |
| Phase 6 | Config / Storage / Security | 🟡 | 配置、SQLite migration、auth persistence、plugin desired_state、grants、secret store、task persistence 已落地；scheduler/聊天侧 permission 仍主要是基座能力 |
| Phase 7 | Web API & Tasks | 🟡 | 管理 HTTP / WebSocket、plugin lifecycle、grants 管理、task 历史持久化与配置热更新已可用；日志历史持久化查询仍未实现 |
| Phase 8 | Web UI | ❌ | `web/package.json` 与 baseline 已有，真实页面与前端交互尚未开始 |
| Phase 9 | Launcher | ❌ | .NET / Avalonia 基线已锁定，真实 Launcher 行为尚未开始 |
| Phase 10 | Render Service | ❌ | render service 与 Chromium 调度尚未开始；`.deps/manifest.json` 仅为 baseline 占位 |

### 判定口径

- “已完成”只用于当前仓库里同时存在主链路实现、测试和可回指证据的能力。
- 已写出独立包实现、但尚未接入 `app` 主运行链路的能力，一律按“进行中”处理。
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

- 前 7 份契约已进入 fixture-ready。
- `cli-commands.yaml` 已 formalize 6 条 CLI 子命令，但尚未进入 fixture-ready。
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
| CLI fixtures / golden cases | ❌ | `cli-commands.yaml` 仍未配套 CLI 专用 fixtures 与回归样例 |

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
| plugin discovery | ✅ | 当前扫描 `examples/plugins` 与 `plugins/installed` |

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
| 三种出站 action | ✅ | `message.send`、`message.reply`、`message.send_image` 已支持请求构造与结果观察 |
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
| `init -> init_ack` | ✅ | 最小启动握手已打通 |
| `shutdown(stop)` | ✅ | 最小优雅停止路径已实现 |
| `ping` / `pong` | ✅ | keepalive 已进入 formal contract、fixtures 与 runtime 实现 |
| adapter -> runtime bridge | ✅ | bridge 已对接当前运行中的 runtime |
| 三种 action bridge | ✅ | `message.send`、`message.reply`、`message.send_image` 均已支持 |
| crash-backoff / dead_letter | ✅ | runtime crash 后的 `crashed` / `backoff` / `dead_letter` 状态流转已接入 app 生命周期 |
| 用户主动 reload | ✅ | `POST /api/plugins/{plugin_id}/reload` 已可用 |
| SDK 与示例插件 | ✅ | Python / Node.js SDK、示例插件与 builtin help 资源已落库，bundled manifests 已通过 contract 校验 |

### 已实现但未接入主链路

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 多插件调度 / fan-out | 🟡 | `internal/dispatch` 已支持 per-plugin queue、fan-out、directed delivery，但 `app` 未实例化 dispatcher |
| Command Parser / routing | 🟡 | `internal/command` 已实现 longest-prefix-first 解析，但主链路未消费该解析结果 |
| 不停机热重载 | 🟡 | `internal/dispatch.ReloadPlugin` 已支持 start-before-stop，当前管理面 reload 仍是 stop-then-start |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 多插件并发 runtime 主链路 | ❌ | `app` 当前只有单个 `runtime.Manager`，仍只会选择首个可启动插件 |
| temporal grants | ❌ | 权限时效窗口仍未实现 |
| 更广插件动作族 | ❌ | 三种 action 之外的动作仍未进入正式链路 |

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

### 已实现但仍是基座能力

| 子任务 | 状态 | 说明 |
|--------|------|------|
| Scheduler persistence / recovery | 🟡 | repository、hydration、tick loop 已进入 app，但 trigger 尚未接到 plugin runtime |
| 聊天侧 Permission / 黑名单 / 冷却限流 | 🟡 | `internal/permission`、`0010_blacklists.sql` 已存在，但 live command path 尚未调用 checker |

---

## 九、Phase 7 — Web API & Tasks 🟡

### 已落地

| 子任务 | 状态 | 说明 |
|--------|------|------|
| Health endpoints | ✅ | `GET /healthz` 与 `GET /readyz` |
| Setup & Session | ✅ | `setup/admin`、`setup/status`、`session/login`、`session logout`、`launcher-token` 已落地 |
| System management | ✅ | `GET /api/system/status`、`POST /api/system/shutdown` |
| Config management | ✅ | `GET /api/config`、`PUT /api/config` |
| Logs query | ✅ | `GET /api/logs` 已提供 bounded in-memory summaries |
| Tasks management | ✅ | `GET /api/tasks`、`GET /api/tasks/{task_id}`、`POST /api/tasks/{task_id}/cancel` |
| Plugin install | ✅ | `local_directory`、`local_zip`、`remote_url` 安装路径已进入真实路由 |
| Plugin lifecycle | ✅ | `enable` / `disable` / `reload` / `DELETE` 已接入真实路由 |
| Plugin grants 管理 | ✅ | `GET/POST/DELETE /api/plugins/{plugin_id}/grants...` 已落地 |
| 4 条管理 WebSocket | ✅ | `/ws/events`、`/ws/tasks`、`/ws/logs`、`/ws/plugins/{id}/console` 已落地 |
| HTTP 鉴权中间件 | ✅ | `RequireAuth`、公开/受保护路由分离、WebSocket `session_token` 兼容已落地 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| 日志历史检索 / 持久化查询 | ❌ | `/api/logs` 与 `/ws/logs` 仍只提供 bounded in-memory summaries |

---

## 十、Phase 8 — Web UI ❌

| 任务项 | 状态 | 说明 |
|--------|------|------|
| `web/package.json` 与 baseline | ✅ | Node / pnpm 基线已锁定，scripts 已占位 |
| auth/session shell | ❌ | 登录、session 生命周期与受保护管理面前端壳尚未开始 |
| 真实页面与布局 | ❌ | 路由、页面结构、状态管理与基础布局尚未开始 |
| HTTP / WebSocket 消费 | ❌ | 插件、任务、日志、events、console 等管理面接口消费尚未开始 |
| 运维交互流 | ❌ | 插件管理、任务查看、日志查看、配置编辑等前端流程尚未实现 |

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
| `contracts.yml` / `validate-contracts` | push main / PR | formal contracts、fixture 引用、example manifests、OpenAPI frozen path set、WebSocket frozen event set、plugin-protocol action shape、CLI contract 与 TaskType enum 交叉校验 |
| `lint.yml` / `baseline` | push main / PR | baseline 版本锁定、必要目录与文件存在性、`.deps/manifest.json` baseline 校验 |
| `lint.yml` / `server-smoke` | push main / PR | `go test ./...` 与 `go build ./cmd/raylea-server` |

### 当前验证结论

- `go test ./...` 当前通过。
- bundled plugin manifests 当前已与 `contracts/plugin-info.schema.json` 对齐。
- 根包 discovery 测试当前覆盖 `echo-python`、`hello-node`、`hello-python`、`notice-logger`。
- 当前主要风险已从“没有实现”转向“库级能力与 app 主链路之间仍有接线断层”。

---

## 十四、下一步行动建议

### 1. 先收敛 Phase 5 的真实边界

当前最优先的不是继续横向铺新功能，而是先定清楚 v0.1 的运行时边界：

1. **收敛为单 runtime 最小稳定闭环**
   - 把当前单 runtime、单 bridge、单主链路作为 v0.1 明确边界。
   - 对 `dispatch`、`command`、zero-gap reload 保持库级预研状态，不在本轮强行接线。

2. **或者继续推进多插件主链路**
   - 将 `internal/dispatch`、`internal/command`、scheduler trigger 正式接入 `app`。
   - 把多插件 fan-out、命令路由、reload 语义从“库级能力”提升为“产品行为”。

这一步不先定，后续文档、测试和实现仍会继续出现“代码已写、但不算真正落地”的口径漂移。

### 2. 收尾当前仍影响可用性的 server 缺口

建议优先顺序：

1. **日志持久化与历史检索**
   - `GET /api/logs` 与 `/ws/logs` 当前仍只依赖 bounded in-memory summaries。

2. **CLI fixtures / golden cases**
   - `cli-commands.yaml` 已 formalize，但 CLI 仍缺少 fixture-ready 配套。

3. **temporal grants**
   - grants storage 与 scope validation 已有，时效型授权仍未实现。

4. **builtin plugin 接线策略**
   - 要么把 `plugins/builtin/` 纳入 discovery 与生命周期；
   - 要么明确其仍仅为随仓库入库资源，不写成已上线能力。

### 3. 在 server 主链路稳定后再进入产品外层

管理 API、WebSocket、任务流与配置管理已经具备进入前端开发的前提，但前提是 server 状态语义不再发生大规模调整。

建议顺序：

1. auth shell
2. plugin list / detail / lifecycle
3. tasks
4. logs
5. config

Launcher 放在 Web UI 之后更稳妥，因为它依赖的 `launcher-token`、`system/status`、`system/shutdown` 已基本稳定，但整体启动体验仍受 server 主链路收敛程度影响。
