# Developer Docs

本目录用于整理 RayleaBot 的开发、调试、诊断与贡献说明。

## 当前开发入口

当前仓库的主要实现面集中在以下区域：

- `server/`：Go 服务端主链路、管理面、适配器、runtime、存储与任务系统
- `contracts/`：当前正式接口、schema、错误码与 release metadata
- `fixtures/` 与 `examples/`：契约样例、golden cases、示例插件与示例配置
- `.github/workflows/`：contracts、baseline 与 server smoke 校验

`web/` 已进入真实开发主线，当前覆盖 `setup/login/session`、系统状态页、`plugins/tasks/logs/config` 页面、plugin install / uninstall / grants / console、`system/shutdown` 交互、响应式布局、无障碍细节、Pinia stores、统一 fetch / WebSocket client，以及 fixture-backed Vitest / Playwright 测试。

`launcher/` 已进入最小真实实现阶段，当前覆盖 loopback bootstrap auth、环境检查、server 启停、健康轮询、打开 Web UI、stderr 诊断摘要与 Windows 测试/发布命令。

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
- `pnpm dev` 启动 Vite 8 开发服务器；默认通过代理消费现有 server management surface。
- `pnpm test` 运行 Vitest 单测，覆盖 route guard、session store、WebSocket manager、plugin detail / grants、task query 自动展开、shutdown state 等关键交互。
- `pnpm test:e2e` 运行 Playwright；当前通过测试专用 mock backend 消费 `fixtures/web-api` 与 `fixtures/websocket`，覆盖 install / grants / shutdown / session 失效 / 移动端导航等正式场景，不依赖 live Go server。

## Launcher 开发入口

- 在仓库根目录执行 `dotnet test ./launcher` 运行 Launcher 状态机与失败路径测试。
- 在仓库根目录执行 `dotnet publish ./launcher -c Release` 构建最小 Launcher 发布产物。
- 当前 Launcher 只复用既有 server management surface：`healthz`、`readyz`、`setup/status`、`session/launcher-token`、`session/launcher-admission`、`system/status`、`system/shutdown`。

## 协作规则

- 开始业务实现前先确认 baseline、contracts 与 `docs/engineering/implementation-order.md` 的边界。
- 开发说明用于提供工作入口、调试路径和排障上下文，不单独定义对外接口。
- 若当前实现与目录说明存在漂移，优先以 `contracts/`、工程基线文件和已落地主链路为准。
