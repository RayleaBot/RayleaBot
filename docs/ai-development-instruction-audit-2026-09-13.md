# RayleaBot AI 开发指令与环境限制审计（2026-09-13）

> **状态：本次任务的审计交付，不是开发指令。** 当前结论以 `788948646daec8b71f5501c96d075b3fccc57f66` 为基准；首次审计基准为 `1913147c`，不能混用两个版本的发现。本文未被 README 或目录索引收录，后续任务不应把它当作常规开发入口。仓库或 skill 变化后需重新核对；若长期保留，应纳入审计索引或归档，不将正文复制进 AGENTS。

本报告覆盖仓库文档要求，也检查当前会话提供的项目级、用户安装、系统内置及插件 skill。**正文优先处理与 RayleaBot 开发直接相关的问题；全局工具的条件性风险放在附录，不参与仓库整改优先级。** 只修改本报告，未修改业务代码、指令、配置或旧审计。

## 结论

当前仓库规则整体已较克制，没有发现需要整体重写 AGENTS 或削弱安全边界的依据。值得先改的是三个局部问题：

1. **F8：契约验证表与按影响选择生成链的规则不一致。** 最小验证表仍无条件列出四个生成器。
2. **F9：Launcher 验证表没有区分 renderer 与 Go 改动。** 现有 typecheck / test 会同时执行 Go 检查；只需先明确按改动面选择，不以新增工程脚本为整改前提。
3. **F11：实施顺序中的两条配套更新要求缺少影响条件。** 数据库和协议变更不应无条件制造 queries、恢复说明、SDK 或示例的改动。

F12 的上游 skill 行数预算、F13 的审计快照时效属于低优先级维护问题。F4、F5 是使用相应 UI 工具时才需注意的流程风险；F6 没有形成当前项目冲突；F7 没有硬拒绝证据；F10 的 Makefile 重检查已有日常命令可避开，均不应与前三项并列。

原 F1、F2、F3、F14 的外部 skill 条款属实，但不构成 RayleaBot 日常开发的首要问题。当前源码和工程依赖检索未发现直接调用 OpenAI API 的实现；不能仅凭“机器人 / AI 项目”将密钥申请流程纳入仓库开发。

以下沿用首次报告的 F / D 编号，方便定位。优先级依据本仓库实际触发频率、影响范围和既有替代入口判断；“可能导致”表示尚未通过任务回放证实的推断。

## 1. 当前仍成立的仓库问题

### F8 · 中：契约最小验证表无条件列出全部生成链

**证据**：[quality-gates](./engineering/quality-gates.md)第 21 行，同时列出 runtime schemas、error codes、plugin wire、Launcher API 四个 verify 和 strict validator。项目 [contract-audit](../.agents/skills/contract-audit/SKILL.md)第 19 行及[实施顺序](./engineering/implementation-order.md)第 28 行明确只检查受影响生成链。

生成器输入并不相同：`scripts/generate-error-codes.py` 第 76 行读取错误码目录，`scripts/generate-launcher-api.py` 第 24 行读取 OpenAPI，`scripts/generate-plugin-wire.py` 第 290–291 行读取协议和配置 schema。单个 fixture 修正不一定改变四条生成链。

**影响**：AI 可能为满足“最小验证”表而执行无关检查，或者把任一无关生成器的问题视为当前任务阻塞。这是已确认的规则范围不一致，不是四个生成器本身存在缺陷。

**最小建议**：保留必要契约校验；把生成器命令改为按输入和实际依赖选择。若全量 verify 被有意保留为廉价兜底，应写清它与必需检查的区别。无需新增另一份完整生成物清单。

### F9 · 中：Launcher 验证表没有区分 renderer 与 Go

**证据**：quality-gates 第 22–23 行统一列出 Web / Launcher 的 typecheck 和 test。[launcher/package.json](../launcher/package.json) 的 typecheck 同时执行 TypeScript 检查与 Go vet；test 同时执行 Vitest 与全部 Go 测试。

