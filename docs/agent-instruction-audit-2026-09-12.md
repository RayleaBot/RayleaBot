# 全仓 AI 指令与开发环境约束审计（2026-09-12）

## 文档状态

- 核对基准：工作区 `340f8765`，2026-09-12；工作区存在大量未提交改动，结论按当前文件内容给出。
- 范围：仓库内所有 `AGENTS.md` / `CLAUDE.md`、项目级 skill、被指令引用为规则的治理文档、CI 与本地门禁、编辑器与 agent harness 的 hook / 权限配置，以及本机用户级的 Codex、Claude、Gemini 指令、skill 与记忆。
- 方法：逐份阅读文件正文，对照 `scripts/check-agent-docs.mjs`、`scripts/check-doc-links.py`、`scripts/check-server-structure.py`、`server/tests/architecture/` 与 `.github/workflows/ci.yml` 确认哪些规则由脚本强制；重复项按“同一约束在不同文件被完整重述”统计，仅作为来源指针的一句引用不计入。
- 第一至五节记录 2026-09-12 的发现与建议；各项处理结果见文末“处理记录”。

## 一、指令与约束盘点

### 1.1 仓库指令文件

| 文件 | 行数 | 生效方式 | 说明 |
| --- | --- | --- | --- |
| [`AGENTS.md`](../AGENTS.md) | 53 | Codex 直接读取；Claude 经 `CLAUDE.md` 的 `@AGENTS.md` 导入；Gemini 经 `.gemini/settings.json` 的 `context.fileName` 读取 | 七个分区：Instruction Scope、Hard Rules、Source of Truth、Working Entrypoints、Testing、Instruction Maintenance、Git and Review |
| [`CLAUDE.md`](../CLAUDE.md) | 9 | Claude Code | 桥接文件，另有 3 条说明 bridge 用法的元规则 |
| [`contracts/AGENTS.md`](../contracts/AGENTS.md) | 27 | 同上，进入目录后叠加 | Definitions、API and State、Errors、Merge Readiness |
| [`docs/AGENTS.md`](../docs/AGENTS.md) | 12 | 同上 | 文档取舍、契约优先、CHANGELOGS 归档不改、链接规则 |
| [`launcher/AGENTS.md`](../launcher/AGENTS.md) | 30 | 同上 | Ownership、Desktop Bridge、State and Security、Go Environment |
| [`server/AGENTS.md`](../server/AGENTS.md) | 29 | 同上 | State and Input、Architecture、Concurrency and Assembly、Testing and Generation |
| [`web/AGENTS.md`](../web/AGENTS.md) | 22 | 同上 | Interfaces and State、Errors、Browser Verification |

每个子目录的 `CLAUDE.md` 只有一行 `@AGENTS.md`。`scripts/check-agent-docs.mjs` 强制：每个 `AGENTS.md` 必须有同级 bridge 且含活动导入；根 `AGENTS.md` ≤150 行、根 `CLAUDE.md` ≤40 行、局部 `AGENTS.md` ≤120 行、`.agents/skills/**/SKILL.md` ≤100 行；反引号内含 `/` 的路径必须存在；含 secret 关键词的长值触发疑似凭据告警。当前门禁通过。

### 1.2 项目级 skill

| Skill | 位置 | 行数 | 谁会自动加载 | 说明 |
| --- | --- | --- | --- | --- |
| contract-audit | [`.agents/skills/contract-audit/SKILL.md`](../.agents/skills/contract-audit/SKILL.md) | 30 | Codex | 契约影响判断、核对影响、合并验收 |
| editing-final-state-content | [`.agents/skills/editing-final-state-content/SKILL.md`](../.agents/skills/editing-final-state-content/SKILL.md) | 25 | Codex | 文档、注释、界面文案的取舍与表达 |
| impeccable 4.2.2 | [`.agents/skills/impeccable/SKILL.md`](../.agents/skills/impeccable/SKILL.md) | 81 | Codex | 上游设计 skill，56 个文件纳入版本控制；`scripts/` 目录只有 launcher 与 live-browser 脚本，没有 `hook.mjs` |
| impeccable 4.1.1 | `.claude/skills/impeccable/`（gitignored） | — | Claude Code | 与 `.agents` 副本版本不同，自带完整 `scripts/` 与 `hook.mjs` |
| impeccable 4.1.1 | `.gemini/skills/impeccable/`（gitignored） | Gemini | 同上 | 与 `.claude` 副本内容不同 |

Claude Code 的可用 skill 列表只包含 `.claude/skills/impeccable` 与系统插件 skill，不包含 `.agents/skills/` 下的两个项目 skill。根 `AGENTS.md` 的“项目自有 skill 位于 `.agents/skills/`，按任务需要使用”和 `docs/AGENTS.md` 的“文本编辑使用 `.agents/skills/editing-final-state-content/SKILL.md`”对 Claude 而言是“按路径读文件”，不是可触发的 skill。

