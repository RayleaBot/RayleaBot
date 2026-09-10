# 仓库评审与重构计划

本文记录 RayleaBot 全仓评审发现，以及在允许兼容性破坏的前提下清理仓库、重整后端包结构的方案和执行顺序。文件位置与行号以评审基线 `main@809fec3a`（2026-09-10）为准；执行时以当时的代码为准，完成的阶段在“分阶段计划”中更新状态。

## 结论

代码能通过 errcheck、staticcheck、govet、unused、ineffassign 等核心静态检查，主要问题不在实现错误，而在冗余：

- 同一事实在契约、Go 字面量、测试替身、Web 文案和文档中各存一份，并且已经出现不一致。
- 为历史数据、为测试、为可能为 nil 的协作者预留的分支散落在业务代码中。
- `server/internal` 按实现过程增长到 59 个包，组合根直接依赖其中 45 个，依赖方向靠构造后调用的 setter 反转。

处理顺序为“先删除、再收敛到契约、后重整结构”：删除兼容层与死代码可以缩小后续改动面；错误码、插件协议帧和配置默认值改由契约生成后，重整包结构时不必再同步多份副本。开始重整前先修复“先修的缺陷”，其中 D1 影响已有的多实例部署。

## 度量

| 项目 | 数值 |
| --- | --- |
| 服务端手写 Go 代码 / 测试 | 67,980 / 57,316 行，另有 sqlc 生成代码 1,665 行 |
| `server/internal` 包 | 59 个；`app` 依赖 45 个，`management` 依赖 27 个 |
| golangci-lint 核心检查 | 0 个问题，只在 nightly 运行 |
| goconst 重复字符串字面量 | 428 处 |
| 认知复杂度大于 30 的函数 | 28 个，最高 108 |
| 零引用 / 只被测试引用的 Go 导出符号 | 64 / 38 个 |
| 未被调用的 sqlc 查询 | 2 个 |
| 非测试代码中含中文的行 | server 1,195；web 在 locales 之外 426；launcher 563 |
| Web 源码 / 测试 | 36,338 / 18,932 行；`App*` 封装组件 37 个，shadcn-vue 源码 96 个文件 |
| 工具链版本号副本 | 12 个文件中 109 处 |

统计方法：golangci-lint 2.12.2 运行核心检查以及 goconst、gocognit；基于 `go/ast` 的脚本统计导出符号引用与内部包依赖，方法调用按名称匹配；TypeScript 导出引用与中文分布用正则统计。零引用清单偏保守，删除前以编译结果为准。

## 发现索引

- 编号前缀：D 缺陷，G 过度防御编程，T 过度或无用测试，H 硬编码，Q 低质量代码，S 过时文档，C 过时文案，R 重复代码，W 重复文案，X 死代码与仓库卫生。
- 严重度：缺陷指行为错误或潜在错误；高指持续制造重复或阻碍重构；中指收益明确的清理；低指细节整理。
- 阶段对应“分阶段计划”中的编号。

| 编号 | 严重度 | 发现 | 阶段 |
| --- | --- | --- | --- |
| D1 | 缺陷 | 多 OneBot 实例时插件动作与事件补全绑定首个实例 | 0、3 |
| D2 | 缺陷 | 仪表盘提示靠匹配服务端文案生成 | 0 |
| D3 | 缺陷 | `config/default.yaml` 被写成科学计数法 | 2 |
| D4 | 潜在缺陷 | Run 监督器共用 `sync.Once` 上报错误 | 0 |
| D5 | 缺陷 | 错误码契约测试只识别三个函数名 | 2 |
| G1 | 高 | 必选依赖处处判 nil，并靠规则防 typed nil | 3 |
| G2 | 高 | Launcher 手写载荷校验并拒绝未知字段 | 4 |
| G3 | 中 | 已校验的值被反复 `TrimSpace` | 3 |
| G4 | 中 | 为不会发生的情况写的分支 | 1 |
| G5 | 中 | 失败被静默吞掉或用类型断言兜底 | 3 |
| T1 | 高 | services 测试手工复刻组合根 | 1 |
| T2 | 高 | 为测试而存在的生产 API | 1 |
| T3 | 中 | 绑定实现细节或构建配置的前端测试 | 1 |
| T4 | 中 | 工具脚本的测试比脚本本身更重 | 1 |
| T5 | 中 | E2E mock 后端重新实现服务端业务 | 4 |
| T6 | 中 | 同一类约束有两套检查 | 1 |
| H1 | 高 | 错误码是散落的字符串字面量 | 2 |
| H2 | 高 | 配置默认值有四份 | 2 |
| H3 | 中 | 工具链版本号在 12 个文件中出现 109 次 | 2 |
| H4 | 中 | 监听地址归一化有四份且规则不一致 | 3 |
| H5 | 中 | 环境变量、路径与超时埋在组合根 | 3 |
| H6 | 低 | 过期版本号与个人环境痕迹 | 1 |
| Q1 | 高 | 组合根靠 setter 与闭包拼装循环依赖 | 3 |
| Q2 | 高 | 配置在类型化结构与 map 之间反复往返 | 3 |
| Q3 | 高 | 插件运行态存了三份 | 3 |
| Q4 | 中 | 超大函数与文件 | 3、4 |
| Q5 | 中 | 薄包、名不副实的包与倒挂依赖 | 3 |
| Q6 | 中 | 低质量或一次性的脚本 | 1、2 |
| Q7 | 低 | 前端结构碎片化 | 4 |
| S1 | 高 | 项目章程与 README 描述旧版本系统 | 5 |
| S2 | 高 | 架构文档没有跟上多适配器 | 3、5 |
| S3 | 中 | 一次性过程记录混在正式文档里 | 1 |
| S4 | 中 | 失效路径、重复页与复述 CI 的文档 | 1 |
| S5 | 中 | 文档复制契约与实现 | 5 |
| C1 | 中 | 服务端下发中文标签，Web 与 fixtures 各存一份 | 2 |
| C2 | 低 | Launcher 保留已不存在的诊断项文案 | 1 |
| C3 | 低 | 变量拼进日志消息 | 3 |
| R1 | 高 | 插件协议帧类型在服务端与 SDK 各手写一份 | 2 |
| R2 | 高 | 插件视图有两层同构结构 | 3 |
| R3 | 中 | 工具函数多处重复 | 3 |
| R4 | 中 | Web 请求与刷新逻辑复制粘贴 | 4 |
| R5 | 中 | schema 快照与迁移并存 | 1 |
| R6 | 低 | 示例插件大同小异 | 1 |
| W1 | 高 | 同一个错误有三种说法 | 2 |
| W2 | 中 | 中文文案没有归属 | 3、4 |
| W3 | 中 | 产品与设计描述分散在多份文档 | 5 |
| X1 | 高 | 订阅功能迁出后遗留 5 张表与集成代码 | 1 |
| X2 | 高 | 历史兼容层 | 1 |
| X3 | 中 | 零引用代码 | 1 |
| X4 | 中 | 仓库体积与多余生成物 | 1、4 |
| X5 | 中 | CI 门禁位置不对 | 0、2、5 |

## 先修的缺陷

这些问题与清理无关，按阶段 0 或所列阶段单独修复并补回归测试。

### D1 多 OneBot 实例时插件动作与事件补全绑定首个实例

组合根保留了单适配器时期的“主 OneBot 实例”。插件调用 `group.member.get`、`message.history.get` 等动作时不看事件的 `source_adapter`，一律发往首个实例；第二个实例的消息事件用首个实例的身份缓存补全；系统状态中的 `adapter_state` 只反映首个实例。未配置 OneBot 时，组合根新建一个从不启动的占位 Shell，动作发往该 Shell，返回传输错误而不是“适配器不可用”。协议 v3 已按实例区分身份，这部分迁移没有完成。

- 位置：`server/internal/app/event_wiring.go:96-104`；`server/internal/app/plugin_wiring.go:94`；`server/internal/plugins/actions/onebot_registry.go:160-192`；`server/internal/app/service_build.go:124-126,182`；`server/internal/onebot11/shell_event_metadata.go:15-18`
- 处理：阶段 0 让本地动作与元数据补全按 `source_adapter` 选择实例；阶段 3 以适配器注册表取代 Primary 概念与占位 Shell。

### D2 仪表盘提示靠匹配服务端文案生成

`protocolSummary` 在错误码与说明文字中搜索 “dial tcp”“authentication failed”“websocket close”“warning” 等片段，`readableSummary` 按是否含中文决定原样展示还是替换。服务端调整一句日志或错误说明，仪表盘提示随之改变，违反 `web/AGENTS.md` 与 `server/AGENTS.md` 中“不比对 message”的约定。同文件的 `formatProtocolIssueSummary`、`formatProtocolEventSummary` 没有调用方。

- 位置：`web/src/lib/management-summary.ts:115-184`
- 处理：事件与诊断载荷只依赖稳定 code 和结构化字段，Web 用 code 查询 locale；删除两个无调用方的导出。

### D3 `config/default.yaml` 被写成科学计数法

文件中出现 `max_response_body_bytes: 4.194304e+06`、`ipc_message_max_bytes: 8.388608e+06`、`file_max_bytes: 1.048576e+07`。schema 默认值经 `json.Unmarshal` 解析为 `map[string]any`，数字成为 float64，再由 yaml.v3 写回文件。

- 位置：`config/default.yaml:38,66,83`；`server/internal/config/default_document.go:22-35`；`server/internal/config/canonical.go:210-216`
- 处理：随 H2 删除 default.yaml；仍需写出配置时从类型化结构序列化。

