# 全面清理评审与执行计划 v1

- 状态：待执行
- 评审基线：`809fec3a`
- 范围：允许兼容性破坏；清理过度防御、重复实现、硬编码、低质量代码、过时文档与文案；重组后端包结构；收敛契约与测试。

本文件同时作为评审记录与执行计划。引用行号对应评审基线，改动后以符号与文件路径为准。每批次完成后在本文件勾选并记录验收结果。

## 目标与约束

- 对外语义变更遵循契约先行：先改 `contracts/`，再同步实现、生成物、fixtures、examples、SDK、Web 与 Launcher。
- 一个批次表达一个逻辑变更，独立合并；批次内允许破坏性调整。
- 每批次运行能证明该改动正确性的最小验证；契约、生成物与 fixtures 变更按 `docs/engineering/quality-gates.md` 执行门禁。
- 不为未发生的需求预埋抽象；合并薄包与重复实现时保留真实复用边界。

## 一、评审发现

### 1.1 过度防御与静默降级

- **[高] 安装后置与回滚错误被丢弃**：`server/internal/app/app_run.go:302,310,318,320`。`lifecycle.Reload`、回滚回调、`RemovePluginTemplates` 的错误被 `_ =` 丢弃后仍返回成功，安装任务结果不可信。动作：错误汇总回传，安装/卸载任务按失败处理。
- **[高] HTTP 层 nil 依赖返回假状态**：`server/internal/management/update.go:46-53`（service nil 回 `200 disabled`）、`plugin_ui_settings.go:115-118`、`plugins.go:260,275`。违反“必选依赖缺失在装配层报错”。动作：构造器返回 error，删除 handler 内 nil 分支。
- **[高] 按错误文案分支**：`wsevents/protocol_targets.go:137-147`（`strings.Contains("timed out"/"not connected")`）、`eventpipeline/dispatch/scheduler_log.go:134`、`deps/manager_prepare.go:159`、`pluginmarket/service.go:156,193`、`storage/store_schema.go:328-330`（`"unique"` 子串、建表 SQL 中的 `'weibo'`）。动作：error 提供稳定 code，分支只依赖 code 与结构化字段。
- **[高] 可选依赖 nil 降级为业务结果**：`management/plugins.go:107-111,192,350,366,391`、`management/plugin_store.go:42-44`、`plugins/actions/default_registry.go:59,94`、`dispatch/outbound_action.go:121-129`。装配缺陷被表达为 `permission_denied` 或 500。动作：所需能力进构造参数，权限依赖缺失返回内部错误。
- **[高] Launcher 状态损坏静默重置**：`launcher/internal/desktop/settings.go:69-82` 解析失败覆盖为默认值并返回成功；`launcher/internal/desktop/service.go:47-54` `ServiceShutdown` 无条件 nil。动作：返回结构化错误并保留诊断。
- **[中] 配置热更新非原子发布**：`configruntime/apply.go:272-344` 逐对象改写、`:321-323` 最后发布配置，读取窗口内可组合出“新配置 + 旧派生状态”。动作：构造完整派生对象组后一次交换，读路径统一取快照。
- **[中] 包级可变全局供测试改写**：`onebot11/shell_api.go:12,88-92`。动作：改为构造期注入。

### 1.2 重复实现

