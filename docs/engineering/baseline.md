# RayleaBot Engineering Baseline

## 目的

本文件固定 RayleaBot 的工程版本线、默认命令、目录职责和长期有效的实现选型。进入具体实现前，应按改动领域读取对应正式来源。

不同领域分别决定，不使用跨领域的全局优先级：

- 产品目标、范围、顶层架构与路线图以 `docs/RayleaBot机器人项目规划.md` 为准。
- HTTP、WebSocket、schema、错误码、事件、CLI、插件协议与发布元数据以 `contracts/` 为准。
- 工具链、默认命令、目录职责与固定工程选型以本文件及对应工程文件为准。

来源之间发生冲突时，先在冲突所属领域的正式来源中作出决定，再同步全部 companion；产品规划不能覆盖已经冻结的对外 contract。

## 工程目录与职责

- `server/` 是产品核心，负责配置、存储、鉴权、任务、插件发现、OneBot11 adapter、多插件 runtime、dispatcher、scheduler trigger、插件浏览器会话、管理面日志持久化。
- `web/` 负责管理控制台主路径。
- `launcher/` 负责 Wails 桌面启动器、本地环境检查、服务进程编排、桌面交互与打开 Web 管理面。
- `.deps/manifest.json` v5 固定图片渲染与插件浏览器会话共用的 Chromium，以及受信本地插件共用的 FFmpeg / FFprobe 资源矩阵和可信来源列表；插件运行不依赖托管语言运行时。
- 运行环境有效根目录按 `config/user.yaml` 的上两级目录推导；Launcher `workdir` 只承担进程工作目录与日志目录职责，不覆盖 `.deps/` 与 `templates/` 的位置。

## 固定版本线

Go、Node.js、Python、pnpm、npm、Corepack 和 sqlc 的版本值由根目录 `.tool-versions` 维护。工作流在安装工具前读取该文件；doctor、契约校验与插件开发工作区也从此处取值。`go.mod`、`package.json` 等生态必填声明由 doctor 与契约校验器核对，下表供阅读。

| 领域 | 固定基线 |
| --- | --- |
| Server | Go `1.27.1` |
| Web / build runtime | Node.js `26.10.0` + npm `11.19.1` |
| JS package bootstrap | Corepack `0.35.0` |
| JS package manager | `pnpm 11.25.0` |
| Web UI | Vue `3.5.41` + Vite `8.2.1` + Reka UI `2.10.4` + shadcn-vue 自有组件源码 + Motion for Vue `2.4.2` + Vue Router `5.2.0` + Pinia `4.0.3` |
| Launcher runtime | Wails v3 `v3.0.0-beta.9` + `@wailsio/runtime 3.0.0-beta.9` + Go `1.27.1` + TypeScript `5.9.3` + Vue `3.5.41` + Reka UI `2.10.4` + Motion for Vue `2.4.2` + Vite `8.2.1` + `@vitejs/plugin-vue 6.0.8` |
| Repository scripting | Go `1.27.1`（`tools/`）+ Node.js `26.10.0`；插件协议生成与发布工具保留 Python `3.14.8` |
| Go static analysis | golangci-lint `v2.13.0`（支持 Go 1.27） |
| SQL generation | sqlc `v1.31.1` |
| Plugin backend | 当前平台预编译原生 artifact；官方 Go 插件使用 Go `1.27.1` 与 `CGO_ENABLED=0` 构建 |
| Plugin UI | Vue `3.5.41` + TypeScript `5.9.3` + Vite `8.2.1` + `@rayleabot/plugin-ui` |
| Database | SQLite via `modernc.org/sqlite v1.56.0` |
| Render | `chromedp 0.16.0` + Chrome for Testing `152.0.7977.42` |
| Media tools | Windows / Linux 使用 BtbN FFmpeg Builds `n9.0.1-11-ge47273f4d9-20260831` GPL shared build；macOS arm64 使用 Martin Riedl `9.0.2-1789931890` GPL release build |
| macOS release runner | `macos-26` |

