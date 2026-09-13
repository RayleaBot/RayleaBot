# Quality Gates

本页说明 RayleaBot 当前正式采用的验证入口、CI 门禁和发布回归层次。

## 默认验证命令

各工程的安装、构建、测试与类型检查命令以[工程基线的默认命令](./baseline.md)为准。门禁另有以下要求：

- Server：需要真实平台凭据或人工参与的用例单独登记于 [人工 Smoke](./manual-smoke.md)，通过 `manual_smoke` build tag 显式执行。
- Launcher：`pnpm test:e2e` 由 Renderer 配合模拟桌面桥运行，覆盖窗口边界、字体加载、初始化失败和减少动态效果；真实 Wails 系统集成另行验证。
- Plugins：主仓库 `sdk/go` 与 Go 示例执行 `go test -race ./...`；每个独立插件仓库自行执行 Go race test、Vue typecheck/test/build 和三平台 artifact 构建。

## 按改动面的最小验证

日常改动只运行与改动面对应的命令，其余检查由 PR 与发布门禁补足。命令在所列目录执行，未注明目录时在仓库根执行。

纯文案或样式改动复核受影响内容，布局与交互变化按风险增加浏览器验证；不默认运行整套代码检查或新增测试。下表的 Web 与 Launcher 代码检查按实际改动选择。

| 改动面 | 本地最小验证 | 由 CI 或按风险补充 |
| --- | --- | --- |
| Server Go 代码 | `server/`：`go test ./<受影响包>/...`；装配或跨包流程变化时运行 `go test ./...` | 关键并发包 `-race`、golangci-lint（含 Windows 源码）、`govulncheck`、`server-windows` 回归 |
| SQL 结构或查询 | `server/`：`sqlc generate`、`sqlc diff` 与受影响的存储测试 | — |
| 契约、fixtures、examples | `python scripts/ci/validate_contracts.py --mode=strict`；按输入变化和实际依赖选择受影响的生成器：`node scripts/generate-runtime-schemas.mjs --verify`、`python scripts/generate-error-codes.py --verify`、`python scripts/generate-plugin-wire.py --verify`、`python scripts/generate-launcher-api.py --verify`；需要重新生成时去掉对应命令的 `--verify` | OpenAPI 或 WebSocket 变化时，Web 与 Launcher 的 `pnpm generate:types` 漂移检查 |
| Web 代码 | `web/`：`pnpm run typecheck`、`pnpm test <受影响测试文件>`；构建配置变化时运行 `pnpm build` | `pnpm run check:indent`、完整 `pnpm test`、Playwright E2E |
| Launcher renderer 代码 | `launcher/`：`pnpm exec tsc -p tsconfig.renderer.json --noEmit`、`node ../scripts/run-vitest.mjs run <受影响测试文件>` | 组合 `pnpm run typecheck` / `pnpm test`、`pnpm build`、Renderer E2E |
| Launcher Go host / bridge | `launcher/`：`node scripts/run-go.mjs vet:platform ./<受影响包>/...`、`node scripts/run-go.mjs test:platform ./<受影响包>/...`；桥接定义变化时运行 `pnpm generate:wails` 和 renderer 类型检查 | 全量 Go vet/test、Wails bindings 漂移、`pnpm build` 与真实系统集成 |
| 插件 SDK 与示例 | `sdk/go`：`GOWORK=off go test ./...`；`sdk/vue`：`pnpm run typecheck && pnpm test` | `-race`、示例插件 UI 构建 |
| 设计 token 与图标 | `node scripts/generate-design-tokens.mjs --check`；图标变化时运行 `node scripts/generate-launcher-icons.mjs --check` | — |
| 文档与指令 | `python scripts/check-doc-links.py`；指令文件变化时运行 `node scripts/check-agent-docs.mjs` | — |
| CI 脚本 | `python scripts/ci/detect_changes.py --self-test` 与 `scripts/tests/` 中对应测试 | 两平台 `ci-self-check` |
| 依赖变化 | 对应工程的安装、类型检查与测试 | `python scripts/release/generate_third_party_notices.py --check --output THIRD_PARTY_NOTICES.md`、`pnpm audit` |

