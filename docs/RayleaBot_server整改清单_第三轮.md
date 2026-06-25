# RayleaBot Server 架构整改清单（第三轮）

> 适用版本：`RayleaBot-main(2).zip`  
> 范围：`server/` 目录  
> 目标：围绕文件数过多、目录杂糅、结构不清晰、组合根过重、API 合同漂移、migration 不规范、插件/集成/管理 API 复杂度过高等问题，形成可执行、可核对、可分阶段验收的整改清单。  
> 说明：本清单基于静态审计和关键路径抽样。完整 `go test ./...` 未在当前环境执行，因为当前环境 Go 版本低于项目 `server/go.mod` 声明的 `go 1.25.8`。

---

## 0. 当前结构基线

整改前先冻结当前基线，后续每轮整改应使用相同口径重新统计，并做到指标不反弹。

| 指标 | 当前值 | 风险判断 | 下一阶段建议目标 |
|---|---:|---|---:|
| `server` 文件总数 | 1091 | 文件体量仍在增长 | 不再增长，优先删除/合并噪声文件 |
| `server` 目录总数 | 163 | 已改善，但仍偏多 | ≤155 |
| Go 文件数 | 1074 | 代码体量持续增长 | 只允许伴随功能必要增长 |
| 生产 Go 文件 | 841 | 偏多 | 控制新增，优先合并低价值拆分 |
| 测试 Go 文件 | 233 | 测试较多，仍需场景化 | 控制大文件和重复 fixture |
| Go 总行数 | 133657 | 中大型服务规模 | 不设硬降，但要求复杂度下降 |
| 生产 Go 行数 | 76675 | 业务复杂度高 | 新增需有明确领域归属 |
| 测试 Go 行数 | 56982 | 测试维护成本高 | 按场景拆分和复用 fixture |
| 生产 package 数 | 141 | 仍偏多 | ≤135 |
| 单文件生产 package | 37 | 偏多 | ≤30 |
| 两文件生产 package | 15 | 偏多 | ≤12 |
| `module.go` 单文件 package | 4 | 有包装目录倾向 | ≤2 |
| `internal/app/**` external internal import union | 56 | 仍偏重 | ≤50 |
| `management/router/modules.go` 内部 fan-out | 约 35 | 新的 HTTP 组合巨石 | ≤20 |
| `internal/plugins/actions` fan-out | 22 | 中央分发表膨胀 | ≤14 |
| `internal/plugins/lifecycle` fan-out | 17 | 生命周期包偏重 | ≤12 |
| `internal/integrations/bilibili/source` fan-out | 15 | Bilibili source 依赖偏多 | ≤12 |
| `internal/render/service` 生产文件 | 23 | render service 口袋化 | ≤20 或拆分职责 |

---

## 1. 整改优先级总览

### P0：必须优先处理

| 编号 | 问题 | 影响 | 完成状态 |
|---|---|---|---|
| P0-01 | OpenAPI 与 server DTO 存在三方账号 proxy 字段漂移 | 前端按 contract 提交会被后端拒绝或保存失败 | [x] |
| P0-02 | `management/router/modules.go` 成为新的 HTTP 组合中心 | 维护成本高，新增 API 继续膨胀 router | [x] |
| P0-03 | migration 模型仍是“当前 base schema + 兼容补丁”混合状态 | 新库/旧库演进路径不一致，schema 真相源不清 | [x] |
| P0-04 | `plugins/actions/dispatch.go` 继续中央化膨胀 | 插件 action 扩展成本高，权限/错误/schema 治理分散 | [x] |
| P0-05 | OpenAPI request/response contract test 覆盖不足 | 无法及时发现 handler 与 contract 漂移 | [~] 三方账号范围完成 |

### P1：重要架构治理

| 编号 | 问题 | 影响 | 完成状态 |
|---|---|---|---|
| P1-01 | `internal/plugins` 仍是最大复杂度中心 | 插件 runtime、management view、action、lifecycle 易混杂 | [x] |
| P1-02 | `internal/integrations/common` 成为新的通用抽屉 | 登录、账号、HTTP、错误、校验职责混在一起 | [x] |
| P1-03 | Bilibili 子系统未完全模块化 | app/integration module 仍了解 Bilibili 内部细节 | [x] |
| P1-04 | Douyin 解析/浏览器职责仍偏集中 | 阅读困难、测试场景不清晰 | [x] |
| P1-05 | manual SQL 例外过多 | sqlc 与手写 SQL 双轨并行，审计成本高 | [x] |
| P1-06 | `context.Background()` 在请求/生命周期路径中仍偏多 | 请求取消和 shutdown 语义不稳定 | [x] |
| P1-07 | management API DTO 仍以手写为主 | API shape 容易继续与 OpenAPI 漂移 | [x] |

### P2：持续可维护性优化

| 编号 | 问题 | 影响 | 完成状态 |
|---|---|---|---|
| P2-01 | 泛化文件名仍较多 | 搜索和阅读成本高 | [x] |
| P2-02 | 单文件 package 仍有 27 个 | package 过碎，目录噪声仍明显 | [x] |
| P2-03 | `render/service` 文件过多 | render service 口袋化 | [x] |
| P2-04 | `plugins/lifecycle` flatten 后包变重 | 减目录但未真正降认知复杂度 | [x] |
| P2-05 | `deps` 包像独立子系统但边界不清 | 与 render/browser/doctor/plugin runtime 关系不清 | [x] |
| P2-06 | 运维诊断聚合不足 | 问题定位依旧分散 | [x] |
| P2-07 | Go 工具链离线体验仍不友好 | 新人、本地、CI、运维排障成本高 | [ ] |

---

# 2. P0 整改项

---

## P0-01：修复 OpenAPI 与 server DTO 的三方账号 proxy 字段漂移

### 问题描述

当前 `contracts/web-api.openapi.yaml` 中的三方账号结构包含：

```yaml
proxy_url
proxy_enabled
```

数据库 schema 中也存在：

```sql
proxy_url TEXT NOT NULL DEFAULT ''
proxy_enabled INTEGER NOT NULL DEFAULT 0
```

但 server 端三方账号 DTO、domain model、repository 查询和 upsert 逻辑没有完整支持这两个字段。

当前风险链路：

```text
OpenAPI 声明支持 proxy_url/proxy_enabled
  ↓
Web/Launcher generated types 生成对应字段
  ↓
前端可能提交 proxy_url/proxy_enabled
  ↓
server strict JSON decoder 可能因 unknown field 拒绝
  ↓
或保存后刷新丢失，造成用户体验问题
```

### 影响

- API 合同不可信。
- 前端生成类型与后端实际行为不一致。
- 用户在管理端保存代理配置可能失败、丢失或被忽略。
- 后续 agent/开发者会以 OpenAPI 为准继续写错代码。
- contract-first 规则被破坏。

### 实施路径

#### 方案 A：正式支持三方账号代理字段（推荐）

需要补齐：

- [x] `internal/integrations/thirdparty` 或对应 account model 增加 `ProxyURL`、`ProxyEnabled`。
- [x] Upsert request 增加 `ProxyURL`、`ProxyEnabled`。
- [x] List/ListEnabled SQL 查询读出 `proxy_url`、`proxy_enabled`。
- [x] Upsert SQL 写入 `proxy_url`、`proxy_enabled`。
- [x] `thirdpartyapi` request DTO 增加 `proxy_url`、`proxy_enabled`。
- [x] `thirdpartyapi` response summary 增加 `proxy_url`、`proxy_enabled`。
- [x] 对 proxy URL 做基础校验：允许空值；非空时必须是合法 HTTP/HTTPS/SOCKS 代理 URL。
- [x] proxy_enabled 为 true 但 proxy_url 为空时返回结构化错误。
- [x] 更新 OpenAPI fixtures（当前无独立 examples）。
- [x] 更新 Web/Launcher generated types。
- [x] 增加管理 API request/response contract test。
- [x] 增加 repository upsert/list 单元测试。
- [x] 确认错误响应与 HTTP request 日志不输出完整 proxy URL 中可能包含的用户名密码。

