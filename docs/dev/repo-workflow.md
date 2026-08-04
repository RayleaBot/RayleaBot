# Repository Workflow

本页说明 RayleaBot 仓库中的版本控制边界和常规忽略策略。

## 版本控制边界

- 主仓库不保存业务插件源码或产物；官方插件各自使用独立 Git 仓库和发布流程。
- `examples/plugins/` 只承担 SDK 示例职责，纳入版本控制但不进入发现、商店或发布主链。
- 用户安装插件、开发同步产物、运行缓存、日志和用户配置不进入版本控制。
- 本地插件仓库通过被忽略的 `plugin-workspace.local.json` 引用，不作为主仓库 submodule 或源码 discovery root。

## 当前常规忽略项

```plain
data/
cache/
logs/
plugins/installed/*
!plugins/installed/.gitkeep
config/user.yaml
plugin-workspace.local.json
.tmp/plugin-dev/
.deps/
node_modules/
dist/
.env
```

## 协作原则

- 目录职责以正式发布目录和工程基线为准，不为开发便利再造第二套路径模型。
- 用户数据目录、程序托管目录和仓库受控内容保持清晰分离。
- 运行时生成物和本地缓存不进入正式提交面。
- 插件 SDK 的本地联调通过临时 `go.work` 与 `.rayleabot/sdk/vue` 完成，不提交 `replace`、镜像 SDK 或跨仓库构建产物。

## 本地插件联调

主仓库启动器负责编排本机独立插件仓库，不依赖 GitHub 构建：

1. `plugin-workspace.local.json` 声明需要联调的仓库；该文件及 `.tmp/plugin-dev/` 均不进入版本控制。
2. 首次启动为所有启用插件生成临时 `go.work`，镜像当前主仓库 Vue SDK，调用各插件自己的 `tools/build` 构建当前平台完整 artifact。
3. 启动器在 Server 未运行时调用 `plugin dev-sync`，经正式校验与原子安装事务写入 `plugins/installed/`；运行期仍只发现已安装产物，不直接发现源码目录。
4. `watch` 模式按 500ms 窗口和插件 ID 合并变更，后续只重建本批变化的插件；构建过程中发生的新变更进入下一批。
5. 候选构建或同步失败时保留并恢复上一个已安装 artifact。

插件仓库的 GitHub Actions 只处理 `v*` tag 的三平台 Release，商店再通过签名 catalog 分发这些发布包。日常修改插件或与本地主仓库 SDK 联调不需要创建 tag、提交远端或等待 GitHub Actions。

独立插件仓库、本地工作区和启动模式见 [插件商店与独立开发](../plugin/store-and-development.md)。
