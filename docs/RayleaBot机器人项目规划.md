# RayleaBot 机器人项目规划

RayleaBot 面向个人开发者和 GitHub 开源协作者，定位为自托管聊天机器人框架。项目首阶段聚焦 QQ / OneBot11 场景，以单实例、低门槛、可扩展、易维护为原则，先完成一个稳定可用的 v0.1 闭环，再逐步扩展协议、插件生态和分发能力。

## 一、产品目标与边界

### 1.1 项目定位

- RayleaBot 是一个围绕聊天平台事件处理、插件扩展和可视化管理构建的机器人系统。
- 首个目标平台为 QQ，首个接入协议为 OneBot11。
- 项目以自托管部署为主，不依赖云端控制面板，不默认引入多租户和分布式架构。

### 1.2 目标用户

- 个人开发者。
- 自用机器人部署者。
- 希望在公开仓库下协作开发插件和扩展能力的开源贡献者。

### 1.3 v0.1 核心目标

- 支持 OneBot11 反向 WebSocket 单协议接入。
- 支持单实例运行和基础生命周期管理。
- 提供 Python / Node.js 两类插件运行能力。
- 提供 Web 管理面板，用于查看状态、管理插件、查看日志、修改基础配置。
- 提供平台级图片渲染服务，用于帮助菜单、状态面板、排行榜和信息卡片等图片化输出。
- 使用 SQLite 保存运行状态和管理数据。
- 提供 Electron 桌面启动器，承担基础启动与管理入口。

### 1.4 暂不纳入 v0.1 的范围

- 多协议并行接入。
- 正向 WebSocket、HTTP 上报、HTTP API 组合等其他 OneBot11 传输模式（v0.1 强制仅支持反向 WebSocket；配置错误时启动前明确拒绝）。
- 分布式、多节点、高可用部署。
- 正式插件市场与远程分发平台。
- 强沙盒和完整资源配额控制。
- Rust / Go 官方运行时支持。
- 面向社区的长期兼容承诺。

### 1.5 设计原则

- 先保证闭环可用，再扩展功能面。
- 先保证职责边界清晰，再追求高度抽象。
- 用户可编辑配置与系统运行状态分开存储。
- 插件接口必须版本化，内部实现可以迭代。
- 早期版本默认不承诺完全向后兼容，但要尽量减少无意义破坏。
- 对外接口要优先稳定数据边界和协议边界，再稳定 UI 与实现细节。

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
      │              │    (embedded)                ▼
      │              │      │                 Image Cache
      ▼              ▼      ▼
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

注：

- v0.1 的 `Plugin Runtime` 已内嵌为 `raylea-server` 内部的 Runtime Manager，不再作为独立二进制或独立顶层服务存在。
- `Desktop Launcher` 正式支持 `windows-x64`、`linux-x64` 与 `macos-arm64`；Linux 端同时保留 `raylea-server` + `systemd` 的 server-only 交付形态。

### 2.2 核心职责划分

| 子系统 | 职责 |
| --- | --- |
| Adapter | 对接聊天协议，接收事件、发送动作、归一化平台数据 |
| Bot Core | 核心调度层，负责事件分发、Command Parser（命令解析与路由）、Permission System（Chat）（聊天侧用户权限判定）、Capability Grant Manager（插件能力授权）、配置读取、插件编排 |
| Plugin Runtime | 作为 `server` 内部 Runtime Manager 装载插件进程，负责启动、监控、回收、协议转发和状态上报 |
| Render Service | 平台统一图片渲染能力，负责模板解析、资源管理、缓存与渲染任务调度 |
| Web API | 提供管理接口与实时推送能力 |
| Web UI | 提供可视化管理入口 |
| CLI | 提供本地离线恢复与运维命令入口，如 `reset-admin`、`backup`、`restore`、`doctor`、`migrate`，不承担常规在线管理面职责 |
| Launcher | 提供本地进程启停、环境完整性检查（含 `.deps/`、Chromium、渲染模板资源）、新版本提示和打开 Web 入口（详见 3.13） |

说明：

- v0.1 默认将 `Plugin Runtime` 以内嵌 Runtime Manager 的形式实现于 `server` 内部，不单独拆分为独立二进制；如后续隔离和部署诉求显著提升，再评估抽离。
- `Plugin Runtime` 不承担完整环境安装器职责，只负责检查并调用发行包或启动器已经准备好的运行时。
- 图片渲染能力由平台统一提供，不由插件各自实现浏览器截图、Canvas 布局或自绘模板逻辑。
- `Launcher` 不复制 Web 管理逻辑，服务启动后以 Web API / WebSocket 作为主要管理通道。

### 2.3 开发目录结构

```plain
RayleaBot
│
├─ server
│  ├─ cmd
│  │  └─ raylea-server
│  │     └─ main.go
│  ├─ internal
│  │  ├─ adapter
│  │  │  └─ onebot11
│  │  ├─ api
│  │  ├─ config
│  │  ├─ core
│  │  ├─ event
│  │  ├─ permission
│  │  ├─ plugin
│  │  ├─ render
│  │  │  ├─ api
│  │  │  ├─ assets
│  │  │  ├─ engine
│  │  │  │  ├─ browser
│  │  │  │  └─ cache
│  │  │  ├─ schema
│  │  │  └─ templates
│  │  ├─ runtimebridge
│  │  ├─ runtimemanager
│  │  ├─ scheduler
│  │  └─ storage
│  └─ pkg
├─ plugins
│  ├─ builtin             # 官方内置插件，纳入版本控制
│  ├─ installed           # 用户安装插件
│  └─ dev                 # 本地开发调试插件
├─ web
├─ launcher
├─ config
├─ data
├─ cache
│  ├─ render
│  └─ downloads
├─ logs
├─ .deps
├─ Dockerfile
└─ docker-compose.yml
```

### 2.4 关键运行流程

1. Adapter 接收来自 OneBot11 的事件并转换为统一事件模型。
2. Bot Core 将事件写入 EventBus，并执行基础过滤和上下文补全（填充 `actor.nickname`、`actor.role`、`target.name` 等可选字段）。
3. Command Parser 检查消息是否匹配已配置的命令前缀；若匹配，提取命令名和参数列表写入 `payload.command` 和 `payload.args`（详见 3.4.5）。
4. Permission System 执行权限判定：检查黑名单、验证用户权限级别、执行用户侧冷却限流（详见 3.4.6、3.11.2）；不满足条件的事件在此阶段被拦截，不进入插件。
5. Plugin Manager 根据启用状态和订阅关系，将事件转发给 `server` 内部的 Plugin Runtime Manager。
6. Plugin Runtime Manager 将事件投递到目标插件子进程，并收集插件动作、结果和错误。
7. 当插件需要图片化输出时，Bot Core 调用 Render Service，根据模板与数据生成或复用缓存图片。
8. Bot Core 根据插件动作调用 Adapter 执行发消息，或将渲染结果交给后续消息动作处理。
9. Web API 暴露系统状态、插件状态、渲染状态、配置和日志流；Launcher 在服务运行后优先复用这些接口。

## 三、核心子系统设计

### 3.1 协议适配层

- v0.1 只实现 OneBot11。
- 适配层负责协议连接、事件解析、消息发送、状态探测，不承担业务逻辑。
- Adapter 对外输出统一事件结构，例如消息事件、通知事件、请求事件、生命周期事件。
- 统一事件模型要屏蔽不同平台的命名差异，保证插件侧尽量不依赖平台私有字段。
- OneBot11 模块建议保留 `client.go`、`event_parser.go`、`message_sender.go` 等拆分结构。

#### 3.1.1 OneBot11 传输方式约束

- v0.1 仅支持 OneBot11 反向 WebSocket 传输模式，作为唯一正式接入方式。
- 正向 WebSocket、HTTP 上报、HTTP API 组合模式和其他混合部署方式不进入首版交付范围。
- 鉴权、心跳、重连、连接状态展示和部署说明都以反向 WebSocket 语义为准，不为未支持的传输方式预留隐式兼容分支。
- 如用户配置了 v0.1 未支持的 OneBot11 传输方式，服务应在启动前给出明确配置错误，而不是在运行期静默失败。

**反向 WebSocket 鉴权细节**：

当 `config/user.yaml` 中配置了 `access_token` 时，Adapter 在建立反向 WebSocket 连接时应附加鉴权信息。支持以下两种方式：

- **Header 方式（推荐）**：在 WebSocket 握手请求中添加 `Authorization: Bearer <access_token>` 请求头。
- **Query 参数方式（兼容）**：在连接 URL 中附加 `?access_token=<token>` 查询参数。适用于不支持自定义 Header 的 OneBot11 实现。

鉴权处理规则：

- Adapter 默认使用 Header 方式。
- 若 `access_token` 为空字符串或未配置，Adapter 不附加任何鉴权信息，但应记录 `WARN` 级别日志提示安全风险。
- 鉴权失败（OneBot11 端返回 HTTP 401 或 403）时，Adapter 将连接状态设为 `auth_failed`（见 3.3.3），并映射为 `adapter.auth_failed` 错误码（见 3.11.3）。
- 鉴权失败不触发自动重连，需用户检查 Token 配置后手动重连或重启服务。

**连接参数与实例边界**：

- `adapter.connect_timeout_seconds` 默认建议为 `15`，用于约束单次反向 WebSocket 握手与首包等待时间；超时后进入重连退避流程。
- `adapter.reconnect_initial_seconds = 2`、`adapter.reconnect_multiplier = 2`、`adapter.reconnect_max_seconds = 120`、`adapter.reconnect_jitter_ratio = 0.2` 作为 v0.1 默认重连参数，避免网络恢复时出现无抖动的集中重连风暴。
- `adapter.reconnect_jitter_ratio` 作用于每次退避结果的上下浮动窗口；实现可按 `base_delay * (1 +/- jitter)` 生成最终等待值，但不得突破 `reconnect_max_seconds` 上限。
- v0.1 仅支持一个 OneBot11 上游实例连接，`config/user.yaml` 中只维护一组 `endpoint`、`access_token` 与等价身份配置；不支持多 OneBot 实例并行接入、自动故障切换或按群分流。
- 若未来扩展多实例接入，必须同时引入多 `bot_id` 路由、状态隔离、Web UI 多实例视图和审计模型；v0.1 不做隐式预留。

#### 3.1.2 OneBot11 事件类型映射表

Adapter 负责将 OneBot11 原始事件转换为统一事件模型的 `event_type`。以下为完整映射关系：

| OneBot11 字段组合 | 统一 `event_type` | v0.1 支持 |
| --- | --- | --- |
| `post_type=message, message_type=private` | `message.private` | 是 |
| `post_type=message, message_type=group` | `message.group` | 是 |
| `post_type=notice, notice_type=group_increase` | `notice.member_increase` | 是 |
| `post_type=notice, notice_type=group_decrease` | `notice.member_decrease` | 是 |
| `post_type=notice, notice_type=group_ban` | `notice.group_ban` | 否（v0.2） |
| `post_type=notice, notice_type=friend_add` | `notice.friend_add` | 否（v0.2） |
| `post_type=notice, notice_type=group_admin` | `notice.group_admin` | 否（v0.2） |
| `post_type=notice, notice_type=notify, sub_type=poke` | `notice.poke` | 否（v0.2） |
| `post_type=notice, notice_type=group_recall` | `notice.group_recall` | 否（v0.2） |
| `post_type=notice, notice_type=friend_recall` | `notice.friend_recall` | 否（v0.2） |
| `post_type=notice, notice_type=group_upload` | `notice.group_upload` | 否（v0.2） |
| `post_type=notice, notice_type=group_card` | `notice.group_card` | 否（v0.2+） |
| `post_type=notice, notice_type=essence` | `notice.essence` | 否（v0.2+） |
| `post_type=request, request_type=friend` | `request.friend` | 否（v0.2） |
| `post_type=request, request_type=group` | `request.group` | 否（v0.2） |
| `post_type=notice, notice_type=notify, sub_type=honor` | `notice.honor` | 否（v0.2+） |
| `post_type=notice, notice_type=notify, sub_type=lucky_king` | `notice.lucky_king` | 否（v0.2+） |
| `post_type=notice, notice_type=notify, sub_type=title` | `notice.group_title` | 否（v0.2+） |
| `post_type=notice, notice_type=group_msg_emoji_like` | `notice.msg_emoji_like` | 否（v0.2+） |
| `post_type=notice, notice_type=notify, sub_type=profile_like` | `notice.profile_like` | 否（v0.2+） |
| `post_type=notice, notice_type=group_dismiss` | `notice.group_dismiss` | 否（v0.2+） |
| `post_type=meta_event, meta_event_type=heartbeat` | `meta.heartbeat` | 是 |
| `post_type=meta_event, meta_event_type=lifecycle` | `meta.lifecycle` | 是 |

说明：

- v0.1 优先支持消息类事件（`message.group`、`message.private`）和基础通知事件（成员增减），这是插件闭环的最小集。
- `meta.heartbeat` 和 `meta.lifecycle` 用于连接状态管理，不投递给普通插件。
- 未支持的事件类型在 v0.1 中由 Adapter 静默丢弃并记录 `DEBUG` 日志，不产生运行时错误。
- 映射表应作为 Adapter 实现的唯一参考来源，避免散落在多处代码中。

**`self_id` 字段处理说明**：

- 每一个 OneBot11 事件都携带 `self_id` 字段（接收事件的机器人 QQ 号），Adapter 应读取该字段并与 `get_login_info` API 返回的 `bot.id` 校验一致性。
- v0.1 单实例部署下，`self_id` 校验失败时记录 `WARN` 级别日志（不阻断事件处理），用于排查多机器人实例误连同一 WebSocket 的问题。
- `self_id` 不写入统一事件模型顶层字段，插件侧通过 `init` 消息中的 `bot.id` 获取机器人身份信息。

**v0.1 通知事件 `payload` 结构说明**：

v0.1 支持的通知事件的 `payload` 字段包含以下信息（从 OneBot11 事件顶层字段映射）：

| 统一 `event_type` | `payload` 关键字段 | 来源 |
| --- | --- | --- |
| `notice.member_increase` | `sub_type`（`approve` 管理员同意 / `invite` 被邀请）、`operator_id`（操作者 ID，转 `string`） | OneBot11 事件顶层字段 |
| `notice.member_decrease` | `sub_type`（`leave` 主动退群 / `kick` 被踢 / `kick_me` bot 被踢）、`operator_id`（操作者 ID，转 `string`） | OneBot11 事件顶层字段 |

v0.2 预留事件的 `payload` 参考结构：

| 统一 `event_type` | `payload` 关键字段 | 说明 |
| --- | --- | --- |
| `notice.group_recall` | `operator_id`（操作者 ID）、`message_id`（被撤回的消息 ID） | 撤回操作 |
| `notice.friend_recall` | `message_id`（被撤回的消息 ID） | 好友消息撤回 |
| `notice.group_upload` | `file`（含 `id`、`name`、`size`、`url`） | 上传文件的基础信息 |

说明：

- 通知事件的 `actor` 映射为事件触发者（如入群的用户、被踢的用户），`operator_id` 保留在 `payload` 中用于区分操作者（如踢人的管理员与被踢的用户不是同一人）。
- 通知事件的 `message` 字段为 `null`（非消息类事件），业务数据统一放入 `payload`。

**`meta.lifecycle` 事件处理说明**：

- OneBot11 的 `meta.lifecycle` 事件包含 `sub_type` 字段，值为 `enable`（WebSocket 连接启用）或 `disable`（连接禁用）。
- Adapter 收到 `sub_type=enable` 的 lifecycle 事件后，应将连接状态从 `connected` 切换为 `authenticated`（见 3.3.3 状态流转规则），确认协议层已可用。
- `sub_type=disable` 在 v0.1 中记录 `WARN` 日志，不主动断开连接（由后续心跳超时或连接层错误自然触发重连）。
- `meta.lifecycle` 事件不投递给插件，仅用于 Adapter 内部状态管理。

#### 3.1.3 Adapter 内部 API 调用清单

Adapter 在事件归一化和消息发送过程中，需要内部调用以下 OneBot11 API。本清单仅限 Adapter 内部使用，不直接对插件开放（插件侧 API 能力见 3.6.1）。

| OneBot11 API | 用途 | 调用时机 | v0.1 支持 |
| --- | --- | --- | --- |
| `get_login_info` | 获取机器人自身 QQ 号和昵称 | 连接建立后调用一次，缓存结果 | 是 |
| `get_group_member_info` | 获取群成员详情（角色、名片等） | `sender.role` 缺失时查询回填 `actor.role` | 是 |
| `get_group_info` | 获取群信息（群名等） | `identity_groups` 缓存未命中时查询回填 `target.name` | 是 |
| `get_stranger_info` | 获取陌生人信息（昵称等） | 私聊场景下 `identity_users` 缓存未命中时查询 | 是 |
| `send_msg` | 发送消息（私聊/群聊） | 插件通过 `message.reply` 或 `message.send` 动作触发 | 是 |
| `delete_msg` | 撤回消息 | v0.2 消息撤回功能预留 | 否（v0.2） |
| `get_msg` | 获取消息详情（含完整消息段和 sender 信息） | v0.2 消息撤回确认、上下文补全预留 | 否（v0.2） |

说明：

- 上述 API 的查询结果应与 `identity_users` / `identity_groups` / `identity_group_members` 缓存配合（见 3.10.4），避免对 OneBot11 端产生高频重复请求。
- `get_group_member_info` 和 `get_group_info` 的缓存策略参见 3.10.4 中的 `identity_users`、`identity_groups` 与 `identity_group_members` 表说明。
- API 调用通过反向 WebSocket 的 `echo` 请求-响应配对机制完成（见 3.1.4）。

#### 3.1.4 WebSocket 请求-响应配对机制

Adapter 通过反向 WebSocket 调用 OneBot11 API 时，必须使用 `echo` 字段配对请求和响应。OneBot11 的反向 WebSocket 是一条共享通道，既用于接收事件推送，也用于发送 API 请求和接收 API 响应。

**请求格式**：

```json
{
  "action": "get_group_member_info",
  "params": {
    "group_id": 123456,
    "user_id": 654321
  },
  "echo": "adapter-req-001"
}
```

**响应格式**：

```json
{
  "status": "ok",
  "retcode": 0,
  "data": {
    "user_id": 654321,
    "nickname": "用户昵称",
    "card": "群名片",
    "role": "member"
  },
  "echo": "adapter-req-001"
}
```

**事件与响应区分规则**：

- 收到的 JSON 消息中包含 `echo` 字段的为 API 响应，应根据 `echo` 值匹配对应的 pending request。
- 不包含 `echo` 字段的为事件推送，走正常的事件解析和投递流程。

**内部实现要求**：

- Adapter 维护一个 pending request map（`map[string]chan Response`），以 `echo` 值为 key。
- 每个请求使用唯一的 `echo` 值（建议格式：`adapter-{自增ID}` 或 `adapter-{uuid}`）。
- 请求超时默认 30 秒，超时后从 pending map 中移除并返回超时错误。
- 收到响应时从 pending map 中取出对应 channel 并投递，若 `echo` 值无对应 pending request 则记录 `WARN` 并丢弃。
- **断线清理**：当 WebSocket 连接断开（状态切换为 `reconnecting` 时），Adapter 必须立即清空 pending map 中所有挂起请求，向每个等待方返回 `adapter.connection_lost` 错误，而不是任由其在内存中继续等待超时。这既避免内存泄漏，也防止重连后旧请求与新会话的响应发生交叉污染。

**OneBot11 API 响应字段说明**：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `status` | `string` | 响应状态：`ok`（成功）、`failed`（失败）、`async`（异步处理） |
| `retcode` | `int` | 返回码：`0` 为成功，非零为各类错误 |
| `data` | `object/null` | 响应数据，具体结构取决于请求的 API |
| `message` | `string` | 错误信息（`status=failed` 时） |
| `wording` | `string` | 错误信息的人类可读描述（部分实现提供） |
| `echo` | `any` | 请求时传入的 `echo` 值原样返回 |

**retcode 到平台错误码映射**：

| OneBot11 `retcode` | 含义 | 平台错误码（见 3.11.3） |
| --- | --- | --- |
| `0` | 成功 | —（无错误） |
| `1` | 已提交异步处理 | —（非错误，记录 `INFO` 日志） |
| `1400` | 请求参数无效 | `adapter.send_failed` |
| `1404` | 目标不存在（如群/用户不存在） | `adapter.send_failed` |
| 其他非零值 | 未知错误 | `adapter.send_failed` |

说明：

- Adapter 以 `status == "failed"` 作为主要失败判据，`retcode` 作为辅助信息写入日志，便于调试。
- `send_msg` 成功时，`data.message_id` 为新消息的 ID，Adapter 应将其返回给发起发送请求的插件（通过 API 响应的 `result.data.message_id`）。
- `status == "async"` 表示请求已被 OneBot11 端接受但尚未完成，v0.1 将其视为成功处理。

### 3.2 内部统一事件模型

#### 3.2.1 统一事件模型

统一事件模型建议至少包含以下字段：

| 字段 | 说明 |
| --- | --- |
| `event_id` | 平台无关的事件唯一标识 |
| `source_protocol` | 来源协议，如 `onebot11` |
| `source_adapter` | 来源适配器，如 `adapter.onebot11` |
| `event_type` | 统一事件类型，如 `message.group`、`notice.member_increase` |
| `actor` | 触发者信息，如用户 ID、昵称、角色 |
| `target` | 目标上下文，如群、频道、私聊会话 |
| `payload` | 归一化业务载荷，适用于非消息类事件或通用扩展字段 |
| `message` | 归一化后的消息体，含消息段数组、纯文本摘要和附件引用 |
| `raw_payload` | 原始协议载荷，仅保留在 Core / Adapter / 调试链路中，默认不投递给普通插件；插件需额外声明并获授 `event.raw_payload` 能力后才可接收 |
| `timestamp` | 事件时间戳 |

设计要求：

- 本节是平台内部事件结构的唯一正式来源，其他章节只引用，不再重复发明事件字段集。
- 协议事件、系统内部事件、插件事件都应尽量落到统一事件结构后再进入 EventBus。
- 插件优先消费统一字段，减少与协议强耦合。
- `message` 主要服务消息类事件，其他类型事件优先放入 `payload`。
- `raw_payload` 保留给调试和高级扩展使用，不鼓励作为主业务输入；Plugin Runtime 在默认插件事件序列化阶段应剔除该字段。
- 如插件确实需要底层协议原始载荷，应额外声明并获授 `event.raw_payload` 相关敏感权限。
- 统一事件模型是 SDK、日志和调试工具的共同基础，应尽早稳定。

**`payload` 字段约定**：

- 消息类事件（`message.group`、`message.private`）的 `payload` 至少包含：
  - `message_id`：OneBot11 分配的消息标识（通常为整数，转为 `string`），用于引用回复和 v0.2 撤回操作。全链路说明见 3.2.3。
  - `command`：若消息匹配命令前缀，填入命令名（不含前缀）；否则为 `null`。解析规则见 3.4.5。
  - `args`：命令参数数组；非命令消息为空数组。
- 通知类事件的 `payload` 按事件类型携带不同字段，具体结构见 3.1.2 的事件 payload 说明。
- 内部事件的 `payload` 由产生事件的子系统定义，如 `scheduler.trigger` 的 `payload` 包含插件注册时传入的自定义数据（见 3.2.4）。

#### 3.2.2 `actor` 与 `target` 字段结构

`actor` 和 `target` 是统一事件模型中描述事件触发者和目标上下文的正式字段。所有引用这两个字段的章节（包括 3.4.6 权限判定、3.7.5 事件投递示例等）均以本节定义为准。

**`actor`（事件触发者）字段表**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | `string` | 是 | 触发者用户 ID（如 QQ 号） |
| `nickname` | `string` | 否 | 触发者昵称，来源为 `identity_users` 缓存或协议事件携带的昵称字段 |
| `role` | `string` | 否 | 触发者在当前上下文中的角色：`owner`（群主）、`admin`（群管理员）、`member`（普通成员）；私聊场景为 `null` |

**`target`（目标上下文）字段表**：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `type` | `string` | 是 | 目标类型：`group`（群聊）、`private`（私聊） |
| `id` | `string` | 是 | 目标 ID（群号或私聊用户 ID） |
| `name` | `string` | 否 | 目标名称（群名等），来源为 `identity_groups` 缓存或协议事件携带的群名字段 |

说明：

- `actor.role` 反映的是用户在**当前消息上下文**中的群内角色，与 3.4.6 中的权限系统配合使用。超级管理员的判定不依赖 `actor.role`，而是通过 `admin.super_admins` 配置列表匹配 `actor.id`。
- 可选字段（`nickname`、`role`、`name`）由 Adapter 在归一化阶段尽力填充；如协议事件未携带相关信息，则为 `null`。
- 非消息类事件（如 `meta.heartbeat`）的 `actor` 和 `target` 可能为 `null`。
- 内部事件（如 `scheduler.trigger`）的 `actor` 和 `target` 均为 `null`（见 3.2.4）。

**OneBot11 `sender` 对象到 `actor` 的映射规则**：

OneBot11 消息事件的 `sender` 对象包含触发者详细信息，Adapter 按以下规则映射到统一事件模型的 `actor` 字段：

| OneBot11 `sender` 字段 | 统一 `actor` 字段 | 映射规则 |
| --- | --- | --- |
| `sender.user_id` | `actor.id` | 转换为 `string` 类型 |
| `sender.card` / `sender.nickname` | `actor.nickname` | 优先使用 `sender.card`（群名片）；`card` 为空字符串或缺失时回退到 `sender.nickname` |
| `sender.role` | `actor.role` | 直接映射：`owner`、`admin`、`member`；私聊事件中 `sender.role` 缺失，`actor.role` 设为 `null` |

**OneBot11 事件到 `target` 的映射规则**：

| 消息类型 | `target.type` | `target.id` 来源 | `target.name` 来源 |
| --- | --- | --- | --- |
| 群消息（`message_type=group`） | `group` | 事件顶层 `group_id`，转 `string` | `identity_groups` 缓存（见 3.10.4） |
| 私聊消息（`message_type=private`） | `private` | 事件顶层 `user_id`，转 `string` | `identity_users` 缓存或 `sender.nickname` |

**补充说明**：

- 当 `sender.role` 缺失且为群消息场景时，Adapter 应通过 `get_group_member_info` API（见 3.1.3）查询并回填 `actor.role`。查询结果应写入 `identity_group_members` 缓存以避免重复请求。
- OneBot11 的 `sender.sex`、`sender.age`、`sender.level`、`sender.title` 等字段在 v0.1 中不映射到 `actor`，保留在 `raw_payload` 中供调试使用。
- 非消息类事件（如 `notice.member_increase`）的 `actor` 映射从事件顶层的 `user_id` 获取，不经过 `sender` 对象。

#### 3.2.3 归一化消息段模型

一条消息由一个有序的消息段（Segment）数组构成。`message` 字段的正式结构如下：

```json
{
  "segments": [
    { "type": "text", "data": { "text": "天气 " } },
    { "type": "at", "data": { "user_id": "123" } },
    { "type": "image", "data": { "url": "https://...", "file_id": "abc" } }
  ],
  "plain_text": "天气 @某人 [图片]"
}
```

- `segments`：消息段有序数组，是消息的正式结构化表示。
- `plain_text`：消息的纯文本摘要，由平台在归一化阶段自动生成，用于日志、命令解析和快速匹配。非文本段用占位符表示（如 `[图片]`、`@某人`）。

**v0.1 支持的消息段类型**：

| 段类型 | `data` 字段 | 说明 |
| --- | --- | --- |
| `text` | `text: string` | 纯文本内容 |
| `image` | `url?: string`, `file_id?: string`, `file?: string` | 图片，`url` 为网络地址，`file` 为本地 `file://` 或 `base64://` 引用 |
| `at` | `user_id: string` | @某人 |
| `at_all` | _(无)_ | @全体成员 |
| `face` | `face_id: string` | QQ 表情 |
| `reply` | `message_id: string` | 引用回复标记 |

**v0.2+ 扩展预留段类型**：

| 段类型 | 说明 |
| --- | --- |
| `file` | 文件消息 |
| `voice` | 语音消息（对应 OneBot11 的 `record` 消息段类型；接收方向 `record` → `voice`，发送方向 `voice` → `record`） |
| `video` | 视频消息 |
| `forward` | 合并转发 |
| `json` | JSON 卡片消息 |

**已知但暂不纳入规划的 OneBot11 消息段类型**：

以下消息段类型在真实 OneBot11 实现（LLOneBot、NapCat 等）中存在，但 v0.1/v0.2 暂不提供专用映射：

| OneBot11 段类型 | 说明 | 状态 |
| --- | --- | --- |
| `music` | 音乐分享 | 暂不支持 |
| `contact` | 推荐好友/群 | 暂不支持 |
| `location` | 位置消息 | 暂不支持 |
| `poke` | 戳一戳（消息段形式） | 暂不支持 |
| `dice` | 骰子 | 暂不支持 |
| `rps` | 猜拳 | 暂不支持 |
| `mface` | 表情包/动画表情 | 暂不支持 |
| `markdown` | Markdown 消息 | 暂不支持 |
| `xml` | XML 卡片消息 | 暂不支持 |
| `node` | 合并转发节点 | 暂不支持（与 `forward` 相关） |
| `keyboard` | 按钮/键盘消息 | 暂不支持 |
| `miniapp` | 小程序卡片 | 暂不支持（NapCat 扩展） |
| `shake` | 窗口抖动 | 暂不支持（已废弃） |

处理规则：

- 接收方向遇到上述类型时，Adapter 将其作为 `unknown` 段保留在 `raw_payload` 中，`plain_text` 中使用 `[未支持消息]` 占位符。
- 发送方向不支持上述类型的构造和发送。
- 后续版本将根据社区需求和使用频率，逐步将高频类型纳入正式支持。

**OneBot11 消息段映射规则**：

| OneBot11 CQ 码 / 消息段 | 统一段类型 |
| --- | --- |
| `[CQ:text,text=...]` / `{ "type": "text" }` | `text` |
| `[CQ:image,file=...,url=...]` / `{ "type": "image" }` | `image` |
| `[CQ:at,qq=...]` / `{ "type": "at" }` | `at`（`qq` → `user_id`）|
| `[CQ:at,qq=all]` / `{ "type": "at", "data": { "qq": "all" } }` | `at_all` |
| `[CQ:face,id=...]` / `{ "type": "face" }` | `face`（`id` → `face_id`）|
| `[CQ:reply,id=...]` / `{ "type": "reply" }` | `reply`（`id` → `message_id`）|

说明：

- Adapter 在归一化阶段完成 OneBot11 消息段到统一段模型的转换。
- OneBot11 同时支持 CQ 码字符串和消息段数组两种格式，Adapter 应两种都能解析。
- CQ 码格式中使用以下特殊字符转义，Adapter 在解析 CQ 码时必须还原：`&amp;` → `&`、`&#91;` → `[`、`&#93;` → `]`、`&#44;` → `,`（仅在 CQ 码参数值内生效）。
- 建议发送方向统一使用消息段数组格式（JSON Array），以避免 CQ 码转义带来的解析复杂度。
- 正向映射中 `[CQ:at,qq=all]` 和 `{ "type": "at", "data": { "qq": "all" } }` 应被识别为 `at_all` 类型而非普通 `at` 类型；Adapter 必须检测 `data.qq == "all"` 并将其转换为独立的 `at_all` 统一段类型。
- 接收消息中的图片默认保留 `url` 字段指向 OneBot11 提供的临时 URL；v0.1 不默认下载到本地，如插件需要持久化图片应自行通过 `http.request` 或 `storage.file` 处理。
- 接收方向图片段的 `file` 字段格式因 OneBot11 实现而异（可能为文件 hash 如 `{GUID}.jpg`、绝对路径 `file:///path`、或 HTTP URL）。Adapter 在归一化时应优先使用 `url` 字段作为统一图片引用，`file` 字段保留原值写入 `data.file` 供高级场景使用。`file_id` 字段在 v0.1 中不做特殊处理，保留原值。
- 发送消息时，Adapter 负责将统一段模型反向转换为 OneBot11 消息格式。
- 平台自动生成的 `fallback_text`、错误摘要、系统提示等纯文本消息，必须一律封装为单一 `{ "type": "text", "data": { "text": "..." } }` 段发送，不允许把整段文本当作 CQ 码字符串透传。
- Adapter 在把 `text` 段反向映射到 OneBot11 时，必须保证其中出现的 `[CQ:`、`[`、`]` 等字符只按普通文本处理，不得因实现细节把纯文本段重新解释为控制指令或 CQ 码。

**`send_msg` 请求构造规则**：

- Adapter 收到平台内部 `message.send` 或 `message.reply` 动作后，构造 OneBot11 的 `send_msg` 请求。字段映射：
  - `target_type: "group"` → `message_type: "group"`, `group_id: target_id`
  - `target_type: "private"` → `message_type: "private"`, `user_id: target_id`
- `auto_escape` 参数始终设为 `false`（或不传，默认 `false`）。RayleaBot 统一使用消息段数组格式发送，不使用 CQ 码字符串，因此不需要转义控制。
- `message.reply` 动作由 Bot Core 根据 `reply_to_event_id` 查找原始事件的 `target` 信息，自动填充 `message_type` 和目标 ID，插件无需手动指定。
- `message.reply` 可选携带 `fallback_to_send_if_missing: true`。当适配器确认“回复目标消息不存在 / 已撤回”这类特定错误时，应先标准化为 `adapter.reply_target_missing`；若插件启用了该开关，平台可自动剥离回复语义并以同一目标重试一次普通发送，重试仍失败再把最终错误返回给插件。
- `reply_to_event_id` 的解析应优先命中 Bot Core 维护的内存 `event_id -> message_id` LRU 缓存；仅在缓存未命中时才回源查询 `event_records` 或等价持久化摘要，避免高频 reply 路径被 SQLite 读 I/O 拖慢。

**OneBot11 反向映射规则（发送方向）**：

发送消息时，Adapter 将统一消息段模型转换为 OneBot11 消息格式，规则如下：

| 统一段类型 | OneBot11 映射 | 说明 |
| --- | --- | --- |
| `text` | `{ "type": "text", "data": { "text": "..." } }` | 直接映射 |
| `image` | `{ "type": "image", "data": { "file": "..." } }` | `file` 优先使用本地路径或 base64，其次使用 `url` |
| `at` | `{ "type": "at", "data": { "qq": "..." } }` | `user_id` → `qq` |
| `at_all` | `{ "type": "at", "data": { "qq": "all" } }` | 映射为 `qq=all` |
| `face` | `{ "type": "face", "data": { "id": "..." } }` | `face_id` → `id` |
| `reply` | `{ "type": "reply", "data": { "id": "..." } }` | `message_id` → `id`；`reply` 段必须位于消息段数组首位 |

反向映射说明：

- 遇到 Adapter 不识别的统一段类型时，静默丢弃该段并记录 `WARN` 级别日志，不中断整条消息的发送。
- 如果整条消息的所有段均被丢弃（即无有效内容可发送），Adapter 应返回 `adapter.send_failed` 错误。
- `reply` 段在 OneBot11 中必须作为消息的第一个段出现；Adapter 在反向映射时应自动将 `reply` 段提升到数组首位。

**`message_id` 全链路流转说明**：

`message_id` 在消息的接收、引用和发送三个阶段各有不同角色，以下为其完整生命周期：

1. **接收**：OneBot11 消息事件携带 `message_id`（通常为整数），Adapter 将其写入 `payload.message_id`。
2. **事件投递**：插件收到事件后可从 `payload.message_id` 获取该值，用于后续引用回复或撤回。
3. **引用回复**：插件发送消息时，若需引用原消息，将 `message_id` 放入 `reply` 消息段：`{ "type": "reply", "data": { "message_id": "12345" } }`。
4. **发送转换**：Adapter 将 `reply` 段转换为 OneBot11 的 `reply` 消息段（`message_id` → `id`），并作为 `send_msg` 请求的消息段数组首位发出。
5. **发送结果**：`send_msg` 成功后，OneBot11 返回新消息的 `message_id`（`data.message_id`），Adapter 通过 API 响应的 `result.data.message_id` 将其返回给插件，供后续引用。
6. **高层回复动作**：若插件走 `message.reply(reply_to_event_id=...)` 这条高层能力路径，Bot Core 需先把 `reply_to_event_id` 解析回对应的 `message_id`，再构造协议级 `reply` 段。

说明：

- `event_id` 与 `message_id` 是两个独立概念：`event_id` 是平台内部生成的事件唯一标识（格式 `evt-{uuid}`），用于日志追踪和事件去重；`message_id` 是 OneBot11 分配的消息标识（通常为整数），用于协议级的消息引用和操作。
- v0.1 中 `message_id` 的直接使用场景为引用回复（`reply` 段）；v0.2 将扩展至消息撤回（`delete_msg`）。
- Bot Core 必须在内存中维护最近事件的 `event_id -> message_id` 映射缓存（建议 LRU，默认至少保留最近 `10000` 条），优先满足 `message.reply` 的热路径查询，避免每次引用回复都回源 SQLite。
- 仅当该内存缓存未命中（如进程刚重启或历史记录被淘汰）时，才允许回源查询 `event_records` 或等价持久化摘要表中的 `message_id`；若持久层也查不到，再按常规回复失败路径处理。

#### 3.2.4 平台内部事件类型

3.1.2 映射了来自 OneBot11 协议的外部事件。平台自身也会产生内部事件，这些事件同样进入统一事件模型和 EventBus，供插件订阅。

内部事件命名格式：`{subsystem}.{event}`，与协议事件的 `{category}.{sub_type}` 格式保持一致。

