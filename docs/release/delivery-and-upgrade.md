# Delivery and Upgrade

本页说明 RayleaBot 的正式发行物、版本检查、一键更新与手动更新方式，以及更新策略的取舍理由。字段与限制以 [`contracts/release-manifest.schema.json`](../../contracts/release-manifest.schema.json) 为准。

## 正式产物矩阵

| `artifact_id` | 产物 | 支持级别 | 更新方式 |
| --- | --- | --- | --- |
| `windows-x64-full` | Windows 桌面完整包 | `first_class` | Launcher 一键更新 |
| `linux-x64-full` | Linux 桌面完整包 | `first_class` | Launcher 一键更新 |
| `macos-arm64-full` | macOS Apple Silicon 桌面完整包 | `first_class` | Launcher 一键更新 |
| `linux-x64-server` | Linux 服务端包 | `first_class` | `raylea-server update apply` |

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
- 每个产物记录平台、文件名、下载地址、大小、文件数、支持级别、smoke profile 和仅供展示的 `update_mode`。
- `build_info.json` 记录当前安装版本、提交、产物标识、构建时间和插件格式版本。

发布脚本严格按契约生成并校验清单。Server 读取时忽略未知字段，只使用版本、发布页地址和当前产物的文件名、下载地址、大小与更新方式；插件格式版本仅供展示，兼容性写在发布说明中。

## 更新检查

Launcher 每 6 小时检查发布版本，发现新版本后提供立即更新和发布页入口。Web 使用 `GET /api/update/status` 查看状态，使用 `POST /api/update/check` 主动检查。CLI 提供：

```text
raylea-server version --json
raylea-server update check --json
raylea-server update download
raylea-server update apply
```

版本检查通过 HTTPS 读取发布清单，比较当前版本并返回发布页地址。当前安装缺少有效 `build_info.json` 时，Launcher 提供项目发布页入口。

## 一键更新

桌面完整包在 Launcher 的“关于应用”中确认后更新：

1. Launcher 调用 `raylea-server update download` 下载当前产物的更新包，服务保持运行。
2. Launcher 停止服务，调用 `raylea-server update apply` 逐个替换安装根中的程序文件：已有文件先移入 `cache/update/replaced/`，`build_info.json` 最后写入。
3. Launcher 启动新版 Launcher 后退出；新版等待旧进程退出再接管单实例。

服务端包停止服务后在安装根执行同一命令：

```text
sudo systemctl stop rayleabot
./raylea-server update apply
sudo systemctl start rayleabot
```

更新只写入发布包包含的文件，不触碰 `config/`、`data/`、`plugins/`、`logs/`、`backups/` 与 `.deps/store/`；新版本不再包含的旧文件保留。数据库在更新后的首次启动时前向迁移。

## 手动更新

1. 从发布页下载对应平台的包。
2. 停止服务。建议先执行 `backup` 保存配置、数据库和插件数据。
3. 把发布包根目录中的内容解压覆盖到安装根。
4. 启动服务，检查健康状态、配置和插件。

数据库 `000001` 在首次启动时事务迁移到 `000002`；从 `000001` 备份恢复的安装同样在首次启动时迁移。

GitHub 自动生成的源代码压缩包不是正式运行时产物。

## 更新策略与理由

以下取舍是有意的设计，不是待补的缺口。改变做法前，先推翻这里的理由并同步本节。

| 不做 | 理由 |
| --- | --- |
| 核心更新包的 SHA-256 摘要 | 发布清单与更新包都经 HTTPS 从同一个 GitHub Release 获取；传输损坏由 HTTPS、清单中的归档大小以及 ZIP CRC32 或 gzip 校验发现。摘要与文件出自同一来源，能替换文件的人也能替换摘要，挡不住篡改。 |
| 发布签名 | 签名是唯一能防止发布源被篡改的手段，但需要长期的密钥管理与轮换；签名体系已被有意删除，不再恢复。 |
| 更新回滚、迁移前数据库副本 | `build_info.json` 最后写入，替换中断时安装仍报告旧版本，重新执行 `update apply` 或手动解压覆盖即可完成。数据库迁移在事务内执行，失败时保持迁移前状态。退回旧版本使用旧发布包与更新前的备份。 |
| 独立更新程序、系统原生脚本 | Windows 允许给正在运行的程序改名，Linux 与 macOS 可以直接替换运行中的文件，`update apply` 以先改名再移入完成替换，Launcher 自行重启即可。脚本需要维护三个平台，难以测试，出错时也无法在界面报告。 |

同类项目同样没有这些机制：Caddy 的 upgrade 命令只依赖 HTTPS，AstrBot 检查 ZIP 结构后原地覆盖，Yunzai 通过 git pull 更新，Koishi 由 npm 更新依赖。

发布清单不是安全边界：读取端宽松，发布端严格。Server 读取清单时忽略未知字段，只校验实际使用的字段，新版本增加字段或调整插件合同不会让旧版本检测不到更新；发布脚本按契约严格生成并校验清单。

摘要只在清单与文件来源不同时才有意义。运行环境依赖清单中的 Chromium 与 FFmpeg 可能来自镜像，插件商店目录与归档分属不同仓库，因此两者继续校验 SHA-256。若将来为核心更新接入镜像或加速地址，再在官方清单中增加摘要。

## 相关文档

- [Acceptance and Risks](./acceptance-and-risks.md)
- [Plugin Store and Independent Development](../plugin/store-and-development.md)
- [Deployment](../user/deployment.md)
- [Recovery](../user/recovery.md)
