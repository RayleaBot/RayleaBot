# RayleaBot 技术栈评估

本文档基于 RayleaBot v0.1 代码库，评估可选补充技术并提供引入建议。所有建议遵循 `docs/engineering/baseline.md` 冻结的工程基线和复用阶梯：仓库现有代码与冻结选型 > 标准库 > 已冻结上游依赖 > 成熟开源 > 薄胶水。

## 冻结技术栈总览

| 层 | 冻结选型 |
|----|----------|
| Server | Go 1.25.8, `net/http` + chi v5.2.5, `database/sql` + `modernc.org/sqlite` v1.47.0, `coder/websocket` v1.8.14, `log/slog`, `gopkg.in/yaml.v3`, `chromedp` 0.14.2 |
| Web UI | Vue 3.5.30, Vite 8.0.0, Element Plus 2.13.6, Pinia 3.0.4, Vue Router 5.0.4, 原生 fetch/WebSocket 薄封装 |
| Launcher | Electron 41.1.0, React 18.3.1, Fluent UI React v9, Vite 8.0.3, electron-builder 26.8.1 |
| 插件 SDK | Python SDK + Node.js SDK, JSONL over stdio |
| 契约 | OpenAPI 3.1, JSON Schema, YAML |

## 架构与规模

Server 包含约 127 个源文件（不含测试），分布在 30 个内部包中（adapter, app, auth, bridge, cli, command, config, console, deps, dispatch, health, httpapi, logging, outbound, permission, pluginconfig, pluginfile, pluginkv, pluginhttp, plugins, recovery, redact, render, runtime, scheduler, schema, secrets, storage, tasks）。SQLite 存储层采用读写分离双连接 + WAL 模式，14 个迁移文件维护 12 张表。配置系统覆盖 20 个配置段，任务系统支持 11 种任务类型（`plugin.install`、`plugin.uninstall`、`plugin.reload`、`backup.create`、`recovery.recheck`、`recovery.confirm`、`restore.apply`、`config.migrate`、`db.migrate`、`runtime.bootstrap`、`render.preview`），通过内存 Registry + SQLite 持久化实现顺序执行与订阅通知。

Web 侧 `web/src/types/api.ts` 约 505 行，手工维护并与 `contracts/web-api.openapi.yaml` 的 26 个路由定义保持同步。

## Server 层

### 数据库访问

`database/sql` + Repository 分层 + 手写 SQL 是冻结选型。各领域包（tasks、plugins、pluginkv、scheduler、auth、secrets 等）各自维护 SQLiteRepository，SQL 和 `rows.Scan` 映射重复度较高。

| 候选 | 定位 | 与冻结选型的关系 |
|------|------|------------------|
| **sqlc** | 以 SQL 文件为单一来源，生成类型安全的 Go 代码，零运行时 | 兼容——保留 Repository 接口层，生成代码替代手写实现，与 `server/internal/storage/migrations/` 的 SQL 直接衔接 |
| GORM | 全自动 ORM + 自动迁移 | 冲突——替换 `database/sql` + 手写 SQL 冻结选型，引入重型依赖 |
| Ent | 代码优先 Schema 定义 | 冲突——理念与 SQL 优先的现状差异大 |

sqlc 是 Server 层最有价值的补充点：消除 `rows.Scan` 手动映射、编译期捕获字段错误，同时不违反冻结选型——底层仍是 `database/sql`，SQL 仍然手写在 `.sql` 文件中。

### API 契约同步

手工维护三处类型定义（OpenAPI → Go types → `web/src/types/api.ts`）是主要重复来源。新增路由时须同步三处，契约不一致风险高。

| 工具 | 用途 |
|------|------|
| **oapi-codegen** | 从 `contracts/web-api.openapi.yaml` 生成 Go types 和 chi server 接口 |
| **openapi-typescript** | 从同一契约文件生成 TypeScript 类型定义 |

两者配合后，契约变更一次即可驱动前后端类型更新。不替换任何冻结选型，仅消除手工同步。

### 配置管理

配置系统成熟度较高：`gopkg.in/yaml.v3` 解析 + `jsonschema/v6` 校验 + 20 个结构化配置段 + 遗留字段兼容映射。

若后续需要环境变量覆盖配置文件（如容器化部署），可评估 **koanf**（分层配置读取，支持文件/环境变量/flag 合并）。当前部署形态以单机自托管为主，暂无引入必要。

### 日志

`log/slog` 是冻结选型，当前满足结构化日志、管理面日志持久化（management_logs 表）和请求维度日志需求。

