# Repository Workflow

本页说明 RayleaBot 仓库中的版本控制边界和常规忽略策略。

## 版本控制边界

- 官方内置插件位于 `plugins/builtin/`，纳入版本控制。
- 用户安装插件、运行缓存、日志、私有运行时环境和用户配置不进入版本控制。
- `plugins/dev/` 是否纳入版本控制，按本地开发策略决定。

## 当前常规忽略项

```plain
data/
cache/
logs/
plugins/installed/*
!plugins/installed/.gitkeep
config/user.yaml
.deps/
node_modules/
dist/
.env
```

## 协作原则

- 目录职责以正式发布目录和工程基线为准，不为开发便利再造第二套路径模型。
- 用户数据目录、程序托管目录和仓库受控内容保持清晰分离。
- 运行时生成物和本地缓存不进入正式提交面。
