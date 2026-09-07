# RayleaBot Engineering Baseline

## 目的

本文件固定 RayleaBot 的工程版本线、默认命令、目录职责和长期有效的实现选型。进入具体实现前，应按改动领域读取对应正式来源。

不同领域分别决定，不使用跨领域的全局优先级：

- 产品目标、范围、顶层架构与路线图以 `docs/RayleaBot机器人项目规划.md` 为准。
- HTTP、WebSocket、schema、错误码、事件、CLI、插件协议与发布元数据以 `contracts/` 为准。
- 工具链、默认命令、目录职责与固定工程选型以本文件及对应工程文件为准。
- fixtures、examples、实现和说明文档必须跟随所属领域的正式来源，不能反向覆盖正式 contract。

来源之间发生冲突时，先在冲突所属领域的正式来源中作出决定，再同步全部 companion；产品规划不能覆盖已经冻结的对外 contract。

## 当前工程落点

- `server/` 是产品核心，负责配置、存储、鉴权、任务、插件发现、OneBot11 adapter、多插件 runtime、dispatcher、scheduler trigger、三方账号、管理面日志持久化与运行指标。
- `web/` 负责管理控制台主路径。
- `launcher/` 负责 Wails 桌面启动器、本地环境检查、服务进程编排、桌面交互与打开 Web 管理面。
- `.deps/manifest.json` v5 固定图片渲染与抖音扫码浏览器兜底共用的 Chromium，以及受信本地插件共用的 FFmpeg / FFprobe 资源矩阵和可信来源列表；插件运行不依赖托管语言运行时。
- 运行环境有效根目录按 `config/user.yaml` 的上两级目录推导；Launcher `workdir` 只承担进程工作目录与日志目录职责，不覆盖 `.deps/` 与 `templates/` 的位置。
- 恢复人工处理与运行环境准备继续复用共享任务模型；`recovery.recheck`、`recovery.confirm` 与 `runtime.bootstrap` 是当前正式操作入口。

## 固定版本线

| 领域 | 固定基线 |
| --- | --- |
| Server | Go `1.26.6` |
| Web / build runtime | Node.js `26.7.0` + npm `11.19.0` |
| JS package bootstrap | Corepack `0.35.0` |
| JS package manager | `pnpm 11.22.0` |
| Web UI | Vue `3.5.41` + Vite `8.2.1` + Reka UI `2.10.4` + shadcn-vue 自有组件源码 + Motion for Vue `2.4.2` + Vue Router `5.2.0` + Pinia `4.0.3`；迁移期保留未替换页面所需 Ant Design Vue `4.2.6` 与 Motion Mini `13.1.0` |
| Launcher runtime | Wails v3 `v3.0.0-beta.9` + `@wailsio/runtime 3.0.0-beta.9` + Go `1.26.6` + TypeScript `5.9.3` + React `19.2.8` + Fluent UI React v9 + Fluent Motion `9.16.2` + Vite `8.2.1` + `@vitejs/plugin-react 6.0.5` |
| Repository scripting | Python `3.14.7` |
| SQL generation | sqlc `v1.31.1` |
| Plugin backend | Go `1.26.6`，`CGO_ENABLED=0` 的平台预编译 artifact |
| Plugin UI | Vue `3.5.41` + TypeScript `5.9.3` + Vite `8.2.1` + 按需 Ant Design Vue |
| Database | SQLite via `modernc.org/sqlite v1.56.0` |
| Render | `chromedp 0.16.0` + Chrome for Testing `152.0.7977.42` |
| Media tools | Windows / Linux 使用 BtbN FFmpeg Builds `n9.0.1-6-g9d4ca21220` full GPL build；macOS arm64 使用 vanloctech `ffmpeg-2026.06.11` |
| Metrics | `github.com/prometheus/client_golang 1.24.1`（Prometheus 文本暴露格式） |
| macOS CI / release runner | `macos-26` |

