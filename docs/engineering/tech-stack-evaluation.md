# RayleaBot 技术栈评估与引入计划

本文档基于 RayleaBot 当前仓库状态，评估已采用与待引入的补充技术，并给出触发条件、实施路径和验收标准。所有候选方案遵循 `docs/engineering/baseline.md` 冻结的工程基线和复用阶梯：仓库现有代码与冻结选型 > 标准库 > 已冻结上游依赖 > 成熟开源 > 薄胶水。

## 冻结技术栈总览

| 层 | 冻结选型 |
|----|----------|
| Server | Go 1.25.8, `net/http` + chi v5.2.5, `database/sql` + `modernc.org/sqlite` v1.47.0, `coder/websocket` v1.8.14, `log/slog`, `gopkg.in/yaml.v3`, `chromedp` 0.14.2, `pgregory.net/rapid` v1.3.0 |
| Web UI | Vue 3.5.34, Vite 8.0.10, Ant Design Vue 4.2.6, Ant Design Icons Vue 7.0.1, Vue Vben Admin 5.7.0 对齐方案, Pinia 3.0.4, Vue Router 5.0.7, vue-i18n 11.3.0, `@vueuse/motion` 3.0.3, RayleaBot HTTP 语义封装, 原生 WebSocket 受控连接封装, Tailwind CSS 4.x, Vitest 4.1.5, Playwright 1.59.1 |
| Launcher | Electron 41.1.0, React 18.3.1, Fluent UI React v9, Vite 8.0.10, `@vitejs/plugin-react` 6.0.1, electron-builder 26.8.1, Vitest 4.1.5 |
| 插件 SDK | Python SDK (3.12+) + Node.js SDK (24+), JSONL over stdio |
| 契约 | OpenAPI 3.1, JSON Schema, YAML |

## 代码库规模

| 维度 | 数量 |
|------|------|
| Server Go 源文件（不含测试） | 198 |
| Server 内部包 | 41 |
| SQLite migration 文件 / 表 | 4 / 23 |
| SQLiteRepository 实现 | 10（auth, tasks, scheduler, logging, plugins, pluginkv, pluginconfig, permission blacklist, permission whitelist, permission whitelist state） |
| sqlc 命名查询 | 38 |
| API 操作（`contracts/web-api.openapi.yaml`） | 66 |
| 插件协议消息类型 | 10（init, init_progress, init_ack, event, action, result, error, ping, pong, shutdown） |
| 任务类型 | 8 |
| 配置段 | 21（含 20 个业务段与 `schema_version`） |
| Web Vue 组件 | 60 |
| Web TypeScript 源文件 | 92 |
| Web 测试（单元 + E2E） | 59 |
| Launcher 源文件（.ts + .tsx） | 57 |
| Launcher 测试 | 36 |
| CI 工作流 | 5（contracts, lint, race, release, self-host-smoke） |

---

## 引入计划

### 计划 A：契约驱动类型生成 ✅

`openapi-typescript` 7.8.0 从 `contracts/web-api.openapi.yaml` 生成 `web/src/types/generated.ts`，覆盖当前 66 个操作的请求 / 响应定义。7 个领域类型文件（common, tasks, plugins, system, logs, config, events）从 `generated.ts` re-export。

lint CI 的 `web-core` job 包含生成文件一致性检查（`pnpm generate:types && git diff --exit-code`）。

`oapi-codegen`（Server 侧 Go 类型生成）待评估：触发条件为 Server handler 手写类型不同步的事件累计达到 2 次。

---

### 计划 B：SQL 代码生成 ✅

sqlc v1.29.0 以 `server/internal/sqlcqueries/` 下 7 个 `.sql` 文件（38 named queries）为单一来源，生成 `server/internal/sqlcgen/`。auth、tasks、scheduler、logging、plugins、pluginkv、pluginconfig 的静态查询由 sqlc 生成；动态 SQL、permission repositories、third-party account service、Bilibili source 和 render template repository 保留手写查询。

配置：`server/sqlc.yaml`，schema 来自 `server/internal/storage/migrations/000001_base.sql`。

lint CI 的 `server-core` job 包含 `sqlc diff` 一致性门禁。

Repository 接口层保持当前存储读写语义，`go test ./...` 是全量回归入口。

---

### 计划 C：Web 管理面技术栈基线 ✅

Web 管理面采用 `Ant Design Vue + Vue Vben Admin` 对齐方案作为组件、页面壳、消息提示、表单、表格和布局规则基线，管理面骨架、请求层和样式系统收口到同一条主线。

**固定工程落点**：

| 方向 | 结论 |
|------|------|
| 主 UI 组件 | **Ant Design Vue 4.2.6** |
| 后台骨架 | **Vue Vben Admin 5.7.0 对齐方案** |
| 工程结构 | 保留 `web/` 单应用，不改成 Vben 官方整仓 `monorepo` / `turbo` |
| Vben 来源 | 以 Vue Vben Admin 5.7.0 作为页面壳、布局、工作区和偏好组织的对齐基线 |
| 数据层 | 保留 RayleaBot HTTP 语义封装，覆盖认证、超时、`401` 清理会话与错误包解析语义 |
| WebSocket | 保留受控连接模型 |
| 样式系统 | Ant Design Vue tokens + Vben 样式体系 + Tailwind CSS 4 + SCSS + CSS Variables |

**约束**：

- Vben 官方以多包仓库维护，更新策略按官方文档采用源码对照与按需合并。
- Vben 官方最低 Node 版本 20.15.0+，当前仓库固定 Node 24.14.0 满足。
- 该基线只影响 Web 实现层与工程基线，不改变 formal contract。

