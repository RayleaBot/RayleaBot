# RayleaBot 技术栈评估与优化建议

本文档基于 RayleaBot v0.1 的代码库现状，分析可引入的技术栈以提升开发效率、降低维护成本并保持系统稳定性。

## 项目现状

| 维度 | 技术栈 |
|------|--------|
| Server | Go 1.25.8，约 200+ 源文件，手写 SQL + Repository 模式 |
| Web UI | Vue 3.5.30 + Vite 8.0 + Element Plus 2.13.5 + Pinia 3.0 |
| Launcher | Electron 41.1 + React 18 + Fluent UI React v9 |
| Database | SQLite via `modernc.org/sqlite`，手写 SQL，WAL 模式 |
| 插件 SDK | Python SDK + Node.js SDK，JSONL over stdio |
| 契约定义 | OpenAPI 3.1、JSON Schema、YAML |

## Server 层 (Go)

### 数据库访问

当前使用 `database/sql` 配合手写 SQL 和 Repository 分层。每个 Repository 重复编写相似的数据库操作代码。

**可选技术栈**:

| 方案 | 特点 | 适用场景 |
|------|------|----------|
| **sqlc** | SQL 为单一来源，生成类型安全代码，零运行时开销 | SQL 为中心的项目，推荐首选 |
| **GORM** | 开发速度快，自动迁移，生态丰富 | 复杂关系模型，快速原型 |
| **Ent** | 类型安全、可视化 Schema、Facebook 出品 | 复杂图关系，团队协作 |

sqlc 与 RayleaBot 的 contract-first 理念契合：以 SQL 文件为真相源，生成 Go structs 和接口，在编译期捕获类型错误，与现有 Repository 模式无缝集成。

### API 层代码生成

当前 OpenAPI 契约与 Go handler 实现独立维护，类型定义存在重复。

**推荐方案**:

- **oapi-codegen**: 从 OpenAPI 生成 Go types、server 接口、chi 路由桩
- 收益：减少 API types 手写代码约 60-70%，消除契约与实现不一致的风险，同时生成客户端 SDK 供 Launcher 调用

### 配置管理

当前采用 YAML 解析 + JSON Schema 校验，配置结构体手动 hydrate。

**可选技术栈**:

| 方案 | 收益 |
|------|------|
| **koanf** | 分层配置（文件+环境变量+flag），统一读取接口 |
| **envconfig** | 环境变量自动映射到 struct tag |
| **validator/v10** | struct tag 校验，补充 JSON Schema 校验 |

### 任务调度与队列

当前任务系统基于 SQLite 持久化，异步执行。对于高并发场景可考虑：

| 方案 | 特点 | 适用场景 |
|------|------|----------|
| **river** | Postgres 优先，类型安全，支持周期任务 | 需要持久化任务队列的场景 |
| **taskq** | Redis 优先，支持任务去重和批处理 | 高吞吐量、低延迟场景 |
| **otiai10/gosseract** | 保持现状，SQLite + 内存队列 | 当前规模足够，复杂度最低 |

### 缓存层

当前渲染服务使用内存缓存。分布式或多实例部署时可考虑：

| 方案 | 特点 |
|------|------|
| **ristretto** | 高性能内存缓存，自动驱逐 |
| **groupcache** | 分布式缓存，单实例飞行中请求合并 |

### 结构化日志增强

当前使用 `log/slog` 标准库。生产环境增强：

| 方案 | 收益 |
|------|------|
| **slog-sampling** | 高频日志采样，降低存储成本 |
| **slog-context** | 请求上下文自动注入 trace_id |

### 测试增强

| 工具 | 用途 |
|------|------|
| **testcontainers-go** | 集成测试数据库隔离 |
| **pgregory/rapid** | 基于属性的测试（已引入） |
| **go-testdeep** | 深度相等断言，可读错误信息 |
| **mockery** | 自动生成接口 mock |

## Web UI 层 (Vue/TypeScript)

### API 类型与客户端

当前 `web/src/types/api.ts` 手动维护，需与 `contracts/` 保持同步。

**推荐技术栈**:

| 方案 | 收益 |
|------|------|
| **openapi-typescript** | 从 OpenAPI 契约生成 TypeScript 类型，消除手动同步 |
| **TanStack Query (Vue Query)** | 数据获取状态管理，内置缓存、重试、乐观更新 |
| **orval** | 从 OpenAPI 生成完整 API 客户端（axios/fetch） |

预期收益：消除契约与前端的同步成本，减少 API 调用样板代码约 50%，自动获得请求缓存和错误重试能力。

### 表单与校验

| 方案 | 收益 |
|------|------|
| **zod** + **vue-zod** | 运行时 schema 校验，类型推导 |
| **vee-validate** | 表单状态管理，错误显示 |
| **jsonforms** | 从 JSON Schema 自动生成表单 |

### 状态管理增强

