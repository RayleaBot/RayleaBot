# RayleaBot 工程选型复核（2026-10-03）

状态：评估结论，尚未进入实施。维护者决定采纳的条目按本目录规则转为执行计划，落地并写入现行文档后删除本文件。报告由多代理只读分析产出，未修改仓库文件，也未运行构建或测试；仓库事实以文中引用的文件为准。

## 总体结论

按 2026 年 10 月的生态状态、忽略迁移成本重新选型，RayleaBot 的主干在本产品约束下仍是最佳或并列最佳选择：Go 单二进制 Server、本地 SQLite + sqlc、原生子进程插件 + JSONL、契约优先的 OpenAPI/JSON Schema、Vue 管理面、Wails v3 桌面宿主。主干选型（Server 语言、存储引擎、Web 框架、桌面宿主）无需更换；需要更换的框架或语言只出现在平行栈收敛上：Launcher 渲染层由 React 改 Vue（R5），仓库脚本去掉 Python（R6，两个维度结论相反）。其余值得替换的都是同一技术栈内的实现问题，包括管理日志逐条同步落盘、SQLite PRAGMA 没有落到每条连接、会话令牌与签名密钥明文入库、FFmpeg 静态包首启同步下载、已归档的 gopkg.in/yaml.v3。证据最硬、三张校验票一致维持的是 SQLite 每连接 PRAGMA；会话令牌哈希存储的 final_verdict 也为替换，仓库证据已核实。Launcher 改 Vue 的方向各维度一致，但收益被下调为中等；脚本收敛到 Go 在两个维度得出了相反的最终结论。被推翻的主要是看起来“更现代”的工具替换，原因多为痛点被夸大、仓库已有等价机制、维护者最近刚做过又撤销（Prometheus 指标、push/PR 门禁、Launcher 的 OpenAPI 类型生成、Python/Node 插件 SDK），或替代方案的关键事实不成立（例如 ulikunitz/xz 的 DictCap 是下限，不是上限）。校验还顺带发现几处与选型无关、但应尽快修的现存缺陷，最紧迫的是 golangci-lint v2.12.2 不支持 Go 1.27，以及 macOS FFmpeg 来源构建时启用了 `--enable-nonfree`，见第 7 节。

| 项目 | 数量 |
|---|---|
| 分析维度 | 19（16 个主维度 + 3 个完整性补充维度） |
| 建议总数 | 111 |
| 原判为替换或考虑 | 62 |
| 最终“替换” | 8 |
| 最终“考虑” | 8 |
| 被校验推翻（原判替换或考虑，最终保持） | 46 |
| 原判即“保持” | 49 |
| 校验票 | 186（缓存 91，联网 95） |

## 1. 总表

| 维度 | 当前选型 | 结论 | 更佳方案 | 一句话理由 |
|---|---|---|---|---|
| 配置与密钥；Server 核心库（两维度同题） | gopkg.in/yaml.v3 v3.0.1（server 与 launcher 直接依赖） | 替换 | go.yaml.in/yaml/v3（YAML 组织维护的同一代码线） | 上游已于 2025-04-01 归档；只需换导入路径，属维护线切换，稳态收益偏小 |
| 可观测性与运维 | 管理日志在 slog 写锁内逐条同步 INSERT，再同步执行清理 DELETE；写句柄 synchronous=FULL | 替换 | 有界通道 + 单事务批量写入 + 定时清理，JSONL spool 兜底 | 每条日志一次 fsync，串进消息处理路径，并与业务争抢唯一写连接 |
| 数据库 | PRAGMA 经 `db.ExecContext` 执行，只落到当时那一条连接 | 替换 | DSN 简写键或连接钩子，写句柄加 `_txlock=immediate` | 写连接被中断替换后，外键与 busy_timeout 静默失效；三票一致 |
| 管理面认证与会话 | HMAC 签名声明 + 服务端查表；签名密钥与 session_id 明文入库 | 替换 | 256-bit 随机不透明令牌，库内只存 SHA-256 | 签名并不省去查表；拿到备份即可伪造会话；final_verdict 为替换，仓库证据已核实（三票见补充维度数据） |
| Launcher 渲染层 | React 19 + Fluent UI React v9 + motion 13 | 替换 | Vue 3 + Reka UI + motion-v + @vue/test-utils | Fluent 外观已被整体覆盖，React 只为 Launcher 存在 |
| 整体系统形态 | 仓库脚本同时用 Python 3.14 与 Node 两种语言 | 替换 | 契约、生成、发布、工具链改为 Go 工具；Node 只保留前端与开发编排 | 生成器与被生成代码同语言；仓库工具维度同题为“保持”，见 R6 |
| 渲染与媒体 | BtbN FFmpeg 静态 GPL 包，首启在 HTTP 监听前同步下载 | 替换 | 同一发布的 gpl-shared 变体；FFmpeg 不再阻塞首启 | 下载量约减半；慢网下不再被 15 分钟就绪预算反复终止 |
| Web 前端框架 | Web 用 Vue、Launcher 用 React，两套并存 | 考虑 | 统一到 Vue | 三票调整后均为替换，但三票都判反驳，降一档为考虑；内容并入 R5 |
| 契约与代码生成 | websocket-events.yaml 内嵌并复制 payload schema，配模板式 TS 生成器 | 考虑 | payload schema 迁入 OpenAPI components | 已出现真实漂移（logs.appended.protocol）；三票一致 |
| 数据库 | management_logs 用 TEXT 时间戳 + 表达式索引 | 考虑 | INTEGER 纳秒 + 普通索引；定时清理；WAL 下 synchronous=NORMAL | 收益温和；根因由日志写入模型那一项解决 |
| 数据库 | VACUUM INTO 快照占用唯一写连接 | 考虑 | 改走仓库已有的独立连接快照函数 | 约一行改动，但当前规模下阻塞不到 1 秒 |
| 可观测性与运维 | 诊断包只有业务状态 JSON | 考虑 | 加入 goroutine/heap/allocs profile 与 runtime/metrics 快照 | 标准库、零依赖；但仓库无泄漏记录，属预防性 |
| Launcher 宿主 | Windows 缺 WebView2 时没有任何提示 | 考虑 | 启动前查注册表，缺失时弹原生提示框 | 约 50 行、零新依赖；三票一致 |
| Launcher 宿主 | macOS .app 未完整签名封存，也没有放行说明 | 考虑 | 文档写明 xattr 放行；完整 ad hoc 签名的效果待实机验证 | 不涉及任何密钥；付费公证已被否 |
| 质量工具 | 架构门禁有 Python 正则与 Go AST 两套实现 | 考虑 | 只保留 Go AST 架构测试 | 两套规则已经分叉；三票一致 |
| 配置与密钥 | YAML + JSON Schema 2020-12；secret 明文存 SQLite；无环境变量覆盖 | 保持 | — | schema 已是单一来源；表单元数据已有单测守护 |
| 插件 SDK 与分发 | Go SDK、sdk/vue、iframe + CSP、catalog 静态 JSON 单源 | 保持 | — | 没有可信的免费镜像；协议夹具已与语言无关 |
| Web 前端框架 | Vue 3.5 + Vite 8 + Pinia 4 + 自写 http.ts/ws.ts | 保持 | — | Pinia Colada、openapi-fetch、Vitest 5 的收益都不实质 |
| 可观测性与运维 | slog；不用 OTel 与 Prometheus；VACUUM INTO 备份 | 保持 | — | Prometheus 端点已试过并删除；FTS5 不适合中文两字词 |
| Server 核心库 | Go 1.27 + chi + coder/websocket + xi2/xz | 保持 | — | ulikunitz/xz 无法设字典上限；json/v2 的收益已自动生效 |
| 整体系统形态 | Go 单二进制 + 原生插件 + SQLite + Vue SPA + 独立 Launcher | 保持 | — | 把托盘并入 Server、embed web/dist 都不成立 |
| 契约与代码生成 | 手写 OpenAPI 3.1 + JSON Schema + 自定义 YAML；Server 侧不做代码生成 | 保持 | — | 没有 handler 漂移记录；oapi-codegen 无法直接消费本契约 |
| CI、发布与分发 | nightly + tag 发布；不签名、不发 SHA-256、不做镜像 | 保持 | — | 签名、公证、attestation、容器镜像在当前规模与约束下都不成立 |
| 数据库 | SQLite + modernc + sqlc + 手写迁移 | 保持 | — | 引擎、驱动、访问层都是最佳匹配 |
| 仓库工具 | .tool-versions + Corepack + Makefile + 独立 pnpm 工作区 + 自写 start-dev | 保持 | — | mise、Taskfile、根 workspace、uv 的收益被高估 |
| Launcher 渲染层（其余） | Wails bindings 适配层；独立渲染层 | 保持 | — | 共享运行时、文案 i18n、契约生成类型都被推翻 |
| 插件运行时 | 原生子进程 + JSONL v4；无沙盒 | 保持 | — | 多语言 SDK 已在 v0.3 有意删除 |
| 渲染与媒体（其余） | chromedp + Chrome for Testing 完整构建 + html/template | 保持 | — | 换 headless-shell 会失去可见窗口登录；统一启动栈的收益不成立 |
| Web UI 与样式 | Reka + shadcn-vue + Tailwind 4 + SCSS + motion-v + vue-i18n | 保持 | — | 去 SCSS、去 vue-i18n、去 motion-v 都被推翻 |
| Launcher 宿主（其余） | Wails v3 beta.9 + 独立 Go module | 保持 | — | Tauri、Electron、Wails v2、Fyne、纯托盘方案都更差 |
| 质量工具（其余） | Go testing + rapid；golangci 5 个 linter；不用 ESLint/Prettier；无 push 门禁 | 保持 | — | 各工程内部风格一致；push 门禁是刚有意删除的 |
| QQ 官方适配器 | 自写 WebSocket 网关 | 保持 | — | 官方未下线 WS；botgo 已停更 |
| OneBot11 协议层 | 自写 OneBot11 四种传输 | 保持 | — | NapCat 不支持 Milky |
| 管理面认证（其余） | CSRF 派生令牌 + 精确 Origin；cookie 与 Bearer 双传输 | 保持 | — | 标准库 CrossOriginProtection 比现状更宽松 |

## 2. 建议替换

### R1 YAML 库：gopkg.in/yaml.v3 → go.yaml.in/yaml/v3

**现状与仓库证据**
- `server/go.mod:15` 与 `launcher/go.mod:8` 直接依赖 gopkg.in/yaml.v3 v3.0.1。这是该路径的最后一个版本，发布于 2022-05-27。
- 使用面很轻：
  - `server/internal/config/canonical.go:74,90`：Unmarshal 到 map，以及 Marshal 规范文档；
  - `server/internal/platform/runtimepaths/database.go:25`；
  - `launcher/internal/desktop/management.go:16,53`；
  - 另有 testutil 与 8 个测试文件。
- 合计 13 处导入。`docs/engineering/baseline.md:67` 把它列为固定选型。

**更佳方案**
- 换成 go.yaml.in/yaml/v3（v3.0.5，2026-07-26），只改导入路径，server 与 launcher 两个 module 同时切换。
- 不采用 v4：仍停在 v4.0.0-rc.6（2026-06-17）。
- 不采用 goccy/go-yaml：Marshal 输出细节不同，会破坏 `canonical.go` 的规范化写回。

**为什么更佳**
- 上游 go-yaml/yaml 已于 2025-04-01 归档，README 标注 unmaintained；YAML 组织接手的是同一代码线。
- Wails v3 beta.9 的模块图里已经有 go.yaml.in/yaml/v3 v3.0.4，Kubernetes、Prometheus、Forgejo 都已迁移。

