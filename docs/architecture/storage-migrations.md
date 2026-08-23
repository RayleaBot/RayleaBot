# Storage Migration Architecture

存储迁移策略以[工程迁移说明](../engineering/storage-migrations.md)为准。

## 架构不变量

- 全新数据库与升级后的数据库最终收敛到相同的 schema 结构。
- `server/internal/storage/schema.sql` 是当前 schema 快照。
- `server/internal/storage/migrations/*.sql` 保存现有数据库所需的兼容迁移步骤。
- `schema_migrations` 记录已应用的兼容版本及名称。
- 存储代码不得依赖 SQLite 列顺序。
