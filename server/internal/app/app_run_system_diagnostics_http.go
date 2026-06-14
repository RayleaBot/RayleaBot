package app

import (
	"archive/zip"
	"bytes"
	"context"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/cli"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func (s *systemService) BuildDiagnosticsArchive(ctx context.Context) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)

	if err := addJSONToZip(writer, "system-status.json", s.ManagementStatusSnapshot()); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "readiness.json", s.CurrentReadiness()); err != nil {
		return nil, err
	}
	doctorReport := cli.BuildDoctorReport(cli.Command{
		ConfigPath: s.state.Summary.ConfigPath,
		SchemaPath: s.state.Summary.SchemaPath,
	})
	if err := addJSONToZip(writer, "doctor.json", doctorReport); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "plugins.json", map[string]any{"items": s.plugins.List()}); err != nil {
		return nil, err
	}
	if err := addJSONToZip(writer, "config-summary.json", s.state.Summary); err != nil {
		return nil, err
	}
	if summary := s.state.recoverySummarySnapshot(); summary != nil {
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
	if databasePath, err := resolveDatabasePath(s.state.Summary.ConfigPath, s.state.Config.Database.Path); err == nil {
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
