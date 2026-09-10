# 管理集合的读取与刷新

正式参数和响应结构见 [OpenAPI](../../contracts/web-api.openapi.yaml)，事件结构见 [WebSocket 契约](../../contracts/websocket-events.yaml)。列表在过滤和排序后分页，默认与最大页大小均为 100。`total` 是过滤后的总数，`next_cursor` 缺失表示到达末页。

`cursor` 是当前排序中的十进制偏移量，不代表数据库快照。集合变化、搜索或筛选条件改变时，客户端从第一页刷新；刷新只重取用户已经加载的页数，更多条目由用户继续加载。`query` 最长 200 个字符，保证 ASCII 大小写不敏感的字面子串匹配，不把 `%`、`_` 当 SQL 通配符。

| HTTP 集合 | 过滤 / 顺序 | 实际读取边界 | WebSocket 与客户端策略 |
| --- | --- | --- | --- |
| `/api/governance/blacklist`、`/api/governance/whitelist` | `query`、`entry_type`；创建时间倒序、ID 倒序 | SQLite 在同一读取事务内计数并执行 LIMIT/OFFSET；每页用户和群条目合计不超过 100 | `governance.changed` 只通知变化；重新读取已加载页。`entry_count` 保留未过滤总数，白名单空名单确认据此判断 |
| `/api/system/scheduler/jobs` | `query`、`status`；名称顺序，或最近执行/耗时倒序，插件 ID 与作业 ID 稳定破同值 | 调度引擎持有全部活动作业，领域视图过滤排序后最多返回 100 条；运行队列不受展示分页影响 | 无全量作业 WS 推送；进入页面、手动刷新、触发后刷新已加载页 |
| `/api/system/render/templates` | ID、名称、描述、插件 ID/名称子串；模板 ID 升序 | 文件与缓存的领域同步仍扫描全部有效来源；HTTP 只投影当前页，不将分页当成模板发现范围 | 无全量模板 WS 推送；详情可按 ID 读取，未加载页中的深链仍有效 |
| `/api/plugins` | `query`、`state`、`source`；插件 ID 升序 | Catalog 是运行时全集，先筛选轻量摘要，再对当前页构建展示字段 | WS 按单个插件发送状态/命令变化；列表合并刷新已加载页。命令页、菜单、日志选择器使用独立分页集合，按 ID 补齐选中详情 |
| `/api/third-party/accounts` | 平台、账号 ID、标签、昵称子串；平台与账号 ID 升序 | SQLite 计数与 LIMIT/OFFSET；释放读取游标后再查询 secret 状态，单读取连接也可完成 | `third_party.account.changed` 只通知变化。新建请求使用 `create_only`，与未加载页同名时返回冲突，避免覆盖凭据 |
| `/api/plugin-store/sources` | ID、名称、URL 子串；来源 ID 升序 | 来源配置的缓存集合过滤排序后只返回当前页 | 无全量来源 WS 推送；保存和删除后刷新已加载页 |

适配器状态是配置实例的完整快照，按配置顺序提供 HTTP 与 WS 状态；它不承担历史集合的增量遍历。服务状态是单对象，治理/账号事件是失效通知，插件事件是单插件变化。日志与商店目录已有各自分页语义，保持其原有游标、过滤和上限。

数据库分页不等于插件内部读取 API 的改变：聊天策略仍查询完整有效治理规则，调度引擎仍装载所有需执行的作业，Catalog 与模板服务仍拥有完整运行状态。HTTP 查询不会删减这些权威数据。

分页回归覆盖超过两页的治理名单、后页插件的搜索/筛选、账号单连接读取、同名并发创建、浏览器加载更多与条件切换，以及未加载插件/模板的详情定位。
