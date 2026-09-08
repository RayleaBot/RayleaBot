# Web Agent Guide

先遵守根 `AGENTS.md`；界面工程基线见 `docs/engineering/web-admin-baseline.md`，页面职责见 `docs/user/management-surface.md`。

## Interfaces and State

- HTTP 实现入口是 `web/src/lib/http.ts`；`web/src/request/http.ts` 只做兼容 re-export。WebSocket 复用现有受控连接封装。
- 服务端接口类型由 `contracts/web-api.openapi.yaml` 生成至 `web/src/types/generated.ts`。类型不足时先检查契约与生成配置，不手写第二套 API 定义。
- 服务端是正式状态源；页面负责展示、编辑和受控跳转，不解析日志反推状态。写操作成功后优先回拉正式结果。
- 查询参数驱动的工作区使用稳定 `viewKey`；管理面深链复用 `web/src/lib/management-links.ts`，避免重复页签和散写路由。
- 使用产品组件与共享视觉 token；组件职责与浮层行为见工程基线。

## Errors

- 错误分支依赖稳定 `code` 与结构化 `details`，不比对 `message`。
- 网络、服务端和鉴权错误复用统一处理策略；同一错误码保持一致语义，不按接口路径硬编码另一套解释。

## Browser Verification

- 受保护页面使用有效管理会话验证，登录方法参考 `web/tests/e2e/web-ui.spec.ts` 的 helper；登录页截图不能证明目标页面正常。
- mock 后端也通过正式登录接口建立会话，不能只写本地存储；真实后端使用已授权的本地凭据。
- 不为视觉验证修改 router guard、session store 或 API 鉴权；临时日志、快照与 trace 不进入提交。
