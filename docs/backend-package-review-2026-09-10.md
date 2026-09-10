# 后端包结构评审与整合方案

评审日期：2026-09-10。基线：`809fec3afd39f8d810f318cd755a0edd26b088a6`。

状态：**待实施的整改建议**。本轮只新增评审文档与索引，没有迁移后端代码；本文的目标路径不代表当前实现。现行边界仍以 [Server Guide](../server/AGENTS.md)、[Platform Architecture](./architecture/platform-architecture.md) 和 [Implementation Order](./engineering/implementation-order.md) 为准。

## 1. 结论与范围

建议采用“按业务子系统归组、保持有效 Go 包边界、先收回混杂职责”的渐进整合。保留单一 Server Go module，不引入统一的 `controller/service/repository/model` 横向分层，也不把每个领域复制成多层空架子。

当前主要问题是边界粒度不一致：插件、事件管线已有子系统结构，适配器、管理事件、运行配置和技术能力仍平铺；部分包名称与实际职责不符；少数共享模型包携带数据库或高层服务依赖。重命名目录只能改善导航，不能解决后两类问题。

按本方案完整归组后，`internal/` 一级目录由 43 个收敛到 16 个。这个数字是目录映射的结果，不是质量指标；Go 包数不作为验收目标。原本需要独立生命周期的包继续独立，新增包仅用于已经存在的跨入口复用或明确的依赖隔离。

本次范围包括 `server/cmd`、`server/internal`、`server/tests`，以及依赖这些路径的脚本、生成配置、CI 和架构说明。HTTP、WS、插件 action、CLI、SQLite schema、数据目录、发行目录和版本线均按现有正式契约保持兼容。若实施时决定改变正式行为，另行先更新对应契约。

## 2. 当前规模与已有基础

统计口径：`go list` 使用本机 Windows 构建上下文；文件统计遍历 `internal/**/*.go`，包含其他平台文件，排除测试后单独标注生成代码。行数包含空行和注释。

| 指标 | 当前结果 |
| --- | --- |
| `internal/` 一级目录 | 43 |
| `internal/` Go 包目录 | 59 |
| `go list ./...` 包数 | 66，含 2 个 cmd 包与 5 个 tests 包 |
| internal 非测试 Go 文件 | 377，其中 14 个 sqlc 生成文件 |
| internal 手写非测试 Go 行数 | 73,487 |
| internal 测试文件 | 254 |
| app | 19 个生产文件，3,303 行，直接依赖 45 个本模块包 |
| management | 29 个生产文件，5,646 行，直接依赖 27 个本模块包 |

高依赖数对组合根 `app` 本身是正常现象，不能据此认定它是“大泥球”。当前值得保留的基础包括：

- App 的装配、启动失败回收和最终关闭已经集中，按 wiring 文件区分职责足够。
- `chatpolicy → bridge → dispatch → outbound`、插件 `catalog/lifecycle/runtime/actions` 已有可辨认的责任边界。
- tasks、scheduler、auth、logging 等通过包内文件表达仓储和服务职责，没有必要统一拆出 repository 子包。
- `config` 本身没有本模块依赖，是适合广泛消费的基础配置包。
- 包内单测与 `tests/architecture`、`tests/services`、`tests/integration`、`tests/ws` 已经分层。
- SQLite 迁移、sqlc 生成、发布更新核心已有稳定来源和发布引用，应避免只为外观重排。

## 3. 需要整改的结构问题

下列“优先”表示实施顺序，不是线上故障等级。

### 3.1 优先：适配器运行服务藏在 wsevents

`ProtocolService` 同时保存 OneBot/QQ 实例集合、执行配置热重载、提供协议查询，并持有 `pubsub.Hub[Frame]`。入站凭据、反向 WS 连接和 webhook 接入也由这个包暴露。证据：[protocol.go:125–180](../server/internal/bot/adapters/protocol.go)、[ingress.go:24](../server/internal/bot/adapters/ingress.go)、[protocol_targets.go:65](../server/internal/bot/adapters/protocol_targets.go)。

影响：寻找协议运行逻辑必须进入一个看似只有 WS 推送的包；管理展示和适配器生命周期难以独立演进。把整个包移入 `management/events` 会进一步固化这个错误归属。

