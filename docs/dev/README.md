# Developer Docs

本目录整理 RayleaBot 的开发、调试、诊断和仓库协作说明。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [repo-workflow.md](./repo-workflow.md) | 仓库跟踪边界与常规忽略策略 |
| [diagnostics.md](./diagnostics.md) | 正式诊断入口与排障文档 |
| [text-resources.md](./text-resources.md) | 文本资源和国际化边界 |

## 当前原则

- 默认命令、版本线和 CI 门禁以 `docs/engineering/` 为准。
- 本目录说明开发入口和调试路径，不单独定义对外接口。

## 本地启动

- Windows 本地开发入口为仓库根目录的 `start.bat`，Linux 和 macOS 使用 `sh start.sh`；两个包装器都调用 `scripts/start-dev.mjs`。
- 两个包装器都从 `.tool-versions` 读取 Node 版本，并要求实际版本精确匹配。
- `start.sh` 使用 `PATH` 中的 Node；`start.bat` 依次检查父进程提供的 `RAYLEA_START_NODE`、`%USERPROFILE%\.local\opt\node-v<version>-win-x64\node.exe` 和 `PATH`。
- `RAYLEA_START_NODE` 必须由父进程在运行 `start.bat` 前设置；包装器在 Node 启动前选择可执行文件，因此不会从 `.env` 读取该覆盖项。
- 统一编排器会加载仓库根目录的 `.env`；可复制 `.env.example` 并按注释配置本地启动参数，父进程已设置的环境变量优先。
- Windows 环境执行开发命令使用 `gbash -lc '<command>'`。
- 默认 profile 使用 Web 开发服务器，管理面地址为 `http://127.0.0.1:4173/`。
- Web 开发服务器代理到 `config/user.yaml` 中的 `server.host` / `server.port`；自定义后端地址使用 `VITE_BACKEND_TARGET`。
- WebSocket 后端地址使用 `VITE_WS_BASE_URL`，缺省值与 `VITE_BACKEND_TARGET` 一致。
- Launcher 打开的管理面地址使用 `RAYLEA_WEB_UI_BASE_URL`，缺省值为 `http://127.0.0.1:4173/`。

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

## 增量构建与环境复用

重复运行启动包装器时，同一工作区、相同启动配置和脚本版本的健康 Server / Web 开发环境保持运行，Launcher 自动打开或聚焦。设置 `RAYLEA_START_RESTART=1` 可优雅停止旧环境后重新启动。未受当前工作区租约管理的 Server 不会被接管。

`.tmp/dev-cache/` 保存内容摘要与构建产物，覆盖 Server、插件构建工具、插件后端、UI、展开 artifact、Launcher bindings、前端和原生程序。输入内容、工具链、平台或构建参数变化会使对应缓存失效；产物缺失或内容变化会触发修复。修改文件时间或重复启动不会单独触发编译。构建期间收到的修改会在切换运行时前重新检查。

开发依赖安装显式限制当前 OS、CPU 和 Linux libc；发布构建的默认平台矩阵保持不变。Vue SDK 镜像按内容同步文件，保留已有 `node_modules`。安装依赖的判断使用 package、lockfile、workspace 配置、SDK package 与工具链内容，不依赖文件更新时间。

插件后端通过当前平台的 `go list` 输入图判定变化，包含本地依赖和 `go:embed` 文件。UI 修改只重建 UI 与 artifact；manifest、未嵌入 Go 的模板和资源修改只组装 artifact。开发 artifact 使用标准展开目录，不生成 ZIP；许可证、notices 和 SBOM 仍随产物保留。

监听模式覆盖 Server、插件仓库、Go / Vue SDK、Go module / workspace 文件和开发工作区清单。README、测试、CI 文件与其他平台源码不触发无关编译；Go 实际嵌入的文件按构建输入处理。工作区增删或禁用条目会更新监听集合，移除条目不会自动卸载已安装插件。插件管理页使用增量静态构建，页面刷新后读取新资源。

开发插件通过仅本机可用的认证接口进入正常安装事务，Server 不执行源码发现或构建命令。未变化的安装内容跳过任务和重载；更新保留启用/停用状态。插件初始化失败恢复旧包和运行时，其他插件保持运行。Server 源码更新使用候选二进制，通过健康检查后才删除旧版本；构建或启动失败时保留可用版本。

缓存为本地可再生成数据，不应提交。修改启动脚本或切换工具链后应重新运行包装器；Launcher 原生代码联调使用 `launcher-dev`，已有原生窗口不会在后台自行替换。
