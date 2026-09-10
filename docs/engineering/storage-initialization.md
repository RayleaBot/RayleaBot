# 数据库初始化

SQLite 当前结构由 `server/internal/storage/schema.sql` 定义。服务端初始化和 `server/sqlc.yaml` 的查询生成使用同一份文件。

## 初始化与重复启动

- 空数据库在一个事务内创建表、索引、约束和初始数据。
- `schema_metadata` 只有一条记录，保存当前结构版本和初始化时间；版本常量位于 `server/internal/storage/store_schema.go`。
- 重复打开数据库读取这条元数据，不重建表或覆盖业务数据。
- 有业务表但缺失初始化元数据、无效元数据和结构版本不符都会明确报错，不覆盖现有文件。
- 数据库文件锁、WAL 检查点、快照与恢复流程继续保护当前结构和业务数据。

## 修改结构

表、索引、列、约束与初始数据统一更新 `schema.sql`，并同步当前结构版本。运行 `sqlc generate` 和 `sqlc diff` 核对生成代码。应用查询明确列名，不依赖 SQLite 的物理列顺序。

存储测试验证初始化后的结构、重复启动幂等、数据约束、锁与快照。`scripts/release/rehearse_current_recovery.py` 使用真实 Server 验证从空目录初始化、创建管理员、写入插件数据、备份、恢复和再次登录；演练结果中的数据库版本与初始化时间必须保持一致。
