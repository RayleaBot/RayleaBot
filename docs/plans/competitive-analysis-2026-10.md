# RayleaBot 竞品分析（2026-10-04）

状态：评估结论；G1 的发布门禁、通道与失败跟踪已落实，公开发布及实包验收仍待执行，见 G1。其余条目尚未进入实施。维护者决定采纳的条目按本目录规则转为执行计划，落地并写入现行文档后删除本文件。原始报告由多代理只读分析与联网调研产出，未运行构建、测试或任何竞品；未另标状态的仓库事实以文中引用的文件为准，基线为主仓库 `9ac71c35` 与同级 RayleaBotPlugins 工作区。竞品数字是 2026-10-04 至 10-05 的时点数据。2026-10-05 与两份同期独立竞品分析（astra、DeepSeek）逐条交叉核实，成立的内容已并入正文，被推翻的说法列在第 10 节；G78 及之后的编号与 T14–T16 为这一轮新增。

## 总体结论

RayleaBot 当前最接近“Yunzai 游戏生态的工程化替代”：只接 QQ（OneBot11 与 QQ 官方），插件是 Go 原生子进程，配 Web 管理面、桌面 Launcher 和契约先行的工程体系。功能赛道上的直接对手是 Yunzai 系（Miao-Yunzai、TRSS-Yunzai 加 miao-plugin）和 gsuid_core；“群主默认会选谁”这一层的对手是 AstrBot。NoneBot2 和 Koishi 是面向开发者的多平台框架，LangBot 面向企业 IM 与 LLMOps，三者都不在同一赛道正面竞争。

与竞品相比，RayleaBot 当前最大的差距不在功能，而在交付。公开可下载的核心停在 v0.3.1（2026-08-16）。官方插件目录的 6 个条目都要求 0.4.0 且是 manifest v3 包：v0.3.1 读不了 catalog v2，v0.4.0 宿主又会拒绝 v3 包。因此此刻不存在任何能配套使用的“公开核心 + 目录插件”组合。SDK 的 tag 停在协议 v3，nightly 自 2026-09-14 起连续失败。其次是 QQ 官方路径的可用性，三个缺陷叠加：群管理员开启“接收全部群消息”后整群可能收不到消息；插件回复在 QQ 官方上全部按主动推送发出；`base64://` 图片和 at、reply 段会失败或只发出一半。官方游戏插件在 QQ 官方上基本跑不通。这两组问题定为 P0，修复多为 S 到 M 级。

产品面最明显的缺口依次是：没有任何 AI 能力（是本轮对比中唯一一个，核心和官方插件都没有）、不能按群启停插件、插件配置没有自动表单、指令必须带前缀、没有官方 Docker 与 ARM64 Linux 产物、缺少协议端接入教程和社区渠道。上手路径也断在中间：初始化后直接进入状态页，没有“连接 → 装插件 → 首次收发成功”的引导；一条消息没有回复时，入站和策略拒绝日志没有关联 ID，前缀不匹配等最常见的原因只在 debug 级记录。插件数量与 Koishi（4,722）、AstrBot（约 2,385）、NoneBot2（942）相差两到三个数量级，表情包、点歌、群统计、群管这几类完全空白。

RayleaBot 的优势真实存在，但必须限定口径。插件子进程只隔离崩溃，不构成安全边界，也没有资源上限；LangBot 和 MaiBot 同样是每插件一个进程，LangBot 自托管版同样不限制插件资源。其他优势包括：自包含原生包、原子安装事务、契约化的失败语义、宿主内置的模板渲染、由宿主认定调用方的服务调用、探针与严格恢复、默认安全的管理面认证，以及游戏和订阅领域的凭据脱敏。这些相对同进程的 Yunzai、NoneBot2、Koishi、AstrBot 成立，但大多数用户目前拿不到，因为 0.3.1 之后的候选都没有公开发布。

校验推翻或收窄了一批说法：“OneBot11 接入最完整”“AI 风险被限制在插件进程内”“本地渲染独有”“宿主不记录消息正文”“热重载不打断会话”“日志用 request_id 串起整条消息”“竞品都没有健康探针”，以及“游戏插件可以把需要 CK 的数据暴露为 AI 工具”（服务调用只允许一跳）。核实中还发现一个跨条目的约束：manifest 顶层与宿主对插件动作数据都是严格解码，商店目录读取也是严格的。v0.4.0 公开发布前是加入可选字段成本最低的窗口，见 4.4 节。

| 项目 | 数量 |
|---|---|
| 主要竞品 | 5（AstrBot、Yunzai 系、NoneBot2、Koishi、LangBot） |
| 补充画像 | gsuid_core、MaiBot、ZeroBot-Plugin 等约 20 个项目，另含协议与生态态势 |
| 候选结论 | 130（7 个维度簇 114 条，完整性审查补充 16 条） |
| 核实票 | 260（仓库视角与市场视角各 130 张） |
| 交叉核实 | 两份独立报告的 45 项说法（仓库 14、竞品 22、产品判断 9） |
| 合并后条目 | 缺口 83（P0 3、P1 20、P2 42、P3 18），有意取舍 16，优势 11 |
| 核实中下调优先级 / 上调 | 44 票 / 5 票 |

## 1. 对比对象与生态态势

### 1.1 主要竞品

| 项目 | 技术栈与许可 | 最新版本（截至 2026-10-04） | 热度与生态 | 定位 |
|---|---|---|---|---|
| AstrBot | Python 3.12，同进程插件；AGPL-3.0 | v4.28.2（2026-09-27），v4.29.0-beta.1（2026-10-01） | 41.4k stars；云市场约 2,385 个插件（市场接口口径，README 徽章的 1,329 是另一计数器）；约两周一个 minor | Agent 为核心的一体化 IM 机器人平台；17 种可创建的平台类型（16 种外部接入，QQ 官方分 WebSocket 与 Webhook），README 标为官方维护 14 项另有 3 个社区适配器 |
| Yunzai 系 | Node.js，同进程 JS 插件；GPL-3.0 | 无 Release，版本号停在 3.1.3；TRSS 最后推送 2026-09-25 | Miao-Yunzai 1,074 stars，TRSS 625，miao-plugin 1,587（2026-10-04 仍有提交）；插件索引约 385 条（索引仓库 2026-09-29 仍有提交）；原版 GitHub 仓库已被平台禁用 | 米哈游游戏查询为核心的 QQ 机器人家族。TRSS 核心内置 OneBotv11、Milky（2026-03 起）、Satori（2025-09 起）、GSUIDCore 等适配器，ICQQ、QQBot、微信、KOOK、Telegram、Discord 以独立插件接入；要求 Node ≥ 23.11 与 Valkey，有 Windows 安装程序（最新 2026-09-10） |
| NoneBot2 | Python，同进程插件；MIT | v2.5.0（2026-04-01），之后 6 个月未发版 | 7.7k stars；商店 942 个插件（616 个通过测试）；32 个适配器（15 个官方） | 面向开发者的跨平台异步框架；核心无 WebUI（官方可选的 nb-cli-plugin-webui 停在 0.4.2，2024-04）、无 AI |
| Koishi | TypeScript，cordis 上下文，同进程插件；核心 MIT，控制台 AGPL-3.0 | 4.18.11（npm 2026-02-27），核心进入低频维护 | 6.2k stars；市场 4,722 个插件（月下载 ≥100 的 152 个） | Satori 协议的跨平台框架，控制台插件市场与配置表单成熟 |
| LangBot | Python，每插件独立进程；主仓 Apache-2.0，插件运行时 AGPL-3.0 | v4.10.11（2026-09-12），v4.11.0-beta.6（2026-10-01） | 18k stars；LangBot Space 市场 96 个插件（另有 215 个 MCP、146 个 Skill 条目）；补丁约每周一个 | LLM 原生的企业 IM 机器人，接 Dify、Coze、n8n 等 Runner |

### 1.2 补充对象

| 项目 | 与 RayleaBot 的关系 |
|---|---|
| gsuid_core（早柚核心） | 同赛道最直接的对手（GPL-3.0；pyproject 版本 0.11.0，README 仍写 0.10.7，没有正式发布）。原神、星铁、绝区零、鸣潮等游戏插件以独立仓库挂接 NoneBot2、Koishi、Yunzai、AstrBot，也能经 NapCat 插件直连；插件列表已有 AI 对话、表情包（core_plugin_memes）与三平台点歌（MusicUID）。2026 年加装基于 PydanticAI 的 ai_core：人格、四类作用域记忆、RAG、MCP 客户端与可选 MCP server、Skills、按会话 token 预算，控制台有 AI 统计页。网页控制台有日志关键词搜索、定时备份与下载、按插件和服务的黑白名单。默认只监听 localhost，非可信来源连接必须带 WS_TOKEN，令牌为空即拒绝。Docker 分挂载与全量两种模式，镜像内置 Playwright 与 Chromium，支持 amd64/arm64，托管在国内源 |
| MaiBot | 架构最接近：插件跑在独立 Runner 子进程（msgpack RPC），热重载先校验再切换，Pydantic 配置自动生成表单；2025-02 创建，6.1k stars，插件登记 408 条，主打拟人化群聊 |
| ZeroBot-Plugin | 同语言（Go）参照：编译期打包 108 个插件，按群、用户、全局三层启停插件；Release 提供 linux arm64、armv6、armv7 与 deb、rpm |
| HoshinoBot、Mirai、Kirara AI | 老牌或已停更（Mirai 最后 Release v2.16.0 发布于 2023-10-22，最后提交 2024-09-23），主要参考价值在 Service 权限与按群启停模型 |
| CowAgent、OpenClaw、Nekro Agent | AI Agent 方向；OpenClaw 已获 QQ 开放平台与微信 ClawBot 官方接入，其 ClawHub 审计出数百个恶意 skill |

### 1.3 2025–2026 生态态势

- **OneBot11 仍是 QQ 个人号的事实标准**：NoneBot 的 OneBot 适配器月下载 27,762，Milky 适配器 836。NoneBot 商店中显式声明支持 OneBot V11 的插件 594 条、QQ 官方 221 条、Milky 147 条（另有 336 条不限适配器）；计入不限适配器的条目后，QQ 官方路径可用的插件约为 OneBot V11 的六成（557 对 930）。协议端集中在 NapCat（最新 v4.18.30，2026-10-05）、LuckyLilliaBot 和 2025 年出现的 SnowLuma。Lagrange 放弃 OneBot11 改做 Milky，LagrangeV2 于 2026-07-23 归档。NapCat 以 not planned 关闭了 Milky 支持请求（#1024）。
- **协议端暴露事件**：2025-09-05，攻击者针对公网暴露、未设令牌的 OneBot 服务批量调用 send_msg，让机器人发布违规言论；涉事账号和群被永久封禁。协议端开发者 Wesley Young（Milky 与 Acidify 开发者、LagrangeV2 维护者）的复盘博文指出，攻击与具体实现无关，受害者多为 NapCat 用户，部分原因是 NapCat WebUI 当时默认把 OneBot 服务绑定到 0.0.0.0。博文没有给出受影响规模。默认安全配置因此成为用户能感知的卖点。
- **QQ 官方开放平台明显放开**：2026-03 起个人可扫码建 Bot（每个 QQ 号最多 5 个）。2026-06-22 群内主动推送全量恢复，需群管理员打开开关；配额为认证 60 条/分钟、未认证 30 条/分钟、每群每天 1000 条。官方事件页“群消息（全量模式）”写明：开启接收所有消息后，群内每条消息（含 @机器人）都推送 GROUP_MESSAGE_CREATE，与 @ 事件共用 GROUP_AND_C2C_EVENT intent。原生 Markdown 已对所有机器人开放。单聊流式消息只能用于被动回复。群管接口分两批：2026-08-10 新增禁言、入群申请审批、入群自动审批策略与 GROUP_JOIN_REQUEST 事件，文档未标内邀，只要求机器人是群管理员；2026-09-03 新增成员列表、成员信息、批量移除与黑名单，页面标注“正在内邀接入”，未开通返回 11253。AstrBot（v4.26.0 起）与 LangBot 已能在 WebUI 扫码绑定 QQ 官方机器人并自动回填 AppID 与 Secret。
- **微信**：2026-03 推出官方个人号 Bot 接口（iLink/ClawBot），只支持私聊；itchat、gewechat 等非官方方案失效。
- **需求面**：AI 是增长最快的类别。AstrBot 市场下载前列被 AI 伴侣和记忆类占据：self_learning 约 2.0 万、proactive_chat 约 1.2 万、livingmemory 约 1.2 万。稳定刚需包括表情包（meme_manager 约 1.8 万）、链接解析（约 7.7k）、点歌（约 7.5k）、群聊日报（约 6.6k）、群管（约 4k），以及游戏数据查询和订阅推送。Koishi 市场 ai 分类有 409 个包，月下载合计约 1.1 万，其中 ChatLuna 系占约 9.3k（registry 口径，约等于 npmmirror 下载，与 npmjs 官方统计不是同一口径）。NoneBot 下载最多的是基础设施类插件：本地存储、定时任务、跨平台消息、HTML 渲染、ORM。
- **安全事件**：AstrBot 仓库自行发布 2 条安全公告，其中包括硬编码 JWT 导致的 RCE（CVE-2025-55449，Critical，3.5.18 修复）；GitHub Advisory Database 检索 astrbot 共 24 条（8 条已审核、16 条未审核，20 条来自 VulDB，16 条为 Low），不能读成项目自认的漏洞数。Koishi 控制台出过无鉴权任意文件读取（2025-11-17 修复），市场出现过窃取消息的恶意插件。LangBot 一年内 4 条公告，其中 MCP stdio 认证后 RCE 为 CVE-2026-54449。同进程插件生态接连出事，进程隔离和凭据脱敏因此是可感知的差异点；但措辞必须准确，见第 8 节。

## 2. 能力对照

“核心”指框架本体自带；“官方插件”指官方组织维护的可选插件；“社区”指第三方插件。

