# Launcher 与 Web 边界收口

## 实际结论

| 议题 | 结论 | 当前边界 |
| --- | --- | --- |
| Launcher 复合状态混合服务端与本地壳信息 | 成立 | `LauncherSnapshot` 拆成 `server` 与 `launcher` 两层，服务端状态保留原始契约结构，本地壳状态单独表达 |
| Launcher 与 Web 会读取同一组服务端状态 | 成立 | 两端继续直连服务端；Web 通过 `/ws/events` 的 `service_status` 事件触发状态刷新，HTTP 读取保留为首次进入、手动刷新和 socket 断线兜底 |
| `recovery.recheck`、`runtime.bootstrap`、`shutdown` 存在双入口 | 部分成立 | `recovery.recheck` 与 `runtime.bootstrap` 复用现有任务模型与进行中任务检测；`shutdown` 继续由 `shutting_down` 与本地 `processLifecycle=stopping` 共同裁决按钮禁用 |
| Launcher 手工维护契约同构类型 | 成立 | Launcher 从 `contracts/web-api.openapi.yaml` 生成类型，不再手写服务端契约同构结构 |
| HTTP 客户端与错误解释重复 | 成立 | Web 与 Launcher 保持同一套 error envelope 解释语义，优先使用服务端 `error.message`、`error.code`、`error.details` |
| Launcher 内部状态文案存在两套映射 | 成立 | Tray 与 Renderer 共用 `deriveLauncherPresentation()` 和统一状态标签 |
| Web 的 Launcher 自动登录链路属于不合理耦合 | 不成立 | `launcher-token` / `launcher-admission` 继续作为正式入口，`?token=` 只承担 Launcher 打开 Web 时的一次性深链交接 |
| Launcher 环境检查过厚 | 成立 | Launcher 本地检查只保留启动前必须立即确认的本地条件，运行时资源和恢复问题统一由服务端 readiness、recovery 与 diagnostics 表达 |

## 状态来源

- 服务端是正式状态源，`/healthz`、`/readyz`、`/api/system/status`、恢复摘要和任务信息保持原始契约形状。
- `/ws/events` 使用现有 `service_status` 分支推送服务状态变化摘要，字段固定为 `service_status`、`summary`、可选 `reason`、可选 `reason_codes`。
- Web 收到 `service_status` 事件后，更新 recent events，并去抖触发一次 `/readyz` 与 `/api/system/status` 刷新。
- Dashboard 自动刷新只在管理事件 socket 断开时回退到 HTTP 主拉；socket 正常时不做定时系统状态轮询。
- Launcher 继续直接读取正式服务端接口，不通过 Web 代理状态。

## Launcher 职责

- Launcher 负责本地进程编排、桌面交互、打开 Web 管理面和本地启动前检查。
- `launcher.server` 固定承载：
  - `health`
  - `readiness`
  - `systemStatus`
- `launcher.launcher` 固定承载：
  - `processLifecycle`
  - `processOwnership`
  - `environmentChecks`
  - `preflightChecks`
  - `advisoryChecks`
  - `recentStderr`
  - `releaseCheck`
  - `lastLocalError`
  - `settings`
  - `resolvedSettings`
  - `endpoint`
  - `localRecoverySummary`
- Renderer 与 Tray 只消费共享展示推导结果，不再自行拼装第二套服务状态名。

## 本地启动前检查

- Launcher 本地启动前检查只覆盖这些条件：
  - 安装目录可访问
  - 启动器设置可解析
  - `raylea-server` 路径有效
  - `config/user.yaml` 可定位，或可由 `config/default.yaml` 首次生成
  - 工作目录可写
- 以下问题统一由服务端 readiness、恢复摘要与诊断结果表达：
  - `.deps/manifest.json`
  - Chromium、Python、Node.js 运行时资源完整性
  - 模板目录与模板文件
  - 长路径与其他平台运行态问题
- Launcher 环境页展示本地 `preflightChecks`，不把服务端运行态问题混进本地启动前检查列表。

## 登录交接

- `POST /api/session/launcher-token` 用于申请一次性 launcher token。
- `POST /api/session/launcher-admission` 用于把一次性 launcher token 换成正常管理会话。
- `?token=` 是 Launcher 打开 Web 时附带的一次性交接参数。
- token 失效、admission 失败或服务未完成初始化时，Web 回到普通登录或初始化入口，并显示正式错误提示。

## 验收标准

- Launcher `snapshot.server` 完整保留服务端正式返回结构。
- Launcher `snapshot.launcher` 只承载本地桌面壳状态，不混入服务端状态名。
- Tray 与 Renderer 对同一快照给出一致的标题、摘要和限制原因。
- Web 在 `service_status` 事件正常可用时，不依赖定时 HTTP 主拉维持系统状态展示。
- Launcher 本地检查不再生成 `.deps`、运行时、模板和长路径等深层运行态告警。
