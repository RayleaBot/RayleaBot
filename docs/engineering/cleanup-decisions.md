# 清理范围与保留理由

本记录说明 Server 与 Web 清理时核实的删除范围、保留理由和连接复用实验。判断依据是当前生产入口、包引用、构造参数、生成输入与测试风险，出现次数不作为删除目标。

## 已核实的删除范围

| 范围 | 删除内容 | 保留的实际责任与验证 |
| --- | --- | --- |
| 无调用包装 | 旧 logging 构造包装、auth.Revoke、未使用 deps 下载/清单入口、OneBot 提取/归一化转发、插件动作识别转发、生命周期无 ctx 包装、旧 recovery 文件入口、旧 render 预览包装等 | 保留实际消费者调用的入口；运行期模型深复制和正式错误分类未替换为浅复制或文本判断 |
| 重复配置入口 | 未使用的配置常量、SQLite busy timeout Option 链 | 默认值仍从正式 schema 生成；SQLite 保留既定超时，生产与测试均无超时覆盖调用 |
| SQL | `CountNamespace`、`GetPluginStoreSource` | 全仓只有声明和生成文件，已删除输入并执行 sqlc generate/diff；其余加载、写入与本版恢复查询保留 |
| Web | 无引用的布尔标签、命令平铺包装、协议文案猜测、文本转发、view transition 探测导出、旧日志合并函数和信任类型别名 | RetryPanel 实际使用的 `resolveExceptionStatus` 保留；命令聚合、转义、排序日志合并与动画运行入口保留；类型检查通过 |
| 无输入变化的属性测试 | 安装任务查询、HTTP request ID 批量唯一性的 rapid 包装 | 原断言改为普通测试；认证随机字符串/场景、任务数量、命令解析和 Catalog 的真实生成器保留 |


## 保留与不实施项

- `app.New`、认证便捷构造、包内测试的查询 seam 等仍有清晰测试用途；不会因只被测试引用就删除。生产可变的测试全局钩子另由 P20 改为实例依赖。
- OneBot 原始帧、规范化事件、API 调用承担不同输入边界。群名/资料只失效群资料；名片/头衔只失效目标成员；管理员/退群失效该群成员集合。保留各入口的范围差异，未用一个泛化 switch 扩大失效范围。
- Go 标准库的 HTTP/1、HTTP/2 与 MIME 头大小错误没有导出的哨兵。Transport 边界先解开 URL/连接错误，再精确匹配冻结工具链的三个底层错误；真实超限响应与误导性 URL/普通错误文本都有回归。这不用于应用领域的状态判断。
- schema 校验不保证所有可选文本已经 trim，第三方输入与用户配置也不是同一边界；不按 TrimSpace 次数移除规范化。
- 深复制继续隔离嵌套 map/slice。发布版本严格解析与内部容错比较用途不同，未合为宽松解析器。
- 示例保留入门、事件输出、管理页、HTTP/存储、治理、插件发现、渲染、调度和 webhook 的不同教学用途；测试 artifact 与开发工作区仍验证独立 module 构建和协议握手。
- 插件运行时的 `protocolMu` 在写本地动作响应时跨越 stdin 写入持锁（`plugins/runtime/manager_local_actions.go`），读循环也在同一锁下路由帧。它源自退出顺序修复（30734d80）并有对应测试；缩小持锁范围需要先用竞态测试证明帧顺序不变，2026-09 的重构轮次未改动，列为后续课题。

## HTTP 连接复用实验

2026-09-10 在 Windows amd64、Go 1.26.6、Ryzen 7 5700X 上执行：

```powershell
go test ./internal/plugins/actions -run 'TestHTTPClientReauthorizesChangedDNSBeforeDial|TestHTTPClientPrivateGrantDoesNotLeakToAnotherClient' -bench BenchmarkHTTPTransportIsolation -benchtime=100x -count=3
```

| 本机回环请求 | 三次平均请求耗时 | 每请求新连接数 |
| --- | --- | --- |
| 实际隔离且执行授权的客户端 | 1.43 / 1.21 / 2.13 ms | 1 |
| 普通共享 transport 的网络成本下界 | 0.32 / 0.39 / 0.22 ms | 0.01 |

对照组没有域名/DNS 授权、响应限额和重试策略，只能说明复用可能节省连接成本。它不能证明安全等价，也不代表外网吞吐结果。当前客户端会在请求及实际拨号时重新解析和授权；复用旧连接还需处理 DNS 变化、权限变化、插件归属、取消及关闭所有权。本轮保留隔离 transport，不引入未经验证的共享池。

新增回归实际验证了预检后 DNS 变为私网地址时在拨号前拒绝，以及一个客户端的私网授权不会传给另一个客户端。既有超时、响应限额、重试与拒绝跳转测试继续保留。
