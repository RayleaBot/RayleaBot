# 项目全面优化执行计划 v2

- 状态：执行中，按 §3 台账推进。
- 当前执行入口：本文件。三份输入评审保留为历史依据，不再分别回写其阶段清单。
- 代码基线：`809fec3a`；评审归档提交：`2c358c93b55c0ad014b94d4f597ed797c8144a17`。
- 核实日期：2026-09-10；核实对象为当前工作区源码、契约、测试、生成脚本及 CI。
- 分发前提：**按尚未分发或全新分发设计，直接建立目标版本的配置、数据库、协议与安装产物。** 实施与验证使用隔离测试数据，本机运行数据不作为清理对象。

## 1. 目标、范围与执行规则

目标是修正实际行为缺陷，统一正式语义和重复业务，建立清晰的状态与资源所有权，减少维护成本，并使后续发布能够用可观察结果验收。覆盖 Server、Web、Launcher、CLI、SDK、契约、fixtures/examples、脚本、CI、发布和文档。

输入评审：

| 简称 | 来源 | 引用方式 |
| --- | --- | --- |
| R | [仓库评审与重构计划](./repository-review.md) | 保留 D/G/T/H/Q/S/C/R/W/X 原编号 |
| V1 | [全面清理评审与执行计划 v1](./execution-plan-v1.md) | 章节号与条目名称 |
| B | [后端包结构评审与整合方案](./backend-package-review-2026-09-10.md) | 3.1–3.8 等章节号 |

执行规则：

1. 新增或改变对外语义，先确定契约，再同步受影响实现、样例、生成物、消费者与文档；修复已有契约偏差不为迁就实现放宽契约。
2. 配置、数据库、协议和 SDK 按目标结构直接收敛；删除历史迁移、格式转换、旧密码算法和旧协议分支，验证从空目录初始化的完整链路。
3. 本版本的备份恢复、安装事务回滚、进程关闭、并发保护、凭据隔离和发布验证继续属于产品能力；损坏输入应明确报错。
4. 一个逻辑变更一个提交。缺陷修复、对外语义变更、测试替代与纯路径搬移分别可评审；每个提交保持可编译，按风险完成验证。
5. 删除前验证真实消费者：生产入口、生成器、build tags、SDK/export、反射/资源引用与测试用途。代码行数、包数、中文数量、setter 数量和等待时长不作为通过指标。
6. 不升级冻结工具链，不引入新技术栈；生成器或共享库先证明能减少重复且保留语义。新依赖须在任务记录中写明必要性。
7. 独立业务插件按目标协议构建和接入：本仓库负责契约、SDK、示例与接入说明；其他仓库的源码修改与发布另行跟踪，不混入主仓库提交。
8. 每个任务开始前重查 Git 状态与证据。已由其他变更解决的项标为“无需实施”并记录证据，不为了完成计划制造改动。

## 2. 核实结论与方案裁决

“确认”表示已核对源码或执行复现；“部分成立”表示问题存在但原归因、范围或处理方式需修正；“待验证”必须先实验才能实施。下表同时规定后续任务的边界。

| 事项 | 当前核实结果与证据 | 本计划裁决 |
| --- | --- | --- |
| CI 分类阻塞 | `scripts/ci/detect_changes.py:114,193–198,252` 未识别 `go.work.sum`、`design-qa.md`，自测已失败；SQL 例外 JSON 单独变化被判 docs_only | P01 优先修复，保留未分类输入的显式失败 |
| 多实例 OneBot 路由 D1 | `app/plugin_wiring.go` 与 `plugins/actions/onebot_registry.go` 固定主 Shell，动作处理丢弃 ParentEvent；`event_wiring.go` 选主实例，`service_build.go:182` 用它注入 MetadataEnricher | P03 先修行为，再由 P14 分离实例注册与管理事件 |
| 仪表盘文案分支 D2 | `web/src/lib/management-summary.ts:115` 的 protocolSummary 仅由死导出调用；实际活跃的是 readableSummary，以及 recovery 对 summary 中 Chromium 的匹配 | 原影响范围需修正；P22 删除死导出并修活跃分支、RetryPanel 和 exception-status |
| Run 监督器 D4 | `app/app_run.go:216–269` 同一个 `sync.Once` 包住阻塞服务与错误上报；临时隔离复现确认 report 阻塞、后台错误丢失 | P02 拆启动与首错同步，覆盖顺序和取消 |
| 安装任务假成功 G5/V1 | `app/app_run.go:302,310,318,320` 等后置/回滚错误被丢弃 | P04 明确已落盘、回滚失败等终态，不能只把所有错误改成同一种失败 |
| 请求体解码 V1 | management 有 7 处直接 `json.NewDecoder(r.Body)`，与 `httpapi.DecodeStrictJSON` 的限额/单对象检查分离 | P04 统一 HTTP 边界；各端点保持真实 schema 与限额差异 |
| 默认配置 D3/H2 | schema、Go canonical 默认、Web 初始表单存在副本；default.yaml 有科学计数法，而且当前是可覆盖的有效配置层，并非纯样例 | 科学计数法不等于解析失败；P08 统一为 schema 默认与 user.yaml，并同步全部消费者 |
| 错误码检查 D5/H1 | 架构测试有 writer 与部分 DomainError/SystemHTTPError 检查，但漏包装函数和其他包 | P05/P07 收敛目录并补实际输出验证；生成 string 常量不能阻止任意字符串，不能直接删检查 |
| 契约检查命令 | `validate_contracts.py --self-test` 提前返回，只验证内部 CLI fixture 自测 | 自测与 `--mode strict` 必须分开执行；二者本轮均通过 |
| Launcher 校验 G2 | Go 返回 JSONObject，Renderer 手工镜像 shape；契约部分响应为 `additionalProperties:false` | P23 在 Go HTTP 边界验证必填/枚举/范围，再由 Wails 生成绑定；未知字段策略先改契约，不能靠 TS 类型替代运行时验证 |
| 插件状态 Q3 | Catalog 含声明/期望状态与启动投影，Manager 管进程会话，Dispatcher slot 管队列、并发和旧实例 drain | P13 划清权威来源与投影一致性，保留不同生命周期的必要状态 |
| App.Close 与 nil G1 | `app/app.go:153–165` 构造失败调用局部清理；尚未构造的资源确实可能为空 | 必选依赖构造时检查；保留部分构造失败与幂等关闭，禁止机械删除所有 nil 分支 |
| 配置热更新 V1 | `configruntime/apply.go:272–344` 有串行写入，各派生对象依次更新 | 确认跨对象发布非一个事务；尚未证明所有路径有数据竞争。P08 定义需一致的读取组与失败语义，按组快照，避免承诺所有外部副作用可原子回滚 |
| 深复制与 semver R3 | 多处 clone 处理嵌套 map/slice；发布严格解析与内部容错比较承担不同输入责任 | 不以 `maps.Clone` 替换深复制，不统一成宽松发布解析；只收敛等价部分 |
| 测试删除 T1–T4 | services harness 复刻装配成立；Web 启动、构建配置及 Launcher 构建失败恢复测试包含真实风险 | P20/P22 先替代行为覆盖再删重复用例，脚本测试长于脚本不是删除理由 |
| 属性测试与等待 V1 | `plugins/catalog/catalog_test.go:20–24,49–53,78–79` 已使用 rapid 生成器；lifecycle 的 5.25s 等待覆盖 5s 退出预算回归 | 前者误报；后者需可控时钟/预算后替代，不实行“100ms 以上等待清零” |
| 前端组件 Q7 | `ui/*` 为基础组件，AppButton/AppDialog 有加载、无障碍、焦点和退出行为 | 保留基础层与产品封装，按真实职责删冗余；不执行“组件层二选一” |
| E2E T5 | `web/playwright.production.config.ts` 已有 real-server 与 plugin-ui-fixtures | 扩展真实链路覆盖，逐项淘汰 mock 中的业务复制，不从零建立第二套基础设施 |
| trusted_code_confirmed V1 | 商店 boolean 与普通安装 const:true 有不同流程；`pluginmarket/service.go:363,382` 仅特定原因要求确认 | 保留首次安装/来源变化/权限扩张的条件策略，明确契约与测试，不机械统一为 const:true |
| 历史表与迁移 X1/X2/R5 | 订阅遗留表、配置 v3 迁移、旧摘要升级链仍存在 | P09 删除历史链，以目标 schema 建立单一初始化基线 |
| 通用归档与下载 V1 | 多套实现存在，但备份、插件、托管资源、更新的格式、上限和信任来源不同 | P10/P11 提取最小技术原语，策略留在调用方，逐消费者替换 |
| UA 与 HTTP transport V1 | Bilibili IdentityProvider 使用多组指纹；插件 HTTP 每次 Clone transport 并设置授权 DNS 拨号 | 不统一成一个 UA；连接池只在性能测量与授权隔离回归后决定，不直接共享可变 transport |
| 文档与源码治理 S/Q6 | README 的 JSONL v1、CLI 在线标记、Go-only 和签名商店说明与当前正式来源不符；路径检查正则有不可达分支 | P25 修事实与归属；保留安装/恢复步骤与历史变更必要信息，不把所有文档缩成索引 |
| 外部 skill/忽略规则 X4/S4 | 根 AGENTS 明确外部 skill 由上游维护；仓库仍有 Python 工具与测试 | 不修改外部 skill 内部实现；是否移出 vendored 单独评估安装可复现性；不把所有 Python 忽略规则视为垃圾 |

