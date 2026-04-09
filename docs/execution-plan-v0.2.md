# RayleaBot v0.2 执行计划

> `docs/execution-plan-v0.2.md` 作为当前最新执行计划使用。  
> `docs/execution-plan.md` 保留 v0.1 已完成基线与历史对照。  
> 本文档只记录 v0.2 仍需执行的内容，不重复展开 v0.1 已完成项。
>
> 状态图例：`☑️ 已完成` · `◐ 部分完成` · `⚠️ 需先 contract-first` · `🟡 v0.2 本轮待实施` · `❌ 延后到 v0.3+`

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
- ◐ Phase 1 已完成 Batch A = OneBot 主链冻结
- ◐ Phase 2 已完成 Batch A = OneBot 主链 companion updates
- ◐ Phase 8 已完成协议中心连接设置、协议日志终端流、日志详情与部分管理面联动
- ◐ Phase 9 已完成启动器启动失败诊断补强与端口占用识别

### 本轮明确纳入

- 在线模板编辑器
- 更强的可视化管理与编辑体验
- 更宽 `action family`
- OneBot11 剩余兼容面
- OneBot11 正向 WebSocket、HTTP、Webhook
- NapCat 扩展兼容
- 幸运莉莉娅扩展兼容
- 文档中已写明、但尚未进入当前 v0.2 主线的用户侧关键能力

### 本轮明确延后

- 插件市场与远程分发平台
- 强沙盒与更强资源隔离
- 插件间依赖解析
- 自动覆盖更新
- 非 OneBot 生态的多协议扩展

### 总阶段表

| 阶段 | 名称 | 状态 | 当前目标 |
| --- | --- | --- | --- |
| Pre-Phase | 范围重置与前置承接 | ☑️ | v0.2 已作为当前执行计划收口，范围与延后边界已固定 |
| Phase 1 | Contract / Schema 冻结 | ◐ | Batch A 已完成 OneBot 主链冻结，模板编辑器、治理读取面、manifest 元数据仍待继续 |
| Phase 2 | Fixtures / Examples / SDK | ◐ | Batch A 已完成 OneBot 主链 companion updates，模板编辑器、治理与 manifest 相关样例仍待继续 |
| Phase 3 | OneBot11 传输模式补齐 | 🟡 | 完成 reverse WS、forward WS、HTTP、webhook 四条接入链路的正式主链 |
| Phase 4 | OneBot11 事件与消息兼容补齐 | 🟡 | 完成核心事件、消息段、历史消息、详情读取与 provider 扩展兼容矩阵 |
| Phase 5 | Plugin Protocol / Wider Action Family | 🟡 | 扩展 plugin protocol、SDK 与权限模型，完成更宽动作族接线 |
| Phase 6 | 在线模板编辑器 / Render 可视化 | 🟡 | 提供模板编辑、校验、预览、保存、回退与渲染结果可视化 |
| Phase 7 | Plugin Platform / Manifest / Config / Governance | 🟡 | 完成生命周期状态同步、配置迁移、manifest 元数据与治理读取面收口 |
| Phase 8 | Diagnostics / Web API / Web UI | ◐ | 协议中心连接设置与协议日志主线已完成，模板编辑、跨页面联动与其余可视化仍待继续 |
| Phase 9 | Launcher / 本地运维入口 | ◐ | 启动失败诊断与端口占用识别已补强，环境诊断与深链引导仍待继续 |
| Phase 10 | Release / Deployment / Quality Gates | 🟡 | 建立 v0.2 transport、compatibility、template editor 与 wider actions 门禁 |

---

## 二、Pre-Phase — 范围重置与前置承接 ☑️

| 任务项 | 状态 | 说明 |
| --- | --- | --- |
| v0.2 文档主入口收口 | ☑️ | `docs/execution-plan-v0.2.md` 作为当前执行计划，`docs/execution-plan.md` 作为 v0.1 基线参考 |
| v0.1 基线承接 | ☑️ | 单实例、基础 OneBot11 reverse WebSocket、插件运行时、管理面、渲染与恢复链路作为本轮前提 |
| v0.2 范围冻结 | ☑️ | 在线模板编辑器、可视化、更宽 action family、OneBot11 全传输模式、NapCat 与幸运莉莉娅扩展兼容纳入本轮 |
| 延后边界冻结 | ☑️ | 插件市场、强沙盒、插件间依赖、自动覆盖更新、非 OneBot 生态多协议继续后置 |

---

## 三、Phase 1 — Contract / Schema 冻结

### 当前批次

- ☑️ Batch A = OneBot 主链已完成
- 后续批次继续覆盖模板编辑器、治理读取面、manifest 元数据与其余 v0.2 surface

### 本轮必需冻结的正式边界

