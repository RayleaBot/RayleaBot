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
| `nightly.yml` | strict contracts、覆盖率、开发与生产构建的 Web Playwright E2E、安全、依赖、运行环境和 release dry-run 巡检 |
| `release.yml` | 正式产物打包、metadata 校验、packaged 协议与模板 smoke、本版 recovery drill、长期自托管 smoke |
| `self-host-smoke.yml` | 按 artifact 子集复用正式打包路径，长期巡检 packaged 协议与模板 smoke、自托管运行、诊断与恢复全流程 |

## 当前门禁层次

- PR 默认门禁覆盖 contracts-lite、Server 测试/构建/核心 lint、关键并发包 race、Windows 安装、锁与浏览器进程回归、Web 与 Launcher typecheck/test/build、Go/Vue 插件 SDK、示例、design-system、third-party-notices、agent-docs、CI 自检和必需结果汇总。
- `contracts/**`、`fixtures/**`、`examples/**`、`sdk/**` 与 `plugins/**` 变更会触发 `ci.yml` 对应 job，同步执行 Web 与 Launcher 的 OpenAPI 生成类型漂移检查。
- Web 与 Launcher Renderer 的 Playwright E2E 由 `nightly.yml` 自动执行；本版恢复和更长时长自托管巡检进入 release 或手动高成本回归层。
- 发布门禁覆盖正式产物矩阵、release metadata、checksum、packaged `/api/adapters`、`/api/protocols/onebot11/compatibility`、模板预览工作区全流程、packaged recovery drill 和长期自托管 smoke。
- 高成本依赖审计和长时段巡检保留在 `nightly.yml` 或发布门禁，不挤占每个 PR 的默认门禁预算。

## 当前工作流矩阵

| 工作流 | 平台 | PR 门禁 | 说明 |
| --- | --- | --- | --- |
| `ci.yml` | 主 jobs 为 `ubuntu-latest`；`server-windows` 为 `windows-latest`；`ci-self-check` 为两平台 | 是 | 校验 contracts、Server（含核心 lint/race 与 Windows 回归）、Web、Launcher、SDK、设计系统、notices、agent docs、CI 脚本与必需结果汇总 |
| `nightly.yml` | `ubuntu-latest` | 否 | 负责夜间长时段回归、Playwright E2E、依赖、安全和环境巡检 |
| `release.yml` | `windows-latest`、`ubuntu-latest`、`macos-26` | Tag 门禁 | 构建四种正式 artifact，校验 checksum、release metadata、协议读取接口、模板预览、recovery drill 与交付 smoke |
| `self-host-smoke.yml` | `windows-latest`、`ubuntu-latest`、`macos-26` | 否 | 对四种 artifact 运行长期自托管、诊断与恢复探针 |

Nightly 的 Server 测试一次运行同时启用 race 和 atomic coverage，覆盖全部 Go 包。SQL 例外的复审日期到期产生维护提示；登记缺失、字段无效、文件不存在或与实际 SQL 使用不符仍阻止结构检查。

PR 的关键并发包 race 覆盖 App、配置应用、事件管线、插件 Catalog/Runtime/Lifecycle、广播、协议事件、OneBot 回调和存储快照；完整包清单由 `ci.yml` 维护。Server、契约或 CI 规则变化触发服务端门禁；`go.work.sum` 触发工作区相关消费者，SQL 例外登记变化触发 Server 与 CI 自检。跨目录重命名同时按来源和目标路径识别影响范围。

Web 生产构建 E2E 分为 `real-server` 与 `plugin-ui-fixtures`。真实 Server 用例独立使用临时目录、SQLite 和动态端口，覆盖静态路由、鉴权、配置与插件全局设置、治理作用域、调度列表和日志详情。模拟入口保留受控网络、iframe、消息节奏与展示数据；配置应用策略不在 JS 中重算。覆盖范围和运行方式见 [Web 端到端验证](./web-testing.md)。`RAYLEA_E2E_WEB_PORT` 可隔离开发模式的 Web 端口。

Nightly 的 `release-dry-run` 在构建 Server 后执行 `python scripts/release/rehearse_current_recovery.py --server dist/server/raylea-server --output dist/current-recovery-rehearsal`。输出目录必须不存在，保存合成数据、恢复包、进程日志和结果 JSON；验证空目录初始化、当前格式备份、恢复到空目录、登录、配置与插件数据一致性，以及重复启动幂等。

## 验证原则

- 正式语义变化先更新契约；实现、测试、fixtures、examples、生成物和文档按实际影响同步。实现修复以现有契约为准，不要求无关文件制造 diff。
- 工具链版本值由 `.tool-versions` 提供，CI 通过共享 action 读取后安装。doctor 校验生态工程文件，契约门禁核对基线文档；这两类必要副本发生漂移时失败。
- 事件、插件协议、配置、错误码和初始化相关 Golden Fixtures 进入正式门禁，不只停留在文档说明。
- 轻量门禁负责可合并性，发布门禁负责可交付性。
- 恢复、运行环境准备和交付矩阵验证进入正式工作流，不只停留在文档说明。
