# Launcher Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `launcher/` 目录特有、长期有效的规则。

## Launcher Boundary Rules

- `launcher/` 只负责桌面壳、本地环境检查、服务进程编排、启动停止、版本提示和打开 Web 管理面。
- 保持 `Go host / desktop / bindings / renderer / shared` 边界清晰：
  - Go host 负责原生窗口、托盘、单实例、系统对话框与静态资源安全中间件（`internal/frontend`）
  - `internal/desktop` 负责进程、服务控制和本地资源检查
  - Wails generated bindings 只暴露受限的 typed desktop bridge
  - `renderer` 负责界面展示
  - `shared` 承载桌面与渲染层共用模型、校验和生成类型
- Launcher 不复制 Web 业务逻辑，不维护独立状态模型，不解析 config/user.yaml 作为在线管理真相。

## Desktop Bridge Rules

- 新桌面调用必须有 typed request / response，并由固定的 Wails service method 暴露。
- 不通过桌面桥传递未经校验的任意结构；服务端正式响应保持 contract 生成类型。
- 桌面桥定义优先放在 `src/shared/` 或生成 bindings 中，Go desktop 与 renderer 共同消费。
- renderer 发起的桌面调用必须有明确错误路径，不吞掉异常或静默失败。

## Renderer Security Rules

- renderer 不直接操作本地文件系统、不直接启动或停止服务进程、不接触 secret 或凭据。
- renderer 只通过生成的 Wails bindings 与 Go desktop 层交互。
- 不在 renderer 中读取或解析 config/user.yaml、日志文件或任何本地配置文件。
- 用户输入在 renderer 中只做展示层校验，业务校验由服务端正式接口完成。

## Shared Surface Rules

- 与服务端共享的正式接口继续来自 `contracts/web-api.openapi.yaml`，生成文件固定为 `launcher/src/shared/web-api.generated.ts`。
- 系统状态与服务端诊断使用服务端正式 snapshot；Launcher 不从局部字段重建业务状态。
- `preflightChecks` 与 `recentStderr` 是 `internal/desktop` 持有的本机诊断，不属于服务端 contract。
- 恢复摘要在服务尚不可用时可由 `internal/desktop` 从本机日志目录的 recovery-summary.json 读取兜底；服务可用后由服务端 snapshot 覆盖。
- 常规打开管理面只传普通 URL，Web 会话由管理面建立。`setup_required` 是唯一例外：Launcher 把一次性 `setup_token` 放入 URL fragment，Web 立即清除 fragment，并只通过初始化请求头提交该 token。

## Error and Diagnostics Rules

- 启动、停止、恢复和诊断流程必须同时暴露用户可读错误和机器可读 `code`。
- 用户可读错误使用稳定文案，不拼接动态异常堆栈或内部路径。
- 服务端返回的机器可读 `code` 原样复用 `contracts/error-codes.yaml` 文件中的错误码，不转写、不发明 launcher 变体。
- Launcher 本机产生的错误（桌面桥、环境预检）使用本机命名空间：`launcher.*` 与 `chromium.*`、`deps.*`、`workdir.*` 等预检前缀；这些本机码不进入 `contracts/error-codes.yaml`。
- 诊断信息结构化输出，便于脚本和 CI 解析；人类可读摘要与机器可读字段共存。
- 本地环境检查失败时，给出明确修复指引或文档链接，不只返回失败状态码。

## Change Rules

- 新桌面桥调用、桌面展示状态或本地校验结果，要先确认不会和 Web 或 Server 发明第二套名称。
- 优先复用现有 `internal/desktop`、shared models 和 Fluent UI 组件，不新增平行 service layer 或第二套设计系统。
- 合同变更影响桌面类型时，保持 `pnpm generate:types` 后生成文件一致；Go service 变更同步运行 `pnpm generate:wails`。

## Verification

- Launcher Go module 与仓库根 `go.work` 隔离；直接运行 Go 命令时使用 `GOWORK=off`，Linux 同时使用 `-tags gtk3`，常用 `pnpm` 脚本已固定这些参数。
- 类型检查：`pnpm run typecheck`
- 单元测试：`pnpm test`
- 构建：`pnpm build`

## Consult Before Major Changes

- 工程基线与固定栈：`docs/engineering/baseline.md`
- Web / Launcher 边界：`docs/user/management-surface.md`
- 正式接口与类型来源：`contracts/README.md`
