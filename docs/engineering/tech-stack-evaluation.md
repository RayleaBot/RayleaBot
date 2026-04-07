# RayleaBot 技术栈评估与引入计划

本文档基于 RayleaBot v0.1 代码库实际状态，评估可选补充技术并给出引入时机、实施路径和验收标准。所有候选方案遵循 `docs/engineering/baseline.md` 冻结的工程基线和复用阶梯：仓库现有代码与冻结选型 > 标准库 > 已冻结上游依赖 > 成熟开源 > 薄胶水。

## 冻结技术栈总览

| 层 | 冻结选型 |
|----|----------|
| Server | Go 1.25.8, `net/http` + chi v5.2.5, `database/sql` + `modernc.org/sqlite` v1.47.0, `coder/websocket` v1.8.14, `log/slog`, `gopkg.in/yaml.v3`, `chromedp` 0.14.2, `pgregory.net/rapid` v1.2.0 |
| Web UI | Vue 3.5.30, Vite 8.0.0, Element Plus 2.13.6, Pinia 3.0.4, Vue Router 5.0.4, vue-i18n 11.x, lucide-vue-next, 原生 fetch/WebSocket 薄封装, Vitest 3.2.4, Playwright 1.59.1 |
| Launcher | Electron 41.1.0, React 18.3.1, Fluent UI React v9, Vite 8.0.3, electron-builder 26.8.1, Vitest 4.1.2 |
| 插件 SDK | Python SDK (3.12+) + Node.js SDK (24+), JSONL over stdio |
| 契约 | OpenAPI 3.1, JSON Schema, YAML |

## 代码库规模

| 维度 | 数量 |
|------|------|
| Server Go 源文件（不含测试） | 125 |
| Server 内部包 | 30 |
| SQLite 迁移文件 / 表 | 14 / 12 |
| SQLiteRepository 实现 | 7（auth, tasks, scheduler, logging, plugins, pluginkv, pluginconfig） |
| API 路由（`contracts/web-api.openapi.yaml`） | 31 |
| 插件协议消息类型 | 10（init, init_progress, init_ack, event, action, result, error, ping, pong, shutdown） |
| 任务类型 | 11 |
| 配置段 | 21（含 4 个遗留兼容映射） |
| Web Vue 组件 | 19 |
| Web TypeScript 源文件 | 44 |
| Web 测试（单元 + E2E） | 24 + 1 |
| Launcher 源文件（.ts + .tsx） | 46 |
| Launcher 测试 | 29 |
| CI 工作流 | 5（contracts, lint, race, release, self-host-smoke） |

---

## 引入计划

### 计划 A：契约驱动类型生成 ✅

`openapi-typescript` 7.8.0 从 `contracts/web-api.openapi.yaml` 生成 `web/src/types/generated.ts`（1922 行），覆盖全部 31 个路由的请求/响应定义。7 个领域类型文件（common, tasks, plugins, system, logs, config, events）从 `generated.ts` re-export。

lint CI 的 `web-core` job 包含生成文件一致性检查（`pnpm generate:types && git diff --exit-code`）。

`oapi-codegen`（Server 侧 Go 类型生成）待评估：触发条件为 Server handler 手写类型不同步的事件累计达到 2 次。

---

### 计划 B：SQL 代码生成 ✅

sqlc v1.29.0 以 `server/internal/sqlcqueries/` 下 7 个 `.sql` 文件（34 named queries）为单一来源，生成 `server/internal/sqlcgen/`（9 个 Go 文件、12 model structs）。全部 7 个 SQLiteRepository（auth, tasks, scheduler, logging, plugins, pluginkv, pluginconfig）的静态查询均由 sqlc 生成；动态 SQL（`ESCAPE` 子句、运行时 `IN` 列表、动态 `WHERE`）保留为手写。

配置：`server/sqlc.yaml`，schema 来自 `server/internal/storage/migrations/`。

lint CI 的 `server-core` job 包含 `sqlc diff` 一致性门禁。

Repository 接口层未因迁移产生 breaking change，`go test ./...` 全量通过。

---

### 计划 C：数据获取层（优先级：中）

**问题**：Web 侧 8 个页面各自在 Pinia store 或 `onMounted` 中手动管理 API 请求、加载状态和刷新逻辑。随着 31 个路由接入前端，缓存、去重、后台刷新需求重复出现。

**候选方案**：

| 方案 | 收益 | 条件 |
|------|------|------|
| **TanStack Query (Vue)** | 自动缓存、后台刷新、请求去重、乐观更新 | 与 Pinia 互补——TanStack Query 管理服务端数据缓存，Pinia 管理客户端状态；不替换 `http.ts` |
| 继续手写 | 零依赖、完全可控 | 当前 8 个页面仍可维护 |

**触发条件**：Web 页面数量超过 12，或同一份服务端数据被 3 个以上组件消费。

**实施路径**：

1. 将 `@tanstack/vue-query` 加入 `web/package.json`
2. 在 `web/src/main.ts` 注册 `VueQueryPlugin`
3. 从一个纯读取页面（如 `LogsPage`）试点，将 store 中的 fetch + loading 逻辑迁移为 `useQuery`
4. 确认与现有 Pinia store、WebSocket 实时推送的共存模式
5. 逐步迁移其余读取型页面

**验收标准**：
- 试点页面的加载、刷新、错误状态表现不变
- 网络请求数不增加（去重生效）
- 现有单元测试通过，组件测试可用 `queryClient.setQueryData` 注入

---

### 计划 D：前端表单校验增强（优先级：低）

