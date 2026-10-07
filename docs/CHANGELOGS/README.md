# Changelogs

本目录按版本归档 RayleaBot 的能力清单；候选版本的发布正文见 [`docs/release/notes/`](../release/notes/README.md)。

## 当前归档

| 版本 | 文件 | 主线焦点 |
| --- | --- | --- |
| v0.1 | [v0.1.md](./v0.1.md) | 单实例基线、OneBot11 reverse WebSocket、插件运行时、管理面、渲染服务、恢复与发布基线 |
| v0.2 | [v0.2.md](./v0.2.md) | OneBot11 完整传输能力与兼容矩阵、在线模板编辑器、Web 管理面 Vben 对齐、Launcher 收口 |
| v0.3 | [v0.3.md](./v0.3.md) | 管理治理、插件信任与分发、Go artifact 运行时、自定义管理页、更新信任与事务安装 |
| v0.4 | [v0.4.md](./v0.4.md) | 多适配器实例与 QQ 官方机器人、插件合同 v4（manifest v4 / protocol v4 / artifact v2）与插件商店、多轮对话与后台事件等插件能力、全新分发（config v4 / SQLite 000006 / backup v3）、一键更新、管理面与 Launcher 重做 |

## 范围

- 已交付版本的正式范围以本目录归档为准。v0.4 对应尚未发布的 v0.4.0，发布前随候选修订；0.3.1 之后以 0.5.0、0.7.0 编号的未公开候选已并入 v0.4。
- 工程基线、目录职责、固定版本线见 [`../engineering/baseline.md`](../engineering/baseline.md)。
- 组件职责与演进边界见 [`../architecture/README.md`](../architecture/README.md)。
- 对外接口、错误码、release metadata 以 `contracts/` 为准。

## 维护原则

- 历史版本归档不再回写，已交付能力的最终行为以 `contracts/` 与现行文档为准。
