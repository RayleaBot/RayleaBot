# RayleaBot Server 架构评审与整改清单

> 评审对象：`RayleaBot-main/server`
> 评审类型：静态架构审计、目录结构审计、可维护性评审、安全与工程治理评审
> 输出目标：形成可直接拆分为 issue / milestone / 迭代任务的整改清单
> 备注：本清单基于 server 代码结构、关键路径、配置、存储、插件、管理 API、集成与测试目录的静态分析。由于本地工具链低于项目 `go.mod` 声明版本，未完成完整编译和全量测试运行，因此涉及运行时行为的结论需要在后续 CI 或目标 Go 工具链环境中复核。

---

## 1. 总体结论

当前 `server` 的核心问题不是单纯“文件多”，而是已经出现了结构性复杂度失控的早期迹象：

- 文件数、目录数、package 数偏高；
- 目录维度混杂，技术层、业务域、装配层、API 层并列堆放；
- `internal/app`、`servicegraph`、`httpwire` 等组合根过重；
- 插件、渲染、第三方集成、事件管线等领域已经成长为子系统，但边界仍然不够清晰；
- 配置系统功能强，但存在自动写回、默认值多源、secret 明文返回等严重问题；
- 数据库 schema 演进方式落后，存在 `schema.sql` 与运行时补丁迁移的“双真相源”；
- 管理 API、配置错误、系统状态对运维用户不够友好；
- 测试量大是优点，但部分测试文件过重，维护成本开始上升。

建议整改路线：

```text
先修安全和数据库演进，
再收束 app/servicegraph/httpwire，
再重构配置/API/观测体系，
最后进行领域目录重组和 package 降噪。
```

最高优先级整改项：

1. **配置 API secret 脱敏**：禁止 `/api/config` 或类似接口返回明文 token / secret。
2. **数据库 migration 正规化**：替换 `schema.sql + ensure*Columns` 运行时补丁模式。
3. **降低组合根 fan-out**：让插件、渲染、事件管线、第三方集成等模块自装配，`app` 只负责生命周期。
4. **目录结构降噪**：减少单文件 package、过细文件夹、泛化文件名和无语义拆分。
5. **增强人机交互和运维体验**：配置错误、启动失败、系统健康状态需要结构化、可定位、可排障。

---

## 2. 当前结构量化基线

| 指标 | 当前情况 | 评价 |
|---|---:|---|
| `server` 文件总数 | 约 1043 | 偏高 |
| `server` 目录总数 | 约 169 | 明显偏高 |
| Go 文件总数 | 约 1029 | 偏高 |
| Go 总行数 | 约 12.86 万行 | 中大型 Go 服务规模 |
| 生产 Go 文件 | 约 827 | 拆分较细 |
| 测试 Go 文件 | 约 202 | 测试意识强，但部分文件过重 |
| 生产 package 数 | 约 150 | 对当前规模来说偏碎 |
| 单 Go 文件 package | 约 46 | 过度拆包信号明显 |
| 两个 Go 文件 package | 约 13 | 进一步说明目录颗粒度偏细 |
| `internal` 文件数 | 约 997 | 几乎所有复杂度集中在 `internal` |
| `internal/plugins` 文件数 | 约 237 | 插件域复杂度最高 |
| `internal/management` 文件数 | 约 133 | 管理 API 拆分较碎 |
| `internal/integrations` 文件数 | 约 117 | 第三方集成复杂且边界不够清晰 |
| `internal/render` 文件数 | 约 74 | 渲染域已经成为独立子系统 |
| `internal/bot` 文件数 | 约 64 | OneBot 适配层体量不小 |

### 2.1 重复泛化文件名

| 文件名 | 出现次数 | 问题 |
|---|---:|---|
| `types.go` | 约 20 | 语义过泛，打开前不知道包含什么类型 |
| `routes.go` | 约 11 | 路由职责分散，API 合同难统一 |
| `repository.go` | 约 10 | 数据层命名模板化，业务含义不足 |
| `service.go` | 约 9 | “service” 成为万能抽屉 |
| `http.go` | 约 8 | HTTP 职责表达不精确 |
| `registry.go` | 约 7 | 多个注册中心概念容易冲突 |
| `helpers.go` | 约 7 | 工具函数归属不清 |

### 2.2 目标结构指标

| 指标 | 当前 | 建议目标 |
|---|---:|---:|
| `server/internal` 目录数 | 约 165 | 100 ~ 120 |
| 生产 package 数 | 153 | 80 ~ 100 |
| 单文件 package | 49 | 10 ~ 15 |
| `servicegraph` 直接内部 import | 18 | 20 以下 |
| `httpwire` 直接内部 import | 5 | 20 以下 |
| 单测试文件最大行数 | 约 2000 | 600 左右 |
| 新增泛化文件名 | 较多 | 禁止新增或需要架构评审 |

---

## 3. 严重级别定义

| 级别 | 含义 | 处理要求 |
|---|---|---|
| P0 | 安全、数据一致性、核心架构演进风险，可能造成泄露、升级失败、维护崩溃 | 立即进入最近一个迭代，优先修复 |
| P1 | 明显影响维护效率、扩展效率、排障效率或长期架构质量 | 1 ~ 2 个迭代内规划整改 |
| P2 | 可读性、规范性、体验性、局部复杂度问题 | 随功能迭代逐步治理 |
| P3 | 风格优化、命名优化、长期演进建议 | 结合重构窗口处理 |

---

## 4. P0 整改清单

### P0-01：配置接口返回明文 secret / token

| 字段 | 内容 |
|---|---|
| 问题 | 配置读取接口存在返回 OneBot token、webhook secret、reverse/forward/http secret 等明文字段的风险；脱敏逻辑基本没有真正生效。 |
| 风险 | 管理端页面、浏览器 DevTools、日志、代理、截图、前端错误上报都可能泄露密钥。配置查看权限等价于密钥读取权限。 |
| 涉及位置 | `internal/configruntime/document_service.go`、`internal/management/configapi`、配置 fixtures、集成测试。 |
| 整改动作 | 实现配置脱敏；GET 配置只返回掩码和 `redacted_fields`；PUT/PATCH 配置支持“不提交则保留旧 secret，提交新值则替换，提交空值则清除”。 |
| 验收标准 | `/api/config` 不返回任何真实 token；测试覆盖所有 secret 字段；fixtures 不再固化明文返回；日志中不出现 token。 |
| 建议优先级 | 最高。 |
| 状态 | 已完成。 |

