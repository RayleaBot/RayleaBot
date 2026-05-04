# RayleaBot v0.2 执行计划

> `docs/execution-plan-v0.2.md` 记录 v0.2 已完成基线与收口结果。  
> `docs/execution-plan.md` 保留 v0.1 已完成基线与历史对照。  
> 本文档记录 v0.2 的正式范围、完成结果和延后边界，不重复展开 v0.1 已完成项。
>
> 状态图例：`☑️ 已完成` · `❌ 延后到 v0.3+`

---

## 一、总览

### v0.2 主线目标

- 补齐 OneBot11 完整可用面，覆盖 reverse WebSocket、forward WebSocket、HTTP 调用与 webhook 上报。
- 补齐 OneBot11 核心能力与 NapCat、幸运莉莉娅扩展兼容矩阵。
- 扩展插件协议与 SDK，使更宽 `action family` 与更宽消息段进入正式主链。
- 增加在线模板编辑器、模板可视化预览与更强管理面可视化能力。
- 继续完成生命周期状态同步、配置迁移、局部热更新、诊断与恢复闭环。

### 前置承接

v0.1 已提供单实例基线、基础 OneBot11 reverse WebSocket、插件运行时、管理面、渲染服务、恢复与发布基线。v0.2 以这些既有能力为前提，直接进入补齐、扩展与收口。

当前已完成：

- ☑️ Pre-Phase 已收口
- ☑️ Phase 1 已完成 Batch A = OneBot 主链冻结
- ☑️ Phase 1 已完成 Batch B = 模板编辑器、治理读取面与插件 metadata 冻结
- ☑️ Phase 1 已完成 Batch C = 生命周期 `display_state` 与配置保存 `apply_effects` 冻结
- ☑️ Phase 1 已完成 Batch D = Plugin Protocol 与 release metadata 收口
- ☑️ Phase 2 已完成 Batch A = OneBot 主链 companion updates
- ☑️ Phase 2 已完成 Batch B = 模板编辑器、治理与插件 metadata companion updates
- ☑️ Phase 2 已完成 Batch C = 生命周期与配置 companion updates
- ☑️ Phase 2 已完成 Batch D = Plugin Protocol 与 release metadata companion updates
- ☑️ Phase 3 已完成 transport 主链收口与协议异常可见性补齐
- ☑️ Phase 4 已完成 OneBot11 核心事件、消息段、历史 / 详情读取与兼容矩阵收口
- ☑️ Phase 5 已完成 Plugin Protocol / Wider Action Family 收口
- ☑️ Phase 6 已完成在线模板编辑器 / Render 可视化
- ☑️ Phase 7 已完成 Plugin Platform / Manifest / Config / Governance 收口
- ☑️ Phase 8 已完成管理面跨页钻取、诊断入口与 Web 基线收口
- ☑️ Phase 9 已完成状态模型拆分、环境检查收口、WebSocket 事件驱动与深链诊断引导
- ☑️ Phase 10 已完成 Release / Deployment / Quality Gates 收口

### v0.2 正式范围

- 在线模板编辑器
- 更强的可视化管理与编辑体验
- Web 管理面技术栈迁移（Ant Design Vue + Vue Vben Admin 对齐）
- 更宽 `action family`
- OneBot11 剩余兼容面
- OneBot11 正向 WebSocket、HTTP、Webhook
- NapCat 扩展兼容
- 幸运莉莉娅扩展兼容
- 文档中定义的用户侧关键能力

### 延后边界

- 插件市场与远程分发平台
- 强沙盒与更强资源隔离
- 插件间依赖解析
- 自动覆盖更新
- 非 OneBot 生态的多协议扩展

### 总阶段表

