# 文档与运行文案整改清单

审查日期：2026-09-05。基线提交：41ced23328beaec832e9448e343234b42d6e3344。证据来自该提交上的当前工作区，包含已有的认证页面、PRODUCT.md、DESIGN.md、Web 设计规范及 sidecar 改动。文件位置与行号按本次快照记录。

共整理 35 组整改建议：P1 6 组、P2 26 组、P3 3 组。英文指定词的 36 处待处理命中逐条列在附录 A；同一句话可能同时涉及词语、句式和表达方式，数量按各自口径统计。P1/P2/P3 表示文案处理顺序。

## 审查依据与范围

正文应直接说明动作、事实、条件和结果。删去没有信息增量的强调词、重复收尾、自问自答、贬低替代做法的比较，以及与读者任务无关的“不涉及”“保持不变”说明。含混的标签应展开为具体对象和动作。英文描述中的连字符组合，优先改为普通词语和介词短语。

句子承载权限边界、失败语义、操作后果或版本迁移事实时，整改需要保留这些事实。原文中的否定词、技术术语和连接符需要结合用途判断。

扫描清单由以下命令取得，并读取各路径的当前文件内容：

```powershell
git ls-files --cached --others --exclude-standard -z
```

共读取 1,783 个 UTF-8 文本文件，372,222 行。其中包括 133 份 Markdown、6 份文本文件、18 份 README、9 份 AGENTS 和 9 份 CLAUDE。docs/ 下有 61 份 Markdown。计数包含锁文件、测试、生成文件等扫描输入；各子集存在包含关系。

| 覆盖位置 | 审查内容 |
| --- | --- |
| docs/ 全部子目录 | 产品规划、架构、设计、开发、工程、插件、用户、发布及更新历史 |
| 根目录及隐藏目录 | README、PRODUCT、DESIGN、AGENTS、CLAUDE、PR 模板、项目技能、设计说明与已纳入仓库的设计记录 |
| web/src/ | 17 份语言资源文件、页面、组件、空态、说明、确认框、错误反馈、可访问名称及硬编码文本 |
| launcher/src/、launcher/internal/ | 确认框、退出流程、状态展示、环境检查、诊断和更新提示 |
| server/ | 日志生成语句、动态消息模板、错误和诊断摘要、CLI 输出、管理事件及源码注释 |
| contracts/、fixtures/、examples/ | 可读描述、错误 message、summary、expect.notes、示例命令和页面文案 |
| scripts/、sdk/、templates/、config/、packaging/、.github/ | 开发工具输出、SDK 说明、模板固定文案、配置说明和发布内容 |
| 许可证与第三方通知 | 纳入文本扫描，按原作者和许可证文本的用途复核 |

方法为全量词语及句式检索，再核对候选所在段落、相邻规则和必要的调用点。检索覆盖指定英文表达、中文近似句式、抽象标签、无比较基线的保留说明，以及英文连字符和否定转折。下面的清单记录已经复核的整改项。

日志审查依据仓库中的生成代码和消息模板。实际运行日志、数据库记录、外部插件仓库、Git 忽略的安装产物及依赖目录未纳入本次快照；本轮没有启动各条运行分支或进行浏览器视觉验收。页面结论依据资源及调用位置。

## 整改顺序

| 优先级 | 数量 | 处理目标 |
| --- | ---: | --- |
| P1 | 6 | 先处理当前确认框、协议说明、兼容矩阵，以及会直接要求助手输出套话的技能正文 |
| P2 | 26 | 修订当前文档、诊断摘要、技能措辞和可读工具输出 |
| P3 | 3 | 清理入口元说明和历史编辑注释；更新历史按归档规则处理 |

## 逐项整改

### W01 · P1 · 停止服务确认框补充取消后的无变化说明

取消按钮已有明确含义，末句占用确认框空间。停止对象的进程来源和确认后的动作才影响决定。

| 位置 | 原文摘录 |
| --- | --- |
| [launcher/src/renderer/src/ActionConfirmDialog.tsx:39](../launcher/src/renderer/src/ActionConfirmDialog.tsx) | 该服务并非由当前启动器拉起。确认后，启动器会通过管理接口请求它安全停止；取消或关闭此对话框不会更改服务状态。 |

建议改为：“该服务由其他进程启动。确认后，启动器会请求它停止运行。”

### W02 · P1 · 退出说明重复保证文件保留

退出入口附加配置和文件保留说明，读者需要据此了解的是窗口、托盘和服务进程的退出行为。

| 位置 | 原文摘录 |
| --- | --- |
| [launcher/src/renderer/src/AppShellSettingsSection.tsx:163](../launcher/src/renderer/src/AppShellSettingsSection.tsx) | 关闭窗口和托盘入口，不影响已保存配置与服务文件。 |
| [launcher/src/renderer/src/ExitConfirmDialog.tsx:88](../launcher/src/renderer/src/ExitConfirmDialog.tsx) | 结束窗口与托盘进程，保留配置和服务文件。 |

删除两处关于配置和服务文件的后半句。分别使用“关闭启动器窗口和托盘入口。”与“结束启动器窗口与托盘进程。”

完全退出时服务进程如何处理应按现有关闭规则说明；避免用文件保留说明替代进程行为。

### W03 · P2 · 密度说明使用多余的否定转折

该提示可以直接说明紧凑模式调整的尺寸。正文与操作目标的大小属于此设置的相关信息，可正面表达。

| 位置 | 原文摘录 |
| --- | --- |
| [web/src/locales/zh-CN/app.ts:95](../web/src/locales/zh-CN/app.ts) | 紧凑模式减少留白，但不缩小正文和关键操作。 |

建议改为：“紧凑模式减少留白，正文和操作按钮使用标准尺寸。”

调用点：web/src/components/shell/PreferencesDrawer.vue:88。

### W04 · P1 · 协议页说明复述页面组织方式

“集中展示”“按类别展示”“会明确标出”描述界面如何呈现内容；操作指引可以直接说明查看和设置什么。

| 位置 | 原文摘录 |
| --- | --- |
| [web/src/locales/zh-CN/protocols.ts:21](../web/src/locales/zh-CN/protocols.ts) | 检测到当前通道存在运行异常，请根据诊断信息进行排查： |
| [web/src/locales/zh-CN/protocols.ts:24](../web/src/locales/zh-CN/protocols.ts) | 集中展示传输状态、异常反馈和通道连接设置。 |
| [web/src/locales/zh-CN/protocols.ts:38](../web/src/locales/zh-CN/protocols.ts) | 正式支持的兼容能力按类别展示，未支持项会明确标出。 |

分别建议：“连接异常，请按下方诊断提示处理：”“查看连接状态并设置连接参数。”“查看事件、消息段和动作的支持情况。”