建议返回结构示例：

```json
{
  "config": {
    "onebot": {
      "reverse": {
        "access_token": "********"
      }
    }
  },
  "redacted_fields": [
    "onebot.reverse.access_token"
  ]
}
```

---

### P0-02：数据库 schema 演进缺少正式 migration

| 字段 | 内容 |
|---|---|
| 问题 | 数据库 schema 演进需要由版本化 migration 单一路径管理。 |
| 风险 | 新库、旧库、测试库、线上库可能 schema 不一致；排障时无法判断数据库真实版本；字段新增容易遗漏。 |
| 涉及位置 | `internal/storage/migrations/`、`store_schema.go`、`server/sqlc.yaml`。 |
| 整改动作 | 引入正式 migration 目录和 `schema_migrations` 表；将运行时补丁迁移成显式版本脚本。 |
| 验收标准 | 新库和老库均通过 migration 初始化或升级；无运行时隐式补列；CI 校验 migration 可重复执行；sqlc 输入与 migration 最终状态一致。 |
| 建议优先级 | 最高。 |
| 状态 | 已完成。 |

建议目录：

```text
internal/storage/migrations/
  000001_base.sql
  000002_add_third_party_account_columns.sql
  000003_expand_third_party_account_platforms.sql
  000004_add_bilibili_source_room_cover_url.sql
```

建议 migration 表：

```sql
CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);
```

---

### P0-03：`app/servicegraph/httpwire` 组合根过重

| 字段 | 内容 |
|---|---|
| 问题 | `internal/app/servicegraph`、`httpwire`、`App.New()` 等装配路径导入大量内部包，理解和修改成本高。 |
| 风险 | 任一中型功能变更都可能穿透 app、servicegraph、stack、httpwire、runtime state 多层结构；改动放大效应明显。 |
| 涉及位置 | `internal/app`、`internal/app/servicegraph`、`internal/app/httpwire`、`internal/app/*stack`。 |
| 整改动作 | 引入模块自装配机制；插件、渲染、事件管线、第三方集成各自暴露 `Module`；`app` 只负责生命周期和模块注册。 |
| 验收标准 | `servicegraph` 和 `httpwire` 直接内部 import 数降低到 20 以下；新增业务功能不需要修改多个 app 装配文件；模块具备独立测试入口。 |
| 建议优先级 | 高。 |
| 状态 | 已完成。 |

目标：

```text
internal/app
  app.go             # 启动/停止
  lifecycle.go       # 统一 goroutine 生命周期
  modules.go         # 模块注册

internal/plugin/module.go
internal/render/module.go
internal/eventpipeline/module.go
internal/integration/onebot/module.go
```

---

### P0-04：Go 工具链版本门槛高且失败提示不友好

| 字段 | 内容 |
|---|---|
| 问题 | `go.mod` 声明 `go 1.25.8` 并使用较新的 Go module 指令，低版本 Go 无法解析或测试。 |
| 风险 | 新人、CI、离线环境、运维排障环境容易先卡在工具链问题，无法快速进入业务问题定位。 |
| 涉及位置 | `server/go.mod`、CI、开发文档、Makefile。 |
| 整改动作 | 增加 `make doctor` / `check-toolchain.sh` / `.tool-versions` / 开发容器；在错误信息中明确当前版本和要求版本。 |
| 验收标准 | 任意开发者 clone 后执行 `make doctor` 可获得明确环境诊断；CI 使用固定 Go 版本；README 有明确安装路径。 |
| 建议优先级 | 高。 |
| 状态 | 已完成。 |

参考：Go module `go` 指令与 `tool` 指令说明见 Go 官方文档：<https://go.dev/ref/mod>

---

## 5. P1 整改清单：目录结构与文件组织

### P1-01：顶级 `internal` 目录维度混杂

| 字段 | 内容 |
|---|---|
| 问题 | `internal` 下同时存在启动维度、业务域、技术层、API 层、协议层、配置层、测试工具等多个维度。 |
| 风险 | 新人无法从目录名判断功能归属；改动路径长；跨包跳转频繁。 |
| 涉及位置 | `internal/app`、`auth`、`bot`、`bridge`、`dispatch`、`eventingress`、`management`、`plugins`、`render`、`thirdparty` 等。 |
| 整改动作 | 改为“领域优先 + 平台基础设施”结构；将事件管线、插件、渲染、集成、管理 HTTP、平台能力分组。 |
| 验收标准 | 一条 OneBot 消息从接收到回复的主路径可以在一个领域树内阅读；顶级目录数量减少；目录名表达业务边界。 |
| 状态 | 已完成。 |

建议目标结构：

```text
internal/
  app/
  platform/
  http/
  plugin/
  eventpipeline/
  integration/
  render/
  task/
  scheduler/
  governance/
  testkit/
```

---

### P1-02：单文件 package 过多

| 字段 | 内容 |
|---|---|
| 问题 | 存在大量只有一个 `.go` 文件的 package。 |
| 风险 | import 路径膨胀；跳转层级变深；抽象收益低于维护成本；循环依赖风险上升。 |
| 整改动作 | 制定拆包规则；不满足独立生命周期、多个调用方、独立测试价值等条件的小包合并回上级包。 |
| 验收标准 | 单文件 package 从约 46 降到 10 ~ 15；新增单文件 package 需要架构说明。 |
| 状态 | 已完成。 |

拆包规则建议：

```text
只有满足以下至少两条，才允许独立 package：
1. 有清晰领域模型；
2. 有独立生命周期；
3. 有多个调用方；
4. 有独立测试价值；
5. 未来需要替换或扩展；
6. 内部文件数预计超过 4 个。
```

---

### P1-03：泛化文件名过多

