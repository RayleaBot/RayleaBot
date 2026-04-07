# Engineering Docs

本目录用于承载 RayleaBot 的工程治理内容，固定版本线、默认命令、仓库边界与实施顺序。

## 当前工程状态

- `server/` 主链路完整，覆盖配置（热重载快照）、SQLite 存储（12 张表、14 个迁移）、鉴权（HMAC-SHA256 session）、任务（11 种类型、顺序执行器）、插件 runtime（7 种状态、local actions）、dispatcher/scheduler、render service（Chromium 渲染队列）、聊天权限（blacklist/cooldown/cooldown reply）、recovery/backup、diagnostics 与管理面全路由；约 30 个内部包，约 127 个 Go 源文件
- `web/` 已形成真实管理面主流程，`launcher/` 已形成最小桌面闭环；PR 默认门禁使用 Linux 核心链路，跨平台回归由 `release.yml` 与 `self-host-smoke.yml` 承担
- `contracts/` 已具备 10 份 fixture-ready formal contracts，覆盖配置、错误码、管理 HTTP / WebSocket、插件 manifest、插件协议、release metadata、CLI、backup manifest 与 deps manifest
- `.deps/manifest.json` 已固定 Chromium、Python 与 Node.js 资源的版本、来源、SHA256 与平台矩阵

## 文档分工

- `baseline.md`：版本线、默认命令、目录职责、冻结选型
- `implementation-order.md`：长期有效的阶段边界与进入条件
- `../execution-plan.md`：当前进度与下一步行动记录
- `../../contracts/README.md`：formal contracts 与 contract 级 TODO 概览

## 维护规则

- 对外接口裁决不在本目录，而在 `contracts/`。
- 本目录用于固定工程实现边界、命令入口和协作规则，不替代执行计划。
- 任何基线变更都必须同步更新对应工程文件与 CI，而不是只改文档。
