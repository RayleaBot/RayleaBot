---
name: repo-validation
description: 本仓库改动需要验证时使用。按受影响面选择最小命令集并运行，检查 contract/类型/fixture 漂移或缺失的生成物。
---

# Repo Validation

本 skill 是可复用工作流，不定义项目真相。仓库真相仍在 `contracts/`、根/局部 `AGENTS.md` 及其引用的工程文档中。

## 适用场景

- 完成代码、contract、fixture 或文档修改后需要验证
- 不确定该运行哪些测试、类型检查或构建命令
- 需要确认生成物（generated types、sqlc 输出等）与 contract 一致
- 需要检查 fixture 或 example 是否漂移出 contract 定义

## 工作流

1. 读取根 `AGENTS.md` 和受影响目录的局部 `AGENTS.md`。
2. 识别改动面：
   - `server/` → Go build + 受影响包的 Go test；外部行为或共享路径变化再扩到 `go test ./...`
   - `server/` 触及并发路径（配置热更新、订阅广播、共享状态、goroutine 生命周期）→ 对相关包追加 `go test -race`
   - `server/` 触及包结构、包搬移或跨包依赖 → 确认 `server/tests/architecture` 仍通过
   - `web/` → pnpm typecheck + pnpm test + pnpm build
   - `launcher/` → pnpm typecheck + pnpm test + pnpm build
   - `contracts/` → 检查对应 generated types（`web/src/types/generated.ts`、`web/src/types/websocket.generated.ts`、`launcher/src/shared/web-api.generated.ts`）
   - `sdk/` → SDK build + SDK test
   - `scripts/` → 脚本自测或相关 CI 验证
3. 运行最小命令集：
   - 只在受影响的子工程目录执行对应命令。
   - 不运行与本次改动无关的全量测试。
   - 纯搬移、包合并、等价改名或普通文案调整用构建和现有相关测试证明，不新增测试。
4. 检查生成物：
   - 若 contract 变更，确认 generated types 已重新生成且一致。
   - 若 SQL 变更，确认 `sqlc diff` 无漂移。
   - 若 fixture/example 变更，确认它们仍只表达已冻结结构。
5. 检查产物存在性：命令 exit 0 不足以证明成功；确认目标产物（dist、app.asar、node_modules/.electron/ 等）真实存在。
6. 输出验证摘要。

## 输出

- 本次改动的最小验证命令清单
- 生成物一致性结论
- fixture / example / contract 漂移结论
- 缺失产物或异常退出项清单

## 禁止

- 运行与改动无关的全量测试或构建以“保险起见”。
- 把普通文案、文件搬迁或等价结构收拢误判成必须新增测试的行为变化。
- 仅凭 exit code 0 就认定验证通过；必须确认产物存在。
- 跳过 generated types、fixtures 或 sqlc 生成物的同步检查。
- 把验证流程写成 `AGENTS.md` 中的长命令列表；验证策略应沉淀到本 skill，根文件只保留最小命令索引。