| 字段 | 内容 |
|---|---|
| 问题 | `types.go`、`service.go`、`routes.go`、`repository.go`、`helpers.go` 等泛化命名大量重复。 |
| 风险 | 文件语义不清；搜索结果噪声大；review 时难以判断变更范围。 |
| 整改动作 | 文件名改为表达业务职责；禁止新增无语义泛化文件名。 |
| 验收标准 | 新增文件名能直接表达职责；架构测试禁止新增 `helpers.go` 或空泛 `service.go`；旧文件逐步重命名。 |
| 状态 | 已整改：OneBot11 访问令牌默认走 `Authorization: Bearer`；`access_token_query_compat` 仅作为兼容开关；HTTP / WebSocket 日志不记录 raw query。 |

推荐命名示例：

```text
plugin_catalog.go
plugin_lifecycle.go
render_template_store.go
config_snapshot_handler.go
bilibili_session_store.go
onebot_ws_adapter.go
```

---

### P1-04：`thirdparty` 与 `integrations` 概念重叠

| 字段 | 内容 |
|---|---|
| 问题 | 第三方账号、平台客户端、平台 API、管理端接口曾分散在 `internal/thirdparty`、`internal/integrations/*`、`management/thirdpartyapi`、`management/bilibiliapi` 等目录。 |
| 风险 | 第三方账号、平台客户端、平台 API、管理端接口职责边界不清；新增平台时归属不明确。 |
| 整改动作 | 统一到 `internal/integrations` 领域；平台公共账号能力放到 `internal/integrations/thirdparty`；各平台内部再分 client/session/source。 |
| 验收标准 | 新增平台只需在 `internal/integrations/<platform>` 下扩展；管理 API 不直接依赖平台内部实现细节。 |
| 状态 | 已完成结构归并：第三方账号模型、持久化和 secret reference 已移动到 `internal/integrations/thirdparty`。 |

建议结构：

```text
internal/integrations/
  thirdparty/
  bilibili/
    client/
    session/
    source/
    management/
  douyin/
  weibo/
  netease/
  common/
```

---

## 6. P1 整改清单：配置系统

### P1-05：配置加载存在写文件副作用

| 字段 | 内容 |
|---|---|
| 问题 | 配置加载过程中可能 canonicalize 并写回配置文件或模板文件。 |
| 风险 | 服务启动修改用户配置；YAML 注释丢失；Git diff 出现程序修改；容器只读配置挂载时出错。 |
| 整改动作 | 服务启动路径只读配置；初始化、格式化、规范化改为显式 CLI 命令。 |
| 验收标准 | `server run` 不主动修改配置文件；`config init`、`config normalize`、`config validate` 分别完成初始化、格式化、校验。 |
| 状态 | 已完成。 |

建议命令：

```text
raylea-server config init
raylea-server config normalize
raylea-server config validate
raylea-server run
```

---

### P1-06：默认值来源分散

| 字段 | 内容 |
|---|---|
| 问题 | 默认值可能来自 `default.yaml`、schema、canonical document、typed config、fixtures、前端假设等多个位置。 |
| 风险 | 文档、schema、运行时、UI 默认值不一致；测试 fixture 老化。 |
| 整改动作 | 确定唯一默认值来源；其他文件通过生成或校验保持一致。 |
| 验收标准 | 一个字段的默认值只有一个权威来源；CI 校验默认值一致性。 |
| 状态 | 已完成。 |

可选方案：

```text
方案 A：config schema + default template 是唯一源，Go typed config 只解析。
方案 B：Go typed defaults 是唯一源，通过命令生成 schema/default.yaml。
```

---

### P1-07：配置热更新策略容易遗漏

| 字段 | 内容 |
|---|---|
| 问题 | 配置变更影响通过路径 diff 判断 reload / restart，新增字段如果忘记登记，容易出现错误热更新策略。 |
| 风险 | 本应重启的配置被热更新；本应热更新的配置要求重启；线上行为不可预测。 |
| 整改动作 | 每个配置字段声明 apply policy。 |
| 验收标准 | schema 中每个字段都有 `hot_reload`、`restart_required`、`secret_only`、`read_only` 等策略；测试扫描缺失策略。 |
| 状态 | 已完成。 |

策略示例：

```yaml
onebot.reverse.ws_url:
  apply_policy: restart_required
onebot.reverse.access_token:
  apply_policy: secret_only
logging.level:
  apply_policy: hot_reload
```

---

### P1-08：配置校验错误不够面向用户

| 字段 | 内容 |
|---|---|
| 问题 | 管理 API 配置更新失败时，错误信息可能过于泛化，例如只返回 `invalid_config`。 |
| 风险 | 用户不知道哪个字段错、为什么错、怎么修。 |
| 整改动作 | 返回字段路径、原因、期望格式、是否需要重启、修复建议。 |
| 验收标准 | 前端可以直接展示字段级错误；运维截图即可定位配置项。 |
| 状态 | 已完成。 |

建议错误结构：

```json
{
  "code": "invalid_config",
  "message": "配置校验失败",
  "details": [
    {
      "path": "onebot.reverse.ws_url",
      "reason": "must be a valid websocket URL",
      "hint": "请填写 ws:// 或 wss:// 开头的地址"
    }
  ]
}
```

---

## 7. P1 整改清单：数据库与存储层

### P1-09：单体 `schema.sql` 已经过大

| 字段 | 内容 |
|---|---|
| 问题 | SQLite 表结构由版本化 migration 管理，避免单体 `schema.sql` 混合多个领域。 |
| 风险 | code review 难看出变更影响；merge conflict 增多；历史演进不可追踪。 |
| 整改动作 | 不仅拆文件，更要引入版本化 migration。 |
| 验收标准 | 每个 schema 变更都有 migration 文件、测试、版本号、回归用例。 |
| 状态 | 已完成。 |

---

### P1-10：sqlc 与手写 SQL 混用缺少制度边界

