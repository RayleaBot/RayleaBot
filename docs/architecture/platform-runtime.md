# Platform Runtime

本页说明 RayleaBot 平台运行时的内部模型，覆盖配置、存储、日志、恢复、诊断、Launcher 控制面和兼容策略。

## 配置与运行目录

- 平台使用 `config/default.yaml` 与 `config/user.yaml` 生成有效配置。
- 运行根目录围绕 `config/`、`data/`、`cache/`、`logs/`、`plugins/installed/` 和 `.deps/` 组织。
- Launcher 本地设置位于 `data/launcher.json`，用于安装根选择、关闭行为和本地覆盖项，不替代 `config/user.yaml`。
- 配置读取、schema 校验、热更新快照和 `restart_required` 语义由服务端统一裁决。
- 插件不能直接读写 `config/user.yaml`，配置读写必须通过正式能力入口。

## 存储与日志

- SQLite 是唯一正式状态库，负责鉴权、任务、插件实例、授权、日志、调度、三方账号摘要和 Bilibili source 状态持久化。
- `data/` 保存状态库、插件业务数据和 Launcher 本地设置；`cache/` 保存可重建缓存；`logs/` 保存结构化日志与诊断输出。
- 敏感信息通过受控 secret store 管理，不把明文 secret 放入公开用户配置；Bilibili CK 使用三方账号 secret 命名空间保存。
- 管理日志、插件日志和诊断导出复用同一套结构化摘要口径。

## 恢复、诊断与运行环境准备

- 恢复预检、启动后的兼容检查和人工处理摘要统一收敛到 `logs/recovery-summary.json`。
- `doctor`、诊断导出、Web 管理面和 Launcher 状态页共享同一份恢复摘要和资源问题摘要。
- `runtime.bootstrap` 负责运行环境资源准备；`recovery.recheck` 和 `recovery.confirm` 负责恢复摘要再检查与人工确认。
- 平台把恢复、兼容检查、运行环境资源准备和人工处理建议视为同一条正式运维链路的一部分。
- `data/launcher.json` 随同机目录保留，不进入正式恢复包范围。

## Launcher 与 Server 控制面

- 服务端是正式状态源，`healthz`、`readyz`、`setup/status`、`launcher/status` 和 `launcher/shutdown` 保持正式契约。
- Launcher 通过受控进程编排启动 `raylea-server`，并直接调用本机 launcher surface。
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
- Chromium、Python、Node.js、模板资源和运行态问题由服务端 readiness、恢复摘要与 diagnostics 统一裁决。
- Launcher 可以展示这些问题，但不单独发明第二套运行态语义。

## 兼容与演进边界

- 优先稳定统一事件模型、插件 manifest、插件协议、能力授权模型和渲染接口。
- Patch 版本只用于修复缺陷、补充非破坏性观测或校正文档，不引入破坏既有数据语义的变更。
- RayleaBot 在协议层复用 OneBot11 生态，不追求直接兼容其他框架的插件运行时。
- 不内建 LLM / AI 平台能力；相关能力可由插件通过现有能力集自行组合。
- 多协议、多实例和更宽动作族保持在当前正式范围之外。

## 相关文档

- [Bot Core](./bot-core.md)
- [Render Service](./render-service.md)
- [State Model](./state-model.md)
