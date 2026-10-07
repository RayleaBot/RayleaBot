package release

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/archiveio"
	"github.com/RayleaBot/RayleaBot/tools/internal/contractdata"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

func writeFixture(t *testing.T, path, body string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte(body), 0755); e != nil {
		t.Fatal(e)
	}
}
func packageFixture(t *testing.T, id string) PackageOptions {
	t.Helper()
	dir := t.TempDir()
	matrix, e := Matrix(repo.Root())
	if e != nil {
		t.Fatal(e)
	}
	a := matrix[id]
	o := PackageOptions{ArtifactID: id, Version: "0.7.0-beta.1", GitCommit: strings.Repeat("a", 40), BuiltAt: "2026-10-06T00:00:00Z", OutputDir: filepath.Join(dir, "out"), ServerBin: filepath.Join(dir, a.ServerBinary), WebDist: filepath.Join(dir, "web"), DepsDir: filepath.Join(dir, "deps"), TemplatesDir: filepath.Join(dir, "templates"), LicenseFile: filepath.Join(dir, "LICENSE"), ThirdPartyNotices: filepath.Join(dir, "THIRD_PARTY_NOTICES.md"), ReleaseNotesRef: "https://example.invalid/releases/v0.7.0-beta.1"}
	for _, p := range []string{o.ServerBin, o.LicenseFile, o.ThirdPartyNotices, filepath.Join(o.WebDist, "index.html"), filepath.Join(o.TemplatesDir, "help.menu/template.json"), filepath.Join(o.TemplatesDir, "status.panel/template.json")} {
		writeFixture(t, p, "fixture")
	}
	b, e := os.ReadFile(filepath.Join(repo.Root(), ".deps/manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	writeFixture(t, filepath.Join(o.DepsDir, "manifest.json"), string(b))
	for _, p := range []string{filepath.Join(o.DepsDir, "store/runtime/binary"), filepath.Join(o.DepsDir, "cache/download.zip"), filepath.Join(o.WebDist, "app.js.map"), filepath.Join(o.WebDist, "README.md"), filepath.Join(o.TemplatesDir, "help.menu/template.test.mjs")} {
		writeFixture(t, p, "excluded")
	}
	if a.LauncherRequired {
		o.LauncherBundle = filepath.Join(dir, "launcher")
		for _, p := range a.RequiredPaths {
			if strings.HasPrefix(p, "RayleaLauncher") || p == "WINDOWS-RUNTIME.md" {
				writeFixture(t, filepath.Join(o.LauncherBundle, p), "launcher fixture")
			}
		}
		if id == "macos-arm64-full" {
			o.LauncherBundle = filepath.Join(o.LauncherBundle, "RayleaLauncher.app")
			writeFixture(t, filepath.Join(o.LauncherBundle, "Contents/Info.plist"), "<plist/>")
		}
	} else {
		o.SystemdFile = filepath.Join(dir, "rayleabot.service")
		writeFixture(t, o.SystemdFile, "[Service]\nExecStart=/opt/raylea/raylea-server\n")
	}
	return o
}
func TestAllArtifactLayoutsSmokeAndMetadata(t *testing.T) {
	for _, id := range []string{"windows-x64-full", "linux-x64-full", "macos-arm64-full", "linux-x64-server"} {
		t.Run(id, func(t *testing.T) {
			o := packageFixture(t, id)
			s, e := Stage(repo.Root(), o)
			if e != nil {
				t.Fatal(e)
			}
			if e = Smoke(repo.Root(), id, s.ArchivePath); e != nil {
				t.Fatal(e)
			}
			names := []string{}
			e = archiveio.Walk(s.ArchivePath, func(entry archiveio.Entry, r io.Reader) error { names = append(names, entry.Name); return nil })
			if e != nil {
				t.Fatal(e)
			}
			for _, name := range names {
				for _, excluded := range []string{".deps/store/", ".deps/cache/", "app.js.map", "README.md", "template.test.mjs"} {
					if strings.Contains(name, excluded) {
						t.Fatalf("development content packaged: %s", name)
					}
				}
			}
			sidecarDir := filepath.Join(t.TempDir(), "relocated")
			if e = copyFile(s.ArchivePath, filepath.Join(sidecarDir, s.FileName)); e != nil {
				t.Fatal(e)
			}
			if e = copyFile(s.ArchivePath+".artifact.json", filepath.Join(sidecarDir, s.FileName+".artifact.json")); e != nil {
				t.Fatal(e)
			}
			loaded, e := LoadSidecar(filepath.Join(sidecarDir, s.FileName+".artifact.json"))
			if e != nil {
				t.Fatal(e)
			}
			if loaded.ArchivePath != filepath.Join(sidecarDir, s.FileName) {
				t.Fatal("sidecar ignored adjacent archive")
			}
			metadata := MetadataOptions{Version: o.Version, GitCommit: o.GitCommit, BuiltAt: o.BuiltAt, ConfigSchemaVersion: "4", DBSchemaVersion: "000008", PluginProtocolVersion: "4", ReleaseNotesRef: o.ReleaseNotesRef, DownloadBaseURL: "https://example.invalid/download", OutputDir: filepath.Join(o.OutputDir, "metadata")}
			p, e := BuildMetadata(repo.Root(), metadata, []Sidecar{loaded})
			if e != nil {
				t.Fatal(e)
			}
			var m Manifest
			if e = readJSON(p, &m); e != nil {
				t.Fatal(e)
			}
			if m.Channel != "beta" || m.PluginManifestVersion != "4" || m.Artifacts[0].UpdateMode != "guided" || m.Artifacts[0].DownloadURL != "https://example.invalid/download/"+s.FileName {
				t.Fatalf("metadata semantics: %+v", m)
			}
			if id == "macos-arm64-full" && m.Artifacts[0].SupportLevel != "experimental" {
				t.Fatal("macOS support promoted")
			}
			metadata.ReleaseNotesRef = "http://example.invalid/notes"
			metadata.OutputDir = filepath.Join(o.OutputDir, "invalid")
			if _, e = BuildMetadata(repo.Root(), metadata, []Sidecar{s}); e == nil {
				t.Fatal("accepted metadata outside contract")
			}
			if _, e = os.Stat(filepath.Join(metadata.OutputDir, "release_manifest.v2.json")); !os.IsNotExist(e) {
				t.Fatal("invalid metadata was written")
			}
			b, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			if bytes.Contains(b, []byte("sha256")) {
				t.Fatal("release digest introduced")
			}
		})
	}
}
func TestPackagingRejectsMissingRequiredAndDevelopmentInputs(t *testing.T) {
	for _, change := range []func(*PackageOptions){func(o *PackageOptions) { o.LauncherBundle = "" }, func(o *PackageOptions) { o.ThirdPartyNotices = filepath.Join(t.TempDir(), "missing") }, func(o *PackageOptions) { os.Remove(filepath.Join(o.LauncherBundle, "WINDOWS-RUNTIME.md")) }, func(o *PackageOptions) {
		writeFixture(t, filepath.Join(o.LauncherBundle, "node_modules/extra.js"), "source")
	}} {
		o := packageFixture(t, "windows-x64-full")
		change(&o)
		if _, e := Stage(repo.Root(), o); e == nil {
			t.Fatal("accepted invalid bundle")
		}
	}
	o := packageFixture(t, "linux-x64-full")
	writeFixture(t, filepath.Join(o.LauncherBundle, "src/main.go"), "package main")
	if _, e := Stage(repo.Root(), o); e == nil {
		t.Fatal("source bundled")
	}
	o = packageFixture(t, "linux-x64-server")
	o.SystemdFile = ""
	if _, e := Stage(repo.Root(), o); e == nil {
		t.Fatal("server package without service")
	}
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "plugins/installed/fixture/bin/fixture"), "binary")
	if e := assertClean(dir); e == nil {
		t.Fatal("packaged business plugin accepted")
	}
}
func TestMetadataBoundaries(t *testing.T) {
	o := packageFixture(t, "linux-x64-server")
	s, e := Stage(repo.Root(), o)
	if e != nil {
		t.Fatal(e)
	}
	metadata := MetadataOptions{Version: o.Version, GitCommit: o.GitCommit, BuiltAt: o.BuiltAt, ConfigSchemaVersion: "4", DBSchemaVersion: "000008", PluginProtocolVersion: "4", ReleaseNotesRef: o.ReleaseNotesRef, DownloadBaseURL: "https://example.invalid", OutputDir: filepath.Join(t.TempDir(), "metadata")}
	for _, change := range []func(*Sidecar){func(s *Sidecar) { s.FileCount = 0 }, func(s *Sidecar) { s.FileCount = 100001 }, func(s *Sidecar) { s.ExpandedSizeBytes = 0 }, func(s *Sidecar) { s.ExpandedSizeBytes = 8<<30 + 1 }, func(s *Sidecar) { s.UpdateMode = "automatic" }, func(s *Sidecar) { s.FileName = "../escape" }} {
		bad := s
		change(&bad)
		if _, e = BuildMetadata(repo.Root(), metadata, []Sidecar{bad}); e == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	if _, e = ParseReleaseTime("2026-10-06T00:00:00"); e == nil {
		t.Fatal("accepted timezone-free timestamp")
	}
	stamp, e := ParseReleaseTime("2026-10-06T08:00:00.123+08:00")
	if e != nil || stamp.Format("2006-01-02T15:04:05Z07:00") != "2026-10-06T00:00:00Z" {
		t.Fatal(stamp, e)
	}
}

func TestISOReleaseTimestampCompatibility(t *testing.T) {
	for _, source := range []string{"20261006T080000+0800", "2026-W41-2T08:00:00+08", "2026-10-06 08:00+08:00", "2026-10-06🕒08:00:00+08:00", "2026-10-06T00:00:01+00:00:01"} {
		value, err := ParseReleaseTime(source)
		if err != nil || value.Format("2006-01-02T15:04:05Z07:00") != "2026-10-06T00:00:00Z" {
			t.Fatalf("%s: %v %v", source, value, err)
		}
	}
	for _, source := range []string{"2026-13-06T00:00:00Z", "2026-10-06T00:00:00+24:00", "2026-W55-2T00:00:00Z"} {
		if _, err := ParseReleaseTime(source); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}
}

func TestSmokeRequiresArtifactCompressionFormat(t *testing.T) {
	o := packageFixture(t, "windows-x64-full")
	s, err := Stage(repo.Root(), o)
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(o.OutputDir, "staging", strings.TrimSuffix(s.FileName, ".zip"))
	wrong := filepath.Join(o.OutputDir, "wrong.tar.gz")
	if err = archiveio.Create(stage, wrong, false); err != nil {
		t.Fatal(err)
	}
	if err = Smoke(repo.Root(), o.ArtifactID, wrong); err == nil {
		t.Fatal("accepted tar.gz for a ZIP artifact")
	}
}
func TestPolicyAndReleaseNotes(t *testing.T) {
	for _, tag := range []string{"v0.7.0", "v0.7.0+build-1", "v0.7.0-beta.1", "v0.7.0-rc.2+build.3"} {
		p, e := PublicationSettings(repo.Root(), tag)
		if e != nil {
			t.Fatal(e)
		}
		beta := strings.Contains(strings.SplitN(tag, "+", 2)[0], "-")
		if (p.Channel == "beta") != beta || (p.Prerelease == "true") != beta || (p.MakeLatest == "false") != beta {
			t.Fatalf("publication %+v", p)
		}
	}
	for _, tag := range []string{"0.7.0", "v../outside", "v0.7", "v01.2.3", "v0.7.0\nchannel=stable"} {
		if _, e := PublicationSettings(repo.Root(), tag); e == nil {
			t.Fatalf("accepted %q", tag)
		}
	}
	for _, v := range [][2]string{{"0.7.0-beta.1", "stable"}, {"0.7.0", "beta"}, {"0.7.0", "unknown"}} {
		if _, e := Channel(repo.Root(), v[0], v[1]); e == nil {
			t.Fatal("mismatching channel")
		}
	}
	dir := t.TempDir()
	output := filepath.Join(dir, "output")
	args := []string{"--tag", "v1.2.3", "--notes-dir", dir, "--github-output", output}
	var log bytes.Buffer
	if code := RunPolicy(args, &log, &log); code != 1 {
		t.Fatal("missing notes accepted", code)
	}
	if _, e := os.Stat(output); !os.IsNotExist(e) {
		t.Fatal("published outputs before notes validation")
	}
	path := filepath.Join(dir, "v1.2.3.md")
	for _, body := range []string{"", " \n\t", "<!-- 编辑提示 -->", "# v1.2.3\n\n## 升级说明\n---\n<!-- 编辑提示 -->", "修复{{触发条件}}。", "正文\n<!-- {{PLACEHOLDER}} -->", "{{\n未填写\n}}", string([]byte{0xff, 0xfe, 0})} {
		writeFixture(t, path, body)
		if _, e := ValidateNotes(repo.Root(), "v1.2.3", dir); e == nil {
			t.Fatalf("accepted notes %q", body)
		}
	}
	body := "\ufeff修复更新检查失败。\n\n## 升级说明\n\n请参阅升级指南。\n"
	writeFixture(t, path, body)
	if code := RunPolicy(args, &log, &log); code != 0 {
		t.Fatal(log.String())
	}
	b, e := os.ReadFile(output)
	if e != nil || string(b) != "value=1.2.3\nchannel=stable\nprerelease=false\nmake_latest=legacy\n" {
		t.Fatal(string(b), e)
	}
	b, e = os.ReadFile(path)
	if e != nil || string(b) != body {
		t.Fatal("release notes modified")
	}
}
func TestValidationEvidencePreservesFailureAndLogs(t *testing.T) {
	for _, code := range []int{0, 7} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "evidence")
			v, e := NewEvidence(dir, PackageOptions{ArtifactID: "linux-x64-server", Version: "1.2.3", GitCommit: "abcdef1"})
			if e != nil {
				t.Fatal(e)
			}
			var out bytes.Buffer
			failure := v.Run("archive-smoke", &out, func(w io.Writer) int { fmt.Fprintln(w, "check output"); return code })
			if (failure != nil) != (code != 0) {
				t.Fatal(failure)
			}
			if e = v.Finish(failure); e != nil {
				t.Fatal(e)
			}
			var stored EvidenceResult
			if e = readJSON(filepath.Join(dir, "validation.json"), &stored); e != nil {
				t.Fatal(e)
			}
			if (stored.Status == "passed") != (code == 0) || *stored.Checks[0].ExitCode != code {
				t.Fatalf("incorrect evidence %+v", stored)
			}
			b, e := os.ReadFile(filepath.Join(dir, "archive-smoke.log"))
			if e != nil || !bytes.Contains(b, []byte("check output")) {
				t.Fatal("lost check output", e)
			}
			if _, e = NewEvidence(dir, PackageOptions{}); e == nil {
				t.Fatal("overwrote evidence")
			}
		})
	}
}
func TestPackageArtifactCLIRecordsActualArchiveAndSmoke(t *testing.T) {
	o := packageFixture(t, "linux-x64-server")
	evidence := filepath.Join(t.TempDir(), "evidence")
	args := []string{"--artifact-id", o.ArtifactID, "--version", o.Version, "--git-commit", o.GitCommit, "--release-notes-ref", o.ReleaseNotesRef, "--server-bin", o.ServerBin, "--web-dist", o.WebDist, "--deps-dir", o.DepsDir, "--templates-dir", o.TemplatesDir, "--systemd-file", o.SystemdFile, "--license-file", o.LicenseFile, "--third-party-notices", o.ThirdPartyNotices, "--output-dir", o.OutputDir, "--evidence-dir", evidence, "--run-smoke"}
	var output bytes.Buffer
	if code := RunPackageArtifact(args, &output, &output); code != 0 {
		t.Fatal(code, output.String())
	}
	var r EvidenceResult
	if e := readJSON(filepath.Join(evidence, "validation.json"), &r); e != nil {
		t.Fatal(e)
	}
	if r.Status != "passed" || len(r.Checks) != 2 || r.Checks[1].Name != "archive-smoke" {
		t.Fatalf("%+v", r)
	}
	i, e := os.Stat(filepath.Join(o.OutputDir, r.Archive.FileName))
	if e != nil || i.Size() != r.Archive.SizeBytes {
		t.Fatal("archive identity incorrect", e)
	}
}
func TestRuntimeMetadataAndCompactRetry(t *testing.T) {
	var m map[string]any
	if e := readJSON(filepath.Join(repo.Root(), ".deps/manifest.json"), &m); e != nil {
		t.Fatal(e)
	}
	if e := ValidateDeps(repo.Root(), m); e != nil {
		t.Fatal(e)
	}
	resource := m["resources"].([]any)[0].(map[string]any)
	resource["entrypoints"] = map[string]any{}
	if e := ValidateDeps(repo.Root(), m); e == nil {
		t.Fatal("incomplete runtime entrypoints accepted")
	}
	transient := t.TempDir()
	source, target := filepath.Join(transient, "source"), filepath.Join(transient, "target")
	writeFixture(t, filepath.Join(source, "content"), "retained")
	first := true
	if e := replaceWithRetry(source, target, time.Second, func(a, b string) error {
		if first {
			first = false
			return os.ErrPermission
		}
		return os.Rename(a, b)
	}); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(filepath.Join(target, "content")); e != nil || string(b) != "retained" {
		t.Fatal("transient sharing violation lost content", e)
	}
	for _, locked := range []bool{false, true} {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		target := filepath.Join(dir, "target")
		writeFixture(t, filepath.Join(source, "content"), "kept")
		calls := 0
		err := replaceWithRetry(source, target, 1, func(a, b string) error {
			calls++
			if locked {
				return os.ErrPermission
			}
			return os.Rename(a, b)
		})
		if locked {
			if !errors.Is(err, os.ErrPermission) {
				t.Fatal(err)
			}
			if _, e := os.Stat(filepath.Join(source, "content")); e != nil {
				t.Fatal("lost source", e)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	destination := filepath.Join(dir, "validation")
	root := filepath.Join(destination, "RayleaBot-v1.2.3-windows-x64-full")
	writeFixture(t, filepath.Join(root, "content"), "kept")
	compact, e := CompactRoot(root, destination, "windows")
	if e != nil {
		t.Fatal(e)
	}
	if filepath.Dir(compact) != dir || !strings.HasPrefix(filepath.Base(compact), "r-") {
		t.Fatal(compact)
	}
	b, e := os.ReadFile(filepath.Join(compact, "content"))
	if e != nil || string(b) != "kept" {
		t.Fatal(e)
	}
}
func TestVersionsMatchSchemaAndDatabaseSource(t *testing.T) {
	versions, e := Versions(repo.Root())
	if e != nil {
		t.Fatal(e)
	}
	if versions["database"] == "" {
		t.Fatal("missing database version")
	}
	config, e := contractdata.Value(repo.Root(), "config.user.schema.json", "/properties/schema_version/const")
	if e != nil || versions["config"] != config {
		t.Fatal(versions, e)
	}
}
