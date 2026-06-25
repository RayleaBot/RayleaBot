# RayleaBot Server 架构整改清单（第二轮）

> 适用版本：`RayleaBot-main(10).zip`
> 范围：`server/`，重点关注文件数量、目录结构、包边界、组合根、插件系统、集成系统、配置与数据库演进、管理 API、安全与运维体验。
> 说明：本清单基于静态审计和结构统计生成。当前工作区已补充结构检查、contract 校验和 Go 测试复验记录；最终仍以项目声明的 Go 工具链和 CI 结果为准。

---

## 0. 当前基线与总体目标

### 0.1 当前结构基线

| 指标 | 当前值 | 风险判断 |
|---|---:|---|
| `server` 文件总数 | 1078 | 偏高，且较上一轮继续增加 |
| `server` 目录总数 | 173 | 偏高，目录层级继续膨胀 |
| Go 文件数 | 1060 | 偏高 |
| 生产 Go 文件数 | 833 | 偏高 |
| 测试 Go 文件数 | 227 | 测试治理有进步，但测试体量继续增长 |
| Go 总行数 | 约 132708 | 中大型服务规模 |
| 生产 Go 行数 | 约 76343 | 生产代码继续增长 |
| 测试 Go 行数 | 约 56365 | 测试较重 |
| 生产 package 数 | 153 | 明显偏多 |
| 单文件生产 package 数 | 48 | 明显偏多，是目录过碎的强信号 |
| 两文件生产 package 数 | 16 | 继续说明拆包颗粒度偏细 |
| `internal/app/**` 外部 internal import union | 约 77 | 组合根整体耦合仍高 |
| `internal/app/httpwire/routemodule` 内部 import | 33 | 新复杂度中心 |
| `internal/app/servicegraph/pluginmodule` 内部 import | 25 | 新复杂度中心 |
| `internal/app/servicegraph/integrationmodule` 内部 import | 17 | 仍偏重 |
| 生产文件超过 500 行 | 3 | 大文件不多，但集中在生成文件和 Douyin 集成 |

### 0.2 当前复杂度中心

| 目录 | 文件数 | 主要问题 |
|---|---:|---|
| `internal/plugins` | 240 | 插件系统仍是最大复杂度中心，模型、action、runtime、management view 边界需收束 |
| `internal/management` | 135 | API 包较多，错误模型、DTO、OpenAPI 同步仍需加强 |
| `internal/integrations` | 125 | 第三方集成成为新的大抽屉，provider 边界不清 |
| `internal/render` | 74 | 已是独立子系统，但对 app/plugin/management 暴露边界仍需收窄 |
| `internal/bot` | 68 | OneBot/适配层体量不小，需要保持边界稳定 |
| `internal/eventpipeline` | 60 | 归并方向正确，但仍需建立清晰事件流主轴 |
| `internal/app` | 26 | 文件数不算最多，但组合根和装配层认知负担最高 |

### 0.3 本轮整改总目标

- [x] 文件、目录、package 数进入递减轨道，而不是继续增长。
- [x] `internal/app/**` 不再通过新增子包转移复杂度，组合根真正变薄。
- [x] 单文件 package 明显减少，避免“一个文件一个目录”的伪模块化。
- [x] 数据库 migration 模型从“当前快照 + 兼容补丁”变为可解释、可验证的正式演进机制。
- [x] 配置 secret、第三方 cookie、错误响应等安全边界统一治理。
- [x] 插件、集成、渲染、事件管线形成稳定 bounded context。
- [x] 管理 API 的错误模型、DTO、OpenAPI 与前端交互更稳定。
- [x] 测试从“按行数拆 part2”升级为“按行为场景拆分”。
- [x] 运维诊断信息更聚合、更面向问题定位。

---

## 1. P0 整改项：立即阻止复杂度继续扩散

### P0-01：建立架构复杂度递减预算

**问题描述**

本轮整改后，`server` 文件数、目录数、生产 package 数、单文件 package 数均继续增加。当前已有架构测试，但预算基本等于当前现状，更多是在“冻结当前复杂度”，没有推动复杂度下降。

**影响**

- 新增功能容易继续通过新目录、新 package、新 `module.go` 扩展。
- 目录层级越来越深，代码阅读路径越来越长。
- 架构测试可以防止明显恶化，但不能阻止复杂度被转移。
- 维护者难以判断是否应该新建包、合并包或移动代码。

**实施路径**

1. 新增或增强架构统计脚本。
   - 建议路径：`server/tests/architecture/structure_metrics_test.go` 或 `server/scripts/architecture_metrics.go`。
   - 统计以下指标：
     - 生产 package 总数。
     - 单文件生产 package 数。
     - 两文件生产 package 数。
     - `internal/app/**` 子树 external internal import union。
     - 每个 package 的内部 import fan-out。
     - `module.go` 单文件 package 数。
     - 目录总数。
     - 泛化文件名重复次数。
2. 将当前值作为起始 baseline，但设置递减目标。
3. 新增“禁止恶化”门禁。
4. 每完成一轮目录收敛后，下调预算。
5. 在 PR 模板或 AGENTS 规则中要求：新增 package 必须说明边界理由。

**建议预算**

| 指标 | 当前值 | 第一阶段预算 | 第二阶段预算 | 长期目标 |
|---|---:|---:|---:|---:|
| 生产 package 数 | 153 | ≤145 | ≤135 | ≤115 |
| 单文件 package 数 | 48 | ≤40 | ≤32 | ≤20 |
| 两文件 package 数 | 16 | ≤14 | ≤10 | ≤8 |
| `internal/app/**` external import union | 约 77 | ≤65 | ≤55 | ≤45 |
| `routemodule` import fan-out | 33 | ≤25 | ≤18 | ≤12 |
| `pluginmodule` import fan-out | 25 | ≤18 | ≤14 | ≤10 |
| `module.go` 单文件 package | 当前值 | 不得增加 | 递减 | 仅保留必要模块 |

**验收标准**

- [ ] 有自动化脚本或架构测试输出上述指标。
- [ ] CI 中可以运行架构预算检查。
- [ ] 新增 package 时若导致预算增长，CI 会失败或要求白名单说明。
- [ ] 架构预算文件中记录当前 baseline 与目标值。
- [ ] 后续每轮整改至少降低一项预算，而不是只冻结当前值。

---

### P0-02：修复 `internal/app/**` 复杂度被转移的问题

**问题描述**

上一轮中 `internal/app/servicegraph` 和 `httpwire` 的直接 import 看似下降，但复杂度被拆到了：

```text
internal/app/httpwire/routemodule
internal/app/httpwire/configmodule
internal/app/servicegraph/pluginmodule
internal/app/servicegraph/integrationmodule
```

整个 `internal/app/**` 子树对外部 internal 包的依赖集合仍约为 77，说明组合根整体认知负担没有下降。

**影响**

- `app` 层仍然知道过多业务细节。
- 业务域无法自装配，新增能力仍需修改 app 子树。
- `routemodule`、`pluginmodule`、`integrationmodule` 可能成为新的小巨石。
- 架构测试如果只统计直接目录，会被新增子包绕过。

**实施路径**

1. 架构测试改为统计整个 `internal/app/**` 子树，而不是只统计 `internal/app/servicegraph`、`internal/app/httpwire` 的直接文件。
2. 给 `internal/app/**` 建立规则：
   - 只负责生命周期、模块装配和启动停止。
   - 不实现业务规则。
   - 不拼 HTTP response shape。
   - 不理解插件 action 细节。
   - 不理解 render engine 细节。
   - 不理解第三方平台内部登录/账号/session/source 细节。
3. 为业务域建立自装配接口。
   - 插件域暴露 `PluginModule`。
   - 渲染域暴露 `RenderModule`。
   - 集成域暴露 `IntegrationModule`。
   - 事件管线暴露 `EventPipelineModule`。
4. 将 `routemodule` 中按 API 域逐个构造 handler 的逻辑，逐步下沉到 `management` 或各 domain module。
5. 将 `pluginmodule` 中对插件 runtime、catalog、storage、action、render、webhook 的直接装配，改为插件模块内部装配。
6. 将 `integrationmodule` 中对 Bilibili、third-party、source、session 的直接装配，改为 integration provider 或 integration module 内部装配。

**可拆分任务**

- [ ] 新增 `internal/app/**` 子树 fan-out 检查。
- [ ] 梳理 `routemodule` 依赖清单，标记哪些应下沉到 `management`。
- [ ] 梳理 `pluginmodule` 依赖清单，标记哪些应下沉到 `plugins`。
- [ ] 梳理 `integrationmodule` 依赖清单，标记哪些应下沉到 `integrations`。
- [ ] 定义 `Module` 或 `Registrar` 接口，避免 app 直接引用具体业务实现。
- [ ] 每移除一组 app import，同步更新架构预算。

**验收标准**

