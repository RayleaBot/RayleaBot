# Fixtures

本目录承载由 `contracts/` 派生的 golden cases，面向后续自动校验、契约回归和实现对照，不是演示文档目录。

## 当前分类

- `config/`
  - `contracts/config.user.schema.json` 的输入样例。
  - 统一采用 `input + expect` 结构。
- `web-api/`
  - `contracts/web-api.openapi.yaml` 的请求 / 响应样例。
  - 统一采用 `request + response` 结构。
- `websocket/`
  - `contracts/websocket-events.yaml` 的帧样例。
  - 统一采用 `frame + expect` 结构。
- `errors/`
  - `contracts/error-codes.yaml` 的目录校验和适用范围样例。
  - 统一采用 `input + expect` 结构。

## 命名规则

- 正常路径：`ok.<scenario>.json` 或 `ok.<scenario>.yaml`
- 失败路径：`invalid.<scenario>.json` 或 `invalid.<scenario>.yaml`
- 边界路径：`edge.<scenario>.json` 或 `edge.<scenario>.yaml`

规则：

- 文件名必须稳定、可扩展、可直接被 CI 枚举。
- 同一分类中后续新增 case 继续沿用 `ok|invalid|edge` 前缀，不改历史命名风格。

## 编写规则

- fixture 不能先于 contract 发明字段、状态名、错误码、事件名或接口。
- fixture 必须直接引用正式 contract 路径或契约标识，不能引用实现文件。
- `ok` case 表达“应被接受 / 应被视为合法”。
- `invalid` case 表达“应被拒绝 / 应被视为不合法”。
- `edge` case 表达“仍合法，但处于关键边界或退化语义”。
- 如 contract 尚未冻结某个字段或事件名，不允许先在 fixture 里占位。

## 后续扩展规则

- 若新增 contract 文件进入 fixture-ready，先在本目录增加对应子目录，再让 contract 通过 `x-fixtures` 或等价字段引用样例。
- 若 contract 改名、改状态、改错误码，必须同步更新对应 fixture。
- 任何会影响行为判断的变更，都应至少补一条 `ok`、一条 `invalid` 或一条 `edge` case，不能只改契约正文。
