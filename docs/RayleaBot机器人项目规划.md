# RayleaBot 机器人项目规划

RayleaBot 面向个人开发者和 GitHub 开源协作者，定位为自托管聊天机器人框架。
本文件是项目总纲，负责保留总目标、总边界、顶层架构、专题索引和版本路线图。细节设计分别维护在 `docs/` 各专题目录中。

## 一、产品目标与边界

### 1.1 项目定位

- RayleaBot 围绕聊天平台事件处理、插件扩展和可视化管理构建。
- 首个目标平台为 QQ，首个正式接入协议为 OneBot11。
- 项目以自托管部署为主，不依赖云端控制面板，不默认引入多租户和分布式架构。

### 1.2 目标用户

- 个人开发者
- 自用机器人部署者
- 希望在公开仓库下协作开发插件和扩展能力的开源贡献者

### 1.3 当前正式能力

- 支持 OneBot11 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook` 传输模式
- 支持单实例运行和基础生命周期管理
- 支持 Python / Node.js 插件运行能力
- 提供 Web 管理面板
- 提供平台级图片渲染服务
- 使用 SQLite 保存运行状态和管理数据
- 提供 Electron 桌面启动器

### 1.4 当前边界

- 多协议并行接入
- 分布式、多节点和高可用部署
- 正式插件市场与远程分发平台
- 强沙盒和完整资源配额控制
- Rust / Go 官方托管运行时支持
- 面向社区的长期兼容承诺

### 1.5 设计原则

- 先保证闭环可用，再扩展功能面
- 先保证职责边界清晰，再追求高度抽象
- 用户可编辑配置与系统运行状态分开存储
- 插件接口必须版本化，内部实现可以迭代
- 对外接口先稳定数据边界和协议边界，再稳定 UI 与实现细节

## 二、总体架构

### 2.1 总体架构图

```plain
               QQ / OneBot11
                     │
                     ▼
             Protocol Adapter
                     │
                     ▼
                  Bot Core
      ┌──────┬───────┼───────┬──────┬──────────────┐
      │      │       │       │      │              │
      ▼      ▼       ▼       ▼      ▼              ▼
  EventBus  Cmd    Perm   Plugin  Scheduler   Render Service
            Parser System Manager                  │
      │              │      │                      ▼
      │              │      ▼                 Render Engine
      │              │  Runtime Manager            │
      │              │      │                      ▼
      ▼              ▼      ▼                 Image Cache
   Web API        Plugins
      │
      ▼
    Web UI

Desktop Launcher
     │
      ├─ start / stop server
      ├─ env check / update check
      └─ open Web UI
```

### 2.2 顶层职责

| 子系统 | 职责 |
| --- | --- |
| Adapter | 对接聊天协议，接收事件、发送动作、归一化平台数据 |
| Bot Core | 负责事件分发、命令解析、聊天权限、配置读取、插件编排 |
| Plugin Runtime | 负责插件启动、监控、回收、协议转发和状态上报 |
| Render Service | 负责模板渲染、资源管理、缓存与渲染任务调度 |
| Web API / Web UI | 提供管理接口与可视化入口 |
| CLI | 提供本地离线恢复与运维入口 |
| Launcher | 提供本地进程启停、环境检查和打开 Web 入口 |

### 2.3 专题文档索引

| 目录 | 说明 |
| --- | --- |
| [`architecture/`](./architecture/README.md) | 内部设计、状态模型、事件模型、运行链路 |
| [`engineering/`](./engineering/README.md) | 工程基线、目录职责、阶段边界、质量门禁 |
| [`plugin/`](./plugin/README.md) | 插件生命周期、manifest、能力授权、协议与 SDK |
| [`user/`](./user/README.md) | 管理入口、配置、恢复、CLI 与部署 |
| [`release/`](./release/README.md) | 交付矩阵、升级回滚、验收口径与风险 |
| [`dev/`](./dev/README.md) | 仓库协作、诊断入口与文本资源规则 |

### 2.4 主题映射表

| 原主题 | 当前落点 |
| --- | --- |
| `3.1 协议适配层`、`3.2 内部统一事件模型` | [`architecture/event-model.md`](./architecture/event-model.md) |
| `3.3 状态模型` | [`architecture/state-model.md`](./architecture/state-model.md) |
| `3.4 Bot Core` | [`architecture/bot-core.md`](./architecture/bot-core.md) |
| `3.5 插件系统边界` | [`plugin/lifecycle.md`](./plugin/lifecycle.md) |
| `3.6 平台能力清单与插件声明` | [`plugin/capabilities-and-manifest.md`](./plugin/capabilities-and-manifest.md) |
| `3.7 插件通信协议规划` | [`plugin/protocol.md`](./plugin/protocol.md) |
| `3.8 图片渲染引擎` | [`architecture/render-service.md`](./architecture/render-service.md) |
| `3.9 Web 管理面板、Launcher 与安全边界` | [`user/management-surface.md`](./user/management-surface.md) |
| `3.10 配置、数据存储与日志` | [`architecture/platform-runtime.md`](./architecture/platform-runtime.md)、[`user/configuration.md`](./user/configuration.md) |
| `3.11 错误处理、恢复与运行约束` | [`architecture/platform-runtime.md`](./architecture/platform-runtime.md)、[`user/recovery.md`](./user/recovery.md) |
| `3.12 CLI 工具` | [`user/cli.md`](./user/cli.md) |
| `3.13 桌面启动器` | [`architecture/platform-runtime.md`](./architecture/platform-runtime.md)、[`user/management-surface.md`](./user/management-surface.md) |
| `3.14 兼容性与演进策略` | [`architecture/platform-runtime.md`](./architecture/platform-runtime.md) |
| `4.1 技术栈` | [`engineering/baseline.md`](./engineering/baseline.md)、[`engineering/tech-stack-evaluation.md`](./engineering/tech-stack-evaluation.md) |
| `4.2`、`4.3`、`4.4` | [`release/delivery-and-upgrade.md`](./release/delivery-and-upgrade.md) |
| `4.5 Docker 与本地部署` | [`user/deployment.md`](./user/deployment.md) |
| `4.6 Git 与忽略策略` | [`dev/repo-workflow.md`](./dev/repo-workflow.md) |
| `4.7 GitHub Actions 规划`、`4.9 测试策略` | [`engineering/quality-gates.md`](./engineering/quality-gates.md) |
| `4.8 可观测性与诊断` | [`dev/diagnostics.md`](./dev/diagnostics.md) |
| `4.10 文档体系规划` | 本文件与各目录 README |
| `4.11 国际化与文本资源规范` | [`dev/text-resources.md`](./dev/text-resources.md) |
| `4.12 风险与缓解措施`、`4.13 验收标准` | [`release/acceptance-and-risks.md`](./release/acceptance-and-risks.md) |

## 三、核心子系统专题入口

### 3.1 协议适配层

RayleaBot 以 OneBot11 作为当前正式接入协议，覆盖 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook` 传输模式；事件归一化、入站 / 出站消息模型和管理 WebSocket 事件统一收口到架构专题。
文档：[`architecture/event-model.md`](./architecture/event-model.md)