详细工程规则见 [`web-admin-baseline.md`](./web-admin-baseline.md)。

---

### 计划 D：前端表单与配置校验收口（候选）

配置编辑器（ConfigPage）覆盖 20 个业务配置段、100+ 字段。当前表单使用 Ant Design Vue Form + Vben Form adapter 做即时交互校验，提交时直接消费 `PUT /api/config` 的服务端校验结果与 `apply_effects`。若后续需要在前端加入契约驱动的最终校验层，建议引入 `ajv` + `ajv-formats` 并以 `contracts/config.user.schema.json` 为来源；是否引入取决于是否出现大量“提交后才暴露字段级错误”的用户反馈。

---

### 计划 E：Launcher IPC 类型安全（优先级：低）

**问题**：Launcher preload 层暴露 26+ IPC channel，channel 名和载荷类型以字符串约定。新增 channel 时须同步 main handler、preload bridge 和 renderer 调用三处。

**候选方案**：

| 方案 | 收益 |
|------|------|
| **zod** | 运行时消息校验 + TypeScript 类型推导，单一定义同时充当校验和类型 |
| 手写 TypeScript interface | 零依赖，编译期类型约束 |
| electron-trpc | tRPC 全栈类型安全，依赖面偏重 |

**触发条件**：IPC channel 数量超过 35，或因类型不匹配导致运行时错误。

**实施路径**：

1. 在 `launcher/src/shared/` 定义统一 IPC channel registry（TypeScript interface map）。
2. 将 preload 和 main 的 handler 注册改为从 registry 派生类型。
3. 评估是否需要 zod 运行时校验；若只需编译期约束，TypeScript interface 足够。

---

## 当前不采用项

以下方案与冻结选型冲突、会引入平行栈，或当前阶段不需要，明确不作为当前主线：

| 方案 | 排除原因 |
|------|----------|
| GORM / Ent | 替换 `database/sql` + 手写 SQL 冻结选型 |
| axios / ky | 与 Vben request 风格封装重复 |
| Socket.IO client | 替换原生 WebSocket 受控连接封装 |
| TanStack Query (Vue) 作为主数据层 | 当前正式方向是 request 层与 Pinia / Vben stores 收口，主数据层保持单一，不引入第二套服务端状态主框架 |
| Vuetify | 不属于当前冻结的 Web UI 方向 |
| Redux / MobX | 替换 Pinia 冻结选型 |
| Chakra UI / Material UI | 替换 Fluent UI React v9 冻结选型 |
| Vben 官方整仓 `monorepo` / `turbo` 直接并入 | 会引入第二套工程组织，与当前仓库边界冲突 |
| Git 子模块、Git URL 直连或镜像整仓依赖 Vben | 升级与维护风险高，超出当前冻结策略 |
| 分布式任务队列（river, taskq） | 单实例自托管架构，内存 Registry + SQLite 持久化足够覆盖 8 种正式任务类型 |
| 外部缓存（Redis, groupcache） | 单实例架构无外部缓存需求 |
| 外部日志框架（zap, zerolog） | `log/slog` 是冻结选型 |

---

## 低成本优化项

以下优化在当前冻结选型内可独立实施：

### slog Handler 链扩展

| 扩展 | 收益 | 触发条件 |
|------|------|----------|
| 采样中间件 | 对高频热点日志降频，减少 `management_logs` 表写入压力 | 日志量导致 SQLite 写入延迟 > 50ms |
| 请求上下文自动注入 | 自动关联 `request_id` / `plugin_id` 到所有日志条目 | 跨包追踪问题变困难时 |

实施方式：自定义 `slog.Handler` wrapper，与 `server/internal/logging/logger.go` 的现有 bootstrap 链集成。

### Web 开发体验

| 工具 | 收益 | 触发条件 |
|------|------|----------|
| **unplugin-vue-components** | Ant Design Vue 组件自动导入，减少模板样板 | `.vue` 文件超过 30 个 |
| **pinia-plugin-persistedstate** | 指定 store 持久化到 localStorage | 用户偏好需跨刷新保留时 |
| **msw** | API mock，支持脱离后端的前端测试 | 前端独立测试覆盖率目标提升时 |

### 测试工具

| 工具 | 收益 | 触发条件 |
|------|------|----------|
| **mockery** | 从 Go interface 生成 mock 实现 | interface 数量增长、手写 mock 维护成本上升时 |

---

## 插件 SDK 技术路线

Python SDK 和 Node.js SDK 均以 `contracts/plugin-protocol.schema.json` 为协议来源，提供事件 / 命令注册、消息发送和协议帧解析。

| 方向 | 状态 | 说明 |
|------|------|------|
| Python SDK 类型化 | ✅ | `sdk/python/rayleabot/models.py` 基于 `dataclasses` 定义全部 10 种帧类型和 6 种 segment 类型，零额外依赖 |
| Node.js SDK TypeScript 化 | ✅ | `sdk/nodejs/src/` 以 TypeScript 重写并输出 `.d.ts` |
| 协议消息 Schema 校验 | 待定 | 两端均从 `contracts/plugin-protocol.schema.json` 驱动运行时校验，与 SDK 正式发布同步 |

---

## 环境变量覆盖配置

配置系统覆盖 21 个配置段，当前通过 `config/user.yaml` + JSON Schema 校验实现。部署形态以单机自托管为主。

**候选**：**koanf**（分层配置读取，支持文件 / 环境变量 / flag 合并）。

**触发条件**：容器化部署或 CI 环境需要环境变量覆盖配置文件。当前暂无引入必要。