### 2.1 统一的后端目标

采用 B 的“先收回职责、再按领域分组”，不采用 R 的约 35 包目标或 V1 的 runtime/actions、pipeline 全合包方案。下表给出目标，未列路径保持现状。目录分组不自动成为 Go 根包。

| 当前 | 目标 | 必须先完成的语义工作 |
| --- | --- | --- |
| app、cli、management | 保留 | app 为装配/Run/Close owner；CLI 与 HTTP 收回业务到领域服务 |
| wsevents | bot/adapters + management/events | 实例管理/reload/入站与 Frame/订阅分开；reload 错误不由 configruntime 拥有 |
| chatevent、command、builtinmenu、governance、permission | bot 下对应包，builtinmenu→menu | 中立模型/窄接口，解除 menu→actions 等实现依赖 |
| onebot11、qqofficial、reconnect | bot/adapters 下对应包 | 实例路由、身份与出站错误中立化 |
| eventpipeline 四个子包 | bot/pipeline 下保留边界 | chatpolicy/bridge/dispatch/outbound 按责任协作，不合成大包 |
| plugins 根包 | 保留稳定模型/接口 | SQLite 实现移 catalog；领域视图保留有效字段投影 |
| plugins/catalog、lifecycle、runtime、actions、artifact、webhook | 保留 | 声明、进程、安装与投递职责分清 |
| pluginmarket、plugins/pluginstore | plugins/market、plugins/storage | 商店与插件私有数据明确区分 |
| HTTP/action 配置与凭据编排 | plugins/settings | 统一默认值、changed_keys、通知、命令刷新、secret 引用 |
| render/service、render/repository | render | 去重复内部模型，保留浏览器/队列关闭责任 |
| config、configruntime | config、config/runtime | 配置模型不反向依赖运行服务 |
| system、diagnostics、backup、recovery | operations 下同名包 | 保留在线/离线边界，CLI restore 复用 recovery |
| health | platform/health + operations/system + management | 中性诊断模型、readiness 编排、HTTP 分开 |
| auth、secrets、logging、console、deps、httpapi、pubsub、filelock、logpath、redact、semver | platform 下同名包 | 保留真实技术能力，不合入 common/utils |
| runtimepaths | platform/runtimepaths + plugins/catalog/lifecycle | 只保留路径计算，发现/清理回领域 |
| testenv | server/tests/testenv | 不并入携带重依赖的 testutil |
| integrations、tasks、scheduler、releaseupdate、storage、sqlcqueries、sqlcgen | 保留 | 不借归组改 provider 架构、链接器路径或 sqlc 输入输出 |

禁止业务包反向依赖 app/cli/management；中立事件/模型不得依赖具体协议、runtime 或 SQLite 实现；management/events 不持有适配器生命周期。Go 无循环只是最低条件，还要检查通过具体协作者形成的传递耦合。接口由实际消费者需要决定，不为凑依赖箭头增加空壳。

## 3. 推进顺序与可回写台账

优先级：P0 为阻塞验证或已确认行为错误；P1 为语义、数据与资源边界；P2 为收敛和维护。规模 S/M/L 表示相对改动面，L 必须拆成多个逻辑提交，不代表固定工期。

状态只使用：未开始、进行中、阻塞、待验收、完成、无需实施。标“完成”需有提交、测试证据和剩余限制；代码改完但缺平台验证时仍为待验收。工作包内已完成子项在 §7 单独记录。

| 完成 | ID | 优先级/规模 | 工作包 | 前置 | 状态 | 提交 / 验证 / 阻塞 |
| --- | --- | --- | --- | --- | --- | --- |
| [ ] | P01 | P0/S | CI 输入分类与基线 | — | 待验收 | 分类/结构/Windows/core race/Linux 目标 lint 通过；托管 CI 待运行 |
| [ ] | P02 | P0/M | 监督器与资源关闭 | P01 | 进行中 | 首错、完整关闭与快照任务回收 |
| [ ] | P03 | P0/M | 多实例路由与事件补全 | P01 | 进行中 | 动作/元数据子任务完成；集合摘要与占位 Shell 后续 |
| [ ] | P04 | P0/M | 安装失败传播与 HTTP 输入边界 | P01 | 进行中 | HTTP 子任务完成；安装后置/回滚待实施 |
| [ ] | P05 | P1/L | 错误码与结构化失败语义 | P01 | 未开始 | — |
| [ ] | P06 | P1/L | 契约、样例与校验覆盖 | P01 | 未开始 | — |
| [ ] | P07 | P1/L | 错误目录、协议与版本生成链 | P05、P06 | 未开始 | — |
| [ ] | P08 | P1/L | 默认配置、类型化与热更新 | P05、P06 | 未开始 | — |
| [ ] | P09 | P1/M | 数据初始化基线与历史代码清理 | P06、P08 | 未开始 | — |
| [ ] | P10 | P1/L | 文件路径与归档原语 | P04、P06 | 未开始 | — |
| [ ] | P11 | P1/M | 资源清单与下载一致性 | P06、P10 | 未开始 | — |
| [ ] | P12 | P1/M | 插件 settings 业务统一 | P05、P06 | 未开始 | — |
| [ ] | P13 | P1/L | 插件状态、生命周期与装配 | P02、P04、P12 | 未开始 | — |
| [ ] | P14 | P1/L | 适配器服务与管理事件分离 | P03、P08 | 未开始 | — |
| [ ] | P15 | P1/M | 中立出站与观测归属 | P03、P05 | 未开始 | — |
| [ ] | P16 | P2/M | 模型、视图与辅助依赖收回 | P12、P13 | 未开始 | — |
| [ ] | P17 | P2/M | Render 合包与资源回收 | P16 | 未开始 | — |
| [ ] | P18 | P2/M | 运维、调度与 CLI 业务归属 | P09、P16 | 未开始 | — |
| [ ] | P19 | P2/L | 后端目录分组迁移 | P08、P10–P18 | 未开始 | — |
| [ ] | P20 | P1/L | 测试体系替代与离线回归 | P01；随各行为包推进 | 未开始 | — |
| [ ] | P21 | P2/M | 死代码、重复 helper 与集成精简 | 相关行为包、P20 | 未开始 | — |
| [ ] | P22 | P1/L | Web 状态、错误、请求与真实 E2E | P05–P08、P12 | 未开始 | — |
| [ ] | P23 | P1/L | Launcher 类型边界与失败可见性 | P05、P06、P11 | 未开始 | — |
| [ ] | P24 | P2/M | 工具链、开发与发布脚本收敛 | P07–P11 | 未开始 | — |
| [ ] | P25 | P2/M | 文档、文案、资产与说明归属 | 已完成任务随轮更新 | 未开始 | — |
| [ ] | P26 | P1/L | 全新分发验收与交付准备 | P01–P25 | 未开始 | — |

执行波次：

1. P01 解锁，P02/P03/P04 分支并行；P05/P06 核实并冻结新的正式语义。P20 同步补保护。
2. 契约稳定后并行推进配置/数据、归档/资源、插件业务、Web/Launcher；同一文件或同一状态机只保留一个编辑 owner。
3. 完成职责收回后做 P19 纯路径迁移。P21/P24/P25 逐批收敛，不积累一个巨型收尾提交。
4. P26 对完整产物验收。只有涉及的完整链路验收通过才勾选任务，不用“目录搬完”代表项目优化完成。

## 4. 工作包定义

下列路径省略 `server/internal/` 时均指 Server 内部源码；现有路径为核实入口，P19 后按迁移结果同步。

### P01 CI 输入分类与基线

