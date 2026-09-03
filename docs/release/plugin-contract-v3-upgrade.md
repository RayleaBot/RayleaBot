# Plugin Contract v3 Upgrade

RayleaBot 当前插件合同由 manifest v3、protocol v2、artifact v2 和 management bridge v3 组成。升级是一次性破坏性合同升级：运行时不兼容 manifest v2、protocol v1、artifact v1 或 bridge v2，也不保留旧合同解析分支。

## 升级行为

- 主程序升级保留 `config/user.yaml`、`data/**` 和 `plugins/installed/**`。
- 插件设置、密钥、KV、文件和已发布数据不会因合同升级被清除。
- 旧 manifest v2 / artifact v1 包会被识别为不受支持并保持禁用，不会被转换或执行。
- 重新安装 manifest v3 / artifact v2 包后，插件继续使用同一插件 ID 对应的持久化数据。
- 主程序不会编译插件源码、安装语言依赖或执行插件安装脚本。

本次升级不提供破坏性 reset 脚本，也不要求先清空插件目录。需要回退主程序时，仍应使用升级前保存在安装根之外的完整备份。

## 备份与恢复

当前只接受 backup manifest v3。清单记录 protocol v2、manifest v3、artifact v2 和 management bridge v3；backup manifest v2 直接拒绝。

v3 备份允许记录旧插件包事实并恢复其持久化数据。恢复不会使旧包变为可运行状态，也不会为了兼容旧包修改包内容。管理员应安装当前合同包，再重新启用插件。

## 新插件包

插件包是目标平台的单根目录 ZIP，入口是对应平台的原生可执行文件。artifact v2 不声明实现语言，插件身份和版本只来自根目录 `info.json`。官方和社区插件均由各自仓库构建，通过 HTTPS 商店目录或本地 artifact 进入同一安装校验流程。

开发者使用 `raylea-plugin inspect` 检查项目和产物，使用 `raylea-plugin pack` 打包任意已构建原生入口，Go 插件可以使用 `raylea-plugin build-go` 复用一等构建流程。
