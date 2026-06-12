# Developer Docs

本目录整理 RayleaBot 的开发、调试、诊断和仓库协作说明。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [repo-workflow.md](./repo-workflow.md) | 仓库跟踪边界与常规忽略策略 |
| [diagnostics.md](./diagnostics.md) | 正式诊断入口与排障面 |
| [text-resources.md](./text-resources.md) | 文本资源和国际化边界 |
| [electron-glass-ui-lessons.md](./electron-glass-ui-lessons.md) | 现有桌面 UI 经验记录 |

## 当前原则

- 默认命令、版本线和 CI 门禁以 `docs/engineering/` 为准。
- 本目录说明开发入口和调试路径，不单独定义对外接口。

## 本地启动

- Windows 本地开发入口为仓库根目录的 `start.bat`。
- Windows 环境执行开发命令使用 `gbash -lc '<command>'`。
- `start.bat` 使用 Web 开发服务器，管理面地址为 `http://127.0.0.1:4173/`。
- Web 开发服务器代理到 `config/user.yaml` 中的 `server.host` / `server.port`；自定义后端地址使用 `VITE_BACKEND_TARGET`。
- WebSocket 后端地址使用 `VITE_WS_BASE_URL`，缺省值与 `VITE_BACKEND_TARGET` 一致。
- Launcher 打开的管理面地址使用 `RAYLEA_WEB_UI_BASE_URL`，缺省值为 `http://127.0.0.1:4173/`。

| Profile | 用途 | 命令 |
| --- | --- | --- |
| `web-dev` | Web 热更新、Server 构建、Launcher 启动 | `./start.bat` |
| `build` | 后端托管静态管理面验证 | `RAYLEA_START_PROFILE=build ./start.bat` |
| `launcher-dev` | Launcher 本体热更新 | `RAYLEA_START_PROFILE=launcher-dev ./start.bat` |

兼容环境变量：

- `RAYLEA_START_PROFILE=build ./start.bat` 使用构建产物启动 Web 管理面。
- `RAYLEA_START_SKIP_LAUNCH=1 ./start.bat` 执行准备与启动检查，不打开 Electron。

依赖安装策略：

| `RAYLEA_START_INSTALL` | 行为 |
| --- | --- |
| `auto` | `node_modules` 缺失或 lockfile 更新时间较新时安装依赖 |
| `always` | 每次启动安装依赖 |
| `skip` | 跳过依赖安装 |

端口与日志：

- Web 开发服务器使用 `127.0.0.1:4173`。
- `4173` 上已有 RayleaBot Web 开发服务器时直接复用。
- `4173` 被其他程序占用时，启动脚本会显示占用原因并退出。
- 启动日志位于 `logs/dev/start.log`。
- Web 开发服务器输出位于 `logs/dev/web-dev.log`。
- Launcher 输出位于 `logs/dev/launcher.log`。