整改：适配器集合、reload、入站能力及协议领域快照进入 `bot/adapters`；Frame 构造、管理事件投影、订阅转发进入 `management/events`。领域服务发布自己的类型化快照/变化通知，管理层消费后构造正式 WS Frame。复用现有 pubsub，不另造事件总线，也不改变订阅初始快照、顺序和关闭语义。

### 3.2 优先：插件配置和凭据业务跨入口重复

HTTP handler 直接执行配置写入、默认值合并、命令刷新和变更通知；插件 action 中有另一条同类流程。secret key 校验与命名空间也重复。证据：[plugin_ui_settings.go:86–103](../server/internal/management/plugin_ui_settings.go)、[default_registry.go:116–158](../server/internal/plugins/actions/default_registry.go)、[plugin_ui_secrets.go:147](../server/internal/management/plugin_ui_secrets.go)。

影响：以后增加校验或调整通知行为，需要同步两条写入路径；handler 继续承担领域编排。当前证据证明代码重复，不等于已经证明两条入口的所有行为发生分歧。

整改：建立 `plugins/settings`，复用现有配置仓储和 secret store，集中插件配置读写、有效值合并、凭据命名空间以及写后刷新/通知。命令刷新和通知使用构造期注入的窄接口，不导入 actions 或 management。HTTP 与 action 各自保留入口身份校验、权限控制和协议错误映射。保持 changed_keys、无变更请求、失败后的状态和通知次数等现有语义。

### 3.3 优先：聊天入口通过具体协作者依赖整个插件执行栈

内置菜单仅为 `RenderIdentityData` 导入 `plugins/actions`；Ingress 直接持有 `*lifecycle.Controller`，由此传递依赖进程 runtime。证据：[menu.go:17、406](../server/internal/builtinmenu/menu.go)、[render_identity.go:8–18](../server/internal/render/service/identity.go)、[ingress.go:14、30](../server/internal/eventpipeline/chatpolicy/ingress.go)。

整改：身份渲染投影移入现有 render 入口包，menu 和 actions 均调用它；其输入已有 chatevent/config，不需要依赖插件实现。Ingress 只声明实际使用的身份协调方法，由 App 注入 lifecycle 实现。保留 bridge、dispatch、outbound 的独立边界，不把整条事件链合成一个大包。

### 3.4 优先：通用出站边界仍固化 OneBot11

dispatch/outbound 使用 OneBot 错误类型表达通用出站失败；QQ 官方有自己的发送错误类型。更直接的行为证据是 `outboundAdapterLabel` 恒返回 `onebot11`，通用出站日志固定 `component=adapter.onebot11`。证据：[outbound_action.go:195–211](../server/internal/eventpipeline/dispatch/outbound_action.go)、[observability.go:49–59](../server/internal/eventpipeline/outbound/observability.go)、[QQ outbound.go:24](../server/internal/qqofficial/outbound.go)。

影响：经该通用路径发送的 QQ 官方消息会被记入 OneBot 标签，协议接入边界已经影响观测准确性。这是静态代码可确认的归属问题；本轮没有连接真实平台复现。

整改：稳定的发送失败分类由 `bot/chatevent` 定义最小协议中立类型或接口，适配器映射各自错误，dispatch/outbound 不导入具体适配器。指标与日志根据实际协议/发送结果归属，沿用既有标签取值约束。**行为修复单独提交**，不得藏在目录移动中；如要改变契约规定的字段或指标语义，先更新正式来源。

### 3.5 优先：共享模型包引入了不必要的高层依赖

| 当前依赖 | 证据与影响 | 调整 |
| --- | --- | --- |
| `plugins → storage/sqlcgen` | [repository.go:9–18](../server/internal/plugins/catalog/repository.go)；根包被 16 个生产包直接消费，模型消费者同时依赖 SQLite 实现 | 根保留模型和仓储接口，SQLite 实现移到已存在的 `plugins/catalog/repository.go`；不新增只放一个实现的 state 包 |
| `render/service → health → recovery → plugins` | [health.go:7–20](../server/internal/health/health.go)、[render_service.go:433](../server/internal/render/service/render_service.go)；渲染只是使用 DiagnosticIssue，却继承恢复领域和 HTTP handler 所在包 | `platform/health` 只保留中性的 DiagnosticIssue；ReadinessReport 归 `operations/system`，HTTP health handlers 归 management |
| `runtimepaths → catalog/recovery` | [paths.go:9–19、38–43、101](../server/internal/runtimepaths/paths.go)；路径包还负责插件扫描参数和清理安装残留 | 路径推导归 `platform/runtimepaths`；插件发现参数归 catalog，安装临时目录清理归 lifecycle；根路径推导由单一公共实现提供 |
| `configruntime → plugins/actions` | [deps.go:30](../server/internal/configruntime/deps.go)；只为持有 PluginLogLimiter 具体类型 | 改为消费方的 `ApplyConfig(config.Config)` 接口；运行配置仍与基础 config 分包 |

