-- name: ReadSchemaMetadata :one
SELECT version, initialized_at FROM schema_metadata WHERE singleton_id = 1;
