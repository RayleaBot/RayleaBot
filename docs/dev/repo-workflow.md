# Repository Workflow

本页说明 RayleaBot 仓库中的版本控制边界和常规忽略策略。

## 版本控制边界

- 主仓库不保存业务插件源码或产物；官方插件各自使用独立 Git 仓库和发布流程。
- `examples/plugins/` 只承担 SDK 示例职责，纳入版本控制但不进入发现、商店或发布主流程。
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
/.deps/*
!/.deps/manifest.json
node_modules/
dist/
.env
```

## 协作原则

- 目录职责以正式发布目录和工程基线为准，不为开发便利再造第二套路径模型。
- 用户数据目录、程序托管目录和仓库受控内容保持清晰分离。
- 运行时生成物和本地缓存不进入正式提交。
- 插件 SDK 的本地联调通过临时 `go.work` 与 `.rayleabot/sdk/vue` 完成，不提交 `replace`、镜像 SDK 或跨仓库构建产物。

## 本地插件联调

主仓库开发启动脚本（`node scripts/start-dev.mjs`，Windows 下为 `start.bat`）负责编排本机独立插件仓库，不依赖 GitHub 构建：

独立 Go 插件后端采用 `cmd/<plugin>`、`internal/plugin` 与可选 `internal/assets` 目录；统一的 `raylea-plugin build-go` 从 `info.json` 推导插件 ID 和默认后端 package，也可通过参数覆盖入口并把内部资源映射到稳定 artifact 路径。其他语言或构建系统把目标平台原生可执行文件写入 `dist/native/<platform>/<plugin-id>[.exe]`，再使用 `raylea-plugin pack`。

1. `plugin-workspace.local.json` 声明需要联调的仓库；该文件及 `.tmp/plugin-dev/` 均不进入版本控制。
2. 为存在 `go.mod` 的启用插件生成临时 `go.work`，按内容镜像主仓库 Vue SDK，复用当前平台的后端、UI 和 artifact 缓存；非 Go 插件使用工作区约定的原生产物路径。
3. 启动前先构建开发插件，在停服状态下使用 `plugin dev-sync` 同步，再启动 Server 加载插件。Server 与插件同时变更时，也在 Server 停止后先同步插件再启动新版本。只更新插件时，通过本机认证开发接口在线同步。两种同步方式均经正式校验与原子安装事务写入 `plugins/installed/`，内容未变化时跳过安装，运行期不直接发现源码目录。
4. `watch` 模式按 500ms 窗口和插件 ID 合并变更。构建期间的新修改在安装前重新检查；SDK 与工作区清单变化会更新对应依赖和监听集合。
5. 在线插件更新只切换目标插件，保留 desired state；初始化失败恢复旧版本。只有 Server 自身构建输入变化才重启 Server。

插件仓库的 GitHub Actions 只处理 `v*` tag 的正式 Release，官方目录定时读取每个仓库的当前 Release 并收录实际发布的平台包。日常修改插件或与本地主仓库 SDK 联调不需要创建 tag、提交远端或等待 GitHub Actions。

独立插件仓库、本地工作区和启动模式见 [插件商店与独立开发](../plugin/store-and-development.md)。