| 阶段 | 名称 | 状态 | 当前目标 |
| --- | --- | --- | --- |
| Pre-Phase | 范围重置与前置承接 | ☑️ | v0.2 范围与延后边界已固定，并作为已完成基线保留 |
| Phase 1 | Contract / Schema 冻结 | ☑️ | Batch A、Batch B、Batch C、Batch D 已完成，v0.2 正式边界已冻结 |
| Phase 2 | Fixtures / Examples / SDK | ☑️ | Batch A、Batch B、Batch C、Batch D 已完成，已冻结边界的 companion updates 已补齐 |
| Phase 3 | OneBot11 传输模式补齐 | ☑️ | reverse WS、forward WS、HTTP、webhook 四条接入链路、协议快照与 transport 异常可见性已收口 |
| Phase 4 | OneBot11 事件与消息兼容补齐 | ☑️ | 核心事件、消息段、历史消息、详情读取与 provider 扩展兼容矩阵已进入正式边界与管理面 |
| Phase 5 | Plugin Protocol / Wider Action Family | ☑️ | capability 名称、action kind、运行时授权、SDK helper、示例与文档口径已统一 |
| Phase 6 | 在线模板编辑器 / Render 可视化 | ☑️ | 模板列表、源码编辑、校验、手动预览、保存、版本历史、回退与渲染结果可视化已进入真实链路 |
| Phase 7 | Plugin Platform / Manifest / Config / Governance | ☑️ | rich plugin detail、治理读取面、插件授权重确认与 `config.migrate` task-only 边界已收口 |
| Phase 8 | Diagnostics / Web API / Web UI | ☑️ | 协议中心、日志中心、任务、插件、模板编辑器和指令中心的跨页钻取、诊断入口与前端工作区基线已收口 |
| Phase 9 | Launcher / 本地运维入口 | ☑️ | 状态模型已拆分、环境检查已收口为本地预检、Web 状态刷新已接入 WebSocket 事件驱动，深链与诊断引导已完成 |
| Phase 10 | Release / Deployment / Quality Gates | ☑️ | PR 门禁、打包 smoke、恢复演练和自托管巡检已覆盖 v0.2 transport、compatibility、template editor 与 wider actions 回归 |

### 当前边界说明

| 边界 | 当前说明 |
| --- | --- |
| OneBot 兼容矩阵读取面 | 正式管理面包含 `GET /api/protocols/onebot11/compatibility`，协议中心提供 `/protocols/compatibility` 子页展示 `events`、`message_segments`、`read_capabilities`、`provider_extensions` |
| LuckyLillia 的 HTTP / SSE 接收兼容说明 | 当前正式 transport 只包含 `reverse_ws`、`forward_ws`、`http_api` 和 `webhook`；`SSE` 不在正式配置模型、协议快照枚举和协议中心范围内 |
| OneBot companion updates | 协议快照、兼容矩阵、fixtures、examples、tests、管理面与类型生成保持一致 |

---

## 二、Pre-Phase — 范围重置与前置承接 ☑️

| 任务项 | 状态 | 说明 |
| --- | --- | --- |
| v0.2 文档主入口收口 | ☑️ | `docs/execution-plan-v0.2.md` 作为 v0.2 已完成基线保留，`docs/execution-plan.md` 作为 v0.1 基线参考 |
| v0.1 基线承接 | ☑️ | 单实例、基础 OneBot11 reverse WebSocket、插件运行时、管理面、渲染与恢复链路作为本轮前提 |
| v0.2 范围冻结 | ☑️ | 在线模板编辑器、可视化、更宽 action family、OneBot11 全传输模式、NapCat 与幸运莉莉娅扩展兼容纳入本轮 |
| 延后边界冻结 | ☑️ | 插件市场、强沙盒、插件间依赖、自动覆盖更新、非 OneBot 生态多协议继续后置 |

---

## 三、Phase 1 — Contract / Schema 冻结

### 当前批次

- ☑️ Batch A = OneBot 主链已完成
- ☑️ Batch B = 模板编辑器、治理读取面、manifest 元数据已完成
- ☑️ Batch C = 生命周期 `display_state` 与配置保存 `apply_effects` 已完成
- ☑️ Batch D = Plugin Protocol 与 release metadata 已完成

### 正式 contract 边界

| 正式边界 | 状态 | 本轮冻结方向 |
| --- | --- | --- |
| `contracts/web-api.openapi.yaml` | ☑️ | OneBot transport、协议快照、日志详情、模板编辑器、治理核心读取面、plugin `display_state` 正式枚举与 `PUT /api/config` `apply_effects` 已冻结 |
| `contracts/websocket-events.yaml` | ☑️ | 协议状态、日志主链与插件生命周期 `display_state` 已补齐；模板编辑器预览继续复用现有 task event 链，不新增预览专用事件 |
| `contracts/plugin-info.schema.json` | ☑️ | `default_config`、`role`、`icon`、`repo`、`homepage`、`keywords`、`screenshots`、`platforms`、`system_dependencies`、`concurrency` |
| `contracts/plugin-protocol.schema.json` | ☑️ | OneBot 主链所需更宽 `action`、消息段与共享 `error.details` 语义已冻结 |
| `contracts/config.user.schema.json` | ☑️ | OneBot 多 transport 配置主模型已冻结；治理核心读取面直接投影现有 cooldown 与默认权限配置，迁移与热更新读取口径已进入正式范围 |
| `contracts/error-codes.yaml` | ☑️ | OneBot transport、compatibility、provider extension 与模板编辑相关错误码已冻结 |
| `contracts/release-manifest.schema.json` | ☑️ | `release_manifest.json` 与 `build_info.json` 的最小 metadata surface 已冻结，`SHA256SUMS.txt` 保持在 release tool 校验边界 |

