# RayleaBot Repository Guide

## Purpose

RayleaBot 是一个面向个人开发者和 GitHub 开源协作者的自托管聊天机器人框架。
本文件只保留仓库级、长期有效的硬规则。进入具体目录后，需要继续遵守该目录就近的 `AGENTS.md`。
可复用工作流放在 `.agents/skills/`；一次性任务要求以用户当前请求为准。

## Always

- 触及 HTTP / WebSocket / schema / errors / events / CLI / release metadata：先改 `contracts/`，再改实现。
- 触及 `server/`、`web/`、`launcher/`、storage、render 的实现设计或依赖选择：先读 `.agents/skills/glue-coding/SKILL.md`
- 触及文档、注释或用户可见文本：先读 `.agents/skills/editing-final-state-content/SKILL.md`
- 边界不清、跨面或复杂任务：先读 `.agents/skills/phase-boundary-check/SKILL.md`，必要时先出计划，不直接落代码
- 完成前运行与改动面直接相关的最小验证，并同步 tests / fixtures / examples / docs / generated files
- 非必要不写兜底代码，避免过度设计，过度防御。

## Instruction Layers and Source of Truth

- 当前任务目标与交付范围以用户请求为准。
- `AGENTS.md` 按仓库根到当前目录逐层叠加，越近越优先；局部文件只能补充或收窄规则，不能弱化根规则。
- 产品目标、范围、顶层架构与路线图：`docs/RayleaBot机器人项目规划.md`
- 当前正式 execution plan 默认以最新版本的 `docs/execution-plan-v*.md` 为准；
  `docs/CHANGELOGS/v*.md` 仅作为历史基线归档。
  若用户明确指定某个 execution plan 文件，按指定文件执行，并在完成后回写对应文件。
- 对外接口、schema、错误码、事件、CLI、发布元数据：`contracts/`
- 工程基线、固定版本线、默认命令、目录职责：`docs/engineering/baseline.md`与对应工程文件
- 实施顺序与边界判断：`docs/engineering/implementation-order.md`
- CI 门禁与发布验证：`.github/workflows/*.yml`、`docs/engineering/quality-gates.md`
- 目录特有规则：`server/AGENTS.md`、`contracts/AGENTS.md`、`fixtures/AGENTS.md`
- 可复用流程skill说明：`.agents/skills/`、`~\.agents\skills`
- `docs/architecture/` 负责架构、状态模型、事件模型与跨层边界说明。

补充规则：

- `contracts/` 是对外边界的唯一正式来源；实现、README、fixtures、examples 都不能反向裁决它。
- `fixtures/` 与 `examples/` 只能从 `contracts/` 派生。
- `references/` 只提供参考资料，不裁决正式行为。
- 局部 `AGENTS.md` 只能补充所在目录规则，不能弱化本文件。
- `.agents/skills/` 只承载流程，不承载项目真相来源。

## Read the Right Files First

按任务选最短阅读路径，避免在大仓库里无差别扫全文：

- 变更 API、schema、状态、错误码、事件、配置、CLI、发布元数据：先读 `contracts/README.md`、相关 contract、对应 `fixtures/`、`examples/`、tests、docs
- 修改服务端：先读 `server/README.md`、`server/AGENTS.md`，再进入 `server/internal/<area>/`
- 修改 Web：先读 `web/package.json`、`web/src/lib/http.ts`、`web/src/lib/ws.ts`、`web/src/stores/`、`web/src/views/`
- 修改 Launcher：先读 `launcher/package.json`、`launcher/src/main/services/`、`launcher/src/shared/`、`launcher/src/renderer/src/`
- 修改插件协议、插件 manifest 或 SDK：先读 `docs/plugin/README.md`、`contracts/plugin-info.schema.json`、`contracts/plugin-protocol.schema.json`、`sdk/`
- 修改渲染模板或渲染链路：先读 `docs/architecture/render-service.md`、`templates/`
- 修改发布、打包、恢复演练、自托管 smoke：先读 `docs/release/`、`scripts/release/`、`.github/workflows/release.yml`、`.github/workflows/self-host-smoke.yml`
- 修改仓库协作、诊断或忽略策略：先读 `docs/dev/`
- 修改文档、注释或用户可见文案：先读 `.agents/skills/editing-final-state-content/SKILL.md`
- 选择依赖、框架、抽象层或复用策略：先读 `.agents/skills/glue-coding/SKILL.md`
- 边界是否允许、是否抢跑后续能力不清楚：先读 `.agents/skills/phase-boundary-check/SKILL.md`
- 需要做 contract 漂移审计：先读 `.agents/skills/contract-audit/SKILL.md`