**影响**：纯 renderer 样式改动按该行照做，会扩大到 Go 检查。Web 普通文案或 CSS 修改也需要按风险判断，而不是为了表格补造测试。共享组件行为、类型和构建变化则可能合理地需要更广验证。

**最小建议**：在表中说明纯展示、renderer 逻辑、Go host / bridge 按实际改动分别选择现有检查；不要求所有任务先执行组合命令。不以新增脚本、拆分 package scripts 或新建测试作为本项整改前提。

### F11 · 中：实施顺序的两条配套清单缺少影响条件

**证据**：实施顺序第 44 行要求数据库结构变更先更新当前 schema、queries、fixtures 和恢复说明；第 61 行要求协议扩展同步 schema、fixtures、SDK 和示例插件。

**影响**：例如只增加索引，也可能被理解为必须修改查询和恢复文档；不影响某个 SDK 或示例行为的协议修改，也可能制造无意义 diff。根 [AGENTS](../AGENTS.md)第 17 行已有“按实际影响”的范围约束。

**最小建议**：给这两句补足实际影响条件，保留真实变化涉及的 schema、生成物、样例和消费端同步义务。

**不纳入本项**：[baseline](./engineering/baseline.md)第 13 行的“再同步全部 companion”承接“来源之间发生冲突时”和“在冲突所属领域作出决定”。它指向该冲突涉及的配套，不能脱离上下文视为每次修改都必须更新全仓。原报告对这一句的解读过宽。

### F12 · 低：上游 skill 与项目自有 skill 共用行数预算

**证据**：[check-agent-docs.mjs](../scripts/check-agent-docs.mjs)第 50–53、138–144 行，将仓库枚举到的 `.agents/skills/**/SKILL.md` 都纳入 100 行预算。上游 Impeccable 当前入口为 82 行；根 AGENTS 第 11 行同时要求其由上游维护。

**影响**：未来上游入口超过预算，可能出现“修改上游正文”和“通过项目门禁”的维护矛盾。当前门禁通过，没有实际升级阻塞。

**建议**：维护检查器或升级该 skill 时，区分项目自有规则与上游副本的预算；保留必要结构和引用检查。不为尚未发生的超限立即修改上游文件。

### F13 · 低：审计快照存在过期检索和入口缺失问题

**证据**：旧审计前部保留首次发现，第 165 行起列处理记录。只读取搜索片段，仍可能命中“没有最小验证表”“hook 入口不存在”等已处理描述。当前 README 与目录索引没有引用旧报告或本报告。

**影响**：这是快照时效和可发现性问题，不能据此认定 AI 已实际重复修复。只读片段的风险也适用于本报告自身。

**本报告的处理**：首部列当前基准、首次基准、用途和重新核对条件；正文给出当前结论，已解决事项集中列出，不继续保留原本偏高的结论作为活动待办。

**后续建议**：如果这些报告需要长期保存，再建立审计索引或归档入口；如果只是任务临时交付，不将其列为正式开发来源。未被索引本身不是指令违规，也不值得为此扩大到整套文档治理重构。

## 2. 重复项与已经收敛的部分

同一事实服务不同读者，不等于有害重复。以下八组主要用于判断责任归属，不是八项必须删除的缺陷。