- [ ] `internal/app/**` external internal import union 低于第一阶段预算。
- [ ] `routemodule` import fan-out 低于第一阶段预算。
- [ ] `pluginmodule` import fan-out 低于第一阶段预算。
- [ ] 新增业务 API 或插件 action 不再必须修改 `internal/app/httpwire/routemodule`。
- [ ] 新增集成 provider 不再必须修改 `internal/app/servicegraph/integrationmodule` 的大量装配代码。
- [ ] app 层代码能用少量模块接口解释，而不是需要理解每个业务域内部结构。

---

### P0-03：收敛单文件 package，停止“伪模块化”

**问题描述**

当前单文件生产 package 约 48 个，包含大量只有一个 `module.go`、`registry.go`、`config.go`、`view.go` 或工具文件的目录。例如：

```text
internal/app/httpwire/configmodule
internal/app/httpwire/routemodule
internal/app/servicegraph/pluginmodule
internal/app/servicegraph/integrationmodule
internal/integrations/bilibili/accountusage
internal/integrations/bilibili/credential
internal/integrations/bilibili/diagnostics
internal/integrations/bilibili/media
internal/integrations/bilibili/proxy
internal/integrations/bilibili/subscriptions
internal/plugins/actions/configaction
internal/plugins/actions/governanceaction
internal/plugins/actions/logaction
internal/plugins/actions/renderaction
internal/plugins/actions/scheduleraction
internal/plugins/actions/secretaction
internal/plugins/actions/webhookaction
internal/render/bootstrap
internal/render/engine
internal/render/plugintemplates
internal/protocolcap
internal/textsafe
```

**影响**

- 目录很多，但边界并不一定更清楚。
- import 路径变长，代码跳转成本提高。
- 新功能容易照搬“一个概念一个目录”的模式。
- 架构看起来模块化，实际只是增加包边界。
- 小包之间更容易产生循环依赖或反向依赖。

**实施路径**

1. 定义新建 package 门槛。

   独立 package 至少满足以下两项：
   - 有独立领域模型。
   - 有独立生命周期。
   - 有多个调用方。
   - 有独立测试价值。
   - 预计长期超过 4 个生产文件。
   - 未来可能替换实现。

2. 建立单文件 package 分类表。

   | 分类 | 处理方式 |
   |---|---|
   | 纯工具/常量/值对象 | 合并到上级 package 或 platform/common |
   | 单 action 包 | 合并到 `plugins/actions` 按文件组织 |
   | 单 module 包 | 合并回父 package，除非有清晰生命周期 |
   | 单 DTO/view 包 | 合并到调用方或管理 view 层 |
   | 单 engine/bootstrap 包 | 合并到 render module 或明确扩展成多文件子系统 |

3. 优先处理低风险单文件包。
4. 每合并一批，更新 import 路径并运行相关测试。
5. 在架构测试中禁止新增未白名单单文件 package。

**优先处理清单**

- [ ] 合并 `internal/plugins/actions/*action` 中简单 action 包。
- [ ] 评估并合并 `internal/integrations/bilibili/*` 中没有独立生命周期的单文件包。
- [ ] 评估 `internal/render/bootstrap`、`internal/render/engine`、`internal/render/plugintemplates` 是否需要独立 package。
- [ ] 评估 `internal/protocolcap` 是否可并入 protocol 或 capability 相关包。
- [ ] 评估 `internal/textsafe` 是否应归入 `redact`、`httpapi` 或 platform utility。
- [ ] 对保留的单文件 package 写明保留理由。

**验收标准**

- [ ] 单文件 package 数低于第一阶段预算。
- [ ] 新增单文件 package 会被架构测试拦截。
- [ ] 每个保留的单文件 package 都有注释或白名单说明。
- [ ] action 类目录数量减少，`plugins/actions` 的目录层级更浅。
- [ ] 代码搜索和跳转路径变短。

---

### P0-04：修正数据库 migration 模型

**问题描述**

当前已经引入：

```text
internal/storage/migrations/000001_base.sql
internal/storage/migrations/000002_add_third_party_account_columns.sql
internal/storage/migrations/000003_expand_third_party_account_platforms.sql
internal/storage/migrations/000004_add_bilibili_source_room_cover_url.sql
```

这是进步，但当前 `000001_base.sql` 已经接近“最终 schema 快照”，同时后续 migration 又重复添加其中已有字段，runner 通过 skip 或 duplicate column 忽略来兼容。这形成了新的双真相源。

**影响**

- 新库和老库演进逻辑难解释。
- migration 不是干净历史，难以审计。
- sqlc 看到的是被编辑过的 base，而不是真实演进结果。
- 自研 SQL split 和 duplicate-column 字符串判断比较脆弱。
- 后续 schema 修改容易继续堆兼容补丁。

**实施路径**

1. 明确选择 migration 策略。

   **方案 A：真实历史 migration**
   - `000001_base.sql` 保持初始 base schema。
   - 后续新增列、索引、约束全部通过新 migration 表达。
   - 新库从 000001 逐步执行到最新版本。

   **方案 B：当前 schema 快照 + 历史 migration 分离**
   - `schema.sql` 或 `000001_snapshot.sql` 表示当前完整 schema。
   - `migrations/` 表示从某个版本之后的演进历史。
   - 明确声明 sqlc 使用哪个 schema 来源。
   - 增加 drift test 保证 snapshot 与 migrations 最终结果一致。

   推荐优先选择方案 A。如果项目尚未正式发布数据库版本，可以趁早整理成真实历史。

2. 清理重复 migration。
   - 检查 `third_party_accounts` 字段是否重复出现在 base 和 add-columns migration。
   - 检查 `bilibili_source_rooms.cover_url` 是否重复出现在 base 和 add-column migration。
   - 移除依赖 duplicate-column error string 的常规路径。

3. 引入或规范 migration runner。
   - 可保留轻量自研 runner，但必须支持事务、注释、SQL 分号边界、版本表和错误定位。
   - 更建议引入成熟 migration 工具，例如 goose、golang-migrate、Atlas 等。

4. 建立 `schema_migrations`。

   ```sql
   CREATE TABLE IF NOT EXISTS schema_migrations (
     version INTEGER PRIMARY KEY,
     name TEXT NOT NULL,
     applied_at TEXT NOT NULL
   );
   ```

5. 让 sqlc 读取正式 schema 来源。
6. 新增 migration drift 测试。

**可核对任务**

- [ ] 写一份 `docs/architecture/storage-migrations.md`，说明选择方案 A 或 B。
- [ ] 清理 `000001_base.sql` 与后续 migration 的重复列。
- [ ] 建立 `schema_migrations` 表。
- [ ] 新库初始化只通过正式 migration 流程完成。
- [ ] 老库升级测试覆盖 000001 到最新版本。
- [ ] sqlc 输入与正式 schema 来源一致。
- [ ] 删除或限制 `ignoreDuplicateColumn` 作为常规演进方式。
- [ ] 禁止新增 `ensure*Columns` 作为数据库演进方式。

**验收标准**

- [ ] 空数据库执行 migration 后能得到最新 schema。
- [ ] 从旧版本测试数据库执行 migration 后能得到最新 schema。
- [ ] 重复执行 migration 不会重复修改 schema，也不会依赖脆弱字符串判断。
- [ ] sqlc 查询与 migration 最终 schema 一致。
- [ ] 架构测试能检查 schema drift。

---

### P0-05：第三方登录与 cookie 响应安全整改

**问题描述**

二维码登录或第三方账号登录流程中，存在将 cookie 或类似 `SESSDATA` 的 credential 作为响应字段返回给管理端浏览器的设计风险。即使测试中使用 fixture 值，这种 API shape 也容易在真实环境中导致敏感凭据进入浏览器、日志、截图或前端错误上报。

**影响**

- 浏览器 DevTools 可直接看到平台 cookie。
- 前端错误上报或代理日志可能带出 credential。
- 管理 API 查看权限可能变成 credential 读取权限。
- 测试快照和 fixture 容易固化不安全设计。
- 第三方账号接入越多，泄露面越大。

**实施路径**

1. 梳理所有第三方登录成功响应。
   - Bilibili。
   - Douyin。
   - Weibo。
   - Netease Music。
   - 通用 third-party login。
2. 移除对浏览器返回 raw cookie、CK、access token、refresh token 的字段。
3. 改为以下两种安全模式之一：

   **模式 A：服务端直接持久化 credential**
   ```json
   {
     "status": "confirmed",
     "account_id": "acc_xxx",
     "display_name": "xxx",
     "expires_at": "2026-12-31T00:00:00Z"
   }
   ```

   **模式 B：一次性 credential handle**
   ```json
   {
     "status": "confirmed",
     "credential_handle": "one_time_xxx"
   }
   ```

   handle 必须短期有效、一次性使用、不可反查 raw credential。

