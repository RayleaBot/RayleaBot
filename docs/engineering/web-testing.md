# Web 端到端验证

业务状态由真实 Server 验证。浏览器模拟入口用于控制外部平台响应、WebSocket 消息节奏、网络故障、iframe 与大批展示数据，不作为 Server 校验、持久化或配置应用策略的实现依据。

## 真实 Server

在 `web/` 执行 `corepack pnpm run build`，然后执行 `corepack pnpm exec playwright test --config playwright.production.config.ts --project real-server`。

[`real-server.fixture.ts`](../../web/tests/production/real-server.fixture.ts) 在每个 worker 构建一次 Server。每个测试独立创建运行目录、配置、SQLite、管理初始化令牌和动态端口，通过正式初始化与登录接口建立会话。用例结束后优先通过 Launcher 控制接口关闭进程，超时才终止该子进程；退出后核对临时目录范围并删除目录。失败用例保留有界进程日志附件。

| 场景 | 当前验证入口 | 核对结果 |
| --- | --- | --- |
| 生产静态资源、保护路由、登录、治理作用域、配置落盘、日志详情 | [`management.real.spec.ts`](../../web/tests/production/management.real.spec.ts) | 真实 HTTP 响应与 UI 操作结果一致；同目标 ID 的不同机器人规则互不覆盖 |
| IPC 限流和抖音浏览器配置 | [`config.real.spec.ts`](../../web/tests/production/config.real.spec.ts) | Server 返回实际的即时应用与需重启字段，刷新后读取持久化结果 |
| Cookie/CSRF、全局插件配置、空适配器与空调度列表 | [`settings.real.spec.ts`](../../web/tests/production/settings.real.spec.ts) | 未初始化状态互相隔离；无 CSRF 的写入拒绝；保存后刷新仍保留配置 |
| 插件 iframe 隔离、握手、错误恢复 | [`plugin-management-ui.spec.ts`](../../web/tests/production/plugin-management-ui.spec.ts) | 生产构建使用受控插件来源；只通过指定来源、窗口、nonce 和 MessagePort 交换数据 |

前三行使用全新的真实 Server。最后一行属于 `plugin-ui-fixtures` project，因为这些用例需要精确控制 iframe 生命周期和错误响应。

## 受控浏览器场景

[`web-ui.spec.ts`](../../web/tests/e2e/web-ui.spec.ts) 保留网络断开、会话失效、平台扫码状态、插件安装故障、日志持续追加、列表大数据以及焦点、主题、减少动画和窄屏交互。它们通过正式登录接口进入受保护页面。

配置应用策略、恢复被遮罩凭据以及协议可用性不在模拟后端重算。配置写入使用预设响应场景，运行状态来自契约 fixture；协议新增连接的重启提示由对应测试显式指定。生产配置与全局插件设置的持久化用例已移到上表中的真实 Server 用例。

日志持续追加由 Playwright 的 APIRequestContext 注入，先确认新行已到达浏览器，再进行滚动断言；不用被跨源策略阻止的页面 fetch 充当成功的后台流量。模拟入口只保留假的配置凭据遮罩，真实凭据保存与引用规则由 Server 测试负责。

新增用例先选择真实 Server；仅在需要可控响应、消息序列或展示数据时使用模拟入口。不得为了截图或断言绕过鉴权、修改路由守卫或用日志反推正式状态。