**v0.1 支持的内部事件**：

| 内部事件类型 | 来源子系统 | 说明 | v0.1 支持 |
| --- | --- | --- | --- |
| `scheduler.trigger` | Scheduler | 定时任务触发，`payload` 包含插件在 `scheduler.create` 时传入的自定义数据 | 是 |
| `config.changed` | Config Manager | 当插件命名空间配置被 Web UI、CLI 或其他受控入口修改后，下发给对应插件；`payload` 至少包含 `scope`、`plugin_id`、`changed_keys` 和可安全回显的最新值摘要 | 是 |
| `webhook.received` | Webhook Gateway | 平台固定 Webhook 路由收到外部 `POST` 请求后，按插件与子路由定向下发；`payload` 至少包含 `route`、`headers`、`query`、`content_type`、`body_text/body_json` 摘要和来源 IP | 是 |

**v0.2 预留内部事件**：

| 内部事件类型 | 来源子系统 | 说明 |
| --- | --- | --- |
| `lifecycle.plugin_started` | Runtime Manager | 插件成功启动并完成 `init_ack` |
| `lifecycle.plugin_stopped` | Runtime Manager | 插件停止运行（正常退出或崩溃） |
| `lifecycle.connection_changed` | Adapter | 协议连接状态发生变化（如 `connected` → `reconnecting`） |

说明：

- 内部事件与协议事件共享统一事件模型结构（`event_id`、`source_protocol`、`event_type`、`actor`、`target`、`payload` 等）。
- 内部事件的 `source_protocol` 为 `internal`，`source_adapter` 为产生事件的子系统标识（如 `scheduler`、`runtime_manager`）。
- 插件通过 `init_ack.subscriptions` 声明订阅内部事件，与协议事件订阅方式一致。
- `scheduler.trigger` 事件的 `actor` 为 `null`（无用户触发），`target` 为 `null`（非消息上下文）。
- `config.changed` 仅投递给受影响的插件，不广播给所有插件；如目标插件当前未运行，则不做离线补投，插件在下次启动或主动 `config.read` 时读取最新配置。
- `webhook.received` 仅由平台固定入口生成，不允许插件自行在宿主机拉起额外监听端口作为官方能力路径；事件的 `actor` / `target` 默认为 `null`，鉴权、签名校验或来源 IP 白名单由 Bot Core 的 Webhook Gateway 在投递前完成。

### 3.3 状态模型

本节是平台状态枚举的唯一正式来源；插件恢复、日志、Web UI、Launcher 和诊断能力都应引用本节定义，而不是各自定义状态名。

#### 3.3.1 服务状态

建议统一使用以下服务状态（本节是平台状态枚举的唯一正式来源）：

- `setup_required`
- `stopped`
- `starting`
- `running`
- `degraded`
- `stopping`
- `failed`

说明：

- `setup_required` 仅在首次启动且不存在管理员账户时进入，用于管理员初始化引导。
- 初始化完成后，服务应退出 `setup_required` 并直接进入 `starting -> running` 的正常启动流程。
- Web UI、Launcher、状态同步和诊断能力都必须引用本节定义，而不是额外发明服务状态名。

#### 3.3.2 插件状态

对外展示和 API 返回建议统一使用以下插件状态：

- `installed`
- `enabled`
- `starting`
- `running`
- `stopping`
- `crashed`
- `backoff`
- `dead_letter`
- `disabled`

内部建模建议拆成三层，而不是只用一个字段混合表达：

- `registration_state`：`installed` / `removed`
- `desired_state`：`enabled` / `disabled`
- `runtime_state`：`starting`、`running`、`stopping`、`crashed`、`backoff`、`dead_letter`、`stopped`

说明：

- `installed` 表示插件包已被平台注册并可管理。
- `enabled` 表示用户希望插件处于启用状态。
- `runtime_state` 表示插件当前实际运行情况。
- Web UI、Launcher、数据库和日志展示应基于同一套状态定义，避免各处各说各话。

#### 3.3.3 连接状态

协议连接状态建议统一使用以下状态：

- `disconnected`
- `connecting`
- `connected`
- `authenticated`
- `auth_failed`
- `reconnecting`

说明：

- 连接状态应同时服务于 Web 面板、启动器、日志分类和 API 返回。
- `authenticated` 用于区分“底层链路已建立”和“协议鉴权已完成”。
- `auth_failed` 用于显式表达鉴权失败，而不是把所有失败都折叠成断线。
- `reconnecting` 应显式可见，便于用户区分暂时抖动和完全断线。

**连接状态流转规则**：

| 当前状态 | 触发条件 | 目标状态 | 说明 |
| --- | --- | --- | --- |
| `disconnected` | 调用连接方法 | `connecting` | 服务启动或手动重连时发起 |
| `connecting` | WebSocket 握手成功 | `connected` | 底层链路已建立 |
| `connecting` | 握手失败或超时 | `reconnecting` | 进入退避重连（见 3.11.1） |
| `connected` | 鉴权成功（首次心跳或 lifecycle 事件到达） | `authenticated` | 协议层确认可用 |
| `connected` | 鉴权失败（HTTP 401/403） | `auth_failed` | 不自动重连，需用户检查 Token（见 3.1.1） |
| `authenticated` | 连续 3 个心跳周期未收到心跳 | `reconnecting` | 见 3.3.4 |
| `authenticated` | WebSocket 连接断开或读写错误 | `reconnecting` | TCP 层断连或 close frame |
| `reconnecting` | WebSocket 握手成功 | `connected` | 重新进入鉴权流程 |
| `auth_failed` | 用户修改 Token 后手动重连或重启服务 | `connecting` | 手动恢复 |
| 任意状态 | 服务停机 | `disconnected` | 优雅停机清理 |

说明：

- `connected` → `authenticated` 的判定以收到首次 `meta.heartbeat` 或 `meta.lifecycle`（`sub_type=enable`）事件为准。
- `auth_failed` 状态下不执行自动重连，避免 Token 错误时无意义重试。
- 状态变化应同步写入日志、推送到 Web UI 的 `/ws/events`，便于管理端实时感知。

#### 3.3.4 心跳事件结构与健康检测

OneBot11 的 `meta.heartbeat` 事件携带连接健康信息，Adapter 应据此维护连接状态。

**心跳事件载荷示例**：

```json
{
  "post_type": "meta_event",
  "meta_event_type": "heartbeat",
  "self_id": 123456,
  "time": 1700000000,
  "status": {
    "online": true,
    "good": true
  },
  "interval": 5000
}
```

**处理规则**：

- 默认心跳间隔约 5000ms（由 OneBot11 实现端决定，Adapter 从首次心跳的 `interval` 字段读取实际值）。
- Adapter 应维护最近一次心跳接收时间戳，用于超时判定。
- 超时规则：连续 3 个心跳周期（即 `interval × 3`）未收到心跳事件，Adapter 将连接状态切换为 `reconnecting`（参见 3.3.3）。
- 收到心跳时，若 `status.online == false` 或 `status.good == false`，记录 `WARN` 级别日志，但不主动断开连接——由后续心跳或连接层错误触发重连。
- 心跳事件不投递给插件，仅用于 Adapter 内部连接健康管理。

### 3.4 Bot Core

Bot Core 是服务进程的主控制层，建议至少包含以下模块：

- EventBus：统一事件流转入口。
- Command Parser：负责消息命令前缀匹配、command/args 提取与定向投递优化（详见 3.4.5）。
- Permission System（Chat）：负责聊天侧黑名单、超级管理员、群管理员、everyone 四级权限判定与用户侧冷却限流（详见 3.4.6、3.11.2）。
- Plugin Manager：负责插件发现、注册、启停、状态同步。
- Runtime Manager：负责插件生命周期管理，包括启动、停止、重启、热重载、健康监控和状态同步。
- Scheduler：负责定时任务和延迟任务。
- Capability Grant Manager：负责插件能力声明审核、权限授予状态机（详见 3.6.2）和管理操作权限。
- Config Manager：负责配置读取、覆盖、校验和热更新入口。
- Logger：统一结构化日志输出。
- Render Service：负责模板渲染、资源管理、缓存和渲染任务调度。
- Runtime Bridge：负责对 Runtime Manager 与插件子进程之间的通信进行抽象封装。

优雅停机要求：

- 当终端收到 `SIGINT` / `Ctrl+C`，或服务管理器发送 `SIGTERM` 时，Bot Core 必须进入统一优雅停机流程。
- 停机流程至少包括：停止接收 Adapter 的新事件、并发向所有运行中的插件发送 `shutdown` 并等待有限退出窗口、安全关闭 SQLite 的读写句柄与其他关键资源。
- 如插件在窗口期内未退出，Runtime 可执行强制回收；但数据库句柄和日志刷盘仍应按受控顺序关闭，避免因为粗暴退出导致 `.wal` 恢复成本增加或状态不一致。

#### 3.4.1 事件投递与丢弃策略

插件并发事件处理语义：

- 默认行为：Runtime 向同一插件子进程投递事件时允许并发，即不等前一事件处理完就可发送下一事件。插件子进程内部由 SDK 或插件自身决定并发处理方式。
- 并发上限：同一插件的并发处理事件数受 `runtime.max_concurrent_tasks_per_plugin`（默认 `4`）限制。当达到上限时，新事件进入等待队列（上限 `runtime.max_pending_events_per_plugin`，默认 `16`）；队列也满时按丢弃策略处理。
- 插件可在 manifest 中声明 `concurrency: 1` 表示自身只能串行处理事件（如涉及状态修改的插件），Runtime 将保证同一时刻最多只向该插件投递一个事件。
- 未声明 `concurrency` 时默认采用 `runtime.max_concurrent_tasks_per_plugin` 的全局配置值。

事件投递与丢弃规则：

- v0.1 对插件事件投递采用 `fire-and-forget` 策略，不为单个插件维护长期事件堆积队列。
- `scheduler.trigger`、`config.changed` 以及后续同类平台控制事件不得和普通聊天事件共用同一条饱和后直接丢弃的业务队列。Runtime / EventBus 必须为这类内部控制事件保留独立的高优先级控制通道或保留队列（建议 `runtime.max_pending_control_events_per_plugin = 4`），使其不因群聊刷屏或普通消息风暴而被饿死。
- EventBus 在 fan-out 投递到不同插件的异步队列时，不得把原始大事件对象的 Go 指针直接共享给多个插件队列。应优先把事件规整为紧凑、不可变的受控表示（如已序列化 JSON 字节）后再入队；若实现采用共享缓冲，则必须配套显式引用计数、生命周期控制或对象池，避免单个慢插件把整棵原始对象图长期钉在堆上。
- 当目标插件未处于 `running` 状态，例如 `starting`、`stopping`、`backoff`、`dead_letter` 或热重载切换窗口期，EventBus 应直接丢弃该插件的待投递事件，并写入一条 `DEBUG` 级别日志。
- 该策略只影响插件侧事件消费，不影响核心状态事件、日志事件和协议层链路本身的处理。
- v0.1 不在内存中为插件补投历史事件，以避免在插件反复重启或长时间不可用时积压事件导致内存膨胀。
- 是否记录该类丢弃日志应受 `log.level` 控制，避免在默认运行态制造过多噪声。
- 平台应至少维护 `dropped_event_count_total` 与 `dropped_event_count_by_plugin` 两类统计项；如实现成本可接受，建议进一步按 `plugin_not_running`、`plugin_backoff`、`plugin_dead_letter`、`reload_window` 等原因分类。
- Web UI 的插件详情页与诊断包应展示最近一段时间的事件丢弃摘要，避免排障时只能依赖临时开启的细粒度日志。
- 当普通业务队列已满时，允许继续按保留配额接受控制事件；若控制队列也达到上限，`config.changed` 可按插件维度做最新值合并，`scheduler.trigger` 应至少记录结构化告警并保留 `task_id` 级失败摘要，不得静默丢弃。

#### 3.4.2 插件事件订阅与分发语义

- v0.1 插件订阅以统一事件模型中的 `event_type` 为核心，优先支持精确匹配，如 `message.group`、`notice.member_increase`。
- 为减少首版复杂度，v0.1 可选支持有限前缀级通配订阅，如 `message.*`、`notice.*`；不支持正则表达式、复杂谓词或脚本化订阅条件。
- 多个插件同时命中同一事件时，默认采用 fan-out 分发，所有匹配插件都可收到该事件。
- v0.1 不提供事件停止传播、插件优先级抢占、互斥回复或“首个处理者获胜”等高级分发机制。
- 如实现需要确定性调试顺序，可按插件注册顺序或 `plugin_id` 排序投递；但插件不得把接收顺序当作业务契约依赖。
- 更细粒度的业务过滤应由插件自身在收到统一事件后完成，而不是让 EventBus 承担复杂规则引擎职责。

**v0.2+ 演进方向：事件处理管线**：

- v0.1 的事件处理采用固定顺序流程（见 2.4 关键运行流程步骤 1-8），各阶段（事件归一化 → 命令解析 → 权限校验 → 插件分发）由平台内部模块串行执行，插件仅在分发阶段介入。这一设计刻意保持简单，足以支撑首版闭环。
- v0.2+ 应评估将固定流程演进为可扩展的分阶段事件处理管线（Pipeline），使平台扩展或高级插件能在特定处理阶段注册中间件（Middleware）钩子，例如：
  - 预处理阶段：消息内容过滤、敏感词检测、语音转文字等。
  - 命令解析阶段：自定义命令解析策略（见 3.4.5）。
  - 权限校验阶段：自定义权限规则扩展（见 3.4.6）。
  - 后处理阶段：消息日志、上下文记忆、统计采集等。
- v0.2+ 应同步评估事件停止传播（Stop Propagation）、基于优先级的投递排序和"首个处理者获胜"等高级分发语义，作为 fan-out 之上的可选增强。
- 管线扩展必须保持对 v0.1 fan-out 语义的向后兼容：未声明管线参与的插件继续以 fan-out 方式接收事件，不受管线引入的影响。

#### 3.4.3 Scheduler 行为语义

- Scheduler 统一使用服务主时区，默认取宿主机本地时区；如 `config/user.yaml` 显式配置时区，则以配置为准。
- 容器化部署（Docker / LXC）时，不应假定“宿主机本地时区”会自动传入容器。若未显式配置 `scheduler.timezone` 或容器环境变量 `TZ`，调度行为可能退回 UTC 并导致定时任务在错误时间触发。
- 定时任务应至少带 `plugin_id`、`task_id`、触发规则、下次执行时间和启用状态，避免热重载或重启后重复注册。
- 服务重启后，Scheduler 默认不补跑离线期间错过的周期性任务，而是按当前时间重新计算下一次触发时间。
- 一次性任务若在服务离线期间错过触发窗口，v0.1 默认标记为 `missed` 或 `expired`，不自动追补执行。
- 同一插件的同一 `task_id` 在恢复、热重载和重复注册时应执行去重更新，而不是生成多份重复任务。
- 当插件被禁用、进入 `dead_letter` 或被卸载时，其关联任务应暂停或移除，不得继续脱离插件生命周期独立运行。

#### 3.4.4 启动时序与 Ready 条件

- 服务启动顺序应固定为：配置加载 -> 迁移检查 -> 运行时与渲染资源检查 -> 管理员初始化判定 -> 插件注册与调度恢复 -> 本地控制面可用 -> Adapter 建链 -> 服务进入 `running`、`degraded`、`setup_required` 或 `failed`。
- 若配置加载、迁移检查、`.deps/` / Chromium / 模板资源检查任一步失败，服务不得进入 `running`，并应把失败摘要暴露给日志、Launcher、CLI 和 Web 管理端。
- 若系统判定处于 `setup_required`，则在管理员初始化完成前不得加载插件、建立 OneBot11 连接或开始调度任务。
- 当关键资源可用、初始化状态满足且本地管理控制面已可用时，服务即可完成主进程启动；若 Adapter 成功建立受支持的 OneBot11 链路，则切换为 `running`；若用户暂未配置 OneBot11 连接，Adapter 保持 `idle`，服务仍满足 Ready 条件。
- 若用户已配置 OneBot11 反向 WebSocket，且外部链路暂时不可用、正在重连或认证尚未完成，但本地控制面与核心资源均已正常，则服务应进入 `degraded` 并持续重连，而不是把整个进程视为硬启动失败。
- 非致命问题如个别插件恢复失败、外部协议链路暂不可用，可在记录错误并标记状态后让服务进入 `degraded`，但不得伪装成完全就绪。
- Ready 条件应以本地管理控制面、关键资源和初始化状态为主；任一本地关键检查失败默认进入 `failed` 或维持在阻塞态，而不是误用 `degraded` 掩盖启动失败。

#### 3.4.5 命令解析与路由机制

Bot Core 在将消息事件投递给插件之前，提供平台级的基础命令解析能力。

**命令前缀配置**：

- 命令前缀通过 `config/user.yaml` 中的 `command.prefixes` 配置，默认值为 `["/"]`。
- 支持配置多个前缀，如 `["/", "!", "。"]`。
- 空字符串 `""` 表示无前缀模式（直接匹配命令名），适用于私聊或特殊场景，但不建议在群聊中默认启用。

**命令解析流程**：

1. Bot Core 收到 `message.group` 或 `message.private` 事件后，从 `message.plain_text` 中检测是否以已配置的命令前缀开头。
2. 若匹配前缀，提取前缀后的第一个词作为 `command`，剩余部分按空格分割为 `args` 数组。
3. 解析结果填入 `payload.command`（命令名，不含前缀）和 `payload.args`（参数列表）。
4. 非命令消息的 `payload.command` 为 `null`，`payload.args` 为空数组。

**插件命令声明**：

- 插件可在 manifest（`info.json`）中通过 `commands` 字段声明自己处理的命令列表：

```json
{
  "commands": [
    {
      "name": "weather",
      "aliases": ["天气", "tq"],
      "description": "查询指定城市天气",
      "usage": "/weather <城市名>",
      "permission": "everyone"
    }
  ]
}
```

- `name`：主命令名。
- `aliases`：命令别名列表，允许多语言或缩写触发同一命令。
- `description`：命令说明，用于帮助菜单生成。
- `usage`：使用示例。
- `permission`：命令权限级别，详见 3.4.6。

**定向投递优化**：

- 当消息被解析为命令时，Bot Core 优先将事件投递给声明了该命令的插件，减少无关插件的处理开销。
- 若无插件声明该命令，事件仍按订阅关系 fan-out 分发给所有订阅了 `message.*` 的插件。
- 多个插件声明同一命令名时，按注册顺序全部投递（fan-out），不做互斥；若后续版本需要互斥处理，可通过优先级机制扩展。
- 平台保留 `raylea:*` 命名空间作为系统保留命令前缀，仅允许官方内置插件声明；第三方插件不得占用该保留前缀。
- 对非保留命令名，v0.1 维持 fan-out 语义不变，但 Web UI 必须显式提示同名命令冲突，避免管理员误以为平台会自动做互斥仲裁。

**非命令消息处理**：

- 关键词触发、自然语言处理等场景不走命令解析路径。
- 这类插件应订阅 `message.group` 或 `message.*` 事件，自行从 `message.plain_text` 或 `message.segments` 中匹配。
- 非命令消息的 `payload.command` 为 `null`，插件可据此判断是否为命令调用。

#### 3.4.6 聊天侧用户权限模型

平台在聊天侧提供基础的用户权限控制能力，用于管控"哪些 QQ 用户可以使用哪些命令"。

**权限级别**：

插件可在 manifest 的 `commands` 字段中为每个命令声明权限级别：

| 权限级别 | 说明 |
| --- | --- |
| `super_admin` | 仅 Bot 超级管理员可用 |
| `group_admin` | 群主、群管理员和超级管理员可用 |
| `everyone` | 所有用户可用（默认值） |

**超级管理员**：

- 由 `config/user.yaml` 中的 `admin.super_admins` 字段指定，值为 QQ 号列表。
- 超级管理员拥有所有命令的使用权限，不受权限级别限制。
- 超级管理员列表变更为"需要局部重载"的配置项。

**群管理员角色继承**：

- QQ 群主和群管理员自动继承 `group_admin` 权限级别。
- 角色信息从 `identity_group_members` 缓存中获取，由 Adapter 在事件归一化阶段填入 `actor.role` 字段。
- `actor.role` 取值：`owner`（群主）、`admin`（群管理员）、`member`（普通成员）。

**黑名单 / 白名单机制**：

- 用户级黑名单：将指定 QQ 号加入黑名单后，该用户发送的所有命令消息不会触发插件处理（静默忽略）。
- 群级黑名单：将指定群号加入黑名单后，该群的所有消息不会触发插件处理。
- 黑名单数据存储在 `permission_grants` 表或等价结构中，通过 Web UI 管理。
- v0.1 不实现白名单模式（即"只允许指定用户/群使用"），留待后续版本。

**权限检查流程**：

1. Bot Core 在事件分发前执行前置权限过滤。
2. 检查顺序：超级管理员判定 → 黑名单 → 命令权限级别匹配 → 投递。
3. **超级管理员优先**：超级管理员身份具有最高特权，强制无视个人黑名单和群黑名单的拦截规则，确保系统始终保有最后的人工干预通道。
4. 黑名单用户的事件直接丢弃，不投递给任何插件（错误码 `permission.blacklisted`）。
5. 超级管理员跳过权限级别检查，直接投递。
6. 非命令消息不受权限级别限制，正常 fan-out 分发。
7. 权限检查不通过时，默认返回简短提示消息（可配置是否回复）。

**与 Web 管理权限的关系**：

- 聊天侧权限模型与 3.9.5 的 Web 管理权限模型相互独立。
- 聊天侧超级管理员不等同于 Web 管理员，两者的鉴权机制和作用域完全分离。
- 插件如需更精细的业务权限控制（如自定义角色、积分门槛等），可在插件内部自行扩展，平台不强制统一。

#### 3.4.7 后台任务与长操作模型

平台中的插件安装、依赖解析、备份、恢复、迁移、渲染任务和热重载等长操作，统一抽象为后台任务，而不是各自定义半同步接口。

建议最小任务字段如下：

| 字段 | 说明 |
| --- | --- |
| `task_id` | 任务唯一标识 |
| `task_type` | 任务类型，如 `plugin.install`、`backup.create`、`restore.apply`、`plugin.reload` |
| `status` | `pending` / `running` / `succeeded` / `failed` / `cancelled` / `interrupted` |
| `progress` | 0-100 的进度百分比，未知时可为空 |
| `summary` | 当前阶段摘要，如“解析 manifest”“安装 Python 依赖”“执行迁移” |
| `started_at` | 任务开始时间 |
| `finished_at` | 任务结束时间 |
| `result` | 成功结果摘要，可选 |
| `error` | 失败错误摘要，可选 |

统一约束：

- Web UI、CLI、WebSocket 和日志系统应复用同一套任务模型，不为每种长操作额外发明独立状态字段。
- `POST /api/plugins/install`、`raylea backup`、`raylea restore`、`raylea migrate` 等长操作都应返回或关联 `task_id`。
- `/ws/tasks` 负责实时推送任务阶段、摘要和最终结果；`GET /api/tasks/{id}` 用于查询最新任务快照。
- 任务状态变化必须进入结构化日志与诊断包，便于排查”HTTP 已返回但后台还在运行”的情况。

**任务列表、取消与服务重启行为**：

- `GET /api/tasks`：v0.1 应提供任务列表查询接口，返回最近 N 条任务（默认 50），支持按 `status` 和 `task_type` 过滤。该接口是 Web UI 任务管理页的数据来源。
- 可取消的任务类型：`plugin.install`（安装中可中断依赖下载）、`backup.create`（备份中可中止打包）。
- 不可取消的任务类型：`db.migrate`（数据库迁移一旦启动不可中断）、`restore.apply`（恢复流程中断可能导致数据不一致）。
- `plugin.reload` 仅允许在 `pending` 阶段取消；一旦任务进入 `running` 并已向旧插件进程发出 `shutdown(reason=reload)`，视为进入不可逆阶段，取消请求必须返回 `409 Conflict`。
- `plugin.uninstall` 仅允许在 `pending` 或 `running` 的预检阶段取消；一旦开始停用插件、删除插件包目录或清理注册状态，视为进入不可逆阶段，取消请求必须返回 `409 Conflict`。
- 取消请求通过 `POST /api/tasks/{id}/cancel` 发起；当取消被接受时返回 `202 Accepted`，任务在合适的检查点响应取消并切换为 `cancelled`；任务不存在时返回 `404 Not Found`。
- 所有涉及外部网络 I/O 或外部子进程执行的长任务（至少包括 `plugin.install`）都必须具备绝对超时上限；默认建议 `runtime.dependency_install_timeout_seconds = 900`（15 分钟）。超过上限后，任务调度器必须强制终止底层执行器并释放安装锁、临时目录与任务占用资源，最终将任务标记为 `failed`，错误码为 `platform.task_timeout`。
- 所有会真正调用 `pip` / `npm`、写入共享下载缓存或构建插件运行环境的任务（至少包括 `plugin.install`、`plugin.upgrade` 与显式“重试安装”）在 v0.1 必须共享同一条全局环境构建队列。默认 `runtime.max_concurrent_dependency_installs = 1`，同一时刻只允许一个底层依赖安装任务进入执行态，其余任务保持 `pending` 并展示“等待环境构建槽位”摘要，避免共享缓存目录被并发写坏。
- 强制终止应按目标平台采用等价的“不可捕获回收”策略：Unix 类系统可使用 `SIGKILL` 或进程组强杀，Windows 端应使用等价的 terminate / job object 回收方式；不得只发送温和退出信号后无限等待。
- `plugin.install` 因网络抖动、镜像站卡死或包源无响应而超时失败后，默认不做隐式自动重试；Web UI 与 CLI 应提供显式“重试安装”入口，允许管理员在调整镜像源、网络或超时时间后重新发起。
- 服务重启后的任务行为：所有 `running` 状态的任务在服务重启后标记为 `interrupted`（新增状态），不自动恢复执行。`pending` 状态的任务丢弃。Web UI 应展示 `interrupted` 任务并允许管理员决定是否重新发起。

**各类任务的可取消性与回滚语义**：

| 任务类型 | 可取消性 | 接受取消的阶段 | 取消后的自保 / 回滚规则 |
| --- | --- | --- | --- |
| `plugin.install` | 条件可取消 | `pending`、依赖下载 / 临时目录构建阶段 | 必须强杀 `pip` / `npm`、释放全局构建锁、删除临时目录与半完成依赖环境；正式安装目录、插件注册状态和旧版本运行实例保持不变 |
| `plugin.upgrade` | 条件可取消 | `pending`、下载 / 预检 / 临时构建阶段 | 一旦尚未原子切换到新版本，可按安装任务规则撤销；若已进入切换窗口，则取消请求返回 `409 Conflict`，最终由升级事务决定成功或失败回滚 |
| `plugin.reload` | 仅 `pending` 可取消 | `pending` | 接受取消时，旧进程继续保留并维持原状态；一旦已向旧进程发送 `shutdown(reason=reload)` 或开始拉起新进程，任务即进入不可逆阶段，必须返回 `409 Conflict` |
| `plugin.uninstall` | 条件可取消 | `pending`、预检阶段 | 若已开始停用插件、删除插件包目录、清理注册状态或迁移审计信息，则不可取消；取消后插件包和业务数据目录都必须保持原样 |
| `backup.create` | 可取消 | 打包、压缩、导出阶段 | 取消后必须删除半成品压缩包、临时导出文件和未提交 manifest，不得把半成品登记为有效备份 |
| `restore.apply` | 不可取消 | 无 | 一旦开始覆盖恢复目录、导入状态库或恢复插件目录，即视为不可中断流程；后续依靠“恢复后首启检查”自保，不暴露假性的“取消中”状态 |
| `db.migrate` | 不可取消 | 无 | 一旦进入迁移执行窗口，必须跑完成功或失败回滚；不得在数据库迁移中途开放取消，以免留下不可判定的 schema 状态 |

补充约束：

- `restore.apply` 与 `db.migrate` 的不可取消性必须同时体现在 Web API、CLI 帮助、任务流和 Web UI 按钮禁用逻辑中，避免入口层误导用户。
- 若任务因取消或失败留下需人工处理的残留物，`result` / `error` 中必须给出清理建议，而不是只返回笼统错误。

#### 3.4.8 事件去重与幂等建议

- v0.1 不承诺平台级 `exactly-once` 语义；在协议重连、插件崩溃恢复、热重载或调度恢复过程中，事件可能出现重复投递或重复触发。
- 平台统一事件模型中的 `event_id` 与消息链路中的 `message_id` 应作为插件侧幂等判断的基础键。
- 涉及积分扣减、数据库写入、外部通知、支付、长链路工作流等有副作用的插件，必须自行按 `event_id`、`message_id`、`task_id` 或业务键实现幂等保护。
- Scheduler 触发的任务同样不承诺天然去重；插件应结合 `task_id` 与业务侧幂等键避免重启后重复执行造成副作用。
- 内置插件与第三方插件遵循同一幂等约束，不因为“官方插件”身份跳过此规则。

### 3.5 插件系统边界

#### 3.5.1 插件分类

- `plugins/builtin/`：官方内置插件，跟随主项目版本节奏演进，纳入版本控制。
- `plugins/installed/`：用户安装的第三方插件，不纳入版本控制。
- `plugins/dev/`：本地开发调试插件，是否纳入版本控制由仓库策略或开发者自行决定。

约束：

- 采用标准插件协议运行的官方能力，优先归入 `plugins/builtin/`，而不是散落到核心代码中。
- `plugins/builtin/` 用于平台随包发布的官方内置插件，`plugins/installed/` 与 `plugins/dev/` 分别用于用户安装和开发调试来源。
- 原 `modules/` 目录已废弃，所有官方内置插件统一迁移至 `plugins/builtin/` 并使用标准插件协议与生命周期。

#### 3.5.2 语言支持策略

- 插件系统采用语言无关的进程通信模型设计。
- v0.1 官方支持 Python / Node.js 两类插件运行时。
- Rust / Go 插件只保留预编译二进制插件接口预留，不进入首版交付承诺。
- 后续如扩展其他语言，应以运行时适配层或预编译产物形态接入，而不是在 v0.1 承诺自动安装所有工具链。

官方 SDK 承诺：

- v0.1 同时提供 `rayleabot-sdk-python` 和 `rayleabot-sdk-nodejs`。
- 插件开发者应优先通过 SDK 的 `PluginBase`、事件装饰器和能力封装开发插件，而不是手动处理 stdin/stdout JSON 协议。
- SDK 负责屏蔽底层协议细节，统一暴露事件订阅、`message.send`、`render.image`、配置读写、日志等常用能力。
- 总规划文档中的 JSON 协议示例主要服务实现与调试，不作为普通插件作者的首选接入方式。

#### 3.5.3 插件发布形态规范

建议将插件发布形态明确分成三类：

- 平台内置 Python / Node.js 环境插件：使用平台内置 Python / Node.js 环境，由平台统一调用，面向普通用户。
- 二进制插件：由 Go / Rust / C# 等语言编译为独立产物，直接运行，面向普通用户。
- 开发者源码插件：依赖系统环境，仅开发模式使用，允许更灵活的依赖安装和调试。

设计要求：

- 面向普通用户的发行包优先支持“平台内置 Python / Node.js 环境插件”和“二进制插件”。
- “开发者源码插件”不作为正式发行包的默认承诺能力。
- v0.1 对二进制插件保留发布形态和 manifest 兼容性，不要求与平台内置 Python / Node.js 环境插件具备同等成熟的安装、调试和热重载体验。
- v0.1 的核心验收闭环仍聚焦 Python / Node.js 平台内置环境插件；二进制插件不进入首版核心验收路径。
- 远期的插件索引、分发能力、安装器和依赖策略都应基于这三类发布形态设计，避免语义混乱。

#### 3.5.4 Runtime 职责

`Plugin Runtime` 在 v0.1 的职责应收紧为：

- 发现插件入口和可执行元数据。
- 检查运行时是否存在且可调用。
- 按插件作用域从 `secret_store` 解析敏感凭据，并在子进程启动前以环境变量形式注入，详见 3.10.2.1。
- 为插件子进程构造最小白名单环境，只继承必要运行时变量与受控注入项，而不是透传宿主机完整环境变量集合。
- 启动、停止、重启插件进程。
- 支持手动热重载与开发态目录监听。
- 转发事件、动作、结果和错误。
- 监控插件健康状态和退出码。
- 执行退避重试并上报状态变化。
- 在拉起平台内置 Node.js 环境插件子进程时，必须强制附加 `--max-old-space-size=<limit_mb>`（或等价 `NODE_OPTIONS`）启动参数，对单插件 V8 Old Space 做硬上限约束；默认值建议为 `256 MB`，并允许通过 `config/user.yaml` 中的 `runtime.nodejs_max_old_space_size_mb` 调整，避免多个 Node.js 插件并行时挤爆宿主机物理内存。

实现建议：

- v0.1 由 `server` 内部 Runtime Manager 统一管理插件子进程，不额外引入独立 runtime 二进制。
- Web UI、Launcher 和其他管理入口只和主服务交互，避免围绕插件运行时形成第二套控制面。
- 如后续需要更强隔离、独立部署或多进程扩展，再把该模块抽离为独立服务或二进制。
- Runtime Manager 应统一处理启动、停止、重启、手动热重载和开发态目录监听，避免不同插件类型各自实现生命周期逻辑。
- 对平台内置 Node.js 环境插件的内存上限，平台配置值是唯一来源；插件 manifest 不得自行声明更高的 V8 堆限制来绕过平台保护阈值。
- Runtime 在拉起托管插件时，必须为该插件建立独立的进程组或等价作业对象。Linux 下应优先使用 `Setpgid=true` 或等价机制，Windows 下应使用 Job Object 并启用“随父级回收”语义，避免插件再衍生的孙子进程脱离监管。
- 对插件执行强制回收时，不得只终止主 PID；必须面向整个进程组 / 作业对象执行终止，确保由插件通过 `os.system`、`subprocess`、`child_process.spawn` 等方式拉起的违规衍生进程一并被回收。

插件超时策略：

- `runtime.plugin_init_timeout_seconds = 30`：插件收到 `init` 后必须在此窗口内回复 `init_ack`，或至少发送一次 `init_progress` 以刷新静默超时计时器。
- `runtime.plugin_init_max_total_seconds = 300`：单次初始化从收到 `init` 到最终 `init_ack` / 失败退出的总时长上限；即使插件持续发送 `init_progress`，超过该总上限仍视为初始化失败。
- `runtime.plugin_event_timeout_seconds = 60`：单次事件处理的最大时长；超时后 Runtime 记录告警并标记该次处理为超时，但不默认强制回收插件进程（避免误杀长耗时合法操作）。
- 超时检测主要依靠 `ping` / `pong` 心跳机制：Runtime 按固定间隔（如 15 秒）向插件发送 `ping`，若连续 2 次未收到 `pong` 响应，则判定插件卡死并执行强制回收。
- 官方 SDK 必须保证 `ping` / `pong` 响应由独立于用户业务回调的 I/O 协调层处理，而不是与 `on_event`、`on_command` 等业务 handler 共用同一个可能被阻塞的执行路径。Python SDK 应优先使用独立后台线程负责 stdin 读取和心跳响应；Node.js SDK 应使用 Worker、辅助线程或等价机制把标准流 I/O 与用户事件循环解耦。
- 因此，`ping` / `pong` 的语义应固定为“插件进程及其协议 I/O 层是否仍存活”，而不是“当前业务事件循环是否空闲”。长耗时业务处理可以触发 `runtime.plugin_event_timeout_seconds` 告警，但不应仅因业务 handler 阻塞而让心跳机制误判进程死亡。
- 初始化超时后的状态流转：`starting` → `crashed`（`plugin.init_timeout`）→ 退避重试 → 达阈值后 `dead_letter`。
- 事件处理超时后的状态流转：不改变插件 `runtime_state`，仅记录告警；若心跳同时失败才进入崩溃回收流程。

不应由 `Plugin Runtime` 承担的职责：

- 下载和安装完整 Python / Node.js 工具链。
- 解析远程插件市场索引。
- 负责 UI 层面的插件管理流程。

#### 3.5.5 `.deps/` 目录定位

`.deps/` 仅用于以下内容：

- 平台提供的 Python / Node.js 环境。
- 平台下载的插件依赖缓存。
- 平台为插件准备的非用户直接编辑资源。

约束：

- v0.1 仅支持平台提供的 Python / Node.js 环境。
- 不支持自动安装 Rust / Go 工具链。
- `.deps/` 由发行包或启动器准备，`Plugin Runtime` 只做检查与调用。

Python 插件依赖隔离策略：

- Python 插件安装时，平台应在插件目录下创建独立虚拟环境，而不是把第三方依赖混装到全局托管环境中。
- 建议路径为 `plugins/installed/<plugin_id>/.venv/`；开发者源码插件可使用 `plugins/dev/<plugin_id>/.venv/`。
- `.deps/` 中的 Python 运行环境负责提供基础解释器和可复用的下载缓存，不作为所有插件共享依赖安装目录。
- 插件 manifest 中的 `dependencies` 由平台解析后安装到该插件自己的 `.venv/` 内，避免不同插件对同一库版本产生冲突。
- Runtime 启动 Python 插件时应优先调用该插件虚拟环境中的解释器或入口脚本，而不是直接调用全局 Python。
- 职责划分应固定为：`.deps/` 托管平台解释器与共享下载缓存，`plugins/*/<plugin_id>/.venv/` 托管单个 Python 插件的隔离依赖环境。
- 官方 SDK 与核心文档应优先推荐纯 Python 依赖或可直接获取预编译 wheel 的依赖，尽量避免要求用户自行补齐本地编译工具链。
- 插件开发文档应补充常见预编译 wheel 依赖清单与避坑说明，减少用户误用需要本地编译工具链的包。

