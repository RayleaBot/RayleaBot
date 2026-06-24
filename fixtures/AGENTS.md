# Fixtures Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `fixtures/` 目录特有、长期有效的规则。

## Fixture Rules

- `fixtures/` 只承载由 `contracts/` 派生的 golden cases，不是正式来源，也不是示例目录。
- fixture 不能先于 contract 发明字段、状态、错误码、事件名、消息类型或接口。
- 同一分类继续沿用稳定前缀：`ok`、`invalid`、`edge`。
- 文件命名必须稳定、可扩展、可直接被 CI 枚举。

## Structure Rules

- fixture 结构必须与对应 contract 类型一致，例如：
  - `input + expect`
  - `request + response + expect`
  - `frames + expect`
- `expect.notes` 只能解释正式 contract 已有字段的语义，不引入第二套状态模型。

## Expect Semantics

- `ok`：应被接受或应被视为合法。
- `invalid`：应被拒绝或应被视为不合法。
- `edge`：仍合法，但处于关键边界、顺序窗口或退化语义。

## Secret Rules

- fixture 中只允许显式假值，例如 `fixture-only-secret`、`example-token`、`test-ck`。
- 不允许出现真实平台 cookie、CK、access token、refresh token 或任何可误认为是真实凭据的长字符串。
- 配置读取类 fixture 必须包含对 secret 字段脱敏的断言，防止测试把明文 secret 返回固化成正确行为。
- 若 fixture 需要模拟鉴权失败，使用明显假值并配合错误码 fixture，不构造接近真实格式的 token。

## Companion Updates

- contract 改名、改状态、改错误码、改协议消息类型时，必须同步更新对应 fixture。
- 新增进入 fixture-ready 的 contract 时，同轮补齐最小 `ok`、`invalid` 或 `edge` case。
- fixture 引用、README 和 CI 校验要与目录内文件保持一致。

## Consult Before Editing

- fixture 分类与命名：`fixtures/README.md`
- 正式契约清单与范围：`contracts/README.md`
- 仓库级规则与门禁：根 `AGENTS.md`
