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

主仓库开发启动脚本（`node scripts/start-dev.mjs`，Windows 下为 `start.bat`）负责编排本机独立插件仓库，不依赖 GitHub 构建；日常修改插件或与本地主仓库 SDK 联调不需要创建 tag、提交远端或等待 GitHub Actions。工作区文件、同步入口与监听规则见[插件商店与独立开发](../plugin/store-and-development.md)，增量构建与环境复用见[开发者文档](./README.md)，插件目录约定与统一构建工具见[插件 SDK](../plugin/sdk/README.md)。

## 显式开发工具路径

POSIX 启动入口允许 `RAYLEA_NODE_EXECUTABLE=/absolute/path/to/node ./start.sh`；路径必须是可执行的绝对文件路径，仍检查 `.tool-versions` 中的固定版本。含空格路径在赋值时加引号。无效显式路径会报错，不改用 PATH 中的其他版本。

Server 与 Launcher 开发脚本共用 `scripts/process-invocation.mjs` 解析 Go：优先 `RAYLEA_GO_EXECUTABLE` 的绝对路径，再查 PATH，Windows 再查 Program Files 下的 Go。启动参数与子进程退出码继续传回调用方。
