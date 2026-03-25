# RayleaBot Repository Guide

## Project Overview

RayleaBot 是一个面向个人开发者和 GitHub 开源协作者的自托管聊天机器人框架。
本文件只保留每次代理开始工作前都必须遵守的仓库级规则；阶段边界、实施顺序和当前里程碑状态不在这里维护。

## Instruction Precedence

本仓库的单向优先级固定为：

`docs/RayleaBot机器人项目规划.md  > contracts/ > fixtures/examples > code`

补充规则：

- `contracts/` 是对外接口、schema、错误码和发行元数据的唯一正式来源。
- Markdown 用于解释设计意图，不是最终接口裁决依据。
- 若 Markdown 与 `contracts/` 冲突，以 `contracts/` 为准，并在同一变更中修正文档说明。
- 若某项契约尚未完整，也必须先产出正式骨架文件和显式 `TODO`，不能直接跳去写实现。
- 局部 `AGENTS.md` 只补充所在目录特有、长期有效的规则，不能弱化根规则。
- repo-local skills 只承载可复用工作流，不承载项目真相来源，也不能覆盖 `contracts/` 与正式文档。

## Repository Layout

- `contracts/`：对外 HTTP API、WebSocket、plugin manifest、plugin protocol、config schema、error codes、release manifest 的正式来源
- `docs/engineering/`：baseline、CI 门禁、实施顺序、仓库治理
- `docs/architecture/`：架构设计、状态模型、统一事件模型、跨层边界
- `docs/dev/`：本地开发、调试、诊断、贡献流程
- `docs/plugin/`：插件开发文档、manifest、Capabilities、协议与生命周期
- `docs/plugin/sdk/`：Python / Node.js 官方 SDK 文档
- `docs/user/`：安装、初始化、配置、运行、恢复与排障
- `docs/release/`：版本说明、迁移说明、已知问题
- `fixtures/`：golden cases、契约样例、回归基线
- `examples/`：示例插件、示例配置、示例请求/响应；只能演示已被 contract 确认的结构
- `server/`：Go 服务端工程
- `web/`：Web UI 工程
- `launcher/`：Windows Launcher 工程
- `.deps/`：Chromium、托管 Python / Node.js 运行时及相关资源清单
- `references/`：参考资料，不是正式契约来源

## Default Commands

### Server

- 构建：`go build ./cmd/raylea-server`
- 测试：`go test ./...`

### Web

- 安装：`pnpm install --frozen-lockfile`
- 开发：`pnpm dev`
- 构建：`pnpm build`
- 单元测试：`pnpm test`
- E2E：`pnpm test:e2e`

### Launcher

- 构建：`dotnet publish ./launcher -c Release`
- 测试：`dotnet test ./launcher`

规则：

- Server、Web、Launcher 各自只允许一种默认构建入口。
- CI 必须直接复用这些默认命令，不允许本地、CI、发布各维护一套不一致入口。

## Engineering Rules

- 对外边界优先走 contract-first：先定契约，再写实现。
- `fixtures/` 与 `examples/` 只能从 `contracts/` 派生，不能反向覆盖 `contracts/`。
- 不得在 `contracts/` 之外定义新的对外接口字段并直接实现。
- 不得在未更新 `contracts/` 的情况下引入新的状态名、新错误码、新目录职责、新接口。

禁止以下跨层捷径：

- Adapter 直接写状态库。
- Launcher 复制 Web 业务逻辑。
- Web UI 通过解析日志推断真实状态。
- 插件运行时直接读写 `config/user.yaml`。
- 插件绕过 Capability 校验直连平台内部模块。
- CLI、Launcher、Web UI 各自发明不同状态名、错误码或任务状态。

## Change Verification

任一涉及以下边界的变更：

- 协议
- schema
- 状态机
- 配置
- 数据库结构
- 插件安装流程
- 渲染输入输出
- Web API
- WebSocket
- 错误码
- 迁移

合并前必须同时满足四件套：

- 实现代码更新
- 契约更新
- 测试更新
- 示例更新

禁止“代码先改，契约/测试/示例以后补”。

如用户要求的修改涉及与现有`RayleaBot机器人项目规划.md`冲突的，必须修改文档使其与用户要求一致。

如果是按照`execution-plan.md`文档进行的操作，完成后必须更新该执行文档。

## Security and Secrets

- 不要硬编码、提交或在示例中放入真实 secrets、token、凭据。
- 不要在日志、fixtures、examples、docs 中输出敏感字段原值。
- 涉及鉴权、配置或外部连接时，优先遵守现有 contract 与 baseline 中已经冻结的最小权限和本地优先安全约束。

## Consult Deeper Docs

