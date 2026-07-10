# Delivery and Upgrade

本页说明 RayleaBot 的正式发行物、发布信任、升级事务和回滚边界。字段与限制以 [`contracts/release-manifest.schema.json`](../../contracts/release-manifest.schema.json) 为准。

## 正式产物矩阵

| `artifact_id` | 产物 | 支持级别 | 更新方式 |
| --- | --- | --- | --- |
| `windows-x64-full` | Windows 桌面完整包 | `first_class` | 满足全部签名门槛时 `automatic`，否则 `guided` |
| `linux-x64-full` | Linux 桌面完整包 | `first_class` | `guided` |
| `macos-arm64-full` | macOS Apple Silicon 桌面完整包 | `first_class` | `guided` |
| `linux-x64-server` | Linux 服务端包 | `first_class` | `guided` 或 `manual` |

`automatic` 表示用户确认后的事务式安装，不表示静默下载或静默安装。当前正式 Windows 证书缺失时，`windows-x64-full` 必须发布为 `guided`。

## 发布包目录

发行包根目录按产物形态包含：

- server 二进制与 Launcher 桌面入口；
- `web/dist`、`plugins/builtin/`、`templates/`；
- `config/default.yaml` 与 `.deps/manifest.json`；
- `build_info.json`；
- 根仓库 `LICENSE` 与生成、审阅后的 `THIRD_PARTY_NOTICES.md`。

依赖许可证未知或缺失时，发布必须失败。运行时配置和插件 schema 内置于 server；源码仓库中的 `contracts/` 仍是正式来源。

## 发布信任元数据

每次正式 Release 同时发布：

- `release_manifest.v2.json`；
- `release_manifest.v2.sig.json`；
- 各平台 artifact。

### Manifest v2

Manifest 固定包含版本、提交、构建时间、channel、发布时间、过期时间、更新协议版本、配置/数据库/插件协议版本和 artifact 列表。每个 artifact 至少声明：

- 唯一 `artifact_id`、平台和 basename 文件名；
- SHA-256、归档大小、展开大小和文件数；
- `update_mode` 与最小 updater 协议版本；
- 支持级别、`.deps/manifest.json` 摘要和 smoke profile；
- Windows 自动安装所需的 signer 证书 SHA-256 摘要。

归档上限为 2 GiB，展开上限为 8 GiB，文件上限为 100,000，压缩比上限为 100:1。

### Ed25519 signature envelope

签名 envelope 固定使用 Ed25519，记录原始 manifest 的 SHA-256、主 `key_id` 和一至两个签名。双签 envelope 用于新旧公钥轮换。

更新仓库地址和受信 Ed25519 公钥注册表编译进更新核心；可修改的 `build_info.json` 不能改变仓库或信任根。更新器保存已见最高版本和 manifest digest，拒绝：

- 签名或摘要不匹配；
- manifest 过期；
- 版本降级或旧 manifest 重放；
- 同版本对应不同 manifest digest；
- artifact 的平台、版本、协议、文件名、大小或摘要不一致。

过期 manifest 只允许走手动更新。首个可信更新基线必须手动安装，无法验证 v2 元数据的客户端不能进入自动安装链路。

## 更新检查与用户确认

Launcher 后台每 6 小时检查一次更新，只展示可用版本和发布说明。用户确认后才允许下载、停服、备份、替换或回滚。

Web 通过 `GET /api/update/status` 查看状态，通过 `POST /api/update/check` 主动检查；Web 不执行安装。CLI 提供：

```text
raylea-server version --json
raylea-server update check --json
raylea-server update verify --manifest <path> --signature <path> --artifact <path>
```

## Windows 自动安装门槛

`windows-x64-full` 只有同时满足以下条件才可标记为 `automatic`：

- manifest v2 与 Ed25519 signature envelope 验证通过；
- artifact SHA-256、大小、平台、版本和 updater 协议验证通过；
- Launcher、server 与 updater 全部通过 Authenticode；
- signer 与 manifest 中的证书摘要一致；
- release job 通过标准 Windows certificate-store 接口使用本机或云签名证书，签名算法为 SHA-256，并带 RFC3161 timestamp；
- release job 的 `signtool verify /pa /all` 和 packaged E2E 通过。

自签名证书不能满足正式门槛。任一条件失败时，发布清单必须使用 `guided`，Launcher 不显示安装按钮。

## 事务式安装

外置 `raylea-updater.exe` 位于安装根之外，并在执行前重新完成签名、摘要、平台、版本、归档结构和磁盘空间检查。下载使用 partial 文件与原子 rename；metadata 请求超时 10 秒，下载空闲超时 30 秒，总时限 30 分钟。

用户确认后执行：

1. 记录升级前服务状态并停止原服务。
2. 运行正式 offline backup，并把备份复制到安装根之外的事务目录。
3. 在同卷 staging 解包，拒绝路径穿越、reparse point、大小写冲突和额外根目录。
4. 使用新 server 执行 restore、doctor 和 preflight。
5. 以双 rename 交换旧安装根和 staging。
6. 启动新 Launcher 并执行 postflight。
7. 原服务运行时验证 `healthz`、`readyz`；原服务停止时验证 Launcher heartbeat、build info 和文件版本。
8. postflight 成功后提交事务；失败时停止新版、恢复旧安装根和旧状态，并按升级前状态启动旧版。

事务保留：

- `config/user.yaml`；
- `data/**`；
- `plugins/installed/**`。

`cache/` 和 `logs/` 不参与恢复，也不能阻止安装。回滚失败进入 `rollback_failed` 并禁止自动启动。旧版本与 offline backup 至少保留 7 天，并至少保留到下一次成功升级。

## Guided update

Linux、macOS、server 包以及未满足 Windows 自动安装门槛的发行物使用 `guided`：

1. 使用受信更新入口检查并验证 manifest、signature envelope 和 artifact。
2. 停止服务并生成 offline backup。
3. 按平台说明替换程序文件，同时保留用户配置、数据和已安装插件。
4. 运行 doctor、兼容检查和健康检查。
5. 失败时使用升级前包与 offline backup 恢复。

GitHub 自动生成的源代码压缩包不是正式运行时产物。

## 相关文档

- [Acceptance and Risks](./acceptance-and-risks.md)
- [Deployment](../user/deployment.md)
- [Recovery](../user/recovery.md)
