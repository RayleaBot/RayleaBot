# Quality Gates

本页说明 RayleaBot 当前正式采用的验证入口、自动化工作流和发布回归层次。

## 默认验证命令

各工程的安装、构建、测试与类型检查命令以[工程基线的默认命令](./baseline.md)为准。门禁另有以下要求：

- Server：需要真实平台凭据或人工参与的用例单独登记于 [人工 Smoke](./manual-smoke.md)，通过 `manual_smoke` build tag 显式执行。
- Launcher：`pnpm test:e2e` 由 Renderer 配合模拟桌面桥运行，覆盖初始化失败流程；视觉细节交由人工审核，真实 Wails 系统集成另行验证。
- Plugins：主仓库 `sdk/go` 与 Go 示例执行 `go test -race ./...`；每个独立插件仓库自行执行 Go race test、Vue typecheck/test/build 和三平台 artifact 构建。

## 按改动面的最小验证

日常改动只运行与改动面对应的命令，其余检查由 nightly 与发布门禁补足。命令在所列目录执行，未注明目录时在仓库根执行。

纯文案或样式改动复核受影响内容，布局与交互变化按风险增加浏览器验证；不默认运行整套代码检查或新增测试。桌面界面验证统一使用 16:9：Web 使用 `1920×1080` 视口，Launcher 使用 `1920×1080` 屏幕上的默认 `1280×720` 窗口，两者的 Playwright 配置按此设置视口；低于 `1920×1080` 的桌面分辨率不在支持与验证范围内。下表的 Web 与 Launcher 代码检查按实际改动选择。

| 改动面 | 本地最小验证 | 由 nightly 或按风险补充 |
| --- | --- | --- |
| Server Go 代码 | `server/`：`go test ./<受影响包>/...`；装配或跨包流程变化时运行 `go test ./...` | 全部包 `-race`、golangci-lint（含 Windows 源码）、`govulncheck`、Windows 全量测试 |
| SQL 结构或查询 | `server/`：`sqlc generate`、`sqlc diff` 与受影响的存储测试 | — |
| 契约、fixtures、examples | `python scripts/ci/validate_contracts.py --mode=strict`；按输入变化和实际依赖选择受影响的生成器：`node scripts/generate-runtime-schemas.mjs --verify`、`go run ./tools/cmd/generate-error-codes --verify`、`python scripts/generate-plugin-wire.py --verify`；需要重新生成时去掉对应命令的 `--verify` | OpenAPI 或 WebSocket 变化时，Web 的 `pnpm generate:types` 漂移检查 |
| Web 代码 | `web/`：`pnpm run typecheck`、`pnpm test <受影响测试文件>`；构建配置变化时运行 `pnpm build` | `pnpm run check:indent`、完整 `pnpm test`、Playwright E2E |
| Launcher renderer 代码 | `launcher/`：`pnpm exec tsc -p tsconfig.renderer.json --noEmit`、`node ../scripts/run-vitest.mjs run <受影响测试文件>` | 组合 `pnpm run typecheck` / `pnpm test`、`pnpm build`、Renderer E2E |
| Launcher Go host / bridge | `launcher/`：`node scripts/run-go.mjs vet:platform ./<受影响包>/...`、`node scripts/run-go.mjs test:platform ./<受影响包>/...`；桥接定义变化时运行 `pnpm generate:wails` 和 renderer 类型检查 | 全量 Go vet/test、Wails bindings 漂移、`pnpm build` 与真实系统集成 |
| 插件 SDK 与示例 | `sdk/go`：`GOWORK=off go test ./...`；`sdk/vue`：`pnpm run typecheck && pnpm test` | `-race`、示例插件 UI 构建 |
| 设计 token 与图标 | `node scripts/generate-design-tokens.mjs --check`；图标变化时运行 `node scripts/generate-launcher-icons.mjs --check`（需要已安装 Launcher 依赖） | `repo-checks` 检查 token，`launcher` 构建检查图标 |
| 文档与指令 | `go run ./tools/cmd/check-doc-links`；指令文件变化时运行 `node scripts/check-agent-docs.mjs` | — |
| 仓库与 release 脚本 | Go 工具：`go vet ./tools/...`、`go test -count=1 ./tools/...`；Node 与剩余 Python 工具：`scripts/tests/` 或 `scripts/release/tests/` 中对应测试 | 两平台 `repo-checks`、`release-dry-run` |
| 依赖变化 | 对应工程的安装、类型检查与测试 | `python scripts/release/generate_third_party_notices.py --check --output THIRD_PARTY_NOTICES.md`、`pnpm audit` |