可选的轻量扩展：

| 扩展 | 收益 | 条件 |
|------|------|------|
| **slog 采样中间件** | 对高频热点日志降频 | 日志量造成存储压力时 |
| **请求维度上下文注入** | 自动关联 request_id / plugin_id | 追查跨组件问题变困难时 |

两者均可基于 `log/slog` Handler 链实现，不引入外部依赖。

### 任务系统

内存 Registry + SQLite 持久化 + 单 goroutine 顺序执行器，支持取消和超时（默认 15 分钟）。面向单实例自托管场景，当前架构已包含持久化恢复和订阅通知，足够覆盖 11 种任务类型。

分布式任务队列（river、taskq 等）在多实例部署明确前无引入价值。

### 缓存

渲染服务使用内存缓存。单实例架构下无外部缓存需求。若多实例部署，可评估 **groupcache**（请求合并 + 分片缓存）。

### 测试

| 工具 | 状态 | 说明 |
|------|------|------|
| `pgregory.net/rapid` v1.2.0 | 已冻结 | 属性测试 |
| `testing` 标准库 | 使用中 | 单元/集成测试 |

SQLite 测试使用内存模式（`:memory:`），无需 testcontainers。**mockery**（接口 mock 生成）可在接口数量增长后评估。

## Web UI 层

### 类型同步

`web/src/types/api.ts`（~505 行）手工维护，与契约同步成本高。**openapi-typescript** 从 `contracts/web-api.openapi.yaml` 生成等价 TypeScript 定义，是最直接的改善。

### HTTP/WebSocket 客户端

`web/src/lib/http.ts`（原生 fetch 薄封装）和 `web/src/lib/ws.ts`（ManagedSocket，内置重连）是冻结选型。不需要引入平行 HTTP client 或 WebSocket client。

### 数据获取

Pinia 3.x 是冻结的全局状态方案。对于单纯的服务端数据获取（列表加载、轮询刷新），可评估 **TanStack Query (Vue)**：

- 自动缓存、后台刷新、请求去重
- 与 Pinia 互补（TanStack Query 管理服务端缓存，Pinia 管理客户端状态）
- 不替换 Pinia，不替换 `http.ts`

引入条件：Web 主链路页面数量达到需要统一数据获取模式时。

### 表单校验

Element Plus 内置表单校验（基于 async-validator）覆盖常规场景。若配置编辑器等复杂表单需要 JSON Schema 驱动的运行时校验，可引入 **ajv**（与 `contracts/config.user.schema.json` 共享 schema）。

### 可选补充

| 方案 | 收益 | 条件 |
|------|------|------|
| **unplugin-vue-components** | Element Plus 组件自动导入 | 页面/组件数量增长后减少样板 |
| **pinia-plugin-persistedstate** | 指定 store 持久化到 localStorage | 用户偏好或会话状态需跨刷新保留时 |
| **msw** | 脱离后端的 API 测试 mock | 前端独立测试需求明确时 |

### 前端测试

| 工具 | 状态 |
|------|------|
| Vitest 3.2.4 + @vue/test-utils 2.4.6 | 已冻结，单元/组件测试 |
| Playwright 1.59.1 | 已冻结，E2E 测试 |

当前工具链完整，无需额外测试框架。

## Launcher 层

### IPC 通信

主进程与渲染进程通过 preload 暴露受限 IPC API 通信。Launcher 侧冻结选型为 Electron main + preload + renderer 分层 + TypeScript service layer。

若 IPC 消息类型增长导致手工维护成本上升，可评估 **zod**（运行时消息校验 + 类型推导）。**electron-trpc** 引入 tRPC 全栈，依赖面偏重，需充分论证。

### Server API 复用

Launcher 调用 Server API 时，可复用 **openapi-typescript** 生成的类型定义，确保与 Web UI 类型一致。

### 构建与打包

Vite 8.0.3 + electron-builder 26.8.1 是冻结选型。自动更新（electron-updater）在发布流程确立后评估。

## 插件 SDK

Python SDK 和 Node.js SDK 均为骨架状态（各含入口文件和协议定义）。

| 方向 | 说明 |
|------|------|
| Python SDK 类型化 | 基于 `dataclasses` 或 **pydantic** 定义协议消息。pydantic 提供运行时校验和 JSON 序列化，但增加安装依赖；dataclasses + 手写校验依赖面为零 |
| Node.js SDK TypeScript 化 | 以 TypeScript 重写，发布时附带 `.d.ts`。运行环境侧已有 Node.js runtime，无额外成本 |
| 协议消息校验 | 两端均可从 `contracts/plugin-protocol.schema.json` 驱动校验，与 Server 侧共享 schema |