### D4 Run 监督器共用 `sync.Once` 上报错误

`GoCritical` 在 `once.Do` 内执行阻塞的 `ListenAndServe`，`report` 使用同一个 `once.Do`。服务运行期间后台任务返回错误时，上报阻塞到 HTTP 服务退出后被丢弃；后台任务先失败时，HTTP 服务不会启动，错误却被包装为 “listen on …”。当前唯一可能返回错误的 `AccountValidation.Run` 只在重复启动时出错，因此尚未触发。

- 位置：`server/internal/app/app_run.go:216-269`；`server/internal/integrations/accountvalidation/service.go:288-335`
- 处理：每个任务使用独立 goroutine，首个错误写入缓冲 channel 并取消上下文，HTTP 服务不与后台任务共用 `sync.Once`。

### D5 错误码契约测试只识别三个函数名

测试遍历 management 包的语法树，只识别 `writeAppError`、`writeAuthError`、`writeError` 与 `httpapi.WriteError`。代码中已经没有 `writeAppError`；`writeCoreAuthError`、`writeGovernanceError`、`writePluginStoreError` 等十余个包装函数传入的错误码，以及 runtime、actions、webhook 包中的错误码都不在检查范围内。

- 位置：`server/tests/architecture/error_codes_test.go:144-159`；`server/internal/management/core.go:224`；`server/internal/management/plugin_store.go:208`；`server/internal/management/third_party.go:234`
- 处理：随 H1 从 `contracts/error-codes.yaml` 生成 Go 常量，以编译期约束取代该测试。

## 分类发现

同一问题只归入最主要的类别。

### 过度防御编程

#### G1 必选依赖处处判 nil，并靠规则防 typed nil（高）

组合根和服务把本应必填的依赖当成可选：server 非测试代码中有 34 处 `ctx == nil`；`App.Close` 连续 12 次判断 `a != nil`；配置服务的每个闭包都判断 `deps.Runtime == nil`；`NewPluginRoutes` 已校验依赖非空，handler 内仍逐个判空；`system.Service` 同时接受函数和值两种依赖再做回退。单字母接收者的 nil 判断约 100 处。为了不让这些判断被 typed nil 绕过，`server/AGENTS.md` 增加了“装配到接口字段前检查具体指针”的规则，代码中配套出现 4 段解释性注释。

- 位置：`server/internal/app/app.go:67-69`；`server/internal/app/platform.go:48-51`；`server/internal/app/plugin_stack.go:49-52`；`server/internal/app/render_wiring.go:37-40,84-87`；`server/internal/app/app_close.go:13-72`；`server/internal/app/config_service.go:44-104`；`server/internal/app/management_routes.go:59-83`；`server/internal/management/plugins.go:192-196,260,350,366,391`；`server/internal/system/system_service.go:54-75,137-177,216-235`；`server/internal/app/service_build.go:103-108,187-196`；`server/internal/wsevents/ingress.go:95-117`
- 处理：构造函数要求全部必填依赖，缺失时在装配期返回 error，业务方法内不再判空；确实可选的能力（如抖音浏览器）使用显式空实现。完成后删除 typed nil 规则。

#### G2 Launcher 手写载荷校验并拒绝未知字段（高）

`expectAllowedKeys` 遇到未知字段就抛错。服务端给 `/readyz` 或 `/api/launcher/status` 增加一个展示字段，已安装的 Launcher 就会把服务判为载荷无效，与 `contracts/AGENTS.md` 中“可扩展展示枚举允许兼容降级”的策略相反。Go 侧 `ManagementClient` 把响应解码为无类型的 `JSONObject` 交给渲染层，渲染层再逐字段校验，两层都不是类型化的。

- 位置：`launcher/src/shared/server-payload-validation.ts:26-31,216-277`；`launcher/internal/desktop/management.go:83-105,140-146`
- 处理：Go 侧解码为结构体，经 Wails 绑定生成 TS 类型；渲染层不再二次校验，未知字段直接忽略。

#### G3 已校验的值被反复 `TrimSpace`（中）

server 非测试代码调用 `strings.TrimSpace` 1,257 次，集中在 `plugins/runtime/actions.go`（51）、`builtinmenu/menu.go`（48）、`eventpipeline/outbound/observability.go`（36）、`integrations/douyin/browser_runtime.go`（32）、`logging/summary_writer.go`（27）。多数调用作用在已经过 schema 校验或由服务端生成的值上，读者无法判断真正的输入边界在哪里。

- 处理：只在边界规范化一次，包括 HTTP 解码、配置加载、插件帧解析和第三方响应；内部类型保证值已规范。

#### G4 为不会发生的情况写的分支（中）

以下分支对应的情况在现有调用方下不会出现，却增加阅读和测试负担：

- `server/internal/configruntime/apply.go:235-241`：为两个 map 长度之和做 int 溢出检查。
- `server/internal/httpapi/httpapi.go:134-137,201-203`：耗时为 0 时改成 1ns；`accessLogLevel` 两个参数都未使用，恒返回 Debug。
- `server/internal/eventpipeline/dispatch/dispatch.go:134-154`：可变参数 `controlQueueSize ...int` 加回退值 16 与 4。
- `server/internal/onebot11/shell.go:96-121`：logger 为 nil 时构造一个丢弃输出的 logger。
- `web/src/stores/socket-router.ts:204-208`：检查 `queueMicrotask` 是否存在。

处理：直接删除；需要默认值的地方由配置提供。

#### G5 失败被静默吞掉或用类型断言兜底（中）

插件安装成功后的重载错误被 `_, _ =` 丢弃，而同一回调中的开发插件分支会返回错误；生命周期控制器多处用 `if snapshot, err := SetRuntimeState(...); err == nil` 忽略写入失败；安装检查 handler 把安装服务断言为 `InstallInspector`，断言失败时返回 500。

- 位置：`server/internal/app/app_run.go:291-311`；`server/internal/plugins/lifecycle/controller.go:241-243,272-283,321-331,419`；`server/internal/management/plugins.go:192-196`
- 处理：回调统一返回并记录错误；安装服务直接依赖声明了 `Inspect` 的接口。

### 过度或无用测试

#### T1 services 测试手工复刻组合根（高）

`harness_test.go` 按注释所说的 “the same way the composition root does” 重新组装 Ingress、LocalActions、Governance 与 Webhook，自带一份带 nil 接收者判断的 `harnessState`；`executeOneBotLocalAction` 与 `executeLocalAction` 实现相同；`setTestLocalActions` 的 `webhookService` 参数被 `_ =` 丢弃；5 个 handler 方法只转发到已导出方法。依赖它的 18 个文件共 5,963 行，测试的其实是 chatpolicy、actions 的包内行为，与包内测试重叠，组合根一改就要同步两处。

- 位置：`server/tests/services/harness_test.go:36-68,259,283-289,468-486`
- 处理：删除 harness。包内行为回到包内测试；装配与跨包流程改用真实组合根加临时 SQLite。

#### T2 为测试而存在的生产 API（高）

38 个导出符号只被测试引用，例如 `app.New`（生产使用 `NewWithContext`）、`auth.NewManager`、`(auth.Manager).Login`、`(auth.Manager).Issue`、`management.RequireAuth`、`management.NewGovernanceHandlers`、`(scheduler.Engine).UpsertTask`，以及被 97 处测试调用的 `(permission.CooldownTracker).Cleanup`。另有专供测试的 `onebot11.NewForTest`、全局钩子 `deps.SetSystemChromiumFinderForTest`、包级变量 `resolveManagedBrowserPath`，`app.Options` 中两个字段的注释写明是测试缝。带与不带 ctx 的双版本 API 也多为测试保留：`New`、`NewManager`、`Login`、`Issue` 的无 ctx 版本只被测试使用，`Revoke`、`StopAndResetPlugin` 的无 ctx 版本已无调用方。

- 位置：`server/internal/onebot11/shell.go:93`；`server/internal/deps/manager_prepare.go:24`；`server/internal/app/render_wiring.go:112`；`server/internal/app/app.go:36-41`；完整清单见附录
- 处理：每个操作只保留带 ctx 的版本；测试替身通过构造参数注入，这条规则已写在 `server/AGENTS.md` 中。

#### T3 绑定实现细节或构建配置的前端测试（中）

`main.spec.ts`（498 行）mock 掉 vue、pinia、router、http 和 4 个 store，断言启动时的调用；`web/tests/unit/vite-config.spec.ts` 与 Launcher 的 `vite-config.test.ts`、`package-metadata.test.ts` 测试构建配置函数；`navigation-icons.spec.ts` 断言菜单图标互不相同。这些测试随实现改动而改动，却难以发现真实回归，不符合根 `AGENTS.md` 的测试原则。

- 位置：`web/tests/unit/main.spec.ts`；`web/tests/unit/vite-config.spec.ts`；`web/tests/unit/navigation-icons.spec.ts`；`launcher/tests/scripts/vite-config.test.ts`；`launcher/tests/scripts/package-metadata.test.ts`
- 处理：删除，启动流程由 E2E 覆盖。

#### T4 工具脚本的测试比脚本本身更重（中）

`test_start_bat.py`（86 行）和 `test_start_sh.py`（101 行）用伪造的 node 验证两个启动包装脚本；`check-agent-docs.test.mjs`（129 行）测试一个大段正则并不生效的检查脚本（见 Q6）；`historical-log-redaction.test.mjs` 测试一次性的历史日志清洗。`scripts/tests` 共 15 个文件约 1,530 行，CI 在 Ubuntu 与 Windows 上各运行一遍。

