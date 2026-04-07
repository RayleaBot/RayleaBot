# Architecture Docs

本目录收纳 RayleaBot 的架构设计说明，帮助读者从子系统边界理解当前工程。

## 当前系统拓扑

RayleaBot 以 `server/` 为产品核心，主链路由以下子系统组成：

### 基础设施层
- **配置**（`internal/config`）：`config/user.yaml` 读取、`contracts/config.user.schema.json` 校验、20 个配置段、运行时热重载快照
- **存储**（`internal/storage`）：SQLite 读写分离双连接、WAL 模式、14 个迁移文件、12 张核心表
- **鉴权**（`internal/auth`）：管理员 bootstrap、HMAC-SHA256 session、滑动续期、多 session 上限
- **任务**（`internal/tasks`）：内存 Registry + SQLite 持久化、顺序执行器、取消与超时控制、订阅通知
- **日志**（`internal/logging`）：结构化 slog 输出、management_logs 持久化、retention 截断
- **密钥**（`internal/secrets`）：SQLite KV 密钥存储，session signing key 生命周期管理

### 适配器层
- **Adapter**（`internal/adapter`）：OneBot11 reverse WebSocket 接入、ready gating、重连 backoff、心跳超时、消息 / notice 最小归一化

### 运行时层
- **Bridge**（`internal/bridge`）：adapter 事件映射到 plugin protocol 事件
- **Runtime Manager**（`internal/runtime`）：per-plugin 子进程、`init / init_progress / init_ack`、`ping / pong`、`shutdown`、crash / backoff / dead_letter 状态机、本地 action RPC 处理
- **Dispatcher**（`internal/dispatch`）：订阅 fan-out、命令定向投递、per-plugin 投递队列与 worker goroutine
- **Scheduler**（`internal/scheduler`）：cron 表达式调度、SQLite 持久化、跨重启任务恢复、`scheduler.trigger` 投递

### 出站层
- **Outbound**（`internal/outbound`）：`message.send` / `message.reply`、shared message.segments 到 OneBot11 消息段数组映射
- **Plugin HTTP**（`internal/pluginhttp`）：`http.request` scoped by `http_hosts`、DNS 预检、SSRF 防护、受控私网例外

### 业务层
- **Plugins**（`internal/plugins`）：discovery（builtin / installed / examples）、catalog、install / uninstall、enable / disable / reload 生命周期、grants 管理、temporal grants 时效过滤
- **Permission**（`internal/permission`）：命令权限、blacklist、cooldown、cooldown reply
- **Plugin KV**（`internal/pluginkv`）：插件隔离 KV 持久化，含存储上限、plugin_data 文件区路径遍历防护
- **Plugin Config**（`internal/pluginconfig`）：插件配置 `config.read` / `config.write`
- **Plugin File**（`internal/pluginfile`）：`storage.file` scoped read / write / delete / list
- **Schema**（`internal/schema`）：JSON Schema 校验，服务于 manifest 静态验证
- **Redact**（`internal/redact`）：日志脱敏

### 管理面层
- **HTTP API**（`internal/httpapi`）：chi 路由注册、request ID 中间件、错误体序列化
- **Health**（`internal/health`）：`/healthz` 存活探测、`/readyz` 就绪探测（含 adapter / render 状态）
- **Console**（`internal/console`）：`/ws/plugins/{id}/console` 插件 stderr 实时流
- **Recovery**（`internal/recovery`）：`recovery-summary.json` 读写、recheck / confirm 任务路径
- **Render**（`internal/render`）：Chromium 渲染队列、模板注册、input schema 校验、artifact registry、`render.preview` 任务、资源诊断
- **Command**（`internal/command`）：聊天命令文本解析器，支持多前缀、最长匹配
- **CLI**（`internal/cli`）：`reset-admin`、`backup`、`restore`、`doctor`、`migrate`、`cleanup`

### 顶层装配
- **App**（`internal/app`）：平台层构建链、HTTP server 装配、路由注册、信号处理

### 外部界面
- **Web 管理面**（`web/`）：setup/login/session、系统状态、插件生命周期与 console、任务/日志/配置、render 预览、恢复操作与 shutdown
- **Launcher**（`launcher/`）：Electron 桌面启动器，loopback bootstrap auth、环境检查、server 启停、健康轮询、托盘管理、打开 Web UI

## 事件主链路

```
OneBot11 WS → adapter → bridge → dispatcher
                                     ├── plugin A runtime (subprocess, JSONL)
                                     ├── plugin B runtime
                                     └── ...
                                          ↓ local action RPC
                               outbound / storage / http / render / scheduler
                                     ↓ message result
                              adapter → OneBot11 WS
```

## 跨层边界规则

- adapter 只做平台协议适配，不直接写存储或业务状态。
- runtime manager 只通过正式 local action RPC surface 向基础设施发请求，不直接访问内部模块。
- 插件不得绕过 capability 校验、协议约束或 scoped path 限制访问平台内部。
- Web UI 消费管理面 HTTP / WebSocket，不解析日志推断状态。
- Launcher 复用 server management surface，不维护独立状态模型。

## 目录结构

| 文件 | 内容 |
|------|------|
| `README.md`（本文件） | 系统拓扑、子系统职责、跨层边界 |
| `state-model.md` | 插件 runtime 状态机、任务状态机、注册状态与 desired state |
| `event-model.md` | OneBot11 事件归一化、plugin protocol 事件模型、WebSocket 管理事件 |

## 阅读入口

- `contracts/`：对外接口、协议、schema、错误码正式来源
- `docs/engineering/baseline.md`：版本线、固定命令、冻结选型
- `server/README.md`：server 已接线能力完整列表
- `docs/plugin/`：插件 manifest、local action 边界、SDK 说明

## 维护规则

- 本目录解释职责分层、状态模型和跨层边界，不裁决对外字段、事件名与错误码。
- 文档中的运行链路、状态描述与能力范围须能回指到 `contracts/`、工程基线或已落地实现。
- 若某项能力仅有 contract 或工程骨架，本目录只描述边界，不当作可用能力记录。