- **[高] ZIP 路径穿越校验 6+ 份且规则不一致**：`releaseupdate/archive.go:115-167`、`plugins/lifecycle/install.go:1296-1299`、`plugins/pluginstore/file.go:290`、`plugins/artifact/artifact.go:298`、`management/plugin_icon.go:48`、`management/plugin_ui_http.go:346-352`、`management/development.go:69-73`。动作：抽出统一 `fsguard`（zip-slip、目录包含、Windows 保留名）。
- **[高] ZIP 读写/解压 6 个包各一套**：`backup/archive.go`、`cli/restore.go`、`deps/archive.go`、`releaseupdate/archive.go`、`plugins/lifecycle/install.go:1264-1373`、`system/system_diagnostics_http.go:70-110`。限额、符号链接、去重、进度回调行为分散。动作：共享 `archive` 原语，调用方只提供策略参数。
- **[高] 下载与摘要校验两份**：`deps/download.go:30-54` vs `releaseupdate/download.go:37-174`；`pathInside` 两份（`cmd/raylea-updater/main.go:403` vs `releaseupdate/archive.go:240`）。动作：共享 `download` 原语与路径判断。
- **[高] 请求体解码副本且绕过限额**：7 处直接 `json.NewDecoder(r.Body)` 无 `MaxBytesReader`（`management/plugin_ui_http.go:113`、`plugin_ui_settings.go:79`、`plugin_ui_secrets.go:47,91`、`system_handlers.go:257`、`system_recovery_handlers.go:84`、`plugins.go:284`）；`plugins.go:283-296` 的 `decodeStrictJSON` 是 `httpapi.DecodeStrictJSON` 的降级副本。动作：全部改用统一入口。
- **[高] 凭据与会话校验两份**：`management/account.go:26-35` vs `auth/credentials.go:16-19`；常量在 `config/runtime_constraints.go:14-16` 与 `auth/manager.go:123-134` 重复。动作：校验归领域层，常量为单一来源。
- **[高] Web 日志两页 425 行重复**：`web/src/views/operations/LogsView.vue`（581 行）与 `LogsHistoryView.vue`（634 行）。动作：抽共享工作区组件/composable，仅保留范围差异。
- **[中] management 错误写出包装上百次**：`management/auth.go:225,347`、`core.go:224`、`plugins.go:424,428`、`ws_handlers.go:56-78` 等，均为 `httpapi` 薄包装。动作：建立 code→status/message 单一日录。
- **[中] 平台中文名映射两份**：`integrations/accountvalidation/service.go:273-286,478-491` vs `integrations/thirdparty/credential_status.go:14-24`。动作：平台元数据集中。
- **[中] OneBot 缓存失效 switch 两份**：`onebot11/cache.go:199-248` 与 `:250-283`，字段口径不一致。动作：合并状态机，入口适配差异。
- **[中] Web 重复块**：`AccessListsView.vue:54-208` 与 `:255-426` 黑白名单复制；`stores/socket-router.ts:20-142` 三段同构 debounce；时间戳归一化三份（`stores/log-state.ts:244-256`、`stores/plugin-console.ts:174-201`、`lib/format.ts:152-165`）；状态色调两份且漂移（`lib/display.ts:87-101` vs `lib/status-tone.ts:3-23`）。动作：各收敛为单一实现。

### 1.3 硬编码