**校验中被更正的事实**
- yaml/go-yaml#432 不是“已在新分支修复的 emitter 问题”，而是 2026-09-30 新开、面向 v4 的未合并性能 PR。
- gopkg.in/yaml.v3 v3.0.1 目前没有未修复的漏洞条目；fork 接管 18 个月里也未发布过任何安全修复。“获得安全修复”只是前瞻性的保险。
- govulncheck 不感知仓库是否归档，切换后 CI 信号不变。
- 改动面不止原文说的“3 个文件 + 2 个测试辅助”，而是 13 处导入、2 份 go.mod/go.sum、`baseline.md` 与 `THIRD_PARTY_NOTICES.md`。
- v3.0.1 到 v3.0.5 之间有没有非测试行为差异，三票说法不一：
  - 一票列出三处：合并键不可哈希时由 panic 改为返回错误；块标量以换行开头时保留首换行；新增可选的 CompactSeqIndent。
  - 另两票称默认 Marshal 输出逐字节相同。
  - 仓库写回只用 `yaml.Marshal(map)`，规范化输出不受影响。

**残余风险**
- v3 冻结分支不修非安全 bug。
- go.yaml.in 域名依赖 YAML 组织持续托管，可由 GOPROXY 缓解。
- 两个维度的稳态价值票都认为，这只是“同样好，外加一份免费保险”。

**与既有取舍的关系**
- `baseline.md` 要求替换固定选型时说明理由，这里的理由就是上游归档。
- 按 AGENTS.md，需要同步 `baseline.md`、两份 go.mod/go.sum 与第三方声明。

### R2 管理日志写入模型：逐条同步 → 异步批量

**现状与仓库证据**
- 写入路径：
  - `server/internal/platform/logging/summary_writer.go:36-66` 的 Write() 持有 w.mu 时调用 `stream.appendNormalized`；
  - `stream.go:98-141` 对每条日志同步 SaveSummary（5 秒超时），成功后再同步 PruneOlderThan；
  - `store_open.go` 的写句柄 MaxOpenConns=1，synchronous=FULL。
- slog 的 JSONHandler 在自身互斥锁内调用 Writer，所以整个进程的日志调用都排在一次 SQLite 提交之后。
- `docs/dev/logging.md` 规定默认逐条记录消息收发，OneBot shell 又是单 goroutine 派发，因此每条 QQ 消息的处理路径上都有一次 fsync。这条路径还与插件 KV、任务等业务写入争抢唯一的写连接。
- 维护者近两天的 4 个 perf(logging) 提交（67af5b4f、1c525f18、4158ab51、e6dcd069）都在压缩这条路径的常数项，没有改变“每行一次 fsync”的模型。
- 反例就在同一仓库：`server/internal/bot/messagestats/service.go` 已经用 30 秒批量事务刷盘。

**更佳方案**
- 保留 slog、SQLite 和同一个库文件。
- SummaryWriter 只负责脱敏和生成摘要，然后把摘要投入有界通道。
- 由单个 goroutine 按“每 ≤100 ms 或每 N 条”在一个事务里批量 INSERT。
- 保留期清理改为定时范围 DELETE，可参照 `RunKVExpiryLoop`（`app_run.go:130-132`）。
- JSONL spool 改作通道溢出和数据库不可用时的落盘；关闭时先 drain 再退出。

**校验剥离的部分**
- 不重写为 Record 级 tee Handler：e6dcd069 有意让 stdout 与摘要共享同一棵脱敏解析树，JSON 往返的开销与 fsync 相比可以忽略。
- 表达式索引不会因此减少：其中 5 个服务于读路径。
- 不拆独立的 logs.db：那样要同时修改备份清单和恢复流程。

**校验中被更正的事实**
- 零行 DELETE 不弄脏页面，既不写 WAL 也不 fsync。只有运行超过默认 7 天的保留期之后，才接近每条日志两次 fsync。
- `synchronous=FULL` 没有任何文档或提交说明，而且它本来就等于 modernc 内置 SQLite 的编译默认值。在 WAL 下改用 NORMAL 是更便宜的同向杠杆，见 C2。

**残余风险**
- 进程崩溃会丢失最后一个批次；stdout 镜像中的文本仍是完整的。
- 需要翻转 `TestStreamAppendsAfterSavingRepositoryDetail` 固化的“先持久化再推送”不变量，约 7 个集成测试要改用 flush 钩子。
- 一票认为，加上通道、溢出策略和 drain 之后，净代码量大致持平，因此判为“考虑”。

**与既有取舍的关系**
- 文档里没有任何关于“必须逐条同步”的取舍理由。
- 批量刷盘是仓库内已经接受的范式。

### R3 SQLite 连接配置：PRAGMA 落到每条连接

**现状与仓库证据**
- `server/internal/storage/store_open.go:46-70` 打开两个池：写池 1 条连接，读池 4 条。
- `configureHandle`（80-105 行）和 `PRAGMA query_only` 都通过 `db.ExecContext` 执行，按 database/sql 的语义只作用于当时取到的那一条连接。
- `store_test.go` 的断言是顺序执行的，始终复用同一条连接，因此掩盖了这个问题。
- modernc 不设默认 busy_timeout。

**更佳方案**
- 首选 modernc v1.55.0 起提供的 DSN 简写键：`_busy_timeout`、`_foreign_keys`、`_journal_mode`、`_synchronous`、`_query_only`。它们在解析期校验，按固定顺序应用，比原样拼接的 `_pragma` 更安全。
- 也可以用 RegisterConnectionHook，或 v1.56.0 新增的 NewConnector。
- 写句柄加 `_txlock=immediate`。
- 补一个开多条连接、逐条校验 PRAGMA 的测试。

**为什么更佳**
- 真正的暴露面在写池：
  - 文件库连接上的语句被 ctx 中断后，modernc 通过 ResetSession/IsValid 让 database/sql 丢弃该连接（`conn.go:940-967`，CHANGELOG v1.51.0）；
  - database/sql 随后用一条未配置的新连接替换唯一的写连接；
  - 结果是 foreign_keys=OFF、busy_timeout=0 一直持续到进程退出，而 `schema.sql` 中有两条外键。
- 读池的 query_only 也只保护了第一条连接。

**校验中被更正的事实**
- synchronous 与 wal_autocheckpoint 落不到新连接，并没有行为差异：编译默认值就是 `SQLITE_DEFAULT_SYNCHRONOUS=2`、`SQLITE_DEFAULT_WAL_AUTOCHECKPOINT=1000`。
- 读连接上的 foreign_keys 对只读查询没有意义。
- “消除并发读的 SQLITE_BUSY”被夸大了：WAL 读者的多数路径由 SQLite 内部处理。

**残余风险**
- 若将来日志改用 synchronous=NORMAL，需要为读写池区分 DSN。

**与既有取舍的关系**
- `docs/CHANGELOGS/v0.1.md:15` 把 WAL、busy_timeout、读写句柄分离列为既定设计，这项修复让既定设计真正生效。
- 全部在冻结版本线内，三票一致维持。

### R4 会话令牌：签名声明 → 哈希存储的不透明令牌

**现状与仓库证据**
- `server/internal/platform/auth/token.go` 签发 HMAC 签名的 JSON 声明（v/sid/sub/iat/exp）。
- `sessions.go` 的 ValidateWithContext 先验签，再按 sid 查 `m.sessions` 并比对 sub/iat。
- 滑动续期只修改服务端的 ExpiresAt，令牌里的 exp 从未被读取。
- 签名密钥存了两份：secrets 中的 `platform.auth.session_signing_key` 与 `auth_bootstrap_state.signing_key`，hydrate 时用后者覆盖前者。
- `server/internal/sqlcqueries/auth.sql` 中 signing_key 与 session_id 都是明文，`server/internal/operations/backup/archive.go` 又把整个库打进备份。

**更佳方案**
- 会话令牌改为 256-bit 随机不透明令牌。
- `admin_sessions` 以 SHA-256(token) 为主键，内存 map 也以哈希为键。
- 删除签名与验签、令牌版本字段、签名密钥的生成、双份存储与轮换代码。

**为什么更佳**
- 签名没有省掉任何状态查询。
- 攻击链：拿到 30 天内的备份或数据库文件 → 伪造仍有效的会话 → 以 Bearer 方式绕过 Origin/CSRF → 调用 `/api/plugins/install`（支持远程 URL，插件是完全可信的原生代码）。结果是“备份泄露”升级为“主机代码执行”。
- 只存哈希就能切断这条链。OWASP Session Management Cheat Sheet 和 The Copenhagen Book 都推荐单向存储。
- 以哈希为键查表，对计时侧信道的防护与现状等价。

**校验中被更正的事实**
- `server/internal/app/platform.go:91-112` 的“新生成密钥即清空会话”在已初始化的实例上并不能完成轮换：hydrate 会换回 bootstrap 中的旧密钥，只留下清空会话的副作用。reset-admin 之后重新初始化，又会把同一把密钥复制回去，所以这把密钥实际上从不轮换。
- 若单独实施，必须重新定义 CSRF 的派生方式：当前派生依赖签名密钥。可改为从原始令牌做带域分隔的派生，或为每个会话另存一个随机值。
- 新增迁移 000007 必须同步 `contracts/backup-manifest.schema.json` 的结构版本枚举、对应 fixtures 与 `docs/user/recovery.md`，这是常规契约改动。
- 安全收益比原文写的窄：同一 OS 用户和插件进程本来就有同等权限，真正增加的防护只针对“备份或库文件被带离主机”这一种情况。

**残余风险**
- 升级后旧会话全部失效，需要重新登录。
- 备份中的插件 secret 仍是明文，备份泄露整体上依然高危。

**与既有取舍的关系**
- 仓库里没有关于“为什么用签名令牌”的文档。
- 这是纯 server 内部调整，只用到已在用的 crypto/rand 与 crypto/sha256。final_verdict 为替换，三票均维持替换（票据见补充维度 admin-auth-session 数据，不在输入文件中），仓库证据已核实。

### R5 Launcher 渲染层：React + Fluent → Vue + Reka

**现状与仓库证据**
- `launcher/package.json`：react 19.2.8、@fluentui/react-components 9.74.6、@fluentui/react-icons、motion 13.3.0、@testing-library/react。
- 规模：27 个 TSX 共 2,793 行；TS+TSX 共 4,868 行，其中 472 行是生成的 bindings；另有 CSS 2,430 行。
- Fluent 的外观已被整体覆盖：
  - `launcher/src/renderer/src/launcherTheme.ts`（121 行）重映射约 40–45 个 Fluent token 与 fontFamilyBase；
  - `surfaces.css`（1860 行）含 50 处 `.fui-*` 选择器、63 处 `launcher-fluent-provider` 前缀，注释直接写明在与 Griffel 运行时注入的样式争优先级；
  - `LauncherDialog.tsx` 关闭了 Fluent 自带动画，改由 Motion 重做；
  - `launcher/pnpm-workspace.yaml` 用 overrides 钉住 @fluentui/react-motion 9.16.2；
  - `launcher/vitest.config.ts` 需要 inline @fluentui 与 tabster 才能在 jsdom 下运行。
- 设计目标已与 Web 一致：`docs/design/launcher-design-system.md` 写明“Launcher 与 Web 管理面使用同一套表面规则”。
- 维护者正在向 Web 靠拢：b7d9d2ca（2026-09-30）把 Launcher 改为白底浅灰；9c38a1bc（2026-09-15）的正文说明，原先 Fluent Motion 的写法“与 Web 的 Motion for Vue 不一致”。
- Fluent 的来历：953aae90（Avalonia FluentTheme 外壳）→ 95694cc8（Electron + Vue）→ 3 小时后的 1c52ad21 换成 React + Fluent，正文没有给出理由 → f9e81192（迁到 Wails）。
- React 只因 Launcher 存在；Vue 同时用于 web、sdk/vue 和示例插件 UI。

