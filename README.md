# RayleaBot

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/RayleaBot/RayleaBot)](https://github.com/RayleaBot/RayleaBot/releases)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![Node.js](https://img.shields.io/badge/Node.js-26+-339933?logo=nodedotjs)](https://nodejs.org)
[![Vue](https://img.shields.io/badge/Vue-3.5-42b883?logo=vuedotjs)](https://vuejs.org)

面向个人开发者和开源协作者的自托管聊天机器人框架。通过 OneBot11 或 QQ 官方机器人适配器接入聊天平台，提供插件扩展、Web 管理控制台和桌面启动器，运行状态与凭据由本机管理。

## 核心特性

- **自托管**：服务端、插件、管理面板全部运行在本地，无需云端控制面板。
- **多适配器实例**：支持 OneBot11 和 QQ 官方机器人，同一协议可配置多个实例。OneBot11 提供 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook` 四种连接方式；身份、凭据和消息路由按实例隔离。
- **原生插件**：插件后端使用当前平台的预编译原生可执行文件，实现语言不限，通过 JSONL v3 协议与服务端通信。Go SDK 和构建器提供一等开发支持；插件管理页由包内静态资源组成，官方页面使用 Vue 并运行在独立插件域。
- **三方账号**：支持 Bilibili、微博、抖音和网易云音乐账号的 CK 保存、扫码登录、资料与凭据校验；订阅与内容监控由独立插件 `raylea.subscription-hub` 提供。
- **Web 管理控制台**：仪表盘、插件列表与商店、三方账号、菜单中心、指令中心、权限与限流、任务调度、日志检索和模板预览。
- **桌面启动器**：基于 Wails，支持 Windows / macOS / Linux，提供一键启动、环境预检、进程编排和原生系统托盘。
- **契约驱动**：HTTP / WebSocket / 插件协议等对外接口统一以 `contracts/` 为准，实现和测试都对照它编写。

## 快速开始

### 方式一：下载发行包（推荐）

在 [GitHub Releases](https://github.com/RayleaBot/RayleaBot/releases) 下载对应平台的完整包：

| 平台 | 发行包 | 入口 |
|---|---|---|
| Windows | `RayleaBot-v<版本>-windows-x64-full.zip` | `RayleaLauncher.exe` |
| Linux 桌面 | `RayleaBot-v<版本>-linux-x64-full.tar.gz` | `RayleaLauncher` |
| macOS (Apple Silicon) | `RayleaBot-v<版本>-macos-arm64-full.tar.gz` | `RayleaLauncher.app` |
| Linux 无桌面 / 服务器 | `RayleaBot-v<版本>-linux-x64-server.tar.gz` | `raylea-server` + `systemd` |

1. 下载并解压到固定目录，该目录即运行根目录。
2. 运行桌面入口或 `raylea-server`；服务器包可参考包内 `systemd/rayleabot.service` 托管。
3. 浏览器访问 `http://127.0.0.1:8080`，按引导完成管理员初始化。

Windows Launcher 需要系统安装 Microsoft Edge WebView2 Runtime，Linux 桌面 Launcher 需要 GTK 3 和 WebKit2GTK 4.1；对应完整包内的 `WINDOWS-RUNTIME.md`、`LINUX-RUNTIME.md` 提供安装说明。

首个支持签名更新的版本需要手动安装。Launcher 每 6 小时检查一次更新；Windows 只有在 Ed25519 发布签名和正式 Authenticode 全部通过时才提供用户确认后的事务安装，Linux、macOS 和未满足签名门槛的 Windows 包使用引导更新。

完整部署与发布信任说明见 [`docs/user/deployment.md`](./docs/user/deployment.md) 和 [`docs/release/delivery-and-upgrade.md`](./docs/release/delivery-and-upgrade.md)。

### 方式二：从源码启动

前置条件：Go 1.26.6、Node.js 26.7.0（自带 npm 11.19.0）、Corepack 0.35.0、pnpm 11.22.0、Python 3.14.7、sqlc 1.31.1、Git 2.x，以及系统 Chrome / Chromium / Edge 或已经准备完成的托管 Chromium。FFmpeg / FFprobe 由运行环境清单准备，无需单独安装。Node.js 26 需要先运行 `npm install --global corepack@0.35.0`。
`.tool-versions` 只固定 Go、Node.js、Python 和 pnpm；npm 随 Node.js 提供，Corepack 与 sqlc 需要单独安装并由 doctor 脚本校验。
工具链检查：`make doctor`；无 make 环境时运行 `python scripts/check-toolchain.py` 和 `python scripts/check-server-structure.py`。离线环境需要预装 Go 1.26.6，并设置 `GOTOOLCHAIN=local` 让版本错误在本地直接失败。
Devcontainer 位于 `.devcontainer/`，可直接提供 server tests 所需的 Go、Node、pnpm、sqlc、Chromium 与 SQLite 环境。

```bash
git clone https://github.com/RayleaBot/RayleaBot.git
cd RayleaBot

# Windows
start.bat

# Linux / macOS
sh start.sh

# 也可在任意平台直接运行统一编排器
node scripts/start-dev.mjs
```

开发模式下，服务端监听 `http://127.0.0.1:8080`，Web 开发服务器运行在 `http://127.0.0.1:4173`。

主仓库没有内置插件。需要联调独立插件时，复制 `plugin-workspace.example.json` 为 `plugin-workspace.local.json`。本地启动参数可复制 `.env.example` 为 `.env`；其中同时设置 `RAYLEA_PLUGIN_DEV=watch` 与 `RAYLEA_SERVER_RELOAD=watch` 即可持续联调。存在插件工作区但未显式设置模式时，启动脚本默认在 Server 启动前构建并同步所有启用插件。

`watch` 首次启动执行一次全量同步，之后按插件 ID 合并变更并只重新构建本批发生变化的插件；构建期间到达的后续变更保留到下一批。主仓库通过统一 `raylea-plugin` 工具构建或打包完整 artifact，开发同步复用正式安装事务进入 `plugins/installed/`，运行中的服务通过受控开发接口完成同步。本地联调不请求 GitHub；插件仓库的 GitHub Actions 只负责 `v*` tag 的三平台正式 Release。完整流程见[插件商店与独立开发](./docs/plugin/store-and-development.md#本地同步开发)。

## 使用简介

- 管理面板默认只在本机开放，远程访问需在配置中显式开启，并建议通过 HTTPS 反向代理。
- 在协议中心添加 OneBot11 或 QQ 官方机器人实例并完成连接配置后，即可在相应聊天窗口与机器人交互。
- 插件商店展示官方和自定义 HTTPS 目录中的条目，并保留各来源最后一次成功读取的缓存；安装前会展示插件身份、权限和本机原生代码确认要求。
- 所有插件统一安装在运行根目录的 `plugins/installed/`，只接受与当前平台匹配、通过 artifact 结构与原生入口校验的目录或单根目录 ZIP。
- 管理员可在管理面板中配置权限策略、黑白名单、命令前缀、任务调度等。

## 文档

| 文档 | 说明 |
|---|---|
| [项目章程](./docs/RayleaBot机器人项目规划.md) | 产品使命、长期边界与工程原则 |
| [界面设计](./docs/design/README.md) | 共享视觉规范、各界面规范与采用状态 |
| [架构总览](./docs/architecture/README.md) | 内部设计、事件模型、状态模型 |
| [插件开发](./docs/plugin/README.md) | 生命周期、manifest、协议、SDK |
| [插件商店与独立开发](./docs/plugin/store-and-development.md) | 商店信任、独立发布和本地同步联调 |
| [用户指南](./docs/user/README.md) | 部署、配置、CLI、恢复 |
| [0.4.0 候选分发说明](./docs/release/notes/v0.4.0.md) | 本轮全新安装、插件接入、本版恢复与尚待完成的发布验收 |
| [工程基线](./docs/engineering/baseline.md) | 版本线、选型、目录职责 |
| [CHANGELOGS](./docs/CHANGELOGS/) | 版本变更记录 |

## 贡献与开发

独立 Go 插件统一使用 `cmd/<plugin>` 进程入口、`internal/` 实现与嵌入资源以及可选 `ui/`/`templates/` 资源，并使用 `raylea-plugin build-go`；其他语言先生成原生入口，再使用 `raylea-plugin pack`。完整目录约定见[插件 SDK](./docs/plugin/sdk/README.md#raylea-plugin)。

```bash
# Server
cd server && go test ./...

# Web
cd web && pnpm install --frozen-lockfile && pnpm test

# Launcher
cd launcher && pnpm install --frozen-lockfile && pnpm test

# Go 插件 SDK
cd sdk/go && go test ./...

# Vue 插件 UI SDK
cd sdk/vue && pnpm install --frozen-lockfile && pnpm run typecheck && pnpm test

# 在主仓库调用统一工具构建相邻 Go 插件的当前平台 artifact
go run ./sdk/go/cmd/raylea-plugin build-go --plugin ../RayleaBotPlugins/plugin-fortune --target windows-x64 --out ../RayleaBotPlugins/plugin-fortune/dist
```

项目采用契约优先（contract-first）模式。修改任何对外接口前，请先更新 `contracts/` 中的对应契约文件，再同步实现与测试。

## License

[AGPL-3.0](LICENSE)

## 仓库动态

> 以下图表由 [`.github/workflows/repo-stats.yml`](.github/workflows/repo-stats.yml) 在推送到 `main` 时自动生成，反映本仓库最近一年的提交活动。

![月度提交折线图](https://raw.githubusercontent.com/RayleaBot/RayleaBot/output/repo-activity-line.svg)

![周提交热力图](https://raw.githubusercontent.com/RayleaBot/RayleaBot/output/repo-activity-heatmap.svg)