- 来源：R-X5、V1-1.3/1.8、B-3.8。入口：[detect_changes.py](../scripts/ci/detect_changes.py)、[CI](../.github/workflows/ci.yml)、[结构检查](../scripts/check-server-structure.py)。
- 修 `go.work.sum`、根草稿及 `docs/engineering/manual-sql-exceptions.json` 分类；后者触发实际消费者且不再 docs_only。加入单项、混合、重命名/删除与未知路径正反例。
- 记录当前包依赖、真实测试入口、生成输入输出和现有失败；为新增目录规则提供非法依赖反例，避免只替换字符串后检查空转。
- 将现有核心 lint 与关键并发包 race 纳入相应 PR；Windows 专属回归进入 Windows job。全量覆盖率/长时自托管仍按现有成本层级执行。
- 验收：分类自测通过；逐项输入触发正确 job；合法/非法依赖样例分别通过/拒绝；lint 实跑后记录结果，不沿用旧评审“零问题”统计。

### P02 监督器与资源关闭

- 来源：R-D4/G1/Q1；入口 `app/app_run.go`、`app/app_close.go`、`app/app.go`、`integrations/accountvalidation/service.go`。
- 每个后台任务独立启动；缓冲首错通道配合取消，HTTP 正常退出不能吞掉已发生的后台失败。区分 listen 失败与其他任务失败。
- 梳理 App 最终 owner 与局部构造失败清理；保留 Stop/Close 幂等、任务 drain、扫码、Chromium、数据库与锁释放。
- 任一 Stop/closer 失败仍继续释放其他资源，聚合保留任务首错与清理错误；errCh 与 ctx.Done 同时就绪时不随机吞掉真实任务错误。
- 验收：后台先失败、HTTP 先失败、HTTP 阻塞时后台失败、首错与取消竞争、closer 失败及二次关闭均有可观察断言；部分构造失败不残留资源；相关并发测试在支持环境运行 race。

### P03 多实例路由与事件补全

- 来源：R-D1/H4/S2；入口 `app/event_wiring.go`、`app/plugin_wiring.go`、`plugins/actions/onebot_registry.go`、`onebot11/shell_event_metadata.go`。
- 依据事件 `source_adapter`/目标 adapter_id 选择实例；先核对每个 action 是否已提供所需上下文。无父事件的主动动作不得隐式落到首个实例，缺少选择信息时按契约明确失败或要求显式选择。
- 复用 `eventpipeline/outbound/router.go` 已有 message.send 选路规则。普通本地动作若需新增 selector，先补协议/SDK/fixtures，不能偷读未声明字段；HTTP/WS summary 统一为目标多实例语义，删除单实例派生入口。
- 元数据补全、身份缓存、动作请求和诊断归属使用同一个实例标识；移除未启动占位 Shell，真正无能力时返回稳定错误。
- 验收：至少两个 OneBot 实例使用相同 group/user ID 时缓存隔离；覆盖 QQ-only、无适配器、禁用/未知/协议不匹配实例、移除/重排、无上下文动作与 parent event/显式 selector 冲突；断言正确连接收到请求、另一连接零请求，不能仅测 registry getter。

### P04 安装失败传播与 HTTP 输入边界

- 来源：R-G5/G4，V1-1.1/1.2；入口 `app/app_run.go`、`plugins/lifecycle/controller.go`、`management/plugin_ui_*.go`、`management/plugins.go`、`httpapi/httpapi.go`。
- 分两个逻辑变更：安装后置/回滚错误传播；管理请求体统一解码。前者定义操作已提交后重载失败、清理失败和回滚失败的状态/重试方式，必要时先由 P05 补契约。
- 安装服务显式声明 Inspect 等实际能力；必选依赖装配失败立即报错，可选能力用清晰不可用结果。
- 统一限额、未知字段、尾随 JSON、空体和错误映射；不要对本来接受任意 JSON 的插件配置误加固定字段限制。
- 验收：安装重载失败不报成功，回滚错误可定位且无凭据；超限、多个 JSON、非法字段与合法边界输入覆盖全部替换入口；保留鉴权/CSRF/Origin 检查。

### P05 错误码与结构化失败语义

- 来源：R-D2/D5/H1/W1/C1/C3，V1-1.1/1.7。入口：[错误目录](../contracts/error-codes.yaml)、[OpenAPI](../contracts/web-api.openapi.yaml)、`management/plugin_store.go`、`plugin_ui_http.go`、`recovery`、`diagnostics`、`system/readiness.go`。
- 建立 code→scope/status/message_key/details 表：资源不存在 404 与运行依赖缺失 503、请求错误 400 与状态冲突 409 分清；插件内部失败的 HTTP 映射明确登记。
- 区分 HTTP error、action error、诊断检查 ID、任务结果 code，不把所有字符串都登记成 HTTP 错误；清理死码前搜实际序列化/外部文档与 SDK。
- 消除基于文案的业务分支；内部用 `errors.Is/As` 或领域枚举，跨进程用正式 code/details；scheduler 对 code 的 timeout 子串也改为显式分类。
- 领域 code 与人类消息分离，但日志/CLI 仍可输出有意义文本。固定日志模板，动态值进入脱敏后的结构化字段；前端错误资源与短提示可分职责，避免生成覆盖人工说明。
- 保留 request_id、脱敏后的安全 message、send_unconfirmed 不自动重发语义；本地网络/取消错误与服务端目录分开。`store_schema.go` 检查建表 SQL 属于旧结构识别，不能误作匹配错误文案，随 P09 清理。
- 验收：真实错误响应的 code/status/key/details 与契约一致；服务端 message 改变不改变 Web/Launcher 行为；未知 code 有稳定降级，安全字段仍严格校验。

### P06 契约、样例与校验覆盖

- 来源：V1-1.7、R-X5/R6；入口 `contracts/`、`fixtures/`、`examples/http/`、`scripts/ci/validate_contracts.py`。
- 逐项处理插件重复 shape、过时 required、列表有界性/排序/分页、adapters 鉴权声明、商店 releases 的实际集合含义和版本描述。保留有界 snapshot，只有无界集合引入分页并同步消费者。
- trusted_code_confirmed 保持基于风险原因的商店确认策略；补条件语义与升级权限扩张反例。
- 将 PluginTrustSummary 从 required level+label 改为稳定 level，删除服务端 trust.label 及相关 fixture 字段；Web 列表/详情从 level 查 locale。官方来源计算与安装信任判断仍在服务端，不能由文案推断。
- bridge/errors fixtures 增加真实 shape/语义校验，HTTP examples 纳入 CI；新增接口从 OpenAPI 枚举并明确豁免理由，不仅靠静态路径清单。孤儿 fixture 先追测试直接引用再处理。
- HTTP example 建立 operationId、request/response、status、media type 的机器映射，复用 validator，不从文件名猜 schema。已核实 `/api/adapters` 是契约缺 security，运行端已有鉴权；补声明并验证未登录/已登录，不改变为公开接口。
- 商店 releases 当前最多一项：采用 `current_release` 可空对象简化，先改契约与消费者；无界列表分页须同步 WS 全量快照和客户端状态策略。四个未被契约引用的 fixture 当前仍由目录枚举校验，应补引用或去重复，不能称为完全未验证。
- 实施时回写 endpoint→真实上限/排序/过滤/分页/WS 策略台账，至少包含治理名单、scheduler jobs、render templates、plugins、third-party accounts、plugin-store sources；名单与调度 SQL 已确认无 LIMIT，不能只增加 HTTP limit 而让 WS 继续无界推送。
- 真正共享的 schema fragment 用当前离线解析链支持的本地引用；不同边界的权限/确认要求不为复用而放宽。
- 验收：自测与 strict 分开通过；错误输入能使校验器失败；真实 HTTP/action/WS 响应与样例一致；x-fixtures 可解析。各协议版本与 API 契约版本按各自含义确定，同步目标分发中的生成物、SDK 与客户端。

### P07 错误目录、协议与版本生成链