已确认三处资源分别用于 ProtocolsView.vue:338、357 和 ProtocolCompatibilityView.vue:162。

### W05 · P2 · 账号状态说明夹带 HTTP 状态码和反向解释

普通账号页面需要解释凭据状态如何确认。单次 HTTP 403 的判定细节更适合诊断说明。

| 位置 | 原文摘录 |
| --- | --- |
| [web/src/locales/zh-CN/builtin-features.ts:61](../web/src/locales/zh-CN/builtin-features.ts) | 插件遇到平台接口拒绝时会请求服务器复检。单次 HTTP 403 不会直接判定 CK 失效，账号卡片展示服务器最终检查结果。 |

建议改为：“平台接口拒绝请求时，插件会请求服务器复查凭据。账号状态以服务器检查结果为准。”

调用点：web/src/views/builtin/ThirdPartyAccountsView.vue:688。保留服务器负责判定凭据状态的事实。

### L01 · P2 · 桥接诊断摘要描述内部观测实现

摘要把事件处理结果与观测数据保留方式写在一起，后半句不能帮助判断本次处理结果。

| 位置 | 原文摘录 |
| --- | --- |
| [server/internal/eventpipeline/bridge/bridge.go:21](../server/internal/eventpipeline/bridge/bridge.go) | 插件桥接已处理最近的适配器事件，桥接与运行时观测仅保留汇总数据 |

建议改为：“插件事件处理统计已更新”。数据保留方式在观测模型文档中说明。

该值由 observability_publish.go:56 写入 events.received 的 bridge_runtime 分支。web/src/lib/management-summary.ts:4 会过滤该分支；它是管理 WebSocket 摘要，未据此认定它出现在普通 INFO 日志或首页。

### L02 · P1 · 兼容矩阵使用内部流程术语和抽象职责词

“进入正式……集合”“保持传输状态信号职责”需要用户理解内部实现，才能知道是否支持事件和消息。

| 位置 | 原文摘录 |
| --- | --- |
| [server/internal/wsevents/protocol_compatibility.go:13](../server/internal/wsevents/protocol_compatibility.go) | 私聊消息事件进入插件事件主流程。 |
| [server/internal/wsevents/protocol_compatibility.go:19](../server/internal/wsevents/protocol_compatibility.go) | 心跳事件进入插件事件主流程，同时保持传输状态信号职责。 |
| [server/internal/wsevents/protocol_compatibility.go:20](../server/internal/wsevents/protocol_compatibility.go) | 生命周期事件进入插件事件主流程，同时保持传输状态信号职责。 |
| [server/internal/wsevents/protocol_compatibility.go:27](../server/internal/wsevents/protocol_compatibility.go) | 文本消息段进入正式入站与出站消息段集合。 |
| [examples/http/protocol-compatibility.response.json:16](../examples/http/protocol-compatibility.response.json) | 群消息事件进入正式插件事件主链。 |
| [fixtures/web-api/ok.protocol-onebot11-compatibility.yaml:21](../fixtures/web-api/ok.protocol-onebot11-compatibility.yaml) | 私聊消息事件进入正式插件事件主链。 |

同类事件改为“支持向插件投递私聊消息事件”等具体句子；心跳和生命周期说明使用“用于更新连接状态，并投递给插件”；消息段使用“支持接收和发送文本消息”等。同步该文件第 13–20、27–30 行的同类说明，以及示例第 16、26 行和 fixture 第 21、28、35、42、49、63、70 行。

只调整可读 summary，沿用各事件名、消息段名和支持矩阵。前端将这些 summary 用于兼容矩阵说明。

### L03 · P3 · 错误映射注释保留上一轮修改叙述

“不再用……”记录了编辑过程，维护者需要了解当前错误映射及触发条件。

| 位置 | 原文摘录 |
| --- | --- |
| [server/internal/plugins/actions/thirdparty.go:209](../server/internal/plugins/actions/thirdparty.go) | 502，不再用「缺少必要资源」的 resource_missing 语义。 |

与上一行合并为：“登录 profile 槽忙映射为 409，可重试；其余上游失败映射为 502。”

### D01 · P2 · 标题使用落点、落地和已接线等抽象表达

标题应说明章节内容，避免以实现过程或含义宽泛的词代替模块职责。

| 位置 | 原文摘录 |
| --- | --- |
| [server/README.md:5](../server/README.md) | 当前已接线能力 |
| [docs/dev/text-resources.md:5](../docs/dev/text-resources.md) | 当前落点 |
| [docs/engineering/baseline.md:16](../docs/engineering/baseline.md) | 当前工程落点 |
| [docs/engineering/web-admin-baseline.md:23](../docs/engineering/web-admin-baseline.md) | 当前工程落点 |
| [docs/architecture/state-model.md:3](../docs/architecture/state-model.md) | 当前已落地的核心状态机 |

依次建议：“服务端能力”“文本资源归属”“工程目录与职责”“组件与页面结构”；状态模型导语改为“本文档说明 RayleaBot 的核心状态机”，后接原有覆盖范围。

### D02 · P2 · 实施顺序使用接线代替具体开发动作

“接线”未说明连接对象或实现工作。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/engineering/implementation-order.md:76](../docs/engineering/implementation-order.md) | 管理 HTTP/WebSocket 在领域语义稳定后接线。 |
| [docs/engineering/implementation-order.md:91](../docs/engineering/implementation-order.md) | 客户端接线不能引入新的状态名、错误码、字段别名或信任根。 |

第 76 行建议：“领域语义确定后，实现管理 HTTP/WebSocket 接口。”第 91 行建议：“客户端接入时使用契约定义的状态名、错误码、字段和信任根。”

### D03 · P2 · 插件工具说明使用一等这一评价词

“一等”没有可核实的级别定义，读者需要的是 SDK 和构建命令的用途。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/plugin/lifecycle.md:18](../docs/plugin/lifecycle.md) | Go SDK 与构建器是一等开发工具，但不是运行时约束。 |
| [docs/release/plugin-contract-v3-upgrade.md:25](../docs/release/plugin-contract-v3-upgrade.md) | Go 插件可以使用 `raylea-plugin build-go` 复用一等构建流程。 |

生命周期段落可写为：“插件在开发者环境中编译并打包；Go 插件可使用仓库提供的 SDK 和构建器。服务端运行已经构建的原生可执行文件。”升级说明改为：“Go 插件可使用 raylea-plugin build-go 构建并打包。”

保留上一行关于 manifest/artifact 不声明实现语言的正式事实。

### D04 · P2 · 入口说明引入读者未提出的对立用途

