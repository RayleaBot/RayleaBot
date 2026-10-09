# Plugin Docs

本目录说明 RayleaBot 插件平台的 manifest、协议、生命周期和 SDK。

正式定义以 `contracts/plugin-info.schema.json`、`contracts/plugin-artifact.schema.json`、`contracts/plugin-protocol.schema.json`、插件商店目录 schema 和管理页契约为准；发行包中的服务端使用同一组校验规则。

## 阅读入口

| 文档 | 主题 |
| --- | --- |
| [store-and-development.md](./store-and-development.md) | 插件商店、独立仓库发布与本地同步开发 |
| [lifecycle.md](./lifecycle.md) | 插件来源、运行时支持、安装、重载和卸载边界 |
| [manifest.md](./manifest.md) | manifest v4、事件、命令与静态资源声明 |
| [protocol.md](./protocol.md) | JSONL 协议、消息语义和 local action RPC |
| [management-ui.md](./management-ui.md) | Vue 管理页、同源加载与 CSP |
| [sdk/README.md](./sdk/README.md) | Go 插件 SDK、artifact 构建器与 Vue UI SDK |

## 当前边界

- 插件通过正式 manifest 和协议帧接入平台。
- 主仓库不携带内置业务插件；商店安装、本地安装和开发同步都写入 `plugins/installed/`。
- 插件后端使用当前平台的预编译原生 artifact，实现语言不限，通过 JSONL v4 与宿主通信；Go 插件可使用官方 SDK 和构建器。管理页使用 artifact 内的静态资源，推荐 Vue 3 技术栈。
- 平台统一管理插件生命周期、artifact 完整性、聊天命令权限和出站消息语义。

## 合同兼容承诺

manifest v4、JSONL protocol v4 与 artifact v2 至少在 0.4.x 与下一个 minor 版本内保持兼容：按这些合同构建的插件无需重新构建即可继续运行，宿主不删除已有字段、动作与事件，也不改变它们的语义。期间的新能力只以可选字段、新动作或新事件加入，并由插件以 `min_core_version` 声明所需的最低核心版本。

兼容规则按方向不对称：

- 宿主发给插件的事件与结果可以增加字段，插件应忽略不认识的字段；Go SDK 按此解码。
- 插件发给宿主的 manifest 与动作数据严格校验。使用新字段或新动作的插件必须把 `min_core_version` 设为提供该能力的核心版本；旧核心安装这类插件时先检查最低核心版本，提示需要升级核心。
- 商店目录读取忽略未知字段与不认识平台的资产，目录增加字段不影响已部署的核心。

超出上述范围的不兼容变更会在发布说明中提前说明并提供迁移方式。