Node.js 插件依赖隔离策略：

- Node.js 插件安装时，平台应在插件目录下创建独立 `node_modules/`，而不是共享全局依赖目录。
- v0.1 强制仅使用 `npm` 作为 Node.js 插件依赖安装器，避免引入 `pnpm`、`corepack` 和额外环境垫片导致的运行环境复杂度上升。
- 平台应通过 `.deps/` 中托管的 `npm install --ignore-scripts --prefix <plugin_dir>` 驱动本地安装，确保行为可控且与发行包内运行时一致，并默认阻断第三方包 `postinstall` / `preinstall` / `install` 生命周期脚本在宿主机原生权限下执行。
- 若插件目录包含 `package-lock.json`，平台应优先执行 `npm ci --ignore-scripts --prefix <plugin_dir>`，保证依赖版本锁定与更快的安装路径；仅在锁文件缺失时回退到 `npm install --ignore-scripts`。
- `manifest.dependencies.nodejs` 由平台解析后安装到该插件自己的工作目录内；`.deps/` 中的 Node.js 只提供基础解释器与共享下载缓存，不共享全局 `node_modules/`。
- Runtime 启动 Node.js 插件时应优先使用插件目录下已经解析完成的依赖环境，避免不同插件之间的版本污染。
- Runtime 在调用 `npm` 时，必须显式注入受控的用户配置路径，例如 `NPM_CONFIG_USERCONFIG=<empty-controlled-npmrc>`，切断宿主机 `~/.npmrc`、企业代理配置或其他全局 npm 配置对插件依赖解析结果的污染。Windows 与 Linux 均应指向平台生成的空白受控配置文件，而不是依赖宿主机默认用户目录。
- 若插件确实依赖本地构建脚本（如原生扩展编译、二进制下载器等），平台不得静默去掉 `--ignore-scripts`。只能在安装入口由管理员显式进行高危确认后，针对该次安装临时允许执行脚本，并在任务摘要、审计日志和插件来源风险提示中留下明确痕迹。
- 如 Node.js 插件 manifest 声明 `require_install_scripts = true`，平台必须在真正开始安装前就弹出高风险确认，而不是等首次安装失败后再让用户猜测是否需要重新授权。管理员拒绝时，安装应直接在预检阶段失败，并返回明确的风险拒绝摘要。
- 依赖安装因超时或网络失败中止后，不应保留半完成状态继续复用；下一次“重试安装”应从干净的临时目录重新解析，避免 `node_modules/` 残留污染后续结果。
- 其他 Node.js 包管理器如 `pnpm`、`corepack` 等扩展路径，放到 `v0.2+` 结合运行环境策略统一评估。

##### 插件安装与依赖解析流程

- Web UI、CLI 或其他管理入口触发安装时，平台先解析 `info.json` 的 `type`、`runtime` 和 `dependencies`。
- 对本地目录来源，平台也应先复制到临时工作目录完成校验和依赖安装，再原子替换到正式安装目录，避免把半完成状态直接暴露给运行中的服务。
- 对 Python 插件，平台使用 `.deps/` 中托管的 Python / pip 在插件目录下创建或更新 `.venv/`，并默认优先拉取预编译 wheel。
- 对 Node.js 插件，平台使用 `.deps/` 中托管的 Node.js 与 `npm` 在插件目录下创建或更新 `node_modules/`。
- Python / `pip` 与 Node.js / `npm` 依赖安装子进程必须受绝对超时保护；默认以 `runtime.dependency_install_timeout_seconds` 为上限。超时后 Runtime 必须强制终止子进程、释放安装锁并清理临时目录，最终把安装任务标记为 `failed`（`platform.task_timeout`），不得让后台任务无限停留在 `running`。
- Node.js 依赖安装默认必须带 `--ignore-scripts`。若管理员显式允许本次安装执行生命周期脚本，任务模型必须把该安装标记为高风险路径，并在 Web UI / CLI 中显示“允许安装脚本”警告，避免用户误以为仍处于默认安全策略下。
- 若 manifest 已声明 `require_install_scripts = true`，安装入口必须在依赖解析前先展示高风险确认；管理员确认后，该次任务才允许去掉 `--ignore-scripts`。未声明该字段但安装时仍因脚本被禁用而失败时，平台可以在失败摘要中提示“该插件可能需要安装脚本授权”，但不得自动放宽策略并偷偷重试。
- 由于 `.deps/` 与 `cache/downloads/` 下的共享缓存可能被 `pip` / `npm` 高频读写，v0.1 不允许多个环境构建任务并发调用底层包管理器。所有会写共享缓存的依赖安装、升级、重建任务必须经过同一把全局互斥锁或顺序队列，避免缓存索引损坏、锁争用和偶发安装失败。
- 对 zip 安装包，平台必须先在临时目录完成解包，并要求解包后只识别一个插件根目录；`info.json` 缺失或校验失败时，安装应在依赖解析前直接终止。
- 在 ZIP 解包阶段，平台必须强制校验所有文件条目的解压目标路径；任何包含 `../`、绝对路径或解析后跳出插件临时目录的条目，都必须直接触发安装熔断并清理临时文件，防御 Zip Slip 目录穿越攻击。
- 已存在同 `id` 插件时，默认拒绝直接覆盖；仅升级流程允许在校验通过后以原子替换方式更新 `plugins/installed/<plugin_id>/`。
- 安装成功后，平台更新 `plugin_packages` / `plugin_instances` 等状态记录，并把插件标记为 `installed`。
- `plugin_packages` 或等价对象应记录至少以下来源元数据：安装来源类型（`zip` / 本地目录 / `builtin` / `dev`）、包 hash 或 manifest hash、安装时间、当前生效版本。
- 安装失败时，平台应清理临时目录、回滚半完成状态、保留错误摘要，并把插件置于可观测的失败状态，而不是留下不可预测的脏目录。
- 如 Python 依赖因缺失系统编译工具链或 C 扩展构建失败，平台应明确返回依赖编译错误摘要，并阻止插件进入自动重启循环。
- 当 Python 插件因缺失宿主机系统级动态库导致模块加载失败（如 `ImportError: libGL.so.1`、`OSError: libffmpeg` 等特征错误）时，Runtime 应捕获该类异常模式，提取 manifest 中 `system_dependencies` 字段的提示信息（若存在），并将结构化的系统依赖缺失摘要暴露给 Web UI 和日志，引导自托管用户在宿主机补充相应 OS 包，而不是将原始堆栈直接抛给插件作者。
- 对 Web 安装入口，依赖解析与安装过程应以后台任务方式执行，而不是阻塞单个 HTTP 请求直到完成；进度和终端输出应通过任务流回传给前端。
- 升级与卸载的数据保留策略详见 3.5.6；v0.1 仅强制实现临时目录安装、失败清理和同 `id` 默认拒绝覆盖。

#### 3.5.6 插件升级与卸载数据保留规则

- 物理目录约定应固定为：插件包与私有运行时位于 `plugins/installed/<plugin_id>/`，插件业务数据位于 `data/plugins/<plugin_id>/`，插件缓存位于 `cache/plugins/<plugin_id>/`，配置快照可位于 `data/plugins/<plugin_id>/config_snapshot/` 或等价的受控持久化位置。
- `plugins/dev/<plugin_id>/` 与 `plugins/builtin/<plugin_id>/` 只承载源码或随包分发内容，不应用作长期业务数据目录；需要持久化的数据仍统一落到 `data/plugins/<plugin_id>/`。
- 升级插件时，默认保留插件已授权的配置、启用状态和业务数据，不因包版本替换而自动清空用户侧数据。
- Python 插件的 `.venv/` 可在依赖声明兼容时复用；如依赖变更较大或锁文件不兼容，则应在临时目录重新构建后再原子替换。
- Node.js 插件的 `node_modules/` 不应把“依赖声明兼容”视为宽松复用条件。只要 `package.json` / `package-lock.json` 中的依赖集发生变化，或平台提供的 Node.js 环境 ABI 发生变化，平台就应在临时目录执行全新的 `npm ci` / `npm install` 后再原子替换，避免 `NODE_MODULE_VERSION mismatch`、原生扩展残留和玄学崩溃。
- 如插件声明了 `data_schema_version` 或等价元数据，平台在升级后应先触发插件数据迁移窗口，再决定是否恢复自动启用状态。
- 只要本次升级会触发 `data_schema_version` 变化，平台在真正执行插件私有数据迁移前，必须先把 `data/plugins/<plugin_id>/` 原子快照到 `data/plugins/<plugin_id>/snapshots/schema-v{old_version}-{timestamp}/` 或等价受控目录，再进入迁移窗口。快照失败时不得继续执行迁移。
- 插件私有数据语义由插件自身负责；平台只负责提供迁移触发时机、记录迁移结果和在失败时阻止插件自动启用，不负责理解或改写插件私有业务数据结构。
- 插件数据迁移失败时，平台应保留原业务数据目录与错误摘要，把插件保持在 `installed` 或明确的受阻状态，等待人工处理，而不是静默跳过迁移继续运行。
- 若后续检测到管理员正在降级插件版本，或恢复了一个较旧版本的插件包，而当前业务数据已经处于更高 `data_schema_version`，平台不得直接自动启用旧插件读取新数据。若存在匹配版本的数据快照，应提示管理员显式回滚到对应快照；若不存在，则至少保持阻止启用并输出明确兼容性摘要。
- 卸载插件时，默认移除插件包目录与私有运行时环境，但保留插件业务数据和配置快照，便于后续重新安装或人工恢复。
- 如用户明确选择“彻底删除”，平台才额外清理插件数据、配置快照和状态记录；该操作应在 Web UI 或 CLI 中显式提示不可逆。

#### 3.5.7 隔离与恢复策略

- v0.1 优先通过能力授权、工作目录约束与显式降级告警实现风险控制，不承诺操作系统级强沙盒。
- Python / Node.js 插件默认通过 `subprocess` 以普通子进程运行，由 Runtime 统一管理启动、退出、工作目录和可观测性。
- 出站网络访问优先通过平台 `http.request` 能力发起，并受 `permissions.scopes.http_hosts` 约束；如需接收外部回调，则应通过平台 `event.expose_webhook` 能力走统一入站网关，而不是让插件自建监听端口。如目标平台无法完整实施受支持网络边界，Runtime 必须显式记录降级告警，而不是静默放宽边界。
- 插件崩溃恢复由 `Plugin Runtime` 负责，不由 Core 直接管理单个插件进程。
- 异常退出时执行指数退避重试。
- 连续崩溃达到 3 次后进入 `dead_letter`。
- 进入 `dead_letter` 后仅允许手动重启或重新启用。
- 所有恢复动作写入结构化日志，并通过事件流推送给 Web UI / Launcher。

**v0.1 最小沙盒执行矩阵**：

| 维度 | 目标态 | 允许的例外 | 无法保证时的降级态 | UI / `doctor` / 诊断包展示 |
| --- | --- | --- | --- | --- |
| 网络访问 | 官方支持的网络能力分为出站 `http.request` 与入站 `event.expose_webhook`；官方 SDK、内置插件和示例插件不得依赖裸 socket、任意 TCP/UDP、插件自建 HTTP Server 或其他绕过平台代理的监听方式 | `permissions.scopes.http_hosts` 白名单内的主机；Webhook 仅允许走平台固定路由并要求显式鉴权策略；如需访问内网地址，必须由用户在 `user.yaml` 中显式授予受控例外 | `sandbox.direct_network_unrestricted` | 必须统一显示为“插件可直接外连，平台网络边界未完全生效”，并附 remediation 提示，不得仍标记为“已隔离” |
| 文件访问 | 平台公开的可写文件能力仅为 `storage.file`，范围固定在 `data/plugins/<plugin_id>/`；插件包目录、`.deps/`、`config/`、`data/` 根目录不属于受支持可写 API | 无 | `sandbox.package_dir_write_unenforced` | 必须显示受影响目录范围和风险说明，提示用户不要把插件包目录视为只读可信边界 |
| 进程衍生 | 平台内置 Python / Node.js 环境插件不得依赖再启动额外子进程作为受支持能力；Runtime 管理的插件主进程是 v0.1 唯一正式承诺的执行边界 | 预编译二进制插件不属于本规则约束范围，但也不进入 v0.1 主验收路径 | `sandbox.child_process_unenforced` | 必须标记为“衍生进程不可受控”，并在插件详情、环境检查和诊断包中统一暴露 |
| 环境变量 | 插件默认只继承最小白名单环境变量和显式授予的 `RAYLEABOT_SECRET_*`；宿主机其他环境变量不得默认透传 | `config/user.yaml` 中存在显式白名单时可追加少量变量 | `sandbox.extra_env_passthrough` | 必须显示额外透传的环境变量名、来源和授予入口；不得只在实现内部默许 |
| stdout / stderr / IPC | `stdout` 只承载 JSONL 协议，`stderr` 和插件日志接口承载调试输出；单条 IPC 消息默认上限 8 MB，且 Bridge 必须使用有界队列、背压和速率阈值约束插件输出 | 无 | `sandbox.ipc_limit_unenforced` | 必须显示为“协议边界退化”，并给出当前实际阈值或未生效项，便于定位洪泛风险 |

说明：

- 上表定义的是 v0.1 的“最低受支持执行边界”，不是操作系统级强沙盒承诺。
- 任何无法落实的边界都必须通过 `doctor`、诊断包、环境检查或管理界面显式暴露为降级状态，而不是靠文档默认假设其已生效。
- 官方 SDK、内置插件和示例插件必须严格遵守上述最小矩阵；第三方插件即使绕过，也不应被视为平台正式支持的能力面。
- 对所有沙盒降级项，Web UI、`doctor` 和诊断包必须复用同一条降级记录结构，至少包含 `code`、`severity`、`summary` 和 `remediation` 四个字段，避免不同入口各自发明展示口径。

崩溃时的资源清理行为：

- 已提交但未完成的渲染任务：由 Render Service 按 `render.timeout_seconds` 超时回收，渲染结果作废。
- 已注册的定时任务：Scheduler 暂停该插件的所有关联任务，直到插件恢复到 `running` 状态后重新激活。
- 进行中的消息发送：由 Adapter 层按已有超时策略回收；已经提交到 OneBot11 的消息不可撤回。
- 未完成的 KV 写入：v0.1 不提供事务回滚保证，SQLite 层面的单次写入具有原子性，但插件层面的多步操作序列不保证原子。
- 未完成的 HTTP 请求：由平台 HTTP 代理层按 `http.timeout_seconds` 超时取消。
- 插件进程回收后，Runtime 应清理与该进程关联的所有挂起 `request_id`，向等待结果的 Bot Core 返回 `plugin.internal_error`。

#### 3.5.8 官方内置插件与示例插件规划

v0.1 建议至少规划以下 `plugins/builtin/` 官方内置插件：

- `help`
- `status`
- `ping`
- `menu`
- `echo`

v0.1 建议至少规划以下示例插件：

| 示例插件 | 目标语言 | 主要覆盖能力 | 目的 |
| --- | --- | --- | --- |
| `example-min-reply` | Python | `event.subscribe`、`message.reply` | 最小消息闭环，验证官方 SDK 的最低可用路径 |
| `example-command-router` | Python | 命令解析、`message.send`、权限级别 | 演示命令插件、参数处理与权限控制 |
| `example-node-basic` | Node.js | `event.subscribe`、`logger.write`、`config.read` | 验证 Node.js 官方运行时与 SDK 基础链路 |
| `example-render-card` | Python | `render.image`、`message.send` | 覆盖渲染服务、缓存键和图片发送路径 |
| `example-scheduler` | Python | `scheduler.create`、`scheduler.trigger` | 验证调度事件、幂等与控制事件通道 |
| `example-config-panel` | Node.js | `config.read`、`config.write` | 演示插件命名空间配置读写与热更新感知 |
| `example-webhook` | Python | `event.expose_webhook`、`webhook.received` | 演示统一入站 Webhook 网关、签名校验与事件投递 |
| `example-permission-scope` | Python | `http.request`、`storage.file`、scope 授权失败路径 | 覆盖最小权限、作用域拒绝和审计记录 |
| `example-crash-backoff` | Node.js | 崩溃恢复、`backoff`、`dead_letter` | 验证 Runtime 退避、告警与恢复状态展示 |

说明：

- 官方内置插件用于保证首装即可体验平台能力，并与第三方插件共用同一套能力模型、生命周期和管理入口。
- 示例插件用于降低插件作者学习成本，并验证 SDK、协议和渲染能力是否真实可用。
- 官方内置插件与示例插件都必须走同一套 Capability / Permissions / 审计模型；除系统保留命令命名空间外，不允许存在“内置插件隐式特权”。

#### 3.5.9 插件热重载策略

v0.1 统一定义热重载协议与状态流转；优先保证手动热重载统一可用，其中仅强制 `plugins/dev/` 支持自动文件监听热重载，`plugins/builtin/` 与 `plugins/installed/` 保持手动热重载支持，更复杂的自动化热重载策略在后续版本逐步完善。

统一规则：

- v0.1 对 `plugins/builtin/`、`plugins/installed/` 和 `plugins/dev/` 都统一提供“不重启 `raylea-server` 的手动热重载”能力，但只有 `plugins/dev/` 默认承担自动文件监听热重载要求。
- Runtime 执行热重载时，应先向旧进程发送 `shutdown` 指令并标记原因为 `reload`，等待短暂优雅退出窗口；超时后再强制回收。
- 新进程启动后必须重新完成 `init`、能力注册、事件订阅和状态上报，最终恢复到原插件的 `desired_state`。
- 热重载失败时，不应影响核心服务继续运行；原插件应进入明确的错误或 `backoff` 状态，并向 Web UI / Launcher 推送结果。

触发方式：

- `plugins/dev/`：开发模式下默认启用文件监听，必要时可手动关闭。检测到 `.py`、`.js`、manifest 或关键模板文件发生保存修改时，自动热重载对应插件。
- `plugins/installed/`：v0.1 支持手动热重载；当插件包被更新、重新安装或配置变更要求重载时，由 Runtime 执行同一套热重载流程。
- `plugins/builtin/`：v0.1 支持手动热重载；更完整的监听策略放到后续版本按需补充。

实现边界：

- v0.1 不要求做到“进程内状态无损迁移”，但要求做到“服务不停机、插件进程替换、失败可观测”。
- 热重载期间可复用既有状态模型，按 `running -> stopping -> starting -> running` 或失败回落到 `backoff` / `dead_letter`，不额外引入专门的 `reloading` 状态。
- 长耗时任务中断与重投、细粒度静态资源监听规则、`plugins/installed/` / `plugins/builtin/` 的更完整自动化热重载策略，放到 v0.2 以后逐步完善。
- Web UI、CLI 和后续自动化入口应复用同一套热重载接口，不得为不同插件类型发明多套流程。

#### 3.5.10 插件包生命周期状态机

以下状态机将插件从发现到移除的完整生命周期串联起来，覆盖安装、授权、启用、运行、升级、迁移和卸载各阶段。实现时应以本节为唯一参考，避免各模块各自定义中间状态。

```
discovered → installing → installed → permission_pending → ready_to_enable
                                          ↓                        ↓
                                    permission_denied         enabling (starting)
                                                                   ↓
                                                              running ⇄ crashed → backoff → dead_letter
                                                                   ↓
                                                     upgrade_pending → migration_pending → ready_to_enable
                                                                   ↓
                                                           uninstalling → removed
```

**主要状态说明**：

| 状态 | 对应建模层 | 说明 |
| --- | --- | --- |
| `discovered` | — | 插件包被扫描发现，尚未注册到平台 |
| `installing` | — | 后台任务执行中：解析 manifest、安装依赖、校验签名 |
| `installed` | `registration_state=installed` | 插件包已注册，依赖已就绪 |
| `permission_pending` | `desired_state=enabled` | 存在未确认的 `required` 权限（见 3.6.2 权限授予状态机） |
| `ready_to_enable` | `desired_state=enabled` | 权限已确认，可进入启动流程 |
| `enabling` | `runtime_state=starting` | Runtime 正在启动插件进程，等待 `init_ack` |
| `running` | `runtime_state=running` | 插件正常运行中 |
| `crashed` / `backoff` / `dead_letter` | `runtime_state=*` | 崩溃恢复流程（见 3.5.7） |
| `upgrade_pending` | — | 升级后检测到权限扩大或版本不兼容，等待重新确认 |
| `migration_pending` | — | `data_schema_version` 变化，等待数据迁移完成 |
| `uninstalling` | — | 卸载后台任务执行中 |
| `removed` | `registration_state=removed` | 插件包已移除，审计记录保留 |

说明：

- `permission_pending` 和 `upgrade_pending` 是阻塞状态：插件不得自动进入 `enabling`，直到管理员完成确认。
- `migration_pending` 中迁移失败时，插件保持该状态且阻止启用，错误摘要暴露给 Web UI 和日志。
- 本状态机是对 3.3.2 三层状态模型（`registration_state` / `desired_state` / `runtime_state`）的流程补充，不替代三层状态的正式定义。

### 3.6 平台能力清单与插件声明

插件目录最小结构：

```plain
plugins
├─ installed
│  └─ weather
│     ├─ info.json
│     └─ plugin.py
└─ dev
   └─ debug-echo
      ├─ info.json
      └─ plugin.js
```

#### 3.6.1 平台能力清单（Capabilities）

平台建议向插件开放一组稳定的标准能力，而不是让插件直接依赖内部实现细节。

建议首批能力至少包括：

| 能力 | 说明 |
| --- | --- |
| `message.send` | 发送消息 |
| `message.reply` | 回复当前消息上下文 |
| `event.subscribe` | 订阅平台统一事件 |
| `event.raw_payload` | 访问协议原始载荷，默认关闭，仅高级调试或协议敏感插件使用 |
| `config.read` | 读取插件可访问配置 |
| `config.write` | 写入插件授权范围内配置 |
| `storage.kv` | 读写插件独立 KV 存储 |
| `storage.file` | 读写插件工作目录文件 |
| `render.image` | 调用平台统一图片渲染服务 |
| `http.request` | 发送受限 HTTP 请求 |
| `event.expose_webhook` | 在主 HTTP 服务上注册受控 Webhook 子路由，并将入站请求转为 `webhook.received` 内部事件 |
| `scheduler.create` | 创建计划任务或延迟任务 |
| `logger.write` | 写入插件结构化日志 |

设计要求：

- SDK 应优先面向平台能力设计，而不是面向内部模块设计。
- `rayleabot-sdk-python` 和 `rayleabot-sdk-nodejs` 应提供 `PluginBase`、事件装饰器、能力调用封装和错误处理包装，避免插件作者直接拼装裸协议消息。
- `rayleabot-sdk-python` 的底层通信层应基于 `asyncio` 实现，并使用强类型校验模型（如 `Pydantic`）描述事件、动作、配置和返回结果。
- Python SDK 应提供类似路由注册的事件装饰器或注册器，同时暴露同步与异步两套事件处理包装器；简单逻辑可直接使用同步函数，涉及网络 I/O 或并发等待的插件再使用 `async` 处理函数。
- Python SDK 必须提供完整 Type Hints，使插件作者可直接获得 IDE 自动补全，例如 `event.message.text`、`event.target.group_id` 等字段提示。
- `rayleabot-sdk-nodejs` 应基于 TypeScript 类型定义与 `async/await` 提供等价的能力封装和 IDE 补全体验，保持跨语言开发体验一致。
- SDK 应为事件回调自动注入统一的 `ctx`（Context）对象，封装当前 `event_id`、消息上下文、回复目标和常用能力；插件作者调用 `ctx.reply(...)`、`ctx.send(...)` 时不应手动拼装底层请求字段。
- 官方 SDK 提供的 `ctx.http` 或等价网络请求能力必须默认走异步非阻塞实现；在 `async` 事件处理函数中，文档应明确警告不要直接使用 `requests.get()`、`time.sleep()` 等阻塞调用。
- Python SDK 应至少在开发态对单次事件处理长时间阻塞 Event Loop 的情况输出 Warning，例如同一事件处理持续阻塞超过 5 秒时给出明确提示，帮助初学者定位错误用法。
- 官方 SDK 必须接管未捕获异常处理（如 Python 的 `sys.excepthook`、Node.js 的 `uncaughtException` / `unhandledRejection`），在输出堆栈到 `stderr`、日志或调试控制台前，强制掩码所有以 `RAYLEABOT_SECRET_` 开头的环境变量值，避免凭据随崩溃信息泄漏。
- Bot Core 在将 `message.send`、`message.reply` 或由 `fallback_text` 触发的降级文本最终递交给 Adapter 前，必须对所有文本承载字段执行一次快速脱敏扫描。任何命中当前已加载 `secret_store` 有效值的文本片段，都必须替换为 `***`，以阻断插件作者误把 API Key、Token 或其他敏感凭据反射到公共聊天界面。
- 上述脱敏扫描至少应覆盖当前有效 secret 的原始值及常见直接拼接形态（如 `Bearer <token>` 与 URL query 中的裸值）；实现可跳过过短或明显不适合作为凭据的值，但不得因此放弃对高敏密钥的默认保护。
- 为避免弱密钥误杀正常文本语义，自动脱敏默认只对长度不小于 `8` 个字符，且不属于明显低复杂度常见词 / 纯短数字串的 secret 值生效。像 `test`、`123456`、`admin` 这类弱值不应默认进入全局文本替换规则。
- 若后续引入更强的 secret 元数据，可允许将个别短值显式标记为 `force_redact`；但在 v0.1 默认策略下，平台应优先保证聊天文本语义稳定，再兼顾自动脱敏覆盖率。
- 同一套全局脱敏引擎还必须作为中间层复用到 `logger.write`、结构化日志落盘、`/ws/logs`、`/ws/plugins/{id}/console`、诊断包和 CLI / Web 错误摘要；任何最终会被持久化到文件或广播给前端管理会话的字符串，都不得绕过脱敏滤网。
- `capabilities` 用于插件声明“我会调用哪些平台能力”，`permissions` 用于平台授予“这些能力在什么范围内可用”。
- 权限系统应围绕能力授权和作用域控制构建，不应把能力声明和授权结果混成一个字段。
- RPC 协议中的动作设计应和能力模型保持一致。
- `config.read` 只用于读取非敏感配置，不应返回 `secret_store` 中的插件密钥或令牌。
- `http.request` 必须由平台统一实现 DNS 解析后地址校验，默认拦截解析到局域网、回环、链路本地和其他 Bogon 地址段的请求；仅当用户在 `user.yaml` 中显式授予某插件内网访问权限时才可放行。
- 外部系统回调、Webhook 与其他入站 HTTP 通知不应通过插件自建监听端口实现。v0.1 如需支持入站 HTTP 事件，必须统一走 Bot Core 提供的固定网关路由（建议 `/api/webhooks/{plugin_id}/{route}`），并以 `webhook.received` 内部事件投递给目标插件。
- `event.expose_webhook` 默认属于高敏能力：插件必须显式声明、管理员显式授予后方可启用；Gateway 还应要求插件为每条路由声明最小鉴权策略（如固定 token、HMAC 签名或来源 IP 白名单），不允许“裸公开”回调入口默认暴露到网络。
- Webhook 网关不应猜测插件私有业务密钥。插件在注册 `event.expose_webhook` 路由时，必须显式提交该路由的网关鉴权契约，例如 `auth_strategy`、签名 / Token 所在请求头、可选签名前缀以及 `secret_ref`（指向插件作用域下由平台管理的 `secret_store` 条目）。Core 只负责按声明的通用策略做前置校验，不负责读取插件 `storage.kv` 或执行插件私有逻辑。

##### 3.6.1.1 高价值 Scope 正式示例

为避免不同模块、不同 AI 会话各自发明 scope 语义，v0.1 对高价值能力至少应收敛到以下示例口径：

| 能力 | `permissions.scopes` 示例 | v0.1 正式约束 |
| --- | --- | --- |
| `http.request` | `"http_hosts": ["api.github.com", "api.weather.example"]` | v0.1 正式支持的最小粒度是主机白名单；默认仅放行 `https`；路径级规则不进入首版正式契约；涉及内网地址时必须额外通过平台配置显式放行 |
| `storage.file` | `"storage_roots": ["plugin_data"]` | `plugin_data` 固定映射到 `data/plugins/<plugin_id>/`；插件传入路径必须是相对路径；不支持通过 scope 把根目录扩大到插件目录外 |
| `event.expose_webhook` | `"webhooks": [{ "route": "github", "auth_strategy": "hmac_sha256", "header": "X-Hub-Signature-256", "secret_ref": "plugin.github.webhook_secret", "source_ips": ["140.82.112.0/20"] }]` | `route` 只允许受控短路径片段，不得含 `..`、`/` 或查询串；每条路由必须声明最小鉴权策略；来源 IP 白名单是可选收紧项，不是裸开放替代品 |
| `config.read` | `"config_read": ["plugin:<plugin_id>:settings", "plugin:<plugin_id>:cache"]` | 只允许读取调用方插件自己的非敏感命名空间；`secret_store`、平台全局根配置和其他插件命名空间不在可读范围内 |
| `config.write` | `"config_write": ["plugin:<plugin_id>:settings"]` | 只允许写入调用方插件自己的非敏感配置命名空间；写入行为必须保留审计记录并走配置 schema 校验 |
| `event.raw_payload` | 不依赖额外 scope 列表，必须显式单独授予能力 | 原始协议载荷属于高敏能力，不以宽泛 scope 代替显式确认；即使获授，也不应自动扩大平台日志、诊断包和调试控制台的敏感数据暴露范围 |

补充规则：

- 上表仅示例字段名与能力语义，最终机器可校验定义必须落在 `contracts/plugin-info.schema.json` 与配套权限 schema 中。
- 如后续确需扩展 path 粒度、方法粒度或更复杂的 webhook 校验器，必须先更新 `contracts/` 下的正式 schema，再更新实现和 UI。

**官方 SDK API 设计草案**：

- Python SDK 以 `PluginBase` + 装饰器 / 注册器为主接口，建议最小方法集合为：`on_load(ctx)`、`on_unload(ctx, reason)`、`on_event(ctx, event)`；其中消息与通知类事件优先通过更细粒度装饰器暴露，而不是要求插件作者手写事件分发。
- Python SDK 事件处理建议形态：

```python
class WeatherPlugin(PluginBase):
    @on_message(command="weather")
    async def handle_weather(self, ctx: EventContext) -> None:
        city = ctx.args[0]
        result = await ctx.http.get_json("https://api.weather.example", params={"q": city})
        await ctx.reply(f"{city}: {result['temp']}C")

    @on_event("notice.member_increase")
    async def welcome(self, ctx: EventContext) -> None:
        await ctx.send_text("欢迎新成员")
```

- Node.js SDK 的 v0.1 正式基线以注册式 API 为主，而不是强制使用 TypeScript 方法装饰器；原因是装饰器会额外引入编译配置、提案兼容和调试复杂度，不利于首版工程收口。
- Node.js SDK 建议形态：

```ts
export default definePlugin({
  setup(api) {
    api.onMessage({ command: "weather" }, async (ctx) => {
      const city = ctx.args[0];
      const result = await ctx.http.getJson("https://api.weather.example", { q: city });
      await ctx.reply(`${city}: ${result.temp}C`);
    });

    api.onEvent("notice.member_increase", async (ctx) => {
      await ctx.sendText("欢迎新成员");
    });
  }
});
```

- 若后续 Node.js SDK 额外提供 `@onMessage()` 或等价装饰器语法，应仅作为语法糖包裹上述注册式接口；底层契约、示例项目和测试用例仍以注册式 API 为唯一基线，避免 class decorator 成为强制依赖。
- 两套 SDK 都应统一暴露 `ctx.reply()`、`ctx.send()`、`ctx.renderImage()`、`ctx.http.*`、`ctx.storage.*`、`ctx.logger.*` 和 `ctx.config.*` 等高频能力，并保证签名和错误模型尽量对齐。

`storage.kv` API 语义：

- 操作类型：`get`、`set`、`delete`、`list`（可选）。
- 每个插件拥有独立的 KV 命名空间，key 前自动加 `plugin_id` 前缀，插件无法跨命名空间访问。
- KV 单值大小限制：默认 `64 KB`；超过限制时返回 `platform.value_too_large` 错误。
- 总容量限制：默认每个插件 `16 MB` KV 总存储，可通过 `config/user.yaml` 调整。
- KV 底层由 SQLite 实现，v0.1 不需要额外引入 Redis 等外部依赖。
- `list` 操作支持按前缀查询，返回匹配的 key 列表（不含 value），用于插件自行管理数据结构。

`storage.file` API 语义：

- `storage.file` 是 `data/plugins/<plugin_id>/` 目录的受控 API 封装，插件通过平台能力间接操作该目录。
- 文件操作：`read`、`write`、`delete`、`list`。
- 路径约束：所有路径必须是相对路径。实现层必须先执行路径规范化（如 Go 的 `filepath.Clean`）去除 `.`、`..` 和重复分隔符，再基于受控根目录拼接绝对路径；任何规范化后落到插件数据目录外的路径都必须直接拒绝。
- 软链接防御：对目标文件及其父目录链中的符号链接必须做显式检查。若任一路径分量本身是符号链接，或经 `EvalSymlinks` / 等价解析后落到 `data/plugins/<plugin_id>/` 外，平台必须拒绝访问，不能只依赖字符串前缀判断。
- 单文件大小限制：默认 `10 MB`，可配置。
- 插件工作目录总大小受 `storage.plugin_workdir_soft_limit_mb` 配置约束。
- `read` 操作返回文件内容（文本或 base64 编码）；`list` 返回文件名列表。
- v0.1 不提供目录创建或递归操作，插件如需子目录结构应通过 key 中的路径分隔符自行管理。

OneBot11 原生 API 转发规范：

- v0.1 优先通过平台标准能力封装常用操作（`message.send`、`message.reply`），不直接开放 OneBot11 原生 API 透传。
- 如插件需要调用 OneBot11 的原生 API（如 `get_group_member_list`、`set_group_ban` 等），v0.1 暂不提供通用透传通道。
- 后续版本可按需将高频原生 API 封装为平台能力（如 `group.get_members`、`group.ban`），通过统一的能力声明和权限控制开放。
- v0.1 开放的消息动作限定为 `message.send` 和 `message.reply` 两种，覆盖绝大多数插件场景。

v0.1 SDK 最小承诺范围：

- 事件订阅与事件对象封装。
- 同步 / 异步事件处理包装器与装饰器注册模式。
- `message.send` 与 `message.reply`。
- `render.image`。
- 基于 `ctx` 的回复 / 发送上下文封装，以及带 `fallback_text` 的渲染降级辅助。
- `logger.write`。
- 基础配置读取。
- 常见协议错误与平台错误包装。
- **Action 请求客户端超时**：SDK 必须在底层为所有发往 Core 的 Action 请求（如 `message.send`、`render.image`、`storage.kv` 等）实现客户端级超时控制，默认 30 秒。若超时未收到 `result` 或 `error`，SDK 应主动抛出超时异常或返回失败结果，确保插件的事件循环不会因 Core 端无响应（如渲染队列阻塞、Core 进程重启）而被永久卡死。该超时值应可通过 SDK 配置调整。

稳定性边界：

- v0.1 明确承诺稳定的是插件协议、统一事件模型、Capabilities 命名体系和 manifest 结构。
- SDK 内部实现细节、目录组织和高级辅助函数不作为 v0.1 的长期稳定 API 承诺，可在小版本内继续调整。

以下能力不要求在 v0.1 SDK 首版完整提供：

- 高级调度器封装。
- 高级热重载辅助工具。
- 复杂存储抽象。
- 远程安装、发布或插件市场辅助能力。

#### 3.6.2 插件 manifest 规划

`info.json` 建议字段如下：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `id` | 是 | 插件稳定唯一标识 |
| `name` | 是 | 插件展示名称 |
| `version` | 是 | 插件版本，建议语义化版本 |
| `manifest_version` | 是 | manifest 结构版本，v0.1 固定为 `1` |
| `plugin_protocol_version` | 是 | 插件 JSONL 协议版本，用于与 Runtime 判定兼容性 |
| `sdk_min_version` | 否 | 插件依赖的官方 SDK 最低版本 |
| `type` | 是 | 插件形态，如 `managed_runtime`、`binary`、`dev_source` |
| `entry` | 是 | 插件入口文件或可执行文件路径 |
| `runtime` | 是 | 运行时类型，如 `python`、`nodejs`、`binary` |
| `runtime_version` | 否 | 运行时版本下限要求，如 `>=3.10`、`>=18` |
| `data_schema_version` | 否 | 插件私有数据结构版本，用于升级时触发插件数据迁移窗口 |
| `license` | 是 | 开源许可证标识，如 `MIT` |
| `min_core_version` | 否 | 插件要求的最小核心版本 |
| `platforms` | 否 | 支持的平台列表，如 `windows-x64`、`linux-x64` |
| `role` | 否 | 插件语义角色，如 `builtin`、`user`、`example`、`dev` |
| `icon` | 否 | 相对插件目录的图标路径，用于 Web UI 和菜单展示 |
| `description` | 否 | 插件简介 |
| `author` | 否 | 作者标识 |
| `capabilities` | 否 | 插件声明会使用的能力集合，如 `event.subscribe`、`message.send`、`render.image` |
| `commands` | 否 | 插件处理的命令声明列表，含命令名、别名、权限级别等，详见 3.4.5 |
| `concurrency` | 否 | 插件并发处理上限，如 `1` 表示串行处理，不填则使用全局默认值，详见 3.4.1 |
| `permissions` | 否 | 插件权限声明，建议结构化而非简单字符串数组 |
| `dependencies` | 否 | 插件级依赖声明 |
| `require_install_scripts` | 否 | 布尔值，仅在 Node.js 插件确实需要 `npm` 生命周期脚本或本地构建时声明；用于安装前高风险确认 |
| `repo` | 否 | 插件仓库或主页地址 |
| `homepage` | 否 | 插件主页或文档地址 |
| `keywords` | 否 | 插件检索关键词数组，供后续索引或市场使用 |
| `screenshots` | 否 | 相对插件目录的预览图路径数组，供后续管理台或插件市场展示 |
| `system_dependencies` | 否 | 宿主机系统级依赖声明（纯提示性，不强制自动安装），如 `["libgl1", "ffmpeg"]`；用于运行时缺失检测与 Web UI 提示 |