| 字段 | 内容 |
|---|---|
| 问题 | 项目已有 sqlc 查询，也因 SQLite parser 支持问题保留了部分手写 SQL。 |
| 风险 | SQL 注入风险、查询风格不统一、变更难追踪。 |
| 整改动作 | 约定默认使用 sqlc；手写 SQL 必须注明原因、测试覆盖、注入风险说明。 |
| 验收标准 | 手写 SQL 集中在 `manualsql` 或明确目录；CI 可扫描未登记手写 SQL。 |
| 状态 | 待整改。 |

建议结构：

```text
internal/storage/
  migrations/
  queries/        # sqlc input
  sqlcgen/        # generated
  repo/           # repository implementation
  manualsql/      # documented exceptions
```

参考：sqlc 官方文档说明 sqlc 不执行 migration，但支持解析多种 migration 工具格式：<https://docs.sqlc.dev/en/latest/howto/ddl.html>

---

### P1-11：repository/service 模式存在模板化倾向

| 字段 | 内容 |
|---|---|
| 问题 | 很多目录强行出现 `repository.go`、`service.go`，但不一定都有独立业务价值。 |
| 风险 | 层数增加但抽象收益低；小功能修改需要穿过多层；调用链冗长。 |
| 整改动作 | 简单 CRUD 保持薄 service；复杂业务保留 domain/application service；避免空转层。 |
| 验收标准 | repository 不返回 API DTO；handler 不拼复杂业务状态；service 有明确业务含义。 |
| 状态 | 待整改。 |

---

## 8. P1 整改清单：HTTP / API / 管理端

### P1-12：管理 API 合同约束偏弱

| 字段 | 内容 |
|---|---|
| 问题 | 虽然有 OpenAPI 和 fixtures，但 server handler 与 OpenAPI 没有足够强的编译期或 CI 约束。 |
| 风险 | API、前端类型、fixtures、实际响应结构可能漂移。 |
| 整改动作 | 引入 OpenAPI 校验流程；route、response fixture、前端类型统一生成或 CI 对比。 |
| 验收标准 | 新增 API 必须更新 OpenAPI；CI 校验实际响应和合同一致；前端类型不再手写漂移。 |
| 状态 | 待整改。 |

---

### P1-13：API 错误模型不统一

| 字段 | 内容 |
|---|---|
| 问题 | 各 handler 可能各自写错误响应，错误码、message、details、request_id 不统一。 |
| 风险 | 前端处理复杂；用户错误截图无法定位日志；运维排障效率低。 |
| 整改动作 | 统一错误响应结构。 |
| 验收标准 | 所有管理 API 错误均包含 `code`、`message`、`request_id`、可选 `details`；日志可按 request_id 检索。 |
| 状态 | 待整改。 |

建议结构：

```json
{
  "code": "plugin_not_found",
  "message": "插件不存在",
  "request_id": "req_xxx",
  "details": {}
}
```

---

### P1-14：通用 HTTP 指标和访问日志不足

| 字段 | 内容 |
|---|---|
| 问题 | 已有业务 metrics，但通用 HTTP request count、status、latency、route pattern、panic count 等指标不明显。 |
| 风险 | 接口变慢、错误率上升、认证失败、panic 等问题不容易第一时间定位。 |
| 整改动作 | 增加统一 HTTP middleware：request ID、访问日志、Prometheus 指标、panic recover、query redaction。 |
| 验收标准 | 每条请求有 request_id；metrics 按 route/status/method 统计；日志不打印 raw token query。 |
| 状态 | 已完成。 |

参考：chi 兼容标准 `net/http` middleware，可继续使用，无需替换路由框架：<https://github.com/go-chi/chi>

---

### P1-15：管理端 route 测试维护成本高

| 字段 | 内容 |
|---|---|
| 问题 | route 测试中维护大量硬编码路径。 |
| 风险 | 新增/删除 API 时测试维护负担大；OpenAPI 和 route 列表可能不一致。 |
| 整改动作 | 从 OpenAPI 生成 expected route，或 router 注册表结构化输出供测试对比。 |
| 验收标准 | route 测试不再手写维护大段路径；新增 API 只需改一个合同源。 |
| 状态 | 待整改。 |

---

## 9. P1 整改清单：插件系统

### P1-16：`plugins` 成为全局中心包

| 字段 | 内容 |
|---|---|
| 问题 | `internal/plugins` 被大量内部包导入，承载 manifest、snapshot、状态、UI、render templates、actions、capabilities、dependencies 等多类职责。 |
| 风险 | 插件领域成为上帝模型；新增字段容易忘记 clone/mapping；runtime 与 management view 混杂。 |
| 整改动作 | 拆分插件模型：manifest、installation、runtime state、capability、management view、render binding。 |
| 验收标准 | 底层 runtime 不依赖管理端展示 DTO；API view 与内部状态有明确转换层；插件包 fan-in 降低。 |
| 状态 | 待整改。 |

建议模型：

```text
plugin.Manifest
plugin.Installation
plugin.RuntimeState
plugin.CapabilitySet
plugin.ManagementView
plugin.RenderBinding
```

---

### P1-17：`plugins/actions/*action` 小包过多

| 字段 | 内容 |
|---|---|
| 问题 | 许多 action 被拆成独立小目录，目录层级膨胀。 |
| 风险 | 小改动需要跨目录；新增 action 容易复制模板；dispatcher 变重。 |
| 整改动作 | 简单 action 合并到 `plugin/action` 包内；复杂 action 再独立子包。 |
| 验收标准 | action 目录数下降；新增简单 action 不新增 package；复杂 action 有明确独立理由。 |
| 状态 | 待整改。 |

建议结构：

```text
plugin/action/
  dispatcher.go
  config.go
  secret.go
  storage.go
  http.go
  scheduler.go
  render.go
  onebot.go
```

---

### P1-18：action dispatcher 容易继续中心化

| 字段 | 内容 |
|---|---|
| 问题 | action kind 注册集中在 dispatcher，未来会成为新的 service locator。 |
| 风险 | 新增 render/scheduler/onebot action 都要修改插件 action 总表。 |
| 整改动作 | 改为模块注册 action。 |
| 验收标准 | 新增某领域 action 只修改该领域模块；dispatcher 只负责注册表和调度。 |
| 状态 | 待整改。 |

建议接口：

