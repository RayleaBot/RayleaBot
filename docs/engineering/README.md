# Engineering Docs

本目录说明 RayleaBot 的工程治理：固定版本线、目录职责、实施顺序和质量门禁。

## 工程目录模型

| 路径 | 作用 |
| --- | --- |
| `server/` | Go 服务端主链路 |
| `web/` | Web 管理面 |
| `launcher/` | Wails 桌面启动器 |
| `contracts/` | 正式接口、schema、错误码与 release metadata |
| `fixtures/` | 契约样例与回归基线 |
| `examples/` | 示例插件、manifest 和示例请求/响应 |
| `plugins/installed/` | 统一插件安装与发现目录；商店、本地 artifact 和开发同步共用 |
| `config/` | 默认配置与用户配置 |
| `data/` | SQLite 状态库与插件业务数据 |
| `cache/` | 渲染缓存、下载缓存与临时缓存 |
| `logs/` | 结构化日志与诊断输出 |
| `.deps/` | Chromium 与 FFmpeg / FFprobe 运行环境资源清单 |
| `.github/workflows/` | CI、打包与发布门禁 |
| `docs/` | 文档总纲与专题说明 |

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [baseline.md](./baseline.md) | 固定版本线、默认命令、目录职责与固定的技术选型 |
| [implementation-order.md](./implementation-order.md) | 长期依赖顺序、状态归属与跨层边界 |
| [quality-gates.md](./quality-gates.md) | 默认验证命令、CI 门禁与发布回归 |
| [web-admin-baseline.md](./web-admin-baseline.md) | Web 管理面 Ant Design Vue + Vue Vben Admin 工程基线 |
| [Web 前端迁移计划 v1](../execution-plan-v1.md) | 已选定的新 UI 技术路线、迁移阶段和验收条件 |
| [`../CHANGELOGS/`](../CHANGELOGS/README.md) | 历史版本能力归档 |

## 维护规则

- 对外接口不由本目录决定，以 `contracts/` 为准。
- 工程基线变化必须同步对应工程文件和 CI。
- 本目录负责约束实现边界和协作规则，不替代正式契约。
