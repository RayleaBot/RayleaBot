# Pull Request Template

## 改动摘要

<!-- 简要描述本次改动的目的、范围和关键变更。 -->

## 改动范围

请勾选本次 PR 涉及的范围：

- [ ] 改动 `AGENTS.md` / `CLAUDE.md` / `.agents/skills/`
- [ ] 改动 `contracts/` / schemas / OpenAPI / WebSocket events
- [ ] 改动 `server/` / `web/` / `launcher/` 实现代码
- [ ] 其他（请说明）：

## 验证检查

请确认已完成以下验证：

- [ ] 已按 `docs/engineering/quality-gates.md` 运行与改动面对应的最小验证
- [ ] 若改动 `AGENTS.md` / `CLAUDE.md` / `.agents/skills/`，已运行 `node scripts/check-agent-docs.mjs`
- [ ] 若对外正式语义变化，已先更新对应 contract，并补齐受影响的样例与生成物

## 备注

<!-- 补充说明：迁移影响、破坏性变更、待办事项等。 -->
