# Developer Docs

本目录整理 RayleaBot 的开发、调试、诊断与贡献说明。

## 当前开发入口

当前仓库的主要实现面集中在以下区域：

- `server/`：Go 服务端主链路、管理面、适配器、runtime、存储与任务系统
- `contracts/`：正式接口、schema、错误码与 release metadata
- `fixtures/` 与 `examples/`：契约样例、golden cases、示例插件与示例配置
- `.github/workflows/`：contracts、baseline、测试、打包与发布门禁

`web/` 提供管理控制台，覆盖 `setup/login/session`、系统状态页、`plugins/tasks/logs/config` 页面、plugin install / uninstall / grants / console、`system/shutdown`、响应式布局、无障碍细节、Pinia stores、统一 fetch / WebSocket client，以及 fixture-backed Vitest / Playwright 测试。

`launcher/` 提供 Electron 桌面启动器，覆盖 loopback bootstrap auth、环境检查、server 启停、健康轮询、打开 Web UI、stderr 诊断摘要、托盘关闭语义、桌面设置持久化、跨平台打包与 Vitest 回归。

## 调试与验证重点

- 默认命令与版本线以 `docs/engineering/baseline.md` 为准。
- 当前主验证入口包括：
  - `go test ./...`
  - `go build ./cmd/raylea-server`
  - `pnpm build`
  - `pnpm test`
  - `pnpm test:e2e`
- 涉及接口、schema、错误码、事件、插件协议或 release metadata 的变更，先同步 `contracts/`，再更新实现、fixtures、示例与文档。

## Web 开发入口

- 在 `web/` 下执行 `pnpm install --frozen-lockfile` 安装前端依赖。
- `pnpm dev` 启动 Vite 开发服务器。
- `pnpm test` 运行 Vitest 单测。
- `pnpm test:e2e` 运行 Playwright 端到端回归。

## Launcher 开发入口

- 在 `launcher/` 下执行 `pnpm install --frozen-lockfile` 安装 Electron 启动器依赖。
- `pnpm test` 运行主进程、服务层、托盘菜单与渲染层测试。
- `pnpm build` 产出当前平台的 Electron 目录包。
- 当前 Launcher 只复用既有 server management surface：`healthz`、`readyz`、`setup/status`、`session/launcher-token`、`session/launcher-admission`、`system/status`、`system/shutdown`、`GET /api/system/diagnostics/export`。

## CLI 子命令

`raylea-server` 提供以下管理子命令，适用于脱机维护场景：

| 子命令 | 说明 |
|--------|------|
| `reset-admin` | 重置管理员密码 |
| `backup` | 创建数据备份归档 |
| `restore` | 从备份归档恢复数据 |
| `doctor` | 环境与数据一致性诊断 |
| `migrate` | 手动触发数据库迁移 |
| `cleanup` | 清理过期数据与临时文件 |

## CI 门禁文件

`.github/workflows/` 当前包含以下门禁工作流：

| 文件 | 用途 |
|------|------|
| `contracts.yml` | 验证 contracts fixtures 与 schema 一致性 |
| `lint.yml` | Go lint（golangci-lint v1.64.8）+ 覆盖率门禁（55%）|
| `race.yml` | Go race detector 回归 |
| `release.yml` | 跨平台构建、打包与发布 |
| `self-host-smoke.yml` | 自托管烟雾测试 |

## 协作规则

- 开始业务实现前先确认 baseline、contracts 与 `docs/engineering/implementation-order.md` 的边界。
- 开发说明用于提供工作入口、调试路径和排障上下文，不单独定义对外接口。
- 若当前实现与目录说明存在漂移，优先以 `contracts/`、工程基线文件和已落地主链路为准。
