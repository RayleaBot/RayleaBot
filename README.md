# RayleaBot

面向个人开发者和 GitHub 开源协作者的自托管聊天机器人框架。围绕聊天平台事件处理、插件扩展和可视化管理构建，全程数据自控。

## 项目亮点

- **自托管闭环** — 不依赖云端控制面板，从协议接入到插件执行到管理面板全部运行在本地
- **多语言插件系统** — 同时支持 Python 和 Node.js 插件运行时，子进程隔离，JSONL 协议通信
- **Bilibili 平台深度集成** — 内置三方账号管理、直播/动态实时监控，自研反风控体系（设备指纹、代理池、WBI 签名、验证码自动过检、brotli 直播流）
- **完整管理面** — Web 控制台覆盖仪表盘、插件管理、权限策略、任务调度、日志检索、渲染模板、配置热更新
- **桌面启动器** — Electron 原生壳层，本地进程编排、环境预检、一键启动
- **契约驱动** — 所有对外接口（HTTP / WebSocket / 事件 / 错误码）以 `contracts/` 为唯一正式来源，实现与 fixture 双向校验

## 架构概览

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
       │      │              │                      ▼
       │      │              ▼                 Render Engine
       │      │        Runtime Manager              │
       │      │              │                      ▼
       ▼      ▼              ▼                 Image Cache
    Web API  Plugins
       │
       ▼
     Web UI

 Desktop Launcher
      │
       ├─ start / stop server
       ├─ env check / update check
       └─ open Web UI
```

## 功能

### 核心平台

| 能力 | 说明 |
|------|------|
| 协议接入 | OneBot11 `reverse_ws` / `forward_ws` / `http_api` / `webhook` |
| 事件分发 | 统一事件模型，按命令声明和事件订阅 fan-out 到插件 |
| 插件运行时 | Python 3.12 / Node.js 24，子进程隔离，7 种生命周期状态 |
| 任务调度 | cron 表达式定时任务，插件内声明，管理面编辑 |
| 聊天权限 | 命令冷却、黑白名单、超级管理员与默认权限策略 |
| 渲染服务 | Chromium 模板渲染，artifact 管理与缓存 |
| 本地能力 | 插件通过 local action 调用平台 API（HTTP 请求、调度、存储等） |

### 管理控制台

| 页面 | 功能 |
|------|------|
| 仪表盘 | 连接状态、插件运行指标、恢复摘要、近期事件 |
| 插件管理 | 安装/卸载/启停、manifest 详情、设置页、命令面板 |
| 权限策略 | 超级管理员、默认权限、命令冷却 |
| 访问控制 | 用户/群组黑白名单 |
| 任务调度 | cron 任务创建、启停、日志标签 |
| 日志检索 | 按级别/插件/时间过滤，实时追加 + 历史回放 |
| 命令审计 | 命令使用统计与频次 |
| 系统配置 | 配置分组编辑、热更新、校验提示 |
| 渲染模板 | 模板上传、实时预览、变量编辑 |
| 协议管理 | OneBot11 连接配置、传输模式切换、兼容性检查 |
| 恢复中心 | 环境检查、兼容诊断、运行环境准备 |

### Bilibili 平台集成

| 能力 | 说明 |
|------|------|
| 三方账号 | CK 扫码登录、多账号轮转、凭据状态诊断 |
| 直播监控 | WebSocket 实时连接 + HTTP 备用检查，开播/下播事件推送 |
| 动态监控 | 动态轮询、去重推送、自动关注 |
| 反风控 | 设备指纹生成、UA 池轮转、代理池支持、WBI 签名、bili_ticket |
| 验证码 | v_voucher 检测、gaia-vgate 注册、geetest v4 无头绕过、grisk_id 持久化 |
| 直播流 | brotli 解压、直播心跳保活、buvid 注入 |

### 桌面启动器

- Electron 原生壳层，Windows / macOS / Linux 三端
- 本地服务进程编排与生命周期管理
- 环境预检（Node.js / Python / Chromium 可用性）
- 亮色 / 暗色双主题

## 快速开始

### 前置条件

| 依赖 | 版本 |
|------|------|
| Go | 1.25.8 |
| Node.js | 24.14.0 |
| pnpm | 10.32.1 |
| Python | 3.12.13 |
| Git | 2.x |

### 本地启动

```bash
# 克隆仓库
git clone https://github.com/RayleaBot/RayleaBot.git
cd RayleaBot

# Windows 一键启动
start.bat

# 跨平台启动
node scripts/start-dev.mjs

# Server 热重载（需要 air）
set RAYLEA_SERVER_RELOAD=air
node scripts/start-dev.mjs
```

启动后访问 `http://127.0.0.1:4173`，按引导完成初始设置。

### 生产构建

```bash
# Server
cd server
go build ./cmd/raylea-server

# Web
cd web
pnpm install --frozen-lockfile
pnpm build

# Launcher
cd launcher
pnpm install --frozen-lockfile
pnpm build
```

## 项目结构