| 正式边界 | 状态 | 本轮冻结方向 |
| --- | --- | --- |
| `contracts/web-api.openapi.yaml` | ◐ | OneBot transport、协议快照、日志详情与相关主链已冻结；模板编辑器与治理读取面仍待继续 |
| `contracts/websocket-events.yaml` | ◐ | 协议状态与日志主链已补齐；模板预览与其余实时事件仍待继续 |
| `contracts/plugin-info.schema.json` | ⚠️ | `default_config`、`concurrency`、`icon`、`repo`、`homepage`、`keywords`、`screenshots`、`platforms`、`system_dependencies` |
| `contracts/plugin-protocol.schema.json` | ◐ | OneBot 主链所需更宽 `action` 与消息段已冻结；剩余 v0.2 范围仍待继续 |
| `contracts/config.user.schema.json` | ◐ | OneBot 多 transport 配置主模型已冻结；模板与治理可见配置仍待继续 |
| `contracts/error-codes.yaml` | ◐ | OneBot transport、compatibility 与 provider extension 主链错误码已冻结；模板编辑相关仍待继续 |
| `contracts/release-manifest.schema.json` | ◐ | OneBot 主链版本与回归声明已纳入；模板编辑器与其余 v0.2 回归声明仍待继续 |

### 本轮 contract 冻结清单

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| OneBot11 传输模式 | ☑️ | Batch A 已冻结 reverse WebSocket、forward WebSocket、HTTP 调用、webhook 上报，以及幸运莉莉娅 HTTP / SSE 接收兼容说明 |
| OneBot11 兼容矩阵 | ☑️ | Batch A 已冻结核心事件、消息段、动作族、历史消息、消息详情、转发消息、文件与 provider 扩展矩阵 |
| Plugin Protocol 扩展 | ☑️ | Batch A 已冻结更宽 `action family`、更宽消息段、必要的结果数据与结构化错误 |
| 在线模板编辑器 | ⚠️ | 模板列表、详情、源码编辑、schema 校验、实时预览、保存、历史版本与回退 |
| 治理与配置读取面 | ⚠️ | blacklist、cooldown、command permission 剩余读取面，以及生命周期状态同步、配置迁移、局部热更新相关读取面 |

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
- 后续批次继续覆盖模板编辑器、治理与 manifest 相关 examples / SDK / docs

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| transport / event / action fixtures | ◐ | OneBot 主链相关 fixtures 已补齐；模板与治理 surface 仍待继续 |
| OneBot11 provider 分层样例 | ☑️ | fixtures 已按标准 OneBot11、NapCat 扩展、幸运莉莉娅扩展三层组织 |
| examples 同步补齐 | ◐ | OneBot 主链已补现有示例插件与现有 examples；其余 v0.2 样例仍待继续 |
| SDK 示例与文档同步 | ◐ | OneBot 主链已覆盖 widened actions、消息段与 provider namespace；模板与治理文档仍待继续 |
| Golden 回归基线 | ◐ | OneBot 主链回归样例已建立；template editor 与其余 v0.2 回归基线仍待继续 |

---

## 五、Phase 3 — OneBot11 传输模式补齐 🟡

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| reverse WebSocket 收口 | 🟡 | 保留既有主链，并统一鉴权、ready、degraded、reconnect 与错误摘要 |
| forward WebSocket | 🟡 | 纳入正式主链，管理面与诊断面展示连接状态与失败原因 |
| HTTP API 调用 | 🟡 | 纳入 OneBot11 HTTP 调用主链，与 WS 模式共享鉴权、错误与状态语义 |
| webhook 事件上报 | 🟡 | 纳入正式接入模式，与 transport 状态和调试面统一 |
| 幸运莉莉娅 HTTP / SSE 兼容 | 🟡 | 作为 provider-specific 兼容矩阵中的正式条目处理 |
| 单实例约束 | 🟡 | 保持单实例、单活跃 OneBot 连接模型，不引入多 bot / 多实例并行管理 |

---

## 六、Phase 4 — OneBot11 事件与消息兼容补齐 🟡

### 核心事件兼容

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| message / notice / request / meta 完整矩阵 | 🟡 | 补齐 friend、group request、recall、admin、ban、poke、like、essence、upload、flash file 等事件 |
| 历史与详情读取 | 🟡 | 补齐历史消息、消息详情、转发消息详情、文件详情等读取面 |
| provider 扩展事件 | 🟡 | NapCat 与幸运莉莉娅已公开的 OneBot 扩展事件进入兼容矩阵 |
| 不兼容项明示 | 🟡 | 以 provider 级兼容矩阵记录缺口，避免模糊 TODO 口径 |

### 消息段兼容矩阵

