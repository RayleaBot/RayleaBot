# RayleaBot Repository Guide

RayleaBot 是包含 `server/`、`web/`、`launcher/` 和 `contracts/` 的自托管聊天机器人框架。
进入具体目录后读取就近的 `AGENTS.md`。

## Instruction Scope

- 用户当前请求定义目标和交付范围。
- 根到局部的 AGENTS 逐层叠加；局部规则补充或收窄根规则。
- 每份 AGENTS 由同级 `CLAUDE.md` 导入，Claude 专属说明留在 bridge 中。
- 项目自有 skill 位于 `.agents/skills/`，按任务需要使用；外部安装的 skill（如 Impeccable）由上游维护，不纳入项目修改。

## Hard Rules

- `contracts/` 定义对外正式语义；实现、生成物、README、fixtures 和 examples 不得反向覆盖它。
- 新增或改变对外接口、协议、schema、状态、错误码、事件、CLI 或发布元数据的正式语义时，先更新对应契约，再同步受影响实现。修复实现以符合现有契约时，直接修实现并按风险验证。
- 按实际影响更新实现、测试、fixtures、examples、生成物和文档，不要求无关文件制造 diff。契约变更合并前，引用的样例和必要验证必须齐备。
- 优先搜索现有实现并复用；新依赖须说明必要性，不引入平行技术栈。不升级冻结版本线，除非任务明确要求并同步 baseline、工程文件、lockfile、CI 与发布说明。
- 不在配置响应、fixtures、examples、日志、文档或测试快照中暴露真实凭据。
- 可能并发读写的共享可变状态必须由原子快照或锁保护。
- 完成前运行能证明本次改动正确性的最小验证；生成、构建和运行任务还需确认预期产物或效果。

## Source of Truth

- 对外接口与协议：`contracts/README.md` 及对应契约。
- 工具链、固定版本线与默认命令：`docs/engineering/baseline.md` 和对应工程脚本。
- 长期依赖顺序与跨层边界：`docs/engineering/implementation-order.md`、`docs/architecture/`。
- 产品目标与范围：`docs/RayleaBot机器人项目规划.md`。
- 用户操作与管理面：`docs/user/`。

## Working Entrypoints

- 服务端改动先读 `server/README.md`；Web 与 Launcher 改动先读各自 `package.json`，Launcher 还需读 `launcher/go.mod`。
- 开发启动与工作区说明：`docs/dev/repo-workflow.md`。
- 设计工具共享上下文：指定应用目标前，按 `docs/design/README.md` 设置 `IMPECCABLE_CONTEXT_DIR`。
- 验证选择：`.agents/skills/repo-validation/SKILL.md`；既有 CI 与发布门禁见 `docs/engineering/quality-gates.md`。

## Testing

- 测试对应可说明的业务、边界、错误处理、并发、兼容或历史回归风险；普通文案、样式微调、纯重命名和等价搬移不默认新增测试。
- 在能可靠覆盖风险的最小层次断言可观察结果。允许包内测试验证内部函数的行为，避免绑定实现步骤、普通文案、无关框架行为或不稳定输入；字面值承载契约或关键分支语义时可直接断言。

## Instruction Maintenance

- 同一规则在最接近责任方的位置维护；改动指令时复核重复、冲突、引用和检查脚本结果。
- 重复问题优先修代码、测试或就近说明；只有跨任务反复出现且无法直接发现的稳定约束才进入 AGENTS。原因消失或已有可靠实现约束时，删除对应规则。

## Git and Review

- 提交遵守 Conventional Commits：`<type>[optional scope]: <description>`，subject 说明具体变更。
- 每次提交必须撰写非空正文，与 subject 之间空一行；正文直接说明改动内容与原因，不写验证过程或结果。
- 一个 commit 表达一个逻辑变更；保留无关工作区改动，只暂存核对过的相关文件或 hunks。
