# Developer Docs

本目录整理 RayleaBot 的开发、调试、诊断和仓库协作说明。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [repo-workflow.md](./repo-workflow.md) | 仓库跟踪边界与常规忽略策略 |
| [diagnostics.md](./diagnostics.md) | 正式诊断入口与排障文档 |
| [logging.md](./logging.md) | 日志分类与重复故障汇总 |
| [text-resources.md](./text-resources.md) | 文本资源和国际化边界 |
| [onebot-compatibility.md](./onebot-compatibility.md) | OneBot11 各实现端的事件、消息段与扩展支持范围 |

## 本地启动

- Windows 本地开发入口为仓库根目录的 `start.bat`，Linux 和 macOS 使用 `sh start.sh`；两个包装器都调用 `scripts/start-dev.mjs`。
- 两个包装器都从 `.tool-versions` 读取 Node 版本，并要求实际版本精确匹配。
- `start.sh` 优先使用父进程提供的绝对路径 `RAYLEA_NODE_EXECUTABLE`，未设置时使用 `PATH` 中的 Node；`start.bat` 依次检查父进程提供的 `RAYLEA_START_NODE`、`%USERPROFILE%\.local\opt\node-v<version>-win-x64\node.exe` 和 `PATH`。
- `RAYLEA_START_NODE` 必须由父进程在运行 `start.bat` 前设置；包装器在 Node 启动前选择可执行文件，因此不会从 `.env` 读取该覆盖项。
- 统一编排器会加载仓库根目录的 `.env`；可复制 `.env.example` 并按注释配置本地启动参数，父进程已设置的环境变量优先。
- 默认 profile 使用 Web 开发服务器，管理面地址为 `http://127.0.0.1:4173/`。
- Web 开发服务器代理到 `config/user.yaml` 中的 `server.host` / `server.port`；自定义后端地址使用 `VITE_BACKEND_TARGET`。
- WebSocket 后端地址使用 `VITE_WS_BASE_URL`，缺省值与 `VITE_BACKEND_TARGET` 一致。
- `web-dev` 和 `launcher-dev` 模式下，Launcher 打开的管理面固定使用脚本管理的 Vite 开发入口 `http://127.0.0.1:4173/`，支持源码热更新；`.env` 或父进程中的 `RAYLEA_WEB_UI_BASE_URL` 不覆盖该入口。`build` 模式仍打开 Server 托管的静态页面。

| Profile | 用途 | 命令 |
| --- | --- | --- |
| `web-dev` | Web 热更新、Server 构建、Launcher 启动 | `start.bat` / `sh start.sh` |
| `build` | 后端托管静态管理面验证 | 设置 `RAYLEA_START_PROFILE=build` 后运行包装器 |
| `launcher-dev` | Launcher 本体热更新 | 设置 `RAYLEA_START_PROFILE=launcher-dev` 后运行包装器 |

兼容环境变量：

- `RAYLEA_START_PROFILE=build` 使用构建产物启动 Web 管理面。
- `RAYLEA_START_SKIP_LAUNCH=1` 执行准备与启动检查，不打开 Wails Launcher。

只开发 Launcher 时，可在仓库根目录运行 `pnpm --dir launcher dev`。

依赖安装策略：

| `RAYLEA_START_INSTALL` | 行为 |
| --- | --- |
| `auto` | 依赖输入内容、工具链变化或安装产物缺失时安装依赖 |
| `always` | 每次启动安装依赖 |
| `skip` | 跳过依赖安装 |

端口与日志：

- Web 开发服务器使用 `127.0.0.1:4173`。
- `4173` 上已有 RayleaBot Web 开发服务器时直接复用。
- `4173` 被其他程序占用时，启动脚本会显示占用原因并退出。
- 启动日志位于 `logs/dev/start/YYYY-MM-DD.log`。
- Server 热重载输出位于 `logs/dev/server/YYYY-MM-DD.log`。
- Web 开发服务器输出位于 `logs/dev/web/YYYY-MM-DD.log`。
- Launcher 输出位于 `logs/dev/launcher/YYYY-MM-DD.log`。
- 构建输出位于 `logs/dev/build/YYYY-MM-DD.log`；启动日志记录编排和子日志位置，不重复保存子进程全文。
- 终端显示阶段耗时、插件进度与运行摘要；构建资源清单保存在构建日志中，失败时显示诊断与日志位置。重定向输出自动使用静态行，`NO_COLOR` 可关闭交互终端配色。详见[日志说明](./logging.md)。