- **[高] 插件安装目录语义散落**：`"plugins/installed"`、`.plugin-install-` 同时充当发现标签、身份判定、清理前缀与归档路径，见 `runtimepaths/paths.go:93,103,115`、`cli/cli.go:217`、`plugins/catalog/discovery_conflict.go:43`、`recovery/manifest.go:76`、`plugins/lifecycle/install.go:595,796,950,1015`、`uninstall.go:80-86`、`backup/archive.go:176`。动作：提为 `plugins` 包常量/类型化扫描根。
- **[高] 契约版本常量至少 12 处手写**：`sdk/go/types.go:11`、`sdk/go/pluginbuild/build.go:23-28`、`sdk/vue/src/client.ts:10`、`scripts/plugin-dev-workspace.mjs:47,138`、`scripts/release/release_tool.py:320-322` 等。动作：由 `contracts/` 生成各语言常量，统一升级。
- **[高] CI 变更分类器无法自检**：`scripts/ci/detect_changes.py:25,114,193-198,252` 未分类 `go.work.sum` 与 `design-qa.md`，CI `ci.yml:47,439` 会失败并跳过下游 job（当前实测 `--self-test` exit 1）。动作：补齐分类规则并清理根目录草稿。
- **[中] 默认值多份**：限流 `20/10s`、熔断 `30s` 在 `config/canonical_typed.go:43-76`、`eventpipeline/outbound/rate_limiter.go:14-17`、`circuit_breaker.go:16`、`config/contracts/config.user.schema.json:641,645`；`builtinmenu/menu.go:190` 再写 `help/帮助`。动作：schema 与代码共享常量源。
- **[中] 发布命名常量散落**：`windows-x64-full`、`.rayleabot-update-*` 见 `releaseupdate/trust.go:273`、`archive.go:21`、`client.go:117`、`transaction.go:115,308,649`、`cmd/raylea-updater/main.go:119,135,417`。动作：`releaseupdate` 导出常量与校验函数。
- **[中] Chrome UA 5 包 3 个版本**：`integrations/weibo/qrcode_login.go:23`、`netease_music/qrcode_login.go:24`、`thirdparty/avatar.go:43`、`plugins/actions/render_resources.go:28`、`bilibili/session/identity_provider.go:19-49`。动作：共享 UA 构造函数。
- **[中] 基础设施常量重复**：默认端点 `127.0.0.1:8080` 至少 6 处（`launcher/internal/desktop/management.go:31-72`、`snapshots.go:12`、`launcher/src/renderer/src/AppState.shared.ts:59-63`、`scripts/start-dev-support.mjs:176-177`、`scripts/release/package_runtime.py:223`、`config/default.yaml:80-81`）；工具链版本 5 处；平台命名映射 6 处；`X-Raylea-Launcher-Control` 4 处。动作：单一清单/共享常量模块。
- **[中] Web 硬编码**：断点值十余个且 1px 漂移（`styles/_responsive.scss:1,15,60,151` 与各视图）；配置工作台手工镜像 schema 默认值（`lib/config-form.ts:36,52-66,188-209,252-382`）；WebSocket 路径与事件名手写（`stores/socket-controller.ts:28,36,44`、`stores/socket-router.ts:212,222,252,256`）。动作：token 化、schema 生成、生成器导出通道常量。

### 1.4 低质量与死代码

- **[高] `management` god 包**：29 文件约 5000 行；`governance.go:16-25` 在 HTTP 层构造领域服务并持有具体类型；`plugin_view_summary.go:71-137`、`plugin_store.go:224-229` 复制领域组装。动作：按资源拆子包，视图回领域包。
- **[中] 死类型/死参数/死常量**：`management/plugins.go:432-442`（与 `httpapi/httpapi.go:28-38` 重复且未用）、`plugins/summary_view.go:175-177`、`httpapi/httpapi.go:201-203`（`accessLogLevel` 恒返回 Debug）、`plugins/runtime/manager_delivery.go:281-289`、`dispatch/outbound_action.go:199-208`、`plugins/artifact/artifact.go:22`（`ProtocolVersion = "2"` 残留）。动作：删除。
- **[中] 不可达/恒真分支**：`sdk/go/actions.go:28-30,74-77`、`sdk/go/protocol.go:186-189`、`launcher/internal/desktop/coordinator_process.go:127-129`、`process_windows.go:129`（永假 `ESRCH`）、`scripts/release/release_tool.py:230-239,361-363`、`sdk/go/pluginbuild/build.go:142,604`、`scripts/start-dev.mjs:1131-1132`。动作：删除或将不变量改为注释。
- **[中] Web `main.ts` 双注册**：`web/src/main.ts:258-266` 注册后 `:269` 经 `installAvailabilityHandlers` 在 `:202-205` 覆盖同名回调，首份成为死代码。动作：合并为单入口。
- **[中] 复制粘贴残留分支**：`plugins/runtime/actions.go:354-358` blacklist 保留了 whitelist 的 `set_enabled` 前置条件，随后必走 default 拒绝。动作：删除该条件。
- **[中] 超大文件**：`PluginDetailView.vue` 1588 行、`SchedulerJobsView.vue` 1444 行、`AccessListsView.vue` 1206 行、`BasicLayout.vue` 1002 行、`plugins/lifecycle/controller.go` 1099 行、`builtinmenu/menu.go` 906 行、`plugins/runtime/actions.go` 812 行。动作：按职责拆分。
- **[低] Web 死代码**：`lib/display.ts:77`、`stores/ui-shell.ts:205,293`（`resetRestoredTabs` 仅测试）、`stores/session.ts:8-14,43,157`（bearer 残留）、`components/ui/{alert-dialog,dialog,tabs,separator,badge,dropdown-menu}` 无引用。动作：删除或标注 vendored。
- **[低] 每请求重建 HTTP transport**：`plugins/actions/http_client.go:201-202`。动作：复用 transport/连接池。