### 本轮 contract 冻结清单

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| OneBot11 传输模式 | ☑️ | Batch A 已冻结 reverse WebSocket、forward WebSocket、HTTP 调用、webhook 上报，以及幸运莉莉娅 HTTP / SSE 接收兼容说明 |
| OneBot11 兼容矩阵 | ☑️ | Batch A 已冻结核心事件、消息段、动作族、历史消息、消息详情、转发消息、文件与 provider 扩展矩阵 |
| Plugin Protocol 扩展 | ☑️ | Batch A 与 Batch D 已冻结更宽 `action family`、更宽消息段、共享 `error` 帧语义与结构化错误 |
| 在线模板编辑器 | ☑️ | 模板列表、详情、源码编辑、schema 校验、任务式预览、保存、历史版本与回退已进入正式边界 |
| 治理与配置语义 | ☑️ | blacklist / command-policy 读取面、plugin `display_state` 正式枚举与 `PUT /api/config` `apply_effects` 已进入正式边界 |
| Release Metadata | ☑️ | `release_manifest.json`、`build_info.json` 与 release 校验边界已进入正式范围 |

### 本轮排除项

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 插件市场与远程分发平台 | ❌ | 保持在 v0.3+ |
| 强沙盒与更强资源隔离 | ❌ | 保持在 v0.3+ |
| 插件间依赖解析 | ❌ | 保持在 v0.3+ |
| 自动覆盖更新 | ❌ | 保持在 v0.3+ |
| 非 OneBot 生态多协议扩展 | ❌ | Satori、Milky 等协议不进入 v0.2 |

---

## 四、Phase 2 — Fixtures / Examples / SDK

### 当前批次

- ☑️ Batch A = OneBot 主链已完成
- ☑️ 当前已补齐 protocol snapshot、compatibility matrix、widened actions、provider namespace、SDK helper 与管理面 companion updates
- ☑️ Batch B 已补齐模板编辑器、治理与 manifest 相关 fixtures / examples / docs
- ☑️ Batch C 已补齐生命周期 `display_state`、配置保存 `apply_effects` 与管理面 companion updates
- ☑️ Batch D 已补齐 Plugin Protocol 与 release metadata companion updates

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| transport / event / action fixtures | ☑️ | OneBot 主链、模板编辑器、治理 surface、lifecycle `display_state`、config `apply_effects`、共享 `error` 帧语义与 release metadata fixtures 已补齐 |
| OneBot11 provider 分层样例 | ☑️ | fixtures 已按标准 OneBot11、NapCat 扩展、幸运莉莉娅扩展三层组织 |
| examples 同步补齐 | ☑️ | OneBot 主链、模板编辑器、治理、config `apply_effects`、release metadata 与共享错误语义示例已补齐 |
| SDK 示例与文档同步 | ☑️ | OneBot 主链已覆盖 widened actions、消息段、provider namespace 与 `ActionError.details` 用法 |
| Golden 回归基线 | ☑️ | OneBot 主链、template editor、治理、lifecycle/config、release metadata 与共享错误语义回归样例已建立 |

---

## 五、Phase 3 — OneBot11 传输模式补齐 ☑️

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| reverse WebSocket 收口 | ☑️ | 回连入口、鉴权、ready、degraded、reconnect 与错误摘要已进入统一协议快照 |
| forward WebSocket | ☑️ | 主动连接主链、错误摘要与管理面可见性已进入正式运行路径 |
| HTTP API 调用 | ☑️ | HTTP 调用主链与 WS 模式共享鉴权、错误与状态语义 |
| webhook 事件上报 | ☑️ | webhook 接入、transport 状态与协议异常摘要已进入正式运行路径 |
| 幸运莉莉娅 HTTP / SSE 兼容 | ☑️ | 兼容说明固定在 provider 级边界，`SSE` 不进入正式 transport 枚举、配置模型或协议中心 |
| 单实例约束 | ☑️ | 单实例、单活跃 OneBot 连接模型继续作为正式边界 |

