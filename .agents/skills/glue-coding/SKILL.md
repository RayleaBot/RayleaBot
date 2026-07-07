---
name: glue-coding
description: 在 server、web、launcher、storage、render 或依赖选型上规划、编写 RayleaBot 实现改动时使用，尤其是在决定是否引入新库、新框架、新服务或跨面模式时。优先复用仓库现有代码、冻结技术栈、标准库和薄胶水层，而不是平行栈或重新造轮子。
---

# Glue Coding

本 skill 是可复用工作流，不定义项目真相。仓库真相仍在根/局部 `AGENTS.md`、`contracts/` 及其引用的工程文档中。

## 工作流

1. 读取根 `AGENTS.md` 和更近的局部 `AGENTS.md`。
2. 在选择框架、库或实现形态前，先读 `docs/engineering/baseline.md`。
3. 若任务触及对外边界，同时读取 `contracts/`、`fixtures/`、`examples/` 中的相关文件。
4. 在设计新的 helper、抽象、wrapper 或依赖之前，先在仓库内搜索既有先例。
5. Go 服务端代码把目录当作真实包边界；同一生命周期、同一调用路径、共享状态或测试的代码优先用包内文件分组。
6. 选择能安全解决任务的最低复用层级。
7. 自写代码保持薄，只做编排、适配、contract 投影、数据转换和仓库特有业务规则。
8. 总结中列出复用的既有构件，并说明无法避免的自写胶水部分。

## 复用层级

按以下顺序选择：

1. 仓库现有代码与冻结技术栈
2. 标准库或平台内建能力
3. 官方 SDK 或仓库已冻结的上游依赖
4. 成熟、经生产验证、依赖面最小的 OSS
5. 薄自写胶水

在上一层被证明不足之前，不得跳到下一层。

## 复用锚点

- Server：从 `server/internal/*` 出发，尤其是既有 repository、service、HTTP handler、runtime、adapter、scheduler、storage、logging 包。
- Server 订阅/广播：一律复用 `server/internal/pubsub` 的泛型 Hub，不手写订阅表。
- Server 投影：领域视图只在领域包构建一次（如 `plugins.BuildSummaryView`）；`management` 层只做序列化标注，不复制投影逻辑。
- Web：从 `web/src/lib/http.ts`、`web/src/lib/ws.ts`、`web/src/stores/*`、`web/src/components/*` 和既有页面模式出发。
- Launcher：从 `launcher/src/main/services/*`、`launcher/src/shared/*` 和现有 Electron `main` / `preload` / `renderer` 分层出发。
- Contracts 与示例：`contracts/`、`fixtures/`、`examples/` 是冻结结构、示例 payload 和回归锚点的首选来源。

## 依赖门禁

引入新依赖前，逐项确认：

- 仓库中不存在合适的既有实现或冻结栈选择。
- 标准库或平台能力不足以完成任务。
- 候选依赖是官方或维护良好的项目。
- License 明确且可接受。
- 项目有真实生产采用或强维护信号。
- 引入面窄，不与仓库文档已冻结的栈重复。
- 版本可通过既有 lockfile、manifest 或对应面的工程文件固定。

任一项不满足时，回到更高复用层级或写最小胶水代码。

## 注入与状态

- 测试替身通过构造期注入（如 `app.Options`），不为测试在服务上加运行期 setter。
- 运行期可变共享状态必须原子快照或锁保护；热更新写路径不得与读路径共享裸字段。
- 必选依赖缺失在装配层 fail-fast；不在业务方法里加接收者 nil 样板或静默降级。

## 禁止

- 在未证明冻结选择不足之前，不引入第二套 router、ORM、日志栈、状态管理、HTTP client、WebSocket client、UI 组件体系或 launcher 服务层。
- 黑盒集成可行时，不 fork、不 vendor、不 patch 上游库。
- 直接对接当前仓库模式已够用时，不造通用的"面向未来"抽象。
- 除非能消除循环依赖、保护外部边界或形成已验证的复用边界，不把同一领域的服务端 helper 拆成薄 `model`、`spec`、`process`、`protocol`、`repository` 包。
- 实际是在重造成熟轮子的手写代码，不得描述为胶水。