CLI 与 fixture 的正面用途已经明确，后半句再次通过对立项定义自身。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/user/cli.md:5](../docs/user/cli.md) | CLI 是本地离线恢复与运维入口，不是第二套常规在线管理面。 |
| [fixtures/README.md:3](../fixtures/README.md) | 本目录存放由 `contracts/` 派生的 golden cases，用于自动校验、契约回归和实现对照，不是演示文档目录。 |

CLI 使用“CLI 提供本地离线恢复与运维命令。”fixture 使用“本目录存放由 contracts/ 派生的 golden cases，用于自动校验、契约回归和实现对照。”

### D05 · P2 · 组件职责重复排除相邻职责

相邻条目已明确 Local Action Service 的职责，重复对比使句子更长。独立 ingress 包的不存在也属于实现说明中的附加排除。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/architecture/message-flow.md:108](../docs/architecture/message-flow.md) | Runtime Manager 是插件进程协议职责方，不是平台能力职责方。 |
| [docs/architecture/event-pipeline.md:31](../docs/architecture/event-pipeline.md) | 不存在独立的 ingress 包。 |

第一处改为“Runtime Manager 管理插件进程协议。”第二处删除末尾分号后的句子，保留 eventpipeline/chatpolicy 承担的全部职责。

### D06 · P2 · 错误与诊断说明通过反向对比表达动作

可以直接写出错误映射方式和排障入口。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/architecture/render-service.md:69](../docs/architecture/render-service.md) | 失败时返回结构化错误，而不是浏览器原始报错 |
| [docs/dev/diagnostics.md:72](../docs/dev/diagnostics.md) | 排障优先使用正式诊断入口，而不是依赖临时日志拼接。 |

分别建议：“渲染失败时，将浏览器错误映射为契约定义的结构化错误。”“排障优先使用本页列出的诊断入口。”

### D07 · P2 · 安装校验说明先构造不可靠做法再对比

包内容独立校验是必要事实，可以直接描述校验依据。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/plugin/store-and-development.md:70](../docs/plugin/store-and-development.md) | 安装器从 ZIP 或展开目录扫描真实内容，而不是信任包内清单。 |

建议改为：“安装器独立扫描 ZIP 或展开目录，以实际文件内容执行校验。”后接原有路径、资源和摘要检查清单。

### D08 · P2 · 长期 Web 基线保留改动范围声明

“保持不变”“当前前端基线不修改……”没有明确比较基线，像一次改版的交付说明。长期基线应指向当前约束的来源。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/engineering/web-admin-baseline.md:9](../docs/engineering/web-admin-baseline.md) | 对外 HTTP API、WebSocket 事件、错误码、配置 schema 和外部类型保持不变。 |
| [docs/engineering/web-admin-baseline.md:14](../docs/engineering/web-admin-baseline.md) | 当前前端基线不修改以下正式契约： |

将相关段落整理为“接口与状态来源”：说明 HTTP、WebSocket、错误码、配置及插件接口使用随后列出的契约；保留新增接口须先更新契约的要求。删除第 9 行的无比较基线声明。

### D09 · P2 · 设计规范以原有、不影响和不修改指代有效规则

长期文档中的“原有边界”“原有含义”没有可定位的定义；字体和打包的实际归属应直接写明。

| 位置 | 原文摘录 |
| --- | --- |
| [DESIGN.md:189](../DESIGN.md) | 该映射不修改共享基础 token，也不影响 Launcher 或独立 iframe 的字体。 |
| [docs/design/web-management-ui.md:12](../docs/design/web-management-ui.md) | 共享基础字体 token、Launcher 系统正文与独立 iframe 字体各自保持原有边界。 |
| [docs/design/web-management-ui.md:107](../docs/design/web-management-ui.md) | 共享品牌 token 保持原有含义。 |
| [docs/design/launcher-design-system.md:112](../docs/design/launcher-design-system.md) | 桌面桥、应用 identity 与正式 release metadata 语义保持原有边界 |
| [docs/design/plugin-management-surface.md:18](../docs/design/plugin-management-surface.md) | 宿主视觉规范不修改插件 manifest、API、状态、配置或 bridge |

字体归属建议明确为：“Web 正文使用 Noto Sans SC；Launcher 使用共享系统字体栈；iframe 字体由插件页面提供。”认证段落以已有颜色配比及信息、验证、提交能力说明结束。打包段落保留 Wails 版本、资源生成与 asInvoker 要求，并给桥接和发布元数据提供正式来源链接。插件视觉段落保留 iframe 配色、字体、布局及组件的维护职责。

DESIGN.md、Web 设计文档和 sidecar 已有工作区改动。本条以本次快照为依据；实施时合并当前正文，再通过现有生成器同步 narrative。

### D10 · P2 · 开发文档补充发布矩阵保持不变

增量构建章节用一次改动前后的比较描述发布规则。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/dev/README.md:70](../docs/dev/README.md) | 发布构建的默认平台矩阵保持不变。 |

删除该分句；如该段需要发布平台信息，链接到交付与升级文档中的平台矩阵。保留开发依赖按 OS、CPU、libc 安装及缓存判定条件。

### D11 · P2 · 日志规范夹带精简工作的范围说明

“精简日志不改写……”描述一次文案修订的影响，维护者需要的是日志与业务结果的职责关系。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/dev/logging.md:7](../docs/dev/logging.md) | 精简日志不改写插件业务结果、调度统计或历史日志。 |

建议改为：“插件业务结果与调度统计由对应执行流程记录；历史日志保存当时的事件记录。”

本页的排重条件、恢复日志、发送结果不确定性和凭据脱敏范围属于运维约定，应完整保留。

### D12 · P2 · 品牌和字号规则使用难以执行的评价

品牌承诺中的“工具退到任务之后”和字号规则中的“稍大”缺少直接的界面约束。

| 位置 | 原文摘录 |
| --- | --- |
| [PRODUCT.md:37](../PRODUCT.md) | 界面保持优雅但不追求装饰性惊喜，熟悉的交互和稳定的状态反馈应让工具退到任务之后。 |
| [docs/design/launcher-design-system.md:89](../docs/design/launcher-design-system.md) | 真实主状态可以使用稍大字号，不形成巨型指标区。 |

品牌句建议：“界面使用紧凑排版、稳定的控件位置和清楚的状态反馈。”字号句应写明主状态元素、实际字号和适用条件；从当前样式与组件中核对后填写具体值。

“安静、精密、可信”和“雾白·青瓷”承担品牌命名职责。字号条目需在实施时核对具体元素，本报告不推定新的视觉参数。

### D13 · P2 · 设计目录使用分面和包络等未解释标签

这里描述的是 Web、Launcher 和插件页面的规范。使用“分面映射”“兼容包络”增加理解步骤。

