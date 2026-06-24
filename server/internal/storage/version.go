package storage

import "fmt"

func CurrentSchemaVersion() string {
	return fmt.Sprintf("%06d", latestSchemaMigrationVersion())
}

func latestSchemaMigrationVersion() int {
	latest := 0
	for _, migration := range schemaMigrations {
		if migration.version > latest {
			latest = migration.version
		}
	}
	return latest
}