### 1.5 测试体系

- **[高] 竞态回归在 PR 门禁空转**：`ci.yml:113-115` server 无 `-race`（`:297` 仅覆盖示例模块），唯一 `-race` 在 `nightly.yml:56`；`app/internal/app/app_runtime_state_test.go:10`、`plugins/catalog/catalog_test.go:90-129` 等无断言、仅 `-race` 下有效。动作：CI 增加核心包 `-race`。
- **[高] 测试替身复制生产实现**：`plugins/lifecycle/test_helpers_test.go:96-171` 逐行复刻 `plugins/runtime/runtime_registry.go:12-112`；`management/plugins_catalog_test_support_test.go:64-79` 复刻 Catalog 冲突规则。动作：改用真实轻量实现，副作用走 Options。
- **[中] 无生成器的 property 测试**：`management/plugins_install_test.go:122-156`、`tests/integration/auth_middleware_test.go:248-283`、`plugins/catalog/catalog_test.go:19-88`。动作：删除 rapid 外壳或补真实生成器。
- **[中] 固定 sleep 与忙等**：`plugins/lifecycle/operation_regression_test.go:93`（5.25s）、`onebot11/shell_test.go:108,145,480`、`app/app_close_test.go:33-42`、`app/metrics_registry_test.go:25-35`（真实端口）。动作：注入时钟/channel，超过 100ms 的等待清零。
- **[中] 断言用户可见文案**：`tests/services/app_run_chat_policy_test.go:249`、`chatpolicy/policy_service_test.go:47-48`、`wsevents/protocol_http_test.go:46-82`、`outbound/observability_test.go:127-143`。动作：改断言 code/结构化字段。
- **[中] fixture 自证测试**：`tests/integration/http_health_test.go:54-185` 将 fixture 反向映射成对象再断言回 fixture。动作：用真实降级依赖构造 report。
- **[中] 测试 helper 大规模重复**：`tests/services/harness_test.go:165-343` 复刻装配且含 `_ =` 死参数与重复 wrapper；`tests/ws/helpers_test.go:43-99` ≈ `tests/integration/config_http_test.go:589-645`；render 假 runner 四份；`TestMain` 进程级 `os.Chdir`。动作：testutil 提供唯一装配入口。
- **[中] 覆盖盲区**：`integrations/netease_music`（加密、扫码登录）零测试；Windows 专用 `install_rename_retry_windows_test.go` 在 ubuntu CI 不执行；`qqofficial/live_*_test.go` 永远 skip 且无录制回放。动作：补离线回归与 Windows job。

### 1.6 文档与文案

