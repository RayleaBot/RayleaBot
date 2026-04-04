# Fixtures

本目录承载由 `contracts/` 派生的 golden cases，面向后续自动校验、契约回归和实现对照，不是演示文档目录。

## 当前分类

- `config/`
  - 对应 `contracts/config.user.schema.json`
  - 统一采用 `input + expect` 结构
- `web-api/`
  - 对应 `contracts/web-api.openapi.yaml`
  - 统一采用 `request + response + expect` 结构
- `websocket/`
  - 对应 `contracts/websocket-events.yaml`
  - 统一采用 `frame + expect` 结构
- `errors/`
  - 对应 `contracts/error-codes.yaml`
  - 统一采用 `input + expect` 结构
- `plugin-info/`
  - 对应 `contracts/plugin-info.schema.json`
  - 统一采用 `input + expect` 结构
  - 主要服务 schema validation、安装前静态检查、权限重确认判断和迁移边界判断
- `plugin-protocol/`
  - 对应 `contracts/plugin-protocol.schema.json`
  - 统一采用 `frames + expect` 结构
  - 主要服务消息 schema validation、消息顺序 golden cases 和 request-response 关联校验
- `release-manifest/`
  - 对应 `contracts/release-manifest.schema.json`
  - 统一采用 `input + expect` 结构
  - 主要服务产物元数据校验，以及后续 doctor、launcher、release 共享结构验证
- `deps-manifest/`
  - 对应 `contracts/deps-manifest.schema.json`
  - 统一采用 `input + expect` 结构
  - 主要服务 Python / Node.js 运行环境来源、校验值、归档格式与相对入口的契约回归
- `cli/`
  - 对应 `contracts/cli-commands.yaml`
  - 统一采用 `input + expect` 结构
  - 主要服务 CLI 命令可用模式、退出码、task model 与 fixture-ready 覆盖校验

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
- `edge` case 表达“仍合法，但处于关键边界、顺序窗口或退化语义”。
- 如 contract 尚未冻结某个字段、消息类型或元数据，不允许先在 fixture 里占位。

## Schema Validation 与语义 Golden 的边界

- 只由 schema 就能判断的约束，应优先通过 `expect.valid` 表达。
- 对“schema 暂时无法单独表达、但规划已明确”的语义，可在 `expect.notes` 中明确未来语义校验点。
- `expect.notes` 只能解释正式 contract 已有字段的语义，不得借机引入新字段或第二套状态模型。

## 后续扩展规则

- 若新增 contract 文件进入 fixture-ready，先在本目录增加对应子目录，再让 contract 通过 `x-fixtures` 或等价字段引用样例。
- 若 contract 改名、改状态、改错误码、改协议消息类型，必须同步更新对应 fixture。
- 任何会影响行为判断的变更，都应至少补一条 `ok`、一条 `invalid` 或一条 `edge` case，不能只改契约正文。