Race 测试需要 CGO 与 C 编译器；本机缺少时由 nightly 覆盖，并在结果中说明未在本地运行。[发布验收](#发布验收)是发布前的验收范围，不作为日常改动的默认清单。

真实浏览器渲染测试默认使用已准备的 Chromium。可通过 `RAYLEA_TEST_BROWSER_PATH` 指定受支持的 Chrome、Chromium 或 Edge 可执行文件；验证结果须说明实际使用的浏览器。

## 工作流

仓库没有按 PR 或推送触发的门禁：可提交性由本地最小验证保证，完整回归由 `nightly.yml` 每日执行，可交付性由 `release.yml` 在打 tag 时验证。工作流通过 `.github/actions/tool-versions` 从 `.tool-versions` 读取工具链版本后再安装运行时。

| 工作流 | 触发 | 平台 | 职责 |
| --- | --- | --- | --- |
| `nightly.yml` | 每日 18:00 UTC 与手动触发 | 主 jobs 为 `ubuntu-latest`；`repo-checks` 为两平台；`server-windows` 为 `windows-latest` | 下表所列的完整回归 |
| `nightly-status.yml` | nightly 结束 | `ubuntu-latest` | 对默认分支最新运行维护一个失败 issue，失败时创建或重新打开，成功后关闭；不参与 nightly 验证结论 |
| `release.yml` | `v*` tag | `windows-latest`、`ubuntu-latest`、`macos-26` | 通过 `release-build.yml` 构建四种正式 artifact，校验 release metadata 并对归档条目与运行环境准备前提做 smoke，再发布 GitHub Release |
| `repo-stats.yml` | 推送到 `main` | `ubuntu-latest` | 生成 README 引用的提交活动图，不参与验证 |

`nightly.yml` 的 job 与职责：

| Job | 内容 |
| --- | --- |
| `repo-checks` | 两平台：`scripts/tests/` 的 Python 与 Node 测试、strict contracts 及其自检、运行时 schema / 错误码 / 插件协议生成物漂移；Linux 另运行 agent docs、文档链接与设计 token 检查 |
| `server` | doctor、全部 Go 包 `-race` 与 atomic coverage、golangci-lint（含 `GOOS=windows`）、构建、`govulncheck` 二进制扫描、`sqlc diff` |
| `server-windows` | Windows 上执行全部 Go 包测试，覆盖插件进程、文件锁与浏览器归属等平台差异 |
| `web` | 生成类型漂移、缩进检查、typecheck、带覆盖率的单元测试、构建与生产构建 Playwright E2E |
| `launcher` | Wails bindings 漂移、typecheck、带覆盖率的 Renderer 与 Go 测试、Renderer E2E、构建（构建时校验 Launcher 图标生成物） |
| `plugins` | `sdk/go` 与 Go 示例 `-race`；`sdk/vue` 与示例插件 UI 的 typecheck/test/build |
| `dependencies` | Web 与 Launcher `pnpm audit`、第三方声明漂移 |
| `release-dry-run` | release 脚本测试、Server 构建、本版恢复演练、Web 构建与 Linux server 包打包 smoke |

Nightly 的 Server 测试一次运行同时启用 race 和 atomic coverage，覆盖全部 Go 包。

发布标签必须指向已有 nightly 成功记录的同一完整提交 SHA。发布工作流在构建前和上传前检查该提交最新运行的最后一次尝试；失败、取消、进行中、缺少记录或 API 不可用都阻止发布，其他提交或更早运行的成功不能替代。先推送代码与对应标签的发布说明，运行 nightly，再对通过的提交推送版本标签；具体步骤见[发布流程](../release/delivery-and-upgrade.md#发布流程与通道)。

Web E2E 只运行 `real-server` project，独立使用临时目录、SQLite 和动态端口，覆盖静态路由、登录与账户更新、插件安装与启停、配置及密钥遮罩、插件全局设置、治理作用域与名单增删、调度列表、日志详情、状态页备份与诊断导出和实际示例插件 iframe；视觉细节不写 E2E。在 `web/` 执行 `corepack pnpm run test:e2e:production` 构建 Web 与示例插件 UI 并运行用例；需要安装本地插件包的用例写入临时 `build_info.json`，使最低 Core 版本检查可以执行。

Nightly 的 `release-dry-run` 在构建 Server 后执行 `python scripts/release/rehearse_current_recovery.py --server dist/server/raylea-server --output dist/current-recovery-rehearsal`。输出目录必须不存在，保存合成数据、恢复包、进程日志和结果 JSON；验证空目录初始化、当前格式备份、恢复到空目录、登录、配置与插件数据一致性，以及重复启动幂等。

## 发布验收

发布前按受影响面完成：

- strict contracts 与生成物漂移（运行时 schema、错误码、插件协议与 Web 生成类型）；
- 目标包 `-race`、Server 测试与构建、二进制漏洞扫描；
- Web 与 Launcher 类型检查、测试、构建和风险对应的 E2E；
- Go SDK 与示例 `-race`、Vue SDK 与示例页面 typecheck/test/build、SDK 打包与全新环境安装；
- 四种归档的 `LICENSE`、`THIRD_PARTY_NOTICES.md`、release metadata 与归档 smoke；
- doctor、agent docs、文档链接与 `git diff --check`。

只有退出码不能证明真实产物时，继续检查生成文件、归档内容或运行效果。

## 验证原则

- 工具链版本值由 `.tool-versions` 提供，工作流通过共享 action 读取后安装。doctor 与契约校验器核对 `go.mod`、`package.json` 等生态工程文件，必要副本发生漂移时失败。
- 事件、插件协议、配置、错误码和初始化相关 Golden Fixtures 进入正式门禁，不只停留在文档说明。
- 本地最小验证负责可提交性，nightly 负责完整回归，发布门禁负责可交付性。
- 恢复、运行环境准备和交付矩阵验证进入正式工作流，不只停留在文档说明。
