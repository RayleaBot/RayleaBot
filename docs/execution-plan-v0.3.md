# RayleaBot v0.3 执行计划

> `docs/execution-plan-v0.3.md` 作为 v0.3 的正式执行计划使用。  
> `docs/execution-plan-v0.2.md` 继续保留 v0.2 已完成基线与收口结果。  
> 本文档记录 v0.3 的正式范围、完成定义和延后边界。
>
> 状态图例：`☑️ 已完成` · `🟡 规划中` · `❌ 延后到后续版本`

---

## 一、总览

### v0.3 核对结论

- v0.2 主线已完成。
- 运维下的权限策略工作区已经覆盖黑名单管理与白名单前置裁决，`/commands` 提供命令治理只读快照。
- 当前真实缺口集中在“有来源展示、无正式可信校验”和“有版本检查、无完整受控更新引导”。
- 当前插件来源 `trust` 主要用于展示，不等于正式可信校验结果。
- 当前发布与桌面入口已具备版本检查、发布页跳转和 release metadata 校验基础，自动覆盖更新仍不在正式范围。

### v0.3 主线目标

- 补齐治理操作链，使运维下的权限策略工作区承担治理管理，`/commands` 保留命令治理读面。
- 补齐插件来源可信校验，使来源展示、安装任务、插件详情和诊断面共享同一套可信结果。
- 补齐发布可信校验与更新引导，使 Web、Launcher 和 CLI 都能沿正式升级与恢复路径完成版本查看、校验和下载引导。

### 前置承接

v0.2 已提供完整的管理面、插件生命周期、治理读取面、日志与诊断入口、模板编辑器、发布 smoke、恢复 drill、Launcher 版本检查和 release metadata 基线。v0.3 以这些能力为前提，重点补齐仍未闭环的正式能力。

### v0.3 正式范围

- 治理闭环：`/governance` 承担治理管理，`/commands` 保留命令治理读面。
- 可信来源：插件安装来源从展示型 `trust` label 补成正式可信校验。
- 受控更新：发布可信校验和更新引导补齐，但不做自动覆盖更新。

### 延后边界

- 插件市场与远程分发平台
- 强沙盒与更强资源隔离
- 插件间依赖解析
- 自动覆盖更新
- 非 OneBot 生态的多协议扩展
- 官方 Docker / Compose / 容器镜像交付
- 多语言正式 rollout
- 模板市场、远程模板分发、高级页面池和更复杂渲染并发优化

### 总阶段表

| 阶段 | 名称 | 状态 | 当前目标 |
| --- | --- | --- | --- |
| Pre-Phase | 真实缺口核对与边界重排 | ☑️ | v0.3 范围已固定为治理闭环、可信来源和受控更新引导 |
| Phase 1 | Governance / Commands | ☑️ | `/governance` 已形成黑白名单管理闭环，`/commands` 保留命令治理读面，默认权限与冷却继续沿用现有配置模型 |
| Phase 2 | Trusted Plugin Sources | 🟡 | 把插件来源 `trust` 从展示信息补成正式可信校验与诊断链 |
| Phase 3 | Release Trust / Guided Update | 🟡 | 补齐版本查看、发布可信校验、受控下载与升级引导，不进入自动覆盖更新 |
| Phase 4 | Companion Updates / Acceptance | 🟡 | 固定 contract-first、fixtures、examples、tests、docs 和验收要求 |

### 当前边界说明

| 边界 | 当前说明 |
| --- | --- |
| 治理 surface | 正式治理接口包含黑名单读写、白名单读写与 `GET /api/governance/command-policy` |
| `/governance` 页面职责 | 当前管理面展示治理总览，并通过黑白名单 tab、添加条目弹窗、目标 ID 复制、“确认启用空白名单”确认提示和“白名单已启用且当前为空”风险提示承担治理管理 |
| `/commands` 页面职责 | 当前管理面展示当前生效命令策略与全部声明命令 |
| 配置入口 | 默认权限与冷却继续通过配置页管理；黑白名单通过 `/governance` 管理 |
| 插件来源展示 | 插件列表与详情页已有 `source`、`trust`、`package_source_type` 与 `package_source_ref` 展示 |
| 可信来源校验 | `remote_url`、`local_zip` 与 `local_directory` 当前没有统一正式可信校验模型 |
| 发布可信基础 | 发布包已包含 `release_manifest.json`、`build_info.json`、`SHA256SUMS.txt`，Launcher 已有版本检查与发布页跳转 |
| 自动覆盖更新 | 继续保持在正式范围之外 |

---

## 二、Pre-Phase — 真实缺口核对与边界重排 ☑️

