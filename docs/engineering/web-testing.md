# Web 端到端验证

Web 端到端用例只验证业务流程与可观察的功能结果，全部运行在真实 Server 上；样式、布局、焦点外观、主题、动画与响应式尺寸不写 E2E，交由人工审核。

## 运行方式

在 `web/` 执行 `corepack pnpm run test:e2e:production`。该命令构建 Web 与示例插件 UI，再运行 `playwright.production.config.ts` 的 `real-server` project。已有最新构建时，可执行 `corepack pnpm exec playwright test --config playwright.production.config.ts --project real-server`。

[`real-server.fixture.ts`](../../web/tests/production/real-server.fixture.ts) 在每个 worker 构建一次当前源码的 Server。每个测试独立创建运行目录、配置、SQLite、管理初始化令牌和动态端口，通过正式初始化与登录接口建立会话。需要插件 UI 的测试在该临时目录编译、安装 `example-config-panel`；需要安装本地插件包的测试写入临时 `build_info.json`，使最低 Core 版本检查可以执行。用例结束后优先通过 Launcher 控制接口关闭进程，超时才终止该子进程；退出后核对临时目录范围并删除目录。失败用例保留有界进程日志附件。

| 场景 | 当前验证入口 | 核对结果 |
| --- | --- | --- |
| 生产静态资源、保护路由、登录、治理作用域、配置落盘、日志详情 | [`management.real.spec.ts`](../../web/tests/production/management.real.spec.ts) | 真实 HTTP 响应与 UI 操作结果一致；同目标 ID 的不同机器人规则互不覆盖 |
| 插件安装与启停 | [`plugins.real.spec.ts`](../../web/tests/production/plugins.real.spec.ts) | 构建测试插件归档，确认可信代码后经管理面安装，启用与停用后读取 Server 返回的实际状态 |
| 配置需重启字段 | [`config.real.spec.ts`](../../web/tests/production/config.real.spec.ts) | Server 返回实际的即时应用与需重启字段，刷新后读取持久化结果 |
| Cookie/CSRF、全局插件配置、空适配器与空调度列表 | [`settings.real.spec.ts`](../../web/tests/production/settings.real.spec.ts) | 未初始化状态互相隔离；无 CSRF 的写入拒绝；保存后刷新仍保留配置 |
| 管理员密码与用户名更新 | [`account.real.spec.ts`](../../web/tests/production/account.real.spec.ts) | 当前密码由真实 Server 检查，更新使会话失效，新凭据能够重新登录 |
| 协议连接草稿、配置保存与密钥遮罩 | [`adapters.real.spec.ts`](../../web/tests/production/adapters.real.spec.ts) | 不完整或取消的表单不发写请求；新连接保存后的重启提示与遮罩结果来自 Server；后续编辑保留被遮罩密钥 |
| 时区选择与生效范围 | [`timezone.real.spec.ts`](../../web/tests/production/timezone.real.spec.ts) | 浏览器提交指定时区，Server 保存新值并返回原生效时区与需重启字段 |
| 黑白名单增删、开关与空名单确认 | [`governance.real.spec.ts`](../../web/tests/production/governance.real.spec.ts) | 初始数据通过 API 创建；增删与开关操作后读取真实持久化结果 |
| 插件管理页同源加载、CSP 与错误恢复 | [`plugin-management-ui.real.spec.ts`](../../web/tests/production/plugin-management-ui.real.spec.ts) | 当前 Server 托管实际示例插件，iframe 从 `/plugin-ui/` 路径加载并直接调用管理 API；失败恢复只拦截 iframe 的网络请求 |

外部平台登录与消息发送不属于本组覆盖。新增用例使用真实 Server；会话建立与浏览器验证边界见 [`web/AGENTS.md`](../../web/AGENTS.md#browser-verification)。
