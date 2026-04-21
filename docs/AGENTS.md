# Docs Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `docs/` 目录特有、长期有效的规则。

## Documentation Rules

- `docs/` 负责解释正式范围、工程边界、架构、用户操作、插件能力和发布说明，不裁决对外接口。
- 对外接口、schema、错误码、事件名与 release metadata 最终以 `contracts/` 为准。
- 文档只写最终态，直接表达当前能力、限制、入口和结果。
- 非变更记录、迁移说明、发布说明，不保留过程叙事、阶段口吻或实现顺序解释。

## Directory Responsibilities

- `docs/engineering/`：工程基线、实施顺序、质量门禁和治理规则。
- `docs/architecture/`：架构、状态模型、统一事件模型、跨层边界。
- `docs/design/`：视觉、交互和桌面端设计说明。
- `docs/dev/`：开发、调试、诊断、贡献流程。
- `docs/plugin/`：插件 manifest、capabilities、协议、SDK、插件内置管理页。
- `docs/user/`：安装、初始化、配置、管理面、恢复与排障。
- `docs/release/`：版本说明、交付、升级、风险与已知问题。

## Sync Rules

- 执行计划、README、用户文档和工程文档都要与当前正式范围一致。
- 当前执行计划入口固定为 `docs/execution-plan-v0.3.md`。
- 当 contract 或正式行为变化影响文档时，同轮修正对应文档，不把文档拖到后续批次。
- 若文档与 `contracts/` 冲突，以 `contracts/` 为准并同步修正文案。

## Consult Before Editing

- 工程基线与目录职责：`docs/engineering/baseline.md`
- 当前执行计划：`docs/execution-plan-v0.3.md`
- 正式契约范围：`contracts/README.md`
- 文档成稿规则：`.agents/skills/editing-final-state-content/SKILL.md`