| 编号 | 来源与内容 | 当前判断 |
| --- | --- | --- |
| D1 | 根 AGENTS 第 15–17 行；[contracts/README](../contracts/README.md)第 125–128 行；[设计入口](./design/README.md)第 5 行；[Server README](../server/README.md)第 13 行：契约优先 | 根规则持有开发动作要求，各读者入口保留简短正式来源指针即可。当前不需要继续全仓去重。 |
| D2 | 根 AGENTS 第 18 行；[平台架构](./architecture/platform-architecture.md)第 61、98 行；实施顺序第 76 行；Launcher AGENTS 第 7、22 行：状态和职责归属 | 架构解释、局部应用及例外有各自职责。保留必要数据流，不机械删除相同事实。 |
| D3 | 根 AGENTS 第 22 行；Server AGENTS 第 17 行；实施顺序第 38、51 行：共享状态同步 | 根规则覆盖 Server 和 Launcher；热更新说明负责具体场景。保留根并发规则的决定有源码依据。 |
| D4 | [Contracts AGENTS](../contracts/AGENTS.md)第 25–31 行；contract-audit 第 24 行；[fixtures/README](../fixtures/README.md)第 75–76、91–94 行：配套登记 | `9fbb365c` 已集中登记入口，skill 已改为引用该入口。文件编写细节仍留 fixtures / examples；不再报告缺少统一入口。 |
| D5 | [Docs AGENTS](./AGENTS.md)第 8 行；[editing-final-state-content](../.agents/skills/editing-final-state-content/SKILL.md)第 12–23 行：文本取舍 | Docs 说明文档职责，skill 细化编辑判断；低风险摘要重复，不需要再拆规则。 |
| D6 | [DESIGN](../DESIGN.md)、[PRODUCT](../PRODUCT.md)、界面规范与 Impeccable：视觉和无障碍 | `4bd6da5b` 已将重复的界面尺寸、构成和材质参数归到 Web 规范，DESIGN 保留共享规则和引用。反例与具体页面规则可继续并存。 |
| D7 | baseline 第 25–43 行、`.tool-versions`、工程声明：版本值 | 工程必填声明属于必要副本，已有一致性检查；不增加手工版本来源即可。 |
| D8 | 全局 AGENTS、项目 AGENTS、相关记忆：范围、验证和无关改动保护 | 内容总体一致。当前文件能直接发现的工程细节无需再复制进记忆；旧记忆只作线索，不覆盖当前规则。 |

五份局部 CLAUDE 各只有一行同级导入；不存在五套复制的项目正文。各局部 AGENTS 也已去掉重复的“先遵守根规则”开场。

## 3. 应降级或撤回的原判断

### F6 · 不列为当前项目冲突

Impeccable 入口第 23 行及 [craft-floor](../.agents/skills/impeccable/reference/craft-floor.md)第 3、21、25–27 行，在“brief 优先”和绝对审美禁令之间确有张力。但 RayleaBot 的 PRODUCT 第 47、49 行、DESIGN 第 208、406–407 行本身也反对装饰性眉题及相应的无任务意义卡片结构。

因此，原报告没有证明这些条款在本仓库导致互相矛盾的动作，中等优先级不成立。保留为上游通用适用性观察；若未来任务明确要求相反设计，再判断具体冲突，不预先要求改掉项目或上游规则。

### F7 · 环境观察，没有硬拒绝或过度确认的复现证据

本机 Codex / Claude manifest 调用现有 Impeccable `hook` 入口。项目配置 enabled，最多输出 5 条 / 8000 字符；本地 consent 为 accepted，没有设置 quiet。[hooks 参考](../.agents/skills/impeccable/reference/hooks.md)第 5–11、103–106 行明确：Codex / Claude 是编辑后提示，Cursor 才有写前阻止。

当前没有证据表明提示已经被 AI 当成硬拒绝；原标题将这种可能性提升为问题，依据不足。也没有测量到提示造成的重复确认或明显耗时，不建议仅凭推测改 quiet 或关闭规则。`hook.quiet` 只减少 clean / pending 提示，本地 consent 也不能证明宿主已经批准并调度 hook。

### F10 · 低优先级可选维护，不是当前开发阻塞

[Makefile](../Makefile)第 3–8、17–35 行的细分测试和 typecheck 目标仍依赖全量 doctor；[工具链脚本](../scripts/check-toolchain.py)第 415–455、468–482 行已经支持按 task 检查。

