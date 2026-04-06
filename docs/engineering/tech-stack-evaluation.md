# RayleaBot 技术栈评估与优化建议

本文档基于 RayleaBot v0.1 代码库现状，分析可选技术栈以提升开发效率、降低维护成本并保持系统稳定性。

## 项目现状

| 维度 | 技术栈 |
|------|--------|
| Server | Go 1.25.8，约 150+ 源文件，手写 SQL + Repository 分层 |
| Web UI | Vue 3.5.30 + Vite 8.0 + Element Plus 2.13.6 + Pinia 3.0.4 + Vue Router 5.0.4 |
| Launcher | Electron 41.1 + React 18 + Fluent UI React v9 + Vite 8.0.3 |
| Database | SQLite via `modernc.org/sqlite` v1.47.0，手写 SQL，WAL 模式 |
| 渲染 | `chromedp` 0.14.2 + Chromium 浏览环境 |
| 插件 SDK | Python SDK + Node.js SDK，JSONL over stdio |
| 契约定义 | OpenAPI 3.1、JSON Schema、YAML |

## 架构上下文

RayleaBot 核心事件流：`adapter -> bridge -> dispatch -> plugin runtimes`。插件以子进程形式运行，通过 stdin/stdout 上的 JSONL 协议与平台通信。持久化存储使用 SQLite，当前具备 14 张核心表。

技术栈选型须遵循 `docs/engineering/baseline.md` 冻结的工程基线：Go 标准库优先，冻结选型不足时才引入新依赖。

## Server 层 (Go)

### 数据库访问层

当前实现：`database/sql` + Repository 分层 + 手写 SQL。每个 Repository 需重复编写 CRUD 模板代码，类型转换与错误处理占据相当比例。

**可选方案**：

| 方案 | 特点 | 与当前架构契合度 |
|------|------|------------------|
| **sqlc** | SQL 为单一来源，生成类型安全代码，零运行时开销 | **高** — 与手写 SQL 现状直接衔接，保留 Repository 接口层 |
| **GORM** | 全自动 CRUD，自动迁移，生态丰富 | 中 — 需替换现有 Repository 模式，引入较重 |
| **Ent** | 类型安全 Schema 定义，Facebook 出品 | 中 — 面向代码优先而非 SQL 优先，与现状理念差异较大 |

**sqlc 具体收益**：
- 将 `server/internal/storage/migrations/` 下的 SQL 作为唯一来源
- 自动生成 Go structs、CRUD 方法、接口定义
- 编译期捕获字段类型错误，消除 `rows.Scan` 阶段的手动映射
- Repository 接口层继续保留，实现层由生成代码替代

### API 层代码生成

当前痛点：`web/src/types/api.ts` 约 440 行手工维护，需与 `contracts/web-api.openapi.yaml` 保持同步；新增接口时需同步更新契约、Go types、前端 types 三处。

**推荐方案**：

| 工具 | 用途 | 优先级 |
|------|------|--------|
| **oapi-codegen** | 从 OpenAPI 生成 Go types、server 接口、chi 路由桩 | 高 |
| **openapi-typescript** | 从 OpenAPI 生成 TypeScript 类型 | 高 |

**预期收益**：
- 消除契约与实现之间的类型重复
- 新增接口时，契约变更一次，前后端类型自动生成
- 减少约 60-70% 的 API 相关手写代码

### 配置管理增强

当前：`gopkg.in/yaml.v3` 解析 + JSON Schema 校验（`github.com/santhosh-tekuri/jsonschema/v6`），配置结构体手动 hydrate。

**可选补充**：

| 方案 | 收益 | 适用场景 |
|------|------|----------|
| **koanf** | 分层配置（文件+环境变量+flag），统一读取接口 | 需要环境变量覆盖配置文件的场景 |
| **validator/v10** | struct tag 校验，与 JSON Schema 双重校验 | 需要更友好的字段级错误提示 |

### 任务调度与队列

