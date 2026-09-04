# AGENTS 与项目 Skill 优化执行记录

状态：已实施并完成审查与本地验证。日期：2026-09-04。实施基线：`HEAD 0d3082b3` 与开始实施时的工作区。
日志与运行时修复已记录在既有独立提交中；提交前审查基线为 `fbfa43a8`。本记录覆盖指令精简、配套门禁与审查修正。

## 完成范围

保留九份 AGENTS 与同级 CLAUDE bridge，精简其中八份指令；七个自建项目 skill 收敛为三个。检查脚本同步调整，现有 CI 继续使用原入口。

| 统计对象 | 实施前 | 实施后 |
| --- | --- | --- |
| 根 AGENTS | 102 行 | 50 行 |
| 九份 AGENTS 合计 | 487 行 | 203 行 |
| 自建项目 skill | 7 个，428 行 | 3 个，91 行 |
| AGENTS 与自建 SKILL.md 合计 | 915 行 | 294 行 |

上述内容减少 621 行，约 68%。统计用于本次核对，不作为新增 CI 指标，也不包含 bridge、impeccable 或本记录。

## 规则与入口

- [根规则](../AGENTS.md) 区分对外语义变更与实现修复，按实际影响同步配套内容；测试根据具体风险选择，允许包内测试验证真实行为。
- [Launcher 规则](../launcher/AGENTS.md) 明确 Go service/model → Wails bindings → renderer，保留必要的前端校验和展示适配模型；服务端 API 类型独立跟随 OpenAPI。
- [文档规则](./AGENTS.md) 按文档职责保留设计理由、数据流、依赖顺序与操作步骤；执行计划维护规则归到文档目录。
- 清除完整命令表、技能索引、配置键和能力清单、测试功能白名单、固定文案替换表及个人 shell 操作偏好。
- 契约优先和样例就绪表述同步到 [契约 README](../contracts/README.md)、[fixture README](../fixtures/README.md)、[实施顺序](./engineering/implementation-order.md)、[质量门禁](./engineering/quality-gates.md) 和 [PR 模板](../.github/PULL_REQUEST_TEMPLATE.md)。
- 插件 helper 的退出竞态说明保留在 [测试辅助函数](../server/internal/plugins/runtime/manager_helper_process_test.go) 旁，仅添加原因注释。

保留的自建 skill：

| Skill | 职责 |
| --- | --- |
| [contract-audit](../.agents/skills/contract-audit/SKILL.md) | 核对契约语义、实现偏差和受影响配套项，不维护独立契约清单 |
| [repo-validation](../.agents/skills/repo-validation/SKILL.md) | 根据行为、并发、依赖和生成输入选择验证，区分测试结果与构建产物 |
| [editing-final-state-content](../.agents/skills/editing-final-state-content/SKILL.md) | 按读者任务和文本职责编辑，保留准确术语与必要说明 |

已删除 `phase-boundary-check`、`agent-instruction-maintenance`、`glue-coding`、`rayleabot-evidence-scan` 四个项目目录，包括证据扫描 skill 的配套 agent 配置。活动指令、配置和代码中没有残留引用。

## 检查脚本

[check-agent-docs.mjs](../scripts/check-agent-docs.mjs) 保留行数、路径和疑似凭据检查，并完成以下调整：

- 从磁盘枚举 skill，取消根文件必须列出全部技能的要求。
- 检查所有层级的 AGENTS 是否有同级 bridge，并确认真实导入同级文件；代码示例、缩进示例和 HTML 注释中的文本不能代替导入。
- 疑似凭据告警只报告位置；检测出的候选值也从其他诊断中隐藏，避免路径错误提示再次带出候选值。

新增 [20 个回归用例](../scripts/tests/check-agent-docs.test.mjs)，通过临时 Git 仓库运行真实检查脚本。用例覆盖入口发现、bridge 缺失或无效导入、原有路径和行数限制、凭据报告与明确假值；其中 3 个用例验证注释示例不会吞掉真实导入，移除注释也不能拼出不存在的导入。对应修复前失败已复现，最终 20 个全部通过，无跳过。

新增测试被现有 CI 的 Node 测试 glob 包含；变更识别脚本对检查脚本和测试文件输出 `ci=true`。CI 配置和触发策略未调整，远端 CI 未运行。

## 语义验收

以下为针对最终规则文本的静态复核，不代表已经执行对应应用功能测试。

| 场景 | 复核结果 |
| --- | --- |
| HTTP handler 内部重排且正式语义相同 | 无需契约 diff，按实际风险验证 |
| 错误映射修复为契约已有错误码 | 修实现与必要回归，不改契约凑 diff |
| 新增响应字段、鉴权条件或状态含义 | 先明确并更新契约，再同步受影响内容 |
| 新契约编辑时尚未补齐 fixture | 可以继续编辑，合并前引用与必要样例须齐备 |
| 内部 helper 处理文件生命周期 | 允许包内测试创建、内容和清理结果 |
| 普通文案或样式微调 | 不默认新增字面值或样式测试 |
| 桌面桥增加本机字段 | 从 Go 生成 bindings，检查前端适配，不自动扩大 Server 契约 |
| Launcher 原生进程或平台集成变化 | 选择必要 Go、平台或构建验证 |
| 并发、包依赖或生成输入变化 | 分别选择相关 race、架构或生成漂移检查 |
| 架构文档或计划说明执行顺序 | 保留履行文档职责所需的说明 |
| 契约数量变化或某契约移除 | 更新正式来源与真实依赖者，skill 无数量或枚举副本 |

Server 状态归属、renderer 权限、凭据保护、共享状态并发保护、Launcher 独立 Go module 与平台参数、逻辑提交原则均保留。归档规则及其历史内容保持原状。

## 验证结果与边界

- `node --test scripts/tests/check-agent-docs.test.mjs`：20/20 通过。
- `node scripts/check-agent-docs.mjs`：通过。
- `python scripts/check-doc-links.py`：通过，检查 134 份 Markdown。
- 三个保留 skill 均通过 skill-creator 提供的 frontmatter、命名与占位内容校验。
- 本轮已跟踪文件的 `git diff --check` 通过；新增文件补充 UTF-8 与空白检查。
- 与实施前逐文件 SHA256 基线比较，范围外仓库文件内容未改变。所有 bridge、impeccable 与归档历史保持一致，原有日志和插件运行时工作已在既有独立提交中保留。
- 提交前验证使用 Node `26.7.0`、Python `3.14.7`，`python scripts/check-toolchain.py` 通过；本轮未运行远端 CI、应用浏览器测试、race 或原生打包。
- 本次应用代码变化仅为 helper 注释；应用运行时行为和正式 schema 未改动。
