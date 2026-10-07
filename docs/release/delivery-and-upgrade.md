# Delivery and Upgrade

本页说明 RayleaBot 的正式发行物、版本检查、一键更新与手动更新方式，以及更新策略的取舍理由。字段与限制以 [`contracts/release-manifest.schema.json`](../../contracts/release-manifest.schema.json) 为准。

## 正式产物矩阵

| `artifact_id` | 产物 | 支持级别 | 更新方式 |
| --- | --- | --- | --- |
| `windows-x64-full` | Windows 桌面完整包 | `first_class` | Launcher 一键更新 |
| `linux-x64-full` | Linux 桌面完整包 | `first_class` | Launcher 一键更新 |
| `macos-arm64-full` | macOS Apple Silicon 桌面完整包 | `experimental` | Launcher 一键更新 |
| `linux-x64-server` | Linux 服务端包 | `first_class` | `raylea-server update download` 后停服执行 `update apply` |

## 发布包目录

发行包根目录按产物形态包含：

- server 二进制与 Launcher 桌面入口；
- `web/dist` 与核心 `templates/`；
- `.deps/manifest.json`（用户配置由服务端内嵌默认值初始化）；
- `build_info.json`；
- 根仓库 `LICENSE` 与生成、审阅后的 `THIRD_PARTY_NOTICES.md`。

Windows 完整包以根目录的 Wails 程序 `RayleaLauncher.exe` 作为唯一桌面入口，不附带嵌套桌面运行时目录。Launcher 依赖系统安装的 Microsoft Edge WebView2 Runtime；包内 `WINDOWS-RUNTIME.md` 与 [Windows Desktop Runtime](./windows-desktop-runtime.md) 说明联网和离线安装方式。

Linux 完整包使用根目录的 `RayleaLauncher`，macOS 完整包使用 `RayleaLauncher.app`。

两种 Linux 包都包含 `LINUX-RUNTIME.md`，说明 Chromium 共享库和字体要求。Launcher 还依赖 GTK 3 和 WebKit2GTK 4.1 动态库，压缩包不内嵌这些发行版组件；安装要求见 [Linux Runtime](./linux-desktop-runtime.md)。

主程序 release workflow 不 checkout、不构建也不打包业务插件。正式归档中不得出现 `plugins/` 业务产物、插件 `.go`、`.py`、`.ts`、`.vue`、测试、源码 SDK、`node_modules` 或语言运行时；`.deps/manifest.json` v5 声明 Chromium 与 FFmpeg 资源。每个平台资源必须提供按顺序选择的 `sources`（`upstream` / `mirror`，可按测速结果选源）、归档格式和 SHA-256；Chromium 提供 `entrypoints.browser`，FFmpeg 资源同时提供 `entrypoints.ffmpeg` 与 `entrypoints.ffprobe`。FFmpeg 可另附 `ffprobe_archive`，独立声明来源、归档格式与摘要；其内容解压到资源根的 `ffprobe/`，所有入口验证通过后才启用整个资源目录。运行环境准备完成后，核心从这些相对入口定位可执行文件。

官方和社区插件都由各自仓库构建一个或多个平台的单根目录 ZIP，再通过 HTTPS 插件目录或本地 artifact 走统一安装流程。框架不会编译源码、安装语言依赖或执行安装脚本。恢复流程由 nightly 的本版恢复演练在构建后的 Server 上验证，演练数据不进入应用归档。

依赖许可证未知或缺失时，发布必须失败。运行时配置和插件 schema 内置于 server；源码仓库中的 `contracts/` 仍是正式来源。

## 发布元数据

每次正式 Release 发布各平台 artifact 和 `release_manifest.v2.json`，随包附带 `build_info.json`。字段以发布契约为准：

- 发布清单记录版本、提交、构建与发布时间、channel、配置/数据库/插件格式版本、产物列表和发布页地址。
- 每个产物记录平台、文件名、下载地址、大小、文件数、支持级别、smoke profile 和仅供展示的 `update_mode`。
- `build_info.json` 记录当前安装版本、提交、产物标识、构建时间和插件格式版本。

发布脚本严格按契约生成并校验清单。Server 读取时忽略未知字段，只使用版本、发布页地址和当前产物的文件名、下载地址、大小与更新方式；插件格式版本仅供展示，兼容性写在发布说明中。