| 位置 | 原文摘录 |
| --- | --- |
| [README.md:84](../README.md) | 项目级视觉规范、分面映射与采用矩阵 |
| [docs/design/README.md:14](../docs/design/README.md) | 分面文档 |
| [docs/design/README.md:20](../docs/design/README.md) | 第三方页面兼容包络 |
| [docs/design/plugin-management-surface.md:8](../docs/design/plugin-management-surface.md) | reduced-motion 包络 |
| [docs/design/plugin-management-surface.md:62](../docs/design/plugin-management-surface.md) | Web 分面规范 |

展示文案建议使用“各界面规范”“第三方页面兼容要求”“减少动态效果的要求”和“Web 界面规范”。同步 DESIGN.md 及三份界面规范中的相同中文引用。

compatible-envelope 是此工具使用的状态值；将展示解释写清楚即可。

### D14 · P2 · 协议说明用真正加强未知的判定

“未知”已承担协议分类含义，“真正”没有增加判定条件。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/plugin/protocol.md:71](../docs/plugin/protocol.md) | 真正未知或非法协议帧仍按协议违规处理。 |
| [docs/release/log-runtime-repair-2026-09-04.md:14](../docs/release/log-runtime-repair-2026-09-04.md) | 真正未知帧仍拒绝 |

分别使用“未知或非法协议帧按协议违规处理。”和“拒绝未知帧”。保留迟到响应与未知帧的具体区别。

### D15 · P2 · 状态说明使用伪装这一评价性比喻

“伪装”给实现赋予动机，没有说明状态转换或能力声明的要求。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/architecture/bot-core.md:88](../docs/architecture/bot-core.md) | 但不会伪装成完全就绪。 |
| [docs/architecture/event-model.md:47](../docs/architecture/event-model.md) | 未进入正式范围的事件不会伪装成已支持能力。 |

前者以“外部协议链路暂时不可用时，可进入 degraded”结束。后者建议：“支持能力清单与已定义的事件范围一致。”

### A01 · P3 · 指令入口解释自身编排方式

首段可直接要求读取相关规则，省去“本文件只保留/只补充”的编辑说明。

| 位置 | 原文摘录 |
| --- | --- |
| [AGENTS.md:4](../AGENTS.md) | 本文件只保留仓库级约束；进入具体目录后读取就近的 `AGENTS.md`。 |
| [docs/CHANGELOGS/AGENTS.md:3](../docs/CHANGELOGS/AGENTS.md) | 本文件只补充 `docs/CHANGELOGS/` 目录特有、长期有效的规则。 |
| [.impeccable.md:3](../.impeccable.md) | 本文件只为读取 `.impeccable.md` 的设计工具提供兼容入口，不复制产品或视觉规范。 |

根文件保留“进入具体目录后读取就近的 AGENTS.md。”更新历史指令保留“先遵守根 AGENTS.md。”兼容入口建议：“设计工具从以下链接读取产品与视觉规范。”

9 份 AGENTS 和 9 份 CLAUDE 均纳入扫描。其余硬规则主要承担职责、凭据、并发和契约约束；CLAUDE 文件均为导入行。

### A02 · P2 · 验证技能通过对立句重申覆盖要求

前后半句表达同一项覆盖要求。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/repo-validation/SKILL.md:29](../.agents/skills/repo-validation/SKILL.md) | 必要的跨包、集成或既有 CI 门禁仍须完成；最小验证不是跳过与改动有关的高风险路径。 |

建议改为：“验证必须覆盖与改动有关的高风险路径，并完成必要的跨包、集成检查和既有 CI 门禁。”

### S01 · P2 · 英文技能及脚本使用 genuinely

该词属于本次明确要求避免的表达，30 处位置见附录。它在指令、可读输出及注释中均有出现。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/impeccable/reference/clarify.md:69](../.agents/skills/impeccable/reference/clarify.md) | the audience genuinely knows |
| [.agents/skills/impeccable/reference/critique.md:133](../.agents/skills/impeccable/reference/critique.md) | A 4 means genuinely excellent. |
| [.agents/skills/impeccable/reference/polish.md:83](../.agents/skills/impeccable/reference/polish.md) | Promote genuinely reusable values to tokens |

删除无信息增量的 genuinely。必要时明确条件，例如“Promote values shared by multiple components to tokens.”附录逐条列出全部命中和建议。

附录中的普通源码注释按 P3 处理；生成文件随其来源重新生成。

### S02 · P2 · 英文指令与输出使用 leverage

可读标题和错误信息用抽象动词或名词代替具体浏览器能力，共 6 处。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/impeccable/reference/new-work.md:97](../.agents/skills/impeccable/reference/new-work.md) | Build the form's web leverage. |
| [.agents/skills/impeccable/scripts/concept-seed.mjs:242](../.agents/skills/impeccable/scripts/concept-seed.mjs) | WEB LEVERAGE: |
| [.agents/skills/impeccable/scripts/lib/concept-catalog.mjs:145](../.agents/skills/impeccable/scripts/lib/concept-catalog.mjs) | needs web leverage of 20–240 characters |

标题使用“Implement the browser capabilities required by the selected design.”；可读输出使用“BROWSER CAPABILITIES”；校验提示说明需要“a browser capability description of 20–240 characters”。完整位置见附录。

既有 webLeverage 字段属于机器结构。另 1 处 leverage the power 是检测规则样本，见保留项。

### S03 · P1 · 简化技能直接使用被点名的对立句式

Simplicity is not about... It's about... 与本次禁止句式同构；ruthless 与泛化励志段也没有给出操作条件。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/impeccable/reference/distill.md:26](../.agents/skills/impeccable/reference/distill.md) | Simplicity is not about removing features. It's about removing obstacles between users and their goals. |
| [.agents/skills/impeccable/reference/distill.md:30](../.agents/skills/impeccable/reference/distill.md) | Create a ruthless editing strategy: |
| [.agents/skills/impeccable/reference/distill.md:37](../.agents/skills/impeccable/reference/distill.md) | Simplification is hard. It requires saying no to good ideas to make room for great execution. Be ruthless. |

第 26 行使用“Remove obstacles that prevent users from completing their task. Keep elements that support that task.”第 30 行使用“Plan the edits:”。第 37 行删除；其前后的具体步骤已说明如何简化。

### S04 · P1 · 动效技能包含模式表演、自问自答和口号收尾