- **[高] CLI 文档与契约冲突**：`docs/user/cli.md:45,48` 称 backup/cleanup 在线可用，契约 `contracts/cli-commands.yaml:232,316` 为 `online: false`。动作：按契约修正可用性说明。
- **[高] 项目规划过时**：`docs/RayleaBot机器人项目规划.md:20,21,57` 仍称 QQ 官方为 planned；`:24` 称预编译 Go 是唯一正式插件后端，与 `contracts/plugin-artifact.schema.json`“Language-neutral”冲突。动作：更新为当前能力，或改为引用 PRODUCT/契约。
- **[高] README 协议版本过时**：`README.md:15` 仍写 JSONL v1，契约已是 protocol v3。动作：改为 v3 并收敛表述。
- **[高] 插件文档旧语义**：`docs/plugin/permissions-and-manifest.md:119`（artifact.json 全量文件清单）、`docs/plugin/management-ui.md:10`（artifact.json 含 `ui` 字段）、`docs/plugin/README.md:22`、`examples/plugins/README.md:10`、`server/internal/deps/README.md:3,24`（Go-only 表述）。动作：按 artifact v2 / 语言中立修正。
- **[中] 商店签名机制描述错误**：`docs/release/acceptance-and-risks.md:58` 与 `docs/RayleaBot机器人项目规划.md:14` 称签名静态目录，契约无签名字段。动作：改为 HTTPS 来源 + 归档 SHA-256 校验。
- **[中] 过程记录混入现行文档**：根 `design-qa.md`、`docs/release/log-runtime-repair-2026-09-04.md`（含 protocol v2 表述）。动作：删除或移入 `docs/CHANGELOGS/`，并从阅读入口移除。
- **[中] 架构文档缺 QQ 官方**：`docs/architecture/platform-architecture.md:26,49,94,135`、`docs/architecture/README.md:10,12`、`docs/engineering/baseline.md:18`、`docs/architecture/message-flow.md:80`。动作：补齐 adapter 与出站路由。
- **[中] Web i18n 双轨与死键**：视图硬编码中文与 locale 并存（`views/protocols/AdaptersView.vue:101-133`、`views/operations/SchedulerJobsView.vue`）；85 个错误码仅 28 个有文案且 `errors.common.saveFailed` 未定义；基础组件默认文案硬编码（`components/AppDataTable.vue:12,40` 等）；用翻译文案做分支（`components/RetryPanel.vue:28-39`、`lib/exception-status.ts:34-40`）。动作：单轨化并补覆盖校验。

### 1.7 契约与 fixtures / examples

- **[高] `platform.resource_missing` 双语义**：`contracts/error-codes.yaml:158-166` 声明 503，但 `contracts/web-api.openapi.yaml:1465-1466` 与 `management/plugin_icon.go:43`、`render.go:251` 按 404 使用。动作：拆分为 503/404 两个 code 或定义 scope 并对齐。
- **[高] `platform.invalid_request` 声明 400、实现按 409**：`management/plugin_store.go:213,215`、`plugin_ui_http.go:311,316`。动作：冲突场景新增 409 码。
- **[高] `plugin.internal_error` 契约无 http 语义、实现以 502 返回**：`error-codes.yaml:218-226`、`plugin_ui_http.go:130-136`。动作：补齐契约或改用已声明码。
- **[高] 未登记错误码**：`recovery/recovery.go:188,198,207,437-448,468-484`、`diagnostics/report.go:54,95`、`system/readiness.go:19,37`；架构测试只扫 management（`tests/architecture/error_codes_test.go:31`）。动作：登记全部 code 并扩展测试范围。
- **[高] 3 个死错误码**：`adapter.transport_degraded`、`adapter.transport_invalid_config`、`adapter.compatibility_not_supported`（`error-codes.yaml:832-870`）。动作：删除或在实现中按定义使用。
- **[高] `examples/http` 至少 8 份漂移且 CI 不校验**：`examples/http/config-update.response.json:3,6`、`governance-*.json`（缺 `scope`）、`recovery-confirm.task-detail.json` 等。动作：按 OpenAPI 重写并纳入校验。
- **[高] fixture-ready 宣称不成立**：bridge 与 errors fixtures 只做存在性检查（`scripts/ci/validate_contracts.py:66-91`）。动作：补 schema/语义校验。
- **[中] 同一 shape 三处重复且放宽**：`plugin-info.schema.json:128-229` vs `web-api.openapi.yaml:4309-4378` vs `websocket-events.yaml:187-246`；`trusted_code_confirmed` 一处 const、一处 boolean。动作：共享 shape 定义。
- **[中] 其他契约问题**：无界列表缺分页/排序语义；store detail 的 `releases` 实际恒 0/1 条；`config.user.schema.json:565-585` 已废弃字段仍 required；`/api/adapters` 缺 security；OpenAPI `info.version` 仍 0.1.0；4 个孤儿 fixture。动作：逐项在破坏窗口处理。

