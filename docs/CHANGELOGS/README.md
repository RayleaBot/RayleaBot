# Changelogs

本目录归档 RayleaBot 已交付版本的能力清单与边界。

## 当前归档

| 版本 | 文件 | 主线焦点 |
| --- | --- | --- |
| v0.1 | [v0.1.md](./v0.1.md) | 单实例基线、OneBot11 reverse WebSocket、插件运行时、管理面、渲染服务、恢复与发布基线 |
| v0.2 | [v0.2.md](./v0.2.md) | OneBot11 完整传输面与兼容矩阵、在线模板编辑器、Web 管理面 Vben 对齐、Launcher 收口 |

## 范围

- 当前正式执行计划见 [`../execution-plan-v0.3.md`](../execution-plan-v0.3.md)。
- 工程基线、目录职责、固定版本线见 [`../engineering/baseline.md`](../engineering/baseline.md)。
- 长期实施顺序与阶段边界见 [`../engineering/implementation-order.md`](../engineering/implementation-order.md)。
- 对外接口、错误码、release metadata 以 `contracts/` 为准。

## 维护原则

- 历史版本归档不再回写，已交付能力的最终行为以 `contracts/` 与现行文档为准。
- 新版本完成后，把已归档的执行计划压缩为本目录新增条目。