要求固定仪式性开场；正文多次用抽象评价和对立句表达目标；最后一段又重复主题，符合本次要求避免的写法。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/impeccable/reference/overdrive.md:5](../.agents/skills/impeccable/reference/overdrive.md) | Entering overdrive mode... |
| [.agents/skills/impeccable/reference/overdrive.md:8](../.agents/skills/impeccable/reference/overdrive.md) | This isn't just about visual effects. It's about using the full power of the browser |
| [.agents/skills/impeccable/reference/overdrive.md:10](../.agents/skills/impeccable/reference/overdrive.md) | But a settings page with instant optimistic saves and animated state transitions? That's extraordinary too. |
| [.agents/skills/impeccable/reference/overdrive.md:111](../.agents/skills/impeccable/reference/overdrive.md) | ship the version that feels inevitable. |
| [.agents/skills/impeccable/reference/overdrive.md:127](../.agents/skills/impeccable/reference/overdrive.md) | "Technically extraordinary" isn't about using the newest API. |

删除第 1–6 行固定开场要求。开篇直接要求依据用户任务选择浏览器技术并说明可观察效果。第 10 行改为“Use optimistic saves and state transitions when they help users confirm settings changes.”第 111 行改为检查缓动、错峰时序和次级运动。删除第 127 行重复收尾。第 82 行的范围提醒改为“Improve the presentation and interaction of the product's existing features.”

保留功能扩展须依据产品需求、性能验证及兼容回退的具体要求。

### S05 · P2 · 审查与维护技能通过否定另一类工作定义任务

任务开头可以直接指定审查对象、产物和操作范围。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/impeccable/reference/audit.md:3](../.agents/skills/impeccable/reference/audit.md) | This is a code-level audit, not a design critique. |
| [.agents/skills/impeccable/reference/audit.native.md:3](../.agents/skills/impeccable/reference/audit.native.md) | This is a code-level audit, not a design critique. |
| [.agents/skills/impeccable/reference/doctor.md:3](../.agents/skills/impeccable/reference/doctor.md) | This is maintenance, not design. |
| [.agents/skills/impeccable/reference/doctor.md:5](../.agents/skills/impeccable/reference/doctor.md) | What this owns, and what it does not |

两份 audit 使用“Audit measurable behavior in the implementation.”doctor 使用“Repair the artifacts named in the report.”，章节名使用“Maintenance scope”。保留原生平台工具选择和具体文件范围约束。

### S06 · P2 · 设计检查指南通过贬低替代做法评价成果

“built rather than assembled”“the one models skip most reliably”是评价性比较，未提供检查方法。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/impeccable/reference/craft-floor.md:15](../.agents/skills/impeccable/reference/craft-floor.md) | This is the cheapest signal that a page was built rather than assembled, and the one models skip most reliably. |

删除这句评价；检查项直接列出文本选择、光标、滚动条、焦点、下划线和表格数字，并说明应按项目样式规则核对。

### S07 · P2 · 强化视觉技能使用抽象比喻替代操作

sovereign、conviction、rhythm 等表述用于解释任务时缺少可检查的对象。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/impeccable/reference/bolder.md:7](../.agents/skills/impeccable/reference/bolder.md) | Scope is sovereign |
| [.agents/skills/impeccable/reference/bolder.md:5](../.agents/skills/impeccable/reference/bolder.md) | The reflex answer, reaching for more effects, is the opposite of bold; reject it first. |
| [.agents/skills/impeccable/reference/bolder.md:19](../.agents/skills/impeccable/reference/bolder.md) | If every element got louder, the section got flatter. |

章节名使用“Target scope”。第 5 行末句改为“Identify the target's hierarchy problem before choosing an effect.”第 19 行末句改为“Emphasize the target with type size, spacing, or contrast.”

该文件引用的用户约束“Everything else stays”属于范围条件，按上下文保留。

### S08 · P2 · 英文技能使用连字符组合描述

这些词描述操作或对象，拆成普通介词短语即可说明含义。完整的技术名和代码标识另按其来源处理。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/impeccable/reference/polish.md:61](../.agents/skills/impeccable/reference/polish.md) | same-role typography |
| [.agents/skills/impeccable/reference/polish.md:74](../.agents/skills/impeccable/reference/polish.md) | platform-appropriate touch targets |
| [.agents/skills/impeccable/reference/polish.md:76](../.agents/skills/impeccable/reference/polish.md) | permission-limited content |
| [.agents/skills/impeccable/reference/polish.md:81](../.agents/skills/impeccable/reference/polish.md) | polish-created duplication |
| [.agents/skills/impeccable/reference/layout.md:53](../.agents/skills/impeccable/reference/layout.md) | container-aware components |

依次建议：“typography for the same role”“touch targets sized for the platform”“content restricted by permissions”“duplication introduced during the edits”“components that adapt to their container”。两份 audit 中 code-level 的处理见 S05。

检索还覆盖了连字符、破折号及 not/but、rather than、instead of 结构；本条列出已确认可直接展开的描述。文件名、CSS 属性、状态值、第三方名称按正式拼写引用。

### S09 · P2 · 设计检测器的说明本身含评价性套话

报告消息中的“most recognizable tell”“reads as ... not ...”以风格评价替代具体可观察现象及动作。

| 位置 | 原文摘录 |
| --- | --- |
| [.agents/skills/impeccable/scripts/detector/registry/antipatterns.mjs:8](../.agents/skills/impeccable/scripts/detector/registry/antipatterns.mjs) | the most recognizable tell of AI-generated UIs. |
| [.agents/skills/impeccable/scripts/detector/registry/antipatterns.mjs:55](../.agents/skills/impeccable/scripts/detector/registry/antipatterns.mjs) | the most recognizable tells of AI-generated UIs. |
| [.agents/skills/impeccable/scripts/detector/registry/antipatterns.mjs:121](../.agents/skills/impeccable/scripts/detector/registry/antipatterns.mjs) | reads as placeholder clip art, not illustration. |

第 8 行说明卡片单侧存在粗彩色边框，并指向项目边界样式规则；第 55 行直接说明检测到的紫色渐变或青色暗底；第 121 行直接说明大幅 SVG 由大量基础形状组成，并要求按图像用途选择资产。用完整英文句子填写实际现象和处理方式。

detect-antipatterns-browser.js 含生成副本。文件头记录了上游构建脚本；本仓库未找到该脚本，实施时先确认当前技能包的生成或更新入口。

### H01 · P3 · 更新历史保存收口、一等等含混描述

历史条目可作为今后发布文案的反例；其中“保持不变”还需判断是否承载该版本的迁移信息。