```go
type ActionModule interface {
    RegisterActions(registry *actions.Registry)
}
```

---

### P1-19：插件状态模型可能重复

| 字段 | 内容 |
|---|---|
| 问题 | 插件状态、runtime manager 状态、展示状态、project state 等概念分布多处。 |
| 风险 | 内部状态和 UI 状态不一致；新增状态忘记映射；日志/metrics/API 命名不一致。 |
| 整改动作 | 定义唯一状态源，并建立内部状态到 API view 的显式映射。 |
| 验收标准 | 状态枚举集中定义；API、日志、metrics 统一命名；新增状态必须补测试。 |
| 状态 | 待整改。 |

建议状态：

```text
RuntimeState:
  stopped
  starting
  running
  degraded
  stopping
  failed

InstallState:
  not_installed
  installing
  installed
  upgrading
  uninstalling
  failed

HealthState:
  healthy
  warning
  error
```

---

## 10. P1 整改清单：事件管线、协议与 OneBot

### P1-20：消息事件主流程分散

| 字段 | 内容 |
|---|---|
| 问题 | `eventingress`、`chatpolicy`、`bridge`、`dispatch`、`outbound`、`protocolcap`、`bot/adapter/onebot11` 等共同参与事件链路，曾分散在多个顶级包。 |
| 风险 | 无法快速理解“一条消息从进入到回复”的主流程；跨包耦合高。 |
| 整改动作 | 收束到 `internal/eventpipeline` 下。 |
| 验收标准 | 有明确事件流主轴；新增策略、分发、出站能力只在事件管线领域内扩展。 |
| 状态 | 已完成：`eventingress`、`chatpolicy`、`bridge`、`dispatch`、`outbound` 已移动到 `internal/eventpipeline/`，并由架构测试阻止旧顶级包恢复。 |

建议结构：

```text
internal/eventpipeline/
  eventingress/
  chatpolicy/
  bridge/
  dispatch/
  outbound/
```

或：

```text
internal/integration/onebot/
  adapter/
  protocol/
  ingress/
  outbound/
```

---

### P1-21：URL query token 使用风险

| 字段 | 内容 |
|---|---|
| 问题 | OneBot / WebSocket 相关逻辑中存在 query 参数 token 兼容模式。 |
| 风险 | URL 可能进入日志、代理、错误上报、浏览器历史，导致 token 泄露。 |
| 整改动作 | 优先使用 `Authorization: Bearer`；query token 仅作为兼容模式；所有 URL 日志先 redact。 |
| 验收标准 | raw query 不进入日志；query token 有配置开关；默认文档推荐 Authorization header。 |
| 状态 | 待整改。 |

---

## 11. P1 整改清单：渲染系统

### P1-22：render 已是子系统，但边界仍需强化

| 字段 | 内容 |
|---|---|
| 问题 | `internal/render` 包含 artifact、browser、catalog、engine、pluginsync、templates、repository、service 等能力，与 plugin、management、app 有耦合。 |
| 风险 | 渲染逻辑、模板同步、插件模板、管理 API 互相影响；浏览器依赖问题难排查。 |
| 整改动作 | render 模块只暴露少量接口：RenderService、TemplateRegistry、ArtifactStore、HealthSnapshot。 |
| 验收标准 | plugin 不直接操作 render 内部 repository；management 不理解 engine 细节；app 只启动 render module。 |
| 状态 | 待整改。 |

建议接口边界：

```text
render.RenderService
render.TemplateRegistry
render.ArtifactStore
render.HealthSnapshot
```

---

### P1-23：浏览器渲染依赖的运维可观测性不足

| 字段 | 内容 |
|---|---|
| 问题 | chromedp/browser 类依赖对环境要求高，但健康状态暴露不够集中。 |
| 风险 | 线上渲染失败时难以判断是浏览器路径、启动参数、模板、超时、队列还是依赖问题。 |
| 整改动作 | 在 health snapshot 中暴露浏览器路径、启动参数、最近错误、模板同步状态、队列长度、平均耗时。 |
| 验收标准 | 管理端可直接查看 render 子系统健康状态；日志包含 request_id / render_id。 |
| 状态 | 待整改。 |

---

## 12. P1 整改清单：第三方集成

### P1-24：Bilibili 集成体量过大，需作为独立 bounded context

| 字段 | 内容 |
|---|---|
| 问题 | `internal/integrations/bilibili` 文件量大，包含 account usage、session、source、dynamic、值对象等多个概念。 |
| 风险 | 平台内部复杂度外溢；管理 API、存储、调度、账号状态耦合。 |
| 整改动作 | 将 Bilibili 拆成 account/client/session/source/dynamic/storage/management 等子域。 |
| 验收标准 | Bilibili 内部职责清晰；其他包只依赖 Bilibili 对外接口，不依赖内部文件结构。 |
| 状态 | 待整改。 |

建议结构：

```text
internal/integration/bilibili/
  account/
  client/
  dynamic/
  live/
  source/
  session/
  storage/
  management/
```

---

### P1-25：Douyin 浏览器自动化文件过大

| 字段 | 内容 |
|---|---|
| 问题 | Douyin 相关 browser/resolve 文件体量偏大。 |
| 风险 | 浏览器控制、URL 解析、内容提取、错误处理混在一起，难以测试和维护。 |
| 整改动作 | 拆分 browser session、page resolver、URL normalizer、extraction、errors。 |
| 验收标准 | 单文件行数下降；浏览器控制与业务解析可分别测试。 |
| 状态 | 待整改。 |

建议结构：

```text
douyin/
  browser_session.go
  page_resolver.go
  url_normalizer.go
  extraction.go
  errors.go
```

---

## 13. P1 整改清单：安全治理

### P1-26：secret 存储策略不统一

| 字段 | 内容 |
|---|---|
| 问题 | 项目已有 `secret_store`，但部分 OneBot、webhook、transport token 仍直接出现在配置文档中。 |
| 风险 | secret 分散，权限、脱敏、轮换、审计策略难统一。 |
| 整改动作 | 配置文件保存 secret reference，真实值进入 secret store 或环境变量。 |
| 验收标准 | 配置中不直接保存高敏 secret；secret 读取、更新、审计有统一入口。 |
| 状态 | 待整改。 |

