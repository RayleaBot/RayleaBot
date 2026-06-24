# Launcher Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `launcher/` 目录特有、长期有效的规则。

## Launcher Boundary Rules

- `launcher/` 只负责桌面壳、本地环境检查、服务进程编排、启动停止、版本提示和打开 Web 管理面。
- 保持 `main / preload / renderer / shared` 四层边界清晰：
  - `main` 负责进程、IPC、服务控制和本地资源检查
  - `preload` 只暴露受限桌面桥接
  - `renderer` 负责界面展示
  - `shared` 承载桌面与渲染层共用模型、校验和生成类型
- Launcher 不复制 Web 业务逻辑，不维护独立状态模型，不解析 `config/user.yaml` 作为在线管理真相。

## IPC Rules

- 新 IPC 必须有 typed request / response，使用固定 channel 名和明确载荷结构。
- 不通过 IPC 传递裸对象、动态键对象或未经校验的任意结构。
- IPC 定义优先放在 `src/shared/` 或等效类型契约中，main 和 renderer 共同消费。
- renderer 发起的 IPC 调用必须有明确错误路径，不吞掉异常或静默失败。

## Renderer Security Rules

- renderer 不直接操作本地文件系统、不直接启动或停止服务进程、不接触 secret 或凭据。
- renderer 只通过 preload 暴露的受限 API 与 main 层交互。
- 不在 renderer 中读取或解析 `config/user.yaml`、日志文件或任何本地配置文件。
- 用户输入在 renderer 中只做展示层校验，业务校验由服务端正式接口完成。

## Shared Surface Rules

- 与服务端共享的正式接口继续来自 `contracts/web-api.openapi.yaml`，生成文件固定为 `launcher/src/shared/web-api.generated.ts`。
- 启动器展示的系统状态、恢复摘要和诊断结果都直接消费服务端正式接口。
- Launcher 打开 Web 管理面只传普通 URL；Web 会话由管理面自己的初始化和登录流程建立。

## Error and Diagnostics Rules

- 启动、停止、恢复和诊断流程必须同时暴露用户可读错误和机器可读 `code`。
- 用户可读错误使用稳定文案，不拼接动态异常堆栈或内部路径。
- 机器可读 `code` 优先复用 `contracts/` 中已定义的错误码，不发明 launcher 专属错误码。
- 诊断信息结构化输出，便于脚本和 CI 解析；人类可读摘要与机器可读字段共存。
- 本地环境检查失败时，给出明确修复指引或文档链接，不只返回失败状态码。

## Change Rules

- 新 IPC、桌面展示状态或本地校验结果，要先确认不会和 Web 或 Server 发明第二套名称。
- 优先复用现有 services、shared models 和 Fluent UI 组件，不新增平行 service layer 或第二套设计系统。
- 合同变更影响桌面类型时，保持 `pnpm generate:types` 后生成文件一致。

## Verification

- 类型检查：`pnpm run typecheck`
- 单元测试：`pnpm test`
- 构建：`pnpm build`

## Consult Before Major Changes

- 工程基线与固定栈：`docs/engineering/baseline.md`
- Web / Launcher 边界：`docs/user/management-surface.md`
- 正式接口与类型来源：`contracts/README.md`