| 位置 | 原文摘录 |
| --- | --- |
| [docs/CHANGELOGS/README.md:10](../docs/CHANGELOGS/README.md) | Launcher 收口 |
| [docs/CHANGELOGS/v0.2.md:11](../docs/CHANGELOGS/v0.2.md) | 管理面跨页钻取与诊断引导收口 |
| [docs/CHANGELOGS/v0.2.md:12](../docs/CHANGELOGS/v0.2.md) | Launcher 状态模型拆分、环境检查收口 |
| [docs/CHANGELOGS/v0.2.md:49](../docs/CHANGELOGS/v0.2.md) | Launcher 本地预检收口为安装目录 |
| [docs/CHANGELOGS/v0.4.md:10](../docs/CHANGELOGS/v0.4.md) | 评审修复收口 |
| [docs/CHANGELOGS/v0.4.md:37](../docs/CHANGELOGS/v0.4.md) | 一等开发工具 |
| [docs/CHANGELOGS/v0.4.md:65](../docs/CHANGELOGS/v0.4.md) | 隔离 origin、CSP、nonce、MessagePort 与密钥保护保持不变。 |

今后发布说明使用具体交付行为，例如“Launcher 本地预检检查安装目录、设置、服务端程序、配置和工作目录可写性”；评审修复直接列出配置快照、控制事件顺序等已修复问题。历史索引若后续修订，可用对应版本的具体能力替换“收口”。

docs/CHANGELOGS/AGENTS.md:7 要求“已归档的更新历史不得修改，除非用户明确要求。”本轮交付是审查文档；历史记录的后续处理需遵循这项要求。

## 需要保留的事实与表达

以下命中已经按上下文复核。实施整改时保留其约定，再决定是否需要简化句子。

| 位置 | 原文或内容 | 保留理由 |
| --- | --- |
| [.agents/skills/impeccable/scripts/detector/engines/regex/detect-text.mjs:547](../.agents/skills/impeccable/scripts/detector/engines/regex/detect-text.mjs) | leverage the power；第 553 行还有 seamless experience 等词条 | 这些字符串是套话检测器的规则输入。改写会改变检测行为。 |
| [contracts/error-codes.yaml:262](../contracts/error-codes.yaml)、[web/src/locales/zh-CN/config.ts:189](../web/src/locales/zh-CN/config.ts) | 优雅退出、优雅关闭 | 指允许进程在强制终止前完成收尾的技术过程。它与赞美视觉效果的“优雅”用途不同。 |
| [docs/plugin/permissions-and-manifest.md:84](../docs/plugin/permissions-and-manifest.md)、[PRODUCT.md:31](../PRODUCT.md) | permissions 不是操作系统沙箱；签名不构成代码安全证明 | 说明管理员执行本机插件代码的信任边界。简化后仍须明确权限控制的实际能力。 |
| [web/src/locales/zh-CN/auth.ts:26](../web/src/locales/zh-CN/auth.ts)、[launcher/src/renderer/src/ActionConfirmDialog.tsx:32](../launcher/src/renderer/src/ActionConfirmDialog.tsx) | 重置凭据后保留配置、数据和插件，原有会话失效 | 信息直接影响重置操作的决定，适合保留在恢复及重置说明中。 |
| [web/src/locales/zh-CN/plugins.ts:24](../web/src/locales/zh-CN/plugins.ts) | 删除插件源时，已安装插件不会被删除 | 说明删除对象及其影响，避免误解为卸载插件。 |
| [server/internal/onebot11/shell_outbound.go:135](../server/internal/onebot11/shell_outbound.go)、[web/src/locales/zh-CN/errors.ts:3](../web/src/locales/zh-CN/errors.ts) | 消息可能仍会送达；未自动重发 | 说明发送结果不确定性及重试行为，帮助避免重复发送。 |
| [docs/plugin/protocol.md:71](../docs/plugin/protocol.md) | 取消本地等待不代表宿主动作已取消 | 这是动作生命周期约定；同段的“真正”可删除，取消语义须保留。 |
| [docs/user/configuration.md:26](../docs/user/configuration.md)、[docs/plugin/sdk/README.md:59](../docs/plugin/sdk/README.md) | 校验命令不写文件；绝对路径按原义解析 | 分别说明命令副作用和路径解析规则。 |
| [docs/release/log-runtime-repair-2026-09-04.md:56](../docs/release/log-runtime-repair-2026-09-04.md) | 交叉构建通过，尚未完成对应平台运行验收 | 表达验证范围，能防止读者将构建结果当作运行证明。 |
| [docs/release/log-runtime-repair-2026-09-04.md:104](../docs/release/log-runtime-repair-2026-09-04.md) | 验收后二进制、开发模式及安装内容的记录 | 属于验证快照；具体环境与产物范围有追溯用途，后续发布正文按读者需要引用。 |
| [AGENTS.md:18](../AGENTS.md)、[server/AGENTS.md:9](../server/AGENTS.md)、[web/AGENTS.md:21](../web/AGENTS.md) | 凭据、鉴权与验证边界中的禁止项 | 这些句子直接约束危险或错误做法，具备明确执行对象。 |
| [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md)、[web/public/fonts/OFL-Noto-Sans-SC.txt](../web/public/fonts/OFL-Noto-Sans-SC.txt) | 第三方通知和字体许可证 | 原作者声明和许可证条款按正式文本保留。 |

机器字段、API 名、错误码、协议版本、文件名、CSS 属性及工具状态值按正式来源维护。已有约束中的 contract-first、prefers-reduced-motion、compatible-envelope 等应保留其技术含义。反例引文和历史需求记录也需要保留其出处。

## 实施与验收

1. 按 P1 清单修改当前界面资源及技能正文，逐条对照本报告的原文、位置和建议。当前页面通过调用点验证修改后的内容可见。

2. 修订文档时明确来源、主语、条件、动作与结果。字号等尚未核定的参数先从实现核实，再写入规范。有正式含义的字段和状态按契约校对。

3. 含生成副本的项目从可维护来源处理。DESIGN.md 的 narrative 按 scripts/generate-design-tokens.mjs 同步；Impeccable 浏览器检测器先核实该版本技能的生成或更新流程。

4. 验证普通文案时检查实际显示、关键事实、链接及原有行为断言。仅因一句话变短而新增全文字面值测试，会使后续正常措辞调整变得困难。既有测试若断言正式状态、错误码或必要信息，应继续覆盖这些语义。

5. 指令变动运行 node scripts/check-agent-docs.mjs；文档变动运行 python scripts/check-doc-links.py。界面或日志实现的检查根据实际修改位置选择已有组件测试、相关 Go 测试或页面验证。

6. 复查本次明确点名的英文词，以及同义的中文转折、空泛标签和收尾。人工核对候选段落，确保修改后仍包含必要条件和失败后果。

### 本轮交付验证