## Commands

只跑能证明当前改动正确性的最小命令集；没有根级统一命令，始终在对应子工程目录执行。

### Server

- 安装：无
- 构建：`cd server && go build ./cmd/raylea-server`
- 测试：`cd server && go test ./...`
- 若改动涉及 SQL 查询或 `sqlc` 生成物：`cd server && go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 && sqlc diff`

### Web

- 安装：`cd web && pnpm install --frozen-lockfile`
- 类型检查：`cd web && pnpm run typecheck`
- 测试：`cd web && pnpm test`
- CI 覆盖率门禁：`cd web && pnpm test:coverage`
- 构建：`cd web && pnpm build`
- 若 OpenAPI 或 WebSocket contract 变更：`cd web && pnpm generate:types`

### Launcher

- 安装：`cd launcher && pnpm install --frozen-lockfile`
- 类型检查：`cd launcher && pnpm run typecheck`
- 测试：`cd launcher && pnpm test`
- CI 覆盖率门禁：`cd launcher && pnpm test:coverage`
- 构建：`cd launcher && pnpm build`
- 若 OpenAPI 变更：`cd launcher && pnpm generate:types`

### Node.js SDK

- 安装：`cd sdk/nodejs && npm ci`
- 构建：`cd sdk/nodejs && npm run build`
- 类型检查：`cd sdk/nodejs && npm run typecheck`
- 测试：`cd sdk/nodejs && node --test tests/*.test.mjs`

### Python SDK and Python Tooling

- Python SDK 测试：`cd sdk/python && python -m unittest discover -s tests`
- 发布脚本测试：`python -m unittest discover -s scripts/release/tests`
- 启动脚本测试：`python -m unittest discover -s scripts/tests`
- 启动脚本 Node 测试：`node --test scripts/tests/start-dev-support.test.mjs`

### 规则

- Server、Web、Launcher 各自只保留一套默认入口。
- CI、发布和本地验证直接复用这些入口，不维护平行命令。
- Windows worktree 中 Go 测试或 benchmark 若因用户级 build cache `Access is denied` 失败，在实际执行目录创建本地 `.gocache`，设置 `GOCACHE` 后重跑同一条 Go 命令。

## Companion Updates and Verification

- 任一涉及协议、schema、状态机、配置、数据库结构、插件安装流程、渲染输入输出、Web API、WebSocket、错误码或迁移的改动，合并前必须满足五件套：
  - 实现代码
  - 相关 contract
  - 相关 tests
  - 相关 fixtures / examples
  - 必要的 docs 同步
- 不允许“代码先改，契约、测试、示例后补”。
- 若用户要求与 `docs/RayleaBot机器人项目规划.md` 冲突，必须同步修正文档。
- 若任务按执行计划文档实施，完成后必须同步回写对应执行计划。

额外要求：

- 修改 `contracts/web-api.openapi.yaml` 时，必须同步更新 `web/src/types/generated.ts` 与 `launcher/src/shared/web-api.generated.ts`
- 修改 `contracts/websocket-events.yaml` 时，必须同步更新 `web/src/types/websocket.generated.ts`
- 变更文档、README、注释、按钮文案、错误文案或其他用户可见文本时，输出最终态内容，不保留编辑过程叙事
- 若当前任务明确按execution plan文件推进，完成后同步更新对应计划文档
- 完成前至少执行与改动面直接相关的构建、测试、类型检查或生成物一致性检查
- 只有 exit code 不足以证明结果正确时，继续检查目标产物、生成文件或运行时效果是否真实存在

## Non-Negotiable Engineering Rules

