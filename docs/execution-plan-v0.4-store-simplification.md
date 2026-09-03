# RayleaBot v0.4 插件商店收敛计划

## 文档状态

- 目标版本：v0.4
- 执行范围：RayleaBot 主仓库与 `RayleaBotPlugins/plugin-catalog`
- 升级方式：一次性更新当前未正式发布的 catalog v2 与 artifact v2，不保留旧形状兼容层
- 当前状态：实现与复验完成，待维护者验收
- 验收方式：本文档保留，由项目维护者逐项验收

状态说明：

- `⬜ 待处理`：尚未开始。
- `🟡 进行中`：已经开始，但尚未满足完成条件。
- `☑️ 已完成`：实现、配套更新和本项验证均已完成。
- `❌ 阻塞`：当前范围内无法完成，已记录原因。

## 固定决策

1. 保留官方插件目录，允许管理员添加和移除自定义 HTTPS 目录。
2. 商店目录不使用独立签名、公钥注入、双钥轮换、内嵌空目录或时间戳防回退。
3. Server 保存每个来源最后一次成功读取的目录；网络不可用时继续使用磁盘缓存。
4. catalog v2 每个插件只记录当前发布版本，不复制 GitHub Releases 历史。
5. catalog 资产只记录平台、HTTPS 下载地址和归档 SHA-256；支持发布任意一个或多个正式平台。
6. artifact v2 只记录目标平台和原生入口。安装器扫描实际文件，拒绝符号链接、路径冲突、缺失入口和错误二进制格式，不维护逐文件摘要清单。
7. 商店安装检查结果进入用户确认界面。首次安装、来源变化或权限扩大时需要确认；同来源且权限未扩大的更新直接进入安装任务。
8. 本地目录、ZIP 和远程 ZIP 安装继续使用检查与确认流程。
9. 商店详情 API 保留，本轮不增加详情页。
10. 官方目录从插件 GitHub Release 自动生成，不再人工复制资产大小、manifest 摘要或签名文件。

## 执行清单

| ID | 工作项 | 状态 | 完成情况 | 验证证据 |
| --- | --- | --- | --- | --- |
| S1 | 收敛 catalog、artifact 与 Web API 合同 | ☑️ 已完成 | 删除目录签名合同；catalog 只保留当前发布，artifact 只保留平台与入口；Web API 增加来源管理和商店检查接口，并去掉 accept 阶段重复的来源与版本字段。 | `validate_contracts.py --self-test` 与 `--mode strict` 通过；嵌入 schema 和 Web OpenAPI 类型已重新生成。 |
| S2 | 简化 artifact 构建与安装校验 | ☑️ 已完成 | Go SDK 生成最小 artifact；SDK 与 Server 扫描实际文件，校验入口、平台、二进制格式、符号链接、非普通文件和大小写冲突；安装元数据移除 manifest、发布者和目录摘要重复字段。 | `go test ./...`（`sdk/go`）通过；Server artifact、lifecycle、CLI、catalog 与 runtime 回归通过。 |
| S3 | 实现来源持久化、目录缓存与来源绑定 | ☑️ 已完成 | SQLite 增加插件源与最后成功目录缓存；默认官方源自动播种；自定义 HTTPS 来源支持增删改查和单独刷新；失败刷新不覆盖缓存。 | migration 000006、sqlc 生成物、storage 与 pluginmarket 单元测试通过。 |
| S4 | 收敛商店安装确认与更新流程 | ☑️ 已完成 | 商店先 inspection，再按首次安装、来源变化或权限扩大决定是否确认；同来源无权限扩大更新直接提交；accept 仅传 inspection、包摘要与确认状态。 | pluginmarket、management、集成安装流和 Web store 测试通过。 |
| S5 | 完善商店来源管理、图标、分类与批量更新界面 | ☑️ 已完成 | 商店页支持来源切换与管理、缓存状态、分类、图标失败回退、批量更新和权限确认；桌面与窄屏均保留主要操作。 | Web typecheck、62 个单元测试文件 312 项测试、production build，以及插件商店 Playwright 桌面/窄屏流程通过。 |
| S6 | 自动生成官方 catalog | ☑️ 已完成 | `plugin-catalog` 用 `sources.json` 维护稳定信息，定时读取最新 GitHub Release、检查单根目录 artifact v2 并生成当前版本 catalog；删除签名脚本与签名文件。 | 3 项生成器单元测试、`sync_catalog.py --validate-only`、Python 编译检查、workflow YAML 解析与 catalog schema 校验通过。 |
| S7 | 同步工程基线、架构和用户文档 | ☑️ 已完成 | 基线、实现顺序、运行架构、生命周期、SDK、权限、发布、商店与合同文档统一为来源缓存、最小 artifact 和当前 Release 语义。 | 133 个 Markdown 文件链接检查与 agent docs 检查通过。 |
| S8 | 全量 review 与验证 | ☑️ 已完成 | 已完成主路径 review，并修复 inspection 后确认按钮保持 loading、旧测试 fixture 仍生成文件清单、accept 请求重复身份字段，以及发布脚本仍写入旧插件合同版本的问题。 | 合同 self-test/strict、Server 全仓测试与构建、SDK 全仓测试、release 测试、Web typecheck/312 项测试/build、2 项商店 E2E、catalog/workflow 校验和两个仓库 `git diff --check` 均通过。 |

## 验收重点

- 官方目录和自定义目录都能读取、缓存、切换和安装。
- 当前平台没有资产时只显示不可安装，不要求插件发布全部平台。
- 商店页面在确认前显示真实插件身份和权限。
- 同来源且权限未扩大的更新不重复弹出风险确认。
- 目录或下载失败不会清空最后一次成功结果。
- catalog、artifact、Server、Web、fixtures、生成类型和文档只有一套字段语义。
- 官方目录发布不依赖目录私钥、签名提交或人工摘要录入。

## 验收结论

- S1 至 S8 全部完成，无阻塞项。
- 商店详情后端按维护者决定保留，当前 Web 不提供详情页。
- 外部业务插件仓库未修改；`plugin-catalog` 已切换为自动生成当前 Release 目录。
- 本文档保留，等待柒柒逐项验收。
