# Fixtures Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `fixtures/` 目录特有、长期有效的规则。

## Fixture Rules

- `fixtures/` 只承载由 `contracts/` 派生的 golden cases，不是正式来源，也不是演示目录。
- fixture 不能先于 contract 发明字段、状态、错误码、事件名、消息类型或接口。
- 同一分类继续沿用稳定前缀：`ok`、`invalid`、`edge`。
- fixture 结构必须与对应 contract 类型保持一致，例如：
  - `input + expect`
  - `request + response + expect`
  - `frames + expect`
- 文件命名必须稳定、可扩展、可直接被 CI 枚举。

## Expect Rules

- `ok`：应被接受 / 应被视为合法。
- `invalid`：应被拒绝 / 应被视为不合法。
- `edge`：仍合法，但处于关键边界、顺序窗口或退化语义。
- `expect.notes` 只能解释正式 contract 已有字段的语义，不能借机引入新字段或第二套状态模型。

## Companion Updates

- contract 改名、改状态、改错误码、改协议消息类型时，必须同步更新对应 fixture。
- 新增进入 fixture-ready 的 contract 时，先建对应 fixture 分类，再建立引用和 CI 校验。
- 任何会影响行为判断的 contract 变更，至少补一条对应的 `ok`、`invalid` 或 `edge` case。

## Consult Before Editing

- fixture 分类与命名规则：`fixtures/README.md`
- 正式契约清单与 TODO：`contracts/README.md`
- 仓库级门禁与优先级：根 `AGENTS.md`
