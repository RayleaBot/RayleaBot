# Engineering Docs

本目录说明 RayleaBot 的工程治理：固定版本线、目录职责、实施顺序和质量门禁。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [baseline.md](./baseline.md) | 固定版本线、默认命令、目录职责与固定的技术选型 |
| [implementation-order.md](./implementation-order.md) | 长期依赖顺序、状态归属与跨层边界 |
| [quality-gates.md](./quality-gates.md) | 默认验证命令、CI 门禁与发布回归 |
| [collection-pagination.md](./collection-pagination.md) | 管理集合分页、数据库读取与事件刷新边界 |
| [cleanup-decisions.md](./cleanup-decisions.md) | 清理时核实过但保留或不实施的替代方案 |
| [web-admin-baseline.md](./web-admin-baseline.md) | Web 管理面 Reka UI、自有组件与 Motion for Vue 工程基线 |
| [`../CHANGELOGS/`](../CHANGELOGS/README.md) | 历史版本能力归档 |

## 维护规则

- 工程基线变化必须同步对应工程文件和 CI。
- 本目录负责约束实现边界和协作规则，不替代正式契约。
