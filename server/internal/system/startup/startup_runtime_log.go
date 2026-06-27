package startup

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
)

func LogFailure(logger *slog.Logger, repoRoot string, kind string, err error) {
	if logger == nil || err == nil {
		return
	}

	fields := []any{
		"component", "app",
		"resource_kind", kind,
	}

	var bootstrapErr *deps.BootstrapError
	pathValues := []string{repoRoot}
	if errors.As(err, &bootstrapErr) {
		pathValues = append(pathValues, bootstrapErr.ArchivePath, bootstrapErr.StoreRoot)
		fields = append(fields, "remediation", logpath.Text(repoRoot, bootstrapErr.Remediation, pathValues...))
	}

	label := runtimePrepareKindLabel(kind)
	logger.Warn(label+"运行环境准备失败，已跳过自动准备", append(fields, "err", logpath.Error(repoRoot, err, pathValues...))...)
}

func LogProgress(logger *slog.Logger, repoRoot string, event deps.PrepareProgress) {
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
		fields = append(fields, "archive_path", logpath.Display(repoRoot, event.ArchivePath))
	}
	if event.StoreRoot != "" {
		fields = append(fields, "store_root", logpath.Display(repoRoot, event.StoreRoot))
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
	message := runtimePrepareProgressMessage(event)
	if event.Status == "failed" {
		logger.Warn(message, fields...)
		return
	}
	logger.Info(message, fields...)
}

func runtimePrepareProgressMessage(event deps.PrepareProgress) string {
	label := strings.TrimSpace(event.Label)
	if label == "" {
		label = runtimePrepareKindLabel(event.Kind)
	}
	stage := runtimePrepareStageLabel(event.Stage)
	status := runtimePrepareStatusLabel(event.Status)
	if event.Summary != "" {
		return "运行环境准备：" + label + "，" + stage + status + "，" + event.Summary
	}
	return "运行环境准备：" + label + "，" + stage + status
}

func runtimePrepareKindLabel(kind string) string {
	switch strings.TrimSpace(kind) {
	case "chromium":
		return "图片渲染 Chromium"
	case "python", "python-runtime":
		return "Python 运行环境"
	case "node", "nodejs-runtime":
		return "Node.js / npm 环境"
	default:
		if kind = strings.TrimSpace(kind); kind != "" {
			return kind
		}
		return "运行环境"
	}
}

func runtimePrepareStageLabel(stage string) string {
	switch strings.TrimSpace(stage) {
	case "resolve":
		return "解析资源"
	case "download":
		return "下载资源"
	case "verify":
		return "校验资源"
	case "extract":
		return "解压资源"
	case "ready":
		return "准备完成"
	default:
		if stage = strings.TrimSpace(stage); stage != "" {
			return stage
		}
		return "处理"
	}
}

func runtimePrepareStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "started":
		return "已开始"
	case "running":
		return "进行中"
	case "succeeded":
		return "成功"
	case "failed":
		return "失败"
	case "skipped":
		return "已跳过"
	default:
		if status = strings.TrimSpace(status); status != "" {
			return status
		}
		return "进行中"
	}
}