#### 方案 B：暂时不支持三方账号代理字段

需要删除或降级：

- [ ] 从 OpenAPI `ThirdPartyAccountSummary` 删除 `proxy_url`、`proxy_enabled`。
- [ ] 从 `ThirdPartyAccountUpsertRequest` 删除 `proxy_url`、`proxy_enabled`。
- [ ] 更新 generated types。
- [ ] 数据库列可以保留，但文档中标记为暂未暴露 API。
- [ ] 增加测试确保前端不再提交这两个字段。
- [ ] 在 roadmap 或 issue 中记录未来恢复支持计划。

### 推荐选择

推荐选择方案 A，因为数据库 schema 和 contract 已经表达了这两个字段，继续隐藏会制造历史包袱。

### 验收标准

- [x] `PUT /api/third-party/accounts/{platform}/{account_id}` 可接收 contract 中声明的所有字段。
- [x] `GET /api/third-party/accounts` response 与 OpenAPI schema 一致。
- [x] 提交 `proxy_enabled=true` 且 `proxy_url=""` 时返回稳定错误码。
- [x] 提交合法 proxy URL 后，再次 GET 能看到保存值。
- [x] OpenAPI request/response contract test 可发现字段漂移。
- [x] 前端 generated types 与 server DTO 不再漂移。
- [x] `scripts/check-server-structure.py` 通过。
- [x] `cd server && go test ./...` 在正确 Go 版本环境通过。

---

## P0-02：拆薄 `management/router/modules.go`

### 问题描述

当前 `internal/management/router/modules.go` 直接理解大量业务域和 handler 构造细节，导入内部包约 35 个，包括 auth、config、console、eventpipeline、governance、health、integrations、logging、plugins、render、scheduler、system、tasks 等。

这使它成为新的 HTTP 组合巨石。

### 影响

- 新增 API 域容易继续往 `BuildDeps` 增字段。
- router 层变成第二个 servicegraph。
- API 注册与业务服务装配耦合。
- review 时必须理解大量 unrelated domain。
- contract drift 更难被局部测试发现。
- app/httpwire 虽然变薄，但复杂度转移到了 management/router。

### 目标状态

`management/router` 只负责：

```text
1. 挂载全局中间件
2. 区分 public/protected routes
3. 按模块注册路由
4. 不直接构造所有业务 handler
```

建议目标接口：

```go
type Module interface {
    RegisterPublicRoutes(r chi.Router)
    RegisterProtectedRoutes(r chi.Router)
}
```

或者：

```go
type RouteModule interface {
    Mount(r chi.Router, scope Scope)
}
```

### 实施路径

#### 第一步：定义 route module 抽象

- [x] 在 `internal/management/router` 或新包中定义 `Module` 接口。
- [x] 明确 public/protected/admin/ws 的注册方式。
- [x] 保留现有 router 外部 API，避免一次性大改。

#### 第二步：先迁移低风险 API 域

优先从依赖少的 API 域开始：

- [x] `coreapi`
- [x] `health/system status`
- [x] `logapi`
- [x] `taskapi`

每个 API 包暴露：

```go
func NewModule(deps Deps) router.Module
```

#### 第三步：迁移复杂 API 域

- [x] `configapi`
- [x] `pluginapi`
- [x] `thirdpartyapi`
- [x] `bilibiliapi`
- [x] `renderapi`
- [x] `governanceapi`
- [x] `protocolapi`

#### 第四步：压缩 `BuildDeps`

- [x] `BuildDeps` 不再包含每个业务 service。
- [x] `BuildDeps` 只包含通用 HTTP 依赖和 route modules。
- [x] 业务 service 注入下沉到对应 module 构造函数。

#### 第五步：加强架构测试

- [x] 增加 `management/router` fan-out 预算。
- [x] 预算初值设置为当前值，下一轮目标 ≤20。
- [x] 禁止 router 直接 import 新的业务内部包，除非白名单。

### 验收标准

- [x] `internal/management/router` fan-out ≤20。
- [x] `BuildDeps` 字段数明显减少。
- [x] 新增 API domain 不需要改大型 router 构造函数。
- [x] 每个 API domain 的 routes 可以在本包内定位。
- [x] OpenAPI 和 route 注册有一致性测试。
- [x] 现有管理 API 路由不丢失。
- [x] 集成测试通过。

---

## P0-03：正规化 storage migration 模型

### 问题描述

整改前已有：

```text
internal/storage/migrations/
  000001_base.sql
  000002_add_third_party_account_columns.sql
  000003_expand_third_party_account_platforms.sql
  000004_add_bilibili_source_room_cover_url.sql
```

整改前 `000001_base.sql` 已经包含当前最终 schema，同时后续 migration 又重复添加字段。例如：

```text
000001_base.sql 已有 profile_uid/proxy_url/proxy_enabled/cover_url
000002/000004 又 add 这些字段
```

这表示整改前 migration 模型是：

```text
当前快照式 base schema + 兼容旧库补丁
```

不是干净的历史 migration。

### 影响

- 新库和旧库迁移路径不一致。
- 开发者不知道该改 base 还是新增 migration。
- sqlc schema 来源和 migration 演进策略不统一。
- 自研 split SQL runner 对复杂 SQL 脆弱。
- skip/duplicate-column 判断需要人工维护。
- 未来线上升级排障困难。

### 整改结果

已采用方案 B：

- `server/internal/storage/schema.sql` 是当前完整 schema snapshot。
- `server/sqlc.yaml` 使用 `server/internal/storage/schema.sql` 作为 schema 来源。
- `server/internal/storage/migrations/000001_base.sql` 是 legacy compatibility base。
- `server/internal/storage/migrations/*.sql` 只负责现有数据库升级到当前 schema。
- 新库初始化直接应用 `schema.sql`，并记录已知 migration 版本。

### 整改决策

必须二选一，并写入文档。

#### 方案 A：真实历史 migration

适用：希望所有库从最早版本一步步迁移到最新。

任务：

- [ ] 将 `000001_base.sql` 还原为最早发布版本 schema。
- [ ] 所有后续 schema 变化用增量 migration 表达。
- [ ] 新库初始化也按 000001 → latest 逐个执行。
- [ ] 删除重复字段 skip 逻辑。
- [ ] drift test 对比最新迁移结果与期望 schema。
- [ ] sqlc schema 来源改为 migration 序列或生成后的当前 schema。

#### 方案 B：当前 schema snapshot + 后续 migration history

适用：希望新库直接用当前完整 schema，旧库通过 migration 升级。

任务：

- [x] 新增或恢复 `internal/storage/schema.sql` 作为当前完整 schema snapshot。
- [x] 明确 `schema.sql` 是新库初始化/sqlc 当前 schema 来源。
- [x] `migrations/` 只表示从某个历史版本到当前版本的兼容升级。
- [x] 文档中解释 snapshot 与 migrations 的职责。
- [x] 增加测试：旧库迁移到最新后的 schema 与 snapshot 等价。
- [x] `000001_base.sql` 不再伪装为历史起点，或改名为 `current_snapshot.sql`。

### 推荐选择

推荐方案 B，原因：

- 当前项目已经把 base schema 改成当前态。
- SQLite 自托管场景下，新安装直接使用当前 schema 更简单。
- 旧库兼容 migration 可以保留，但要明确其兼容性质。

