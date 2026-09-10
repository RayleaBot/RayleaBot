package cli

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestArchivedRestoreConfigurationRejectsUnsafeDatabaseDestinations(t *testing.T) {
	for _, databasePath := range []string{"../outside.db", "data/../../outside.db", "config/state.db", ".hidden/state.db", "state.db", "data/state.db:stream"} {
		t.Run(databasePath, func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "backup.zip")
			file, err := os.Create(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(file)
			entry, err := writer.Create("config/user.yaml")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write([]byte("database:\n  path: " + databasePath + "\n")); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			reader, err := zip.OpenReader(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close() }()
			configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
			if _, _, err := archivedRestoreConfiguration(reader.File, configPath, ""); err == nil {
				t.Fatal("unsafe database destination was accepted")
			}
			if _, err := os.Stat(configPath); !os.IsNotExist(err) {
				t.Fatalf("configuration inspection wrote the destination: %v", err)
			}
		})
	}
}