### 1.3 被当作规则的治理文档

以下文档不是 agent 指令文件，但被 `AGENTS.md` 指为 Source of Truth，或本身用“必须 / 不得 / 不”表述约束，agent 在对应领域会把它们当规则执行。

| 文档 | 行数 | 约束密度 | 主要约束 |
| --- | --- | --- | --- |
| [`docs/engineering/baseline.md`](./engineering/baseline.md) | 191 | 中 | 固定版本线、默认命令、目录职责、新依赖五问、“绕开 baseline 与 contracts 直接写功能代码视为违反仓库治理规则” |
| [`docs/engineering/implementation-order.md`](./engineering/implementation-order.md) | 113 | 高 | 九步依赖顺序；§4 服务端六条实现要求；§9 验收清单 |
| [`docs/engineering/quality-gates.md`](./engineering/quality-gates.md) | 53 | 低 | CI 门禁层次与验证原则 |
| [`docs/engineering/web-admin-baseline.md`](./engineering/web-admin-baseline.md) | 109 | 中 | Web 目录职责、请求与实时通信、工作区 keep-alive 规则、四条“不” |
| [`docs/engineering/web-testing.md`](./engineering/web-testing.md) | 61 | 中 | 真实 Server 优先、模拟入口边界、“不得为了截图或断言绕过鉴权” |
| [`contracts/README.md`](../contracts/README.md) | 128 | 中 | 契约清单、各契约固定语义、通用规则 |
| [`fixtures/README.md`](../fixtures/README.md) | 94 | 高 | 命名、编写、schema 与语义边界、扩展规则，共 12 处“必须 / 不能 / 不得” |
| [`examples/README.md`](../examples/README.md) | 17 | 中 | HTTP 示例登记、示例不是正式接口 |
| [`docs/dev/repo-workflow.md`](./dev/repo-workflow.md)、[`docs/dev/README.md`](./dev/README.md) | 45 / 75 | 低 | 版本控制边界、`gbash` 与本地启动 |
| [`docs/dev/text-resources.md`](./dev/text-resources.md)、[`docs/dev/logging.md`](./dev/logging.md)、[`docs/dev/diagnostics.md`](./dev/diagnostics.md) | 17 / 19 / 74 | 低 | 文案资源归属、日志级别与脱敏、诊断入口 |
| [`docs/architecture/README.md`](./architecture/README.md)、[`docs/architecture/platform-architecture.md`](./architecture/platform-architecture.md)、[`docs/architecture/plugin-runtime.md`](./architecture/plugin-runtime.md) | 26 / 148 / 78 | 中 | 架构不变量、组件“禁止承担”列、信任边界 |
| [`DESIGN.md`](../DESIGN.md)、[`PRODUCT.md`](../PRODUCT.md) | 411 / 76 | 高 | Impeccable 每次 UI 任务都会加载；DESIGN.md 含十余条 Named Rule 与像素级组件规则 |
| [`docs/design/web-management-ui.md`](./design/web-management-ui.md)、[`docs/design/launcher-design-system.md`](./design/launcher-design-system.md)、[`docs/design/plugin-management-surface.md`](./design/plugin-management-surface.md) | 177 / 134 / 64 | 高 | 各界面规范与验收条件 |
| [`docs/RayleaBot机器人项目规划.md`](./RayleaBot机器人项目规划.md) | 95 | 低 | 设计原则、演进规则 |
| [`docs/execution-plan-v0.6-conversation.md`](./execution-plan-v0.6-conversation.md) | 466 | 极高 | 100 处约束用语；只约束 v0.6 任务，`AGENTS.md` 未引用 |
| [`.github/PULL_REQUEST_TEMPLATE.md`](../.github/PULL_REQUEST_TEMPLATE.md) | 27 | 中 | 四项验证勾选，重述契约优先规则 |

### 1.4 自动门禁、hook 与编辑器约束

| 入口 | 位置 | 生效范围 | 约束内容 |
| --- | --- | --- | --- |
| agent-docs 门禁 | `scripts/check-agent-docs.mjs`，CI `agent-docs` job | docs 或 ci 变更时 | 见 1.1 |
| 文档链接门禁 | `scripts/check-doc-links.py` | 同上 | 所有 Markdown 的相对链接与锚点必须存在 |
| 服务端结构门禁 | `scripts/check-server-structure.py`，`make doctor` | Server | 禁止 `common` / `utils` / `helper(s)` 包目录；`internal/` 禁止 `os.Exit` / `log.Fatal`；手写 SQL 必须登记在 `docs/engineering/manual-sql-exceptions.json`；插件、适配器、模型包的 import 边界 |
| 架构测试 | `server/tests/architecture/` | Server | management 不进入领域包；adapter 不 import management / config/runtime / app；共享模型包不传递依赖 storage / render / plugins 实现；事件管线不 import `plugins/runtime`；领域包不 import `app` / `cli` |
| 工具链门禁 | `Makefile` 全部 `test-*` / `*-typecheck` / `server-build` 目标依赖 `doctor` | 本地 | `.tool-versions` 七种工具版本精确匹配，版本错误直接失败 |
| Claude hook | `.claude/settings.local.json`（gitignored） | Claude Code | `PostToolUse(Edit\|Write\|MultiEdit)` 与 `Stop` 运行 impeccable 4.1.1 检测器；另含 49 条权限白名单 |
| Codex hook | `.codex/hooks.json`（gitignored） | Codex | 指向 `.agents/skills/impeccable/scripts/hook.mjs`，该文件不存在，hook 静默跳过 |
| Gemini | `.gemini/settings.json` | Gemini | 仅声明上下文文件为 `AGENTS.md` |
| VS Code | `.vscode/settings.json`（gitignored） | Copilot 终端 | 仅自动批准 `go test ./... 2>&1` |
| devcontainer | `.devcontainer/devcontainer.json` | 容器 | `postCreateCommand: make doctor` |

