# Implementation Order

本文档定义 RayleaBot 的长期实施顺序与阶段边界，用于约束“什么先冻结、什么后接线、进入下一阶段前需要具备什么条件”。当前进度记录见 `../execution-plan.md`。

## 1. 契约文件补全

当前基础：

- `contracts/` 已具备 8 份 fixture-ready formal contracts，覆盖配置、错误码、管理 HTTP / WebSocket、插件 manifest、插件协议、release metadata 与 CLI。

进入本阶段时应继续遵守：

- 新的对外 surface 先进入 `contracts/`，并带上 `x-fixtures` 或等价引用。
- 字段、状态、错误码与事件名在 contract 冻结前不进入实现主链路。

暂不做什么：

- 不绕过 `contracts/` 在代码、README、fixtures 或 examples 中先写新接口。

## 2. fixtures / golden cases

当前基础：

- `fixtures/` 已覆盖 config、web-api、websocket、plugin-info、plugin-protocol、release-manifest 与 CLI。

进入本阶段时应继续遵守：

- 新增 contract 的同一轮变更中同步补齐最小 `ok` / `invalid` / `edge` 样例与 CI 校验。
- fixtures 只从正式 contract 派生，不反向放宽 contract。

暂不做什么：

- 不把 fixture 数据结构直接嵌进正式运行代码。

## 3. server 内核骨架

当前基础：

- server 已从骨架阶段进入真实主链路，当前具备入口、配置校验、健康检查、SQLite、鉴权、任务、插件目录、日志与管理面装配。

进入本阶段时应继续遵守：

- 核心内聚能力仍集中在 `server/`，新能力进入主链前先确认 contracts、baseline 与 migration 边界。
- storage、auth、tasks、logging 等基础设施继续作为后续阶段的共享底座。

暂不做什么：

- 不跨层把 Web UI、Launcher 或插件侧职责挪入 server 以外的第二套状态源。

## 4. adapter

当前基础：

- OneBot11 reverse WebSocket adapter 已接入 ready gating、重连、心跳、最小事件归一化与三种正式消息 action。

进入本阶段时应继续遵守：

- 新 adapter 或 richer action 进入实现前，先冻结对应 contract、错误码与统一事件语义。
- adapter 只负责平台协议适配、连接状态与事件映射，不直接写业务状态库。

暂不做什么：

- 不把 v0.1 范围外的多协议、多实例或更宽动作族直接写入当前主链。

## 5. plugin protocol bridge

当前基础：

- 当前主链已具备 per-plugin runtime manager、`init/init_ack`、`ping/pong`、`shutdown`、dispatcher fan-out、命令定向投递、scheduler `scheduler.trigger` 与 zero-gap reload。

进入本阶段时应继续遵守：

- 插件协议扩展先更新 `contracts/plugin-protocol.schema.json`、fixtures、示例插件与 SDK，再进入 runtime 主链。
- runtime、dispatcher 与 scheduler 仍只消费当前正式 `action` 集合和已冻结消息类型。

暂不做什么：

- 不在协议未冻结前补入额外 action、调试流或复杂流式回传。

## 6. config / storage

当前基础：

- 配置 schema 校验、SQLite migration、auth persistence、task persistence、plugin desired_state、grants、secret store、scheduler persistence 与日志持久化已经接入 server。

进入本阶段时应继续遵守：

- 配置、迁移、权限与存储结构变更先更新 contracts、baseline 和 migration，再进入业务路径。
- 聊天侧 permission / blacklist / cooldown 与 temporal grants 仍属于这一阶段的收尾项。

暂不做什么：

- 不让插件直写 `config/user.yaml`；不跳过 schema 校验直接消费配置或状态数据。

## 7. web api

当前基础：

- 管理 HTTP / WebSocket、setup/session、config、system status、tasks、logs、plugin lifecycle、grants 与 console 已进入真实路由和主链。

进入本阶段时应继续遵守：

- 新管理面能力先进入 OpenAPI / WebSocket contracts，再补 handler、鉴权、fixtures、示例与文档。
- CLI、Web UI、Launcher 共用同一套状态名、错误码与任务模型。

暂不做什么：

- 不在 handler 中私自发明字段；不把 CLI 或 Launcher 变成独立状态源。

## 8. web ui

前置条件：

- 管理 HTTP / WebSocket、状态枚举、错误码、任务流 contract 与 server 状态语义稳定到可消费。

产出物：

- Web UI 工程脚手架、登录壳、插件列表、任务流、日志流、配置页等最小管理面。

暂不做什么：

- 不通过解析日志推断真实状态；不在前端私自补接口字段。

## 9. launcher

前置条件：

- `healthz`、`readyz`、`session/launcher-token`、`system/status` 与 `system/shutdown` 的 server 行为稳定。

产出物：

- Windows Launcher 的环境检查、启动/停止、打开 Web UI、版本检查最小闭环。

暂不做什么：

- 不复制 Web 业务逻辑；不维护独立状态模型；不自行解析用户配置作为在线管理源。

## 10. render service

前置条件：

- Render 动作 contract、错误码、`.deps/manifest.json` 与浏览器资源准备策略明确。

产出物：

- 渲染任务队列、受控 Chromium 调度、模板 schema 校验、最小缓存与错误返回。

暂不做什么：

- 不先做在线模板编辑器；不让插件各自实现浏览器截图链路；不跳过受控资源清单直接依赖宿主机浏览器。