约束：

- 插件安装校验、CI 契约校验和后续 SDK 类型生成应以 `contracts/plugin-info.schema.json` 为唯一机器可校验来源；本节正文用于解释语义，不替代正式 schema。

`permissions` 建议结构：

```json
{
  "required": [
    "message.send",
    "http.request"
  ],
  "optional": [
    "event.subscribe"
  ],
  "scopes": {
    "http_hosts": [
      "api.weather.example"
    ],
    "storage": [
      "plugin_data"
    ]
  }
}
```

说明：

- v0.1 可以只强校验 `required`，但结构必须预留 `optional` 和 `scopes`，避免后续从字符串数组重构成对象。
- `permissions.required` 与 `permissions.optional` 应与 `capabilities` 使用同一套能力命名体系，便于审计和鉴权。
- `license` 是开源协作与后续插件分发的基础元数据，v0.1 就应作为必填字段校验。
- `icon` 应使用相对插件目录的路径，便于 Web UI、菜单和后续索引页统一展示。
- `repo`、`homepage`、`keywords`、`screenshots` 在 v0.1 主要作为元数据保留字段；Web UI 可选择性展示链接与本地预览，但不得在安装阶段自动抓取远程截图或远程仓库内容。
- `dependencies` 只描述插件所需依赖，不直接定义完整工具链安装策略；Python 插件的第三方库应默认安装到插件目录下的独立 `.venv/` 中。
- `require_install_scripts` 只是插件对风险路径的显式声明，用于让平台在安装前就触发确认；它本身不等于自动授权，也不能绕过默认的 `--ignore-scripts` 安全策略。
- v0.1 的 `dependencies` 仅覆盖语言级包依赖（Python pip 包、Node.js npm 包），不支持插件间依赖（即插件 A 依赖插件 B）。v0.3+ 可评估引入 `plugin_dependencies` 字段或等价机制，允许插件按 `id` 和版本范围声明对其他插件的依赖，由平台负责安装校验、加载排序和缺失依赖错误报告。在此之前，依赖其他插件能力的插件应在 `description` 中文档化前置要求，并在运行时优雅降级。
- `runtime_version` 用于约束平台托管解释器的最低版本；当 `.deps/` 中的运行时版本低于要求时，平台应在安装或启用前直接拒绝，而不是等插件进程启动后抛出语法或运行时错误。
- `data_schema_version` 只描述插件私有数据版本，不改变平台状态库 schema；当版本升级触发数据迁移时，平台只负责调用受控迁移窗口并记录结果。
- 需要平台渲染能力的插件应在 `capabilities` 中声明 `render.image`。
- 如插件确实需要访问底层协议原始载荷，需额外声明 `event.raw_payload`，并由平台在高敏权限审核后显式授予。
- `role` 主要用于标识插件来源与管理语义，不应用目录路径代替该字段做唯一判断。
- `manifest_version` 用于校验 `info.json` 结构本身是否可被当前 Core 解析；不兼容时应在安装阶段直接拒绝。
- `plugin_protocol_version` 用于判定插件与 Runtime 的 JSONL 协议兼容性；不兼容时应拒绝启用或加载。
- `sdk_min_version` 仅用于约束官方 SDK 辅助能力的最低版本，不影响非官方 SDK 或手写协议插件的基础协议加载。

`role` 字段与目录位置的关系说明：

- `role` 是 manifest 中的声明性字段，用于管理语义分类和 Web UI 展示（如标记为"内置"、"用户"、"开发"等）。
- 目录位置（`builtin/`、`installed/`、`dev/`）是运行时的物理来源标识，决定插件的加载路径和管理入口。
- 两者应保持一致但不完全等价：`builtin/` 下的插件 `role` 应为 `builtin`，`installed/` 下应为 `user`，`dev/` 下应为 `dev`。
- 平台在加载插件时应校验 `role` 与目录位置的一致性；不一致时记录 `WARN` 级别日志，但不阻止加载。
- 不一致的典型场景：开发者在 `dev/` 下调试一个 `role: user` 的插件，此时平台按 `dev/` 目录的行为规则运行（如自动热重载），同时保留 manifest 中的 `role` 元数据。

权限授予流程：

- 安装插件时，`permissions.required` 必须由管理员显式确认后才可生效；未完成确认的插件不得自动启用。
- `permissions.optional` 默认不启用，用户可在后续管理界面中按需单独授予。
- 插件升级若新增 `required` 权限、扩大 `scopes` 或引入更高敏感等级能力，平台不得沿用旧授权结果，必须重新进入待确认状态。
- 插件升级若删除了部分权限声明，平台应同步回收或忽略多余授权，避免遗留“幽灵权限”继续生效。
- 权限授予、撤销、升级触发重新确认等动作都必须持久化到 `permission_grants` 与审计记录中，至少记录插件 `id`、权限变更内容、操作者、时间和来源入口。
- Web UI 与诊断包应能追溯当前生效授权和最近一次权限变更摘要，便于排查”为何插件突然获得某项能力”。

**权限授予状态机**：

插件的每项能力授权在生命周期中经历以下状态流转：

```
declared → pending_grant → granted → revoked
                                  ↗
              upgrade_pending_regrant
```

| 状态 | 含义 | 触发条件 |
| --- | --- | --- |
| `declared` | 插件在 manifest 中声明了该能力，但尚未安装或提交审批 | 插件包注册时 |
| `pending_grant` | 等待管理员确认授予 | 首次安装或升级引入新 `required` 能力时 |
| `granted` | 管理员已确认，能力生效 | 管理员在 Web UI 中批准 |
| `revoked` | 管理员主动撤销授权 | 管理员在 Web UI 中撤销，或插件卸载时 |
| `upgrade_pending_regrant` | 插件升级导致该能力需重新确认 | 升级后 `required` 新增、`scopes` 扩大或敏感等级提升 |

**自动授予与人工确认规则**：

- `event.subscribe`、`logger.write` 等基础低敏能力可在首次安装时自动授予（`declared → granted`），不要求人工确认。
- `http.request`、`event.expose_webhook`、`storage.file`、`render.image`、`event.raw_payload` 等涉及网络访问、入站暴露面、文件操作或敏感数据的能力必须人工确认（`declared → pending_grant → granted`）。
- 自动授予的能力范围应可通过 `config/user.yaml` 的 `permission.auto_grant_capabilities` 配置项调整，默认值仅包含最小安全集。

**升级重确认触发条件**：

- 升级后新增 `permissions.required` 中的能力条目。
- 升级后 `permissions.scopes` 扩大（如 `http_hosts` 新增域名）。
- 升级后引入更高敏感等级的能力（如从 `storage.kv` 升级到 `storage.file`）。
- 以上任一条件命中时，受影响的能力进入 `upgrade_pending_regrant`，插件保持 `installed` 但不自动启用。

**卸载后审计保留**：

- 插件卸载后，`permission_grants` 中的授权记录和 `audit_logs` 中的权限变更记录应保留（不随插件卸载删除），保留时长遵循 `audit_logs` 的默认留存策略。
- 保留的授权记录应标记为 `revoked`（卸载触发），便于重新安装时追溯历史授权。

兼容性规则：

- `manifest_version` 不兼容时，平台必须在安装或扫描阶段直接拒绝加载，不进入依赖安装和插件启用流程。
- `plugin_protocol_version` 不兼容时，平台必须拒绝建立插件协议会话，而不是在运行期边试边错。
- 仅 manifest 元数据微调且不涉及 `plugin_protocol_version`、`runtime_version`、`data_schema_version`、权限扩大时，可通过手动热重载生效。
- `data_schema_version` 变化时，平台必须先触发插件数据迁移窗口；迁移失败时阻止自动启用。
- `sdk_min_version` 不满足时，官方 SDK 插件应明确拒绝启用并给出升级建议；平台不得把这类错误伪装成普通运行时崩溃。

参考示例：

```json
{
  "id": "weather",
  "name": "Weather",
  "version": "1.0.0",
  "manifest_version": "1",
  "plugin_protocol_version": "1",
  "sdk_min_version": "0.1.0",
  "type": "managed_runtime",
  "entry": "plugin.py",
  "runtime": "python",
  "runtime_version": ">=3.10",
  "data_schema_version": "2",
  "license": "MIT",
  "min_core_version": "0.1.0",
  "platforms": [
    "windows-x64",
    "linux-x64"
  ],
  "role": "user",
  "icon": "assets/weather.png",
  "description": "基础天气查询插件",
  "author": "raylea",
  "capabilities": [
    "event.subscribe",
    "message.send",
    "render.image"
  ],
  "commands": [
    {
      "name": "weather",
      "aliases": ["天气"],
      "description": "查询指定城市天气",
      "usage": "/weather <城市名>",
      "permission": "everyone"
    }
  ],
  "permissions": {
    "required": [
      "message.send",
      "http.request"
    ],
    "optional": [],
    "scopes": {
      "http_hosts": [
        "api.weather.example"
      ],
      "storage": [
        "plugin_data"
      ]
    }
  },
  "dependencies": {
    "python": [
      "requests>=2.0.0"
    ]
  },
  "require_install_scripts": false,
  "repo": "https://github.com/example/weather",
  "homepage": "https://example.com/weather",
  "keywords": ["weather", "utility"],
  "screenshots": ["assets/preview1.png"]
}
```

### 3.7 插件通信协议规划

#### 3.7.1 通信边界

**stdout 保留规则：官方 SDK 必须默认将 `print` / `console.log` 重定向到 `stderr` 或专用插件日志接口，避免普通调试输出污染 JSONL 协议流；Runtime 检测到非 JSONL 文本时应记录明确错误摘要。**

- Core 只和 `server` 内部的 Plugin Runtime Manager 通信。
- Plugin Runtime Manager 负责和实际插件子进程通信，本身不是独立可执行进程。
- v0.1 插件通信采用基于 `stdin/stdout` 的 JSON 消息通道，减少端口占用和跨语言复杂度。
- 所有基于 `stdin/stdout` 的协议消息必须采用 JSON Lines（JSONL）格式：每个 JSON 对象序列化为单行，并强制以换行符 `\n` 结尾。
- Bot Core、Runtime Bridge 与官方 SDK 的流读取器都应按行读取和反序列化，不依赖多行 JSON 拼接或长度前缀协议，以降低跨语言实现复杂度并避免粘包 / 截断歧义。
- 在使用 `stdin/stdout` 承载协议的前提下，`stdout` 必须视为保留给协议层的纯净通道；插件作者的调试输出、`print`、`console.log` 等普通文本默认不得直接写入该通道。
- 官方 SDK 应在初始化阶段把 Python 的 `sys.stdout`、Node.js 的 `console.log` / 标准输出包装重定向到 `stderr` 或插件日志接口，避免普通调试输出污染 JSONL 协议流。
- Runtime 如检测到插件 `stdout` 出现无法解析为 JSONL 的普通文本，应把它视为协议违规并给出明确错误摘要，而不是让 Core 端静默崩溃；该摘要应进入日志与调试控制台，便于插件作者定位问题。
- **JSONL 美化输出防护**：官方 SDK 底层的 JSON Encoder 必须强制关闭所有 `indent` / 美化选项（如 Python 的 `json.dumps(indent=...)` 和 Node.js 的 `JSON.stringify(obj, null, 2)`），并在序列化后确保整个 payload 中除结尾的 `\n` 外不包含任何字面换行符。Core 端的行读取器在捕获到无法完整解析的残缺 JSON 行时，应丢弃该行并记录 `plugin.protocol_violation` 告警，防止主解析循环因等待多行拼接而挂起。
- **stdout 管道缓冲防护**：Python 和 Node.js 在检测到 stdout 连接到管道（Pipe）而非终端（TTY）时，默认开启全缓冲（Block Buffering），导致 JSONL 消息被卡在 OS 缓冲区中（通常达 4-8KB 才刷出），引发无端的 `plugin.event_timeout`。官方 SDK 在底层向 stdout 写入 JSONL 消息时，必须在每写入一行后显式调用 `.flush()`。Runtime 在启动 Python 插件子进程时，必须通过 `PYTHONUNBUFFERED=1` 环境变量强制关闭 Python 输出缓冲。
- **Node.js stdout 背压防护**：Node.js SDK 不得把 `process.stdout.write()` 视为“永不阻塞”的同步调用。SDK 必须检查每次 `process.stdout.write()` 的返回值；若返回 `false`，说明内核 pipe 已满，SDK 必须暂停继续消费本地待发 JSONL 队列，并等待 `process.stdout` 的 `drain` 事件后再恢复写入。否则在 Core 暂时读慢时，Node.js 主线程会被 stdout 管道阻塞，进而连带卡死事件循环、`ping/pong` 和定时器。
- **心跳与业务回调解耦**：官方 SDK 不得把 stdin 读取、`ping` / `pong` 响应和用户业务 handler 复用到同一条可被用户代码长时间阻塞的执行路径上。Python SDK 应以独立后台线程持续读取 stdin、优先处理 `ping` / `shutdown` 等控制消息；Node.js SDK 应通过 Worker 线程、辅助 I/O 协调层或等价机制确保即使主线程正在执行同步 CPU 密集逻辑，也不会因为无法读取 stdin 而把心跳误判为进程死亡。
- **行读取器容量一致性**：若 Runtime Bridge 使用 Go `bufio.Scanner` 读取 JSONL，必须显式把 `Scanner.Buffer` 上限提升到 `runtime.ipc_message_max_bytes`；否则 Go 默认 `64 KB` token 限制会把合法大消息误判为协议错误。也可改用 `bufio.Reader` + 按行受控读取，但无论采用哪种实现，都不得存在无上限缓冲。
- **背压与积压防护**：Runtime Bridge 必须对每个插件维护有界的动作暂存队列与速率计数器，不能把 `stdout` 读取结果直接写入无界 channel。若插件在短时间内发出超过 `runtime.ipc_action_burst_limit` 的 `action` 请求，或待处理动作数超过 `runtime.ipc_pending_actions_max`，Bridge 应先施加背压（例如暂停继续读取该插件 stdout，让 OS pipe 反向阻塞插件写入）；若在短暂宽限窗口后仍持续超限，则必须终止整个插件进程组，并按 `plugin.protocol_violation` 处理。
- **stderr 洪泛防护**：`stderr` 虽不承载 JSONL 协议，也不得视为无上限黑洞。Runtime Bridge 必须对每个插件的 `stderr` 读取施加滑动窗口或令牌桶限流，默认按 `runtime.stderr_rate_limit_bytes_per_second` 做硬阈值保护；超过阈值的输出应直接截断丢弃，并向该插件日志与调试控制台注入一条系统告警（如 `[System] stderr rate limit exceeded, output truncated`）。当输出速率回落到阈值内后，Bridge 才恢复正常透传，避免纯文本洪泛拖垮磁盘 I/O、日志队列或 Core 内存。
- 插件协议的正式机器可校验定义应收敛到 `contracts/plugin-protocol.schema.json`（或同等结构化契约文件）；本章中的示例和说明不得与其冲突。

#### 3.7.2 公共消息字段

所有协议消息建议共享以下字段：

| 字段 | 说明 |
| --- | --- |
| `protocol_version` | 运行时协议版本 |
| `type` | 消息类型 |
| `timestamp` | 消息发送时间 |
| `plugin_id` | 插件标识 |
| `request_id` | 请求标识；请求-响应类消息必须携带 |

ID 格式与生成规范：

- `event_id`：由 Bot Core 在事件进入 EventBus 时生成，格式为 `evt-{uuid-v4}`（例如 `evt-550e8400-e29b-41d4-a716-446655440000`），全局唯一。同一事件投递给不同插件时共享同一个 `event_id`。
- `request_id`：由消息发起方生成。Runtime 生成 `init`、`event`、`shutdown`、`ping` 消息的 `request_id`；Plugin 生成 `action` 消息的 `request_id`。格式不做强制约束，但推荐使用 `req-{uuid-v4}` 或 `{prefix}_{sequence}` 格式，保证插件会话内唯一即可。
- 响应消息（`result`、`error`、`pong`、`init_ack`）必须携带与对应请求相同的 `request_id`，用于请求-响应配对。
- 日志和调试工具应支持按 `event_id` 和 `request_id` 进行链路追踪。

#### 3.7.3 消息类型

| 类型 | 方向 | 说明 |
| --- | --- | --- |
| `init` | Runtime -> Plugin | 初始化插件上下文 |
| `init_ack` | Plugin -> Runtime | 插件初始化响应，声明就绪状态与事件订阅 |
| `event` | Runtime -> Plugin | 投递统一事件 |
| `init_progress` | Plugin -> Runtime | 初始化阶段的进度续期与阶段摘要 |
| `action` | Plugin -> Runtime | 请求执行动作 |
| `result` | Runtime -> Plugin 或 Plugin -> Runtime | 返回动作处理结果 |
| `error` | 双向 | 返回错误信息或协议错误 |
| `ping` | 双向 | 健康探测 |
| `pong` | 双向 | 健康响应 |
| `shutdown` | Runtime -> Plugin | 通知插件优雅退出 |

约束：

- `shutdown` 消息必须携带 `reason` 字段，枚举值固定为 `stop`、`restart`、`reload`，用于区分普通停止、重启与热重载场景。
- 热重载应复用同一套 `shutdown(reason=reload)` -> 进程退出 -> 新进程 `init` 的协议流程，而不是发明独立热重载私有协议。
- `init_progress` 只允许在收到 `init` 且尚未完成 `init_ack` 的窗口内发送；进入 `running` 后不得继续发送该类型消息。

#### 3.7.4 初始化示例

Runtime → Plugin（`init`）：

```json
{
  "protocol_version": "1",
  "type": "init",
  "timestamp": 1710000000,
  "plugin_id": "weather",
  "request_id": "init_0001",
  "bot": {
    "id": "10001",
    "nickname": "RayleaBot"
  },
  "capabilities": [
    "message.send",
    "event.subscribe"
  ]
}
```

Plugin → Runtime（`init_ack`）：

```json
{
  "protocol_version": "1",
  "type": "init_ack",
  "timestamp": 1710000000,
  "plugin_id": "weather",
  "request_id": "init_0001",
  "status": "ready",
  "subscriptions": [
    "message.group",
    "message.private"
  ]
}
```

Plugin → Runtime（`init_progress`）：

```json
{
  "protocol_version": "1",
  "type": "init_progress",
  "timestamp": 1710000005,
  "plugin_id": "weather",
  "request_id": "init_0001",
  "summary": "loading local model..."
}
```

初始化协议说明：

- `init` 消息中的 `bot` 字段包含机器人自身标识信息，`bot.id` 为机器人 QQ 号，`bot.nickname` 为机器人昵称。来源为 Adapter 连接成功后通过 `get_login_info` 获取的信息。插件可用于过滤自身发送的消息、识别 `@bot` 事件等。
- 插件收到 `init` 后，必须在静默超时窗口内（默认 `runtime.plugin_init_timeout_seconds = 30`）回复 `init_ack`，或至少发送一次 `init_progress` 证明初始化仍在推进。
- 插件若需要执行合法的长耗时初始化步骤，可在 `init_ack` 之前周期性发送 `init_progress`。Runtime 收到 `init_progress` 后，应刷新本轮初始化空闲超时计时器，并把 `summary` 透传到任务流、日志或诊断信息，便于管理员判断当前卡在哪个阶段。
- `init_progress` 只延长“静默超时”窗口，不应允许无限续命。v0.1 应额外设置初始化总时长上限 `runtime.plugin_init_max_total_seconds = 300`；超过该上限后，即使持续收到 `init_progress`，仍应判定为 `plugin.init_timeout`。
- `init_ack.status` 为 `ready` 表示插件初始化成功，可以开始接收事件。
- `init_ack.subscriptions` 声明插件实际订阅的事件类型列表，Runtime 据此做定向投递优化。
- 若插件初始化失败，应回复 `init_ack` 并将 `status` 设为 `error`，附带 `error_message` 字段。
- 若静默超时窗口内既未收到 `init_ack` 也未收到 `init_progress`，或超过初始化总时长上限后仍未完成 `init_ack`，Runtime 将插件标记为 `crashed`（错误码 `plugin.init_timeout`），进入退避重试流程。
- 初始化失败的状态流转：`starting` → `crashed` → `backoff`（重试）→ 达到阈值后 `dead_letter`。

#### 3.7.5 事件投递示例

```json
{
  "protocol_version": "1",
  "type": "event",
  "timestamp": 1710000001,
  "plugin_id": "weather",
  "request_id": "evt_0001",
  "event": {
    "event_id": "evt_0001",
    "source_protocol": "onebot11",
    "source_adapter": "adapter.onebot11",
    "event_type": "message.group",
    "actor": {
      "id": "123",
      "nickname": "小明",
      "role": "member"
    },
    "target": {
      "type": "group",
      "id": "456",
      "name": "测试群"
    },
    "payload": {
      "command": "weather",
      "args": [
        "上海"
      ]
    },
    "message": {
      "segments": [
        { "type": "text", "data": { "text": "天气 上海" } }
      ],
      "plain_text": "天气 上海"
    },
    "timestamp": 1710000001
  }
}
```

说明：

- 默认投递给插件的统一事件不包含 `raw_payload`，以减少 IPC 序列化开销和无意义内存占用。
- 仅当插件声明并获授 `event.raw_payload` 后，Runtime 才可在受控模式下额外附加原始协议载荷。
- `payload.command` 和 `payload.args` 由 Bot Core 的命令解析器在事件投递前填充（详见 3.4.5 命令解析与路由机制）。当消息匹配已配置的命令前缀时，解析器提取命令名和参数列表写入 `payload`；非命令消息的 `payload.command` 为 `null`。

#### 3.7.6 动作请求示例

发送纯文本消息：

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000002,
  "plugin_id": "weather",
  "request_id": "req_0001",
  "action": "message.send",
  "data": {
    "target_type": "group",
    "target_id": "456",
    "message": {
      "segments": [
        { "type": "text", "data": { "text": "上海今日晴，气温 15-22°C" } }
      ]
    }
  }
}
```

发送图片消息（引用渲染结果）：

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000003,
  "plugin_id": "weather",
  "request_id": "req_0002",
  "action": "message.send",
  "data": {
    "target_type": "group",
    "target_id": "456",
    "message": {
      "segments": [
        { "type": "image", "data": { "file": "file://cache/render/weather-001.png" } }
      ]
    }
  }
}
```

发送混合内容消息（文本 + 图片 + @）：

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000004,
  "plugin_id": "weather",
  "request_id": "req_0003",
  "action": "message.send",
  "data": {
    "target_type": "group",
    "target_id": "456",
    "message": {
      "segments": [
        { "type": "at", "data": { "user_id": "123" } },
        { "type": "text", "data": { "text": " 你查询的天气如下：" } },
        { "type": "image", "data": { "file": "file://cache/render/weather-001.png" } }
      ]
    }
  }
}
```

回复消息（`message.reply`）：

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000005,
  "plugin_id": "weather",
  "request_id": "req_0004",
  "action": "message.reply",
  "data": {
    "reply_to_event_id": "evt_0001",
    "fallback_to_send_if_missing": true,
    "message": {
      "segments": [
        { "type": "text", "data": { "text": "上海今日晴，气温 15-22°C" } }
      ]
    }
  }
}
```

说明：

- `message.send` 需要显式指定 `target_type` 和 `target_id`，适用于主动发送消息。
- `message.reply` 通过 `reply_to_event_id` 引用原始事件，平台自动解析回复目标和引用关系；Adapter 负责将其转换为平台侧的引用回复格式。
- 若插件把 `fallback_to_send_if_missing` 设为 `true`，则当适配器返回 `adapter.reply_target_missing` 时，平台可自动退化为同目标的普通 `message.send` 一次，避免高价值消息因为引用失败而整体丢失。
- 发送消息的段模型与接收消息一致，均使用 `segments` 数组表示。
- 图片引用支持 `file://` 本地路径（通常来自 `render.image` 结果）和 `base64://` 编码两种方式。
- 受限于 `stdin/stdout` JSONL IPC 管道的消息体大小与内存压力，官方 SDK 在发送图片时应优先通过 `storage.file` 或受控缓存目录落盘，再传递 `file://` 路径；`base64://` 仅适用于体积较小且明确受控的场景。
- v0.1 应把 `base64://` 视为“小对象兜底路径”而不是常规传输方式。对于原始二进制体积超过 `1 MB` 的图片，官方 SDK 必须在底层自动把内容落到受控临时目录（如 `cache/plugins/<plugin_id>/ipc/` 或等价插件私有临时目录），再透明改写为 `file://` 引用发给 Core，避免大字符串 JSON 序列化 / 反序列化导致的瞬时内存尖峰。
- 在 `message.send`、`message.reply` 与 `fallback_text` 的最终发送路径上，Bot Core 必须对所有文本承载字段做一次核心层脱敏；命中当前有效凭据值的文本在交给 Adapter 前统一替换为 `***`，不允许把原始密钥直接下发到聊天平台。

#### 3.7.7 渲染动作示例

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000002,
  "plugin_id": "help_menu",
  "request_id": "req_render_0001",
  "action": "render.image",
  "data": {
    "template": "help.menu",
    "theme": "default",
    "output": "png",
    "fallback_text": "帮助菜单暂时不可用，请稍后重试。",
    "data": {
      "title": "帮助菜单",
      "items": [
        { "name": "help", "description": "显示帮助菜单", "usage": "/help" },
        { "name": "status", "description": "查看机器人状态", "usage": "/status" },
        { "name": "rank", "description": "查看排行榜", "usage": "/rank" }
      ]
    }
  }
}
```

#### 3.7.8 结果与错误示例

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000003,
  "plugin_id": "weather",
  "request_id": "req_0001",
  "status": "success",
  "data": {
    "message_id": "msg_1001"
  }
}
```

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000004,
  "plugin_id": "help_menu",
  "request_id": "req_render_0001",
  "status": "success",
  "data": {
    "image_path": "cache/render/help-menu-001.png",
    "mime": "image/png",
    "cache_key": "render:help.menu:abc123"
  }
}
```

```json
{
  "protocol_version": "1",
  "type": "error",
  "timestamp": 1710000004,
  "plugin_id": "help_menu",
  "request_id": "req_render_0001",
  "code": "platform.render_timeout",
  "message": "render job exceeded timeout"
}
```

协议要求：

- 插件必须能显式返回 `plugin.not_handled`、`permission.denied`、`plugin.internal_error`、`platform.render_timeout` 等失败语义，错误码格式须与 3.11.3 统一错误码目录一致。
- `request_id` 必须贯穿请求与响应，便于日志、调试和超时处理。
- `ping` / `pong` 用于健康检查和卡死探测，而不是业务事件处理。
- 图片渲染和消息发送应拆分为两个动作；渲染结果返回文件路径或资源引用，不直接隐式发送消息。
- 当 `render.image` 请求包含 `fallback_text` 且存在当前消息上下文时，Bot Core 可在渲染失败时自动发送该文本作为降级消息，并在结果中标记 `fallback_sent`。
- 平台自动发送 `fallback_text` 时，必须把它包装成单一 `text` 段并走统一消息段发送路径，不允许把 `fallback_text` 当作原始 CQ 码字符串或未转义的适配器私有格式直接下发。
- 若 `fallback_text` 的自动发送也失败（如 `platform.rate_limited`、Adapter 断连或 `adapter.send_failed`），Core 不得在内部进入无限重试、补发或额外排队；应直接把原始渲染失败结果连同 `fallback_attempted: true`、`fallback_sent: false` 和底层 `fallback_error` 一并返回给插件，由插件决定后续容错策略。
- 未来如需破坏性调整，优先升级 `protocol_version`、`manifest_version` 或 `plugin_protocol_version`，而不是在旧结构上打补丁。

#### 3.7.9 补充协议示例

**`scheduler.create`（创建定时任务）**：

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000020,
  "plugin_id": "daily_report",
  "request_id": "req_sched_0001",
  "action": "scheduler.create",
  "data": {
    "task_id": "daily_morning_report",
    "cron": "0 8 * * *",
    "event_type": "scheduler.trigger",
    "payload": { "report_type": "daily" }
  }
}
```

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000020,
  "plugin_id": "daily_report",
  "request_id": "req_sched_0001",
  "status": "success",
  "data": {
    "task_id": "daily_morning_report",
    "next_run": "2024-03-11T08:00:00+08:00"
  }
}
```

**`config.read`（读取配置）**：

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000030,
  "plugin_id": "weather",
  "request_id": "req_config_0001",
  "action": "config.read",
  "data": {
    "keys": ["api_key_name", "default_city", "unit"]
  }
}
```

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000030,
  "plugin_id": "weather",
  "request_id": "req_config_0001",
  "status": "success",
  "data": {
    "values": {
      "api_key_name": "weather_api",
      "default_city": "北京",
      "unit": "celsius"
    }
  }
}
```

**`http.request`（发起 HTTP 请求）**：

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000040,
  "plugin_id": "weather",
  "request_id": "req_http_0001",
  "action": "http.request",
  "data": {
    "method": "GET",
    "url": "https://api.weather.example/v1/current?city=上海",
    "headers": { "Accept": "application/json" },
    "timeout_seconds": 10
  }
}
```

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000041,
  "plugin_id": "weather",
  "request_id": "req_http_0001",
  "status": "success",
  "data": {
    "status_code": 200,
    "headers": { "Content-Type": "application/json" },
    "body": "{\"temp\":22,\"weather\":\"晴\"}"
  }
}
```

**`event.expose_webhook`（注册受控 Webhook 路由）**：

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000045,
  "plugin_id": "repo_watcher",
  "request_id": "req_webhook_0001",
  "action": "event.expose_webhook",
  "data": {
    "route": "github",
    "methods": ["POST"],
    "auth_strategy": "hmac_sha256",
    "signature_header": "X-Hub-Signature-256",
    "signature_prefix": "sha256=",
    "secret_ref": "webhook.github.secret"
  }
}
```

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000045,
  "plugin_id": "repo_watcher",
  "request_id": "req_webhook_0001",
  "status": "success",
  "data": {
    "route": "github",
    "url": "/api/webhooks/repo_watcher/github"
  }
}
```

**`storage.kv`（KV 存储 set / get）**：

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000050,
  "plugin_id": "weather",
  "request_id": "req_kv_0001",
  "action": "storage.kv",
  "data": {
    "operation": "set",
    "key": "user:123:default_city",
    "value": "上海"
  }
}
```

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000050,
  "plugin_id": "weather",
  "request_id": "req_kv_0001",
  "status": "success",
  "data": {}
}
```

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000051,
  "plugin_id": "weather",
  "request_id": "req_kv_0002",
  "action": "storage.kv",
  "data": {
    "operation": "get",
    "key": "user:123:default_city"
  }
}
```

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000051,
  "plugin_id": "weather",
  "request_id": "req_kv_0002",
  "status": "success",
  "data": {
    "key": "user:123:default_city",
    "value": "上海",
    "exists": true
  }
}
```

#### 3.7.10 Shutdown 协议交互示例

Runtime → Plugin（`shutdown`）：

```json
{
  "protocol_version": "1",
  "type": "shutdown",
  "timestamp": 1710000100,
  "plugin_id": "weather",
  "request_id": "shutdown_0001",
  "reason": "stop"
}
```

- `reason` 字段枚举值：`stop`（普通停止）、`restart`（服务重启）、`reload`（热重载，见 3.5.9）。

Plugin → Runtime（可选 `shutdown_ack`）：

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000101,
  "plugin_id": "weather",
  "request_id": "shutdown_0001",
  "status": "success",
  "data": {
    "cleanup_completed": true
  }
}
```

协议说明：

- Runtime 发送 `shutdown` 后进入优雅退出等待窗口，默认 `runtime.shutdown_grace_seconds = 10`。
- 插件收到 `shutdown` 后应完成资源清理（关闭文件句柄、刷写缓存、取消未完成任务等），然后退出进程或回复 `result` 确认清理完成。
- Runtime 向插件发送 `shutdown` 后，该插件会话的 IPC 通道必须立即进入“半关闭”状态：Core 仍可向该插件返回先前已接受请求的 `result` / `error`，但必须拒绝任何新的 `action` 请求，并以结构化错误 `plugin.stopping` 立即响应，防止插件在退出窗口内继续开启新的 HTTP、渲染或消息发送工作。
- 插件回复 `result`（`shutdown_ack`）是可选的：若插件在 grace window 内正常退出进程，Runtime 视为优雅退出成功。
- 若 grace window 超时且插件进程仍未退出，Runtime 执行强制终止（`SIGKILL` 或等效操作），并将插件状态标记为 `crashed`（错误码 `plugin.shutdown_timeout`）。
- 热重载场景（`reason: "reload"`）复用同一套流程：`shutdown(reason=reload)` → 进程退出 → 新进程 `init` → `init_ack`（见 3.5.9）。
- 服务停机（`reason: "stop"`）时，Runtime 并发向所有运行中的插件发送 `shutdown`，共享同一个 grace window。
- 官方 SDK 在收到 `shutdown` 后也应主动停止新的定时器回调、后台协程或后续 Action 尝试，把注意力收敛到清理与退出，而不是把 `plugin.stopping` 当作可忽略噪声继续刷请求。

### 3.8 图片渲染引擎

#### 3.8.1 定位与目标

- 图片渲染服务应作为平台级基础能力，由平台统一提供菜单、状态图、排行榜、信息卡片等图片生成能力。
- 插件默认只传模板名、数据和输出选项，不直接各自实现浏览器截图、Canvas 布局或 HTML 自绘逻辑。
- 首期目标是统一风格、统一调用方式、统一性能与缓存策略，避免插件生态在视觉和实现层面失控。

#### 3.8.2 渲染分层

推荐按以下分层组织：

```plain
Plugin
  │
  ▼
Render API
  │
  ▼
Render Service
  ├─ Template Manager
  ├─ Asset Manager
  ├─ Cache Manager
  └─ Render Engine
```

建议目录：

```plain
server/internal/render
├─ api
├─ engine
│  ├─ browser
│  └─ cache
├─ templates
│  ├─ help-menu
│  ├─ status-panel
│  ├─ info-card
│  ├─ rank-list
│  └─ notice-card
├─ assets
│  ├─ fonts
│  ├─ icons
│  └─ themes
└─ schema
```

#### 3.8.3 v0.1 技术路线

- v0.1 主路线采用 HTML/CSS 模板渲染 + Chromium 离屏截图输出 PNG。
- 首期优先保证模板表现力和开发效率，而不是过早自建 Canvas 布局系统。
- v0.1 图片渲染固定采用 Chromium + CDP 路线，Go 侧默认实现选用 `chromedp`；如后续更换实现库，需同步修订规划文档与构建方案。
- 浏览器引擎应组织在 `engine/browser`，由平台内部封装 `chromedp` 调用细节，避免业务层和插件侧直接耦合具体库。
- v0.1 默认采用常驻 Chromium Worker 池，默认 `worker_count = 1`，以保持低配宿主机上的串行稳定性。
- 在受控配置下可提升 `render.worker_count`，让高配多核宿主机并行处理渲染任务；默认值仍应保守为 `1`。
- v0.1 允许复用单页或少量固定页面上下文，但不引入复杂页面池调度和跨 worker 的高级负载均衡。
- `config/user.yaml` 应暴露 `render.worker_count` 与 `render.browser_args` 等配置项；默认浏览器参数应包含 `--disable-gpu`，高配宿主机仅在明确验证通过后才按需附加 `--enable-gpu` 等 Chromium 启动参数。
- 当用户主动提高 `render.worker_count` 或启用 `--enable-gpu` 等硬件加速参数时，部署文档必须同步提示宿主机或虚拟化环境需要准备对应的显卡/图形设备访问能力。
- 对 PVE LXC、容器或其他轻量虚拟化场景，如未完成 GPU 相关设备节点映射、驱动注入或等价 passthrough 准备，则不应启用 GPU 加速参数；否则 Chromium 可能在启动阶段直接崩溃。
- v0.1 不做多浏览器兼容，不依赖用户系统已安装的 Chrome / Edge；图片渲染运行环境由发行包统一提供，或由平台托管依赖目录按需准备。
- Go 在启动 Chromium 子进程时，必须绑定父子进程生命周期，例如 Linux 下通过 `SysProcAttr` 配置 `Pdeathsig=SIGKILL`、Windows 下绑定 Job Object，确保 `raylea-server` 意外退出时底层浏览器实例会被操作系统自动回收，避免形成长期驻留的僵尸进程。
- 浏览器实例复用、页面池、Render Worker 并发调度和更复杂的防泄漏治理，放到后续高频场景阶段再做。
- Web 前端的设计 token、字体、图标体系可以与渲染模板共用，但渲染服务不应直接依赖 Web 页面截图作为主方案。

#### 3.8.4 模板与资源体系

模板体系建议分两类：