4. credential 写入 `secret_store` 或第三方账号 secret 存储。
5. 更新 OpenAPI、前端类型、fixtures 和测试。
6. 增加响应体扫描测试，禁止返回 cookie-like 字段。

**可核对任务**

- [ ] 搜索所有 API response DTO 中的 `Cookie`、`cookie`、`CK`、`SESSDATA`、`access_token`、`refresh_token` 字段。
- [ ] 删除或替换管理 API 中返回 raw credential 的字段。
- [ ] OpenAPI 中不再暴露 raw credential 字段。
- [ ] fixtures 中不再把 cookie 作为成功响应字段。
- [ ] 前端不再依赖 raw credential 完成账号确认。
- [ ] 增加测试断言登录成功响应不包含 raw credential。

**验收标准**

- [ ] 管理端成功登录第三方账号后，只能看到账号状态和展示信息，不能看到 cookie。
- [ ] API 响应体扫描测试通过。
- [ ] 日志和错误响应不会包含 credential。
- [ ] 第三方账号仍能正常持久化和使用。

---

### P0-06：统一第三方登录和管理 API 的 typed error

**问题描述**

部分业务 API 仍存在直接返回 `err.Error()` 或通过字符串匹配判断错误类型的情况，例如超时、二维码过期、限流、代理失败、登录态失效等。

**影响**

- 错误分类脆弱，上游错误文案变化会破坏逻辑。
- 用户看到的错误信息不稳定、不友好。
- 错误 message 可能包含 URL、cookie、token、代理地址、平台返回体片段。
- 前端只能靠字符串判断，不利于稳定交互。
- 运维排障缺少稳定 error code。

**实施路径**

1. 定义统一业务错误类型。

   ```go
   type DomainError struct {
       Code        string
       HTTPStatus  int
       SafeMessage string
       Details     map[string]any
       Cause       error
   }
   ```

2. 建立错误码命名规则。

   ```text
   third_party_qrcode_expired
   third_party_qrcode_timeout
   third_party_rate_limited
   third_party_proxy_failed
   third_party_cookie_invalid
   config_invalid_field
   plugin_action_denied
   render_browser_unavailable
   ```

3. handler 层统一把 `DomainError` 映射为 HTTP error envelope。
4. 日志记录 `Cause`，API 只返回 `SafeMessage`。
5. OpenAPI 中登记稳定错误码和 details shape。
6. 前端改为根据 `code` 和 `details` 分支，而不是 message 字符串。

**可核对任务**

- [ ] 新增或完善 `internal/httpapi` 错误转换函数。
- [ ] 第三方登录流程不再直接返回 `err.Error()`。
- [ ] 字符串 contains 判断错误类型的逻辑被 typed error 替代。
- [ ] OpenAPI error codes 同步。
- [ ] 前端调用处不再依赖错误 message 分支。
- [ ] 日志中保留底层 cause，但响应中不泄露敏感内容。

**验收标准**

- [ ] 所有 management API 错误响应都有稳定 `code`。
- [ ] 错误响应包含 `request_id`。
- [ ] 用户可见 `message` 可读且不含敏感信息。
- [ ] 关键错误都有测试覆盖。

---

### P0-07：补齐 Go 工具链与开发环境自检

**问题描述**

项目 `server/go.mod` 声明 `go 1.25.8`，并使用较新的模块指令。低版本 Go 或离线环境无法直接测试。当前如果工具链不匹配，失败信息可能先暴露在 `go test` 或 `go list` 阶段，不够友好。

**影响**

- 新人 clone 后难以快速定位失败原因。
- 运维排障时先卡在工具链，而不是业务问题。
- 离线 CI 或受限环境无法自动下载 toolchain。
- IDE、本地、CI、容器环境容易不一致。

**实施路径**

1. 新增工具链检查脚本。

   建议路径：
   ```text
   server/scripts/check-toolchain.sh
   scripts/check-toolchain.sh
   ```

2. 检查内容：
   - 当前 Go 版本。
   - `go.mod` 声明版本。
   - 是否支持 `tool` 指令。
   - 是否能执行 `go env`、`go list`。
   - 是否处于离线模式。
3. 增加 `make doctor` 或 `make check`。
4. 提供 dev container、mise/asdf 配置或 Docker 测试入口。
5. 如果必须锁定 Go patch 版本，在错误提示中明确安装方式。

**可核对任务**

- [ ] `make doctor` 能给出清晰工具链诊断。
- [ ] 本地 Go 版本不足时，报错包含 required/current 版本。
- [ ] 文档中提供推荐安装方式。
- [ ] CI 使用与 `go.mod` 一致的 Go 版本。
- [ ] 离线环境失败时能明确提示原因。

**验收标准**

- [ ] 工具链不匹配时不再出现难懂的 Go parser 错误。
- [ ] 开发者能在 1 条命令内确认环境是否满足要求。
- [ ] CI、本地、容器使用同一版本线。

---

## 2. P1 整改项：收束核心业务域边界

### P1-01：插件系统模型拆分与边界收束

**问题描述**

`internal/plugins` 当前约 240 个文件，是最大复杂度中心。插件 manifest、安装状态、运行状态、capability view、runtime manager、management UI、actions、render binding、storage 等概念互相靠近，容易形成大模型和中心化 service。

**影响**

- 新增插件能力时容易修改多个层级。
- runtime model 与 management view 可能互相污染。
- 插件状态含义可能在不同 API、日志、UI 中不一致。
- actions dispatcher 可能成为新的中心巨石。

**实施路径**

1. 明确插件领域模型分层。

   | 模型 | 职责 |
   |---|---|
   | `Manifest` | 插件静态声明、版本、入口、capabilities |
   | `Installation` | 安装、升级、卸载、本地文件状态 |
   | `RuntimeState` | 进程运行状态、启动失败、健康状态 |
   | `CapabilitySet` | 插件声明可用能力和权限 |
   | `ActionSurface` | 插件可调用平台动作 |
   | `ManagementView` | 管理端展示 DTO |
   | `StorageBinding` | KV/File/Config storage 绑定 |
   | `RenderBinding` | 渲染模板和 artifact 绑定 |

2. 禁止 runtime 直接依赖 management DTO。
3. 禁止 management view 成为内部状态源。
4. 将 plugin snapshot 拆为内部 state 与 API view 两层。
5. 建立状态映射测试。

**可核对任务**

- [x] 梳理当前 plugin snapshot 字段，标记其所属模型。
- [x] 拆分 runtime state 与 management view。
- [x] 插件 API 返回对象只由 view mapper 生成。
- [x] 插件 runtime 不 import management 包。
- [x] 插件 management handler 不直接修改 runtime 内部状态。
- [x] 状态映射有单元测试。

**验收标准**

- [x] 插件内部状态、API 展示状态、UI 状态命名一致但模型分离。
- [x] 新增管理端展示字段不需要修改 runtime state。
- [x] 新增 runtime 状态不自动泄露到 API。

---

### P1-02：插件 actions 从中心分发表改为模块注册

**问题描述**

`internal/plugins/actions` 当前仍是高 fan-out 包。多个简单 action 被拆成单文件 package，中心 dispatcher 继续承担多平台能力注册。

**影响**

- 新增平台能力时容易修改中心 dispatcher。
- actions 目录继续横向扩张。
- action 权限、参数 schema、错误码容易分散。
- 插件可调用面难以审计。

**实施路径**

1. 定义 action 注册接口。

   ```go
   type ActionModule interface {
       RegisterActions(registry *actions.Registry)
   }
   ```

2. 各领域模块自行注册 action。
   - render module 注册 render action。
   - scheduler module 注册 scheduler action。
   - secret module 注册 secret action。
   - governance module 注册 governance action。
   - webhook module 注册 webhook action。
3. 合并简单 action 子包为 `plugins/actions/*.go` 文件。
4. 每个 action 必须声明：
   - action kind。
   - capability。
   - 参数 schema。
   - 返回 schema。
   - 权限边界。
   - 是否访问 secret。
   - 是否写文件。
   - 是否访问网络。
   - 错误码。
   - 审计日志字段。
5. 增加 action registry 快照测试。

**可核对任务**

- [ ] 新增 `ActionModule` 注册接口。
- [ ] render/scheduler/secret/governance/webhook action 改为模块注册。
- [ ] 合并简单 `*action` 单文件子包。
- [ ] 每个 action 有 capability 和权限声明。
- [ ] 每个 action 有参数校验测试。
- [ ] action registry 快照与 contract 同步。

**验收标准**

- [ ] 新增 render action 不需要修改中心 dispatcher 大表。
- [ ] action 权限边界可被自动列出。
- [ ] 简单 action package 数明显减少。
- [ ] 插件 action 错误码稳定并进入 OpenAPI/插件 contract。

---

### P1-03：第三方集成统一 provider 边界

**问题描述**

`internal/integrations` 当前约 125 个文件，包含 Bilibili、Douyin、Weibo、Netease、thirdparty、thirdpartylogin 等多个概念。集成目录逐渐成为新的大抽屉。