## 契约工具链

### 代码生成管道

| 环节 | 推荐 |
|------|------|
| 生成入口 | Makefile 或 `go generate` 统一调度 oapi-codegen + openapi-typescript |
| CI 守卫 | 生成产物纳入 git 跟踪，CI 执行 `make generate && git diff --exit-code` 检测漂移 |
| OpenAPI lint | **redocly/cli**：校验 `contracts/web-api.openapi.yaml` 规范合规性 |

### Schema 校验

Server 侧 `jsonschema/v6` 是冻结选型。Web 侧若需前端 JSON Schema 校验（如配置编辑器实时校验），可引入 **ajv**——它是 JSON Schema 生态中最成熟的 JavaScript 实现。

## 工具链

### 代码质量

| 工具 | 说明 |
|------|------|
| **golangci-lint** | Go 多维度静态分析，建议纳入 CI 门禁 |
| **sqlfluff** | SQL 迁移文件风格统一（可选） |

### 发布与 CI

| 工具 | 说明 | 条件 |
|------|------|------|
| **goreleaser** | Go 跨平台构建 + 发布自动化 | 正式发布流程确立后引入 |
| **codecov** | 覆盖率追踪与趋势可视化 | CI 门禁对覆盖率有要求时 |

### 本地开发

| 工具 | 说明 |
|------|------|
| **air** | Go 文件变更自动重编译重启 |

## 可观测性

RayleaBot 以单机自托管为主要部署形态。完整的 Prometheus / OpenTelemetry 栈在此规模下不具备必要性。

实际有价值的可观测手段：

| 手段 | 说明 |
|------|------|
| `/healthz` + `/readyz` | 已实现，覆盖存活和就绪探测 |
| `/api/system/status` | 已定义，聚合 adapter、runtime、存储状态 |
| `/api/system/diagnostics/export` | 已定义，导出诊断快照 |
| 管理面日志（management_logs 表） | 已实现，结构化审计日志 |
| `log/slog` 结构化输出 | 已实现，JSON 格式 |

若后续引入容器编排或多实例部署，再评估 Prometheus exporter 和 OpenTelemetry。

## 引入优先级

### 高价值 · 低侵入

| 方案 | 收益 | 理由 |
|------|------|------|
| **openapi-typescript** | 消除 ~505 行手工 TypeScript 类型维护 | 纯代码生成，不改冻结选型，不增加运行时依赖 |
| **oapi-codegen** | Go API types 和 chi server 接口从契约生成 | 与 chi + `net/http` 冻结选型兼容，生成代码替代手写 |
| **redocly/cli** | CI 中 OpenAPI 规范校验 | 开发工具，不进入运行时 |

### 中价值 · 需评估适配

| 方案 | 收益 | 前置条件 |
|------|------|----------|
| **sqlc** | 消除 Repository 层手写 `rows.Scan` 映射 | 需将分散在各包的 SQL 重组为 sqlc query 文件，评估与现有 Repository 接口的对接成本 |
| **TanStack Query (Vue)** | 统一服务端数据获取模式 | Web 主链路页面数量增长、数据获取模式需统一时 |
| **Node.js SDK TypeScript 化** | 插件开发类型安全 | SDK 骨架填充为正式实现时一并完成 |
| **golangci-lint** | 静态分析门禁 | 在 CI 配置中启用 |

### 按需引入

| 方案 | 触发条件 |
|------|----------|
| koanf | 需要环境变量/flag 覆盖配置文件 |
| ajv | 前端需要 JSON Schema 驱动的实时校验 |
| pydantic (Python SDK) | Python SDK 正式实现且需要运行时协议校验 |
| goreleaser | 跨平台发布流程确立 |
| air | 开发者日常启动流程需要热重载 |
| 可观测性栈 | 容器编排或多实例部署 |

## 约束

- `contracts/` 是接口、schema、错误码的唯一正式来源。代码生成工具以契约文件为输入，生成产物不得反向覆盖契约。
- 引入新依赖须遵循复用阶梯，优先证明冻结选型、标准库和现有实现不足。
- Go 侧不引入 ORM、平行路由栈或平行日志框架。
- Web 侧不引入平行 HTTP client、WebSocket client 或平行状态管理。
- Launcher 侧不引入平行渲染框架、设计系统或重复 service layer。
- JS 依赖通过 `pnpm install --frozen-lockfile` 管理，版本锁定在 lockfile 中。
