# Platform Runtime

本页说明 RayleaBot 平台运行时的内部模型，覆盖配置、存储、日志、恢复、诊断、Launcher 控制面和兼容策略。

## 配置与运行目录

- 平台使用 内嵌 schema 默认值与 `config/user.yaml` 生成有效配置。
- 运行根目录围绕 `config/`、`data/`、`cache/`、`logs/`、`plugins/installed/` 和 `.deps/` 组织。
- Launcher 本地设置位于 `data/launcher.json`，用于安装根选择、关闭行为和本地覆盖项，不替代 `config/user.yaml`。
- 配置读取、schema 校验、热更新快照和 `restart_required` 语义由服务端统一决定。
- 服务启动先获取 `<config-path>.runtime.lock`；同一配置文件已有运行实例时 fail-fast。锁在构建失败或服务关闭完成时释放，离线 `config init` / `config normalize` 在锁被占用时拒绝写入。
- 插件不能直接读写 `config/user.yaml`，配置读写必须通过正式能力入口。

## 存储与日志

- SQLite 是唯一正式状态库，负责鉴权、任务、插件实例、日志、调度和三方账号摘要持久化。
- `data/` 保存状态库、插件业务数据和 Launcher 本地设置；`cache/` 保存可重建缓存；`logs/` 保存结构化日志与诊断输出。
- 敏感信息通过受控 secret store 管理，不把明文 secret 放入公开用户配置；平台 Cookie / CK 使用三方账号 secret 命名空间保存。
- 管理日志、插件日志和诊断导出复用同一套结构化摘要口径。

## 恢复、诊断与运行环境准备

- 恢复预检、启动后的兼容检查和人工处理摘要统一写入 `logs/recovery-summary.json`。
- Web 管理面使用聚合系统 diagnostics，诊断导出收集受限运行信息；CLI `doctor` 与 Launcher preflight 各自检查本地职责范围。各入口可以复用恢复摘要，但不共享完整资源问题列表。
- `runtime.bootstrap` 负责运行环境资源准备；`recovery.recheck` 和 `recovery.confirm` 负责恢复摘要再检查与人工确认。
- 平台把恢复、兼容检查、运行环境资源准备和人工处理建议视为同一条正式运维链路的一部分。
- `data/launcher.json` 随同机目录保留，不进入正式恢复包范围。

### 数据初始化与本版恢复

SQLite 从 `server/internal/storage/schema.sql` 在事务内一次创建当前结构，并在 `schema_metadata` 保存唯一版本与初始化时间。重复启动复用同一结构与元数据，不重新初始化业务记录。配置版本为 `4`，数据库结构版本为 `000001`，管理员密码使用带随机盐的 Argon2id 格式。

恢复包从实际归档配置和 SQLite 快照读取当前格式版本；未包含数据库时清单明确记录 `absent`。恢复使用归档配置决定数据库落点，相对路径在目标根目录内保留，绝对来源路径重定位为 `data/rayleabot.db`。启动后检查资源与插件状态，并生成恢复摘要。

`scripts/release/rehearse_current_recovery.py` 使用真实 Server 在新建目录中初始化、创建管理员与插件业务数据，执行备份，再恢复到另一个空目录。演练核对配置、数据库、插件文件、恢复后的登录和重复启动结果；输出包含过程日志与结果 JSON。

## Launcher 与 Server 控制面

- 服务端是正式状态来源，`healthz`、`readyz`、`setup/status`、`launcher/status` 和 `launcher/shutdown` 保持正式契约。
- Launcher 通过受控进程编排启动 `raylea-server`，并直接调用本机 launcher surface。
- `desktop.Coordinator` 组装进程、设置、更新与监控；`startupGate` 独立持有启动许可、取消句柄和停止阻塞计数。快照组装与发布共享同一受保护状态，更新结果不会被一次较早的服务探测覆盖。
- Launcher 快照分成两组数据：
  - `server`：`health`、`readiness`、`systemStatus`
  - `launcher`：`processLifecycle`、`processOwnership`、环境检查、最近 stderr、版本提示、设置与本地错误
- Tray 与 Renderer 共用同一套展示推导函数，由同一份 `server` 与 `launcher` 快照生成标题、摘要和操作可用性。
- 若本机已经存在健康服务，但并非 Launcher 当前持有的子进程，Launcher 会明确标示为“检测到现有服务”。
- 启动失败摘要来自健康探测、stderr 和日志尾部，不要求用户自行拼接多处信息。
- 本机直连服务的优雅停机走 `/api/launcher/shutdown`，再回退到操作系统级回收；非本机服务只支持连接检查和打开 Web。
- Web 与 Launcher 都直接访问服务端，不通过对方代理状态或管理请求。
- Launcher 打开 Web 时只打开管理面 URL；Web 管理面通过初始化和登录接口建立会话。

## Launcher 预检边界

- Launcher 启动前阻塞项只覆盖本地必须立即确认的条件：
  - 安装根可用
  - `raylea-server` 路径有效
  - `config/user.yaml` 可定位
  - 工作目录可用
  - Launcher 设置可解析
- 模板基线目录和插件进程问题由服务端 readiness 与 system diagnostics 判定；实际浏览器可用性通过 readiness、system diagnostics 与 Launcher 状态展示。
- Launcher 可以展示这些问题，但不单独发明第二套运行态语义。

## 兼容与演进边界

- 优先稳定统一事件模型、插件 manifest、插件协议、能力声明、能力参数和渲染接口。
- Patch 版本只用于修复缺陷、补充非破坏性观测或校正文档，不引入破坏既有数据语义的变更。
- RayleaBot 在协议层复用 OneBot11 生态，不追求直接兼容其他框架的插件运行时。
- 不内建 LLM / AI 平台能力；相关能力可由插件通过现有能力集自行组合。
- OneBot11 与 QQ 官方机器人通过适配器实例接入；新增协议或动作族先定义契约和来源身份，再接入既有事件与动作边界。

## 相关文档

- [Bot Core](./bot-core.md)
- [Render Service](./render-service.md)
- [State Model](./state-model.md)