建议：

```yaml
onebot:
  reverse:
    access_token_ref: secret://onebot/reverse/access_token
```

---

### P1-27：日志脱敏需要强制化

| 字段 | 内容 |
|---|---|
| 问题 | 项目已有 redaction 能力，但需要保证所有 HTTP/WS/配置/第三方集成日志都使用。 |
| 风险 | token、cookie、session、proxy URL、账号信息进入日志。 |
| 整改动作 | 建立统一 logger wrapper；敏感字段列表集中维护；CI 扫描常见敏感字段日志输出。 |
| 验收标准 | 日志测试覆盖 token、cookie、authorization、access_token、secret、proxy_url 等字段。 |
| 状态 | 待整改。 |

---

### P1-28：包初始化阶段不应直接 `log.Fatalf`

| 字段 | 内容 |
|---|---|
| 问题 | 部分包初始化逻辑在解析硬编码网络前缀失败时直接 `log.Fatalf`。 |
| 风险 | 库代码 init 阶段退出进程，难以测试和恢复。 |
| 整改动作 | 改为构造函数返回 error，或使用 `mustParse` + 明确 panic，并有单元测试覆盖。 |
| 验收标准 | 非 main 包不调用 `log.Fatal` / `os.Exit`；架构测试可扫描。 |
| 状态 | 待整改。 |

---

## 14. P1 整改清单：测试与工程治理

### P1-29：部分测试文件过大

| 字段 | 内容 |
|---|---|
| 问题 | 存在超过 1000 行甚至接近 2000 行的测试文件。 |
| 风险 | 场景互相干扰；失败难定位；重构时需要理解过多上下文。 |
| 整改动作 | 按场景拆分测试文件；fixture 初始化复用但不要集中巨型测试。 |
| 验收标准 | 新增测试文件建议低于 600 行；旧大文件逐步拆分。 |
| 状态 | 待整改。 |

建议拆法：

```text
app_run_policy_reload_test.go
app_run_plugin_actions_test.go
app_run_shutdown_test.go
management_config_http_test.go
management_plugin_http_test.go
```

---

### P1-30：架构测试需要增加结构预算

| 字段 | 内容 |
|---|---|
| 问题 | 已有架构测试方向正确，但还缺 package 数、单文件 package、fan-out、secret、schema drift 等关键预算。 |
| 风险 | 项目继续增长时，复杂度无门禁，目录和依赖会继续恶化。 |
| 整改动作 | 增加结构治理测试。 |
| 验收标准 | package 数、单文件 package 数、app fan-out、泛化文件名、schema drift、secret 明文均有 CI 门禁。 |
| 状态 | 部分完成：package 数、单文件 package、app fan-out、泛化文件名和测试体量已有预算；OpenAPI route 漂移门禁待处理。 |

建议测试项：

```text
生产 package 总数不得增加，除非白名单；
单文件 package 不得增加；
internal/app/servicegraph 直接 import 数不得增加；
新增 config 字段必须有 apply policy；
新增 API route 必须在 OpenAPI 中出现；
fixtures 不允许包含真实格式 secret；
非 main 包禁止 log.Fatal/os.Exit。
```

---

## 15. P2 整改清单：可读性、人机交互与运维体验

### P2-01：启动失败信息需要更面向运维

| 字段 | 内容 |
|---|---|
| 问题 | 启动失败如果只输出底层 error，不利于运维定位。 |
| 整改动作 | 启动失败统一包装，输出配置路径、数据库路径、schema 版本、端口、浏览器依赖、OneBot 连接状态等上下文。 |
| 验收标准 | 常见启动失败能直接从日志看出原因和修复建议。 |
| 状态 | 待整改。 |

应包含信息：

```text
- 配置文件路径
- schema 校验错误
- 数据库路径
- migration 当前版本 / 目标版本
- 端口占用
- 浏览器依赖缺失
- OneBot 连接失败原因
- 插件运行时依赖缺失
```

---

### P2-02：需要统一系统状态总览

| 字段 | 内容 |
|---|---|
| 问题 | 各子系统可能各自有状态，但缺少一个面向运维的聚合健康视图。 |
| 整改动作 | 提供 `system health snapshot` API。 |
| 验收标准 | 管理端首页能展示 server、database、schema、onebot、plugins、render、scheduler、third_party_accounts 等状态。 |
| 状态 | 待整改。 |

示例：

```json
{
  "server": "ok",
  "database": "ok",
  "schema_version": "000012",
  "onebot": "connected",
  "plugins": {
    "running": 12,
    "failed": 1
  },
  "render": "degraded",
  "scheduler": "ok",
  "third_party_accounts": {
    "bilibili": "expired"
  }
}
```

---

### P2-03：管理端配置变更需要明确“是否需要重启”

| 字段 | 内容 |
|---|---|
| 问题 | 用户修改配置后可能不知道是否已生效、是否需要重启。 |
| 整改动作 | 配置 diff 返回影响范围。 |
| 验收标准 | API 返回 `applied`、`restart_required`、`reload_failed` 等结果；UI 明确提示用户。 |
| 状态 | 待整改。 |

---

### P2-04：日志与 API 错误应通过 request_id 串联

| 字段 | 内容 |
|---|---|
| 问题 | 用户看到 API 错误时，如果没有 request_id，排障需要猜测时间窗口。 |
| 整改动作 | 所有 HTTP 请求生成 request_id；错误响应返回 request_id；日志带 request_id。 |
| 验收标准 | 用户截图错误后，运维可直接按 request_id 搜索日志。 |
| 状态 | 待整改。 |

---

## 16. 技术栈评审

### 16.1 不建议整体替换

| 当前技术 | 是否建议替换 | 结论 |
|---|---|---|
| Go | 不建议 | 服务端、并发、插件 runtime、单二进制部署都适合。 |
| chi | 不建议 | 轻量、兼容 `net/http` middleware，当前足够。 |
| SQLite | 暂不建议 | 单机产品、轻部署场景适合；已使用 versioned migration 管理 schema 演进。 |
| slog | 不建议 | 标准库方向，足够使用。 |
| Prometheus | 不建议 | 已有指标基础，应补 HTTP 和子系统指标。 |
| sqlc | 不建议 | 显式 SQL 风格比 ORM 更适合当前项目。 |