### 是否引入迁移工具

建议评估：

- [x] `goose`
- [x] `golang-migrate`
- [x] `Atlas`

短期可以保留当前 runner，但需：

- [x] 将 split SQL 使用限制写入文档。
- [x] 禁止复杂 SQL/trigger 进入 split runner，或增加 parser。
- [x] 每个 migration 有 apply test。
- [x] 不再通过 err string 判断 duplicate column 作为常规逻辑。

### 验收标准

- [x] `docs/engineering/storage-migrations.md` 明确 migration 策略。
- [x] 开发者能判断“新增字段该改哪里”。
- [x] `000001_base.sql` 不再同时承担历史和当前快照双角色。
- [x] sqlc schema 来源与 migration 策略一致。
- [x] drift test 覆盖新库初始化与旧库迁移。
- [x] 新增 schema 字段没有重复 ADD COLUMN。
- [x] CI 中新增 migration 检查。

---

## P0-04：下沉 plugin actions 注册，拆薄中央分发表

### 问题描述

`internal/plugins/actions/dispatch.go` 当前约 478 行，仍是插件 action 中央分发表。虽然引入了 `actionModule`，但 `defaultActionModules` 仍集中注册所有 action：

```text
log
storage
config
plugin
secret
governance
http
scheduler
webhook
render
onebot
```

`internal/plugins/actions` fan-out 已达 22，超过 warning 阈值。

### 影响

- 新增 action 需要改中心分发表。
- `registryDeps` 越来越像 service locator。
- action 权限、schema、错误码、审计字段分散。
- 简单 action package 过多。
- 插件能力扩展不具备模块自治性。
- 未来 actions 包会成为新的 servicegraph。

### 目标状态

`plugins/actions` 只保留：

```text
Registry
Action handler interface
Request/Response types
common capability check
common error helpers
audit hooks
```

具体 action 注册下沉到所属 domain module：

```text
render module 注册 render action
scheduler module 注册 scheduler action
governance module 注册 governance action
secret module 注册 secret action
webhook/event module 注册 webhook action
onebot/protocol module 注册 onebot action
```

### 实施路径

#### 第一步：定义注册接口

- [x] 在 `plugins/actions` 定义最小注册接口：

```go
type Registrar interface {
    RegisterActions(registry *Registry)
}
```

或带依赖：

```go
type Registrar interface {
    RegisterActions(registry *Registry, deps Deps)
}
```

- [x] 确定 deps 不再是大 service locator，而是按模块局部构造。

#### 第二步：迁移简单 action

优先迁移：

- [x] `logaction`
- [x] `configaction`
- [x] `secretaction`
- [x] `scheduleraction`
- [x] `webhookaction`
- [x] `governanceaction`
- [x] `renderaction`

#### 第三步：合并单文件 action package

- [x] 单文件且无独立生命周期的 action 合并回所属 domain 或 actions 内部文件。
- [x] 复杂 action 如 `httpaction`、`storageaction`、`onebot` 可保留独立目录。
- [x] 每次合并后运行架构脚本更新预算。

#### 第四步：action 元数据标准化

每个 action 必须声明：

- [x] action name
- [x] capability
- [x] request schema
- [x] response schema
- [x] required permission
- [x] 是否读 secret
- [x] 是否写 secret
- [x] 是否访问网络
- [x] 是否写文件
- [x] audit event fields
- [x] stable error codes

#### 第五步：压缩 dispatch.go

- [x] `dispatch.go` 只保留 dispatch framework。
- [x] 删除中心 `defaultActionModules` 大表或将其变为由 app/module 注入。
- [x] 将文件降到 250 行以下。

### 验收标准

- [x] `internal/plugins/actions` fan-out ≤14。
- [x] `dispatch.go` ≤250 行。
- [x] 新增 action 不再要求修改中心注册大表。
- [x] 单文件 action package 减少至少 5 个。
- [x] action 元数据有测试覆盖。
- [x] plugin action contract 与实现保持一致。
- [x] plugin runtime 测试通过。

---

## P0-05：扩大 OpenAPI request/response contract test 覆盖

### 问题描述

当前 OpenAPI response contract test 覆盖较少，主要集中在：

```text
/api/setup/status
/api/setup/admin
/api/system/status
/api/launcher/status
```

不足以发现三方账号 proxy 字段这类 drift。

### 影响

- OpenAPI 文件合法，但 handler 不一定支持。
- generated TS 类型可能与 server 实际响应不一致。
- strict JSON decoder 可能拒绝 contract 中允许的字段。
- fixture 不能覆盖真实 API 行为。
- contract-first 规则缺少自动约束。

### 实施路径

#### 第一步：建立 contract test 工具层

- [x] 封装 OpenAPI schema loader。
- [x] 封装 request fixture validator。
- [x] 封装 response validator。
- [x] 支持按 path + method + status 验证。

#### 第二步：补高风险 GET response test

至少覆盖：

- [x] `GET /api/config`
- [x] `GET /api/third-party/accounts`
- [x] `GET /api/plugins`
- [x] `GET /api/plugins/{plugin_id}`
- [x] `GET /api/render/templates`
- [x] `GET /api/system/status`
- [x] `GET /api/bilibili/source/status`
- [x] `GET /api/tasks`
- [x] `GET /api/logs`

#### 第三步：补高风险 write request test

至少覆盖：

- [ ] `PUT /api/config`
- [x] `PUT /api/third-party/accounts/{platform}/{account_id}`
- [ ] `POST /api/third-party/accounts/{platform}/login/qrcode`
- [ ] `POST /api/plugins/{plugin_id}/start`
- [ ] `POST /api/plugins/{plugin_id}/stop`
- [x] `POST /api/render/templates/{id}/preview`
- [ ] `POST /api/tasks/{id}/cancel`

#### 第四步：验证 strict decode 与 OpenAPI 一致

- [x] 三方账号 OpenAPI request schema 中出现的字段必须能被 handler DTO 接收。
- [x] 三方账号 handler DTO 中可接受的 JSON 字段必须在 OpenAPI 中声明。
- [ ] 对每个 PUT/POST/PATCH endpoint 至少一个 request fixture 覆盖 optional 字段。
- [ ] unknown field 测试只针对真正未声明字段。

#### 第五步：纳入 CI

- [ ] PR 轻量模式检查变更相关 API。
- [ ] nightly strict 模式跑全量。
- [x] contract drift 失败信息要显示 path/method/field。

### 验收标准

- [x] 三方账号 proxy 字段 drift 可被测试发现。
- [ ] 每个 management API domain 至少 1 个 request/response contract test。
- [x] 三方账号 generated types drift 与 server handler drift 都可被 CI 捕获。
- [ ] 新增 API 未补 contract test 会失败。
- [x] contract test 输出可定位到具体字段。

---

# 3. P1 整改项

---

## P1-01：继续治理 `internal/plugins` 复杂度中心

### 问题描述

`internal/plugins` 约 238 个文件，是 server 最大子系统，包含：

```text
actions
capabilityview
catalog
configstore
discovery
filestore
httpclient
install
kvstore
lifecycle
managementui
manifest
repository
runtime
uninstall
webhook
```

当前风险不是单个文件过大，而是插件系统内概念多且边界还未完全硬化。

### 影响

- runtime state、install state、health state、management view 容易混用。
- lifecycle 包吸收过多职责。
- action 注册与 platform service 强耦合。
- management UI projection 可能反向污染 runtime。
- 插件能力新增路径不稳定。

### 目标模型

将插件系统按稳定模型分层：