迁移插件 SQLite 实现不会要求根 plugins 反向导入 catalog：具体构造点位于 App、CLI 和跨包测试，catalog 本来就消费根模型。迁移 health 时避免把含 recovery 汇总的 ReadinessReport 一并搬到 platform，否则传递依赖仍然存在。

另一个已有跨域复用点是 [permission/checker.go:187–229](../server/internal/permission/checker.go) 的 RateLimit 参数与解析：插件 IPC 和日志限流也使用它。将这部分配置值和格式解析放入已有 `config/rate_limit.go`，再把 permission 归入 bot，避免插件 runtime 仅为配置解析依赖聊天权限仓储。各 limiter 的排队、拒绝和 cooldown 算法仍留在自身服务，不能因参数相同而合并行为。

### 3.6 后续：领域视图和离线业务尚未完全回到责任包

- scheduler 的列表排序、payload 兼容和 conversation ID 构建位于 [scheduler_handlers.go:92、197](../server/internal/management/scheduler_handlers.go)，插件名与时区还绕经 system。视图构建收回 `scheduler/view.go`，通过窄依赖提供插件名和有效时区；传输格式处理留在 management。
- CLI 中的 [restore.go:17、136、192](../server/internal/cli/restore.go) 同时实现归档校验、兼容性判断和恢复写入；[cli.go:168](../server/internal/cli/cli.go) 直接处理管理员重置 SQL。恢复能力归 `operations/recovery`，管理员重置归 auth，CLI 保留参数、执行锁、输出和退出码。
- `backup` 已依赖 recovery；从 CLI 下移恢复实现时，不要再让 recovery 导入 backup 形成环。归档读取/校验复用 recovery 已有 manifest 能力，backup 继续负责创建备份。
- system 的 `_http.go` 文件实际实现任务编排或 ZIP 构建，没有 HTTP transport。只调整为 `backup_export.go`、`diagnostics_archive.go` 等职责名称，不搬进 management。

### 3.7 后续：render 的唯一内部仓储制造重复模型

`render/repository` 仅由 `render/service` 一个生产包直接消费，约 216 行。两包分别定义 TemplateSource、TemplateFiles、TemplateSummary 等，并维护逐字段转换。证据：[repository.go:22–58](../server/internal/render/repository/repository.go)、[render_service.go:33–69](../server/internal/render/service/render_service.go)、[service_catalog.go:151–188](../server/internal/render/service/service_catalog.go)。

建议两包合为 `internal/render`，用 `repository.go`、`catalog.go`、`chromium_runner.go`、`worker.go` 等文件表达内部职责，保留 Service 作为外部入口；持久化实现和仅内部使用的类型收窄可见性。删去已不需要的复制模型与转换，而不是改名后继续保留两套。

浏览器分配器、队列和 Service 的关闭责任保持不变；已有真实浏览器测试继续由直接 owner 关闭资源。不要因文件数增长重新拆出一组没有独立责任的薄包。

### 3.8 迁移前处理：门禁依赖旧路径，且输入识别有缺口

[structure_test.go:19–99](../server/tests/architecture/structure_test.go) 只检查几组指定依赖；[check-server-structure.py:102–114](../scripts/check-server-structure.py) 对 management 的部分检查只匹配根包。它们不能识别 wsevents 的混合职责，也不能阻止通过 lifecycle 形成的进程传递依赖。搬路径后可能出现检查失效或扫描目录不存在。

另外，`docs/engineering/manual-sql-exceptions.json` 是结构检查输入，但当前变更识别把它单独判为 docs_only，server 与 ci 均不触发。证据：[detect_changes.py](../scripts/ci/detect_changes.py)、[CI server job](../.github/workflows/ci.yml)。本轮直接调用分类函数确认了这一结果。

先补这个输入触发，再随每批迁移同步明确的 import 规则。为检查脚本提供合法/非法依赖的正反例，防止新子目录绕过；不增加“目录不得为空”“测试文件必须如何命名”或包数量阈值。这些形式门禁不能证明领域边界。

