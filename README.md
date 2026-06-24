# RayleaBot

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/RayleaBot/RayleaBot)](https://github.com/RayleaBot/RayleaBot/releases)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![Node.js](https://img.shields.io/badge/Node.js-24+-339933?logo=nodedotjs)](https://nodejs.org)
[![Python](https://img.shields.io/badge/Python-3.12+-3776AB?logo=python)](https://www.python.org)

面向个人开发者和开源协作者的自托管聊天机器人框架。基于 OneBot11 协议接入 QQ，提供插件扩展、Web 管理控制台和桌面启动器，所有数据运行在本地。

## 核心特性

- **自托管**：服务端、插件、管理面板全部运行在本地，无需云端控制面板。
- **多平台接入**：支持 OneBot11 的 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook`。
- **多语言插件**：同时支持 Python 3.12 和 Node.js 24 插件，子进程隔离，通过 JSONL 协议通信。
- **Bilibili 集成**：直播监控、动态监控、扫码登录、多账号轮转，内置反风控与验证码处理。
- **Web 管理控制台**：仪表盘、插件管理、权限策略、任务调度、日志检索、模板预览等。
- **桌面启动器**：基于 Electron，支持 Windows / macOS / Linux，提供一键启动、环境预检和进程编排。
- **契约驱动**：HTTP / WebSocket / 插件协议等对外接口统一维护在 `contracts/`，实现与测试双向校验。

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

完整部署说明见 [`docs/user/deployment.md`](./docs/user/deployment.md)。

### 方式二：从源码启动

前置条件：Go 1.25.8、Node.js 24.14.0、pnpm 10.32.1、Python 3.12.13、Git 2.x。
工具链检查：`python scripts/check-toolchain.py`；安装 make 的环境可使用 `make doctor`。

```bash
git clone https://github.com/RayleaBot/RayleaBot.git
cd RayleaBot

# Windows
start.bat

# 跨平台
node scripts/start-dev.mjs
```

开发模式下，服务端监听 `http://127.0.0.1:8080`，Web 开发服务器运行在 `http://127.0.0.1:4173`。

## 使用简介

- 管理面板默认只在本机开放，远程访问需在配置中显式开启，并建议通过 HTTPS 反向代理。
- 通过 OneBot11 协议适配器接入 QQ 后，即可在聊天窗口与机器人交互。
- 插件安装在运行根目录的 `plugins/installed/`，支持从本地目录或压缩包安装。
- 管理员可在管理面板中配置权限策略、黑白名单、命令前缀、任务调度等。

## 文档

| 文档 | 说明 |
|---|---|
| [项目规划](./docs/RayleaBot机器人项目规划.md) | 产品目标、架构与路线图 |
| [架构总览](./docs/architecture/README.md) | 内部设计、事件模型、状态模型 |
| [插件开发](./docs/plugin/README.md) | 生命周期、manifest、协议、SDK |
| [用户指南](./docs/user/README.md) | 部署、配置、CLI、恢复 |
| [工程基线](./docs/engineering/baseline.md) | 版本线、选型、目录职责 |
| [CHANGELOGS](./docs/CHANGELOGS/) | 版本变更记录 |

## 贡献与开发

```bash
# Server
cd server && go test ./...

# Web
cd web && pnpm install --frozen-lockfile && pnpm test

# Launcher
cd launcher && pnpm install --frozen-lockfile && pnpm test

# Node.js SDK
cd sdk/nodejs && node --test tests/*.test.mjs

# Python SDK
cd sdk/python && python -m unittest discover -s tests
```

项目采用契约优先（contract-first）模式。修改任何对外接口前，请先更新 `contracts/` 中的对应契约文件，再同步实现与测试。

## License

[AGPL-3.0](LICENSE)

## 仓库动态

> 以下图表由 [`.github/workflows/repo-stats.yml`](.github/workflows/repo-stats.yml) 每日自动生成，反映本仓库最近一年的提交活动。

![月度提交折线图](https://raw.githubusercontent.com/RayleaBot/RayleaBot/output/repo-activity-line.svg)

![周提交热力图](https://raw.githubusercontent.com/RayleaBot/RayleaBot/output/repo-activity-heatmap.svg)
