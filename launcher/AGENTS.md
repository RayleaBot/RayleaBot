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

## Shared Surface Rules

- 与服务端共享的正式接口继续来自 `contracts/web-api.openapi.yaml`，生成文件固定为 `launcher/src/shared/web-api.generated.ts`。
- 启动器展示的系统状态、恢复摘要、诊断结果、任务和登录交接，都直接消费服务端正式接口。
- `launcher-token` 与 `launcher-admission` 继续作为 Launcher 打开 Web 管理面的正式 bootstrap surface。

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