### 1.8 Launcher / SDK / 脚本 / 模板

- **[高] CI 分类器自检失败**：见 1.3，`scripts/ci/detect_changes.py --self-test` 实测 exit 1。
- **[高] `.deps/manifest.json` 三份校验严格度不同**：`server/internal/deps/metadata.go:12-58`、`launcher/internal/desktop/environment.go:267-303`、`scripts/release/package_runtime.py:267-303`。动作：抽公共校验。
- **[中] Go 可执行解析两份**：`scripts/start-dev.mjs:1010-1030` vs `launcher/scripts/process-invocation.mjs:22-53`。动作：共享模块。
- **[中] 发布清单重复、冒烟清单被覆盖**：`scripts/release/release_tool.py:51-96` vs `package_runtime.py:101-142`；`smoke_release.py:22-87` 条目全部包含在 `REQUIRED_PATHS`。动作：合并为单一声明。
- **[中] Launcher 超时/轮询常量 10+ 处**：`launcher/internal/desktop/coordinator_process.go:67-436`、`coordinator.go:130,134`、`release.go:23-27`、`management.go:26`。动作：集中为带说明的常量。
- **[中] semver 校验三份宽严不一**：`contracts/release-manifest.schema.json:12-14`、`launcher/internal/desktop/release.go:30`、`launcher/scripts/package-metadata.mjs:3`。动作：统一来源。
- **[低] 模板孤儿文件**：`templates/leaderboard.list/preview.html` 与 `preview.json` 机制冲突。动作：删除并同步设计允许清单。
- **[低] 工具链不一致**：`make doctor` 在 macOS 漏检 App Bundle 浏览器（`scripts/check-toolchain.py:312`）；`start.sh` 不支持 `RAYLEA_START_NODE`（与 `start.bat:23-34` 不对称）。动作：复用同一候选列表与环境变量支持。

## 二、后端包结构重组目标

现状核心问题：

- 底层包反向依赖功能包：`configruntime/deps.go:9-10,30` 直接引用 `plugins/actions` 与 `render/service`。
- 协议中立层依赖具体协议错误：`eventpipeline/outbound/sender.go:11,144-181`、`rate_limiter.go:10,79`、`dispatch/outbound_action.go:11,22` 构造并识别 `onebot11.Error`。
- 按技术层切出薄包：`render/repository` 仅被 `render/service` 引用；`testenv` 位于生产树。
- 路径解析包泄漏领域类型：`runtimepaths/paths.go:11-19,54-134` 导出 `plugincatalog.ScanRoot` 并依赖 `recovery`。

目标边界：

```text
internal/
  core/           chatevent, pubsub, storage(+sqlcgen), secrets, config(模型/schema/迁移)
  support/        fsguard, archive, download, logpath, filelock, semver, reconnect, redact
  adapters/       onebot11, qqofficial；统一对外 AdapterError，pipeline 不再 import 协议包
  eventpipeline/  bridge+chatpolicy+dispatch 合并；outbound 独立且协议中立
  pluginhost/     plugins/runtime + plugins/actions 合并；lifecycle 保留，压缩包处理下沉 support
  pluginmarket/   商店与 catalog 职责保持
  render/         repository 并入 service
  management/     按 auth/system/plugins/thirdparty 拆子包；领域视图回领域包；错误映射单一日录
  system/         health + diagnostics 合并
  app/            唯一装配层；必选依赖缺失在此报错
```

依赖规则：

- `core` 不依赖功能包；`support` 只依赖标准库与 `core` 基础类型。
- `adapters` 只暴露平台无关事件与错误；`eventpipeline` 不 import 具体协议包。
- `runtimepaths` 只返回字符串与通用结构，扫描根由 `app` 组装。
- `management` 不构造领域服务，不复制领域逻辑。
- `testenv` 移至 `server/tests/testutil`。