```
RayleaBot/
├── server/                  # Go 服务端
│   ├── cmd/raylea-server/   # 主入口
│   └── internal/            # 内核实现
│       ├── adapter/         # OneBot11 协议适配
│       ├── app/             # 路由与 HTTP handler
│       ├── bilibili/        # Bilibili 源与反风控
│       ├── dispatch/        # 事件分发
│       ├── localaction/     # 插件本地能力
│       ├── pluginconfig/    # 插件配置
│       ├── pluginhttp/      # 插件 HTTP 代理
│       ├── recovery/        # 恢复与诊断
│       ├── render/          # 渲染引擎
│       ├── runtime/         # 插件运行时
│       ├── scheduler/       # 任务调度
│       ├── secrets/         # 密钥存储
│       ├── sqlcqueries/     # SQL 查询
│       ├── storage/         # SQLite 存储层
│       └── thirdparty/      # 三方账号服务
├── web/                     # Vue 3 管理控制台
│   └── src/
│       ├── components/      # 通用组件
│       ├── locales/         # 国际化文本
│       ├── router/          # 路由配置
│       ├── stores/          # Pinia 状态管理
│       ├── styles/          # 全局样式与设计 token
│       ├── types/           # TypeScript 类型（含自动生成）
│       └── views/           # 页面视图
├── launcher/                # Electron 桌面启动器
│   └── src/
│       ├── main/            # 主进程与服务编排
│       ├── preload/         # 受限 IPC 桥接
│       └── renderer/        # React 渲染层
├── sdk/                     # 插件 SDK
│   ├── nodejs/              # Node.js SDK
│   └── python/              # Python SDK
├── contracts/               # 正式接口契约（唯一真相来源）
├── fixtures/                # 契约配套测试数据
├── docs/                    # 架构与工程文档
│   ├── architecture/        # 架构设计
│   ├── engineering/         # 工程基线
│   ├── plugin/              # 插件系统
│   ├── user/                # 用户指南
│   ├── release/             # 发布与交付
│   └── dev/                 # 开发协作
├── templates/               # 渲染模板
├── scripts/                 # 构建与发布脚本
└── .agents/                 # AI 协作工作流
```

## 开发

### 运行测试

```bash
# Server
cd server && go test ./...

# Web
cd web && pnpm test

# Web E2E
cd web && pnpm test:e2e

# Launcher
cd launcher && pnpm test

# Node.js SDK
cd sdk/nodejs && node --test tests/*.test.mjs

# Python SDK
cd sdk/python && python -m unittest discover -s tests
```

### 代码生成

```bash
# Web — OpenAPI 类型 + WebSocket 事件类型
cd web && pnpm generate:types

# Launcher — OpenAPI 类型
cd launcher && pnpm generate:types

# Server — SQL 查询生成物校验
cd server && go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 && sqlc diff
```

### 契约体系

项目采用 contract-first 开发模式。修改任何对外接口时：

1. 先更新 `contracts/` 中的对应契约文件
2. 同步更新 `fixtures/` 中的测试数据
3. 运行代码生成命令更新类型文件
4. 再进入实现代码

`contracts/` 目录下的文件是接口行为的唯一正式裁决来源。

## 插件系统

插件以子进程形式运行，通过 stdin/stdout JSONL 协议与平台通信。平台通过 capability grant 控制插件的能力授权边界。

```
插件子进程
    ↕ JSONL (stdin/stdout)
Runtime Manager
    ↕
Dispatcher → EventBus → Adapter → 聊天平台
    ↕
Local Action Service → 平台能力（HTTP / 存储 / 调度 / 渲染）
```

插件 manifest 声明插件元信息、能力需求和命令订阅。平台在安装时校验 manifest，运行时通过 capability token 限制插件可用的平台能力。

相关文档：[`docs/plugin/`](./docs/plugin/)

## 技术栈

| 层面 | 选型 |
|------|------|
| 后端语言 | Go 1.25 |
| HTTP 路由 | `net/http` + `go-chi/chi` |
| WebSocket | `github.com/coder/websocket` |
| 数据库 | SQLite (`modernc.org/sqlite`) |
| 前端框架 | Vue 3.5 + TypeScript |
| 构建工具 | Vite 8 |
| UI 组件库 | Ant Design Vue 4 |
| 样式方案 | Tailwind CSS 4 + Sass + CSS Variables |
| 状态管理 | Pinia 3 |
| 桌面框架 | Electron 41 + React 18 + Fluent UI |
| 插件 SDK | Python 3.12 / Node.js 24 |
| 渲染引擎 | chromedp + Chromium |
| 运行指标 | Prometheus |

完整版本线见 [`docs/engineering/baseline.md`](./docs/engineering/baseline.md)。

## 文档索引

| 文档 | 说明 |
|------|------|
| [项目规划](./docs/RayleaBot机器人项目规划.md) | 产品目标、架构、路线图 |
| [工程基线](./docs/engineering/baseline.md) | 版本线、工程选型、目录职责 |
| [架构总览](./docs/architecture/README.md) | 内部设计、事件模型、状态模型 |
| [插件文档](./docs/plugin/README.md) | 生命周期、manifest、协议、SDK |
| [用户指南](./docs/user/README.md) | 部署、配置、恢复、CLI |
| [发布文档](./docs/release/README.md) | 交付矩阵、升级回滚、验收 |
| [开发协作](./docs/dev/README.md) | 仓库工作流、诊断、文本资源 |

## License

MIT
