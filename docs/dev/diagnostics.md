# Diagnostics

本页说明 RayleaBot 当前用于开发、排障和运行诊断的正式信息入口。

## 当前正式诊断入口

| 入口 | 作用 |
| --- | --- |
| `/healthz` | 进程存活检查 |
| `/readyz` | 本地控制面与关键资源就绪检查 |
| `GET /api/system/diagnostics` | 读取仪表盘使用的聚合诊断快照 |
| `GET /api/system/diagnostics/export` | 导出诊断包 |
| `raylea doctor` | 检查本地配置、SQLite `quick_check` 与依赖元数据 |
| `/api/logs`、`/api/logs/{log_id}` 与 `/ws/logs` | 查看实时日志、历史日志、日志详情与当前启动窗口增量日志 |
| `/ws/plugins/{id}/console` | 查看插件 stderr |
| `logs/launcher/YYYY-MM-DD.log` | 查看 Launcher 自身诊断和进程编排错误 |
| `logs/server/YYYY-MM-DD.log` | 查看 `raylea-server` 的文本输出镜像 |

## 诊断信息范围

- 配置与 schema 校验结果
- 关键目录与运行环境资源状态
- Adapter 与渲染资源可用性
- 服务运行时长、插件总数、启用数与运行数
- 最近错误摘要、最近任务失败与渲染异常
- 后台任务结果和错误摘要
- 本次服务端启动日志与按时间范围筛选的历史日志
- 命令策略拒绝记录，包含 `command_name`、`error_code`、`reason`、`policy_stage` 和匹配插件上下文
- 脱敏后的协议消息详情、消息段、异常原因、payload preview 和 echo 类型

## 管理面诊断路径

系统状态页、协议中心、日志中心与模板预览页的诊断入口、日志筛选字段和跨页跳转规则见[管理面说明](../user/management-surface.md)。

## 健康接口语义

| 接口状态 | HTTP | wire body |
| --- | --- | --- |
| `/healthz` 可达 | `200 OK` | `{"status":"ok"}` |
| `/readyz` 为 `ready` | `200 OK` | `status=ready` |
| `/readyz` 为 `degraded` | `200 OK` | `status=degraded`，可附退化原因与 checks |
| `/readyz` 为 `setup_required` | `503 Service Unavailable` | `status=setup_required` |
| `/readyz` 为 `failed` | `503 Service Unavailable` | `status=failed`，可附失败摘要与 checks |
| 进程不可达 | 连接失败 | 无响应体 |

- `/healthz` 只反映进程是否存活，适合 Launcher、`systemd`、Docker 和 LXC。
- `/readyz` 反映管理认证、管理员初始化、SQLite 存活、FFmpeg 准备状态和渲染资源状态，只返回本次实际执行的检查；认证不可用或尚未初始化时省略 `checks`。
- `database` 每次通过已打开的连接执行最长 1 秒的 SQLite 存活探测，失败时为 `unavailable`，整体状态为 `failed`。
- `runtime` 读取启动与手动准备共用的 FFmpeg 内存状态：`ok`、`preparing` 或 `resource_missing`。准备中不产生问题、不降低整体状态；失败时携带可准备资源 `runtime_resources: [ffmpeg]`。Chromium 与模板资源由 `render` 检查。
- 运行或渲染资源缺失时为 `degraded`；数据库失败优先，资源问题不能将 `failed` 降为 `degraded`。外部聊天连接状态由协议快照报告，不参与服务就绪判断。
- 健康接口返回 JSON，至少包含 `status`，可附带 `reason`、`reason_codes` 和 `checks`。
- `starting`、`running`、`stopping`、`stopped` 是管理 WebSocket `service_status` 的展示词，不是健康探针 wire 值；该展示词集还可使用 `degraded`、`setup_required` 和 `failed`。

## 诊断包内容

- 程序版本、构建信息和运行环境摘要
- 关键目录、资源检查和配置摘要
- 插件列表、插件状态和最近错误快照
- 最近 100 条日志摘要，以及存在时的日志 spool 与 quarantine 文件
- `runtime/goroutine.txt`：所有 goroutine 的栈文本（`debug=2`）
- `runtime/heap.pprof` 与 `runtime/allocs.pprof`：存活对象与历史内存分配的采样 profile，可用 `go tool pprof` 分析
- `runtime/goroutineleak.pprof`：运行时提供 `goroutineleak` profile 时导出泄漏 goroutine 的栈采样
- `runtime/metrics.txt`：`runtime/metrics` 全部指标的一次快照，包含数值及直方图的桶边界与计数

运行时 profile 不含消息正文或堆对象内容，但包含函数名、源码路径和栈信息。

## 使用原则

- `/api/system/diagnostics` 是 Web 仪表盘使用的聚合运行时快照；诊断导出在此基础上收集受限的运行信息和日志摘要。
- `/readyz` 只反映关键资源就绪状态；CLI `doctor` 检查本地配置、SQLite 和依赖元数据，Launcher preflight 检查安装根、启动文件与本机环境。各入口不共享完整问题列表。
- 排障优先使用本页列出的诊断入口。
- 高风险问题在多个入口保持同一份 `code`、`severity`、`summary` 和 `remediation` 口径。
- OneBot API response 的 `echo` 缺失、空值或非字符串时，诊断面记录 warning 与结构化详情；真实 JSON 解析错误、读超时和连接错误继续按断链处理。

## 敏感信息边界

- 诊断包、错误摘要和 CLI 输出不直接暴露 `secret_store` 明文。
- 如需引用敏感项，只显示键名、来源说明或掩码值。
