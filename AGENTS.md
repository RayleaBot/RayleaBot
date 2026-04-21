# RayleaBot Repository Guide

## Purpose

RayleaBot 是一个面向个人开发者和 GitHub 开源协作者的自托管聊天机器人框架。

本文件只保留仓库级、长期有效的硬规则。进入具体目录后，需要继续遵守该目录就近的 `AGENTS.md`。

## Instruction Layers

仓库内的规则层级固定为：

`docs/RayleaBot机器人项目规划.md > contracts/ > fixtures/examples > code`

补充规则：

- 根 `AGENTS.md` 与就近目录 `AGENTS.md` 同时生效。
- 局部 `AGENTS.md` 只补充所在目录特有规则，不能削弱根规则。
- repo-local skills 只承载工作流，不裁决仓库真相。
- 当前执行计划入口是 `docs/execution-plan-v0.3.md`。
- `docs/execution-plan.md` 只承担 v0.1 基线与历史对照。

## Repository Truth

- `contracts/` 是对外 HTTP API、WebSocket、plugin manifest、plugin protocol、config schema、error codes、release metadata 的唯一正式来源。
- `docs/engineering/baseline.md` 固定工程版本线、默认命令、目录职责和长期有效选型。
- `docs/engineering/implementation-order.md` 固定长期实施顺序与阶段边界。
- `docs/architecture/` 负责架构、状态模型、事件模型与跨层边界说明。
- `fixtures/` 与 `examples/` 只能从 `contracts/` 派生，不能反向裁决 `contracts/`。

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

- 安装：`pnpm install --frozen-lockfile`
- 测试：`pnpm test`
- 构建：`pnpm build`

规则：

- Server、Web、Launcher 各自只保留一套默认入口。
- CI、发布和本地验证直接复用这些入口，不维护平行命令。

## Repository Rules

- 对外边界优先走 contract-first：先定契约，再写实现。
- 不得在 `contracts/` 之外发明新的对外字段、状态名、错误码、事件名或配置键。
- 任一涉及协议、schema、状态机、配置、数据库结构、插件安装流程、渲染输入输出、Web API、WebSocket、错误码或迁移的改动，合并前必须满足四件套：
  - 实现代码更新
  - 契约更新
  - 测试更新
  - 示例更新
- 不允许“代码先改，契约、测试、示例后补”。
- 若用户要求与 `docs/RayleaBot机器人项目规划.md` 冲突，必须同步修正文档。
- 若任务按执行计划文档实施，完成后必须同步回写对应执行计划。

禁止以下跨层捷径：

- Adapter 直接写状态库。
- Launcher 复制 Web 业务逻辑。
- Web UI 通过解析日志推断真实状态。
- 插件运行时直接读写 `config/user.yaml`。
- 插件绕过 Capability 校验直连平台内部模块。
- CLI、Launcher、Web UI 各自发明不同状态名、错误码或任务状态。

## Reuse-First Implementation

- 默认实现阶梯固定为：`仓库现有代码与已冻结选型 > 标准库或平台内建能力 > 官方 SDK 或已冻结上游依赖 > 成熟且依赖面最小的开源项目 > 薄胶水层自定义代码`。
- 只在现有实现与固定选型不足时引入新依赖，并说明原因。
- 新依赖必须许可清晰、维护稳定、生产验证充分、依赖面最小，并通过现有 lockfile 或工程文件锁版本。

## Security and Secrets

- 不要硬编码、提交或在示例中放入真实 secrets、token、凭据。
- 不要在日志、fixtures、examples、docs 中输出敏感字段原值。
- 涉及鉴权、配置或外部连接时，优先遵守当前 contract 与 baseline 中已经冻结的最小权限和本地优先安全约束。

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

## Git Commit Rules

所有提交都必须遵守 Conventional Commits：

`<type>[optional scope][!]: <description>`

允许的 `type`：

- `feat`
- `fix`
- `refactor`
- `perf`
- `docs`
- `test`
- `build`
- `ci`
- `chore`

补充规则：

- 一个提交只表达一个逻辑变化。
- 若变更同时跨越 contracts、实现、存储、CI、docs、tests、fixtures/examples，按逻辑边界拆分提交。
- 标题描述具体变化，不写阶段汇报或笼统总结。
- 需要时补提交正文，按“原因、能力变化、验证方式”的顺序表达。

## Consult Before Major Changes

- 工程基线、目录职责、固定命令：`docs/engineering/baseline.md`
- 实施顺序与阶段边界：`docs/engineering/implementation-order.md`
- 工程治理与门禁：`docs/engineering/README.md`
- 正式契约清单与范围：`contracts/README.md`
- 架构与跨层边界：`docs/architecture/README.md`
- repo-local workflows：`.agents/skills/`

## Delete Rules

- 删除任何文件或目录时，禁止使用 PowerShell。
