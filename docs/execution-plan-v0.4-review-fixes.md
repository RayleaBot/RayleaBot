# RayleaBot v0.4 评审修复计划

## 文档状态

- 目标版本：v0.4
- 执行范围：仅 RayleaBot 主仓库
- 当前状态：F1 至 F8 已完成，等待项目维护者验收
- 合并条件：本计划 F1 至 F8 全部完成并回写验证证据
- 验收方式：本文档长期保留，由项目维护者逐项验收

状态说明：

- `⬜ 待处理`：尚未开始。
- `🟡 进行中`：已开始，但尚未满足完成条件。
- `☑️ 已完成`：实现、配套更新和本项验证均已完成。
- `❌ 阻塞`：当前范围内无法消除，必须记录原因和所需决策。

## 修复边界

### 允许

- 修复评审确认的 7 个 major。
- 同轮处理与 v0.4 合同纪元、P6 清理承诺或回归门禁直接相关的 minor。
- 修改 contracts、fixtures、Server、Go SDK、Web、开发脚本、示例、文档和 CI 检查。
- 删除生成到示例源码目录的二进制和已经没有生产调用者的兼容代码。

### 不在范围内

- 修改外部插件仓库或官方 `plugin-catalog` 仓库。
- 增加兼容旧 manifest、protocol、artifact、bridge 或 backup 合同的运行分支。
- 引入新的前端格式化依赖、semver 依赖、状态管理层或并行配置服务。
- 借修复评审问题改动无关产品视觉或扩展插件能力。

## 固定修复决策

1. v0.4 合同版本和字段形状保持不变；实现向现有 contract 收敛。
2. 插件有效配置始终是 `default_config` 与持久值合并后的完整快照，持久值优先。
3. `config.changed` 必须携带 `config`，空对象表示完整配置为空；缺失或类型错误属于协议违规。
4. Go SDK 在读循环中按输入顺序应用 `config.changed` 与 `bot.identity.changed`，再启动并发业务 handler。
5. Server 内只保留 `internal/semver` 一套比较实现；商店展示、安装门禁和恢复判断共用它。
6. Dispatcher 缺少权限检查器时拒绝高权限出站动作，不允许 fail-open。
7. 菜单测试按职责拆分：组件测试覆盖 HTML/字体安全处理，页面测试覆盖配置保存和根/插件预览数据。
8. 非 Go 开发插件继续使用 `dist/native/<platform>/<plugin-id>[.exe]` 约定；watch 对该目录例外放行并补正式文档。
9. `raylea-plugin` 的相对 `--binary`、`--out` 和 `--include` 源路径以 `--plugin` 指定的项目根目录解析；绝对路径保持不变。
10. 本轮恢复被误删的 Ant Design 主题 token，不保留未经需求确认的视觉变化。

## 问题映射

| 评审项 | 修复工作项 | 状态 | 验收重点 |
| --- | --- | --- | --- |
| Major 1：baseline 命令错误 | F1 | ☑️ 已完成 | 文档命令与 CLI flags 一致并由 CLI 集成测试覆盖 |
| Major 2：E2E inspect mock 旧合同 | F5 | ☑️ 已完成 | `permissions` 与 artifact v2 正确进入安装确认页 |
| Major 3：配置默认值与持久值语义分叉 | F2 | ☑️ 已完成 | 升级新增默认键在 init、管理页、config.write 广播中一致 |
| Major 4：SDK 控制事件应用乱序 | F3 | ☑️ 已完成 | 后续业务事件必定读取已经按序应用的最新状态 |
| Major 5：空配置快照无法传播 | F3 | ☑️ 已完成 | `{}` 从 Server 到 SDK 原样替换旧快照 |
| Major 6：semver 双实现 | F4 | ☑️ 已完成 | 商店与安装门禁对所有边界版本结论一致 |
| Major 7：菜单安全与保存流测试净损失 | F5 | ☑️ 已完成 | HTML 转义、字体改写、sandbox、根预览和保存流有定向回归 |
| Contract/fixture minor | F1 | ☑️ 已完成 | 旧术语、负向 epoch fixture、核心错误码和代表性 action golden 已补齐 |
| Server 卫生与 fail-open minor | F4、F7 | ☑️ 已完成 | 默认核心版本、权限缺失、webhook/command 死代码和 nil 防御收敛 |
| SDK、示例与开发流 minor | F6 | ☑️ 已完成 | 示例命名真实、无假命令/二进制，非 Go watch 与路径语义可复现 |
| Web 与文档 minor | F5、F7 | ☑️ 已完成 | v3 fixture、两空格缩进、语言无关文案、主题 token 和最终态文档一致 |