Web 管理面按 [`web-admin-baseline.md`](./web-admin-baseline.md) 迁移至 Reka UI 与自有产品组件。正式工作区逐批切换，迁移范围和退出条件见[执行计划](../execution-plan-v1.md)。

## 工具链获取

- 仓库根目录的 `.tool-versions` 只固定 Go、Node.js、Python 与 pnpm，可由 mise 或 asdf 读取。npm 随 Node.js 提供；Corepack 与 sqlc 不在该文件中，由下列独立安装步骤和 doctor 校验覆盖。
- `server/go.mod` 的 `go 1.26.6` 是 CI 与本地 server 测试的 Go 版本来源；当前保持 patch 级锁定，不使用单独 `toolchain` 指令替代。离线环境需要预装 Go 1.26.6，并设置 `GOTOOLCHAIN=local` 让版本错误在本地直接失败。
- Node.js 使用 26.7.0，并使用其内置 npm 11.19.0。Node.js 26 不再随发行包提供 Corepack，因此先执行 `npm install --global corepack@0.35.0`，再执行 `corepack enable` 与 `corepack prepare pnpm@11.22.0 --activate`。
- sqlc 固定为 v1.31.1，安装命令为 `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`。
- 无网络环境需要提前把 Go、Node.js、Corepack pnpm、sqlc 和 `.deps/manifest.json` 对应的 Chromium、FFmpeg 资源放入镜像或工作站。Chromium 可使用系统 Chrome / Chromium / Edge，也可使用 `.deps/store/` 中已展开的托管资源；FFmpeg 与 FFprobe 使用清单内固定的托管资源。
- Linux 构建 Wails Launcher 固定使用 Wails v3.0.x 支持的 `gtk3` 兼容标签，需要 GTK 3 与 WebKit2GTK 4.1 开发包；Ubuntu 使用 `libgtk-3-dev` 和 `libwebkit2gtk-4.1-dev`。
- 仓库提供 devcontainer，包含 Go 1.26.6、Node.js 26.7.0、npm 11.19.0、Corepack 0.35.0、pnpm 11.22.0、Python 3.14.7、sqlc v1.31.1、Chromium、SQLite 与 `make doctor`。
- 本地环境诊断入口是仓库根目录的 `make doctor`，无 make 环境时运行 `python scripts/check-toolchain.py` 和 `python scripts/check-server-structure.py`。

## 固定工程选型

| 领域 | 固定选型 |
| --- | --- |
| HTTP 路由 | `net/http` + `go-chi/chi v5.3.1` |
| WebSocket | `github.com/coder/websocket v1.8.15` |
| 运行指标 | `github.com/prometheus/client_golang` + 受 admin session 保护的 `/api/system/metrics` |
| 日志 | `log/slog` |
| 配置解析 | `gopkg.in/yaml.v3` |
| 数据访问 | `database/sql` + repository / service 分层 + `internal/sqlcqueries` → `internal/sqlcgen` 的 sqlc 生成主路径；必须保留的手写 SQL 登记在 `docs/engineering/manual-sql-exceptions.json` |
| Web 路由 | Vue Router `5.x` |
| Web 全局状态 | Pinia `4.x` + Vben stores 对齐组织 |
| Web HTTP | Vben request 风格封装 + RayleaBot 鉴权 / 错误语义适配 |
| Web 实时通信 | 原生 `WebSocket` + 受控连接封装 |
| Web 样式 | 共享设计 token + 自有 shadcn-vue 组件 + Tailwind CSS `4.x` + Vue SFC SCSS / CSS Variables；旧页面暂保留 Ant Design tokens |
| Web 动效 | Motion for Vue 管理产品浮层与内容变化；既有页面保留 View Transition API / `motion/mini` 至迁移完成，CSS transition 承担简单控件状态 |
| Launcher 桌面宿主 | Wails v3 Go host + `internal/desktop` typed service layer |
| Launcher 桌面桥接 | Wails generated bindings 暴露受限 typed API |
| Launcher 渲染层 | React 19 + Fluent UI React v9 + Fluent Motion + WAAPI + View Transition API + Vite 单页面桌面壳，支持亮/暗双色主题 |
| 仓库级 JS 包管理器 | `pnpm` |
| 插件后端 | 当前平台原生可执行文件；实现语言不限。Go 插件可使用独立 module 与 `sdk/go`，`cmd/<plugin-id>` 为推荐入口 |
| 插件管理页 | 独立 Vue package + `sdk/vue`；Vite 固定 `base: "./"`，产物位于 artifact 的 `ui/` |
| 插件构建 | `raylea-plugin inspect/pack/build-go` 统一检查、通用原生打包和 Go 构建；输出 artifact v2 单根目录 ZIP 与可选展开目录 |
| 插件商店 | 默认使用 `RayleaBot/plugin-catalog` 的 catalog v2，并允许管理员添加自定义 HTTPS 来源；Server 持久化各来源最后一次成功目录 |
| 运行环境资源准备 | `.deps/manifest.json` 可信来源测速 + `cache/downloads/runtime/` + `.deps/store/<resource-id>/<version>/`；图片渲染和抖音扫码浏览器兜底可复用已安装的 Chrome、Chromium、Edge 或托管 Chromium，受信本地插件通过启动环境读取托管 FFmpeg / FFprobe 入口 |

