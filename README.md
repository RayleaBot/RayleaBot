<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="web/public/favicon-dark.svg">
  <img src="web/public/favicon.svg" width="140" alt="RayleaBot">
</picture>

# RayleaBot

自托管的 QQ 机器人框架，连接、插件与权限都在浏览器里管理

[![Release](https://img.shields.io/github/v/release/RayleaBot/RayleaBot?include_prereleases&label=release)](https://github.com/RayleaBot/RayleaBot/releases)
[![Nightly](https://img.shields.io/github/actions/workflow/status/RayleaBot/RayleaBot/nightly.yml?branch=main&label=nightly)](https://github.com/RayleaBot/RayleaBot/actions/workflows/nightly.yml)
[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
<br>
[![OneBot v11](https://img.shields.io/badge/OneBot-v11-black)](https://github.com/botuniverse/onebot-11)
[![QQ 官方机器人](https://img.shields.io/badge/QQ-%E5%AE%98%E6%96%B9%E6%9C%BA%E5%99%A8%E4%BA%BA-12B7F5?logo=tencentqq&logoColor=white)](https://bot.q.qq.com/wiki/)
[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Node.js](https://img.shields.io/badge/Node.js-26+-339933?logo=nodedotjs&logoColor=white)](https://nodejs.org)
[![Vue](https://img.shields.io/badge/Vue-3.5-42b883?logo=vuedotjs&logoColor=white)](https://vuejs.org)

[快速开始](#快速开始) · [部署指南](./docs/user/deployment.md) · [插件开发](./docs/plugin/README.md) · [更新记录](./docs/CHANGELOGS/) · [问题反馈](https://github.com/RayleaBot/RayleaBot/issues)

</div>

## 简介

RayleaBot 通过 NapCat 等 OneBot V11 协议端或 QQ 官方机器人接入 QQ，运行在你自己的电脑或服务器上，配置、数据和账号凭据都保存在本机。日常操作在 Web 管理面中完成；桌面版附带启动器，负责启停服务、检查运行环境和一键更新。

插件是独立运行的原生程序，可以用任何语言编写，从插件商店安装和更新。

## 功能特性

- **多账号接入**：同时连接多个 OneBot V11 协议端和 QQ 官方机器人，每个连接的身份、消息发送和黑白名单分开管理，增删连接无需重启。
- **Web 管理面**：查看运行状态和各连接的收发消息趋势，管理插件、指令、权限、限流与定时任务，按级别、模块、插件或请求筛选日志。
- **插件商店**：从官方插件源或自定义插件源安装、更新插件；依赖其他插件的插件，安装时会列出需要先装的插件。
- **独立进程插件**：插件崩溃不会拖垮主程序；提供 Go SDK、管理页 Vue SDK 和打包工具，其他语言按插件协议实现即可。
- **图片渲染**：内置模板渲染，插件可以把菜单和查询结果渲染成图片发送。
- **桌面启动器**：支持 Windows、Linux 和 macOS，常驻系统托盘，发现新版本后一键更新。

## 接入方式

| 接入方式 | 连接方式 | 说明 |
| --- | --- | --- |
| [OneBot V11](https://github.com/botuniverse/onebot-11) | 反向 WebSocket、正向 WebSocket、HTTP API、Webhook | QQ 个人号，需要 NapCat、LuckyLilliaBot 等协议端登录 QQ；兼容范围见 [OneBot11 兼容矩阵](./docs/dev/onebot-compatibility.md) |
| [QQ 官方机器人](https://bot.q.qq.com/wiki/) | 官方 WebSocket 网关 | 在 QQ 开放平台创建的机器人，支持群聊与单聊 |

RayleaBot 本身不登录 QQ：个人号由协议端负责登录，官方机器人在 [QQ 开放平台](https://q.qq.com)创建。

## 快速开始

1. 从 [Releases](https://github.com/RayleaBot/RayleaBot/releases) 下载对应平台的完整包；没有桌面环境的 Linux 服务器下载服务端包。各平台的文件名和运行要求见[部署指南](./docs/user/deployment.md)。
2. 解压到一个固定目录，配置和数据都会保存在这里。运行其中的 `RayleaLauncher`，服务端包运行 `raylea-server`。
3. 创建管理员账号。桌面版在启动器中启动服务后点击“打开管理界面”；服务端包用控制台打印的一次性“首次设置地址”打开。管理面地址为 `http://127.0.0.1:8080`，默认只允许本机访问，局域网访问见[部署指南](./docs/user/deployment.md#本机与局域网访问)。
4. 在“协议中心”添加连接。OneBot V11 把页面显示的回连地址和访问令牌填进协议端；QQ 官方机器人填写 AppID 与 AppSecret。
5. 在“插件商店”安装插件，然后向机器人发送 `/帮助` 查看可用指令。

桌面版在启动器的“关于应用”中一键更新；服务端包依次执行 `raylea-server update download`、停止服务、`raylea-server update apply`，详见[交付与更新](./docs/release/delivery-and-upgrade.md)。

### 从源码运行

所需工具及版本见 `.tool-versions`，可以用 `make doctor`（没有 make 时运行 `go run ./tools/cmd/check-toolchain`）检查本机环境。

```bash
git clone https://github.com/RayleaBot/RayleaBot.git
cd RayleaBot
node scripts/start-dev.mjs   # Windows 也可运行 start.bat，Linux / macOS 也可运行 sh start.sh
```

服务端监听 `http://127.0.0.1:8080`，Web 开发服务器监听 `http://127.0.0.1:4173`。主仓库不带插件，联调独立插件见[本地同步开发](./docs/plugin/store-and-development.md#本地同步开发)。

## 文档

| 文档 | 内容 |
| --- | --- |
| [用户指南](./docs/user/README.md) | 部署、配置、管理面、命令行与备份恢复 |
| [发布说明](./docs/release/README.md) | 发布包、更新方式与各版本说明 |
| [插件开发](./docs/plugin/README.md) | 插件清单、通信协议、SDK 与生命周期 |
| [插件商店与独立开发](./docs/plugin/store-and-development.md) | 插件源、发布插件与本地联调 |
| [架构总览](./docs/architecture/README.md) | 组件职责、消息处理流程与状态归属 |
| [项目规划](./docs/RayleaBot机器人项目规划.md) | 产品目标、范围与工程原则 |

## 参与贡献

欢迎提交 Issue 和 Pull Request，较大的改动请先开 Issue 讨论方案。

仓库由服务端 `server/`、管理面 `web/`、桌面启动器 `launcher/`、插件 SDK `sdk/` 和接口契约 `contracts/` 组成。修改 HTTP、WebSocket 或插件协议等对外接口时，先更新 `contracts/` 中的契约再改实现。提交信息使用中文的 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)。构建、测试与检查命令见[工程基线](./docs/engineering/baseline.md)和[质量门禁](./docs/engineering/quality-gates.md)。

## 致谢

- [OneBot](https://github.com/botuniverse/onebot-11) 与 [NapCat](https://github.com/NapNeko/NapCatQQ)：QQ 个人号接入
- [Wails](https://github.com/wailsapp/wails)：桌面启动器
- [Vue](https://github.com/vuejs/core) 与 [Reka UI](https://github.com/unovue/reka-ui)：管理面与启动器界面
- [chromedp](https://github.com/chromedp/chromedp)：图片渲染
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite)：数据存储

全部第三方组件及许可证见 [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md)。

## 许可证

RayleaBot 使用 [AGPL-3.0](LICENSE) 许可证。插件 SDK（`sdk/go` 与 `sdk/vue`）使用 [MIT](sdk/go/LICENSE) 许可证，插件可以自行选择许可证。

## 仓库动态

<a href="https://star-history.com/#RayleaBot/RayleaBot&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=RayleaBot/RayleaBot&type=Date&theme=dark">
    <img alt="Star History" src="https://api.star-history.com/svg?repos=RayleaBot/RayleaBot&type=Date">
  </picture>
</a>

最近一年的提交情况，由 [repo-stats](.github/workflows/repo-stats.yml) 工作流在推送到 `main` 时生成：

![月度提交折线图](https://raw.githubusercontent.com/RayleaBot/RayleaBot/output/repo-activity-line.svg)

![周提交热力图](https://raw.githubusercontent.com/RayleaBot/RayleaBot/output/repo-activity-heatmap.svg)