### 16.2 建议引入或替换的能力

| 领域 | 当前问题 | 建议 |
|---|---|---|
| DB migration | 已使用内置 migration runner | 后续 schema 变更继续新增版本化 migration。 |
| OpenAPI 落地 | 合同存在，但 server 手写响应 | 引入生成或 CI 校验流程。 |
| 配置 secret | 明文配置和 API 返回 | 引入 secret reference / 脱敏机制。 |
| 生命周期管理 | `App.Run()` 管太多 goroutine | 建立统一 lifecycle supervisor。 |
| 架构治理 | 只限制部分 import / 行数 | 增加 package、fan-out、schema drift、secret 门禁。 |
| HTTP 观测 | 业务 metrics 多，通用 HTTP metrics 不明显 | 增加统一 HTTP middleware。 |

---

## 17. 建议目标目录结构

```text
server/
  cmd/
    raylea-server/

  internal/
    app/
      app.go
      lifecycle.go
      modules.go

    platform/
      config/
      auth/
      storage/
      logging/
      metrics/
      runtimepaths/

    http/
      management/
        router.go
        middleware/
        handlers/
        dto/
      public/
      ws/

    plugin/
      manifest/
      catalog/
      install/
      lifecycle/
      runtime/
      action/
      storage/
      webhook/
      managementview/

    eventpipeline/
      ingress/
      policy/
      bridge/
      dispatch/
      outbound/

    integration/
      onebot/
        adapter/
        protocol/
        outbound/
      bilibili/
        account/
        session/
        source/
        client/
      douyin/
      weibo/
      netease/
      common/

    render/
      engine/
      browser/
      template/
      artifact/
      service/

    task/
    scheduler/
    governance/
    testkit/
```

迁移原则：

```text
不要一次性大爆炸重构；
先建立新边界；
新代码进入新结构；
旧代码通过适配层迁移；
每次迁移都必须有测试和架构预算保护。
```

---

## 18. 分阶段整改计划

### 阶段 1：止血与安全修复

| 编号 | 任务 | 级别 | 验收标准 |
|---|---|---|---|
| S1-01 | 修复配置 API 明文 secret 返回 | P0 | GET 配置不返回真实 secret；测试覆盖。 |
| S1-02 | 日志 raw query / token 脱敏 | P0 | access_token、authorization、cookie 不进日志。 |
| S1-03 | 增加 `make doctor` 工具链检查 | P0 | 工具链版本错误有明确提示。 |
| S1-04 | 增加 secret fixture 扫描 | P1 | fixtures 不含真实格式 token。 |
| S1-05 | 增加非 main 包禁止 `log.Fatal/os.Exit` 检查 | P1 | CI 可拦截。 |

### 阶段 2：数据库 migration 正规化

| 编号 | 任务 | 级别 | 验收标准 |
|---|---|---|---|
| S2-01 | 建立 `schema_migrations` 表 | P0 | 数据库可记录版本。 |
| S2-02 | 将当前 schema 设为 `000001_base.sql` | P0 | 新库通过 migration 初始化。 |
| S2-03 | 将 `ensure*Columns` 改为 migration | P0 | 运行时不再隐式补列。 |
| S2-04 | CI 校验 migration 可从空库跑通 | P1 | 每次提交自动验证。 |
| S2-05 | sqlc 输入与最终 schema 对齐 | P1 | 查询生成稳定。 |

### 阶段 3：配置系统治理

| 编号 | 任务 | 级别 | 验收标准 |
|---|---|---|---|
| S3-01 | 服务启动路径只读配置 | P1 | `run` 不改写配置文件。 |
| S3-02 | 增加 `config init/normalize/validate` | P1 | 配置初始化和格式化显式执行。 |
| S3-03 | 确定唯一默认值来源 | P1 | 默认值无多源漂移。 |
| S3-04 | 每个配置字段增加 apply policy | P1 | CI 扫描无遗漏。 |
| S3-05 | 配置错误返回字段级 details | P1 | UI 可定位字段错误。 |

### 阶段 4：组合根瘦身

| 编号 | 任务 | 级别 | 验收标准 |
|---|---|---|---|
| S4-01 | 为 plugin/render/eventpipeline/integration 定义 Module | P0 | app 只注册模块。 |
| S4-02 | 降低 `servicegraph` import fan-out | P0 | 直接内部 import 小于 20。 |
| S4-03 | 降低 `httpwire` import fan-out | P0 | 直接内部 import 小于 20。 |
| S4-04 | 建立 lifecycle supervisor | P1 | goroutine 生命周期统一管理。 |
| S4-05 | 模块各自提供 health snapshot | P1 | 系统状态可聚合。 |

### 阶段 5：目录降噪和领域重组

| 编号 | 任务 | 级别 | 验收标准 |
|---|---|---|---|
| S5-01 | 合并低价值单文件 package | P1 | 单文件 package 降至 10 ~ 15。 |
| S5-02 | 收束事件管线目录 | P1 | 消息主流程清晰。 |
| S5-03 | 重组 plugin 子系统 | P1 | manifest/runtime/action/management 分离。 |
| S5-04 | 重组 integration / thirdparty | P1 | 平台账号和平台客户端边界清晰。 |
| S5-05 | 泛化文件名治理 | P2 | 新增文件名表达业务职责。 |

### 阶段 6：API、观测和测试治理

| 编号 | 任务 | 级别 | 验收标准 |
|---|---|---|---|
| S6-01 | 统一 API 错误模型 | P1 | 所有错误含 code/message/request_id。 |
| S6-02 | 增加 HTTP metrics 和 access log | P1 | request count/status/latency 可观测。 |
| S6-03 | OpenAPI 与 route/fixture 校验 | P1 | 合同漂移 CI 失败。 |
| S6-04 | 拆分超大测试文件 | P2 | 新测试文件建议不超过 600 行。 |
| S6-05 | 增加结构预算测试 | P1 | package、fan-out、secret、schema drift 有门禁。 |