## 执行清单

| ID | 工作项 | 状态 | 完成情况 | 验证证据 |
| --- | --- | --- | --- | --- |
| F0 | 固化评审修复计划 | ☑️ 已完成 | 已复核评审问题，记录范围、固定决策、依赖顺序、完成条件和验证矩阵。 | 132 个 Markdown 文件链接检查和计划文档 `git diff --check` 通过。 |
| F1 | 合同、fixtures 与命令文档校正 | ☑️ 已完成 | baseline 已改为真实 `raylea-plugin` flags；permissions 文案、错误 fixture、backup v2/bridge v2 负向 fixture、四类代表性 action golden 和空配置 golden 已补齐，嵌入 schema 已再生成。 | 合同 self-test/strict、runtime schema verify 和定向 `git diff --check` 通过。 |
| F2 | Server 完整配置快照统一 | ☑️ 已完成 | `pluginstore.MergeValues` 成为默认值与持久值合并入口；lifecycle init/manifest refresh、管理页、command hydration 和 `config.write` 广播均使用有效快照。`SeedDefaults` 改为逐键补充，保留已有用户值。 | `go test ./internal/plugins/pluginstore ./internal/plugins/lifecycle ./internal/plugins/actions ./internal/management ./internal/app` 通过。 |
| F3 | Protocol v2 控制状态顺序与空快照 | ☑️ 已完成 | Server 通过可选指针保留空 `config` 对象；Go SDK 在 stdin 读循环中先解码并按序应用配置和 Bot 身份，再并发派发 handler。缺失或类型错误的配置快照返回协议违规，空对象清除旧配置，控制事件仍交给 handler。 | Server runtime 与 Go SDK 全包测试通过；SDK 顺序、空快照、非法快照回归通过。当前 Windows `CGO_ENABLED=0`，`go test -race ./...` 无法运行，保留由 Linux CI/nightly 覆盖。 |
| F4 | semver 单一实现与核心门禁 | ☑️ 已完成 | pluginmarket 已删除私有 semver 与 `0.3.0` fallback，显式要求应用注入核心版本；商店、安装和恢复共享 `internal/semver.Compare`。比较器覆盖预发布、超长数字、前导零、build metadata、`v` 前缀和空输入。Dispatcher 缺少权限检查器时改为拒绝出站动作。 | `go test ./internal/semver ./internal/pluginmarket ./internal/plugins/lifecycle ./internal/recovery ./internal/eventpipeline/dispatch` 通过。 |
| F5 | Web 合同 mock 与菜单回归恢复 | ☑️ 已完成 | 安装检查 mock 与断言切换到 `permissions` 和 artifact v2；app-shell fixture 收敛到 bridge v3 单入口。新增原生模板预览安全/字体/边界测试，并恢复菜单根预览、保存流及 exact/setting/pattern 投影回归。恢复四个主题 token；清理 Web 前导 tab，并加入无依赖检查脚本和 CI 门禁。 | Web typecheck 与 62 个单测文件（313 项）通过；安装管理 E2E 定向用例通过；`pnpm run check:indent` 通过。 |
| F6 | 开发工具、非 Go watch 与示例卫生 | ☑️ 已完成 | 示例源码目录中的顶层 `.exe` 已删除并由全局 ignore 规则阻止误提交；示例已改名为 `example-http-storage`，webhook 假命令已移除。非 Go watcher 仅放行 `dist/native/<platform>/`，其他生成目录继续忽略。`raylea-plugin` 的相对 binary/out/include 源路径统一按插件根解析，文档记录正式约定。 | Go SDK/CLI/pluginbuild 全量测试、10 个示例模块构建、示例 manifest 集成测试和 11 项开发工作区测试通过；源码示例目录无顶层 `.exe`。 |
| F7 | Server/Web 死代码、fail-closed 与文档收尾 | ☑️ 已完成 | webhook Registry 只保留 `SyncSnapshots` 生产写入路径，动态 Register/Delete 与命令死 helper 已删除；webhook Service 构造改为校验必需依赖，移除 nil 静默返回。Web 删除四个确认无消费者的字段 locale 与 `message.reply` 死标签；`runtimeConfig` 因仍由详情页消费而保留。发布文档已改名并同步链接，插件正式文档已改为最终态措辞。 | webhook/catalog/lifecycle/app/management/services 定向测试通过；死代码、旧文档链接和死 locale 定向扫描通过。 |
| F8 | 全量验证与最终 review | ☑️ 已完成 | 已逐项复核 F1 至 F7 的合同、实现、测试、示例、文档和最终 Git inventory；7 个 major 全部关闭，未保留兼容分支或预设延后项。 | 合同、Server、Go SDK、开发脚本、Web 单测/E2E/build、示例、文档和仓库卫生验证完成；Web E2E 最终全量 53/53 通过。 |

