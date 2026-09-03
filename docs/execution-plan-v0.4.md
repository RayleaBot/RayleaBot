# RayleaBot v0.4 插件体系精简执行计划

## 文档状态

- 目标版本：v0.4
- 执行范围：RayleaBot 主仓库与 `RayleaBotPlugins/plugin-catalog`
- 升级方式：一次性破坏性升级，不保留旧插件合同兼容层或插件包转换器
- 当前状态：实现与复验完成，待维护者验收
- 验收方式：全部工作完成后保留本文档，由项目维护者逐项验收

状态说明：

- `⬜ 待处理`：尚未开始。
- `🟡 进行中`：已经开始，但尚未满足完成条件。
- `☑️ 已完成`：实现、配套更新和本项验证均已完成。
- `❌ 阻塞`：存在无法在当前范围内消除的阻塞，必须记录原因。

## 固定决策

1. 同时升级插件 manifest v3、插件协议 v2、artifact v2、商店 catalog v2、开发工作区 v2、管理桥接 v3、备份 manifest v3 和 CLI contract v2。
2. 插件运行时只要求当前平台可执行文件，不绑定 Go；Go SDK 和 Go 构建器仍作为一等开发工具提供。
3. 事件订阅由 manifest 的 `events` 静态声明；空数组表示不接收任何普通事件。
4. 完全删除 `http_hosts`。宿主 HTTP 仍统一执行 HTTPS、DNS 与重定向复查、SSRF/私网限制、超时和响应体大小限制。
5. 插件私有日志、配置、KV 和文件能力默认可用；跨系统或高权限能力必须在 `permissions` 中声明。
6. `capabilities` 与 `capability_parameters` 合并为一个 `permissions` 对象，权限参数与作用域在对应权限下声明。
7. 旧 manifest v2、protocol v1 和 artifact v1 不再运行。现有插件包保留但标记为合同版本不受支持，插件设置、密钥、KV、文件和公开数据不清除。
8. 备份 manifest 升级到 v3，拒绝恢复 v2 备份；v3 备份允许收录已经失效的旧 v2 插件包并恢复其数据。
9. `management_ui.entry` 只声明一次，页面只保留 `id` 和 `label`。
10. 默认配置只允许内联 `default_config`，删除 `default_config_file`。
11. 删除 manifest 的 `render_templates`，宿主从 `templates/*/template.json` 自动发现模板。
12. webhook 在 manifest 中完整静态声明并由宿主自动注册，删除运行时 `event.expose_webhook`。
13. 命令统一为 `commands` 数组。每条命令具有稳定 ID 和 exact、pattern 或 setting 触发器；`command_groups` 只能引用真实命令；帮助内容从命令与分组生成，不能再塞入无对应指令的项目。
14. SDK 从宿主 init 获取插件 ID，不再要求插件手工配置 ID、订阅事件或最大并发数。
15. 新增统一的 `raylea-plugin` 开发工具，提供检查、通用打包和 Go 构建能力，并整合现有开发同步流程。
16. 保留 `RayleaBot/plugin-catalog` 作为默认官方商店目录；目录从插件当前 GitHub Release 自动生成，不维护独立签名或历史版本。
17. 官方目录固定为默认来源，管理员可增加自定义 HTTPS 目录；每个来源持久化最后成功缓存，目录不维护独立签名或公钥注册表。

## 合同版本矩阵

| 表面 | 目标版本 | 兼容策略 |
| --- | ---: | --- |
| plugin-info | 3 | 仅接受 v3 |
| plugin-protocol | 2 | 仅接受 v2 |
| plugin-artifact | 2 | 仅接受 v2 |
| plugin-store-catalog | 2 | 仅接受 v2 |
| plugin-development-workspace | 2 | 仅接受 v2 |
| plugin-management-ui / bridge | 3 | 仅接受 v3 |
| backup-manifest | 3 | 仅接受 v3 |
| CLI contract | 2 | 仅维护 v2 命令面 |