- 位置：`scripts/tests/`；`.github/workflows/ci.yml:408-441`
- 处理：保留插件开发工作区、构建缓存等含真实逻辑的脚本测试，其余随脚本删除或简化。

#### T5 E2E mock 后端重新实现服务端业务（中）

`mock-backend.mjs`（2,019 行）从 `mock-config.mjs` 引入 `computeConfigApplyEffects`、`redactConfigSecrets`、`restoreRedactedConfigSecrets`、`computeProtocolSnapshotFromConfig`，配置生效策略与脱敏规则在 JS 中另有一份。`web-ui.spec.ts` 单文件 2,591 行、58 个用例，2026 年 7 月以来改动 40 次，是全仓改动最频繁的文件。仓库已有面向真实 Server 的 production 配置。

- 位置：`web/tests/e2e/mock-backend.mjs:1`；`web/tests/e2e/web-ui.spec.ts`；`web/playwright.production.config.ts`
- 处理：E2E 以真实 Server 为主，mock 只保留插件 iframe 握手等本地难以复现的场景；按工作区拆分超大 spec。

#### T6 同一类约束有两套检查（中）

`check-server-structure.py`（由 `make doctor` 执行）与 `server/tests/architecture/structure_test.go` 各检查一部分导入规则；手写 SQL 需要在 `manual-sql-exceptions.json` 登记 category A–D、owner（server-storage、server-plugins 等并不存在的团队）、target_action 与 revisit_after，再由 Python 脚本逐项校验。

- 位置：`scripts/check-server-structure.py`；`server/tests/architecture/structure_test.go`；`docs/engineering/manual-sql-exceptions.json`
- 处理：只保留一份 Go 架构测试；手写 SQL 在代码旁用注释说明原因。

### 硬编码

#### H1 错误码是散落的字符串字面量（高）

goconst 统计到 `plugin.internal_error` 35 处、`platform.resource_missing` 24 处、`plugin.permission_denied` 16 处、`plugin.protocol_violation` 15 处、`platform.invalid_request` 14 处，各包又分别定义同值常量。Web 还使用了契约中不存在的 `platform.unknown`。

- 位置：`server/internal/plugins/runtime/runtime_model.go:25-39`；`server/internal/eventpipeline/bridge/bridge.go:15-16`；`server/internal/management/plugins.go:27-30`；`server/internal/management/core.go:18-21`；`web/src/lib/http.ts:13`
- 处理：从 `contracts/error-codes.yaml` 生成 Go 常量与错误目录（HTTP 状态、message、message_key），handler 只调用一个 `Fail(w, r, code, details)`。

#### H2 配置默认值有四份（高）

默认值同时来自 JSON Schema（契约及其嵌入副本）、运行时写回并随发行包分发的 `config/default.yaml`、Go 常量（`DefaultRenderFooterTemplate`、`DefaultUserCommandRateLimit` 等，其中 `DefaultCooldownReply` 零引用），以及构造函数中的回退值（`dispatch.New` 的 16 与 4、`tasks.NewExecutor(…, 5*time.Minute)`、`console.NewStream(1000, 2*1024*1024)`）；Launcher 再硬编码一次 `127.0.0.1:8080`。加载时按 schema 默认值、default.yaml、user.yaml 三层合并，`loadCanonicalDocument` 与 `normalizeCanonicalDocument` 重复约 40 行。

- 位置：`server/internal/config/canonical.go:15-21,30-129`；`server/internal/app/app_run_app_build_initialize.go:45`；`server/internal/app/platform.go:178`；`launcher/internal/desktop/management.go:31-64`
- 处理：schema 默认值作为唯一来源，删除 default.yaml 与 Go 默认常量；运行期常量在所属包顶部命名集中。

#### H3 工具链版本号在 12 个文件中出现 109 次（中）

Go、Node.js、Python、pnpm、Corepack 与 sqlc 的版本号分布在 `.tool-versions`、`.devcontainer/Dockerfile`、`README.md`、`docs/engineering/baseline.md`（16 次）、`scripts/check-toolchain.py`（22 次）、两个 `package.json`，以及 `ci.yml`（25 次）、`nightly.yml`（15 次）、`release.yml`、`self-host-smoke.yml`、`repo-stats.yml` 中逐 job 重复的 `node-version`、`python-version`。`docs/engineering/quality-gates.md` 却声明 CI 不单独维护另一套版本号。

- 处理：CI 使用 `go-version-file`、`node-version-file`、`python-version-file` 读取 `go.mod` 与 `.tool-versions`；文档只链接工程文件，不抄写版本号。

#### H4 监听地址归一化有四份且规则不一致（中）

把通配地址换成 `127.0.0.1` 的逻辑有四份：`main` 只处理空值、`0.0.0.0` 与 `::`；httpapi 多了 `[::]`；Launcher 多了 `*`；开发脚本四种都处理。回环地址判断另有两份。

- 位置：`server/cmd/raylea-server/main.go:111-118`；`server/internal/httpapi/httpapi.go:205-219`；`launcher/internal/desktop/management.go:66-72`；`scripts/start-dev-support.mjs:23`；`server/internal/app/management_routes.go:221-228`；`server/internal/management/core.go:182-205`
- 处理：服务端收敛为一个函数；Launcher 与开发脚本使用同一组规则并在一处说明。

#### H5 环境变量、路径与超时埋在组合根（中）

组合根在两处直接读取 `RAYLEA_WEB_UI_BASE_URL`；抖音登录 profile 目录硬编码为 `data/douyin-login-profile`；协议服务中拼接 qlogo 头像地址；关闭流程的 5 秒超时出现 6 次。

- 位置：`server/internal/app/http_wiring.go:132`；`server/internal/app/management_routes.go:167`；`server/internal/app/integration_wiring.go:98-105`；`server/internal/wsevents/protocol.go:241-251`；`server/internal/app/app_run.go:156-173`；`server/internal/app/app_close.go:18`
- 处理：环境变量只在 `main` 读取并放入 `Options`；运行目录由路径模块统一推导；头像地址归 OneBot 适配器；关闭超时定义为命名常量。

#### H6 过期版本号与个人环境痕迹（低）

`web/package.json` 与 `launcher/package.json` 的 version 仍为 0.1.0，而 CHANGELOGS 已记录到 v0.4，`sdk/vue` 为 0.2.0；README 示例命令指向本机相邻仓库 `../RayleaBotPlugins/plugin-fortune`；`scripts/gbash.ps1` 硬编码 `C:\Program Files\Git`；`.gitignore` 包含 `临时文件夹/`、`external/`、`/reference/` 以及 `.kiro`、`.gemini`、`.sisyphus/` 等个人工具目录。

- 位置：`web/package.json:4`；`launcher/package.json:3`；`README.md:113`；`scripts/gbash.ps1:3`；`.gitignore`
- 处理：版本号由发布流程写入；个人工具目录放入全局 gitignore。

### 低质量代码

#### Q1 组合根靠 setter 与闭包拼装循环依赖（高）

`configureAppRuntimeCallbacks` 在所有服务构造完成后补线：安装服务的 `SetAfterSuccess`、`SetAfterRollback`、`SetBeforeReplace`、`SetRenderTemplateValidator`，卸载服务的 `SetStopPlugin`、`SetAfterSuccess`，运行时的 `SetOnCrash`，以及每个适配器的事件、就绪与状态处理器；首个 OneBot Shell 的处理器被设置两次，后者覆盖前者。调度触发通过一个起初为 nil 的闭包转发，状态发布依赖延迟引用。服务端用于装配的 `Set*`、`Bind*` 方法约 30 个，组合根中的 deps 与 state 结构体有 20 余个。

关闭流程同样重复：正常关闭时 `stopRuntimeManagers` 先由 `shutdownFromContext` 调用，再由 `Close` 调用一次，Dispatcher 被关闭多次；`shutdownFromContext` 与 `shutdownAfterServerExit` 逻辑重复。

- 位置：`server/internal/app/app_run.go:151-185,282-352`；`server/internal/app/app.go:126-134`；`server/internal/app/service_build.go:122,148-152`；`server/internal/app/app_close.go:11-91`；`server/internal/app/event_wiring.go:143-149`
- 处理：按依赖顺序构造，必填依赖进入构造函数；真正的环（进程崩溃与调度触发回到生命周期）改为事件订阅；关闭使用构造时登记的有序 closer 列表。

#### Q2 配置在类型化结构与 map 之间反复往返（高）

保存配置时，类型化的 `Config` 先转换为 map（`canonical_typed.go` 共 294 行），逐路径比较后按路径查询生效策略；随后 `applyHotReloadableFieldsLocked` 又手写逐字段比较日志、限流与渲染设置，与 schema 的 `x-apply-policy` 元数据形成两套判断；解析 secret 引用时再做一次 JSON 往返。`apply.go:331` 用 `!=` 比较错误而不是 `errors.Is`。

- 位置：`server/internal/configruntime/apply.go:48-94,124-143,272-345`；`server/internal/config/canonical_typed.go`；`server/internal/configruntime/config_secret_refs.go:61-83`
- 处理：配置全程类型化；每个服务实现 `ApplyConfig(old, new)` 并自行判断相关字段；schema 元数据只用于响应中的 `restart_required_fields`。

#### Q3 插件运行态存了三份（高）

运行态同时存在于运行时 Manager、Catalog 快照的 `RuntimeState` 与 Dispatcher 注册表中，生命周期包用 25 处 `SetRuntimeState` 调用手工保持同步；`plugin_model.go` 为快照维护 50 行深拷贝；控制器混用字符串 `"installed"`、`"enabled"` 与常量 `plugins.DesiredStateEnabled`。

