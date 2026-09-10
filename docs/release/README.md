# Release Docs

本目录说明 RayleaBot 的产物矩阵、发布签名与信任、升级安装、回滚和验收条件。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [Release Notes](./notes/README.md) | 中文版本说明的编写、检查和 GitHub Release 发布流程 |
| [Release Notes Template](./release-notes-template.md) | 功能版与补丁版说明模板、栏目和填写规则 |
| [Delivery and Upgrade](./delivery-and-upgrade.md) | 产物矩阵、manifest v2、Ed25519、Authenticode、自动检查、事务安装与 guided update |
| [Acceptance and Risks](./acceptance-and-risks.md) | 风险控制、发布门禁、故障注入和真实签名 Windows E2E |
| [Plugin Protocol v3 Upgrade](./plugin-protocol-v3-upgrade.md) | 多适配器身份快照、SDK 重建与备份协议边界 |
| [Plugin Contract v3](./plugin-contract-v3-upgrade.md) | 本版插件合同、原生 artifact、安装与恢复边界 |
| [Windows Desktop Runtime](./windows-desktop-runtime.md) | Windows WebView2 运行时要求与联网、离线安装指引 |
| [Linux Desktop Runtime](./linux-desktop-runtime.md) | Linux GTK 3、WebKitGTK 运行时要求与发行版安装命令 |

## 正式来源

- 发布元数据字段与限制：[`contracts/release-manifest.schema.json`](../../contracts/release-manifest.schema.json)
- 更新状态、阶段与 Web API：[`contracts/web-api.openapi.yaml`](../../contracts/web-api.openapi.yaml)
- CLI 更新入口：[`contracts/cli-commands.yaml`](../../contracts/cli-commands.yaml)
- 固定工具链与发布基线：[`docs/engineering/baseline.md`](../engineering/baseline.md)

在正式 Authenticode 证书和真实签名 Windows packaged E2E 通过之前，`windows-x64-full` 的更新方式固定为 `guided`（引导更新）。

## 历史排障记录

[2026-09-04 日志运行修复](./log-runtime-repair-2026-09-04.md) 保留当时的问题、处理与验证边界，不作为当前操作流程或本轮验收证据。