| 任务项 | 状态 | 说明 |
| --- | --- | --- |
| v0.2 完成态承接 | ☑️ | `docs/execution-plan-v0.2.md` 作为已完成基线，v0.2 已交付能力不进入 v0.3 待办 |
| 旧 v0.3 口径重排 | ☑️ | 旧规划中的“生态基础设施”范围收口为治理闭环、可信来源和受控更新引导 |
| 未闭环能力识别 | ☑️ | v0.3 聚焦治理写面、插件可信来源和发布可信校验 |
| 延后边界冻结 | ☑️ | 插件市场、强隔离、插件间依赖、自动更新、非 OneBot 多协议和容器正式交付继续后置 |

本阶段结论：

- v0.3 以真实缺口为范围，不以旧规划标题直接裁决。
- 当前没有 formal contract 的能力，不写成现有正式能力。
- v0.3 当前承接治理闭环、可信来源和受控更新引导三条主线。
- 白名单作为 v0.3 新治理概念单独冻结对象范围、裁决顺序和错误语义。
- 新治理写面、可信来源结果和更新引导 surface 后续都先按 contract-first 进入实现主链。

---

## 三、Phase 1 — Governance / Commands ☑️

### 当前真相

| 项目 | 当前情况 |
| --- | --- |
| 正式治理接口 | `GET /api/governance/blacklist`、`POST /api/governance/blacklist/entries`、`DELETE /api/governance/blacklist/entries/{entry_type}/{target_id}`、`GET /api/governance/whitelist`、`PUT /api/governance/whitelist/state`、`POST /api/governance/whitelist/entries`、`DELETE /api/governance/whitelist/entries/{entry_type}/{target_id}`、`GET /api/governance/command-policy` |
| Web 工作区 | `/governance` 展示治理总览，并通过黑白名单 tab、添加条目弹窗、目标 ID 复制、“确认启用空白名单”确认提示和“白名单已启用且当前为空”风险提示承担治理管理，`/commands` 展示有效命令策略与全部声明命令 |
| 默认权限与冷却 | 已通过现有配置模型管理 |
| 黑名单 | 用户 / 群黑名单已接入聊天侧命令裁决和管理工作区 |
| 白名单 | 用户 / 群白名单已具备正式 contract、正式存储、启用开关和管理工作区语义 |
| 命令级人工覆盖 | 当前不存在正式写模型 |

### 目标边界

- `/governance` 作为治理工作区，`/commands` 保留命令治理读面。
- 黑名单和白名单都采用单条 upsert 与单条删除。
- 白名单固定只作用于“是否进入命令分发”的前置裁决。
- 白名单优先于黑名单判断，但不绕过命令权限和冷却限制。
- 白名单支持显式启用开关，群聊采用“用户命中或群命中任一条即可通过”的规则。
- 默认权限与冷却继续沿用现有配置模型，不新增第二套命令策略编辑器。
- 不纳入命令级人工权限覆盖。

### 完成定义

| 子任务 | 状态 | 完成定义 |
| --- | --- | --- |
| 黑名单管理 | ☑️ | 管理面可新增、删除、查看用户 / 群黑名单，并复用现有聊天裁决链 |
| 白名单正式化 | ☑️ | formal contract、存储、裁决顺序、管理面展示和操作链一致 |
| `/governance` 工作区补齐 | ☑️ | 同一工作区内固定展示治理总览、黑白名单 tab、添加条目弹窗、目标 ID 复制反馈，以及“确认启用空白名单”确认提示和“白名单已启用且当前为空”风险提示 |
| `/commands` 工作区分工 | ☑️ | 命令策略与声明命令保持只读工作区语义 |
| 配置边界保持一致 | ☑️ | 默认权限与冷却继续通过现有配置入口管理，治理工作区只承载黑白名单管理 |
| 诊断与日志口径 | ☑️ | 黑白名单命中、权限拒绝和冷却拒绝保持正式错误码与诊断摘要一致 |

### Contract-first 要求

- 治理写接口、白名单对象范围、裁决顺序和错误语义已进入正式 contract。
- fixtures、examples、server、web、docs 和测试保持同轮更新。

---

## 四、Phase 2 — Trusted Plugin Sources

### 当前真相

| 项目 | 当前情况 |
| --- | --- |
| 安装来源 | `local_zip`、`local_directory`、`remote_url` 都已存在 |
| 管理面展示 | 插件列表与详情页展示 `source`、`trust`、来源目录与来源引用 |
| 当前 `trust` | 主要根据来源类型推导，属于展示型标签 |
| 当前缺口 | 没有统一正式可信校验模型，也没有把可信结果进入任务、诊断和安装判定 |

### 目标边界

- 插件安装进入正式可信来源校验模型。
- 可信结果进入 contract、安装任务、插件列表、插件详情和诊断面。
- `remote_url` 未验证来源继续允许安装，但必须有稳定高风险提示。
- `local_directory` 与 `local_zip` 继续保留人工来源路径，不写成正式可信发布渠道。
- 不把插件市场、远程分发平台或签名服务整套基础设施直接带入本轮。

### 完成定义

