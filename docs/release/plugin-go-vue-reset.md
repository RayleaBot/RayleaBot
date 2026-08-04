# Plugin Go + Vue Reset

RayleaBot 的插件平台从 manifest v1 断代到 manifest v2。新版本只运行预编译 Go 后端和 artifact 内的 Vue 管理页，不读取旧 manifest、Python/Node 插件运行时、bridge v1、退役配置键或旧插件数据。

## 升级前

1. 停止服务。
2. 使用 `scripts/release/breaking_plugin_epoch_reset.py backup` 把 `config/`、`data/` 和 `plugins/installed/` 备份到仓库外。
3. 使用同一工具的 verify 命令校验备份清单和 SHA-256。

外部备份只用于人工留档或回退到旧版本，不能恢复进 Go+Vue 插件 epoch。

## 重置范围

重置会清除：

- 插件 desired state、package metadata、settings、secrets、KV、scheduler 与模板状态；
- `plugins/installed/` 中的旧插件文件；
- subscription-hub 的旧第三方账号和 Bilibili 插件状态。

管理员身份、OneBot 配置、全局治理规则和审计日志保持不变。旧安装目录离开 discovery root 后不再被扫描。

## 新插件安装

只安装目标平台的 manifest v2 / artifact v1 目录或单根目录 ZIP。主程序发行包不含业务插件；官方和社区插件均由各自仓库完成 Go 与 Vue 构建，再通过签名商店目录或本地 artifact 进入统一安装流程。框架不执行数据转换、源码编译、依赖安装或安装脚本。

旧 epoch 的备份恢复或原位升级返回 `plugin.reset_required`。出现该错误时应继续使用外部备份回退旧版本，或接受重置后重新安装 Go artifact，不能修改清单绕过门禁。