```text
manifest         静态声明
catalog          已发现插件目录与来源
install          安装/卸载/版本状态
runtime          插件进程、协议、状态机
lifecycle        启停、恢复、重载、desired state
action           local action surface
storage          KV/File/Config/Secret storage
webhook          webhook 暴露
managementview   管理端 DTO/projection
```

### 实施路径

- [x] 梳理插件所有公开状态字段，分为 install/runtime/health/view。
- [x] 禁止 runtime 包 import management view。
- [x] 禁止 management DTO 直接复用 runtime mutable struct。
- [x] `plugins/lifecycle` 按职责重排文件，不再作为所有逻辑入口。
- [x] `plugins/runtime/manager` 按状态机、进程管理、协议收发、错误恢复拆分。
- [x] lifecycle 中的 render template sync、metrics、desired state 分别明确归属。
- [x] 插件状态转换建立表驱动测试。
- [x] plugin snapshot clone/DTO mapping 测试覆盖所有字段。

### 验收标准

- [x] `plugins/lifecycle` fan-out ≤12。
- [x] `plugins/actions` fan-out ≤14。
- [x] runtime state 与 management view 有明确 mapping。
- [x] 插件状态模型文档化。
- [x] 新增插件状态必须更新状态转换测试。
- [x] 运行架构脚本无 warning 或 warning 减少。

---

## P1-02：拆解 `internal/integrations/common` 抽屉包

### 问题描述

`internal/integrations/common` 当前承载：

```text
HTTP client
QR login model
QR login service
login persist
profile
validator
cooldown
errors
fingerprint
```

这个包已经不再是“common”，而是三方集成基础平台。继续发展会变成新的杂物抽屉。

### 影响

- 平台 provider、登录流程、账号模型、HTTP 工具、错误模型混杂。
- 新平台接入会继续塞 common。
- Bilibili/Douyin/Weibo/Netease 的边界不清。
- 错误模型和校验策略重复。
- 单元测试难以按职责定位。

### 目标结构

建议拆为：

```text
internal/integrations/
  account/
    account.go
    credential.go
    validator.go
    repository.go

  login/
    qrcode/
      service.go
      session.go
      provider.go
      persist.go
    cookie/
      validator.go

  httpclient/
    client.go
    url_guard.go
    errors.go

  fingerprint/
    fingerprint.go

  provider/
    provider.go
```

如果暂不改目录名，也至少重命名 common 内部文件并建立边界。

### 实施路径

- [x] 列出 `common` 每个导出类型的调用方。
- [x] 按职责分类：account/login/http/fingerprint/error。
- [x] 先迁移纯类型和错误模型。
- [x] 再迁移 qrcode login service。
- [x] 最后迁移 HTTP client 和 validator。
- [x] 删除 compatibility wrapper。
- [x] 更新 import。
- [x] 增加架构测试：禁止新增 `integrations/common` 导出符号。

### 验收标准

- [x] `integrations/common` 不再作为新代码入口。
- [x] 新平台接入只实现 provider interface。
- [x] account/login/httpclient/fingerprint 职责清晰。
- [x] common 文件数为 0 且无兼容层。
- [x] integration 测试通过。

---

## P1-03：将 Bilibili 子系统显式模块化

### 问题描述

Bilibili 目录已包含：

```text
accountusage
captcha
credential
diagnostics
dynamic
fingerprint
live
media
monitoring
proxy
session
source
subscriptions
values
```

它已经是独立子系统，而不是一个普通 provider。

### 影响

- app/integration module 仍需理解 Bilibili 内部细节。
- Bilibili source fan-out 偏高。
- management/bilibiliapi 可能直接依赖内部实现。
- 新增 Bilibili 功能容易跨多个目录修改。
- 诊断、账号、source、session 的边界不够清晰。

### 目标状态

Bilibili 对外暴露一个窄 module：

```go
type Module struct {
    Accounts AccountService
    Login LoginProvider
    Sources SourceService
    Media MediaService
    Diagnostics DiagnosticsService
}
```

对 app/management 层只暴露接口，不暴露内部 package。

### 实施路径

- [x] 梳理当前 Bilibili 外部 import 调用点。
- [x] 新增 `bilibili.Module` 或 `bilibili.Services`。
- [x] 把 source/session/media/proxy/diagnostics 构造收束到 Bilibili 内部。
- [x] `integrationmodule.Build` 只调用 `bilibili.Build(...)`。
- [x] `management/bilibiliapi` 只依赖 Bilibili 对外 service interface。
- [x] Bilibili 内部 package 不被 app 直接 import。
- [x] 降低 `bilibili/source` fan-out。

### 验收标准

- [x] `integrationmodule` 不直接构造 Bilibili source 内部依赖。
- [x] Bilibili 外部可见接口文档化。
- [x] `internal/integrations/bilibili/source` fan-out ≤12。
- [x] 管理 API 测试和 source 测试通过。
- [x] 新增 Bilibili 功能不需要修改 app/servicegraph 内部装配细节。

---

## P1-04：继续拆分 Douyin 解析与浏览器职责

### 问题描述

Douyin 已经拆分了部分 browser 文件，但 `resolve.go` 仍约 476 行，是当前较大的生产文件之一。

### 影响

- URL 规范化、浏览器补偿、搜索解析、profile 抽取、错误分类混在一起。
- 难以为每类解析异常写精准测试。
- 第三方平台行为变化时定位困难。
- 风控/验证码/登录态异常和普通解析错误容易混杂。

### 目标结构

```text
douyin/
  resolve_url.go
  resolve_search.go
  resolve_profile.go
  resolve_browser.go
  extract_profile.go
  browser_session.go
  browser_qrcode.go
  browser_runtime.go
  errors.go
  diagnostics.go
```

### 实施路径

- [x] 从 `resolve.go` 中先抽出纯 URL normalize 函数。
- [x] 抽出 profile extraction，不依赖 browser。
- [x] 抽出 browser fallback 流程。
- [x] 抽出错误分类和安全错误文案。
- [x] 每个拆分文件配对应测试。
- [x] 避免拆出新的单文件 package，只在同 package 内拆文件。

### 验收标准

- [x] `douyin/resolve.go` ≤250 行。
- [x] URL normalize、profile extract、browser fallback 有独立测试。
- [x] 第三方错误不会直接返回上游原文。
- [x] Douyin 集成测试通过。

---

## P1-05：治理 manual SQL 例外，减少 sqlc 双轨

### 问题描述

当前仍有大量手写 SQL 例外，覆盖 storage、secrets、render repository、logging repository、permission、thirdparty、Bilibili source、plugin config/kv 等。

### 影响

- sqlc 与手写 SQL 双轨并行。
- 查询结构变更时不易发现漂移。
- SQL 注入和参数错误靠人工审查。
- exception JSON 容易变成永久豁免清单。
- repository 风格不统一。

### 分类标准

将 manual SQL 分为：

```text
A 类：sqlc 可支持，应迁回 sqlc
B 类：sqlc parser gap，保留手写
C 类：动态查询/复杂分页，保留手写但封装 query builder
D 类：SQLite PRAGMA/维护命令，保留手写
```

### 实施路径

- [x] 给 `docs/engineering/manual-sql-exceptions.json` 每项增加 `category`。
- [x] 增加 `reason`、`owner`、`revisit_after` 字段。
- [x] 按文件排序，优先迁移 A 类。
- [x] 新增手写 SQL 必须写 exception。
- [x] handler 层禁止直接拼 SQL。
- [x] 对保留的 C 类动态查询增加专门测试。
- [x] 每轮减少 manual SQL 文件数量。

### 验收标准

- [x] exception JSON 中所有条目都有分类和复查日期。
- [x] A 类迁移计划明确。
- [x] manual SQL 文件数量较当前下降。
- [x] 新增手写 SQL 没有 exception 会 CI 失败。
- [x] sqlc diff 和相关 repository 测试通过。