- 位置：`server/internal/plugins/lifecycle/controller.go:222-336`；`server/internal/plugins/lifecycle/reload.go`；`server/internal/plugins/plugin_model.go:246-297`
- 处理：Catalog 只保存声明与期望态，运行态在读取时从 Manager 投影。

#### Q4 超大函数与文件（中）

认知复杂度最高的函数是 `(*Dispatcher).worker`（108）、`(*ChromedpBrowser).Poll`（74）、`(*InstallService).runInstall`（68）、`backup.Create`（67）与 `(*InstallService).prepareSource`（60）。`plugins/lifecycle/install.go` 有 1,280 行，`plugins/lifecycle/controller.go` 有 1,100 行；Web 的 `PluginDetailView.vue` 有 1,397 行，`SchedulerJobsView.vue` 有 1,285 行。完整列表见附录。

- 位置：`server/internal/eventpipeline/dispatch/worker.go:17`；`server/internal/plugins/lifecycle/install.go:774,1011,1264`
- 处理：迁入新包时按阶段拆分：排队、投递、结果归档；下载、解包、校验、提交。

#### Q5 薄包、名不副实的包与倒挂依赖（中）

59 个包中有 9 个不足 130 行：`testenv`、`health`、`command`、`semver`、`reconnect`、`pubsub`、`logpath`、`filelock`、`console`。`wsevents` 实际承载协议快照、适配器列表、OneBot 入站与兼容矩阵；`plugins/lifecycle` 同时负责安装与卸载；OneBot 适配器类型名为 `Shell`。依赖方向存在倒挂：`runtimepaths` 为一个 `ScanRoot` 类型依赖 `plugins/catalog`，`eventpipeline/dispatch` 为错误类型依赖 `onebot11`，`configruntime` 依赖 `plugins/actions` 与 `render/service`。`onebot11/shell.go` 的 import 分组被打乱，PR 门禁没有格式检查。

- 位置：`server/internal/runtimepaths/paths.go:18,43`；`server/internal/eventpipeline/dispatch/outbound_action.go:22-23`；`server/internal/onebot11/shell.go:3-19`
- 处理：按“现有包的去向”合并与改名。

#### Q6 低质量或一次性的脚本（中）

`check-agent-docs.mjs` 的两个巨型正则列举了上百种音频编解码器和标准组织，而前一行条件使第 200 行永远为假，`EXTENSIONS` 从未生效；`redact-historical-logs.mjs` 是一次性的历史日志清洗；`generate-repo-stats.py`（310 行）只为 README 生成提交统计图。`check-toolchain.py` 要求 Python 恰好为 3.14.7、npm 恰好为 11.19.0，而 Makefile 的 `test-server`、`server-build` 都依赖它，只运行 `go test` 也必须安装 Node.js、pnpm、Corepack、Python 与 sqlc，CI 的 server job 因此安装了全部工具链。

- 位置：`scripts/check-agent-docs.mjs:152-153,198-200`；`scripts/check-toolchain.py:18-24,402-406`；`Makefile:17-35`；`.github/workflows/ci.yml:80-108`
- 处理：删除一次性脚本与统计图；doctor 只检查当前任务所需工具，版本不符时给出警告而不是失败。

#### Q7 前端结构碎片化（低）

仪表盘拆成 8 个 composable，其中 `useDashboardPage.ts` 只有 10 行、`useDashboardDerivedState.ts` 只有 28 行；Launcher 渲染层有 15 个 `AppShell*` 文件，另有 2,097 行全局 `style.css` 覆盖 Fluent UI 组件；Web 同时维护 96 个 shadcn-vue 源码文件与 37 个 `App*` 封装组件。

- 位置：`web/src/views/dashboard/`；`launcher/src/renderer/src/style.css`；`web/src/components/ui/`
- 处理：组件层二选一；composable 按页面职责组织，不按数据片段拆文件。

### 过时文档

#### S1 项目章程与 README 描述旧版本系统（高）

章程写着“OneBot11 是当前唯一已交付的正式聊天协议”，架构图中 QQ 官方机器人仍标为 planned；“预编译 Go 可执行文件是唯一正式插件后端”与 artifact v2 不限实现语言相矛盾；“独立插件通过签名静态目录发现”与插件商店现状不符，`pluginmarket` 代码和 catalog schema 中已没有签名字段。README 仍写“基于 OneBot11 协议接入 QQ”和“JSONL v1 协议”，当前协议为 v3；发布验收文档也写着“只消费受信签名静态目录”。

- 位置：`docs/RayleaBot机器人项目规划.md:14,20,24,57`；`README.md:9,15`；`docs/release/acceptance-and-risks.md:57`
- 处理：章程只保留使命、非目标与原则，能力清单交给现行文档；README 改为多适配器与协议 v3 的描述。

#### S2 架构文档没有跟上多适配器（高）

平台架构图、事件管线图与关闭流程都只画了一个 OneBot11 适配器。代码中真实存在的“首个实例”语义没有写进文档，却已经进入配置差异计算的注释：实例顺序决定管理面把哪个实例当作 primary。

- 位置：`docs/architecture/platform-architecture.md:26,49`；`docs/architecture/event-pipeline.md:7,20`；`docs/architecture/server-lifecycle.md:39`；`server/internal/configruntime/apply.go:194-196`
- 处理：阶段 3 引入适配器注册表时同步重写这三篇，删除 primary 语义。

#### S3 一次性过程记录混在正式文档里（中）

根目录 `design-qa.md` 是一次配置页视觉验收记录，引用本机 `.codex` 生成图片目录等临时路径；`docs/release/log-runtime-repair-2026-09-04.md` 是事故复盘，包含二进制指纹与观察时间窗口，并写着“插件协议保持版本 2”，当前协议为 v3；`plugin-contract-v3-upgrade.md` 与 `plugin-protocol-v3-upgrade.md` 内容交叠。

- 位置：`design-qa.md`；`docs/release/log-runtime-repair-2026-09-04.md:32`；`docs/release/plugin-contract-v3-upgrade.md`；`docs/release/plugin-protocol-v3-upgrade.md`
- 处理：删除前两份；两份升级说明并入下一个破坏性版本的版本说明。

#### S4 失效路径、重复页与复述 CI 的文档（中）

`web-admin-baseline.md` 的目录表列出不存在的 `request/` 目录；`docs/architecture/storage-migrations.md` 只有 12 行，转述 `docs/engineering/storage-migrations.md`，后者是工程文档中唯一用英文写的一篇；`quality-gates.md` 逐 job 复述 `ci.yml`；`.gitignore` 仍保留 Python 插件目录的 `.venv` 与 `__pycache__` 规则。

- 位置：`docs/engineering/web-admin-baseline.md:43`；`docs/architecture/storage-migrations.md`；`docs/engineering/quality-gates.md:34-58`
- 处理：修正路径，合并重复页；质量门禁文档只保留原则，并链接到工作流文件。

#### S5 文档复制契约与实现（中）

2026 年 7 月以来的改动次数：`contracts/README.md` 23 次、`docs/engineering/baseline.md` 22 次、`docs/plugin/protocol.md` 20 次、`docs/engineering/web-admin-baseline.md` 19 次、`server/README.md` 16 次。这些文件频繁变动，是因为它们用散文重述契约与实现：`contracts/README.md` 描述作用域规则、会话词表、消息段集合与 provider 动作名；`web-admin-baseline.md` 记录动画时长、iframe 缩放原点、时区列表排序等实现细节。

- 位置：`contracts/README.md:49,82-98`；`docs/engineering/web-admin-baseline.md:19-34,106`
- 处理：文档只写意图、边界与索引，字段级事实只保留在契约中。

### 过时文案

#### C1 服务端下发中文标签，Web 与 fixtures 各存一份（中）

插件信任视图由服务端返回“官方”“开发中”“未验证来源”“第三方”；Web locale 中有同样的“第三方”“未验证来源”；7 个 web-api fixtures 断言 `label: 未验证来源`。修改一个词需要同时修改 Go、TypeScript 与契约样例。

- 位置：`server/internal/plugins/summary_view.go:238-250`；`web/src/locales/zh-CN/plugins.ts:227,332`；`fixtures/web-api/ok.plugins-list-response.yaml:57` 等 7 个文件
- 处理：服务端只返回 `level` 枚举，标签由客户端 locale 提供；fixtures 删除 label。

#### C2 Launcher 保留已不存在的诊断项文案（低）

`diagnosticCheckNameLabels` 中仍有 `bilibili_source: "Bilibili 来源"`，服务端诊断已没有这一项，订阅功能也已迁到独立插件。

- 位置：`launcher/src/renderer/src/AppShell.copy.ts:38`
- 处理：删除该条；未知诊断 code 原样显示。

#### C3 变量拼进日志消息（低）

“服务正在启动，管理地址：”+serverURL、“请求处理异常：%s %s”、“日志级别已从 … 调整为 …”、“配置文件”+actionLabel+“失败：”等写法把变量拼进 message，同样的值又写入结构化字段。日志无法按消息聚合，管理日志界面中出现大量近似文案。

- 位置：`server/internal/app/app_run.go:136`；`server/internal/httpapi/httpapi.go:114,144`；`server/internal/configruntime/apply.go:283,332`；`server/internal/cli/cli.go:108,111`
- 处理：消息使用固定模板，变量只进入结构化字段，展示时再格式化。

### 重复代码

#### R1 插件协议帧类型在服务端与 SDK 各手写一份（高）

