# Web Agent Guide

先遵守根 `AGENTS.md`，本文件只补充 `web/` 目录特有、长期有效的规则。

## Web Stack Rules

- Web 管理面固定采用 `Ant Design Vue + Vue Vben Admin` 对齐方案、Vue Router、Pinia 和当前工程锁定版本。
- 当前默认请求入口是 `web/src/request/http.ts`，不要新增平行 HTTP client。
- WebSocket 继续复用现有受控连接封装，不新增平行实时通信层。
- OpenAPI 生成类型继续来自 `contracts/web-api.openapi.yaml`，输出到 `web/src/types/generated.ts`。

## UI and State Rules

- 服务端是正式状态源；前端负责展示、编辑和受控跳转，不解析日志反推真实状态。
- 查询参数驱动的工作区继续使用稳定 `viewKey`，避免 query 变化拆出重复页签。
- 管理面内部深链优先复用现有 helper，如 `web/src/lib/management-links.ts`，不要在页面内散写路由对象。
- 页面写操作成功后优先回拉正式结果，不拼装本地假状态。

## Current Surface Expectations

- 运维分组当前正式页面包含：`/permission-policy`、`/access-lists`、`/commands`、`/tasks`、`/logs`、`/logs/history`、`/protocols`、`/protocols/compatibility`、`/config`。
- 插件详情页 `/plugins/:id` 保持单插件详情页签语义，并在页内承载概览、实时控制台和插件内置管理页工作区。
- 插件内置管理页通过 `/plugin-ui/{plugin_id}/...` 静态资源路由与正式桥接消息工作，不把管理 session、请求库或全局 store 直接暴露给插件页面。
- 权限策略、指令中心、日志中心、任务和协议中心继续通过稳定字段互相钻取，不靠摘要文案猜目标。

## Change Rules

- 新页面、新 query、状态名、错误展示或接口字段必须先与 `contracts/`、正式 docs 和现有页面语义对齐。
- 若 OpenAPI 改动影响 Web 类型，保持 `pnpm generate:types` 后生成文件一致。
- 保持当前样式体系、组件体系和布局壳，不引入新的前端框架或第二套设计系统。

## Verification

- 类型检查：`pnpm exec vue-tsc --noEmit`
- 单元测试：`pnpm test`
- E2E：`pnpm test:e2e`

## Consult Before Major Changes

- 工程基线与固定栈：`docs/engineering/baseline.md`
- Web 迁移与分层基线：`docs/engineering/web-antdv-vben-migration-plan.md`
- 管理面页面职责：`docs/user/management-surface.md`
- 正式接口与类型来源：`contracts/README.md`