**更佳方案**
- 渲染层改为 Vue 3.5 + Reka UI 2.x + motion-v + @lucide/vue + @vue/test-utils。
- 以下部分不变：Wails bindings、`wailsDesktopApi.ts` 适配层、Go host 与 CSP。
- Playwright 的桌面桥模拟是 49 行按模块路由替换的 shim，与框架无关，可以沿用。

**为什么更佳**
- 去掉仓库里唯一的 React 栈，以及第二套组件库、测试库和动效绑定。
- Fluent 在这里只提供约 10 个外观被重绘的基础原语，却带来运行时 CSS-in-JS 与持续的覆盖摩擦。
- 单人加 agent 的维护模式下，少一套规范和一条升级线。
- 三个维度的同题建议中，共有 6 张票主张替换：launcher-renderer 两票、web-framework 三票、system-architecture 一票。

**校验中被更正的事实**
- 共享组件包被高估：
  - `web/src/components` 的 App* 组件依赖 `@/i18n`、Tailwind 的 `cn()` 和 SCSS，部分还依赖 Pinia 或 Router；
  - 仓库没有根 workspace，要复用就得新建共享包（并给 Launcher 引入 Tailwind），或复制约 10 个组件；
  - `sdk/vue/src/theme.css` 是插件 iframe 的变量桥，不是可复用的主题。
- 体积收益很小：`dist/assets` 约 25 MB，其中 24.6 MB 是三份 HarmonyOS 字体，JS 约 935 KB，不到 4%。
- “首帧不再等待 Griffel”不成立：`main.go` 的 initialWindowReadyTimeout=10s 等的是 WebView 导航完成事件，与 Griffel 无关。
- Launcher 的 token 输出只有约 8–30 行，换框架后作为独立包仍需保留。
- 两个工程已共用 vitest、jsdom、Playwright、Vite、TS 的同一版本，差异只在 testing-library 与 test-utils、plugin-react 与 plugin-vue。
- 并非“没有任何依据”：`PRODUCT.md:42` 写有“沿用 Fluent 标准控件与桌面操作词汇”，只是没有写选型理由。

**残余风险**
- Fluent 自带的高对比度样式已被整体覆盖，WCAG 2.2 AA、forced-colors 与键盘焦点需要在新组件上重新人工验收。
- Reka 的 Portal 挂载需要主题容器策略，可沿用 `web/src/components/AppDialog.vue` 的做法。
- 稳态价值票认为收益属中等。

**与既有取舍的关系**
- `docs/RayleaBot机器人项目规划.md:35` 把 React 列入 Frozen stack，替换意味着修改冻结栈声明（变为 Go + Vue + Wails + SQLite），并同步 `baseline.md`、`DESIGN.md`、`launcher-design-system.md` 与 `PRODUCT.md:42`。
- `PRODUCT.md:42` 的“沿用 Fluent 标准控件与桌面操作词汇”需改为与 Web 一致的控件表述。这是除 Frozen stack（`规划.md:35`）之外，第二处需要维护者确认放弃的既有表述。
- system-architecture 维度的同题建议（launcher-ui-unify-vue）合并为“保持”。原因是它的具体论据被更正，例如“直接复用 `web/src/components` 与 sdk/vue 主题”，而方向本身没有被否定，见第 4 节。

### R6 仓库脚本：Python + Node → Go + Node

**现状与仓库证据**
- Python 3.14.8 约 7.5 千行，其中测试约 2.1 千行；另有 `.github/scripts/generate-repo-stats.py` 359 行。主要文件：
  - `scripts/ci/validate_contracts.py`（1902 行）；
  - `scripts/release/*.py`；
  - `check-toolchain.py`、`check-server-structure.py`；
  - `generate-plugin-wire.py`、`generate-error-codes.py`。
- 依赖没有声明：仓库没有 requirements、pyproject 或 lockfile，5 处工作流内联 `pip install pyyaml==6.0.3 jsonschema==4.26.0`。
- 生成器要绕路生成 Go：
  - 通过 subprocess 调 gofmt；
  - 把 `scripts/templates/pluginwire_test.go` 当文本模板；
  - `generate-plugin-wire.py` 还要专门为 Python 发布工具生成 `contract_versions_generated.py` 与 `artifact_ids_generated.py` 两份桥接常量。
- `.devcontainer/Dockerfile` 要从 python 镜像多阶段复制。
- `check-server-structure.py` 与 `server/tests/architecture` 规则重复，见 C7。

**更佳方案**
- 契约校验、代码生成、发布打包和工具链核对改为仓库内的 Go 工具：
  - 放在独立的 tools module 并加入 go.work；
  - 只依赖标准库、jsonschema/v6 与 go.yaml.in/yaml，不 import `server/internal`；
  - 用 go/format 直接格式化输出。
- Node 保留开发编排、设计 token、openapi-typescript 与 WS 类型生成。
- `.tool-versions` 去掉 python。

**为什么更佳**
- 生成器与被生成代码同语言，可以去掉 gofmt 子进程、文本模板和两份 Python 桥接常量。
- Go 运行时校验调用了 AssertFormat，更严格。而 CI 没装 format 扩展，Python 侧 34 处 date-time 与 19 处 uri 的 format 校验实际被静默跳过。
- 少一门语言、一套 unittest、9 处 setup-python 与 5 处内联 pip install（一组无清单的依赖）。

**校验中被更正的事实**
- 三份 `.tool-versions` 读取器不会消失：`read-tool-versions.sh` 是任何运行时启动前的引导读取器，`tool-versions.mjs` 供 Node 脚本使用，Go 还要新增一份。
- 8 个 nightly job 中只有 4 个安装 Python，其中 server job 只为运行 `make doctor`。setup-python 命中缓存只需数秒，CI 时间上没有可度量的收益。
- 复用 server 现有代码的设想不成立：Go 的 internal 规则禁止外部 module 导入 `server/internal`；放进 server module 又会出现生成器依赖自身产物的引导循环。
- `start.bat` 与 `start.sh` 并不调用 Python。
- `validate_contracts.py` 没有使用任何 OpenAPI 库，OpenAPI 细则全部是自写的，Go 侧也不必引入 libopenapi 或 kin-openapi。
- “校验器与运行时同一实现、消除分歧”的价值，有一部分已由 `config_fixture_contract_test.go`、`schema_test.go`、`manifest_fixture_test.go` 获得。

**两个维度的分歧**
- repo-tooling 维度的同题建议（unify-repo-scripts-into-go）最终为“保持”。理由有三：
  - 稳态价值票认为 Python 在冻结工具链下几乎不产生维护动作；
  - 多数痛点可以就地修，例如补依赖清单、放宽 `check-toolchain.py:246` 对精确补丁版本的要求；
  - `baseline.md` 把 Python 3.14.8 列为 Repository scripting 的冻结选型。
- 两个维度的约束契合票与事实票都支持收敛到 Go，两张稳态价值票都认为差异不实质；system-architecture 维度最终为替换，repo-tooling 维度最终为保持。
- 多数痛点可先就地修补（见第 7 节）。

**残余风险**
- 1902 行校验器中有大量 OpenAPI 结构检查与 self-test，需按 fixtures 逐条做到行为等价。
- release 工具的 tar/zip 细节需要在 Go 中重写，包括 `archive_io.py` 的 Windows 属性处理，以及 memlimit=66<<20 的 LZMA 解码。
- `go run` 首次编译会慢几秒。

**与既有取舍的关系**
- AGENTS.md 要求“不引入平行技术栈”。
- `baseline.md` 的当前评估方向称，只用于仓库脚本的依赖说明必要性即可。
- CHANGELOG v0.3 已删除 Python/Node 插件运行时，此后 Python 只剩脚本角色，没有文档说明为何保留。

### R7 FFmpeg：静态包首启同步下载 → gpl-shared 变体且不阻塞首启

**现状与仓库证据**
- `.deps/manifest.json` 中，Windows 与 Linux 固定为 BtbN n9.0.1 full GPL 静态构建，三平台都只有 GitHub 单一来源：
  - win64 zip：169,202,846 B（约 161.4 MiB）；
  - linux64 tar.xz：约 120.7 MiB。
- `archive_extract.go` 全量展开。本机 `.deps/store/ffmpeg-windows-x64` 共 428 MB：
  - ffmpeg.exe 144.9 MB；
  - ffprobe.exe 144.7 MB；
  - ffplay.exe 146.9 MB（manifest 只声明前两个入口）。
- 首启必备：`startup_runtime_prepare.go:115-122` 的 startupRequiredRuntimeKinds 无条件加入 ffmpeg，测试 `TestStartupRequiredRuntimeKindsKeepsFFmpegWhenBrowserPathConfigured` 固化了这一行为。
- 首启阻塞链：
  - `app_run.go` 在 ListenAndServe、插件 ReconcileRuntime 与 startAdapters 之前，同步调用 AutoPrepareRuntimeEnvironments；
  - `download.go` 总超时 30 分钟、空闲超时 30 秒，不支持断点续传，失败即删除临时文件；
  - Launcher `timing.go` 的 startupReadinessBudget 为 15 分钟，到期 ForceKill。
- 核心自身不调用 FFmpeg，只通过环境变量把路径注入插件进程。

**更佳方案**
- 换用同一 BtbN 发布中的 gpl-shared 变体，只需改 manifest 的 URL、sha256 与 entrypoints：
  - 体积：win64 76,434,028 B（约 72.9 MiB），linux64 约 57.4 MiB；
  - 编解码器：gpl-shared 就是 gpl 加 `--enable-shared`，x264/x265 都保留；
  - 可运行性：Linux 链接时带 rpath `$ORIGIN/../lib`，Windows 的 DLL 与 exe 同在 bin/，`archive_extract.go` 已支持归档内的相对符号链接。
- FFmpeg 不再阻塞首启 HTTP 监听：
  - 把它移出 startupRequiredRuntimeKinds，或改为后台准备；
  - 按需准备沿用已有入口：`POST /api/system/runtime/bootstrap` 已接受 `resources: ["ffmpeg"]`（`system_handlers.go:219-237`），`DashboardView.vue:243` 已有对应按钮；
  - `StartupRuntimePhaseNotRequired` 已存在，Launcher 预检对 FFmpeg 缺失也已返回 Severity "ok"。

**校验剥离的部分**
- xz 解码器不换成 ulikunitz/xz，原因见第 4 节。
- 不加 FFmpeg 镜像：
  - 不存在可信的 BtbN 镜像。npmmirror 上的 ffmpeg-static 是 eugeneware 的构建；另一个 ffmpeg-builds 目录是第三方版本线，最新 v8.1.3，没有 9.0 线；
  - 自建镜像会让项目承担 GPL 二进制再分发义务（提供对应源码等）。当前 FFmpeg 不进入发布包、由用户运行时下载，这是现状描述，不是文档写明的取舍；`docs/release/delivery-and-upgrade.md:105` 本身允许依赖资源来自镜像。不加镜像的主要理由是不存在可信的 BtbN 镜像。

**为什么更佳**
- FFmpeg 下载量减少约 55%。
- 没装媒体插件的部署不必在首启等待 161 MiB。
- 到 GitHub 的速度低于约 180 KB/s 时，现状下首启可能被反复 ForceKill，每次都从零重下；不阻塞首启后就不会出现这种情况。

**校验中被更正的事实**
- 磁盘占用降不到原文说的“150 MB 以下”：gpl-shared 解压约 191 MiB（238 个条目），只保留 bin/ 并去掉 ffplay 约 160 MiB。
- shared 变体中的 ffplay 只有 17.3 MiB，所以“跳过 ffplay”的大幅收益只对静态变体成立，两者不能叠加。
- “按插件声明触发”仍需修改 plugin-info 契约与 readiness 语义。最小实现是把 FFmpeg 从“首启必备”改为“后台或按需准备”。

**残余风险**
- BtbN 月末构建保留两年，仍需定期滚动版本。
- 插件在 FFmpeg 就绪前调用会失败，需要清晰的错误码和修复提示。
- 稳态价值票认为只有 gpl-shared 一项站得住，“不阻塞首启”由另两张票支持，属多数结论。