- 来源：R-H1/H3/R1/W1/D5、V1-1.3。入口 `scripts/generate-runtime-schemas.mjs`、`plugins/runtime/runtime_model.go`、`sdk/go/protocol.go`、`sdk/go/types.go`、`sdk/vue/src/client.ts`。
- 当前生成器只复制嵌入 schema 并生成 bridge TS。先用代表性帧验证 optional/required、null/zero、oneOf、整数、未知字段和帧大小约束，再决定扩展生成器。
- 协议模型以契约为源，采用同一生成器向 Server 与 SDK 输出 wire 模型，两个输出都由 verify 约束，避免当前 Server 新增对 SDK 的运行依赖。若实验表明公共协议子包更简单，须先证明独立 module 可发布/解析再记录裁决；不能让 SDK import Server internal 或依赖主仓库 checkout。
- 生成错误目录及 code/key 映射，保留未知码/实际响应检查；生成协议版本等跨语言常量，不混同 artifact/bridge/protocol 的不同版本。
- 为每个生成器提供 generate/verify、确定性输出和删除/新增漂移检测；生成代码不能代替 JSON Schema 运行时验证。
- 保留 go:embed 所需 schema 副本，它们是受漂移检查的构建产物；跨语言脱敏用共享正反测试向量校准，不强造跨语言运行库。
- 验收：改一处契约能更新所有相关消费者；Go SDK 与 Vue SDK、示例和插件打包在独立 module/package 环境通过；清洁工作树重跑生成零漂移。

### P08 默认配置、类型化与热更新

- 来源：R-D3/H2/Q2/G3/H5，V1-1.1/1.3，B-3.5。入口 `config/default_document.go`、`canonical*.go`、`configruntime/apply.go`、Web `lib/config-form.ts`、发布 `package_runtime.py`。
- 默认值从 schema 的有效默认语义派生，保留跨字段校验与 secret 引用规则；类型化结构承担运行时配置，map 限于外部编辑文档/动态插件配置边界。
- 新配置采用“内嵌 schema 默认 + user.yaml”合成，删除可覆盖的 default.yaml 配置层；CLI init/normalize 直接创建/整理 user.yaml，整数稳定写出。删除文件前同步配置契约/说明、CLI、Launcher 缺配置初始化及发行根识别、release_tool 的 default-config 参数、release workflow、打包 REQUIRED_PATHS 与 smoke 全部消费者。
- 删除已废弃的两种 data retention 和 backup.mode 字段及 typed config/工作台/样例，不能只移出 required 继续保留死字段；缺省与显式 false、0、空集合必须区分。
- 规范化只在已证明的输入边界做一次，移除有证据的内部重复 TrimSpace；限流参数解析归 config，config/runtime 依赖窄接口。
- 为热更新定义一致性组、配置 revision、应用成功/失败与需重启结果。先构建/验证新策略后发布；跨服务外部动作按明确顺序和失败反馈执行，不把并发安全等同跨域事务。
- 验收：schema 默认→文件→加载→Web 编辑/保存往返语义一致；secret 不泄露；并发读者不看到同组新旧混合；应用失败、取消、restart_required 与持久文件一致，目标 race 通过。

### P09 数据初始化基线与历史代码清理

- 来源：R-X1/X2/R5、V1-1.6。入口 `storage/schema.sql`、`storage/migrations/`、`store_schema.go`、`config/migrate.go`、`auth/password_hash.go`、`configruntime/config_secret_refs.go`、`scripts/release/rehearse_data_migration.py`。
- 按目标 schema 建立单一数据库初始化基线，编号可从 1 开始；版本元数据仅表达当前结构，并与配置、备份和发布元数据一致。
- 删除无生产消费者的 `bilibili_source_*` 五表与历史升级代码、旧密码算法、secret 键搬迁和开发 air 别名；保留当前凭据校验/扫码需要的 Bilibili 代码。
- `server/sqlc.yaml:5` 当前读取 `internal/storage/schema.sql`，同时该文件用于新库初始化；保留它作为当前结构唯一来源。移除历史 migrations 及由其推导 CurrentSchemaVersion 的逻辑，同步当前版本读取、备份元数据和 `.github/workflows/release.yml:372` 的版本提取，避免生成与发布链依赖已删除文件。
- 删除历史配置、数据库、密码摘要的转换测试及旧数据迁移演练，替换为：空目录启动→初始化配置与数据库→写入配置/插件数据→本版备份→恢复到空目录→登录并核对数据。
- 验收：初始化后的表、索引、约束与目标 schema 一致，重复启动幂等；本版备份恢复及锁、失败事务回滚有效；sqlc generate/diff、Launcher 首次启动和发布演练同步通过。

### P10 文件路径与归档原语

- 来源：V1-1.2、R-R3。入口 `releaseupdate/archive.go`、`deps/archive.go`、`backup/archive.go`、`cli/restore.go`、`plugins/lifecycle/install.go`、`plugins/artifact/artifact.go`、`plugins/pluginstore/file.go`。
- 列出现有策略差异后，提取目的目录包含判断、规范路径、限额复制及归档遍历等真正共享原语。先从两个语义接近消费者证明，再扩展，避免大而全 archive 框架。
- 路径字符串检查、磁盘 symlink/reparse 检查、归档条目检查分开；更新/插件保留摘要、签名、根布局、Windows 保留名、大小写冲突和允许格式策略。
- 保留总展开量、单项/项数、压缩比、重复路径、取消、写入/关闭失败与进度语义；tar.xz 外部工具执行保持受控。
- 验收：合法包与恶意/损坏样例在每个消费者的结论一致或有明确策略差异；目标外零写入，失败不覆盖旧安装，资源释放完整。平台特定风险在对应 OS 运行。

### P11 资源清单与下载一致性

- 来源：V1-1.2/1.8。入口 `deps/metadata.go`、`deps/download.go`、`releaseupdate/download.go`、Launcher `environment.go`、`scripts/release/package_runtime.py`。
- `.deps/manifest.json` 以契约与共享正反例定义一致校验；当前 Server 解析 URL host/摘要字符，Launcher 只查前缀/长度，Python 另有实现。跨语言共享 schema/测试向量，不为共用函数让 Launcher 导入 Server internal。
- 下载共享受限复制、摘要、临时文件收尾与取消；发布端保留可信 URL/重定向、磁盘预检、idle/total timeout、artifact size 与身份校验。托管资源多个来源测速/重试保留。
- 验收：空 host、重复来源、非法摘要、越界 entrypoint、错误格式一致拒绝；下载中断/大小不符/摘要不符不发布正式缓存；重试和进度终态可观察。

### P12 插件 settings 业务统一

- 来源：B-3.2、V1-1.2。入口 `management/plugin_ui_settings.go`、`plugin_ui_secrets.go`、`plugins/actions/default_registry.go`、`wiring_runtime.go`、`plugins/pluginstore/config.go`。
- 新建真实复用的 settings 服务，HTTP 与 action 保留各自鉴权/解码，默认值、写入、secret 引用、命令刷新与事件通知由服务执行。
- 当前 HTTP 仅非空 changed_keys 通知，action 可通知空改动；底层 overwrite 对相同值也报告 changed_keys。按“有效值变化才报告并通知”的目标语义检查契约，必要时先明确更新，再实现两入口一致。
- 显式写成默认值仍可能新增持久化 override，必须存储；是否通知与是否持久化分别定义。已声明的凭据删除保持原语义，不隐式新增 config.delete；两入口各自权限规则保留，只统一授权通过后的业务副作用。
- 明确持久化成功但通知失败如何返回与重试；插件身份不能由任意请求覆盖，secret 不出现在 changed payload 或日志。
- 已接受的 config.changed 不因原 action 父事件结束而被取消；保留当前 `context.WithoutCancel` 的目的与 `TestConfigChangedDispatcherDetachesCallerCancellation`，同时受服务退出/队列上限管理。
- 验收：设置默认值、覆盖/合并、相同值、删除、凭据替换、刷新失败、并发写与无权限两入口一致；真实执行副作用，不只断言 wrapper 被调用。

### P13 插件状态、生命周期与装配

- 来源：R-Q1/Q3/G1/G5、V1-1.5。入口 `plugins/catalog`、`plugins/lifecycle`、`plugins/runtime`、`eventpipeline/dispatch`、`app/plugin_*`。
- 形成状态所有权表：声明/启用意图归 Catalog，进程会话归 Runtime，投递队列与 drain 归 Dispatcher，跨状态机操作归 Lifecycle；管理视图是只读投影。
- 逐个状态转移列触发、唯一写入者、取消与终态；解决投影失败静默吞错，避免 creating/starting 尚无进程时被误判停止。
- 窄接口替代 chatpolicy 对 lifecycle/runtime 的具体依赖；装配按依赖顺序构造。只清理为循环闭包补洞的 setter，允许有明确注册/解绑生命周期的回调。
- 验收：启停/崩溃/重载/替换/卸载/队列满/取消/关闭；预算内已接收事件完成，drain 超时或取消有明确结果，不重复或误投，新旧实例隔离；旧进程使用独立退出预算回收。多次 Close 与部分失败释放、race 和真实组合根用例通过。

### P14 适配器服务与管理事件分离

