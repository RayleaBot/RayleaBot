# Configuration

本页说明 RayleaBot 当前用户可见的配置文件模型、目录职责和运行根目录语义。

正式配置结构以源码中的 `contracts/config.user.schema.json` 为准。发行包中的服务端程序内置运行时配置校验规则。

## 配置文件模型

- 当前用户配置固定为 `schema_version: "4"`。
- `config/user.yaml` 保存用户自定义配置。
- `data/launcher.json` 保存 Launcher 的本机设置，例如安装根选择、关闭行为和本地覆盖项。
- 服务运行时将内嵌 schema 默认值与 `user.yaml` 合成有效配置。
- 常规启动与配置验证只读取文件；初始化和规范化命令仅写入 `user.yaml`。缺省字段使用内嵌默认值，显式的 `false`、`0` 和空集合按 schema 校验并保留。
- Launcher 检测到 `user.yaml` 缺失且服务端程序可用时，会执行配置初始化并重新检查环境，再启动服务。
- 日志和诊断输出会过滤 `Authorization`、`access_token`、`token` 等敏感键。
- `adapters` 是已配置的聊天适配器实例列表，缺省为空表示不接收任何聊天流量。每个实例由 `id` 命名，`type` 决定生效的设置块（`onebot11` 或 `qqofficial`）；同一协议可以配置多个实例，各自持有自己的凭据与入站地址。
- `id` 决定实例的对外身份：它出现在事件的 `source_adapter`、出站路由、secret store 键名以及入站地址 `/api/adapters/{id}/reverse-ws`。重命名实例等于重新标识该适配器，并改变上述地址，需要同步修改协议端的回连配置。
- 通过管理端保存 OneBot11 访问令牌时，明文值写入本地 secret store，`user.yaml` 只保存 `secret://adapters/<实例 id>/onebot11/<transport>/access_token` 引用。
- QQ 开放平台官方机器人以 `type: qqofficial` 的实例接入。`app_id` 不是机密；`app_secret` 与 OneBot 访问令牌同样处理，明文写入 secret store，`user.yaml` 只保存 `secret://adapters/<实例 id>/qqofficial/app_secret` 引用。
- `qqofficial.intents` 按名称声明订阅的事件族，适配器负责映射为网关位掩码；名称集合由 schema 约束，写入未知名称会被拒绝。
- 实例的 `enabled` 是总开关。关闭的实例不连接、不接收入站流量、也不承载出站消息；OneBot 实例的传输配置仍可查看和修改，便于配置完再启用。
- OneBot11 访问令牌默认通过 `Authorization: Bearer` 传递。只有旧服务端或旧 webhook 客户端必须使用 URL query token 时，才将对应入口的 `access_token_query_compat` 显式设为 `true`。

## 配置文件维护

| 命令 | 作用 |
| --- | --- |
| `raylea-server config init` | 按内嵌默认值创建或规范化用户配置 |
| `raylea-server config normalize` | 按当前 schema 整理用户配置 |
| `raylea-server config validate` | 校验配置文件，不修改文件内容 |

缺少配置文件或需要整理配置格式时，使用显式配置命令处理。`raylea-server` 启动路径不承担初始化或格式化职责。

## 配置生效方式

管理 API 的 `config` 返回已保存的期望配置。`revision` 是当前服务进程内的配置修订号，从 1 开始，每次成功持久化后递增；校验失败、取消及写入失败不会增加修订号。需要重启的值保留在用户配置文件中，运行中的服务继续使用已生效值。

保存先校验完整配置、准备并解析 secret 引用，再保存凭据和用户文件；保存失败会尝试恢复本轮已写入的凭据，并返回服务端错误。持久化完成后，日志、限流、账号检查、渲染、聊天策略与适配器依次应用。聊天策略按完整快照切换；每个限流窗口的次数与时长一起替换；一次渲染使用同一份输出格式、页脚与缩放快照。不同服务和连接动作不组成跨服务事务。

`apply_effects.failed_groups` 表示已经保存但运行时应用失败的配置组，相关字段同时列入需重启项。适配器应用失败会尝试恢复原设置；完成恢复或重启前应检查连接状态。`effective_timezone` 始终表示当前运行时区。

常规字段以 schema 的四种 `x-apply-policy` 为准：

| 策略 | 典型内容 | 保存后的效果 |
| --- | --- | --- |
| `read_only` | `schema_version` | 只用于标识当前配置格式，不作为运行期可变设置 |
| `hot_reload` | 命令前缀、内置菜单、权限、渲染输出与队列参数、三方账号检查间隔、存储配额、日志、消息、用户和 HTTP 参数 | 保存后直接应用，列入 `apply_effects.applied_now` |
| `adapter_reload` | OneBot11 连接地址、兼容开关、QQ 官方机器人的 AppID / 订阅事件 / 沙箱开关，以及 adapter 连接和重连参数 | 保存后受控重载对应实例，列入 `apply_effects.reloaded_now` |
| `restart_required` | Server 与数据库、管理会话、渲染浏览器与 worker、抖音扫码浏览器、调度时区、插件运行限制、Web | 配置已保存，但服务重启后才生效，列入 `apply_effects.restart_required_fields` |

