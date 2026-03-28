# Architecture Docs

本目录收纳 RayleaBot 的架构设计说明，帮助读者从子系统边界理解当前工程，而不是从实现细节反推产品结构。

## 当前系统拓扑

RayleaBot 以 `server/` 为产品核心，主链路由以下部分组成：

- 配置加载、schema 校验、SQLite 存储、鉴权、任务与插件目录装配
- OneBot11 reverse WebSocket adapter 与统一事件归一化
- per-plugin runtime manager、dispatcher fan-out、命令定向投递、scheduler `scheduler.trigger`
- management HTTP / WebSocket、task history、management log summary 持久化与历史回放
- Web 管理面，覆盖 setup/login/session、plugins/tasks/logs/config、plugin install / uninstall / grants / console 与 shutdown
- Electron 桌面启动器，覆盖 loopback launcher bootstrap、环境检查、server 启停、健康轮询、托盘关闭语义与打开 Web UI

Render Service 仍未进入完整实现阶段，本目录当前只描述其边界、依赖和进入条件。

## 阅读入口

- `contracts/`：对外接口、协议、schema、错误码的正式来源
- `docs/engineering/`：工程基线、固定命令、实施顺序
- `server/README.md`：当前 server 主链路与管理面能力
- `docs/plugin/`：插件 manifest、runtime 协议与能力边界

## 维护规则

- 本目录用于解释职责分层、状态模型和跨层边界，不裁决对外字段、事件名与错误码。
- 文档中的运行链路、状态描述与能力范围需要能回指到 `contracts/`、工程基线文件或已落地实现。
- 若某项能力仍只有 contract 或工程骨架，本目录只描述边界，不把它写成可用能力。
