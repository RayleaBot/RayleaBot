package system

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/deps"
)

var prepareManagedRuntimeWithReport = func(ctx context.Context, repoRoot, kind string) (*managedRuntimePrepareReport, error) {
	report, err := deps.NewManager(repoRoot).PrepareWithReport(ctx, kind)
	if err != nil {
		return nil, err
	}
	return runtimePrepareReportFromDeps(report), nil
}

var prepareManagedRuntimeWithProgress = func(ctx context.Context, repoRoot, kind string, progress deps.PrepareProgressReporter) (*managedRuntimePrepareReport, error) {
	if progress == nil {
		return prepareManagedRuntimeWithReport(ctx, repoRoot, kind)
	}
	report, err := deps.NewManager(repoRoot).PrepareWithReportOptions(ctx, kind, deps.PrepareOptions{Progress: progress})
	if err != nil {
		return nil, err
	}
	return runtimePrepareReportFromDeps(report), nil
}

func runtimePrepareReportFromDeps(report *deps.PrepareReport) *managedRuntimePrepareReport {
	if report == nil {
		return nil
	}
	return &managedRuntimePrepareReport{
		Kind:               report.Kind,
		ArchivePath:        report.ArchivePath,
		StoreRoot:          report.StoreRoot,
		UsedPreparedStore:  report.UsedPreparedStore,
		UsedCachedArchive:  report.UsedCachedArchive,
		UsedSystemBrowser:  report.UsedSystemBrowser,
		AttemptedSources:   append([]string{}, report.AttemptedSources...),
		SelectedSource:     report.SelectedSource,
		PreparedEntrypoint: report.PreparedEntrypoint,
	}
}