- 官方模板：平台内置模板，插件只传数据，适合作为 v0.1 主体方案。
- 自定义模板：由高级插件自带模板目录并声明使用，建议放到 v0.2 之后逐步开放。

平台应统一管理以下资源：

- 默认字体与字号体系。
- 默认色板与主题 token。
- 图标、背景、插画和默认占位图。
- 模板版本与 schema。
- 官方模板依赖的字体文件应随发行包提供，并通过 CSS `@font-face` 从 `assets/fonts/` 或等价受控目录显式加载，而不是依赖宿主机系统字体库。
- 官方模板应显式维护 `template_id`、`template_version` 与输入 schema 版本；向后兼容的字段扩展可复用原模板版本，破坏性字段变化必须提升模板版本或提供兼容适配层。
- 当插件请求的模板版本或输入 schema 与当前模板实现不兼容时，Render Service 应返回结构化错误码，而不是静默兜底为错误图片。

**v0.1 模板文件格式约束**：

- 官方模板固定采用 `HTML + CSS + JSON 数据上下文` 方案，不引入第二套与 Web UI 无关的专用 DSL。
- 官方模板目录建议至少包含：`template.html`、`template.css`、`template.meta.json` 和可选的 `assets/` 子目录；其中 `template.meta.json` 应记录 `template_id`、`template_version`、`input_schema_ref`、`theme_slots` 与缓存键参与字段。
- 渲染时的数据注入固定为单一根对象：`data` 为业务数据、`theme` 为主题变量、`meta` 为平台注入的只读上下文（如 `rendered_at`、`template_version`）。模板不得依赖插件自定义的全局变量命名。
- 服务端模板绑定固定采用一套安全模板引擎，优先选择 Go `html/template` 或等价具备自动转义能力的实现；v0.1 不同时引入 Mustache、Jinja、Nunjucks 等多套模板语法。
- v0.1 官方模板默认不执行模板自带 JavaScript，不允许插件把任意脚本注入渲染页面；需要布局计算时，应优先通过预定义 CSS 布局、受控辅助样式或平台内置脚本完成。
- 主题系统固定通过 CSS Variables 组织，例如在 `:root[data-theme="default"]` 或等价节点下声明 `--rl-color-primary`、`--rl-font-body`、`--rl-radius-card` 等变量；模板只消费 token，不直接写死平台级颜色和字体常量。
- 模板输入 schema 必须与 `template.meta.json` 中的 `input_schema_ref` 对齐；渲染前先做 schema 校验，不合格时直接返回结构化错误，而不是交给 Chromium 运行期报错。

**模板版本兼容语义**：

- 模板输入 schema 的变更分为两类：
  - **向后兼容扩展**（新增可选字段、扩展枚举值）：不提升 `template_version`，旧插件调用仍可正常渲染。
  - **破坏性变更**（删除必填字段、修改字段类型、重构 schema 结构）：必须提升 `template_version`。
- 当插件请求的 `template_version` 低于当前模板版本时：
  - 若当前版本仍向后兼容该请求版本，正常渲染。
  - 若当前版本与请求版本不兼容，返回 `platform.template_version_mismatch` 错误码，附带当前支持的版本范围。
- v0.1 内置模板数量有限，版本兼容问题不突出；但平台应从首版就在 Render Service 中实现版本检查逻辑，为后续模板演进建立基础。

目标是确保插件产出的图片在字体、配色、圆角、阴影、间距和层级上保持统一产品风格。

#### 3.8.5 插件调用模型

- 需要图片渲染能力的插件应在 `capabilities` 中声明 `render.image`。
- 插件统一通过 `render.image` 动作调用平台渲染服务，而不是直接操作底层浏览器实例。
- 推荐输入结构包含 `template`、`data`、`theme`、`output`，以及可选的 `fallback_text`。
- 推荐输出结构包含 `image_path`、`mime`、`cache_key` 等字段。
- 图片渲染和消息发送必须拆分，插件可在拿到结果后自行决定发送、存档、预览或复用。
- 当渲染队列已满或渲染失败返回结构化错误时，插件应能感知错误码，并按需使用 `fallback_text` 或其他降级路径完成消息输出。
- 官方 SDK 应提供默认容错辅助：当插件传入 `fallback_text` 且当前存在消息上下文时，可直接复用平台降级逻辑自动发送文本回复，尽量避免每个插件作者都手写一套 `try/catch`。

**v0.1 渲染输入边界**：

- `render.image.data` 在序列化后的 JSON 大小默认不得超过 `256 KB`；超出时应在进入 Render Queue 前直接返回 `platform.render_input_too_large`。
- 单次渲染请求最多引用 `16` 个图片资源或等价外部资产句柄；超过上限时直接拒绝，不等待 Chromium 执行阶段报错。
- 单个模板中的重复列表项、排行榜条目或卡片行数默认上限为 `200`；超过上限时由平台或 SDK 在调用前裁剪或返回结构化错误，防止长列表把模板布局和截图链路拖垮。
- 官方模板默认不允许在渲染阶段直接联网抓取任意远程资源；如插件需要远程图片、头像或附件，应先经 `http.request` 下载到受控缓存或插件私有目录，再通过本地 `file://` 资源交给模板消费。
- 所有输入边界必须在 `contracts/plugin-protocol.schema.json`、模板 `input_schema_ref` 与 SDK 参数校验中保持一致，不允许 Runtime、Render Service 和 SDK 各写一套不同阈值。

##### 渲染到发送的完整链路协议示例

以下展示一个插件从调用 `render.image` 到最终发送图片消息的完整三步交互：

