# Acceptance and Risks

本页说明 RayleaBot 当前正式验收口径、关键场景和主要风险。

## 当前主要风险

| 风险 | 当前控制方向 |
| --- | --- |
| Chromium 渲染环境缺失、队列拥塞或浏览器超时 | 受控队列上限、执行超时、资源检查和结构化错误 |
| SQLite 锁争用、容器或网络文件系统导致状态库异常 | 本地文件系统要求、短时重试、降级摘要和恢复指引 |
| OneBot11 鉴权失败、链路断连或持续重连失败 | 明确连接状态、退避重连和管理面告警 |
| 插件升级后权限扩大、数据版本不兼容或运行时反复崩溃 | 重确认、兼容检查、`dead_letter` 和人工干预 |
| 运行环境资源缺失、模板资源缺失或恢复后兼容检查失败 | `doctor`、Launcher、管理面和恢复摘要共用同一份问题口径 |

## 验收结论

当前发布门槛不是“开发者环境能运行”，而是用户能按受支持文档完成安装、初始化、插件启用、基础管理、协议接入和恢复排障闭环。

## 核心验收场景

- OneBot11 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook` 都能建立受控链路。
- 鉴权失败进入明确失败状态，而不是静默重试。
- packaged `/api/protocols/onebot11` 能稳定返回四条 transport 状态、provider、readiness 和摘要。
- packaged `/api/protocols/onebot11/compatibility` 能稳定返回 `events`、`message_segments`、`read_capabilities`、`provider_extensions` 四类矩阵和代表项。
- 群聊或私聊消息能够进入插件处理并返回回复。
- `message_sent.private` 与 `message_sent.group` 能进入日志中心、桥接链路和插件协议。
- 协议中心能够显示当前 transport 状态、摘要和最近传输问题。
- 日志中心能够区分本次服务端启动日志与历史日志，并可查看单条日志详情中的摘要字段与脱敏后的 `details` JSON。
- 插件列表和指令中心能够直接显示已声明命令，并按插件筛选查看。
- 插件完成 `init -> init_ack` 握手。
- 插件崩溃后进入 backoff；超过阈值后进入 `dead_letter`。
- 权限授予、重确认、重载和卸载都可追溯。
- 首次启动没有管理员账户时，服务进入 `setup_required`，仅允许本机初始化。
- 用户能完成初始化、登录、查看状态、管理插件、查看日志和编辑配置。
- 插件安装使用异步任务模型，管理面可持续看到阶段、输出摘要和最终结果。
- Launcher 能启动 / 停止服务、检查环境并打开管理面。
- Launcher 检测到现有服务、端口占用、启动失败或自动登录失败时，都会给出可读提示。
- Web UI、CLI 和 Launcher 不形成第二套状态源。
- 官方模板可通过统一渲染链路输出图片。
- 渲染队列已满或 Chromium 缺失时返回结构化错误。
- 运行环境资源缺失时，`doctor`、Launcher 和管理面给出同一份失败摘要。
- 相同模板与相同数据可命中缓存，渲染失败时仍保留文本降级路径。
- `backup`、`restore`、`doctor`、`migrate`、`reset-admin` 至少具备一条受支持执行路径。
- 恢复流程遵循停服、导入、迁移、兼容检查和人工处理摘要路径。
- `/healthz`、`/readyz`、诊断导出和恢复摘要能支撑基本排障闭环。
- 恢复后发现插件或数据不兼容时，平台保留插件包与业务数据，但阻止自动启用。
- 安装缺失 manifest、字段不合法或版本不满足要求的插件时，平台直接拒绝安装，不留下半完成目录。
- 管理员重置后，旧管理会话和一次性 Launcher token 全部失效。
- 配置或数据库迁移失败时，服务不会进入 `running`。
- 默认配置下，管理接口以 loopback 为主，API 与 WebSocket 都要求鉴权。
- packaged 模板预览工作区支持列表、模板详情、输入结构、自动预览、artifact 和任务详情跳转。
- 正式发布包通过 release metadata 校验、packaged recovery drill 和长期自托管 smoke。

## 当前边界

- 多协议、多实例和平台级 LLM 能力不属于当前正式验收范围。
- 自动覆盖更新、插件市场和强沙盒不属于当前正式交付范围。
