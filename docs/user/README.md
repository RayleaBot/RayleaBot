# User Docs

本目录用于承载 RayleaBot 的安装、初始化、配置、管理、恢复与排障说明。

## 当前用户可用形态

当前仓库里可直接使用的产品表面集中在 server 主链路：

- `config/user.yaml` 与 `contracts/config.user.schema.json` 驱动的配置与启动流程
- `POST /api/setup/admin`、session 登录、配置读取与更新、任务查询、日志查询、插件安装与生命周期管理
- `/ws/events`、`/ws/tasks`、`/ws/logs`、`/ws/plugins/{id}/console` 管理 WebSocket
- `reset-admin`、`backup`、`restore`、`doctor`、`migrate`、`cleanup` 六个 CLI 子命令
- `web/` 管理面已提供 `setup/login/session`、系统状态、渲染预览、插件安装与卸载、grants 管理、插件 console、任务查看与取消、日志查看、配置编辑与服务关闭入口
- 正式 artifact matrix 当前覆盖 `windows-x64-full`、`linux-x64-full`、`macos-arm64-full` 与 `linux-x64-server`
- full artifact 的桌面入口位于发行包根目录：`RayleaLauncher.exe`、`RayleaLauncher` 或 `RayleaLauncher.app`

## 正式安装与启动

- full artifact 的正式启动入口是发行包根目录的 Launcher；`linux-x64-server` 的正式启动入口是根目录 `raylea-server`。
- 单个发行包根目录同时也是默认运行根目录。`config/`、`data/`、`cache/`、`logs/` 与 `plugins/installed/` 都以该目录为准。
- Launcher 的正式配置模型以安装目录为主，服务端路径、配置文件路径与运行目录默认从安装目录派生；高级覆盖只用于排障或特殊复用场景。
- 首次启动时，如 `config/user.yaml` 缺失，服务会基于 `config/default.yaml` 生成首份用户配置。
- 正式包携带 `.deps/manifest.json`。服务启动后会自动准备 Python、Node.js 与 npm 环境；Chromium 浏览环境、Python 与 Node.js 资源按 `.deps/manifest.json` 中的有序来源列表下载到 `cache/downloads/runtime/`，并展开到 `.deps/store/<resource-id>/<version>/`。
- 运行环境与模板资源的有效根目录按 `config/user.yaml` 的上两级目录推导；Launcher 的运行目录覆盖只影响进程工作目录、日志目录与本地运行数据，不改变 `.deps/` 与 `templates/` 的位置。
- 复用已有工作区时，Launcher 直接对准现有 RayleaBot 根目录，并继续使用其中已有的 `config/`、`data/`、`cache/`、`logs/` 与 `plugins/installed/`。
- 升级后启动时，用户目录继续沿用原根目录；正式支持路径是不覆盖 `config/`、`data/` 与 `plugins/installed/`，并在原根目录内继续启动。

Render Service 已提供管理面预览调试入口、任务详情图片预览与同源 artifact 读取面；在线模板编辑与更宽用户侧工作流不在当前说明范围内。

## 当前恢复边界

- 受支持的恢复路径为：升级前导出备份，停服务后执行 `restore`，再重新启动服务并进入迁移与兼容检查。
- 升级默认不覆盖 `config/`、`data/` 与 `plugins/installed/`。
- 回退旧版本时，正式支持路径是使用升级前备份恢复；直接用旧版本二进制读取较新的状态库不在支持范围内。
- `restore` 会先读取备份中的 `backup-manifest.json`，在真正解包前校验程序版本、配置 schema 与数据库 schema 兼容性。
- 恢复预检与启动后的兼容检查共享 `logs/recovery-summary.json`。CLI `restore`、CLI `doctor`、Web 管理面、Launcher 与 diagnostics 导出都读取同一份恢复摘要。
- 恢复摘要会标明本次操作属于 `restore`、`upgrade` 或 `rollback`，并列出需要人工处理的资源问题、被跳过的插件和下一步建议。
- 启动后的兼容检查会覆盖运行时资源、模板资源、Chromium 浏览环境与已恢复插件。兼容性不满足的插件会保留安装目录和数据，但默认保持禁用，等待人工处理后再手动启用。
- `post_startup` 恢复摘要在 `degraded` 状态下会稳定保留 `manual_actions`、`next_steps` 与跳过插件列表；在 `compatible` 状态下不保留人工处理建议。
- diagnostics 导出中的 `recovery-summary.json`、Web 管理面与 Launcher 状态页会投影同一份人工处理建议和下一步列表。
- Web 管理面与 Launcher 状态页提供“重新检查恢复状态”入口；完成运行时准备、插件替换、插件卸载或其他人工处理后，可直接触发 `recovery.recheck` 任务并等待恢复摘要收敛到 `compatible`。
- 运行时资源类问题可通过“准备运行环境”入口触发 `runtime.bootstrap` 任务；任务结果会返回每类资源的缓存归档与展开目录明细，并在 Chromium 准备完成后立即刷新 render preview 可用性。
- 离线或受限网络环境下，可把已校验归档预置到 `cache/downloads/runtime/`，或把资源预展开到 `.deps/store/<resource-id>/<version>/`。内网镜像与公开来源共用同一份 `.deps/manifest.json` 资源定义。Chromium 仍支持通过 `render.browser_path` 显式覆盖浏览器路径。

## 当前长期自托管巡检边界

- 正式发行包支持面向包内 `raylea-server` 的长期自托管巡检，覆盖启动、初始化、管理登录、诊断导出、在线备份、优雅停机与重启后再探活。
- 长期巡检以发行包根目录为工作根目录，直接复用其中的 `config/`、`data/`、`cache/`、`logs/` 与 `plugins/installed/`。
- 长期巡检与恢复 drill 都会在启动前按 `.deps/manifest.json` 准备 Chromium、Python、Node.js 与 npm 环境，并在巡检过程中校验 `runtime.bootstrap` 与 `recovery.recheck` 的正式任务路径。
- full artifact 的长期巡检目标是服务可长期管理，不包含 Launcher 图形界面自动化。
- 巡检过程中若存在 `recovery_summary`，可接受的非阻断状态与 CLI、Web、Launcher 保持同一口径，不单独定义额外状态名。

## 编写边界

- 用户配置正式结构以 `contracts/config.user.schema.json` 为准。
- 用户侧说明需要与 `server/README.md`、contracts、baseline 和当前发布形态保持一致。
- 若某项能力仅停留在 contract、fixture 或工程骨架，本目录只描述边界和前置条件。
