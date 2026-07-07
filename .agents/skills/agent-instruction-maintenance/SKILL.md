---
name: agent-instruction-maintenance
description: 修改本仓库的 AGENTS.md、CLAUDE.md 或任何 .agents/skills/SKILL.md 时使用。在定稿前检查文件行数、路径存在性、bridge 完整性、重复、冲突与 secret 泄露。
---

# Agent Instruction Maintenance

本 skill 是可复用工作流，不定义项目真相。仓库真相仍在 `contracts/`、根/局部 `AGENTS.md` 及其引用的工程文档中。

## 适用场景

- 新增、修改或删除 `AGENTS.md`、`CLAUDE.md`、`.agents/skills/**/SKILL.md`
- 用户提到 agent instruction、rules、memory、Claude Code 配置或 agent 工作流调整
- 需要确认 bridge 是否完整、路径是否过期、规则是否重复或冲突
- 需要检查 agent 指令文件是否包含真实 secret 或凭据形态

## 工作流

1. 读取根 `AGENTS.md` 和当前目录的局部 `AGENTS.md`（如果存在）。
2. 读取目标 `CLAUDE.md` 或 `SKILL.md` 的当前内容。
3. 检查行数预算：
   - 根 `AGENTS.md` 建议不超过 150 行。
   - 根 `CLAUDE.md` 建议不超过 40 行。
   - 局部 `AGENTS.md` 建议不超过 120 行。
   - `SKILL.md` 建议不超过 100 行。
4. 检查路径存在性：扫描反引号路径，确认文件或目录真实存在；排除命令、URL、glob、配置 key。
5. 检查 bridge：确认有 `AGENTS.md` 的一级目录旁存在同级 `CLAUDE.md`。
6. 检查重复与冲突：
   - 根规则是否在局部被弱化或放宽。
   - 局部规则是否与根规则矛盾。
   - 同一规则是否在不同文件中重复展开。
7. 检查 secret：确认文件中无真实 token、cookie、access key、CK、密码等凭据形态。
8. 运行 `scripts/check-agent-docs.mjs`（如果存在），并处理其输出。
9. 输出修正建议或确认当前修改可提交。

## 输出

- 文件长度是否超标
- 过期或不存在路径清单
- 缺失的 `CLAUDE.md` bridge 清单
- 重复或冲突规则清单
- 疑似 secret 的位置
- `scripts/check-agent-docs.mjs` 运行结果摘要

## 禁止

- 把实现细节、当前功能清单或易过期的路径写进 `AGENTS.md` 或 `CLAUDE.md`。
- 在 agent 指令文件中暴露真实凭据或凭据格式。
- 让局部规则放宽或覆盖根规则中的硬约束（如 contract-first、secrets、frozen stack）。
- 把多步骤流程直接塞进 `AGENTS.md`；流程应沉淀到 skill。