### 1.5 系统级（本机用户目录）

| 入口 | 内容 | 对本仓库的影响 |
| --- | --- | --- |
| `~/.codex/AGENTS.md` | 两条：避免琐碎进度播报；称用户为“柒柒” | 全局风格，无冲突 |
| `~/.codex/.AGENTS.md.bkup` | 旧版更长的沟通规范（去术语、只写结论、验证标准） | 已停用，仅备份 |
| `~/.codex/config.toml` | `personality = "pragmatic"`、`model_reasoning_effort = "high"`、`memories.use_memories = true`、`disable_on_external_context = true`、MCP（memory、sequential-thinking、context7、fetch、playwright） | 记忆会在每次 Codex 会话注入 |
| `~/.codex/memories/MEMORY.md`、`memory_summary.md` | 十余组任务记忆，含约 30 条“User preferences”，其中 RayleaBot 相关的核心是：审计先调查不改代码、只修确有漂移的文档、最小充分验证并声明未跑项、结合项目实际分析、按范围分次提交并保留无关脏文件 | 与根 `AGENTS.md` 的 Git 与验证规则重叠，见二、三 |
| `~/.codex/rules/default.rules` | 允许 `gbash -lc` 前缀；两条指向已迁移路径 `C:\Users\26789\...` 的 dotnet 规则 | 后两条已失效 |
| `~/.codex/skills/codex-windows-fast-patch/` | 与 Codex Desktop 修补相关的独立 skill，`SKILL.md` 约 600 行 | 与本仓库无关；描述覆盖大量 Codex Desktop 关键词，不会被本仓库任务触发 |
| `~/.agents/skills/`（16 个） | openai/skills：aspnet-core、linear、notion-spec-to-implementation、playwright、playwright-interactive、screenshot、security-best-practices、security-threat-model；wshobson：github-actions-templates、openapi-spec-generation、python-testing-patterns；antfu / vuejs-ai：pinia、vite、vue-router-best-practices、vue-testing-best-practices | 与本仓库栈相关的是 pinia、vite、vue-router、vue-testing、playwright 系列、openapi。两个 security skill 均声明“仅在用户明确要求时触发”，不会自行加严 |
| `~/.claude/` | 无 `CLAUDE.md`、无 `settings.json`、无 skills；项目记忆两条（默认合并到 main 且不 push；v0.5 清理已完成及保留项） | 记忆与仓库规则一致 |
| `~/.gemini/settings.json` | 仅 MCP 配置 | 无指令 |

## 二、重复项