P1-05 验证：

- [x] `python scripts/check-server-structure.py`。
- [x] `go test ./internal/secrets ./internal/configruntime ./internal/plugins/managementui ./internal/integrations/thirdparty -run "Test"`。
- [x] `sqlc diff`。

---

## P1-06：清理请求路径中的 `context.Background()`

### 问题描述

生产代码仍存在较多 `context.Background()`。部分用于 CLI 或后台 worker 是合理的，但 HTTP handler、配置更新、插件 lifecycle 操作中应使用请求 context 或 app lifecycle context。

### 影响

- HTTP 请求取消后后台写入仍继续。
- app shutdown 时任务不受统一 context 控制。
- 超时和取消语义不稳定。
- 测试难以控制长耗时操作。
- 出错时 observability 断链。

### 实施路径

- [x] 统计所有 `context.Background()` 使用点。
- [x] 分类：允许/需替换/待评估。
- [x] 建立白名单文件。
- [x] HTTP handler 中直接使用 `context.Background()` 的全部替换为 `r.Context()`。
- [x] app 构造和运行期传入 lifecycle context。
- [x] background worker 从 supervisor context 派生。
- [x] plugin lifecycle、config update、render sync 使用传入 ctx。
- [x] 增加架构测试：非白名单新增 `context.Background()` 失败。

### 验收标准

- [x] HTTP handler 中无直接 `context.Background()`。
- [x] 配置更新和 secret resolve 支持请求 context。
- [x] plugin lifecycle save desired state 使用请求 context 或 lifecycle context。
- [x] app shutdown 能取消后台任务。
- [x] 架构测试覆盖 context 白名单。

P1-06 验证：

- `python scripts/check-server-structure.py` 通过；仅保留既有 `internal/plugins/actions` fan-out warning。
- `cd server && go test ./tests/architecture -run TestContextBackgroundUsesAreAllowlisted` 通过。
- 请求路径修正点：plugin desired state 保存使用 `r.Context()`；auth bootstrap/login/logout/validate 使用 `r.Context()`；plugin install/uninstall after-success 和 catalog refresh 使用任务 context；app startup 使用 `NewWithContext`，运行期 plugin lifecycle 绑定 app `Run(ctx)`。

---

## P1-07：加强 management API DTO 与 OpenAPI 绑定

### 问题描述

management API DTO 大量手写。当前已经出现三方账号 proxy 字段漂移，说明只靠人工同步不足。

### 影响

- 新字段可能只加 OpenAPI，不加 DTO。
- DTO 字段可能未写回 repository。
- generated TS 类型和 server runtime 行为不一致。
- strict JSON decoder 放大漂移。
- API 文档可信度下降。

### 可选方案

#### 方案 A：引入 `oapi-codegen` 生成 DTO

- [ ] 只生成 request/response types，不强制生成 router。
- [ ] handler 使用 generated type。
- [ ] 自定义 mapping 到 domain model。
- [ ] 保留 chi router。

#### 方案 B：不引入生成，增强 contract test

- [x] 每个有 request body 的 fixture 与 OpenAPI request schema 做字段对齐测试。
- [x] response JSON 通过 OpenAPI schema validation。
- [x] fixture 覆盖 optional 字段。
- [x] DTO tag 扫描检查字段是否出现在 OpenAPI。

### 推荐

短期采用方案 B，减少技术引入成本。中期评估为高风险 API domain 引入 generated DTO。

### 验收标准

- [x] 每个 OpenAPI operation 都有 `x-fixtures` 登记的 fixture。
- [x] 高风险 DTO 字段与 OpenAPI schema 不一致会失败。
- [x] 新增/删除字段必须同轮更新 contract、handler、fixture、generated types。
- [x] 三方账号 proxy 漂移类问题不再复发。

P1-07 验证：

- `cd server && go test ./tests/integration -run "Test(ActualManagementResponsesMatchOpenAPI|WebAPIRequestFixturesMatchOpenAPI|OpenAPIFixtureRegistryCoversOperations)$"` 通过。
- 真实 response 合同覆盖新增 `GET /api/config`、`GET /api/plugins`、`GET /api/plugins/{plugin_id}`、`GET /api/system/render/templates`、`GET /api/system/render/templates/{template_id}`、`POST /api/system/render/templates/{template_id}/preview-html`、`GET /api/tasks`、`GET /api/logs`。
- `TestOpenAPIFixtureRegistryCoversOperations` 要求每个 OpenAPI operation 都在 `x-fixtures` 中登记至少一个 fixture；本轮补登记 `ok.third-party-user-resolve.yaml`。
- 修正 OpenAPI 校验根因：测试校验器把 `./config.user.schema.json` 解析到 `contracts/`；`RenderTemplateDetail` 与 `PluginDetail` 不再使用 `allOf + additionalProperties: false` 组合，避免真实字段被误判为额外字段。

---

# 4. P2 整改项

---

## P2-01：减少泛化文件名

### 问题描述

当前仍有较多泛化文件名：

```text
repository.go
module.go
config.go
errors.go
resolve.go
login.go
paths.go
manifest.go
validator.go
identity.go
build.go
```

### 影响

- 搜索结果噪声大。
- 文件名不表达业务语义。
- 新人不知道文件负责什么。
- review 时需要打开文件才能理解职责。

### 实施路径

- [x] 对重复文件名做清单统计。
- [x] 优先重命名高频目录中的泛化文件。
- [x] 用业务语义命名替代模板命名。

示例：

```text
repository.go -> account_repository.go / template_repository.go
module.go     -> bilibili_module.go / route_module.go
config.go     -> redaction_config.go / runtime_config.go
errors.go     -> domain_errors.go / provider_errors.go
resolve.go    -> user_resolve.go / media_resolve.go
login.go      -> qrcode_login.go / cookie_login.go
```

### 验收标准

- [x] 新增文件禁止默认使用 `service.go`、`types.go`、`helpers.go`。
- [x] `module.go/login.go/resolve.go/validator.go` 数量下降。
- [x] 文件名能表达主要职责。
- [x] 架构脚本输出泛化文件名统计。

P2-01 验证：

- `server/internal/integrations/{weibo,douyin,netease_music}/login.go` 改为 `qrcode_login.go`。
- `server/internal/integrations/{weibo,douyin,netease_music}/resolve.go` 改为 `user_resolve.go`。
- `server/internal/integrations/{weibo,douyin,netease_music}/validator.go` 改为 `account_validator.go`。
- `server/internal/app/httpwire/configmodule/module.go` 改为 `config_http_module.go`。
- `python scripts/check-server-structure.py` 输出泛化文件名指标，并通过 `docs/engineering/server-architecture-budget.json` 的 `generic_filenames` 预算阻止数量回涨。
- 当前计数：`login.go=2`、`resolve.go=2`、`validator.go=1`、`module.go=5`。

---

## P2-02：继续减少单文件 package

### 问题描述

当前仍有 27 个单文件 production package。部分是合理边界，部分是过度拆包。

### 影响

- 目录层级变深。
- import 路径变长。
- package 数膨胀。
- 低价值边界增加认知成本。
- 架构看似模块化，实际只是文件夹化。

### 实施路径

- [x] 列出当前 27 个单文件 package。
- [x] 为每个标记：`merge` / `expand` / `keep`。
- [x] 对 `keep` 项写明理由。
- [x] 对 `merge` 项写明归并目标。
- [x] 对 `expand` 项写明补齐边界和测试的目标。
- [x] allowlist 增加 owner、target_action、due_stage。

### 优先处理对象