**第一步：插件发送 `render.image` 动作**

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000010,
  "plugin_id": "help_menu",
  "request_id": "req_render_0010",
  "action": "render.image",
  "data": {
    "template": "help.menu",
    "theme": "default",
    "output": "png",
    "fallback_text": "帮助菜单暂时不可用，请稍后重试。",
    "data": {
      "title": "帮助菜单",
      "items": [
        { "name": "weather", "description": "查询天气", "usage": "/weather <城市>" },
        { "name": "status", "description": "查看状态", "usage": "/status" }
      ]
    }
  }
}
```

**第二步：平台返回渲染结果**

```json
{
  "protocol_version": "1",
  "type": "result",
  "timestamp": 1710000011,
  "plugin_id": "help_menu",
  "request_id": "req_render_0010",
  "status": "success",
  "data": {
    "image_path": "cache/render/help-menu-abc123.png",
    "mime": "image/png",
    "cache_key": "render:help.menu:abc123"
  }
}
```

**第三步：插件用渲染结果构造 `message.send` 发送图片消息**

```json
{
  "protocol_version": "1",
  "type": "action",
  "timestamp": 1710000012,
  "plugin_id": "help_menu",
  "request_id": "req_send_0010",
  "action": "message.reply",
  "data": {
    "reply_to_event_id": "evt_0001",
    "message": {
      "segments": [
        { "type": "image", "data": { "file": "file://cache/render/help-menu-abc123.png" } }
      ]
    }
  }
}
```

说明：

- 渲染与发送始终是两个独立动作，插件在拿到渲染结果后可自行决定是发送、预览、存档还是丢弃。
- 若第一步渲染失败且包含 `fallback_text`，Bot Core 可在当前消息上下文下自动发送降级文本，并在结果中标记 `fallback_sent: true`。
- 自动降级文本必须复用统一消息段构造与 Adapter 发送链路，作为纯文本段发送，不允许绕过段模型直接拼接 CQ 码或适配器私有文本协议。
- 若自动降级文本也发送失败，平台必须停止在该层继续重试，并把 `fallback_attempted`、`fallback_sent: false` 与底层发送错误一起返回；是否继续补发、换目标发送或彻底放弃，由插件或上层业务自行决定。
- 插件通过 SDK 调用时，上述三步通常被封装为一次 `ctx.render_and_reply()` 或类似的高级辅助方法。

#### 3.8.6 缓存、性能与失败降级

平台统一控制以下运行时行为：

- 渲染超时。
- 渲染 Worker 数量、队列长度与后续可扩展的并发上限。
- 模板资源缓存。
- 渲染结果缓存。
- 临时文件 TTL 清理。

建议缓存路径：

```plain
cache/render/
```

缓存策略建议至少包含：

- 模板资源缓存：字体、图标、CSS、背景资源。
- 渲染结果缓存：相同模板 + 相同数据 hash 命中后复用。
- 临时文件缓存：按 TTL 自动清理，避免长期堆积。
- 渲染缓存键至少应包含 `template_id`、`template_version`、`theme_version`、`data_hash` 和 `render_engine_version`，避免模板、主题或引擎升级后命中脏缓存。

当渲染失败时，平台应返回明确错误，而不是卡住插件进程；是否降级为文本消息或默认提示图由插件或上层调用者自行决定。

补充约束：

- 当平台内置字体、图标、模板或主题资源缺失时，Render Service 必须返回结构化平台错误码，而不是静默输出不完整图片。
- Render Service 必须使用有界队列；当渲染队列已满时，应立即返回结构化错误码，例如 `platform.render_queue_full`，而不是无限排队阻塞调用方。
- 插件或上层调用者收到 `platform.render_queue_full` 后，可自行决定是否走 `fallback_text` 或其他文本降级路径。
- Render Service 必须区分“排队等待超时”和“实际执行超时”。v0.1 默认建议 `render.queue_wait_timeout_seconds = 15`：若任务在队列中滞留超过该阈值，则在被 Worker 取出时直接丢弃并返回 `platform.render_timeout`，不再把过期任务送入 Chromium 执行。
- `render.queue_wait_timeout_seconds` 不应低于单 Worker 常规模板渲染耗时的 `p95` 量级；若部署保持 `render.worker_count = 1` 且模板平均耗时较高，应优先同步上调 `worker_count` 或排队等待超时，而不是把默认值压到几秒内导致正常请求系统性饿死。
- `render.timeout_seconds` 仅用于约束任务进入 Chromium Worker 之后的实际执行时长，不应把排队等待时间与浏览器执行时间混为一个超时预算。
- 服务启动预检以及 Web UI / Launcher 的环境检查不应只检查 Chromium 是否存在，还应覆盖内置渲染资源完整性。
- 平台模板资源错误应和插件业务错误区分记录，便于定位到底是模板资源损坏还是插件调用数据不合法。

**渲染 Worker 强制回收机制**：

- 单个 Chromium 页面上下文（Tab / Context）在长期运行中会因 V8 内存碎片累积产生不可完全回收的泄漏。Render Service 必须实现强制回收策略，防止慢性内存泄漏最终导致宿主机 OOM。
- 回收触发条件（满足任一即触发）：
  - 单个 Context 累计成功渲染次数达到 `render.context_recycle_count`（默认 `1000`）。
  - 单个 Context 连续发生 `3` 次渲染超时。
  - 单个 Context 的 RSS 内存占用超过 `render.context_max_memory_mb`（默认 `512 MB`，需 Chromium CDP 的 `Runtime.getHeapUsage` 或进程级内存监控支持）。
- 回收流程：标记当前 Context 为待销毁 → 等待当前渲染任务完成（或超时强制终止）→ 销毁 Context → 拉起新页面实例 → 恢复接受新任务。
- 回收过程对插件透明，Render Service 在回收窗口内排队的新任务等待新 Context 就绪后继续处理。
- `render.context_recycle_count` 和 `render.context_max_memory_mb` 应暴露到 `config/user.yaml` 中，允许高配宿主机适当放宽阈值。

#### 3.8.7 v0.1 内置模板规划

v0.1 建议优先内置以下模板：

- `help.menu`
- `status.panel`
- `info.card`
- `rank.list`
- `notice.card`

这些模板已经能覆盖帮助菜单、状态面板、通用卡片、排行榜和错误提示等高频场景。

各内置模板最小输入 Schema 参考：

- `help.menu`：
  ```json
  { "title": "帮助菜单", "items": [{ "name": "weather", "description": "查询天气", "usage": "/weather <城市>" }] }
  ```
- `status.panel`：
  ```json
  { "bot_name": "RayleaBot", "uptime": "3d 2h", "plugin_count": 5, "connection_status": "connected" }
  ```
- `info.card`：
  ```json
  { "title": "用户信息", "fields": [{ "label": "昵称", "value": "Raylea" }], "footer": "查询时间: 2024-01-01" }
  ```
- `rank.list`：
  ```json
  { "title": "活跃排行", "entries": [{ "rank": 1, "name": "用户A", "score": 1200 }] }
  ```
- `notice.card`：
  ```json
  { "level": "warning", "title": "维护通知", "message": "服务将于今晚 22:00 进行升级维护" }
  ```

说明：

- 以上为最小必填 Schema，各模板可根据实际设计扩展可选字段（如 `icon`、`color`、`max_items` 等）。
- 模板 Schema 应在 `server/internal/render/schema/` 下以 JSON Schema 文件形式维护，Render Service 在渲染前校验输入数据合法性。

#### 3.8.8 模板开发与实时预览

- v0.1 不要求提供完整在线模板编辑器，但应预留单模板调试入口或预览 API，确保模板开发不需要重启整套服务链路。
- v0.2 建议在 Web UI 中提供模板实时预览（Live Preview）工具：左侧编辑 HTML/CSS 模板与注入 JSON 数据，右侧展示同一条 Chromium 渲染链路生成的完整图片。
- 实时预览必须复用正式渲染引擎、模板资源和主题 token，而不是额外维护一套浏览器截图实现。
- 预览工具仅面向开发模式或受控调试入口，不默认暴露给普通管理用户。

### 3.9 Web 管理面板、Launcher 与安全边界

#### 3.9.1 Web UI 范围

v0.1 Web UI 聚焦以下能力：

- 首次初始化向导。
- 查看系统运行状态。
- 查看插件列表与启用状态。
- 启用、禁用、安装、卸载插件。
- 查看核心日志和插件日志。
- 查看插件 stdout / stderr 的实时调试控制台（v0.1 优先保证受控、可脱敏、仅管理员可访问的基础 console 能力；更完整的调试体验如历史回放、高级过滤、多插件聚合视图归入 v0.2）。
- 查看和修改基础配置。
- 管理唯一管理员账户登录状态。

补充规划：

- v0.1 不把完整在线模板编辑器纳入必做范围；模板实时预览（Live Preview）工具作为开发者调试能力优先在 v0.2 落地，并复用正式渲染引擎与 3.8.8 定义的同一链路。
- v0.2 应增加调试聊天面板（Debug Chat），允许管理员在 Web UI 中模拟发送消息（含命令、纯文本和各类事件类型）并查看机器人响应，无需连接真实 QQ / OneBot11 实例。调试聊天面板应复用统一事件模型（见 3.2），通过内置虚拟 "debug" 适配器源注入模拟事件，而非构建独立的聊天实现。该面板面向开发、测试和演示场景，不替代生产协议连接。

Web UI 前端服务策略：

- v0.1 建议 Go 服务内嵌 Web 静态文件（`go:embed`），发行包运行时无需额外 Web 服务器。
- 开发态使用 Vite 开发服务器 + 反向代理到 Go 后端 API，保持前后端独立热更新。
- SPA 路由建议使用 hash 模式（`createWebHashHistory`），避免 Go 端额外处理 HTML5 History fallback 逻辑。
- Web 构建产物输出到 `web/dist/`，由 Go 的 `go:embed` 在编译时嵌入。
- 开发态的 Vite 配置应将 `/api/*` 和 `/ws/*` 代理到 Go 后端地址。
- 插件管理页应对命令冲突做显式检测：当多个启用插件声明了同名命令时，应高亮提示管理员当前采用 fan-out 语义，必要时引导禁用冲突插件。
- 插件管理页应按插件的 `role`（见 3.6.2）和安装来源元数据展示信任等级标识：`builtin` 插件显示"官方"徽标；`user`（第三方安装）插件显示"第三方"标签，未经签名验证的本地 zip 安装源额外显示"未验证来源"提示；`dev` 插件显示"开发中"标签。
- 信任等级标识应通过颜色、图标或标签在视觉上清晰区分，使管理员可在插件列表页一眼识别插件信任级别。
- v0.3 的插件签名校验（见 v0.3 路线图）将提供更强的信任信号；在此之前，来源标识是主要的风险提示机制。

#### 3.9.2 本地优先安全模型

- 默认只监听 `127.0.0.1`。
- 默认只开放本机访问，远程访问必须由用户显式开启。
- Web API 和 WebSocket 都必须鉴权。
- Launcher 打开 Web UI 时，统一打开本机管理入口；若服务已完成初始化，可最佳努力附带一次性 Token 帮助自动进入管理页面。
- 即使启用远程访问，也不默认开放无鉴权调试接口。
- 第三方插件默认不视为可信管理主体，必须通过最小权限模型和 Runtime 沙盒边界访问平台能力。

Web 安全策略补充：

- API 接口依赖 Token 鉴权作为基础防护；所有状态变更类 API 必须携带有效管理会话 Token，不依赖 Cookie 或 Referer 检查作为唯一 CSRF 防线。
- 登录接口添加失败次数限制，默认 5 次失败 / 5 分钟后临时锁定该来源 IP，防止暴力破解。
- 服务应设置基础 `Content-Security-Policy` 响应头，至少限制 `script-src` 为 `'self'`，防止内嵌脚本注入。
- 远程访问开启时，文档应明确建议用户配合 HTTPS 反向代理（如 Nginx、Caddy）使用，避免管理 Token 在网络中明文传输。
- v0.1 不强制要求内置 TLS 证书管理，但应在配置和文档中为反向代理 + HTTPS 的部署方式提供指引。

管理面暴露等级：

| 暴露等级 | 典型场景 | 默认监听 | 初始化接口 | 风险提示 |
| --- | --- | --- | --- | --- |
| `localhost_only` | 默认开发机 / 本机部署 | `127.0.0.1` | 允许，仅本机 | 默认模式，无额外提示 |
| `lan_enabled` | 家庭内网或可信局域网 | 用户显式改为内网地址 | 默认禁止远程初始化 | 必须提示“已暴露到局域网” |
| `public_via_reverse_proxy` | 反向代理后公网访问 | 建议仍监听本地地址，由代理转发 | 禁止 | 必须提示 HTTPS、反向代理与额外暴露风险 |

#### 3.9.3 首次初始化与管理员引导

- 首次启动若不存在管理员账户，系统进入 `setup_required` 引导模式；该模式用于初始化，并已纳入 3.3 中的正式服务状态枚举。
- `setup_required` 阶段默认只允许本机访问初始化页面和初始化 API，不对远程访问开放。
- Launcher 只负责打开本机 Web 管理入口；由 Web UI 自己判断显示初始化页还是登录页，本机浏览器也可通过本地地址访问初始化向导。
- 初始化流程必须创建首个管理员凭据，并在成功后建立首个管理会话。
- 初始化完成前，除基础健康检查和初始化接口外，不开放正常插件管理、配置修改和日志查询接口。
- 如管理员凭据丢失、管理会话损坏或无法登录，v0.1 官方恢复路径限定为“停服务后通过本机 Launcher 或本地 CLI 重置命令触发重置向导”，清理现有管理会话并重新进入 `setup_required`。
- 管理员恢复 / 重置流程不应默认清空用户配置、已安装插件和插件业务数据；是否重建管理员凭据与是否清理业务数据必须分开处理。
- `setup_required` 阶段默认禁用所有插件加载、OneBot 连接和事件处理，仅允许初始化 API 调用，防止未配置完成时暴露服务或执行插件代码。
- 管理员登录会话应持久化到 SQLite 的 `admin_sessions`，默认采用有限 TTL；v0.1 建议普通登录会话默认 7 天有效，重置管理员凭据时必须使旧会话全部失效。
- v0.1 默认采用滑动续期会话：管理员在有效期内持续活跃时可延长 TTL，但单个会话仍应设置绝对有效期上限，避免无限续期。
- v0.1 允许少量并发管理会话，但应受控，例如默认不超过 3 个；超过上限时，平台应拒绝新会话或回收最旧会话，而不是无限叠加。
- Launcher 附带的一次性 Token 只作为本机自动登录增强能力，不等价于长期管理 Session；完成校验后仍应换发标准管理会话。
- Launcher 自动登录失败、令牌过期或 admission 校验未通过时，Web UI 不应白屏、死循环跳转或只返回裸 `401`。应直接回到普通登录页或初始化页，并给出简短可读提示。
- 登录页与初始化页在提交失败时必须给出明确、可见的人类可读错误提示，不得只依赖控制台报错、toast 或无反馈静默失败。

#### 3.9.4 Launcher（3.13）与 Web API 分工

`Launcher` 在 v0.1 只负责以下能力：

- 本地进程启动、停止、重启。
- 本地环境检查。
- 查看启动失败摘要或极简本地尾部日志。
- 打开 Web UI。
- 检查是否存在新版本。

服务启动后，以下能力应优先统一走 Web API / WebSocket：

- 插件管理。
- 完整日志浏览与筛选。
- 实时调试控制台。
- 状态展示。
- 在线配置管理。
- 备份与诊断导出。

这样可以避免形成两套管理逻辑和两套状态源。

刚性规则：

- `Launcher` 不维护独立配置解析逻辑，在线配置以服务端配置模型和 Web API 为唯一管理源。
- `Launcher` 不维护独立状态模型，所有运行状态展示应复用服务端状态接口与事件流。
- `Launcher` 不承担初始化、登录或管理会话恢复流程判断；这些流程由 Web UI 决定与呈现。
- 备份导出、配置编辑、插件管理和完整日志查看都应优先由 Web UI 承载，Launcher 只负责把用户带到该入口。
- Launcher 如需显示日志，仅限启动失败摘要或极简本地尾部日志，不单独发展第二套日志浏览界面。

**管理入口职责矩阵**：

| 场景 | Web UI / API | Launcher | CLI |
| --- | --- | --- | --- |
| 在线状态查看 | 主 | 读代理（复用 Web API） | 否 |
| 在线启停 / 重载插件 | 主 | 仅转发到 Web API | 否 |
| 首次初始化 | 主（初始化页面） | 打开 Web 入口 | 否 |
| 管理员凭据丢失恢复 | 否（服务可能无法登录） | 可触发 CLI | 主（`reset-admin`） |
| 备份导出 | 触发后台任务 | 否 | 主（`raylea backup`） |
| 恢复导入 | 否（需停服） | 否 | 主（`raylea restore`） |
| 环境检查与诊断 | 可用 | 启动前检查 | 主（`raylea doctor`） |
| 配置迁移 | 否（需停服） | 否 | 主（`raylea migrate`） |

说明：

- "主"表示该入口是该场景的首选或唯一正式路径。
- "否"表示该入口不应提供该能力，防止形成多套状态源。
- Launcher 与 CLI 如需触发管理操作，应复用 Web API 或共享后端逻辑，不独立维护状态。

#### 3.9.5 最小权限模型

v0.1 至少应明确以下五类权限主体：

- 本地管理者：本机启动和维护服务的用户，拥有最高本地控制权。
- Web 管理会话：通过登录建立的管理会话，可调用管理 API 和 WebSocket。
- 插件权限：由 manifest 声明并由平台授予的能力与作用域。
- 插件沙盒：运行时对子进程施加的进程边界、工作目录边界与受支持网络边界控制。
- 远程访问开关：决定管理面板是否允许非本机访问的部署级开关。

约束：

- Web 面板默认要求登录。
- 启动器打开 Web 面板时可附带临时 Token 或本地 Session。
- 插件权限与管理权限必须严格隔离，插件不得天然继承管理权限。
- 插件沙盒与权限模型必须协同工作，插件即使声明了能力，也不应绕过 Runtime 的进程与网络边界。
- 远程访问关闭时，即便用户已登录，也不应默认暴露到局域网或公网。

#### 3.9.6 规划中的 Web API

| 接口 | 说明 |
| --- | --- |
| `GET /api/setup/status` | 查询是否处于首次初始化模式 |
| `POST /api/setup/admin` | 首次初始化时创建首个管理员账户 |
| `POST /api/session/login` | 管理员登录 |
| `DELETE /api/session` | 退出登录 |
| `POST /api/session/launcher-token` | Launcher 获取一次性登录 Token，用于本机自动登录 Web UI |
| `POST /api/system/shutdown` | 触发服务内部优雅停机流程；默认仅允许本机回环地址上的受控管理入口调用 |
| `GET /api/system/status` | 获取系统状态、版本、运行时间、协议连接状态 |
| `GET /api/plugins` | 获取插件列表和状态 |
| `POST /api/plugins/install` | 异步安装插件，v0.1 支持本地 zip 包、本地目录和 remote_url 来源，立即返回 `202 Accepted` 与 `task_id` |
| `POST /api/plugins/{id}/enable` | 启用插件 |
| `POST /api/plugins/{id}/disable` | 禁用插件 |
| `POST /api/plugins/{id}/reload` | 触发插件热重载 |
| `DELETE /api/plugins/{id}` | 卸载插件 |
| `GET /api/config` | 读取当前可公开配置 |
| `PUT /api/config` | 更新基础配置 |
| `GET /api/logs` | 查询日志列表或日志片段 |
| `GET /api/tasks/{id}` | 查询长任务状态、进度摘要与结果 |

**受控插件 Webhook 路由（非管理 API）**：

| 路由 | 说明 |
| --- | --- |
| `POST /api/webhooks/{plugin_id}/{route}` | 受控入站 Webhook 入口；仅当目标插件声明并获授 `event.expose_webhook` 后启用，请求将被封装为 `webhook.received` 内部事件定向投递 |

**Rate Limit 格式规范**：

- 配置文件中所有速率限制字段统一采用 `"<count>/<duration>"` 格式。
- `<count>`：正整数，表示允许的操作次数。
- `<duration>`：正整数 + 时间单位，如 `s`（秒）、`m`（分钟）、`h`（小时）。
- 有效示例：`"10/60s"`（60秒10次）、`"200/10s"`（10秒200次）、`"5/1m"`（1分钟5次）。
- 无效示例：`"10"`（缺少持续时间）、`"/60s"`（缺少计数）、`"10/"`（缺少持续时间）。
- 解析器应拒绝格式错误配置，并在启动时给出明确错误提示。
- 默认语义为：`<count>` 同时表示该时间窗口内的总配额和允许的最大瞬时突发量（`burst = count`）。例如 `"10/60s"` 的默认含义是“60 秒内最多 10 次，且允许在窗口起始时瞬间用完这 10 次”，而不是强制平均成“每 6 秒 1 次”。
- 对用户侧防刷冷却（如 `user.command_rate_limit`、`group.command_rate_limit`），平台必须保留上述可突发语义，避免把配置实现成违背直觉的“严格等间隔”节流。
- 对插件日志、调试输出等高频内部流量（如 `log.rate_limit_per_plugin`），底层实现可以用令牌桶、漏桶或等价平滑算法，但对外暴露的预算语义仍必须与 `"<count>/<duration>"` 一致，不得 silently 引入更小的默认 burst。

说明：

- v0.1 不追求完整 REST 纯度，但接口命名尽量资源化。
- 敏感配置默认不通过该接口原样返回。
- 插件安装、卸载、配置修改必须要求管理员鉴权。
- `POST /api/plugins/install` 不应同步阻塞等待 `pip` / `npm` 完成；后端应立即返回任务标识，由前端通过任务状态流或轮询获取进度。
- `POST /api/plugins/install` 的标准交互应为：立即返回 `202 Accepted` 与 `task_id`，前端随后通过 `/ws/tasks` 持续接收安装阶段、进度摘要与 `pip` / `npm` 输出，而不是保持单个 HTTP 请求长时间阻塞。
- `GET /api/plugins` 的正式返回应包含 `registration_state`、`desired_state`、`runtime_state` 三层状态，以及供前端展示的可选 `display_state`；不应只压缩成单一扁平状态字段。
- 首次初始化接口仅在 `setup_required` 阶段开放，且默认限制为本机访问。
- `POST /api/system/shutdown` 用于触发进程内的统一 Graceful Shutdown，而不是直接依赖操作系统信号。对 Windows 而言，它应作为 Launcher 停止服务的首选路径；只有接口超时无响应时，Launcher 才回退到系统 API 强制终止。
- `/api/webhooks/{plugin_id}/{route}` 不属于管理会话 API，不能复用管理员登录态作为外部服务鉴权方案；其安全性必须由插件声明的共享密钥、签名校验、来源白名单和平台能力授权共同保证。
- 远程插件安装或远程索引拉取不进入 v0.1 主线，留到后续插件索引 / 分发阶段再评估。
- v0.2+ 可扩展远程 URL、Git 或插件索引来源，但需要和插件分发能力一起规划，不在首版提前透支。
- HTTP 管理接口的最终契约必须以 `contracts/web-api.openapi.yaml` 为唯一来源；本节正文用于解释边界和意图，不作为前后端各自补字段的依据。

**统一错误响应格式**：

- 除 `204 No Content` 之外，所有非 2xx HTTP 响应都应返回统一 JSON envelope，避免前端为不同接口维护多套错误解析逻辑。
- 推荐结构如下：

```json
{
  "error": {
    "code": "platform.invalid_request",
    "message": "请求参数不合法",
    "message_key": "errors.platform.invalid_request",
    "request_id": "req_01HXYZ...",
    "details": {
      "field": "plugin_id"
    }
  }
}
```

- `code` 必须来自 `contracts/error-codes.yaml` 的正式错误码枚举；`message` 为人类可读文案，允许随语言环境变化；`message_key` 为前端、Launcher 和 CLI 共享的资源键。
- `request_id` 必须同时进入结构化日志，便于从 Web UI、诊断包和服务端日志串联一次失败请求。
- `details` 仅承载结构化附加信息（字段名、约束、当前状态等），不得把原始堆栈或敏感配置直接返回给前端。

#### 3.9.7 规划中的 WebSocket

| 接口 | 用途 | 消费者 |
| --- | --- | --- |
| `/ws/logs` | 实时日志流 | Web UI、Launcher |
| `/ws/events` | 服务状态、插件状态、关键事件流 | Web UI、Launcher、调试工具 |
| `/ws/tasks` | 长任务进度、阶段与结果流 | Web UI、调试工具 |
| `/ws/plugins/{id}/console` | 插件 stdout / stderr 实时终端流 | Web UI、调试工具 |

约束：

- WebSocket 连接需携带已登录 Token。
- `ws/logs` 默认推送摘要和新增日志，不默认回放全部历史。
- `ws/events` 聚焦运行状态变化，不把所有原始聊天消息无差别广播给前端。
- `/ws/tasks` 用于插件安装、迁移、恢复等长任务的实时阶段回传，可附带受控的 `pip` / `npm` 输出摘要，避免前端因同步等待而超时。
- `ws/plugins/{id}/console` 用于开发与排障，不替代结构化日志归档；stdout 与 stderr 需要带来源和时间戳。
- `/ws/plugins/{id}/console` 仅管理员会话可用；默认应对敏感配置、Token、密钥等内容执行脱敏处理。
- 开发态插件可在受控调试模式下额外开启更完整的原始输出，但不应作为默认行为暴露给普通管理会话。
- WebSocket 通道名、消息 envelope、服务端主动推送事件和关闭语义，必须以 `contracts/websocket-events.yaml` 或等价结构化接口清单为唯一来源；本节仅描述其职责范围。

**统一 WebSocket envelope**：

- 所有 WebSocket 通道都必须使用同一套顶层 envelope，至少包含 `channel`、`type`、`timestamp`、`data` 四个字段；仅在需要关联用户操作或返回结构化失败时，才附加可选的 `request_id` 与 `error`。
- 推荐结构如下：

```json
{
  "channel": "tasks",
  "type": "task.updated",
  "timestamp": "2026-03-17T09:30:00Z",
  "request_id": "req_task_001",
  "data": {
    "task_id": "task_123",
    "status": "running"
  },
  "error": null
}
```

- `channel` 用于区分来源通道（如 `logs`、`events`、`tasks`、`plugin_console`）；`type` 用于区分该通道下的具体事件种类；`data` 只承载通道私有 payload，不得把通道名或错误码再平铺到顶层。
- `error` 存在时应复用 HTTP 错误 envelope 的核心字段语义，至少包含 `code`、`message_key` 和可选 `details`；不得为不同 WS 通道发明不兼容的错误字段风格。
- 时间字段、关闭原因和服务端主动推送消息类型的最终枚举，必须在 `contracts/websocket-events.yaml` 中统一定义；前端 mock、Launcher 和后端实现都只能从该文件派生。

**WebSocket 会话续期与过期规则**：

- WebSocket 连接建立时必须验证 Token 有效性；Token 无效或已过期时拒绝升级并返回 HTTP 401。
- 已建立的 WebSocket 连接在 Token 过期后的行为：平台应在 Token TTL 到期时主动向客户端发送 `session_expired` 消息，随后在短暂宽限窗口（如 30 秒）后断开连接。不允许已过期的 WebSocket 连接无限保持。
- 前端收到 `session_expired` 后应尝试使用刷新后的会话 Token 重新建立 WebSocket 连接，不需要用户重新登录（配合 3.9.3 的滑动续期策略）。
- 管理员重置凭据或执行 `reset-admin` 后，所有现有 WebSocket 连接必须立即断开，不保留宽限窗口。

#### 3.9.8 后台任务与长操作接口模型

- Web、CLI 与 Launcher 触发的长操作统一复用 3.4.7 定义的任务模型，不单独发明“安装任务”“迁移任务”“备份任务”的私有状态字段。
- `task_type` 建议至少覆盖 `plugin.install`、`plugin.uninstall`、`plugin.reload`、`backup.create`、`restore.apply`、`config.migrate`、`db.migrate`、`render.preview`。
- 后台任务应支持最小状态集：`pending`、`running`、`succeeded`、`failed`、`cancelled`、`interrupted`（服务重启时 `running` 任务自动标记为 `interrupted`，详见 3.4.7）。
- 任务结果应统一通过 `GET /api/tasks/{id}` 查询，实时进度与阶段摘要统一通过 `/ws/tasks` 推送。
- 长操作如需输出终端日志，应通过任务流或受控日志摘要回传，而不是把整个 HTTP 请求保持到超时。

### 3.10 配置、数据存储与日志

#### 3.10.1 配置策略

- 开发态允许使用 `.env` 和 `.env.example`，仅用于本地调试与快速启动。
- 发行版统一以 `config/user.yaml` 作为用户主配置入口。
- `config/user.yaml` 的正式机器可校验定义应以 `contracts/config.user.schema.json` 为唯一来源；本章中的配置示例、默认值和热更新说明必须与该 schema 保持一致。
- 项目提供 `config/default.yaml` 作为默认模板和回退基线。
- 当 `user.yaml` 不存在时，由 server 在启动链路中基于 `default.yaml` 生成首份用户配置。
- 配置文件建议包含 `schema_version`，为后续迁移预留基础。
- `config/user.yaml` 还应暴露渲染相关配置项，如 `render.worker_count`、`render.browser_args`、`render.browser_path`，允许用户在高配宿主机上调优并行渲染能力或指定系统 Chromium 路径。

##### 3.10.1.1 配置变更生效矩阵

| 类别 | 典型配置项 | 生效方式 |
| --- | --- | --- |
| 热更新可立即生效 | `log.level`、`log.retention_days`、平台级限流阈值、`runtime.max_pending_events_per_plugin`、`runtime.max_pending_control_events_per_plugin`、`runtime.dependency_install_timeout_seconds`、`runtime.max_concurrent_dependency_installs`、`runtime.ipc_pending_actions_max`、`runtime.ipc_action_burst_limit`、`runtime.stderr_rate_limit_bytes_per_second`、`render.queue_wait_timeout_seconds`、插件命名空间下的非敏感配置、部分非敏感展示类配置 | 由 Config Manager 热更新并立即广播，不要求插件重载或服务重启 |
| 需要局部重载或重连 | OneBot11 连接地址 / Token、`render.worker_count`、`render.browser_args`、`render.browser_path`、`runtime.nodejs_max_old_space_size_mb`、插件权限授予结果、调度时区 | 触发 Adapter 重连、Render Worker 回收重建、插件重载或 Scheduler 重算，不要求整个服务完全退出 |
| 必须重启服务 | Web 监听地址 / 端口、SQLite 路径、关键目录根路径、运行环境根路径 | 明确要求重启主服务后生效，不允许实现层静默热切换 |

说明：

- 配置是否支持热更新应以本矩阵为准，不允许不同入口各自猜测生效方式。
- 若某项配置理论上支持局部重载，但当前实现尚未具备对应能力，平台应明确提示“需重启服务”而不是伪装成已即时生效。
- 对“热更新可立即生效”和“需要局部重载或重连”的配置，若应用新配置失败，平台应回退到上一份已知可用配置，并向 Web UI / CLI / 日志暴露同一份失败摘要。
- 对 `必须重启服务` 的配置，平台可以先保存为待生效状态，但不得声称已经应用；真正生效时间点必须以服务重启为准。

##### 3.10.1.2 `user.yaml` 参考结构

以下为合并各章节配置项后的完整 `user.yaml` 参考示例。具体语义和约束以各子系统设计章节为准，本节仅提供结构全貌。

```yaml
# 服务配置
server:
  host: "0.0.0.0"
  port: 8080

# OneBot11 协议配置
onebot:
  ws_url: ""
  access_token: ""

# 命令系统
command:
  prefixes: ["/", "!"]        # 命令前缀列表（见 3.4.5）

# 管理员与权限
admin:
  super_admins: []             # 超级管理员 QQ 号列表（见 3.4.6）
permission:
  default_level: "everyone"    # 未声明权限级别的命令默认权限

# 渲染服务
render:
  worker_count: 1              # 渲染 Worker 并发数
  browser_args: ["--disable-gpu"]  # Chromium 额外启动参数
  browser_path: ""             # 可选，ARM64 或自定义系统 Chromium 路径
  timeout_seconds: 30          # 单次渲染超时
  queue_wait_timeout_seconds: 15 # 渲染任务在队列中的最大等待时间
  queue_max_length: 32         # 渲染队列最大长度

# 调度器
scheduler:
  timezone: ""                 # 空字符串表示使用服务所在环境时区；容器场景建议显式设置

# 运行时与插件
runtime:
  plugin_init_timeout_seconds: 30
  plugin_init_max_total_seconds: 300
  plugin_event_timeout_seconds: 60
  max_pending_events_per_plugin: 16 # 单插件普通业务事件待处理队列上限
  max_pending_control_events_per_plugin: 4 # 单插件控制事件保留队列上限
  nodejs_max_old_space_size_mb: 256  # 单个 Node.js 插件 V8 堆上限
  dependency_install_timeout_seconds: 900 # pip/npm 依赖安装绝对超时（15 分钟）
  max_concurrent_dependency_installs: 1 # 同时允许进入 pip/npm 构建阶段的任务数
  ipc_pending_actions_max: 256  # 单插件待处理 action 队列上限
  ipc_action_burst_limit: "100/1s" # 单插件 IPC action 突发阈值
  stderr_rate_limit_bytes_per_second: 262144 # 单插件 stderr 透传速率上限（256 KB/s）
  max_concurrent_tasks_per_plugin: 4
  crash_backoff_initial_seconds: 2
  crash_backoff_max_seconds: 60
  shutdown_grace_seconds: 10     # 插件优雅退出等待窗口（见 3.7.10）
  ipc_message_max_bytes: 8388608 # 单条 JSONL 协议消息体上限 8MB

# 存储
storage:
  kv_value_max_bytes: 65536    # KV 单值大小上限
  plugin_workdir_soft_limit_mb: 256

# 数据留存
data:
  audit_logs_retention_days: 90    # 审计日志默认保留 90 天
  event_records_retention_days: 7  # 事件记录默认保留 7 天
  download_cache_retention_days: 15 # 下载缓存超过 15 天未访问后可清理

# 日志
log:
  level: "info"                # DEBUG / INFO / WARN / ERROR
  retention_days: 7
  rate_limit_per_plugin: "200/10s"

# 消息与限流
message:
  rate_limit_per_plugin: "20/10s"  # 单插件消息发送速率限制
  rate_limit_per_target: "5/5s"
  circuit_breaker_seconds: 30

# 用户侧防刷
user:
  command_rate_limit: "10/60s"
  cooldown_reply: true
group:
  command_rate_limit: "30/60s"

# 适配器重连
adapter:
  reconnect_initial_seconds: 2
  reconnect_multiplier: 2
  reconnect_max_seconds: 120
  reconnect_jitter_ratio: 0.2
  connect_timeout_seconds: 15

# HTTP 代理（插件 http.request 使用）
http:
  timeout_seconds: 10
  max_retries: 2
```

说明：

- 本参考结构汇总自各子系统设计章节（3.4.5、3.4.6、3.5.4、3.6.1、3.8、3.11 等），具体默认值和语义约束以各章节为准。
- 运行时以 `config/default.yaml` 中的默认值为基线，`user.yaml` 中的同名键覆盖默认值。
- 实现者应基于本结构生成 `default.yaml` 模板，确保所有配置项都有明确的默认值。

#### 3.10.2 配置职责边界

| 位置 | 职责 |
| --- | --- |
| `config/default.yaml` | 默认配置模板，不直接作为用户长期编辑入口 |
| `config/user.yaml` | 用户可编辑基础配置，如协议地址、Web 端口、日志级别 |
| `system_configs` 表 | 运行态或管理态配置，不鼓励用户直接编辑 |
| `secret_store` 表 | 敏感数据与凭据，单独存储，不混入普通配置 |

说明：

- 敏感数据不写入公开用户配置文件，由后端统一管理存储；首版可落库并为后续加密能力预留接口。
- v0.1 不承诺对 `secret_store` 提供数据库内静态加密，默认依赖操作系统文件权限、部署目录访问控制和备份访问控制保护敏感数据；如需更强静态保护，留待后续版本引入受控加密方案。
- 诊断包、错误摘要、CLI 输出和 Web 调试界面默认不得包含 `secret_store` 中的明文值；最多只允许显示键名、来源说明或掩码后的片段。
- `user.yaml` 是用户主配置入口，不依赖数据库即可人工修复。
- `system_configs` 与 `secret_store` 的边界需要在实现阶段保持严格分离。

##### 3.10.2.1 插件敏感凭据注入机制

- 注入责任属于 Runtime 启动流程的一部分，职责边界见 3.5.4。
- `secret_store` 中的插件专属密钥不应通过 `config.read` 或普通 RPC 流量直接下发给插件。
- Runtime 在启动插件进程时，应按插件作用域把获授权的敏感值注入为环境变量，例如 `RAYLEABOT_SECRET_*`。
- 插件进程默认不应继承宿主机完整环境变量；Runtime 只应传递最小白名单环境，例如必要的 `PATH`、临时目录、语言运行时必需变量和受控注入的 `RAYLEABOT_SECRET_*`。
- 来自 CI、开发机或宿主机系统的其他环境变量不得默认透传给插件，除非平台存在显式白名单配置并在诊断信息中可观测。
- 插件 SDK 应优先从环境变量读取密钥；stdout / stderr、结构化日志和调试控制台都不应默认回显这些值。
- 若插件通过 `event.expose_webhook` 注册需要网关层验签的路由，其 `secret_ref` 也必须指向该插件作用域下的 `secret_store` 条目，而不是依赖 `storage.kv` 或插件私有数据目录；这样 Bot Core 才能在事件投递前完成签名校验。

##### 3.10.2.2 插件配置存储模型

插件的非敏感运行配置统一存储在 `system_configs` 表中，使用命名空间前缀隔离：

- 插件配置的键格式为 `plugin:<plugin_id>:<config_key>`，例如 `plugin:weather:default_city`、`plugin:daily_report:send_time`。
- `config.read` 和 `config.write` 协议动作自动限定到调用方插件的命名空间，插件无法读写其他插件的配置。
- 插件可在 manifest 中声明可选的 `default_config` 字段，提供初始默认配置值：

```json
{
  "default_config": {
    "default_city": "北京",
    "update_interval": 3600
  }
}
```

- 当插件首次启用时，Runtime 检查 `system_configs` 中是否已存在该插件的配置条目；若不存在，则将 `default_config` 写入作为初始值。
- 已存在的用户自定义配置不会被 `default_config` 覆盖，即使插件版本升级也保留用户已修改的值。
- `config/user.yaml` 仅用于平台级配置（如服务端口、日志级别、OneBot 连接参数等），不承载插件业务配置。
- 插件敏感凭据的注入机制见 3.10.2.1，不通过 `config.read` 暴露。
- 当 Web UI、CLI 或其他受控管理入口修改某个插件命名空间下的配置时，Config Manager 应向对应运行中插件投递一次 `config.changed` 内部事件（见 3.2.4），使插件可以在内存中热更新配置而无需轮询 `config.read`。
- `config.changed` 的 `payload` 至少应包含 `plugin_id`、`changed_keys` 与可安全回显的最新值摘要；敏感配置和 `secret_store` 内容不得通过该事件下发。
- 若目标插件当前不在 `running`，则仅持久化配置，不补发事件；插件在下一次启动时应从最新配置初始化。

#### 3.10.3 配置迁移策略

- 配置文件与数据库表都应带版本号。
- 升级时执行显式迁移，而不是隐式猜测旧字段含义。
- v0.1 可以只支持有限迁移，但迁移入口和版本标识应从首版就立起来。
- 程序启动时自动检测 `config/user.yaml` 与 `data/` 下的 `schema_version`；但启动阶段的自动迁移仅限于非破坏性、向前兼容且已被标记为 `startup_safe` 的变更，例如新增表、新增可空字段、补充索引或可幂等的默认值回填。
- 若检测到包含字段删除、字段语义重写、数据清洗、需要人工确认的批量回填、跨大版本跨度或其他被标记为 `offline_required` 的迁移，服务必须在启动阶段主动熔断，返回 `platform.migration_required`，并要求管理员在停服窗口下先执行 `raylea migrate`。
- `raylea migrate` 与启动自动迁移必须复用同一套 migration plan 与元数据分类，不允许维护两套彼此分叉的迁移逻辑；差异只在于“是否允许在启动阶段自动执行”。
- 迁移失败时必须中止进入 `running`，并把失败摘要暴露给 Launcher 与 Web 管理面板。
- 提供 `raylea backup` 命令，默认将 `config/`、`data/`、`plugins/installed/` 打包为 zip 备份；Web 管理面板可调用同等能力的后端入口。
- 由于常规备份默认包含 `data/` 与状态库，备份文件可能间接包含 `secret_store` 中的敏感信息；v0.1 至少应在 CLI / Web 导出时给出明确风险告警，并提示用户为备份文件设置访问控制或密码保护。
- 常规备份默认不包含 `cache/`、`logs/`、`.deps/`、Chromium 运行时、下载缓存和渲染缓存；这些内容应按“可重建运行时资源”处理，而不是混入恢复用备份包。
- 诊断包与恢复备份必须分离：前者面向排障，后者面向恢复，不应在首版实现中混成一个归档入口。

**备份包 manifest 规范**：

备份包内应包含一个 `backup_manifest.json` 元数据文件，供 `raylea restore` 在导入前完成校验和兼容检查。建议最小字段：

| 字段 | 说明 |
| --- | --- |
| `created_at` | 备份创建时间（ISO 8601） |
| `core_version` | 创建备份时的 `raylea-server` 版本 |
| `config_schema_version` | `user.yaml` 的 `schema_version` |
| `db_schema_version` | SQLite 状态库的 schema 版本 |
| `consistency` | `offline`（停服备份）或 `online`（在线快照） |
| `plugins` | 已安装插件列表，含 `id`、`version`、`data_schema_version` |
| `directories` | 备份包包含的目录清单（如 `config/`、`data/`、`plugins/installed/`） |

说明：

- `raylea restore` 在解包前应先读取 manifest，与当前 Core 版本和 DB schema 版本做兼容检查；不兼容时拒绝恢复并输出摘要。
- `consistency: online` 的备份包应在 Web UI / CLI 中提示"非严格一致性快照"。
- 在线备份模式（`consistency: online`）仅保证核心状态库的一致性快照；`config/`、`plugins/installed/` 及插件私有业务文件只承诺尽力而为（Best-effort）的近似时间点打包，不应向用户暗示“整包所有文件都严格处于同一瞬时点”。
- manifest 中的 `plugins` 列表用于恢复后批量校验插件兼容性，避免逐个插件启动后才发现不兼容。

**在线备份的 SQLite 安全导出管线**：

- `raylea backup` 在 `consistency: online` 模式下，严禁直接读取并压缩 `data/` 目录下的活跃 `.db` 和 `.db-wal` 文件——服务运行时 WAL 文件处于持续写入状态，直接拷贝必然产生不一致的快照。
- 正确的执行管线为：先通过 SQLite 的 `VACUUM INTO 'temp_backup.db'` 命令（或 `sqlite3_backup_init` API）将当前一致性快照导出到临时目录，然后 ZIP 归档器仅打包该临时导出文件（重命名为原文件名），最后清理临时文件。
- `config/` 和 `plugins/installed/` 等非数据库目录可在状态库导出完成后再进行文件级打包（近似同一时间点）。
- 在线备份执行期间，写入句柄不应被阻塞；`VACUUM INTO` 在 WAL 模式下不干扰正常读写。

#### 3.10.4 数据库存储策略

数据库采用 SQLite，默认开启 WAL 模式。

**文件系统约束**：SQLite 状态库文件所在目录（`data/`）必须位于本地文件系统（ext4、xfs、NTFS 等）上。严禁将状态库部署在 NFS、SMB、CIFS 等网络共享存储上——WAL 模式依赖的 POSIX 文件锁和共享内存（`.shm`）在网络文件系统上行为不可靠，将导致 `database is locked` 错误甚至数据库文件不可逆损坏。服务启动时应检测 `data/` 目录的文件系统类型；若检测到网络文件系统，应在日志中输出 `ERROR` 级别警告并阻止服务进入 `running`。

Go 侧并发控制策略：

- v0.1 默认拆分 `Read DB Handle` 与 `Write DB Handle`，不建议读写混用同一个连接池。
- 数据库 DSN 必须显式附加 WAL 与忙等待相关参数，例如 `_journal_mode=WAL`、`_busy_timeout=5000`，确保底层驱动在短暂锁争用时优先等待而不是立刻抛错。
- 数据库初始化时还应显式配置 `PRAGMA wal_autocheckpoint`，避免在高频写入或长读事务下完全依赖默认检查点策略。
- 写入路径必须通过单写连接或单写仓储层串行化；默认要求写入句柄执行 `SetMaxOpenConns(1)`。
- 只读句柄可在 WAL 模式下设置更大的连接数，例如 `10` 或 `20`，以发挥并发读取能力；所有写操作、迁移与状态变更仍必须走写入句柄。
- 相比事后对 `database is locked` 做盲重试，优先在进程内把写请求排队，减少 SQLite 写锁争用；读路径则应避免被单写连接池拖慢。
- 对 `event_records`、诊断计数器、非关键回溯摘要等高频内部写入，仓储层不应逐条开启独立短事务写盘；应优先采用短窗口写聚合（batch insert / batched upsert）或等价的内存缓冲后合批提交，摊薄单写连接的 fsync 成本。
- `storage.kv` 面向插件的用户关键状态写入默认仍应以“返回成功即已持久化”为语义，不建议为追求吞吐而偷偷改成异步最终一致；但文档和 SDK 应明确不鼓励插件把高频计数器或刷屏级临时状态逐条写入 `storage.kv`，应在插件侧先做内存聚合后再批量提交。
- 为防止 `.wal` 文件在极端长读事务或并发压力下异常膨胀，平台应在低频维护任务中执行受控检查点，例如每天低峰期触发一次 `PRAGMA wal_checkpoint(TRUNCATE)`，并把异常膨胀情况暴露给诊断与告警。

SQLite 主要保存以下内容：

- 管理员会话。
- 插件安装与注册状态。
- 插件启用配置与运行状态快照。
- 用户与群组映射缓存。
- 权限规则。
- 调度任务定义。
- 运行态配置与敏感配置。

建议的数据对象包括：

- `identity_users`
- `identity_groups`
- `identity_group_members`
- `admin_sessions`
- `system_configs`
- `secret_store`
- `audit_logs`
- `permission_grants`
- `schedules`
- `plugin_packages`
- `plugin_instances`
- `plugin_states`
- `event_records`
- `schema_migrations`

补充约定：

- `plugin_packages` 应至少记录插件来源类型、安装时间、包 hash / manifest hash 与当前生效版本，便于诊断、升级比对和来源追踪。
- 群内成员角色、群名片和成员级 TTL 不得扁平化写入 `identity_users`；这类“用户与群的关系数据”必须单独存入 `identity_group_members`，否则无法正确表达同一用户在不同群中的不同角色。

**核心表结构草案**：

- `plugin_packages`：记录“一个被安装或扫描过的插件包版本”。建议字段至少包含 `package_id`、`plugin_id`、`version`、`source_type`、`source_ref`、`runtime`、`entry`、`manifest_json`、`manifest_hash`、`package_hash`、`installed_at`、`installed_by`。建议唯一约束为 `(plugin_id, version, manifest_hash)`，并建立 `(plugin_id, installed_at DESC)` 索引以支撑版本回溯与最近安装记录查询。
- `plugin_instances`：记录“当前环境中这个插件逻辑实例的注册与期望状态”。建议字段至少包含 `instance_id`、`plugin_id`、`package_id`、`role`、`registration_state`、`desired_state`、`enabled_at`、`disabled_at`、`last_operation_task_id`、`updated_at`。`plugin_id` 在 v0.1 应保持唯一；`package_id` 外键指向 `plugin_packages.package_id`。
- `plugin_states`：记录“当前实例的易变运行态快照”。建议字段至少包含 `instance_id`、`runtime_state`、`pid`、`crash_count`、`backoff_until`、`last_heartbeat_at`、`last_error_code`、`last_error_summary`、`updated_at`。`instance_id` 同时作为主键和外键指向 `plugin_instances.instance_id`，避免把运行态和注册态混进同一表。
- 三表关系应固定为：`plugin_packages` 管历史包版本，`plugin_instances` 管当前已注册插件，`plugin_states` 管当前运行态。运行态变化只更新 `plugin_states`，安装 / 升级 / 卸载才改动 `plugin_packages` 与 `plugin_instances`。
- `identity_users`：缓存全局用户信息。建议字段至少包含 `user_id`、`nickname`、`avatar_url`、`updated_at`、`expires_at`；主键为 `user_id`，并建立 `expires_at` 索引用于 TTL 清理。
- `identity_groups`：缓存群信息。建议字段至少包含 `group_id`、`group_name`、`avatar_url`、`updated_at`、`expires_at`；主键为 `group_id`，并建立 `expires_at` 索引。
- `identity_group_members`：缓存“某用户在某群中的关系信息”。建议字段至少包含 `group_id`、`user_id`、`role`、`card`、`nickname_snapshot`、`updated_at`、`expires_at`；主键固定为 `(group_id, user_id)`，并建立 `user_id` 与 `expires_at` 索引，供权限判定和 TTL 清理复用。
- `event_records`：仅保存短期调试与回溯用事件摘要。建议字段至少包含 `event_id`、`event_type`、`source_protocol`、`actor_id`、`target_id`、`message_id`（可空，仅消息类事件填充）、`summary_json`、`created_at`、`expires_at`；建立 `created_at DESC`、`expires_at` 与按需的 `message_id` / `event_id` 索引，避免清理任务全表扫描并支持 reply 回源查询。
- `event_records` 的清理实现应采用“小批量、可中断”的批处理策略，例如启动后与每日低峰各执行一次；每次先选出一批过期主键，再按主键删除，而不是做无上限大事务删除，避免 WAL 暴涨和长时间写锁。
- `event_records` 的写入实现也应避免“每收到一条事件就单独 `INSERT` + 单独提交”这种最差路径；在高流量场景下建议用受控小批次（如几十条一组）合并到短事务，既降低 fsync 次数，也减少单写连接排队时间。

`identity_users`、`identity_groups` 与 `identity_group_members` 用途说明：

- `identity_users` 与 `identity_groups` 用于缓存 QQ 用户与群的基础资料（昵称、头像 URL、群名等），`identity_group_members` 用于缓存群成员关系信息（群内角色、群名片等），共同减少对 OneBot11 API 的重复查询。
- 由 Adapter 在收到事件时自动填充或按需更新：首次遇到新用户/群时写入基础信息，后续按 TTL 或事件触发增量刷新。
- 为插件提供用户/群信息查询能力，插件可通过平台能力获取用户昵称、群名称等基础信息，而无需自行调用底层协议 API。
- 与聊天侧权限模型的关系：`identity_group_members` 中的群角色信息（群主、管理员、普通成员）可作为权限系统判断群管理员身份的数据来源。
- v0.1 仅缓存基础字段（用户 ID、昵称、群 ID、群名称、群角色 / 群名片），不追求完整用户画像。

设计原则：

- 配置主入口不依赖数据库，避免用户无法脱离 UI 修改配置。
- 日志不以数据库为主存储，避免高频写入拖累状态库。
- 如后续需要数据库级日志，应优先拆成 `audit_logs`、`plugin_events` 等明确语义表，而不是笼统使用 `logs`。
- “已安装”“是否启用”“当前是否在线”“是否处于 dead_letter”应分开建模，不应混在一个字段里。
- 表名优先按职责命名，避免过早固化为具体业务表语义。
- `event_records` 默认仅用于有限调试记录、错误回溯或短期审计，不作为 v0.1 的全量聊天历史长期留存方案。

**数据留存与清理策略**：

| 数据对象 | 默认留存 | 清理方式 | 说明 |
| --- | --- | --- | --- |
| `event_records` | 7 天 | 平台定时清理过期记录 | 仅保留最近事件用于调试和错误回溯 |
| `audit_logs` | 90 天 | 平台定时清理过期记录 | 权限变更、插件安装/卸载、管理员操作等审计记录 |
| `cache/downloads/` | 15 天未访问 | 平台定时按最后访问时间清理 | 仅保留近期可能复用的下载缓存，避免长期吃满磁盘 |
| 插件日志文件 | 7 天（同 `log.retention_days`） | 日志轮转策略统一管理 | 单插件日志目录上限建议 `50 MB`，超限后触发轮转 |
| 插件 console ring buffer | 最近 1000 条或 2 MB | 内存环形缓冲，进程重启清零 | `/ws/plugins/{id}/console` 的回放缓冲 |
| `admin_sessions` | 过期自动清理 | TTL 到期后标记失效 | 结合 3.9.3 的会话 TTL 策略 |

说明：

- 以上默认值应可通过 `config/user.yaml` 调整（如 `data.event_records_retention_days`、`data.audit_logs_retention_days`）。
- 清理任务应在服务低峰期执行，不阻塞正常事件处理。
- v0.1 的清理机制可以简单实现为启动时 + 每日定时两个触发点，不要求实时监控。
- 下载缓存清理必须跳过当前仍被安装任务占用的文件，优先基于锁文件、最后访问时间或活跃任务引用做判定，避免把正在使用的缓存误删。

#### 3.10.5 日志系统

日志采用结构化 JSON Lines 输出，建议目录如下：

```plain
logs
├─ core.log
├─ adapter.log
├─ launcher.log
├─ plugins
│  ├─ weather.log
│  └─ music.log
└─ error.log
```

日志系统要求：

- 至少区分核心日志、协议日志、启动器日志、插件日志。
- 插件日志建议按插件拆分，便于排查。
- Web UI 与 Launcher 默认读取日志文件或日志流，不直接查询数据库日志表。
- 日志轮转应作为核心服务的强制默认行为，默认最多保留 7 天备份文件，避免长期运行后撑满磁盘。
- 最多保留天数属于核心配置项，后续可通过 Web UI 暴露修改入口，但插件不得自行覆盖或绕过该限制。
- 轮转天数建议通过 `log.retention_days` 配置项暴露，默认值为 `7`。

日志与调试输出脱敏矩阵：

| 输出通道 | 默认采集 | 允许内容 | 禁止内容 |
| --- | --- | --- | --- |
| 核心结构化日志 | 是 | 状态变化、错误码、任务摘要、资源检查结果 | `secret_store` 明文、`RAYLEABOT_SECRET_*` 实际值 |
| 插件结构化日志 | 是 | 插件生命周期、能力调用摘要、受控错误信息 | 密钥明文、完整请求头中的认证值 |
| 插件 stdout / stderr 控制台 | 开发与排障场景 | 调试输出、受控堆栈、任务阶段输出 | 未脱敏的密钥、令牌、会话值 |
| 诊断包 | 是 | 日志摘要、状态快照、任务摘要、配置摘要 | 明文 secret、完整访问令牌、完整 Cookie / 会话值 |
| CLI / Web 错误摘要 | 是 | 错误码、失败阶段、掩码后的关键信息 | 明文密码、环境变量 secret、数据库中的敏感字段原值 |

补充约束：

- Web 调试控制台默认只做基础掩码透传，不应把未经处理的原始 stdout / stderr 直接暴露给普通管理会话。
- 开发模式如需查看更完整的原始输出，应通过显式调试开关开启，并在 UI 中给出高风险提示。
- `logger.write` 及其等价日志能力返回的字符串字段，必须在进入 `slog` / 文件写入器前先经过同一套全局脱敏中间层；插件不得通过“把密钥写进日志”绕过聊天链路的脱敏防线。
- `/ws/logs`、`/ws/plugins/{id}/console` 和诊断包导出同样必须复用该中间层，确保“会落盘或会广播”的字符串值只走一套脱敏规则，而不是各子系统各自补丁式实现。

### 3.11 错误处理、恢复与运行约束

#### 3.11.1 错误与恢复策略

平台至少应明确以下错误处理与恢复规则：

- 插件崩溃：按指数退避重试，多次失败后进入 `dead_letter`。
- 协议断连：OneBot 连接状态切换到 `reconnecting`，按重连策略恢复并记录日志。

OneBot11 重连默认参数：

- `adapter.reconnect_initial_seconds = 2`：首次重连等待 2 秒。
- `adapter.reconnect_multiplier = 2`：每次失败后按指数退避放大等待窗口。
- `adapter.reconnect_max_seconds = 120`：指数退避上限 120 秒。
- `adapter.reconnect_jitter_ratio = 0.2`：最终等待时间增加 20% 抖动窗口，降低集中重连。
- `adapter.connect_timeout_seconds = 15`：单次连接握手与首包等待超时。
- 重试次数：无限重试，不设最大次数限制。
- 当连续失败时间超过 `3600` 秒（1 小时）时，降频至 `reconnect_max_seconds` 间隔重试，并将日志级别从 `WARN` 提升至 `ERROR`，同时向 Web UI 暴露长期断连告警。
- 每次成功重连后重置退避计数器和告警级别。

- 协议鉴权失败：连接状态切换到 `auth_failed`，阻止服务误判为可用状态，并向管理端展示明确错误摘要。
- SQLite 被锁：执行短时重试和告警上报；持续失败时应进入降级状态而不是静默卡死。
- 渲染失败：返回显式错误并允许插件降级为文本输出，不应阻塞插件主流程。
- 配置错误：启动前校验失败时应阻止服务进入 `running`，并向启动器 / Web 明确展示错误摘要。

设计要求：

- 错误处理结果必须能映射到统一状态模型，而不是只留在日志文本中。
- 哪些错误只记录日志、哪些错误进入 `backoff`、哪些错误进入 `dead_letter`，应有固定规则。
- 恢复动作必须可观测，便于用户理解系统当前处于“重试中”还是“彻底失败”。

#### 3.11.2 运行约束与资源限额

即使 v0.1 不全部实现，也建议在规划阶段先确定以下约束方向：

- 单插件最大并发任务数。
- 渲染队列最大长度。
- 渲染任务排队等待超时。
- 单次渲染超时。
- HTTP 请求超时与最大重试次数。
- `message.send` / `message.reply` 的平台级速率限制与熔断阈值。
- 单插件 `stderr` 透传速率上限与截断策略。
- 单插件日志写入频率限制。
- 插件工作目录大小限制。
- 临时缓存清理策略。
- Runtime 应预留对子进程施加硬性资源上限的能力，例如单插件最大内存、CPU 占用或进程数限制，避免死循环或内存泄漏直接拖垮宿主机。
- Runtime 与插件之间的单条 JSONL 协议消息体必须有上限，默认 `runtime.ipc_message_max_bytes = 8388608`（8 MB）；超过限制时直接拒绝并返回结构化错误。即使上限提高，图片等大对象仍应优先通过 `file://` 路径、缓存键或渲染产物引用传递，而不是长期依赖 `base64://` 大载荷穿过 IPC。
- Runtime Bridge 必须维护单插件有界的待处理 action 队列，默认 `runtime.ipc_pending_actions_max = 256`；达到上限后优先施加背压，不允许把插件 stdout 映射到无界内存队列。
- 单插件在单位时间内发起的 IPC `action` 请求必须受 `runtime.ipc_action_burst_limit` 约束；持续超限且在短暂宽限窗口后仍不收敛时，应视为协议洪泛并按 `plugin.protocol_violation` 回收整个插件进程组。
- 单插件 `stderr` 透传默认也必须受 `runtime.stderr_rate_limit_bytes_per_second` 约束；超限时允许截断并注入系统告警，但不允许把未限流的原始输出持续写入无界日志队列。
- 底层依赖安装任务默认串行执行，`runtime.max_concurrent_dependency_installs = 1`；v0.1 优先保护共享缓存一致性而不是追求多插件并行构建吞吐。
- 插件普通业务事件队列与控制事件队列必须分离；默认普通队列 `runtime.max_pending_events_per_plugin = 16`，控制队列 `runtime.max_pending_control_events_per_plugin = 4`。`scheduler.trigger`、`config.changed` 等控制事件不得被普通聊天流量直接挤掉。
- `base64://` 内联图片只适合小载荷兜底；官方 SDK 对原始体积超过 `1 MB` 的图片必须自动物化为 `file://` 临时文件，避免大字符串 JSONL 触发序列化 / 反序列化的内存尖峰。

原则：

- 任意单插件都不应轻易拖垮整个平台。
- 平台级限额应优先由 Core、Render Service 和 Runtime 统一控制，而不是让插件各自决定。
- 通过 `http.request` 发起的网络访问除遵守 `permissions.scopes.http_hosts` 外，还应在平台底层执行 SSRF 防御：默认拦截解析到局域网、回环、链路本地和其他保留地址段的请求。
- 如用户确实需要插件访问内网服务，应在 `config/user.yaml` 中显式配置受控白名单或特权开关，而不是依赖宽泛域名匹配绕过平台限制。
- 平台应对 `message.send` 与 `message.reply` 增加保险丝：至少同时限制单插件消息发送速率与单目标短时突发阈值，防止插件 bug 导致刷屏。
- 当消息动作触发限流或熔断时，平台应记录结构化告警，并在短窗口内拒绝继续下发消息动作，而不是让 Adapter 无上限透传。
- 消息限流计数至少应按“插件维度”和“目标会话维度”分别维护，不得只使用单一全局计数器。
- 熔断期间新消息动作默认直接拒绝并返回结构化错误，不进入排队补发；平台不应在熔断结束后自动重放积压消息。
- 熔断恢复后应按滚动时间窗口自然放开，而不是一次性回放所有历史请求；如需人工干预恢复，应在管理界面中可见当前熔断状态。
- Linux 自托管场景可优先结合 `systemd-run`、cgroups 或等价机制为插件子进程施加硬限制；Windows 端可在可行时结合进程组或作业对象实现近似约束。
- 如目标平台暂不支持硬性资源阻断，Runtime 必须在诊断信息和环境检查中明确暴露“仅软限制 / 无硬限制”的降级状态，而不是假定所有部署都具备相同隔离能力。

建议的 v0.1 默认值：

- `runtime.max_concurrent_tasks_per_plugin = 4`
- `runtime.max_pending_events_per_plugin = 16`
- `runtime.max_pending_control_events_per_plugin = 4`
- `render.queue_max_length = 32`
- `render.queue_wait_timeout_seconds = 15`
- `render.timeout_seconds = 30`
- `http.timeout_seconds = 10`
- `http.max_retries = 2`
- `message.rate_limit_per_plugin = 20 / 10 seconds`
- `message.rate_limit_per_target = 5 / 5 seconds`
- `message.circuit_breaker_seconds = 30`
- `runtime.crash_backoff_initial_seconds = 2`
- `runtime.crash_backoff_max_seconds = 60`
- `runtime.nodejs_max_old_space_size_mb = 256`
- 若宿主机为 `4 GB+ RAM` 且仅托管少量 Node.js 插件，可按需上调到 `512`；但 v0.1 默认值仍保持 `256`，以优先保护 `2 GB` 级别自托管环境。
- `runtime.plugin_init_max_total_seconds = 300`
- `runtime.dependency_install_timeout_seconds = 900`
- `runtime.max_concurrent_dependency_installs = 1`
- `runtime.ipc_pending_actions_max = 256`
- `runtime.ipc_action_burst_limit = 100 / 1 seconds`
- `runtime.stderr_rate_limit_bytes_per_second = 262144`
- `storage.plugin_workdir_soft_limit_mb = 256`
- `log.rate_limit_per_plugin = 200 entries / 10 seconds`
- `user.command_rate_limit = 10 / 60 seconds`
- `group.command_rate_limit = 30 / 60 seconds`
- `runtime.ipc_message_max_bytes = 8388608`

上述速率限制配置在 `user.yaml` 中统一采用 `"<count>/<duration>"` 格式（如 `"10/60s"`），详见 3.9.6 的格式规范。

用户侧防刷与冷却机制：

- 平台应对聊天侧用户命令调用施加频率限制，防止单用户或单群短时间内大量触发命令导致插件被淹没。
- 单用户命令调用频率默认限制为 `user.command_rate_limit = 10 / 60 seconds`（每 60 秒最多 10 次命令调用）。
- 单群命令调用频率默认限制为 `group.command_rate_limit = 30 / 60 seconds`（每 60 秒最多 30 次命令调用）。
- 当触发冷却时，Bot Core 应在事件分发前拦截，不将事件投递给插件。
- 冷却提示消息默认开启，可通过 `user.cooldown_reply = true` 配置是否回复提示文本。
- 冷却提示默认文本通过资源键管理，可自定义。
- 冷却机制在 Bot Core 层统一实现，插件无需各自实现防刷逻辑。

说明：

- 上述默认值应保持保守，优先服务首版稳定性，并允许通过 `config/user.yaml` 在受控范围内调优。
- 如部署环境资源明显不足，平台应优先降级渲染与并发吞吐，而不是静默放宽这些保护阈值。

#### 3.11.3 统一错误码目录

平台、插件协议、权限和适配器层使用统一命名规范的错误码，便于日志归因、插件侧容错和 Web UI 展示。

- 错误码的最终枚举、分类、HTTP / 任务映射与默认文案资源键，必须以 `contracts/error-codes.yaml` 为唯一来源；本节目录用于解释语义，不替代正式枚举文件。

**平台错误**：

| 错误码 | 说明 | 推荐处理 |
| --- | --- | --- |
| `platform.render_timeout` | 渲染任务排队超时或执行超时 | 返回降级文本或提示用户稍后重试 |
| `platform.render_queue_full` | 渲染队列已满 | 返回降级文本或排队等待 |
| `platform.invalid_request` | 请求参数或请求体结构不合法 | 返回 `400` 并附带结构化 `details` |
| `platform.task_timeout` | 后台任务超过绝对超时上限 | 强制回收底层执行器并标记任务失败 |
| `platform.install_script_blocked` | 插件依赖要求执行安装脚本，但当前策略默认禁止 | 要求管理员显式确认高风险安装后再重试 |
| `platform.rate_limited` | 消息发送触发平台级限流 | 等待冷却窗口后重试 |
| `platform.config_error` | 配置校验失败 | 阻止服务进入 `running`，输出摘要 |
| `platform.migration_required` | 检测到必须停服执行的迁移 | 拒绝自动启动并要求先执行 `raylea migrate` |
| `platform.resource_missing` | 缺少必要资源 | 阻止服务进入 `running`，提示补齐 |
| `platform.value_too_large` | KV 存储值超过大小限额 | 拒绝写入并返回错误提示 |
| `platform.user_rate_limited` | 用户命令调用触发冷却限流 | Bot Core 拦截事件，不投递给插件 |

**插件协议错误**：

| 错误码 | 说明 | 推荐处理 |
| --- | --- | --- |
| `plugin.not_handled` | 插件声明不处理该事件 | EventBus 尝试下一个匹配插件或丢弃 |
| `plugin.internal_error` | 插件内部处理异常 | 记录日志，不影响其他插件 |
| `plugin.init_timeout` | 插件初始化超时 | 标记插件为 `crashed`，进入退避重试 |
| `plugin.stopping` | 插件已收到 `shutdown`，不再接受新的 Action 请求 | 插件应停止发起新动作并尽快完成清理退出 |
| `plugin.protocol_violation` | 插件输出非 JSONL 内容、消息结构非法或持续 IPC 洪泛 | 记录错误摘要，视严重程度决定是否回收 |
| `plugin.event_timeout` | 插件事件处理超时 | 记录日志并按超时策略处理（见 3.5.4） |
| `plugin.shutdown_timeout` | 插件优雅退出超时 | 强制终止进程，标记为 `crashed`（见 3.7.10） |

**权限错误**：

| 错误码 | 说明 | 推荐处理 |
| --- | --- | --- |
| `permission.denied` | 用户无权执行该命令 | 返回无权提示或静默忽略 |
| `permission.scope_violation` | 插件尝试访问未授权作用域 | 拒绝操作并记录告警 |
| `permission.blacklisted` | 用户或群处于黑名单中 | 静默忽略，不投递事件 |

**适配器错误**：

| 错误码 | 说明 | 推荐处理 |
| --- | --- | --- |
| `adapter.send_failed` | 消息发送失败 | 返回结构化错误给插件 |
| `adapter.reply_target_missing` | 引用回复目标不存在或已失效 | 若插件启用 `fallback_to_send_if_missing`，可自动降级为普通发送一次 |
| `adapter.connection_lost` | 协议连接断开 | 进入 `reconnecting` 状态 |
| `adapter.auth_failed` | 协议鉴权失败 | 进入 `auth_failed` 状态，暴露错误摘要 |

命名规范：

- 错误码统一采用 `{domain}.{error_name}` 的小写蛇形格式。
- `domain` 固定为 `platform`、`plugin`、`permission`、`adapter` 四类。
- 所有错误码必须在协议 `error` 消息的 `code` 字段中使用，并与日志中的 `error_code` 保持一致。

**默认文案与国际化预留**：

- `contracts/error-codes.yaml` 中的每个错误码都必须带 `message_key`，并允许同时声明默认 `zh-CN` 文案与可选的其他语言文案。
- v0.1 至少保证 `zh-CN` 资源可用，并为 `en-US` 或后续语言保留同一套 `message_key` 结构；Web UI、Launcher 与 CLI 均通过 `message_key` 查找本地化文案，而不是各自硬编码字符串。
- HTTP API、任务摘要和 UI 展示中对外暴露的稳定标识始终是 `code`；`message` 只作为可本地化的人类可读文本，不得被前端当作程序判断依据。

### 3.12 CLI 工具

#### 3.12.1 定位

- CLI 是 v0.1 的本地离线恢复与运维入口，不是第二套常规在线管理面。
- CLI 应由主程序提供受控子命令；文档中统一记为 `raylea <subcommand>`，实际可由发行包中的主二进制或等价包装入口承载。
- 当服务已正常运行且 Web 管理面板可用时，常规插件管理、状态查看和日志浏览应优先走 Web UI / Web API，而不是重复在 CLI 中建设完整控制面。

#### 3.12.2 v0.1 命令边界

- `raylea reset-admin`：本机恢复管理员凭据并重新进入初始化向导。
- `raylea backup`：导出恢复用备份包。
- `raylea restore`：在停服窗口下从受支持的恢复包恢复 `config/`、`data/` 与 `plugins/installed/`。
- `raylea doctor`：执行本地环境与资源完整性检查。
- `raylea cleanup`：回收可重建缓存、过期下载缓存与遗留临时目录，不触碰 `config/`、状态库和插件业务数据。
- `raylea migrate`：执行显式配置或数据库迁移。

约束：

- CLI 命令默认面向本机、离线、恢复或诊断场景，不承担远程管理职责。
- CLI 可调用与 Web 共用的服务端内部能力，但不得发明独立状态模型、独立配置模型或独立插件管理协议。
- Launcher 如需触发恢复、检查或备份能力，应优先复用 CLI 或共享后端逻辑，而不是各自复制实现。
- `raylea cleanup` 仅允许清理“可重建”目录，如 `cache/downloads/`、过期渲染缓存、失败安装残留临时目录等；不得清理 `data/plugins/<plugin_id>/`、`plugins/installed/` 或其他用户业务数据目录。
- `raylea doctor` 在 Windows 上必须额外检查系统是否开启长路径支持（`LongPathsEnabled`）；若未开启，应输出明显 `WARN`，提示 Node.js `node_modules/`、Python `.venv/` 或深层插件目录可能因 `MAX_PATH` 触发诡异 I/O 错误。

#### 3.12.3 命令可用性矩阵

| 命令 / 能力 | 在线可用 | 停服后使用 | 说明 |
| --- | --- | --- | --- |
| `raylea reset-admin` | 否 | 是 | 重置管理员凭据并重新进入初始化流程，必须在停服窗口执行 |
| `raylea migrate` | 否 | 是 | 受控执行配置或数据库迁移，避免与运行中写入竞争 |
| `raylea restore` | 否 | 是 | 恢复包导入必须在停服状态执行，恢复后再统一进行迁移与兼容检查 |
| `raylea backup` | 是 | 是 | 在线可导出恢复备份；若追求强一致性，建议停服后执行 |
| `raylea doctor` | 是 | 是 | 可在线或停服执行，用于检查运行时与资源完整性 |
| `raylea cleanup` | 是 | 是 | 可在线执行，但必须跳过当前活跃安装 / 渲染任务占用的缓存与临时文件 |
| 常规插件管理 | 否 | 否 | 统一走 Web UI / Web API，不在 CLI 中复制实现 |
| 完整日志浏览 | 否 | 否 | 统一走 Web UI；CLI 最多输出摘要或诊断信息 |

#### 3.12.4 暂不承担的职责

- 常规在线插件管理。
- 完整日志浏览与筛选。
- 独立的实时调试控制台。
- 第二套配置编辑器。
- 远程管理与集群编排。

### 3.13 桌面启动器

#### 3.13.1 定位

- 启动器是 v0.1 的桌面配套能力，不是单独的高级运维平台。
- `windows-x64-full`、`linux-x64-full` 与 `macos-arm64-full` 统一以启动器作为桌面入口。
- `linux-x64-server` 面向无桌面环境、自托管服务进程与 `systemd` 管理场景。
- Linux 端的 server-only 发行包提供 `systemd` 服务模板或安装脚本，适配 LXC / 家庭服务器自托管场景；`nohup` 仅作为临时调试入口。

#### 3.13.2 职责范围

- 启动 / 停止 / 重启 Bot 服务。
- 打开 Web 管理面板。
- 本地环境完整性检查，包括 `.deps/`、Chromium、Python / Node.js 环境以及内置渲染模板资源是否齐备。
- Windows 环境检查中必须覆盖长路径支持状态（`LongPathsEnabled` 注册表项）；若未开启，应在 UI 中高亮 `WARN` 并引导用户开启，否则深层 `node_modules/`、`.venv/` 或插件缓存目录可能触发 `MAX_PATH` 类错误。
- 查看启动失败摘要或极简本地尾部日志。
- 新版本提示与基础离线版本比对。
- Windows 用户文档应明确说明：Launcher、本地 Python / Node.js 环境和 Chromium 浏览环境首次运行时可能触发 Defender / SmartScreen 扫描或误报提示；用户应通过 `release_manifest.json` 与 `SHA256SUMS.txt` 校验正式发行包，而不是被引导去关闭系统安全防护。
- macOS 用户文档应明确说明：目录包默认以未签名归档交付，首次打开前需完成本地校验并按系统提示授予运行许可。

Launcher 与 Server 通信机制：

- Launcher 通过桌面端进程控制层启动 `raylea-server`，并持有该进程句柄用于管理。
- 服务运行状态检测：Launcher 启动服务后轮询 `GET /healthz` 检测服务是否存活，轮询间隔建议 `1-2 秒`。
- 若本机管理入口已存在健康服务，但并非 Launcher 当前持有的子进程，Launcher 应显示“检测到现有服务”，允许用户直接打开管理界面或在明确确认后停止该服务；不得把这种场景误报为 Launcher 已接管的运行中实例。
- 服务启动失败信息获取：Launcher 同时监听子进程 stderr 输出和启动超时；若 `/healthz` 在合理窗口内（如 30 秒）持续不可达，则判定为启动失败，并从 stderr 或日志文件尾部提取错误摘要展示给用户。
- 打开 Web UI：Launcher 统一打开 `http://127.0.0.1:{port}/`；若服务已完成初始化，可先向服务端 `POST /api/session/launcher-token` 获取一次性 Token，并以 `?token={one_time_token}` 作为最佳努力自动登录参数附加到该入口。
- 停止服务：Launcher 应优先调用本机 `POST /api/system/shutdown` 触发进程内优雅停机；若在合理窗口内无响应，再回退到操作系统层的终止 API 强制回收。Windows 平台不得把“发送 `SIGTERM`”视为可靠主路径。

**Launcher 窗口行为规范**：

- Launcher 必须支持最小化到系统托盘（System Tray）。当用户点击窗口关闭按钮（"X"）时，默认行为应是隐藏窗口至托盘区保持后台运行，而不是直接退出程序并终止 Bot 服务进程。
- 仅当用户在托盘图标右键菜单中选择"完全退出"时，才触发本机优雅停机流程并退出 Launcher。
- 首次点击关闭按钮时，应弹出一次性提示告知用户"已最小化到托盘"，避免用户误以为服务已停止。
- 该行为防止用户在自托管挂机场景下习惯性关闭窗口导致服务意外终止或产生孤儿进程。

#### 3.13.3 更新检查边界

- v0.1 只做“检查是否有新版本”。
- v0.1 不做自动覆盖更新。
- 如后续支持自动更新，需要额外考虑插件兼容、失败回滚和运行中升级风险。
- 版本检查由 Launcher 独立实现，不依赖 Web API，保证启动器在服务未启动或离线场景下仍可使用。

#### 3.13.4 暂不承担的职责

- 配置编辑。
- 备份导出。
- 完整日志浏览与筛选。
- 复杂调试编排。
- 远程集群管理。
- 完整插件市场前端。
- 深度系统监控与性能诊断。

### 3.14 兼容性与演进策略

- 早期版本默认不承诺完全向后兼容。
- 对外接口中最需要尽早稳定的是统一事件模型、Capabilities、插件 manifest、`manifest_version` / `plugin_protocol_version` 兼容规则和图片渲染接口。
- Web API 在 v0.1 阶段允许迭代，但要避免无意义改名。
- 配置文件变更应尽量提供自动迁移或默认值补齐。

#### 3.14.1 外部生态定位与互操作

- RayleaBot 选用 OneBot11 作为首个协议标准，这使其在协议层天然与 OneBot 生态中的其他项目（go-cqhttp、LLOneBot、NapCat 等 OneBot11 实现端，以及 NoneBot2、Graia Ariadne 等上层框架）保持互操作性。
- RayleaBot 不追求直接加载或执行其他框架的插件（如 Mirai Console 插件、NoneBot2 插件、Yunzai-Bot JS 插件）。各框架拥有不同的插件模型、生命周期和 API 表面，强行兼容会引入显著复杂度且收益有限。
- 推荐的生态策略是：复用协议层标准（OneBot11，未来可扩展 OneBot12），而不是追求插件层兼容。
- 插件若需与其他机器人框架交互，应通过其外部 API（HTTP、WebSocket 等）经 `http.request` 能力完成，而非内部运行时垫片。
- v0.2+ 可评估适配器级多协议支持作为主要互操作路径（参见 1.4 对多协议的范围排除）。
- 欢迎社区贡献者将其他生态的热门插件逻辑（非运行时）移植到 RayleaBot 插件模型，项目文档应为此提供迁移示例和指引。
- 插件间依赖声明机制评估放在 v0.3+（详见 3.6.2 dependencies 说明），当前版本仅支持语言级包依赖。

#### 3.14.2 LLM / AI 能力扩展方向

- 当前大语言模型（LLM）与 AI Agent 技术（多模型调用、工具使用、多轮对话、RAG 等）是聊天机器人生态的重要演进方向。
- v0.1 不包含任何内置 LLM 集成、Agent 框架或模型管理能力。这是范围收敛的刻意决定，而非架构遗漏。
- RayleaBot v0.1 的现有能力模型已为 LLM 插件提供了必要的构建基础：
  - `http.request`：调用外部 LLM API（OpenAI、Google Gemini、本地 Ollama 等）。
  - `storage.kv` / `storage.file`：管理对话历史和上下文。
  - `event.subscribe`（`message.*`）：接收用户对话。
  - `message.send` / `message.reply`：发送回复。
  - `render.image`：将结构化 LLM 输出渲染为图片卡片。
  - `config.read`：管理 API Key 和模型参数。
- v0.2+ 应根据社区反馈评估是否需要超出插件自行实现的平台级 LLM 能力，例如：
  - 统一模型路由、Token 计量、速率限制和成本追踪的 `llm.completion` 平台能力。
  - 平台级对话上下文管理能力。
  - Web UI 中的模型提供商配置管理。
- 任何 LLM 相关能力均应遵循现有 Capabilities / Permissions 模型（参见 3.6.1、3.6.2），不绕过权限体系。

#### 3.14.3 版本升级策略总则

- Patch 版本（如 `0.1.x`）只允许修复缺陷、补充实现细节、增加非破坏性观测字段或修正文档 / fixture；不得删除字段、改变既有错误码语义或引入需要用户手工改配置的破坏性变化。
- Minor 版本（如 `0.1 -> 0.2`）允许新增向后兼容的能力、字段和任务类型，但必须保持既有契约可继续工作；新增字段默认应可选，新增行为应提供安全默认值。
- Major 版本或等价破坏性升级，才允许删除 / 重命名对外字段、改变核心状态机语义、提升必填项、修改插件协议兼容规则或引入必须人工干预的数据迁移。
- 破坏性升级必须同时提供：迁移说明、契约版本提升、对应 schema / fixture 更新、回滚边界说明和 Release Notes；不得只在代码里“自然演进”。
- `manifest_version`、`plugin_protocol_version`、模板 `template_version` 和配置 / 数据 `schema_version` 的变更，应分别表达各自边界，不得混用一个“版本号”承载所有兼容语义。
- 进入 Beta 前，应把以下边界视为冻结清单，非必要不再轻易改字段名或顶层结构：统一事件模型、消息段模型、插件 `info.json`、插件 JSONL 协议、WebSocket envelope、错误码键名、`config/user.yaml` 关键结构。
- Beta 后若必须调整冻结边界，必须先更新 `contracts/`、迁移说明、Golden Fixtures 和兼容策略，再动实现代码；“先改实现，后补文档”不再是可接受路径。

## 四、交付与工程化

### 4.1 技术栈

- 后端：Go `1.25.8`。
- Web 前端：Node.js `24.14.0`（LTS）+ `pnpm 10.32.1` + Vue `3.5.30` + Vite `8.0.0` + Element Plus `2.13.5` + Vue Router `5.0.3` + Pinia `3.0.4`。
- Python 插件运行时：Python `3.12.13`。
- Node.js 插件运行时：Node.js `24.14.0`（与 Web 构建基线统一）。
- 数据库：SQLite，通过 Go 驱动 `modernc.org/sqlite` 接入。
- 桌面启动器：Electron `41.1.0` + Node.js `24.14.0` + `pnpm 10.32.1` + TypeScript `6.0.2` + React `18.3.1` + Fluent UI React v9 + Vite `8.0.3`，正式交付 `windows-x64`、`linux-x64` 与 `macos-arm64` 桌面版本。
- 图片渲染：v0.1 采用 `chromedp 0.14.2` + Chromium 浏览环境的统一渲染方案。

#### 4.1.1 Go 关键依赖固定选型

| 用途 | 推荐选项 | 说明 |
| --- | --- | --- |
| HTTP 框架 | 标准库 `net/http` + `go-chi/chi` `v5.2.5` | 轻量路由器，兼容标准 `http.Handler` 接口 |
| WebSocket | `github.com/coder/websocket` `v1.8.14` | 延续 `nhooyr` 风格，`context` 语义更自然，维护状态也更明确 |
| SQLite 驱动 | `modernc.org/sqlite` | 纯 Go 实现，无需 CGO，降低交叉编译和分发复杂度；具体 module patch 在建仓时冻结到 `go.mod` |
| 日志 | Go `1.25.8` 标准库 `log/slog` | 结构化日志输出，无需引入第三方日志库 |
| 配置解析 | `gopkg.in/yaml.v3` | 社区标准 YAML 解析库 |
| 浏览器控制 | `chromedp` `v0.14.2` | CDP 协议封装，用于渲染引擎 |

说明：

- v0.1 不再保留 `gorilla/websocket` / `nhooyr.io/websocket` 的二选一，统一收敛到 `github.com/coder/websocket`。
- 优先使用标准库和低依赖库，减少供应链风险和构建复杂度。
- SQLite 驱动的选择直接影响是否需要 CGO；如选用 `mattn/go-sqlite3` 则需要在 CI 和分发环节配置 C 编译工具链。

#### 4.1.2 版本冻结与实现基线

由于规划文档中的实际落地代码可能主要由 AI 协作生成，v0.1 需要尽早把“技术栈方向”收敛为“仓库内固定基线”，避免不同会话、不同阶段生成出彼此不兼容的实现。以下版本以 `2026-03-16` 查询到的官方源为依据，作为 v0.1 建仓基线：

| 领域 | v0.1 固定基线 | 说明 |
| --- | --- | --- |
| Server | Go `1.25.8` | 当前仍在 Go 官方支持窗口内；对刚进入实现的新项目，比刚发布不久的 `1.26.x` 更稳妥 |
| Web / Node 运行时 | Node.js `24.14.0`（LTS）+ `pnpm 10.32.1` | 统一前端构建与 Node.js 插件运行时基线，避免双版本并存 |
| Web UI | Vue `3.5.30` + Vite `8.0.0` + Element Plus `2.13.5` + Vue Router `5.0.3` + Pinia `3.0.4` | 新项目直接锁稳定版本；Vite `8.0.0` 要求 Node `^20.19.0 || >=22.12.0`，与 Node `24.14.0` 兼容 |
| Python 插件运行时 | Python `3.12.13` | 相比 `3.14.x` 更利于第三方依赖兼容、Windows 打包与插件分发稳定性 |
| Go Web / API 依赖 | `go-chi/chi` `v5.2.5` + `coder/websocket` `v1.8.14` | 明确单一依赖选择，减少 AI 生成代码时的分叉实现 |
| SQLite 接入 | `modernc.org/sqlite` | 具体 module patch 在建仓时冻结到 `go.mod`；并在基线文档记录其对应的 SQLite upstream version / source id |
| 渲染 | `chromedp 0.14.2` + Chromium 浏览环境 | 具体 Chromium build、SHA256 和下载来源写入 `.deps/manifest.json` 或等价发布元数据，不只写在 Markdown 中 |
| Launcher | Electron `41.1.0` + TypeScript `6.0.2` + React `18.3.1` + Fluent UI React v9 + Vite `8.0.3` + `electron-builder 26.8.1` | Electron 桌面壳与 Node `24.14.0` / `pnpm 10.32.1` 基线统一，适配桌面目录包交付与 typed IPC 分层 |

补充约束：

- `当前稳定版` 只适用于前期选型讨论；一旦进入 v0.1 正式实现，必须把 Go、Node.js、pnpm、Python、Electron、TypeScript、Vue 主版本等写入仓库中的明确基线文件或工程文件，不再依赖口头约定。
- 进入实现后，4.1.1 中带有“或”的依赖选项必须收敛为单一选择，并同步体现在 `go.mod`、`package.json`、`global.json`、CI 工作流和文档中；不得让 AI 在不同模块中自由选择不同库。
- 应新增一份独立的工程基线文档，例如 `docs/engineering/baseline.md`，记录工具链版本、包管理器、构建入口、目录职责、本地开发命令，以及 `.deps/manifest.json` 中运行环境资源的版本与校验信息，作为 AI 实现时的第一优先上下文之一。
- Web、Server、Launcher 各自只允许一种默认构建入口，例如 `go build`、`pnpm build`；不得同时维护多套等价但不一致的脚手架命令。
- SQLite 上游 `3.52.0` 已在 `2026-03-06` 被撤回，因此在 `modernc.org/sqlite` 的具体 module patch 被确认前，规划文档不应假设 v0.1 会落在 `3.52.0` 线上。
- 如后续升级工具链或关键依赖主版本，必须先更新基线文档、CI 和迁移说明，再允许批量修改实现代码；禁止“代码先漂移，文档后补”。

#### 4.1.3 v0.1 固定工程基线

在 4.1.2 的版本冻结之外，v0.1 还应固定以下工程实现边界，确保 AI 与人工协作时不会在“实现风格”层继续分叉：

- Web UI 固定采用 Vue Router `5.x` 作为唯一路由层；不引入文件路由、双路由栈或自定义路由 DSL。
- Web UI 固定采用 Pinia `3.x` 作为唯一全局状态层；不再额外引入 Vuex、RxJS store 或其他状态框架。
- Web UI 的 HTTP 请求层固定采用浏览器原生 `fetch` + 统一薄封装 API Client；v0.1 不引入 `axios` 作为第二套请求抽象。
- Web UI 的实时通信固定采用浏览器原生 `WebSocket` + 统一重连封装；不引入 Socket.IO 或其他额外实时协议层。
- Web UI 默认采用 Element Plus 组件库 + Vue SFC `lang="scss"` + 全局 CSS Variables 主题变量；v0.1 不引入 Tailwind、CSS-in-JS 或额外原子化样式框架。
- 日志流、任务流和插件 console 视图优先使用轻量自定义滚动流组件；v0.1 不引入重量级终端模拟器作为默认方案，因为当前场景是“受控日志流查看”而非交互式 shell。
- Server 数据访问固定采用 `database/sql` + repository / service 分层 + 手写 SQL；v0.1 不引入 ORM，也不引入会在 SQL 边界之外生成隐式查询的自动化数据访问框架。
- 数据库 schema 迁移固定采用显式 migration 文件 + `raylea migrate` 命令执行，不依赖运行期自动建表或隐式 schema 演化。
- 涉及 WAL、锁竞争、备份恢复和迁移校验的数据库集成测试，固定使用临时 SQLite 文件而不是纯内存数据库，以避免测试语义与生产路径不一致。
- 仓库级 JavaScript 包管理器固定为 `pnpm`；Web 前端与仓库内 JS 工具脚本统一使用 `pnpm`，不允许同仓库同时维护 `npm` / `yarn` / `pnpm` 多套锁文件。
- 平台内置 Node.js 环境插件的依赖安装链路固定为“Node.js 环境 + `npm` 安装器”；原因是 `npm` 随 Node.js 一起分发，更适合作为 Runtime Manager 的最低依赖。官方 Node 插件模板和示例必须兼容该安装链路。
- 平台内置 Node.js 环境插件的运行链路固定由 Runtime 注入 `--max-old-space-size=<limit_mb>`；默认以 `runtime.nodejs_max_old_space_size_mb = 256` 作为单插件 V8 堆上限，避免多个 Node.js 子进程在小内存宿主机上叠加触发 OOM。
- 平台内置 Python 环境插件的依赖安装链路固定为“Python 环境 + 每插件独立 `.venv/` + `pip` 安装”；不采用共享站点包目录，避免插件间依赖污染。
- `.deps/` 中运行环境资源的来源、版本、校验值和适用平台必须写入 `.deps/manifest.json`；Launcher、CLI `doctor` 与发布流程应复用同一份清单，而不是各自维护独立常量。

默认构建与测试命令应一并固定：

- Server：默认构建命令为 `go build ./cmd/raylea-server`，默认测试命令为 `go test ./...`。
- Web：默认安装命令为 `pnpm install --frozen-lockfile`，本地开发命令为 `pnpm dev`，生产构建命令为 `pnpm build`。
- Web 测试：默认单元 / 组件测试工具固定为 Vitest，命令建议统一为 `pnpm test`；默认端到端 / 管理台关键流程测试工具固定为 Playwright，命令建议统一为 `pnpm test:e2e`。
- Launcher：默认安装命令为 `pnpm install --frozen-lockfile`，默认测试命令为 `pnpm test`，默认构建命令为 `pnpm build`。
- CI 应直接复用上述默认命令，不应再为本地开发、CI 门禁、发布构建分别发明三套不同入口。

#### 4.1.4 仓库级强制基线文件

为避免“规划已固定、仓库仍漂移”，v0.1 在进入正式实现后，仓库内至少应存在并维护以下强制文件：

| 路径 | 作用 | 最低要求 |
| --- | --- | --- |
| `docs/engineering/baseline.md` | 人类可读的工程基线说明 | 记录工具链版本、支持平台、默认构建 / 测试命令、仓库目录职责 |
| `server/go.mod` + `server/go.sum` | Go 依赖与 toolchain 锁定 | 固定 Go 版本线与服务端依赖 patch；不得与文档和 CI 漂移 |
| `web/package.json` | Web 工程基线 | 必须写明 `packageManager`、`engines.node`、统一脚本入口 |
| `web/pnpm-lock.yaml` | JS 依赖锁文件 | 作为 Web 与仓库内 JS 工具脚本的唯一锁文件 |
| `launcher/package.json` | Launcher 工程基线 | 固定 `packageManager`、`engines.node`、Electron/Vite/React/Fluent UI 版本与统一脚本入口 |
| `launcher/pnpm-lock.yaml` | Launcher JS 锁文件 | 作为 Launcher 工程唯一锁文件 |
| `contracts/` 目录 | 机器可校验契约根目录 | 必须承载 v0.1 约定的 schema、OpenAPI、错误码和发行 manifest 契约 |
| `.deps/manifest.json` | 运行环境资源清单 | 固定 Chromium、Python / Node.js 环境等资源的版本、来源、SHA256 和适用平台 |

#### 4.1.5 开发环境准备 Checklist

进入正式实现前，开发环境应至少通过以下最小检查：

- Go `1.25.8` 已安装，且 `golangci-lint` 版本与 CI 保持一致。
- Node.js `24.14.0` 与 `pnpm 10.32.1` 已安装；`pnpm -v` 与仓库 `packageManager` 字段一致。
- Python `3.12.13` 可用，且本地可创建 `venv` / `.venv`。
- Node.js `24.14.0` 与 `pnpm 10.32.1` 可用，能在 `launcher/` 执行 Electron 启动器工程命令。
- 本地已具备用于渲染调试的 Chromium 或浏览环境；Web E2E 场景还应准备 Playwright 浏览器二进制。
- 开发机应能执行基线命令：`go test ./...`、`pnpm install --frozen-lockfile`、`pnpm build`、`pnpm test`；若其中任一命令失败，不应开始实现业务代码。

基线文件与开发环境联动规则：

- 缺少以上任一基线文件时，不应视为 v0.1 工程基线已完成收口。
- 文档、工程文件和 CI 之间如出现版本不一致，必须以仓库中的强制基线文件为准并立即修复其余位置。
- 允许通过自动生成补充派生文件，但不得让派生文件反过来成为真实来源。

### 4.2 构建与发布目标

构建目标：

- 一键运行。
- 降低首次部署复杂度。
- 尽量避免用户手动安装 Python / Node.js 运行环境。
- 尽量避免插件各自携带图片渲染环境。

建议构建产物结构：

```plain
RayleaBot-x.y.z-windows-x64
│
├─ raylea-server.exe
├─ RayleaLauncher.exe
├─ web
├─ plugins
│  ├─ builtin
│  ├─ installed
│  └─ dev
├─ config
├─ data
├─ cache
├─ logs
└─ .deps
   └─ manifest.json
```

说明：

- Windows 为首要完整发行目标。
- Linux 保证服务端可运行与可管理，发行物以 `raylea-server` + Web 面板 + `systemd` 模板为主，不交付桌面 Launcher。
- Go 服务尽量保持可移植构建方式，优先使用简单稳定的发布链路。
- `.deps/` 由发行包或启动器准备运行环境和依赖缓存，不作为通用工具链安装目录。
- `.deps/manifest.json` 应记录运行环境资源的版本、下载来源、SHA256 和适用平台，至少覆盖 Chromium 以及后续可能补充的语言环境资源。
- `Plugin Runtime` 作为主服务内的 Runtime Manager 依赖这些受控资源，不负责自举完整环境安装逻辑。
- 渲染引擎应作为受控组件随发行包或首次启动准备，不应要求插件自行携带浏览器内核，也不默认依赖用户系统已安装浏览器。
- `.deps/` 需要提供 Chromium 浏览环境及相关缓存；浏览器运行环境会显著增加发行包体积，具体大小随平台和构建方式变化。
- Chromium 精简运行时通常会为发行包额外带来约 90-150MB 体积增量，具体取决于平台和精简方式。
- 默认发行包建议提供离线可用版本；若浏览器补充文件缺失，可通过 Launcher 的环境检查或 CLI 流程一键补齐。
- Linux 发行物仅包含 `raylea-server` 二进制、`systemd.service` 示例模板和 Web 面板，不包含 `RayleaLauncher.exe`。
- v0.1 内嵌的 Chromium 浏览环境只承诺覆盖 `x86_64 / amd64` 架构；ARM64 环境默认不保证随包提供可用 Chromium。
- 对 ARM64 宿主机（如树莓派或 ARM64 LXC），平台应允许通过 `config/user.yaml` 中的 `render.browser_path` 显式指定宿主机系统 Chromium 路径，作为 `.deps/` 内置运行时的替代方案。

#### 4.2.1 最低硬件建议

为避免用户在极低配置宿主机上获得不可预测体验，首版建议至少给出以下资源基线：

- 最低建议：`1 vCPU / 1.5 GB RAM / 2 GB` 可用磁盘，适合单实例、低插件数量、低频图片渲染场景。
- 推荐配置：`2 vCPU / 2 GB RAM / 5 GB+` 可用磁盘，适合常驻 Chromium、多个平台内置环境插件和较稳定的 Web 管理体验。
- 强烈推荐 `2 GB+ RAM`，否则 Chromium 渲染体验会明显下降。
- Chromium 渲染通常至少需要 `1.2-1.5 GB` 空闲内存，低于此值时渲染成功率和速度会显著下降。
- 当资源低于最低建议时，最先暴露的问题通常会是 Chromium 启动失败、渲染超时、插件冷启动变慢或 SQLite / 日志 I/O 抖动；部署文档应提前明确这一点。

### 4.3 发布包结构规范

发布包结构建议进一步明确为：

| 路径 | 是否默认存在 | 是否允许用户直接编辑 | 说明 |
| --- | --- | --- | --- |
| `config/` | 是 | 是 | 存放默认配置模板和用户可编辑配置 |
| `data/` | 是 | 否 | 存放状态库、迁移记录和 `data/plugins/<plugin_id>/` 等插件持久化业务数据；其中 `data/plugins/<plugin_id>/` 用于插件 KV、文件存储等持久化业务数据，与插件包目录严格分离，便于升级或卸载时保留用户数据（详见 3.5.6） |
| `cache/` | 否 | 否 | 存放 `cache/render/`（渲染结果）、`cache/downloads/`（依赖下载缓存）、`cache/plugins/<plugin_id>/` 等可清理缓存数据 |
| `logs/` | 是 | 否 | 存放运行日志，由程序自动生成和轮转 |
| `plugins/builtin/` | 是 | 否 | 官方内置插件目录，随版本发布 |
| `plugins/installed/` | 是 | 可有限操作 | 用户安装插件目录，Python 插件的独立 `.venv/` 也位于此目录下 |
| `plugins/dev/` | 否 | 是 | 开发者调试插件目录 |
| `.deps/` | 否 | 否 | 平台提供的 Python / Node.js / Chromium 环境及依赖缓存目录 |

原则：

- 程序首次启动时应自动补齐必要目录。
- 用户可编辑目录和程序托管目录必须职责清晰，避免手改导致损坏。
- `data/`、`cache/`、`logs/`、`.deps/` 应视为程序管理目录，而不是常规人工维护目录。
- `plugins/installed/` 既承载插件包，也承载插件私有运行环境，应视为需要长期保留的用户数据目录。

#### 4.3.1 发行 manifest、校验与交付矩阵

v0.1 的发行流程除构建实际压缩包外，还应产出一组可审计的发行元数据，避免“依赖有 manifest、整包没有可信链”：

- 每次正式 Release 必须同时发布 `release_manifest.json` 与 `SHA256SUMS.txt`（或等价 checksum 文件）。
- 每个发行包根目录内应包含 `build_info.json`，用于离线排障与回滚判断；其内容应是对应 release manifest 的子集，而不是另一套独立来源。
- Release 流程在上传资产前，必须校验每个产物的 SHA256 与 `release_manifest.json` 中记录的值一致；不一致则整个发布失败。

`release_manifest.json` 建议最小字段：

| 字段 | 说明 |
| --- | --- |
| `version` | RayleaBot 版本号 |
| `git_commit` | 对应提交哈希 |
| `built_at` | 构建时间（ISO 8601） |
| `config_schema_version` | 平台配置 schema 版本 |
| `db_schema_version` | 平台数据库 schema 版本 |
| `plugin_protocol_version` | 插件协议版本 |
| `artifacts` | 发行产物列表，含 `artifact_id`、`file_name`、`platform`、`sha256`、`size`、`support_level`、`deps_manifest_sha256`、`smoke_profile` |
| `release_notes_ref` | 对应版本说明或 Release URL |

`build_info.json` 建议最小字段：

| 字段 | 说明 |
| --- | --- |
| `version` | 当前包对应的 RayleaBot 版本 |
| `git_commit` | 构建提交哈希 |
| `artifact_id` | 当前包的产物标识 |
| `built_at` | 构建时间 |
| `release_manifest_sha256` | 对应 `release_manifest.json` 的摘要 |

v0.1 首批正式交付矩阵：

| `artifact_id` | 产物形态 | 支持级别 | 最低 smoke profile |
| --- | --- | --- | --- |
| `windows-x64-full` | `zip`，含 `raylea-server.exe`、Electron Launcher、Web UI、官方内置插件、`.deps/` | First-class | `windows_full_smoke` |
| `linux-x64-full` | `tar.gz`，含 `raylea-server`、Electron Launcher、Web UI、官方内置插件、`.deps/` | First-class | `linux_full_smoke` |
| `macos-arm64-full` | `tar.gz`，含 `raylea-server`、Electron Launcher `.app`、Web UI、官方内置插件、`.deps/` | First-class | `macos_full_smoke` |
| `linux-x64-server` | `tar.gz`，含 `raylea-server`、Web UI、官方内置插件、`systemd` 示例文件、运行环境资源 | First-class | `linux_server_smoke` |

说明：

- `macos-arm64-full` 属于正式桌面交付矩阵。
- 回滚与恢复判断必须优先依据 `config_schema_version`、`db_schema_version`、`plugin_protocol_version` 和 `artifact_id`，而不是仅凭包名或版本号猜测兼容性。
- GitHub 自动生成的源代码压缩包不属于 v0.1 受支持的运行时交付产物，不纳入正式交付矩阵。

#### 4.3.2 容器化部署 Volume 挂载规范

当使用 Docker 或其他容器化方式长期运行时，至少应将以下目录映射到宿主机 Volume：

- `config/`：必须挂载，避免容器重建后丢失用户配置。
- `data/`：必须挂载，避免状态库、迁移记录和插件数据丢失。**注意：`data/` 目录包含 SQLite 状态库，必须挂载在本地文件系统（ext4、xfs、NTFS 等）上。严禁将其部署在 NFS、SMB、CIFS 等网络共享存储上——SQLite WAL 模式严重依赖 POSIX 文件锁和共享内存机制，网络文件系统无法正确支持，将导致 `database is locked` 死锁甚至不可逆的数据库文件损坏。**
- `plugins/installed/`：必须挂载，避免已安装插件和 Python 插件私有 `.venv/` 丢失。

建议挂载的目录：

- `logs/`：便于宿主机侧排障和日志留存。
- `cache/`：便于保留渲染缓存和下载中间缓存，减少容器重建后的冷启动成本。
- `.deps/`：如希望避免重复下载运行环境和依赖缓存，可作为可选持久化目录挂载。

通常不建议挂载的目录：

- `plugins/builtin/`：这是发行包随版本提供的官方内置插件，通常应跟随镜像版本更新，而不是作为用户数据卷管理。
- `web/`：属于程序发布内容，不应与运行数据混放。

容器部署原则：

- 容器镜像负责提供程序本体、官方内置插件和受控运行环境。
- Volume 负责保存用户配置、状态数据、已安装插件和可选缓存。
- 容器重建后，只要保留上述必挂载目录，就应能恢复到原有运行状态。
- 容器镜像不应成为定义运行目录结构的事实标准；目录结构仍以本地发行包规范为准。
- **UID/GID 映射**：官方 Docker 镜像必须支持通过 `PUID` 和 `PGID` 环境变量动态映射运行用户。Entrypoint 脚本应在服务启动前先以 root 权限根据环境变量修正 `config/`、`data/`、`logs/` 等挂载卷的属主权限，随后降级到普通用户身份拉起 `raylea-server` 主进程，确保文件系统读写权限开箱即用。未设置 `PUID`/`PGID` 时，默认以镜像内置的非 root 用户运行。

### 4.4 升级与回滚策略

即使 v0.1 只做“检查更新”，也建议先明确以下原则：

- 升级默认不覆盖用户 `config/`、`data/` 和 `plugins/installed/`。
- 升级前应检查配置版本、数据库版本和插件兼容风险。
- 发现不兼容变更时，应给出明确提示，而不是静默升级。
- 如后续支持自动更新，需优先保留回滚入口和失败恢复能力。
- 回滚至少应明确“程序版本回滚”和“用户数据是否兼容”是两个独立问题。

#### 4.4.1 恢复策略

- v0.1 应把恢复视为与备份对等的正式流程，至少支持一条受控恢复路径：停服务 -> 恢复 `config/` 与 `data/` -> 恢复 `plugins/installed/` -> 重新启动 -> 自动迁移与兼容检查。
- 恢复操作应优先通过 `raylea restore` 或等价的受控后端入口执行，不建议在服务运行中手工覆盖目录。
- 若用户要求最强一致性的恢复备份，默认应在停服窗口执行；在线备份只能作为便利能力，不应默认承诺跨目录的严格同一时间点一致性。
- 在线备份如被启用，状态库导出应使用 SQLite 官方 backup API 或等价一致性快照机制；`plugins/installed/` 与其他文件目录仍按近似同一时间点采集处理。
- 恢复包导入后，服务启动时仍必须先执行配置 / 数据库迁移检查、运行时与渲染资源检查，再恢复插件注册信息与协议连接。
- v0.1 主要保证同一主线版本内的恢复，例如 `0.1.x` 之间；跨大版本恢复需要额外迁移说明或明确拒绝。
- 恢复后如发现插件 `runtime_version`、`min_core_version` 或 `data_schema_version` 不兼容，平台应保留插件包与业务数据，但阻止该插件自动启用，并在日志、CLI 与 Web UI 中给出摘要。
- v0.1 不支持跨版本状态库的隐式降级；若用户需要从新版本回退到旧版本，必须依赖升级前导出的恢复备份，而不是直接用旧版本二进制读取新版本 SQLite 文件。

支持矩阵：

| 场景 | 支持级别 | 说明 |
| --- | --- | --- |
| 同平台、同小版本线恢复 | 支持 | 作为 v0.1 首要受控恢复路径 |
| 同平台、跨小版本恢复 | 受控支持 | 需先通过迁移与兼容检查，再决定是否自动启用插件 |
| 跨大版本恢复 | 默认不支持 | 需要额外迁移说明或显式拒绝 |
| 跨平台恢复 | 仅配置与业务数据参考恢复 | `.deps/`、运行环境与二进制插件不保证可直接复用 |

- 恢复完成后，旧的 `admin_sessions` 与一次性 Token 必须全部失效，避免把旧会话直接带回新实例。
- `.deps/`、Chromium、Python / Node.js 运行环境默认允许在恢复后按需重建，不要求随备份包一起恢复。
- 如未来引入二进制插件，跨平台恢复时二进制插件目录可能不可直接复用，平台应保留插件元数据但阻止不兼容产物自动启用。

**恢复后首启流程**：

1. 服务启动后先进入受控的“恢复后检查阶段”，此时本地控制面可准备就绪，但不得在兼容检查完成前直接恢复插件运行。
2. 先校验恢复包 manifest、平台版本线、配置 schema 与状态库 schema；校验失败时进入 `failed` 或 `setup_required` 的受控阻断态，而不是盲目继续启动。
3. 再执行运行时、模板资源、Chromium 浏览环境与插件兼容检查；不兼容的插件应保留包与数据，但默认保持 `disabled` 或 `upgrade_pending_regrant`，不得自动启用。
4. 若 Core 本体、配置和状态库均通过检查，即使存在个别不兼容插件，系统仍可进入可管理的 `running` 或 `degraded`，由管理员后续逐个处理插件问题；不应因为单个插件不兼容而把整个控制面锁死。
5. Web UI、CLI、Launcher 和诊断包都必须能看到同一份“恢复摘要”，至少包含恢复来源、跳过的插件列表、需要人工处理的兼容问题和下一步建议。

### 4.5 Docker 与本地部署

- Dockerfile 和 `docker-compose.yml` 作为附加部署方式保留。
- v0.1 的主要体验目标仍是本地直接运行的桌面 / 自托管包。
- 容器化部署属于补充交付形态，不反向约束 v0.1 的 Windows 本地优先体验与发行目录设计。
- Docker 方案优先服务开发测试和服务器自托管场景，不得反向绑架 Windows 首要发行目标和本地发布路径设计。
- 容器镜像结构、挂载约定和启动脚本都应服从本地发行目录设计，不为 Docker 单独发明第二套配置与数据布局。
- 容器化运行时必须遵守发布包结构中的 Volume 挂载规范，避免容器重建后丢失配置、状态库和已安装插件。
- 对 Windows / macOS 上的 Docker Desktop，必须额外高亮警告：不要把宿主机目录直接 bind mount 到容器内的 `data/` 作为 SQLite 状态库存放位置。Docker Desktop 的 gRPC FUSE / VirtioFS 语义对 SQLite WAL 锁并不可靠，`data/` 至少应使用 Docker Named Volume，避免 `database is locked`、只读锁异常或状态库损坏。
- `docker-compose.yml` 与等价容器示例必须显式暴露时区设置；示例可默认写成 `TZ=Asia/Shanghai`，并要求用户按实际部署地区修改，而不是依赖容器默认 UTC。
- `doctor` 与启动日志在检测到“运行于容器内、未显式配置 `scheduler.timezone`、且 `TZ` 缺失或仍为 UTC”时，应输出明显的 `WARN`，提示定时任务可能按错误本地时间触发。

#### 4.5.1 Linux systemd / LXC 自托管指引

以下指引面向 Linux 自托管场景。`systemd` 服务部署是 v0.1 正式支持的部署路径；LXC / 轻量虚拟化是受支持的部署场景，但以文档指引优先，v0.1 不承诺为特定 LXC 发行版做专门的适配工程或自动化脚本。

- 对 Proxmox VE LXC、普通 Linux 小主机和轻量虚拟化环境，优先推荐“原生 `raylea-server` + `systemd` 服务 + Web 面板”的部署路径，而不是再叠一层 Docker。
- v0.1 应提供 `raylea-server.service` 示例文件或一键安装脚本，至少覆盖 `WorkingDirectory`、环境文件、自动重启、日志输出与启动顺序。
- `systemd` 部署应复用发布包目录结构，确保 `config/`、`data/`、`plugins/installed/`、`cache/`、`logs/` 在宿主机上有稳定路径，便于迁移和诊断。
- 对 LXC 场景，应在文档中明确 Chromium 运行时、字体资源与 `systemd` 服务权限的准备步骤，避免因为轻量容器默认裁剪能力而导致渲染或启动失败。
- 对非特权 LXC 场景，如通过 bind mount 挂载 `data/`、`logs/` 或 `plugins/installed/`，宿主机必须正确配置 `subuid` / `subgid` 映射，或将宿主机目录 owner 调整为容器内对应的偏移 UID / GID；否则 SQLite 可能表现为只读、锁异常或无法创建 `-wal` / `-shm` 文件。
- 如用户计划在 LXC 中启用 Chromium GPU 加速，应先验证 `/dev/dri/renderD128` 或等价设备节点映射、驱动权限和 passthrough 均已正确配置；未完成验证前，保持默认 CPU 渲染更稳妥。
- 对 ARM64 Linux / LXC 场景，如发行包未提供 Chromium 浏览环境，部署文档应明确要求用户先安装宿主机可用的 Chromium，并通过 `render.browser_path` 指向实际可执行文件路径。

### 4.6 Git 与忽略策略

建议忽略以下文件和目录：

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

原则：

- 官方内置插件放在 `plugins/builtin/`，纳入版本控制。
- 用户安装插件、缓存、日志、敏感配置、插件运行依赖不进入版本控制。
- `plugins/dev/` 是否纳入版本控制，可按开发策略单独决定。

### 4.7 GitHub Actions 规划

建议至少划分三类工作流：

- `lint`：静态检查与格式校验。
- `ci`：单元测试、基础集成测试、构建门禁。
- `release`：按 tag 构建发行包并发布 Release。

补充说明：

- Release 流程可以追加自动汇总更新日志。
- 如使用模型生成 Release Notes，应基于仓库机密执行，并作为非阻塞辅助步骤。
- CI 的核心职责是门禁和可构建性验证，不在首版引入过度复杂的自动化。
- `ci` 应显式安装并校验固定技术基线，至少覆盖 Go `1.25.8`、Node.js `24.14.0`、`pnpm 10.32.1`、Python `3.12.13` 与 Electron 启动器工程基线；一旦仓库中的 `go.mod`、`packageManager`、`package.json`、Python 环境配置与工作流声明不一致，CI 应直接失败。
- 版本来源应以仓库内的基线文件和工程文件为准，工作流中尽量避免再手写一套独立版本字符串，减少“文档、CI、工程文件”三处漂移。
- Web UI 的 Playwright 场景在 v0.1 默认进入 `release` 与按需高成本回归层，而不是所有 PR 的必过门禁；PR 层只保留最小 smoke 闭环，避免前端回归门禁过重。

#### 4.7.1 v0.1 CI 门禁矩阵

为把“测试层次应映射到 CI”落实为可执行规则，v0.1 建议使用以下门禁矩阵：

| 工作流 | 主要内容 | 平台 | PR 合并门禁 | 说明 |
| --- | --- | --- | --- | --- |
| `lint` | Go / Web / Launcher 基线校验、静态检查、基础类型检查 | `ubuntu-x64` | 是 | 最快失败层，优先暴露低成本问题 |
| `contracts` | 校验 `contracts/` 下的 OpenAPI、JSON Schema、错误码表、契约与 fixture 同步性 | `ubuntu-x64` | 是 | 契约漂移必须在合并前阻断 |
| `ci-server` | `go test ./...`、Server 构建、事件 / 协议 / 配置 / 迁移 fixtures | `ubuntu-x64`、`windows-x64` | 是 | 核心后端路径必须跨主平台通过 |
| `ci-web` | `pnpm install --frozen-lockfile`、`pnpm build`、Vitest | `ubuntu-x64` | 是 | Web UI 构建与单元测试门禁 |
| `ci-launcher` | `pnpm test`、`pnpm build` | `windows-x64`、`linux-x64`、`macos-arm64` | 是 | Launcher 在三套正式桌面平台上进入门禁 |
| `smoke-pr` | 轻量闭环 smoke：初始化、登录、插件列表、任务流、`/healthz` / `/readyz`、最小渲染验证 | `ubuntu-x64` | 是 | PR 级别只跑最小闭环，不跑最重恢复场景 |
| `release` | 构建正式产物、校验 `release_manifest.json` / checksum、运行交付矩阵 smoke profile 后发布 | `windows-x64`、`linux-x64`、`macos-arm64` | Tag 门禁 | 不通过则不得发布正式 Release |

执行原则：

- `contracts`、`ci-server`、`ci-web`、`ci-launcher`、`smoke-pr` 必须全部通过，PR 才可合并。
- 事件 / 插件协议 / 配置 / 错误码 / 迁移相关 Golden Fixtures 必须进入 `contracts` 或 `ci-server` 的 PR 门禁，而不是只留给高成本回归场景。
- Chromium 重渲染 golden、备份恢复、跨版本迁移等高成本场景优先放入 `release` 与按需回归，避免把所有 PR 门禁拖得过重。
- `macos-arm64` 作为正式桌面交付平台进入 launcher 门禁与 release 门禁。

### 4.8 可观测性与诊断

v0.1 至少建议暴露以下可观测信息：

- 服务运行时长。
- 当前插件总数、启用数、运行数。
- OneBot 当前连接状态。
- 最近错误摘要。
- 最近事件吞吐概览。
- 最近渲染耗时概览。
- 插件崩溃计数与 `dead_letter` 数量。
- 最近 24 小时被静默丢弃的未支持事件类型计数与未知消息段类型计数（便于用户定位"某类通知完全没反应"的问题，无需翻 DEBUG 日志）。

v0.1 还应提供两类极简健康接口：

- `GET /healthz`：只反映进程是否存活，适合 Launcher、`systemd`、Docker / LXC 或外部守护器执行基础活性探测。
- `GET /readyz`：反映服务是否满足 3.4.4 定义的 Ready 条件；当处于 `setup_required`、迁移失败或关键本地资源缺失时，应明确返回非就绪状态。若 Adapter 处于 `idle` 且用户未配置 OneBot11 连接，则返回就绪；若已配置 OneBot11 链路且外部连接暂不可用，则返回就绪并附带 `degraded` 细节。

说明：

- `/readyz` 的语义必须直接复用统一状态模型与启动时序，不应由不同部署形态各自定义“就绪”标准；外部协议链路可用性不应与本地关键资源缺失混为一谈。
- 健康接口应尽量轻量，不承担完整管理或调试职责。

**健康接口 HTTP 语义**：

| 服务状态 | `/healthz` | `/readyz` |
| --- | --- | --- |
| `starting`（启动中） | `200 OK` | `503 Service Unavailable` |
| `running` | `200 OK` | `200 OK`，body 含 `{"status": "ready"}` |
| `degraded`（已配置 OneBot 链路不可用，本地控制面正常） | `200 OK` | `200 OK`，body 含 `{"status": "degraded", "reason": "..."}` |
| `setup_required` | `200 OK` | `503 Service Unavailable`，body 含 `{"status": "setup_required"}` |
| `failed`（启动失败/迁移失败） | `200 OK` | `503 Service Unavailable`，body 含 `{"status": "failed", "reason": "..."}` |
| 进程不可达 | 连接拒绝 | 连接拒绝 |

说明：

- `/healthz` 只要进程存活就返回 `200`，不关心业务状态；适用于 `systemd`、Docker 和 Launcher 的活性探测。
- `/readyz` 区分"就绪可接受流量"和"存活但不可用"：`200` 表示服务可正常接受管理和协议流量，`503` 表示尚未就绪。
- `adapter=idle` 表示当前未配置 OneBot11 连接，本地控制面和管理 API 仍处于就绪状态。
- OneBot11 `auth_failed`、持续重连或外部链路暂不可用都属于 `degraded` 的具体原因，而不是单独把本地控制面判为不就绪；推荐在 `reason_codes` 中使用 `adapter.auth_failed`、`adapter.reconnecting` 等稳定代码。
- `degraded` 返回 `200` 而非 `503`，因为本地控制面和插件管理仍可用；`reason` 字段说明人类可读的退化原因，`reason_codes` 提供稳定枚举给 Web UI、Launcher 和 `doctor` 复用。
- `/healthz` 与 `/readyz` 的返回体都应为 JSON，对外至少统一包含 `status` 字段；推荐附带 `reason`、`reason_codes` 和 `checks`。其中 `checks` 用于暴露 `config`、`database`、`runtime`、`adapter`、`render` 等子检查结果，避免不同入口各自猜测状态。

推荐返回体：

```json
{
  "status": "degraded",
  "reason": "OneBot authentication failed",
  "reason_codes": ["adapter.auth_failed"],
  "checks": {
    "config": "ok",
    "database": "ok",
    "runtime": "ok",
    "adapter": "auth_failed",
    "render": "ok"
  }
}
```

后续可扩展方向：

- 健康检查页。
- 简单指标页。
- 诊断包导出能力。

#### 4.8.1 诊断包规划

建议预留“导出诊断信息”能力，至少包含：

- 程序版本号。
- 平台与运行环境信息。
- 最近日志摘要。
- 插件列表与插件状态。
- 配置摘要。
- 最近错误快照。

该能力可由 Web 面板或 CLI 提供入口，用于测试反馈和用户问题排查。

补充约束：

- 诊断包、错误摘要和 CLI 诊断输出默认不得包含 `secret_store` 明文；如需引用敏感项，只允许使用键名、来源说明或掩码值。

### 4.9 测试策略

建议将测试体系至少拆成以下层次：

- 单元测试：覆盖核心模块、状态机、配置处理、权限判断、渲染调度等逻辑。
- 集成测试：覆盖 OneBot adapter、EventBus、Plugin Runtime、Render Service 等模块协作。
- 协议测试：覆盖 manifest、插件协议、配置 schema、渲染动作输入输出。
- 回归测试：覆盖启动器基础流程、插件启停、手动热重载、开发态文件监听、Web 实时调试控制台、连接恢复和渲染 / 消息发送闭环。
- 验收测试：围绕 v0.1 关键使用场景进行端到端验证。

原则：

- 测试分层应直接映射到 CI 工作流，不应只在文档中存在。
- 插件协议、状态模型和渲染接口属于高返工区，应尽早建立回归用例。

#### 4.9.1 AI 协作实现约束

在“规划文档定义产品边界，代码主要由 AI 逐步生成”的前提下，v0.1 还需要补充以下实现约束：

- 采用“契约优先，代码随后”的顺序：统一事件模型、插件 manifest、插件 IPC 协议、Web API、配置 schema、错误码、备份 manifest 等核心边界，应先形成独立契约文件或结构化文档，再生成实现代码。
- 规划文档、独立契约文档、机器可校验 schema、测试 fixture、实现代码之间必须建立单向优先级：规划与契约高于代码；若代码与契约冲突，应先修改契约与迁移说明，再修改代码，而不是反过来用代码“倒逼”文档。
- AI 实现不得跨层补洞。至少应明确禁止以下捷径：Adapter 直接写状态库、Launcher 复制 Web 业务逻辑、Web UI 通过解析日志推断真实状态、插件运行时直接读写 `config/user.yaml`、插件绕过 Capability 校验直连平台内部模块。
- 对外暴露边界必须优先做成“机器可验证”的定义，而不是只保留在总规划文档里。v0.1 至少应落地以下工件：插件 `info.json` 的 JSON Schema、插件协议消息 schema、Web API 的 OpenAPI 或等价接口清单、配置 schema、错误码枚举表。
- AI 生成的实现若引入新字段、新状态、新错误码、新目录职责或新接口，必须在同一变更中同步更新契约文档、示例和测试；不允许把“后续补文档”作为默认路径。
- 对外接口、状态流转和持久化结构的变更应优先走“文档 / schema -> fixture -> 代码 -> 回归测试”链路；这样才能在多次 AI 接力实现时保持一致性。

#### 4.9.2 契约测试与 Golden Fixtures

考虑到 AI 更容易在边界细节上产生漂移，v0.1 建议补充一组可执行的 Golden Fixtures 作为长期回归基线：

- 将 3.2、3.6、3.7、3.8、3.9、3.10 中的关键示例逐步沉淀为仓库内可运行的 fixture，而不是只保留在 Markdown 中。
- 建议至少维护以下目录：`server/testdata/events/`、`server/testdata/plugin_protocol/`、`server/testdata/render/`、`server/testdata/config/`、`server/testdata/migrations/`、`web/src/mocks/`。
- 统一事件模型应提供“原始 OneBot11 输入 -> 归一化事件输出”的 golden case；插件协议应提供“请求 / 响应 / 错误 / 超时 / 协议违规”的 golden case；渲染应提供“输入 payload -> 结构化结果 / 降级错误”的 golden case。
- 配置迁移、备份恢复、插件安装校验、权限拒绝和健康检查语义都应至少各自保留一组固定样例，防止 AI 在重构时无意改变行为。
- 当文档示例、schema 与 fixture 不一致时，应优先修复为同一套语义后再继续开发；CI 应把这类不一致视为门禁失败，而不是提醒级告警。
- 对高风险边界，建议同时保留“成功样例”和“故障样例”；只有 happy path 的 fixture 不足以约束 AI 生成代码。

#### 4.9.3 变更门禁与完成定义补充

- 任一涉及协议、schema、状态机、配置迁移、数据库结构、插件安装流程、渲染输入输出或 Web API 的变更，合并前必须同时满足“实现代码 + 契约更新 + 测试更新 + 示例更新”四件套。
- 不应把“手工点过一遍能跑”视为 AI 生成代码的完成标准；核心路径至少需要自动化单测 / 集成测 / fixture 校验之一作为证据。
- 对 v0.1 核心闭环中的临时占位实现，应显式标注为 `TODO(v0.2+)` 并禁止进入关键路径；不得以 mock、空实现或仅日志占位冒充已交付能力。
- 每完成一个核心子系统，应追加最小回归清单，确认不会破坏前面已落地的协议、状态模型和权限边界，避免 AI 多轮增量实现时出现“后做的功能覆盖先做的约束”。

#### 4.9.4 契约工件落地清单

为把“契约优先”真正落到仓库中，v0.1 在进入 Beta 前至少应补齐以下正式工件：

| 路径 | 作用 | 最低覆盖范围 |
| --- | --- | --- |
| `contracts/plugin-info.schema.json` | 插件 `info.json` 正式 schema | 插件安装校验、CI 校验、示例插件校验 |
| `contracts/plugin-protocol.schema.json` | 插件 JSONL 协议 schema | `init` / `init_ack`、事件投递、动作请求、结果、错误、关闭语义 |
| `contracts/web-api.openapi.yaml` | HTTP API 正式契约 | `/api/setup/*`、`/api/session/*`、`/api/plugins*`、`/api/config`、`/api/logs`、`/api/tasks*`、`/healthz`、`/readyz` |
| `contracts/websocket-events.yaml` | WebSocket 通道与消息清单 | `/ws/logs`、`/ws/events`、`/ws/tasks`、`/ws/plugins/{id}/console` 的消息 envelope、主动推送事件与关闭原因 |
| `contracts/config.user.schema.json` | `config/user.yaml` 正式 schema | 平台配置、默认值、可热更新项、敏感字段说明 |
| `contracts/error-codes.yaml` | 错误码目录 | 错误码、HTTP / 任务 / UI 映射、默认文案资源键 |
| `contracts/release-manifest.schema.json` | 发行元数据契约 | `release_manifest.json` 与 `build_info.json` 的字段结构 |

规则：

- `contracts/` 是 v0.1 对外接口、schema 和错误码的唯一正式来源；总规划文档、示例代码、前端 mock 和生成代码都只能从它派生，不得反向覆盖它。
- Markdown 章节用于解释设计意图、边界和示例，不作为最终接口裁决依据；若 Markdown 与 `contracts/` 冲突，必须以 `contracts/` 为准，并在同一变更中修正文档说明。
- 任一工件缺失、schema 无法通过校验、或与 fixtures / 代码生成结果不一致，都应视为 CI 门禁失败。
- 若变更影响 HTTP API、WebSocket 消息、manifest、配置或错误码，必须先更新 `contracts/` 下对应文件，再更新实现代码和测试。

#### 4.9.5 故障注入与演练场景

除了功能验收与 Golden Fixtures，v0.1 在进入 Beta 前还应至少完成一轮受控故障演练，覆盖以下场景：

| 场景 | 预期结果 |
| --- | --- |
| OneBot11 心跳断续 / 短暂断连 | 连接状态进入 `reconnecting` 或 `degraded`，本地控制面保持可用，不误报 `running` |
| OneBot11 Token 错误导致 `auth_failed` | 自动停止无意义重连，并在 `/readyz`、Web UI、Launcher 中暴露统一失败摘要 |
| SQLite 启动时被占用或 WAL 锁异常 | 服务拒绝进入就绪态，输出明确错误码和修复建议 |
| 高频写入导致 SQLite 写争用 | 内部高频写路径发生受控退化，不因锁等待把整机拖死 |
| Chromium Worker 卡死或启动失败 | 渲染任务按超时回收，Worker 被重建，`fallback_text` 路径可观测 |
| 渲染队列满载 | 新请求返回 `platform.render_queue_full` 或排队超时错误，不无限阻塞 |
| `pip` / `npm` 安装超时或镜像站卡死 | 后台任务进入 `failed(platform.task_timeout)`，构建锁与临时目录被正确回收 |
| 插件升级后权限扩大 | 平台进入重新授权状态，不沿用旧授权结果静默启用 |
| 插件协议污染 `stdout` 或 `stderr` 洪泛 | Bridge 执行限流 / 截断 / 违规处理，不拖垮主进程 |
| 恢复包版本不兼容 | 恢复后首启进入受控检查阶段，阻止不兼容插件自动启用 |
| WebSocket 会话在任务执行中途过期 | 前端收到 `session_expired` 并能重连；后台任务本身不因会话过期丢失 |
| Webhook 签名校验失败或来源不在白名单 | 请求在网关层被拒绝，不把未经验证的事件投递给插件 |
| `message.reply` 引用目标已撤回 | 平台按策略执行柔性降级或返回结构化错误，而不是静默吞掉消息 |
| Launcher 一次性 Token 过期 | Web UI 回到普通登录页或初始化页，并显示简短提示，不陷入空白或裸 `401` |
| 恢复后存在单插件数据 schema 不兼容 | 系统控制面仍可启动，但该插件保持禁用并在恢复摘要中可见 |

执行原则：

- 上述演练场景应逐步转化为 smoke 或 release 工作流中的可复用自动化用例，而不是只保留在 Markdown 中。
- 故障注入结果必须优先验证“状态是否被正确暴露”和“是否能安全失败”，而不是只验证 happy path 是否能恢复。

### 4.10 文档体系规划

建议后续将文档按用途分层：

- `docs/architecture/`：架构设计、状态模型、事件模型、协议设计。
- `docs/dev/`：开发说明、环境准备、调试流程。
- `docs/plugin/`：插件开发文档、manifest、Capabilities、RPC 协议、渲染接口。
- `docs/plugin/sdk/`：Python / Node.js 官方 SDK 使用说明、`PluginBase`、事件装饰器、能力调用示例。
- `docs/user/`：用户使用文档、部署说明、配置说明。
- `docs/release/`：版本说明、迁移说明、已知问题。

原则：

- 插件协议、SDK 和渲染接口不应只存在于总规划文档中，后续应拆成独立文档维护。
- 用户文档和开发文档应分离，避免混写导致两边都难用。

### 4.11 国际化与文本资源规范

即使 v0.1 主要面向中文用户，也建议尽早建立文本资源规范：

- 平台核心文本应集中到资源表，而不是散落在代码和模板中。
- 渲染模板中的固定文案应支持资源键而不是硬编码文本。
- 插件可自行管理多语言，但平台接口和模板系统应预留国际化能力。
- v0.1 已统一使用资源键和中文默认资源文件，不要求首版完成英文覆盖，但需为后续补充英文保留同一套键结构。

### 4.12 风险与缓解措施

- 渲染队列耗尽或浏览器 OOM：应设置队列长度上限、渲染超时和浏览器重启策略；当触发保护阈值时，优先返回显式错误并允许插件降级输出。
- 插件恶意占用 CPU 或磁盘：依赖 Runtime 的低权限执行、进程组管理、工作目录限制和日志 / 缓存配额；超限后进入告警、限流或手动干预流程。
- SQLite WAL 锁死导致服务卡住：应采用短时重试、状态降级和错误上报，而不是无限阻塞；必要时引导用户先导出备份再修复状态库。
- OneBot 反向 WebSocket 被封导致无限重连：应对重连使用退避策略和最大重试窗口，并在 Web / Launcher 中明确展示“持续重连失败”状态，避免静默空转。
- 升级后插件版本元数据不兼容：应在升级前执行 `manifest_version`、`plugin_protocol_version`、`sdk_min_version`、`min_core_version` 检查，发现不兼容时阻止自动启用并提示用户回滚或升级插件。

### 4.13 v0.1 验收标准（Definition of Done）与质量保障场景

#### 4.13.1 v0.1 验收标准（Definition of Done）

- v0.1 的发布门槛不是“开发者环境能跑起来”，而是按受支持文档完成安装的非开发者用户，能够完成首次初始化、启用至少一个官方或示例插件，并在默认配置下稳定使用一周而不出现不可恢复的数据损坏或管理面失控。

Adapter：

- 能稳定建立 OneBot11 反向 WebSocket 链路，并在配置为未支持传输模式时启动前明确拒绝。
- 鉴权失败时进入 `auth_failed`，连续心跳丢失后进入 `reconnecting` 或 `degraded`，而不是静默卡死。
- 外部链路暂时不可用时，本地控制面仍可进入可管理状态，并通过 `/readyz` 暴露真实就绪语义。

插件运行时：

- 插件必须完成 `init -> init_ack` 握手，初始化失败进入受控退避。
- 插件崩溃后能进入 `backoff` 并在阈值后进入 `dead_letter`，热重载不要求重启主服务。
- 权限授予、撤销、升级重确认和插件来源元数据都可在状态库与审计信息中追溯。

Web / 管理面：

- 能完成首次初始化、登录、查看系统状态、启停插件、查看日志与调试控制台。
- 插件安装必须采用异步任务模型，前端可通过 `/ws/tasks` 实时看到阶段与输出摘要。
- 常规在线管理必须统一走 Web UI / Web API，不与 CLI / Launcher 形成第二套状态源。

渲染：

- 官方内置模板可通过统一渲染引擎输出图片，模板资源缺失返回结构化错误。
- 渲染队列已满时立即返回 `platform.render_queue_full`，而不是无限阻塞。
- 渲染与消息发送是分离动作；存在 `fallback_text` 时可完成明确的文本降级。

运维与恢复：

- `backup`、`restore`、`doctor`、`migrate`、`reset-admin` 至少具备一条受支持的可观测执行路径。
- 恢复流程必须先停服务、再恢复配置和状态、再执行迁移与兼容检查。
- `/healthz`、`/readyz`、诊断包和审计日志能支撑基本自托管排障闭环。

#### 4.13.2 v0.1 关键验收场景

1. 首次启动若不存在管理员账户，系统进入 `setup_required` 引导模式，仅允许本机完成管理员初始化；初始化完成后，用户可继续基础配置并成功通过 OneBot11 反向 WebSocket 建链。
2. 机器人接收到群消息后，可由插件处理并返回回复。
3. 需要图片输出的插件可通过统一渲染服务生成帮助菜单或状态卡片图片，再通过消息能力发送。
4. Web UI 可显示服务状态、插件状态、OneBot 连接状态、最近日志和插件实时终端输出。
5. 管理员可通过 Web UI 启用、禁用、重启和热重载插件。
6. 插件异常退出后，Runtime 可执行指数退避重试，并在达到阈值后进入 `dead_letter`。
7. 启动器可启动 / 停止服务、完成本地环境检查并打开 Web UI。
8. 默认配置下 Web 管理接口仅监听 `127.0.0.1`，WebSocket 与 API 均要求鉴权。
9. 发行包内置或按需准备的 Chromium 浏览环境可独立完成图片渲染，不依赖系统已安装浏览器。
10. 相同模板和相同数据重复渲染时可命中缓存，渲染超时不会卡死插件进程。
11. 插件通过 `render.image` 动作拿到图片路径后，可直接构造 `message.send` 动作发送图片消息，支持 `file://` 或 `base64`，且 Web UI 可预览该图片。
12. `plugins/dev/` 下的源文件或 manifest 变更后，对应插件可自动热重载；`plugins/builtin/` 与 `plugins/installed/` 可在不重启核心服务的前提下完成手动热重载。
13. Web 面板可导出基础诊断信息，用于问题反馈和排障。
14. 在容器化部署场景中，保留 `config/`、`data/`、`plugins/installed/` Volume 后重建容器，系统仍可恢复基础配置、状态数据和已安装插件。
15. 管理员凭据丢失时，可停服务后通过 Launcher 或本地 CLI 触发重置向导（不破坏现有配置与插件数据）。
16. 安装缺失 `info.json`、字段不合法或 `runtime_version` 不满足要求的插件时，系统应明确拒绝安装，不留下半完成目录或脏状态。
17. 当 `.deps/` 中缺失所需运行时、Chromium 或渲染模板资源时，`doctor`、Launcher 和启动日志都应给出明确错误，且服务不得误报为 `running`。
18. 当渲染队列已满并返回 `platform.render_queue_full` 时，插件可观测到结构化错误，并可按 `fallback_text` 正常降级为文本回复。
19. 管理员执行重置向导后，旧的管理会话与一次性 Token 必须全部失效，不能继续访问 Web API 或 WebSocket。
20. 配置或数据库迁移失败时，服务必须拒绝进入 `running`，并在 Launcher、CLI 与 Web 初始化入口中暴露同一份失败摘要。
21. 插件进入 `dead_letter` 后，Web UI、诊断包和状态接口都必须可见该状态，且平台不得在未人工干预前自动恢复该插件。
22. 如用户把 OneBot11 配置成 v0.1 未支持的正向 WebSocket、HTTP 上报或其他传输模式，服务必须在启动前明确拒绝并保持非 `running` 状态。
23. 执行恢复流程时，服务必须要求处于停服窗口；恢复完成后应先执行迁移与兼容检查，再决定是否进入 `running`，不得在目录覆盖后直接跳过校验。
24. 当插件因 `data_schema_version` 升级触发私有数据迁移且迁移失败时，平台必须阻止该插件自动启用，保留原业务数据，并在 Web UI、CLI 与日志中暴露错误摘要。
25. 当服务处于 `setup_required`、迁移失败、关键运行时缺失或渲染资源检查未通过时，`/readyz` 必须明确返回非就绪状态，而不是把服务误报为已就绪。
26. 当插件 ZIP 安装包包含 `../`、绝对路径或其他越界解包条目时，平台必须直接拒绝安装、清理临时目录，并记录明确的 Zip Slip 防御错误摘要。
27. 通过 `POST /api/plugins/install` 安装依赖较重的插件时，接口必须立即返回 `202 Accepted` 与 `task_id`；Web UI 可通过任务流持续看到安装阶段和输出摘要，而不是等待 HTTP 长连接同步完成。
28. 当插件把普通文本写入保留给协议的 `stdout` 时，Runtime 必须把它识别为协议违规并输出明确错误摘要；插件调试输出应默认经 `stderr` 或插件日志接口可见。

29. 配置命令前缀后，Bot Core 可正确将匹配前缀的消息解析为命令并投递给声明处理该命令的插件；不匹配前缀的消息仍然作为普通消息事件分发给订阅了 `message.*` 的插件。
30. 聊天侧超级管理员可使用所有命令，普通用户只能使用权限级别为 `everyone` 的公开命令；被加入黑名单的用户发送的命令不会触发插件处理。
31. 插件通过 `message.reply` 动作回复消息时，消息平台侧可正确显示为对原消息的引用回复。
32. 单用户短时间内高频发送命令时，平台侧冷却机制生效并拒绝后续调用，不会导致插件被淹没。
33. 安装或升级插件时如新增高敏 `required` 权限，平台必须进入重新确认状态，并能在 Web UI 或诊断包中追溯授权审计记录。
34. 局部热更新或局部重连配置应用失败时，平台必须回退到上一份已知可用配置，并对外暴露统一失败摘要。
35. 在线备份若在服务运行中执行，状态库必须使用一致性快照导出；如无法满足强一致性，CLI / Web 必须提示用户停服重试。
36. 多个启用插件声明相同命令名时，Web UI 必须给出冲突告警；保留命令前缀 `raylea:*` 不允许第三方插件占用。

其中 1-10 为核心闭环，11-20 为管理面与运维，21-30 为边界与异常，31-36 为安全与权限。
以上场景作为 v0.1 开发验收 Checklist 使用。
开发过程中建议将以上场景逐步转化为自动化回归测试用例。

#### 文档与设计自检要求

- 目录结构、架构图和子系统描述必须一致。
- v0.1 范围必须能覆盖“接协议、收事件、跑插件、发消息、看日志、改配置、启动服务、渲染图片”的闭环。
- 插件发布形态、Capabilities、manifest、插件协议、渲染接口、Web API、配置和数据库职责不能互相冲突。
- 多协议、插件市场、强沙盒等后续能力不得侵占 v0.1 主线。
- Launcher 与 Web API 的职责边界必须保持单一管理源，不得形成双套逻辑。
- 容器化部署说明必须明确 `config/`、`data/`、`plugins/installed/` 的必挂载要求。
- 进入正式实现后，工具链版本、关键依赖选择、接口契约文件和 Golden Fixtures 必须与总规划文档保持一致，不得各自漂移。

## 五、版本路线图

### v0.1 稳定可用 MVP

- OneBot11 反向 WebSocket 单协议接入。
- 单实例运行。
- Python / Node.js 插件运行时，其中 Python 插件依赖默认安装到插件目录下的独立 `.venv/`。
- 官方 `rayleabot-sdk-python` 与 `rayleabot-sdk-nodejs`，其中 Python SDK 提供同步 / 异步事件处理包装。
- 明确平台内置环境插件、二进制插件、开发者源码插件三类发布形态。
- 统一热重载协议与状态流转，优先保证手动热重载统一可用；`plugins/dev/` 默认启用文件监听热重载，`plugins/builtin/` 与 `plugins/installed/` 保持手动热重载。
- 平台标准 Capabilities 与最小权限模型。
- 平台级图片渲染服务，采用 Chromium + CDP（`chromedp`）实现 HTML/CSS 模板渲染 + PNG 输出，首版默认 `worker_count = 1`，并允许按配置扩展并发 Worker。
- 内置帮助菜单、状态面板、信息卡片、排行榜、提示卡片等基础模板。
- `plugins/builtin/` 下的官方内置帮助、状态、ping、菜单、回显插件和基础示例插件。
- Web 管理面板基础功能。
- Web 实时调试控制台。
- 本地 CLI 工具，至少包含 `reset-admin`、`backup`、`restore`、`doctor`、`migrate`。
- 基础 `/healthz` 与 `/readyz` 健康检查接口。
- SQLite 状态库。
- Electron 桌面启动器。
- 基础 Linux `systemd` / LXC 自托管部署指引。
- 基础日志、配置管理、配置迁移 / 备份与插件生命周期能力。
- 本地优先安全模型。
- 归一化消息段模型（`segments` 数组），统一接收与发送方向的消息结构化表示。
- 命令解析与路由机制，支持可配置前缀、命令名提取和参数解析，匹配后定向投递给声明处理该命令的插件。
- 聊天侧用户权限模型，支持超级管理员、群管理员、普通成员和黑名单四级权限判定。
- 用户侧防刷与冷却机制，支持单用户和单群命令调用频率限制，在 Bot Core 层统一拦截。

### v0.2 管理与运行时完善

- 完善插件生命周期状态同步。
- 完善 `plugins/installed/` / `plugins/builtin/` 的自动化热重载策略、长任务中断与重投、细粒度资源监听规则。
- 增强日志筛选与查询体验。
- 完善配置校验、配置迁移和局部热更新。
- 增加受控的调试事件重放（Event Replay）能力：允许管理员基于 `event_records` 中的历史事件，以显式 `replay` 来源重新注入 `debug adapter` / EventBus 用于开发调试；默认不得伪装成真实生产事件。
- 补充基础调度能力和更多系统状态观测。
- 增加模板实时预览（Live Preview）、渲染主题切换和模板 schema 校验能力。
- 增加诊断包导出和更完整的可观测信息展示。
- 逐步开放受控的自定义模板模式。
- 评估事件处理管线与中间件扩展能力，作为 3.4.2 fan-out 语义之上的可选分发增强（详见 3.4.2 演进方向）。
- 增加 Web 管理端调试聊天面板，支持在不连接 QQ 的情况下模拟发送消息测试机器人响应（详见 3.9.1）。
- 评估 LLM / AI 集成的平台级能力需求，基于社区反馈决定是否引入专用 `llm.*` 能力（详见 3.14.2）。

### v0.3 生态基础设施

- 完善插件依赖管理。
- 引入插件签名校验或可信来源校验。
- 增强插件隔离策略。
- 为插件索引和分发能力准备元数据规范。
- 评估浏览器实例复用、页面池、Render Worker 并发调度等高频渲染优化能力。
- 评估高频图片场景的轻量渲染路径，以及 SVG / Canvas 优化方案。
- 评估插件间依赖声明机制，支持插件声明对其他插件的依赖关系（详见 3.6.2）。
- 补充外部生态互操作文档与插件迁移示例（详见 3.14.1）。

### v0.4 扩展生态阶段

- 评估插件市场或插件索引服务。
- 评估多协议适配能力。
- 评估更强的运行时隔离与资源控制。
- 根据社区反馈收敛对外稳定接口。

## 补充结论

- RayleaBot 首阶段不追求“大而全”，而是优先完成一个能稳定跑起来、能管理、能扩展的机器人基础平台。
- 插件系统是项目的长期核心竞争力，但首版必须收敛范围，优先打通 Python / Node.js 的完整闭环。
- 图片渲染能力应作为平台内建能力统一提供，避免插件各自维护浏览器截图、Canvas 布局和视觉规范。
- 后续最应避免返工的部分，是统一事件模型、插件协议、manifest 结构、渲染接口、配置边界和状态模型。
- 启动器、Web 面板、配置系统和日志系统不是附属功能，而是降低使用门槛和提升可维护性的关键组成部分。