- 来源：B-3.1、R-Q5/S2。入口 `wsevents/protocol*.go`、`wsevents/ingress.go`、`app/event_wiring.go`、`configruntime`。
- 实例集合、reload、入站与领域 snapshot 进入 adapters 服务；WS Frame、初始快照与订阅生命周期进入 management/events。
- reload 停止错误归 adapters 所有，config/runtime 消费结果；去 Primary 和占位实例后，同步 HTTP/WS 状态与诊断的集合语义。
- 验收：多协议多实例、实例排序、停机 reload、需重启变更、订阅者退出、初始快照与实时更新一致；服务关闭不再有循环 owner。

### P15 中立出站与观测归属

- 来源：B-3.4、R-H4/C3、V1-2。入口 `eventpipeline/outbound`、`dispatch/outbound_action.go`、`qqofficial/outbound.go`、`onebot11`。
- 领域定义发送失败分类，由协议实现转换；限流器/Dispatcher 不构造或识别 `onebot11.Error`。
- OneBot/QQ 的日志、指标、重试与平台归属使用明确协议及实例字段；监听地址和浏览器访问地址转换按各自用途统一，覆盖 IPv6/通配监听/反代，不能把显示地址用于绑定。
- 指标标签保持有界；任意实例 ID 不直接加入高基数指标。实例细节可进入结构化日志/诊断，按实际监控需要设计聚合维度。
- 验收：两协议成功、限流、超时、未连接、权限与上游失败映射；实例状态/日志归属独立，指标按实际 OneBot/QQ 协议及有界结果标签聚合；只有先更新指标约定才扩维，现有队列硬上限不变弱。

### P16 模型、视图与辅助依赖收回

- 来源：B-3.3/3.5、R-R2/Q5。入口 `plugins/repository.go`、`summary_view.go`、`management/plugin_view_summary.go`、`health/health.go`、`runtimepaths/paths.go`、`builtinmenu/menu.go`。
- SQLite 插件实现回 catalog；health 中性模型不依赖 recovery；runtimepaths 不携带 ScanRoot 或执行发现清理；菜单与渲染身份通过真正需要的窄能力协作。
- 合并同构 Summary 模型；保留 `effective_names`、trigger、nil→空对象等实际 wire 投影，不用字段名相同推断可直接输出内部对象。
- 验收：目标依赖规则正反例有效，领域模型无数据库/执行栈传递依赖；HTTP 与 WS 视图字段/空值语义由真实响应契约测试覆盖。

### P17 Render 合包与资源回收

- 来源：B-3.7、R-Q5。入口 `render/service`、`render/repository`、actions 渲染身份模型。
- 合并唯一内部仓储，删重复 TemplateSource/Files/Summary 转换；保留 Service 对外入口、模板所有权和缓存 revision。
- 每个直接创建 Chromium runner 的 owner 负责 Close；测试用 t.Cleanup，临时 profile 与浏览器进程回收有可观察断言。
- 验收：模板列表/预览/插件同步、队列满、超时、取消、关闭、runner 失败；真实浏览器测试无进程/profile 残留，不能只用 mock runner 证明资源关闭。

### P18 运维、调度与 CLI 业务归属

- 来源：B-3.6、R-H5/Q5。入口 `management/scheduler_handlers.go`、`cli/restore.go`、`cli/cli.go`、`system`、`diagnostics`、`recovery`。
- scheduler 负责领域展示与时区计算；CLI 负责参数/输出/退出码，恢复与管理员重置复用领域业务；共享在线/离线诊断但不合并不同锁和服务运行前提。
- 运维超时/路径有清晰归属，避免全部塞入全局常量。错误向上返回，输出 writer 失败不能假成功。
- 验收：真实临时数据库恢复/重置、锁冲突、停服要求、无效备份、退出码与输出格式；调度跨时区/停用/插件移除状态正确。

### P19 后端目录分组迁移

- 来源：B-4/7；采用 §2.1 映射。先做职责变化，再用 git mv 分组迁移；每批同步 import、测试定位、生成器输入、结构规则和文档。
- 顺序建议：platform 中已解耦包→bot→plugins→config/runtime→operations→testenv；迁移顺序可按实际依赖调整，每次记录新旧路径。
- 保留 releaseupdate 链接器注入与 SQL 生成目录；扫描 Windows/Linux/macOS build tags、脚本包名参数、测试 fixture 编译、embed 与 go:generate。
- 验收：每批相关测试与 architecture/结构检查通过；三平台相关构建无缺失；旧生产 import 消失，生成无漂移；不留 re-export 转发空壳，不以包数量验收。

### P20 测试体系替代与离线回归

- 来源：R-T1–T6、V1-1.5。入口 `server/tests/services/harness_test.go`、`tests/testutil`、包内测试、`web/tests`、`launcher/tests`、`scripts/tests`。
- 建立“旧用例→风险→保留/移动/替代/删除→新证据”表。包内业务回包内；装配/跨域走真实组合根+临时 SQLite，不建更大的万能 harness。
- runtime registry/catalog 替身只表达协作者行为，能用轻量真实实现则复用；不为测试新增运行期全局 setter。TestMain Chdir 按隔离路径替代，确认并行测试不共享目录。
- 无生成器的 rapid 包装逐例识别；catalog 已有生成器不删除。固定 sleep 用事件/预算控制替代，保留原回归的时序条件与最终超时上限。
- health 编码 fixture 继续验证 wire shape，另增由真实 system/readiness 依赖降级产生的报告测试；序列化覆盖与领域行为覆盖分别保留，不能互相代替。
- 补网易云加密/扫码离线向量、QQ 官方录制/协议替身、Windows 安装 rename retry；真实在线测试 skip 不算已验证，样本不含真实凭据。
- 启动脚本、构建失败恢复和错误路径等有真实行为的测试保留。SQL/import 检查可合并入口，但迁移所有独有断言后才删除旧脚本或登记；不通过删检查获得绿色 CI。
- 验收：删除清单每项能映射保留的风险保障或证明无风险；失败路径真正执行，随机用例确有输入变化，临时资源全部关闭；核心并发 race、平台测试实际运行。

### P21 死代码、重复 helper 与集成精简

- 来源：R-X1/X3/G3/G4/H6/R3/R6、V1-1.4。按原评审附录重搜实际调用，不把 64/38 等旧计数当删除工单。
- 清理无上下文的旧 wrapper、未调用 SQL 查询、无效分支/参数/导出；只被测试引用的构造或 seam 评估其可替代性，不全部判死。
- Bilibili 仅删订阅迁出后无生产用途部分，保留扫码、资料、凭据验证与指纹所需链路；缓存失效 switch 合并前逐类事件核对失效字段。
- 等价工具采用标准库，深复制保持嵌套隔离，严格/容错 semver 不混用。示例保留最小入门、管理页、关键能力与测试 fixture 的教学/验证职责，不按相似外观砍成两个。
- HTTP transport 复用列为有条件实验：记录延时/连接数基线，证明域名授权、DNS 变化、私网权限与取消隔离后才实施；无收益或无安全等价方案记录“无需实施”。
- 验收：受影响编译/调用方/打包与负例通过；删 SQL 后生成链无漂移；新鲜独立示例可构建；没有共用可变数据导致回归。

### P22 Web 状态、错误、请求与真实 E2E

