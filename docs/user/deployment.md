# Deployment

本页说明 RayleaBot 当前正式支持的安装入口、本地交付形态和附加部署方式。

## 当前交付入口

| 产物 | 正式入口 |
| --- | --- |
| `windows-x64-full` | 包根目录 `RayleaLauncher.exe` |
| `linux-x64-full` | 包根目录 `RayleaLauncher` |
| `macos-arm64-full` | 包根目录 `RayleaLauncher.app` |
| `linux-x64-server` | 包根目录 `raylea-server` |

- full artifact 以 Launcher 作为桌面入口。
- `linux-x64-server` 面向无桌面环境、自托管服务进程与 `systemd` 管理场景。
- 发行包根目录同时是默认运行根目录。

## 平台提示

| 平台 | 当前提示 |
| --- | --- |
| Windows | 首次运行 Launcher、图片渲染 Chromium 或本地运行时资源时，系统可能弹出 Defender / SmartScreen 扫描或提示。正式校验方式是对照 `release_manifest.json` 与 `SHA256SUMS.txt`。 |
| macOS | `macos-arm64-full` 以目录包交付，首次打开前需要先做本地校验，并按系统提示授予运行许可。 |
| Linux | `linux-x64-server` 适合 `systemd` 与无桌面自托管；full artifact 适合桌面环境。 |

## 本地部署

- 单个发行包根目录同时承载 `config/`、`data/`、`cache/`、`logs/`、`plugins/installed/` 和 `.deps/`。
- 升级时继续沿用原根目录，不覆盖用户配置、状态和已安装插件。
- 首次运行可能按 `.deps/manifest.json` 准备图片渲染 Chromium、Python、Node.js 和 npm 资源；系统 Chrome、Chromium 或 Edge 可满足图片渲染 Chromium。

## Docker 边界

- 容器化属于补充自建部署方式，仓库当前不附带正式 Dockerfile、Compose 文件或容器镜像。
- 容器化长期运行时，目录职责仍遵守正式发行包根目录结构。
- SQLite 状态库存放路径应使用稳定可写卷，不直接落在不可靠的网络文件系统上。
- 容器场景应显式设置时区，避免调度任务按错误时区触发。

## Linux systemd / LXC

- Linux 自托管优先推荐“原生 `raylea-server` + `systemd` 服务 + Web 管理面”的路径。
- `linux-x64-server` 包内包含 `systemd/rayleabot.service` 示例服务文件，覆盖工作目录、自动重启和日志输出。
- `systemd` 部署继续复用正式发行包目录结构，不另造第二套路径模型。
- LXC 场景需要额外确认图片渲染 Chromium 资源、字体资源、权限映射和 SQLite 可写性。
- 非特权 LXC 如使用 bind mount，需要确认 `subuid` / `subgid` 或目录 owner 映射正确。
- GPU passthrough 未验证前，图片渲染 Chromium 维持 CPU 渲染是更稳妥的路径。
- ARM64 Linux / LXC 可通过 `render.browser_path` 指向宿主机 Chrome、Chromium 或 Edge。

## 当前边界

- 当前正式范围不包含自动覆盖更新。
- full artifact 的长期巡检目标是服务可长期管理，不包含 Launcher 图形界面自动化。