服务端定义约 30 个帧结构，SDK 使用一个扁平的 `protocolFrame` 联合体，两者都从 `contracts/plugin-protocol.schema.json` 手工翻译，协议字段变化时需要修改契约、服务端与 SDK 三处。敏感信息正则也在 SDK、服务端与开发脚本中各有一份。

- 位置：`server/internal/plugins/runtime/runtime_model.go:83-377`；`sdk/go/protocol.go:14-37`；`server/internal/redact/`；`scripts/log-redaction.mjs:3-5`
- 处理：从 schema 生成 Go 帧类型，服务端与 SDK 共用同一份生成结果。

#### R2 插件视图有两层同构结构（高）

`plugins.BuildSummaryView` 构造 `SummaryView`，`management.ToSummary` 再逐字段拷贝成几乎相同的 `SummaryResponse`；`plugins.normalizeStringViews` 与 `management.NormalizeStringList` 逐行相同。这来自 `server/AGENTS.md` 中“领域视图在领域包构建，management 只负责序列化”的规则：规则要求两层，实现就写了两份。

- 位置：`server/internal/plugins/summary_view.go:39-77,190-206`；`server/internal/management/plugin_view_summary.go:79-98,139-156`
- 处理：二选一。领域视图直接按契约命名并带 JSON 标签，由 api 层原样输出；或者从 OpenAPI 生成 DTO，由领域层填充。

#### R3 工具函数多处重复（中）

`cloneMap` 与 `cloneValue` 有 5 份，其中 config 版本使用 JSON 往返；`stringValue` 有 4 份；`writeJSON` 有 2 份，另有两个纯转发包装；语义化版本比较有 3 份，严格程度各不相同；`recovery.SaveSummary` 与零引用的 `SaveSummaryToFile` 函数体相同；`UninstallCoordinator` 接口在两个包中各定义一次。多数可以直接改用 `maps.Clone`、`slices.Contains` 与内置 `max`。

- 位置：`server/internal/plugins/plugin_model.go:377-412`；`server/internal/plugins/pluginstore/config.go:185-193`；`server/internal/logging/details.go:25`；`server/internal/tasks/task_registry.go:431`；`server/internal/config/canonical.go:306`；`server/internal/semver/semver.go`；`server/internal/releaseupdate/semver.go`；`launcher/internal/desktop/release.go:30`；`server/internal/recovery/summary.go:32-81`；`server/internal/management/plugins.go:101,424-430`
- 处理：改用标准库，只保留一个 semver 实现。

#### R4 Web 请求与刷新逻辑复制粘贴（中）

`apiRequest` 与 `apiDownload` 的超时、fetch 与网络错误处理几乎逐行相同；socket-router 为服务状态、治理与三方账号各写了一套“防抖、进行中、排队”的刷新调度；`configureApiRuntime` 逐字段用 if 赋值。

- 位置：`web/src/lib/http.ts:44-64,197-312`；`web/src/stores/socket-router.ts:46-142`
- 处理：提取一个请求内核与一个 `createCoalescedRefresh(fn)`；超时改用 `AbortSignal.any` 与 `AbortSignal.timeout`。

#### R5 schema 快照与迁移并存（中）

`schema.sql`（244 行）与 `000001_base.sql`（246 行）同时存在，另有 7 个迁移、3 个“目标结构已存在则跳过”的检测函数、`schema_migrations.name` 列回填，以及重复的 `recordMigration` 与 `recordMigrationTx`。

- 位置：`server/internal/storage/store_schema.go:52-92,261-335`；`server/internal/storage/migrations/`
- 处理：破坏兼容时合并为一个基线 schema，迁移编号从 1 重新开始。

#### R6 示例插件大同小异（低）

`go.work` 纳入 10 个示例模块，其中 `echo-go`（27 行）、`hello-go`（15 行）、`notice-logger`（31 行）与 `example-plugin-list`（40 行）结构几乎相同；服务端测试数据中还有一份 echo 插件。

- 位置：`go.work`；`examples/plugins/`；`server/tests/testutil/testdata/echo-plugin/`
- 处理：保留一个最小示例与一个带管理页的完整示例，其余改为文档片段。

### 重复文案

#### W1 同一个错误有三种说法（高）

错误文案同时存在于 `contracts/error-codes.yaml` 的 message、服务端 handler 的中文字面量，以及 Web 的 `errors.ts`。“内部错误”“请求参数不合法”“缺少必要资源”“当前用户无权执行该操作”在 20 个文件中出现 131 次，并且与 Web 措辞不同：服务端为“请求参数不合法”，Web 为“请求参数不正确，请检查后重试。”；服务端为“插件包包含不安全条目”，Web 为“插件包包含不安全文件。”。Web 还需要把 snake_case 的 message_key 转为 camelCase 才能查到 locale。

- 位置：`server/internal/management/plugins.go:189-323`；`web/src/locales/zh-CN/errors.ts`；`web/src/lib/error-text.ts:4-8`
- 处理：错误文案只在 `error-codes.yaml` 中维护，Go 错误目录与 Web locale 都由它生成。

#### W2 中文文案没有归属（中）

非测试代码中含中文的行：server 有 1,195 行，集中在 `plugins/lifecycle/install.go`（82）、`wsevents/protocol_snapshot.go`（55）与 `eventpipeline/bridge/summary.go`（43）；Web 在 locales 之外有 426 行，集中在 `SchedulerJobsView.vue`（107）、`AdapterConfigDialog.vue`（40）与 `management-summary.ts`（33）；Launcher 有 563 行，其中 Go 桌面层的 `release.go` 与 `environment.go` 分别为 51 行和 46 行。`docs/dev/text-resources.md` 说明 Web 使用 vue-i18n 资源，实际约四分之一的 Web 中文不在资源文件中。

- 处理：服务端只下发 code 与结构化字段，面向用户的文案归 Web 与 Launcher；Web 文案全部进入 locale，并增加检查拦截 locales 之外的中文。

#### W3 产品与设计描述分散在多份文档（中）

组件职责与边界在 `README.md`、`PRODUCT.md`、项目章程、`server/README.md` 与 `docs/architecture/platform-architecture.md` 中各写一遍，已经互相矛盾（见 S1）。设计规范分布在 `DESIGN.md`（2026 年 7 月以来改动 24 次）、`docs/design/web-management-ui.md`（23 次）、`docs/design/launcher-design-system.md` 与 `.impeccable/design.json`。

- 处理：每类事实指定一个归属文件，其余文件只链接。

### 死代码与仓库卫生

#### X1 订阅功能迁出后遗留 5 张表与集成代码（高）

`bilibili_source_config`、`bilibili_source_rooms`、`bilibili_source_seen`、`bilibili_source_dynamics`、`bilibili_source_state` 除了 schema、迁移 000004 与 `store_test.go` 中“表必须存在”的断言，没有任何代码使用，订阅已由独立插件 `raylea.subscription-hub` 承担。`integrations/bilibili/session`（1,975 行）中大量导出零引用或只被测试引用，包括 WBI 签名、验证码与直播请求头，`NewSessionClient` 本身也只被测试调用。

- 位置：`server/internal/storage/schema.sql:184-246`；`server/internal/storage/store_test.go:56-83`；`server/internal/integrations/bilibili/session/`
- 处理：删除表、迁移与断言；bilibili 包只保留扫码登录、资料与凭据校验所需部分。

#### X2 历史兼容层（高）

仓库保留了多组面向旧版本数据的兼容代码：配置 v3→v4 迁移（206 行，另有 333 行测试）与每次启动都执行的 secret 键搬迁；旧版无盐 SHA-256 密码摘要仍可登录，登录时再升级；迁移 000002–000004 的跳过检测；nightly 中专门演练 v3 配置、v7 数据库与旧密码摘要的 `rehearse_data_migration.py`；开发脚本中的 `air` 旧别名。

- 位置：`server/internal/config/migrate.go`；`server/internal/configruntime/config_secret_refs.go:133-173`；`server/internal/app/app.go:166-172`；`server/internal/auth/password_hash.go:94-109,189-192`；`scripts/release/rehearse_data_migration.py`；`.github/workflows/nightly.yml:275-279`；`scripts/start-dev-support.mjs:21-22`
- 处理：在破坏性版本中一次删除，版本说明写明需要全新安装或手工迁移。

#### X3 零引用代码（中）

64 个 Go 导出符号没有任何引用，例如 `deps.Extract`、`deps.Zip`、`deps.TarGz`、`deps.HTTPSFile`、`logging.New`、`logging.NewWithStream`、`pubsub.NewHub`、`recovery.RemoveSummary`、`recovery.HasSummary`、`(auth.Manager).Revoke`、`(scheduler.Engine).UnregisterByPlugin`、`thirdparty.IsRiskControlError` 与 `onebot11.NormalizeAPIList`；sqlc 查询 `CountNamespace` 与 `GetPluginStoreSource` 没有调用方；Web 有 15 个导出零引用，如 `buildProtocolWorkbenchActions`、`toSafeDisplayText` 与 `mergeLogItemsAsc`。

- 处理：按附录清单删除；PR 门禁加入 unused 检查，Web 增加未使用导出检查。

#### X4 仓库体积与多余生成物（中）

`.agents/skills/impeccable` 占 2.1 MB，其中 `font-index.json` 为 1 MB、`live-browser.js` 为 523 KB，而根 `AGENTS.md` 说明外部 skill 由上游维护、不纳入项目修改；`celadon-glass.png` 占 1.6 MB，只用于保留生成提示词，运行时使用 webp；`templates/help.menu` 带 100 个字体分片，共 4.5 MB；Launcher 生成了 186 KB 的完整 OpenAPI 类型，而 Go 侧只调用 `/healthz`、`/readyz`、`/api/launcher/status` 与 `/api/launcher/shutdown` 四个端点。