- 来源：R-D2/R4/T3/T5/W2/Q4/Q7、V1 的 Web 条目。入口 `lib/management-summary.ts`、`lib/http.ts`、`stores/socket-router.ts`、`main.ts`、`views/`、`tests/production/management.real.spec.ts`。
- 仪表盘/重试面板按稳定 code/details 分支；删除未调用导出；locale 归并错误与 UI 文案，动态 key 有覆盖校验，允许测试、数据样例、注释等合理中文。
- trust.level 覆盖官方、开发、第三方、未验证来源四类与未知展示回退；同一 code/details 下替换中英文 message 不改变操作建议或资源选择。
- 请求与下载共用鉴权/超时/取消/错误内核，保留 acceptStatuses、204、JSON/blob、文件名、CSRF 轮换和代理 503 差异。修请求前已取消信号仍发请求、成功后 abort 监听未解除；如用 AbortSignal.timeout，同步 TimeoutError 分类和 timeoutMs<=0 语义。
- 刷新调度共用防抖/进行中/尾随执行能力，先明确 status 当前丢弃进行中刷新而其他领域排队的策略；登出和卸载取消旧刷新，最后一次变化最终可见。
- 合并日志两页与黑白名单的真实共有业务，保留实时/历史与启用语义差异；规范时间戳单位与无效值，统一状态 tone、响应式 token；清理 main 重复回调注册与 Web bearer 残留。
- 日志复用现有筛选/行/详情控制器，保留实时跟随/未读、历史虚拟列表/分页、keep-alive 和时区；白名单保留启用开关、空名单确认与 adapter scope。max-width 639/min-width 640 可能是合法互补，不把 1px 差异一律判错。
- 删除 `clearStoredBearerToken` 等仅服务历史 localStorage 令牌的分支及相应用例；Web 统一使用当前 cookie 会话，不读取或发送 localStorage bearer。补缺失 `errors.common.saveFailed` 等实际缺键。
- PluginManagementUIHost 使用生成 payload 类型但保留 origin/source/version/nonce/MessageChannel 校验；配置只生成 default/min/max/enum/apply 等机器事实，label、布局与控件仍属产品层。
- 大视图按数据生命周期和用户任务拆分；App 与基础组件层保留，孤立资源/组件经引用和构建验证再删。可选性能改动需前后测量，不承诺靠拆文件提速。
- 扩展现有 real-server 覆盖配置/鉴权/权限/日志/插件设置/调度等；mock 只保留需要控制的网络/iframe/故障场景，列迁移对照后移除 JS 业务副本。
- 验收：重连、深链、401/403、预取消不发请求、用户取消/超时不误报宕机、监听释放、尾随刷新、二进制下载、日志范围与黑白名单行为；插件切换旧响应不回写、iframe 握手单一；真实 E2E 按 spec 隔离临时目录/SQLite/端口并回收进程。无障碍/焦点/减少动画/响应式回归不能由 typecheck 代替。

### P23 Launcher 类型边界与失败可见性

- 来源：R-G2/C2/X4/Q7、V1-1.1/1.8。入口 `launcher/internal/desktop/management.go`、`settings.go`、`service.go`、`src/shared/server-payload-validation.ts`、Go models 与 generated bindings。
- Go HTTP 边界使用正式响应模型，保留 required、enum、范围和非法状态校验；公开的 Wails service model 是生成源。迁移后删 Renderer 重复校验，验证未知展示值与安全状态的不同策略。
- 状态文件损坏返回可诊断错误并保留原文件，不静默覆盖默认；Shutdown 聚合真实错误。集中有业务意义的超时/轮询参数，不用同一常量替代不同时间预算。
- Shutdown 修到 Coordinator 内部：区分优雅退出失败后强杀成功、最终残留、外部进程和幂等关闭；Windows taskkill 按进程状态/退出语义判断，移除依赖英文 not found 和误植 ESRCH 的分支，不把一次可恢复尝试错误直接等同最终失败。
- 删除已无后端来源的 bilibili_source 标签；裁剪未使用 OpenAPI 生成物要先确认 Web API 与本机 Wails DTO 边界及全部引用。
- 需要保留 OpenAPI TS 子集时，由生成器递归保留 $ref 闭包、确定输出并检查漂移；不得手摘类型。该类型文件不进入运行时 bundle，裁剪收益记为维护成本，不宣称运行性能改善。
- AppShell/CSS 仅按职责整理，保留启动、进程、托盘、单实例、焦点和窗口行为；不在本计划新增视觉风格。
- 验收：空字段/非法值/未知字段策略、损坏文件、初始失败、服务退出/超时、generated bindings drift；Renderer E2E 与真实 Wails Windows 平台检查分别记录，Mac/Linux 相关构建也覆盖。

### P24 工具链、开发与发布脚本收敛

- 来源：R-H3/Q6/X5、V1-1.3/1.8。入口 `.tool-versions`、各 go.mod/package.json、Makefile、`check-toolchain.py`、`start-dev*.mjs`、Launcher 构建脚本、`scripts/release/`。
- 各工具使用现有权威版本源；package.json/go.mod 等生态必须字段保留并做一致性检查。doctor 按任务选工具，CI/发布冻结版本错误仍失败，不把必要版本约束全部降为 warning。
- 抽取 Go executable 解析、平台命名、Launcher control header 等实际等价逻辑；POSIX 支持显式 Node 路径，macOS 浏览器识别覆盖 App Bundle，跨平台启动失败信息可行动。
- 发布 artifact 矩阵/必需文件列表合一，保留各平台 archive 类型与 smoke 对真实内容的独立核对。`smoke_release.py` 当前 entries 为集合 update，不是“清单被覆盖”。
- `check-agent-docs.mjs` 清理不可达正则及过宽枚举，保留桥接导入/引用/脱敏等独有规则与负例。历史日志工具、统计图脚本按实际调用/使用价值决定保留或归档。
- 验收：脚本有效/无效输入、路径空格、工具缺失/版本错误、子进程失败、退出码与输出产物；启动 wrapper 两平台回归，打包矩阵内容和缓存失效正确。

### P25 文档、文案、资产与说明归属

- 来源：R-S1–S5/W3/C2/H6/X4、V1-1.6/1.8。即时事实修正与相关实现同行，不等 P26 才修 README/CLI/插件指南。
- 修 JSONL v3、多适配器/QQ 官方、语言中立 artifact、HTTPS 目录+摘要、CLI backup/cleanup 停服要求；正式能力来源指向契约，用户文档保留完整操作条件与失败处理。
- 产品目标归项目章程/PRODUCT 分工，设计职责保留目录索引与应用约束；架构/基线记录必要理由与真实生成入口，避免复制易漂移的字段列表和精确流程常量。
- 历史评审和事故记录明确时间/版本并从当前操作入口分离；本计划完成后按 docs 规则整理 CHANGELOG。根 design-qa 的本机临时路径归档或删除。
- 图片原始素材先保留授权/提示词来源，再核实运行引用与生成职责；字体按实际字形/授权/离线渲染验收，不能仅看 MB 删除。leaderboard preview.html 是否孤儿以 runtime 和设计工具引用共同确认。
- 外部 Impeccable 内部不改；如决定移出 vendored，先交付可复现安装与上下文说明，再在独立任务执行。Python 忽略项按当前工具链保留必要部分。
- 验收：读者能从新装、配置、插件开发到备份恢复完成操作；链接/指令检查通过，正式术语一致；资源无缺字/缺图；无本机绝对路径与真实凭据泄露。

### P26 全新分发验收与交付准备

- 汇总所有工作包结果与证据，清理失效任务；未完成项不能被“全面优化完成”掩盖。
- 干净 checkout 完成契约/生成/各模块测试构建；从空运行目录执行初始化、真实多适配器、插件安装设置重载卸载、配置热更新、模板渲染、浏览器退出、日志与调度、Launcher 启停、本版备份恢复全流程。
- 发布四种正式 artifact，检查归档内容、license/notices、资源清单、metadata/签名与 smoke。Windows 自动更新保留正式 Authenticode/Ed25519 门槛，缺证书时仍 guided，不宣称已通过签名安装。
- 分发说明写明安装前提、首次初始化、目标配置与协议、插件构建接入和本版备份恢复步骤；具体发布版本号在发布前由维护者指定。
- 验收：剩余缺口有 owner/原因/下一步；全新安装、本版恢复和安装事务失败回滚真实通过；任务完成记录可追溯。尚未分发时交付经过同等验收的构建产物与说明。

## 5. 三份评审覆盖索引

### 5.1 R 全量发现映射

“修正采纳”表示接受问题方向但不执行原文的全部方案；详细裁决见 §2/§4。