| 检查 | 结果 |
| --- | --- |
| 原文与行号复核 | 35 组建议的 93 条原文摘录全部与当前源文件对应 |
| 英文命中清单 | 30 处 genuinely、6 处待处理 leverage；1 处检测规则样本已标注 |
| 文档链接 | python scripts/check-doc-links.py 通过，共检查 134 份 Markdown |
| 指令结构 | node scripts/check-agent-docs.mjs 通过 |
| 差异与文本格式 | git diff --check 通过；新增报告另行检查 UTF-8、替换字符与行尾空白，通过 |
| 快照一致性 | 重新核对原有 1,783 个文本文件的 SHA-256，均与扫描时一致 |

本轮交付为本整改清单；运行行为、浏览器展示和整改后的回归检查在相应条目实施时验证。

## 附录 A：英文指定词的完整命中清单

本次字面扫描命中 genuinely 30 处、leverage 7 处。leverage 中 1 处属于检测规则输入，已列在保留项；下表列出其余 36 处。其他明确点名的英文词串未在扫描输入中命中。句式变体在 S03、S04 等条目中列出。

下表与 S01、S02 存在包含关系。生成副本的命中单独计入位置数量，整改时随来源同步。

| 位置 | 原文片段 | 建议 | 用途 |
| --- | --- |
| [.agents/skills/impeccable/reference/bolder.md:9](../.agents/skills/impeccable/reference/bolder.md) | ` … does not already own. If the existing system genuinely cannot express the direction, do not expand it on your own. … ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/clarify.md:69](../.agents/skills/impeccable/reference/clarify.md) | ` without flattening terminology the audience genuinely knows. ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/colorize.md:44](../.agents/skills/impeccable/reference/colorize.md) | ` - Tint neutrals only when the brand hue genuinely creates cohesion. Neutral gray is valid when it serves the … ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/critique.md:133](../.agents/skills/impeccable/reference/critique.md) | ` Be honest with scores. A 4 means genuinely excellent. Most real interfaces score 20-32 out of 40. ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/critique.md:135](../.agents/skills/impeccable/reference/critique.md) | `` … of work), as may any other heuristic that genuinely cannot apply to the surface under review. Write `n/a` in the … `` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/critique.md:414](../.agents/skills/impeccable/reference/critique.md) | ` … on a 0–4 scale. Be honest: a 4 means genuinely excellent, not "good enough." ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/harden.md:81](../.agents/skills/impeccable/reference/harden.md) | ` … the typography guidance sets; 14px only for genuinely secondary text. iOS Safari force-zooms focused inputs under … ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/init.md:23](../.agents/skills/impeccable/reference/init.md) | `` … `android`, or `adaptive` (one product that genuinely adapts its design language per OS). Mobile web remains `web`; … `` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/layout.md:21](../.agents/skills/impeccable/reference/layout.md) | ` … Are repeated cards, columns, or sections genuinely equivalent, or merely a framework default? ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/layout.md:84](../.agents/skills/impeccable/reference/layout.md) | ` … structural parameter only when the topology genuinely branches. Follow [live.md](live.md)'s parameter contract. ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/live.md:104](../.agents/skills/impeccable/reference/live.md) | ` … delete by context. If a stroke's intent is genuinely ambiguous and it changes the brief, ask one short question … ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/live.md:153](../.agents/skills/impeccable/reference/live.md) | ` … them; from those, derive three directions genuinely different from each other AND from the current surface; … ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/new-work.md:35](../.agents/skills/impeccable/reference/new-work.md) | ` … user behavior, ordered by resonance. For a genuinely open whole page, screen, or flow, run: ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/reference/new-work.md:97](../.agents/skills/impeccable/reference/new-work.md) | ` - **Build the form's web leverage.** When the chosen world names a technique (canvas, WebGL, vi ` | 使用 browser capabilities 或 browser capability description | 指令正文 |
| [.agents/skills/impeccable/reference/polish.md:83](../.agents/skills/impeccable/reference/polish.md) | ` - Promote genuinely reusable values to tokens; do not create a system abstraction ` | 删除 genuinely，按需要补充判定条件 | 指令正文 |
| [.agents/skills/impeccable/scripts/concept-seed.mjs:242](../.agents/skills/impeccable/scripts/concept-seed.mjs) | ` WEB LEVERAGE: ${concept.webLeverage} ` | 使用 browser capabilities 或 browser capability description | 可读输出 |
| [.agents/skills/impeccable/scripts/concept-seed.mjs:253](../.agents/skills/impeccable/scripts/concept-seed.mjs) | `` WEB LEVERAGE: ${composition.webLeverage}`; `` | 使用 browser capabilities 或 browser capability description | 可读输出 |
| [.agents/skills/impeccable/scripts/concept-seed.mjs:595](../.agents/skills/impeccable/scripts/concept-seed.mjs) | ` genuinely new grounded candidates from unexplored angles before judging ` | 删除 genuinely，按需要补充判定条件 | 可读输出 |
| [.agents/skills/impeccable/scripts/detector/detect-antipatterns-browser.js:213](../.agents/skills/impeccable/scripts/detector/detect-antipatterns-browser.js) | ` … pulse animation for indicators tied to genuinely live, changing data; a static indicator with clear labeling … ` | 删除 genuinely，按需要补充判定条件 | 生成副本 |
| [.agents/skills/impeccable/scripts/detector/detect-antipatterns-browser.js:3106](../.agents/skills/impeccable/scripts/detector/detect-antipatterns-browser.js) | ` // Every layer up to the document root was genuinely see-through, so the ` | 删除 genuinely，按需要补充判定条件 | 生成副本 |
| [.agents/skills/impeccable/scripts/detector/detect-antipatterns-browser.js:4749](../.agents/skills/impeccable/scripts/detector/detect-antipatterns-browser.js) | ` // genuinely need element rects (line-length, cramped-padding). ` | 删除 genuinely，按需要补充判定条件 | 生成副本 |
| [.agents/skills/impeccable/scripts/detector/detect-antipatterns-browser.js:5140](../.agents/skills/impeccable/scripts/detector/detect-antipatterns-browser.js) | ` // and the nearest content genuinely above / below it. Fires only when two ` | 删除 genuinely，按需要补充判定条件 | 生成副本 |
| [.agents/skills/impeccable/scripts/detector/registry/antipatterns.mjs:102](../.agents/skills/impeccable/scripts/detector/registry/antipatterns.mjs) | ` … pulse animation for indicators tied to genuinely live, changing data; a static indicator with clear labeling … ` | 删除 genuinely，按需要补充判定条件 | 可读输出 |
| [.agents/skills/impeccable/scripts/detector/rules/checks.mjs:1872](../.agents/skills/impeccable/scripts/detector/rules/checks.mjs) | ` // Every layer up to the document root was genuinely see-through, so the ` | 删除 genuinely，按需要补充判定条件 | 注释 |
| [.agents/skills/impeccable/scripts/detector/rules/checks.mjs:3515](../.agents/skills/impeccable/scripts/detector/rules/checks.mjs) | ` // genuinely need element rects (line-length, cramped-padding). ` | 删除 genuinely，按需要补充判定条件 | 注释 |
| [.agents/skills/impeccable/scripts/detector/rules/checks.mjs:3906](../.agents/skills/impeccable/scripts/detector/rules/checks.mjs) | ` // and the nearest content genuinely above / below it. Fires only when two ` | 删除 genuinely，按需要补充判定条件 | 注释 |
| [.agents/skills/impeccable/scripts/hook-lib.mjs:168](../.agents/skills/impeccable/scripts/hook-lib.mjs) | ` // next to source and run 200KB+, while genuinely authored stylesheets in ` | 删除 genuinely，按需要补充判定条件 | 注释 |
| [.agents/skills/impeccable/scripts/lib/composition-catalog.mjs:112](../.agents/skills/impeccable/scripts/lib/composition-catalog.mjs) | `` errors.push(`composition ${id} needs web leverage of 20–240 characters`); `` | 使用 browser capabilities 或 browser capability description | 可读输出 |
| [.agents/skills/impeccable/scripts/lib/concept-catalog.mjs:145](../.agents/skills/impeccable/scripts/lib/concept-catalog.mjs) | `` errors.push(`concept ${id} needs web leverage of 20–240 characters`); `` | 使用 browser capabilities 或 browser capability description | 可读输出 |
| [.agents/skills/impeccable/scripts/lib/concept-catalog.mjs:289](../.agents/skills/impeccable/scripts/lib/concept-catalog.mjs) | `` … warnings.push(`concept ${concept.id} web leverage should be checked for a specific browser-native capability`); `` | 使用 browser capabilities 或 browser capability description | 可读输出 |
| [.agents/skills/impeccable/scripts/lib/staleness-deep.mjs:111](../.agents/skills/impeccable/scripts/lib/staleness-deep.mjs) | `` + 'If it has genuinely drifted, `document` regenerates it from the code.', `` | 删除 genuinely，按需要补充判定条件 | 可读输出 |
| [.agents/skills/impeccable/scripts/live-browser.js:5952](../.agents/skills/impeccable/scripts/live-browser.js) | ` // stay readable; a genuinely new failure (different variant, URL, or ` | 删除 genuinely，按需要补充判定条件 | 注释 |
| [.agents/skills/impeccable/scripts/live-browser.js:7820](../.agents/skills/impeccable/scripts/live-browser.js) | ` // genuinely transparent (no own color, no own image) - in that case ` | 删除 genuinely，按需要补充判定条件 | 注释 |
| [.agents/skills/impeccable/scripts/live-browser.js:8147](../.agents/skills/impeccable/scripts/live-browser.js) | ` … alpha through, so a rounded corner or any genuinely ` | 删除 genuinely，按需要补充判定条件 | 注释 |
| [.agents/skills/impeccable/scripts/live-browser.js:8200](../.agents/skills/impeccable/scripts/live-browser.js) | ` // (genuinely transparent → white is correct). ` | 删除 genuinely，按需要补充判定条件 | 注释 |
| [.agents/skills/impeccable/scripts/serve-question.mjs:1014](../.agents/skills/impeccable/scripts/serve-question.mjs) | ` … the shimmer for the image. Generation is genuinely slow and a ` | 删除 genuinely，按需要补充判定条件 | 注释 |

