# Platform Runtime

本页说明 RayleaBot 平台运行时的内部模型，覆盖配置、存储、日志、恢复、诊断、Launcher 控制面和兼容策略。

## 配置与运行目录

- 平台使用 `config/default.yaml` 与 `config/user.yaml` 生成有效配置。
- 运行根目录围绕 `config/`、`data/`、`cache/`、`logs/`、`plugins/installed/` 和 `.deps/` 组织。
- 配置读取、schema 校验、热更新快照和 `restart_required` 语义由服务端统一裁决。
- 插件不能直接读写 `config/user.yaml`，配置读写必须通过正式能力入口。

## 存储与日志

- SQLite 是唯一正式状态库，负责鉴权、任务、插件实例、授权、日志与调度持久化。
- `data/` 保存状态库和插件业务数据；`cache/` 保存可重建缓存；`logs/` 保存结构化日志与诊断输出。
- 敏感信息通过受控 secret store 管理，不把明文 secret 放入公开用户配置。
- 管理日志、插件日志和诊断导出复用同一套结构化摘要口径。

## 恢复、诊断与运行环境准备

- 恢复预检、启动后的兼容检查和人工处理摘要统一收敛到 `logs/recovery-summary.json`。
- `doctor`、诊断导出、Web 管理面和 Launcher 状态页共享同一份恢复摘要和资源问题摘要。
- `runtime.bootstrap` 负责运行环境资源准备；`recovery.recheck` 和 `recovery.confirm` 负责恢复摘要再检查与人工确认。
- 平台把恢复、兼容检查、运行环境资源准备和人工处理建议视为同一条正式运维链路的一部分。

## Launcher 与 Server 控制面

- Launcher 通过受控进程编排启动 `raylea-server`，并优先复用 `healthz`、`readyz`、`setup/status`、`session/launcher-token`、`session/launcher-admission`、`system/status` 和 `system/shutdown`。
- 若本机已经存在健康服务，但并非 Launcher 当前持有的子进程，Launcher 会明确标示为“检测到现有服务”。
- 启动失败摘要来自健康探测、stderr 和日志尾部，不要求用户自行拼接多处信息。
- 优雅停机优先走正式 shutdown 路径，再回退到操作系统级回收。

## 兼容与演进边界

- 优先稳定统一事件模型、插件 manifest、插件协议、能力授权模型和渲染接口。
- Patch 版本只用于修复缺陷、补充非破坏性观测或校正文档，不引入破坏既有数据语义的变更。
- RayleaBot 在协议层复用 OneBot11 生态，不追求直接兼容其他框架的插件运行时。
- v0.1 不内建 LLM / AI 平台能力；相关能力可由插件通过现有能力集自行组合。
- 多协议、多实例和更宽动作族保持在当前正式范围之外。

## 相关文档

- [Bot Core](./bot-core.md)
- [Render Service](./render-service.md)
- [State Model](./state-model.md)
