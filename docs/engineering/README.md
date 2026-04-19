# Engineering Docs

本目录承载 RayleaBot 的工程治理内容，固定版本线、目录职责、实施顺序和质量门禁。

## 工程目录模型

| 路径 | 作用 |
| --- | --- |
| `server/` | Go 服务端主链路 |
| `web/` | Web 管理面 |
| `launcher/` | Electron 桌面启动器 |
| `contracts/` | 正式接口、schema、错误码与 release metadata |
| `fixtures/` | 契约样例与回归基线 |
| `examples/` | 示例插件、示例配置和示例请求 |
| `plugins/` | 插件根目录，默认发现 `builtin/` 与 `installed/` |
| `config/` | 默认配置与用户配置 |
| `data/` | SQLite 状态库与插件业务数据 |
| `cache/` | 渲染缓存、下载缓存与临时缓存 |
| `logs/` | 结构化日志与诊断输出 |
| `.deps/` | Chromium、Python、Node.js 与相关资源清单 |
| `.github/workflows/` | CI、打包与发布门禁 |
| `docs/` | 文档总纲与专题说明 |

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [baseline.md](./baseline.md) | 固定版本线、默认命令、目录职责与冻结选型 |
| [implementation-order.md](./implementation-order.md) | 长期阶段边界与进入条件 |
| [quality-gates.md](./quality-gates.md) | 默认验证命令、CI 门禁与发布回归 |
| [tech-stack-evaluation.md](./tech-stack-evaluation.md) | 技术栈评估与引入计划 |
| [web-antdv-vben-migration-plan.md](./web-antdv-vben-migration-plan.md) | Web 管理面迁移到 Ant Design Vue + Vue Vben Admin 的完整执行方案 |
| [`../execution-plan-v0.3.md`](../execution-plan-v0.3.md) | 当前执行计划 |
| [`../execution-plan.md`](../execution-plan.md) | v0.1 基线与历史对照 |

## 维护规则

- 对外接口裁决不在本目录，而在 `contracts/`。
- 工程基线变化必须同步对应工程文件和 CI。
- 本目录负责约束实现边界和协作规则，不替代正式契约。