**影响**

- 每个平台可能各自实现登录、账号、代理、刷新、诊断、错误处理。
- 管理 API 需要理解平台细节。
- 新增平台时容易复制旧平台结构。
- Bilibili source、账号、session、diagnostics 等概念混在 integration 内部，边界不稳定。

**实施路径**

1. 定义统一 provider interface。

   ```go
   type Provider interface {
       Platform() Platform
       StartLogin(ctx context.Context, req LoginRequest) (LoginSession, error)
       PollLogin(ctx context.Context, session LoginSession) (LoginStatus, error)
       ValidateAccount(ctx context.Context, account AccountRef) (AccountStatus, error)
       RefreshAccount(ctx context.Context, account AccountRef) error
       Diagnostics(ctx context.Context, account AccountRef) (Diagnostics, error)
   }
   ```

2. 抽出公共账号模型。
   - account id。
   - platform。
   - display name。
   - status。
   - expires at。
   - proxy settings。
   - credential ref。
3. Bilibili、Douyin、Weibo、Netease 分别实现 provider。
4. 管理 API 只依赖 provider registry，不直接依赖平台细节。
5. 平台特有字段通过 typed details 扩展。
6. 统一 typed error。

**可核对任务**

- [x] 建立 `Platform`、`AccountRef`、`LoginSession`、`LoginStatus` 等公共模型。
- [x] Bilibili 登录接入 provider interface。
- [x] Douyin 登录/解析接入 provider interface。
- [x] Weibo/Netease 接入 provider interface 或明确暂不支持的 stub。
- [x] `management/thirdpartyapi` 不再直接理解每个平台的内部 DTO。
- [x] 平台错误统一映射为 domain error。

**验收标准**

- [x] 新增平台只需实现 provider 并注册，不需要复制管理 API 大量逻辑。
- [x] 第三方账号列表 API 可统一展示所有平台账号。
- [x] 各平台登录失败返回统一错误结构。
- [x] credential 不返回浏览器。

---

### P1-04：Bilibili 集成显式模块化

**问题描述**

Bilibili 相关功能已经包含账号、session、source、订阅、diagnostics、proxy、credential、media 等多个子概念，体量已经超过普通 integration。

**影响**

- Bilibili 内部服务装配容易泄露到 app/servicegraph。
- 账号和 source 生命周期混杂。
- source 状态、直播间状态、账号健康状态容易不一致。
- 管理 API 可能直接理解 Bilibili 内部结构。

**实施路径**

1. 将 Bilibili 视为独立 integration module。
2. 模块内部明确：
   - account service。
   - session service。
   - source service。
   - live/dynamic client。
   - diagnostics service。
   - management view mapper。
3. 对外只暴露窄接口。

   ```go
   type BilibiliModule interface {
       Start(ctx context.Context) error
       Stop(ctx context.Context) error
       Health(ctx context.Context) HealthSnapshot
       Accounts() AccountService
       Sources() SourceService
   }
   ```

4. app 层不直接装配 Bilibili 内部服务。
5. management 层通过 module service 获取 view。

**可核对任务**

- [x] 明确 Bilibili module 对外接口。
- [x] app/servicegraph 不再直接 import Bilibili 多个内部子包。
- [x] Bilibili source 状态定义有唯一来源。
- [x] Bilibili account/session/source 状态映射有测试。
- [x] Bilibili diagnostics 不返回 credential。

**验收标准**

- [x] app 层只依赖 Bilibili module 接口或 integration provider registry。
- [x] Bilibili source、account、session 的职责边界清晰。
- [x] 新增 Bilibili 内部能力不需要修改 app 装配细节。

---

### P1-05：拆分 Douyin 大文件

**问题描述**

当前存在较大的 Douyin 集成文件：

```text
internal/integrations/douyin/browser.go   约 690 行
internal/integrations/douyin/resolve.go   约 671 行
```

浏览器自动化和页面解析逻辑混在大文件中，容易继续膨胀。

**影响**

- 解析逻辑、浏览器控制、异常处理、风控处理难以分开测试。
- 修改一个小行为需要阅读大量代码。
- 错误处理容易散落在流程中。
- 后续接入 provider interface 时迁移成本更高。

**实施路径**

1. 按职责拆分文件。

   ```text
   douyin/browser_session.go      # 浏览器生命周期与上下文
   douyin/page_resolver.go        # 主解析流程
   douyin/url_normalizer.go       # URL 规范化
   douyin/extraction.go           # 页面数据抽取
   douyin/challenge.go            # 风控、验证码、登录态异常
   douyin/errors.go               # typed errors
   douyin/diagnostics.go          # 诊断信息
   ```

2. 先不改变行为，只移动代码。
3. 为 URL normalize、页面抽取、错误分类添加独立测试。
4. 接入 integration provider error 模型。

**可核对任务**

- [ ] `browser.go` 降到 400 行以内或明确保留理由。
- [ ] `resolve.go` 降到 400 行以内或明确保留理由。
- [ ] URL normalize 有表驱动测试。
- [ ] 页面抽取逻辑可独立测试。
- [ ] 错误分类不再依赖散落的字符串判断。

**验收标准**

- [ ] Douyin 浏览器控制和业务解析职责分离。
- [ ] 大文件数量下降。
- [ ] provider 接入更容易。

---

### P1-06：渲染子系统对外接口收窄

**问题描述**

`internal/render` 已经是完整子系统，但与 plugin、management、app、actions 的装配关系仍偏强。插件 action 或管理 API 可能理解 render 内部 repository、engine、browser 细节。

**影响**

- render 内部重构会影响多个外部包。
- 浏览器依赖失败时，错误传播路径不清晰。
- render template、artifact、plugin sync、browser health 等概念边界不够稳定。

**实施路径**

1. render 只向外暴露以下接口：
   - `RenderService`。
   - `TemplateRegistry`。
   - `ArtifactStore`。
   - `HealthSnapshot`。
   - `PluginTemplateSync`。
2. plugin action 调用 render service，不直接访问 render repository。
3. management API 调用 render view service，不理解 browser engine 细节。
4. app 只启动 render module。
5. render health 统一输出：
   - browser path。
   - browser start status。
   - recent error。
   - queue length。
   - render latency。
   - template sync status。

**可核对任务**

- [x] 梳理 render 被外部包直接 import 的清单。
- [x] 收敛 render 对外接口。
- [x] plugin action 不直接 import render 内部 repository。
- [x] management/renderapi 不直接理解 browser engine。
- [x] render health snapshot 结构稳定并进入 API contract。

**验收标准**

- [x] render 内部目录调整不影响 app 和 plugin action 的大部分代码。
- [x] 浏览器不可用时，管理端可看到明确诊断。
- [x] render 相关错误使用统一 error envelope。

---

### P1-07：事件管线从目录归并升级为流程主轴

**问题描述**

事件相关目录已经归并到：

```text
internal/eventpipeline/
  bridge/
  chatpolicy/
  dispatch/
  eventingress/
  outbound/
```

这是正确方向，但当前更多是目录层面的归并，还需要形成清晰的事件流主轴。

**影响**

- 新维护者仍需要跨多个包理解消息流。
- 事件进入、策略判断、插件调用、出站回复的状态和错误边界不清。
- 指标和日志可能分散在不同阶段。

**实施路径**

1. 定义事件管线主流程文档。

   ```text
   adapter -> ingress -> policy -> bridge -> dispatch -> plugin/actions -> outbound
   ```

2. 为每个阶段定义输入输出模型。
3. 为每个阶段定义错误语义和 metrics。
4. 在代码中建立 `Pipeline` 或 `Processor` 抽象，减少散落装配。
5. 事件流 tracing/logging 加 request/event id。

**可核对任务**

- [ ] 新增 `docs/architecture/event-pipeline.md`。
- [ ] 定义每个阶段的输入输出结构。
- [ ] dispatch、policy、outbound 错误有稳定 code。
- [ ] 事件流日志包含 event id。
- [ ] 事件管线 metrics 覆盖 ingress、dispatch、outbound。

**验收标准**

- [ ] 新人可以通过一张图理解消息从 OneBot 到回复的路径。
- [ ] 事件失败能定位到具体阶段。
- [ ] 事件管线新增阶段不需要修改 app 组合根大量代码。

---

## 3. P1 整改项：配置、安全与管理 API

### P1-08：配置 secret 与 apply policy 改为 schema/tag 驱动

**问题描述**

当前配置 secret 脱敏和 apply policy 已有明显改善，但 secret path 仍主要由手工路径表维护。未来新增 secret 字段时，若忘记加入 redaction path，仍可能泄露。

**影响**

- secret 治理依赖人工记忆。
- schema、后端、前端、文档对 secret/重启/热更新的理解可能不一致。
- 管理 UI 难以自动显示“该字段需要重启”或“该字段已脱敏”。