## 附录 B：README 与指令文件覆盖

各文件均已纳入扫描。下表关联已列出的整改项；“未单列整改”表示本清单没有为该文件建立独立条目。

| README | 清单关联 |
| --- | --- |
| [README.md](../README.md) | D13 |
| [contracts/README.md](../contracts/README.md) | 未单列整改 |
| [docs/CHANGELOGS/README.md](../docs/CHANGELOGS/README.md) | H01 |
| [docs/architecture/README.md](../docs/architecture/README.md) | 未单列整改 |
| [docs/design/README.md](../docs/design/README.md) | D13 |
| [docs/dev/README.md](../docs/dev/README.md) | D10 |
| [docs/engineering/README.md](../docs/engineering/README.md) | 未单列整改 |
| [docs/plugin/README.md](../docs/plugin/README.md) | 未单列整改 |
| [docs/plugin/sdk/README.md](../docs/plugin/sdk/README.md) | 未单列整改 |
| [docs/release/README.md](../docs/release/README.md) | 未单列整改 |
| [docs/release/notes/README.md](../docs/release/notes/README.md) | 未单列整改 |
| [docs/user/README.md](../docs/user/README.md) | 未单列整改 |
| [examples/README.md](../examples/README.md) | 未单列整改 |
| [examples/plugins/README.md](../examples/plugins/README.md) | 未单列整改 |
| [examples/plugins/hello-go/README.md](../examples/plugins/hello-go/README.md) | 未单列整改 |
| [fixtures/README.md](../fixtures/README.md) | D04 |
| [server/README.md](../server/README.md) | D01 |
| [server/internal/deps/README.md](../server/internal/deps/README.md) | 未单列整改 |

| AGENTS | 同目录 CLAUDE | 清单关联 |
| --- | --- | --- |
| [AGENTS.md](../AGENTS.md) | [CLAUDE.md](../CLAUDE.md) | A01 |
| [contracts/AGENTS.md](../contracts/AGENTS.md) | [contracts/CLAUDE.md](../contracts/CLAUDE.md) | 未单列整改 |
| [docs/AGENTS.md](../docs/AGENTS.md) | [docs/CLAUDE.md](../docs/CLAUDE.md) | 未单列整改 |
| [docs/CHANGELOGS/AGENTS.md](../docs/CHANGELOGS/AGENTS.md) | [docs/CHANGELOGS/CLAUDE.md](../docs/CHANGELOGS/CLAUDE.md) | A01 |
| [examples/AGENTS.md](../examples/AGENTS.md) | [examples/CLAUDE.md](../examples/CLAUDE.md) | 未单列整改 |
| [fixtures/AGENTS.md](../fixtures/AGENTS.md) | [fixtures/CLAUDE.md](../fixtures/CLAUDE.md) | 未单列整改 |
| [launcher/AGENTS.md](../launcher/AGENTS.md) | [launcher/CLAUDE.md](../launcher/CLAUDE.md) | 未单列整改 |
| [server/AGENTS.md](../server/AGENTS.md) | [server/CLAUDE.md](../server/CLAUDE.md) | 未单列整改 |
| [web/AGENTS.md](../web/AGENTS.md) | [web/CLAUDE.md](../web/CLAUDE.md) | 未单列整改 |