### 3.2 内部统一事件模型

平台把协议事件、插件协议消息和管理面事件统一整理为受控模型，字段裁决仍以 formal contract 为准。
文档：[`architecture/event-model.md`](./architecture/event-model.md)

### 3.3 状态模型

插件运行时、后台任务、授权时效和 OneBot11 连接状态使用统一状态模型，并投影到管理面。
文档：[`architecture/state-model.md`](./architecture/state-model.md)

### 3.4 Bot Core

Bot Core 负责事件分发、命令解析、聊天权限、调度、后台任务和服务级 ready 语义，是服务进程中的主控制层。
文档：[`architecture/bot-core.md`](./architecture/bot-core.md)

### 3.5 插件系统边界

插件平台覆盖 discovery roots、运行时支持、安装升级、热重载、卸载和数据目录边界。
文档：[`plugin/lifecycle.md`](./plugin/lifecycle.md)

### 3.6 平台能力清单与插件声明

插件 manifest、能力声明、命令声明和授权作用域继续以正式 schema 和 capability grant 模型维护。
文档：[`plugin/capabilities-and-manifest.md`](./plugin/capabilities-and-manifest.md)

### 3.7 插件通信协议规划

插件与平台之间继续使用 JSONL 协议，生命周期消息、事件消息和 local action RPC 收口在同一协议面。
文档：[`plugin/protocol.md`](./plugin/protocol.md)

### 3.8 图片渲染引擎

渲染服务是平台内建能力，负责模板渲染、Chromium 调度、artifact 管理和资源诊断。
文档：[`architecture/render-service.md`](./architecture/render-service.md)

### 3.9 Web 管理面板、Launcher 与安全边界

管理入口继续由 Web 管理面、Launcher 和 CLI 组成，日常管理以 Web 为主，Launcher 不承担第二套控制面。
文档：[`user/management-surface.md`](./user/management-surface.md)

### 3.10 配置、数据存储与日志

平台继续把用户可见配置、目录职责和运行根目录与内部配置 / 存储 / 日志实现分开维护。
文档：[`user/configuration.md`](./user/configuration.md)、[`architecture/platform-runtime.md`](./architecture/platform-runtime.md)

### 3.11 错误处理、恢复与运行约束

恢复摘要、兼容检查、人工处理和运行环境准备继续沿用同一条正式任务链路与诊断口径。
文档：[`user/recovery.md`](./user/recovery.md)、[`architecture/platform-runtime.md`](./architecture/platform-runtime.md)

### 3.12 CLI 工具

CLI 是本地离线恢复与运维入口，不承担常规在线管理职责。
文档：[`user/cli.md`](./user/cli.md)

### 3.13 桌面启动器

Launcher 继续作为 full artifact 的桌面入口，负责本地服务编排、本地启动前预检和打开 Web 管理面。运行时资源诊断由服务端 readiness 与 diagnostics 统一裁决，Launcher 直接展示结果。
文档：[`user/management-surface.md`](./user/management-surface.md)、[`architecture/platform-runtime.md`](./architecture/platform-runtime.md)