**实施路径**

1. 在配置 schema 或 Go typed config 中声明字段元数据。

   schema 示例：
   ```yaml
   x-secret: true
   x-redaction: full
   x-apply-policy: restart_required
   ```

   Go tag 示例：
   ```go
   AccessToken string `secret:"true" redact:"full" apply:"secret_only"`
   ```

2. 生成或加载统一 field metadata。
3. redaction、secret store、apply policy、UI 提示都从 metadata 读取。
4. 测试扫描所有配置 leaf path，确保每个字段都有 apply policy，secret 字段有 redaction policy。

**可核对任务**

- [ ] 确定 schema 驱动还是 Go tag 驱动。
- [ ] 所有配置字段有 apply policy。
- [ ] 所有 secret 字段有 redaction policy。
- [ ] 手工 secret path 白名单减少或只作为兼容层。
- [ ] 前端可读取字段是否 secret、是否需要重启。
- [ ] 测试覆盖新增字段必须声明 metadata。

**验收标准**

- [ ] 新增 secret 字段如果没有 redaction policy，测试会失败。
- [ ] 管理 API 读取配置不返回 secret 明文。
- [ ] 管理 UI 能正确展示脱敏字段和重启提示。

---

### P1-09：配置更新链路传递 request context

**问题描述**

配置更新过程中，secret store 操作可能使用 `context.Background()`。这会导致 HTTP 请求取消、超时或客户端断开后，后端仍可能继续执行 secret 写入或配置更新。

**影响**

- 请求取消不生效。
- 配置更新失败或部分成功时难以追踪。
- 日志与 request id 关联困难。
- 长时间 secret store 操作无法被上游取消。

**实施路径**

1. 修改配置更新入口：

   ```go
   UpdateConfigDocument(ctx context.Context, request map[string]any)
   ```

2. 将 ctx 传递到：
   - StoreConfigSecrets。
   - ResolveConfigSecretRefs。
   - PersistConfig。
   - ValidateConfig。
3. handler 传入 request context。
4. 测试请求取消时不继续写入。

**可核对任务**

- [ ] `UpdateConfigDocument` 接收 context。
- [ ] 配置 secret store 读写使用 request context。
- [ ] 没有新增 `context.Background()` 用于请求路径。
- [ ] 配置更新日志包含 request id。
- [ ] 请求取消测试通过。

**验收标准**

- [ ] 客户端取消请求后，配置更新不会无界继续执行。
- [ ] 配置更新失败能通过 request id 排查。

---

### P1-10：管理 API 与 OpenAPI 强同步

**问题描述**

管理 API handler、DTO、OpenAPI 和前端类型仍主要靠人工同步。当前已有 contracts，但 server handler 未形成强类型绑定或自动漂移检查。

**影响**

- handler 可能返回 contract 外字段。
- 前端可能依赖未记录字段。
- 错误码、状态名、分页结构容易漂移。
- fixtures 可能固化旧行为。

**实施路径**

1. 统一管理 API DTO 层。
2. 建立 OpenAPI drift 测试。
3. 至少检查：
   - route 是否在 OpenAPI 中存在。
   - HTTP method 是否一致。
   - error code 是否登记。
   - response fixture 是否符合 schema。
4. 可评估引入 OpenAPI codegen，但不必一次性全量替换。
5. handler 不直接返回 domain model，必须通过 DTO mapper。

**可核对任务**

- [ ] 管理 API route 与 OpenAPI 有自动一致性检查。
- [ ] 错误码进入统一 contract。
- [ ] response fixture 可按 OpenAPI schema 校验。
- [ ] handler 不直接返回 runtime/domain 内部模型。
- [ ] 前端 generated types 由 OpenAPI 生成并在 CI 检查 drift。

**验收标准**

- [ ] 新增 API 如果未更新 OpenAPI，CI 会失败。
- [ ] OpenAPI 删除字段时，前端类型和 fixtures 会同步暴露问题。
- [ ] API 错误结构稳定。

---

### P1-11：统一管理端健康诊断聚合

**问题描述**

当前各子系统已有状态和 metrics，但运维视角需要一个聚合健康快照，直接回答“哪里坏了、是否需要重启、如何修复”。

**影响**

- 用户需要在多个页面和日志中拼接状态。
- 远程排障依赖截图和口头描述。
- 系统有局部 degraded 状态时，缺少统一呈现。

**实施路径**

1. 定义系统健康聚合 API。

   ```json
   {
     "server": "ok",
     "database": {
       "status": "ok",
       "schema_version": "000004"
     },
     "config": {
       "status": "ok",
       "pending_restart": false
     },
     "onebot": {
       "status": "connected"
     },
     "plugins": {
       "running": 12,
       "failed": 1
     },
     "render": {
       "status": "degraded",
       "last_error_code": "render_browser_unavailable"
     },
     "integrations": {
       "bilibili": "warning"
     },
     "scheduler": "ok"
   }
   ```

2. 每个子系统提供 `HealthSnapshot`。
3. 系统层聚合并映射为管理 API。
4. 管理 UI 提供“诊断总览”。
5. 错误项给出修复建议。

**可核对任务**

- [ ] database health 包含 migration/schema version。
- [ ] config health 包含 pending restart 与最近 apply error。
- [ ] plugin health 包含 running/failed/degraded 数量。
- [ ] render health 包含 browser 状态和最近错误。
- [ ] integration health 包含账号过期、代理失败、平台限流等摘要。
- [ ] scheduler health 包含队列/任务失败摘要。

**验收标准**

- [ ] 管理端一个页面能看到系统总体状态。
- [ ] 每个 degraded/error 状态都有机器可读 code 和用户可读建议。
- [ ] 健康 API 不返回 secret。

---

## 4. P1 整改项：测试治理

### P1-12：测试文件从 `part2` 拆分升级为场景拆分

**问题描述**

本轮已经减少超过 1000 行的测试文件，但出现多个 `*_part2_test.go`。这能降低单文件行数，却不能改善认知结构。

**影响**

- `part2` 文件名不表达业务场景。
- reviewer 难以判断新增测试属于哪个行为。
- fixture 初始化可能继续重复。
- 测试失败定位不够直接。

**实施路径**

1. 对超过 600 行的测试文件建立拆分计划。
2. 按行为场景命名，而不是按 part 编号命名。

   示例：
   ```text
   manager_start_stop_test.go
   manager_restart_test.go
   manager_failure_test.go
   source_room_lifecycle_test.go
   source_event_dispatch_test.go
   shell_auth_test.go
   shell_api_lifecycle_test.go
   config_redaction_test.go
   migration_upgrade_test.go
   ```

3. 提取通用 fixture builder。
4. 复杂集成测试按 API 域拆分。
5. 架构测试限制 `part2` 命名继续增加。

**可核对任务**

- [ ] 列出所有超过 600 行测试文件。
- [ ] 每个大测试文件对应一个拆分方案。
- [ ] `*_part2_test.go` 改为场景命名。
- [ ] 公共 fixture/helper 只保留必要抽象，不制造 test util 巨石。
- [ ] 新增测试文件名称表达行为。

**验收标准**

- [ ] 超过 600 行测试文件数量低于第一阶段预算。
- [ ] 不再新增 `part2`、`part3` 这类测试文件名。
- [ ] 测试失败从文件名即可看出所属场景。

---

### P1-13：新增安全回归测试

**问题描述**

本轮已修复配置 secret 明文返回，但第三方 credential、日志、错误响应、fixtures 仍需要系统性防泄漏测试。

**影响**

- 后续新增 secret 字段可能重新泄露。
- 错误 message 或日志可能含敏感内容。
- fixtures 可能固化危险 shape。

**实施路径**

1. 建立敏感字段扫描测试。
2. 扫描对象：
   - API response fixtures。
   - config GET 响应。
   - third-party login 响应。
   - logs API 响应。
   - WebSocket events。
   - docs/examples/fixtures 中疑似真实 token。
3. 规则包含：
   - `SESSDATA`。
   - `bili_jct`。
   - `access_token` 明文字段。
   - `refresh_token` 明文字段。
   - `Cookie` header 内容。
   - 长随机 secret 模式。
4. 对允许的 fixture-only 假值建立白名单。

**可核对任务**

- [x] 新增 secret leak scanner 测试。
- [x] 配置 API fixture 不含明文 secret。
- [x] 第三方登录 fixture 不含 raw cookie。
- [x] 日志 API 测试验证 query/header redaction。
- [x] WebSocket event 不含 secret。
- [x] docs/examples 中只允许显式假值。

**验收标准**

- [x] 添加一个新的 secret 字段但未声明 redaction 时，测试失败。
- [x] 返回 raw cookie 的 API fixture 会被测试拦截。
- [x] 日志中 token/query 被脱敏。

---

## 5. P2 整改项：目录、命名与阅读体验

### P2-01：减少泛化文件名

**问题描述**

