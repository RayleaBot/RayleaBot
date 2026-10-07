package release

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

type Check struct {
	Name           string   `json:"name"`
	Status         string   `json:"status"`
	StartedAt      string   `json:"started_at"`
	ExitCode       *int     `json:"exit_code"`
	ElapsedSeconds *float64 `json:"elapsed_seconds,omitempty"`
	FinishedAt     string   `json:"finished_at,omitempty"`
}
type EvidenceResult struct {
	ArtifactID string           `json:"artifact_id"`
	Version    string           `json:"version"`
	GitCommit  string           `json:"git_commit"`
	Status     string           `json:"status"`
	Checks     []*Check         `json:"checks"`
	Archive    *ArchiveIdentity `json:"archive,omitempty"`
	FinishedAt string           `json:"finished_at,omitempty"`
}
type ArchiveIdentity struct {
	FileName  string `json:"file_name"`
	SizeBytes int64  `json:"size_bytes"`
}
type Evidence struct {
	Directory string
	Result    EvidenceResult
}

func evidenceTime() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000000+00:00") }
func NewEvidence(directory string, o PackageOptions) (*Evidence, error) {
	if e := os.MkdirAll(filepath.Dir(directory), 0755); e != nil {
		return nil, e
	}
	if e := os.Mkdir(directory, 0755); e != nil {
		return nil, e
	}
	v := &Evidence{directory, EvidenceResult{ArtifactID: o.ArtifactID, Version: o.Version, GitCommit: o.GitCommit, Status: "running", Checks: []*Check{}}}
	return v, v.save()
}
func (e *Evidence) save() error {
	return writeJSON(filepath.Join(e.Directory, "validation.json"), e.Result)
}
func (e *Evidence) Finish(failure error) error {
	e.Result.Status = "passed"
	if failure != nil {
		e.Result.Status = "failed"
	}
	e.Result.FinishedAt = evidenceTime()
	return e.save()
}
func (e *Evidence) RecordArchive(archive string) error {
	i, err := os.Stat(archive)
	if err != nil {
		return err
	}
	e.Result.Archive = &ArchiveIdentity{filepath.Base(archive), i.Size()}
	return e.save()
}

// Run records observable check output and status; packaging shares the Go implementation.
func (e *Evidence) Run(name string, out io.Writer, check func(io.Writer) int) (err error) {
	c := &Check{Name: name, Status: "running", StartedAt: evidenceTime()}
	e.Result.Checks = append(e.Result.Checks, c)
	if err = e.save(); err != nil {
		return err
	}
	start := time.Now()
	defer func() {
		c.Status = "passed"
		if err != nil {
			c.Status = "failed"
		}
		elapsed := math.Round(time.Since(start).Seconds()*1000) / 1000
		c.ElapsedSeconds = &elapsed
		c.FinishedAt = evidenceTime()
		err = errors.Join(err, e.save())
	}()
	log, err := os.Create(filepath.Join(e.Directory, name+".log"))
	if err != nil {
		return err
	}
	code := check(io.MultiWriter(out, log))
	c.ExitCode = &code
	err = log.Close()
	if code != 0 {
		err = errors.Join(err, fmt.Errorf("%s failed with exit code %d", name, code))
	}
	return err
}
func RunPackageArtifact(args []string, out, stderr io.Writer) int {
	fs := flags("package-artifact", stderr)
	o := packageFlags(fs, true)
	evidenceDir := fs.String("evidence-dir", "", "new directory for validation results and logs")
	smoke := fs.Bool("run-smoke", false, "check the produced archive")
	if e := parse(fs, args, "artifact-id", "version", "git-commit", "release-notes-ref", "server-bin"); e != nil {
		return cli.ErrorTo(stderr, e)
	}
	if e := artifactChoice(o.ArtifactID); e != nil {
		return cli.ErrorTo(stderr, e)
	}
	var evidence *Evidence
	var e error
	if *evidenceDir != "" {
		evidence, e = NewEvidence(*evidenceDir, *o)
		if e != nil {
			return result(out, stderr, "", e)
		}
	}
	execute := func(name string, check func(io.Writer) int) error {
		fmt.Fprintln(out, "+ "+name)
		if evidence != nil {
			return evidence.Run(name, out, check)
		}
		if code := check(out); code != 0 {
			return fmt.Errorf("%s failed with exit code %d", name, code)
		}
		return nil
	}
	var sidecar Sidecar
	e = execute("package", func(w io.Writer) int {
		var err error
		sidecar, err = Stage(repo.Root(), *o)
		return result(w, w, sidecar.ArchivePath, err)
	})
	if e == nil && evidence != nil {
		e = evidence.RecordArchive(sidecar.ArchivePath)
	}
	if e == nil && *smoke {
		e = execute("archive-smoke", func(w io.Writer) int {
			return result(w, w, "release smoke passed", Smoke(repo.Root(), o.ArtifactID, sidecar.ArchivePath))
		})
	}
	if evidence != nil {
		e = errors.Join(e, evidence.Finish(e))
	}
	return result(out, stderr, "", e)
}
