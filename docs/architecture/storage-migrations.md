# Storage Migrations

RayleaBot uses a current-schema snapshot plus numbered legacy migrations.

## Source Of Truth

- Current SQLite schema snapshot: `server/internal/storage/migrations/000001_base.sql`
- Legacy upgrade steps: `server/internal/storage/migrations/000002_*.sql` and later
- SQL query generation input: `server/sqlc.yaml`, currently reading the current-schema snapshot
- Runtime migration list: `server/internal/storage/store_schema.go`

`000001_base.sql` describes the complete schema expected by new databases. Later migration files describe upgrades from earlier released or development schemas to the same current shape.

## Runtime Rules

- New databases are initialized by the current schema snapshot and recorded through `schema_migrations`.
- Existing databases are upgraded by applying unapplied numbered migrations.
- `schema_migrations` records `version`, `name`, and `applied_at`; old metadata rows are backfilled with the migration name during startup.
- A migration may use an explicit inspection function to skip itself when the target schema state is already present.
- Migration code does not ignore duplicate-column errors by matching database error strings.
- Schema changes must update the snapshot, add a numbered migration when existing databases need an upgrade path, and update storage tests in the same change.

## Drift Checks

Storage tests verify that:

- An empty database opens with all expected tables, indexes, and columns.
- A legacy database migrates to the current version.
- A migrated legacy database has the same schema shape as a fresh current database.

Schema shape comparison intentionally ignores column order because SQLite appends `ALTER TABLE ... ADD COLUMN` fields at the end for existing databases. Code must use explicit column names rather than relying on `SELECT *` column order.
