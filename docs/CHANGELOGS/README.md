# Changelogs

本目录归档 RayleaBot 各已交付版本的能力清单。

## 候选记录

[0.4.0 全新分发候选说明](../release/notes/v0.4.0.md) 记录本轮变化、首次安装、插件接入、本版备份恢复和验证限制；验证证据见[候选验收记录](../release/validation-v0.4.0.md)。该候选尚未公开发布，其代码范围已按 v0.4 归档之后的全部提交归纳为下表的 v0.5，不改写既有历史记录。

## 当前归档

| 版本 | 文件 | 主线焦点 |
| --- | --- | --- |
| v0.1 | [v0.1.md](./v0.1.md) | 单实例基线、OneBot11 reverse WebSocket、插件运行时、管理面、渲染服务、恢复与发布基线 |
| v0.2 | [v0.2.md](./v0.2.md) | OneBot11 完整传输能力与兼容矩阵、在线模板编辑器、Web 管理面 Vben 对齐、Launcher 收口 |
| v0.3 | [v0.3.md](./v0.3.md) | 管理治理、插件信任与分发、Go artifact 运行时、自定义管理页、更新信任与事务安装 |
| v0.4 | [v0.4.md](./v0.4.md) | 插件合同纪元升级（manifest v3 / protocol v2 / artifact v2 / bridge v3）、统一开发工具、插件商店收敛与来源管理 |
| v0.5 | [v0.5.md](./v0.5.md) | 多适配器实例与 QQ 官方机器人、插件协议 v3、全新分发（config v4 / SQLite 000001 / backup v3）、Reka UI 管理面、契约生成物与服务端重组清理 |

## 范围

- 已交付版本的正式范围以本目录归档为准；执行期计划按需在 `docs/` 建立。
- 工程基线、目录职责、固定版本线见 [`../engineering/baseline.md`](../engineering/baseline.md)。
- 长期依赖顺序与实现边界见 [`../engineering/implementation-order.md`](../engineering/implementation-order.md)。
- 对外接口、错误码、release metadata 以 `contracts/` 为准。

## 维护原则

- 历史版本归档不再回写，已交付能力的最终行为以 `contracts/` 与现行文档为准。
- 新版本发布后，把对应执行计划整理为本目录的新增条目。