| 原编号 | 判定 | 工作包 |
| --- | --- | --- |
| D1 | 确认 | P03、P14、P15 |
| D2 | 活跃路径问题确认；protocolSummary 本身为死链 | P05、P22 |
| D3 | 部分成立，修默认源与格式，不宣称 YAML 必然无效 | P08 |
| D4 | 确认并隔离复现 | P02 |
| D5 | 确认漏检，修正测试能力描述 | P05、P07 |
| G1 | 修正采纳，保留局部失败清理/幂等 | P02、P04、P13 |
| G2 | 修正采纳，保留边界运行时校验 | P23 |
| G3 | 边界规范化逐项确认 | P08、P21 |
| G4 | 候选清理，不批量删除默认/资源保护 | P04、P21 |
| G5 | 确认 | P04、P13 |
| T1 | 确认装配重复，先替代测试 | P20 |
| T2 | 部分成立，不把测试 seam 一律判死 | P20、P21 |
| T3 | 部分成立，拒绝整文件删除 | P20、P22、P23 |
| T4 | 拒绝按测试长度删除，逐例简化 | P20、P24 |
| T5 | 确认重复，已有 real-server 基础 | P22 |
| T6 | 部分重叠，独有 SQL/退出检查仍需接替 | P01、P20 |
| H1 | 确认 | P05、P07 |
| H2 | 确认副本，删除有前置 | P08 |
| H3 | 确认，生态版本字段保留一致性约束 | P07、P24 |
| H4 | 修正采纳，区分监听/显示/连接用途 | P03、P15 |
| H5 | 部分成立，按责任归属参数 | P08、P18、P24 |
| H6 | 候选，需分清有效版本常量与历史数据 | P21、P25 |
| Q1 | 确认耦合，不实行零 setter | P02、P13、P16 |
| Q2 | 确认，动态文档边界仍可用 map | P08 |
| Q3 | 状态职责不同，拒绝简单删三份 | P13 |
| Q4 | 确认维护热点，按职责拆 | P13、P22、P23 |
| Q5 | 修正采纳 B 的边界方案 | P14–P19 |
| Q6 | 部分成立，不放宽冻结版本门槛 | P24 |
| Q7 | 拒绝组件二选一/按文件大小合并 | P22、P23 |
| S1 | 确认 | P25 |
| S2 | 确认 | P03、P14、P25 |
| S3 | 历史记录归档/去当前入口，不默认无价值 | P25 |
| S4 | 部分成立，Python 忽略规则仍有用途 | P25 |
| S5 | 收敛事实归属，保留可操作说明 | P25 |
| C1 | 确认，字段删改先改契约 | P05、P06、P22 |
| C2 | 确认候选，核查全部诊断消费者 | P23 |
| C3 | 确认，固定模板+结构化字段 | P05、P15 |
| R1 | 确认手写协议副本，先证明生成完整性 | P07 |
| R2 | 部分同构，保留实际 wire 投影 | P16 |
| R3 | 部分等价，保留深复制/严格解析 | P10、P21 |
| R4 | 确认，保留下载与请求差异 | P22 |
| R5 | 单一目标 schema 与初始化基线，删除历史迁移链 | P09 |
| R6 | 教学/验证角色盘点后精简 | P06、P21 |
| W1 | 确认重复，默认错误文案与 UI 指引可分工 | P05、P07、P22 |
| W2 | 收敛用户文案，不禁止所有中文 | P05、P22、P23、P25 |
| W3 | 确认归属漂移 | P25 |
| X1 | 确认遗留表，Bilibili 代码逐个消费者核实 | P09、P21 |
| X2 | 全新分发前提下删除历史分支 | P09、P24 |
| X3 | 附录作为候选，编译与跨平台引用再确认 | P21 |
| X4 | 修正采纳，外部 skill/字形/生成来源受约束 | P23、P25 |
| X5 | 确认输入/门禁缺口，保留有用自检 | P01、P06、P24、P26 |

### 5.2 V1 补充条目映射

下表覆盖 V1 超出 R 的具体条目；列“专项验证”者保留为有明确入口的调查任务，不当作已复现缺陷。

| V1 章节/条目组 | 处理与覆盖 |
| --- | --- |
| 1.1 安装后置/回滚、HTTP nil、可选权限依赖 | P04/P13；构造错误与业务禁用区分 |
| 1.1 文案分支、Launcher 状态/Shutdown、配置发布、OneBot 测试全局 | P05/P23/P08/P20；分别结构化、保留损坏源、定义一致性组、构造注入 |
| 1.2 ZIP/pathInside/下载/解压 | P10/P11；策略矩阵先行 |
| 1.2 请求体、凭据校验/会话常量、HTTP 错误包装 | P04/P05/P08；凭据归 auth，复用校验且保留输入边界 |
| 1.2 日志两页、黑白名单、debounce、时间戳、状态 tone | P22；测试实时/历史与单位差异 |
| 1.2 平台中文元数据、OneBot 缓存失效 | P21/P25；确认字段消费者及各类事件，不能只合 switch |
| 1.3 安装目录/临时前缀/扫描根、发布命名 | P16/P24；领域语义归对应 owner |
| 1.3 协议版本/默认值/基础设施/工具链副本 | P07/P08/P24；不同版本域不合并 |
| 1.3 UA、断点、WS path/event、配置表单 | UA 保留 provider 指纹，P21 专项；其余 P07/P08/P22 |
| 1.4 management 大包、视图、死类型/参数/常量 | P16/P18/P21；不按 endpoint 再碎拆 |
| 1.4 SDK/Launcher/release/start-dev 恒真分支 | P21/P24；按当前调用、平台和失败语义专项验证 |
| 1.4 main 双注册、blacklist 残留、Web bearer/死组件 | P13/P21/P22；确认消费者后清理 |
| 1.4 大视图/lifecycle/menu/actions、HTTP transport | P13/P22；transport 为 P21 有条件性能实验 |
| 1.5 PR race、生产副本替身、helper/Chdir | P01/P20；替代保障先落地 |
| 1.5 property、sleep、文案断言、health fixture 自证 | P20；catalog 误报；领域 readiness 增真实依赖降级覆盖，编码 fixture 继续验证 wire shape，契约承载的文案断言可保留 |
| 1.5 网易云/QQ/Windows 覆盖 | P20/P26；录制/离线与实际平台分别记账 |
| 1.6 CLI/README/规划/插件文档/商店/过程记录/多协议 | P25；按契约和当前代码修事实 |
| 1.6 Web locale/死键/缺 key/按翻译分支 | P05/P22；动态 key 和未知码策略纳入验收 |
| 1.7 resource_missing/invalid_request/internal_error | P05；按状态和触发条件对齐 |
| 1.7 未登记/死 code、examples、fixture-ready | P05/P06；区分诊断 ID 与正式 error code |
| 1.7 shared shape/trusted_code_confirmed | P06；保留商店条件确认，禁止机械 const 化 |
| 1.7 列表分页/releases/required/security/info.version/孤儿 fixture | P06/P08；分页同步 WS，废弃字段整链删除，鉴权缺声明，孤儿仍有目录验证，info.version 含义先明确 |
| 1.8 deps manifest/Go 可执行/发布清单/超时 | P11/P23/P24；跨语言用契约及测试向量收敛 |
| 1.8 semver、模板 preview、macOS/Node 入口 | P21/P25/P24；保留严格发布边界，模板工具引用专项确认 |
| 2/3 后端合包/目录/批次、1.5 等待时长清零 | 由 §2.1 和 §3 替代；不合并真实独立生命周期，不按时长清零 |

### 5.3 B 映射

| B 章节 | 工作包与裁决 |
| --- | --- |
| 3.1 适配器运行服务 | P03、P14 |
| 3.2 settings/secrets | P12；补充无变化与通知失败语义 |
| 3.3 chatpolicy/menu 具体依赖 | P13、P16 |
| 3.4 出站中立边界 | P15 |
| 3.5 模型/health/runtimepaths/config 依赖 | P08、P16 |
| 3.6 scheduler/CLI 业务 | P18 |
| 3.7 render | P17 |
| 3.8 门禁/SQL 例外输入 | P01、P20 |
| 4/5/7 目录、依赖、生命周期、路径链 | P19 采用边界，P02/P26 验证；不以一级目录数验收 |
| 4.2 暂不扩大 provider/module/sqlc/release 路径 | 保持；数据初始化基线由独立 P09 实施，不夹在路径搬迁 |

## 6. 验证矩阵与证据格式

命令从当前工程脚本取用；执行时先核对 package.json、go.mod 与 CI。下面是现有入口，不表示本轮已全部运行。Windows 遵循 baseline 优先使用 gbash；Launcher Go module 使用 `GOWORK=off`。

