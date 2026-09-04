# Launcher Agent Guide

先遵守根 `AGENTS.md`；职责边界见 `docs/user/management-surface.md`，工具入口见 `launcher/package.json` 与 `launcher/scripts/run-go.mjs`。

## Ownership

- Launcher 负责本机环境检查、进程编排、系统集成和打开管理面，不复制 Web 业务页或 Server 状态机。
- Go host 负责窗口、托盘、单实例、系统对话框与静态资源安全；`internal/desktop` 负责进程和本机资源；renderer 负责界面展示。
- 前端复用既有适配层和 Fluent UI 组件，不另建服务层或设计系统。

## Desktop Bridge

- 桌面桥以 Go service/model 为定义来源；变更后生成 Wails bindings，renderer 通过生成的 typed 接口调用，不手改生成文件。
- `src/shared/` 可以保留前端校验与展示适配类型，不作为 Go 桥接定义的来源。
- 服务端 API 类型由 `contracts/web-api.openapi.yaml` 生成至 `launcher/src/shared/web-api.generated.ts`，与 Wails 生成链分别跟随各自输入。
- 桥接输入有明确类型和校验，调用失败有明确错误路径，不吞异常或静默失败。

## State and Security

- renderer 不直接操作文件系统或服务进程，不读取本地配置、日志或平台凭据；本机操作通过 Go desktop 层执行。
- renderer 做展示层校验，本机参数由 Go desktop 层校验，服务端业务由正式 API 校验。
- 服务端业务状态和诊断使用正式 snapshot；Go desktop 层可维护本机进程、预检和启动诊断。
- 服务不可用时，恢复摘要可由 Go desktop 层读取本机日志目录的 recovery-summary.json；服务可用后由服务端 snapshot 覆盖。
- 常规打开管理面只传普通 URL，Web 自行建立会话。`setup_required` 时由 Launcher 将一次性 `setup_token` 放入 URL fragment，Web 立即清除 fragment，并只通过初始化请求头提交 token。
- 服务端错误码原样复用，本机错误使用本机命名空间，不写入服务端错误目录；错误同时提供可读说明和机器可读 code。
- 诊断结构化输出，失败给出修复指引；用户可见错误不拼接异常堆栈或内部路径。

## Go Environment

- Launcher Go module 与根 `go.work` 隔离；直接运行 Go 命令使用 `GOWORK=off`，Linux 同时使用 `-tags gtk3`；常用工程脚本已固定这些参数。