- 位置：`.agents/skills/impeccable/`；`web/src/assets/auth/celadon-glass.png`；`templates/help.menu/assets/fonts/`；`launcher/src/shared/web-api.generated.ts`
- 处理：外部 skill 改为安装说明；提示词另存为文本后删除 PNG；Launcher 只生成用到的类型。

#### X5 CI 门禁位置不对（中）

golangci-lint 只在 nightly 运行，本地结果已是 0 个问题，却没有进入 PR 门禁。PR 门禁运行的是要求精确工具链版本的 `make doctor` 与自研的 `validate_contracts.py`（1,480 行），后者包含手工维护的 `STRICT_OPENAPI_PATHS` 路径清单，每增加一个接口都要在这里再登记一次；`detect_changes.py`（254 行）自行实现路径过滤。

- 位置：`.github/workflows/nightly.yml:59-64`；`.github/workflows/ci.yml:107-108`；`scripts/ci/validate_contracts.py:112`；`scripts/ci/detect_changes.py`
- 处理：阶段 0 把 lint 加入 PR 门禁；阶段 2 让契约校验从 OpenAPI 读取路径；阶段 5 把 PR 门禁整理为 lint、生成物漂移、契约校验与各端测试。

## 架构方案

目标是在允许破坏兼容的前提下，把服务端从按实现过程增长的 59 个包收敛为按业务能力划分、依赖只向下的约 35 个包，并用契约生成的代码取代手写副本。

### 现状

下表来自 `server/internal` 非测试文件的导入统计。组合根依赖大量包属于正常情况；问题在于 management、生命周期与本地动作等业务包也横向依赖十余个包，事件管线还反向依赖具体适配器与插件生命周期。

| 包 | 手写行数 | 依赖的内部包 | 依赖它的包 |
| --- | ---: | ---: | ---: |
| `app` | 3,303 | 45 | 0 |
| `management` | 5,646 | 27 | 1 |
| `plugins/lifecycle` | 3,248 | 14 | 4 |
| `plugins/actions` | 3,423 | 13 | 3 |
| `system` | 1,999 | 13 | 2 |
| `cli` | 897 | 13 | 0 |
| `eventpipeline/chatpolicy` | 791 | 10 | 1 |
| `plugins/runtime` | 3,963 | 10 | 2 |
| `storage` | 1,010 | 3 | 17 |
| `config` | 2,002 | 0 | 25 |

结构问题：

- **没有适配器端口。** `onebot11` 被 dispatch、outbound、actions、runtime、wsevents 与 app 直接依赖；QQ 官方以旁路方式接入，于是出现“首个 OneBot 实例”这一隐式概念（D1）。
- **依赖方向靠 setter 反转。** 安装、生命周期、渲染与 webhook 之间形成环，调度触发与进程崩溃又回调生命周期，组合根只能先构造对象再补线（Q1）。
- **横切关注点没有归属。** 配置生效、错误码、文案与运行态投影分散在各层，同一事实处处有副本（Q2、Q3、H1、W1）。
- **读模型层层复制。** wsevents、system、领域视图与 management DTO 各有一层同构结构（R2）。

### 目标原则

- 包按业务能力划分，依赖只向下：入口 → 组合根 → 领域与能力 → 平台。同层协作使用构造时注入的小接口，不使用 setter。
- 聊天平台一律实现 `chat.Adapter` 并以实例 id 注册；事件、动作、出站与身份都按 `source_adapter` 路由。
- 错误码目录与插件协议帧由契约生成；服务端只下发 code 与结构化字段，面向用户的文案归客户端。
- 只保留一个基线 schema、一个配置版本与一个备份清单版本。

### 目标分层

依赖自上而下。同一层内的包可以经接口协作，但不能导入上层；聊天与插件之间只通过 `chat/pipeline` 的投递接口与 `plugin/hostapi` 的动作接口交互。

| 层 | 包 | 职责 |
| --- | --- | --- |
| 入口 | `cmd/raylea-server`、`cmd/raylea-updater`、`api`、`cli` | 解析参数与协议，调用领域服务 |
| 组合根 | `app` | 只负责构造、启动与关闭 |
| 聊天 | `chat`、`chat/adapters`、`chat/onebot11`、`chat/qqofficial`、`chat/pipeline`、`chat/governance`、`chat/menu` | 适配器与消息管线 |
| 插件 | `plugin`、`plugin/process`、`plugin/hostapi`、`plugin/lifecycle`、`plugin/install`、`plugin/datastore`、`plugin/webhook` | 声明、进程与宿主能力 |
| 能力 | `render`、`scheduler`、`thirdparty`、`thirdparty/bilibili`、`thirdparty/weibo`、`thirdparty/douyin`、`thirdparty/netease`、`deps` | 供插件与管理面复用 |
| 运维 | `system`、`recovery`、`update` | 状态、恢复与更新 |
| 平台 | `config`、`store`、`logs`、`httpx`、`task`、`hub`、`version` | 不依赖任何业务包 |

### 现有包的去向

| 现有包 | 去向 | 要点 |
| --- | --- | --- |
| `app` | `app` | 按依赖顺序构造、有序关闭；删除测试缝与 setter |
| `management`、`wsevents` | `api` | 直接序列化领域视图；事件发布回到所属领域 |
| `httpapi` | `httpx` | 加入生成的错误目录与 `Fail` 入口 |
| `chatevent`、`command`、`reconnect` | `chat` | 事件、消息段、身份、命令解析与重连退避 |
| `onebot11`、`qqofficial` | `chat/onebot11`、`chat/qqofficial` | 实现 `chat.Adapter`；协议快照与兼容矩阵随适配器 |
| `app` 中的实例表、`outbound` 路由、`wsevents` 的适配器与入站部分 | `chat/adapters` | 实例注册、路由、身份与热更新 |
| `eventpipeline/chatpolicy`、`bridge`、`dispatch`、`outbound` | `chat/pipeline` | bridge 并入 dispatch；限流与熔断随出站 |
| `permission`、`governance` | `chat/governance` | 黑白名单、命令策略与冷却 |
| `builtinmenu` | `chat/menu` | 经 pipeline 接口发送 |
| `plugins`、`plugins/catalog`、`plugins/artifact` | `plugin` | 声明、发现与 artifact 校验 |
| `plugins/runtime`、`console` | `plugin/process` | 协议帧改为生成；运行态唯一来源 |
| `plugins/actions` | `plugin/hostapi` | 按能力分文件，OneBot 动作经注册表路由 |
| `plugins/lifecycle` 的控制器部分 | `plugin/lifecycle` | 启停、重载与崩溃退避 |
| `plugins/lifecycle` 的安装、卸载、开发同步部分，`pluginmarket` | `plugin/install` | 构造时依赖 lifecycle 接口，不再使用回调 setter |
| `plugins/pluginstore`、`plugins/webhook` | `plugin/datastore`、`plugin/webhook` | 基本原样搬迁 |
| `render/service`、`render/repository` | `render` | 合并为一个包 |
| `integrations/thirdparty`、`integrations/accountvalidation` | `thirdparty` | 各平台以 Provider 接口注入，校验调度并入账号服务 |
| `integrations/bilibili/session`、`integrations/fingerprint`、`integrations/weibo`、`integrations/douyin`、`integrations/netease_music` | `thirdparty/*` | fingerprint 只被 bilibili 使用，合并；删除订阅遗留代码 |
| `system`、`health`、`diagnostics` | `system` | 状态、就绪与诊断导出 |
| `recovery`、`backup` | `recovery` | 只支持当前备份清单版本 |
| `releaseupdate` | `update` | 版本比较移到 `version` |
| `config`、`configruntime`、`runtimepaths` | `config` | 类型化配置与路径推导；扫描根改为普通路径，不再依赖插件包 |
| `storage`、`sqlcgen`、`sqlcqueries`、`secrets`、`filelock` | `store` | 一个基线 schema |
| `logging`、`logpath`、`redact` | `logs` | 固定消息模板，变量进入字段 |
| `tasks`、`pubsub`、`semver` | `task`、`hub`、`version` | `hub` 同时承载领域事件订阅 |
| `deps`、`cli` | `deps`、`cli` | cli 经 `store` 访问数据，不再直接调用 `sql.Open` |
| `testenv` | 测试辅助目录 | 不作为生产包存在 |

### 关键设计决定

#### 适配器端口与注册表

删除 primary 概念的前提是每个实例都能独立寻址。实例在发出事件前自行补全元数据；注册表按实例 id 提供发送、协议动作与状态；`Run` 取代 Start、Stop 与三个处理器 setter。

```go
package chat

type Adapter interface {
	ID() string       // 配置中的实例 id，即事件的 source_adapter
	Protocol() string // "onebot11" 或 "qqofficial"
	Run(ctx context.Context, emit func(context.Context, Event)) error
	Send(ctx context.Context, msg Outbound) (SendResult, error)
	Call(ctx context.Context, action string, params map[string]any) (any, error)
	Status() Status
}
```

插件的 OneBot 动作使用 `registry.Get(event.SourceAdapter)`；主动消息必须指定实例；系统状态汇总全部实例。

#### 组合根按依赖顺序构造

- 构造顺序为平台、能力、聊天、插件、运维，最后是 api 与 cli；每一步只拿到已经构造好的依赖。
- 剩下的两个真实环（进程崩溃与调度触发都要回到生命周期）改为 `hub` 上的事件订阅。
- 每构造成功一个组件就登记一个 closer，失败或退出时逆序关闭，取代 `cleanupPartialBuild`、逐项 nil 判断与重复停止。
- 测试调用同一个构造函数，替身通过参数传入；services harness 与 `Options` 测试缝一并删除。

