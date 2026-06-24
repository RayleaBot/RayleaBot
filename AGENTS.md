# RayleaBot Repository Guide

## Project

RayleaBot 是一个面向个人开发者和 GitHub 开源协作者的自托管聊天机器人框架，包含 `server/`、`web/`、`launcher/`、`contracts/` 等子工程。
本文件只保留仓库级、长期有效的硬规则。进入具体目录后，需要继续遵守该目录就近的 `AGENTS.md`。

## Instruction Order

- 用户当前请求定义本轮目标和交付范围。
- `AGENTS.md` 按仓库根到当前目录逐层叠加，越近越优先；局部文件只能补充或收窄规则，不能弱化根规则。
- Claude 通过 `CLAUDE.md` 导入 `AGENTS.md`；有局部 `AGENTS.md` 的目录提供同级 `CLAUDE.md` bridge。
- 可复用工作流放在 `.agents/skills/`；一次性任务要求以用户当前请求为准。

## Hard Rules

- Contract-first: 触及 HTTP / WebSocket / schema / errors / events / CLI / release metadata，先改 `contracts/`，再改实现。
- Single source of truth: 状态名、错误码、事件名、字段名、配置键名只维护一套正式语义。`contracts/` 是对外边界的唯一正式来源；实现、README、fixtures、examples 都不能反向裁决它。
- Companion updates: 涉及协议、schema、状态机、配置、数据库结构、插件安装流程、渲染输入输出、Web API、WebSocket、错误码或迁移的改动，合并前同步实现代码、contract、tests、fixtures/examples、必要的 docs。
- Frozen stack: 不新增平行技术栈，不升级冻结版本线，除非任务明确要求并同步 baseline、lockfile、CI 与发布文档。
- Secrets: 不在配置响应、fixtures、examples、logs、docs、测试快照中暴露真实 token、secret、凭据。
- Minimal verification: 完成前运行能证明本次改动正确性的最小验证。只有 exit code 不足以证明结果时，继续检查目标产物、生成文件或运行时效果是否真实存在。

## Source of Truth

- 对外接口、schema、错误码、事件、CLI、发布元数据：`contracts/`
- 工程基线、固定版本线、默认命令：`docs/engineering/baseline.md`
- 实施顺序与边界判断：`docs/engineering/implementation-order.md`
- 架构、状态模型、事件模型与跨层边界：`docs/architecture/`
- 用户操作与管理面：`docs/user/`
- 产品目标、范围、顶层架构与路线图：`docs/RayleaBot机器人项目规划.md`
- 当前正式 execution plan 默认以最新版本的 `docs/execution-plan-v*.md` 为准；若用户明确指定，按指定文件执行，并在完成后回写对应文件。

## Read First

- 修改服务端：`server/README.md`、`server/AGENTS.md`、相关 contracts/docs。
- 修改 Web：`web/AGENTS.md`、`web/package.json`、相关 generated types 与 contracts。
- 修改 Launcher：`launcher/AGENTS.md`、`launcher/package.json`、相关 generated types 与 contracts。
- 修改 contract：`contracts/AGENTS.md`、`contracts/README.md`、对应 fixtures/examples/tests/docs。
- 修改文档：`docs/AGENTS.md` 与 `editing-final-state-content` skill。
- 选择依赖、框架、抽象层或复用策略：`glue-coding` skill。
- 边界不清、跨面或复杂任务：`phase-boundary-check` skill。
- 需要做 contract 漂移审计：`contract-audit` skill。

## Commands

只运行能证明当前改动正确性的最小命令集；在对应子工程目录执行。

- Server build: `cd server && mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server`
- Server test: `cd server && go test ./...`
- Web typecheck: `cd web && pnpm run typecheck`
- Web test: `cd web && pnpm test`
- Web build: `cd web && pnpm build`
- Launcher typecheck: `cd launcher && pnpm run typecheck`
- Launcher test: `cd launcher && pnpm test`
- Launcher build: `cd launcher && pnpm build`
- Agent docs check: `node scripts/check-agent-docs.mjs`

## Skills

- `contract-audit`: contract、fixture、generated type 或 API drift 检查。
- `glue-coding`: 跨面设计、依赖选择、复用策略。
- `phase-boundary-check`: 阶段边界不清或可能抢跑后续能力。
- `editing-final-state-content`: 文档、注释、用户可见文本。
- `agent-instruction-maintenance`: 修改 AGENTS/CLAUDE/skills。
- `repo-validation`: 选择最小验证命令和 drift 检查。

## Maintaining Agent Instructions

- 根文件短、稳、强约束。
- 目录专属规则放最近的 `AGENTS.md`。
- 多步骤流程放 `.agents/skills/`。
- 不引用不存在路径。
- 不把当前能力清单写进 AGENTS。
- 同一类错误、review feedback 或误读重复出现时，把修正写入最近的 `AGENTS.md` 或对应 skill。

## Git and Review Hygiene

- 提交信息遵守 Conventional Commits：`<type>[optional scope]: <description>`
- 推荐 scope：`server`、`web`、`launcher`、`contracts`、`fixtures`、`docs`、`sdk`、`release`
- 一个 commit 只表达一个逻辑变更；跨 contract / implementation / docs / CI 的混合改动，只有在它们共同冻结同一表面时才放进同一个 commit
- subject 说明具体改动，不写进度汇报式标题
- Windows worktree 若触发 dubious ownership，只读 Git 检查使用 `git -c safe.directory=<repo> ...`，不要为临时扫描改全局 Git 配置。

## Delete Rules

- 删除任何文件或目录时，禁止使用 PowerShell。
