package cli

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigCommandsIgnoreUnknownFields(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"validate", "init", "normalize"} {
		t.Run(command, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
			source := "server:\n  port: 9090\nweb:\n  exposure_mode: ignored-config-marker\nobsolete_section: ignored-config-marker\n"
			writeFile(t, configPath, source)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			if code := Run(Command{Name: "config", ConfigPath: configPath, Logger: logger, Args: []string{command}}); code != 0 {
				t.Fatalf("config %s rejected unknown fields: exit code=%d", command, code)
			}
			after, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if command == "validate" {
				if string(after) != source {
					t.Fatal("config validate rewrote the user file")
				}
			} else if bytes.Contains(after, []byte("ignored-config-marker")) || !bytes.Contains(after, []byte("port: 9090")) {
				t.Fatal("config normalization retained ignored fields or lost the configured port")
			}
		})
	}
}