Windows / Linux 的 FFmpeg 固定使用 BtbN [2026-08-31 月末构建](https://github.com/BtbN/FFmpeg-Builds/releases/tag/autobuild-2026-08-31-13-27)，保留 9.0.1 维护线，使用 gpl-shared 变体：编解码能力与 GPL 静态构建相同（含 libx264/libx265），可执行文件链接包内共享库，下载量约为静态构建的一半。按[上游保留规则](https://github.com/BtbN/FFmpeg-Builds#release-retention-policy)，月末构建保留两年，普通日构建只保留最近 14 版；固定日期 URL 不代表永久可用。每次分发前仍需验证来源与 SHA-256，更新构建时同步资源版本、归档摘要和入口路径。

macOS arm64 使用 [Martin Riedl 的 9.0.2 固定发布构建](https://ffmpeg.martin-riedl.de/download/macos/arm64/1789931890_9.0.2/versions.txt)，包含 libx264/libx265，未启用 nonfree；FFmpeg 与 FFprobe 分包下载，各自校验摘要，在同一临时资源目录准备并完整验证后启用。来源与编码能力变化用于替换许可证不符合 GPL 要求的旧构建，不增加新的编解码技术栈。

Web 管理面使用 Reka UI 与自有产品组件，组件与界面规则见 [`DESIGN.md`](../../DESIGN.md)，工程约束见 [`web/AGENTS.md`](../../web/AGENTS.md)。

## 工具链获取

- 仓库根目录的 `.tool-versions` 固定七种工具的版本。doctor 核对已安装工具、各 Go module 与 JS package 的声明；CI 与开发容器安装步骤从该文件读取版本。Docker 的 Go/Python 基础镜像标签需要在解析 Dockerfile 时确定，保留显式声明，由严格契约门禁检查一致性。
- `go run ./tools/cmd/check-toolchain --task server --toolchain-only` 只检查服务端编译工具；`web`、`launcher`、`contracts`、`sql`、`runtime` 可选择对应任务。默认 `all` 检查全部构建与契约工具，版本错误仍失败；已安装可用 pnpm 时不另要求 Corepack，开发启动脚本仍通过 Corepack 选择工程锁定的 pnpm。
- `server/go.mod` 的 `go` 指令是 Go 工具识别的最低版本声明，与 `.tool-versions` 保持一致；当前保持 patch 级锁定，不使用单独 `toolchain` 指令替代。离线环境需要预装同一 Go 版本，并设置 `GOTOOLCHAIN=local` 让版本错误在本地直接失败。
- npm 随 Node.js 提供；Corepack 单独安装：按 `.tool-versions` 中的版本执行 `npm install --global corepack@<version>`，再执行 `corepack enable` 与 `corepack prepare pnpm@<version> --activate`。
- sqlc 单独安装：按 `.tool-versions` 中的版本执行 `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v<version>`。
- 无网络环境需要提前把 Go、Node.js、Corepack pnpm、sqlc 和 `.deps/manifest.json` 对应的 Chromium、FFmpeg 资源放入镜像或工作站。Chromium 可使用系统 Chrome / Chromium / Edge，也可使用 `.deps/store/` 中已展开的托管资源；FFmpeg 与 FFprobe 使用清单内固定的托管资源。
- Linux 构建 Wails Launcher 固定使用 Wails v3.0.x 支持的 `gtk3` 兼容标签，需要 GTK 3 与 WebKit2GTK 4.1 开发包；Ubuntu 使用 `libgtk-3-dev` 和 `libwebkit2gtk-4.1-dev`。
- Python 脚本依赖集中在 `scripts/requirements.txt`；首次运行执行 `python -m pip install -r scripts/requirements.txt`，其中 jsonschema 的 format 扩展用于日期与 URI 等格式校验。
- `tools/` 是独立 Go module，开发与 CI 命令在仓库根目录通过 `go run ./tools/cmd/<name>` 执行。YAML 解析复用固定的 `go.yaml.in/yaml/v3 v3.0.5`；契约校验使用 `github.com/santhosh-tekuri/jsonschema/v6 v6.0.3` 的 Draft 2020-12 并开启 format 断言，检查日期、URI 等格式。不依赖 OpenAPI 库或 Server 内部包。doctor 的数据库目录检查通过临时文件创建、写入与同步验证权限；SQLite 行为由 Server 存储测试覆盖。
- 仓库提供 devcontainer，预装 `.tool-versions` 中的全部工具、上述 Python 依赖以及 Chromium、SQLite 和 make。
- 本地环境诊断入口是仓库根目录的 `make doctor`，无 make 环境时运行 `go run ./tools/cmd/check-toolchain`。

## 固定工程选型

| 领域 | 固定选型 |
| --- | --- |
| HTTP 路由 | `net/http` + `go-chi/chi v5.3.1` |
| WebSocket | `github.com/coder/websocket v1.8.15` |
| 日志 | `log/slog` |
| 配置解析 | `go.yaml.in/yaml/v3` |
| 数据访问 | `database/sql` + repository / service 分层 + `internal/sqlcqueries` → `internal/sqlcgen` 的 sqlc 生成主路径；sqlc 无法表达的动态查询与 SQLite 维护语句保留手写，并在调用处注释原因 |
| Web 路由 | Vue Router `5.x` |
| Web 全局状态 | Pinia `4.x`，由各领域 store 维护管理状态 |
| Web HTTP | `lib/http.ts` 统一维护 RayleaBot 鉴权、错误与下载语义 |
| Web 实时通信 | 原生 `WebSocket` + 受控连接封装 |
| Web 样式 | 共享设计 token + 自有 shadcn-vue 组件 + Tailwind CSS `4.x` + Vue SFC SCSS / CSS Variables |
| Web 动效 | Motion for Vue 管理产品浮层、内容变化与导航降级动画；页面和主题继续使用受控 View Transition API，CSS transition 承担简单控件状态 |
| Launcher 桌面宿主 | Wails v3 Go host + `internal/desktop` typed service layer |
| Launcher 桌面桥接 | Wails generated bindings 暴露受限 typed API |
| Launcher 渲染层 | Vue 3 + Reka UI 无样式原语与自有组件 + Lucide 图标 + Motion for Vue + View Transition API + Vite 单页面桌面壳，支持亮/暗双色主题；与 Web 共用前端框架与组件原语，组件代码各自维护 |
| Launcher 动效 | Motion for Vue 管理对话框、工作区与界面状态动画，主题菜单使用 CSS 进入动画；主题由 Motion 驱动受控 View Transition 快照，CSS transition 承担简单控件状态 |
| 仓库级 JS 包管理器 | `pnpm` |
| 插件后端 | 当前平台原生可执行文件；实现语言不限。Go 插件可使用独立 module 与 `sdk/go`，`cmd/<plugin-id>` 为推荐入口 |
| 插件管理页 | 独立 Vue package + `sdk/vue`；Vite 固定 `base: "./"`，产物位于 artifact 的 `ui/` |
| 插件构建 | `raylea-plugin inspect/pack/build-go` 统一检查、通用原生打包和 Go 构建；输出 artifact v2 单根目录 ZIP 与可选展开目录 |
| 插件商店 | 默认使用 `RayleaBot/plugin-catalog` 的 catalog v2，并允许管理员添加自定义 HTTPS 来源；Server 持久化各来源最后一次成功目录 |
| 运行环境资源准备 | `.deps/manifest.json` 可信来源测速 + `cache/downloads/runtime/` + `.deps/store/<resource-id>/<version>/`；图片渲染和插件浏览器会话可复用已安装的 Chrome、Chromium、Edge 或托管 Chromium，受信本地插件通过启动环境读取托管 FFmpeg / FFprobe 入口 |

## 当前评估方向

新增运行时依赖或替换固定选型时，说明要解决的具体问题、现有技术栈为何不足、是否引入平行技术栈、回滚路径，以及对 CI、发布打包、lockfile、fixture 和生成文件的影响。只用于开发、测试或仓库脚本的依赖说明必要性即可。

| 领域 | 当前方向 |
| --- | --- |
| 数据库初始化 | 从当前 `schema.sql` 在事务内初始化；旧结构按 `store_schema.go` 的有序前向迁移表升级，每步一个事务并更新 `schema_metadata`，失败回滚；不引入额外迁移框架 |
| OpenAPI 实现 | 保留严格契约校验和生成类型检查；只有 handler 漂移持续发生时才评估 Server 侧 OpenAPI 代码生成 |
| Secret 存储 | secret 原值保存在 SQLite 独立存储，配置只保存 `secret://` 引用，管理面不回显；部署目标要求外部密钥托管时再评估环境密钥、操作系统 keychain 或外部 KMS |
| 架构门禁 | `server/tests/architecture` 中的 Go AST 测试检查包命名、导入边界、内部包退出进程调用及 management 手写 SQL；在 `server/` 执行 `go test ./tests/architecture`，也随全量 Go 测试运行 |
| 媒体处理 | 使用 `.deps/manifest.json` 固定三平台 full GPL FFmpeg / FFprobe 资源，不在各插件内重复打包，也不新增 Go 媒体编解码栈 |

## 默认命令

### Shell

- 直接在当前 shell 中执行 `node`、`go`、`python` 和 `pnpm`，命令语法按所用 shell 调整。
- Windows 开发启动使用 `.\start.bat`；POSIX 环境使用 `sh start.sh`。

### Server

- 构建：`mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server`
- 测试：`go test ./...`

### Web

- 安装：`pnpm install --frozen-lockfile`
- 开发：`pnpm dev`
- 类型检查：`pnpm run typecheck`
- 构建：`pnpm build`
- 单元测试：`pnpm test`
- E2E：`pnpm test:e2e:production`

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

### Repository tools

- 静态检查：`go vet ./tools/...`
- 测试：`go test -count=1 ./tools/...`
- 环境诊断：`make doctor` 或 `go run ./tools/cmd/check-toolchain`
- 文档链接：`go run ./tools/cmd/check-doc-links`
- 契约校验：`go run ./tools/cmd/validate-contracts --mode=strict`；CLI fixture 语义自检：`go run ./tools/cmd/validate-contracts --self-test`
- 错误码生成物：`go run ./tools/cmd/generate-error-codes --verify`

## 目录职责

| 路径 | 职责 |
| --- | --- |
| `contracts/` | 对外正式契约根目录 |
| `docs/engineering/` | 工程基线、质量门禁、人工 smoke |
| `docs/architecture/` | 组件职责、消息主流程与状态归属概览 |
| `docs/dev/` | 开发、调试、诊断、贡献流程 |
| `docs/plugin/` | 插件 manifest、协议、生命周期 |
| `docs/plugin/sdk/` | Go 插件 SDK、构建器与 Vue 管理页 SDK 说明 |
| `docs/user/` | 用户安装、初始化、配置、运行、恢复 |
| `docs/release/` | 版本说明、迁移说明、已知问题 |
| `fixtures/` | Golden fixtures 与可执行样例 |
| `examples/` | 示例插件、manifest 与示例请求/响应 |
| `server/` | Go 服务端工程 |
| `web/` | Web UI 工程 |
| `launcher/` | Wails 桌面启动器工程 |
| `tools/` | 独立 Go module，提供仓库开发、生成与 CI 工具及其测试 |
| `plugins/installed/` | 运行期统一安装目录；只保存经 artifact 校验的商店、社区或开发插件产物，不进入版本控制 |
| `sdk/go/` | Go 插件 JSONL 客户端、typed local-action helpers 与 artifact 构建器 |
| `sdk/vue/` | `@rayleabot/plugin-ui` 同源管理 API client、composables 与主题 |
| `.deps/` | Chromium 与 FFmpeg / FFprobe 资源清单，以及按需展开后的资源目录 |
| `config/` | 用户配置 |
| `data/` | SQLite 状态库与运行数据 |
| `cache/` | 渲染缓存、下载缓存、插件临时缓存 |
| `logs/` | 结构化日志与诊断输出 |

## 仓库级强制基线文件

| 路径 | 约束 |
| --- | --- |
| `server/go.mod` | 固定 `module github.com/RayleaBot/RayleaBot/server`、与 `.tool-versions` 一致的 Go 版本与 server 依赖版本 |
| `server/go.sum` | 维护 server 依赖锁定结果 |
| `web/package.json` | 固定与 `.tool-versions` 一致的 `packageManager` 与 `engines.node` |
| `web/pnpm-lock.yaml` | 作为 Web 工程唯一 JS 锁文件 |
| `launcher/go.mod` | 固定与 `.tool-versions` 一致的 Go 版本、Wails v3 Go module 与桌面宿主依赖 |
| `launcher/go.sum` | 维护 Launcher Go 依赖锁定结果 |
| `launcher/package.json` | 固定与 `.tool-versions` 一致的 `packageManager`、`engines.node`，以及 Wails runtime/Vite/Vue/`@vitejs/plugin-vue` 与构建脚本 |
| `launcher/pnpm-lock.yaml` | 作为 Launcher 工程唯一 JS 锁文件 |
| `tools/go.mod` | 固定仓库工具 module、与 `.tool-versions` 一致的 Go 版本及工具依赖 |
| `go.work` | 连接 tools、server、Go SDK 和 Go 示例的主仓库工作区；Launcher 使用独立 Go module，启动与构建脚本固定 `GOWORK=off`，避免 Wails 依赖改变 server 的模块选择；独立插件只通过本地临时开发工作区连接 |
| `.deps/manifest.json` | 固定资源名、版本线、可信来源列表、SHA256、archive_format、entrypoints 与平台矩阵 |
| `contracts/*` | 对外接口、协议、schema、错误码、事件、CLI 与发布元数据的正式来源；文件清单与职责见 [`contracts/README.md`](../../contracts/README.md) |

## 已冻结的规范化决议

- `contracts/config.user.schema.json` 中 `server.host` 默认值采用 `0.0.0.0`。
- 聊天适配器配置正式形状是 `adapters` 实例列表：连接键名采用 `adapters[].onebot11.reverse_ws.url`、`adapters[].onebot11.forward_ws.url`、`adapters[].onebot11.http_api.url` 与 `adapters[].onebot11.webhook.url`，实例由 `adapters[].id` 标识。
- `launcher/go.mod` 与 `launcher/package.json` 共同锁定 Wails 启动器的 Go host、typed runtime、构建形态与 Node / pnpm 基线。原生托盘和单实例能力依赖当前固定的 Wails v3 预发布版本，变更版本必须同步验证 Go bindings、三平台构建与发布包布局。
- `server/go.mod` 采用 `github.com/RayleaBot/RayleaBot/server` 作为 module path。