| 方案 | 收益 |
|------|------|
| **Pinia Colada** | 与 TanStack Query 类似的 Pinia 官方数据获取方案 |
| **pinia-plugin-persistedstate** | 状态持久化到 localStorage |

### 前端性能优化

| 方案 | 收益 |
|------|------|
| **vite-plugin-pwa** | PWA 支持，离线访问能力 |
| **vite-plugin-compression** | 构建产物压缩 |
| **unplugin-vue-components** | 自动组件导入，减少样板代码 |

### 测试增强

| 工具 | 用途 |
|------|------|
| **msw (Mock Service Worker)** | API mocking，脱离后端进行前端测试 |
| **Playwright 组件测试** | Vue 组件独立测试 |
| **vitest-ui** | 可视化测试报告 |

## Launcher 层 (Electron)

### IPC 通信类型安全

当前手写 IPC 封装，主进程与渲染进程间类型共享有限。

**可选方案**:

- **electron-trpc**: 端到端类型安全的 IPC
- **zod 序列化**: 运行时 IPC 消息校验

### Server API 集成

复用 Web UI 的 OpenAPI 生成客户端，确保 Launcher 与 Server 的 API 调用类型一致。

### 构建优化

| 方案 | 收益 |
|------|------|
| **electron-vite** | 统一的 Electron + Vite 构建配置 |
| **electron-updater** | 自动更新机制 |

## 插件 SDK

### Python SDK

当前基础框架已提供，类型提示有限。

**可选增强**:

| 方案 | 收益 |
|------|------|
| **pydantic** | 运行时类型校验，JSON 序列化 |
| **msgspec** | 高性能 JSON/MsgPack 处理 |
| **rich** | 控制台输出美化，调试友好 |

### Node.js SDK

当前为纯 JS 实现。

**可选增强**:

- **TypeScript 重写**: 类型安全，自动生成 `.d.ts` 声明文件
- **zod**: 协议消息运行时校验
- **tsx**: 开发时直接运行 TypeScript，无需预编译

## 契约与代码生成

### 契约同步工具链

| 工具 | 用途 |
|------|------|
| **Makefile 脚本** | 一键生成所有代码（Go types、TS types） |
| **pre-commit 钩子** | 契约变更时强制同步生成代码 |
| **redocly/cli** | OpenAPI lint 与 bundle |
| **json-schema-faker** | 从 schema 生成 fixtures 数据 |

### Schema 校验增强

| 方案 | 收益 |
|------|------|
| **go-jsonschema** | 从 JSON Schema 生成 Go 校验代码 |
| **ajv + ajv-i18n** | 前端 JSON Schema 校验，中文错误信息 |

## 工具链与开发体验

### 代码质量

| 工具 | 用途 |
|------|------|
| **golangci-lint** | Go 多维度静态分析 |
| **buf** | Protobuf 生态（如后续引入 gRPC） |
| **sqlfluff** | SQL 风格统一 |

### CI/CD 增强

| 工具 | 用途 |
|------|------|
| **goreleaser** | Go 项目跨平台自动发布 |
| **changesets** | 版本管理和 changelog 生成 |
| **codecov** | 测试覆盖率追踪 |
| **sonarqube** | 代码质量门禁 |

### 本地开发

| 工具 | 用途 |
|------|------|
| **air** | Go 热重载开发 |
| **turborepo** | 多包管理，任务管道优化 |
| **devenv/nix** | 可复现开发环境 |

## 监控与可观测性

### Metrics

| 方案 | 收益 |
|------|------|
| **prometheus/client_golang** | 标准 metrics 暴露 |
| **runtime/metrics** | Go 运行时指标（Go 1.16+） |

### Tracing

| 方案 | 收益 |
|------|------|
| **otel-go** | OpenTelemetry 标准链路追踪 |
| **jaeger-client** | 分布式追踪 UI |

### 健康检查增强

| 方案 | 收益 |
|------|------|
| **health-go** | 标准化健康检查端点 |

## 优先级建议

### 高优先级（高 ROI，低侵入）

1. **openapi-typescript**: Web UI 类型自动生成
2. **sqlc**: Go 数据库层代码生成
3. **TanStack Query**: Web 数据获取优化
4. **zod**: 运行时数据校验
5. **msw**: 前端独立测试能力

### 中优先级（长期收益）

1. **oapi-codegen**: Go API 接口生成
2. **koanf**: 配置管理统一
3. **TypeScript SDK**: Node.js 插件类型安全
4. **golangci-lint**: 代码质量门禁
5. **air**: Go 开发体验优化

### 低优先级（可选）

1. **goreleaser**: 自动化发布
2. **Ent**: 如数据模型复杂化时评估
3. **otel-go**: 全链路追踪
4. **river**: 大规模任务队列

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
| 构建时间 | Turborepo 缓存优化后降低 30-50% |
