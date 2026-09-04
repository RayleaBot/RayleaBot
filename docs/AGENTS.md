# Docs Agent Guide

先遵守根 `AGENTS.md`，文本编辑使用 `.agents/skills/editing-final-state-content/SKILL.md`。

## Documentation

- 文档按所属正式来源说明能力、限制、架构与操作，不定义另一套对外接口。
- 保留读者需要的设计理由、前置条件、数据流、依赖顺序和步骤，删除无关编辑过程叙述。
- 契约或正式行为变化影响文档时，同轮更新对应说明；文档与契约冲突时以契约为准。
- 执行计划按需建立为 `docs/execution-plan-v*.md`，按用户指定或当前执行中的计划工作；版本发布后整理为 `docs/CHANGELOGS/` 条目。没有执行计划时使用契约与现行文档。
- 使用大小写正确的仓库相对链接；中文文件通过 README 或所属目录索引提供入口。
