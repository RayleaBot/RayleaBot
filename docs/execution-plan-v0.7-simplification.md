# RayleaBot v0.7 过度设计精简执行计划

## 文档状态与范围

- 目标版本：v0.7；仓库基准 `47a2178f`，2026-09-13。v0.6 会话计划已全部完成并移除。
- 依据：2026-09-13 全仓过度设计审查。插件按“完全可信的本地原生代码”运行，宿主不再围绕它重复设防；前提已经消失的机制直接删除；工程检查只保留能发现真实回归的部分。
- 范围：Core、Launcher、Web、Go 与 Vue SDK、仓库内示例、契约、CI 与文档。一工作项一提交。独立插件仓库按新 SDK 适配不在本计划内，发布说明写明协议变化。
- 正式语义以 [contracts](../contracts/README.md) 为准；删除对外语义同样先改契约和 fixtures，再删实现与生成物。
- 已处理、不再列入：发布清单宽松读取、恢复写入不存在的目标且无文件回滚、旧结构恢复演练、核心更新不加摘要、签名、回滚与独立更新程序（理由见 [Delivery and Upgrade](release/delivery-and-upgrade.md#更新策略与理由)）。

## 一、宿主内部精简

本组不改变插件协议。涉及 Web API、CLI 或配置的删除，同步对应契约与 fixtures；配置加载会忽略已删除的键，配置版本保持 `4`。

### R1 恢复流程

现状：恢复前做兼容性评估，版本不兼容的插件生成 review ID，需要逐条人工确认并记录确认人、备注和审计历史；重新检查与确认各是一个异步任务；恢复摘要在 CLI、Web、Launcher 和诊断导出四处展示。

决定：恢复只校验归档完整性、配置版本与数据库版本是否可迁移，解压到不存在的目标后结束。与当前核心不兼容的插件启动后按普通插件状态显示为不可用，不需要人工确认。删除 review ID、确认记录、审计历史、`/api/system/recovery/recheck` 与 `/api/system/recovery/confirm` 及其任务、恢复摘要文件和四处展示。恢复结果由 CLI 输出，源与目标数据库版本写入启动日志。

### R2 Launcher 响应校验

Launcher 与 Server 同包发布，版本一致。Launcher 解码 Server 响应时忽略未知字段，只读取实际使用的字段。删除 `server_validation.go`、依赖清单的 schema 校验，以及 `scripts/generate-launcher-api.py` 生成的 Go 模型与 TypeScript 子集，改为手写所需结构。

### R3 OneBot 兼容矩阵

管理面不再展示兼容矩阵。删除 `/api/protocols/onebot11/compatibility`、`server/internal/bot/adapters/protocol_compatibility.go` 中写死的矩阵数据、Web 兼容面板和发布包中针对该接口的 smoke 检查。矩阵内容只保留在开发文档 `docs/dev/onebot-compatibility.md`，由开发文档索引提供入口；界面规范、工程基线与质量门禁中关于兼容矩阵的描述同步删除。

### R4 运行指标

删除 `/api/system/metrics` 与 Prometheus 依赖；Web 与 Launcher 均未使用该接口。保留管理面使用的 WebSocket 运行统计。

### R5 出站熔断与限流

删除按目标熔断与 `message.circuit_breaker_seconds`。连续发送失败通常是适配器断线，熔断只会让恢复后的消息继续被拒。限流只保留按目标一层，删除 `message.rate_limit_per_plugin`；平台风控对单个会话刷屏最敏感。

### R6 错误码本地化键

删除错误码目录中的 `message_key`、错误信封的同名字段和 Web 端映射。界面只有中文，服务端消息本身就是中文；程序分支继续依赖 code。

### R7 密钥加密层

删除 AES 加密与 `platform.secret_encryption_key`；加密密钥与密文存于同一存储，这一层不提供保护。保留独立 secret 存储、配置引用与管理面不回显。

数据库新增 `000002 → 000003` 迁移：解密已有密文并删除密钥行。迁移表允许 Go 步骤，仍在单个事务内执行；恢复接受 `000001` 至 `000003`，备份清单的数据库版本同步加入 `000003`。

### R8 插件通信校验与限流

宿主只对 `plugin dev-sync` 安装的开发插件逐帧执行 JSON Schema 校验，便于插件开发者发现协议错误；正式安装的插件和 Go SDK 运行时不再逐帧校验，协议形状由双端生成代码、SDK 测试与 fixtures 保证。运行时保留帧大小上限与 stderr 截断。

删除 `runtime.ipc_action_burst_limit`、`runtime.ipc_pending_actions_max`、`runtime.plugin_init_max_total_seconds` 与 `log.rate_limit_per_plugin`。此前“保留出站校验”的决定由本项取代。

### R9 安装流程

检查、确认与安装合并为一次请求。Web 在提交前展示“完全可信本地代码”确认；服务端校验 ZIP 路径、单根目录、manifest 与目标平台后直接安装，入口文件的可执行位由安装器设置。

删除 10 分钟检查时效、`/api/plugins/install/inspect`、`plugin.install_inspection_required`、`plugin.install_inspection_expired`、`plugin.install_digest_mismatch`，以及 PE、ELF、Mach-O 格式与可执行位嗅探。商店安装的确认原因只保留首次安装与来源变化。

## 二、插件协议 v4

本组把插件协议与 manifest 升为 v4，退役管理页 bridge v3。沿用 v0.5 做法，不保留 v3 插件兼容路径：已安装的 v3 插件显示为合同不受支持，使用新 SDK 重新构建后安装。新插件声明 `min_core_version >= 0.7.0`。

### P1 宿主权限

删除 manifest 的 `permissions`、全部权限名、本地动作权限检查、`plugin.permission_denied`、`init.effective_permissions`、商店确认原因中的权限扩大，以及 Web 与 SDK 的权限展示。架构文档已写明权限不是 OS 沙盒，参考项目也没有插件权限体系。

### P2 宿主代理的 HTTP 请求

删除 `http.request`、SDK helper 与只服务该动作的 `http` 配置段；插件使用自己的 HTTP 客户端。`render.image` 的请求级资源预取保留超时与总量限制，去掉私网拦截。

### P3 宿主代理的文件存储

删除 `storage.file`、`storage.file_max_bytes` 与 `storage.plugin_workdir_soft_limit_mb`。宿主以环境变量 `RAYLEABOT_PLUGIN_DATA_DIR` 传入该插件 `data/plugins/<plugin_id>/` 的绝对路径，与 FFmpeg 路径的注入方式一致；插件直接读写，备份继续包含该目录。

### P4 管理页同源加载

插件管理页改由管理面同源路径提供，仍以 iframe 嵌入。同源后剩下两个真实风险：

1. 插件页从外部 CDN 加载的脚本被投毒；
2. 插件页把聊天内容当 HTML 直接渲染，引出 XSS。

插件页响应返回一条 `Content-Security-Policy` 头：`script-src` 只允许该插件包 UI 路径，不允许内联脚本与 `eval`，并设置 `object-src 'none'` 与 `base-uri 'none'`；图片与样式按实际需要放宽。插件 UI 路由不做重定向，避免 CSP 路径匹配失效。

删除独立来源模板 `web.plugin_ui_origin_template`、按来源隔离的 cookie 策略、一次性 nonce 与 MessageChannel 握手、`plugin-management-ui-bridge.schema.json`。`@rayleabot/plugin-ui` 改为同源直接调用管理 API，主题读取宿主 CSS 变量。`plugin-management-ui.yaml` 只保留静态资源路径与 CSP。

### P5 Webhook

宿主只按 route 转发。删除 manifest webhook 中的 `auth_strategy`、`header`、`secret_ref`、`replay_protection`，以及 `plugin.webhook_replay_rejected` 与 `plugin.webhook_timestamp_skew`。webhook 事件携带请求头与原始正文，插件按对方平台规则自行验签；删除事件中的 `client_timestamp` 与 `client_event_id`。

## 三、工程治理、前端与文档

### G1 契约校验范围

删除工程基线文档中版本字符串的比对，版本一致性由 `.tool-versions`、工具链检查与 doctor 保证。删除“每个 OpenAPI 操作必须有样例”及 `x-fixture-exemption`。删除 CLI 契约中的 availability、task_model、cancellable 与 implemented 元数据。同步 `contracts/AGENTS.md` 的配套登记。

### G2 仓库自检

删除 agent 文档行数预算，并同步根 AGENTS 对检查脚本的描述。删除 `docs/engineering/manual-sql-exceptions.json` 及结构检查中的对应校验，手写 SQL 的原因写在代码注释中。修正变更检测脚本对已删除 `design-qa.md` 的引用。

### G3 CI 工作流

删除 `self-host-smoke.yml`、`scripts/release/self_host_smoke.py` 与 `artifact-validation.yml`。恢复验证只保留 nightly 的 `rehearse_current_recovery.py`，`release.yml` 删除打包恢复演练。

### G4 设计规格

`docs/design/web-management-ui.md` 与 `DESIGN.md` 删除像素值与逐控件尺寸，只写布局与交互原则；尺寸以组件代码为准。设计 token 生成器删除全仓颜色字面量扫描与允许清单；`DESIGN.md` 头部与 Impeccable 设计文件继续生成，供设计工具读取。

### G5 Web 与 Launcher 端到端测试

按根指令，视觉细节不写 E2E，交由人工审核。

- Web 保留 real-server 冒烟：登录、插件安装与启停、配置保存、日志查看。删除 ui-fixtures 项目、模拟后端，以及焦点外观、浮动标签、Reka 覆盖层、UI 加固等视觉细节用例。
- Launcher Renderer E2E 删除窗口尺寸下的控件位置、标题字体加载与减少动态效果用例，保留初始化失败流程。
- 同步质量门禁与 Web 端到端验证文档中的覆盖说明。

### G6 插件开发工具

`plugin-development-workspace.schema.json` 移出 `contracts/`，由开发启动脚本自行校验；删除对应 fixtures。插件构建器不再生成 SPDX SBOM。

### G7 文档

`docs/architecture/` 合并为一份概览，只保留组件职责、消息主流程和状态归属。`implementation-order.md` 的发布验收范围并入质量门禁，独立设计边界并入架构概览，删除该文件并更新根 AGENTS 的引用。工程目录删除只复述代码或页面结构的文档，保留工程基线、质量门禁与人工 smoke。用户、插件与发布文档随各工作项同步。

## 四、保留不动

按会话 lane 的 FIFO 与插件并发、跨层传播与阻断、对话会话、KV 与 TTL、SQLite 迁移链、运行环境依赖的镜像选源与 SHA-256、插件商店目录的归档摘要、secret 存储与管理面不回显、ZIP 路径穿越防护、配置热更新、Launcher 一键更新与 `update apply`。

## 五、执行清单

状态：⬜ 待处理、🟡 进行中、☑️ 已完成、❌ 阻塞。已完成项记录提交与验证证据。

| ID | 工作项 | 依赖 | 状态 | 证据 |
| --- | --- | --- | --- | --- |
| R1 | 恢复只做完整性与版本校验，删除人工确认、审计与摘要展示 | — | ⬜ 待处理 | |
| R2 | Launcher 宽松解码，删除响应与依赖清单校验及生成模型 | R1 | ⬜ 待处理 | |
| R3 | 管理面不再展示兼容矩阵，删除接口、面板与 smoke，矩阵只保留在开发文档 | — | ⬜ 待处理 | |
| R4 | 删除 Prometheus 指标接口与依赖 | — | ⬜ 待处理 | |
| R5 | 删除出站熔断与每插件限流 | — | ⬜ 待处理 | |
| R6 | 删除错误码本地化键与 Web 映射 | — | ⬜ 待处理 | |
| R7 | 删除密钥加密层并迁移到 `000003` | — | ⬜ 待处理 | |
| R8 | 逐帧校验仅限开发插件，删除 IPC 与日志限流配置 | — | ⬜ 待处理 | |
| R9 | 安装合并为一次请求，删除检查时效与格式嗅探 | — | ⬜ 待处理 | |
| P1 | 协议与 manifest 升为 v4，删除宿主权限体系 | R9 | ⬜ 待处理 | |
| P2 | 删除 `http.request` 与 `http` 配置段 | P1 | ⬜ 待处理 | |
| P3 | 删除 `storage.file`，注入插件数据目录 | P1 | ⬜ 待处理 | |
| P4 | 管理页同源加载与 CSP，退役 bridge v3 | P1 | ⬜ 待处理 | |
| P5 | Webhook 只做路由转发并携带请求头与原始正文 | P1 | ⬜ 待处理 | |
| G1 | 收窄契约校验范围 | — | ⬜ 待处理 | |
| G2 | 删除行数预算与 SQL 例外登记，修正变更检测 | — | ⬜ 待处理 | |
| G3 | 删除自托管长时 smoke 与产物验证工作流，恢复验证只留一处 | R3 | ⬜ 待处理 | |
| G4 | 设计规格去像素化，删除颜色字面量扫描 | — | ⬜ 待处理 | |
| G5 | 删除 Web 与 Launcher 的视觉细节 E2E，Web 收敛为 real-server 冒烟 | R1,R3,P4 | ⬜ 待处理 | |
| G6 | 开发工作区契约移出，删除 SBOM 生成 | — | ⬜ 待处理 | |
| G7 | 架构与工程文档合并 | R1-R9,P1-P5,G1-G6 | ⬜ 待处理 | |
| E1 | 总验收与发布说明 | 全部 | ⬜ 待处理 | |

R 组与 G1、G2、G4、G6 互不依赖，可先行。P 组集中在一个协议版本内完成，仓库内示例随各项改用新 SDK。

## 六、验证与完成条件

遵循[质量门禁](engineering/quality-gates.md)按改动面选择检查，并补充：

| 领域 | 必须覆盖 |
| --- | --- |
| 契约删除 | strict contracts、生成器 verify、Web 与 Launcher 类型重生成无漂移；被删语义的 fixtures 与示例同步删除 |
| 密钥迁移 | 新建与 `000001`、`000002` 迁移后结构等价；已有密文解密正确；重复启动幂等；旧备份恢复后首次启动迁移并可登录 |
| 恢复与 Launcher | 恢复到不存在的目标后启动；不兼容插件显示为不可用；Launcher 对新增字段不报错 |
| 插件协议 v4 | 仓库内示例全部改用新 SDK 并编译；会话示例原生子进程集成测试通过；数据目录环境变量可读写；webhook 请求头与正文到达插件 |
| 管理页 | 插件页响应带 CSP 头；外部脚本与内联脚本被拒；示例插件页同源加载并调用管理 API |
| 通信校验 | 开发插件发送非法帧被拒并给出诊断；正式插件不逐帧校验；超大帧与 stderr 截断仍生效 |
| Web 与 CI | real-server 冒烟通过；删除的工作流不再被变更检测与必需结果汇总引用 |

全部已交付项附验证证据；未完成项保留状态。用户数据和真实凭据不进入提交。
