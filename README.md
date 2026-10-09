# RayleaBot

在自己的电脑或服务器上运行的 QQ 机器人。

[![Release](https://img.shields.io/github/v/release/RayleaBot/RayleaBot?include_prereleases)](https://github.com/RayleaBot/RayleaBot/releases)
[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)

[下载](https://github.com/RayleaBot/RayleaBot/releases) · [部署指南](./docs/user/deployment.md) · [管理面说明](./docs/user/management-surface.md) · [插件开发](./docs/plugin/README.md) · [更新记录](./docs/CHANGELOGS/)

RayleaBot 通过 NapCat 等 OneBot11 协议端或 QQ 官方机器人接入 QQ。连接机器人、安装插件、设置权限和查看日志都在浏览器里完成，配置、运行数据和账号凭据都保存在运行它的机器上。

Windows、Linux 和 macOS 有带桌面启动器的完整包，没有桌面环境的 Linux 服务器使用服务端包。

## 功能

- 同时接入多个 QQ 账号。OneBot11 支持反向 WebSocket、正向 WebSocket、HTTP API 和 Webhook，QQ 官方机器人可以单独使用；每个连接的身份、发送和黑白名单分开管理。
- 从官方插件源或自己添加的插件源安装、更新插件。需要搭配其他插件使用的插件，安装时会列出要先装的插件。
- 在管理面查看服务状态和各连接的收发消息趋势，管理插件与指令、权限、黑白名单、限流和定时任务，检索日志，预览出图模板。
- 桌面启动器负责启动和停止服务、检查运行环境，并在发现新版本后一键更新。
- 插件以独立进程运行，可以用任何语言编写；Go 插件有 SDK 和打包工具，插件管理页可以使用 Vue SDK。

## 官方插件

官方插件源目前收录以下插件，在管理面的“插件商店”中安装：

| 插件 | 用途 |
| --- | --- |
| 运势 | 每日运势抽取与统计 |
| 游戏攻略 | 查询《崩坏：星穹铁道》角色攻略图 |
| 订阅与解析 | 订阅平台内容，解析 B 站、微博与抖音链接 |
| 三角洲助手 | 《三角洲行动》摸容器模拟与每日密码查询 |
| 油价查询 | 查询各省市油价和附近的大型品牌加油站 |

## 快速开始

### 1. 下载

从 [Releases](https://github.com/RayleaBot/RayleaBot/releases) 下载对应平台的包：

| 平台 | 文件 | 启动入口 |
| --- | --- | --- |
| Windows x64 | `RayleaBot-v<版本>-windows-x64-full.zip` | `RayleaLauncher.exe` |
| Linux x64 桌面 | `RayleaBot-v<版本>-linux-x64-full.tar.gz` | `RayleaLauncher` |
| macOS Apple Silicon（实验性） | `RayleaBot-v<版本>-macos-arm64-full.tar.gz` | `RayleaLauncher.app` |
| Linux x64 服务器 | `RayleaBot-v<版本>-linux-x64-server.tar.gz` | `raylea-server` |

Windows 需要 Microsoft Edge WebView2 Runtime，Linux 桌面需要 GTK 3 和 WebKit2GTK 4.1，安装方法见包内的 `WINDOWS-RUNTIME.md` 和 `LINUX-RUNTIME.md`。macOS 包没有 Apple 签名，系统阻止打开时按[部署指南](./docs/user/deployment.md#首次安装)放行。

### 2. 启动并创建管理员

把包解压到一个固定目录，配置和数据都保存在这里。运行启动入口：

- 桌面版：在启动器中启动服务，点击“打开管理界面”，按提示创建管理员账号。
- 服务端包：运行后控制台会打印一次性的“首次设置地址”，用浏览器打开它创建管理员账号。包内的 `systemd/rayleabot.service` 可用于托管服务。

管理界面的地址是 `http://127.0.0.1:8080`，默认只有本机能打开。需要从局域网其他设备访问时，见[本机与局域网访问](./docs/user/deployment.md#本机与局域网访问)。

### 3. 连接 QQ

在管理面的“协议中心”添加连接：

- OneBot11：先运行 NapCat 等协议端并登录 QQ。连接方式选反向 WebSocket 时，把 RayleaBot 显示的回连地址和访问令牌填进协议端。
- QQ 官方机器人：填写 [QQ 开放平台](https://q.qq.com)中机器人的 AppID 和 AppSecret。

### 4. 安装插件

在“插件商店”安装插件，然后在 QQ 里给机器人发送 `/帮助`，查看可用指令。

## 更新

桌面版在启动器的“关于应用”中检查并一键更新。服务端包先执行 `raylea-server update download` 下载新版本，停止服务后执行 `raylea-server update apply`。更新方式的细节见[交付与更新](./docs/release/delivery-and-upgrade.md)。

## 文档

| 文档 | 内容 |
| --- | --- |
| [用户指南](./docs/user/README.md) | 部署、配置、管理面、命令行与备份恢复 |
| [发布说明](./docs/release/README.md) | 各版本说明与发布包 |
| [插件开发](./docs/plugin/README.md) | 插件清单、协议、SDK 与生命周期 |
| [插件商店与独立开发](./docs/plugin/store-and-development.md) | 插件源、发布插件与本地联调 |
| [架构总览](./docs/architecture/README.md) | 各组件的职责与消息处理流程 |
| [项目规划](./docs/RayleaBot机器人项目规划.md) | 产品目标与范围 |

## 参与开发

仓库包含服务端 `server/`、管理面 `web/`、桌面启动器 `launcher/`、插件 SDK `sdk/` 和对外接口契约 `contracts/`。修改对外接口时先改 `contracts/`，再改实现。

工具链版本固定在 `.tool-versions`，用 `make doctor`（没有 make 时运行 `go run ./tools/cmd/check-toolchain`）检查本机环境。从源码启动：

```bash
git clone https://github.com/RayleaBot/RayleaBot.git
cd RayleaBot
node scripts/start-dev.mjs
```

Windows 也可以运行 `start.bat`，Linux 和 macOS 可以运行 `sh start.sh`。服务端监听 `http://127.0.0.1:8080`，Web 开发服务器在 `http://127.0.0.1:4173`。

主仓库不带插件。联调独立插件仓库时，复制 `plugin-workspace.example.json` 为 `plugin-workspace.local.json` 并填写插件路径，做法见[本地同步开发](./docs/plugin/store-and-development.md#本地同步开发)。其余开发说明见[开发者文档](./docs/dev/README.md)、[工程基线](./docs/engineering/baseline.md)和[质量门禁](./docs/engineering/quality-gates.md)。

## 许可证

RayleaBot 使用 [AGPL-3.0](LICENSE)。插件 SDK（`sdk/go` 与 `sdk/vue`）使用 [MIT](sdk/go/LICENSE)，插件可以自行选择许可证。

## 仓库动态

最近一年的提交情况，推送到 `main` 时由 [repo-stats](.github/workflows/repo-stats.yml) 工作流生成。

![月度提交折线图](https://raw.githubusercontent.com/RayleaBot/RayleaBot/output/repo-activity-line.svg)

![周提交热力图](https://raw.githubusercontent.com/RayleaBot/RayleaBot/output/repo-activity-heatmap.svg)
