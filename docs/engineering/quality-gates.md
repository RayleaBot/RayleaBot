# Quality Gates

本页说明 RayleaBot 当前正式采用的验证入口、CI 门禁和发布回归层次。

## 默认验证命令

### Server

- `go test ./...`
- `mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server`

### Web

- `pnpm install --frozen-lockfile`
- `pnpm run typecheck`
- `pnpm test`
- `pnpm build`

### Launcher

- `pnpm install --frozen-lockfile`
- `pnpm run typecheck`
- `pnpm test`
- `pnpm build`
- `pnpm test:e2e`：当前 Renderer 配合模拟桌面桥，覆盖窗口边界、字体加载、初始化失败和减少动态效果；真实 Wails 系统集成另行验证。

### Plugins

- 主仓库 `sdk/go` 与 Go 示例：`go test -race ./...`
- 主仓库 `sdk/vue` 与 Vue 示例管理页：`pnpm run typecheck && pnpm test && pnpm build`
- 开发同步：`node --test scripts/tests/plugin-dev-workspace.test.mjs`
- 每个独立插件仓库自行执行 Go race test、Vue typecheck/test/build 和三平台 artifact 构建。

## CI 工作流

| 工作流 | 主要职责 |
| --- | --- |
| `ci.yml` | 变更范围识别、contracts-lite、Server、design-system、Web、Launcher、plugins、release-helper、third-party-notices、agent-docs、ci-self-check 和必需结果汇总 |
| `nightly.yml` | strict contracts、覆盖率、Playwright E2E、安全、依赖、运行环境和 release dry-run 巡检 |
| `release.yml` | 正式产物打包、metadata 校验、packaged 协议与模板 smoke、跨版本 recovery drill、长期自托管 smoke |
| `self-host-smoke.yml` | 按 artifact 子集复用正式打包路径，长期巡检 packaged 协议与模板 smoke、自托管运行、诊断与恢复全流程 |

## 当前门禁层次

- PR 默认门禁覆盖 contracts-lite、Server、Web 与 Launcher typecheck/test/build、Go/Vue 插件 SDK、示例、design-system、third-party-notices、agent-docs、CI 自检和必需结果汇总。
- `contracts/**`、`fixtures/**`、`examples/**`、`sdk/**` 与 `plugins/**` 变更会触发 `ci.yml` 对应 job，同步执行 Web 与 Launcher 的 OpenAPI 生成类型漂移检查。
- Web 与 Launcher Renderer 的 Playwright E2E 由 `nightly.yml` 自动执行；跨版本恢复和更长时长自托管巡检进入 release 或手动高成本回归层。
- 发布门禁覆盖正式产物矩阵、release metadata、checksum、packaged `/api/protocols/onebot11`、`/api/protocols/onebot11/compatibility`、模板预览工作区全流程、packaged recovery drill 和长期自托管 smoke。
- 高成本依赖审计和长时段巡检保留在 `nightly.yml` 或发布门禁，不挤占每个 PR 的默认门禁预算。

## 当前工作流矩阵

| 工作流 | 平台 | PR 门禁 | 说明 |
| --- | --- | --- | --- |
| `ci.yml` | 主 jobs 为 `ubuntu-latest`；`ci-self-check` 为 `ubuntu-latest` + `windows-latest` | 是 | 校验 contracts、Server、Web、Launcher、SDK、设计系统、notices、agent docs、CI 脚本与必需结果汇总 |
| `nightly.yml` | `ubuntu-latest` | 否 | 负责夜间长时段回归、Playwright E2E、依赖、安全和环境巡检 |
| `release.yml` | `windows-latest`、`ubuntu-latest`、`macos-26` | Tag 门禁 | 构建四种正式 artifact，校验 checksum、release metadata、协议读取接口、模板预览、recovery drill 与交付 smoke |
| `self-host-smoke.yml` | `windows-latest`、`ubuntu-latest`、`macos-26` | 否 | 对四种 artifact 运行长期自托管、诊断与恢复探针 |

## 验证原则

- 正式语义变化先更新契约；实现、测试、fixtures、examples、生成物和文档按实际影响同步。实现修复以现有契约为准，不要求无关文件制造 diff。
- 基线版本以工程文件和 `docs/engineering/baseline.md` 为准，CI 不单独维护另一套漂移版本号。
- 事件、插件协议、配置、错误码和迁移相关 Golden Fixtures 进入正式门禁，不只停留在文档说明。
- 轻量门禁负责可合并性，发布门禁负责可交付性。
- 恢复、运行环境准备和交付矩阵验证进入正式工作流，不只停留在文档说明。