- [x] `internal/app/httpwire/configmodule`
- [x] `internal/app/httpwire/routemodule`
- [x] `internal/app/servicegraph/pluginmodule`
- [x] `internal/app/servicegraph/integrationmodule`
- [x] `internal/plugins/actions/*action`
- [x] `internal/integrations/bilibili/*` 中的单文件工具包
- [x] `internal/render/engine`
- [x] `internal/textsafe`

### 验收标准

- [x] 单文件 production package ≤30。
- [x] `module.go` 单文件 package ≤2。
- [x] allowlist 不再是永久豁免。
- [x] 新增单文件 package 必须在 PR 说明中解释。

P2-02 验证：

- `docs/engineering/server-architecture-budget.json` 的 `single_file_production_package_allowlist` 已改为结构化条目，包含 `decision`、`reason`、`owner`、`target_action`、`due_stage`。
- 当前单文件 production package 为 27；`module.go` 单文件 package 为 2。
- `python scripts/check-server-structure.py` 会拒绝缺少结构化 allowlist 的单文件 production package。

---

## P2-03：拆分 `render/service` 口袋包

### 问题描述

`internal/render/service` 当前有 23 个生产文件、31 个总文件，已经超过 warning 阈值。它承载过多 render 子职责。

### 影响

- render service 成为口袋包。
- 插件模板、产物、预览、诊断、浏览器状态混在一起。
- 新增 render 功能难以判断归属。
- action/management/app 可能依赖过多内部细节。

### 目标结构

```text
render/service       对外 facade
render/template      模板管理
render/artifact      渲染产物
render/pluginbinding 插件模板同步
render/preview       预览与测试渲染
render/diagnostics   浏览器/资源诊断
```

### 实施路径

- [x] 列出 `render/service` 文件职责。
- [x] 先抽出纯 DTO/projection 文件。
- [x] 再抽出 artifact 操作。
- [x] 再抽出 plugin template sync。
- [x] 最后抽出 diagnostics。
- [x] 对外保留 `RenderService` facade。
- [x] management/plugin action 只依赖 facade。

### 验收标准

- [x] `render/service` 生产文件 ≤20。
- [x] render 内部职责有 README 或架构注释。
- [x] management/renderapi 不直接依赖内部 repository。
- [x] plugin action 不理解 render 内部 artifact/template 细节。

---

## P2-04：治理 `plugins/lifecycle` flatten 后包变重

### 问题描述

本轮删除了 lifecycle 的多个子目录，减少了目录噪声，但 `plugins/lifecycle` 自身变重，约 20 个生产文件，fan-out 17。

### 影响

- 减少目录但没有完全降低认知复杂度。
- lifecycle 继续吞并 commands、metrics、runtimeconfig、render template sync 等职责。
- 新增插件生命周期逻辑难以判断放哪里。

### 实施路径

- [x] 按插件生命周期流程重排文件：

```text
controller.go
desired_state.go
runtime_start.go
runtime_stop.go
reload.go
recovery.go
manifest_refresh.go
render_templates.go
scheduler.go
metrics.go
```

- [x] 明确哪些是状态机，哪些是副作用集成。
- [x] metrics 只采集，不反向控制 lifecycle。
- [x] render template sync 通过接口调用，不直接依赖 render 内部。
- [x] 如果继续拆子包，只按真实边界拆，不按单文件拆。

### 验收标准

- [x] `plugins/lifecycle` fan-out ≤12。
- [x] lifecycle 状态转换有表驱动测试。
- [x] lifecycle 不直接依赖过多 platform/internal implementation。
- [x] 文件名表达生命周期阶段。

---

## P2-05：明确 `deps` 子系统边界

### 问题描述

`internal/deps` 涉及 archive、download、manifest、system chromium、manager inspect/prepare/resolve、bootstrap messages 等，已经像一个依赖管理子系统。

### 影响

- 与 render browser、CLI doctor、plugin runtime、launcher 打包边界不清。
- 依赖准备失败时错误归属不清。
- 用户诊断信息分散。
- 后续新增依赖容易继续塞入 `deps`。

### 目标边界

```text
deps/manifest      依赖声明和版本
deps/download      下载与校验
deps/runtime       本机路径和可执行探测
deps/diagnostics   doctor/status 输出
```

### 实施路径

- [x] 写 `internal/deps/README.md` 或架构注释，说明 deps 职责。
- [x] 区分 build-time dependency 与 runtime dependency。
- [x] render browser 只通过 deps runtime interface 获取 chromium。
- [x] CLI doctor 只通过 diagnostics interface。
- [x] 不让业务包直接操作 deps 内部下载/解压细节。

### 验收标准

- [x] deps 对外接口清晰。
- [x] render、CLI、app 对 deps 的依赖收窄。
- [x] 依赖缺失错误有用户可读修复建议。
- [x] deps 测试覆盖下载失败、校验失败、路径缺失。

P2-05 验证：

- `internal/deps/README.md` 记录 manifest、download、runtime、diagnostics 的职责边界。
- `deps.NewRuntime(repoRoot)` 负责可能准备或解析 runtime entrypoint 的调用。
- `deps.NewDiagnostics(repoRoot)` 负责只读状态检查。
- render/app/plugin runtime/plugin install 改为通过 runtime 门面获取 Chromium、Python、Node.js、npm entrypoint。
- CLI doctor 和 system diagnostics 改为通过 diagnostics 门面读取依赖状态。
- `cd server && go test ./internal/deps ./internal/render/service ./internal/app/renderstack ./internal/cli ./internal/system ./internal/plugins/install ./internal/plugins/runtime/spec` 通过。

---

## P2-06：增强运维诊断聚合

### 问题描述

当前系统已有多个状态接口，但运维视角仍分散。排障需要跨系统状态、render、Bilibili source、plugin runtime、scheduler、logs、tasks、config 等多个页面/接口。

### 目标接口

建议新增或增强：

```text
GET /api/system/diagnostics
```

聚合：

- [x] server version / build info
- [x] database schema version
- [x] migration applied versions
- [x] config load/apply state
- [x] unresolved secret refs
- [x] OneBot adapter connection state
- [x] plugin runtime summary
- [x] render browser state
- [x] third-party account health
- [x] Bilibili source live/dynamic health
- [x] scheduler pending/running/failed summary
- [x] recent fatal domain errors
- [x] dependency manager status
- [x] filesystem path permissions

### 实施路径

- [x] 定义 diagnostics OpenAPI schema。
- [x] 每个 domain 暴露 `HealthSnapshot` 或 `DiagnosticsSnapshot`。
- [x] system API 聚合，不直接读取每个 domain 内部状态。
- [x] 错误字段区分 user_message 和 internal_reason。
- [x] 前端管理面展示“问题、影响、建议操作”。

### 验收标准

- [x] 一个接口可定位主要子系统健康状态。
- [x] 每个异常项都有修复建议。
- [x] 不泄露 secret、token、cookie、代理密码。
- [x] diagnostics response 有 OpenAPI contract test。

---

## P2-07：改善 Go 工具链离线体验

### 问题描述

项目要求 Go 1.25.8，当前环境若 Go 版本较低且离线，会无法自动下载 toolchain，导致测试无法运行。

### 影响

- 新人本地 clone 后失败。
- CI runner 若未预装正确 Go 版本会失败。
- 运维排障环境需要先解决工具链。
- agent 静态审计可以做，但完整测试难跑。

### 实施路径