但 quality-gates 第 15 行已明确按改动面直接使用工程命令，日常验证不要求经过 Makefile；本机七项工具链也已通过检查。完整 doctor 作为环境体检有用途。只有需要改善 Makefile 使用体验、或出现具体无关依赖阻塞时，再考虑收窄其目标依赖，当前不必改代码。

### F4、F5 · 条件性 UI 流程风险，尚无实际重复验收证据

用户安装的 `playwright-interactive` 第 28–44 行确有完整控件状态清单和至少两个探索场景要求；Impeccable 第 13 行限定批量检查轮次，Product Design 的 design-qa 则要求修复可操作 P0/P1/P2 并继续比对。

这些工具只有在对应流程被选用时才发生作用。Product Design 入口已排除普通实现，get-context 也允许目标明确时同轮继续。本次没有复现三套流程同时执行，不应把“同时可用”写成“已经叠加”。

对 RayleaBot 的建议限于按任务选择主流程、验证与改动风险对应；不新增一套必须阅读的 UI 治理规则，也不要求为常规实现先进入选图或完整产品验收。

## 4. 当前版本已经处理的事项

本轮开始时 Git 中仅本报告未跟踪，以下改动已提交，不再标为“待提交”或“尚未处理”。

| 提交 / 来源 | 当前状态 |
| --- | --- |
| `4bd6da5b` | 界面尺寸与材质参数统一由 Web 规范维护；DESIGN 保留共享规则和引用。 |
| `9fbb365c` | Contracts AGENTS 增加配套登记入口；contract-audit 改为引用它。该清单存在不代表 F8 的命令选择矛盾已经解决。 |
| `96929a09` | 旧审计补充保留理由和完成情况；原始发现与处理记录仍需结合阅读。 |
| `85804c7f` → `78894864` | 可选协作者的表述曾写成构造期 Option 注入，随后改为“可选协作者在构造期注入，需要缺省行为时在构造函数中补齐”，已不强制函数式 Option。 |
| 当前根 AGENTS / contract-audit | 内部调整不默认全面契约审计；描述性契约修正不默认新增 fixture。 |
| 当前 Docs AGENTS | 只修实际漂移、执行计划只约束对应版本，历史归档另有边界。 |
| 当前 baseline 第 89 行 | 区分运行依赖 / 固定选型与开发、测试、脚本依赖；开发依赖只需说明必要性。 |
| 用户安装的 Vue / Vite 相关 skill | 按已安装版本和既有测试惯例工作；不属于系统内置 skill。 |
| 本机 agent 配置 | 旧 Claude / Gemini Impeccable 副本已移除；Codex 规则文件为空；当前 hook manifest 不再指向缺失的旧入口。宿主触发未在本次验证。 |

### 可选协作者的源码依据与统计口径

不应把函数式 Option 写成统一要求，这个判断成立。主流代码已经有 Deps / Options 结构注入，例如 `server/internal/app/app.go` 第 58、99 行、`server/internal/plugins/settings/service.go` 第 68 行、`server/internal/plugins/lifecycle/install.go` 第 103 行。

对当前 Git 跟踪的 Server Go 源文件，排除 `*_test.go` 和 `server/tests/`，按声明检索得到：

| 项目 | 本次数量 | 口径 |
| --- | ---: | --- |
| 函数式 Option 类型 | 2 | 类型名含 Option 且底层为函数；这不等于两个都用于构造协作者。 |
| 接收 Deps / Options 结构类型的 New 函数 | 27 | 包含单行和跨行参数声明。 |
| New 函数 | 119 | 包含直接命名为 `New` 的 16 个和带后缀的 103 个。 |

反馈中的 103 可以复现为只计带后缀的 New 函数；25 可以复现为只计单行 Deps / Options 签名，遗漏安装与卸载构造函数的跨行参数。它们不宜标为全部构造函数数量。上述计数仅辅助说明现有风格，不作为每个构造函数都应改为 Deps / Options 的依据；当前 Server 指令已纠正，无需再次修改。

## 5. 应保留的要求

