# RayleaBot AI 开发指令与环境限制审计（2026-09-13）

> **状态：审计快照，不是开发指令。** 核对基准为 `91911a770c48643c51fcaaa7544c9fabc3b32b67` 及当前工作区修正。仓库或 skill 变化后需重新核对，日常开发按当前用户要求和有效项目指令执行。

本报告覆盖仓库文档要求，也检查当前会话提供的项目级、用户安装、系统内置及插件 skill。**正文优先处理与 RayleaBot 开发直接相关的问题；全局工具的条件性风险放在附录，不参与仓库整改优先级。**

## 结论

当前核对范围内，没有需要立即修正的明确仓库指令问题。以下保留必要重复项的责任边界，以及尚未通过任务回放证实的 UI 和全局工具流程风险。

优先级取决于本仓库的实际触发频率、影响范围和既有替代入口。“可能导致”表示条件性推断，不代表已经发生运行阻塞。

## 1. 重复项的责任边界

同一事实服务不同读者，不等于有害重复。以下重复项用于判断责任归属，不作为必须删除的缺陷。

| 编号 | 来源与内容 | 当前判断 |
| --- | --- | --- |
| D1 | 根 AGENTS 第 15–17 行；[contracts/README](../contracts/README.md)第 125–128 行；[设计入口](./design/README.md)第 5 行；[Server README](../server/README.md)第 13 行：契约优先 | 根规则持有开发动作要求，各读者入口保留简短正式来源指针即可。当前不需要继续全仓去重。 |
| D2 | 根 AGENTS 第 18 行；[平台架构](./architecture/platform-architecture.md)第 61、98 行；实施顺序第 76 行；Launcher AGENTS 第 7、22 行：状态和职责归属 | 架构解释、局部应用及例外有各自职责。保留必要数据流，不机械删除相同事实。 |
| D3 | 根 AGENTS 第 22 行；Server AGENTS 第 17 行；实施顺序第 38、51 行：共享状态同步 | 根规则覆盖 Server 和 Launcher；热更新说明负责具体场景。保留根并发规则的决定有源码依据。 |
| D5 | [Docs AGENTS](./AGENTS.md)第 8 行；[editing-final-state-content](../.agents/skills/editing-final-state-content/SKILL.md)第 12–23 行：文本取舍 | Docs 说明文档职责，skill 细化编辑判断；低风险摘要重复，不需要再拆规则。 |
| D7 | baseline 第 25–43 行、`.tool-versions`、工程声明：版本值 | 工程必填声明属于必要副本，已有一致性检查；不增加手工版本来源即可。 |
| D8 | 全局 AGENTS、项目 AGENTS、相关记忆：范围、验证和无关改动保护 | 内容总体一致。当前文件能直接发现的工程细节无需再复制进记忆；旧记忆只作线索，不覆盖当前规则。 |

五份局部 CLAUDE 各只有一行同级导入，承担跨工具共享规则的入口职责。

## 2. F4、F5：条件性 UI 流程风险

用户安装的 `playwright-interactive` 第 28–44 行确有完整控件状态清单和至少两个探索场景要求；Impeccable 第 13 行限定批量检查轮次，Product Design 的 design-qa 则要求修复可操作 P0/P1/P2 并继续比对。

这些工具只有在对应流程被选用时才发生作用。Product Design 入口已排除普通实现，get-context 也允许目标明确时同轮继续。本次没有复现三套流程同时执行，不应把“同时可用”写成“已经叠加”。

对 RayleaBot 的建议限于按任务选择主流程、验证与改动风险对应；不新增一套必须阅读的 UI 治理规则，也不要求为常规实现先进入选图或完整产品验收。

## 3. 应保留的要求

- **正式来源与状态归属**：契约、Server snapshot、领域服务各有责任；修复实现符合已有契约可以直接进行。
- **共享状态同步**：Server 与 Launcher 都有并发状态，根规则有跨目录依据；它不要求给非共享状态加锁。
- **输入与凭据边界**：schema、secret store、鉴权、CSRF / Origin、真实凭据不进入日志及 fixtures，均有实际风险依据。
- **装配和错误可观察性**：缺少必选依赖不能假成功；可选协作者沿用现有构造方式，不无差别删除历史 nil 检查或强造 no-op 层。
- **生成来源与工程隔离**：SQL / sqlc、OpenAPI / 类型、Go model / Wails bindings 各自有输入；Launcher 的 `GOWORK=off` 和平台参数也有真实工程用途。
- **风险对应的验证**：登录页截图不能证明受保护页面正常；mock 结果不能证明真实持久化、权限或系统集成。准确说明未验证范围，不将所有运行覆盖变成每次任务的前置条件。
- **真正涉及外部效果的确认**：可信插件安装、写入秘密值、未授权发布或消息发送仍有操作边界。产品文档中的确认描述，不能自动变成编辑相关代码前的再次询问。

例如[管理面说明](./user/management-surface.md)第 57、87–89 行和[插件生命周期](./plugin/lifecycle.md)第 37 行，约束的是实际操作者的确认行为；[人工 Smoke](./engineering/manual-smoke.md)约束真实平台验收。这些不是需要删除的“防御性文案”。

## 附录 A：全局工具的条件性风险

以下条款仅在对应工具任务中适用，不计为 RayleaBot 当前缺陷或首要整改项。

本机路径缩写：

- `SYS`：`C:/Users/Raylea/.codex/skills/.system`
- `USR`：`C:/Users/Raylea/.agents/skills`
- `DEV`：`C:/Users/Raylea/.codex/plugins/cache/openai-curated-remote/openai-developers/1.3.0`
- `PD`：`C:/Users/Raylea/.codex/plugins/cache/openai-curated-remote/product-design/0.1.55`
- `EXA`：`C:/Users/Raylea/.codex/plugins/cache/openai-curated-remote/app-69ea4ed2cf7c8191b742ef3622479ddd/3.0.0`
- `DOC`：`C:/Users/Raylea/.codex/plugins/cache/openai-primary-runtime/documents/26.909.22227`

| 编号 | 来源与事实 | 实际适用范围与建议 |
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

当前共 126 份仓库 Markdown（含本报告）：`docs/` 60 份、项目 skill 及参考文档 42 份、其他 24 份。检查方式是全量文件清点和约束检索，重点阅读直接指令、开发规则及其交叉来源，未声称逐行审计全部业务源码和所有外部参考书。

| 指令 | 当前行数 |
| --- | ---: |
| 根 AGENTS / 根 CLAUDE | 54 / 10 |
| Server AGENTS | 25 |
| Web AGENTS | 21 |
| Launcher AGENTS | 30 |
| Contracts AGENTS | 31 |
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

已核对当前工作区的验证说明、配套更新条件和指令检查器。附录记录审计期间检查过的工具及环境范围，不代表全部流程均已执行。

`node --test scripts/tests/check-agent-docs.test.mjs` 的 23 项测试通过，覆盖 AGENTS / CLAUDE 行数预算、项目自有及上游 skill 不限行数，以及长 skill 的路径和凭据检查。变更分类自检及对应 3 项单测通过，覆盖设计工具配置的分类；agent-docs、文档链接和空白检查通过。修改范围为验证说明、实施顺序、指令检查器及其测试、CI 变更分类和本报告。

未执行完整 Go / Web / Launcher 测试、race、E2E、打包发布、跨宿主 hook 触发、真实平台消息、密钥申请或外部 skill 的完整任务回放。此次未修改业务运行逻辑，未将文字上的潜在风险描述为已经发生的运行阻塞。