**与既有取舍的关系**
- 不影响 `baseline.md:41-44`（BtbN 日期固定 URL 每次分发前需验证）与 `:96`（不新增 Go 媒体编解码栈）。
- 改动仍在 `.deps` manifest v5 的边界内。
- 维护者在 99887c87 中自述“首次启动准备 Chromium 或 FFmpeg 需要下载数百 MB”，但只放宽了开发脚本的等待时间。

## 3. 值得考虑

第 8 条“考虑”（web-framework 的 unify-launcher-to-vue）与 R5 同题，见 R5。

### C1 WebSocket 事件 payload schema 并入 OpenAPI components

**现状与仓库证据**
- `contracts/websocket-events.yaml`（499 行）的 payload 内嵌 JSON Schema，并在 174–180 行 `$ref` 了 OpenAPI 的 PluginState 等定义。
- 逐字复制了 OpenAPI 的定义：
  - 204–325 行约 120 行，复制自 AdapterDescriptor、AdapterIdentity、OneBot11ProtocolSnapshotResponse、ProtocolTransportStatus、ProtocolIssue；
  - logs.appended 复制了 LogSummaryFields。
- `web/scripts/generate-websocket-types.mjs`（278 行）不是 schema 驱动的通用生成器：
  - 186–271 行硬编码 TS 模板；
  - 用 branchWithRequired 按 required 键挑选 oneOf 分支；
  - 从文档字段 `event_type.examples` 推导 managementEventTypes；
  - 自写了 `$ref` 解析器。
- 2026 年该生成器 13 次提交中，有 11 次与契约同提交修改。
- 生成器第 245–247 行输出的已是 `components['schemas']['AdapterDescriptor'][]`，说明 Web 类型早就以 OpenAPI 为准，WS 契约文本只是靠人手维护的副本。

**已发生的漂移**
- `logs.appended.protocol` 仍是 `enum: [onebot11]`，自 1517b3c5 起未改。
- OpenAPI 的 `LogProtocol` 在 2026-09-09（052b9dea）已加入 qqofficial。
- `summary_writer.go:221-247` 会经 /ws/logs 推送 `protocol: qqofficial`。也就是说，正式 WS 契约已连续三周把真实帧判为非法。

**更佳方案**
- payload schema 迁入 `web-api.openapi.yaml` 的 components.schemas。
- `websocket-events.yaml` 只保留 channel、admission、envelope、replay_policy，并 `$ref` 这些 schema。
- openapi-typescript 顺带产出 payload 类型，生成器缩减为只输出常量。
- 不采用 AsyncAPI 3。

**校验中被更正的事实**
- Python 校验器现在就已用跨文件 registry 校验 WS fixtures（`validate_contracts.py:1044-1064`），这不是新收益。
- “Fern 指南称 Go 侧只有 code-first 库”无法核实，且说法有误。Go 侧有 lerenn/asyncapi-codegen（不支持 WebSocket）和 bdragon300/go-asyncapi（支持 WebSocket，仍在活跃开发）。不过为 3 类事件引入 AsyncAPI 工具链仍不值得。

**残余风险**
- 依赖 if/then 的 oneOf 分支，判别逻辑仍需手写。
- Go 侧不按 WS schema 校验帧，这是另一处缺口。

**与既有取舍的关系**
- `contracts/README.md:62` 已把“插件状态引用 OpenAPI 同一 schema”写成既定设计，这条建议只是把既定模式推广到全部 payload。
- 按契约优先规则，先改契约再改生成器。三票一致维持。

### C2 management_logs 的时间列与持久化级别

**现状与仓库证据**
- `server/internal/storage/schema.sql:124-165` 的 5 个索引逐字重复同一段 CASE/strftime 表达式，另有 1 个 julianday 清理索引。
- `migration_000005.go` 注释写明“查询必须使用该表达式才能命中索引”。
- `store_open.go:93` 设 synchronous=FULL，没有文档或提交说明，git 只能追溯到 be7d12e2 的拆分重构。

**更佳方案（按价值排序）**
1. 保留期清理移出写路径，改为定时执行，与 R2 合并。
2. 写句柄在 WAL 下改为 synchronous=NORMAL。sqlite.org/pragma.html 说明：WAL 下 NORMAL 不会损坏数据库；应用崩溃时事务仍然持久；只有断电或系统崩溃可能丢失最后几笔已提交事务；官方称它是多数 WAL 应用的最佳平衡。
3. ts 改为 INTEGER 纳秒，配普通复合索引；在迁移 000007 中一次性规范化历史格式。

**校验中被更正的事实**
- 只能用纳秒：`contracts/web-api.openapi.yaml:3362` 规定 9 位小数，`timestamp_test.go` 断言纳秒级排序。
- 手写 SQL 是因为可选过滤与变长 IN，改了时间列类型也回不到 sqlc。
- 索引收益温和：从 6 个降到 5 个，每行约省 110 字节。新行自 2026-09-22 起已是 30 字符的固定格式，走表达式的快速分支。
- 新增迁移必然要更新 backup-manifest 契约的结构版本枚举。
- synchronous 作用于整个写句柄，包括业务小表。
- 根因是同步写入：第 2、3 项只能减轻，R2 才能消除。

**残余风险**
- 断电场景下持久性弱于 FULL。
- `schema.sql` 与 LogTimestampExpression 的一致性已由 `migration_000005_test.go` 守护，维护风险本身不高。

**与既有取舍的关系**
- 表达式索引纯粹是为兼容历史格式而存在（fb557706 之前 ts 还是 julianday）。忽略迁移成本时，这条约束就不存在了。

### C3 快照移出唯一写连接

**现状与仓库证据**
- `server/internal/storage/snapshot.go:52-57` 的 Store.CreateSnapshot 在 MaxOpenConns=1 的写池上执行 VACUUM INTO。
- RunSnapshotLoop 启动时立即执行一次，之后每 6 小时一次（`app_run.go:127`）。
- `system_backup.go:63-65` 在路径匹配时也走写句柄。

**更佳方案**
- 改走已存在的包级函数 `CreateSnapshot(ctx, path)`（`snapshot.go:32-50`）。它使用独立连接并设置 busy_timeout，CLI 离线备份和回退路径已经在用，改动约一行。
- 不改用 Backup API：源库被其他连接写入时，分步备份会从头重启，而本产品持续在写日志。
- 读池也不行：query_only 下执行 VACUUM INTO 会报只读错误。

**校验中被更正的事实**
- 其他写入各自带 5 秒的 ctx，超时即失败告警，不会阻塞到快照结束；5 分钟是快照自身的预算。
- 当前规模下阻塞很短：在维护者本机 52.6 MB 的快照副本上实测，VACUUM INTO 用时 0.32–0.35 秒（Python 3.14 自带 SQLite 3.50.4，synchronous=FULL）。modernc 估计在 1 秒量级，远未触及 5 秒超时；库再大约 10 倍或放在慢盘 VPS 上才会显现。
- WAL 下独立连接执行的 VACUUM INTO 是读事务。现在的排队来自 database/sql 连接池的独占，而不是 SQLite 的锁。

**残余风险**
- 几乎没有。当前数据量下收益很小，这也是它停在“考虑”的原因。

**与既有取舍的关系**
- 引入提交 29b13279 没有正文说明，文档里也没有“快照必须走写句柄”的理由。

### C4 诊断包加入 Go 运行时 profile

**现状与仓库证据**
- `server/internal/operations/system/system_diagnostics_archive.go:20-57` 导出的内容：
  - system-status、readiness、doctor、plugins、config-summary；
  - 最近 100 条日志摘要；
  - spool 与 quarantine 文件。
- server 源码中 pprof、expvar、runtime/metrics 零引用。

**更佳方案（收窄后）**
- 在 zip 内加入 runtime/pprof 的 goroutine（debug=2 文本）、heap、allocs profile，以及 runtime/metrics 快照。
- Go 1.27 的 goroutineleak profile 已 GA，可以一并纳入。
- 全部标准库、零依赖。

**校验剥离的部分**
- 不采集进程 RSS 与句柄数：标准库没有跨平台 API。Linux 要读 /proc；macOS 在 CGO_ENABLED=0 下没有干净的路径；Windows 需要手写 psapi/kernel32 绑定。
- 不导出“最近 N 条原始 JSON 日志”：服务端没有这份数据。原始行只写到 stdout，由 Launcher 镜像到文件，而 Linux 服务端包没有 Launcher。
- 不加 net/http/pprof 路由与配置开关：这属于对外接口和配置契约的变更。

**校验中被更正的事实**
- 仓库里没有任何 goroutine 泄漏或内存增长的实际记录，git log 和 `docs/release/acceptance-and-risks.md` 中都没有，所以这是预防性改进。
- panic 已有覆盖：`httpapi.go:73` 会写出 debug.Stack，Launcher 还镜像了 stderr。
- 真正没覆盖的是“不崩溃的挂起或缓慢增长”。其中 Unix 可以用 SIGQUIT 让运行时打印栈，只有 Windows 没有替代手段。

**残余风险**
- profile 可能包含插件路径或消息片段，需要沿用现有脱敏边界，并更新 `docs/dev/diagnostics.md`。
- 诊断包会增大数百 KB。

**与既有取舍的关系**
- 契约把导出描述为“bounded diagnostics bundle”，zip 内新增文件不改变契约字段。

### C5 Windows 缺 WebView2 时给出原生提示

**现状与仓库证据**
- `docs/release/windows-desktop-runtime.md` 承认，缺少 WebView2 时“Launcher 可能在显示窗口前退出”。
- Launcher 以 `-H windowsgui` 构建，stderr 不可见；Go 代码中没有任何 WebView2 检测，`main.go` 也没有设置 ErrorHandler。
- 三张票对 Wails v3 beta.9 源码路径给出两种推断：
  - 两票（约束契合票、稳态价值票）认为 setupChromium 只记日志就返回，随后在 nil 对象上崩溃：10 秒兜底计时器弹出一个无边框的空白窗口，约 3 秒后 nil controller 触发 panic，或在 nil webview 上调用 Navigate 时崩溃；
  - 另一票（成熟度与事实票）认为会进入 errorCallback，然后 os.Exit(1) 静默退出。
- 两种推断的共同点是：用户看不到任何可操作的提示。`WINDOWS-RUNTIME.md` 放在 zip 里，双击启动的用户通常不会读。

**更佳方案**
- 保持 Wails，在 `application.New` 之前用 `golang.org/x/sys/windows/registry` 读取 HKLM 与 HKCU 下 `EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}` 的 pv 值。
- 缺失时用 MessageBoxW 提示，并用 ShellExecute 打开微软的下载页。
- 实现约为一个 50 行的 `_windows.go`，加一个解析 pv 值的单测。x/sys 已是直接依赖。
- Wails 的 webviewloader 位于 `internal/` 下，无法导入，所以自己检测是最省的路径。

**校验中被更正的事实**
- 并没有“失去 Wails 内置的 download/embed/browser 策略”：那是 Wails v2 `wails build -webview2` 的标志。v3 beta.9 运行时没有这类策略，只有 NSIS 模板宏和 `wails3 generate webview2bootstrapper`，而安装器与“无安装器”的交付方式冲突。
- Evergreen Bootstrapper 约 1.8–2 MB，不是 150 KB；Fixed Version 运行库超过 250 MB。
- “影响主力用户”的说法夸大了：Windows 11 自带运行库，绝大多数 Windows 10 已安装，受影响的是离线、精简或受企业管控的设备。

**残余风险**
- 注册表键名可能随微软策略变化。
- 建议不内嵌 bootstrapper，避免额外的再分发条款审查。