- **正式来源与状态归属**：契约、Server snapshot、领域服务各有责任；修复实现符合已有契约可以直接进行。
- **共享状态同步**：Server 与 Launcher 都有并发状态，根规则有跨目录依据；它不要求给非共享状态加锁。
- **输入与凭据边界**：schema、secret store、鉴权、CSRF / Origin、真实凭据不进入日志及 fixtures，均有实际风险依据。
- **装配和错误可观察性**：缺少必选依赖不能假成功；可选协作者沿用现有构造方式，不无差别删除历史 nil 检查或强造 no-op 层。
- **生成来源与工程隔离**：SQL / sqlc、OpenAPI / 类型、Go model / Wails bindings 各自有输入；Launcher 的 `GOWORK=off` 和平台参数也有真实工程用途。
- **风险对应的验证**：登录页截图不能证明受保护页面正常；mock 结果不能证明真实持久化、权限或系统集成。准确说明未验证范围，不将所有运行覆盖变成每次任务的前置条件。
- **真正涉及外部效果的确认**：可信插件安装、写入秘密值、未授权发布或消息发送仍有操作边界。产品文档中的确认描述，不能自动变成编辑相关代码前的再次询问。

例如[管理面说明](./user/management-surface.md)第 57、87–89 行和[插件生命周期](./plugin/lifecycle.md)第 37 行，约束的是实际操作者的确认行为；[人工 Smoke](./engineering/manual-smoke.md)约束真实平台验收。这些不是需要删除的“防御性文案”。

## 6. 建议的最小处理范围

1. 调整 quality-gates 的契约生成链及 renderer / Go 选择说明，处理 F8、F9。
2. 给实施顺序的数据库和协议配套句补实际影响条件，处理 F11；baseline 的冲突处理句保留。
3. F12 随上游 skill 升级或门禁维护处理；F13 随报告保留 / 归档决定处理。其余低风险重复无需集中制造 diff。

全局插件流程若需要优化，另按其实际使用任务处理，不作为以上仓库修正的前提。验证治理改动时，抽查内部修复、局部样式和单个 fixture 修改的实际动作范围即可；不因此再建立强制全量审计或每次任务回放要求。

## 附录 A：全局工具的条件性风险

这些条款在首次审计中已定位，本轮复核其与仓库的相关性。它们保留在审计范围内，但不计为 RayleaBot 当前缺陷或首要整改项。

本机路径缩写：

- `SYS`：`C:/Users/Raylea/.codex/skills/.system`
- `USR`：`C:/Users/Raylea/.agents/skills`
- `DEV`：`C:/Users/Raylea/.codex/plugins/cache/openai-curated-remote/openai-developers/1.3.0`
- `PD`：`C:/Users/Raylea/.codex/plugins/cache/openai-curated-remote/product-design/0.1.55`
- `EXA`：`C:/Users/Raylea/.codex/plugins/cache/openai-curated-remote/app-69ea4ed2cf7c8191b742ef3622479ddd/3.0.0`
- `DOC`：`C:/Users/Raylea/.codex/plugins/cache/openai-primary-runtime/documents/26.909.22227`