| 能力 | RayleaBot | AstrBot | Yunzai（TRSS） | NoneBot2 | Koishi | LangBot | gsuid_core |
|---|---|---|---|---|---|---|---|
| 内置平台 | 2（OneBot11、QQ 官方） | 17 种平台类型 | 核心与插件十余种（含 Milky、Satori） | 32 个适配器（15 个官方） | 15 个官方，OneBot 为社区 | 21 个适配器配置 | 不直连，挂接宿主 |
| 插件语言与加载 | Go 原生子进程 | Python 同进程 | JS 同进程 | Python 同进程 | TS/JS 同进程 | Python 独立进程 | Python 同进程 |
| 插件市场规模 | 目录 6 条，v0.4.0 可装 0 | 约 2,385 | 索引约 385 | 942（616 可用） | 4,722 | 96 | 文档列表 41–52 项 |
| 插件热重载 | 进程级零间隙 | 核心 | 仅单文件 JS | 整进程重启 | 核心 HMR | 远程调试重载 | 说法不一 |
| 插件配置自动表单 | 无（需插件自写 Vue 页） | 核心 | 锅巴插件 | 无 | 核心（Schemastery） | 核心 | 核心控制台 |
| 按群启停插件 | 无 | 按配置文件路由会话 | 核心 | 社区 | 核心过滤器 | 按流水线绑定 | 核心（服务级黑白名单） |
| @机器人免前缀 / 仅响应@ | 无 | 未核实 | 核心 | 核心 to_me | 核心 | 核心（群响应规则） | 未核实 |
| 聊天沙盒 | 无 | ChatUI | stdin 适配器；社区面板沙盒 | 终端 Console 适配器（`nb create` 可直接选） | 官方插件（模板默认启用） | 对话调试 | 未核实 |
| 首次使用引导 | 无（只有“添加机器人连接”空状态） | 首次登录后三步快速引导；QQ 官方扫码绑定 | 终端交互 | `nb create` 交互式脚手架 | 无向导，靠默认沙盒与文档 | 四步向导（master 分支）；QQ 官方扫码绑定 | 无（注册码登录） |
| 单条消息追踪 | 按 request_id 筛出单个插件的处理链；入站与策略拒绝没有关联 ID | Trace 页（只覆盖部分 Agent 调用） | 未见 | 未见 | 未见 | 监控中心（模型调用、token、工具调用） | 未见（控制台无链路视图） |
| LLM / Agent | 无 | 核心（多提供商、MCP、知识库、Skills） | 社区（chatgpt-plugin） | 社区 | 社区（ChatLuna） | 核心（LiteLLM 统一后端，文档列 24 个内置 LLM 供应商；MCP、插件化 RAG） | 核心 ai_core（PydanticAI） |
| HTML 模板渲染 | 核心（托管 Chromium） | 远程 t2i 服务（可自托管） | 核心（puppeteer、shotium） | 社区 htmlrender | 官方插件 puppeteer | 只有长文本转图 | PIL 与 Playwright |
| 定时任务 | 核心（分钟级 cron，无一次性） | 核心（面向 Agent） | 核心 cron | 官方插件 apscheduler | 核心计时器，cron 靠插件 | 无，靠插件 | APScheduler |
| 数据库 | SQLite | SQLite | Redis 必需加 SQLite | 官方 ORM（SQLite、PostgreSQL、MySQL） | minato（多后端） | SQLite、PostgreSQL | SQLite |
| 官方 Docker | 无 | 有（amd64/arm64） | 脚本含 Docker | nb-cli 生成 | 有（amd64/arm64） | 有（Compose、K8s） | 有（amd64/arm64） |
| ARM64 Linux | 无 | Docker | Termux 与脚本 | 随 Python | Docker | Docker（发布标签 amd64/arm64） | Docker |
| 桌面客户端 | Launcher（Win、Linux、macOS arm64） | Tauri 桌面版与启动器 | Windows 安装程序 | 无 | Koishi Desktop（x64） | 无 | 无 |
| 应用内更新 | Launcher 一键（尚无真实用户走通） | WebUI 一键 | `#更新`（git） | pip / nb-cli | 控制台依赖管理 | 重拉镜像 | git |
| 健康探针与恢复 | healthz/readyz、严格恢复、恢复演练 | WebUI 备份恢复，无探针 | 无 | 无（插件） | 无 | `/healthz`（K8s 清单已配）；无一键备份，只在破坏性迁移前自动备份 SQLite | 控制台定时备份 |
| 日志关键词检索 | 无（按字段筛选） | 对话记录 | 社区面板 | 无 | 弱 | 对话记录 | 核心 |
| 多语言界面 | 仅中文 | 4 种 | 中文 | 文档仅中文 | 核心 i18n | 8 种 | 中文 |
| 2FA / 长期 API Key | 无 | TOTP、带 scope 的 API Key | 无 | 核心无管理面；官方 CLI WebUI 插件低活跃 | 控制台鉴权 | API Key、Workspace 角色 | 单管理员 |

## 3. 优先级总表（P0 与 P1）

P2 见第 6 节，P3 与有意取舍见第 7 节。工作量取核实后的估计。

| 编号 | 主题 | 优先级 | 工作量 | 一句话理由 |
|---|---|---|---|---|
| G1 | 核心公开发布链：v0.3.1 之后无公开版本，nightly 连续失败 | P0 | M | 所有优势都拿不到；发布工作流也没有 prerelease 支持 |
| G2 | 插件目录与 SDK 断档 | P0 | M–L | 当前没有可配套的“公开核心 + 目录插件”组合 |
| G3 | QQ 官方路径：全量群消息、回复按主动推送、消息段失败 | P0 | S–M | 正式适配器在常见配置下基本不可用 |
| G4 | 接入与上手文档 | P1 | S–M | 首次部署最容易在协议端对接和 setup token 处流失 |
| G78 | 首次使用任务清单 | P1 | S–M | 初始化后没有“连接 → 装插件 → 首次收发成功”的引导；只改 Web |
| G5 | OneBot 出站媒体用本地路径 | P1 | S–M | NapCat 在容器或其他主机时，图片卡片全部发不出 |
| G6 | 指令必须带前缀，没有 to_me 语义 | P1 | M | Koishi、NoneBot2、TRSS 核心都有；多机器人同群会重复响应 |
| G7 | 不能按群启停插件 | P1 | M–L | 同赛道 QQ 框架核心标配；非命令消息也缺宿主准入 |
| G8 | 插件配置没有 schema 与自动表单 | P1 | M–L | 群主在 WebUI 上最直接感知的差距 |
| G9 | 没有任何 AI 能力 | P1 | L | 唯一无 AI 的项目；以官方插件落地 |
| G10 | OneBot 入站令牌可留空，叠加候选版默认 0.0.0.0 | P1 | S–M | 可伪造超级管理员指令；v0.4.0 发布前修 |
| G11 | 插件挂起无法被发现 | P1 | S–M | ping/pong 只写在契约里，宿主没有实现 |
| G12 | 插件合同缺稳定承诺 | P1 | S | 开放第三方前的前提 |
| G13 | 社区与可信度信号 | P1 | S | 所有同类项目都有交流渠道与上手材料 |
| G14 | 宿主已匹配命令却不下发 command_id | P1 | S | 三个游戏插件各自重放匹配 |
| G15 | 调度器缺一次性任务与语法说明 | P1 | M | 每分钟轮询作业会挤满队列被丢弃 |
| G16 | 国内网络：FFmpeg 单源阻塞首启，插件下载不走代理 | P1 | M | 首启可能被反复强杀 |
| G17 | 无头服务器初始化断链，systemd 示例以 root 运行 | P1 | S / M | 按现有文档走 systemd 会卡在初始化 |
| G18 | 桌面缺开机自启与服务崩溃拉起 | P1 | S–M | 7×24 挂机用户普遍期待 |
| G19 | 没有官方 Docker | P1 | M–L | NAS 与面板用户的默认路径；需单独设计 |
| G20 | 官方插件覆盖：表情包、点歌、群统计、群管空白 | P1 | L | 竞品市场里的高频类别 |
| G21 | 日志不能按关键词检索与导出 | P1 | S | gsuid 与 Yunzai 社区面板都有，成本低 |
| G79 | 单条消息处理链路无法串联 | P1 | S–M | “没回复”时入站与拒绝日志没有关联 ID，最常见的原因只在 debug 级 |

## 4. P0：发布前必须解决

### G1 核心公开发布链

**状态（2026-10-06）**

发布工作流已要求标签所指完整提交 SHA 的最新 nightly 成功，并在上传前重新检查；失败跟踪由独立的 `nightly-status.yml` 维护同一个 issue。预发布段决定 beta 通道与 GitHub prerelease，不覆盖 latest；macOS 产物标为 experimental，Server 发布构建固定关闭 CGO。Corepack 安装允许替换 runner 已有的工具入口，golangci-lint 已固定为支持 Go 1.27 的 2.13.0。

