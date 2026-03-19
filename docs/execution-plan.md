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
| Phase 4 | Adapter（OneBot11） | 🟡 | 只读 reverse WebSocket adapter shell、状态机、intake 与最小内部事件归一化已落地；出站 action 仍未实现 |
| Phase 5 | Plugin Protocol Bridge | 🟡 | 最小 runtime manager、`init -> init_ack`、`shutdown(stop)` 与单一 `event -> result|error` bridge 已落地；完整 bridge 能力仍未实现 |
| Phase 6 | Config / Storage / Security | 🟡 | 配置解析、schema 校验与既有 `onebot.*` / server 配置消费已完成；存储、安全与迁移仍未落地 |
| Phase 7 | Web API & Tasks | 🟡 | `healthz` / `readyz`、只读插件查询与最小任务状态骨架已存在；内部 aggregate-only events emitter 已落地，但公开 WS 传输与写操作 API 仍未实现 |
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
| `fixtures/web-api` | ✅ | health、ready、plugin 相关响应样例已落库 |
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

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| OneBot 出站 action / request-response path | ❌ | 尚未实现 OneBot API 调用与 action 执行链路 |
| send / reply / API action | ❌ | 仍未实现任何出站消息或动作能力 |
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
| `event -> result | error` | ✅ | 当前只支持最小 `event` 投递与 `result/error` 回收 |
| lazy-start first valid plugin | ✅ | 首个可投递事件到达时可 lazy-start 单个有效插件 |
| bridge/runtime summary state | ✅ | 内存计数与最近摘要状态已落地 |

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| adapter 出站 action 执行 | ❌ | plugin `result` 不会触发 OneBot action 执行 |
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

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| SQLite | ❌ | 状态库仍未实现 |
| migration | ❌ | 迁移执行与版本演进尚未落地 |
| secret store | ❌ | 敏感凭据存储与注入尚未实现 |
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

### 仍未完成

| 子任务 | 状态 | 说明 |
|--------|------|------|
| setup / session / system routes | ❌ | 规划中的更广管理路由尚未实现 |
| 写操作插件 API | ❌ | enable / disable / install 仍未实现 |
| `/api/tasks` 执行型接口 | ❌ | 任务执行、取消与进度接口尚未落地 |
| public WebSocket transport implementation | ❌ | `/ws/events` 等公开管理会话传输尚未实现；当前只有内部 aggregate-only emitter，不是公开会话传输 |
| 全局错误中间件与更完整 session surface | ❌ | 统一错误中间件与管理会话能力仍未完善 |

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
- 管理 `setup` / `session` surface 尚未实现。

这些能力属于 v0.1 路线图的一部分，但当前仓库尚未进入真实实现阶段，不能因为已有 contract、README 或规划正文而误记为“已落地”。

---

## 十四、测试 & CI 现状

- server CI 已执行 `go test ./...` 与 `go build ./cmd/raylea-server`。
- contracts CI 已校验 7 份 formal contracts、必要 fixture 目录、example manifests 与精确的 web-api path set。
- fixture / golden 回归已覆盖 config、web-api、websocket、plugin-info、plugin-protocol、release-manifest。
- 当前 server 测试面已经覆盖 adapter 状态、runtime 生命周期、bridge 投递与 contract-backed fixtures 的关键路径。
- web / launcher 仍主要停留在 baseline scaffold，尚无真实功能测试面。

---

## 十五、下一步行动建议

按当前主线缺口，下一批最小推进建议为：

1. 落地 Phase 7 的 public WebSocket 管理通道。
2. 落地 Phase 6 的 SQLite / migration / state persistence。
3. 补 Phase 4 / Phase 5 中的 outgoing adapter action path 与更完整 plugin bridge。
