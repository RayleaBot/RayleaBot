# Configuration

本页说明 RayleaBot 当前用户可见的配置文件模型、目录职责和运行根目录语义。

正式配置结构以源码中的 `contracts/config.user.schema.json` 为准。发行包中的服务端程序内置运行时配置校验规则。

## 配置文件模型

- `config/default.yaml` 提供发行包默认基线。
- `config/user.yaml` 保存用户自定义配置。
- `data/launcher.json` 保存 Launcher 的本机设置，例如安装根选择、关闭行为和本地覆盖项。
- 服务运行时按 `default.yaml -> user.yaml` 生成有效配置，并在保存时输出 canonical 结构。
- 首次启动如缺少 `config/user.yaml`，服务会基于默认模板生成首份用户配置。
- 日志和诊断输出会过滤 `Authorization`、`access_token`、`token` 等敏感键。

## 配置生效方式

| 类别 | 典型内容 | 生效方式 |
| --- | --- | --- |
| 立即生效 | 日志级别、留存天数、部分插件非敏感配置 | 服务端保存后直接应用 |
| 局部重载或重连 | OneBot11 连接信息、调度时区、`render.browser_path`、部分运行时资源配置 | 触发局部重连、重建或受控重载 |
| 需要重启 | Web 监听地址、SQLite 路径、关键目录根路径 | 保存后进入 `restart_required`，服务重启后生效 |

## 配置提醒

- 容器或跨时区部署建议显式设置 `scheduler.timezone`。
- 自定义浏览器场景可使用 `render.browser_path` 指向 Chrome、Chromium 或 Edge 可执行文件路径。
- `render.default_output` 控制图片生成默认格式，支持 `png` 与 `jpeg`。
- `render.device_scale_percent` 控制图片生成精度，`100` 为当前基础倍率，范围为 `50` 到 `500`。
- 配置结构、默认值和字段约束以 `contracts/config.user.schema.json` 为准。

## 当前目录职责

| 路径 | 用途 |
| --- | --- |
| `config/` | 默认模板与用户配置 |
| `data/` | SQLite 状态库、插件业务数据和 Launcher 本机设置 |
| `cache/` | 渲染缓存、下载缓存和临时缓存 |
| `logs/` | 结构化日志与诊断输出 |
| `plugins/installed/` | 用户安装插件 |
| `.deps/` | 运行环境资源与展开目录 |

## 日志目录

- `logs/launcher/YYYY-MM-DD.log` 保存 Launcher 自身诊断和服务进程编排信息。
- `logs/server/YYYY-MM-DD.log` 保存 `raylea-server` 的文本输出镜像。
- `logs/recovery-summary.json` 保存恢复与兼容摘要。

## 运行根目录

- 发行包根目录同时是默认运行根目录。
- 复用已有工作区时，继续沿用原有 `config/`、`data/`、`cache/`、`logs/` 和 `plugins/installed/`。
- Launcher 的运行目录覆盖只影响进程工作目录和本地数据目录，不改变 `.deps/` 与 `templates/` 的正式位置。
- 运行环境与模板资源的有效根目录跟随 RayleaBot 根目录，而不是临时工作目录。
- `data/launcher.json` 随同机目录保留，不属于正式恢复包范围。

## 配置与管理面

- 正式配置读写入口是 Web 管理面和受控后端逻辑。
- 通用配置页承接协议连接设置之外的正式配置项。
- 协议中心承接 OneBot11 provider、reverse WebSocket 回连地址、forward WebSocket 主动连接地址、HTTP API 地址、webhook 回调地址、各连接方式访问令牌和 adapter 重连参数，保存继续使用统一配置入口。
- 日志中心位于一级菜单下，提供 `/logs` 与 `/logs/history` 两个正式日志页面。
- 字段级热更新与 `restart_required` 由服务端统一判断。
- 插件配置读写必须通过正式插件能力，不直接改写平台用户配置文件。

## 当前边界

- 用户可编辑的是 `config/default.yaml`、`config/user.yaml` 和明确开放的管理入口，不是程序托管目录中的内部状态文件。
- `data/launcher.json` 用于 Launcher 本机设置，不替代 `config/user.yaml`，也不作为常规人工编辑对象。
- `cache/`、`logs/`、`.deps/` 和状态库文件不作为常规人工编辑对象。