#### 配置全程类型化

加载流程为：schema 默认值与 `user.yaml` 合并、校验、解码为 `config.Config`。保存后广播新快照，各服务实现 `ApplyConfig(old, new config.Config)` 并返回需要重启的字段；`x-apply-policy` 只用于接口响应。`default.yaml`、map 往返、配置迁移与 secret 键搬迁全部删除。

#### 错误码与文案由契约生成

`go generate` 从 `contracts/error-codes.yaml` 生成 `httpx/errcode`（常量、HTTP 状态、message_key 与默认文案）以及 Web 的错误 locale。handler 只写 `httpx.Fail(w, r, errcode.PlatformResourceMissing, details)`。视图与事件去掉中文标签，改为枚举；管理日志保留中文，但只使用固定模板。

#### 插件协议类型只有一份

从 `contracts/plugin-protocol.schema.json` 生成帧类型，放在 SDK 的 protocol 包；服务端 `go.mod` 使用 replace 指向 `../sdk/go` 直接引用，发布构建沿用仓库内路径。`runtime_model.go` 中的手写帧随之删除。

#### 状态单一来源与测试分层

- 插件运行态只存在于 `plugin/process` 的管理器中；Catalog 保存声明与期望态，读取时投影出 `state`。
- 包内单元测试覆盖边界与并发；组件测试使用真实组合根、临时目录与 SQLite 覆盖装配与跨包流程；契约测试校验真实响应符合 OpenAPI；Web E2E 以真实 Server 为主。

## 分阶段计划

顺序按风险递增：先删除，缩小后续改动面；再把事实收敛到契约；最后搬迁代码。每个阶段结束时，各端测试与真实 Server 的 E2E 都必须通过，每个逻辑变更单独提交。

| 阶段 | 目标 | 覆盖发现 | 状态 |
| --- | --- | --- | --- |
| 0 | 定基线 | D1、D2、D4、X5 | 未开始 |
| 1 | 删除 | X1、X2、X3、X4、R5、R6、T1、T2、T3、T4、T6、G4、S3、S4、C2、H6、Q6 | 未开始 |
| 2 | 收敛到契约 | H1、H2、H3、W1、R1、C1、D3、D5、Q6、X5 | 未开始 |
| 3 | 重整后端 | D1、G1、G3、G5、Q1、Q2、Q3、Q4、Q5、R2、R3、H4、H5、C3、S2、W2 | 未开始 |
| 4 | 客户端收敛 | G2、R4、T5、W2、Q4、Q7、X4 | 未开始 |
| 5 | 文档与门禁 | S1、S2、S5、W3、X5 | 未开始 |

### 阶段 0：定基线

目标：为大规模删除准备安全网。

- 约定破坏性版本号（例如 v0.5.0），暂停新功能。
- 把 golangci-lint 核心检查加入 PR 门禁（X5）；当前结果为 0 个问题，可以直接开启。
- 修复 D1、D2、D4 并补回归测试；D3 与 D5 随阶段 2 解决。

验收：CI 通过；三项缺陷均有回归测试。

### 阶段 1：删除

目标：以最低风险移除不再需要的代码、测试与文档。

- 兼容层：删除配置迁移与 secret 键搬迁、旧密码摘要与迁移跳过检测，合并为单一基线 schema；删除 `bilibili_source_*` 五张表与 nightly 数据迁移演练（X1、X2、R5）。
- 死代码：删除附录中的零引用导出与 2 个 sqlc 查询；只被测试使用的 API 改为在测试内构造，WithContext 双版本收成一个（X3、T2）。
- 测试：删除 services harness、绑定实现细节的测试、启动脚本测试、Python 结构检查与手写 SQL 登记（T1、T3、T4、T6）。
- 代码与脚本：删除不会发生的分支、一次性脚本与雷同示例（G4、Q6、R6）。
- 文档与仓库：删除 `design-qa.md`、事故复盘与重复的存储迁移页，修正失效路径；清理 `.gitignore` 与个人环境痕迹；把 vendored 的 impeccable 移出版本库，提示词另存为文本后删除 `celadon-glass.png`（S3、S4、C2、H6、X4）。

验收：服务端手写代码与测试行数明显下降；各端测试通过。

### 阶段 2：收敛到契约

目标：每个事实只有一个来源。

- 生成错误码目录（Go 与 Web locale），替换 131 处中文错误字面量与各包重复常量，删除错误码语法树测试（H1、W1、D5）。
- 生成插件协议帧类型，服务端与 SDK 共用（R1）。
- 配置默认值只取 schema；删除 `config/default.yaml` 以及 Launcher 与发布脚本对它的依赖（H2、D3）。
- 信任标签等展示文案改为枚举，fixtures 删除中文 label（C1）。
- CI 从 `go.mod` 与 `.tool-versions` 读取版本；doctor 只检查任务所需工具；`package.json` 版本由发布流程写入；契约校验从 OpenAPI 读取路径清单（H3、Q6、X5）。

验收：修改一个错误码或协议字段时，只需修改契约并重新生成。

### 阶段 3：重整后端

目标：按目标分层搬迁代码，每一步保持可编译、测试通过。

- 依次完成：`chat.Adapter` 与注册表（D1、S2）→ 合并 `chat/pipeline` → 拆分 `plugin/*` 并让运行态单一来源（Q3）→ 配置 `ApplyConfig`（Q2）→ `api` 取代 management 与 wsevents（R2）→ 合并平台包（Q5、R3）→ 按依赖顺序重写组合根（Q1、H4、H5）。
- 使用 `git mv` 搬迁，保留文件历史。
- 搬迁时删除必选依赖的 nil 分支、装配 setter、typed nil 规则与重复规范化（G1、G3、G5），拆分超大函数（Q4），日志消息改用固定模板（C3），服务端不再下发中文展示文案（W2）。

验收：内部包约 35 个；组合根没有 `Set*` 调用；非测试代码中没有 `ctx == nil`。

### 阶段 4：客户端收敛

目标：Web 与 Launcher 只消费契约与生成类型。

- Web：文案全部进入 locale；合并 `http.ts` 的两条请求路径与 socket-router 的刷新调度；组件层二选一；拆分千行以上的页面（W2、R4、Q4、Q7）。
- Launcher：Go 侧类型化解码并通过 Wails 绑定输出类型，删除 `server-payload-validation.ts`；只生成用到的 API 类型；收敛 `style.css` 与 AppShell 文件（G2、X4、Q7）。
- E2E 以真实 Server 为主，mock 后端只保留插件 iframe 场景（T5）。

验收：Web 在 locales 之外没有中文；Launcher 不再手写服务端载荷校验。

### 阶段 5：文档与门禁

目标：文档描述意图与边界，门禁自动守住生成物。

- `README.md`、`PRODUCT.md` 与项目章程各负责一类事实；`contracts/README.md` 缩为索引；架构文档按新包结构重写；新增 v0.5 版本说明，已归档的 CHANGELOGS 保持不变（S1、S2、S5、W3）。
- PR 门禁由 lint、生成物漂移、契约校验与各端测试组成（X5）。
- 更新 AGENTS.md：删除 typed nil 与手写 SQL 登记规则，写入新包结构、文案归属与生成物约定。

验收：文档与门禁中不再有手工同步的版本号、路径清单与字段列表。

## 应保留的做法

- 契约优先与 fixtures；`server/tests/integration/openapi_response_contract_test.go` 用真实响应校验 OpenAPI，是契约落地最直接的保障。
- sqlc 生成数据访问代码，手写 SQL 限于 PRAGMA、维护语句与少量动态查询。
- 队列、下载、归档展开、插件包与响应体都有硬上限。
- 插件安装的检查、可信代码确认与事务安装流程，以及 Windows 更新的签名与回滚设计。
- 核心静态检查已经清零；设计 token 从单一源文件生成到 Web 与 Launcher。
- OneBot 的 `access_token_query_compat` 是协议互通选项，不属于历史兼容层，应当保留。

## 附录

Go 路径相对 `server/internal/`，Web 路径相对 `web/src/`。

### 零引用的 Go 导出符号（64）

方法按名称匹配调用，已排除 String、Error、ServeHTTP 等接口方法。

