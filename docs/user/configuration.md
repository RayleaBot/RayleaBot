# Configuration

本页说明 RayleaBot 当前用户可见的配置文件模型、目录职责和运行根目录语义。

正式配置结构以 `contracts/config.user.schema.json` 为准。

## 配置文件模型

- `config/default.yaml` 提供发行包默认基线。
- `config/user.yaml` 保存用户自定义配置。
- 服务运行时按 `default.yaml -> user.yaml` 生成有效配置，并在保存时输出 canonical 结构。
- 首次启动如缺少 `config/user.yaml`，服务会基于默认模板生成首份用户配置。
- 敏感连接信息和凭据由受控配置入口处理，不把明文 secret 散落到公开界面和诊断面。

## 配置生效方式

| 类别 | 典型内容 | 生效方式 |
| --- | --- | --- |
| 立即生效 | 日志级别、留存天数、部分插件非敏感配置 | 服务端保存后直接应用 |
| 局部重载或重连 | OneBot11 连接信息、调度时区、`render.browser_path`、部分运行时资源配置 | 触发局部重连、重建或受控重载 |
| 需要重启 | Web 监听地址、SQLite 路径、关键目录根路径 | 保存后进入 `restart_required`，服务重启后生效 |

## 配置提醒

- 容器或跨时区部署建议显式设置 `scheduler.timezone`。
- ARM64 或自定义浏览器场景可使用 `render.browser_path` 指向实际 Chromium 路径。
- 配置结构、默认值和字段约束以 `contracts/config.user.schema.json` 为准。

## 当前目录职责

| 路径 | 用途 |
| --- | --- |
| `config/` | 默认模板与用户配置 |
| `data/` | SQLite 状态库和插件业务数据 |
| `cache/` | 渲染缓存、下载缓存和临时缓存 |
| `logs/` | 结构化日志与诊断输出 |
| `plugins/installed/` | 用户安装插件 |
| `.deps/` | 运行环境资源与展开目录 |

## 运行根目录

- 发行包根目录同时是默认运行根目录。
- 复用已有工作区时，继续沿用原有 `config/`、`data/`、`cache/`、`logs/` 和 `plugins/installed/`。
- Launcher 的运行目录覆盖只影响进程工作目录和本地数据目录，不改变 `.deps/` 与 `templates/` 的正式位置。
- 运行环境与模板资源的有效根目录跟随 RayleaBot 根目录，而不是临时工作目录。

## 配置与管理面

- 正式配置读写入口是 Web 管理面和受控后端逻辑。
- 通用配置页承接协议连接设置之外的正式配置项。
- 协议中心承接 OneBot11 连接地址、访问令牌和 adapter 重连参数，保存继续使用统一配置入口。
- 字段级热更新与 `restart_required` 由服务端统一判断。
- 插件配置读写必须通过正式插件能力，不直接改写平台用户配置文件。

## 当前边界

- 用户可编辑的是配置文件和明确开放的管理入口，不是程序托管目录中的内部状态文件。
- `cache/`、`logs/`、`.deps/` 和状态库文件不作为常规人工编辑对象。