OneBot11 `access_token` 与 QQ `app_secret` 使用专门的 `secret_only` 元数据：管理 API 把明文写入本地 secret store，配置文件仅保存 `secret://` 引用；更新后与 adapter 配置一并受控重载。

`apply_effects` 中的字段路径按实例 id 寻址，例如 `adapters.onebot11.onebot11.forward_ws.url`；改变实例的增删或顺序则记为 `adapters` 本身，属于 `restart_required`。

重载只作用于设置发生变化的实例：其他实例的连接不受影响。QQ 官方机器人的凭据、订阅事件与沙箱开关都在建立连接时固定，因此改动这些字段会让该实例立即断开并按新配置重连（不等待重连退避），而未改动时连接原样保留。

## 三方账号检查与抖音扫码浏览器

`third_party_accounts` 控制 CK 自动检查和抖音扫码获取 CK 使用的浏览器：

```yaml
third_party_accounts:
  credential_check_interval_minutes: 360
  douyin_login:
    browser_mode: auto
    remote_debugging_url: ""
```

- `credential_check_interval_minutes` 是服务端自动检查已启用账号 CK 的间隔，默认 `360` 分钟；`0` 关闭自动检查，三方账号页仍可手动检查。非零值范围为 `15` 到 `10080`，保存后立即生效。
- `browser_mode: auto` 优先连接已配置且可用的本机 CDP 专用浏览器，再启动可见的隔离浏览器；服务器没有图形界面时使用无头浏览器。
- `visible`、`headless` 和 `remote_cdp` 是显式模式，启动失败时不会切换到其他模式。
- 本地浏览器优先使用 `render.browser_path`，随后查找系统 Chrome、Edge 或 Chromium，最后使用 RayleaBot 托管的 Chromium。
- `remote_cdp` 必须配置 `remote_debugging_url`。地址只接受无凭据的本机回环 HTTP(S) 或 WS(S) 端点，例如 `http://127.0.0.1:9222`。
- CDP 浏览器应使用专用 profile，不应连接个人默认浏览器。RayleaBot 不读取个人浏览器的 Cookie。
- `douyin_login` 的两个字段保存后均需重启服务生效。

## 配置提醒

- `scheduler.timezone` 默认上海（`Asia/Shanghai`，UTC+08:00）；旧配置留空也采用该默认值。配置页参考 Windows 提供地区时区与常用城市，覆盖 UTC−12 至 UTC+14，以及半小时、四十五分钟偏移，支持按城市、地区、IANA 标识或 UTC 偏移搜索。历史别名与重复技术条目不默认展示，已有配置使用其他有效时区时仍原样保留。显示的偏移按当前日期计算，地区时区遵循夏令时规则。
- 时区影响定时任务、服务日志、管理面时间展示和历史日志筛选。保存后重启服务生效，重启前前后端继续使用当前时区；后台存储与 API 时间戳保持 UTC，服务日志按配置时区输出并携带偏移。历史日志日期输入遇到夏令时跳过的时间会提示修正，回拨时段的范围起止覆盖两次出现的时间。
- 自定义浏览器场景可使用 `render.browser_path` 指向 Chrome、Chromium 或 Edge 可执行文件路径。
- `render.default_output` 控制图片生成默认格式，支持 `png` 与 `jpeg`。
- `render.device_scale_percent` 控制图片生成精度，`100` 为当前基础倍率，范围为 `50` 到 `500`。
- `web.plugin_ui_origin_template` 必须包含 `{plugin_host}`。本机模式可省略并自动派生 `plugins.localhost` 子域；LAN 与反向代理模式必须显式配置不同于管理面的插件域模板。
- 旧 Python/Node 插件运行时配置键必须直接删除；doctor 会报告退役键，不会迁移或忽略后继续运行。
- 配置结构、默认值和字段约束以 `contracts/config.user.schema.json` 为准。

## 当前目录职责

| 路径 | 用途 |
| --- | --- |
| `config/` | 用户配置 |
| `data/` | SQLite 状态库、插件业务数据和 Launcher 本机设置 |
| `cache/` | 渲染缓存、下载缓存和临时缓存 |
| `logs/` | 结构化日志与诊断输出 |
| `plugins/installed/` | 用户安装插件 |
| `.deps/` | Chromium、FFmpeg / FFprobe 资源与展开目录 |

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
- 通用配置页负责协议连接设置之外的配置项。
- 协议中心负责 OneBot11 provider、reverse WebSocket 回连地址、forward WebSocket 主动连接地址、HTTP API 地址、webhook 回调地址、各连接方式访问令牌和 adapter 重连参数，保存继续使用统一配置入口。
- 日志中心位于一级菜单下，提供 `/logs` 与 `/logs/history` 两个正式日志页面。
- 字段级热更新与 `restart_required` 由服务端统一判断。
- 插件配置读写必须通过正式插件能力，不直接改写平台用户配置文件。

## 当前限制

- 用户可编辑的是 `config/user.yaml` 和明确开放的管理入口，不是程序托管目录中的内部状态文件。
- `data/launcher.json` 用于 Launcher 本机设置，不替代 `config/user.yaml`，也不作为常规人工编辑对象。
- `cache/`、`logs/`、`.deps/` 和状态库文件不作为常规人工编辑对象。
