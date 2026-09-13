# Release Docs

本目录说明 RayleaBot 的产物矩阵、更新检查、手动更新和验收条件。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [Release Notes](./notes/README.md) | 中文版本说明的编写、检查和 GitHub Release 发布流程 |
| [Release Notes Template](./release-notes-template.md) | 功能版与补丁版说明模板、栏目和填写规则 |
| [Delivery and Upgrade](./delivery-and-upgrade.md) | 产物矩阵、发布元数据、版本检查与手动更新 |
| [Acceptance and Risks](./acceptance-and-risks.md) | 风险控制、发布门禁和备份恢复验收 |
| [Windows Desktop Runtime](./windows-desktop-runtime.md) | Windows WebView2 运行时要求与联网、离线安装指引 |
| [Linux Desktop Runtime](./linux-desktop-runtime.md) | Linux GTK 3、WebKitGTK 运行时要求与发行版安装命令 |

## 正式来源

- 发布元数据字段与限制：[`contracts/release-manifest.schema.json`](../../contracts/release-manifest.schema.json)
- 更新状态与 Web API：[`contracts/web-api.openapi.yaml`](../../contracts/web-api.openapi.yaml)
- CLI 更新入口：[`contracts/cli-commands.yaml`](../../contracts/cli-commands.yaml)
- 固定工具链与发布基线：[`docs/engineering/baseline.md`](../engineering/baseline.md)
