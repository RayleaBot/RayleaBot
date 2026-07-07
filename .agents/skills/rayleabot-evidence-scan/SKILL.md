---
name: rayleabot-evidence-scan
description: RayleaBot 任务需要"只看证据"式调查时使用，包括每日 bug 扫描、可度量的性能回归观察、近期 PR/review/commit 的 skill 推荐、automation memory 更新、Windows worktree 证据收集。优先采用具体仓库证据、实测产物、定向测试和本地 Git 回退，而不是猜测、按改动量下结论或泛泛建议。
---

# RayleaBot Evidence Scan

本 skill 维护 RayleaBot 周期性扫描的证据边界。仓库真相仍在根/局部 `AGENTS.md`、`contracts/`、正式文档、测试、fixtures 和当前 Git 状态中。

## 工作流

1. 读取根 `AGENTS.md` 和触及区域更近的局部 `AGENTS.md`。
2. 对任务分类：
   - `bug-scan`：审查近期 commit 或 diff，寻找具体 bug
   - `performance-watch`：检查可度量的性能回归
   - `skill-recommendation`：从近期工作中推荐下一个 skill 或学习方向
   - `automation-memory`：为 automation 结果更新 memory
3. 收集能回答任务的最小证据集：
   - 近期 commit、diff、变更文件与归属边界
   - 定向测试、构建、typecheck、CI 或 `git diff --check` 输出
   - contract、generated type、fixture、example 或文档漂移
   - benchmark、trace、profile、计时或 Web Vitals 产物
4. 只报告证据支持的结论。
5. 证据缺失时，简要说明不确定性，并指出能消除它的下一个测量或检查。

## Bug 扫描

- 只有存在具体证据时才报告 bug：失败的命令输出、编译/typecheck 错误、测试失败、无效 diff、contract 漂移或可直接追溯的逻辑回归。
- 被要求修复时，优先最小安全修复。
- 定向证据不支持 bug 时，以"未发现具体 bug"收尾。
- 不从改动量、文件数、commit 标题、宽泛风险或陌生代码推断 bug。

## 性能观察

- 视为测量任务，不是代码形态审查。
- 搜索实测产物：`Benchmark`、`benchstat`、`ns/op`、`allocs/op`、`trace`、`pprof`、`profile`、`flamegraph`、`Lighthouse`、`performance.mark`、`performance.measure`、`console.time`、`PerformanceObserver`、`Web Vitals`、CPU profile、heap profile、计时日志。
- 找不到实测产物时，写 `No measurements found`。
- 本地一次性 benchmark 只能描述为早期信号，除非它已是可比较的既定基线。
- 不把 diff 大小、UI 复杂度、commit 名称或单行修改换算成性能结论。

## Skill 推荐

- 每条推荐都锚定具体证据：PR 主题、review 评论、commit 聚类、触及路径、失败或重复修复。
- GitHub API 或 `gh pr list` 被 `connectex` 或网络策略阻断时，改用本地 Git 历史和触及文件聚类。
- 不给泛泛建议。每条推荐都必须点名重复出现的工作模式和能减少它的 skill。

## Automation Memory

- 使用显式路径 `<user-home>\.codex\automations\<automation-name>\memory.md`。
- 本 Windows 环境不依赖 `$CODEX_HOME` 或 `$env:CODEX_HOME`。
- memory 条目与面向用户的结论及其支撑证据保持一致。

## Windows Worktree

- 只读 Git 命令遇到 dubious ownership 时，使用 `git -c safe.directory=<repo> ...`。
- 不为临时扫描修改全局 Git 配置。
- 扫描 worktree 中已验证的修复要提交到 `main` 时，把无关的脏改动排除在 commit 之外。

## 输出

- 结论先行。
- 给出真实原因和验证结果。
- 性能测量缺失时，原样使用 `No measurements found`。
- 不包含推测、泛泛的风险话术或过程叙述。
