# Engineering Docs

本目录用于承载 RayleaBot 的工程治理内容，固定版本线、默认命令、仓库边界与实施顺序。

## 当前工程状态

- `server/` 已进入真实主链路开发阶段，当前覆盖配置、存储、鉴权、任务、插件发现、OneBot11 adapter、多插件 runtime、dispatcher、scheduler trigger 与管理面日志持久化
- `web/` 与 `launcher/` 已锁定工具链和默认命令，当前仍是工程占位
- `.deps/manifest.json` 已形成资源清单骨架，来源与 SHA256 仍待补齐

## 文档分工

- `baseline.md`：版本线、默认命令、目录职责、冻结选型
- `implementation-order.md`：长期有效的阶段边界与进入条件
- `../execution-plan.md`：当前进度与下一步行动记录
- `../../contracts/README.md`：formal contracts 与 contract 级 TODO 概览

## 维护规则

- 对外接口裁决不在本目录，而在 `contracts/`。
- 本目录用于固定工程实现边界、命令入口和协作规则，不替代执行计划。
- 任何基线变更都必须同步更新对应工程文件与 CI，而不是只改文档。