## 实施顺序与完成条件

### F1 合同、fixtures 与命令文档校正

按 contract-first 顺序先固定已有语义，不新增版本或兼容分支：

- 修正 `docs/engineering/baseline.md` 中 `raylea-plugin pack/build-go` 示例，使用 `--plugin`、`--binary`、`--target` 和 `--out`。
- 将 `plugin.permission_denied` 文案统一为“插件访问未声明权限”，同步错误 fixture 和说明。
- 将 `fixtures/README.md` 的“能力参数”改为 permissions、静态声明和合同版本边界。
- 新增 backup manifest v2、management bridge v2 的精确拒绝 fixture，并加入对应 schema 的 `x-fixtures`。
- 将 `plugin.core_version_incompatible`、`plugin.contract_unsupported` 纳入错误码 fixture。
- 为 protocol v2 保留一组代表性正向 golden：governance、scheduler、render、thirdparty；新增空 `config.changed` fixture。动作细节继续由包内行为测试覆盖，不恢复已经删除的 v1 action。
- 运行合同 self-test、strict 校验和 runtime schema verify。

### F2 Server 完整配置快照统一

Server 只维护一套有效配置合并规则：

- 在 `plugins/pluginstore` 提供深拷贝的有效配置合并函数：先复制 manifest defaults，再覆盖持久值。
- lifecycle init、管理页 GET/PUT、`config.write` 后广播、启动时 command hydration 和 manifest refresh 全部调用同一函数。
- `SeedDefaults` 改为逐键补充缺失默认值，不再因 namespace 已有任意记录而整体跳过；已有用户值不得覆盖。
- 保留现有插件设置、密钥、KV 和文件，不新增数据迁移或清空逻辑。
- 增加以下回归：
  - 已修改过配置的插件升级后获得新增默认键；
  - 用户覆盖值优先于新默认值；
  - init、管理页响应、命令刷新和 config.write 广播得到相同快照；
  - 默认值和持久值的嵌套对象、数组不会共享可变引用。

### F3 Protocol v2 控制状态顺序与空快照

先修 Server wire，再修 Go SDK 消费顺序：

- Server 为 `config.changed` 使用允许空 map 的专用读取路径，避免改变其他 payload 字段的非空语义。
- `ProtocolPayloadFrame.Config` 始终序列化；`config.changed` 的 `{}` 不得被 `omitempty` 删除。
- Go SDK 在 stdin 读循环内完成事件解码，并同步应用配置与 Bot 身份控制状态；完成后再把事件交给并发 handler。
- SDK 收到缺失或类型错误的 `config.changed.config` 时返回 `plugin.protocol_violation`，不能保留旧快照继续运行。
- 增加确定性并发测试：
  - config A、config B、message 连续输入后 message 只看到 B；
  - bot identity change 后的 message 读取新身份；
  - 空 config 清除旧键；
  - control handler 仍接收对应事件，且 handler 并发上限保持不变。
- 在支持 race 的环境运行 SDK 相关 race 测试；当前 Windows 若 `CGO_ENABLED=0`，记录由 Linux CI/nightly 覆盖。

