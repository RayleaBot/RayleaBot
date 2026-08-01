# Release Docs

本目录说明 RayleaBot 的产物矩阵、发布信任、升级事务、回滚和验收门槛。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [Delivery and Upgrade](./delivery-and-upgrade.md) | 产物矩阵、manifest v2、Ed25519、Authenticode、自动检查、事务安装与 guided update |
| [Acceptance and Risks](./acceptance-and-risks.md) | 风险控制、发布门禁、故障注入和真实签名 Windows E2E |
| [Plugin Go + Vue Reset](./plugin-go-vue-reset.md) | manifest v2 断代前备份、重置范围与旧 epoch 拒绝语义 |

## 正式来源

- 发布元数据字段与限制：[`contracts/release-manifest.schema.json`](../../contracts/release-manifest.schema.json)
- 更新状态、阶段与 Web API：[`contracts/web-api.openapi.yaml`](../../contracts/web-api.openapi.yaml)
- CLI 更新入口：[`contracts/cli-commands.yaml`](../../contracts/cli-commands.yaml)
- 固定工具链与发布基线：[`docs/engineering/baseline.md`](../engineering/baseline.md)

正式 Authenticode 证书和真实签名 Windows packaged E2E 通过前，`windows-x64-full` 的正式更新方式为 `guided`。
