package storage

import "io/fs"

func CurrentSchemaVersion() string {
	migrationFS, err := fs.Sub(embeddedMigrations, "migrations")
	if err != nil {
		return ""
	}
	items, err := loadMigrations(migrationFS)
	if err != nil {
		return ""
	}
	return items[len(items)-1].ID
}
