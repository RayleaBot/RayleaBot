# CLAUDE.md

@AGENTS.md

## Claude Code Notes

- `CLAUDE.md` 是 Claude Code 在本仓库中的入口说明文件；共享的仓库级规则统一维护在 `AGENTS.md`。
- 需要对所有代理生效的长期规则，优先写入 `AGENTS.md`；仅对 Claude Code 有意义的补充说明保留在此文件。
- 保持本文件简洁、具体、可验证。若 Claude 专属说明按主题或路径继续增长，优先拆分到 `.claude/rules/`，不要在这里复制整份仓库规则。

## Quick Project Context

- RayleaBot 是一个面向个人开发者和 GitHub 开源协作者的自托管聊天机器人框架。
- 仓库主要组成：
  - `server/`：Go 服务端、管理接口、插件运行时与存储层
  - `web/`：Web UI
  - `launcher/`：Electron 桌面启动器
- 正式来源和深度参考：
  - `contracts/`
  - `docs/engineering/baseline.md`
  - `docs/engineering/implementation-order.md`
  - `docs/architecture/README.md`

## Architecture Snapshot

- OneBot11 事件主链路：`adapter -> bridge -> dispatch -> plugin runtimes`
- 插件以子进程形式运行，通过 stdin/stdout 上的 JSONL 协议与平台通信。
- 持久化存储使用 SQLite，当前基线 schema 位于 `server/internal/storage/schema.sql`。
- 启动器是本地 Electron 壳层，不承担 Web 业务逻辑副本。

## Useful Areas

- 服务端内部实现：`server/internal/`
- 正式接口、错误码、发布元数据：`contracts/`
- 插件文档与 SDK：`docs/plugin/`
- 发布脚本与归档流程：`scripts/release/`

## Default Commands

- Windows shell
  - `C:\Program Files\Git\usr\bin\bash.exe --noprofile --norc -lc '<command>'`
  - Prefix Unix-tool commands with `export PATH="/usr/bin:/bin:$PATH";`
- Server
  - `go build ./cmd/raylea-server`
  - `go test ./...`
- Web
  - `pnpm install --frozen-lockfile`
  - `pnpm test`
  - `pnpm build`
- Launcher
  - `pnpm install --frozen-lockfile`
  - `pnpm test`
  - `pnpm build`

## Debugging Methodology

When a build or run command succeeds (exit 0) but the expected output is missing or the app does not start:

1. **Check actual runtime state, not just exit code** — A command may exit 0 while the underlying tool silently failed or skipped work. For example, `pnpm install --frozen-lockfile` with `ignoredBuildScripts` skips electron's install script, causing a missing binary — but still exits 0.
2. **Verify the artifact exists before assuming the command worked** — Check `node_modules/.electron/`, `dist/`, `app.asar`, or whatever the expected output is immediately after the command returns.
3. **Diagnose the package manager's behavior first** — pnpm 10 skips `install` scripts by default unless `pnpm.onlyBuiltDependencies` is configured. npm and yarn have different defaults. Know which one applies.
4. **Isolate the exact step that fails** — Run each build step (compile, bundle, package) separately instead of running a combined script, so you know which step is actually stuck.

Root-cause first, not symptom-first. Fixing a symptom while the real cause silently fails in the background wastes everyone time.