上一轮减少了 `types.go`、`service.go`、`helpers.go` 等泛化命名，但当前又出现新的重复文件名：

| 文件名 | 出现次数 | 问题 |
|---|---:|---|
| `routes.go` | 11 | 不表达具体 API 域 |
| `repository.go` | 10 | 数据层模板化 |
| `http.go` | 8 | HTTP 职责泛化 |
| `registry.go` | 7 | registry 概念冲突 |
| `module.go` | 6 | 新一轮模块包装泛化 |
| `config.go` | 5 | 配置语义不够具体 |
| `errors.go` | 5 | 错误模型可能重复 |

**实施路径**

1. 不再把 `routes.go`、`repository.go`、`http.go`、`module.go` 作为默认命名模板。
2. 文件名表达业务职责。
3. 架构测试统计泛化文件名数量。
4. 对保留的泛化文件名要求 package 名足够明确。

**命名建议**

| 当前倾向 | 建议 |
|---|---|
| `routes.go` | `plugin_routes.go`、`config_routes.go`、`system_routes.go` |
| `repository.go` | `account_repository.go`、`source_state_repository.go` |
| `http.go` | `qrcode_login_http.go`、`webhook_http.go` |
| `registry.go` | `action_registry.go`、`provider_registry.go` |
| `module.go` | `plugin_module.go`、`render_module.go` |
| `errors.go` | `domain_errors.go`、`login_errors.go` |

**验收标准**

- [ ] 新增文件不默认使用泛化命名。
- [ ] 泛化文件名数量进入递减预算。
- [ ] 搜索文件名时能更快定位业务职责。

---

### P2-02：补齐 server 架构图和阅读入口

**问题描述**

当前代码体量较大，目录较多，但缺少低成本阅读入口。新维护者难以快速理解启动流程、消息事件流和管理 API 流程。

**实施路径**

1. 新增或更新架构文档：

   ```text
   docs/architecture/server-overview.md
   docs/architecture/server-lifecycle.md
   docs/architecture/event-pipeline.md
   docs/architecture/plugin-runtime.md
   docs/architecture/storage-migrations.md
   docs/architecture/management-api.md
   ```

2. 至少绘制三张图：
   - 启动生命周期图。
   - 消息事件流图。
   - 管理 API 调用链图。
3. 每张图必须映射到真实代码路径。
4. 文档中标注哪些包是 domain，哪些包是 platform，哪些包是 composition root。

**可核对任务**

- [ ] 有启动生命周期图。
- [ ] 有事件流图。
- [ ] 有管理 API 调用链图。
- [ ] 有插件 runtime 状态图。
- [ ] 有 migration 演进说明。
- [ ] 文档中的路径经过脚本检查存在。

**验收标准**

- [ ] 新维护者能通过文档定位常见修改入口。
- [ ] 文档路径与实际目录一致。
- [ ] 架构决策不只存在于代码命名中。

---

### P2-03：建立技术栈替换/引入准则

**问题描述**

当前主要问题不是 Go、chi、SQLite、sqlc 等技术本身，而是架构边界和演进机制。盲目换栈会放大风险。

**建议保留**

- [ ] Go：继续作为 server 主语言。
- [ ] chi：继续作为 HTTP router。
- [ ] SQLite：在轻部署、自托管定位下继续保留。
- [ ] sqlc：继续作为显式 SQL 类型生成方案。
- [ ] slog：继续作为标准日志基础。
- [ ] Prometheus：继续作为指标体系。

**建议评估引入或替换**

| 领域 | 当前问题 | 建议 |
|---|---|---|
| DB migration | 自研 runner 与重复 migration 脆弱 | 评估 goose、golang-migrate、Atlas |
| OpenAPI 落地 | handler 与 contract 手工同步 | 评估 oapi-codegen 或 contract test 强化 |
| secret master key | sealed secret 与 key 同域风险 | 支持 env key、OS keychain、外部 KMS |
| 架构治理 | fan-out 可被子包绕开 | 自研结构测试继续加强 |
| 第三方错误 | err string 分类 | typed domain error |

**实施路径**

1. 新增 `docs/architecture/technology-decisions.md`。
2. 每次新增依赖必须写明：
   - 替代什么问题。
   - 为什么现有方案不够。
   - 是否引入并行栈。
   - 如何回滚。
   - 对 CI、构建、发布的影响。
3. 对 migration 工具和 OpenAPI 工具做小范围 spike，不直接全量替换。

**验收标准**

- [ ] 没有为解决架构混乱而盲目更换主技术栈。
- [ ] 新增依赖都有技术决策记录。
- [ ] migration/OpenAPI 工具引入前有小范围验证。

---

## 6. 分阶段实施计划

### 阶段 A：止血与防扩散

目标：先防止复杂度继续增加。

- [x] 建立架构复杂度指标脚本。
- [x] CI 加入 package 数、单文件 package、app fan-out 检查。
- [x] 禁止新增未说明理由的单文件 package。
- [x] 禁止通过 app 子包绕过 servicegraph/httpwire fan-out。
- [x] 第三方登录响应禁止返回 raw cookie。
- [x] 管理 API 错误禁止直接返回敏感 `err.Error()`。
- [x] 工具链自检脚本落地。

**阶段 A 验收**

- [x] 当前所有结构指标有自动化输出。
- [x] 新增复杂度会被 CI 或架构测试发现。
- [x] 配置和第三方登录不返回明文 credential。
- [x] 本地环境不匹配时能清楚报错。

**阶段 A 完成记录（2026-06-25）**

- 根因处理：新增 `docs/engineering/server-architecture-budget.json`，让结构预算由脚本和架构测试共同读取；CI 直接运行工具链检查和结构检查，避免只靠人工 review 发现 package 数、单文件 package、`internal/app/**` fan-out 增长。
- 根因处理：扫码登录成功后由服务端保存凭据，响应只返回账号摘要；OpenAPI、fixtures、服务端 handler、集成测试、Web/Launcher 生成类型和 Web 状态同步已同步更新。
- 根因处理：第三方登录和账号保存失败走稳定错误码与安全提示，原始上游错误只作为内部 cause 保留；管理 API 扫描未发现其它直接返回 `err.Error()` 的位置。
- 当前结构指标：生产 package 150、单文件生产 package 47、两文件生产 package 15、`internal/app/**` external internal import union 77、`routemodule` fan-out 33、`pluginmodule` fan-out 25、`integrationmodule` fan-out 17、`module.go` 单文件 package 4、`server` 目录 173。
- 验证结果：`python scripts/check-toolchain.py`、`python scripts/check-server-structure.py`、`python scripts/ci/validate_contracts.py --mode=pr`、`cd server && go test ./... -count=1`、`cd server && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server`、`cd web && pnpm run typecheck`、`cd web && pnpm test`、`cd web && pnpm build`、`cd launcher && pnpm run typecheck`、`git diff --check` 均通过。
- 环境限制：本机未安装 `make`，`make doctor` 无法直接执行；其包含的工具链检查和结构检查脚本已分别通过，CI 也直接调用这两个脚本。

---

### 阶段 B：修正基础演进机制

目标：让 schema、配置、错误模型成为长期稳定基础。

- [x] 确定 migration 策略。
- [x] 清理重复 migration。
- [x] 建立 schema drift 测试。
- [x] 配置 secret/apply policy 改为 schema/tag 驱动。
- [x] `UpdateConfigDocument` 传递 request context。
- [x] 统一 domain error 与 HTTP error envelope。
- [x] OpenAPI 与 handler/fixtures 建立自动同步检查。

**阶段 B 验收**

- [x] 新库和老库 migration 都可验证。
- [x] 新增配置字段必须声明 apply policy。
- [x] 新增 secret 字段必须声明 redaction。
- [x] API 错误都有稳定 code。
- [x] OpenAPI drift 可被 CI 发现。

**阶段 B 完成记录（2026-06-25）**

- 根因处理：数据库演进策略明确为“当前 schema 快照 + 编号 legacy migration”，新增 `docs/architecture/storage-migrations.md`；migration runner 不再通过 duplicate-column 错误字符串跳过重复字段。
- 根因处理：`schema_migrations` 记录 `version`、`name`、`applied_at`，启动时会补齐旧 metadata 表的迁移名称；存储测试断言新库 schema、旧库升级和 fresh/migrated schema shape 一致。
- 根因处理：配置字段的 apply policy、secret 和 redaction 改由 `contracts/config.user.schema.json` 的 metadata 驱动；运行时读取内置 schema，契约校验强制新增叶子字段声明 `x-apply-policy`，secret 字段声明 `x-redaction`。
- 根因处理：`UpdateConfigDocument` 传入 request context，secret 写入与 secret ref 解析不再脱离请求生命周期。
- 根因处理：`contracts/error-codes.yaml` 补齐实际使用的 `platform.internal_error`，新增架构测试从正式错误码目录校验 management API 的错误 code，避免 handler 和契约继续漂移。
- 根因处理：CI 的 contract-lite 与自检路径改为 strict contract validation，OpenAPI、fixtures、配置 metadata 与生成物 drift 会在 CI 暴露。
- 验证结果：`python -m py_compile scripts/ci/validate_contracts.py`、`python scripts/ci/validate_contracts.py --mode=pr`、`python scripts/ci/validate_contracts.py --mode=strict`、`cd server && go test ./internal/configruntime ./internal/storage ./internal/management/... ./internal/config ./internal/schema ./tests/architecture -count=1` 均通过。

