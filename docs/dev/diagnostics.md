# Diagnostics

本页说明 RayleaBot 当前用于开发、排障和运行诊断的正式信息面。

## 当前正式诊断入口

| 入口 | 作用 |
| --- | --- |
| `/healthz` | 进程存活检查 |
| `/readyz` | 本地控制面与关键资源就绪检查 |
| `GET /api/system/diagnostics/export` | 导出诊断包 |
| `raylea doctor` | 执行本地环境与资源检查 |
| `/api/logs`、`/api/logs/{log_id}` 与 `/ws/logs` | 查看本次服务端启动日志、历史日志、单条日志详情与当前启动窗口内的增量日志 |
| `/ws/plugins/{id}/console` | 查看插件 stderr |
| `logs/launcher.log` | 查看 Launcher 自身诊断和进程编排错误 |
| `logs/server.log` | 查看 `raylea-server` 的文本输出镜像 |
| `logs/recovery-summary.json` | 查看恢复与兼容处理摘要 |

## 诊断信息范围

- 配置与 schema 校验结果
- 关键目录与运行环境资源状态
- Adapter 与渲染资源可用性
- 服务运行时长、插件总数、启用数与运行数
- 最近错误摘要、最近任务失败与渲染异常
- 最近 24 小时的未支持事件类型与未知消息段计数
- 后台任务结果和错误摘要
- 恢复摘要、人工处理建议和最近确认记录
- 本次服务端启动日志与按时间范围筛选的历史日志
- 脱敏后的协议消息详情、消息段、异常原因、payload preview 和 echo 类型

## 健康接口语义

| 服务状态 | `/healthz` | `/readyz` |
| --- | --- | --- |
| `starting` | `200 OK` | `503 Service Unavailable` |
| `running` | `200 OK` | `200 OK` |
| `degraded` | `200 OK` | `200 OK`，返回退化原因 |
| `setup_required` | `200 OK` | `503 Service Unavailable` |
| `failed` | `200 OK` | `503 Service Unavailable`，返回失败摘要 |
| 进程不可达 | 连接失败 | 连接失败 |

- `/healthz` 只反映进程是否存活，适合 Launcher、`systemd`、Docker 和 LXC。
- `/readyz` 反映本地控制面、初始化状态和关键资源是否就绪。
- OneBot11 外部链路暂时不可用时，可返回 `degraded`，不与本地启动失败混淆。
- 健康接口返回 JSON，至少包含 `status`，可附带 `reason`、`reason_codes` 和 `checks`。

## 诊断包内容

- 程序版本、构建信息和运行环境摘要
- 关键目录、资源检查和配置摘要
- 插件列表、插件状态和最近错误快照
- 日志摘要、恢复摘要和人工处理建议

## 使用原则

- Web 管理面、CLI、Launcher 和导出诊断包尽量复用同一份结构化摘要。
- 排障优先使用正式诊断入口，而不是依赖临时日志拼接。
- 高风险问题需要在多个入口保持同一份 `code`、`severity`、`summary` 和 `remediation` 口径。
- OneBot API response 的 `echo` 缺失、空值或非字符串时，诊断面记录 warning 与结构化详情；真实 JSON 解析错误、读超时和连接错误继续按断链处理。

## 敏感信息边界

- 诊断包、错误摘要和 CLI 输出不直接暴露 `secret_store` 明文。
- 如需引用敏感项，只显示键名、来源说明或掩码值。