| 原编号 | 来源与事实 | 实际适用范围与建议 |
| --- | --- | --- |
| F1 | `DEV/skills/openai-platform-api-key/SKILL.md` 第 17、33、50–67、94–104 行：不发请求、不写密钥的 API 代码工作也先询问复用 / 新建并等待 | 适用于调用 OpenAI 或未指定其他供应商的相应 AI 应用构建，不适用于一般仓库审计。当前 Server、Web、Launcher、SDK、示例源码和主要工程依赖检索未发现直接 OpenAI API 调用。作为全局工作流优化，可以将授权贴近实际密钥写入和付费调用；不是本仓库首要问题。 |
| F2 | `SYS/openai-docs/SKILL.md` 第 12–14 行：固定官方搜索优先，先于本地文件检查 | 只涉及 Codex / OpenAI 相关任务。本机诊断应重视安装实况，公开产品事实仍需官方来源；不为普通 RayleaBot 修复增加联网前置流程。 |
| F3 | `EXA/skills/Search/SKILL.md` 第 18–23、39–54 行：检索深度有歧义时确认；Exa 不可用时不回退通用搜索 | 研究任务专用，且已有明确深度时不询问。未复现对本仓库开发的阻塞；仅在实际研究体验受影响时调整。 |
| F14-a | `USR/security-threat-model/SKILL.md` 第 49–53 行：最终报告前确认假设并等待 | 入口已限定明确威胁建模。缺失且会改变排序的背景值得询问；不把此流程引入一般审计或实现。 |
| F14-b | `USR/security-best-practices/SKILL.md` 第 62–74 行：报告后等待用户要求修复 | 仅安全任务；原任务已授权修复时不应机械重问，只有审计授权时继续保留只报告边界。 |
| F14-c | `USR/playwright/SKILL.md` 第 14–32 行：npx 缺失时暂停要求安装 | 当前本机相关工具齐备，没有实际阻塞；是否回退其他浏览器工具取决于对应任务。 |
| F14-d | `DOC/skills/documents/SKILL.md` 第 78–86 行：渲染并重复到 flawless | Word 排版专用，和本报告 Markdown 无关。有界修复可改善该流程，但不是仓库开发要求。 |

## 附录 B：配置、加载状态与检查覆盖

### B.1 本机状态

| 来源 | 核对结果 | 结论边界 |
| --- | --- | --- |
| 当前会话 | Windows / PowerShell，可执行命令和访问文件，无命令审批等待 | 不因假设的环境限制暂停授权内工作；不代表任意外部副作用都已授权。 |
| Codex 配置 | `windows.sandbox` 为 unelevated；启用 memories；配置没有列出 OpenAI Developers / Exa 对应标识 | 静态配置不完整描述本次会话能力，不能据此认定插件未启用。 |
| `.codex-global-state.json` | 能检索到 OpenAI Developers 与 Exa 应用对应记录 | 证明存在本机状态记录，不单独证明每个工具已经连接或执行成功。 |
| 当前可用 skill 清单 | 提供 OpenAI Developers 和 Exa 的入口 | 审计以该会话实际清单为依据；存在于磁盘、被会话提供、正文被选用、工具实际可调用不是同一状态。 |
| Codex / Claude hook | manifest 指向现有 launcher；本地 consent accepted | 未在本次执行跨宿主 hook 回放，不宣称实际已触发或被拒绝。 |
| Codex 规则 / Claude 白名单 | Codex `default.rules` 为空；Claude 36 条 allow，未见旧用户名路径 | 是各宿主的本机规则，不互相替代。 |
| Gemini / VS Code | Gemini 读取 AGENTS；VS Code 有特定 Go 命令自动批准规则 | 不用它们推断 Codex 只能执行哪些命令。 |
| 工具链 | Go 1.26.6、Node 26.7.0、npm 11.19.0、Corepack 0.35.0、pnpm 11.22.0、Python 3.14.7、sqlc 1.31.1 检查通过 | 当前没有缺失工具阻塞；版本固定与检查范围应分别评价。 |