## 执行清单

| ID | 工作项 | 状态 | 完成情况 | 验证证据 |
| --- | --- | --- | --- | --- |
| P0 | 固化 v0.4 执行与验收文档 | ☑️ 已完成 | 已记录范围、固定决策、合同矩阵、实施顺序、验证矩阵和验收区。本文档在执行期间逐项回写，完成后保留。 | 文档结构检查通过。 |
| P1 | 合同纪元与 schema | ☑️ 已完成 | 已升级 manifest v3、protocol v2、artifact v2、catalog v2、workspace v2、bridge v3、backup v3 和 CLI v2；已删除旧合同字段，重建合同 fixtures，迁移主仓库示例 manifest，并同步 Server 嵌入 schema、Vue SDK bridge 类型、Web/Launcher OpenAPI 类型和 WebSocket 类型。 | `validate_contracts.py --self-test`、PR 模式和 strict 模式均通过；runtime schema verify 通过。 |
| P2 | Artifact 与统一开发工具 | ☑️ 已完成 | 已实现无语言字段、无文件角色和无重复插件身份的 artifact v2；新增 `raylea-plugin inspect/pack/build-go`；`inspect` 分别检查项目 manifest 或展开后的 artifact，`pack` 可接收任意已构建的目标平台原生可执行文件；Go 构建自动推导 `cmd/<plugin-id>`、ID 后缀或唯一 cmd 入口，并复用 UI、notices、SBOM、模板与资源流程；workspace v2 从 `info.json` 推导 ID，非 Go 项目使用约定产物路径，临时 go.work 只纳入存在 go.mod 的项目；dev-sync 已删除 `--plugin-id`。 | Go SDK/pluginbuild 全量测试、三平台 artifact 构建检查、Server artifact 包测试、workspace/watch 脚本测试和真实 dev-sync 安装回归通过；合同 strict 与 runtime schema verify 通过。 |
| P3 | 服务端插件发现、安装与恢复 | ☑️ 已完成 | manifest 先经 schema 校验再进入类型化解析；所有安装来源统一校验 `min_core_version`；artifact v2、静态模板发现和静态 webhook 注册已接入；默认官方目录、自定义来源和最后成功缓存已持久化；backup v3 拒绝 v2，同时允许保留旧包事实和持久数据。 | 插件 catalog/artifact/lifecycle/webhook、pluginmarket、recovery、CLI 测试通过；新增最低 Core 版本拒绝测试。 |
| P4 | Protocol v2 与 SDK | ☑️ 已完成 | init 独占协议版本与插件身份并携带完整配置、权限和并发信息；后续帧删除重复身份和时间戳；`config.changed` 原子替换完整配置快照；空 `events` 不接收普通事件；删除 `config.read`、动态 webhook 和外部 `message.reply`；插件私有日志、配置、KV、文件改为隐式命名空间能力；Go/Vue SDK 已同步。 | Server runtime/actions/dispatch 测试、Go SDK 全量测试、Vue SDK typecheck/test/build 通过；新增 v1 envelope 拒绝、空事件订阅和配置快照隔离测试。 |
| P5 | 管理桥接、Web 与 Launcher | ☑️ 已完成 | bridge v3、单入口多页面、permissions、稳定命令 ID、统一触发器、真实命令分组、静态 webhook 与插件源管理已完整接入管理 API、Web 与 Launcher。OpenAPI、WebSocket 和 bridge 类型均已再生成。 | Web 非增量 typecheck、62 文件 312 项全量测试、插件商店 Playwright 回归和 production build 通过；Launcher typecheck、16 文件 64 项测试、Go 测试和完整 package build 通过。 |
| P6 | 示例、文档与废弃面清理 | ☑️ 已完成 | 主仓库示例已统一为 manifest v3 / protocol v2 / artifact v2，并将权限参数示例改名；用户、架构、开发、管理、恢复与发布文档均改为最终态。已删除每插件 `tools/build`、`pluginbuild/buildcmd`、破坏性 epoch reset/backup 脚本及其测试，CI recovery fixture 改用统一工具；运行时测试 helper 不再转译 v1 envelope；历史 changelog 与 v0.3 计划保持不变。 | 示例 manifest 集成测试、开发工作区 39 项脚本测试、release 71 项测试、文档 131 文件链接检查和 agent docs 检查通过；废弃引用定向扫描无非预期命中。 |
| P7 | 全量验证与最终 review | ☑️ 已完成 | 已完成合同、生成物、服务端、SDK、Web、Launcher、统一开发工具、三平台 artifact、旧合同拒绝与数据保留、backup v3/v2、真实 dev-sync、文档和完整 diff 对照。三平台回归额外发现并修复 Windows 交叉构建 Unix artifact 时展开目录权限位误判，ZIP 入口继续强制记录 `0755`。 | 全部适用验证通过；Go `-race` 因当前 Windows Go 环境 `CGO_ENABLED=0` 不适用，常规并发与全量测试通过。最终生成物 verify、doctor、文档/agent 检查、废弃引用扫描、敏感信息定向扫描和 `git diff --check` 通过。 |
| P8 | 官方商店目录与来源边界校正 | ☑️ 已完成 | 保留 `RayleaBot/plugin-catalog` 默认官方目录和远端插件下载流程；目录签名、公钥注入、bootstrap 与历史版本清单已移除；自定义 HTTPS 来源、缓存和必要的安装确认已接入。 | pluginmarket/lifecycle 测试、合同 self-test/strict、runtime schema verify、release YAML 解析、Server 构建、文档链接和定向 `git diff --check` 通过。 |
| P9 | v0.4 评审修复 | ☑️ 已完成 | 7 个 major 与同轮 minor 已按 [`execution-plan-v0.4-review-fixes.md`](./execution-plan-v0.4-review-fixes.md) 完成。 | 修复计划 F1 至 F8 均已通过验证。 |
| P10 | 插件商店收敛 | ☑️ 已完成 | catalog、artifact、来源缓存、安装确认、Web 商店和官方目录自动生成均已按 [`execution-plan-v0.4-store-simplification.md`](./execution-plan-v0.4-store-simplification.md) 收敛。Go SDK v0.4.0 与六个官方插件的三平台 artifact v2 已发布；商店详情后端按维护者要求保留。 | S1 至 S9 全部通过，当前无阻塞项。 |