当前：基于 SQLite 持久化，异步执行。任务类型包括 `plugin.install`、`backup.create`、`runtime.bootstrap` 等 12 类。

**评估**：当前规模下 SQLite + 内存队列足够。若未来任务吞吐量达到每秒数百且需分布式部署，可考虑：

| 方案 | 特点 |
|------|------|
| **river** | Postgres 优先，类型安全，支持周期任务 |
| **taskq** | Redis 优先，支持任务去重和批处理 |

### 缓存层

当前：渲染服务使用内存缓存。若未来支持多实例部署：

| 方案 | 特点 |
|------|------|
| **ristretto** | 高性能内存缓存，自动驱逐 |
| **groupcache** | 分布式缓存，单实例飞行中请求合并 |

### 结构化日志

当前：`log/slog` 标准库。

**可选增强**：

| 方案 | 收益 |
|------|------|
| **slog-sampling** | 高频日志采样，降低存储成本 |
| **slog-context** | 请求上下文自动注入 trace_id |

### 测试工具

| 工具 | 用途 | 状态 |
|------|------|------|
| **testcontainers-go** | 集成测试数据库隔离 | 待评估 |
| **pgregory/rapid** | 基于属性的测试 | **已引入** |
| **go-testdeep** | 深度相等断言 | 可选 |
| **mockery** | 自动生成接口 mock | 可选 |

## Web UI 层 (Vue/TypeScript)

### API 类型与客户端

当前现状：`web/src/types/api.ts` 手工维护，与 `web/src/lib/http.ts`（原生 fetch 薄封装）、`web/src/lib/ws.ts`（原生 WebSocket 薄封装）配合使用。

**推荐技术栈**：

| 方案 | 收益 | 优先级 |
|------|------|--------|
| **openapi-typescript** | 从 OpenAPI 契约生成 TypeScript 类型 | 高 |
| **TanStack Query (Vue Query)** | 数据获取状态管理，内置缓存、重试、乐观更新 | 中 |

**预期收益**：
- 消除契约与前端的同步成本
- API 调用样板代码减少约 50%
- 自动获得请求缓存、错误重试、后台刷新能力

### 表单与校验

| 方案 | 收益 |
|------|------|
| **zod** | 运行时 schema 校验，类型推导，前后端可共享校验规则 |
| **vee-validate** | 表单状态管理，错误显示 |

### 状态管理

当前：Pinia 3.x 已冻结，满足需求。

**可选增强**：

| 方案 | 收益 |
|------|------|
| **pinia-plugin-persistedstate** | 状态持久化到 localStorage |

### 前端性能优化

| 方案 | 收益 |
|------|------|
| **vite-plugin-pwa** | PWA 支持，离线访问能力 |
| **unplugin-vue-components** | 自动组件导入，减少样板代码 |

### 测试增强

| 工具 | 用途 |
|------|------|
| **msw (Mock Service Worker)** | API mocking，脱离后端进行前端测试 |
| **Playwright 组件测试** | Vue 组件独立测试 |

## Launcher 层 (Electron)

### IPC 通信类型安全

当前：主进程与渲染进程间通过手写 IPC 封装通信。

**可选方案**：

| 方案 | 特点 |
|------|------|
| **electron-trpc** | 端到端类型安全的 IPC |
| **zod 序列化** | 运行时 IPC 消息校验 |

### Server API 集成

复用 Web UI 的 OpenAPI 生成客户端，确保 Launcher 与 Server 的 API 调用类型一致。

### 构建优化

当前已使用 Vite 构建，electron-builder 打包。

| 方案 | 收益 |
|------|------|
| **electron-vite** | 统一的 Electron + Vite 构建配置 |
| **electron-updater** | 自动更新机制 |

## 插件 SDK

### Python SDK

当前：基础框架已提供，类型提示有限。

**可选增强**：

| 方案 | 收益 |
|------|------|
| **pydantic** | 运行时类型校验，JSON 序列化 |
| **msgspec** | 高性能 JSON/MsgPack 处理 |

### Node.js SDK

