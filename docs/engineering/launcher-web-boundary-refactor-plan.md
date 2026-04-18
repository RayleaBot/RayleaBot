# Launcher 与 Web 边界收口重构计划

## 实际核对结论

| 议题 | 结论 | 处理方式 |
| --- | --- | --- |
| Launcher 复合状态混合服务端与本地壳信息 | 成立 | `LauncherSnapshot` 拆成 `server` 与 `launcher` 两层，Renderer 与 Tray 只消费统一展示推导 |
| Launcher 与 Web 各自轮询相同端点 | 成立 | 两端继续直连服务端，统一状态解释、错误语义和按钮占用规则 |
| `recovery.recheck`、`runtime.bootstrap`、`shutdown` 存在双入口 | 部分成立 | 复用现有任务流与 `shutting_down` 语义，优先打开已有任务并禁用重复操作 |
| Launcher 手工维护契约同构类型 | 成立 | Launcher 改为从 `contracts/web-api.openapi.yaml` 生成类型 |
| HTTP 客户端与错误解释重复 | 成立 | 对齐 JSON error envelope、纯文本错误、401 和超时的解释语义 |
| Launcher 内部状态文案存在两套映射 | 成立 | Tray 与 Renderer 共用同一套展示推导函数 |
| Web 的 Launcher 自动登录链路属于不合理耦合 | 不成立 | 保留 `launcher-token` / `launcher-admission` 正式入口，并补足 `?token=` 深链交接说明 |
| Launcher 环境检查过厚 | 部分成立 | 启动前阻塞项保留在本地预检，运行环境深层问题交给服务端 readiness、recovery 与 diagnostics |

## 目标边界

- 服务端是正式状态源，`healthz`、`readyz`、`/api/system/status` 与恢复摘要维持原始契约形状。
- Launcher 是桌面壳，负责本地进程编排、预检、桌面交互和打开 Web 管理面。
- Web 与 Launcher 都直接访问服务端，不通过对方代理状态，也不把 Launcher 变成第二个管理后端。
- `POST /api/session/launcher-token` 与 `POST /api/session/launcher-admission` 保持正式入口。
- `?token=` 是 Launcher 打开 Web 时的单次深链交接方式，用于把一次性 launcher token 交给 Web，再换成正常管理会话。

## 重构批次

### 批次 1：状态模型拆分

- `LauncherSnapshot` 固定包含两组数据：
  - `server`：`health`、`readiness`、`systemStatus`
  - `launcher`：`processLifecycle`、`processOwnership`、`environmentChecks`、`preflightChecks`、`advisoryChecks`、`recentStderr`、`releaseCheck`、`lastLocalError`、`settings`、`resolvedSettings`、`endpoint`
- `server` 保持服务端原始字段名与结构。
- `launcher` 只承载桌面壳本地观察，不承载正式服务状态名。
- Tray 与 Renderer 通过共享的 `deriveLauncherPresentation()` 生成标题、摘要、限制原因和操作可用性。

### 批次 2：类型来源与错误语义统一

- Launcher 从 `contracts/web-api.openapi.yaml` 生成 `launcher/src/shared/web-api.generated.ts`。
- `launcher-models.ts` 只保留 Launcher 本地类型与组合快照，不手写服务端契约同构结构。
- Launcher 管理客户端优先使用服务端 `error.message`、`error.code`、`error.details`。
- Web 与 Launcher 对以下错误形态采用一致解释规则：
  - JSON error envelope
  - 纯文本错误
  - 401
  - 超时中断

### 批次 3：系统动作占用与重复入口收口

- `recovery.recheck` 与 `runtime.bootstrap` 继续走现有任务模型。
- 提交前先检查同类任务是否处于 `pending` 或 `running`。
- 若已有同类任务，界面直接打开任务详情，并显示“任务进行中”。
- `shutdown` 继续由服务端 `shutting_down` 与 Launcher 本地 `processLifecycle=stopping` 共同裁决按钮禁用，不额外新增状态接口。

### 批次 4：Launcher 预检边界收口

- 启动前阻塞项只覆盖 Launcher 自己必须知道的本地条件：
  - 安装根可用
  - `raylea-server` 路径有效
  - `config/user.yaml` 可定位
  - 工作目录可用
  - Launcher 设置可解析
- 以下问题由服务端 readiness、recovery 与 diagnostics 统一裁决：
  - Chromium、Python、Node.js 运行环境深层有效性
  - `.deps/manifest.json` 资源一致性
  - 模板资源与渲染运行态问题
  - adapter、render、runtime 的运行期异常

## 验收标准

- Launcher `snapshot.server` 完整保留 `/healthz`、`/readyz`、`/api/system/status` 的原始结构。
- Launcher `snapshot.launcher` 只包含本地壳状态，不混入服务端状态名。
- Tray 与 Renderer 对同一快照输出一致的标题、摘要和限制原因。
- Launcher 与 Web 对 `recovery.recheck`、`runtime.bootstrap` 的重复触发都能复用现有任务，而不是重复提交。
- `shutdown` 进入 `shutting_down` 或 `processLifecycle=stopping` 后，两端都不会继续显示可重复触发。
- `?token=` 自动登录保持可用；token 失效或 admission 失败时，Web 能回到普通登录并显示明确提示。
- `docs/engineering/launcher-web-boundary-refactor-plan.md`、`docs/user/management-surface.md`、`docs/architecture/platform-runtime.md` 对状态源、桌面壳职责和 Launcher token 深链的描述一致。

## 非目标

- 新增管理 HTTP / WebSocket 路由
- 让 Web 改从 Launcher IPC 读取管理状态
- 让 Launcher 代理 Web 的状态轮询或管理请求
- 删除 Launcher 自动登录链路
- 引入新的顶层共享 TypeScript 包