---

### 阶段 C：收束 app 与核心业务域

目标：让 app 变薄，让插件、集成、渲染自成模块。

- [x] `routemodule` 逻辑下沉到 management/domain module。
- [x] `pluginmodule` 逻辑下沉到 plugins module。
- [x] `integrationmodule` 逻辑下沉到 integrations provider/module。
- [x] 插件模型拆分。
- [x] 插件 action 改为模块注册。
- [x] integrations provider interface 落地。
- [x] render 对外接口收窄。
- [x] eventpipeline 建立主流程抽象。

**阶段 C 验收**

- [x] `internal/app/**` fan-out 低于阶段预算。
- [x] 新增业务能力不需要修改 app 大量装配代码。
- [x] 插件、集成、渲染都有清晰对外接口。
- [x] 单文件 package 数明显下降。

**阶段 C 完成记录（2026-06-25）**

- 根因处理：`routemodule` 不再逐个 import 管理 API 子包，路由注册集中到 `internal/management/router` 的模块构建入口，`routemodule` fan-out 从 33 降到 11。
- 根因处理：`pluginmodule` 不再直接装配 action、runtime、manifest refresh 和 render template sync 细节，插件侧提供 runtime registry、action adapter 和 lifecycle platform controller，`pluginmodule` fan-out 从 25 降到 17。
- 根因处理：插件状态展示经由 `management/pluginapi/view` 映射，插件 runtime 不 import management 包，新增展示字段不会自动改变 runtime state。
- 根因处理：`integrationmodule` 只保留 provider 注册和模块组合；Bilibili 扫码登录接入通用 third-party QR login service，Bilibili account/session/source/credential/subscription 装配收束到 `internal/integrations/bilibili` 模块。
- 根因处理：第三方账号 cookie 校验、用户解析和监控快照不再由 `management/thirdpartyapi` 直接引用平台内部登录、解析或 Bilibili source 类型。
- 根因处理：render 对外收敛到 service 层接口，插件模板声明同步、template error、render identity input 判断由 `render/service` 提供；外部包不再 import `render/templates`、`render/plugintemplates` 或 `render/bootstrap`。
- 根因处理：插件 action registry 从中心大表改为模块注册，后续新增 action 不需要继续扩大单个 dispatcher 映射表。
- 当前指标：`internal/app/**` external internal import union 56，生产 package 141，单文件生产 package 37，目录 163；`routemodule` fan-out 11，`pluginmodule` fan-out 17，`integrationmodule` fan-out 13。
- 验证结果：`cd server && go test ./internal/management/protocolapi ./internal/management/thirdpartyapi ./internal/integrations/douyin ./internal/plugins/actions ./internal/plugins/lifecycle ./internal/app/... ./tests/services -count=1`、`cd server && go test ./internal/render/service ./internal/plugins/actions ./internal/management/renderapi ./internal/plugins/lifecycle ./internal/integrations/bilibili/session ./internal/management/bilibiliapi ./tests/services ./tests/architecture -count=1`、`cd server && go test ./internal/management/thirdpartyapi ./internal/management/router ./internal/app/servicegraph/integrationmodule ./tests/integration -run "TestThirdParty|TestManagementHTTP|TestRouter" -count=1`、`python scripts/check-server-structure.py` 均通过。

---

### 阶段 D：目录降噪与阅读体验优化

目标：让结构更容易读、改、测、排障。

- [x] 合并低价值单文件 package。
- [x] 减少泛化文件名。
- [x] Douyin 大文件拆分。
- [x] 测试文件按场景拆分。
- [x] 补齐 server 架构图。
- [x] 健康诊断聚合 API 与页面落地。
- [x] 技术决策记录建立。

**阶段 D 验收**

- [x] package 数、目录数、单文件 package 数进入长期目标轨道。
- [x] 新维护者可以通过文档和目录快速定位修改点。
- [x] 运维可以通过健康总览定位主要问题。

**阶段 D 完成记录（2026-06-25）**

- 根因处理：删除或合并 `internal/app/actionwire`、`internal/plugins/runtime/startup`、`internal/integrations/thirdpartylogin`、`internal/protocolcap`、`internal/plugins/lifecycle/{commands,metrics,runtimeconfig}`、`internal/plugins/manifestrefresh` 等只服务单一调用方或只做转发的小包。
- 根因处理：Douyin `browser.go` 和 `resolve.go` 按浏览器运行、二维码捕获、资料抽取职责拆分。
- 根因处理：`routes.go`、`registry.go`、`http.go` 改成领域化文件名，结构指标不再出现这三类泛化文件名。
- 根因处理：所有 `part2`、`part3`、`part4` 测试文件改为场景名，并新增架构测试禁止继续使用编号式测试文件名。
- 根因处理：新增 `docs/architecture/server-lifecycle.md`、`management-api.md`、`plugin-runtime.md`、`event-pipeline.md`、`technology-decisions.md`，并修正三方扫码登录文档不再描述返回 CK。
- 根因处理：`/api/system/status` 增加 `health` 聚合字段，Dashboard 优先展示该健康总览；OpenAPI、fixture、Web/Launcher 生成类型已同步。
- 根因处理：新增登录成功响应 fixture 的凭据泄漏扫描，防止 raw cookie、CK、access token、refresh token 回到成功响应。
- 当前指标：生产 package 141、单文件生产 package 37、两文件生产 package 15、`server` 目录 163；全部不超过当前预算。
- 验证结果：`python scripts/check-server-structure.py`、`go test ./internal/bot/adapter/onebot11/shell ./internal/deps ./internal/eventpipeline/dispatch ./internal/integrations/bilibili/source ./internal/plugins/runtime/manager ./tests/integration ./tests/services ./tests/architecture -count=1`、`go test ./internal/management/... ./internal/bootstrap ./tests/architecture -count=1`、`python scripts/ci/validate_contracts.py --mode=strict` 均通过。

---

## 7. 每次 PR 的可核对检查项

### 7.1 通用检查

- [ ] 是否新增了 package？如果是，是否满足独立 package 门槛？
- [ ] 是否新增单文件 package？如果是，是否有白名单理由？
- [ ] 是否新增 `module.go`、`routes.go`、`repository.go`、`http.go` 等泛化文件名？如果是，是否有更具体命名？
- [ ] 是否增加 `internal/app/**` 的 import fan-out？
- [ ] 是否修改对外 API？如果是，OpenAPI、前端类型、fixtures、测试是否同步？
- [ ] 是否修改配置字段？如果是，apply policy、redaction policy、文档是否同步？
- [ ] 是否修改数据库 schema？如果是，migration、sqlc、测试是否同步？
- [ ] 是否涉及 secret、cookie、token、CK？如果是，是否确认响应、日志、fixtures 不泄露？
- [ ] 是否返回了 `err.Error()` 给用户？如果是，是否改为 typed error？
- [ ] 是否新增测试超过 600 行？如果是，能否按场景拆分？

### 7.2 server 架构检查

- [ ] `go test ./...` 在项目要求 Go 版本下通过。
- [ ] 架构测试通过。
- [ ] package 数未超过预算。
- [ ] 单文件 package 数未超过预算。
- [ ] `internal/app/**` fan-out 未超过预算。
- [ ] migration drift 测试通过。
- [ ] secret leak scanner 通过。
- [ ] OpenAPI drift 检查通过。

### 7.3 安全检查

- [ ] API response 不包含 raw token、cookie、CK、refresh token。
- [ ] 日志输出经过 redact。
- [ ] fixture 中仅使用显式假值。
- [ ] 配置读取接口返回脱敏字段。
- [ ] 第三方登录只返回 account/status/handle，不返回 credential。
- [ ] 错误 details 不含敏感字段。

---

## 8. 整改完成度跟踪表