| 验证面 | 现有命令/执行位置 | 必须观察的结果 |
| --- | --- | --- |
| 分类/契约 | 根目录 `python scripts/ci/detect_changes.py --self-test`；`python scripts/ci/validate_contracts.py --self-test`；`python scripts/ci/validate_contracts.py --mode strict` | 三条独立成功；负例确实被拒绝 |
| 嵌入 schema | 根目录 `node scripts/generate-runtime-schemas.mjs --verify` | 来源与所有输出一致；新增生成链加入相同 verify 入口 |
| 架构 | 根目录 `python scripts/check-server-structure.py`；server 内 `go test ./tests/architecture -count=1` | 正反规则实际扫描新目录，不因路径为空通过 |
| Go 行为 | server 内 `go test <相关包及直接调用方> -count=1`，跨域批次再 `go test ./...` | 目标测试执行，无意外 skip、泄漏或旧缓存结论 |
| 并发 | 先 `go env GOOS CGO_ENABLED CC` 检查环境；相关包 `go test -race ...` | 支持环境实际通过；Windows/C 工具缺失记录缺口，不伪造结果 |
| SQL | server 内 `sqlc generate` 后 `sqlc diff` | 输入版本/实际数据库表与生成代码一致 |
| Server/updater | server 内按现有 CI 构建两个 cmd 入口；风险对应平台矩阵 | 输出文件确实存在且对应本次源码，包搬移未破坏 ldflags/embed |
| Web | web 内 `corepack pnpm typecheck`、`corepack pnpm test`、`corepack pnpm build`、`corepack pnpm test:e2e:production`；相关生成用 `corepack pnpm generate:types` | 真实服务数据与浏览器交互通过；迁移期保留 `test:e2e` 中尚未替代的故障注入场景 |
| Launcher | launcher 内 `corepack pnpm typecheck`、`corepack pnpm test`、`corepack pnpm build`、`corepack pnpm generate:wails`、`corepack pnpm test:e2e`；定向 Go 用 `node scripts/run-go.mjs test:platform ./internal/desktop` | 脚本已设 GOWORK=off；区分 Renderer 模拟桥与真实 Wails；三平台构建及必要原生验证 |
| SDK/示例 | sdk/go 与受影响 Go 示例各自 module 内 `GOWORK=off go test ./...`（gbash）；sdk/vue 与管理页 typecheck/test/build；`node --test scripts/tests/plugin-dev-workspace.test.mjs` | 独立 module 可消费、artifact 校验与启动握手真实成功 |
| 脚本 | 对应 `node --test`、Python unittest/现有 CI 自测 | 有效/无效/缺依赖/子进程错误均有正确结果与退出码 |
| 发布 | 调整 release/self-host-smoke 为全新分发验收及 P09 初始化演练 | 四种 artifact 内容、签名、首次初始化、本版备份恢复、安装事务回滚与长时资源状态 |
| 文档/范围 | 根目录 `python scripts/check-doc-links.py`、`git diff --check`；指令改动另跑 `node scripts/check-agent-docs.mjs` | 无失效引用、格式/敏感值问题；仅显式任务文件进入提交 |

### 6.1 规划阶段核实结果（实施前）

| 检查 | 结果 | 解释 |
| --- | --- | --- |
| 三份评审 Git 范围 | 通过 | 提交仅三份评审；提交前修正后端评审 42 处相对链接，不改评审结论 |
| 文档链接 | 通过（最终 137 个 Markdown） | 包含 v2 与 README 新入口 |
| 计划完整性与文件检查 | 通过 | R 的 51 个编号全部映射；26 个任务定义与台账一致；UTF-8、空白与 git diff 检查通过 |
| detect_changes 自测 | **失败，已复现** | `design-qa.md, go.work.sum` 未分类，纳入 P01 |
| SQL 例外 JSON 单路径分类 | **漏触发，已复现** | server=false、ci=false、docs_only=true，纳入 P01 |
| strict contracts | 通过 | 不代表 errors/bridge/examples 的所有语义均已覆盖 |
| validator 自测 | 通过 | 单独执行，属于 CLI fixture 语义自测 |
| runtime schema verify | 通过 | 当前嵌入 schema/bridge TS 链一致，不等于 Go 帧已生成 |
| HTTP examples 单独按当前 OpenAPI 校验 | **8 份全部失败** | config-update.response；blacklist/whitelist 的 response、entry.request、entry.response；recovery-confirm.task-detail，纳入 P06 |
| bridge 20 份正反样例直接校验 | 预期结果全部匹配 | 样例本身可用，但现 CI 未调用该验证，P06 接入并补负例自测 |
| fixture 引用枚举 | 4 份缺契约引用 | unknown-core-version、log-detail outbound-onebot11、logs-list outbound-message、logs-appended outbound-onebot11；现有目录验证仍执行 |
| Server 结构脚本 | 通过，0 warning | 当前规则无法证明完整领域边界 |
| 选定后端测试 | 通过 | server 内 `go test ./internal/configruntime ./internal/plugins/catalog ./internal/eventpipeline/dispatch ./internal/semver -count=1` |
| Run 监督器隔离复现 | 复现 D4 | 原监督器实现摘录到临时目录验证，不修改生产文件；不替代未来集成回归 |

规划核实阶段没有运行全量 Server/SDK/Web/Launcher 测试、race、真实外部协议连接、浏览器 E2E、原生桌面、发布打包或签名安装。后续实施结果见 §7；原评审的重复字符串/零引用数量未全量重算，不作为当前质量基线。

## 7. 回写与完成标准

每个工作包完成时更新 §3 台账，并在下表追加一行；阻塞时写具体缺失条件和可独立推进项。子提交可以先记录，整包的勾选只能在全部验收完成后进行。

| 日期 | ID / 子任务 | 状态变化 | 提交 | 实际改动/裁决 | 验证命令、结果及证据位置 | 未验证项 / 下一步 |
| --- | --- | --- | --- | --- | --- | --- |
| 2026-09-10 | 输入评审归档 | 完成 | `2c358c93` | 三份评审，修正后端文档相对链接 | 文档链接通过；提交路径仅三份 | 无实现改动 |
| 2026-09-10 | v2 核实与编制 | 完成 | `77d7a482` | 采用尚未分发或全新分发前提；统一方案、台账与验收 | 见 §6.1 | 实施状态见 §3 |
| 2026-09-10 | P01 分类、结构与 PR 门禁 | 待验收 | `235b2870` | 补 go.work.sum/SQL 登记分类；重命名两端均触发；结构规则覆盖子包；PR 增 lint、核心 race 和 Windows 必需 job | Python scripts/tests：19 项，16 通过、3 项 POSIX 测试按平台跳过；分类自测/结构/architecture 通过；核心 lint 0 issues；Windows lifecycle/releaseupdate/filelock 通过；CI YAML/needs 与文档链接通过 | Windows 默认 CGO_ENABLED=0；已定位独立 gcc，race 另行验证；托管 CI 尚未运行 |
| 2026-09-10 | P04.HTTP 管理请求体边界 | 完成 | `e2b1af8b` | 13 入口统一限额/单对象解码；development 16 KiB，其余 1 MiB；可选空体与动态 JSON 保持正式语义 | management/httpapi 包测试；services 的 PluginSettings/Secrets/Management 定向测试；13×13 输入回归；strict 通过；Web/Launcher OpenAPI 临时生成与原文件哈希一致 | P04 安装后置和回滚传播尚未实施 |
| 2026-09-10 | P01 Linux lint 与核心 race 补验 | 待验收 | `f0c76de8` | 修复非 Windows Authenticode 错误串触发的 ST1005 | `GOOS=linux GOARCH=amd64 CGO_ENABLED=0` 下 CI 同参数 lint 0 issues；Windows 用独立 gcc、CGO_ENABLED=1 跑 CI 核心包 race 全部通过 | 托管 Linux/Windows workflow 未实际运行 |
| 2026-09-10 | P03.ROUTING 动作与元数据实例隔离 | 完成 | 本提交 | 协议/SDK 明确实例选择；复用 Router，按父事件约束选路；provider 前剥离 selector；元数据与缓存按实例隔离 | app/actions/runtime/onebot11/outbound/architecture 包及 services OneBot/Provider 测试通过；SDK GOWORK=off 全包通过；strict、生成/verify、文档链接通过；双实例 HTTP 与 LLOneBot/NapCat provider 门禁回归通过；相关核心 race 通过 | 单实例 HTTP/WS/system 摘要及占位 Shell 仍待 P03 后续子任务 |

P01 依赖基线：`go list` 枚举 Server 66 个包，其中 internal 59 个；app/management 直接依赖 internal 包分别为 45/27。生成链仍为 runtime-schema/bridge、OpenAPI/WS、Wails bindings、sqlc；P01 未改变生成输入。上述数量仅记录当前结构，不作为整改目标。

单项回写模板：

```text
ID / 子任务：
执行前 HEAD / 工作区范围：
原证据是否仍成立：
契约决定与受影响消费者：
完成内容 / 删除或替代清单：
提交：
验证命令 / 环境 / pass、fail、skip：
真实产物或行为证据：
未验证限制 / 阻塞 / 下一步：
状态：
```

项目完成条件：P01–P26 均完成或有证据判为无需实施；有效缺陷有回归保障；真实输入输出与契约一致；空目录初始化及本版备份恢复可用；资源无已知泄漏；SDK/客户端/全新分发产物闭环；文档与台账同步。不得以行数下降、测试减少或静态检查全绿替代这些结果。

当前无需新的实施取舍。具体发布版本号、可用正式签名环境及独立插件构建接入安排在 P26 前落实；性能试验等技术选择按各任务证据决定，只有出现新的产品语义或资源约束才再次询问维护者。