---

## 六、Phase 4 — OneBot11 事件与消息兼容补齐 ☑️

### 核心事件兼容

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| message / notice / request / meta 完整矩阵 | ☑️ | friend、group request、recall、admin、ban、poke、like、essence、upload、flash_file、heartbeat 与 lifecycle 已进入正式事件集合 |
| 历史与详情读取 | ☑️ | `message.get`、`message.history.get`、`message.forward.get`、`file.get` 等读取能力已进入正式边界与回归样例 |
| provider 扩展事件 | ☑️ | NapCat 与幸运莉莉娅已公开的 OneBot 扩展事件进入正式兼容矩阵 |
| 不兼容项明示 | ☑️ | provider 级兼容矩阵固定返回 `supported` 或 `unsupported` |

### 消息段兼容矩阵

| 类型组 | 状态 | 范围 |
| --- | --- | --- |
| 基础消息段 | ☑️ | `text`、`image`、`at`、`at_all`、`reply`、`face` |
| 媒体与文件 | ☑️ | `record`、`video`、`file`、`flash_file` |
| 富文本与卡片 | ☑️ | `json`、`xml`、`markdown`、`music`、`contact` |
| 组合与转发 | ☑️ | `forward`、`node` |
| 互动消息段 | ☑️ | `poke`、`dice`、`rps` |
| provider 扩展消息段 | ☑️ | `mface`、`keyboard`、`shake` 进入正式消息段集合，并在兼容矩阵中区分 provider 支持状态 |

### 本轮要求

- 管理面通过协议中心子页展示当前 provider、当前 transport 与当前能力覆盖情况。
- 未支持能力在兼容矩阵中明确显示为 `unsupported`。

---

## 七、Phase 5 — Plugin Protocol / Wider Action Family ☑️

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| capability 与 action kind 对齐 | ☑️ | `capabilities`、`permissions.required`、`permissions.optional` 与正式 OneBot / provider 动作名保持一一对应 |
| 运行时授权校验 | ☑️ | wider action family、provider action、`message.send` 与 `message.reply` 在执行前统一走 grant 检查 |
| SDK helper 完整覆盖 | ☑️ | Python / Node.js SDK 覆盖当前正式 OneBot 单动作、3 个 provider 扩展动作与完整消息段 builder |
| 类型与模型同步 | ☑️ | `flash_file`、`meta_event_type`、`interval`、`status` 与声明文件保持一致 |
| companion updates | ☑️ | fixtures、示例、文档与执行计划口径一致 |

### 当前结果

- capability 粒度固定为单动作能力
- generic fallback 继续保留 `onebotAction` / `providerAction`
- Python / Node.js 继续作为正式托管运行时

### 本轮排除项

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 复杂流式协议 | ❌ | 批量消息、流式回传与独立调试流不进入 v0.2 |
| 插件间依赖 | ❌ | 保持在 v0.3+ |
| 额外托管运行时语言 | ❌ | Go / Rust 官方托管运行时不进入本轮 |

---

## 八、Phase 6 — 在线模板编辑器 / Render 可视化 ☑️

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 模板列表与详情 | ☑️ | 管理面提供模板列表、模板详情、当前版本、最后校验状态和版本历史 |
| 源码编辑 | ☑️ | 浏览器内编辑 `manifest_json`、`html`、`stylesheet` 与 `input_schema_json` |
| schema 校验 | ☑️ | 保存前与预览前都可执行本地 JSON 解析和服务端结构校验 |
| 手动预览 | ☑️ | 基于当前版本或未保存草稿发起 `render.preview` 任务式预览 |
| 保存与版本回退 | ☑️ | 提供保存、历史版本查看与回退能力，使用 `base_revision_id` 防止静默覆盖 |
| 输入结构可视化 | ☑️ | 展示模板输入结构、字段说明、必填状态和层级信息 |
| 渲染结果可视化 | ☑️ | 展示 artifact、缓存命中、任务状态、图片结果和任务详情入口 |
| 错误定位可视化 | ☑️ | 将本地解析错误、服务端校验错误与预览任务错误统一显示在管理面 |

### 本轮排除项

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 拖拽式模板搭建器 | ❌ | 不进入本轮 |
| 模板市场与远程发布 | ❌ | 不进入本轮 |