## 实施顺序与完成条件

### P1 合同纪元与 schema

先冻结新的对外边界，再允许实现引用。完成条件：

- 所有目标合同只表达新版本语义，没有旧字段兼容分支。
- manifest v3 保留 `id`、`name`、`version`、`license`、必填 `min_core_version`、metadata、concurrency、events、permissions、default_config、commands、command_groups、help、management_ui、webhooks。
- manifest 删除 runtime、entry、platforms、plugin_protocol_version、data_schema_version、default_config_file、capabilities、capability_parameters、http_hosts、storage_roots、command_patterns、dynamic_commands、render_templates。
- artifact v2 只记录 artifact_version、target_platform 和 entry；插件身份与版本来自 `info.json`，安装器扫描实际文件。
- fixtures、examples、生成代码和严格合同校验全部同步。

### P2 Artifact 与统一开发工具

完成条件：

- `raylea-plugin inspect` 能校验项目和产物。
- `raylea-plugin pack` 能打包任意已构建的原生可执行文件。
- `raylea-plugin build-go` 复用现有 Go、Vue、notices、SBOM 流程后调用统一打包器。
- artifact 只声明目标平台和入口，安装器扫描真实文件并检查入口为目标平台可执行格式。
- 开发工作区从 `info.json` 推导 ID；启动开发环境不再依赖每个插件自带的重复构建包装器。

### P3 服务端插件发现、安装与恢复