| 编号 | 规则 | 完整重述位置 | 建议归属 |
| --- | --- | --- | --- |
| D1 | 契约优先：改变对外语义先改 `contracts/`；修复实现以符合现有契约直接修实现 | 根 `AGENTS.md` Hard Rules 第 2 条；`contracts/README.md` 通用规则；`baseline.md` 目的与末段；`implementation-order.md` §1；`quality-gates.md` 验证原则；`contract-audit` 判断变化；PR 模板两项；`fixtures/README.md` 编写规则；`docs/design/README.md`；`web-admin-baseline.md` 约束 | 根 `AGENTS.md` 保留一句；`contracts/AGENTS.md` 与 `contract-audit` 保留操作细节；其余改为指向根规则的一句引用（`6ae3c608` 已开始做这件事，尚未收敛完） |
| D2 | 按实际影响更新配套项，不为无关文件制造 diff | 根 `AGENTS.md` Hard Rules 第 3 条；`contracts/AGENTS.md` Merge Readiness；`contract-audit` 两处；`implementation-order.md` §2；`quality-gates.md`；PR 模板 | 根 `AGENTS.md` 一处 + `contract-audit` 一处 |
| D3 | fixture-ready 是合并条件；契约与样例可先后编辑，合并前引用须存在 | 根 `AGENTS.md`；`contracts/AGENTS.md`；`contract-audit` 合并验收；`implementation-order.md` §2；`fixtures/README.md` 两处；`contracts/README.md` 当前状态 | `contracts/AGENTS.md` 与 `fixtures/README.md` 各一处 |
| D4 | Server 是唯一正式状态源；Web / Launcher 不持有第二份业务状态，不从日志推断状态 | `server/AGENTS.md`；`web/AGENTS.md` 两条；`launcher/AGENTS.md` 两处；`docs/architecture/README.md` 不变量；`platform-architecture.md` 两处；`implementation-order.md` §3 与 §7；`web-admin-baseline.md` 三处；`PRODUCT.md` 两处；项目规划两处；`management-surface.md`；`launcher-design-system.md`；`docs/design/README.md` | 架构不变量保留在 `docs/architecture/README.md`；三份局部 `AGENTS.md` 各保留一句面向实现的表述；设计与产品文档改为引用 |
| D5 | 程序分支依赖稳定 `code` / `details` / 枚举，不比对可读 `message` | `server/AGENTS.md`；`web/AGENTS.md`；`contracts/AGENTS.md`；`launcher/AGENTS.md`；`docs/dev/text-resources.md` | `contracts/AGENTS.md` 定义语义；各实现目录一句 |
| D6 | 凭据不进入配置响应、fixtures、examples、日志、文档、快照 | 根 `AGENTS.md`；`server/AGENTS.md`；`fixtures/README.md` 两处；`examples/README.md`；`logging.md` 末段；`diagnostics.md` 敏感信息边界；`plugin-management-surface.md`；`launcher/AGENTS.md`；`check-agent-docs` 正则 | 根 `AGENTS.md` 一处即可覆盖所有产物；其余保留领域细节（如日志脱敏方式），删除泛化重述 |
| D7 | 不引入平行技术栈 / 第二套 client、组件系统、路径模型 | 根 `AGENTS.md`；`baseline.md` 当前评估方向；`web-admin-baseline.md` 约束四条；`web/AGENTS.md`；`launcher/AGENTS.md`；`DESIGN.md` Don't 两条；项目规划 Frozen stack；`PRODUCT.md` 受控扩展；`repo-workflow.md` | 根 `AGENTS.md` + `baseline.md` 新依赖五问；局部文件只列本目录的具体禁止对象 |
| D8 | 测试对应真实风险；普通文案、样式微调不新增测试 | 根 `AGENTS.md` Testing 两条；`editing-final-state-content` 末段；`server/AGENTS.md` 按风险分层；项目规划 Evidence-based quality | 根 `AGENTS.md` |
| D9 | 最小验证并确认产物；只有 exit code 不能证明真实产物时继续检查 | 根 `AGENTS.md` 末条；`implementation-order.md` §9；PR 模板；`quality-gates.md`；Codex 记忆“运行最小充分验证” | 根 `AGENTS.md` + `quality-gates.md` 给出按改动面的最小验证表 |
| D10 | 提交：Conventional Commits、中文、非空正文、一个逻辑变更、保留无关脏改动 | 根 `AGENTS.md` Git and Review；`execution-plan-v0.6` 文档状态；Codex 记忆六条 preferences 与 General Tips 的 hash 校验流程；Claude 记忆“默认合并到 main” | 根 `AGENTS.md`；执行计划删除该句；记忆属于用户侧，不由仓库管理 |
| D11 | 设计反例：嵌套卡片、hero 指标、装饰眉题、渐变文字、彩色侧边条、玻璃只用于浮层、44px 目标、reduced-motion | `DESIGN.md` Don't；`PRODUCT.md` Anti-references；`web-management-ui.md` 验收条件；`launcher-design-system.md` 验收条件；`plugin-management-surface.md` 两处；impeccable `craft-floor.md` Refuse | `DESIGN.md` Do's and Don'ts 为唯一清单；三份界面规范的验收条件只保留本界面特有项 |
| D12 | Web 工作区事实：`viewKey`、页签规则、偏好版本 3、插件卡片列数与高度、日志详情行为 | `web-admin-baseline.md` 工作区与 keep-alive 规则；`DESIGN.md` Components；`web-management-ui.md` 应用壳与页面组合 | 每条事实选一个 owner；当前三处并存导致改一处必须同步三处 |
| D13 | Web 视觉验证不得绕过鉴权、改 router guard 或用日志反推状态 | `web/AGENTS.md` Browser Verification；`web-testing.md` 末段；`web-admin-baseline.md` 约束 | `web/AGENTS.md`；`web-testing.md` 引用 |
| D14 | 生成物不手改、按输入链生成、CI 检查漂移 | `launcher/AGENTS.md`；`server/AGENTS.md`；`web/AGENTS.md`；`implementation-order.md` §2、§7；`docs/plugin/sdk/README.md` | 各目录只保留本目录的生成命令 |
| D15 | 冲突时以契约为准、领域内决定后同步 companion | 根 `AGENTS.md` Source of Truth；`baseline.md` 目的；`contracts/README.md` 通用规则；`docs/AGENTS.md`；`docs/design/README.md` | 根 `AGENTS.md` |
| D16 | `IMPECCABLE_CONTEXT_DIR` 设置方式 | 根 `AGENTS.md` Working Entrypoints；`docs/design/README.md` 设计工具 | `docs/design/README.md` |
| D17 | Windows 执行仓库命令使用 `gbash -lc` | `baseline.md` Shell；`docs/dev/README.md`；`~/.codex/rules/default.rules` | `baseline.md`，并注明适用宿主 |
| D18 | 每份局部 `AGENTS.md` 首句“先遵守根 `AGENTS.md`” | 五份局部文件 | 根 `AGENTS.md` 已声明逐层叠加，可删除首句（不影响门禁） |