---

## 九、Phase 7 — Plugin Platform / Manifest / Config / Governance ☑️

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 生命周期状态同步 | ☑️ | `/api/plugins`、`/api/plugins/{plugin_id}`、enable / disable / reload 响应与 `/ws/events` 插件生命周期分支统一使用正式 `display_state` 枚举 |
| 配置迁移 | ☑️ | `PUT /api/config` 返回正式 `apply_effects` 分类；`config.migrate` 保持现有 task 类型，不提供独立管理路由 |
| manifest 元数据补齐 | ☑️ | rich plugin detail 覆盖 `author`、`license`、`sdk_min_version`、`runtime_version`、`icon`、`repo`、`homepage`、`keywords`、`screenshots`、`system_dependencies` |
| `default_config` / `concurrency` 正式化 | ☑️ | 插件详情页展示 `default_config`、`concurrency`、`declared_capabilities`、`dependencies` 与 `scopes` |
| blacklist / cooldown / command permission 可见性 | ☑️ | 指令中心展示默认权限、冷却配置、黑名单与当前生效命令策略 |
| 插件升级与重确认 | ☑️ | `plugin.permission_pending` 的 `missing_capabilities` 与 `scope_changed` 都可在管理面完成授权后继续启用 |

---

## 十、Phase 8 — Diagnostics / Web API / Web UI ☑️

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 协议 transport 可视化 | ☑️ | 协议中心展示当前 provider、当前状态、连接设置、传输摘要、兼容矩阵入口与相关日志入口 |
| OneBot 兼容矩阵可视化 | ☑️ | 协议中心提供 `/protocols/compatibility` 子页，按 `events`、`message_segments`、`read_capabilities`、`provider_extensions` 展示兼容矩阵 |
| 前端技术栈迁移 | ☑️ | Web 工程基线固定为 Ant Design Vue + Vue Vben Admin 对齐方案，继续保持单应用结构与既有 formal contract |
| 模板编辑与预览界面 | ☑️ | 系统分组提供 `/render/templates/:templateId?` 工作区，集成草稿编辑、校验、任务预览和版本回退 |
| 协议中心增强 | ☑️ | 协议中心、兼容矩阵、日志中心与仪表盘之间提供稳定入口，协议异常可直接进入相关页面 |
| 任务 / 日志 / 协议 / 插件联动钻取 | ☑️ | 命令、插件、协议、日志、任务和模板编辑器之间的跳转统一使用稳定标识与工作区 query |
| 诊断与恢复增强 | ☑️ | 仪表盘、协议中心、日志详情与任务详情统一复用 readiness、transport issue、recovery summary 与结构化详情摘要 |

补充约束：

- 前端技术栈迁移不改变 `contracts/`、OpenAPI、WebSocket 事件、错误码或配置 schema。
- Vben 对齐方案在现有 `web/` 单应用内实施，不扩展为官方整仓 `monorepo` / `turbo` 结构。

---

## 十一、Phase 9 — Launcher / 本地运维入口 ☑️

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 启动 / 停止 / 健康检查 | ☑️ | 启动失败识别、已有服务探测与端口占用识别已补强 |
| 环境检查与资源诊断 | ☑️ | Launcher 本地预检只覆盖安装目录、设置、server 可执行文件、配置与工作目录可写性；运行时资源与深层诊断由服务端 readiness 与 diagnostics 统一裁决 |
| 打开 Web 管理面 | ☑️ | 作为桌面入口打开 Web 主界面，管理会话由 Web 初始化和登录流程建立 |
| Web 页面深链 | ☑️ | 支持打开协议中心、模板编辑器、任务详情等指定 Web 页面 |
| 错误提示与恢复引导 | ☑️ | 启动失败、端口冲突、现有服务探测与恢复摘要可直接区分并展示；环境引导由服务端诊断驱动 |

Launcher 与 Web 边界收口细节见 `docs/engineering/launcher-web-boundary-refactor-plan.md`。

### 本轮排除项

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 协议中心桌面版平行业务界面 | ❌ | Launcher 不复制 Web 协议中心 |
| 模板编辑器桌面版重复实现 | ❌ | Launcher 不复制 Web 模板编辑器 |
| 命令中心 / 插件管理 / 配置管理桌面版 | ❌ | Launcher 不复制 Web 已有业务功能 |

---