## 增量构建与环境复用

重复运行启动包装器时，同一工作区、相同启动配置和脚本版本的健康 Server / Web 开发环境保持运行，Launcher 自动打开或聚焦。设置 `RAYLEA_START_RESTART=1` 可重新启动；Server 与开发插件先完成构建预检，再优雅停止旧环境。预检失败时保留健康旧环境，修复后重新启动。未受当前工作区租约管理的 Server 不会被接管。

`.tmp/dev-cache/` 保存内容摘要与构建产物，覆盖 Server、插件构建工具、插件后端、UI、展开 artifact、Launcher bindings、前端和原生程序。输入内容、工具链、平台或构建参数变化会使对应缓存失效；产物缺失或内容变化会触发修复。修改文件时间或重复启动不会单独触发编译。构建期间收到的修改会在切换运行时前重新检查。

开发子进程的 `TEMP`、`TMP`、`TMPDIR` 与 `GOTMPDIR` 指向 `.tmp/dev-cache/tmp/`，Go 构建及插件打包的临时文件留在项目内。发布归档 smoke 的解压目录位于 `.tmp/release-smoke/`，校验结束后清理。

开发依赖安装显式限制当前 OS、CPU 和 Linux libc。Vue SDK 镜像按内容同步文件，保留已有 `node_modules`。安装依赖的判断使用 package、lockfile、workspace 配置、SDK package 与工具链内容，不依赖文件更新时间。

插件后端通过当前平台的 `go list` 输入图判定变化，包含本地依赖和 `go:embed` 文件。每个插件使用独立的临时 `go.work`，仅连接 SDK 和自身声明的本地模块，避免无关插件的模块错误影响构建。UI 修改只重建 UI 与 artifact；manifest、未嵌入 Go 的模板和资源修改只组装 artifact。开发 artifact 使用标准展开目录，不生成 ZIP；许可证和 notices 仍随产物保留。

监听模式覆盖 Server、插件仓库、插件 `go.work` 引用的本地模块、Go / Vue SDK、Go module / workspace 文件和开发工作区清单，按 500ms 窗口和插件 ID 合并变更；只有 Server 自身构建输入变化才重启 Server。README、测试、CI 文件与其他平台源码不触发无关编译；Go 实际嵌入的文件按构建输入处理。工作区增删或禁用条目会更新监听集合，移除条目不会自动卸载已安装插件。插件管理页使用增量静态构建，页面刷新后读取新资源。

Go 输入图中仅承载 module / workspace 元数据的目录只响应已登记输入文件的变化，Launcher 写入探测和其他临时文件不会触发开发同步；源码与嵌入资源目录仍监听文件新增、删除。输入变化按所属构建目标处理，共享依赖只通知使用它的目标。每轮同步的触发路径记录在启动日志的 DEBUG 条目中。

开发插件独立构建和同步；单个插件失败不阻断同批其他插件，失败项保留到相关源码或工作区变化后重试，不循环重试刷屏。单个仓库的坏清单单独报告，其目录继续监听以发现修复；工作区 JSON 或条目结构错误仍需先修复。没有健康旧环境时，首次启动继续安装可用插件并启动 Server，失败插件保留已有安装包。

开发插件通过仅本机可用的认证接口进入正常安装事务，Server 不执行源码发现或构建命令。未变化的安装内容跳过任务和重载；更新保留启用/停用状态。插件初始化失败恢复旧包和运行时，其他插件保持运行。Server 源码更新使用候选二进制，通过健康检查后才删除旧版本；构建或启动失败时保留可用版本。

缓存为本地可再生成数据，不应提交。修改启动脚本或切换工具链后应重新运行包装器；Launcher 原生代码联调使用 `launcher-dev`，已有原生窗口不会在后台自行替换。

Launcher 的 Wails CLI 版本来自 `launcher/go.mod`，工具二进制保存在 `.tmp/dev-cache/wails-cli/`，用于 bindings 和打包资源生成。构建工具使用 Wails 自身的依赖文件，与 Launcher 运行依赖隔离。Wails 模块和工具依赖已缓存时可以离线生成；首次下载仍需要可用的 Go 模块代理。