| 子任务 | 状态 | 完成定义 |
| --- | --- | --- |
| 可信结果结构 | 🟡 | formal contract 固定可信等级、校验结果、来源说明和高风险提示字段 |
| 安装任务链 | 🟡 | 安装任务能返回可信校验结果和必要的 remediation，而不是仅有展示型 `trust` |
| 管理面可见性 | 🟡 | 插件列表、插件详情、安装对话框和任务详情共享同一份可信摘要 |
| 诊断补齐 | 🟡 | diagnostics、日志和错误面可直接看见可信校验失败或高风险来源摘要 |
| 本地来源边界 | 🟡 | `local_directory`、`local_zip` 继续作为人工来源；不会被误标为正式可信发布源 |

### Contract-first 要求

- 新的可信来源结果结构、错误码和任务细节先进入 formal contract。
- fixtures 与 examples 需要覆盖可信来源、未验证远程来源和人工来源三类场景。
- SDK 与插件运行时不新增平行来源模型，只消费正式结果。

---

## 五、Phase 3 — Release Trust / Guided Update

### 当前真相

| 项目 | 当前情况 |
| --- | --- |
| 版本查看 | Launcher 已有版本检查和发布页跳转 |
| 发布元数据 | 当前已有 `release_manifest.json`、`build_info.json`、`SHA256SUMS.txt` |
| 正式交付校验 | release workflow 已覆盖 release metadata 与 packaged smoke |
| 自动更新 | 自动覆盖更新仍不在正式范围 |

### 目标边界

- 管理面、Launcher 和 CLI 的更新体验围绕“查看版本、校验发布、引导下载、走受控升级/恢复路径”组织。
- 发布可信校验补成正式能力。
- 自动覆盖更新继续后置，不进入 v0.3 完成定义。
- 不把签名服务、增量升级或第二套发布平台直接写成当轮必做项。

### 完成定义

| 子任务 | 状态 | 完成定义 |
| --- | --- | --- |
| 版本信息可见性 | 🟡 | Web、Launcher、CLI 能读取当前版本、可用版本和正式发布页信息 |
| 发布可信校验 | 🟡 | 下载前后可使用正式 release metadata 与 checksum 完成一致校验 |
| 受控下载与引导 | 🟡 | 管理入口提供明确的下载、校验、升级、恢复与回退指引 |
| 升级路径一致性 | 🟡 | 文档、Launcher、CLI 和管理面共享同一套升级 / 恢复正式路径 |
| 自动更新延后边界 | 🟡 | 自动覆盖更新在文档和验收口径中继续单独后置 |

### Contract-first 要求

- 需要新增的版本信息、发布校验和更新引导 surface 先进入 formal contract。
- fixtures、release tests、Launcher 和 Web 类型生成同步更新。
- `release_manifest.json`、`build_info.json` 与现有 release smoke 继续作为正式来源，不新建平行元数据模型。

---

## 六、Phase 4 — Companion Updates / Acceptance

### Companion updates 原则

- 任何涉及治理写面、白名单、可信来源、发布可信校验或更新引导的变更，都先走 contract-first。
- 同一能力的 formal contract、fixtures、examples、实现、测试和文档必须同轮更新。
- Web、Launcher、CLI 不得各自发明新的状态名、错误码或来源判定语义。

### Public API / Types

当前不属于已存在正式能力、需要在 v0.3 新增 formal contract 的 surface：

- 插件可信来源校验结果结构
- 发布可信校验与更新引导所需的正式 surface

当前明确不纳入 v0.3 的 surface：

- 自动覆盖更新
- 命令级人工权限覆盖
- 插件市场
- 插件间依赖解析
- 非 OneBot 多协议

### 测试与验收

| 场景 | 验收目标 |
| --- | --- |
| 治理工作区 | `/governance` 能完成治理总览查看、黑白名单 tab 管理、添加条目弹窗、目标 ID 复制、“确认启用空白名单”确认提示、“白名单已启用且当前为空”风险提示和错误反馈闭环 |
| 命令治理读面 | `/commands` 能完成命令策略查看、声明命令查看和插件筛选 |
| 聊天侧治理裁决 | 黑名单、白名单、命令权限和冷却的正式裁决顺序与 contract 保持一致 |
| 插件可信来源 | 安装任务、插件列表、插件详情和诊断面能显示统一可信结果 |
| 未验证远程来源 | `remote_url` 未验证来源仍可安装，但必须稳定显示高风险提示 |
| 发布可信校验 | Web、Launcher、CLI 都能查看版本、校验发布并进入正式升级 / 恢复路径 |
| 延后边界保持稳定 | 自动覆盖更新、插件市场、插件间依赖、强隔离和非 OneBot 多协议没有被误写进完成定义 |

---

## 七、长期边界

- v0.3 继续优先补齐正式管理闭环和可信校验，不把生态分发平台、强隔离和多协议扩展抢跑进主线。
- 后续任何跨层能力继续遵守 `contracts/` 为正式来源与 companion updates 四件套。
- 需要新增的治理、来源、发布或更新语义，先更新 formal contract，再进入实现主链。