当前：纯 JavaScript 实现。

**可选增强**：

| 方案 | 收益 |
|------|------|
| **TypeScript 重写** | 类型安全，自动生成 `.d.ts` 声明文件 |
| **zod** | 协议消息运行时校验 |
| **tsx** | 开发时直接运行 TypeScript，无需预编译 |

## 契约与代码生成

### 契约同步工具链

| 工具 | 用途 |
|------|------|
| **Makefile 脚本** | 一键生成所有代码（Go types、TS types） |
| **pre-commit 钩子** | 契约变更时强制同步生成代码 |
| **redocly/cli** | OpenAPI lint 与 bundle |

### Schema 校验增强

| 方案 | 收益 |
|------|------|
| **go-jsonschema** | 从 JSON Schema 生成 Go 校验代码 |
| **ajv + ajv-i18n** | 前端 JSON Schema 校验，中文错误信息 |

## 工具链与开发体验

### 代码质量

| 工具 | 用途 | 状态 |
|------|------|------|
| **golangci-lint** | Go 多维度静态分析 | CI 已启用 |
| **buf** | Protobuf 生态（如后续引入 gRPC） | 待评估 |
| **sqlfluff** | SQL 风格统一 | 可选 |

### CI/CD 增强

| 工具 | 用途 |
|------|------|
| **goreleaser** | Go 项目跨平台自动发布 |
| **changesets** | 版本管理和 changelog 生成 |
| **codecov** | 测试覆盖率追踪 |

### 本地开发

| 工具 | 用途 |
|------|------|
| **air** | Go 热重载开发 |
| **turborepo** | 多包管理，任务管道优化 |

## 监控与可观测性

### Metrics

| 方案 | 收益 |
|------|------|
| **prometheus/client_golang** | 标准 metrics 暴露 |
| **runtime/metrics** | Go 运行时指标 |

### Tracing

| 方案 | 收益 |
|------|------|
| **otel-go** | OpenTelemetry 标准链路追踪 |

### 健康检查

| 方案 | 收益 |
|------|------|
| **health-go** | 标准化健康检查端点（如 `/healthz`、`/readyz` 需更复杂逻辑时） |

## 优先级建议

### 高优先级（高 ROI，低侵入）

1. **openapi-typescript**: Web UI 类型自动生成，直接消除 400+ 行手工类型维护
2. **oapi-codegen**: Go API 层代码生成，减少契约与实现重复
3. **TanStack Query**: Web 数据获取优化，替代现有手工状态管理
4. **zod**: 运行时数据校验，前后端可共享校验规则

### 中优先级（中长期收益）

1. **sqlc**: 数据库层代码生成，需评估与现有 Repository 接口的适配成本
2. **koanf**: 配置管理统一，如环境变量覆盖需求增强时引入
3. **TypeScript SDK**: Node.js 插件类型安全
4. **msw**: 前端独立测试能力

### 低优先级（可选）

1. **goreleaser**: 自动化发布流程完善后引入
2. **Ent**: 如数据模型复杂化时评估
3. **otel-go**: 全链路追踪需求明确后引入
4. **river/taskq**: 任务队列需分布式部署时评估

## 约束与注意事项

- `contracts/` 始终是正式来源，代码生成工具不得绕过契约文件
- 引入新技术需评估与 `docs/engineering/baseline.md` 冻结选型的兼容性
- JS 依赖必须通过 `pnpm install --frozen-lockfile` 管理
- Go 标准库优先，ORM 等重型依赖需充分论证必要性
- 优先选择单一依赖、零运行时开销的方案

## 预期改进

| 维度 | 预期改进 |
|------|----------|
| 代码重复率 | 降低 40-50%（SQL、API 类型） |
| 契约同步工作量 | 减少 80%（自动生成替代手动同步） |
| 类型安全 | 编译期捕获更多契约不一致问题 |
| 开发体验 | IDE 自动补全、重构支持更完善 |
| 测试覆盖 | 前后端独立测试能力增强 |