Impeccable 相关重复：`.agents` / `.claude` / `.gemini` 三份 skill 副本、三个 harness 的 hook manifest、`DESIGN.md` 前置数据与 `.impeccable/design.json`、`design/tokens.json` 生成链。副本版本已分叉（4.2.2 与 4.1.1），仅 `.agents` 副本受版本控制。

## 三、容易导致过度防御的要求

“过度防御”指规则会把 agent 推向：小改动也走完整审计、为求稳触碰大量配套文件、动作前反复确认、为不需要的状态加锁或错误路径、把设计文档中的现状描述当作不可动的约束。以下按影响面排序。

| 编号 | 来源 | 触发的行为 | 建议 |
| --- | --- | --- | --- |
| O1 | 契约优先规则在 10 处以上重述（D1）；`baseline.md` 末段“绕开 baseline 与 contracts 直接写功能代码视为违反仓库治理规则”；PR 模板与 `contract-audit` | 内部重构或修 bug 也会先扫一遍 `contracts/`、fixtures、examples、生成链；对“是否算对外语义”拿不准时默认走重路径 | 保留 `contract-audit` 描述中“纯内部调整不默认启动全面契约审计”并提升到根 `AGENTS.md` Hard Rules；删除 `baseline.md` 的“违反治理规则”措辞；其余重述改为指针 |
| O2 | 根 `AGENTS.md` “完成前运行能证明本次改动正确性的最小验证”，但唯一的验证清单是 `implementation-order.md` §9 的九项发布级清单；`Makefile` 所有测试目标先跑 `doctor`，版本不精确匹配即失败 | 小改动也可能被引向 race、E2E、release smoke；本机版本不符时 `make test` 直接失败，agent 要么绕过 Makefile 要么放大说明 | 在 `quality-gates.md` 增加“按改动面的最小验证”表（Go 包 → `go test ./<pkg>/...`；Web 组件 → 对应 vitest；契约 → strict validator + 受影响生成器；文档 → `check-doc-links.py`），根 `AGENTS.md` 指向它；说明 `implementation-order.md` §9 是发布验收而非日常验证 |
| O3 | 根 `AGENTS.md` Hard Rules “可能并发读写的共享可变状态必须由原子快照或锁保护”；`server/AGENTS.md` 热更新串行化；`implementation-order.md` §4 六条 | 规则在根级别对 Web / Launcher 也生效；agent 可能给非共享状态加锁，或改写已有并发模式 | 移到 `server/AGENTS.md`，并限定“新增共享状态”；根级只保留 Source of Truth 指针 |
| O4 | `server/AGENTS.md` “不在业务方法中通过接收者 nil 检查静默降级或假成功” | Claude 记忆记录了 2026-09-10 一轮大范围删除 nil 检查后仍刻意保留了四处（`plugins/settings`、`render.Roots`、`plugins/runtime.Handle`、dispatch sender）；规则未说明可选依赖如何表达，agent 会在两个方向上过度改动 | 补一句“可选依赖用显式 no-op 实现或 Option 表达；既有保留的 nil 检查不主动清理” |
| O5 | 契约变更配套链散落在 `contracts/AGENTS.md`、`fixtures/README.md`、`examples/README.md`、`implementation-order.md` §2、`docs/plugin/sdk/README.md`、`web/AGENTS.md`、`launcher/AGENTS.md`：x-fixtures 登记、≥1 条 ok/invalid/edge、`http/index.yaml` 登记、strict validator、`generate-plugin-wire.py`、`generate-runtime-schemas.mjs`、`pnpm generate:types`（Web 与 Launcher 各一次）、`--verify` | 没有单一清单，agent 为防漏项会把每条链都跑一遍，或反复核对七个文档 | 在 `contract-audit` 或 `contracts/AGENTS.md` 放一份“契约变更配套清单”，其余文档引用 |
| O6 | UI 工作的叠加约束：impeccable PostToolUse 对每次 UI 文件编辑给出发现，Stop 事件跑全量规则；`craft-floor.md` Verify 与 Refuse 清单；`DESIGN.md` 411 行像素级规则；三份界面规范各自的验收条件（WCAG 2.2 AA、forced-colors、reduced-motion、44px、不透明降级）；`web/AGENTS.md` Browser Verification | 一次样式微调会经历 hook 提示 → craft-floor 自检 → 设计文档验收 → 浏览器验证要求四层；impeccable 自身文档承认逐次提示会让模型趋于保守 | 设 `.impeccable/config.json` 的 `hook.quiet: true` 或保持默认两级规则；`DESIGN.md` 中属于实现现状的数值（弹窗留白、z-index、行高估算、列数表）迁回 `web-admin-baseline.md` 或代码注释，`DESIGN.md` 只保留决策性 Named Rules；三份界面规范的验收条件去重（D11） |
| O7 | `contracts/AGENTS.md` Definitions：不写未来能力、不留占位名、不因实现偏差放宽契约；`fixtures/README.md` “任何会影响行为判断的变更，都应至少补一条 ok / invalid / edge case” | 对契约的任何措辞修正都会被理解为需要新 fixture；对已有实现偏差，agent 倾向改实现而不敢先讨论契约是否本来就错 | 保留规则，在 `contract-audit` 明确“描述性修正无需新增 fixture；契约本身可能有误时先向用户确认” |
| O8 | 根 `AGENTS.md` “不升级冻结版本线，除非任务明确要求并同步 baseline、工程文件、lockfile、CI 与发布说明”；`baseline.md` 新依赖五问 | 需要一个小型开发依赖时 agent 可能放弃或改用手写实现 | 保留；补一句“开发期工具依赖按 `server/AGENTS.md` 的独立工具边界处理，不触发版本线规则” |
| O9 | `docs/AGENTS.md` “契约或正式行为变化影响文档时，同轮更新对应说明”；`editing-final-state-content` 要求完成后逐段检查 | 一处行为变化可能触发对全部 docs 的扫描；Codex 记忆“只修改确有漂移的文档”正是对此的纠偏 | 把“只修改确有漂移的文档，历史归档不动”写进 `docs/AGENTS.md`，替代逐段检查的表述 |
| O10 | `check-agent-docs` 的反引号路径存在性检查与 secret 正则 | 编写指令时会回避反引号路径或把示例值改得不自然；对 `token`、`auth` 等词附近的长串会误报 | 保留；在 `AGENTS.md` Instruction Maintenance 说明检查内容，避免作者猜测 |
| O11 | `docs/execution-plan-v0.6-conversation.md` 100 处约束用语，位于 `docs/` 根目录且未被任何索引或 `AGENTS.md` 引用 | 处理会话、派发、KV 相关任务的 agent 会把候选设计当作现行约束 | 文档状态已声明“不代表功能已上线”；建议 `docs/AGENTS.md` 明确“执行计划只约束其标题版本的任务” |
| O12 | Codex 记忆中的 preferences：“For diagnosis and audits, investigate first ... do not edit code unless asked”、“report only reproducible file/line evidence”、“state unrun coverage” | 这些偏好来自审计类任务，但 `use_memories = true` 会在实现类任务中一并注入，可能让 Codex 在普通实现任务里先停下来报告而不动手 | 属于用户侧配置；若希望实现任务更直接，可在 `~/.codex/AGENTS.md` 增加一句“实现任务直接改动并验证，审计任务才只报告” |
| O13 | `web/AGENTS.md` Browser Verification 三条 | 针对已知失败模式（登录页截图冒充目标页、改 guard 取截图），具体且必要 | 保留；属于合理防御 |
| O14 | 系统级 `security-best-practices`、`security-threat-model`、`playwright`、`vite` skill | 均声明只在明确要求时触发或不主动升级；不会加剧防御 | 无需处理 |

