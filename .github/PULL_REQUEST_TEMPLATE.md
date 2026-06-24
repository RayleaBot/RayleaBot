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

- [ ] 已运行与改动面直接相关的最小验证（build / test / typecheck / lint）
- [ ] 若改动 `AGENTS.md` / `CLAUDE.md` / `.agents/skills/`，已运行 `node scripts/check-agent-docs.mjs`
- [ ] 若改动 `contracts/`（OpenAPI / WebSocket / schema），已同步检查并更新 `generated types` / `fixtures` / `examples`
- [ ] 若改动对外接口或状态模型，已同步更新相关 tests 与 docs

## 备注

<!-- 补充说明：迁移影响、破坏性变更、待办事项等。 -->