**与既有取舍的关系**
- 与 `launcher/AGENTS.md` 中“失败给出修复指引”的规则一致。
- 也符合微软 WebView2 分发文档“创建前先检测运行库”的建议。三票一致维持。

### C6 macOS 首启放行（与签名体系无关）

**现状与仓库证据**
- `launcher/scripts/build-package.mjs` 的 darwin 分支手写 Info.plist 后直接 go build，没有 codesign；`release-build.yml` 的 macos-26 job 也没有签名步骤。
- 可执行文件只有 Go 链接器签名（adhoc,linker-signed）：Info.plist 未绑定，没有 `_CodeSignature/CodeResources`，整个 bundle 未被封存。
- `delivery-and-upgrade.md` 把 macos-arm64-full 标为 first_class。
- `docs/user/deployment.md` 的首次安装段没有 macOS 放行步骤。延后状态其实已有记录：`docs/release/notes/v0.5.0.md:57`、`v0.7.0.md:75` 与 `docs/CHANGELOGS/v0.5.md:81` 都写明“不含 Developer ID 签名、公证或 Gatekeeper 验收”，并列为延后项。

**更佳方案（零成本层）**
- 在 `docs/user/deployment.md` 写明对整个解压根执行 `xattr -dr com.apple.quarantine <解压目录>`，同时覆盖 .app 和包根的 raylea-server。
- 在 darwin 构建之后，由内向外用 `codesign --force --sign -` 对 .app 与 raylea-server 做完整的 ad hoc 签名。
- 完整 ad hoc 签名后能否恢复“系统设置 › 隐私与安全性 › 仍要打开”这条路径，票间看法不一：launcher-host 的稳态价值票认为，再做一次包级 `codesign -s -` 也不会改变 Gatekeeper 的结果。这一步的效果需要在 Apple Silicon 实机上验证。
- 付费公证见第 4 节。

**校验中被更正的事实**
- 右键“打开”的绕过方式在 macOS 15.0 Sequoia 就已移除，不是 15.1。15.1 上出现的“无法打开 (null)”是系统 bug，“仍要打开”路径仍然保留。
- 原文引用的 Tahoe 26.6/26.7 CVE 修的是恶意绕过，与合法用户的放行流程无关。当前最新系统是 macOS 27（2026-09-14 发布）。
- 在 Apple Silicon 上，带 quarantine 的未封存 bundle 通常提示“已损坏，无法打开”，而且不出现“仍要打开”。
- 与 Windows/Linux RUNTIME.md 的“不对称”论据不成立：那两份文档讲的是系统运行库，WKWebView 是 macOS 自带的。
- GitHub 上 macOS 产物累计下载次数为 0。

**残余风险**
- 不签名就始终有首启摩擦。
- 若 macOS 用户确实很少，可考虑把 support_level 从 first_class 降为 experimental。这是原建议给出的备选，校验没有否定。

**与既有取舍的关系**
- 不涉及任何密钥，与“发布签名已被有意删除”不冲突。

### C7 架构门禁收敛到 Go AST 测试

**现状与仓库证据**
- `scripts/check-server-structure.py`（Python 正则，经 make doctor 运行）与 `server/tests/architecture/structure_test.go`（go/parser）规则重叠，adapter 边界的禁止集合（management、config/runtime、operations/system、app）逐字相同。
- 重复维护有实证：
  - af732b55 与 29b33492（2026-09-10）在同一提交中把同一条规则写了三遍：脚本、脚本测试、Go 测试；
  - 47761214 重组包路径时，三处一起修改。
- 两套规则已经分叉：c2a52049 只在 Go 侧把共享模型扩到 10 个，并加入 render。
- Python 脚本原本承担的包尺寸预算与 fan-out 棘轮，已在 b0573288 删除。

**更佳方案**
- Go AST 测试作为唯一实现。
- 先把 Python 独有的规则全部迁入 Go 测试，再删除脚本及其测试：
  - 禁用目录名；
  - 包名与目录名不一致的警告；
  - management 层的手写 SQL；
  - os.Exit/log.Fatal，用 go/ast 检查 CallExpr，而不是交给 forbidigo；
  - platform/health 不得 import 任何 internal 包；
  - platform/runtimepaths 的导入边界；
  - render 不得 import plugins/actions、lifecycle、runtime；
  - management 不得 import plugins/runtime。
- 不用 depguard。

**为什么更佳**
- 规则只在一处维护。
- Go 测试随 server-windows 在 Windows 上也会运行；Python 检查只在 Linux nightly 的 make doctor 中运行。
- `quality-gates.md` 的本地最小验证不含 make doctor，迁入 Go 测试后，跨包改动跑 `go test ./...` 就能在本地发现。

**校验中被更正的事实**
- forbidigo 需要 `.golangci.yml` 才能配置禁止模式（默认模式是禁 fmt.Print*），而且只在 nightly 运行。
- “Python 正则脆弱”被夸大：IMPORT_LINE_RE 能处理别名和行注释，`is_generated_go_file` 与 Go 侧的判定逻辑相同。真正的弱点是 check_process_exit_calls 和 check_management_sql 对全文做正则匹配，会误报注释和字符串里的文本。

**残余风险**
- 删除脚本前必须覆盖全部 Python 独有规则，否则门禁会变弱。

**与既有取舍的关系**
- `baseline.md`“保留仓库专用的结构测试，它比通用 linter 更准确”中，比较对象是通用 linter。这条建议正是保留并加强仓库专用的 Go 测试，三票一致维持。

## 4. 被校验推翻的质疑

以下 46 条原判为替换或考虑，最终为保持。“推翻”指它作为选型改动不成立；其中几条在校验中留下了可就地修的小问题，汇总在第 7 节。括号内为原判。

**配置、校验与密钥**
- 由 schema 推导表单元数据（考虑）：
  - 所谓 21 处对 25 处 restartRequired 的“漂移”，其实是覆盖范围不同：差额是 database.engine、adapters 和 adapterInstance 的 id/type，都不在通用表单里。
  - `web/tests/unit/config-workbench.spec.ts` 已逐字段断言 default 与 restartRequired 和 schema 一致。

**插件 SDK 与分发**
- 商店多镜像（替换）：
  - 候选的 raw.githubusercontent.com、cdn.jsdelivr.net、*.github.io 处在同一类封锁面上，jsDelivr 的 /gh/ 也不服务 Release 附件，资产镜像没有免费的提供方。
  - deps 的 1 MiB Range 测速是为大归档设计的，不适合 KB 级的 catalog。
  - 商店与核心更新、FFmpeg 同样是 GitHub 单源，两者一致，并非例外。
  - 三票均降为“考虑”，合并结论为保持。
- 拆分并发布 @rayleabot/plugin-ui（考虑）：
  - sdk/vue 一共 291 行，胶水代码不到 100 行。
  - 在宿主内，主题已由 readHostTheme 从宿主的计算样式读取；theme.css 的默认值只在脱离宿主时生效，对比度也达标。
  - `docs/design/plugin-management-surface.md` 规定宿主不规定插件的配色与字体。
  - 校验另外发现了真实的分发缺口，见第 7 节。
- 多语言 wire 类型与一致性夹具（考虑）：
  - `fixtures/plugin-protocol` 已有 72–75 份语言无关的帧序列夹具，由 Python 校验器与 Go 的 TestContractFrames 两套实现执行。
  - dev-sync 安装的插件还会被逐帧做 schema 校验。
  - 缺的只是一个拉起第三方二进制的执行器。
- 插件脚手架（考虑）：
  - 维护者在 60 天内没有脚手架也建成了 11 个插件仓库，各 release.yml 之间几乎没有漂移。
  - 公开仓库 RayleaBot/plugin-echo 就是经过真实发布验证的模板，缺的只是文档把它标为起点。

**Web 前端框架与数据层**
- Pinia Colada（考虑）：
  - 引用的修复 5d850690、435d2caf 都在建议自己排除的日志流区域。
  - 目标 store 中与 loading、错误、取消相关的代码只占约 14%。
  - 列表去重与“只重取已加载页数”已集中在 `collection-pager.ts` 与 `refresh-scheduler.ts`。
  - defineColadaLoader 来自 vue-router/experimental。
- Vitest 5（考虑）：
  - 唯一的收益是删掉 `scripts/run-vitest.mjs`，而在 Vitest 4.1.10 上用 test.execArgv 配置就能做到。
  - 升到 Vitest 5 只是例行版本跟进，不是选型问题。
- openapi-fetch（考虑）：
  - 42 处 `apiRequest<X>` 的泛型都是 `components['schemas']` 的别名，契约形状一变 vue-tsc 就会报错；可静态解析的 39 处调用中错配为 0。
  - openapi-ts 仓库自 2026-03 起只有 renovate 的提交，openapi-fetch 仍是 0.x，且有运行时依赖。

**可观测性与运维**
- 日志 FTS5 全文检索（考虑）：
  - 群号、QQ 号、昵称和消息文本都已在 management_logs.message 列里，对这一列做子串匹配，再叠加现有筛选即可。
  - trigram 少于 3 个字符不匹配任何行，中文常见的两字排障词会直接返回空结果。
- Prometheus 指标端点（考虑）：
  - 仓库在 2026-05 引入过 client_golang 与 /api/system/metrics（10b459b9）。
  - 2026-09-14 在 v0.7 精简计划 R4 中，以“Web 与 Launcher 均未使用”为由删除（d3001d8e）。稳态使用量为零已有实证。

**Server 语言与核心库**
- xi2/xz 换 ulikunitz/xz（替换）：
  - ulikunitz/xz 的 ReaderConfig.DictCap 是下限：流内声明的字典大小会覆盖它，并立即按该值分配内存，无法复现 xi2/xz 的 ErrMemlimit 64 MiB 上限。
  - `archive_security_test.go` 的 dictionary-limit 用例与 `deps/README.md` 记录的不变量都会失效。
- 采纳 json/v2 与 synctest（考虑）：
  - Go 1.27 的 encoding/json 已由 v2 实现支撑，性能收益已经自动获得。
  - 拒绝重复键会改变协议的接受集合，属于契约变更。
  - 42 个 Sleep 测试文件中，有 32 处已在 synctest 气泡内，其余大多驱动真实 I/O 或子进程，仓库也没有 flaky 记录。

**整体系统形态**
- Launcher UI 统一为 Vue（替换，本维度版本）：
  - 具体论据被更正：sdk/vue 主题不可复用；`web/src/components` 依赖 i18n、Tailwind 与 SCSS，不能直接复用；第二份 token 输出只有约 8–30 行；单测与 E2E 工具链已基本统一。
  - 方向本身由 R5 承接。
- Launcher 并入 Server 托盘（考虑）：
  - Windows PE 只能选一个子系统，托盘 GUI 与控制台 CLI 无法合成一个 exe。
  - fyne.io/systray 的 macOS 后端需要 cgo，会破坏 Server 的 CGO_ENABLED=0。
  - Launcher 约 4.2 千行 Go 的主体是进程监督与更新状态机，只能搬迁，删不掉。
- embed web/dist（考虑）：
  - web/dist 实测 26 MB，其中三份 HarmonyOS TTF 约 24.6 MB；嵌入后二进制从 33 MB 增至约 59 MB。
  - 发布包仍需要 templates、`.deps/manifest.json`、build_info.json 等，不会变成单文件。

**契约与代码生成**
- 测试中引入 libopenapi-validator（考虑）：
  - 2026 年以来没有任何 handler 与 OpenAPI 漂移的修复记录，`baseline.md` 设定的触发条件没有满足。
  - libopenapi-validator 仍是 0.x，多文件契约的跨文件 `$ref` 问题（#314）的修复尚未进入已发布的 tag。
  - 在 testutil 中包装 Handler，按 RoutePattern 定位 operation，零依赖即可扩大覆盖。