完成条件：

- manifest 先通过 schema，再进入类型化解析；删除零散字段提取 helper。
- 所有来源安装都执行 `min_core_version` 校验。
- 旧包显示明确的不受支持原因，不删除插件持久数据。
- 模板和 webhook 按静态声明自动装载；运行期不再动态暴露 webhook。
- catalog v2 每个插件只记录当前 release，asset 只描述平台、URL 和归档 hash。
- 默认官方远程 catalog、自定义 HTTPS 来源和每个来源的最后成功缓存均由 Plugin Store Service 管理。
- v3 备份能携带旧包事实版本；恢复旧包时保持失效状态并恢复其持久数据。

### P4 Protocol v2 与 SDK

完成条件：

- 只有 init 帧携带协议版本和插件 ID，后续帧不重复这两项，也不重复 envelope 时间戳。
- init 提供完整配置快照、生效权限、Bot、管理员、前缀和并发配置。
- `config.changed` 提供完整快照和 changed_keys。
- 插件私有日志、配置写入、KV 和文件访问不要求 manifest 权限声明，但仍按调用插件命名空间隔离。
- SDK 删除 PluginID、Subscriptions 和 MaxConcurrentHandlers 配置项，使用原子配置快照。
- action/event 常量与类型由合同生成或由单一来源维护。

### P5 管理桥接、Web 与 Launcher

完成条件：

- 管理页面共用 manifest 单一入口，页面 ID 作为路由上下文传递。
- 保留隔离 origin、CSP、nonce、MessagePort 和密钥保护。
- 管理详情不再展示 runtime、entry、platforms、data schema、default config 等无意义信息。
- 权限、命令 ID、触发方式和命令分组与合同一致。
- Web 与 Launcher 的生成类型、类型检查、测试和构建同步通过。

### P6 示例、文档与废弃面清理

完成条件：

- 主仓库示例只展示新合同和统一工具链。
- 工程基线、实现顺序、插件开发、管理、恢复、CLI 和发布文档均描述最终态。
- 删除不再可达的兼容代码、旧 fixture、旧 schema、破坏性重置脚本和对应测试。
- 外部业务插件仓库保持不变；只同步官方 `plugin-catalog` 的目录生成流程。

### P7 全量验证与最终 review

完成条件：

- 运行验证矩阵中所有适用命令并记录结果。
- 对新旧包、数据保留、备份恢复、开发同步和管理页面执行行为验证。
- 检查未提交改动、生成物、文档链接、敏感信息和 `git diff --check`。
- 本文档所有工作项均有明确最终状态、完成情况和验证证据。

### P8 官方商店目录与信任边界校正

完成条件：

- Server 默认从 `RayleaBot/plugin-catalog` 获取官方静态目录；Web 继续通过 `/api/plugin-store/**` 使用商店，不直连目录或插件资产。
- 官方目录使用 HTTPS、持久化最后成功缓存与归档 SHA-256；核心更新元数据继续使用独立 Ed25519 公钥注册表。
- 官方目录可收录独立插件仓库的 HTTPS 产物；主程序发布包不携带业务插件，非官方手动安装入口继续可用。
- 商店文档使用 catalog v2、manifest v3、artifact v2 和 workspace v2 的最终语义，不再描述固定内置插件或旧合同。

## 验证矩阵