### 3.14 兼容性与演进策略

平台优先稳定统一事件模型、manifest、插件协议、能力授权和渲染接口，其余扩展按正式边界后置。
文档：[`architecture/platform-runtime.md`](./architecture/platform-runtime.md)

## 四、交付与工程化专题入口

### 4.1 技术栈

固定版本线、默认命令和冻结选型由 engineering 文档统一维护。
文档：[`engineering/baseline.md`](./engineering/baseline.md)、[`engineering/tech-stack-evaluation.md`](./engineering/tech-stack-evaluation.md)

### 4.2 构建与发布目标

正式产物矩阵、目录结构和发布元数据继续以 release 文档为准。
文档：[`release/delivery-and-upgrade.md`](./release/delivery-and-upgrade.md)

### 4.3 发布包结构规范

正式包根目录、产物矩阵和 release metadata sidecar 收口在 release 文档。
文档：[`release/delivery-and-upgrade.md`](./release/delivery-and-upgrade.md)

### 4.4 升级与回滚策略

升级、恢复、回滚和交付元数据之间的关系在 release 文档中统一说明。
文档：[`release/delivery-and-upgrade.md`](./release/delivery-and-upgrade.md)

### 4.5 Docker 与本地部署

本地交付、Docker、systemd 和 LXC 的正式路径收口在用户部署文档。
文档：[`user/deployment.md`](./user/deployment.md)

### 4.6 Git 与忽略策略

仓库跟踪边界、运行生成物与常规忽略项收口在开发文档。
文档：[`dev/repo-workflow.md`](./dev/repo-workflow.md)

### 4.7 GitHub Actions 规划

正式 CI 门禁、发布回归层次和默认验证命令收口在 engineering 质量门禁文档。
文档：[`engineering/quality-gates.md`](./engineering/quality-gates.md)

### 4.8 可观测性与诊断

`healthz`、`readyz`、诊断包、`doctor`、日志与恢复摘要的正式入口收口在开发诊断文档。
文档：[`dev/diagnostics.md`](./dev/diagnostics.md)

### 4.9 测试策略

测试层级、PR 门禁、发布门禁和长期巡检与 recovery drill 的关系收口在 engineering 质量门禁文档。
文档：[`engineering/quality-gates.md`](./engineering/quality-gates.md)

### 4.10 文档体系规划

当前文档采用总纲 + 专题目录结构：`architecture / engineering / plugin / user / release / dev`。
文档：[`architecture/README.md`](./architecture/README.md)、[`engineering/README.md`](./engineering/README.md)、[`plugin/README.md`](./plugin/README.md)、[`user/README.md`](./user/README.md)、[`release/README.md`](./release/README.md)、[`dev/README.md`](./dev/README.md)

### 4.11 国际化与文本资源规范

文本资源和国际化边界继续通过统一资源键组织。
文档：[`dev/text-resources.md`](./dev/text-resources.md)

### 4.12 风险与缓解措施

主要运行风险和缓解方向收口在 release 验收与风险文档。
文档：[`release/acceptance-and-risks.md`](./release/acceptance-and-risks.md)

### 4.13 验收标准与质量保障场景

发布门槛、关键场景和验收结论收口在 release 验收与风险文档。
文档：[`release/acceptance-and-risks.md`](./release/acceptance-and-risks.md)

## 五、版本路线图

### 当前稳定基础平台

- OneBot11 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook` 传输模式
- 单实例运行
- Python / Node.js 插件运行时
- 官方 Python / Node.js SDK
- 平台级图片渲染服务
- Web 管理面板
- 本地 CLI 工具
- SQLite 状态库
- Electron 桌面启动器
- 基础日志、配置管理、恢复与插件生命周期能力

### 管理与运行时完善

- 完善插件生命周期状态同步
- 完善配置迁移与局部热更新
- 增加更多诊断和调试能力
- 增加模板实时预览和更完整的管理体验

### 管理闭环与可信校验

- 权限策略工作区 `/permission-policy` 承担超级管理员、默认权限与命令冷却配置，黑白名单工作区 `/access-lists` 承担名单管理，`/commands` 保留命令治理读面
- 补齐插件可信来源校验，使安装任务、插件列表、插件详情和诊断面共享统一结果
- 补齐发布可信校验与受控更新引导，使 Web、Launcher 和 CLI 共享同一套升级与恢复路径

### 扩展生态阶段

- 评估插件市场或插件索引服务
- 评估插件间依赖解析
- 评估更强的运行时隔离与资源控制
- 评估多协议适配能力

## 补充结论

- RayleaBot 优先维护稳定、可管理、可扩展的自托管机器人基础平台。
- 插件系统是长期核心能力，当前正式运行时为 Python / Node.js。
- 统一事件模型、插件协议、manifest、配置边界、状态模型和渲染接口是最需要避免返工的部分。
- 启动器、管理面、配置系统、日志系统和恢复链路都是正式产品面的一部分，而不是附属能力。