### F4 semver 单一实现与核心门禁

- 将 pluginmarket 的全部版本比较切换到 `server/internal/semver.Compare`，删除私有解析和比较函数。
- 为共享包增加表驱动测试，覆盖正式版/预发布、数字与字母标识、超长数字、前导零、不同标识长度、build metadata、`v` 前缀和空白输入。
- pluginmarket 不维护独立 `defaultCoreVersion`；应用装配必须显式传入 `recovery.DetectCoreVersion` 的结果，缺失时构造失败。
- 增加跨调用方测试，证明商店 `Compatible`、最新可安装版本和 Installer `min_core_version` 门禁结论一致。
- Dispatcher 缺少 permission checker 时拒绝需要权限的出站动作，并补 fail-closed 回归；正式装配继续在插件服务建立后注入 checker。

### F5 Web 合同 mock 与菜单回归恢复

- E2E mock 的安装检查响应改为 `permissions` 对象和 `artifact_version: "2"`，并在 E2E 中断言权限列表与 artifact 版本进入确认界面。
- `app-shell.spec.ts` 的管理页 fixture 改为 bridge v3 单入口结构，删除页面级 `entry` 和类型断言掩盖。
- 新增 `NativeTemplatePreviewFrame` 定向组件测试，覆盖：
  - 字体 `@import` 清理和受控资源 URL 改写；
  - `.ttf`、相对字体 URL 和 script 注入不会进入 srcdoc；
  - HTML 文本与属性转义；
  - iframe sandbox 与预览宽高约束。
- 恢复 Menu Center 的最小高价值页面回归：配置保存、继承前缀、根预览、插件预览、command group 与 exact/pattern/setting trigger 展示。
- 不恢复旧 help group、command_source 或 v2 bridge 断言，不使用大型快照和像素断言。
- 将本次 diff 引入的 tab 缩进统一为两空格；增加无第三方依赖的 Web 缩进检查并接入 CI。
- 恢复 `app.ts`、`auth.ts` 中无需求依据而删除的四个主题 token。

### F6 开发工具、非 Go watch 与示例卫生

- 删除所有生成在 `examples/plugins/**` 源码目录中的 `.exe`，并增加覆盖全部示例目录的 ignore 规则。
- 将 `example-permission-parameters` 改为准确描述现有行为的 `example-http-storage`，同步 manifest ID、入口、go.work、集成测试和文档。
- 删除 `example-webhook` 的 `webhook_register` 假命令；示例只处理 manifest 静态声明的 `webhook.received`。
- 正式记录非 Go 预构建入口 `dist/native/<platform>/<plugin-id>[.exe]`；watch 对无 `go.mod` 插件的 `dist/native` 例外监听，其他生成目录继续忽略。
- `raylea-plugin` 将相对 `--binary`、`--out` 和 `--include` 源路径按插件根目录解析；start-dev 继续传绝对输出路径，避免 watcher 回环。
- 增加 CLI 和开发工作区测试，覆盖从仓库外 CWD 调用、非 Go binary 变化触发重打包、生成 artifact 不触发重复 watch。

### F7 Server/Web 死代码、fail-closed 与文档收尾

- 删除 webhook `Registry.Register`、`DeletePlugin` 及只服务旧动态注册的测试写法；测试通过 `SyncSnapshots` 建立静态注册。
- 删除 `validateCommandPatterns` 等确认无调用者的旧命令 helper。
- webhook Service 在构造或应用装配时校验 Registry、Catalog、Dispatcher、Runtime 和 Secret Store 等必需依赖，删除 `SyncManifestRegistrations` 的 nil receiver 静默返回。
- 删除 Web 中 `runtimeFamily`、`entry`、`platforms`、`dataSchemaVersion` 和 `message.reply` 等确认无消费的旧 locale 键，将安装风险文案改为语言无关的“原生后端”。`runtimeConfig` 仍由插件详情页使用，因此保留。
- 将 `docs/release/plugin-go-vue-reset.md` 重命名为与内容一致的 `plugin-contract-v3-upgrade.md`，同步全部链接。
- 清理正式文档中的“之前”“不再维护”等编辑过程口吻；历史计划和变更记录保留必要的迁移叙事。
- 确认新增计划、permissions 文档和所有新 fixture 均进入最终 Git inventory。