**问题**：配置编辑器（ConfigPage）涉及 21 个配置段、100+ 字段。Element Plus 内置校验（async-validator）覆盖常规规则，但复杂约束（如 CIDR 格式、速率限制字符串 `"10/60s"` 语法）需要自定义 validator。

**候选方案**：

| 方案 | 收益 |
|------|------|
| **ajv** | 直接复用 `contracts/config.user.schema.json`，运行时 JSON Schema 校验 |
| 继续 async-validator 自定义规则 | 零额外依赖 |

**触发条件**：配置字段的自定义 validator 数量超过 10 个。

**实施路径**：

1. 将 `ajv` + `ajv-formats` 加入 `web/package.json`
2. 编写 `web/src/lib/config-validator.ts`，加载 `contracts/config.user.schema.json` 编译校验函数
3. 在 ConfigPage 的 `beforeSave` 中调用 ajv 校验，将错误映射到 Element Plus 表单字段
4. 保留 Element Plus 内置校验作为即时反馈层，ajv 作为提交前最终校验

---

### 计划 E：Launcher IPC 类型安全（优先级：低）

**问题**：Launcher preload 层暴露 26+ IPC channel，channel 名和载荷类型以字符串约定。新增 channel 时须同步 main handler、preload bridge 和 renderer 调用三处。

**候选方案**：

| 方案 | 收益 |
|------|------|
| **zod** | 运行时消息校验 + TypeScript 类型推导，单一定义同时充当校验和类型 |
| 手写 TypeScript interface | 零依赖，编译期类型约束 |
| electron-trpc | tRPC 全栈类型安全 | 依赖面偏重 |

**触发条件**：IPC channel 数量超过 35，或因类型不匹配导致运行时错误。

**实施路径**：

1. 在 `launcher/src/shared/` 定义统一 IPC channel registry（TypeScript interface map）
2. 将 preload 和 main 的 handler 注册改为从 registry 派生类型
3. 评估是否需要 zod 运行时校验（若仅需编译期约束，TypeScript interface 足矣）

---

## 不引入项

以下方案与冻结选型冲突或当前架构不需要，明确排除：

| 方案 | 排除原因 |
|------|----------|
| GORM / Ent | 替换 `database/sql` + 手写 SQL 冻结选型 |
| axios / ky | 替换原生 fetch 薄封装冻结选型 |
| Socket.IO client | 替换原生 WebSocket 薄封装冻结选型 |
| Vuetify / Ant Design Vue | 替换 Element Plus 冻结选型 |
| Redux / MobX | 替换 Pinia 冻结选型 |
| Chakra UI / Material UI | 替换 Fluent UI React v9 冻结选型 |
| 分布式任务队列（river, taskq） | 单实例自托管架构，内存 Registry + SQLite 持久化足够覆盖 11 种任务类型 |
| 外部缓存（Redis, groupcache） | 单实例架构无外部缓存需求 |
| 外部日志框架（zap, zerolog） | `log/slog` 是冻结选型 |

---

## 低成本优化项

以下优化基于冻结选型即可实施，不引入新依赖：

### slog Handler 链扩展

| 扩展 | 收益 | 触发条件 |
|------|------|----------|
| 采样中间件 | 对高频热点日志降频，减少 management_logs 表写入压力 | 日志量导致 SQLite 写入延迟 > 50ms |
| 请求上下文自动注入 | 自动关联 request_id / plugin_id 到所有日志条目 | 跨包追踪问题变困难时 |

实施方式：自定义 `slog.Handler` wrapper，与 `server/internal/logging/logger.go` 的现有 bootstrap 链集成。

### Web 开发体验

| 工具 | 收益 | 触发条件 |
|------|------|----------|
| **unplugin-vue-components** | Element Plus 组件自动导入，减少模板样板 | .vue 文件超过 30 个 |
| **pinia-plugin-persistedstate** | 指定 store 持久化到 localStorage | 用户偏好需跨刷新保留时 |
| **msw** | API mock，支持脱离后端的前端测试 | 前端独立测试覆盖率目标提升时 |

### 测试工具

| 工具 | 收益 | 触发条件 |
|------|------|----------|
| **mockery** | 从 Go interface 生成 mock 实现 | interface 数量增长、手写 mock 维护成本上升时 |

---

## 插件 SDK 技术路线

Python SDK 和 Node.js SDK 均以 `contracts/plugin-protocol.schema.json` 为协议来源，提供事件/命令注册、消息发送和协议帧解析。

| 方向 | 状态 | 说明 |
|------|------|------|
| Python SDK 类型化 | ✅ | `sdk/python/rayleabot/models.py` 基于 `dataclasses` 定义全部 10 种帧类型和 6 种 segment 类型，零额外依赖。`frame_from_dict()` 将原始 dict 解析为对应 dataclass。若需运行时校验可评估 **pydantic**。 |
| Node.js SDK TypeScript 化 | ✅ | `sdk/nodejs/src/` 以 TypeScript 重写（`types.ts`, `protocol.ts`, `index.ts`），`tsconfig.json` 配置 `declaration: true` 生成 `.d.ts`。`typescript` ~5.8.3 + `@types/node` ^24.0.0 作为 devDependencies。 |
| 协议消息 Schema 校验 | 待定 | 两端均从 `contracts/plugin-protocol.schema.json` 驱动运行时校验，与 SDK 正式发布同步 |

---

## 环境变量覆盖配置

配置系统覆盖 21 个配置段，当前通过 `config/user.yaml` + JSON Schema 校验实现。部署形态以单机自托管为主。

**候选**：**koanf**（分层配置读取，支持文件 / 环境变量 / flag 合并）。

**触发条件**：容器化部署或 CI 环境需要环境变量覆盖配置文件。当前暂无引入必要。