- Server 侧 oapi-codegen 生成模型（考虑）：
  - oapi-codegen 无法直接处理 `#/$defs/...` 与整文件 `$ref`。
  - Go 的命名字段字面量省略新字段照样能编译，“新增字段就会报错”不成立。
  - 约 20 处响应直接序列化领域类型，生成模型反而要新增一层拷贝代码。

**CI、发布与分发**
- GitHub Artifact Attestations（考虑）：
  - 需要给执行 pnpm install 的构建 job 发 `id-token: write`，违反 `test_release_workflows.py` 固化的“构建 job 无写权限”。
  - 更新器本身不做验证，受益的只有手动运行 `gh attestation verify` 的人。
  - 若将来要防发布资产被替换，GitHub Immutable Releases（2025-10-28 GA）零工作流改动，而且是阻止而非事后检测。
- macOS Developer ID 与公证（考虑）：
  - 公证是付费会员的持续成本：年费、两个长期 secret、外部排队（2026 年初有 24–72 小时的排队报告）。
  - notarytool 不接受 tar.gz；包根之外的 raylea-server 也要签名并公证。
  - GitHub 上 macOS 产物累计下载 0 次。最低成本的动作已并入 C6。
- Windows Authenticode（SignPath）（考虑）：
  - SignPath Foundation 要求“可验证的声誉”，而项目 0 star，GitHub 上只有 v0.3.0/v0.3.1 两个 Release，Windows 包累计约 4 次下载，大概率不会获批。
  - Azure Artifact Signing 的个人身份只限美国和加拿大。
  - Smart App Control 还会拦截未签名的 Chrome for Testing、FFmpeg 和插件 exe，只签两个 exe 并不能让产品在这类设备上可用。
- GHCR 容器镜像（考虑）：
  - 模板已自带 Noto Sans SC，Chromium 启动参数已包含 `--no-sandbox` 与 `--disable-dev-shm-usage`。
  - update_mode 只用于展示，关不掉容器内的 update apply。
  - 镜像无法进入要求归档字段的发布清单，需要改契约。真实的缺口见第 7 节。

**仓库工具**
- 用 mise 管理工具链并去掉 Corepack（替换）：
  - 只有 `read-tool-versions.sh` 能删；Python 与 Node 的读取器仍被 doctor、契约校验和 go.work 渲染使用。
  - CI 里 pnpm 早已由 pnpm/action-setup 安装，换 mise-action 还要另配缓存。
  - 三票一致认为“去掉 Corepack”本身成立（pnpm 11 能按 packageManager 自管版本），但不需要 mise；这一点没有形成独立的替换结论。
  - 可就地修补的部分见第 7 节：删除 doctor 的 check_corepack 与 CI 中的 Install Corepack 步骤，并修正 `sdk/go/pluginbuild/build.go` 的查找路径。
- Python 脚本迁入 Go（考虑，本维度版本）：见 R6 的分歧说明。
- 根 pnpm workspace + catalog（考虑）：
  - 四份 pnpm-workspace.yaml 是 pnpm 11 规定的设置载体，`validate_contracts.py` 有意要求“packages 只含项目根”。
  - link: 本身就是符号链接；sdk/vue 的 exports 指向源码，是 mirrorVueSDK 所必需的。
- go-task 替代 Makefile（考虑）：
  - Makefile 历史上只有 4 次提交，CI 只用过一次 `make doctor`，维护者本人在 Windows 上也不用 make。
  - `quality-gates.md` 的表是按改动面挑命令的指南，不是任务依赖图。
- 用 uv 管理 Python 依赖（考虑）：
  - 缺少依赖清单的诊断成立，而且比原文写的更严重。
  - 但 uv 仍是 0.x、发版密集，也不读 `.tool-versions` 中的 Python 版本；pip 25.1 起支持 PEP 735 的 `--group`，写一份依赖清单即可。

**Launcher 渲染层（其余）**
- 共享主题与动效运行时（替换）：
  - “逐行等价、300–400 行重复”不实。真正等价的只有圆形展开的几何计算、reduced-motion 查询和几个动效常量，约 40–60 行。
  - 两端的持久化键与格式、CSS 变量名、View Transition 的调度语义都不同。
- 共享前端 workspace（考虑）：
  - 仓库已经通过相对导入、server.fs.allow、link: 和 token 生成器跨工程共享，并非“只能复制”。
  - 主要收益依赖 Launcher 改用 Vue。
- Launcher 文案 i18n（考虑）：
  - `docs/dev/text-resources.md` 明确规定文本资源由各客户端各自持有。
  - 与 Web 重叠的文案只有十几到二十条，已经集中在 `AppShell.copy.ts`。
  - Launcher 以契约枚举为键的 Record 映射，类型约束比 `t(key: string)` 更强。
- 从契约生成 Launcher 的服务端类型（考虑）：
  - 同一方案在 2026-09-14 已被 261eade0 有意删除，理由是同包发布不会版本错配，而生成链要同步三份产物。
  - renderer 拿到的是 Go 结构体经 Wails 重新序列化的数据，从契约生成的类型描述的是错误的那一跳。

**插件运行时**
- 多语言 SDK 类型生成（考虑）：
  - 2026-04 至 08 仓库曾有 sdk/python、sdk/nodejs 和托管语言运行时，v0.3（bf6f7bd6 等）主动删除了它们。
  - 本 schema 大量使用 if/then、unevaluatedProperties、not，通用生成器无法表达。
  - 类型只占 SDK 的约 22%，第二语言的真正门槛是运行时与原生打包。

**渲染与媒体（其余）**
- 托管 Chromium 改用 chrome-headless-shell（考虑）：
  - 找到系统 Edge 或 Chrome 时，deps 会跳过托管下载，Windows 几乎不受益。
  - 唯一的官方使用方是 plugin-subscription-hub：其抖音扫码登录在桌面上优先用 visible 模式，并依赖持久 profile 与设备指纹一致。
  - linux-arm64 的支持来自升级版本线，与是否换 headless-shell 无关。
- macOS FFmpeg 换 martin-riedl 构建（替换）：
  - martin-riedl 把 ffmpeg 与 ffprobe 分开打包，zip 也未公证，与 manifest v5“一个资源只有一个归档和一个 sha256”的契约不兼容。
  - BtbN 没有 macOS 构建目标。
  - 但校验确认现有来源的问题比原文写的更严重，见第 7 节。
- 统一浏览器启动所有权（考虑）：
  - chromedp 自身依赖 gobwas/ws，统一后也去不掉。
  - Browser Manager 的 Job 回收在 2026-10-01 才修好，并不比渲染侧更成熟；它的端口预留存在 TOCTOU 竞态，15 秒固定等待还会重新引入 65b92247 修过的问题。

**Web UI 与样式**
- 去掉 SCSS 改用原生 CSS（替换）：
  - `//` 行注释是 Sass 专有语法，真正可以直接当作纯 CSS 的文件只有 34–38 份。
  - Vue scoped 加原生嵌套的 `:deep()` 存在尚未关闭的作用域泄漏问题（vuejs/core#15205）。
  - 仓库只用 `@use`/`@forward`，Dart Sass 2 对它的影响接近零。
- 去掉 vue-i18n（替换）：
  - 键名不受检查，原因是自写的 `t(key: string)` 包装，不在 vue-i18n。vue-i18n 11 自带类型化消息（PickupPaths 等），改包装即可。
  - 体积约 19 KB gzip，不是“100 KB 级”。
  - 约 68 处动态键与十余处 te() 回退，任何方案都绕不开。
- motion-v 换 WAAPI（考虑）：
  - 维护者在 2026-09-15（9c38a1bc）刚为“两端一致”把 Launcher 从 WAAPI 迁到 Motion。
  - 原文所引的 framer-motion ^12.40 依赖有误，实为 ^13。
  - jsdom 不实现 element.animate，mock 并不会减少。

**Launcher 宿主（其余）**
- 纯托盘 + 系统浏览器（考虑）：
  - 统一到 Vue 可以在保留 Wails 的前提下单独完成。
  - RayleaLauncher.exe 共 38 MB，其中约 24.6 MB 是内嵌字体，浏览器方案仍需内嵌这些字体。
  - 现有桌面桥不开 TCP 端口，改成本地 HTTP 服务会新增安全面。

**质量工具（其余）**
- ESLint + Prettier + ruff（替换）：
  - 实测各工程内部风格 100% 一致（web 单引号、无分号；launcher 双引号、有分号），前导 Tab 为 0。
  - 仓库没有 PR 门禁与 git hooks，“由机器拒绝提交”无从实现。
  - eslint-plugin-jsx-a11y 6.10.2 不支持 ESLint 10，而 ESLint 9 已于 2026-08-06 EOL。
  - 两票降为“考虑”，说明有一份机器可执行的格式定义仍有价值，但这不是选型缺陷。
- golangci 配置文件 + 更多 linter（替换）：
  - 现有 5 个 linter 正是 golangci-lint v2 的 standard 预设，govet 已包含 slog、errorsas、httpresponse 分析器。
  - noctx 与 gosec 会大量命中有意为之的 exec.Command 等写法。
  - 三票均降为“考虑”；版本问题见第 7 节。
- Go fuzz 与 Vitest Browser Mode（考虑）：
  - 插件是可信代码，生产环境已取消逐帧校验（123bafc3）。
  - pluginwire 已有 71 例由契约生成的帧，其中 34 例无效，另有边界测试。
  - 唯一的手写解析 parseCQString，补几条表驱动用例即可。
- push 门禁 + lefthook/commitlint（考虑）：
  - 仓库到 2026-10-02 一直有 push 与 PR 触发的 ci.yml（641 行，配 313 行的 detect_changes.py），维护者在 7a88290d 中有意删除；它存续期间修改过 39 次。
  - 实际推送是批量的，有一次推送包含 376 个提交，推送门禁也定位不到单个提交。
- Renovate 看板模式（考虑）：
  - nightly 只有 schedule 和 workflow_dispatch 两种触发，Renovate 开的 PR 不会被验证。
  - Mend 托管版不执行 postUpgradeTasks，改依赖必然触发 THIRD_PARTY_NOTICES 漂移检查失败。

**补充维度**（依据补充维度数据中的票据，这些票据不在输入文件中）
- QQ 官方 Webhook 作为可选传输（考虑）：
  - `docs/user/deployment.md:43` 把支持范围限定为本机与局域网直连、不要求 HTTPS，而 Webhook 要求公网 HTTPS 与端口白名单，受益者落在支持范围之外。
  - NormalizeDispatch 已与传输解耦，将来需要时再补，增量成本不变。
- 新增 Milky 适配器（考虑）：
  - NapCat 已以 not planned 关闭 Milky 支持请求（#1024），OneBot11 适配器无论如何都要保留，稳态是两套负担叠加。
  - 动作层只有 8 个 kind 真正做了投影，其余参数直接透传 OneBot 字段。
  - milkygen 不产出 Go 代码。
- 改用标准库 CrossOriginProtection、去掉 CSRF 令牌（替换）：
  - 现有的 `validRequestOrigin(required=true)` 比 `http.CrossOriginProtection` 更严格：后者不比较 scheme，两个头都缺失时直接放行；补上包装层后，就等于现有函数。
  - CSRF 相关代码合计约 200 行，而且稳定。

## 5. 如果今天从零选型的连贯目标栈

**Server**

