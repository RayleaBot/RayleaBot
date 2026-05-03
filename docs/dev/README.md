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
- `start.bat` 使用 Web 开发服务器，管理面地址为 `http://127.0.0.1:4173/`。
- Web 开发服务器默认代理到 `http://127.0.0.1:8080`；自定义后端地址使用 `VITE_BACKEND_TARGET`。
- 需要检查后端托管的静态管理面时，使用 `set "RAYLEA_START_WEB_MODE=build" && start.bat`。