## 四、冲突与失效项

| 编号 | 发现 | 证据 | 影响 |
| --- | --- | --- | --- |
| C1 | Codex 的 impeccable hook 指向不存在的文件 | `.codex/hooks.json` 命令为 `.agents/skills/impeccable/scripts/hook.mjs`，`.agents/skills/impeccable/scripts/` 只有 `impeccable` launcher 与 live-browser 脚本 | Codex 会话中设计检测器从未运行；hook 用 `[ ! -f ] ||` 守卫，无报错 |
| C2 | impeccable 三份副本版本分叉 | `.agents` 为 4.2.2（launcher 二进制形态），`.claude` 与 `.gemini` 为 4.1.1（Node 脚本形态），且 `.claude` 与 `.gemini` 的 `SKILL.md` 互不相同 | 同一设计任务在不同 harness 下走不同流程；根 `AGENTS.md` 说“外部 skill 由上游维护”，但只有 `.agents` 副本在版本控制内，另两份为本机状态 |
| C3 | Claude Code 不加载 `.agents/skills/` | 会话可用 skill 列表不含 contract-audit、editing-final-state-content | 根与 `docs/AGENTS.md` 对 Claude 的 skill 引用只能靠显式读文件；措辞“按任务需要使用”对 Claude 不成立 |
| C4 | README 与用户文档对默认监听地址矛盾 | `README.md` 第 67 行“管理面板默认只在本机开放，远程访问需在配置中显式开启”；`docs/user/management-surface.md`、`docs/user/deployment.md`、`baseline.md` 已冻结决议与项目规划均为默认 `0.0.0.0` | 非指令冲突，但会让 agent 在配置与安全相关任务中得出相反的“默认值” |
| C5 | `gbash -lc` 规则对 Git Bash 宿主多余 | `baseline.md` Shell 节与 `docs/dev/README.md` 要求 Windows 使用 `gbash`；Claude Code 的 Bash 工具本身就是 Git Bash | agent 可能在已是 bash 的环境里再包一层 `gbash`；规则应限定 PowerShell / cmd 宿主 |
| C6 | 本机权限白名单与 Codex 规则含失效路径 | `.claude/settings.local.json` 含 `/c/Users/26789/...` 路径、两条一次性 `sed -i` 命令，`Bash(git:*)` 已覆盖四条更窄的 git 规则；`~/.codex/rules/default.rules` 两条 dotnet 规则指向 `C:\Users\26789\...` | 无功能影响，属于噪音 |
| C7 | 局部 `AGENTS.md` 的“先遵守根 `AGENTS.md`”与根文件的叠加声明重复 | 五份局部文件首句 | 无冲突，可删 |

