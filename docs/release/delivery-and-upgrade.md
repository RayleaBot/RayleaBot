# Delivery and Upgrade

本页说明 RayleaBot 的正式发行物、版本检查和手动更新方式。字段与限制以 [`contracts/release-manifest.schema.json`](../../contracts/release-manifest.schema.json) 为准。

## 正式产物矩阵

| `artifact_id` | 产物 | 支持级别 | 更新方式 |
| --- | --- | --- | --- |
| `windows-x64-full` | Windows 桌面完整包 | `first_class` | `guided` |
| `linux-x64-full` | Linux 桌面完整包 | `first_class` | `guided` |
| `macos-arm64-full` | macOS Apple Silicon 桌面完整包 | `first_class` | `guided` |
| `linux-x64-server` | Linux 服务端包 | `first_class` | `guided` 或 `manual` |

## 发布包目录

发行包根目录按产物形态包含：

- server 二进制与 Launcher 桌面入口；
- `web/dist` 与核心 `templates/`；
- `.deps/manifest.json`（用户配置由服务端内嵌默认值初始化）；
- `build_info.json`；
- 根仓库 `LICENSE` 与生成、审阅后的 `THIRD_PARTY_NOTICES.md`。

Windows 完整包以根目录的 Wails 程序 `RayleaLauncher.exe` 作为唯一桌面入口，不附带嵌套桌面运行时目录。Launcher 依赖系统安装的 Microsoft Edge WebView2 Runtime；包内 `WINDOWS-RUNTIME.md` 与 [Windows Desktop Runtime](./windows-desktop-runtime.md) 说明联网和离线安装方式。

Linux 完整包使用根目录的 `RayleaLauncher`，macOS 完整包使用 `RayleaLauncher.app`。

Linux 完整包还包含 `LINUX-RUNTIME.md`。Launcher 依赖系统提供的 GTK 3 和 WebKit2GTK 4.1 动态库，压缩包不内嵌这些发行版组件；安装要求见 [Linux Desktop Runtime](./linux-desktop-runtime.md)。

主程序 release workflow 不 checkout、不构建也不打包业务插件。正式归档中不得出现 `plugins/` 业务产物、插件 `.go`、`.py`、`.ts`、`.vue`、测试、源码 SDK、`node_modules` 或语言运行时；`.deps/manifest.json` v5 声明 Chromium 与 FFmpeg 资源。每个平台资源必须提供按顺序选择的 `sources`（`upstream` / `mirror`，可按测速结果选源）、归档格式和 SHA-256；Chromium 提供 `entrypoints.browser`，FFmpeg 资源同时提供 `entrypoints.ffmpeg` 与 `entrypoints.ffprobe`。运行环境准备完成后，核心从这些相对入口定位可执行文件。

官方和社区插件都由各自仓库构建一个或多个平台的单根目录 ZIP，再通过 HTTPS 插件目录或本地 artifact 走统一安装流程。框架不会编译源码、安装语言依赖或执行安装脚本。release recovery drill 使用外部预编译 Go 测试 fixture 验证插件备份与恢复，该 fixture 只参与测试，不进入应用归档。

依赖许可证未知或缺失时，发布必须失败。运行时配置和插件 schema 内置于 server；源码仓库中的 `contracts/` 仍是正式来源。

## 发布元数据

每次正式 Release 发布各平台 artifact 和 `release_manifest.v2.json`，随包附带 `build_info.json`。字段以发布契约为准：

- 发布清单记录版本、提交、构建与发布时间、channel、配置/数据库/插件格式版本、产物列表和发布页地址。
- 每个产物记录平台、文件名、下载地址、大小、文件数、支持级别、smoke profile 和 `guided` / `manual` 更新方式。
- `build_info.json` 记录当前安装版本、提交、产物标识、构建时间和插件格式版本。

发布脚本严格按契约生成并校验清单。Server 读取时忽略未知字段，只使用版本、发布页地址和当前产物的文件名、下载地址、大小与更新方式；插件格式版本仅供展示，兼容性写在发布说明中。

## 更新检查

Launcher 每 6 小时检查发布版本，发现新版本后提供发布页入口。Web 使用 `GET /api/update/status` 查看状态，使用 `POST /api/update/check` 主动检查。CLI 提供：

```text
raylea-server version --json
raylea-server update check --json
```

版本检查通过 HTTPS 读取发布清单，比较当前版本并返回发布页地址。当前安装缺少有效 `build_info.json` 时，Launcher 提供项目发布页入口。

## 手动更新

1. 从发布页下载对应平台的包。
2. 停止服务，并使用 `backup` 保存配置、数据库和插件数据。
3. 解压新版到独立目录，按[恢复说明](../user/recovery.md)将备份恢复到新目录。
4. 运行 doctor 并启动服务，检查健康状态、配置和插件。

保留旧安装目录和备份，直到确认新版运行正常。备份与恢复支持的格式和操作步骤见[恢复说明](../user/recovery.md)。

数据库 `000001` 备份恢复后，首次启动事务迁移到 `000002`。就地替换 server 二进制时同样由启动流程执行迁移。回退旧核心应使用旧目录或旧格式备份，不把已迁移的数据库交给旧核心。

GitHub 自动生成的源代码压缩包不是正式运行时产物。

## 相关文档

- [Acceptance and Risks](./acceptance-and-risks.md)
- [Plugin Store and Independent Development](../plugin/store-and-development.md)
- [Deployment](../user/deployment.md)
- [Recovery](../user/recovery.md)
