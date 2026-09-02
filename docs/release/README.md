# Release Docs

本目录说明 RayleaBot 的产物矩阵、发布签名与信任、升级安装、回滚和验收条件。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [Delivery and Upgrade](./delivery-and-upgrade.md) | 产物矩阵、manifest v2、Ed25519、Authenticode、自动检查、事务安装与 guided update |
| [Acceptance and Risks](./acceptance-and-risks.md) | 风险控制、发布门禁、故障注入和真实签名 Windows E2E |
| [Plugin Contract v3 Upgrade](./plugin-contract-v3-upgrade.md) | manifest v3、artifact v2、数据保留与旧包禁用语义 |
| [Windows Desktop Runtime](./windows-desktop-runtime.md) | Windows WebView2 运行时要求与联网、离线安装指引 |
| [Linux Desktop Runtime](./linux-desktop-runtime.md) | Linux GTK 3、WebKitGTK 运行时要求与发行版安装命令 |

## 正式来源

- 发布元数据字段与限制：[`contracts/release-manifest.schema.json`](../../contracts/release-manifest.schema.json)
- 更新状态、阶段与 Web API：[`contracts/web-api.openapi.yaml`](../../contracts/web-api.openapi.yaml)
- CLI 更新入口：[`contracts/cli-commands.yaml`](../../contracts/cli-commands.yaml)
- 固定工具链与发布基线：[`docs/engineering/baseline.md`](../engineering/baseline.md)

在正式 Authenticode 证书和真实签名 Windows packaged E2E 通过之前，`windows-x64-full` 的更新方式固定为 `guided`（引导更新）。
