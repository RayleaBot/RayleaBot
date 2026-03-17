# Plugin Docs

本目录承载 RayleaBot 的插件开发文档，包括：

- 插件 manifest。
- Capabilities。
- 生命周期。
- 运行时边界。
- 协议与能力调用说明。

规则：

- 插件对外契约最终以 `contracts/plugin-info.schema.json` 与 `contracts/plugin-protocol.schema.json` 为准。
- 本目录用于解释插件开发语义，不是最终协议裁决来源。
- 插件不得绕过 Capability 校验或跨层访问平台内部模块。