## 默认命令

### Shell

- Windows 环境执行仓库命令优先使用 `gbash -lc '<command>'`。
- `gbash` 包装 `C:\Program Files\Git\usr\bin\bash.exe`，并前置 Git Bash 的 `usr\bin`、`bin`、`cmd`。
- `gbash` 的仓库来源文件为 `scripts/gbash.ps1` 和 `scripts/gbash.cmd`；本机可执行入口位于 `%USERPROFILE%\.local\bin\`。
- 系统 `bash` 可能指向 WSL；仓库命令使用 `gbash`。
- 现有 `.bat` / `.cmd` 启动入口保持 Windows 原生命令文件；执行时可从 Git Bash 调用。

### Server

- 构建：`mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server`
- 测试：`go test ./...`

### Web

- 安装：`pnpm install --frozen-lockfile`
- 开发：`pnpm dev`
- 类型检查：`pnpm run typecheck`
- 构建：`pnpm build`
- 单元测试：`pnpm test`
- E2E：`pnpm test:e2e`

### Launcher

- 安装：`pnpm install --frozen-lockfile`
- 类型检查：`pnpm run typecheck`
- 测试：`pnpm test`
- 构建：`pnpm build`

### Plugin SDK and artifacts

- Go SDK：`cd sdk/go && go test ./...`
- Vue SDK：`cd sdk/vue && pnpm run typecheck && pnpm test && pnpm build`
- 通用打包：`raylea-plugin pack --plugin <plugin-root> --binary <native-executable> --target <windows-x64|linux-x64|macos-arm64> --out <output>`
- Go 插件构建：`raylea-plugin build-go --plugin <plugin-root> --target <windows-x64|linux-x64|macos-arm64> --out <output>`
- 开发工作区验证：`node --test scripts/tests/plugin-dev-workspace.test.mjs`
- 插件矩阵由各独立插件仓库的 GitHub Actions 构建；主仓库 release 不构建业务插件。

## 目录职责

| 路径 | 职责 |
| --- | --- |
| `contracts/` | 对外正式契约根目录 |
| `docs/engineering/` | 工程基线、CI、实施顺序、治理规则 |
| `docs/architecture/` | 架构、状态模型、事件模型、边界说明 |
| `docs/dev/` | 开发、调试、诊断、贡献流程 |
| `docs/plugin/` | 插件 manifest、permissions、协议、生命周期 |
| `docs/plugin/sdk/` | Go 插件 SDK、构建器与 Vue 管理页 SDK 说明 |
| `docs/user/` | 用户安装、初始化、配置、运行、恢复 |
| `docs/release/` | 版本说明、迁移说明、已知问题 |
| `fixtures/` | Golden fixtures 与可执行样例 |
| `examples/` | 示例插件、manifest 与示例请求/响应 |
| `server/` | Go 服务端工程 |
| `web/` | Web UI 工程 |
| `launcher/` | Wails 桌面启动器工程 |
| `plugins/installed/` | 运行期统一安装目录；只保存经 artifact 校验的商店、社区或开发插件产物，不进入版本控制 |
| `sdk/go/` | Go 插件 JSONL 客户端、typed local-action helpers 与 artifact 构建器 |
| `sdk/vue/` | `@rayleabot/plugin-ui` bridge v3 client、composables、主题和 contract 类型 |
| `.deps/` | Chromium 与 FFmpeg / FFprobe 资源清单，以及按需展开后的资源目录 |
| `config/` | 默认配置模板与用户配置 |
| `data/` | SQLite 状态库与运行数据 |
| `cache/` | 渲染缓存、下载缓存、插件临时缓存 |
| `logs/` | 结构化日志与诊断输出 |

## 仓库级强制基线文件

| 路径 | 约束 |
| --- | --- |
| `server/go.mod` | 固定 `module github.com/RayleaBot/RayleaBot/server`、Go `1.26.6` 与 server 依赖版本 |
| `server/go.sum` | 维护 server 依赖锁定结果 |
| `web/package.json` | 固定 `packageManager = pnpm@11.22.0` 与 `engines.node = 26.7.0` |
| `web/pnpm-lock.yaml` | 作为 Web 工程唯一 JS 锁文件 |
| `launcher/go.mod` | 固定 Go `1.26.6`、Wails v3 Go module 与桌面宿主依赖 |
| `launcher/go.sum` | 维护 Launcher Go 依赖锁定结果 |
| `launcher/package.json` | 固定 `packageManager = pnpm@11.22.0`、`engines.node = 26.7.0`、Wails runtime/Vite/React/`@vitejs/plugin-react` 与构建脚本 |
| `launcher/pnpm-lock.yaml` | 作为 Launcher 工程唯一 JS 锁文件 |
| `go.work` | 连接 server、Go SDK 和 Go 示例的主仓库工作区；Launcher 使用独立 Go module，启动与构建脚本固定 `GOWORK=off`，避免 Wails 依赖改变 server 的模块选择；独立插件只通过本地临时开发工作区连接 |
| `.deps/manifest.json` | 固定资源名、版本线、可信来源列表、SHA256、archive_format、entrypoints 与平台矩阵 |
| `contracts/*` | 对外接口与错误码唯一正式来源 |

## 已冻结的规范化决议

- `contracts/config.user.schema.json` 中 `server.host` 默认值采用 `127.0.0.1`。
- 聊天适配器配置正式形状是 `adapters` 实例列表：连接键名采用 `adapters[].onebot11.reverse_ws.url`、`adapters[].onebot11.forward_ws.url`、`adapters[].onebot11.http_api.url` 与 `adapters[].onebot11.webhook.url`，实例由 `adapters[].id` 标识。
- `launcher/go.mod` 与 `launcher/package.json` 共同锁定 Wails 启动器的 Go host、typed runtime、构建形态与 Node / pnpm 基线。原生托盘和单实例能力依赖当前固定的 Wails v3 预发布版本，变更版本必须同步验证 Go bindings、三平台构建与发布包布局。
- `server/go.mod` 采用 `github.com/RayleaBot/RayleaBot/server` 作为 module path。

## `contracts/` 作为正式来源

以下边界的最终定义不在 Markdown，而在 `contracts/`：

- 插件 manifest：`contracts/plugin-info.schema.json`
- 插件 JSONL 协议：`contracts/plugin-protocol.schema.json`
- HTTP API：`contracts/web-api.openapi.yaml`
- WebSocket：`contracts/websocket-events.yaml`
- 用户配置：`contracts/config.user.schema.json`
- 错误码：`contracts/error-codes.yaml`
- 发行元数据：`contracts/release-manifest.schema.json`
- CLI：`contracts/cli-commands.yaml`

规则：

- 若后续变更尝试绕开 baseline 与 contracts 直接写功能代码，应视为违反仓库治理规则。