## 十二、Phase 10 — Release / Deployment / Quality Gates ☑️

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| transport matrix 门禁 | ☑️ | 发布与自托管 smoke 会探测 packaged `/api/protocols/onebot11`，校验 `reverse_ws`、`forward_ws`、`http_api`、`webhook` 四条 transport、provider、readiness 和摘要 |
| compatibility matrix 门禁 | ☑️ | 发布与自托管 smoke 会探测 packaged `/api/protocols/onebot11/compatibility`，校验 `events`、`message_segments`、`read_capabilities`、`provider_extensions` 四类矩阵与代表项 |
| template editor 门禁 | ☑️ | 发布与自托管 smoke 覆盖模板列表、源码、校验、预览、artifact、保存、版本历史、回退与重启后 revision 持久化 |
| wider action family 门禁 | ☑️ | PR 门禁包含 Node / Python SDK 测试、Node SDK `dist` 漂移检查，server / web 继续通过既有测试覆盖 wider action family |
| self-host upgrade / rollback | ☑️ | release workflow 继续执行 packaged recovery drill 和长期自托管 smoke，同步验证 v0.2 协议与模板能力 |
| 发布元数据与校验门禁 | ☑️ | `release_manifest.json`、`build_info.json`、`SHA256SUMS.txt` 继续进入正式校验，`onebot_matrix` 保持可选 metadata |

### 本轮排除项

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 自动覆盖更新 | ❌ | 保持在 v0.3+ |

---

## 十三、测试 & CI 目标

### 默认验证命令

#### Server

- `go test ./...`
- `go build ./cmd/raylea-server`

#### Web

- `pnpm test`
- `pnpm build`
- `pnpm test:e2e`

#### Launcher

- `pnpm test`
- `pnpm build`

### CI 分层

| 层次 | 目标 | 说明 |
| --- | --- | --- |
| PR 轻量门禁 | contracts / fixtures / examples 触发的类型生成漂移、SDK、Server / Web / Launcher 核心检查 | 保证可合并性 |
| 发布门禁 | transport matrix、compatibility matrix、template editor、wider action family、packaged recovery、self-host smoke | 保证可交付性 |
| 手动高成本回归 | provider extension 深回归、大样本消息段 / 文件 / 历史消息矩阵、长时间协议稳定性巡检 | 保留独立回归入口 |

### v0.2 核心验收场景

| 场景 | 验收目标 |
| --- | --- |
| OneBot11 全传输模式 | reverse WS、forward WS、HTTP、webhook 四种接入路径都能建立受控链路 |
| Provider 扩展兼容 | NapCat 与幸运莉莉娅至少形成可核验的兼容矩阵与回归样例 |
| OneBot11 事件与消息兼容 | 核心事件、消息段、历史消息与详情读取进入正式兼容矩阵与回归范围 |
| Wider Action Family | plugin protocol、SDK、fixtures、examples、运行链路保持一致 |
| 在线模板编辑器 | 支持编辑、校验、预览、保存和回退 |
| 生命周期 / 配置 / 诊断 | `display_state` 与 `apply_effects` 保持跨 surface 一致，诊断与恢复继续完成原 v0.2 目标 |
| 治理读取面 | blacklist / cooldown / permission 的剩余管理可见性在 Web 管理面可验证 |
| 管理面联动 | 协议中心、日志、任务、模板编辑器、指令中心之间的跳转与摘要口径一致 |
| Launcher 职责 | Launcher 只验证本地壳职责、诊断深链与打开 Web，不承担 Web 业务回归 |
| 发布与回滚 | packaged smoke、recovery drill 与长期自托管巡检覆盖 transport、compatibility、template editor、wider action family 与 release metadata 校验 |

### Companion updates 原则

- 任何涉及协议、schema、状态、配置、错误码、插件协议、模板编辑、迁移与发布边界的改动，都需要同步更新实现、契约、测试、示例与文档。
- 默认验证命令继续沿用现有入口，不建立第二套本地、CI 或发布命令。

---

## 十四、下一轮规划

### v0.2 收口后的延后能力

- 插件市场与远程分发平台
- 强沙盒与更强资源隔离
- 插件间依赖解析
- 自动覆盖更新
- 非 OneBot 生态的多协议扩展

### 长期边界

- v0.2 结束后，继续优先稳定 OneBot11 兼容矩阵、插件协议、manifest、能力授权、配置迁移、渲染资源诊断与回归门禁。
- 后续扩展继续遵守 `contracts/` 为正式来源与 companion updates 四件套。
