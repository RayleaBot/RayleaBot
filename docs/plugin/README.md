# Plugin Docs

本目录说明 RayleaBot 插件平台的当前边界、manifest、能力授权、协议和 SDK。

正式裁决以源码中的 `contracts/plugin-info.schema.json` 和 `contracts/plugin-protocol.schema.json` 为准。发行包中的服务端程序内置插件 manifest 校验规则。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [lifecycle.md](./lifecycle.md) | 插件来源、运行时支持、安装、重载和卸载边界 |
| [capabilities-and-manifest.md](./capabilities-and-manifest.md) | manifest 结构、能力声明和授权作用域 |
| [protocol.md](./protocol.md) | JSONL 协议、消息语义和 local action RPC |
| [sdk/README.md](./sdk/README.md) | 官方 Python / Node.js SDK 使用边界 |

## 当前边界

- 插件通过正式 manifest、能力授权和协议帧接入平台。
- 平台继续统一裁决生命周期、权限、运行环境和出站消息语义。
- 新增字段、动作或协议帧必须先更新 contract，再同步 SDK、fixtures、示例和本文档集。