| 类型组 | 状态 | 范围 |
| --- | --- | --- |
| 基础消息段 | 🟡 | `text`、`image`、`at`、`reply`、`face` |
| 媒体与文件 | 🟡 | `record`、`video`、`file`、`flash file` |
| 富文本与卡片 | 🟡 | `json`、`xml`、`markdown`、`music`、`contact` |
| 组合与转发 | 🟡 | `forward`、`node` |
| 互动消息段 | 🟡 | `poke`、`dice`、`rps`、reaction / emoji-like |
| provider 扩展消息段 | 🟡 | `mface`、`keyboard`、`shake` 及 NapCat / 幸运莉莉娅已公开段类型 |

### 本轮要求

- 未支持消息段统计从“调试摘要”升级为“兼容补齐清单”。
- 管理面需能展示当前 provider、当前 transport 与当前能力覆盖情况。

---

## 七、Phase 5 — Plugin Protocol / Wider Action Family 🟡

| 动作家族 | 状态 | 本轮目标 |
| --- | --- | --- |
| message / media send family | 🟡 | 扩展文本、图片、语音、视频、文件、音乐卡片、转发、戳一戳等发送能力 |
| message manage / query family | 🟡 | 补齐撤回、已读、历史消息、消息详情、转发表现与读取类能力 |
| friend / user family | 🟡 | 补齐点赞、好友处理、备注、陌生人信息与相关扩展能力 |
| group manage family | 🟡 | 补齐群管理、群成员、群请求、公告、禁言、头衔、卡片等能力 |
| announcement / essence / honor family | 🟡 | 补齐精华、荣誉、公告、待办等群扩展能力 |
| file transfer / file system family | 🟡 | 补齐上传、下载、目录、转永久、闪传与在线文件相关能力 |
| reaction / poke / read-state family | 🟡 | 补齐表情回应、戳一戳、已读状态与相关互动能力 |
| provider-specific extension family | 🟡 | 纳入 NapCat 与幸运莉莉娅已公开且用户侧价值明确的 OneBot 扩展能力 |

### 本轮要求

- plugin protocol、SDK、fixtures、examples、权限模型与错误码同步更新。
- 继续保持 Python / Node.js 作为正式托管运行时。

### 本轮排除项

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 复杂流式协议 | ❌ | 流式回传与独立调试流不进入 v0.2 |
| 插件间依赖 | ❌ | 保持在 v0.3+ |
| 额外托管运行时语言 | ❌ | Go / Rust 官方托管运行时不进入本轮 |

---

## 八、Phase 6 — 在线模板编辑器 / Render 可视化 🟡

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 模板列表与详情 | 🟡 | 管理面提供模板列表、模板详情、当前版本与使用范围读取面 |
| 源码编辑 | 🟡 | 浏览器内编辑模板源码与受控 schema |
| schema 校验 | 🟡 | 保存前与预览前都可执行结构校验 |
| 实时预览 | 🟡 | 基于当前模板与输入数据进行连续预览 |
| 保存与版本回退 | 🟡 | 提供保存、历史版本查看与回退能力 |
| 输入结构可视化 | 🟡 | 展示模板输入结构、字段说明与校验结果 |
| 渲染结果可视化 | 🟡 | 展示 artifact、缓存命中、失败定位与任务结果 |
| 错误定位可视化 | 🟡 | 将模板错误、资源错误、渲染错误以可读方式展示在管理面 |

### 本轮排除项

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 拖拽式模板搭建器 | ❌ | 不进入本轮 |
| 模板市场与远程发布 | ❌ | 不进入本轮 |

---

## 九、Phase 7 — Plugin Platform / Manifest / Config / Governance 🟡

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 生命周期状态同步 | 🟡 | 继续收敛 `registration_state`、`desired_state`、`runtime_state`、`display_state` 与任务、恢复摘要之间的一致性 |
| 配置迁移 | 🟡 | 继续完成迁移结果、保存影响分类、局部热更新与需要重启语义 |
| manifest 元数据补齐 | 🟡 | 补齐 `icon`、`repo`、`homepage`、`keywords`、`screenshots`、`platforms`、`system_dependencies` |
| `default_config` / `concurrency` 正式化 | 🟡 | 将当前文档与实现已涉及、但 contract 尚未完全收口的字段纳入正式边界 |
| blacklist / cooldown / command permission 可见性 | 🟡 | 补齐治理读取面、管理面展示、配置可见性与诊断可见性 |
| 插件升级与重确认 | 🟡 | 保留现有升级、重确认、恢复后重启与权限扩张裁决主线，并补齐 v0.2 新边界 |

### 本轮说明

- blacklist、cooldown、聊天权限的运行内核已存在，本轮聚焦正式边界、管理可见性与验收收口。

---

