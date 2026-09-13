# CLAUDE.md

@AGENTS.md

## Claude Code Notes

- 共享仓库规则维护在 `AGENTS.md`。
- 子目录规则通过同级 `CLAUDE.md` bridge 导入对应 `AGENTS.md`。
- Claude 专属、长期稳定且不可放入通用 `AGENTS.md` 的规则才写在本文件。
- Claude Code 不自动加载 `.agents/skills/` 中的项目 skill；任务符合某个 skill 的描述时，直接读取对应 `SKILL.md` 并按其执行。
