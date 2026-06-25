-- name: ListSchemaMigrations :many
SELECT version, name, applied_at FROM schema_migrations ORDER BY version;
