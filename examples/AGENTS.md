# Examples Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `examples/` 目录特有、长期有效的规则。

## Example Rules

- `examples/` 只演示已经冻结的结构，不是正式来源。
- 示例不能先于 `contracts/` 发明新字段、消息类型、错误码、状态名或配置键。
- 当前示例范围包含示例插件、示例 HTTP 请求响应、`deps-manifest` 样例和 `backup-manifest` 样例。
- 示例中不放真实 secrets、token、凭据或其他敏感信息。

## Examples Are Not Oracles

- examples 用于说明用法和结构，不承担稳定回归语义。
- 需要稳定回归校验的语义写入 `fixtures/`，不在 examples 中重复。
- examples 与 fixtures 职责不混淆：examples 负责可读性，fixtures 负责可校验性。
- 示例代码中的断言、mock 返回值和边界值只服务于演示，不替代正式测试。

## Plugin Example Rules

- `examples/plugins/` 主要服务于 manifest、plugin protocol、local actions 和管理面能力理解。
- 示例插件的 `capabilities` 和 `capability_parameters` 要与代码主路径一致。
- 已冻结的插件内置管理页、治理 local actions、render templates、webhook、scheduler 等能力，可以在示例中演示；未来能力不写进示例。
- 示例插件不是生产模板、市场分发包或官方最佳实践承诺。

## HTTP Example Rules

- `examples/http/` 只承载已冻结 HTTP surface 的稳定请求和响应示例。
- 示例文件命名、字段形状和当前 OpenAPI 保持一致。
- HTTP 示例与 fixture 各自承担不同职责：examples 负责说明用法，fixtures 负责回归校验。

## Secret Rules

- 示例中只能使用明显假值，例如 `example-token`、`fixture-only-secret`、`test-user`。
- 不放真实凭据格式，不构造接近真实平台 cookie、CK、access token 形态的字符串。
- 示例配置中的敏感字段必须标注为假值，并附带说明该字段在真实环境中需要替换。

## Companion Updates

- contract 变化影响示例结构时，同轮更新对应 example。
- 示例插件 manifest 或调用链变化时，同时检查 README、测试和相关 fixtures 是否需要同步。

## Consult Before Editing

- 示例说明：`examples/README.md`
- 正式契约范围：`contracts/README.md`
- 插件能力与协议：`docs/plugin/capabilities-and-manifest.md`、`docs/plugin/protocol.md`