## 五、收敛建议（按优先级）

1. **单一归属**：按第二节“建议归属”列，把 D1、D2、D3、D4、D9、D15 收敛到根 `AGENTS.md` 与 `contracts/AGENTS.md`，其余文档保留一句指向；`6ae3c608`、`f80d1cd8` 已按这个方向清理，剩余重述集中在 `baseline.md`、`implementation-order.md`、`fixtures/README.md`、PR 模板、`web-admin-baseline.md` 与设计文档。
2. **最小验证表**：在 `quality-gates.md` 增加按改动面的最小验证表，并在根 `AGENTS.md` 引用；明确 `implementation-order.md` §9 是发布验收清单。
3. **契约配套清单**：在 `contract-audit` 或 `contracts/AGENTS.md` 集中列出 x-fixtures、examples 登记、strict validator、三条生成链与 `--verify` 的顺序，删除各处零散重述。
4. **并发与 nil 规则下沉**：把共享状态锁规则从根 Hard Rules 移入 `server/AGENTS.md`，并补充可选依赖的表达方式与“既有保留的 nil 检查不清理”。
5. **设计文档分层**：`DESIGN.md` 只保留 Named Rules 与 Do's and Don'ts；像素级现状描述由 `web-admin-baseline.md` 或代码持有；三份界面规范验收条件去重。
6. **修复 hook 与副本**：为 Codex 提供有效的 hook 入口或删除 `.codex/hooks.json`；统一三份 impeccable 副本版本，或在根 `AGENTS.md` 说明只有 `.agents` 副本受管。
7. **措辞限定**：`baseline.md` 删除“视为违反仓库治理规则”；`gbash` 规则限定 PowerShell / cmd 宿主；`docs/AGENTS.md` 明确执行计划只约束其标题版本，并写入“只修改确有漂移的文档”。
8. **顺带修正**：README 第 67 行默认监听地址与其他文档对齐。

## 附：核对方法与未覆盖范围

- 约束密度以 `必须|不得|禁止|不能|严禁|不允许|不要|不可|须|不应|不默认|不为|不在|不把|不通过|不使用|不新增|不引入|不复制|不手写|不另建|不吞|不做` 在每份 Markdown 中的出现次数计，仅用于排序阅读优先级，不作为结论依据。
- 运行 `node scripts/check-agent-docs.mjs` 确认当前指令文件通过结构门禁。
- 未覆盖：`docs/plugin/`、`docs/user/`、`docs/release/`、`docs/CHANGELOGS/` 的正文只抽查了与指令交叉的部分；`server/`、`web/`、`launcher/` 源码内注释中的约束未纳入；`~/.codex/memories/rollout_summaries/` 的逐篇内容未读取，只依据 `MEMORY.md` 与 `memory_summary.md` 的汇总。

## 处理记录（2026-09-13）

本轮处理仓库内跟踪的指令、skill 与治理文档；本机 agent 配置与用户目录中的项目未改动。