| 编号 | 项目 | 优先级 | 状态 | 责任区域 | 验收方式 |
|---|---|---|---|---|---|
| P0-01 | 架构复杂度递减预算 | P0 | [x] 阶段 A 完成；后续阶段继续下调预算 | architecture/tests | 指标脚本 + CI |
| P0-02 | `internal/app/**` fan-out 治理 | P0 | [x] 第一阶段完成，`internal/app/**` external import union 56 | app/servicegraph/httpwire | 架构测试 |
| P0-03 | 单文件 package 收敛 | P0 | [x] 第一阶段完成，单文件生产 package 37 | all server | package 统计 |
| P0-04 | migration 模型修正 | P0 | [x] 完成 | storage | migration 测试 |
| P0-05 | 第三方登录 cookie 安全 | P0 | [x] 完成 | integrations/management | API fixture + security test |
| P0-06 | typed domain error | P0 | [x] 完成；错误码契约门禁已接入 | httpapi/management/integrations | 错误响应测试 |
| P0-07 | Go 工具链自检 | P0 | [x] 脚本与 CI 完成；本机缺少 `make` | scripts/docs | `make doctor` |
| P1-01 | 插件模型拆分 | P1 | [x] 完成：runtime state 与 management view 分离 | plugins | 状态映射测试 |
| P1-02 | 插件 action 模块注册 | P1 | [x] 完成 | plugins/actions | registry 测试 |
| P1-03 | integration provider | P1 | [x] 完成：登录、账号校验、用户解析走 provider/module 边界 | integrations | provider tests |
| P1-04 | Bilibili 模块化 | P1 | [x] 完成：app 只依赖 Bilibili module/root package | integrations/bilibili | app import 降低 |
| P1-05 | Douyin 大文件拆分 | P1 | [x] 完成 | integrations/douyin | 文件行数 + 单测 |
| P1-06 | render 接口收窄 | P1 | [x] 完成：外部包不再 import render 内部模板/bootstrap 包 | render | import/fan-out 检查 |
| P1-07 | eventpipeline 主轴 | P1 | [x] 完成 | eventpipeline | 架构文档 + 测试 |
| P1-08 | 配置 metadata 驱动 | P1 | [x] 完成 | configruntime/schema | config tests |
| P1-09 | 配置更新 context | P1 | [x] 完成 | configruntime/httpapi | cancel test |
| P1-10 | OpenAPI 强同步 | P1 | [x] 完成；strict contract CI + generated type checks | management/contracts | drift test |
| P1-11 | 健康诊断聚合 | P1 | [x] 完成 | system/management | health API test |
| P1-12 | 测试场景拆分 | P1 | [x] 完成：编号式测试文件已改名并加门禁 | tests | test metrics |
| P1-13 | 安全回归扫描 | P1 | [x] 完成：扫码登录成功响应 fixture 扫描已接入 | tests/security | leak scanner |
| P2-01 | 泛化文件名减少 | P2 | [x] 完成：`routes.go`、`registry.go`、`http.go` 已清零 | all server | filename metrics |
| P2-02 | server 架构图 | P2 | [x] 完成 | docs | docs path check |
| P2-03 | 技术决策准则 | P2 | [x] 完成 | docs/architecture | ADR/decision doc |

---

## 9. 最终验收门槛

整改不能只以“代码能跑”为完成标准，建议使用以下门槛作为阶段性验收。

### 9.1 结构指标门槛

- [x] 生产 package 数小于当前 baseline。
- [x] 单文件 package 数小于当前 baseline。
- [x] `internal/app/**` external internal import union 小于当前 baseline。
- [x] `routemodule`、`pluginmodule`、`integrationmodule` fan-out 均下降。
- [x] 不再新增无说明的单文件 package。
- [x] 目录总数进入递减趋势。

### 9.2 安全门槛

- [x] 配置 API 不返回明文 secret。
- [x] 第三方登录 API 不返回 raw cookie/CK/token。
- [x] 错误响应不包含敏感 cause。
- [x] 日志和 WebSocket event 经过 secret redaction。
- [x] 安全扫描测试能拦截扫码登录成功响应中的常见敏感字段。

### 9.3 数据库门槛

- [x] migration 策略清晰并有文档。
- [x] 新库初始化和旧库升级都有测试。
- [x] sqlc 与最终 schema 一致。
- [x] 不再通过 `ensure*Columns` 作为常规演进方式。
- [x] 不依赖 duplicate-column 字符串判断作为正常路径。

### 9.4 可维护性门槛

- [x] app 层只做生命周期和模块装配。
- [x] 插件、集成、渲染、事件管线对外接口清晰。
- [x] 管理 API 错误结构统一。
- [x] 测试文件按场景命名。
- [x] 架构图能对应真实代码路径。

### 9.5 最终复验记录（2026-06-25）

- `cd server && go test ./... -count=1` 通过。
- `cd server && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server` 通过。
- `python scripts/check-server-structure.py` 通过，当前指标为生产 package 141、单文件生产 package 37、`internal/app/**` external internal import union 56、`server` 目录 163。
- `python scripts/ci/validate_contracts.py --mode=strict` 通过。
- `node scripts/check-agent-docs.mjs` 通过。
- `git diff --check` 通过，仅输出 Windows 换行提示。
- `cd web && pnpm run typecheck`、`cd web && pnpm test`、`cd web && pnpm build` 通过。
- `cd launcher && pnpm run typecheck`、`cd launcher && pnpm test`、`cd launcher && pnpm build` 通过；`pnpm test` 首次出现单个 settings-store 测试 5 秒超时，单文件复跑与全量复跑均通过。

### 9.6 安全扫描复验记录（2026-06-25）

- 依赖图状态：Web 与 Launcher 固定 Vite `8.0.16`；Web overrides 固定 `esbuild 0.28.1`、`glob 10.5.0`、`js-cookie 3.0.7`、`js-yaml 4.2.0`；Launcher overrides 固定 `@xmldom/xmldom 0.8.13`、`axios 1.16.0`、`follow-redirects 1.16.0`、`form-data 4.0.6`、`glob 10.5.0`、`ip-address 10.1.1`、`js-yaml 4.2.0`、`lodash 4.18.0`、`tar 7.5.16`、`tmp 0.2.6`、`undici 7.28.0`。
- Server 依赖边界：`server/go.mod` 保持 Go `1.25.8`；开发热重载使用 `scripts/start-dev.mjs` 内置 watcher；`server/go.mod` 不包含 Air、Hugo 或 `tool github.com/air-verse/air`；`server/AGENTS.md` 记录开发辅助工具不得把与 server 运行无关的大型依赖图带入 server 模块。
- 路径安全状态：restore zip 解包、管理页面静态资源、render 模板预览路径均校验最终目标仍在授权根目录内。
- 第三方出站请求状态：集成公共 HTTP helper 校验 HTTPS、平台域名白名单、本机/私网地址和每次重定向；自动跳转与手动跳转使用同一校验；微博、抖音、网易云、订阅中心链接解析统一使用完整主机或点号子域名匹配，避免相似域名绕过。
- 输入边界状态：Bilibili 数字解析检查 int/int64 范围；Bilibili WebSocket 包体和解压数据设置上限；NetEase PKCS#7 padding 和 render 模板 payload 容量检查溢出边界；配置 diff map 容量使用安全加法。
- Web/Launcher/Python 安全状态：插件管理 iframe 使用加密随机 session/request id 与固定 target origin；Launcher 启动脚本测试使用最小 Windows 环境；订阅中心链接解析与 release 依赖源测试使用 URL host 校验；Python SDK 错误帧脱敏 credential 形态文本。
- 验证结果：`cd server && go test ./...`、`cd server && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server`、`cd web && pnpm install --frozen-lockfile`、`cd web && pnpm run typecheck`、`cd web && pnpm test -- --run tests/unit/plugin-management-ui-host.spec.ts`、`cd web && pnpm build`、`cd launcher && pnpm install --frozen-lockfile`、`cd launcher && pnpm run typecheck`、`cd launcher && pnpm test`、`cd launcher && pnpm build`、`node --test scripts/tests/start-dev-support.test.mjs`、`node --test plugins/builtin/subscription_hub/tests/*.test.mjs plugins/builtin/fortune/tests/*.test.mjs`、`python -m unittest discover -s sdk/python/tests`、`python -m unittest scripts.release.tests.test_deps_manifest scripts.release.tests.test_deps_manifest_runtime`、`python scripts/ci/validate_contracts.py --mode=strict`、`node scripts/check-agent-docs.mjs` 均通过。

---

## 10. 后续维护顺序

1. **先看结构预算。**
   目的：防止文件数、目录数、package 数重新增加。

2. **涉及数据库、配置、错误码或 OpenAPI 时先改 contract 和 schema。**
   目的：避免实现代码重新成为事实来源。

3. **涉及第三方账号、cookie、token、CK 时先跑泄漏扫描和响应 fixture 校验。**
   目的：避免 credential 再次出现在浏览器、日志或 fixture。

4. **新增插件、集成、渲染能力时优先使用现有模块入口。**
   目的：避免 app 组合根重新理解业务内部细节。

5. **新增大测试或新文件时按场景命名。**
   目的：保持测试失败和代码搜索能直接定位行为。

---

## 11. 一句话总结

当前 server 的治理重点不是继续增加模块，而是**保持 package 数下降、app fan-out 受控、低价值单文件包不回潮、migration 可验证、错误和 secret 治理不漂移**。
