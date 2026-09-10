# Plugin Contract v3

RayleaBot 当前插件合同由 manifest v3、protocol v3、artifact v2 和 management bridge v3 组成。本次按全新分发建立运行目录、当前配置与数据库结构。

## 安装与持久化

插件包通过统一 artifact 校验和可信代码确认后安装。插件设置、密钥、KV、文件和已发布数据按插件 ID 隔离，插件运行使用当前宿主协议。原生可执行文件由插件作者构建，宿主负责校验、启停和资源回收。

## 备份与恢复

本版备份使用 backup manifest v3，清单记录当前配置、数据库和插件合同版本。恢复到空目录后，平台检查运行资源与插件状态；管理员登录、插件设置、密钥、KV 和文件按归档内容恢复。完整步骤见[恢复说明](../user/recovery.md)。

## 新插件包

插件包是目标平台的单根目录 ZIP，入口是对应平台的原生可执行文件。artifact v2 不声明实现语言，插件身份和版本只来自根目录 `info.json`。官方和社区插件均由各自仓库构建，通过 HTTPS 商店目录或本地 artifact 进入同一安装校验流程。

开发者使用 `raylea-plugin inspect` 检查项目和产物，使用 `raylea-plugin pack` 打包任意已构建原生入口，Go 插件可使用 `raylea-plugin build-go` 构建并打包。

多适配器身份与 SDK 重建步骤见[Plugin Protocol v3 Upgrade](./plugin-protocol-v3-upgrade.md)。