| 方面 | 目标选择 | 与现状 |
|---|---|---|
| 语言与形态 | Go 1.27，CGO_ENABLED=0 单二进制兼 CLI | 相同 |
| HTTP 与 WebSocket | net/http + chi v5；coder/websocket | 相同 |
| 日志 | log/slog；管理日志经有界通道异步批量入库，保留期定时清理，spool 兜底 | 写入模型变化（R2） |
| 数据库 | SQLite（modernc，纯 Go）+ database/sql + sqlc + 手写有序迁移 | 相同 |
| 连接配置 | PRAGMA 经 DSN 落到每条连接；写句柄 `_txlock=immediate` | 变化（R3） |
| 日志表与持久化级别 | ts 存 INTEGER 纳秒；WAL 下 synchronous=NORMAL | 可选（C2） |
| 快照 | VACUUM INTO 在独立连接上执行 | 可选（C3） |
| YAML | go.yaml.in/yaml/v3，server 与 launcher 同步 | 换维护线（R1） |
| JSON Schema | santhosh-tekuri/jsonschema/v6 | 相同 |
| 口令与会话 | argon2id 口令；会话为随机不透明令牌，库内只存 SHA-256；CSRF 派生令牌 + 精确 Origin | 会话存储变化（R4），CSRF 相同 |
| xz 解压 | xi2/xz，保留 64 MiB 字典上限 | 相同 |
| 浏览器 | chromedp 负责渲染，Browser Manager 以裸 exec 管理插件会话；Chrome for Testing 完整构建，系统浏览器优先 | 相同 |
| 媒体 | BtbN FFmpeg gpl-shared 变体，不阻塞首启 | 变化（R7） |
| 聊天适配器 | 自写 OneBot11 四种传输；自写 QQ 官方 WebSocket 网关 | 相同 |
| 观测 | 不用 OTel 与 Prometheus；诊断包加入运行时 profile | 相同，诊断包为可选增强（C4） |
| 调度、任务、事件 | 自写 cron、Task Registry、pubsub.Hub | 相同 |
| 管理面产物 | 从磁盘托管 web/dist | 相同 |

**Web**

| 方面 | 目标选择 | 与现状 |
|---|---|---|
| 框架与构建 | Vue 3.5 + TypeScript 5.9 + Vite 8 + Vue Router 5 + Pinia 4 | 相同 |
| 组件与样式 | Reka UI + 仓库持有的 shadcn-vue + Tailwind 4（限 ui/ 层）+ SCSS + CSS 变量 | 相同 |
| 动效、图标、虚拟列表 | motion-v + View Transition；@lucide/vue；@tanstack/vue-virtual | 相同 |
| 文案 | vue-i18n 11；可就地把 `t(key: string)` 改为按键路径约束的类型 | 相同 |
| API 与实时层 | openapi-typescript + 自写 http.ts；自写 ws.ts | 相同 |
| WS 类型 | 由 OpenAPI components 生成 | 可选（C1） |
| 测试 | vitest 4 + jsdom + @vue/test-utils；Playwright 真实后端 E2E | 相同 |

**Launcher**

| 方面 | 目标选择 | 与现状 |
|---|---|---|
| 宿主 | Wails v3，GA 后一次性跟进；独立 Go module，GOWORK=off | 相同 |
| 渲染层 | Vue 3 + Reka UI + motion-v + @vue/test-utils | 变化（R5） |
| 桥接 | Wails bindings + wailsDesktopApi 适配层 + DesktopHost 接口 | 相同 |
| 打包 | 自写 build-package.mjs；Linux 用 GTK3 | 相同 |
| Windows 首启 | 启动前检测 WebView2 并弹原生提示 | 可选（C5） |
| macOS 首启 | 放行说明 + 完整 ad hoc 签名（效果待实机验证） | 可选（C6） |

**插件**

| 方面 | 目标选择 | 与现状 |
|---|---|---|
| 运行时 | 原生可执行子进程 + JSONL v4 over stdio；无 OS 沙盒 | 相同 |
| SDK | Go SDK 为唯一官方运行时 SDK；sdk/vue 用于管理页 | 相同 |
| 管理页 | 同源 iframe + 逐请求 CSP | 相同 |
| 分发 | artifact v2 单根 ZIP；catalog 静态 JSON 单源；无签名 | 相同 |

**契约与生成**

| 方面 | 目标选择 | 与现状 |
|---|---|---|
| 契约载体 | 手写 OpenAPI 3.1 + JSON Schema 2020-12 + 自定义 YAML | 相同 |
| WS payload | 并入 OpenAPI components | 可选（C1） |
| 生成器与校验器 | error-codes、plugin-wire 生成器与契约校验器改用 Go 实现；openapi-typescript 与 runtime-schemas 留在 Node | 宿主语言变化（R6，存在分歧） |
| Server 侧 | 不做 OpenAPI 代码生成，也不引入运行时 OpenAPI 校验 | 相同 |

**仓库工具**

| 方面 | 目标选择 | 与现状 |
|---|---|---|
| 脚本语言 | Go 与 Node 两种，不再有 Python | 变化（R6，存在分歧） |
| 工具链锁定 | `.tool-versions` 精确锁定（去掉 Python 后为 6 项） | 机制相同，项数依附 R6（存在分歧） |
| JS 包管理 | pnpm，每个工程独立工作区 | 相同 |
| 入口 | Makefile + start.bat/start.sh | 相同 |
| 开发编排 | 自写 start-dev 与 dev-build-cache | 相同 |
| Corepack | 未定：三票认为去掉 Corepack 成立（直接安装 pnpm 11），但没有形成独立建议 | 未定 |

**质量**

| 方面 | 目标选择 | 与现状 |
|---|---|---|
| Go 测试 | 标准 testing + rapid；架构门禁只保留 Go AST 测试 | 门禁收敛为可选（C7） |
| 静态检查 | golangci-lint standard 预设（版本需升到 ≥ v2.13.0）+ govulncheck | 选型相同 |
| 前端检查 | vue-tsc/tsc strict；不引入 ESLint/Prettier | 相同 |
| 门禁 | nightly 全量回归 + tag 发布门禁；不设 PR/push 门禁 | 相同 |
| 依赖策略 | 精确锁定 + 冻结版本线 + pnpm audit；不用 Renovate | 相同 |

**发布**

| 方面 | 目标选择 | 与现状 |
|---|---|---|
| 产物 | GitHub Release 的 zip/tar.gz + release_manifest v2；不签名、不发 SHA-256 | 相同 |
| 更新 | 自写更新器 + Launcher `--wait-for-pid` 接管 | 相同 |
| 打包脚本 | 随 R6 改为 Go | 变化（依附 R6，存在分歧） |
| 渠道 | 无 Docker、无安装器、无包管理器渠道；无代码签名、公证与 attestation | 相同 |

## 6. 保持现状且理由充分

**Server 与核心库**
- Go 语言：负载是 I/O 与进程编排，CGO_ENABLED=0 能交叉编译单文件；Rust、.NET AOT、Bun、Elixir、Kotlin native 都会增加分发或维护负担。
- chi：chi.Walk 支撑路由与 OpenAPI 的对账测试，标准库 ServeMux 没有等价能力。
- coder/websocket：仍在活跃维护，API 基于 context；gorilla 组织自 2025-04 起无人维护。
- log/slog：瓶颈在 fsync，不在日志库；不需要 zap 或 zerolog。
- jsonschema/v6、argon2id、rapid：都是当前版本，用途正确。
- 自写 pubsub.Hub、cron、Task Registry：规模小、有测试，没有更轻的现成替代。

**数据与存储**
- SQLite 单库：DuckDB、嵌入式 Postgres、bbolt 都不满足事务、约束与运维能力的要求。
- modernc 驱动：mattn 需要三平台 C 工具链；ncruces 刚切换代码生成路径；zombiezen 不支持 database/sql。
- sqlc：手写 SQL 只剩 7 处，每处都有清楚的原因。
- 手写迁移：产品有意不做回滚，迁移框架的核心价值用不上。
- 插件 KV 放在同一个库：配额与 TTL 能在一个事务内完成。
- 单写连接、文件锁、quick_check 隔离损坏库，以及 VACUUM INTO 备份：都是单进程 SQLite 应用的最佳实践；不用 litestream。

**配置与密钥**
- YAML + JSON Schema：schema 已经驱动默认值、未知字段过滤与热更新元数据；CUE、Pkl、TOML 会多出一层。
- secret 明文存 SQLite：插件与宿主同属一个 OS 用户，keychain 或 age 在同机场景下没有保护作用，还会破坏整包恢复。
- 不用环境变量覆盖配置，不引入 koanf：避免出现第二个配置来源。

**观测与运维**
- 不用 OTel：单进程，没有 collector。
- 消息统计自写：它是业务状态，必须由 Server 持有。
- healthz/readyz 语义、FailureTracker、spool 与 quarantine 兜底：设计清楚，已写入契约或文档。
- 按 UTC 日期命名的文本日志：是 `docs/dev/logging.md` 明确的设计。唯一缺口是 Launcher 日志没有保留清理。
- 不接崩溃上报：符合自托管用户的隐私预期。

**插件**
- 原生子进程 + JSONL：go-plugin、WASM、goja、Deno sidecar 都与“可信本地代码 + 语言不限 + CGO_ENABLED=0”冲突。
- 继续用 stdio 而非 socket：读循环可以把非协议的 stdout 行降级为 console 输出，低成本消除插件打印导致的崩溃判定。
- 不做沙盒：如需限制资源，可在已持有的 Job Object 上加内存上限。
- 不嵌入消息总线；Go SDK 为唯一官方 SDK；iframe + CSP 承载管理页；ZIP + GitHub Release 分发；用临时 go.work 做本地联调。

**契约与生成**
- 手写契约，不用 TypeSpec、Protobuf、Smithy、CUE：重度依赖 x- 扩展，契约文件本身就要可读。
- 继续用 OpenAPI 3.1：主流工具已全面支持。
- 保留 error-codes、cli-commands、plugin-management-ui 三份自定义格式，以及自写的 plugin-wire 生成器：通用工具无法表达 RawMessage 与字段存在性指针。
- Web 侧不做运行时校验：Server 是正式状态来源。

**Web**
- Vue：Web、sdk/vue 与插件页三处共用；SPA 优于 templ + HTMX/Datastar。
- Vite 8、TS 5.9：等 vue-tsc 支持 TS 7.1 后一步到位。
- Pinia、Vue Router 5、自写 ws.ts：已稳定。
- Reka 加自有 shadcn-vue；Tailwind 只用在 ui/ 层；自写 token 生成器：生成器承担了对比度与字体校验等产品专属工作。
- lucide、tanstack-virtual、vueuse、@internationalized/date、View Transition：用途明确，依赖面小。

**Launcher**
- 保留独立的 Launcher 组件：预检、进程监督和更新都发生在 Server 未运行的时候。
- 宿主用 Wails v3：Tauri 的 updater 强制签名，Electron 会把运行时塞进归档，Wails v2 没有托盘，Fyne 的无障碍支持不足。
- 独立 module、自写打包脚本、GTK3 兼容路径、DesktopHost 接口隔离、不以 Web 路由替代 Launcher 界面：均保持。

**渲染与媒体**
- chromedp：go-rod 已停更，playwright-go 需要 Node driver。
- html/template + Chromium 截图留在核心：takumi、Lightpanda、resvg 都不达标。
- Chrome for Testing + npmmirror 多源测速 + 系统浏览器优先；远程资源预取后用 `file://` 绑定；FFmpeg 以子进程使用，不引入 libav 绑定。

**CI 与发布**
- nightly + tag 发布，不设 PR 门禁；三个原生 runner。
- 核心包不发 SHA-256：清单与归档同源，摘要不增加防篡改能力。
- 不做包管理器渠道和安装器：会与就地自更新形成双重所有权。
- 依赖系统 WebView；自写更新器；不用 GoReleaser 与 release-please；Actions 已在各自当前大版本。

**仓库工具**
- Node 而非 Bun；自写 dev-build-cache；start.bat/start.sh；`.tool-versions` 的格式本身。
- 脚本测试用 node --test 与 unittest；保留 devcontainer。
- `/api/development/*` 作为开发同步的唯一入口；自写 start-dev 编排。

**质量**
- 用标准 testing，不引入 testify。
- `route_security_test.go` 与 `error_codes_test.go` 的契约交叉校验。
- race 测试与 Windows 全量测试；Playwright 分层；vitest + jsdom；govulncheck + pnpm audit。
- 不引入 CodeQL、SonarQube、gremlins。

