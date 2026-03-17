# Implementation Order

## 1. 契约文件补全

前置条件：Phase 0 的 `contracts/` 骨架、baseline、AGENTS、最小 CI 已落地。
产出物：补全后的 schema、OpenAPI、WebSocket 清单、错误码目录、发行元数据契约。
暂不做什么：不写 Adapter、Runtime、Web API、Web UI、Launcher 的业务实现。

## 2. fixtures / golden cases

前置条件：关键 contracts 已至少达到“字段与枚举可校验”状态。
产出物：按 contract 分类的 golden inputs/outputs、最小正反例、错误样例、健康接口样例。
暂不做什么：不把 fixture 嵌进正式运行代码；不先写业务逻辑再倒推样例。

## 3. server 内核骨架

前置条件：plugin manifest、plugin protocol、config schema、错误码、健康接口契约已稳定到可依赖。
产出物：`server/cmd/raylea-server` 入口、基础包结构、配置加载入口、健康接口占位、状态模型占位、空的 repository/service 边界。
暂不做什么：不写 OneBot 适配逻辑、不写插件运行逻辑、不写正式 Web API 业务逻辑。

## 4. adapter

前置条件：统一事件模型、连接状态、错误码、配置 schema 已正式化。
产出物：OneBot11 反向 WebSocket 连接骨架、事件映射、连接状态流转、消息发送边界。
暂不做什么：不跨层写状态库；不偷写 Web UI 状态推断；不支持 v0.1 范围外的传输模式。

## 5. plugin protocol bridge

前置条件：`contracts/plugin-protocol.schema.json`、错误码、插件状态流转、IPC 限额已固定。
产出物：Runtime Bridge、JSONL 行读取器、`init/init_ack/event/action/result/error/ping/pong/shutdown` 基础链路。
暂不做什么：不提前做完整 SDK 语法糖；不让插件绕过 Capability 与协议边界。

## 6. config / storage

前置条件：`contracts/config.user.schema.json`、状态模型、迁移原则、错误码已稳定。
产出物：`config/default.yaml`、`config/user.yaml` 解析、SQLite 打开与最小 migration 骨架、配置热更新/局部重载/需重启矩阵实现。
暂不做什么：不让插件直写 `config/user.yaml`；不跳过 schema 校验直读配置。

## 7. web api

前置条件：OpenAPI、错误响应 envelope、健康接口、任务模型、插件状态模型已明确。
产出物：按 `contracts/web-api.openapi.yaml` 实现的 HTTP handler 骨架与最小管理接口。
暂不做什么：不在 handler 中私自发明字段；不让 CLI/Launcher 形成第二套状态源。

## 8. web ui

前置条件：OpenAPI、WebSocket envelope、状态枚举、错误码、任务流 contract 已明确。
产出物：Web UI 工程脚手架、状态页、插件列表页、日志流页、任务流页的最小壳层。
暂不做什么：不通过解析日志推断真实状态；不在前端私自补接口字段。

## 9. launcher

前置条件：`/healthz`、`/readyz`、`/api/session/launcher-token`、`/api/system/shutdown` 与状态流已稳定。
产出物：Windows Launcher 的环境检查、启动/停止、打开 Web UI、版本检查最小闭环。
暂不做什么：不复制 Web 业务逻辑；不维护独立状态模型；不自行解析用户配置作为在线管理源。

## 10. render service

前置条件：Render 动作 contract、错误码、`.deps/manifest.json`、浏览器资源准备策略已明确。
产出物：渲染任务队列、受控 Chromium 调度、模板 schema 校验、最小缓存与错误返回。
暂不做什么：不先做在线模板编辑器；不让插件各自实现浏览器截图链路；不跳过受控资源清单直接依赖宿主机浏览器。
