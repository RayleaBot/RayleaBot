# Management Surface

本页说明 RayleaBot 当前正式提供的管理入口、页面职责和跨页查看路径。

## 当前正式入口

| 入口 | 作用 |
| --- | --- |
| Web 管理面 | 在线管理主入口 |
| Launcher | 桌面启动、停机、本地预检和打开管理面 |
| CLI | 本地离线恢复与诊断入口 |

常规插件管理、状态查看、日志查询和配置编辑以 Web 管理面为主。服务端是正式状态源，Launcher 负责桌面壳交互与本地进程编排。

## Web 管理面页面

| 页面 | 路由 | 作用 |
| --- | --- | --- |
| 系统状态 | `/` | 查看健康、就绪、恢复摘要、近期变化和系统工具 |
| 插件 | `/plugins` | 安装、卸载、启用、禁用、重载、授权和查看插件列表 |
| 插件详情 | `/plugins/:id` | 查看 rich manifest metadata、权限、命令和实时控制台 |
| 指令中心 | `/commands` | 查看默认权限、冷却配置、黑名单和当前生效命令策略 |
| 任务 | `/tasks` | 查看任务列表、任务详情、恢复摘要和渲染结果 |
| 实时日志 | `/logs` | 查看当前服务启动窗口内的日志和增量更新 |
| 历史日志 | `/logs/history` | 查看按时间范围筛选的历史日志 |
| 协议中心 | `/protocols` | 查看 OneBot11 协议快照、连接设置和传输异常 |
| 协议兼容矩阵 | `/protocols/compatibility` | 查看正式兼容范围与 provider 差异 |
| 配置 | `/config` | 查看和保存通用配置 |
| 模板编辑 | `/render/templates/:templateId?` | 编辑模板、校验、预览、保存、回退和查看版本历史 |

## 页面联动

- 系统状态页的协议提醒和近期变化会提供到协议中心、日志中心、任务和插件详情的入口。
- 协议中心提供兼容矩阵入口和 `protocol=onebot11` 的相关日志入口。
- 指令中心支持 `plugin_id` 过滤，命令表中的插件列可直接进入插件详情。
- 插件详情提供当前插件的指令中心入口和历史日志入口。
- 实时日志与历史日志支持 `level`、`source`、`protocol`、`plugin_id`、`request_id` 和 `log_id` 工作区 query；历史日志额外支持 `start_at` 与 `end_at`。
- 日志详情会根据稳定字段提供插件详情、协议中心和请求 ID 对应日志页入口。
- 任务详情会根据稳定字段提供插件详情、协议中心、请求历史日志和模板编辑器入口。
- 模板编辑页的预览结果会提供任务详情入口，`render.preview` 任务详情会提供返回模板编辑器的入口。

## 状态来源与 Launcher 登录交接

- Web 与 Launcher 都直接访问服务端管理接口。
- Web 使用正式 HTTP API 与管理 WebSocket 读取状态和提交操作。
- Launcher 读取 `healthz`、`readyz`、`/api/system/status`、恢复摘要与任务信息，并把这些正式结果与本地进程状态一起展示。
- Launcher 打开 Web 时，通过 `POST /api/session/launcher-token` 申请一次性 launcher token。
- Web 启动时若 URL 含 `?token=`，会把该 token 交给 `POST /api/session/launcher-admission`，换成正常管理会话。
- `?token=` 仅用于单次深链交接；交接失败、token 失效或 admission 校验失败时，管理面回到初始化页或登录页。

## 本地优先安全边界

- 管理入口默认监听 `127.0.0.1`，远程访问需要显式开启。
- Web API 和 WebSocket 都要求正式管理会话。
- 状态变更类请求依赖受控 Token 鉴权，不把 Cookie 或来源页检查当作唯一保护。
- 登录失败次数受限，避免暴力猜解管理员凭据。
- 远程访问场景优先通过 HTTPS 反向代理暴露，不开放无鉴权调试入口。
- 第三方插件不具备管理权限，插件能力与管理权限保持隔离。

| 暴露等级 | 典型场景 | 监听方式 | 初始化接口 | 风险提示 |
| --- | --- | --- | --- | --- |
| `localhost_only` | 默认本机部署 | `127.0.0.1` | 允许，仅本机 | 默认模式 |
| `lan_enabled` | 家庭或可信局域网 | 用户显式改为内网地址 | 默认禁止远程初始化 | 需提示已暴露到局域网 |
| `public_via_reverse_proxy` | 反向代理后公网访问 | 建议服务仍监听本地地址 | 禁止 | 需提示 HTTPS 与公网暴露风险 |

## 初始化与会话

- 首次启动没有管理员账户时，服务进入 `setup_required`，仅开放本机初始化路径。
- 初始化完成前，不开放常规插件管理、配置修改和日志查询。
- 管理员凭据丢失时，正式恢复路径是停服后通过 Launcher 或 CLI 触发 `reset-admin`。
- 管理会话采用有限 TTL，可做滑动续期，但保留绝对有效期上限。
- 重置管理员凭据后，旧会话与一次性 Launcher token 全部失效。
- Launcher 自动登录失败、Token 过期或 admission 校验不通过时，管理面回到初始化页或登录页，并显示可读提示。

## Launcher

- full artifact 统一以 Launcher 作为桌面入口。
- Launcher 负责启动、停止、重启服务、本地预检、打开管理面和版本提示。
- Launcher 本地预检关注安装根、`raylea-server` 路径、`config/user.yaml`、工作目录和启动器设置。
- Chromium、Python、Node.js、模板资源和运行态异常由服务端 readiness、恢复摘要与诊断结果统一裁决，Launcher 直接展示这些结果。
- 启动时若本机已存在健康服务但不属于当前 Launcher 子进程，界面会标示“检测到现有服务”。
- 启动失败摘要来自健康探测、stderr 和日志尾部。
- 关闭窗口默认隐藏到托盘，不直接结束后台服务。
- 仅在用户明确选择完全退出时，Launcher 才触发优雅停机流程。
- 若服务已完成初始化，Launcher 可用一次性 token 做最佳努力自动登录。

## 入口职责矩阵

| 场景 | Web 管理面 | Launcher | CLI |
| --- | --- | --- | --- |
| 在线状态查看 | 主入口 | 桌面壳视图 | 否 |
| 在线插件管理 | 主入口 | 打开 Web | 否 |
| 首次初始化 | 主入口 | 打开 Web | 否 |
| 凭据丢失恢复 | 否 | 可触发本机恢复入口 | 主入口 |
| 在线备份与诊断导出 | 正式入口 | 否 | 可补充离线入口 |
| 停服恢复与迁移 | 否 | 否 | 主入口 |

## 相关文档

- [Configuration](./configuration.md)
- [Recovery](./recovery.md)
- [CLI](./cli.md)
- [Deployment](./deployment.md)
