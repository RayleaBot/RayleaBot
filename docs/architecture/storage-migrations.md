# Storage Migration Architecture

The storage migration strategy is defined in [`../engineering/storage-migrations.md`](../engineering/storage-migrations.md).

Architecture invariants:

- Fresh databases and upgraded databases converge to the same schema shape.
- `server/internal/storage/schema.sql` is the current schema snapshot.
- `server/internal/storage/migrations/*.sql` contains compatibility steps for existing databases.
- `schema_migrations` records applied compatibility versions and names.
- Storage code must not rely on SQLite column order.
