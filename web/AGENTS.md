# Web Agent Guide

界面与组件规则见 `DESIGN.md` 与 `docs/design/web-management-ui.md`，页面职责见 `docs/user/management-surface.md`。

## Interfaces and State

- HTTP 实现入口是 `web/src/lib/http.ts`。WebSocket 复用现有受控连接封装。
- 服务端接口类型由 `contracts/web-api.openapi.yaml` 生成至 `web/src/types/generated.ts`。类型不足时先检查契约与生成配置，不手写第二套 API 定义。
- 页面负责展示、编辑和受控跳转；写操作成功后优先回拉正式结果。
- 集合列表在搜索、筛选或集合变化时从第一页刷新，只重取已加载的页数。
- 时间格式化与日期输入使用配置响应的 `effective_timezone`，不以浏览器时区替代服务时区。
- 查询参数驱动的页面使用稳定 `viewKey`，只改查询参数时复用同一页面实例；管理面深链复用 `web/src/lib/management-links.ts`，不散写路由。
- 使用产品组件与共享视觉 token；组件职责与浮层行为见 `DESIGN.md`。不新增平行 HTTP client、WebSocket client、状态管理或组件系统。

## Errors

- 网络、服务端和鉴权错误复用统一处理策略；同一错误码保持一致语义，不按接口路径硬编码另一套解释。

## Browser Verification

- 受保护页面使用有效管理会话验证，登录方法参考 `web/tests/production/management.real.spec.ts`；登录页截图不能证明目标页面正常。
- 会话通过正式初始化与登录接口建立，不能只写本地存储；真实后端使用已授权的本地凭据。
- 不为视觉验证修改 router guard、session store 或 API 鉴权；临时日志、快照与 trace 不进入提交。