## 发布流程与通道

按需发布，维护目标为每季度至少一个正式版；预发布不计作正式版，失败门禁与未完成的验收不能因发布节奏跳过。v0.4.0 是 v0.3.1 之后的首次公开分发，先发布 `v0.4.0-beta.1` 预发布；macOS arm64 保持 `experimental`，先验收安装与初始化。

1. 完成目标版本代码和 `docs/release/notes/<完整标签>.md`，将它们提交到同一提交并推送主分支。
2. 在 GitHub Actions 对该提交运行 `nightly`，或等待每日回归。记录完整提交 SHA 与运行链接；代码或发布说明再次变更后，必须对新提交重新运行。
3. 对已通过 nightly 的提交打完整版本标签，例如 `v0.4.0-beta.1`，再推送该标签。预发布同样必须有对应标签的发布正文，不复用另一个标签的文件。
4. `release.yml` 在构建前和发布前都检查该提交最新一次 nightly：必须来自本仓库的定时或手动运行，且状态为完成、结果为成功。其他提交的成功、旧运行的成功、进行中、取消、失败以及无法读取验证结果都不能放行。
5. 构建四个平台包并执行原有 smoke 后发布。Server 产物固定 `CGO_ENABLED=0`；Launcher 使用各平台所需的原生构建环境。
6. 从公开下载入口执行该版本支持的[实包验收](../engineering/manual-smoke.md#公开发行物)，登记真实结果与未执行项。正式版仍需补齐公开插件安装、更新和恢复的全流程记录。

含 SemVer 预发布段的版本使用 `channel: beta`、GitHub `prerelease: true` 与 `make_latest: false`；普通版本使用 `stable`，latest 由 GitHub 的 `legacy` 策略决定。仅构建元数据含连字符（例如 `1.2.3+build-1`）仍是正式版本。生成清单时若显式 channel 与版本不一致，发布工具拒绝执行。默认检查稳定版；Web 的“配置 → 版本与更新”可选择测试版通道，或从发布列表固定具体版本。测试版通道同时包含稳定版，按 SemVer 选择最新版本；自动更新不执行降级。

通道不改变插件版本兼容性。当前 [manifest v4 契约](../../contracts/plugin-info.schema.json)要求 `min_core_version` 至少为 `0.4.0`，不接受 `0.4.0` 的预发布声明；因此 `0.4.0-beta.1` 不能安装 v4 插件。首个预发布先验收核心安装与初始化，插件流程在满足最低核心版本要求的产物上验收。

`nightly-status.yml` 在 nightly 结束后维护同一个失败 issue。仅默认分支最新运行可以更新它，失败时创建或重新打开，恢复后关闭；旧运行、其他分支和取消的运行不改写状态。问题汇总使用独立工作流，不改变 nightly 本身的验证结论。

## 更新检查

Launcher 每 6 小时检查发布版本，发现新版本后提供立即更新和发布页入口。Web 使用 `GET /api/update/status` 查看状态，使用 `POST /api/update/check` 主动检查。CLI 提供：

```text
raylea-server version --json
raylea-server update check --json
raylea-server update download
raylea-server update download --file <本地发布包>
raylea-server update apply
```

版本检查按保存的 `update` 配置通过 HTTPS 读取发布清单，比较当前版本并返回发布页地址及线路观测结果。当前安装缺少有效 `build_info.json` 时，Launcher 提供项目发布页入口。

## 一键更新

桌面完整包在 Launcher 的“关于应用”中确认后更新：

1. Launcher 调用 `raylea-server update download --progress-json`，按保存的通道与版本测速选源、下载并解压检查；服务继续运行，界面显示下载进度。
2. 准备完成后，CLI 原子保存目标版本、归档及暂存目录，返回准备 ID。Launcher 停止服务，调用 `raylea-server update apply --prepared <ID>`，只安装这份已固定的本地结果，不联网、不重新选择版本。已有文件先移入 `cache/update/replaced/`，`build_info.json` 最后写入。
3. Launcher 启动新版 Launcher 后退出；新版等待旧进程退出再接管单实例。更新前由 Launcher 管理且仍在运行的服务，会在新版 Launcher 初始化成功后自动启动一次；启动失败时按普通启动流程显示原因，不自动重试。更新前已停止的服务保持停止。

其他程序启动的服务须先在 Web 管理面停止，再重试安装；取消确认会结束本次更新。退出 Launcher 会取消并等待下载结束；已开始替换程序文件时，退出等待安装完成。停服后安装或重启 Launcher 失败时，服务保持停止，可重试更新或按失败提示手动处理。

服务端包先在服务运行时准备更新，再进入停服窗口安装：

```text
./raylea-server update download
sudo systemctl stop rayleabot
./raylea-server update apply
sudo systemctl start rayleabot
```

更新只写入发布包包含的文件，不触碰 `config/`、`data/`、`plugins/`、`logs/`、`backups/` 与 `.deps/store/`；新版本不再包含的旧文件保留。数据库在更新后的首次启动时前向迁移。

## 加速线路与版本选择

Web 的“配置 → 版本与更新”与 `config/user.yaml` 的 `update` 字段对应，保存后下次检查或准备生效，Launcher 与 CLI 共用这份配置。

- `channel`：`stable` 只接收稳定版；`beta` 包含稳定版与预发布版。
- `version`：空字符串跟随所选通道；填写不带 `v` 的具体版本时固定该版本。目标必须严格高于当前版本，不能借此回退数据库或跨越不兼容安装边界。
- `mode`：`auto` 检测直连与加速源；`direct` 不使用 GitHub 加速前缀；`proxy` 不直连 GitHub。显式配置的完整发布镜像不受此开关影响。
- `proxies`：最多 6 个 HTTPS 前缀，可增删、清空。支持 `https://proxy.example`、`https://proxy.example/{url}` 和已经拼接 GitHub 链接的形式，保存时规范为前缀，去重后使用。仅改写 GitHub、GitHub API 与 raw 主机，不给非 GitHub 地址重复加速，不发送凭据。
- `mirrors`：最多 3 个完整发布镜像根地址，默认为空。布局以发布契约的 `x-update-distribution` 为准。

默认启用 [GH-Proxy](https://gh-proxy.com/docs/github-accelerator)、[ghfast.top](https://ghfast.top/) 与 [ghproxy.net](https://ghproxy.net/) 三个加速前缀，同时保留 GitHub 直连。它们是第三方服务，可用性不作保证，实际选用以部署机器的检测结果为准；使用它们即把它们视为受信的分发路径，见[更新策略与理由](#更新策略与理由)。

`auto` 同时检测直连与各加速源的发布信息，选用其中最新的有效版本，版本相同时选响应更快的来源；网页、错误响应和无效清单不计为可用。Web 的“检查版本与线路”显示本次检测结果，“刷新版本列表”最多列出 30 个版本。部分加速前缀只支持文件下载、不支持 GitHub API，版本列表会改用其他可用线路。

下载前读取各线路归档开头的一小段样本，确认是有效归档并按响应速度排序。下载或包内容校验失败时切换到下一条线路，最多尝试两轮；磁盘、权限等本机错误直接报告，不换线路重新下载。归档的大小、CRC/gzip、版本和平台都通过检查后才进入待安装状态。同一安装同一时间只能进行一个更新准备或安装。

准备与应用分别保存于 `cache/update/`，属于可重建缓存。准备完成后，即使网络断开也能执行 `update apply`；没有准备记录时该命令会提示先下载。应用失败保留本地归档供重试，重试重新解压，不联网。通过 `update download --file <archive>` 可以导入另一台设备取得的正式发布包，导入与安装均不需要网络。

## 可选官方镜像发布

仓库提供 `update-mirror.yml`，在启用后随正式发布运行，也支持手动补同步。此仓库没有预置已部署的官方镜像域名。部署者需要配置仓库变量 `UPDATE_MIRROR_ENABLED=true`、`UPDATE_MIRROR_PUBLIC_URL`、`UPDATE_MIRROR_BUCKET`、`UPDATE_MIRROR_ENDPOINT`，并在 Actions Secrets 中提供 `UPDATE_MIRROR_ACCESS_KEY` 与 `UPDATE_MIRROR_SECRET_KEY`。支持 S3 兼容对象存储，包括 Cloudflare R2；其他 S3 服务可通过 `UPDATE_MIRROR_REGION` 设置所需地域（默认 `auto`）。不把凭据写入用户配置或发布清单。

同步仅处理已发布且使用当前格式的发行物，上传平台包与按版本存放的清单后，才更新版本索引及稳定版／测试版指针；根清单缓存 5 分钟，按版本归档长期缓存。任务串行执行，同步失败使任务失败，可手动重新运行。公开 URL 配置到客户端 `update.mirrors` 后才会使用；仅设置 Actions 变量不会自动改变既有安装的更新来源。

## 手动更新

以下覆盖更新步骤适用于配置与备份格式仍受本版支持的安装。0.3.x 与 0.4.0 不兼容，需要全新安装，见[兼容性说明](./notes/v0.4.0.md#03x-兼容性)。

1. 从发布页下载对应平台的包。
2. 停止服务。建议先执行 `backup` 保存配置、数据库和插件数据。
3. 把发布包根目录中的内容解压覆盖到安装根。
4. 启动服务，检查健康状态、配置和插件。

数据库 `000001`、`000002`、`000003`、`000004`、`000005`、`000006` 在首次启动时逐步事务迁移到 `000007`；从旧结构备份恢复的安装同样在首次启动时迁移。`000004` 新增消息小时计数、最近收信时间、服务运行与连接离线记录；消息统计从升级后服务首次运行时开始。`000005` 为管理日志建立时间排序与过期清理索引，保留历史时间原值和清理规则。`000006` 为插件 KV 建立元数据索引，减少配额核算和前缀列表对 JSON 正文所在数据页的访问，并保留历史非规范数值的求和行为。`000007` 在一个事务中改写管理日志表，将历史 julianday 数值、不同小数精度及带时区偏移的文本转换为 Unix 纳秒整数，保留行 ID 和详情，将时间表达式索引替换为普通复合索引；同一个时间索引也用于保留期范围删除。动态时间、无法解析或超出纳秒整数范围的日志行会清除。首次迁移需要读取已有日志或 KV 数据并构建索引，`000007` 还需要改写整张日志表；数据较多时启动耗时和临时磁盘占用会增加。

GitHub 自动生成的源代码压缩包不是正式运行时产物。

## 更新策略与理由

以下取舍是有意的设计，不是待补的缺口。改变做法前，先推翻这里的理由并同步本节。

| 不做 | 理由 |
| --- | --- |
| 核心更新包的 SHA-256 摘要 | 核心更新信任配置中的分发源，包括默认启用的公共加速源。HTTPS、归档大小与 ZIP CRC32 或 gzip 校验只能发现损坏，不能认证第三方内容；从同一来源取得的摘要同样不能。不信任这些来源时，删除加速地址或选择直连。 |
| 发布签名 | 签名是唯一能防止发布源被篡改的手段，但需要长期的密钥管理与轮换；签名体系已被有意删除，不再恢复。 |
| 更新回滚、迁移前数据库副本 | `build_info.json` 最后写入，替换中断时安装仍报告旧版本，重新执行 `update apply` 或手动解压覆盖即可完成。数据库迁移在事务内执行，失败时保持迁移前状态。退回旧版本使用旧发布包与更新前的备份。 |
| 独立更新程序、系统原生脚本 | Windows 允许给正在运行的程序改名，Linux 与 macOS 可以直接替换运行中的文件，`update apply` 以先改名再移入完成替换，Launcher 自行重启即可。脚本需要维护三个平台，难以测试，出错时也无法在界面报告。 |

同类项目同样没有这些机制：Caddy 的 upgrade 命令只依赖 HTTPS，AstrBot 检查 ZIP 结构后原地覆盖，Yunzai 通过 git pull 更新，Koishi 由 npm 更新依赖。

发布清单不是安全边界：读取端宽松，发布端严格。Server 读取清单时忽略未知字段，只校验实际使用的字段，新版本增加字段或调整插件合同不会让旧版本检测不到更新；发布脚本按契约严格生成并校验清单。

Chromium、FFmpeg 和插件商店继续按各自契约校验 SHA-256。核心更新经加速源取得的清单与归档出自同一来源，无法独立认证内容，使用加速源即信任这些服务。

## 相关文档

- [Acceptance and Risks](./acceptance-and-risks.md)
- [Plugin Store and Independent Development](../plugin/store-and-development.md)
- [Deployment](../user/deployment.md)
- [Recovery](../user/recovery.md)
