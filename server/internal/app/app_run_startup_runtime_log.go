package app

import (
	"errors"
	"log/slog"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

func logStartupRuntimeFailure(logger *slog.Logger, kind string, err error) {
	if logger == nil || err == nil {
		return
	}

	fields := []any{
		"component", "app",
		"resource_kind", kind,
	}

	var bootstrapErr *deps.BootstrapError
	if errors.As(err, &bootstrapErr) {
		fields = append(fields, "remediation", bootstrapErr.Remediation)
	}

	logger.Warn("startup runtime prepare skipped", append(fields, "err", err.Error())...)
}

func logStartupRuntimeProgress(logger *slog.Logger, event deps.PrepareProgress) {
	if logger == nil {
		return
	}
	fields := []any{
		"component", "runtime_prepare",
		"resource_kind", event.Kind,
		"label", event.Label,
		"stage", event.Stage,
		"status", event.Status,
	}
	if event.ResourceID != "" {
		fields = append(fields, "resource_id", event.ResourceID)
	}
	if event.Version != "" {
		fields = append(fields, "version", event.Version)
	}
	if event.SourceLabel != "" {
		fields = append(fields, "source_label", event.SourceLabel)
	}
	if event.SourceURL != "" {
		fields = append(fields, "source_url", event.SourceURL)
	}
	if event.ArchivePath != "" {
		fields = append(fields, "archive_path", event.ArchivePath)
	}
	if event.StoreRoot != "" {
		fields = append(fields, "store_root", event.StoreRoot)
	}
	if event.Progress > 0 || event.Status == "succeeded" {
		fields = append(fields, "progress", event.Progress)
	}
	if event.DownloadedBytes > 0 {
		fields = append(fields, "downloaded_bytes", event.DownloadedBytes)
	}
	if event.TotalBytes > 0 {
		fields = append(fields, "total_bytes", event.TotalBytes)
	}
	if event.ExtractedEntries > 0 {
		fields = append(fields, "extracted_entries", event.ExtractedEntries)
	}
	if event.TotalEntries > 0 {
		fields = append(fields, "total_entries", event.TotalEntries)
	}
	if event.Summary != "" {
		fields = append(fields, "summary", event.Summary)
	}
	if event.Error != "" {
		fields = append(fields, "err", event.Error)
	}
	if event.Status == "failed" {
		logger.Warn("runtime_prepare_progress", fields...)
		return
	}
	logger.Info("runtime_prepare_progress", fields...)
}
