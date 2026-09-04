# Examples Agent Guide

先遵守根 `AGENTS.md`；分类见 `examples/README.md`，插件声明见 `docs/plugin/permissions-and-manifest.md`。

## Examples

- 示例演示契约已确定的结构，不能先于契约定义新字段或能力。
- examples 说明用法，稳定回归语义放 `fixtures/`；演示中的断言与 mock 不替代正式测试。
- 插件 manifest 的事件、权限和入口声明与实际调用一致；调用或声明变化时检查对应 README、测试和 fixtures。
- HTTP 示例的字段形状与当前 OpenAPI 一致；契约变化影响示例时同轮更新。
- 凭据使用明确假值并标注替换用途，不使用真实凭据或仿真凭据。
- 示例插件不是生产模板、市场分发包或官方最佳实践承诺。
