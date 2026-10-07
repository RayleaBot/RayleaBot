package system

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/metrics"
	"runtime/pprof"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/operations/diagnostics"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
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

	if err := addRuntimeProfilesToZip(writer); err != nil {
		return nil, err
	}
	if err := addRuntimeMetricsToZip(writer); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func addRuntimeProfilesToZip(writer *zip.Writer) error {
	for _, name := range []string{"goroutine", "heap", "allocs", "goroutineleak"} {
		profile := pprof.Lookup(name)
		if name == "goroutineleak" && profile == nil {
			continue
		}
		debug, extension := 0, ".pprof"
		if name == "goroutine" {
			debug, extension = 2, ".txt"
		}
		entry, err := writer.Create("runtime/" + name + extension)
		if err != nil {
			return err
		}
		if err := profile.WriteTo(entry, debug); err != nil {
			return err
		}
	}
	return nil
}

func addRuntimeMetricsToZip(writer *zip.Writer) error {
	descriptions := metrics.All()
	samples := make([]metrics.Sample, len(descriptions))
	for i, description := range descriptions {
		samples[i].Name = description.Name
	}
	metrics.Read(samples)

	var snapshot strings.Builder
	for _, sample := range samples {
		fmt.Fprintf(&snapshot, "%s\t", sample.Name)
		switch sample.Value.Kind() {
		case metrics.KindUint64:
			fmt.Fprintln(&snapshot, sample.Value.Uint64())
		case metrics.KindFloat64:
			fmt.Fprintln(&snapshot, sample.Value.Float64())
		case metrics.KindFloat64Histogram:
			histogram := sample.Value.Float64Histogram()
			fmt.Fprintf(&snapshot, "buckets=%v counts=%v\n", histogram.Buckets, histogram.Counts)
		case metrics.KindBad:
			fmt.Fprintln(&snapshot, "unavailable")
		}
	}
	entry, err := writer.Create("runtime/metrics.txt")
	if err != nil {
		return err
	}
	_, err = io.WriteString(entry, snapshot.String())
	return err
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
