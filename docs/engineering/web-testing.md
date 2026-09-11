# Web 端到端验证

业务状态由真实 Server 验证。浏览器模拟入口用于控制外部平台响应、WebSocket 消息节奏、网络故障、iframe 与大批展示数据，不作为 Server 校验、持久化或配置应用策略的实现依据。

## 真实 Server

在 `web/` 执行 `corepack pnpm run test:e2e:production`。该命令构建 Web 与示例插件 UI，再运行 `playwright.production.config.ts`。配置仅包含 `real-server` project，不启动模拟 HTTP 后端。已有最新构建时，可执行 `corepack pnpm exec playwright test --config playwright.production.config.ts --project real-server`。

[`real-server.fixture.ts`](../../web/tests/production/real-server.fixture.ts) 在每个 worker 构建一次当前源码的 Server。每个测试独立创建运行目录、配置、SQLite、管理初始化令牌和动态端口，通过正式初始化与登录接口建立会话。需要插件 UI 的测试在该临时目录编译、安装 `example-config-panel`，通过正式 API 写入初始配置与测试 secret。用例结束后优先通过 Launcher 控制接口关闭进程，超时才终止该子进程；退出后核对临时目录范围并删除目录。失败用例保留有界进程日志附件。

| 场景 | 当前验证入口 | 核对结果 |
| --- | --- | --- |
| 生产静态资源、保护路由、登录、治理作用域、配置落盘、日志详情 | [`management.real.spec.ts`](../../web/tests/production/management.real.spec.ts) | 真实 HTTP 响应与 UI 操作结果一致；同目标 ID 的不同机器人规则互不覆盖 |
| IPC 限流和抖音浏览器配置 | [`config.real.spec.ts`](../../web/tests/production/config.real.spec.ts) | Server 返回实际的即时应用与需重启字段，刷新后读取持久化结果 |
| Cookie/CSRF、全局插件配置、空适配器与空调度列表 | [`settings.real.spec.ts`](../../web/tests/production/settings.real.spec.ts) | 未初始化状态互相隔离；无 CSRF 的写入拒绝；保存后刷新仍保留配置 |
| 管理员密码与用户名更新 | [`account.real.spec.ts`](../../web/tests/production/account.real.spec.ts) | 当前密码由真实 Server 检查，更新使会话失效，新凭据能够重新登录 |
| 协议连接草稿、配置保存与密钥遮罩 | [`adapters.real.spec.ts`](../../web/tests/production/adapters.real.spec.ts) | 不完整或取消的表单不发写请求；新连接保存后的重启提示与遮罩结果来自 Server；后续编辑保留被遮罩密钥 |
| 时区键盘选择与生效范围 | [`timezone.real.spec.ts`](../../web/tests/production/timezone.real.spec.ts) | 浏览器提交指定时区，Server 保存新值并返回原生效时区与需重启字段 |
| 黑白名单增删、开关与空名单确认 | [`governance.real.spec.ts`](../../web/tests/production/governance.real.spec.ts) | 初始数据通过 API 创建；增删与开关操作后读取真实持久化结果 |
| 插件 iframe 隔离、握手、错误恢复 | [`plugin-management-ui.real.spec.ts`](../../web/tests/production/plugin-management-ui.real.spec.ts) | 当前 Server 托管实际示例插件，iframe 使用实际动态端口，插件来源不暴露管理 API；失败恢复只拦截 iframe 的网络请求 |

这些用例均使用真实 Server。第三方平台没有真实登录凭据，QQ、Bilibili、微博、抖音等外部登录与消息发送不属于本组覆盖。

## 受控浏览器场景

在 `web/` 执行 `corepack pnpm exec playwright test --project ui-fixtures`。`playwright.config.ts` 只包含 `ui-fixtures` project，使用 Vite 与 [`mock-backend.mjs`](../../web/tests/e2e/mock-backend.mjs) 提供的受控传输。`RAYLEA_E2E_WEB_PORT` 可指定 Vite 端口，模拟传输固定使用回环端口 `4010`。

[`web-ui.spec.ts`](../../web/tests/e2e/web-ui.spec.ts) 与同目录用例控制网络断开、会话失效、平台扫码状态、插件安装故障、日志持续追加、列表大数据以及焦点、主题、减少动画和窄屏交互。它们通过登录接口进入受保护页面；模拟会话门只为页面建立测试会话，不证明密码、CSRF 或来源授权策略正确。

[`fixture-data.mjs`](../../web/tests/e2e/fixture-data.mjs) 读取仓库契约样例。配置编辑场景只回显测试输入和预设应用结果；适配器状态直接取样例，不根据配置重算。账号校验与扫码按照选定样例返回，不解析 Cookie、推导凭据有效性或调用第三方平台。

模拟入口发送的 JSON 错误同时按 `contracts/error-codes.yaml` 检查 HTTP 适用范围、状态码和 `message_key`。断网场景返回带网关标记的空 `503` 响应，不伪造 Server 业务错误码。

需要写入后重新读取的插件 settings、secret、插件源场景通过 [`fixture-responses.ts`](../../web/tests/e2e/fixture-responses.ts) 指定完整响应和后续读取快照。[`response-plan.mjs`](../../web/tests/e2e/response-plan.mjs) 只匹配方法与路径、切换快照，不解释请求正文，不重算 changed keys、secret 合并、治理去重、源信任或安装检查规则。测试应另外断言发出的请求和页面可观察结果；对固定响应本身的断言不能用来证明 Server 的安全或持久化行为。

日志持续追加由 Playwright 的 APIRequestContext 注入，先确认新行已到达浏览器，再进行滚动断言。小型 [`fixture-log-pages.mjs`](../../web/tests/e2e/fixture-log-pages.mjs) 为这些已知数据提供浏览器滚动所需的页，使用 `fixture-row:` 游标和精确字段匹配。它不处理真实 Server 游标、时间戳归一化、输入校验或数据库排序；对应服务端行为由 Go 测试与真实 Server 用例覆盖。大列表用例通过测试内的请求拦截指定页面数据。

新增用例先选择真实 Server；仅在需要可控响应、消息序列或展示数据时使用模拟入口。不得为了截图或断言绕过鉴权、修改路由守卫或用日志反推正式状态。
