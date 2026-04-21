---
name: contract-audit
description: Use when a task or diff touches contracts, APIs, schemas, states, errors, events, plugin manifest/protocol, or release metadata in this repo, to audit contract drift and required companion updates.
---

# Contract Audit

This skill is a reusable workflow. It does not define project truth. Formal truth still lives in `contracts/`, root/local `AGENTS.md`, and the engineering docs they reference.

## Use This Skill When

- 用户要修改 API、schema、状态、错误码、事件名
- 代码改动疑似影响 contract，但用户没有显式提到 contract
- 你需要判断五件套是否满足
- 你需要做 contract-focused review，而不是普通代码 review

## Inputs

- 任务描述或 diff 范围
- 受影响的 boundary：
  - config
  - web-api
  - websocket
  - error-codes
  - plugin-info
  - plugin-protocol
  - release-manifest

## Workflow

1. 先读根 `AGENTS.md` 和 `contracts/AGENTS.md`。
2. 打开相关 contract 文件与 `contracts/README.md`。
3. 对照对应的：
   - `fixtures/`
   - `examples/`
   - tests
   - docs
4. 逐项检查：
   - 是否新增了 contract 之外的字段 / 状态 / 错误码 / 事件名
   - 是否存在旧名新名并存
   - 是否把尚未 fixture-ready 的内容提前写进正式 contract
   - 是否缺少 fixture / example / test / doc 同步
5. 输出审计结论和缺口清单。

## Outputs

- 受影响 contract 清单
- 命名漂移或 shape 漂移清单
- 缺失的 fixture / example / test / doc 更新
- 是否满足五件套门禁
- 如有需要，给出建议的最小 contract-first 修正顺序

## Do Not

- 把实现行为反向当成 contract 真相
- 自动接受 speculative 字段或未来能力占位
- 在没有 fixtures / examples / tests 支撑时宣布 contract 已就绪
- 跳过错误码、状态名、事件名的一致性检查