操作步骤与发布节奏见[交付与升级](../release/delivery-and-upgrade.md#发布流程与通道)，首个预发布正文见 [v0.4.0-beta.1](../release/notes/v0.4.0-beta.1.md)。0.3.x 与 v0.4.0 不兼容，只提供全新安装说明，不提供旧格式迁移，也不恢复签名资产。

仍需推送待发布提交并对该提交运行远程 nightly，再发布预发布包，按[公开发行物验收](../engineering/manual-smoke.md#公开发行物)登记结果。现行 manifest v4 契约要求最低核心版本至少为 `0.4.0`，因此 `0.4.0-beta.1` 不能安装 v4 插件，只先验收核心安装与初始化。完整插件流程需要满足该版本下限的公开核心与 G2 提供的兼容公开包；工作流与本地测试通过不代表这部分已完成。

**原始问题与仓库证据（2026-10-05）**
- 公开 tag 只有 v0.3.0、v0.3.1（2026-08-16）。此后的候选曾以 0.5.0、0.7.0 编号，均未公开分发，现统一为下一版本 v0.4.0（`docs/release/notes/v0.4.0.md`）。选型复核记录的 Release 下载量：Windows 约 4 次，macOS 0 次。
- v0.3.1 的更新器要读取签名资产 `release_manifest.v2.sig.json`，v0.4.0 已删除签名体系。如果 v0.4.0 成为 GitHub 的 latest，v0.3.1 检查更新会返回 `release.manifest_invalid`；以 prerelease 发布时，`/releases/latest` 仍指向 v0.3.1，旧用户只会看到“已是最新”。
- `.github/workflows/release.yml` 原先没有 prerelease 参数，release-build 也不传 channel，推送 `v0.4.0-beta.1` 这类 tag 会生成非 prerelease 的 Release 并成为 latest；`9a211036` 已按标签设置 `prerelease`、`make_latest` 与 beta 通道。
- nightly 自 2026-09-14 起连续 20 余次失败。最近一次运行的失败点是 server race 测试、server-windows 的 Go 测试、web 生产 E2E 和 Windows 上的 Corepack 安装；golangci-lint 被跳过（v2.12.2 不支持 Go 1.27），恢复演练所在的 release-dry-run 是成功的。截至 2026-10-05，本地 main 领先 origin/main 37 个提交（origin 停在 2026-10-02 的 `d30c6346`），nightly 测的是旧代码。`release-build.yml` 不跑 Server 的 Go 测试与 lint，nightly 是唯一的完整回归入口。

**竞品对照**
- AstrBot 约两周一个 minor、每周有补丁；LangBot 补丁约每周一个；NapCat 跟随 QQ 版本，常一天多版。NoneBot2 虽已 6 个月未发版，但始终有可安装的稳定版。Yunzai 不发 Release，git 主干滚动。
- AstrBot 的 beta 只体现在 tag 命名和 WebUI 开关上，GitHub 上 `prerelease=false`。RayleaBot 不能照搬：旧更新器读 `/releases/latest`，必须真正勾选 prerelease。

**核实中被更正的事实**
- 原稿写“最近 5 次 nightly 失败”“release-dry-run 需要恢复可信”，实际是连续 20 余次失败，而 release-dry-run 本身成功。
- “两个多月没有公开发布”不成立：截至 2026-10-04，距 v0.3.1 发布 49 天。

### G2 插件目录与 SDK 断档

**现状与仓库证据**
- 官方目录 `plugin-catalog/catalog.json` 共 6 条：echo、fortune、game-guide、subscription-hub、delta-force、oil-price。current_release 都是 2026-09-03 发布、`min_core_version` 0.4.0 的 manifest v3 包，面向当时一批未公开的候选核心。v0.4.0 宿主只接受 manifest v4（`min_core_version` ≥ 0.4.0）。v0.3.1 的契约把 `manifest_version` 固定为 `"2"`、`catalog_version` 固定为 `"1"`，并按 schema 校验目录，所以它既读不了 catalog v2，也装不了 v3 包。v3 包因此被公开核心和 v0.4.0 两头拒绝。
- genshin、starrail、zzz、mihoyo-accounts、roulette 五个插件没有远程仓库，也没有 tag。genshin 的 `docs/acceptance-plan.md` 要求先用真实账号验收再发布。
- sdk/go 的 tag 最高是 v0.5.0，对应协议 v3。已有发布工作流的 6 个官方插件以 `.rayleabot-sdk-ref` 固定主仓库提交，`go.mod` 使用对应的 Go 伪版本；其余 5 个尚未建立远端的插件依赖待发布的 `sdk/go v0.6.0`。`@rayleabot/plugin-ui` 是 private 包。公开 SDK 与公开核心早已错位：v0.3.1 用协议 v1，SDK 的 v0.4.0、v0.5.0 是为未公开的候选核心打的 tag，对应协议 v2、v3；核心 v0.4.0 使用协议 v4，与 SDK v0.4.0 同号不同义。`scripts/release` 与 `.github/workflows` 中都没有打 SDK tag 的步骤。
- 6 个插件仓库的 release.yml 仍是 `GO_VERSION: 1.26.6`，而 `sdk/go/go.mod` 自 2026-10-02 起要求 go 1.27.1；Node 与 pnpm 版本也落后于 sdk/vue 的声明。即使打了 tag，插件 CI 也大概率失败（未实跑）。
- 目录读取是严格模式：catalog schema 的根和条目都是 `additionalProperties:false`，`server/internal/plugins/market/service.go:501-516` 先做 schema 校验，再用 `decodeStrictJSON` 解码。今后给目录加任何字段，已部署的 v0.4.0 核心都会整份校验失败，停在最后一次成功的缓存。发布清单已采用“宽松读取、严格发布”，目录还没有。

**竞品对照**
- 被比较的竞品都通过 PyPI 或 npm 分发 SDK。LangBot 在 2026-09-26 一天内由 PR #2581、#2583、#2584 连续固定 Runtime 依赖的 SDK 版本，其中 #2581 是为兼容已认证插件。这说明 SDK 与宿主版本错位会直接伤及第三方。
- AstrBot 同样只在安装或加载时校验版本，约 2,385 个条目中只有 924 个声明了 `astrbot_version`。“下载后才失败”在同类中常见，本条的核心是“可装数为 0”。

**建议**
1. 发布清单第一项：在实际发布 v0.4.0 的提交上打下一个 SDK tag。`sdk/go/v0.4.0`、`v0.5.0` 已被占用，SDK 无法与核心同号，下一版定为 `sdk/go/v0.6.0`；release_tool 校验该 tag 存在并指向发布提交，官方插件随后把 `.rayleabot-sdk-ref` 与 `go.mod` 改为该版本。打 tag 前先决定 SDK 的许可（G50），tag 打出后该版本的许可就固定了。
2. 同步 6 个插件工作流的 Go、Node、pnpm 版本。
3. 目录读取改为忽略未知字段（契约先行），在 v0.4.0 公开前完成，之后目录才能演进而不破坏已部署核心。
4. `sync_catalog.py` 校验 `manifest_version` 为 4、`min_core_version` 不低于 0.4.0，不满足时只发条目元数据、不发资产。
5. 分批上架：先发 6 个已上架插件的 v4 包。subscription-hub 不需要真实游戏账号验收，可以作为首批。游戏插件随验收进度发布。
6. Vue SDK 不发 npm（选型复核已判保持），在模板 README 写明 CI 按 tag 检出主仓库的方式。服务端把旧合同条目标为 `contract_unsupported` 只用于防御第三方目录，且要改 `compatible` 字段语义，列为 P2。

### G1 与 G2 的验收

自动门禁覆盖归档 smoke、SDK 全新环境安装、E2E 中的本地包安装和 nightly 的本版恢复演练（[质量门禁](../engineering/quality-gates.md)）。从公开下载入口开始的验收已登记在[人工 Smoke](../engineering/manual-smoke.md#公开发行物)，尚未执行；“目录可装数为 0”这类问题不能由本地包测试排除。以下端到端验收是 G1、G2 的完成条件：

- 发布后从公开 Release 页和公开 catalog 出发，在干净目录完成：安装 → 初始化 → 经 NapCat 的 OneBot11 真实命令回复 → 商店安装插件 → 插件更新 → 备份并恢复到空目录。
- Windows 完整包与 linux-x64 server 包走全流程；macOS arm64 标 experimental，只验安装与初始化；QQ 官方按 G3 的决定处理。
- v0.4.0 首次公开时目录里每个插件只有一个 v4 版本，“插件更新”一步在首个插件补丁版上执行，或专门发一个 echo 补丁版来演练。
- 备份恢复可以直接在下载的产物上运行现有的恢复演练脚本。
- 结果按 `manual-smoke.md` 的格式登记时间、提交、平台与观察结果。0.3.x 与 v0.4.0 不兼容，验收采用全新安装。这份记录同时是 G83 推荐组合中“最近验证版本”的来源。

### G3 QQ 官方路径可用性

**现状与仓库证据**
- **全量群消息被丢弃**：`NormalizeDispatch` 只处理 C2C_MESSAGE_CREATE、GROUP_AT_MESSAGE_CREATE 和 8 个成员/推送 dispatch（`server/internal/bot/adapters/qqofficial/events.go:13-25,89-97`）。GROUP_MESSAGE_CREATE 在 `client.go:460-463` 被静默丢弃，没有日志。官方事件页写明，开启接收所有消息后，群内每条消息（含 @机器人）都推送 GROUP_MESSAGE_CREATE；多个实现方实测此时不再收到 GROUP_AT_MESSAGE_CREATE（这一点官方未写），该群可能完全收不到消息。
- **插件回复按主动推送发出**：Go SDK 的 `Send`、`SendText` 发出的 `message.send` 不带 `reply_to_event_id`（`sdk/go/runtime.go:491-519`）。宿主只有在带上 `reply_to_event_id` 时才改走 `message.reply`（`server/internal/plugins/host_action.go:63-67`），否则 QQ 官方适配器按主动推送发送（`server/internal/bot/adapters/qqofficial/outbound.go:78-84`，注释写明受配额限制）。官方插件中 `SendText` 有 1,053 处、`Send` 有 47 处，几乎全部走主动推送。这与 `docs/plugin/protocol.md:184,186` 不一致：文档写的是宿主可以按适配器能力转换为回复，被动回复引用入站消息，避免消耗主动推送额度。群主动消息受群管理员开关和配额约束。
- **消息段与 base64 图片**：`deliver` 先把全部文本用 msg_type 0 发出，再逐个上传媒体。at、reply、face 等段要到上传阶段才返回 `adapter.capability_unsupported`，于是文字已送达，插件却收到失败。reply 段没有映射，而 `docs/plugin/protocol.md:182` 要求支持。`base64://` 图片被当作本地路径，`os.Stat` 失败（`server/internal/bot/adapters/qqofficial/media.go:76-91,150-193`）；官方插件非测试代码中有 23 处使用 `base64://`（genshin 9、starrail 7、zzz 4、game-guide 2、mihoyo-accounts 1），其中 20 处是图片（genshin 面板、game-guide 攻略图、mihoyo-accounts 扫码二维码等），3 处是抽卡导出文件段，同样受影响。渲染服务产出的 `file://` 图片已能读成 base64 上传，问题只在插件侧的 `base64://` 约定。
- **后果**：genshin 体力推送 `[At, Text, Image]` 在 QQ 官方上只发出文字；mihoyo-accounts 在群聊扫码时发不出二维码，登录被取消，私聊同样因 base64 失败。

**竞品对照**
- AstrBot 的 #8131（WebSocket 版收不到全量群消息）已由 PR #8838 修复。NoneBot 的 adapter-qq 自 1.7.1 起按 `group_mention_user` 的 `is_you` 判定 @（`is_you` 不在官方字段表里，是适配器的实测做法）。Diana #883 与 dsh-im #311 做了同类修复。QQ 官方“消息收发概述”与“群消息（全量模式）”事件页（2026-09-16 更新）都已列出 GROUP_MESSAGE_CREATE。
- AstrBot 直到 v4.29.0-beta.1（PR #9705，晚于稳定版 v4.28.2）才恢复 QQ 官方群聊 @：发送端改用文本链格式 `<qqbot-at-user id="openid" />`，旧的 `<@id>` 已标为弃用；接收端同时识别新旧两种标记。
- Koishi 的 Satori 消息元素对不支持的平台有明确降级规则：去掉标签只保留子内容，资源退化为 URL，必要时拆成多条发送。

**建议**（第 1–3 项都是修实现以符合现有文档，不需要改契约）
1. GROUP_MESSAGE_CREATE 首版只投递提及机器人的消息（`mentions` 中指向本机器人，或正文含 `<@机器人>`、`<qqbot-at-user>` 标记），并剥离标记，保持“QQ 官方群消息等于 @ 消息”的现有语义。两个事件都要处理，并按消息 ID 去重，以防同时到达。全量投递和 to_me 字段并入 G6。未识别的 dispatch 加 debug 日志。
2. 宿主在消息事件处理期间，把同一会话、未带 `reply_to_event_id` 的 `message.send` 自动按被动回复发送，并允许回退为普通发送；或者让 SDK 的 `SendText` 默认带上 `reply_to_event_id`。被动回复窗口是 5 分钟、每条消息最多 5 次，超出时回退为主动推送。
3. `deliver` 开头一次性校验并转换全部消息段：任何段不可投递时一条都不发；reply 段映射为被动回复或 message_reference；`base64://` 解码为 file_data；at 段按文本链格式 `<qqbot-at-user id="..." />` 发送。降级表写进 `docs/plugin/protocol.md`。
4. 部分投递的结构化信息（如已送达条数）需要给 `SendError` 增加 details 并修订错误码契约，属 M 级，可以排在后面。
5. 用 QQ 官方沙箱实测“接收全部群消息”下的行为，作为回归用例。

**优先级说明**：仓库视角在沙箱确认前倾向 P1，市场视角维持 P0。官方事件页已确认全量模式下 @ 消息也走 GROUP_MESSAGE_CREATE；三处缺陷合起来，v0.4.0 的正式适配器在常见配置下基本不可用，因此定为 P0。如果维护者决定在 v0.4.0 中把 QQ 官方标为 experimental，可以降为 P1。

### 4.4 v0.4.0 公开前的契约窗口

核实中多个条目都撞上同一个约束，影响后续所有“契约加法”：

- **宿主 → 插件方向天然兼容**：SDK 用标准 `json.Unmarshal` 解码事件，可以容忍新字段（`sdk/go/runtime.go:235-262`）。G14 的 `command_id` 属于这一类，不需要门控。
- **插件 → 宿主方向和 manifest 不兼容**：宿主严格拒绝插件动作数据中的未知键（`server/internal/plugins/runtime/actions.go:692-703`，返回 `plugin.protocol_violation`），manifest 顶层是 `additionalProperties:false`。G8 的 `config_schema`、G7 的 `enable_on_default`、G15 的 `run_at_ms` 与 `delay_seconds`、G41 的 KV 条件写、G45 的 `requires_services`，要么在 v0.4.0 公开前加入，要么沿用 KV TTL 的写法按 `min_core_version` 门控（协议里已有“TTL requires min_core_version >= 0.4.0”）。
- **商店目录读取严格**：见 G2。v0.4.0 公开前改为宽松读取，否则 G46、G71、G72、G82 的任何目录字段都会让已部署核心失效。

建议在冻结 v4 之前（G12），一次性决定上述可选字段是否进入 v0.4.0，并在兼容政策里写明不对称的兼容规则。

## 5. P1

### G4 接入与上手文档

- **现状**：`docs/user` 共 6 篇约 437 行，以规则和边界为主。没有 NapCat 或 LuckyLilliaBot 的对接步骤（兼容矩阵放在 `docs/dev/onebot-compatibility.md`）；没有 QQ 官方建 Bot、IP 白名单、群管理员开关的说明；没有 FAQ、安全部署清单和 macOS 放行步骤；用户文档也没有提到 `RAYLEA_SETUP_TOKEN`。Web 协议中心已能自动生成反向 WS 地址并带字段说明，新建 QQ 官方实例也会默认勾选 group_and_c2c（`web/src/views/protocols/AdapterConfigDialog.vue:104-112`），缺的是协议端一侧的步骤和排障。
- **竞品**：AstrBot 有完整的 NapCat 教程，LangBot 分别为 NapCat、Lagrange、LLOneBot 写了 OneBot11 接入指南。AstrBot（v4.26.0 起）与 LangBot 都能在 WebUI 扫码绑定 QQ 官方机器人，经 q.qq.com 的绑定接口自动回填 AppID 与 Secret；AstrBot 文档把扫码创建放在首位，并提示群主打开“获取群内全部消息”和“机器人主动在群聊内发言”。NapCat 官方“接入框架”页列出 12 个框架，其中没有 RayleaBot；NapCat-Docker 仓库自带 AstrBot 的 compose 文件。
- **建议**：
  - 新增 `docs/user/protocol-setup.md`：覆盖 NapCat、LuckyLilliaBot、SnowLuma 的反向 WS 与令牌对应关系，以及 G5 修复前的同机媒体要求；写清 QQ 官方的 AppID、密钥、IP 白名单、沙箱，以及群管理员的“接收全部群消息”“机器人主动发言”两个开关。
  - 新增 `docs/user/security.md`：说明默认 0.0.0.0 的含义、两端都要设令牌、云服务器安全组、反向代理加 HTTPS、以非 root 运行、备份含明文凭据；以 2025-09 公网 OneBot 服务遭攻击事件作反面案例（可引用 Wesley Young 的复盘博文）。同时写明日志含聊天正文：入站、出站和策略拒绝日志都记录消息内容，SQLite 管理日志默认保留 7 天（`log.retention_days` 可调），Launcher 写入的 `logs/server/` 镜像文件和 systemd journal 不受这个保留期约束（见 G81）。
  - 扫码绑定 QQ 官方机器人依赖 q.qq.com 的绑定接口，RayleaBot 未实测，可作为 G4 之后的候选项评估，不在本条范围。
  - FAQ：setup token 在哪里（含 systemd 下去 journal 查找）、KickedOffline、假在线。
  - 向 NapCat 文档提交 RayleaBot 接入条目。
  - 少用截图，多写配置字段名，以免协议端改版后文档失效。

### G78 首次使用任务清单

- **现状**：管理员初始化后直接进入状态页（`web/src/views/auth/SetupView.vue:26-31`），Web 中没有首次使用引导或任务清单。首页唯一带下一步动作的是连接盒子的“还没有机器人连接”空状态；仪表盘的就绪检查刻意隐藏了 adapter 与 plugins 两项（`web/src/views/dashboard/DashboardView.vue:76-77`），零插件时首页没有任何提示；协议中心的空状态盒子没有动作按钮。没有贯通“连接 → 装插件 → 启用 → 首次收发成功”的流程。
- **竞品**：AstrBot 首次登录后进入三步快速引导（配置模型、配置平台机器人、是否允许 Agent 使用电脑），每步可跳过，完成状态取自后端；LangBot master 分支有四步向导（选平台、配置并验证 Bot、AI 引擎、完成），进度保存在后端、可续做，尚未确认进入稳定版。同赛道的游戏框架都没有：Koishi 默认配置里的 oobe 插件是空实现，靠默认启用的沙盒和文档；NoneBot2 靠 `nb create` 交互式脚手架；gsuid 控制台首次登录只要求注册码。所以这一项半是对标 AI 平台，半是 RayleaBot 自己的设计。
- **建议**：在仪表盘加一份可跳过、可收起的清单，步骤是添加连接、连接在线、安装并启用参考插件、测试会话有收有发。区分四种状态：服务就绪、连接在线、插件已运行、成功收发，分别读取 `/readyz`、适配器快照、插件状态和 `GET /api/system/message-stats`（按连接给出收发合计与 `last_received_at`）。各步完成与否只读 Server 的正式结果，跳过或收起是浏览器本地偏好，不改契约。仓库没有按会话评估权限的接口，“检查权限”一步改为提示加深链到权限策略与黑白名单页。“安装参考插件”依赖 G2 先有可装组合。验收：未添加连接、连接离线、插件未运行、没有收发四种情况都显示正确的下一步。工作量 S–M，只改 Web。

### G5 OneBot 出站媒体用本地路径

- **现状**：`render.image` 和内置菜单产出 `file://` URI（`server/internal/render/artifact.go:257-266`，`server/internal/bot/menu/menu.go:840-846`），OneBot 适配器原样转交（`server/internal/bot/adapters/onebot11/outbound_segments.go:90-103`）。合并转发节点同样原样透传（`server/internal/plugins/actions/onebot_project.go:165-190`）。协议端跑在 Docker 或其他主机时，帮助菜单、游戏面板和订阅解析的媒体全部发不出去，用户文档也没有写同机要求。
- **竞品**：OneBot11 规范允许 `base64://`，主流实现端都支持。gsuid 早柚协议规定图片用 base64 或链接下发，与宿主文件系统解耦。
- **建议**：在 OneBot 适配器内部把本地路径和 `file://` 统一转成 `base64://`，递归处理合并转发节点；设大小上限，超限返回结构化错误。这是适配器内部调整，不需要新增配置字段：按回环地址判断的 auto 模式会误判同机 Docker 桥接。视频转成 base64 体积膨胀约 33%，还受协议端 WS 帧上限约束，需要实测 NapCat 与 LLBot 的上限。

### G6 指令前缀与 to_me 语义

- **现状**：`command.prefixes` 配成空也会被归一回 `"/"`（`server/internal/config/command_prefixes.go:14-32`）。插件指令匹配（`server/internal/plugins/command_prefixes.go:91-179`）和内置菜单解析（`server/internal/bot/command/parser.go:29-58`）都必须先命中前缀。宿主不给插件 to_me 或 mentioned 字段，at 段也不参与匹配，所以“@别的机器人 + 指令”同样会触发。starrail 和 zzz 的专属前缀“*”“%”与 miao-plugin、ZZZ-Plugin 相同，可以通过 settings_key 修改。现行契约 `contracts/plugin-info.schema.json:156` 明确规定 mention 不参与指令解析，本条要推翻这一约定。
- **竞品**：Koishi 私聊免前缀，群聊以 @机器人或昵称开头时免前缀；NoneBot2 内置 to_me 规则，`COMMAND_START` 可以为空；TRSS 可按群配置 onlyReplyAt 与 botAlias（`group.yaml`）。
- **建议**：
  - 由宿主统一计算 to_me：OneBot 群消息首段 at 本机器人、私聊、QQ 官方的 GROUP_AT 与 C2C 消息。注意 QQ 官方平台已剥离 at 段，判定要区分协议。
  - 新增热更新配置 `command.mention_without_prefix` 和 `command.private_without_prefix`；at 段全部指向其他身份时不触发。按群的“仅响应@”开关并入 G7 的同一份 Server 状态。
  - 需要改 config、plugin-protocol、plugin-info 三份契约和两套解析器，工作量 M；默认值变化要写进升级说明。
  - 同群同时有 OneBot 号和 QQ 官方 bot 时，两套身份命名空间的比对规则要单独设计。

### G7 按群启停插件与宿主准入

- **现状**：插件只有全局 `desired_state`；白名单只约束指令（`contracts/web-api.openapi.yaml` 原文为 whitelist for command dispatch admission），非命令消息投递给订阅插件时只受黑名单约束。subscription-hub 因此在插件里自建了按会话的解析开关（`subscription-hub/internal/plugin/resolver.go:164-174`），将来 AI 插件也会遇到同样的问题。
- **竞品**：Yunzai、gsuid（按插件和服务配置黑白名单，同时匹配群号和用户）、HoshinoBot、ZeroBot-Plugin、Koishi（过滤器）的核心都能按群启停插件；AstrBot 按配置文件路由会话。这是同赛道群主最普遍的诉求。
- **建议**：
  - 规则由 Server 持有。新表 `plugin_scope_rules` 复用黑白名单的 `governance_scope`，解析顺序为会话规则、全局规则、插件默认。
  - 过滤要放两处：chatpolicy 的指令解析（否则被禁插件仍参与专属前缀遮蔽和权限判定），以及 `dispatch.messageCandidates` 的非命令投递（`server/internal/bot/pipeline/dispatch/layers.go:27-107`）。
  - manifest 的 `enable_on_default` 属于 4.4 节窗口内的加法。
  - 拆成两步：宿主过滤加 Web（M），插件协议 `governance.plugin_scope.*` 动作（S）。
  - 单条指令的启停和权限覆盖可以随本条一起做（见 G29）。落地后，subscription-hub 和 AI 插件改用宿主规则，避免同一业务状态有两个来源。

### G8 插件配置 schema 与自动表单

- **现状**：manifest 只有无类型的 `default_config`，`PUT /api/plugins/{id}/settings` 接受任意顶层键。没有自写 Vue 管理页的插件在 Web 上改不了配置：oil-price（5 项）和 delta-force（3 项）就是这样。7 个官方插件的管理 UI 大部分是业务界面，不是设置页。核心配置工作台是手写字段定义（`web/src/lib/config-form.ts`），并非 schema 驱动，选型复核已判保持。
- **竞品**：Koishi 用 Schemastery 同时生成类型和表单；AstrBot 的 `_conf_schema` 支持 secret 掩码、options、slider 与 i18n；LangBot、MaiBot、gsuid 的核心都会生成插件配置表单。
- **建议**：
  - manifest 增加可选的 `config_schema`，取 JSON Schema 2020-12 的受限子集（基本类型、enum、基本类型数组、至多一层嵌套，加 title、description、default、范围），`writeOnly` 字段映射到 secret 存储。
  - `raylea-plugin inspect` 校验 `default_config` 是否符合 schema。Server 在 PUT settings 和插件的 `config.write` 两条写入路径上都用已有的 `jsonschema/v6` 校验，返回带字段路径的结构化 details。
  - Web 新写一个通用 schema 渲染器，复用字段控件和草稿保存语义。工作量 M–L，manifest 字段受 4.4 节约束。

### G9 AI 能力：以官方插件落地对话 MVP

- **现状**：主仓与 11 个官方插件按 openai、anthropic、llm、mcp、embedding 等关键词 grep，业务代码零命中，宿主动作注册表也没有 llm 类动作。章程既没把 AI 列为目标，也没列为非目标。现有协议已具备 AI 插件需要的大部分原语：非命令消息扇出、`event.detach`（默认 900 秒）、`session.wait`、secret、KV、数据目录、scheduler、`render.image`，以及由宿主认定调用方的 `plugin.call`。
- **竞品**：AstrBot 与 LangBot 的核心内置多提供商、Agent、MCP 和知识库；gsuid 在游戏插件赛道上加装了 ai_core；NoneBot2、Koishi、Yunzai 有成熟的社区插件（llmchat、ChatLuna、chatgpt-plugin），连 ZeroBot-Plugin 合集都带 aichat。RayleaBot 是唯一一个连社区插件层都没有的。
- **建议**：
  - 落地形态选官方插件 raylea.ai（Go 原生子进程），宿主不内置 LLM、Agent、MCP 代码，不改宿主契约，会话与记忆都归插件所有。在章程写明“AI 由官方插件提供”，这只是文档改动。
  - MVP 只做以下几项，人格模板、摘要和第二批原生 SDK 留到后续：
    - 主干只接 OpenAI 兼容接口（base_url 加 model）；
    - 触发方式：私聊、@、唤醒词；
    - 按群启用（G7 落地后改用宿主规则）；
    - 每用户、每群、全局三级配额，加月度预算熔断；
    - 每轮的步数、输出长度和超时上限；
    - 用量账本，记录模型、token、耗时和调用方。
  - LLM 调用一律 detach；manifest 的 `concurrency` 默认为 1，要显式调高。
  - 在插件数据目录用 SQLite 没有先例。在线备份运行时直接复制 `data/`（`server/internal/operations/backup/archive.go:89`），WAL 模式下可能拿到不一致的快照，插件应使用 `journal_mode=DELETE` 或定期 `VACUUM INTO` 快照。
  - 依赖只进插件仓库的 go.mod，优先官方 SDK：openai-go、anthropic-sdk-go、genai。langchaingo 近一年没有发布，不推荐。
  - 不做代码执行、Computer Use 和本地 Shell（见 T10）。
  - 排在 v0.4.0 公开发布与游戏插件上架之后。后续阶段见 G36–G38 与 G63–G66。

### G10 OneBot 入站令牌护栏

- **现状**：OneBot11 入站传输的 `access_token` 默认为空，公开路由 `/api/adapters/{id}/reverse-ws` 与 `/webhook` 在令牌为空时直接放行，令牌比较用的是普通 `==`（`allowOneBotIngress`，`server/internal/management/protocol_handlers.go:136-152`）；管理面令牌已用 `hmac.Equal`（`server/internal/management/security_tokens.go:29`），改为常量时间比较可以直接沿用。Web 文案写着令牌“可留空”（`web/src/locales/zh-CN/protocols.ts:137`）。超级管理员只依据事件里的 user_id 判定，所以能访问端口的人可以伪造超级管理员指令。webhook 不需要 DNS rebinding，任意跨源 no-cors POST 都能送达。公开版 v0.3.1 默认监听 127.0.0.1，默认改为 0.0.0.0 来自未公开发布的 `ed390c77`，因此这个组合只存在于 0.3.1 之后的未公开候选中，v0.4.0 发布后将进入公开版本。
- **竞品**：AstrBot 的 OneBot 反向 WS 默认同样是 0.0.0.0 加空令牌，TRSS 也是，RayleaBot 与它们持平；NoneBot2、Koishi、AstrBot 启动器默认只监听回环地址。同赛道的 gsuid_core 是“非回环强制令牌”的现成先例：默认只监听 localhost，非可信 IP 的连接必须带 WS_TOKEN，令牌为空即拒绝，失败会计数封禁。2025-09 的公网 OneBot 服务遭攻击事件正是服务暴露公网且未设令牌引发的。
- **建议**：
  - 协议中心“添加连接”默认生成 32 字节随机令牌并提供复制按钮。这一步是 S 级，不涉及契约，收益最高，排在最前。
  - v0.4.0 本来就要求重建、重配插件，可以直接规定：`server.host` 不是回环地址时，reverse_ws 与 webhook 必须设令牌。需要先改配置契约、错误码与 fixtures。
  - 诊断与 readyz 对无令牌入站报 degraded；令牌比较改为常量时间。
  - 是否把桌面与 Launcher 包的默认监听改回 127.0.0.1，与章程“默认 0.0.0.0”冲突，交由维护者决定。这些都应在 v0.4.0 发布前完成，也因此不需要为本条单独发 GHSA。

### G11 插件挂起检测

- **现状**：`docs/plugin/lifecycle.md:27` 写着运行中通过 ping/pong 保活，协议也定义了 ping/pong 帧，SDK 会应答 ping，但服务端非测试代码从不发送 ping。事件超时只把当前事件结算为 `plugin.event_timeout`，进程状态仍是 running。消息按 priority/block 分层投递，高优先级插件挂起后，同一消息的低层插件每条都要等满 60 秒超时，并占着会话通道（`server/internal/bot/pipeline/dispatch/layers.go:148-163`，`worker.go:160-200`），积压超过 16 条后开始丢消息。
- **竞品**：LangBot 有重启失败阈值与熔断，MaiBot 有 Runner 存活判定，同进程的 AstrBot 有事件循环看门狗。
- **建议**：
  - 第一层：宿主按固定间隔发 ping，连续两次收不到 pong 即按 crash 处理，走现有的退避与 `recovery_required` 路径。这是按现有契约修实现，工作量 S；如果插件详情要显示“最近一次 pong”，需要在 web-api 新增字段。
  - 第二层：连续多个前台事件超时后回收进程，需要新增 `state_diagnosis` kind 与错误码。外部 API 或渲染缓慢时容易误杀，这类回收应单独计数，不要计入崩溃次数。

### G12 插件合同稳定承诺

- **现状**：插件协议从 v1（SDK v0.2.0，2026-08-04）到 v4（2026-09-14）约六周，经历四代、三次不兼容升级，每次旧插件都要用新 SDK 重新构建；`docs/plugin/lifecycle.md:76` 写明旧合同兼容执行不在正式范围内。0.3.1 之后以 0.4.0、0.5.0、0.7.0 编号的候选都未公开发布，公开用户实际只会经历一次跳变：从 v0.3.1 到 v0.4.0。所以这是前瞻性风险：只要开放第三方，每次无过渡的破坏都会让外部插件全部失效，而目录只保留当前版本，也无法回退。
- **竞品**：Koishi v4 自 2022 年起保持语义化版本；NoneBot2 2.x 内基本稳定，但 2.2.0 仍要求插件更新以适配 Pydantic v2；LangBot v3→v4 时插件从约 119 个重建到 96 个，并让核心长期背负 SDK 版本固定的包袱。
- **建议**：在 `docs/plugin/README.md` 与 `contracts/README.md` 写明：protocol v4、manifest v4 至少覆盖 0.4.x 及下一个 minor；新能力一律走可选字段加 `min_core_version` 门控，兼容规则按 4.4 节写成不对称的形式。这只需要写文档，工作量 S，应作为 v0.4.0 公开发布的前置项。宿主同时接受 N 与 N-1 两代握手需要把单一的 `ProtocolVersion` 拆成两套 wire，属于 L，等第三方插件真正出现后再评估。

### G13 社区与可信度信号

- **现状**：GitHub API（2026-10-05）显示仓库 0 star、0 fork、0 watcher，description、homepage、topics 都为空，community profile 健康度 25%（只有 README 与 LICENSE）。没有 CONTRIBUTING、Issue 模板、交流渠道（GitHub Discussions 未开启，README 没有群号），没有截图或演示。PR 模板是有意删除的（`7a88290d`）。没有 SECURITY.md，也没有开启私密漏洞报告。README 首段写“面向个人开发者和开源协作者”，与第 9 节的定位判断不一致；README 第 15 行仍写“官方页面…运行在独立插件域”，而独立插件域已在 v0.4.0 中删除。仓库只有一位作者，bus factor 为 1。
- **竞品**：所有同类项目都有 QQ 群，AstrBot、Koishi 另有 Discord 或论坛。SECURITY.md 在竞品中并不普及：AstrBot、LangBot、Koishi、NoneBot2、TRSS、MaiBot 都没有。
- **建议**：
  - 零成本先做：设置仓库 description、topics、homepage。
  - 再做三件事：Issue 模板（bug 模板必填 `raylea-server version --json`，并附诊断包；诊断包做了凭据脱敏，但含最近的日志摘要，可能带聊天内容，需提示用户检查）、开启 Discussions、写 CONTRIBUTING。
  - QQ 群等 v0.4.0 公开发布时再开，群公告写明响应预期。
  - 开启 GitHub Private Vulnerability Reporting：只需一个开关，零成本；SECURITY.md 正文的价值次之。
  - README 加 3–4 张 1920×1080 截图，按目标用户改写首段，修正第 15 行。
  - 降低 bus factor 的最低动作：设置 GitHub 账号继承人，并写一份维护者交接清单。
  - “插件上架申请”模板等 G12 冻结合同后再开。

### G14 下发 command_id 与捕获组

- **现状**：命令匹配在 Server 完成，内部的 CommandMatch 带有 manifest 命令 ID，但投递给插件的 payload 只有首词 `command` 和按空白切分的 `args`（`contracts/plugin-protocol.schema.json:399-414`，该 payload 为所有事件类型共用的封闭对象）。genshin、starrail、zzz 各写了一份约 100–150 行的 commandSet，按宿主的顺序重放匹配；宿主一旦调整前缀顺序，插件就可能静默路由错误。
- **竞品**：Yunzai、NoneBot2、Koishi、ZeroBot 都在框架层把 handler 和命令绑定好，没有一家让插件重放匹配。
- **建议**：消息事件 payload 增加可选的 `command_id` 与 `command_captures`（pattern 的命名捕获组，需要改用 `FindStringSubmatch`）。这是宿主→插件方向的加法，不需要门控。Go SDK 增加 `CommandID()` 与 `Captures()` 后，就能删掉三份重放匹配器。本条是 G39（路由器）和 G40（测试 harness）的前置条件。

### G15 调度器

- **现状**：`scheduler.create` 只接受 cron 字符串，契约没有定义语法（`contracts/plugin-protocol.schema.json:2044-2073`，required 为 task_id、cron、event_type）。实现是自研的 5 字段解析（`server/internal/scheduler/cron.go:13-17`），分钟精度，每 30 秒检查一次；不支持宏、名称、秒，也没有一次性任务；日字段与周字段是“与”语义（`cron.go:62`）。游戏插件把抽卡同步、米游社任务、云游戏签到做成每用户一个 `* * * * *` 作业（每类最多 256 个）。同一 tick 的触发会超出非控制事件队列（默认 16），被以 `platform.rate_limited` 丢弃。作业遍历顺序随机，所以会自愈，但会刷失败日志，也可能漏执行。
- **竞品**：TRSS 有核心 cron；gsuid 用 APScheduler，控制台可暂停，但重启后失效；AstrBot 有持久化的未来任务，但面向 Agent，插件只能通过未文档化的内部接口使用。持久化的一次性任务在竞品中并不普遍，本条的优先级主要来自 RayleaBot 自身的可靠性风险。
- **建议**：
  - 在 `scheduler.create` 中新增与 cron 互斥的 `run_at_ms` 和 `delay_seconds`（插件→宿主方向，属于 4.4 节窗口），一次性任务触发后由宿主删除。
  - 在契约里写明 cron 语法：5 字段、时区取 `scheduler.timezone`、启动时逾期补触发一次。日/周保持“与”语义并写明，不改成 Vixie cron 的“或”。
  - 宏与 jitter 列为 P3。
  - 管理面暂停与恢复见 G33。

### G16 国内网络

- **现状**：
  - FFmpeg 三个平台都是 GitHub 单源，不支持断点续传。选型复核 R7 已落地：FFmpeg 改为在管理面按需准备，不再阻塞首次启动；Windows 与 Linux 改用 BtbN 的 gpl-shared 构建，下载量约为静态构建的一半。
  - `server/internal/plugins/lifecycle/install_sources.go` 的下载 Transport 没有设置 Proxy，商店安装也走这条路径，所以所有插件包下载都忽略 `HTTPS_PROXY`。
  - 代理用法没有任何文档。
- **竞品**：gsuid 的 Docker 镜像托管在国内 docker.cnb.cool；TRSS 在 Gitee、GitCode 和自建 git 多处镜像；AstrBot 提供国内部署说明。
- **建议**：
  - 落地 R7：FFmpeg 改为后台准备，并换用 gpl-shared 变体。FFmpeg 路径在插件启动时经环境变量注入（`server/internal/plugins/runtime/spec.go:112-133`），后台准备完成后要重启依赖它的插件。
  - 继续固定 BtbN 每月最后一个构建（保留两年），不要用日构建（只保留 14 天）。
  - 插件下载 Transport 补上 `Proxy: http.ProxyFromEnvironment`。
  - 在 deployment.md 写一节出站代理：说明 `HTTPS_PROXY`、`NO_PROXY` 的用法（含 QQ 官方 API 域名）；QQ 官方 IP 白名单按出口 IP 判定，全局代理会导致白名单拒绝。
  - 不重提核心更新与商店的镜像（选型复核已判保持）。

### G17 无头服务器初始化与 systemd 加固

- **现状**：
  - 首次设置地址只在交互终端打印，systemd 下只能去 journal 里找。用户文档没有提到 `RAYLEA_SETUP_TOKEN`；该变量必须是 base64url（无填充）编码、解码后不少于 32 字节，否则服务启动失败。
  - `packaging/systemd/rayleabot.service` 没有 `User=`，默认以 root 运行。数据目录是 0755，数据库文件权限受 umask 影响，一般是 0644。
  - 两种 Linux 包都没有 Chromium 共享库清单（libnss3、libgbm 等）。
- **竞品**：竞品大多靠默认凭据或默认无鉴权绕开这个问题。RayleaBot 的随机 setup token 更安全，但没有配套文档时，这项安全设计就成了上手门槛。
- **建议**：
  - 文档（S）：写明 EnvironmentFile 注入 token 的步骤，附生成命令（如 `python -c "import secrets;print(secrets.token_urlsafe(32))"`）。
  - systemd 示例：加 `User=`、`UMask=0077`、`PrivateTmp=yes`，并按 `shutdown_budget_seconds` 设置 `TimeoutStopSec`。`NoNewPrivileges` 与 `ProtectHome` 需要先实测对 Chromium 的影响。
  - 启动时把数据目录设为 0700、数据库文件设为 0600。
  - 为两种包写 Chromium 依赖清单。
  - 以下两项需要另行处理：doctor 新增共享库检查要改 `cli-commands.yaml`（M）；把设置地址写入文件与 `docs/dev/logging.md:31` 的原则冲突，需单独设计。

### G18 桌面开机自启与崩溃拉起

- **现状**：Launcher 启动时不会自动启动服务，服务进程意外退出后只提示用户手动重启（`launcher/internal/desktop/coordinator_snapshot.go:39-41`），也没有开机自启。唯一的自动启动是一键更新后的 `--resume-service` 交接（`launcher/internal/desktop/resume.go`）。关闭窗口时可以隐藏到托盘，插件崩溃由 Server 在重试预算内自动重启。
- **竞品**：AstrBot 启动器有开机自启和托盘，Koishi 的命令行监视进程会在崩溃后重启，Yunzai 靠 pm2；没有一家同时具备这两项。
- **建议**：用 Wails v3 beta.9 自带的 AutostartManager 实现“登录时启动并最小化”，再加“Launcher 启动后自动启动服务”和“服务异常退出时退避重启”（10 分钟内最多 3 次）。Server 在启动失败和运行中崩溃时都返回退出码 1，不能按退出码区分，应改用“启动后存活超过 N 秒才算可重启的异常退出”。这些都属于 Launcher 的本机进程编排职责，工作量 S–M。

### G19 官方 Docker

- **现状**：`docs/user/deployment.md` 明确写着不提供正式 Dockerfile、Compose 或镜像，官方容器交付从 v0.2 起延后。选型复核否决 GHCR 镜像的理由有三：镜像无法进入要求归档字段的发布清单；`update_mode` 关不掉容器内的 `update apply`；当前规模下不成立。章程第 91 行要求新部署模型单独设计。
- **竞品与新证据**：AstrBot、Koishi、LangBot、TRSS 的核心或官方脚本都提供 Docker，NoneBot2 由官方 nb-cli 生成；AstrBot、Koishi、LangBot、gsuid 提供 amd64/arm64 双架构。TRSS 的 Docker 是一键脚本（`lib/tools/docker.sh`），默认经国内加速源拉 node:slim 本地构建，不是预构建镜像。Koishi 镜像一年多才更新一次，说明“镜像只含运行环境”这种形态维护成本低。
- **建议**：采用 Koishi 式做法，作为单独设计提出，并以 G1 为前置：
  - 镜像只含运行环境（Chromium 共享库、CJK 字体、tzdata、tini），发布包放在卷里。这样绕开复核的两条技术否决理由。
  - HEALTHCHECK 用 `/healthz`：`/readyz` 在 `setup_required` 时返回 503。
  - 不要在每次容器重启时自动执行 `update apply`，否则会静默跨越插件合同大版本；需要先在 CLI 契约里加 `--if-pending`，与 G53 一起设计。
  - entrypoint 首次下载发布包要允许指定下载地址（见 G16）。
  - 同时推 Docker Hub，或写明镜像加速方式。
  - 先以 `examples/` 下的参考 compose 起步，标为社区支持。
  - 发布构建没有显式设置 `CGO_ENABLED=0`，Linux 二进制可能动态链接 runner 的 glibc，选基础镜像前先用 `ldd` 核实。
  - 向 NapCat-Docker 提交 RayleaBot 的 compose，比自己维护 NapCat 服务定义更省事。

### G20 官方插件覆盖

- **现状**：共 11 个自研插件，没有第三方插件。已覆盖：订阅推送与链接解析（subscription-hub）、游戏查询（genshin、starrail、zzz、delta-force、game-guide）、娱乐（fortune、roulette）、工具（oil-price、echo）。空白：表情包、点歌、群聊统计与词云、群管（入群审核、欢迎、踢人）、AI。链接解析也缺小红书和快手。
- **竞品**：表情包、点歌、群统计、群管在 AstrBot、NoneBot2、Koishi、gsuid 都是高频类别，下载数据见 1.3 节；gsuid 插件列表已有 AI 对话、表情包与三平台点歌。按名称或描述关键词统计，NoneBot 商店有 AI 类约 60 条、群管约 18 条、表情包约 20 条。
- **建议**：不追数量，按需求数据定一份“官方覆盖清单”，排在 G2 发布链之后：
  - 表情包：参照 gsuid 的 core_plugin_memes，对接 MIT 许可的 meme-generator-rs（有 HTTP server 模式），比自己写模板便宜。
  - 群统计与词云：订阅 `message.group`，数据写入插件数据目录，定时出图。
  - 点歌：需要从零实现搜索接口，现有网易云模块只能复用加密与账号部分。宿主已透传 music 消息段，宿主侧不用改。
  - 群管插件：用现有的请求处理与退群动作实现自动审批、入群欢迎和自动退群，补上踢人动作（G22）后再加踢人。在 QQ 官方路径上，可先接 2026-08-10 开放的禁言与入群审批接口（机器人需为群管理员），踢人和黑名单仍处于内邀。
  - 上架后按场景给出推荐组合，见 G83。

### G21 日志关键词检索与导出

- **现状**：`/api/logs` 只能按 level、source、protocol、plugin_id、request_id 和时间范围筛选（`contracts/web-api.openapi.yaml:1304-1395`），不能搜索正文；Web 没有导出，诊断包里只有最近 100 条。Launcher 托管时，`logs/server/` 下有按日期命名的日志文件，可以作为离线替代；这些文件只做凭据脱敏，含完整聊天正文，Launcher 也不自动清理（见 G81）。
- **竞品**：gsuid 核心控制台的历史日志支持查找字符，Yunzai 社区的 guoba-plugin-next 支持关键词搜索。AstrBot、LangBot 能搜索和导出的是对话记录，不是运行日志。
- **建议**：`/api/logs` 增加 `q` 参数，对 message 列做转义后的 LIKE 子串匹配（中文子串有效），必须与范围筛选组合使用；新增按当前筛选流式导出 JSONL 的端点，设行数上限。先改 OpenAPI 与 fixtures，再按 7 天数据量做一次查询基准。FTS5 不适合中文两字词，选型复核已否决。与 G79 的 event_id 筛选共用同一次 OpenAPI 改动。

### G79 单条消息处理链路

- **现状**：
  - request_id 在插件投递时才由 runtime 生成（`server/internal/plugins/runtime/manager_delivery.go:44`），只能串起单个插件的处理、插件日志和终端回复的发送结果。
  - bridge 的入站日志和指令策略拒绝日志写在投递之前，request_id 是保留值 `system`（`server/internal/platform/logging/logger.go:121-131`），契约规定它不关联任何日志，Web 也不提供关联入口；入站日志里也没有 event_id。
  - 事件中途经本地动作发出的 `message.send` 和插件 `logger.write` 使用 SDK 生成的 `local_*` 独立 ID，日志不记 parent_request_id。
  - 最常见的“没回复”原因（前缀或指令不匹配、没有插件接受、没有可投递插件）只在 debug 级记录（`server/internal/bot/pipeline/bridge/events.go:23-48`），默认 info 级下看不到。
  - 插件处理失败经 FailureTracker 聚合，同一原因首次记一条、之后每 5 分钟汇总，单条失败不一定有自己的日志行。
  - 结果是现有日志无法按一条消息串起“收到、匹配、拒绝、超时、发送确认”。
- **竞品**：AstrBot 的 Trace 页自注只记录部分主 Agent 的模型调用路径；LangBot 监控中心按消息记录模型调用、token、工具调用与错误。两者都只覆盖 AI 调用，没有竞品为命令型机器人提供“为什么没回复”的逐条原因，所以这是差异点，不是追平项。
- **建议**：
  - 第一步（P1，S–M）：在 bridge 接纳时就把 event_id 写入入站、策略拒绝、队列丢弃、插件投递、本地动作与出站各条日志，本地动作日志补记 parent_request_id；`/api/logs` 增加 event_id 筛选（先改 OpenAPI）；日志详情提供“同一条消息”入口。不新增持久化，也不从日志文本反推业务状态。
  - 第二步（P2，M）：做“单条消息原因视图”，回答是否收到、匹配了什么、被哪条规则拒绝、在哪一步超时、发送是否得到确认。为控制日志量，只对定向消息（@ 或私聊）把忽略原因提升为 info 摘要，依赖 G6 的 to_me。
  - 验收场景：连接未就绪、前缀不匹配、权限拒绝、插件异常或超时、发送未确认。

## 6. P2

| 编号 | 主题 | 工作量 | 竞品对照 | 建议要点 |
|---|---|---|---|---|
| G22 | 群管动作：踢人与受控原生 API 透传 | S / M | 主要竞品都能踢人，NoneBot2、AstrBot 可透传任意动作 | 补回 `group.member.kick`，换掉把 kick 当反例的测试；透传限定在 provider 命名空间，需先改契约；踢人从未支持过，并非被删 |
| G23 | QQ 官方 Markdown、按钮、撤回、交互回调 | M–L | NoneBot adapter-qq、LangBot、gsuid 支持 Markdown；AstrBot 没有 | 先做原生 Markdown（msg_type 2，已对所有机器人开放）；按钮与回调要加 interaction intent；撤回与引用解析放进 `qqofficial_action_kind`，并为群管接口预留与 OneBot 同名的语义（禁言与入群审批已开放、需机器人为群管理员，移除成员与黑名单仍内邀）；QQ 官方成为首选路径时升 P1 |
| G24 | QQ 官方用户不能成为超级管理员 | M | AstrBot 管理员接受任意平台 ID，Koishi 有 authority 与 bind | 契约明确不授权 openid；`super_admins` 还经 init 帧下发给插件；新增并列的作用域字段，不改原字段类型；先实测 member_openid 跨群是否一致；配一个“查询我的 ID”指令 |
| G25 | 账号离线（假在线）检测 | M | NapCat #2071 显示静默离线时不发 bot_offline | 以心跳 `status.online` 或 `get_status` 轮询为主信号、bot_offline 为辅，去抖后写入适配器快照；需要改快照状态契约 |
| G26 | 适配器能力声明与 SDK 长消息助手 | M–L | 多数竞品的长文本处理默认关闭或只对 QQ 个人号生效 | 在 init.bots 下发能力枚举，插件按能力而非协议名分支；长消息与合并转发降级放在 Go SDK |
| G27 | 账号级出站节流与群发错峰 | S | 主流竞品的限流都管入站，只有 HoshinoBot 一类做群发间隔 | 在 window_limiter 上按 bot 身份加令牌桶，默认值写明是经验值；与 v0.4.0 收缩出站策略的方向相反，要在契约变更说明里写理由 |
| G28 | 请求事件绕过黑名单，没有入群与加好友策略 | S（宿主） | Yunzai 开箱即有 autoFriend、autoGroup、autoQuit | 决定黑名单是否覆盖 `request.*`（语义变化，契约先行）；审批策略做进 G20 的群管插件 |
| G29 | 指令级覆盖：权限、冷却、别名、每日次数 | M | Koishi 默认安装的 rate-limit 支持单条指令的每日上限与间隔；gsuid 可按服务改启用与权限 | 启停与权限随 G7 交付；冷却、别名、每日次数第二步；extra_aliases 要纳入冲突检测；冷却提示文案可配置 |
| G30 | 走完整策略链路的聊天沙盒 | L | AstrBot ChatUI、Koishi 官方 sandbox、LangBot 对话调试 | 新增 source_protocol 会撞上多处只认两种协议的闭合校验；更省事的做法是 Server 内的 OneBot11 内存回环传输，或一个独立的“假 NapCat”开发工具；菜单和模板已有预览 |
| G31 | 统计只到连接级 | M | Koishi 官方 analytics 有按指令、按频道的维度 | 新增指令与插件处理的小时桶统计，复用 dispatcher 的计数埋点；按群只记收信数并单设保留期；定时任务已有逐任务统计 |
| G32 | 认证：会话列表与吊销、TOTP、长期 API Token | M–L | AstrBot 有 TOTP 与带 scope 的 API Key（v4.18.0 起），LangBot 有 API Key | 选型复核 R4 的哈希令牌已随迁移 `000008` 落地；会话列表与吊销另行实现；保持单管理员 |
| G33 | Web 不能重启服务；定时任务不能暂停 | S–M | AstrBot、Yunzai、gsuid 能远程重启；gsuid 能暂停任务但重启后失效 | 重启：shutdown 增加 intent=restart、退出码 3，需改三份契约与 Launcher；暂停：`scheduler_jobs.enabled` 已有，但内存侧写死 Enabled:true，upsert 也会覆盖，契约要写明管理员暂停优先 |
| G34 | 聊天内运维动作 | M | Yunzai、gsuid 核心都有聊天运维指令 | 先做只读的 `system.status.read`（`plugin.list` 已含插件状态）；跨插件启停与更新检查放第二步，并与生命周期锁互斥 |
| G35 | 群发通知官方插件 | S | gsuid 核心与 guoba-next 都有批量发送 | 做成官方插件，逐个目标回报 QQ 官方主动消息的失败，默认限速 |
| G36 | AI 工具桥接 | L | gsuid 的 to_ai 会捕获命令输出交回模型；AstrBot 用 llm_tool | 服务调用只允许一跳（`server/internal/plugins/runtime/services.go:45`），需要 CK 的数据无法经游戏插件提供，要在放开两跳、AI 插件直连 accounts@1、只用缓存数据三者中选一；`plugin.request` 不能 detach，工具只能返回缓存 |
| G37 | AI 长期记忆与知识库 | L | AstrBot 有 FAISS 加 BM25，LangBot 有插件化 RAG，chatgpt-plugin 有群记忆 | FTS5 不适合中文，要在应用层分词或先只做向量检索；插件 SQLite 的备份一致性同 G9；宿主日志里虽有正文，但不能作为上下文来源 |
| G38 | AI 拟人群聊、主动发言与防抖 | M | AstrBot 下载第一的就是这类插件，MaiBot 主打 | 观察 handler 只写环形缓冲后立即返回，LLM 调用 detach；零优先级订阅收不到被定向的命令消息；scheduler 只到分钟级 |
| G39 | Go SDK 命令路由与中间件 | M | 主流竞品都在核心提供命令 DSL | 依赖 G14；SDK 已有 typed 动作、服务分派和 Ask 续接，缺的是路由器、中间件与参数解析 |
| G40 | 官方插件测试 harness | M | 只有 NoneBug 与 Koishi plugin-mock（2024-06 后未更新），AstrBot、LangBot、MaiBot 都没有 | 把三份约 300 行的假宿主收敛成 `sdk/go/plugintest`；主要收益是维护者内部降本 |
| G41 | KV 配额按插件计；条件写与批量 | S / M | AstrBot、LangBot 按插件隔离存储 | 全局 16 MB 一个插件就能耗尽，先改为按插件计；CAS 与 batch 是插件→宿主方向的加法，没有出错证据，可降为 P3 |
| G42 | manifest 诊断与 webhook 上限不一致 | S | MaiBot 用严格模式校验清单 | 未知事件名只给 warning（契约有意保持开放）；webhook 上限契约写 10 MiB、实现截到 1 MiB，二者择一统一 |
| G43 | 帮助菜单渲染失败时降级为文字 | S | TRSS 有免浏览器的 shotium 与远程 browserless | 菜单数据在 Server 内现成可用，属内部改动；同时修正 `server/internal/bot/menu/menu.go:137` 与行为不符的日志 |
| G44 | 第三方发布流水线 | M–L | NoneBot2 用 NoneFlow 自动校验，MaiBot、AstrBot 有登记库 | echo 已有三平台矩阵，缺的是去掉主仓库检出并标为模板；独立的 community-catalog 加 CI 握手冒烟；前置 G2、G12 与目录宽松读取 |
| G45 | 插件间依赖声明 | M | 只有 Koishi、NoneBot2 具备 | 先零成本：商店描述与 README 写明“需配合米游社账号插件”（genshin 已会提示）；契约级 `requires_services` 等第三方出现再做 |
| G46 | 目录历史版本与撤回 | M | MaiBot 已有多版本加 yanked 结构，但多数条目只用单版本 | 需要先让目录读取宽松；sync 改为列出 Release 列表；这是插件包版本选择，不是核心回滚 |
| G47 | 插件更新后台检查与提示 | S–M | AstrBot 社区有更新管理插件，TRSS 每天检查 | 用服务端内部定时循环（参照 kv_expiry），不走插件 scheduler；只需契约里的计数字段 |
| G48 | gsuid_core 桥接插件 | M | gsuid 已能经 NapCat 直连或挂接 AstrBot | 价值在统一管理和避免重复响应，而非“能不能用”；gsuid 需要另装 Python 环境（约 50 个依赖），没有正式核心版本，与“运行期不装依赖”的卖点相悖；由维护者决定覆盖哪些游戏 |
| G49 | 从 Yunzai 一次性批量迁移 | M | 竞品都没有跨框架导入 | 单项导入已有（Cookie/SToken、PlayerData、抽卡记录）；缺一次性批量导入；Miao 现在把 CK 存在 `data/db/data.db`，旧 yaml 只作兜底 |
| G50 | SDK 许可：标注 MIT 的插件二进制实际受 AGPL 约束 | S | 本赛道的 copyleft 宿主生态照样繁荣，许可不是采用的决定因素 | 改 SDK 许可（维护者是唯一版权人），或至少让 writeNotices 把 AGPL 文本带进产物；在打 sdk tag 前决定 |
| G51 | 卸载不能清除数据与凭据 | S–M | AstrBot 卸载有 delete_config 与 delete_data 两个选项 | DELETE 增加 purge_data（默认 false）；最低成本是先把 lifecycle.md:58 改成与契约一致 |
| G52 | ARM64 Linux 产物 | L | AstrBot、Koishi、LangBot、gsuid 的 Docker 覆盖 arm64；ZeroBot-Plugin 发 arm 包 | Chrome for Testing 自 153 起提供 linux-arm64，阻断条件已消失；但平台枚举在 5 份契约中，Chromium 152 属冻结版本线，11 个插件都要多出 arm64 包；与 G19 的多架构镜像一起交付时升 P1 |
| G53 | 服务端包只能 SSH 更新 | S / M | 只有 AstrBot（非 uv、非 Docker 安装）与 Yunzai 能在应用内更新服务端 | 先按 `update_mode=manual` 在 Web 展示命令；“安装并重启”需要 CLI 契约加 `--if-pending`，与 G19 合并设计 |
| G54 | 在线备份没有定时、轮转、下载 | M | AstrBot 有 WebUI 备份恢复，gsuid 有定时备份与下载 | 现有 scheduler 归属插件，核心定时备份应另起内部循环（参照 RunSnapshotLoop）；下载前提示含明文平台密钥 |
| G55 | 缺 WebView2 时没有原生提示 | S | — | 已落地（选型复核 C5）：创建窗口前检查 WebView2，缺失时显示原生安装提示 |
| G56 | 资源占用与端到端基线 | S–M | — | 维护者本机已有仓库外的消息热路径基准（62 个场景，NativeEcho 含 P50/P95/P99），但日志写入丢弃、不含持久化，也没测 RSS 与冷启动。补四项：空载 RSS 与冷启动（含 FFmpeg、Chromium 准备）、装满官方插件后的 RSS、渲染峰值 RSS、开启真实日志持久化时单群突发的端到端 P50/P95、排队时间与 drops_by_reason；记录硬件、配置、插件版本与输入；第四项结果作为 G15、G59 是否升级的依据；写进 deployment.md 与 README，没有数据不宣传“低内存” |
| G57 | 插件内存与 CPU 上限 | M–L | LangBot 配置里有每插件 1 CPU、512 MB 与实例总预算，但只在云端共享 Runtime（nsjail，有 cgroup v2 委派时才有内存与 CPU 硬限）执行，自托管版以普通子进程启动插件、限额不生效 | 在这一点上 RayleaBot 与 LangBot 自托管持平；Windows 用 `JOB_OBJECT_LIMIT_JOB_MEMORY`（S）；Linux 用 cgroup v2 `memory.max`，需要委派并设计降级；不用与 Go 运行时冲突的 RLIMIT_AS |
| G58 | 过载丢弃对管理员不可见 | S | 竞品都没有直接展示丢弃数 | Web 直接消费契约已有的 `dropped_count` 与 `drops_by_reason`，不改契约；会话分道丢弃纳入统计与可配置上限放第二步 |
| G59 | 管理日志逐条同步落盘 | M | 竞品都不在消息主路径上同步持久化每条日志 | 已落地（选型复核 R2）：管理日志改为有界队列加批量事务写入 |
| G80 | 1080p 笔记本 125% 缩放落在支持范围之外 | S | AstrBot、MaiBot 在做移动端适配 | 设计规范写“1920×1080 为最低分辨率，低于该分辨率的桌面窗口不在支持范围”（`docs/design/web-management-ui.md:153`），测试按 1920×1080 CSS 视口；而 `PRODUCT.md:77` 又要求支持浏览器缩放，两处口径冲突。14–17 英寸 1080p 笔记本默认 125% 缩放，有效分辨率 1536×864，按 CSS 视口理解会被排除在外。建议把基准写清为物理分辨率，在 1536×864 下对初始化与登录、仪表盘、连接弹窗、插件列表与商店、日志与详情窗口、配置工作台做人工验收，不写视觉 E2E；不承诺 1366×768 与 150% 缩放；发现整页横向滚动或操作被遮挡时升 P1。布局没有页面级最小宽度，唯一断点在 2300px，实际渲染尚未验证 |
| G81 | 日志正文的隐私与保留 | S–M | — | 入站、出站与策略拒绝日志都记录消息正文（`server/internal/bot/pipeline/bridge/event_log_attrs.go:40-45,68-69`，`server/internal/bot/pipeline/outbound/observability.go:105-106`），SQLite 保留 7 天，但 Launcher 的 `logs/server/` 镜像文件没有清理逻辑（`launcher/internal/desktop/process.go:485-502`），systemd 下进入 journal；用户文档没有说明。文档说明并入 G4；另提供不记录正文的日志选项（先改配置契约），并给 Launcher 镜像文件加保留期 |
| G82 | 商店与插件详情不展示截图、主页与描述 | S / M | Koishi 市场、AstrBot 市场都有条目详情 | manifest 已有 description、homepage、keywords、screenshots 字段，但 11 个官方插件都没填，Web 也不渲染截图；目录没有截图字段，商店卡片不显示 description 与 homepage，`store.fetchDetail` 没有被任何视图调用。第一步只改 Web，加商店详情抽屉；第二步给目录加 screenshots（https URL 加 alt），属 4.4 节窗口内的契约加法，并为已安装插件提供截图读取端点 |
| G83 | 按场景推荐插件组合 | S | 竞品按类别组织市场，未见按场景给出官方组合 | 等 G2 有可装组合后，用文档给出三类组合：游戏群（genshin、starrail、zzz、mihoyo-accounts、game-guide、delta-force）、内容订阅（subscription-hub）、日常工具（oil-price、fortune、echo、roulette）；每个插件写明适用协议（含 QQ 官方当前限制）、依赖插件或账号、关键命令、仓库链接与最近验证版本（来自 G1/G2 验收记录）；不新增目录字段；目录 6 条全部 `recommended=true`，没有区分度 |

## 7. P3 与有意取舍

### 7.1 P3

| 编号 | 主题 | 建议要点 |
|---|---|---|
| G60 | QQ 频道 intents 可勾选但事件全部丢弃 | 不要从枚举中删除（会让已有配置校验失败），在 Web 隐藏并在 schema 描述标注“当前不投递” |
| G61 | 出站内容过滤 | 竞品的同类能力都绑在 LLM 流水线上；等 AI 插件落地时一起设计；2025-09 NapCat 事件绕过了框架，不能作为本条依据 |
| G62 | 群临时会话回复不带 group_id | 先在 NapCat、LLBot 上实测文字与图片；SnowLuma 在临时会话里发图本来就会失败 |
| G63 | AI：共享 llm@1 服务 | 目前没有第二个调用方；`plugin.request` 不能 detach，推理模型常超 30 秒 |
| G64 | AI：MCP 客户端 | 目标用户需求弱（Koishi MCP 客户端月下载 43）；stdio 默认关闭，只读管理员手写的配置文件 |
| G65 | AI：Dify、Coze、n8n Runner | 在 raylea.ai 预留接口即可；存量用户与 QQ 游戏群主重合度低 |
| G66 | AI：TTS、STT、生图 | FFmpeg 没有 SILK 编码器，QQ 官方语音要求需另行核实 |
| G67 | 事件预处理与出站装饰钩子 | 子进程同步改写会放大延迟和故障面；优先 Server 声明式规则 |
| G68 | llms.txt 与插件作者 skill | LangBot、Koishi、MaiBot 已提供；在主仓库生成 llms-full.txt，模板仓库发布时复制 |
| G69 | 插件 KV 只读浏览 | 只做只读，宿主侧删除会绕过插件的不变量；预览套用日志脱敏 |
| G70 | 黑名单到期时间 | 插件已能用拉黑加 scheduler 到期删除变通；每日次数并入 G29 |
| G71 | 目录风险与弃用标记 | 需要先让目录读取宽松；只有官方插件时边际价值低，与 G44 一起做 |
| G72 | 目录下载量与 stars 信号 | 由目录 CI 写入，不做实例遥测 |
| G73 | Linux 一键安装脚本 | G17 文档落地后边际价值低；ZeroBot 式 deb/rpm 更可比 |
| G74 | Launcher 恢复与迁移向导 | restore 只写入未启动过的新目录，“切换安装根”会牵动一键更新，工作量 L |
| G75 | 远程渲染后端（browserless 类） | 需要新增配置契约并写明信任边界；不引入第二套渲染引擎 |
| G76 | 诊断包加入 Go 运行时 profile | 已落地（选型复核 C4）；进程 RSS 视图已被复核剥离，不重提 |
| G77 | macOS 首启放行说明 | 部署文档已补充 quarantine 放行步骤，包内应用改为完整的 ad hoc 签名（选型复核 C6）；能否改用系统设置中的“仍要打开”待 Apple Silicon 实机验证；macOS 下载量为 0，support_level 标 experimental |

### 7.2 有意取舍

| 编号 | 取舍 | 竞争代价 | 建议与重新评估条件 |
|---|---|---|---|
| T1 | 只接 QQ；平台只能由核心实现 | 平台数 2，对比 AstrBot 17 种平台类型、LangBot 21 个适配器配置、Koishi 15 个官方适配器、NoneBot2 32 个适配器（口径各不相同）；对 QQ 游戏群主几乎没有损失 | 保持。若扩展，按性价比排序：QQ 官方深度 → 微信 iLink（官方、仅私聊）→ KOOK/Discord；每个新平台都要扩展闭合枚举并写验证矩阵 |
| T2 | 个人号只押 OneBot11，不接 Milky、Satori、OneBot12 | Lagrange 已转 Milky；NapCat 仍拒绝 Milky | 维持“考虑”；把 SnowLuma 加进实现端识别与兼容矩阵（S）；NapCat 停止 OneBot11 或长期不可用时重新评估 |
| T3 | QQ 官方只走 WebSocket | AstrBot、NoneBot2、LangBot 两种都支持；LangBot 默认 WebSocket，AstrBot 把 WebSocket 标为推荐 | 平台目前 WebSocket 与 Webhook 可切换，官方事件文档未见弃用 WebSocket 的表述；平台再次宣布弃用 WebSocket 时实施（需 ed25519 验签与回调验证） |
| T4 | 用户标识不跨协议、不跨实例 | 插件主键含实例 id，换 bot 账号或改实例名都会丢绑定 | 不建宿主统一用户体系；插件侧提供一次性绑定码迁移 |
| T5 | 只有 Go SDK | Python、TS 生态在插件数量上占绝对优势 | 不恢复托管运行时；写一页“非 Go 插件”；bun、deno 产物通常 70–110 MB，超出包内单文件 64 MiB 上限（`docs/plugin/lifecycle.md:74`）；一致性执行器在复核中最终为保持，重提需要新证据（如 v0.4.0 公开数月后仍没有第三方 Go 插件） |
| T6 | 事件、命令、路由只能静态声明 | 不能运行时注册 matcher | 换来安装前冲突检测与自动帮助菜单，写进插件开发首页作为差异点 |
| T7 | Web 不提供手机、平板与窄屏布局 | AstrBot、MaiBot 都在做移动端适配 | 维持；移动场景交给聊天运维（G34）；窄屏页与 `PRODUCT.md` 的范围冲突，需维护者先改产品范围。1080p 笔记本 125% 缩放不属于本条取舍，按 G80 纳入支持 |
| T8 | 插件是完全可信代码，没有 OS 沙盒 | 同类中常态；子进程模型与 LangBot、MaiBot 同档，强于同进程框架 | 先改措辞（S）：`docs/architecture/README.md:84` 与安装确认文案写明子进程只隔离崩溃，插件可以读取本机全部 RayleaBot 数据与凭据；资源上限归 G57；Landlock 等文件系统限制在第三方源出现前再评估 |
| T9 | 平台密钥原值存储，备份含明文凭据 | 与 AstrBot、Koishi、Yunzai 持平；配置文件只存 `secret://` 引用，略优 | 在备份确认文案与 `docs/user/recovery.md` 写明含明文凭据；`backup --exclude-secrets` 会产生不完整的备份，收益与成本不匹配 |
| T10 | 不做 Agent 代码执行、Computer Use 与本地 Shell | AstrBot、LangBot、Nekro 有，同时也是 CVE、docker.sock 挂载与 Windows 故障的来源 | 在 raylea.ai 文档列为非目标；确有需要时只经远程 MCP 连接用户自管的外部沙盒 |
| T11 | 不恢复 Prometheus 端点 | 主要竞品核心都没有，代价很小 | 落地 C4 的运行时 profile 即可 |
| T12 | 不签名、不公证 | 未签名摩擦在生态里普遍存在 | 补放行文档（G77）与 WebView2 提示（G55） |
| T13 | 不做多语言界面、多管理员与组织 RBAC、多租户、SSO 与计费 | 目标用户几乎都用中文；AstrBot、gsuid 也是单管理员；LangBot 已做 Workspace 多租户基础与 5 种角色 | 补一份英文 README；t 函数改为按键路径约束的类型化写法，保留扩展余地；出现明确的多人运维或组织需求时再评估 |
| T14 | 不做集群、高可用与远程状态库 | 竞品有 K8s 清单，但未见验证过的高可用 | 与单实例 SQLite 和状态归属冲突（章程第 25、91 行）；单机瓶颈与可用性目标有数据支撑时再评估 |
| T15 | 不做可视化工作流画布 | LangBot、AstrBot 经 Runner 接 Dify、n8n 等外部编排；Kirara AI 有工作流编辑器但已停更 | 外部编排经 G65 的 Runner 接入验证；用户反复需要本地复杂编排、且调度加插件无法满足时再评估 |
| T16 | 不做直接运行 Yunzai 或 NoneBot 插件的兼容层 | Yunzai 生态迁移最直接的路径；ALemonJS 等项目在做加载 Yunzai 插件 | 与原生 artifact、运行期不装依赖的模型冲突；迁移靠命令对照（A11）、数据导入（G49）与协议级桥接（G48）；出现明确的高价值插件迁移专项时再评估 |

## 8. RayleaBot 的优势与对外口径

| 编号 | 优势 | 相对谁成立 | 口径限定 |
|---|---|---|---|
| A1 | 插件子进程崩溃隔离；自包含原生包，没有依赖地狱，运行期不装依赖；安装先 staging 再原子替换，初始化失败回滚 | 相对 Yunzai、NoneBot2、Koishi、AstrBot、gsuid 的同进程、共享环境模型 | 只能说“崩溃隔离”：没有资源上限（G57），高优先级插件挂起会经优先级分层拖慢同一消息的低层插件（G11）；插件可以读状态库，不是安全边界；LangBot、MaiBot 同样是独立进程，LangBot 自托管版同样不限资源；来源变化只是再次确认，目录摘要只保证完整性，不能说防投毒 |
| A2 | 契约先行：事件、消息段、动作、错误码都有 schema 与 fixtures，生成物有漂移校验；出站失败语义明确（`send_unconfirmed` 禁止重发，额度用尽与回复窗口过期分开） | 契约先行与漂移校验在同类中少见 | 测试数量不独占领先：LangBot 有 415 个插件集成测试，NoneBot2 有 NoneBug；正式契约是 13 份约 1.22 万行；宣传前要清理第 12 节列出的漂移并让 nightly 转绿；QQ 官方侧的失败语义受 G3 削弱 |
| A3 | OneBot11 接入：四种传输可同时启用并按事件 ID 去重，多实例隔离，实现端自动识别加兼容矩阵，令牌存 secret store | 相对 AstrBot（只支持反向 WS）与 TRSS | 不能说“最完整”：ZeroBot 同样有 4 类 driver，NoneBot2、AstrBot 能调任意动作，RayleaBot 缺踢人；OneBot11 实际映射 25 种事件；动作名闭合但参数透传；兼容矩阵已从管理面移除，只在文档里 |
| A4 | 宿主内置 HTML 模板渲染：模板发现、输入 schema 校验、资源预取、字体与主题、Web 预览 | 相对 AstrBot（插件渲染走 t2i 服务）、NoneBot2、Koishi、LangBot | 与 Yunzai 相当，TRSS 还有 shotium 与远程 browserless；“可离线”需首次准备 Chromium；缺 Markdown 模板 |
| A5 | 由宿主认定调用方身份的版本化服务调用；米游社凭据只留在账号插件里 | 相对同进程框架；gsuid 所有插件共用核心数据库 | MaiBot 有接近的跨进程 API；凭据隔离是协议约束，不是 OS 隔离（DPAPI 主密钥同一用户可解）；只允许一跳，限制 AI 工具底座（G36） |
| A6 | 零间隙的进程级热重载：新进程握手成功才切换，失败保留旧版本；开发态增量在线同步 | 相对 NoneBot2（整进程重启）、Yunzai（仅单文件 JS） | 不能说“不打断会话”：detach 事件、session.wait 对话和未完成的 plugin.call 会结束；MaiBot 同一水平 |
| A7 | 运维可靠性：healthz/readyz 分级、诊断包、严格校验的空目录恢复、nightly 恢复演练 | 分级探针加严格恢复演练的组合在竞品中未见 | LangBot 也有 `/healthz`（Core、Plugin Runtime、Box）与 Box 的 `/readyz`，K8s 清单配了探针，但 Core 没有就绪检查，也没有用户可用的备份恢复；备份易用性落后 AstrBot（WebUI 备份恢复）与 gsuid（定时备份、下载）；演练只覆盖离线 backup |
| A8 | 管理面认证默认安全（无默认口令、每实例随机密钥、argon2id、SameSite=Strict、CSRF 与 Origin、一次性 setup token）；日志、插件控制台按米游社与 B 站 Cookie 字段做领域化脱敏 | 相对 TRSS、旧版 NapCat 与 LangBot 的历史漏洞 | 与当前版 AstrBot 大体持平，还少一个 TOTP；先修 G10，否则会被指“管理面安全、机器人入口不设防”；只覆盖管理面，不含聊天入站 |
| A9 | 管理面可追溯：策略拒绝记录 policy_stage、错误码与匹配插件，单个插件的处理与终端回复共用 request_id；配置逐字段标注生效方式（5 种 x-apply-policy）；治理内置并按实例隔离、fail-closed；插件异常按错误码展示并可就地恢复 | 相对 NoneBot2、Yunzai（核心无 Web）；竞品的单消息追踪只覆盖 AI 调用，RayleaBot 覆盖策略与插件分派，两者互补 | 入站与策略拒绝日志没有关联 ID，不能串起整条消息（G79）；日志检索落后 gsuid 与 guoba-next（G21）；相对 AstrBot、LangBot、gsuid 治理基本持平，Koishi 有按指令的每日配额 |
| A10 | Go 单二进制，不依赖 Python、Node 或 Redis，Chromium 与 FFmpeg 自动准备 | 相对需要语言运行时与 Redis 的 Yunzai、NoneBot2、Koishi | 发布构建未设 `CGO_ENABLED=0`，Linux 包可能依赖 glibc；Docker 用户看到的差异会被稀释；没有实测前不宣传“低内存”（G56） |
| A11 | 游戏垂类与订阅解析：genshin、starrail、zzz 按 Miao-Yunzai 写法实现命令，许可链可追溯到上游固定提交；subscription-hub 覆盖 B 站、微博、抖音的订阅推送与链接解析 | 相对 Yunzai 迁移用户 | 尚未发布、仍在验收，命令与上游存在有意差异，需附对照表；GenshinUID 的素材授权列得同样完整；链接解析少小红书、快手 |

另外，桌面一键更新（下载、停服、原子替换、续接服务、不碰用户数据）与 AstrBot、Koishi 持平，领先 Yunzai、LangBot、NoneBot2，但这条链路从未被真实用户走过：v0.3.1 是另一套带签名的更新器，v0.4.0 又尚未发布。在 v0.4.0 到下一个补丁版的实包演练完成前，不应列为卖点。

## 9. 定位与推进顺序

### 9.1 定位判断

1. **核心用户**：自托管的 QQ 群群主，以米哈游游戏和 B 站、抖音订阅解析的娱乐群为主；部署以 Windows 桌面为主，家用 NAS 与小 VPS 次之；插件开发者是次要用户，AI 助手用户与企业 IM 用户暂不作为目标。依据：11 个官方插件里规模最大的四个是原神、星铁、绝区零、米游社账号，subscription-hub 第五；只接 QQ、只有 Go SDK、Web 只支持桌面，这些取舍都与这类用户一致。README 首段应据此改写。
2. **正面对手**：功能赛道上是 Yunzai 系与 gsuid_core。gsuid 已能挂接 AstrBot 并经 NapCat 直连，进一步削弱了“原生移植”的稀缺性。“群主默认会选谁”这一层是 AstrBot：4.1 万 stars、约 2,385 个插件，有 WebUI、首次登录引导、桌面版、面板应用，还有 AI。NoneBot2 与 Koishi 是开发者框架，RayleaBot 只有 Go，没必要和它们抢开发者。
3. **差异化主张**：做“Yunzai 游戏生态的工程化替代”。命令与数据保持兼容，同时提供 Yunzai 没有的东西：Web 管理、崩溃隔离、不依赖 Redis 的单二进制、安装事务、诊断与恢复。对 AstrBot 不比 AI 和平台广度，只比稳定性、凭据边界与游戏深度。这一主张有三个前提：发布链打通（G1、G2）；迁移摩擦降低（G5、G6、G49；走 QQ 官方的还需 G3）；“低内存、一键部署”有实测与 Docker 支撑（G56、G19）。按迁入来源分列如下：

   | 迁入来源 | RayleaBot 需要先具备的条件 |
   |---|---|
   | Yunzai（Miao、TRSS） | 附与上游有意差异的命令对照表（A11）；这类用户多走 OneBot（NapCat），关键是 G5 媒体路径、G6 to_me 与仅响应@、G7 按群开关、G49 批量导入；走 QQ 官方的另需 G3。不把“聊天内安装与更新”列为前提，Web 商店已覆盖，聊天运维归 G34 |
   | gsuid_core | 抽卡记录走 UIGF（genshin 已支持导入导出），账号可扫码或用 Cookie/SToken 导入，不需要专用导入器；差距在游戏覆盖（G48，鸣潮现行插件是 XutheringWavesUID）、控制台的日志检索与定时备份（G21、G54）和 AI（G9） |
   | 新用户 | G1、G2 不可用、首次使用没有引导（G78）时，判断会被推向 AstrBot 桌面版或 Docker；gsuid 依赖宿主框架或 Docker 镜像，并非单独的一键部署 |
4. **AI 立场**：建议在章程中明确“AI 由官方插件提供，宿主不内置 LLM、Agent、MCP 代码”，不再留空。切入点是对话 MVP（G9），之后是游戏数据问答（G36，受一跳限制）和拟人群聊（G38）。
5. **协议路线**：OneBot11 为主路径，QQ 官方作为合规对冲。2025-09 的封号事件说明非官方协议端风险真实存在，而 2026 年 QQ 官方已向个人开放建 Bot、进群、全量消息、群内主动推送与 Markdown。需要维护者决定的是：QQ 官方是否成为首选上手路径。若是，G23、G24 一并升为 P1。
6. **生态策略**：走“官方主导的精选集”路线（ZeroBot-Plugin、gsuid 的模式），不与 Koishi、AstrBot 比开放市场的数量。邀请第三方之前，先完成 G12 稳定承诺和 G2 的目录宽松读取，再做 G44 社区目录。gsuid 桥接（G48）可以短期补齐鸣潮等游戏的覆盖，但要先想清楚它与原生移植的关系。
7. **部署主形态**：近期以桌面为主路径（Launcher 相对 Yunzai、NoneBot2 是差异点），用“卷内放发布包”的 Docker 方案补上服务器路径（G19），ARM64 随多架构镜像一起交付（G52）。避免两头都做不深。

### 9.2 推荐推进顺序

| 阶段 | 内容 | 说明 |
|---|---|---|
| v0.4.0 公开发布前 | G1、G2、G3、G10（默认生成令牌加非回环强制）；4.4 节的契约窗口决策（含 G82 的目录截图字段）；G12 稳定承诺；R4 会话令牌（G32 第一步）；开启私密漏洞报告；设置仓库描述与 topics；修正 README | 以 S、M 级为主，决定 v0.4.0 能否作为可用的公开版本 |
| v0.4.0 公开发布时 | G1、G2 的端到端验收 | 从公开下载入口出发，结果登记为验收记录 |
| 发布后 1–2 个版本 | G4、G78、G5、G6、G7、G8、G11、G14、G15、G16、G17、G18、G21、G79 第一步、G13；游戏插件分批上架 | 降低首次部署、日常调参与排障摩擦；G78 排在 G2 之后 |
| 其后 | G9 AI 对话 MVP、G19 Docker（单独设计）、G20 插件覆盖与 G83 推荐组合 | 补齐产品面的主要缺口 |
| 按需 | 第 6 节其余 P2 条目，其中 G52 随 G19 推进，G23、G24 随 QQ 官方定位调整，G80 的人工验收可随下一次 Web 改版做 | — |

## 10. 被校验推翻或收窄的说法

| 原说法 | 核实结论 |
|---|---|
| OneBot11 接入面在 QQ 生态竞品中最完整 | 收窄为传输并行去重、多实例隔离、实现端识别与凭据管理领先；动作覆盖不如可调任意动作的 NoneBot2、AstrBot 和声明支持完整 OB11 加 NapCat 扩展的 ZeroBot |
| 入站 37 种事件、43 个强类型动作 | 37 是全词表，其中 8 个宿主内部事件、4 个只属于 QQ 官方，OneBot11 实际映射 25 种；动作名闭合，但参数原样透传（`contracts/plugin-protocol.schema.json:1311-1314`） |
| 踢人动作在 manifest v3 合同重置时被删除 | 踢人从未支持过；`dc47f044` 删除的是把 kick 当“不支持动作”的反例 fixture |
| QQ 官方原生 Markdown 需要日活 2000 | 原生 Markdown 已对所有机器人开放，模板 Markdown 才需申请 |
| QQ 群 v2 没有群管接口 | 2026-08-10 新增禁言与入群审批（文档未标内邀，机器人需为群管理员）；2026-09-03 新增成员列表、批量移除与黑名单，处于内邀，未开通返回 11253 |
| AI 风险被限制在插件进程内 | 只能主张“崩溃隔离加宿主没有 Agent 执行面”；插件没有资源上限，能读状态库里明文存放的 secret |
| 进程隔离、本地 Chromium 渲染是独有优势 | LangBot、MaiBot 同样每插件一个进程；Yunzai 核心有 puppeteer 与 shotium，MaiBot 宿主有 html2png，NoneBot2 有 htmlrender |
| 宿主不记录消息正文 | bridge 按 info 级别记录已投递消息的 plain_text、segments、raw_message（`server/internal/bot/pipeline/bridge/event_log_attrs.go:40-45`），进入管理日志保留 7 天；只有后台事件与服务路由日志不记正文 |
| 游戏插件可以把需要 CK 的数据暴露为 AI 工具 | 服务处理器不能再发起 plugin.call（`server/internal/plugins/runtime/services.go:45`，`docs/plugin/protocol.md:115`） |
| 热重载不会打断进行中的会话 | detach 事件以 `plugin.event_canceled` 结束，session.wait 对话被清理，未完成的 plugin.call 被结束 |
| game-guide 订阅名拼错导致收不到身份更新 | `bot.identities.changed` 是定向控制事件，不看订阅照常投递；拼错的订阅只是无用配置 |
| 账号只能扫码、抽卡只能在聊天里导入、不读 PlayerData | mihoyo-accounts 已有 Cookie 与 SToken 导入，genshin、starrail 已能导入旧 PlayerData，管理页也有抽卡导入 |
| 缺账号插件时用户不知道原因 | genshin 会明确提示“米游社账号插件未运行，请先启用并扫码登录” |
| 合同变动让群主每次升级都集体禁用插件 | 0.3.1 之后的候选从未公开，公开用户只会经历一次跳变；属于前瞻性风险 |
| LangBot v3→v4 插件生态大幅缩水 | 从约 119 个重建到 96 个，降幅约两成，而且仍在增长 |
| Koishi 的治理靠老化的社区插件 | rate-limit、dataview、sandbox、analytics、bind 都在官方模板的默认依赖中 |
| SECURITY.md 是竞品普遍做法 | AstrBot、LangBot、Koishi、NoneBot2、TRSS、MaiBot 都没有 |
| OneBot 空令牌与会话密钥明文入库比竞品弱 | 与 AstrBot、TRSS 持平；只比默认回环的 NoneBot2、Koishi 弱；0.0.0.0 默认值只存在于未发布的候选 |
| `/readyz` 可用于 Docker 探活 | 文档写的是 `/healthz` 适合 systemd、Docker、LXC；`/readyz` 在 `setup_required` 时返回 503 |
| 契约 16 份共 12,412 行；测试规模高于全部对标项目 | 正式契约 13 份约 1.22 万行（其余为 AGENTS、CLAUDE、README）；LangBot、NoneBot2 的测试工程同样扎实 |
| 一键更新与备份恢复全面领先 | 一键更新与 AstrBot、Koishi 持平；备份易用性落后 AstrBot 与 gsuid |
| gsuid 核心带 41 个游戏插件 | 插件是独立仓库；文档列表 41 个（工具 34、娱乐 7），列表页约 52 项 |
| Karin 兼容 Yunzai 插件与数据；TRSS 2026-04 新增 autoInvite | 都没有一手证据；TRSS 的对应能力是 `other.yaml` 中一直就有的 autoFriend、autoGroup、autoQuit |
| 出站内容过滤能防住 2025-09 NapCat 事件 | 攻击者直接调用协议端的 OneBot 端口，完全绕过框架 |
| QQ 官方超级管理员是低成本缺口 | 要改 config 与 plugin-protocol 两份契约、SDK 和插件，工作量 M |

以下来自与两份独立报告的交叉核实，包括本报告初稿与对方报告各自被更正的说法：

| 原说法 | 出处 | 核实结论 |
|---|---|---|
| 日志用 request_id 串联整条消息；现有日志已基本能定位单条消息的原因 | 本报告初稿 A9；astra | request_id 在插件投递时才生成，入站与策略拒绝日志是保留值 `system`，没有 event_id；前缀不匹配等原因只在 debug 级，见 G79 |
| 健康端点与严格恢复在主要竞品中都没有 | 本报告初稿 A7 | LangBot 有 `/healthz` 与 Box 的 `/readyz`，K8s 清单配了探针；分级探针加严格恢复演练的组合仍未见 |
| LangBot 插件配额默认是软限制 | 本报告初稿 G57 | 自托管 OSS 版不施加任何资源限制，配额只在云端共享 Runtime 执行 |
| QQ 官方群管接口（含禁言、入群申请）都在内邀 | 本报告初稿 | 禁言与入群审批（2026-08-10）文档未标内邀；移除成员与黑名单（2026-09-03）标注内邀，未开通返回 11253 |
| QQ 官方已补齐群管接口 | DeepSeek | 只补齐一半，移除成员与黑名单仍是内邀 |
| NoneBot2 有 16 个官方适配器 | 本报告初稿 | registry 中 32 个适配器、15 个官方 |
| AstrBot 有 18 个平台适配器（初稿）；14 个官方适配器、文档枚举 19 个 key（DeepSeek） | 本报告初稿；DeepSeek | 17 种可创建平台类型（16 种外部接入），README 标为官方维护 14 项另有 3 个社区适配器；“19 个 key”找不到来源 |
| AstrBot 只有 2 条安全公告 / 有 24 条安全公告 | DeepSeek；本报告初稿 | 两者口径不同：仓库自发 2 条，Advisory Database 检索 24 条（16 条为未审核的 VulDB 低危条目） |
| 契约约 1.18 万行 | DeepSeek | 13 份正式契约 12,240 行，约 1.22 万行 |
| SendText 1,072 处、Send 56 处、25 处以上 base64 图片 | DeepSeek | 1,072 混入了被 gitignore 的本地草稿目录；Send 在任何口径下最多 52；非测试代码 23 处 `base64://`，其中 20 处图片、3 处文件段 |
| LangBot 市场约 458 个插件 | DeepSeek | 458 来自 sitemap，是插件 96、MCP 215、Skill 146 的混合口径；插件为 96 个 |
| ChatLuna 月下载约 3.3k / 约 980 | DeepSeek；初稿调研 | 都成立：npmjs 官方统计 3,272，Koishi registry（约等于 npmmirror）990 |
| gsuid 记忆分 GROUP、USER、GLOBAL 三级 | DeepSeek | 代码中是 group、user_global、user_in_group、self 四类作用域 |
| OneBot11 有心跳看门狗，超时三倍即重连 | DeepSeek | 是对任意帧的读超时（心跳间隔的 3 倍）；正向 WS 会重连，反向 WS 只回到监听；宿主不读心跳里的 `status.online`，发现不了“有心跳但账号离线”（G25） |
| 复用 manifest 已有的截图与项目资料字段即可 | astra | 字段在契约里，但没有插件填写，Web 不渲染截图，目录没有截图字段，见 G82 |
| 首次使用任务清单是对标竞品 | astra | 只有 AstrBot、LangBot 两个 AI 平台有向导；同赛道游戏框架都没有，属于 RayleaBot 自己的设计，见 G78 |
| 首次使用清单为 P0 | astra | 不阻断交付，阻断交付的是 G1、G2；定为 P1 |
| 迁入 RayleaBot 需要聊天内安装与更新；QQ 官方修复是 Yunzai 用户迁入的前提 | DeepSeek | Web 商店已覆盖安装与更新，聊天运维归 G34；Yunzai 用户多走 OneBot，关键是 G5、G6、G7，G3 只影响走 QQ 官方的用户 |
| Mirai 最后 Release 在 2024-10-22 | 初稿调研 | v2.16.0 发布于 2023-10-22，最后提交 2024-09-23 |

## 11. 与工程选型复核重叠的条目

以下条目同属工程选型复核（2026-10）的结论，已于 2026-10-07 全部落地，实施结果见 [v0.4 变更记录](../CHANGELOGS/v0.4.md)；本节保留竞品视角的补充。

| 复核条目 | 本报告对应 | 竞品视角补充 |
|---|---|---|
| R2 管理日志异步批量 | G59 | 竞品都不在消息主路径上同步持久化每条日志 |
| R3 SQLite 每连接 PRAGMA | —（实现缺陷） | 核实中两个视角都认为是 S 级正确性修复 |
| R4 会话令牌哈希存储 | G32 | AstrBot 同样把 jwt_secret 明文写入配置；属于纵深防御 |
| R7 FFmpeg 不阻塞首启 | G16 | 国内群主是主力用户，首启不应等待大文件下载 |
| C4 诊断包加入运行时 profile | G76 | 主要竞品核心都没有指标出口 |
| C5 WebView2 缺失提示 | G55 | 属缺陷，违反 Launcher 的“不静默失败”规则 |
| C6 macOS 首启放行 | G77 | macOS 下载量为 0 |
| 复核第 7 节：golangci-lint 不支持 Go 1.27；插件工作流引用不存在的 SDK tag；linux-server 包缺 Chromium 共享库说明 | G1、G2、G17 | 已于 2026-10-06 修复；nightly 失败面比复核记录的更广，见 G1 |

## 12. 顺带发现的现存缺陷（与竞品无关）

| 问题 | 位置 |
|---|---|
| webhook 正文上限：契约允许声明到 10 MiB，实现静默截到 1 MiB | `contracts/plugin-info.schema.json:205`；`server/internal/platform/httpapi/httpapi.go:27`；`server/internal/plugins/webhook/webhook_http.go:62-66` |
| 生命周期文档写运行中通过 ping/pong 保活，宿主从不发送 ping | `docs/plugin/lifecycle.md:27`；见 G11 |
| 生命周期文档写卸载“按卸载接口的正式选项处理”，契约中没有任何清除选项 | `docs/plugin/lifecycle.md:58`；`contracts/web-api.openapi.yaml:2084-2116` |
| 用户文档写“OneBot11 设置包括 provider”，schema 中没有该字段（实现端由运行时自动识别） | `docs/user/configuration.md:94` |
| 内置菜单渲染失败时日志写“已改用文字菜单回复”，实际只发送固定报错 | `server/internal/bot/menu/menu.go:133-138` |
| `plugin.list` 判断超级管理员时没有像权限检查那样限定 `source_protocol==onebot11`，只影响可见性 | `server/internal/plugins/actions/plugin_list.go:116`；`server/internal/bot/permission/checker.go:66` |
| README 仍写官方插件页面“运行在独立插件域”，独立插件域已在 v0.4.0 中删除 | `README.md:15`（以 HEAD 为准） |
| 插件仓库工作流 `GO_VERSION: 1.26.6`，`sdk/go/go.mod` 要求 go 1.27.1（未实跑） | 各插件仓库 `.github/workflows/release.yml`；`sdk/go/go.mod` |
| 脱敏规则缺 `e_hk4e_token` | `server/internal/platform/redact/sensitive_text.go` |
| Launcher 写入的 `logs/server/` 镜像日志只做凭据脱敏、含完整聊天正文，且没有清理逻辑，不受 7 天保留期约束 | `launcher/internal/desktop/process.go:485-502`；见 G81 |
| 设计规范把 1920×1080 写成“最低分辨率”并排除更小的桌面窗口，`PRODUCT.md:77` 又要求支持浏览器缩放，两处口径冲突 | `docs/design/web-management-ui.md:153`；`PRODUCT.md:34,77`；见 G80 |
| 插件详情 API 返回 screenshots，但没有截图文件的服务端点，Web 也不渲染；locale 中“截图”相关文案没有被任何组件引用 | `contracts/web-api.openapi.yaml`（PluginDetail）；`web/src/locales/zh-CN/plugins.ts:146-152`；见 G82 |

## 13. 证据与方法

**流程**
- 盘点：3 个代理只读盘点 RayleaBot 的聊天接入、插件平台、管理与运维三块，每条能力与限制都附仓库证据，并区分文档声明的取舍与“只是没做”。
- 调研：7 个代理联网调研 AstrBot、Yunzai 系、NoneBot2、Koishi、LangBot、其他同类项目与协议生态。优先一手来源（GitHub 仓库、Releases、API、官方文档、市场接口），每个竞品覆盖 16 个维度。
- 分析：7 个维度簇（平台与消息、AI、生态、插件开发、管理面、部署运维、安全与质量）各由一个代理产出差距、劣势、优势与取舍，共 114 条；完整性审查补充 16 条，并标出 19 组重复与 10 条可疑结论。
- 核实：每条结论由两个视角对抗核实。仓库视角逐条打开文件与行号，评估架构可行性、契约影响与工作量；市场视角联网核对竞品说法与优先级。共 260 票：仓库视角确认 70、更正 59、无法核实 1；市场视角确认 62、更正 68；没有整条推翻的结论。优先级票中 44 票下调、5 票上调。
- 合并：按审查意见合并重复项。两个视角优先级不一致时，取与目标用户和单人维护现实更一致的一侧，分歧在条目中注明。关键的新发现（G3 的回复路径与 base64、商店目录严格读取、默认监听地址的来历）由主持人回到代码再次确认。
- 交叉核实（2026-10-05）：把两份同期独立竞品分析（astra、DeepSeek）中与本报告不同或本报告没有的 45 项说法拆成仓库产品事实、仓库数字与位置、AI 平台、开发者框架与公开面、QQ 与游戏生态、产品判断六组，各由一个代理独立核实，不因任何一份报告的写法采信。成立的内容并入正文（G78–G83、T14–T16、G1/G2 验收、迁移表等），两边被更正的说法列在第 10 节。NoneBot2 官方适配器数两个代理结论不一，由主持人直接读取 registry 确认为 15 个。

**经联网核实的主要外部事实**

| 事实 | 核实结论 |
|---|---|
| AstrBot | 41.4k stars；v4.28.2（2026-09-27）；云市场约 2,385 个插件（2026-10-05）；17 种可创建平台类型；仓库自发安全公告 2 条，Advisory Database 检索 24 条；API Key 自 v4.18.0；WebUI 支持 TOTP 与首次登录三步引导；v4.29.0-beta.1 才恢复 QQ 官方群聊 @（`<qqbot-at-user>` 文本链） |
| Yunzai | Miao-Yunzai 1,074 stars、TRSS 625；版本号停在 3.1.3；插件索引约 385 条；TRSS 2026-03 适配 Milky，2026-08-31 引入 shotium |
| NoneBot2 | 7.7k stars；v2.5.0（2026-04-01）；registry 942 条（616 条有效），显式声明支持 OneBot V11 594 条、QQ 官方 221 条、Milky 147 条，另有 336 条不限适配器；32 个适配器（15 个官方）；adapter-qq 1.7.3（2026-09-26）；官方 nb-cli-plugin-webui 停在 0.4.2（2024-04） |
| Koishi | 6.2k stars；4.18.11（npm 2026-02-27）；registry 4,722 个（verified 85、insecure 112、deprecated 275，2026-10-05）；ai 分类 409 个包；默认配置的 oobe 插件是空实现；控制台任意文件读取于 2025-11-17 修复 |
| LangBot | 主仓 Apache-2.0，插件运行时 AGPL-3.0；Docker 发布标签含 amd64/arm64；K8s 清单为 Core、Plugin Runtime、Box 配了 `/healthz` 探针；自托管版不限制插件资源；Space 市场插件 96、MCP 215、Skill 146 |
| gsuid_core | GPL-3.0；AI 基于 PydanticAI，含 MCP 客户端与可选 MCP server、Skills、按会话 token 预算；非可信来源连接必须带 WS_TOKEN，令牌为空即拒绝 |
| LangBot | 18k stars；v4.10.11（2026-09-12）；市场 96 个插件；v4.3.0 起每插件独立进程；4 条安全公告（3 条有 CVE） |
| gsuid_core | 无核心版本发布；ai_core 的 budget 模块 2026-06-20 加入，quota_guard 2026-09-29 加入；Docker 支持 amd64/arm64 |
| QQ 开放平台 | 事件表含 GROUP_MESSAGE_CREATE（2026-07-21 更新）；被动回复 5 分钟、每条最多 5 次；群主动消息认证 60 条/分钟、未认证 30 条/分钟、每群每天 1000 条；原生 Markdown 对所有机器人开放；单聊流式只能用于被动回复；IP 白名单对新机器人默认开启 |
| 协议生态 | NapCat v4.18.30（2026-10-05）；LagrangeV2 于 2026-07-23 归档；Milky 规范 1.3.0；NapCat #1024 以 not planned 关闭；2025-09-05 公网 OneBot 服务遭攻击事件有协议端开发者 Wesley Young 的复盘博文 |
| QQ 开放平台补充 | “群消息（全量模式）”事件页（2026-09-16 更新）；群管接口 2026-08-10 批次（禁言、入群审批）未标内邀，2026-09-03 批次（成员、批量移除、黑名单）内邀中 |
| Chrome for Testing | 自 153.0.8001.0（2026-08-11）起提供 linux-arm64；Stable 154.0.8037.92 已包含，npmmirror 已同步 |
| BtbN FFmpeg | 每月最后一个 autobuild 保留两年，日构建只保留 14 天 |
| golangci-lint | v2.13.0 起支持 go1.27，最新 v2.14.0 |
| RayleaBot 公开状态 | 公开 tag 只有 v0.3.0、v0.3.1；nightly 徽章显示 failing；GitHub API（2026-10-05）显示 0 star、0 fork、0 watcher，description、homepage、topics 为空，Discussions 未开启，community profile 健康度 25% |

**仍存在的不确定性**
- G3 的平台行为：“全量模式下 @ 消息也走 GROUP_MESSAGE_CREATE”已有官方原文；“此时不再推送 GROUP_AT_MESSAGE_CREATE”仍只有第三方实测；`mentions[].is_you` 不在官方字段表里；群管理员“允许主动发送”的默认状态与单聊主动消息的月度配额数字未核实。
- QQ 官方 2026-08-10 批次的群管接口（禁言、入群审批）是否对所有机器人开放，仅凭文档未标内邀推断，未实测。
- LangBot 的首次使用向导只在 master 分支确认，尚不确定已进入稳定版；Koishi、gsuid“没有向导”“没有链路视图”只限于本次读到的官方资料。
- G80 依据的 Windows 默认缩放来自微软归档文档的自动 DPI 表；1536×864 下的实际渲染没有启动应用验证。
- G24 依赖的 member_openid 是否跨群一致，官方新旧文档说法不一，需要实测。
- QQ 官方 2026-08 起 WebSocket 与 Webhook 可自由切换，来自社区帖，没有官方公告原文。
- 2025-09 NapCat 事件的规模细节多为二手转述，事件本身已由多方印证。
- 插件市场数字是条目数，包含弃用、不安全和不兼容条目，不等于活跃插件数；NoneBot AI 插件数等统计口径不一。
- Linux 包是否动态链接 glibc、插件 CI 是否因 Go 版本失败，都是从配置推断，未实跑。
- 本轮没有安装或运行任何竞品，也没有做性能或资源对比；RayleaBot 有组件级的消息热路径基准（仓库外），但没有 RSS、冷启动和开启持久化的端到端数据。
- 调研中 GitHub API 多次触发匿名限流，部分数字改用网页或 raw 文件核对，少数提交日期以网页显示为准。

**后续验证**

产品没有遥测，下列指标只能靠验收记录与人工测试取得：
- 插件可用率以 G1、G2 的发布验收记录为来源，按平台、协议与插件组合统计。
- v0.4.0 公开且开通交流渠道后，邀请约 5 位目标群主做有人陪同的上手测试，记录完成情况、卡点与耗时，写明样本量；首次成功时间以 15 分钟为待验证目标，不作为承诺。
- 问题定位时间用 G79 列出的故障场景做内部演练。
- 不引入实例遥测；持续使用只看 Release 下载量、issue 与群内反馈这些弱信号。