- [x] 增强 `scripts/check-toolchain.py` 输出，给出明确修复命令。
- [x] 新增 `mise.toml` 或 `.tool-versions`。
- [x] 新增 `Dockerfile.dev` 或 devcontainer。
- [x] 文档说明离线环境预装 Go 1.25.8。
- [x] `make doctor` 在所有测试前先检查工具链。
- [x] CI 使用 `actions/setup-go` 且明确版本来源。
- [x] 评估 `go` 指令是否需要锁 patch，或只锁到必要 minor，并用 toolchain 指令表达首选版本。

### 验收标准

- [x] 本地版本错误时错误信息可读。
- [x] 离线环境有明确安装路径。
- [ ] devcontainer 能直接跑 server tests；本机 Docker Desktop daemon 未运行，且 `Start-Service com.docker.service` 权限不足，未完成实机验证。
- [x] README/engineering baseline 中工具链说明一致。

---

# 5. 架构预算整改

## 5.1 预算文件结构调整

当前 `server-architecture-budget.json` 已有预算，但建议改为更适合 ratchet 的结构。

### 当前问题

部分目标字段已经低于或高于当前值，不利于判断下一步。

### 建议格式

```json
{
  "production_package_count": {
    "current": 141,
    "max": 141,
    "next_target": 135,
    "long_term_target": 115
  },
  "single_file_production_package_count": {
    "current": 37,
    "max": 37,
    "next_target": 30,
    "long_term_target": 20
  }
}
```

### 任务

- [x] 将预算字段统一改成 `current/max/next_target/long_term_target`。
- [x] `max` 只允许等于或低于当前主干值。
- [x] 每轮整改后更新 `current` 和降低 `max`。
- [x] CI 中禁止超过 `max`。
- [x] `next_target` 作为整改计划，不作为 CI 阻塞。
- [x] warning 项转成明确 task。

## 5.2 下一轮预算目标

| 指标 | 当前 | 下一轮目标 | 完成 |
|---|---:|---:|---|
| production package count | 133 | ≤135 | [x] |
| single-file package count | 27 | ≤30 | [x] |
| two-file package count | 15 | ≤12 | [ ] |
| module.go single-file package count | 2 | ≤2 | [x] |
| server directory count | 155 | ≤155 | [x] |
| app external import union | 68 | ≤50 | [ ] |
| management/router fan-out | 2 | ≤20 | [x] |
| plugins/actions fan-out | 14 | ≤14 | [x] |
| plugins/lifecycle fan-out | 12 | ≤12 | [x] |
| bilibili/source fan-out | 12 | ≤12 | [x] |
| render/service production files | 19 | ≤20 | [x] |

---

# 6. 分阶段实施计划

## 阶段 1：修复合同漂移与高风险行为

目标：优先解决已发现的真实 bug 和 contract 不一致。

任务：

- [x] P0-01：修复三方账号 proxy 字段漂移。
- [x] P0-05：补三方账号 request/response OpenAPI contract test。
- [x] 检查三方账号 OpenAPI request 字段是否能被 handler strict decode 接收。
- [x] 新增非 invalid Web API request fixture 的 OpenAPI schema 校验；`PUT /api/config` 由 config schema 专测覆盖。
- [x] 检查三方账号 generated TS 字段是否有 server 实现。
- [x] 更新 fixtures 和 docs。
- [x] 运行 web/launcher generate types。

验收：

- [x] 三方账号保存/读取 proxy 字段行为一致。
- [x] OpenAPI contract test 能捕捉字段漂移。
- [x] CI 轻量 contract test 通过。

阶段 1 验证：

- [x] `go test ./internal/integrations/thirdparty ./internal/management/thirdpartyapi ./internal/management/bilibiliapi ./tests/integration -run "Test(Upsert|ThirdParty|BilibiliAccountSummaryFieldsMatchOpenAPI|ThirdPartyAccountDTOFieldsMatchOpenAPI|ActualManagementResponsesMatchOpenAPI)"`。
- [x] `go test ./tests/integration -run "TestThirdPartyAccountAndBilibiliSourceHandlers"`。
- [x] `go test ./tests/integration -run "Test(ActualManagementResponsesMatchOpenAPI|WebAPIRequestFixturesMatchOpenAPI)"`。
- [x] `python scripts/ci/validate_contracts.py --mode pr`。
- [x] `python scripts/check-server-structure.py`。
- [x] `pnpm run typecheck` in `web/`。
- [x] `pnpm run typecheck` in `launcher/`。
- [x] `cd server && go test ./...`。
- [x] `gbash -lc 'mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server'`。
- [x] `git diff --check`。

---

## 阶段 2：拆薄 HTTP 组合根

目标：避免 `management/router` 成为第二个 servicegraph。

任务：

- [x] P0-02：定义 route module。
- [x] 迁移低风险 API domain。
- [x] 迁移 config/plugin/thirdparty/bilibili/render API。
- [x] 压缩 `BuildDeps`。
- [x] 增加 `management/router` fan-out 预算。

验收：

- [x] `management/router` fan-out ≤20。
- [x] 新增 API 不需要改大型 router 构造函数。
- [x] 所有路由测试通过。

---

## 阶段 3：治理 plugin actions

目标：避免 action 中央分发表继续膨胀。

任务：

- [x] P0-04：定义 action registrar。
- [x] action 注册下沉到 domain module。
- [x] 合并简单单文件 action package。
- [x] 补 action metadata。
- [x] 压缩 `dispatch.go`。

验收：

- [x] `plugins/actions` fan-out ≤14。
- [x] `dispatch.go` ≤250 行。
- [x] 新增 action 不需要改中心大表。

---

## 阶段 4：正规化 migration

目标：明确 schema 真相源和演进路径。

任务：

- [x] P0-03：选择 migration 策略。
- [x] 清理 base/migration 重复字段。
- [x] 对齐 sqlc schema 来源。
- [x] 增强 migration drift test。
- [x] 评估 goose/golang-migrate/Atlas。

验收：

- [x] 文档说明清楚。
- [x] 新库和旧库 schema 收敛测试通过。
- [x] 新增字段路径明确。

阶段 4 验证：

- [x] `gofmt -w server/internal/storage/store_schema.go server/internal/storage/migration_test.go`。
- [x] `cd server && go test ./internal/storage`。
- [x] `cd server && sqlc diff`。
- [x] `cd server && sqlc generate`，`server/internal/sqlcgen/` 无生成代码变更。
- [x] `cd server && go test ./...`。
- [x] `python scripts/check-server-structure.py`。
- [x] `python scripts/ci/validate_contracts.py --mode pr`。
- [x] `cd server && mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server`。
- [x] `server/dist/raylea-server.exe` 已生成。
- [x] `git diff --check` 无空白错误。

---

## 阶段 5：治理 plugins/integrations/render 复杂域

目标：压缩最大复杂度中心。

任务：

- [x] P1-01：插件模型分层。
- [x] P1-02：拆解 integrations/common。
- [x] P1-03：Bilibili module 化。
- [x] P1-04：Douyin resolve 继续拆分。
- [x] P2-03：render/service 职责拆分。
- [x] P2-04：plugins/lifecycle 重排。

验收：

- [x] warning 项减少。
- [x] 单文件 package ≤30。
- [x] production package ≤135。
- [x] 目录数 ≤155。

阶段 5 验证：

- [x] `go test ./internal/integrations/thirdparty ./internal/integrations/qrcode ./internal/integrations/douyin ./internal/integrations/weibo ./internal/integrations/netease_music ./internal/integrations/bilibili ./internal/app/servicegraph/integrationmodule ./internal/management/thirdpartyapi ./internal/management/bilibiliapi`。
- [x] `go test ./internal/integrations/bilibili ./internal/integrations/bilibili/source ./internal/app/servicegraph/integrationmodule`。
- [x] `go test ./internal/render/service ./internal/management/renderapi ./internal/plugins/actions`。
- [x] `go test ./internal/plugins/lifecycle ./internal/app/pluginstack ./internal/app/servicegraph/pluginmodule ./internal/metrics ./internal/app ./internal/app/servicegraph`。
- [x] `python scripts/check-server-structure.py`。