### F8 全量验证与最终 review

- 对 F1 至 F7 逐项检查实现、contract、fixtures、tests、examples 和 docs 是否齐全。
- 检查完整 diff，确认没有恢复旧合同字段、兼容分支、生成二进制、无关视觉改动或新的技术栈。
- 按验证矩阵执行所有适用命令；每项完成后立即回写执行清单和结果。
- 所有 major 必须关闭。minor 只有在确认不属于本轮范围且记录维护者决策后才能保留；当前没有预设延后项。

## 验证矩阵

| 验证接口 | 计划命令或行为 | 状态 | 结果 |
| --- | --- | --- | --- |
| 合同 | `python scripts/ci/validate_contracts.py --self-test`、`--mode strict` | ☑️ 已完成 | self-test 与 strict 校验均通过。 |
| 生成物 | `node scripts/generate-runtime-schemas.mjs --verify`，检查 Web/Launcher/Server/Vue SDK 生成物 | ☑️ 已完成 | runtime schema verify 通过，嵌入和生成合同无漂移。 |
| Server 定向 | config repository/lifecycle/actions/runtime、pluginmarket/semver、dispatch/webhook 包测试 | ☑️ 已完成 | F2、F4、F7 所列定向包测试全部通过。 |
| Server 全量 | `go test ./...`、Server build、architecture tests | ☑️ 已完成 | `server` 全量 Go 测试通过；`dist/raylea-server.exe` 构建成功且产物非空，架构检查包含在全量测试中。 |
| Go SDK | `go test ./...`；适用时运行控制事件相关 `-race` | ☑️ 已完成 | Go SDK 全量测试通过。当前 Windows `CGO_ENABLED=0` 无法运行 race detector，Linux CI/nightly 保留 `-race` 覆盖。 |
| 开发脚本 | `node --test scripts/tests/plugin-dev-workspace.test.mjs` 和 CLI 定向测试 | ☑️ 已完成 | 开发脚本全量 40 项、开发工作区 11 项及 CLI/pluginbuild 定向测试通过。 |
| Web 类型与单测 | `pnpm run typecheck`、菜单/安装/壳定向测试、`pnpm test` | ☑️ 已完成 | typecheck 通过；62 个单测文件、313 项测试全部通过。 |
| Web E2E | 插件安装检查、权限确认和菜单管理路径 | ☑️ 已完成 | 最终全量 Playwright E2E 53/53 通过，包含 artifact v2、permissions、菜单和插件管理路径。 |
| Web 构建 | `pnpm build`，确认 `dist/` 产物存在 | ☑️ 已完成 | Web build 通过，`dist/` 产物已生成。验证主机使用 Node 24.18.0，低于项目声明的 26.7.0，但未影响本轮 typecheck、测试和构建结果。 |
| 示例 | manifest 集成测试、统一工具实际 inspect/pack/build-go | ☑️ 已完成 | 10 个示例模块构建和示例 manifest 集成测试通过；CLI 集成测试覆盖仓库外 CWD 的 inspect/pack/build-go 路径解析。 |
| 文档 | `python scripts/check-doc-links.py`、终态措辞和旧术语扫描 | ☑️ 已完成 | 132 个 Markdown 文件链接检查、agent docs 检查及旧术语/旧链接定向扫描通过。 |
| 仓库卫生 | Web tab 扫描、未跟踪二进制扫描、敏感信息扫描、`git diff --check` | ☑️ 已完成 | 237 个 Web 源码与测试文件缩进检查通过；示例目录无生成 `.exe`；敏感信息扫描仅命中脱敏 fixture 占位值；旧 semver、动态 webhook 注册、死 validator、旧 install mock 字段均无残留；`git diff --check` 通过（仅报告 Git 行尾转换警告）。 |

## 验收区

- 最终实现状态：评审修复已完成，7 个 major 和纳入范围的 minor 均已关闭。
- 未完成项：无。
- 当前阻塞项：无。
- 外部插件与官方目录仓库：不在本轮修改范围内。
- 维护者验收：待柒柒验收。