| 编号 | 处理结果 |
| --- | --- |
| D1、D2、D3、D9、D15 | 根 `AGENTS.md` 保留契约规则，并补充“正式语义不变的内部调整不需要契约改动”；删除 `baseline.md`、`implementation-order.md`、`quality-gates.md`、`contract-audit` 与 PR 模板中的重复表述 |
| D4、D5 | 状态来源与错误分支规则移入根 `AGENTS.md`；删除 `server/`、`web/`、`contracts/` 局部指令与 `web-admin-baseline.md` 约束节中的重复条目；`implementation-order.md` 第 7 节中的日志推断状态表述也已删除 |
| D6 | `server/AGENTS.md` 只保留 secret store 的专属要求 |
| D7 | 复核后未改：根 `AGENTS.md` 保留通用规则，其余文档只列本领域的具体对象，符合原建议的归属方式 |
| D8 | 删除 `editing-final-state-content` 中与根 Testing 重复的测试要求 |
| D10 | 删除 v0.6 执行计划中的提交格式要求 |
| D11 | 三份界面规范的验收条件改为引用 `DESIGN.md` 的 Do's and Don'ts；插件管理页规范的官方页面布局节保留逐条禁用项，读者是独立插件仓库 |
| D12 | 页签显示规则归 Web 界面规范，缓存与恢复语义和偏好版本归 Web 工程基线，`DESIGN.md` 的插件中心页签规则改为引用 Web 界面规范；修正 Web 界面规范把只读快捷键列表写成偏好项的描述 |
| D13 | `web-testing.md` 改为链接 `web/AGENTS.md` 的 Browser Verification |
| D14、D16 | 复核后未改：各目录只写本目录的生成命令，根 `AGENTS.md` 对设计工具环境变量只保留指向 |
| D17、C5 | 已由 `23acb63e` 移除 gbash 包装与相关要求 |
| D18、C7 | 删除五份局部 `AGENTS.md` 首句的“先遵守根 AGENTS.md” |
| O1 | 删除 `baseline.md` 末节“视为违反仓库治理规则”的表述，契约来源说明并入仓库级强制基线文件表 |
| O2、O5 | `quality-gates.md` 新增“按改动面的最小验证”表，覆盖契约门禁与生成器命令；根 `AGENTS.md`、`contracts/AGENTS.md`、`contract-audit` 与 PR 模板引用该表；`implementation-order.md` 第 9 节注明为发布验收范围；`Makefile` 测试目标依赖 doctor 的行为未改，本机工具链检查通过，日常验证不经过 Makefile |
| O3 | 复核后保留在根 `AGENTS.md`：Launcher `internal/desktop` 同样持有并发读写的共享状态 |
| O4 | nil 检查规则限定为新增的业务方法 |
| O7 | `contract-audit` 注明只调整描述的契约修改不需要新增 fixture，怀疑契约本身有误时向用户确认 |
| O8 | `baseline.md` 的新依赖说明要求限定为运行时依赖与固定选型替换 |
| O9、O11 | `docs/AGENTS.md` 写入“只修改确有漂移的内容”与“执行计划只约束对应版本的任务” |
| O10 | 根 `AGENTS.md` 说明 agent-docs 检查脚本的检查内容 |
| O12 | Codex 全局 `AGENTS.md` 增加执行方式：实现与修复类任务直接修改，诊断、审计和评审类任务先报告；Codex 记忆中的 gbash 建议与已删除报告的引用已加注 |
| O6 | 设计检测 hook 默认已把逐次编辑限定为即时规则、完整规则留到 Stop 事件，未改配置。`DESIGN.md` 是 Impeccable 读取的设计上下文，未迁出组件细节；改为修正界面规范中与其冲突的重复描述，见 C8、C9 |
| C1 | `impeccable hooks on` 已修复 Codex 的 hook 配置；Claude 的 hook 改为调用 `.agents/skills/impeccable/scripts/impeccable` 的 `hook` 子命令，已通过管道测试与实际触发验证，引擎二进制与发布页 sha256 一致 |
| C2 | `.claude/` 与 `.gemini/` 下的旧版副本已在 2026-09-12 被移除，版本分叉不再存在 |
| C3 | `CLAUDE.md` 说明 Claude Code 需直接读取 `.agents/skills/` 中的 `SKILL.md` |
| C4 | 已由 `ed390c77` 修正 README 默认监听地址 |
| C6 | 删除 Claude 本机权限白名单中 10 条失效或冗余的规则，以及 Codex 规则文件中 2 条指向旧用户目录的 dotnet 规则 |
| C8 | 新发现：`web-management-ui.md` 与 `plugin-management-surface.md` 称 Web 浮层使用 12px 背景模糊，`DESIGN.md` 与 Web 代码均为不透明表面；已按代码修正 |
| C9 | 新发现：`web-management-ui.md` 称桌面控件默认高度 36px，Web 的 `AppButton`、`AppInput`、`AppSelect` 实为 40px；已改为引用 `DESIGN.md` Components |
| C10 | 新发现：Impeccable doctor 按文件修改时间报告 `.impeccable/design.json` 过期；sidecar 内容由仓库生成器维护且与 `DESIGN.md` 一致，刷新文件修改时间后不再报告。该判断依赖本地文件时间，重新检出后可能再次出现 |

尚未处理：

- Codex 首次使用新的 hook 配置前需要在 Codex 的 `/hooks` 中批准，按用户决定暂不处理。
