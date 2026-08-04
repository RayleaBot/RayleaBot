# Plugin Docs

本目录说明 RayleaBot 插件平台的当前边界、manifest、能力声明、能力参数、协议和 SDK。

正式裁决以 `contracts/plugin-info.schema.json`、`contracts/plugin-artifact.schema.json`、`contracts/plugin-protocol.schema.json`、插件商店/开发工作区 schema 和管理页契约为准。发行包中的服务端嵌入同一组校验规则。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [store-and-development.md](./store-and-development.md) | 插件商店、独立仓库发布与本地同步开发 |
| [lifecycle.md](./lifecycle.md) | 插件来源、运行时支持、安装、重载和卸载边界 |
| [capabilities-and-manifest.md](./capabilities-and-manifest.md) | manifest 结构、能力声明和能力参数 |
| [protocol.md](./protocol.md) | JSONL 协议、消息语义和 local action RPC |
| [management-ui.md](./management-ui.md) | Vue 管理页、独立插件域与 bridge v2 |
| [sdk/README.md](./sdk/README.md) | Go 插件 SDK、artifact 构建器与 Vue UI SDK |

## 当前边界

- 插件通过正式 manifest、声明能力、能力参数和协议帧接入平台。
- 主仓库不携带内置业务插件；商店、本地安装和开发同步统一写入 `plugins/installed/`。
- 插件后端只支持预编译 Go artifact；管理页只支持 artifact 内的静态资源，推荐使用与 Web 一致的 Vue 3 技术栈。
- 平台继续统一裁决生命周期、artifact 完整性、聊天命令权限和出站消息语义。
- 新增字段、动作或协议帧必须先更新 contract，再同步 SDK、fixtures、示例和本文档集。