有关通用加载和权限概念，可参阅[AGENTS 加载规则](https://learn.chatgpt.com/docs/agent-configuration/agents-md)、[Skill 说明](https://learn.chatgpt.com/docs/build-skills)、[权限说明](https://learn.chatgpt.com/docs/security)。这些链接用于解释通用机制，不覆盖当前安装文件和有效会话状态。

### B.2 文档与直接指令

首次清点有 126 份仓库 Markdown：`docs/` 60 份、项目 skill 及参考文档 42 份、其他 24 份；加上本报告为 127 份。检查方式是全量文件清点和约束检索，重点阅读直接指令、开发规则及其交叉来源，未声称逐行审计全部业务源码和所有外部参考书。

| 指令 | 当前行数 |
| --- | ---: |
| 根 AGENTS / 根 CLAUDE | 54 / 10 |
| Server AGENTS | 25 |
| Web AGENTS | 21 |
| Launcher AGENTS | 30 |
| Contracts AGENTS | 31（首次基准为 27） |
| Docs AGENTS | 12 |
| 五份局部 CLAUDE | 各 1 |
| Codex 全局 AGENTS | 12 |

### B.3 Skill 覆盖索引

会话提供 47 个入口：项目内 3 个、仓库外 44 个。另检查 Product Design 的 `get-context`、`design-qa`、`research`、`share`、`user-context` 五个内部入口及相关 overrides。以下是覆盖记录，不是要求日常任务全部加载。

| 归属 | 入口 | 与本仓库的关系 |
| --- | --- | --- |
| 项目自有 | `contract-audit`、`editing-final-state-content` | 直接适用，当前已有范围例外和责任入口。 |
| 项目上游副本 | `impeccable` 4.2.2 | UI 工作使用；入口 82 行。通用禁令需结合当前项目目标判断。 |
| 系统内置 | `openai-docs`、`imagegen`、`skill-creator`、`skill-installer`、`plugin-creator` | 按各自专用任务触发；普通文档审计不触发安装、创建或密钥工作。 |
| 用户安装 | `aspnet-core`、`github-actions-templates`、`linear`、`notion-spec-to-implementation`、`openapi-spec-generation` | 技术栈、CI、外部项目管理和接口任务分别适用，不全局叠加。 |
| 用户安装 | `pinia`、`vite`、`vue-router-best-practices`、`vue-testing-best-practices` | 跟随现有项目版本与测试惯例；不是系统内置 skill。 |
| 用户安装 | `playwright`、`playwright-interactive`、`playwright-best-practices`、`screenshot` | 浏览器、测试和系统截图工具；避免把工具流程扩大为整页验收。 |
| 用户安装 | `python-testing-patterns`、`security-best-practices`、`security-threat-model` | Python 测试与明确安全任务适用。 |
| 用户安装 | `codex-windows-fast-patch` | Codex 本机修补专用，入口 604 行；其中归档旧 skill 不算当前入口。 |
| Exa / Deep Research | `Search`、`deep-research` | 研究任务，附录 A 保留相应观察。 |
| OpenAI Developers | `agents`、`build-chatgpt-app`、`chatgpt-app-submission`、`openai-api-troubleshooting`、`openai-platform-api-key` | API / Apps 专用；不能只因 RayleaBot 名称或产品类型而启用凭据流程。 |
| Plugin Management | `plugin-management` | 插件发现和管理，不为普通开发擅自安装。 |
| Product Design | `index`、`audit`、`ideate`、`image-to-code`、`url-to-code` | 设计探索、审查和忠实还原；入口已排除普通实现。 |
| Sites | `sites-building`、`sites-hosting`、`sites-preview-troubleshooting` | 不把 Sites 或云端预览要求套入本仓库普通开发。 |
| 产物插件 | `documents`、`pdf`、`presentations`、`spreadsheets`、`excel-live-control`、`template-creator` | Office、PDF、表格和模板专用，不适用于本报告的 Markdown 编辑。 |

## 验证与限制

本轮复核当前提交及首次基准之后的五个提交，核对相关指令、配套登记、设计反例、工程脚本、生成器输入和本机插件记录。构造方式计数按上述文件和声明口径进行，不据此推导所有构造函数的设计要求。

本轮完成文档链接、agent-docs、工具链版本及声明检查；报告 UTF-8、空白检查通过，修改前后的既有跟踪文件摘要一致。当前变更范围只有本报告。

未执行完整 Go / Web / Launcher 测试、race、E2E、打包发布、跨宿主 hook 触发、真实平台消息、密钥申请或外部 skill 的完整任务回放。这些不是本次 Markdown 修订的必要验证；未将文字上的潜在风险描述为已经发生的运行阻塞。