---

## 阶段 6：运维与工具链体验

目标：降低部署、排障和本地测试成本。

任务：

- [x] P2-06：新增系统 diagnostics 聚合。
- [ ] P2-07：完善工具链文档和 devcontainer；devcontainer 已配置，实机构建验证被本机 Docker daemon 权限阻塞。
- [x] `make doctor` 覆盖 Go、Node、pnpm、sqlc、chromium、DB 权限。
- [x] 诊断输出不泄露 secret。

验收：

- [x] 新人可按文档跑通 server tests。
- [x] 管理端可看到主要子系统状态。
- [x] 离线环境失败提示明确。

阶段 6 验证：

- [x] `go test ./internal/scheduler ./internal/system ./internal/app/servicegraph ./internal/management/systemapi ./tests/integration -run "Test(EngineRunningCountDuringTrigger|SystemDiagnostics|DiagnosticsIssuesExpose|DiagnosticsIssueGroupsExpose|ActualManagementResponsesMatchOpenAPI|WebAPIRequestFixturesMatchOpenAPI)"`。
- [x] `python -m unittest scripts.tests.test_check_toolchain`。
- [x] `pnpm run typecheck` in `web/`。
- [x] `pnpm run typecheck` in `launcher/`。
- [x] `python scripts/ci/validate_contracts.py --mode pr`。
- [x] `python scripts/check-server-structure.py`。
- [x] `mingw32-make doctor`。
- [x] `git diff --check`。
- [ ] `docker build -f .devcontainer/Dockerfile -t rayleabot-devcontainer-check .`；本机 Docker Desktop daemon 未运行，且当前权限无法启动 `com.docker.service`。

---

# 7. PR 可核对清单

每个涉及 `server/` 的 PR 应检查：

## 7.1 通用检查

- [ ] 是否新增 production package？如是，是否满足至少两个条件：独立领域模型、独立生命周期、多个调用方、独立测试价值、长期预计超过 4 个文件、未来可替换。
- [ ] 是否新增单文件 package？如是，是否写明原因并加入/更新 allowlist。
- [ ] 是否新增泛化文件名？如 `service.go`、`types.go`、`helpers.go`、`module.go`、`repository.go`。
- [ ] 是否增加 `internal/app/**` 或 `management/router` fan-out？
- [ ] 是否新增 `context.Background()`？如是，是否属于白名单。
- [ ] 是否新增手写 SQL？如是，是否登记 manual SQL exception。
- [ ] 是否改动 API shape？如是，是否先改 OpenAPI。
- [ ] 是否改动配置字段？如是，是否补 `x-apply-policy`、`x-secret`、`x-redaction`。
- [ ] 是否涉及 secret、token、cookie、proxy auth？如是，是否有脱敏测试。
- [ ] 是否新增第三方集成错误？如是，是否使用安全 DomainError。

## 7.2 合同检查

- [ ] OpenAPI 已更新。
- [ ] request fixture 已更新。
- [ ] response fixture 已更新。
- [ ] web generated types 已更新。
- [ ] launcher generated types 已更新。
- [ ] server DTO 与 OpenAPI 字段一致。
- [ ] strict JSON decode 与 OpenAPI request schema 一致。
- [ ] 错误码已登记。
- [ ] docs/examples 已同步。

## 7.3 数据库检查

- [ ] schema 改动有 migration 或 snapshot 更新。
- [ ] sqlc schema 来源同步。
- [ ] sqlc generated code 更新。
- [ ] migration drift test 更新。
- [ ] 旧库升级测试通过。
- [ ] 新库初始化测试通过。
- [ ] 没有重复 ADD COLUMN 或脆弱 skip 逻辑。

## 7.4 插件检查

- [ ] 新 action 有 capability。
- [ ] 新 action 有 request/response schema。
- [ ] 新 action 有权限边界。
- [ ] 新 action 有审计字段。
- [ ] 新 action 有稳定错误码。
- [ ] 新 action 注册是否下沉到 domain module。
- [ ] 没有继续扩大 `dispatch.go` 中央表。

## 7.5 集成检查

- [ ] provider 错误不直接返回上游原文。
- [ ] cookie/token 不返回给浏览器。
- [ ] proxy URL 日志脱敏。
- [ ] HTTP client 有 SSRF/url guard。
- [ ] 登录态、账号态、source 态边界清晰。
- [ ] Bilibili/Douyin 变更有专门测试。

---

# 8. 建议验证命令

在正确工具链环境中执行。

## 8.1 架构预算

```bash
python scripts/check-server-structure.py
```

## 8.2 Server 测试

```bash
cd server
go test ./...
```

## 8.3 Server 构建

```bash
cd server
mkdir -p dist
go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server
```

## 8.4 sqlc drift

```bash
cd server
sqlc diff
```

## 8.5 OpenAPI / generated types

```bash
cd web
pnpm generate:types
git diff --exit-code src/types/generated.ts
```

```bash
cd launcher
pnpm generate:types
git diff --exit-code src/shared/web-api.generated.ts
```

## 8.6 Web/Launcher 受影响时

```bash
cd web
pnpm run typecheck
pnpm test
pnpm build
```

```bash
cd launcher
pnpm run typecheck
pnpm test
pnpm build
```

---

# 9. 最终验收门槛

本轮清单完成后，应达到：

| 验收项 | 目标 |
|---|---|
| 三方账号 proxy 字段 | OpenAPI、DTO、domain、SQL、前端类型一致 |
| OpenAPI contract test | 覆盖每个 management API domain |
| `management/router` fan-out | ≤20 |
| `plugins/actions` fan-out | ≤14 |
| `plugins/actions/dispatch.go` | ≤250 行 |
| migration 策略 | 文档明确，base/snapshot/history 不混用 |
| production package | ≤135 |
| single-file package | ≤30 |
| module.go single-file package | ≤2 |
| server directory count | ≤155 |
| `internal/app/**` import union | ≤50 |
| `render/service` production files | ≤20 |
| manual SQL exception | 有分类、owner、复查日期 |
| `context.Background()` | 非白名单不新增，HTTP handler 不直接使用 |
| secret/cookie/proxy | API、日志、fixtures 不泄露 |
| diagnostics | 有聚合系统诊断接口或明确设计文档 |

---

# 10. 建议执行顺序

推荐按以下顺序执行，不建议并行大爆炸重构：

1. **先修 P0-01/P0-05**：解决已发现的 API 合同漂移。
2. **再修 P0-02**：拆薄 `management/router`，避免新增 API 继续放大组合根。
3. **再修 P0-04**：下沉 plugin action 注册，压缩中央分发表。
4. **再修 P0-03**：正规化 migration，避免继续积累 schema 债务。
5. **之后处理 P1-02/P1-03**：拆 integration common 和 Bilibili module。
6. **最后做 P2 降噪**：单文件 package、泛化文件名、render/service、deps、诊断、工具链。

---

# 11. 备注

当前 server 已经从“无约束膨胀”进入“有预算治理”阶段。下一步不要只追求“目录看起来更模块化”，而要追求：

```text
更少的 package
更少的单文件目录
更低的 fan-out
更明确的 contract 单一事实来源
更干净的 migration 策略
更稳定的插件 action 注册机制
更清晰的 integration provider 边界
更可验证的 OpenAPI/DTO/fixture 同步
```

最终目标不是把 server 拆得更碎，而是让维护者能用更少跳转、更少上下文、更稳定的边界完成变更。
