# Examples Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `examples/` 目录特有、长期有效的规则。

## Example Rules

- `examples/` 只演示已经冻结的结构，不是正式来源。
- 示例不能先于 `contracts/` 发明新字段、消息类型、错误码、状态名或配置键。
- 当前示例范围包含示例插件、示例 HTTP 请求响应、`deps-manifest` 样例和 `backup-manifest` 样例。
- 示例中不放真实 secrets、token、凭据或其他敏感信息。

## Plugin Example Rules

- `examples/plugins/` 主要服务于 manifest、plugin protocol、local actions 和管理面能力理解。
- 示例插件的 `capabilities`、`permissions.required` 和 `permissions.optional` 要与代码主路径一致。
- 已冻结的插件内置管理页、治理 local actions、render preview、webhook、scheduler 等能力，可以在示例中演示；未来能力不写进示例。
- 示例插件不是生产模板、市场分发包或官方最佳实践承诺。

## HTTP Example Rules

- `examples/http/` 只承载已冻结 HTTP surface 的稳定请求和响应示例。
- 示例文件命名、字段形状和当前 OpenAPI 保持一致。
- HTTP 示例与 fixture 各自承担不同职责：examples 负责说明用法，fixtures 负责回归校验。

## Companion Updates

- contract 变化影响示例结构时，同轮更新对应 example。
- 示例插件 manifest 或调用链变化时，同时检查 README、测试和相关 fixtures 是否需要同步。

## Consult Before Editing

- 示例说明：`examples/README.md`
- 正式契约范围：`contracts/README.md`
- 插件能力与协议：`docs/plugin/capabilities-and-manifest.md`、`docs/plugin/protocol.md`