| 验证面 | 计划命令或行为 | 状态 | 结果 |
| --- | --- | --- | --- |
| 合同自检 | `python scripts/ci/validate_contracts.py --self-test` | ☑️ 通过 | 合同校验器自检通过。 |
| 合同严格校验 | `python scripts/ci/validate_contracts.py --mode strict` | ☑️ 通过 | 新合同、fixtures、CLI 语义、OpenAPI 和 WebSocket 严格校验通过。 |
| 生成物漂移 | 运行 schema、OpenAPI、SDK 生成与 check 模式 | ☑️ 通过 | Server 嵌入 schema、Vue/Web bridge、Web/Launcher OpenAPI、WebSocket 类型和 Launcher 原生图标均已生成并通过 verify/check。 |
| Server | 受影响包测试、`go test ./...`、Server build | ☑️ 通过 | Server 全仓测试和 `raylea-server.exe` 构建通过；新增 backup v2 精确拒绝与真实 dev-sync 安装回归通过。 |
| Go SDK | 单元测试，适用时运行 race | ☑️ 通过 | `go test ./...` 通过，覆盖 protocol v2 并发动作、终态、panic 隔离、原子配置快照、项目检查和三平台构建；`-race` 因当前环境禁用 CGO 不适用。 |
| Vue SDK | 类型检查、测试、构建 | ☑️ 通过 | bridge v3 生成类型、`pnpm run typecheck`、`pnpm test`、`pnpm run build` 通过。 |
| raylea-plugin | inspect、pack、build-go 及错误路径 | ☑️ 通过 | pluginbuild 单测覆盖项目 manifest 检查、Go 构建、原生 pack、artifact 扫描、目标格式和错误路径；`inspect --plugin` 实际命令回归通过。 |
| Artifact | Windows、Linux、macOS 目标产物结构校验 | ☑️ 通过 | 同一 Go fixture 完成 Windows x64、Linux x64、macOS arm64 交叉构建；逐目标检查原生格式、实际文件扫描和 ZIP 入口 `0755`。 |
| 旧插件包 | v2 包被拒绝，设置、密钥、KV、文件与公开数据保留 | ☑️ 通过 | manifest v2、protocol v1、artifact v1 仅作为负向输入被拒绝；backup v3 允许旧插件事实并给出 warning，通用恢复往返验证 data/插件状态文件保持不变。 |
| 备份恢复 | v3 往返、v2 拒绝、含旧插件包的 v3 备份恢复 | ☑️ 通过 | recovery 与 CLI 测试覆盖 v3、v2 拒绝及含旧插件事实的恢复。 |
| 开发流 | workspace、sync/watch、ID 推导与无 go.mod 场景 | ☑️ 通过 | workspace v2、ID 推导、非 Go 项目、go.work 过滤、watch 队列与 Vue SDK 镜像测试通过；真实 `plugin dev-sync` 从 artifact 推导 ID、安装包、写入 development metadata 并设为启用的回归通过。 |
| Web | `pnpm run typecheck`、`pnpm test`、`pnpm build` | ☑️ 通过 | 非增量 `vue-tsc` 通过；62 个测试文件 312 项测试、插件商店桌面与窄屏 Playwright 流程和 production build 通过。 |
| Launcher | `pnpm run typecheck`、`pnpm test`、`pnpm build` | ☑️ 通过 | typecheck、16 个测试文件 64 项测试、Go 平台测试和完整 package build 通过；原生图标摘要已再生成并通过构建门禁。 |
| 官方插件目录 | 来源缓存与 GitHub Release 自动同步 | ☑️ 通过 | 目录签名与目录公钥已移除；官方 catalog 已收录六个插件的当前 Release 和三平台 artifact v2；Server 持久化每个来源最后成功目录；catalog、同步脚本与 workflow 均通过校验。 |
| 文档与仓库 | agent docs、链接、doctor、`git diff --check` | ☑️ 通过 | agent docs、133 个 Markdown 文件链接、Server structure、固定 Node/Go/Python/pnpm 工具链 doctor、废弃引用、敏感信息定向扫描和 `git diff --check` 均通过。 |

## 验收区

- 最终实现状态：v0.4 插件体系与插件商店收敛完成。
- 未完成或阻塞项：无。商店详情后端按维护者决定暂时保留，不属于未完成项。
- 外部插件同步：六个官方插件仓库、Go SDK v0.4.0 标签和 `plugin-catalog` 均已发布并同步。
- 维护者验收：待柒柒验收。
