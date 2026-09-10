package system

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/diagnostics"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func (s *Service) BuildDiagnosticsArchive(ctx context.Context) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)

	if err := addJSONToZip(writer, "system-status.json", s.StatusSnapshot()); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "readiness.json", s.CurrentReadiness()); err != nil {
		return nil, err
	}
	doctorReport := diagnostics.Build(ctx, diagnostics.Options{
		ConfigPath: s.summary().ConfigPath,
		SchemaPath: s.summary().SchemaPath,
	})
	if err := addJSONToZip(writer, "doctor.json", doctorReport); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "plugins.json", map[string]any{"items": s.plugins.List()}); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "config-summary.json", s.summary()); err != nil {
		return nil, err
	}
	if summary := s.recoverySummarySnapshot(); summary != nil {
		if err := addJSONToZip(writer, "recovery-summary.json", summary); err != nil {
			return nil, err
		}
	}
	if s.logRepository != nil {
		logs, err := s.logRepository.ListSummaries(ctx, logging.Query{Limit: 100})
		if err != nil {
			return nil, err
		}
		if err := addJSONToZip(writer, "recent-logs.json", map[string]any{"items": logs}); err != nil {
			return nil, err
		}
	}
	if databasePath, err := s.databasePath(s.summary().ConfigPath, s.config().Database.Path); err == nil {
		spoolPath := logging.SpoolPathForDatabase(databasePath)
		if err := addOptionalFileToZip(writer, spoolPath, filepath.ToSlash(filepath.Join("data", filepath.Base(spoolPath)))); err != nil {
			return nil, err
		}
		quarantinePath := filepath.Join(filepath.Dir(spoolPath), "management-logs.spool.quarantine.jsonl")
		if err := addOptionalFileToZip(writer, quarantinePath, filepath.ToSlash(filepath.Join("data", filepath.Base(quarantinePath)))); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func addJSONToZip(writer *zip.Writer, path string, value any) error {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	entry, err := writer.Create(path)
	if err != nil {
		return err
	}
	_, err = entry.Write(bytes)
	return err
}

func addFileToZip(writer *zip.Writer, sourcePath, archivePath string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func(release func() error) { _ = release() }(file.Close)

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(archivePath)
	header.Method = zip.Deflate

	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, file)
	return err
}

func addOptionalFileToZip(writer *zip.Writer, sourcePath, archivePath string) error {
	if _, err := os.Stat(sourcePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return addFileToZip(writer, sourcePath, archivePath)
}
