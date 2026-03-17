---
name: phase-boundary-check
description: Use when planning or reviewing a change in this repo to verify whether it stays within the intended implementation boundary, touched directories, and contract-first rules defined by AGENTS.md and docs/engineering/implementation-order.md.
---

# Phase Boundary Check

This skill is a reusable workflow. It does not define project truth. Repository truth still lives in root/local `AGENTS.md`, `contracts/`, `docs/engineering/baseline.md`, and `docs/engineering/implementation-order.md`.

## Use This Skill When

- 用户提出一个实现任务，但边界是否越界还不清楚
- 任务跨越多个目录，可能触发 contract-first、四件套或跨层限制
- 你需要把“大任务”切成当前阶段可以落地的最小切片
- 你在 code review / planning 时需要判断某个改动是否抢跑了后续能力

## Inputs

- 任务描述
- 计划改动的目录或文件
- 用户明确给出的阶段、里程碑或边界条件（如果有）

## Workflow

1. 先读根 `AGENTS.md`。
2. 如果涉及具体目录，再读对应局部 `AGENTS.md`。
3. 读取：
   - `docs/engineering/implementation-order.md`
   - `docs/engineering/baseline.md`
   - 相关 `contracts/*` 与 `contracts/README.md`
4. 把请求拆成：
   - 当前边界内可做
   - 需要先补 contract / fixture / example 才能做
   - 明显越界、应延后的内容
5. 给出最小可执行切片，避免把后续 phase 的能力一并带入。

## Outputs

- 一份边界判断：
  - allowed
  - blocked
  - requires-contract-first
- 一份最小实施切片建议
- 一份 companion updates 清单：
  - contracts
  - fixtures
  - examples
  - tests
  - docs

## Do Not

- 发明新的 phase 或新的长期规则
- 把 README / examples / 实现细节当成正式来源
- 因为“顺手能做”就默许越界扩张
- 跳过局部 `AGENTS.md` 与 `contracts/` 直接给出范围判断
