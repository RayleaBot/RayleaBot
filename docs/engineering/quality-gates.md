# Quality Gates

本页说明 RayleaBot 当前正式采用的验证入口、CI 门禁和发布回归层次。

## 默认验证命令

### Server

- `go test ./...`
- `go build ./cmd/raylea-server`

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
| `contracts.yml` | baseline 校验、formal contracts 校验、fixture 引用与 frozen set 校验 |
| `lint.yml` | Server、Web、Launcher Linux 核心链路、PR smoke 与手动依赖审计 job |
| `race.yml` | 手动 Go race 回归检查 |
| `release.yml` | 正式产物打包、metadata 校验、smoke、跨版本 recovery drill、长期自托管 smoke |
| `self-host-smoke.yml` | 按 artifact 子集复用正式打包路径做长期巡检 |

## 当前门禁层次

- PR 默认门禁覆盖 contracts、baseline、Server 核心检查、Web 核心检查、Launcher Linux 核心检查和轻量 smoke。
- Playwright E2E、Chromium 重渲染 golden、跨版本恢复和更长时长自托管巡检进入 release 或手动高成本回归层。
- 发布门禁覆盖正式产物矩阵、release metadata、checksum、packaged recovery drill 和长期自托管 smoke。
- 高成本依赖审计保留为 `lint.yml` 中的手动 job，不挤占每个 PR 的默认门禁预算。

## 当前工作流矩阵

| 工作流 | 平台 | PR 门禁 | 说明 |
| --- | --- | --- | --- |
| `contracts.yml` | `ubuntu-x64` | 是 | 校验 baseline、contracts、schema 与 fixtures 同步性 |
| `lint.yml` | `ubuntu-x64` | 是 | 负责 Server、Web、Launcher Linux 核心检查与 PR smoke，手动触发时也承载依赖审计 |
| `race.yml` | `ubuntu-x64` | 否 | 作为按需 Go race 回归入口 |
| `release.yml` | `windows-x64`、`linux-x64`、`macos-arm64` | Tag 门禁 | 负责正式打包、checksum、release metadata 与交付 smoke |
| `self-host-smoke.yml` | `windows-x64`、`linux-x64`、`macos-arm64` | 否 | 负责长期自托管巡检 |

## 验证原则

- 契约、测试、实现和示例同步更新。
- 基线版本以工程文件和 `docs/engineering/baseline.md` 为准，CI 不单独维护另一套漂移版本号。
- 事件、插件协议、配置、错误码和迁移相关 Golden Fixtures 进入正式门禁，不只停留在文档说明。
- 轻量门禁负责可合并性，发布门禁负责可交付性。
- 恢复、运行环境准备和交付矩阵验证进入正式工作流，不只停留在文档说明。
