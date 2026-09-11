# Release Notes Template

RayleaBot 发布说明使用中文，面向安装、升级和使用机器人的用户。标题为 `RayleaBot <tag>`，正文保存在 [`notes/`](./notes/README.md) 下的同名标签文件中。

功能版以约一页正文为目标；补丁版通常为 150–300 个中文字，不计下载表和链接。每版保留摘要、实际变化、升级说明和下载入口；有比较基线时提供完整变更链接。

## 功能版本

复制下面代码块的内容，替换占位符并删除编辑注释。`TAG` 为完整标签，如 `v0.5.0`；`VERSION` 不含 `v`。功能亮点通常为 2–4 项，没有内容的可选栏目直接省略。

```markdown
{{一两句话说明本版解决的主要问题，以及哪些用户值得关注。}}

<!-- 有破坏性变更、必须先处理的迁移或严重已知问题时保留，放在亮点之前。 -->
> [!IMPORTANT]
> {{受影响的版本或用户}}：{{升级后的行为变化}}。升级前请{{具体操作}}；详见[迁移说明]({{MIGRATION_URL}})。

## 本次重点

- **{{重点一}}**：{{用户现在能完成什么，或什么问题得到解决。}}
- **{{重点二}}**：{{可观察的变化或使用入口。}}

## 新增与改进

- **{{功能或模块}}**：{{使用场景、变化与结果。}}（[{{PR_OR_COMMIT_LABEL}}]({{CHANGE_URL}})）

## 问题修复

- 修复{{触发条件}}下{{用户遇到的问题}}。{{必要时说明修复后的结果。}}（[{{PR_OR_COMMIT_LABEL}}]({{CHANGE_URL}})）

## 升级说明

- **升级范围**：{{已确认支持的来源版本，以及是否需要先升级到中间版本。}}
- **配置与数据**：{{是否需要手动调整；涉及迁移时说明备份要求、数据保留及恢复限制。}}
- **插件兼容性**：{{现有插件能否继续运行；受影响插件需要安装什么版本或包。}}
- **操作方法**：[升级指南](https://github.com/RayleaBot/RayleaBot/blob/{{TAG}}/docs/release/delivery-and-upgrade.md)。{{本版额外的升级操作。}}

<!-- 无特殊要求时，将上面的详细列表缩成一句经核实的兼容性说明和升级指南链接。 -->
## 下载与安装

| 使用环境 | 下载 |
| --- | --- |
| Windows x64 桌面完整包 | [ZIP](https://github.com/RayleaBot/RayleaBot/releases/download/{{TAG}}/RayleaBot-v{{VERSION}}-windows-x64-full.zip) |
| Linux x64 桌面完整包 | [tar.gz](https://github.com/RayleaBot/RayleaBot/releases/download/{{TAG}}/RayleaBot-v{{VERSION}}-linux-x64-full.tar.gz) |
| macOS Apple Silicon 桌面完整包 | [tar.gz](https://github.com/RayleaBot/RayleaBot/releases/download/{{TAG}}/RayleaBot-v{{VERSION}}-macos-arm64-full.tar.gz) |
| Linux x64 服务端包 | [tar.gz](https://github.com/RayleaBot/RayleaBot/releases/download/{{TAG}}/RayleaBot-v{{VERSION}}-linux-x64-server.tar.gz) |

{{根据本版各产物的 update_mode，说明通过 Launcher 确认安装、引导更新或手动更新的实际入口。}}

首次使用请阅读[安装说明](https://github.com/RayleaBot/RayleaBot/blob/{{TAG}}/docs/user/deployment.md)。业务插件从插件商店或各插件 Release 单独安装。下方 Assets 中的 Source code 为源码归档，运行请下载上表对应的平台包。

[SHA-256 校验清单](https://github.com/RayleaBot/RayleaBot/releases/download/{{TAG}}/SHA256SUMS.txt) · [发布清单](https://github.com/RayleaBot/RayleaBot/releases/download/{{TAG}}/release_manifest.v2.json) · [清单签名](https://github.com/RayleaBot/RayleaBot/releases/download/{{TAG}}/release_manifest.v2.sig.json)

<!-- 以下两节有对应内容时才保留。影响升级的开发者变更也要在升级说明中摘要提示。 -->
## 开发者说明

- **{{API／SDK／插件协议}}**：{{受影响的接口、行为或版本要求}}；见[适配说明]({{DEVELOPER_GUIDE_URL}})。

## 已知问题

- **{{问题}}**：影响{{平台或条件}}；临时处理方式为{{操作或暂无替代方案}}。跟踪：[{{ISSUE_LABEL}}]({{ISSUE_URL}})。

---

[完整变更：{{PREVIOUS_TAG}} → {{TAG}}](https://github.com/RayleaBot/RayleaBot/compare/{{PREVIOUS_TAG}}...{{TAG}})

感谢 {{CONTRIBUTORS}} 的代码、问题反馈、测试与文档贡献。
```

