---
name: editing-final-state-content
description: Use when editing documentation, code comments, or user-facing copy in this repo to keep text in final-state form and remove change-trace wording, stage labels, and unnecessary design, technical, or intent explanations.
---

# Editing Final-State Content

This skill is a reusable workflow. Repository truth still lives in root/local `AGENTS.md`, `contracts/`, and the formal docs they reference.

## Use This Skill When

- 任务会修改 `docs/`、`AGENTS.md`、README、代码注释、错误文案、按钮标签、空态文案、提示文案或其他用户可见文本
- 你准备解释界面行为、接口消费顺序、内部实现顺序、数据流拼接顺序或设计用意
- 你不确定一段文字是否泄露了编辑过程、实现过程或阶段性口径

## Workflow

1. 先读根 `AGENTS.md` 中“文档、注释与面向用户显示内容修改规则”。
2. 只写最终态成稿，让文本看起来从一开始就是当前版本。
3. 只保留读者完成当前任务必须知道的事实、约束、操作和结果。
4. 删除编辑痕迹、迁移叙事、阶段标签、实现顺序说明和无直接用途的设计说明。
5. 提交前逐段重读，确认每句话都服务于读者当前要看到的最终内容。

## Keep

- 稳定事实：能力、限制、参数、路径、前置条件、结果
- 对读者有直接作用的操作说明
- 为避免误用或歧义而必须保留的最小技术事实

## Remove

- 编辑痕迹：`不再`、`已改为`、`已移到`、`前后对比`、`不会再`、`这里改成`、`原来`、`之前`、`现改为`
- 开发态标签：`阶段1`、`第一步`、`phase 1` 等阶段或步骤叙事，除非当前文档本身就是变更记录、迁移说明或发布说明
- 不必要的设计说明、技术说明、用意说明
- 通过“先……再……”解释实现顺序、拉流顺序或接口拼接顺序的句子，除非缺少它会直接导致使用错误

## Rewrite Direction

- 把“如何改过来的”改成“现在是什么”
- 把“内部怎么串起来”改成“读者要做什么”或“读者会看到什么”
- 把“作者为什么这样设计”改成“读者必须知道的约束”

## Red Flags

- 句子在解释修改过程，而不是交付内容
- 句子在解释内部实现顺序，而不是外部行为
- 句子删掉之后，读者依然能正确使用内容
- 句子读起来像评审说明、提交说明或口头补充

## Bad Patterns

- `列表先读 HTTP，再吃 \`/ws/tasks\` 增量更新。`
- `先回放 \`/api/logs\`，再接 \`/ws/logs\` 追加。`
- `这里改成统一入口。`
- `之前使用旧字段。`

## Final Check

- 扫描禁用措辞与阶段词
- 逐段删除对最终读者没有直接价值的解释
- 仅在 changelog、migration guide、release notes 中保留变更叙事