---

## 19. 可直接创建的 Issue 清单

下面清单可直接复制到项目管理工具中。

### 安全类

- [x] 修复 `/api/config` 明文 secret 返回问题。
- [x] 配置 GET 接口增加 `redacted_fields`。
- [x] 配置 PUT 接口支持 secret 保留、替换、清除语义。
- [x] 日志系统强制脱敏 `access_token`、`authorization`、`cookie`、`secret`、`proxy_url`。
- [x] 禁止 raw query 进入 HTTP / WebSocket 日志。
- [x] fixtures 中禁止出现真实格式 secret。
- [x] query token 改为兼容模式，默认推荐 Authorization header。
- [x] secret 统一进入 secret store 或 secret reference。

### 数据库类

- [x] 新增 `schema_migrations` 表。
- [x] 将当前 `schema.sql` 固化为 `000001_base.sql`。
- [x] 将 `ensureThirdPartyAccountColumns` 转换为 migration。
- [x] 将 `ensureBilibiliSourceRoomColumns` 转换为 migration。
- [x] 删除或降级运行时 schema 补丁逻辑。
- [x] 增加空库 migration 测试。
- [x] 增加旧库升级 migration 测试。
- [x] sqlc 输入与 migration 最终 schema 对齐。
- [x] 手写 SQL 例外集中登记。

### 架构类

- [x] 为 plugin 模块定义 `Module` 装配接口。
- [x] 为 render 模块定义 `Module` 装配接口。
- [x] 为 eventpipeline 模块定义 `Module` 装配接口。
- [x] 为 integration 模块定义 `Module` 装配接口。
- [x] 降低 `internal/app/servicegraph` 直接 import 数。
- [x] 降低 `internal/app/httpwire` 直接 import 数。
- [x] 建立统一 lifecycle supervisor。
- [x] 增加 app fan-out 架构测试。
- [x] 增加单文件 package 数量预算。
- [x] 增加生产 package 总数预算。

### 目录结构类

- [x] 统一 `thirdparty` 与 `integrations` 概念。
- [x] 收束 `eventingress/chatpolicy/bridge/dispatch/outbound` 到事件管线领域。
- [x] 插件系统拆成 manifest/catalog/install/runtime/action/storage/webhook/managementview。
- [x] 渲染系统只暴露少量对外接口。
- [x] 合并低价值单文件 package。
- [x] 禁止新增无语义 `helpers.go`。
- [x] 限制新增泛化 `service.go` / `types.go`。
- [x] 重命名已有泛化文件为业务语义名。

### 配置类

- [x] `server run` 不再自动写回配置文件。
- [x] 增加 `config init` 命令。
- [x] 增加 `config normalize` 命令。
- [x] 增加 `config validate` 命令。
- [x] 确定默认值唯一来源。
- [x] 每个配置字段声明 apply policy。
- [x] 配置错误返回字段级 details。
- [x] 配置变更返回是否需要重启。

### API / 运维类

- [x] 统一 API 错误响应结构。
- [x] 所有 API 错误返回 request_id。
- [x] 所有日志包含 request_id。
- [x] 增加 HTTP request count 指标。
- [x] 增加 HTTP latency 指标。
- [x] 增加 HTTP status code 指标。
- [x] 增加 panic recover 指标。
- [x] 增加系统健康聚合 API。
- [x] 管理端展示数据库 schema version。
- [x] 管理端展示 OneBot 连接状态。
- [x] 管理端展示 plugin 运行/失败数量。
- [x] 管理端展示 render 健康状态。

### 测试类

- [x] 拆分超过 1000 行的测试文件。
- [x] 新增测试文件建议不超过 600 行。
- [x] route 测试从 OpenAPI 或注册表生成 expected routes。
- [x] 增加 OpenAPI 与实际响应结构校验。
- [x] 增加 schema drift 测试。
- [x] 增加 secret redaction 测试。
- [x] 增加配置 apply policy 完整性测试。
- [x] 增加非 main 包禁止 `log.Fatal/os.Exit` 测试。

---

## 20. 验收门禁建议

建议在 CI 中增加以下门禁：

```text
1. gofmt / go vet / staticcheck 基础检查；
2. 目标 Go 版本检查；
3. migration 从空库执行检查；
4. migration 从历史库升级检查；
5. sqlc generate 后无 diff；
6. OpenAPI 与 route 注册一致性检查；
7. OpenAPI 与 fixture 响应一致性检查；
8. 配置字段 apply policy 完整性检查；
9. secret 明文扫描；
10. 日志敏感字段输出扫描；
11. package 总数预算；
12. 单文件 package 数预算；
13. app/servicegraph/httpwire fan-out 预算；
14. 非 main 包禁止 os.Exit/log.Fatal；
15. 单文件行数预算；
16. 超大测试文件预算。
```

---

## 21. 最终建议

当前 server 不建议做整体重写，也不建议为了“看起来现代”而更换 Go、chi、SQLite、sqlc 等基础技术。真正的问题是工程结构、边界治理、配置安全、数据库演进、人机交互和可观测性没有跟上系统规模。

最合理的路线是：

```text
安全先行：secret 脱敏、日志脱敏、配置接口修复；
数据先行：migration 正规化，消除 schema 双真相源；
架构收束：app 变薄，模块自装配，领域边界清晰；
体验提升：配置错误、启动失败、健康状态可定位；
长期治理：package 数、目录数、fan-out、测试体量纳入 CI 预算。
```

只要按以上顺序推进，当前 server 的“文件数过多、文件杂糅、目录结构不清晰、文件夹过多、耦合高、维护困难、阅读困难”等问题会明显缓解，并且可以在不重写系统的情况下逐步恢复可维护性。

---

## 22. 参考资料

- Go Modules Reference：<https://go.dev/ref/mod>
- sqlc DDL / migration 说明：<https://docs.sqlc.dev/en/latest/howto/ddl.html>
- chi router / middleware：<https://github.com/go-chi/chi>