Race 测试需要 CGO 与 C 编译器；本机缺少时由 CI 覆盖，并在结果中说明未在本地运行。[实施顺序第 9 节](./implementation-order.md#9-验收与发布)是发布验收范围，不作为日常改动的默认清单。

## CI 工作流

| 工作流 | 主要职责 |
| --- | --- |
| `ci.yml` | 变更范围识别、contracts-lite、Server、design-system、Web、Launcher、plugins、release-helper、third-party-notices、agent-docs、ci-self-check 和必需结果汇总 |
| `nightly.yml` | strict contracts、覆盖率、开发与生产构建的 Web Playwright E2E、安全、依赖、运行环境和 release dry-run 巡检 |
| `release.yml` | 正式产物打包、metadata 校验、packaged 协议与模板 smoke、本版 recovery drill、长期自托管 smoke |
| `self-host-smoke.yml` | 按 artifact 子集复用正式打包路径，长期巡检 packaged 协议与模板 smoke、自托管运行、诊断与恢复全流程 |

## 当前门禁层次

- PR 默认门禁覆盖 contracts-lite、Server 测试/构建/核心 lint、关键并发包 race、Windows 锁与浏览器进程回归、Web 与 Launcher typecheck/test/build、Go/Vue 插件 SDK、示例、design-system、third-party-notices、agent-docs、CI 自检和必需结果汇总。
- `contracts/**`、`fixtures/**`、`examples/**`、`sdk/**` 与 `plugins/**` 变更会触发 `ci.yml` 对应 job，同步执行 Web 与 Launcher 的 OpenAPI 生成类型漂移检查。
- Web 与 Launcher Renderer 的 Playwright E2E 由 `nightly.yml` 自动执行；本版恢复和更长时长自托管巡检进入 release 或手动高成本回归层。
- 发布门禁覆盖正式产物矩阵、release metadata、packaged `/api/adapters`、`/api/protocols/onebot11/compatibility`、模板预览工作区全流程、packaged recovery drill 和长期自托管 smoke。
- 高成本依赖审计和长时段巡检保留在 `nightly.yml` 或发布门禁，不挤占每个 PR 的默认门禁预算。

## 当前工作流矩阵

| 工作流 | 平台 | PR 门禁 | 说明 |
| --- | --- | --- | --- |
| `ci.yml` | 主 jobs 为 `ubuntu-latest`；`server-windows` 为 `windows-latest`；`ci-self-check` 为两平台 | 是 | 校验 contracts、Server（含核心 lint/race 与 Windows 回归）、Web、Launcher、SDK、设计系统、notices、agent docs、CI 脚本与必需结果汇总 |
| `nightly.yml` | `ubuntu-latest` | 否 | 负责夜间长时段回归、Playwright E2E、依赖、安全和环境巡检 |
| `release.yml` | `windows-latest`、`ubuntu-latest`、`macos-26` | Tag 门禁 | 构建四种正式 artifact，校验 release metadata、协议读取接口、模板预览、recovery drill 与交付 smoke |
| `self-host-smoke.yml` | `windows-latest`、`ubuntu-latest`、`macos-26` | 否 | 对四种 artifact 运行长期自托管、诊断与恢复探针 |

Nightly 的 Server 测试一次运行同时启用 race 和 atomic coverage，覆盖全部 Go 包。SQL 例外的复审日期到期产生维护提示；登记缺失、字段无效、文件不存在或与实际 SQL 使用不符仍阻止结构检查。

PR 的关键并发包 race 覆盖 App、配置应用、事件管线、插件 Catalog/Runtime/Lifecycle、广播、协议事件、OneBot 回调和存储快照；完整包清单由 `ci.yml` 维护。Server、契约或 CI 规则变化触发服务端门禁；`go.work.sum` 触发工作区相关消费者，SQL 例外登记变化触发 Server 与 CI 自检。跨目录重命名同时按来源和目标路径识别影响范围。

Web 生产构建 E2E 只运行 `real-server` project，独立使用临时目录、SQLite 和动态端口，覆盖静态路由、鉴权与账户更新、配置及密钥遮罩、插件全局设置、治理作用域与名单增删、调度列表、日志详情和实际示例插件 iframe。开发模式的 `ui-fixtures` project 保留受控网络、消息节奏和展示数据；写入后的固定快照由测试指定，不承担 Server 规则的证明。覆盖范围和运行方式见 [Web 端到端验证](./web-testing.md)。`RAYLEA_E2E_WEB_PORT` 可隔离开发模式的 Web 端口。

Nightly 的 `release-dry-run` 在构建 Server 后执行 `python scripts/release/rehearse_current_recovery.py --server dist/server/raylea-server --output dist/current-recovery-rehearsal`。输出目录必须不存在，保存合成数据、恢复包、进程日志和结果 JSON；验证空目录初始化、当前格式备份、恢复到空目录、登录、配置与插件数据一致性，以及重复启动幂等。

同一工作流另以 `--legacy-schema --output dist/legacy-recovery-rehearsal` 验证真实 `000001` 合成备份和首次启动迁移到当前结构。迁移演练与当前结构恢复各使用独立输出目录。

## 验证原则

- 工具链版本值由 `.tool-versions` 提供，CI 通过共享 action 读取后安装。doctor 校验生态工程文件，契约门禁核对基线文档；这两类必要副本发生漂移时失败。
- 事件、插件协议、配置、错误码和初始化相关 Golden Fixtures 进入正式门禁，不只停留在文档说明。
- 轻量门禁负责可合并性，发布门禁负责可交付性。
- 恢复、运行环境准备和交付矩阵验证进入正式工作流，不只停留在文档说明。