## 4. 推荐目标目录

以下为必需整改完成后的结构。`bot`、`platform`、`operations` 作为目录分组，不创建汇总所有子包的 Go 根包。`bot/adapters` 则有真实的适配器管理服务，可以是 Go 包。

```text
server/
├── cmd/
│   ├── raylea-server/
│   └── raylea-updater/
├── internal/
│   ├── app/                         # 装配、Run、Close、跨域观测接线
│   ├── cli/                         # 参数、离线执行边界、输出与退出码
│   ├── management/                  # HTTP/WS handlers、认证与错误映射
│   │   └── events/                  # 管理 Frame、事件投影与订阅转发
│   ├── bot/
│   │   ├── chatevent/               # 中立事件、消息命令、发送失败分类
│   │   ├── command/                 # 命令解析
│   │   ├── menu/                    # 内置菜单服务
│   │   ├── governance/              # 聊天治理服务与视图
│   │   ├── permission/              # 权限与名单策略、仓储
│   │   ├── adapters/                # 实例管理、reload、领域快照与入站
│   │   │   ├── onebot11/
│   │   │   ├── qqofficial/
│   │   │   └── reconnect/
│   │   └── pipeline/
│   │       ├── chatpolicy/
│   │       ├── bridge/
│   │       ├── dispatch/
│   │       └── outbound/
│   ├── plugins/                     # 插件模型、接口与稳定错误
│   │   ├── catalog/                 # 声明、发现、期望状态持久化
│   │   ├── lifecycle/               # 启停、安装、卸载、替换与回收
│   │   ├── runtime/                 # 进程、JSONL、握手与事件 session
│   │   ├── actions/                 # action 校验、权限与能力委托
│   │   ├── artifact/                # artifact 校验
│   │   ├── market/                  # 商店来源、目录缓存与安装委托
│   │   ├── storage/                 # 插件私有配置、KV、文件
│   │   ├── settings/                # 跨 HTTP/action 的配置与凭据业务
│   │   └── webhook/                 # 插件 webhook 入口
│   ├── integrations/                # 保留现有第三方账号与 provider 包
│   ├── render/                      # Service 及包内模板、仓储、浏览器
│   ├── config/                      # 模型、schema、加载、校验
│   │   └── runtime/                 # 文档更新、脱敏、串行应用与通知
│   ├── tasks/                       # 后台任务执行与持久化
│   ├── scheduler/                   # 调度、领域视图与插件触发
│   ├── operations/
│   │   ├── system/                  # 在线诊断、readiness、运维任务编排
│   │   ├── diagnostics/             # 在线/离线共享检查
│   │   ├── backup/                  # 备份创建
│   │   └── recovery/                # 兼容性评估与离线恢复
│   ├── releaseupdate/               # 保留发布核心与链接器注入路径
│   ├── storage/                     # SQLite、schema、migrations
│   ├── sqlcqueries/                 # 保留现有生成输入路径
│   ├── sqlcgen/                     # 保留现有生成输出路径
│   └── platform/                    # 有明确用途的共享技术能力
│       ├── auth/
│       ├── secrets/
│       ├── logging/
│       ├── console/
│       ├── health/                  # 仅中性诊断模型
│       ├── deps/                    # 托管运行资源
│       ├── httpapi/
│       ├── pubsub/
│       ├── filelock/
│       ├── runtimepaths/            # 不含插件发现/清理流程
│       ├── logpath/
│       ├── redact/
│       └── semver/
└── tests/
    ├── architecture/
    ├── services/
    ├── integration/
    ├── ws/
    ├── testutil/
    └── testenv/                     # 从 internal 移来的 race 构建标记
```

### 4.1 完整迁移映射

表内省略 `server/internal/` 前缀；除注明拆分/合并外，保持原 Go 包边界。