## 十、Phase 8 — Diagnostics / Web API / Web UI ◐

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 协议 transport 可视化 | ◐ | 协议中心已展示当前 provider、当前状态、连接设置、失败原因与调试摘要 |
| OneBot 兼容矩阵可视化 | 🟡 | 兼容信息保留在文档与正式边界，不进入协议中心页面 |
| 模板编辑与预览界面 | 🟡 | 把模板编辑器与 artifact、任务、日志联动到同一管理流 |
| 协议中心增强 | ◐ | 协议中心已具备连接设置、协议日志终端流、日志详情与独立日志子页，剩余管理联动仍待继续 |
| 任务 / 日志 / 协议 / 插件联动钻取 | 🟡 | 提供从命令、插件、协议、日志、任务之间的联动查看路径 |
| 诊断与恢复增强 | ◐ | 协议日志详情、OneBot ignored response 观测、部分启动失败诊断已完成，统一诊断口径仍待继续 |

---

## 十一、Phase 9 — Launcher / 本地运维入口 ◐

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 启动 / 停止 / 健康检查 | ◐ | 已补强启动失败识别、已有服务探测与端口占用识别，其他运维动作仍按本地服务壳继续完善 |
| 环境检查与资源诊断 | 🟡 | 检查模板资源、协议资源、运行环境资源与关键目录状态 |
| 打开 Web 管理面 | 🟡 | 继续作为主要桌面入口，打开 Web 主界面 |
| Web 页面深链 | 🟡 | 支持打开协议中心、模板编辑器、任务详情等指定 Web 页面 |
| 错误提示与恢复引导 | ◐ | 启动失败的真实原因、健康服务占用与端口冲突已能直接区分，恢复与环境引导仍待继续 |

### 本轮排除项

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| 协议中心桌面版平行业务界面 | ❌ | Launcher 不复制 Web 协议中心 |
| 模板编辑器桌面版重复实现 | ❌ | Launcher 不复制 Web 模板编辑器 |
| 命令中心 / 插件管理 / 配置管理桌面版 | ❌ | Launcher 不复制 Web 已有业务功能 |

---

## 十二、Phase 10 — Release / Deployment / Quality Gates 🟡

| 子任务 | 状态 | 说明 |
| --- | --- | --- |
| transport matrix 门禁 | 🟡 | 建立 reverse WS、forward WS、HTTP、webhook 回归门禁 |
| compatibility matrix 门禁 | 🟡 | 建立标准 OneBot11、NapCat、幸运莉莉娅兼容门禁 |
| template editor 门禁 | 🟡 | 建立模板编辑、校验、预览、保存、回退回归门禁 |
| wider action family 门禁 | 🟡 | 建立扩展 action family 的 contract、SDK、Server、Web 联合回归 |
| self-host upgrade / rollback | 🟡 | 将 v0.2 新能力纳入打包回归、升级回滚与长期自托管验证 |
| 发布元数据补齐 | 🟡 | release metadata 明确声明 transport、compatibility 与 plugin protocol 版本信息 |

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
| PR 轻量门禁 | contracts / fixtures / SDK shape、Server / Web / Launcher 核心检查 | 保证可合并性 |
| 发布门禁 | transport matrix、compatibility matrix、template editor、wider action family、packaged recovery、self-host smoke | 保证可交付性 |
| 手动高成本回归 | provider extension 深回归、大样本消息段 / 文件 / 历史消息矩阵、长时间协议稳定性巡检 | 保留独立回归入口 |

### v0.2 核心验收场景

| 场景 | 验收目标 |
| --- | --- |
| OneBot11 全传输模式 | reverse WS、forward WS、HTTP、webhook 四种接入路径都能建立受控链路 |
| Provider 扩展兼容 | NapCat 与幸运莉莉娅至少形成可核验的兼容矩阵与回归样例 |
| OneBot11 事件与消息兼容 | 剩余核心事件、消息段、历史消息、详情读取进入正式兼容矩阵与回归范围 |
| Wider Action Family | plugin protocol、SDK、fixtures、examples、运行链路保持一致 |
| 在线模板编辑器 | 支持编辑、校验、预览、保存和回退 |
| 生命周期 / 配置 / 诊断 | 生命周期状态同步、配置迁移、局部热更新、诊断与恢复继续完成原 v0.2 目标 |
| 治理读取面 | blacklist / cooldown / permission 的剩余管理可见性在 Web 管理面可验证 |
| 管理面联动 | 协议中心、日志、任务、模板编辑器、指令中心之间的跳转与摘要口径一致 |
| Launcher 职责 | Launcher 只验证本地壳职责、诊断深链与打开 Web，不承担 Web 业务回归 |
| 发布与回滚 | transport、compatibility、template editor、wider action family 进入交付门禁与升级回滚回归 |

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

- v0.2 结束后，继续优先稳定 OneBot11 兼容矩阵、插件协议、manifest、能力授权、配置迁移、模板编辑与渲染接口。
- 后续扩展继续遵守 `contracts/` 为正式来源与 companion updates 四件套。