## 三、执行批次

### 批次 0：解锁门禁与清理草稿（无破坏性）

- [ ] 修复 `scripts/ci/detect_changes.py` 对 `go.work.sum`、`design-qa.md` 的分类，`--self-test` 通过。
- [ ] 删除根 `design-qa.md`；将 `docs/release/log-runtime-repair-2026-09-04.md` 移入 `docs/CHANGELOGS/` 或删除，并更新 `docs/release/README.md` 入口。
- [ ] 修正 `docs/user/cli.md`、`README.md`、`docs/RayleaBot机器人项目规划.md`、插件文档的过时表述（1.6）。
- [ ] 验收：`python scripts/ci/detect_changes.py --self-test`、`python scripts/check-doc-links.py` 通过。

### 批次 1：契约破坏性收敛（契约先行）

- [ ] 错误码：拆分 `resource_missing`/`invalid_request`；补齐 recovery/diagnostics/readiness/task 的 code；删除 3 个死码；`plugin.internal_error` 明确 http 语义。
- [ ] schema：共享插件命令/触发/webhook/管理页 shape；收紧 `trusted_code_confirmed`；废弃字段移出 required；补列表分页语义；`/api/adapters` 补 security。
- [ ] fixtures/examples：bridge 与 errors 增加语义校验；example HTTP 文档按 OpenAPI 重写并纳入 CI；清理孤儿 fixture 与重复样例。
- [ ] 同步生成物、SDK、Web 类型、Launcher 常量。
- [ ] 验收：`validate_contracts.py --self-test --mode=strict`、受影响模块测试与生成物 `--verify` 通过。

### 批次 2：共享原语替换重复实现

- [ ] 新建 `support/fsguard`、`support/archive`、`support/download`，替换 ZIP 校验/解压/下载的 6+ 份实现。
- [ ] 统一 `httpapi.DecodeStrictJSON` 与请求体限额，删除本地副本。
- [ ] 收敛凭据校验、平台元数据、错误写出目录。
- [ ] 集中默认值、UA、发布命名、工具链版本、平台映射常量。
- [ ] 验收：`go test ./...`（含受影响包）、脚本单测通过。

### 批次 3：后端包结构重组

- [ ] 解除 `configruntime` 反向依赖与 pipeline 对 `onebot11.Error` 的依赖。
- [ ] 合并 `eventpipeline` 子包、`plugins/runtime`+`plugins/actions`、`render/repository` 并入 `render`。
- [ ] 按目标结构迁移包；`runtimepaths` 去领域依赖；`testenv` 迁出生产树。
- [ ] 拆分 `management` 并建立错误映射目录；`health`+`diagnostics` 合入 `system`。
- [ ] 热更新改为快照原子发布；删除死类型/死参数/残留逻辑。
- [ ] 验收：`go build ./...`、`go vet ./...`、`go test ./...`；结构检查脚本通过。

### 批次 4：测试与 Web 清理

- [ ] CI 增加核心包 `-race` 与 Windows job；删除无生成器 property 测试与替身副本；固定等待降至 100ms 以下；文案断言改 code。
- [ ] testutil 提供唯一装配入口，删除 integration/ws/services 重复 helper 与 `TestMain` `os.Chdir`。
- [ ] Web：合并日志两页与黑白名单；i18n 单轨化并补错误码文案；`PluginManagementUIHost` 改用生成类型；配置元数据从 schema 派生；拆分超大视图。
- [ ] 验收：`go test ./...`、Web `pnpm test` + typecheck、Launcher 测试通过；手动回归日志、插件详情、定时任务三个页面。

## 四、暂缓项

- 增量/差分更新、调试流、批量消息与复杂流式回传按 `contracts/README.md` 的延后边界处理，不在本轮展开。
- `plugins/installed/` 为本机运行产物，不纳入清理提交。