| 当前路径 | 目标路径 | 处理 |
| --- | --- | --- |
| app、cli | 原路径 | 保留入口；CLI 业务按 3.6 收回 |
| management | 原路径 | 收回 settings/scheduler 业务，不逐 endpoint 拆包 |
| wsevents | bot/adapters、management/events | 先拆责任，后迁移 |
| chatevent、command、builtinmenu | bot/chatevent、bot/command、bot/menu | 先断开 menu→actions |
| governance、permission | bot/governance、bot/permission | 保持服务与策略/持久化边界 |
| onebot11、qqofficial、reconnect | bot/adapters 下同名子包 | 适配器实现保持分离 |
| eventpipeline/* | bot/pipeline/* | 保留现有 4 个包 |
| plugins 根包 | 原路径 | SQLite 实现移入 catalog，保留模型/接口 |
| plugins/catalog、lifecycle、runtime、actions、artifact、webhook | 原路径 | 收回相关业务或收窄依赖，不合成一个包 |
| pluginmarket | plugins/market | 与私有存储在名称上明确区分 |
| plugins/pluginstore | plugins/storage | Config/KV/File 仍属插件私有数据；与 SQLite 基础包区分 import 别名 |
| 两入口 settings/secrets 编排 | plugins/settings | 真实复用服务，不复制底层 store |
| integrations/* | 原路径 | 本轮保持账号、扫码、校验与 provider 的边界 |
| render/service、render/repository | render | 合包并删除内部重复模型 |
| config、configruntime | config、config/runtime | 归组，不合包 |
| tasks、scheduler | 原路径 | 包内文件组织，scheduler 接回领域视图 |
| system、diagnostics、backup、recovery | operations 下同名子包 | 保留在线/离线边界；收回 CLI 恢复 |
| releaseupdate、storage、sqlcqueries、sqlcgen | 原路径 | 保留发布与数据库生成链 |
| auth、secrets、logging、console、deps、httpapi | platform 下同名子包 | 独立共享能力，只调整归组 |
| pubsub、filelock、logpath、redact、semver | platform 下同名子包 | 保持小包；已有多个真实消费者 |
| health | platform/health、operations/system、management | 分开中性诊断、readiness 组合模型与 HTTP handlers |
| runtimepaths | platform/runtimepaths、plugins/catalog、plugins/lifecycle | 路径推导、发现和安装清理分别归属 |
| testenv | server/tests/testenv | 保持无业务依赖，不并入较重的 testutil |

### 4.2 暂不纳入的扩大重构

- 不把独立 module 拆成多个 module，不新增平行数据库或技术栈。
- 不因 `integrations/bilibili/session` 层级较深、`fingerprint` 只有一个生产消费者，就同时改 provider 架构。`thirdparty` 同时承载账号类型和存储服务的耦合可另评，但本轮不再增加一套 models/services 包。
- 不把 lifecycle 与 runtime、market 与私有存储、config 与 config/runtime 合包；这些边界有不同生命周期、消费者或依赖方向。
- 安装/卸载有独立队列，具备日后抽为 `plugins/install` 的依据；本轮先保留在 lifecycle，以文件区分事务和进程协调，避免在修运行边界的同时扩大安装/回滚变更。大文件拆分可在原包内完成。
- 不把所有小工具合进 common/utils。command、pubsub、filelock、reconnect 等有真实复用，文件少不是合包理由。
- 不统一 `semver.Compare` 与 releaseupdate 的严格版本解析：前者接受已验证输入并容忍来源格式，后者承担发布校验。目录整改不能顺手改变版本信任语义。
- SQL 输入/输出以后可单独评估归入 storage，但不作为本次完成条件；当前集中生成与统一 schema 来源继续保留。

## 5. 依赖与生命周期验收规则

目录分组不是统一层级。例如 `plugins/actions` 可以调用 bot 出站服务，而 `bot/pipeline/dispatch` 可以消费 plugins 根模型；禁止的是具体反向实现依赖和循环，而不是一律禁止两个子系统互相出现 import。

| 责任方 | 允许的方向 | 必须阻止的方向 |
| --- | --- | --- |
| app | 装配所有必要服务与入口 | 业务包反向 import app/cli |
| management/events | 消费领域快照并构造 Frame | 持有适配器集合、执行 reload 或进程控制 |
| config 根包 | schema 与标准配置模型 | import config/runtime 或运行服务 |
| platform/* | 按需依赖配置、storage/sqlcgen 与其他明确技术包 | import management、app、bot 执行管线、插件实现或运维编排 |
| bot/chatevent、plugins 根模型 | 必需的基础类型 | 具体适配器、runtime、管理层、SQLite 实现 |
| bot/pipeline | 中立事件、插件声明/投递接口、出站服务 | 依赖 lifecycle/runtime 实现；dispatch/outbound 依赖具体协议错误 |
| bot/adapters | 两协议实现与中立消息类型 | import management/events 或 config/runtime |
| plugins/settings | 插件私有仓储、secret store、注入的刷新/通知接口 | import HTTP handlers 或 action registry |
| plugins/lifecycle | 编排 catalog/runtime/dispatch/scheduler 等现有协作者 | 在 runtime 或 management 内复制安装和回收状态机 |
| render | 配置、资源、存储、中性诊断/事件模型 | 为诊断或身份投影依赖 actions/lifecycle/recovery |

`ErrProtocolStopped` 当前由 configruntime 定义、wsevents 返回。拆协议服务时，把内部 reload 状态/错误归属移到 adapters；config/runtime 消费它或注入的 reload 结果，adapters 不再导入 config/runtime。只改内部所有权，保持现有“需要重启”结果映射。

App 仍是运行资源的最终 owner：构造失败清理、Stop/Close 顺序、任务 drain、扫码会话与 Chromium 释放、数据库与配置锁释放均沿用现有链路。领域内共享状态继续由已有锁或原子快照保护；迁移不复制第二份运行状态。

## 6. 分批实施与交付顺序

每批形成可编译、可回退的逻辑提交。下表是工作包，不要求一个工作包挤进一个 commit；行为变化、边界调整与纯路径替换应可分别评审。不创建长久 re-export 兼容包，也不跨提交留下断裂的 import。

| 批次 | 工作 | 完成标准 | 相对风险 |
| --- | --- | --- | --- |
| A：建立迁移保护 | 修 SQL 例外登记的 CI 输入识别；为随后改变的边界规则补正反例；记录包和路径基线 | 检查覆盖其真实输入，新目录不能绕过规则；不重引形式检查 | 低 |
| B：解除模型与辅助依赖 | 插件 SQLite 实现回 catalog；身份投影回 render；Ingress 使用窄接口；health、runtimepaths 和配置 limiter 收窄；限流参数解析归 config | 模型不携带 SQLite；render 不再经 health 导入恢复栈；聊天入口不依赖 lifecycle/runtime 实现 | 中 |
| C：收回运行服务 | 从 wsevents 分出 adapters 运行服务；管理 Frame 留 events；处理 reload 错误归属 | 多实例、reload、入站与 WS 订阅结果保持兼容，关闭仍只有一个 owner | 高 |
| D：统一业务入口 | 引入 plugins/settings；收回 scheduler 视图、CLI restore 与 admin reset | 两个 settings 入口共享写入副作用；离线锁、恢复结果与退出码保持兼容 | 中到高 |
| E：修出站协议边界 | 中立失败分类与两协议观测归属 | OneBot/QQ 各自成功和失败能正确归属；独立行为修复提交 | 中 |
| F：执行目录映射 | 依次归组 platform、bot、plugins、config、operations；迁 testenv；每批同步所有调用方与门禁 | 每个提交构建通过，旧生产 import 消失；16 个一级目录目标完整落地 | 中 |
| G：合并 render 内部包 | 合并 service/repository，收窄实现可见性，删除复制模型和无效转换 | 模板来源、所有权校验、缓存与 artifact 行为不变，浏览器资源可关闭 | 中 |
| H：收尾 | 更新当前代码地图与相关 AGENTS 路径；全体 Server 门禁与产物检查；将本方案状态改为已完成并按仓库规则整理 | 文档描述真实实现；没有遗留兼容壳、失效扫描路径或未披露验证缺口 | 中 |

B 是 C/F 的依赖准备；A 的门禁路径更新与每批代码迁移同提交保持可运行，不能提前把仍使用的旧路径全部替换。E 可独立于大部分目录迁移实施。D 中 CLI 恢复可另一个工作包执行，避免与适配器并发状态重构混在同一审阅范围。

## 7. 路径同步清单与验证

### 7.1 同步实际受影响的引用

| 引用位置 | 迁移时要核对的内容 |
| --- | --- |
| [结构测试](../server/tests/architecture/structure_test.go)、[错误码测试](../server/tests/architecture/error_codes_test.go) | 扫描根、允许入口、protected import 前缀；合并 render 后用新边界替换旧 repository 保护规则 |
| [结构脚本](../scripts/check-server-structure.py)、[SQL 例外登记](./engineering/manual-sql-exceptions.json) | 新路径、management 子包覆盖、手写 SQL 的责任与登记 |
| Go 测试中的 fixtures/schema 路径 | 包目录深度变化后 `../../..` 等相对定位；尤其 lifecycle/runtime、config/runtime 和 integrations 调用方 |
| [testutil 路径定位](../server/tests/testutil/paths.go) | 保持 testutil 所在层级；testenv 迁移不应影响测试根定位 |
| [server/sqlc.yaml](../server/sqlc.yaml) | 本方案保留输入/输出路径；若后续另搬，必须 generate/diff 一起处理 |
| [runtime schema 生成器](../scripts/generate-runtime-schemas.mjs)、[config embed](../server/internal/config/schema.go) | config 根及其 contracts 目录保留，不能随 runtime 子包误搬 |
| [发布工作流](../.github/workflows/release.yml) | releaseupdate 的三处 `-X` 信任根注入、storage schema 版本提取；本方案保持这些路径 |
| [恢复演练](../scripts/release/rehearse_data_migration.py) | migrations 定位、真实 Server 备份/恢复调用 |
| [Server Guide](../server/AGENTS.md)、架构代码地图 | pubsub、SQL 生成链、render、协议与事件的实际路径，跟随每批结果更新 |

数据库 SQL、schema 或 migrations 内容不因 Go import 迁移而改变。若只是路径移动，也不能顺便提升 schema version、改 migration 编号或修改已发布 migration。

### 7.2 按风险执行的验证

在仓库根执行结构/文档脚本；Go 命令在 `server/` 内执行，Windows 按 baseline 使用 gbash。

- 每批迁移：`python scripts/check-server-structure.py`、`go test ./tests/architecture -count=1`，再运行迁移包及直接调用方的既有测试。依赖清单覆盖 native `go list`，另检查 OS build-tag 文件中的引用。
- 检查工具变化：现有 Python 脚本测试和 `scripts/ci/detect_changes.py --self-test`，加入对应输入分类与非法依赖回归；这类测试证明工具规则，不复制业务实现。
- B/C：复用 adapter reload、协议 HTTP、WS 初始快照/订阅、App 失败回收与关闭测试；关注多实例、新增实例需重启、停机时 reload、订阅者退出。并发所有权调整在支持环境运行目标包 race。
- D：HTTP 与 action 的 settings 默认值、changed_keys、无变更、错误路径、通知/命令刷新；scheduler 展示与时区；CLI 恢复兼容性、锁冲突和管理员重置。优先复用已有测试，仅对缺失的可观察风险补用例。
- E：OneBot11/QQ 官方发送与失败分类、指标标签和日志归属；不要仅断言某个 helper 返回字符串。
- G：模板来源与所有权、插件模板同步、渲染失败与取消、队列关闭、Chromium 进程/临时 profile 回收；真实浏览器测试前确认测试资源可用。
- 整体合并前：沿用当前 CI 的 doctor、embedded schema 验证、`go test ./...`、Server 构建与 `sqlc diff`；覆盖 updater 入口，Windows/Linux/macOS 的相关构建验证按现有平台矩阵执行。
- 如修改生成输入：执行对应 generate 和 drift 检查，不因为碰到 contracts 就运行无关生成链。没有改变 Web/Launcher/SDK 的正式输入时，不制造它们的代码 diff。
- 文档收尾：`python scripts/check-doc-links.py`、`git diff --check`；修改 AGENTS 后再运行 `node scripts/check-agent-docs.mjs`。查看最终产物和变更文件范围。

仅包搬移不默认新增业务测试。全量 race、平台集成或真实协议连接未执行时，明确记录缺口，不能以架构测试通过替代运行效果。

## 8. 本轮验证与交付状态

- `go list ./...`：通过；本机构建上下文下没有发现 import cycle，已取得实际包依赖清单。
- `go test ./tests/architecture -count=1`：通过。
- `python scripts/check-server-structure.py`：通过，0 warning。
- `python scripts/check-doc-links.py`：通过，检查 134 个 Markdown 文件；`git diff --check` 与新增方案文件的空白/UTF-8 检查通过。
- SQL 例外登记的变更分类：已直接调用分类函数，确认它单独变化时未触发 server/ci。
- 已核对当前源码、脚本、CI 和近 15 次 server 相关提交；没有把已完成并删除的旧 cleanup plan 当成本轮待办。
- 本轮未运行全量后端测试、race、真实平台连接、浏览器集成或发布构建；未实施代码整改。

建议从 A+B 开始，随后处理 C+D+E；这些批次解决真实耦合和重复业务。F+G 完成目录与包整合，H 确认工程链和文档跟上最终结构。