- `app/metrics_registry.go:176` `(MetricsRegistry).PrometheusRegistry`
- `auth/sessions.go:26` `(Manager).Revoke`
- `config/canonical.go:21` `DefaultCooldownReply`
- `configruntime/apply.go:117` `ConfigApplyPolicyHotReload`
- `deps/archive.go:23` `Extract`
- `deps/archive.go:52` `Zip`
- `deps/archive.go:111` `TarGz`
- `deps/download.go:19` `WithProgress`
- `deps/download.go:26` `HTTPSFile`
- `deps/manifest.go:71` `LoadManifestPath`
- `integrations/bilibili/session/session.go:154` `(SessionClient).InvalidateWBI`
- `integrations/bilibili/session/session_cookie.go:80` `MergeCookieValues`
- `integrations/bilibili/session/session_errors.go:20` `ErrorTicket`
- `integrations/bilibili/session/session_errors.go:21` `ErrorDevice`
- `integrations/bilibili/session/session_errors.go:72` `ValidateCookieForLogin`
- `integrations/bilibili/session/session_errors.go:88` `APIError`
- `integrations/bilibili/session/session_errors.go:133` `ClassifyHTTPStatus`
- `integrations/bilibili/session/session_errors.go:154` `IsAuthError`
- `integrations/bilibili/session/session_errors.go:163` `IsRiskControlError`
- `integrations/bilibili/session/session_errors.go:175` `IsRiskControlErrorText`
- `integrations/bilibili/session/session_errors.go:187` `ShouldRetryWBI`
- `integrations/bilibili/session/session_wbi.go:76` `IsBilibiliURLForWBI`
- `integrations/fingerprint/device.go:47` `GenDeviceID`
- `integrations/fingerprint/device.go:56` `GenRandomHex`
- `integrations/netease_music/qrcode_login.go:264` `HasLoginCookie`
- `integrations/netease_music/user_resolve.go:15` `ResolveUser`
- `integrations/thirdparty/accounts.go:386` `(Service).UpdateCookie`
- `integrations/thirdparty/accounts.go:517` `(Service).UpdateCredentialStatus`
- `integrations/thirdparty/errors.go:14` `ErrorCSRF`
- `integrations/thirdparty/errors.go:18` `ErrorSignature`
- `integrations/thirdparty/errors.go:73` `IsRiskControlError`
- `integrations/thirdparty/errors.go:78` `IsRateLimitError`
- `integrations/thirdparty/qr_login_service.go:357` `CloseQRLoginProviderSession`
- `integrations/weibo/qrcode_login.go:186` `HasLoginCookie`
- `integrations/weibo/user_resolve.go:26` `ResolveUser`
- `logging/logger.go:55` `New`
- `logging/logger.go:62` `NewWithStream`
- `onebot11/api_values.go:30` `ExtractStringField`
- `onebot11/api_values.go:57` `NormalizeAPIList`
- `onebot11/api_values.go:88` `NormalizeAPIResult`
- `onebot11/api_values.go:124` `ExtractStringValue`
- `onebot11/intake.go:311` `FrameEcho`
- `onebot11/intake.go:323` `FrameStatusText`
- `plugins/actions/onebot_registry.go:57` `IsOneBotLocalAction`
- `plugins/actions/onebot_registry.go:62` `IsOneBotProviderExtensionAction`
- `plugins/artifact/artifact.go:22` `ProtocolVersion`
- `plugins/artifact/artifact.go:23` `BridgeVersion`
- `plugins/lifecycle/controller.go:513` `(Controller).StopAndResetPlugin`
- `plugins/lifecycle/controller.go:640` `PluginRuntimeSuperAdmins`
- `plugins/lifecycle/controller.go:840` `SchedulerPluginDisplayName`
- `plugins/plugin_model.go:323` `CloneSettingValue`
- `plugins/summary_view.go:175` `BuildHelpView`
- `pubsub/hub.go:16` `NewHub`
- `recovery/manifest.go:101` `ScanRepoPaths`
- `recovery/summary.go:44` `RemoveSummary`
- `recovery/summary.go:52` `HasSummary`
- `recovery/summary.go:57` `LoadSummaryFromFile`
- `recovery/summary.go:72` `SaveSummaryToFile`
- `recovery/summary.go:83` `AvailableRecoveryLogFiles`
- `render/service/service_catalog.go:86` `(Service).GetTemplatePreviewData`
- `scheduler/scheduler.go:100` `FormatDuration`
- `scheduler/scheduler.go:438` `(Engine).UnregisterByPlugin`
- `storage/store.go:38` `WithBusyTimeout`
- `system/startup_runtime_prepare.go:111` `(Service).SetStartupRuntimeState`

### 只被测试引用的 Go 导出符号（38）

括号内为测试中的引用次数。

- `app/app.go:62` `New`（10）
- `auth/bootstrap.go:102` `(Manager).Login`（16）
- `auth/manager.go:117` `NewManager`（13）
- `auth/sessions.go:10` `(Manager).Issue`（16）
- `config/document.go:10` `LoadDocument`（10）
- `configruntime/metadata.go:65` `ConfigFieldMetadataPaths`（1）
- `deps/manager_prepare.go:24` `SetSystemChromiumFinderForTest`（1）
- `deps/manifest.go:79` `ManifestPlatform`（2）
- `eventpipeline/bridge/observability_publish.go:27` `(Bridge).ObservabilitySubscriberCount`（2）
- `eventpipeline/chatpolicy/ingress.go:106` `(Ingress).SetMetadataEnricher`（1）
- `eventpipeline/chatpolicy/ingress.go:118` `(Ingress).Policy`（4）
- `eventpipeline/dispatch/command_registry.go:85` `(Dispatcher).HasPlugin`（1）
- `httpapi/client_ip.go:154` `RequestPeerIP`（1）
- `httpapi/client_ip.go:164` `RequestUsesTrustedProxy`（2）
- `integrations/bilibili/session/captcha.go:54` `NewCaptchaClient`（1）
- `integrations/bilibili/session/captcha.go:65` `(CaptchaClient).TrySolve`（1）
- `integrations/bilibili/session/identity_provider.go:85` `(IdentityProvider).WithFixedUA`（1）
- `integrations/bilibili/session/identity_provider.go:118` `(IdentityProvider).JitteredDelay`（3）
- `integrations/bilibili/session/identity_provider.go:156` `(IdentityProvider).ApplyLiveHeaders`（1）
- `integrations/bilibili/session/session.go:82` `NewSessionClient`（3）
- `integrations/bilibili/session/session.go:103` `(SessionClient).PrepareCookie`（2）
- `integrations/bilibili/session/session.go:122` `(SessionClient).SignURL`（1）
- `integrations/fingerprint/device.go:37` `GetDmImg`（1）
- `logging/spool.go:53` `(SpoolQueue).QuarantinePath`（1）
- `management/auth.go:235` `RequireAuth`（8）
- `management/governance.go:20` `NewGovernanceHandlers`（1）
- `management/system_handlers.go:150` `NewSchedulerHandlers`（5）
- `onebot11/cache.go:54` `(IdentityCache).Clear`（1）
- `onebot11/cache.go:88` `(IdentityCache).GetLogin`（3）
- `onebot11/cache.go:99` `(IdentityCache).SetLogin`（2）
- `onebot11/shell.go:93` `NewForTest`（2）
- `permission/checker.go:273` `(CooldownTracker).Cleanup`（97）
- `plugins/actions/dispatch.go:22` `(Registry).Kinds`（1）
- `plugins/runtime/manager_control.go:135` `(Manager).Ping`（4）
- `render/service/artifact.go:92` `(Service).LookupArtifact`（1）
- `scheduler/scheduler.go:369` `(Engine).UpsertTask`（6）
- `scheduler/scheduler.go:424` `(Engine).Unregister`（2）
- `system/startup_runtime_prepare.go:107` `(Service).StartupRuntimeState`（1）

### 未调用的 sqlc 查询（2）

- `CountNamespace`
- `GetPluginStoreSource`

### 零引用的 Web 导出（15）

- `lib/display.ts:77` `getBooleanLabel`
- `lib/exception-status.ts:6` `resolveExceptionStatus`
- `lib/management-links.ts:286` `buildProtocolWorkbenchActions`
- `lib/management-links.ts:305` `buildDashboardProtocolActions`
- `lib/management-summary.ts:27` `formatProtocolIssueSummary`
- `lib/management-summary.ts:35` `formatProtocolEventSummary`
- `lib/plugin-commands.ts:51` `flattenPluginCommands`
- `lib/protocols.ts:3` `ONEBOT11_PROTOCOL`
- `lib/protocols.ts:24` `isProtocolIssue`
- `lib/protocols.ts:33` `isProtocolEvent`
- `lib/text-safety.ts:9` `toSafeDisplayText`
- `motion/runtime.ts:43` `supportsViewTransitions`
- `stores/log-state.ts:120` `mergeLogItemsAsc`
- `types/plugins.ts:6` `PluginTrustLevel`
- `types/plugins.ts:7` `PluginInstallSourceType`

### 认知复杂度最高的函数

| 函数 | 位置 | 复杂度 |
| --- | --- | ---: |
| `(*Dispatcher).worker` | `eventpipeline/dispatch/worker.go:17` | 108 |
| `(*ChromedpBrowser).Poll` | `integrations/douyin/browser.go:362` | 74 |
| `(*InstallService).runInstall` | `plugins/lifecycle/install.go:774` | 68 |
| `Create` | `backup/archive.go:43` | 67 |
| `(*InstallService).prepareSource` | `plugins/lifecycle/install.go:1011` | 60 |
| `extractZipSource` | `plugins/lifecycle/install.go:1264` | 54 |
| `(*Downloader).Download` | `releaseupdate/download.go:37` | 54 |
| `(*Shell).callAPIAnyOnTransport` | `onebot11/shell_api.go:110` | 53 |
| `(*Installer).Install` | `releaseupdate/transaction.go:76` | 51 |
| `ExtractWindowsArtifact` | `releaseupdate/archive.go:20` | 48 |
| `RequireAuthWithConfig` | `management/auth.go:242` | 47 |
| `(*EventsHandler).streamEventsWebSocket` | `management/ws_handlers.go:171` | 45 |

### 重复次数最多的字符串字面量

goconst 按包统计，表中为单个包内的最大出现次数。

| 字面量 | 出现次数 |
| --- | ---: |
| `plugin.internal_error` | 35 |
| `platform.resource_missing` | 24 |
| `chromium` | 17 |
| `plugin.permission_denied` | 16 |
| `plugin.protocol_violation` | 15 |
| `platform.invalid_request` | 14 |
| `enabled` | 13 |
| `text` | 12 |
| `succeeded` | 11 |
| `unsupported` | 10 |