- 对外边界一律走 contract-first。任何新增字段、状态、错误码、事件名、命令模型、消息类型，先更新 `contracts/`，再进入实现。
- 保持单一真相来源。Server、Web、Launcher、CLI、fixtures、examples、docs 使用同一套正式状态名、任务状态、错误码与接口字段。
- `fixtures/` 与 `examples/` 只能表达已冻结结构，不能抢跑新能力。
- 若所需对外边界还未冻结，先补 formal contract；必要时只允许窄范围骨架和显式 TODO，不能直接跳去写实现主链。
- 默认使用严格复用阶梯：`仓库现有代码与冻结选型 > 标准库或平台内建能力 > 已冻结官方依赖 > 成熟且依赖面最小的 OSS > 薄胶水层自定义代码`。
- 只在现有实现与固定选型不足时引入新依赖，并说明原因。
- 新依赖必须许可清晰、维护稳定、生产验证充分、依赖面最小，并通过现有 lockfile 或工程文件锁版本。
- 不新增平行栈，除非现有冻结选型被明确证明不足。
  - Server：不要再造第二套路由、ORM、日志栈
  - Web：不要再造第二套 HTTP client、WebSocket client、状态管理、组件系统
  - Launcher：不要复制 Web 业务逻辑，不要再造第二套桌面 service layer 或设计系统
- Web 与 Launcher 只能消费正式 HTTP / WebSocket 面，不通过解析日志推断真实状态。
- 插件只能通过正式 local action surface 访问平台能力，不直接读写 `config/user.yaml`，不绕过 Capability 校验。
- 涉及鉴权、配置、日志、fixtures、examples、docs 、测试时，禁止提交或暴露真实 secrets、token、凭据。
- 未经明确需求，不升级 Go / Node / pnpm / Electron / TypeScript / Vite / Vue / React / SQLite / chromedp 等冻结版本线；
  若必须升级，同步更新 baseline、lockfile、CI 与发布文档。
- `web/src/types/generated.ts`、`web/src/types/websocket.generated.ts` 与 `launcher/src/shared/web-api.generated.ts` 属于生成物；
  相关 contract 变更后通过对应 generate 命令重生成，不手工编辑。

## Documentation and User-Facing Text

- 任何修改文档、README、注释、错误文案、按钮文案或其他用户可见文本前，先读取 `.agents/skills/editing-final-state-content/SKILL.md` 并按该技能执行。
- 默认输出最终态成稿，只保留读者当前需要知道的事实、约束、操作和结果。
- 除非文件本身就是变更记录、迁移说明或发布说明，不保留编辑痕迹或过程叙事。
- 禁止出现这类措辞：`不再`、`已改为`、`已移到`、`前后对比`、`不会再`、`这里改成`、`原来`、`之前`、`现改为`。
- 非必要不解释实现顺序、数据流拼接顺序或接口消费顺序。

## Skill Discovery and Reuse

- 在计划或编码前，先检查当前环境是否已有合适 skill。
- 任务涉及文档、注释或用户可见文案时，优先使用 `editing-final-state-content`。
- 任务涉及 `server/`、`web/`、`launcher/`、storage、render 的实现设计、依赖选择或跨面改动时，优先使用 `glue-coding`。
- skills 是工作流助手，不是正式来源；`contracts/`、规划文档和 `AGENTS.md` 优先。

## Git and Review Hygiene

- 提交信息遵守 Conventional Commits：`<type>[optional scope]: <description>`
- 推荐 scope：`server`、`web`、`launcher`、`contracts`、`fixtures`、`docs`、`sdk`、`release`
- 一个 commit 只表达一个逻辑变更；跨 contract / implementation / docs / CI 的混合改动，只有在它们共同冻结同一表面时才放进同一个 commit
- subject 说明具体改动，不写“阶段推进”“杂项整理”“several updates”这类进度汇报式标题
- 需要说明原因、风险、迁移影响或验证方式时，再补 commit body
- Windows worktree 若触发 dubious ownership，只读 Git 检查使用 `git -c safe.directory=<repo> ...`，不要为临时扫描改全局 Git 配置。

## Delete Rules

- 删除任何文件或目录时，禁止使用 PowerShell。

## Keep This File Useful

- 根 `AGENTS.md` 只保留仓库级规则；目录特有规则写到更近的 `AGENTS.md`
- 可复用流程优先沉淀为 `.agents/skills/`，不要把长流程直接堆在根文件
- 同一类错误、同一类 review feedback 或同一类误读重复出现时，把修正写入最近的 `AGENTS.md` 或对应 skill
- 保持内容短、准、可执行；模糊口号不如准确命令和明确边界
- 应主动写入，避免询问用户。当用户反复提及相关流程、错误，主动写入最近的 `AGENTS.md` 或创建、修改相关 skill
