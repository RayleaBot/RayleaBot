# Quality Gates

本页说明 RayleaBot 当前正式采用的验证入口、CI 门禁和发布回归层次。

## 默认验证命令

### Server

- `go test ./...`
- `mkdir -p dist && go build -o "dist/raylea-server$(go env GOEXE)" ./cmd/raylea-server`

### Web

- `pnpm install --frozen-lockfile`
- `pnpm test`
- `pnpm build`
- `pnpm test:e2e`

### Launcher

- `pnpm install --frozen-lockfile`
- `pnpm test`
- `pnpm build`

## CI 工作流

| 工作流 | 主要职责 |
| --- | --- |
| `ci.yml` | 变更范围识别、strict contracts 校验、Server/Web/Launcher/SDK 核心检查、OpenAPI 生成类型漂移检查和必需结果汇总 |
| `nightly.yml` | 夜间长时段回归、依赖和运行环境巡检 |
| `release.yml` | 正式产物打包、metadata 校验、packaged 协议与模板 smoke、跨版本 recovery drill、长期自托管 smoke |
| `self-host-smoke.yml` | 按 artifact 子集复用正式打包路径，长期巡检 packaged 协议与模板 smoke、自托管运行、诊断与恢复闭环 |

## 当前门禁层次

- PR 默认门禁覆盖 strict contracts、Server 核心检查、Web 核心检查、Launcher 核心检查、Node / Python SDK 回归和必需结果汇总。
- `contracts/**`、`fixtures/**`、`examples/**` 与 `sdk/**` 变更会触发 `ci.yml` 对应 job，同步执行 Web 与 Launcher 的 OpenAPI 生成类型漂移检查。
- Playwright E2E、Chromium 重渲染 golden、跨版本恢复和更长时长自托管巡检进入 release 或手动高成本回归层。
- 发布门禁覆盖正式产物矩阵、release metadata、checksum、packaged `/api/protocols/onebot11`、`/api/protocols/onebot11/compatibility`、模板预览工作区闭环、packaged recovery drill 和长期自托管 smoke。
- 高成本依赖审计和长时段巡检保留在 `nightly.yml` 或发布门禁，不挤占每个 PR 的默认门禁预算。

## 当前工作流矩阵

| 工作流 | 平台 | PR 门禁 | 说明 |
| --- | --- | --- | --- |
| `ci.yml` | `ubuntu-x64` | 是 | 校验 strict contracts、Server、Web、Launcher、Node / Python SDK、OpenAPI 生成类型漂移和必需结果汇总 |
| `nightly.yml` | `ubuntu-x64` | 否 | 负责夜间长时段回归、依赖巡检和环境巡检 |
| `release.yml` | `windows-x64-full`、`linux-x64-full`、`macos-arm64-full`、`linux-x64-server` | Tag 门禁 | 负责正式打包、checksum、release metadata、协议读取面、兼容矩阵、模板预览工作区、recovery drill 与交付 smoke |
| `self-host-smoke.yml` | `windows-x64-full`、`linux-x64-full`、`macos-arm64-full`、`linux-x64-server` | 否 | 负责长期自托管巡检，并复用协议读取面、兼容矩阵、模板预览工作区、诊断与恢复探针 |

## 验证原则

- 契约、测试、实现和示例同步更新。
- 基线版本以工程文件和 `docs/engineering/baseline.md` 为准，CI 不单独维护另一套漂移版本号。
- 事件、插件协议、配置、错误码和迁移相关 Golden Fixtures 进入正式门禁，不只停留在文档说明。
- 轻量门禁负责可合并性，发布门禁负责可交付性。
- 恢复、运行环境准备和交付矩阵验证进入正式工作流，不只停留在文档说明。
