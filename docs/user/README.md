# User Docs

本目录用于承载 RayleaBot 的安装、初始化、配置、管理、恢复与排障说明。

## 当前用户可用形态

当前仓库里可直接使用的产品表面集中在 server 主链路：

- `config/user.yaml` 与 `contracts/config.user.schema.json` 驱动的配置与启动流程
- `POST /api/setup/admin`、session 登录、配置读取与更新、任务查询、日志查询、插件安装与生命周期管理
- `/ws/events`、`/ws/tasks`、`/ws/logs`、`/ws/plugins/{id}/console` 管理 WebSocket
- `reset-admin`、`backup`、`restore`、`doctor`、`migrate`、`cleanup` 六个 CLI 子命令
- `web/` 管理面已提供 `setup/login/session`、系统状态、插件安装与卸载、grants 管理、插件 console、任务查看与取消、日志查看、配置编辑与服务关闭入口
- `launcher/` 已提供最小 Windows 桌面壳，可执行环境检查、拉起 / 停止 `raylea-server`、轮询健康与系统状态、打开 Web 管理面并通过 loopback token 自动入会话

Render Service 仍未形成面向用户的完整体验；当前用户侧说明可围绕 server 管理面、CLI、Web 管理面和最小 Launcher 闭环展开。

## 编写边界

- 用户配置正式结构以 `contracts/config.user.schema.json` 为准。
- 用户侧说明需要与 `server/README.md`、contracts、baseline 和当前发布形态保持一致。
- 若某项能力仅停留在 contract、fixture 或工程骨架，本目录只描述边界和前置条件。