- 工程基线、版本线、固定命令：`docs/engineering/baseline.md`
- 实施顺序与阶段边界：`docs/engineering/implementation-order.md`
- 正式契约清单与契约级 TODO：`contracts/README.md`
- 架构、状态模型、事件模型、跨层边界：`docs/architecture/README.md`
- repo-local workflows：`.agents/skills/`

实施顺序与阶段边界见 `docs/engineering/implementation-order.md`。

## 文档与面向用户显示内容修改规则

- 默认输出“最终态成稿”，像该内容一开始就是这样写的。
- 除非当前文档本身就是变更记录、迁移说明或发布说明，否则不要保留编辑痕迹式表述。
- 禁止出现这类措辞：`不再`、`已改为`、`已移到`、`前后对比`、`不会再`、`这里改成`、`原来`、`之前`、`现改为`。
- 修改完成后，做一次清理检查：删除解释修改过程的句子，只保留最终应交付给读者的内容。
- 禁止在非文档区域残留开发态表述如`阶段1`、`第一步`、`phase 1`等。

## Git commit rules

- All commits must follow Conventional Commits:

  `<type>[optional scope][!]: <description>`

- Allowed types:
  - `feat`: new capability
  - `fix`: bug fix or correctness repair
  - `refactor`: internal restructuring without intended behavior change
  - `perf`: performance improvement
  - `docs`: documentation-only change
  - `test`: test-only change
  - `build`: build system, dependencies, packaging
  - `ci`: CI workflow or automation
  - `chore`: repository maintenance that does not fit the above

- Prefer scopes when helpful, especially repo-native scopes such as:
  `server`, `contracts`, `fixtures`, `docs`, `web`, `launcher`, `auth`, `adapter`, `bridge`, `runtime`, `storage`.

- A commit must represent **one logical change** only.
- If a change mixes multiple concerns, split it into separate commits. Common split boundaries include:
  - contracts / protocol freezing
  - server implementation
  - storage / migration
  - docs or execution-plan sync
  - CI/workflow changes
  - test-only changes
  - fixture/example-only changes

- A commit may include `contracts` + `fixtures` + minimal `ci` updates together **only when they freeze or validate the same single surface**.
- If a commit includes both behavior changes and storage/migration changes, split it unless they are inseparable for correctness.
- If a commit adds multiple protocol/message/action kinds, split it unless they are part of the same narrow capability and share the same validation path.

- The subject must describe the **concrete change**, not project progress or a milestone.
- Prefer a short **subject line**; target **<= 72 characters for the subject line when practical**, but do **not** hard-wrap or forcibly reflow text to satisfy this.
- Do not use progress-report subjects such as:
  - `近期主线推进`
  - `本轮开发进展`
  - `阶段性整理`
  - `several updates`
  - `misc changes`

- Use `feat` only when the commit primarily adds a real capability.
- If a commit would reasonably need different commit types, it must be split.

- Add a body when the reason, risk, migration impact, or validation is not obvious.
- The body should explain, in this order when practical:
  - why the change exists
  - what changed at a capability level
  - how it was validated
- Do not use the body as a raw task checklist, changelog dump, or file-by-file inventory.

- Format commit bodies for scan readability:
  - use short paragraphs or bullet lists
  - keep one bullet per concern
  - do **not** hard-wrap lines mechanically at 72 characters; wrap only when it improves readability
  - avoid awkward line breaks inside a phrase or sentence
  - put validation in a separate final paragraph or `Validation:` block

- Mark breaking changes with either:
  - `!` after the type/scope, or
  - a `BREAKING CHANGE:` footer

Examples:
- `feat(server): add bootstrap admin setup route`
- `fix(adapter): handle OneBot api_response echo matching`
- `docs(docs): sync execution plan with current progress`
- `ci(contracts): validate plugin action fixtures`

## Skill discovery and reuse

- Before planning or coding, always check whether there are existing skills relevant to the task.
- Skill discovery must cover all skill locations visible in the current environment, including:
  - repo-local skills
  - user/global skills
  - admin/system skills
  - any other supported skill locations exposed by the current Codex runtime
- Reuse an existing skill when it already matches the task or a substantial part of the task.
- Do not recreate a workflow in the prompt if an existing skill already covers it.

- At the start of a substantial task, briefly state:
  - which skills were selected, if any
  - why they were selected
  - or that no suitable skill was found

- If multiple skills are relevant, prefer the narrowest combination that fits the task.
- If a task spans planning, contract review, implementation, and validation, check for skills for each stage instead of assuming one skill should cover the whole workflow.

- Skills are workflow helpers, not sources of truth.
- `contracts/`, planning docs, and `AGENTS.md` remain authoritative over any skill instructions.
- If a skill conflicts with repository rules or formal contracts, follow the repository rules and contracts.

- If no suitable skill exists for a recurring workflow, note that gap and consider adding a new repo-local skill after the task is completed and validated.