## 补丁版本

补丁版聚焦本次修复及升级影响。需要先处理的迁移或严重已知问题仍放在摘要之后，使用功能模板中的重要提醒格式。

```markdown
本版本修复{{主要问题}}，建议{{受影响的用户}}升级。

## 问题修复

- 修复{{场景}}下{{可观察的问题}}。（[{{PR_OR_COMMIT_LABEL}}]({{CHANGE_URL}})）

## 升级说明

{{本次相对于前一版本的配置、数据和插件兼容性结论，以及需要采取的操作。}}

[升级指南](https://github.com/RayleaBot/RayleaBot/blob/{{TAG}}/docs/release/delivery-and-upgrade.md)

## 下载

{{插入功能模板中的平台下载表、实际更新方式和校验文件链接。}}

[完整变更](https://github.com/RayleaBot/RayleaBot/compare/{{PREVIOUS_TAG}}...{{TAG}})
```

## 填写规则

- 每条内容说明变化对使用者的影响。相关提交合并为一条功能或修复；亮点与后续条目相互补充，避免重复铺开同一内容。
- 普通重构、CI 整理和版本号更新留在完整变更中。影响安全、兼容性或平台要求的依赖升级需要进入正文。
- 破坏性变更说明谁受影响、行为如何变化、需要做什么。必要操作不能只放进折叠区域。
- 升级兼容性、备份格式和可回退范围依据目标标签的 contract、实现及验证结果填写，不预填“无破坏性变更”“可直接覆盖”或“所有插件兼容”。插件能否运行与数据是否保留分别说明。
- 下载表与实际上传的产物一致。文件名、平台、校验文件和更新方式核对本版 release manifest 与 Assets。Windows 的安装方式按签名及验证结果填写，不能把 Ed25519 清单签名等同于 Windows Authenticode。
- 已知问题写清影响条件、临时处理方式或跟踪入口。没有已确认条目时删除该节，不自动生成“没有已知问题”。
- 首次发布没有比较基线时，使用该标签的版本说明或提交历史入口。稳定版通常与上一稳定版比较，预发布明确比较基线。
- 预发布在摘要说明测试用途、验证范围和已知限制，并核对 GitHub 预发布标记与正式 channel。
- 仅列实际贡献者；直接提交使用 commit 链接，不虚构 PR。完整变更使用 compare 链接，避免重复粘贴自动生成的全量清单。
- 文档链接优先固定到本次标签，确认对应文件存在。下载链接在产物生成后核对。
- `{{...}}` 是编辑占位符，正文检查在文本、链接、注释和代码块中均会拒绝残留。若需展示实际的双花括号语法，应改写示例或使用独立文档链接。

## 栏目取舍

| 部分 | 规则 |
| --- | --- |
| 摘要 | 保留，一两句话 |
| 重要提醒 | 有必须先处理的事项时置顶 |
| 本次重点 | 有明显主题或较多变化时保留，通常 2–4 条 |
| 新增与改进／问题修复 | 保留有内容的类别 |
| 升级说明 | 保留；无特殊操作时缩成一句和文档入口 |
| 下载与校验 | 保留，使用本版真实产物 |
| 开发者说明 | 有 API、SDK 或插件适配要求时保留 |
| 已知问题 | 有已确认问题时保留 |
| 完整变更 | 有比较基线时提供 compare 链接 |
| 致谢 | 有实际贡献者时保留 |