**QQ 官方适配器**
- 自写 WebSocket 网关：官方 2026-08 恢复了 WS 与 Webhook 可切换。
- TokenSource 提前续期；不用 botgo（最后提交在 2024-12-18）。
- 暂不做 Webhook；sandbox 开关与 intents 闭合枚举；共用 coder/websocket。

**OneBot11 协议层**
- OneBot11 作为唯一个人号协议；自写四种传输（ZeroBot 是 GPL 的框架式项目，会带来平行依赖栈）。
- CQ 码与 JSON 数组双格式解析；中立 kind + provider 扩展；不接 Satori 与 OneBot v12。

**管理面认证**
- cookie 设为 HttpOnly、SameSite=Strict、host-only；setup token 通过 URL fragment 一次性传递。
- 离线 reset-admin；Launcher 的 control token；WebSocket 只接受 cookie 或 Authorization 加精确 Origin。
- Bearer 与 cookie 双传输；插件 iframe 复用管理面会话；按 IP 的内存登录限流。
- 不做 Launcher 登录票据、Passkeys、OIDC：默认的 HTTP/IP 访问下 WebAuthn 不可用；OIDC 依赖外部 IdP。
- 保留 CSRF 令牌。

## 7. 校验中顺带发现的现存缺陷（与选型无关）

处理状态（2026-10-06）：下表缺陷已修复；选型替换建议仍按各自条目单独评估。

| 问题 | 修复结果 | 主要位置 |
|---|---|---|
| golangci-lint 与 Go 1.27 不兼容 | 两个 lint 步骤固定为 v2.13.0，同步工程基线 | `.github/workflows/nightly.yml`；`docs/engineering/baseline.md` |
| macOS FFmpeg 构建启用 nonfree | 改为 Martin Riedl 固定的 9.0.2 GPL 发布构建，保留 libx264/libx265；FFmpeg 与 FFprobe 分包下载、独立校验并一起启用 | `.deps/manifest.json`；`contracts/deps-manifest.schema.json`；`server/internal/platform/deps/` |
| WS 日志协议枚举遗漏 qqofficial | 补齐协议枚举和有效样例，与 HTTP 日志摘要一致 | `contracts/websocket-events.yaml`；`fixtures/websocket/ok.logs-appended.protocol-qqofficial.json` |
| Python 依赖与 format 校验缺失 | 统一使用 requirements；CI 与 devcontainer 安装 format 扩展，校验器在缺少格式检查能力时拒绝运行，生成器给出安装指引；修复严格校验暴露的时间戳样例 | `scripts/requirements.txt`；`scripts/ci/validate_contracts.py`；`.devcontainer/Dockerfile` |
| Windows 插件构建器只能查找 Node 同目录 Corepack | 同时查找 PATH、用户 npm 目录及独立 pnpm CLI；删除 doctor 的冗余 Corepack 检查和 Server job 安装步骤，保留实际调用 Corepack 的工作流步骤 | `sdk/go/pluginbuild/build.go`；`scripts/check-toolchain.py`；`.github/workflows/nightly.yml` |
| 官方插件依赖不存在的 SDK tag，fresh clone 缺少 Vue SDK | 6 个已有发布工作流的独立插件仓库固定到远端可获取的 SDK 提交与对应 Go 伪版本；fortune 与 subscription-hub 的 UI 安装前自动准备 Vue SDK | 各插件仓库的 `.rayleabot-sdk-ref`、`go.mod`、`release.yml` 和 UI 安装脚本 |
| 示例 UI 未参与工具链检查 | 使用实际的 `examples/plugins/*/ui/package.json` 路径，并增加版本漂移回归测试 | `scripts/check-toolchain.py`；`scripts/tests/test_check_toolchain.py` |
| Linux server 包没有渲染共享库说明与诊断 | 两种 Linux 包均分发运行库说明；doctor 检查当前架构的 Chromium 共享库，并输出修复指引 | `scripts/release/release_tool.py`；`server/internal/operations/diagnostics/`；`docs/release/linux-desktop-runtime.md` |
| Launcher 镜像日志无限累积 | `logs/server/` 与 `logs/launcher/` 保留最近 7 个 UTC 日期，新日期首次写入清理过期日志 | `launcher/internal/desktop/process.go`；`docs/dev/logging.md` |

本节末尾引用的缺陷也按原有选型修复：R3 的连接 PRAGMA 覆盖全部池连接和替换连接；R4 的签名密钥更新与会话撤销在同一事务完成，reset-admin 清除旧签名密钥；R7 的 FFmpeg 改为管理面按需准备，不阻塞首次启动；C5 在创建 Wails 窗口前检查 WebView2，并在缺失时显示原生安装提示；C6 在部署文档补充 macOS 解压目录的 quarantine 放行步骤。依照根目录 AGENTS 的发布约束，本次不引入签名；macOS Gatekeeper 与媒体工具的原生运行仍需实机验收。

验证结果：Server、Launcher、Go SDK、全部 Go 示例及 6 个官方插件仓库的全量 race 测试通过，浏览器用例使用系统 Edge。Launcher Go vet、Windows/Linux 目标 lint、Web 类型检查、strict contracts、生成物与 sqlc 漂移、仓库及发布脚本测试通过。两个插件 UI 均从不含 SDK 镜像的新目录完成安装、类型检查、测试与构建。此前记录的 subscription-hub 微博临时目录清理失败在本轮 race 中未复现；macOS 原生运行与 Gatekeeper 尚未实机验证。

## 8. 证据与方法

**流程**
- 维度：先按 16 个主维度做只读分析，每个维度阅读相关代码与文档，产出 current_stack、usage_evidence、keep_as_is 与若干建议；之后由完整性批评补充 3 个维度（QQ 官方适配器、OneBot11 协议层、管理面认证）。
- 建议：共 111 条，其中 62 条原判为替换或考虑。
- 校验：每条非保持建议由三个视角各投一票，分别是产品约束契合度、成熟度与事实、稳态价值，共 186 票。缓存票 91 张，其 reasoning 存于输入文件。联网票 95 张，其中主维度的 83 张随合并表内联提供 reasoning 与更正事实；补充维度的 12 张联网票未包含在输入文件与合并表中，只随补充维度数据单独提供，相关结论以补充维度的 final_verdict、这些票据和仓库核查为据。
- 合并：从合并结果反推，规则等价于“取三票调整后结论的多数，三票互不相同时取最保守，再在两票及以上判定反驳时降一档”。本报告的结论以合并表的 final_verdict 为准。

**经联网核实的主要外部事实（均来自校验票或分析记录）**

| 事实 | 核实结论 |
|---|---|
| gopkg.in/yaml.v3 | 上游 2025-04-01 归档；go.yaml.in/yaml/v3 v3.0.5 发布于 2026-07-26；v4 仍为 rc.6（2026-06-17） |
| modernc.org/sqlite | v1.56.0 内含 SQLite 3.53.3；v1.55.0 起提供 DSN 简写键；最新为 v1.60.1（2026-09-29） |
| Go 1.27 | 2026-08 发布；encoding/json 由 v2 实现支撑；goroutineleak profile 已 GA |
| golangci-lint | v2.13.0（2026-08-19）起支持 go1.27；v2.14.0 发布于 2026-09-24 |
| Wails v3 | 最新为 v3.0.0-beta.27（2026-10-01），全部标签仍为预发布；v3 运行时没有 WebView2 缺失处理策略 |
| chromedp / go-rod | chromedp 在 2026-10-03 前后连续发布 v0.17 以上版本，含 API 的破坏性重写；go-rod 上游自 2024-07 起停止发布 |
| BtbN FFmpeg | autobuild-2026-08-31 各资产大小已核实（win64-gpl 161.4 MiB，win64-gpl-shared 72.9 MiB） |
| Chrome for Testing | 152 版各归档大小已核实；npmmirror 镜像的 headless-shell 返回 200；linux-arm64 自 153.0.8001.0 起提供 |
| ulikunitz/xz | v0.5.17 源码中 DictCap 会被流内声明值覆盖，不是上限 |
| Vitest | 5.0.0 发布于 2026-09-03，localStorage 修复来自 PR #10293，未回移到 4.x |
| Pinia Colada | 1.4.7 发布于 2026-10-02 |
| openapi-fetch | 最新 0.17.0（2026-02-11）；所在仓库自 2026-03 起只有 renovate 的提交 |
| vue-i18n | 稳定线为 11.4.13；v12 仍为 alpha.4 |
| Vue 3.6 | 仍为 rc |
| Dart Sass 2.0 | 计划 2026-12 发布 |
| CSS 嵌套 | 2026-06 成为 Baseline Widely Available |
| ESLint | ESLint 10（2026-02）只支持 flat config；ESLint 9 于 2026-08-06 EOL；eslint-plugin-jsx-a11y 6.10.2 不支持 ESLint 10 |
| mise | v2026.10.0 发布于 2026-10-02 |
| Corepack | Node 25 起不再随发行版附带 |
| uv | 最新 0.12.23（2026-10-03），仍为 0.x；Astral 于 2026-03-19 宣布被 OpenAI 收购 |
| go-task | v3.54.0 发布于 2026-10-01 |
| SignPath Foundation | 条款要求“可验证的声誉” |
| Azure Artifact Signing | 个人身份只限美国和加拿大 |
| GitHub Immutable Releases | 2025-10-28 GA |
| macOS Gatekeeper | Sequoia 15.0 移除右键绕过；notarytool 只接受 zip/dmg/pkg |
| prometheus client_golang | v1.24.1 声明支持的 Go 版本为 1.25 与 1.26 |
| OTel Go | Logs 于 v1.47.0（2026-10-02）进入 Stable |
| QQ 官方机器人 | 2026-08 恢复 WS 与 Webhook 可切换；Webhook 要求 HTTPS 与端口白名单 |
| botgo | 最后一次提交在 2024-12-18 |
| Milky | 规范为 v1.3.0；NapCat #1024 以 not planned 关闭 |
| Go 标准库 | `http.CrossOriginProtection` 自 Go 1.25 提供，两个头都缺失时放行 |
| WebAuthn | 要求安全上下文，RP ID 必须是域名 |

**仍存在的不确定性**
- 缓存票的 reasoning 在输入文件中被截断，部分论证只能读到前半段；结论与更正事实字段完整。
- 票间存在冲突，均基于源码推断，未实机运行：
  - yaml v3.0.1 到 v3.0.5 是否有非测试行为差异；
  - macOS 完整 ad hoc 签名能否恢复“仍要打开”路径；
  - Windows 缺 WebView2 时的确切失败路径：静默退出，还是空白窗口后 panic。
- 快照耗时是用 Python 自带的 SQLite 3.50.4 测得的，不是 modernc。
- 以下都是 2026-10-03 前后的时点数据：
  - star 数、各 Release 的下载量；
  - nightly 的连续失败次数；
  - GitHub 上只有 v0.3.0/v0.3.1 两个 Release。
- 无法核实的外部来源：Fern 的 AsyncAPI 指南（404）、知乎文章（403）、v3.wails.io/status（403）、“trigram 索引约为原文 3 倍体积”。
- 少数外部版本号在票间略有出入，例如 Wails 最新是 beta.26 还是 beta.27、chromedp 新版本的具体日期；本报告取联网核实票的较新值。
- 跨维度存在两处结论不一致，均已按合并表如实列出：Launcher 改 Vue（替换、考虑、保持各一）；脚本收敛到 Go（替换与保持）。
- 合并规则在部分条目上比票面多数更保守，例如商店多镜像、mise、golangci 配置三条的三票都是“考虑”，合并后为“保持”。这些条目留下的小问题已列入第 7 节，或写在第 4 节的说明中。
- 本报告没有重新运行仓库的构建或测试，仓库事实来自分析代理与校验代理的只读核查。