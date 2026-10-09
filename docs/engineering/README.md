# Engineering Docs

本目录说明 RayleaBot 的工程治理：固定版本线、默认命令、质量门禁与人工 smoke。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [baseline.md](./baseline.md) | 固定版本线、默认命令、目录职责与固定的技术选型 |
| [quality-gates.md](./quality-gates.md) | 按改动面的验证、nightly 工作流与发布验收 |
| [manual-smoke.md](./manual-smoke.md) | 需要外部平台凭据或人工操作的 smoke 登记 |
| [security-alert-triage.md](./security-alert-triage.md) | CodeQL 告警核查依据与合入后关闭条件 |
| [`../CHANGELOGS/`](../CHANGELOGS/README.md) | 历史版本能力归档 |

## 维护规则

- 工程基线变化必须同步对应工程文件和 nightly 工作流。
- 本目录约束实现边界和协作规则，不复述代码或页面结构，也不替代正式契约。
