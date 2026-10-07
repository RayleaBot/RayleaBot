package release

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	d "github.com/RayleaBot/RayleaBot/tools/internal/ordered"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

func workflow(t *testing.T, name string) *d.Object {
	t.Helper()
	v, e := d.Read(filepath.Join(repo.Root(), ".github/workflows", name))
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func at(t *testing.T, v any, pointer string) any {
	t.Helper()
	value, e := d.At(v, pointer)
	if e != nil {
		t.Fatal(e)
	}
	return value
}
func step(t *testing.T, job any, name string) *d.Object {
	t.Helper()
	for _, v := range d.List(d.Obj(job).Get("steps")) {
		s := d.Obj(v)
		if s.Get("name") == name {
			return s
		}
	}
	t.Fatalf("step missing: %s", name)
	return nil
}
func TestReleaseWorkflowPermissionsAndGates(t *testing.T) {
	release, build := workflow(t, "release.yml"), workflow(t, "release-build.yml")
	if at(t, release, "/jobs/build/uses") != "./.github/workflows/release-build.yml" || at(t, release, "/jobs/build/needs") != "version" {
		t.Fatal("release build bypasses version gate")
	}
	if !reflect.DeepEqual(d.Strings(at(t, release, "/on/push/tags")), []string{"v*"}) || len(*d.Obj(release.Get("on"))) != 1 {
		t.Fatal("publication trigger changed")
	}
	if at(t, release, "/permissions/contents") != "read" || at(t, build, "/permissions/contents") != "read" {
		t.Fatal("build received write permissions")
	}
	if len(*d.Obj(build.Get("on"))) != 1 || !d.Obj(build.Get("on")).Has("workflow_call") {
		t.Fatal("build is not reusable-only")
	}
	if d.Obj(at(t, build, "/on/workflow_call")).Has("secrets") {
		t.Fatal("build receives publication secrets")
	}
	for _, j := range *d.Obj(build.Get("jobs")) {
		job := d.Obj(j.Value)
		for _, v := range *d.Obj(job.Get("permissions")) {
			if v.Value == "write" {
				t.Fatal("write permission in build job")
			}
		}
		for _, v := range d.List(job.Get("steps")) {
			if strings.HasPrefix(d.Str(d.Obj(v).Get("uses")), "softprops/action-gh-release") {
				t.Fatal("build publishes artifacts")
			}
		}
	}
	publisher := at(t, release, "/jobs/publish")
	if at(t, publisher, "/permissions/contents") != "write" || at(t, publisher, "/permissions/actions") != "read" {
		t.Fatal("publication permissions drift")
	}
	needs := d.Strings(d.Obj(publisher).Get("needs"))
	slices.Sort(needs)
	if !slices.Equal(needs, []string{"build", "version"}) {
		t.Fatal("publication bypasses build")
	}
	condition := d.Str(d.Obj(publisher).Get("if"))
	if !strings.Contains(condition, "github.event_name == 'push'") || !strings.Contains(condition, "'refs/tags/v'") {
		t.Fatal("publication tag restriction lost")
	}
	for _, v := range [][2]string{{"version", "Require successful nightly for the release commit"}, {"publish", "Recheck nightly before publication"}} {
		job := at(t, release, "/jobs/"+v[0])
		gate := step(t, job, v[1])
		if !strings.Contains(d.Str(gate.Get("run")), `go run ./tools/cmd/nightly-status check --sha "$(git rev-parse HEAD)"`) || at(t, job, "/permissions/actions") != "read" {
			t.Fatal("nightly gate does not verify the checked-out commit")
		}
		if v[0] == "publish" {
			steps := d.List(d.Obj(job).Get("steps"))
			gateIndex, publishIndex := -1, -1
			for i, s := range steps {
				if d.Obj(s) == gate {
					gateIndex = i
				}
				if d.Obj(s).Get("name") == "Publish GitHub release" {
					publishIndex = i
				}
			}
			if gateIndex < 0 || publishIndex <= gateIndex {
				t.Fatal("nightly gate follows publication")
			}
		}
	}
	if at(t, release, "/jobs/build/with/channel") != "${{ needs.version.outputs.channel }}" || at(t, step(t, publisher, "Publish GitHub release"), "/with/prerelease") != "${{ needs.version.outputs.prerelease == 'true' }}" || at(t, step(t, publisher, "Publish GitHub release"), "/with/make_latest") != "${{ needs.version.outputs.make_latest }}" {
		t.Fatal("publication ignores semver policy")
	}
	if at(t, build, "/on/workflow_call/inputs/channel/required") != true || at(t, build, "/env/RELEASE_CHANNEL") != "${{ inputs.channel }}" {
		t.Fatal("build channel not required")
	}
	assemble := at(t, build, "/jobs/assemble")
	metadata := d.Str(step(t, assemble, "Generate release metadata").Get("run"))
	if !strings.Contains(metadata, `--channel "$RELEASE_CHANNEL"`) {
		t.Fatal("metadata channel lost")
	}
	reporter := workflow(t, "nightly-status.yml")
	if at(t, reporter, "/permissions/issues") != "write" || at(t, reporter, "/concurrency/cancel-in-progress") != false || at(t, step(t, at(t, reporter, "/jobs/reconcile"), "Checkout trusted default branch"), "/with/ref") != "${{ github.event.repository.default_branch }}" {
		t.Fatal("nightly reporter trust boundary changed")
	}
}
func TestNativeBuildMatrixAndMetadataInventory(t *testing.T) {
	build := workflow(t, "release-build.yml")
	full, server := at(t, build, "/jobs/build-full"), at(t, build, "/jobs/build-linux-server")
	matrix := d.List(at(t, full, "/strategy/matrix/include"))
	expected := map[string]string{"windows-x64-full": "windows-latest", "linux-x64-full": "ubuntu-latest", "macos-arm64-full": "macos-26"}
	for _, v := range matrix {
		m := d.Obj(v)
		id := d.Str(m.Get("artifact_id"))
		if expected[id] != m.Get("runner") {
			t.Fatal("artifact not on native runner", m)
		}
		delete(expected, id)
	}
	if len(expected) != 0 || at(t, server, "/runs-on") != "ubuntu-latest" {
		t.Fatal("missing native target")
	}
	if d.Obj(build.Get("env")).Has("CGO_ENABLED") {
		t.Fatal("desktop cgo disabled globally")
	}
	for _, v := range []struct {
		job  any
		name string
	}{{full, "Package full artifact"}, {server, "Package linux server artifact"}} {
		if at(t, step(t, v.job, "Build server"), "/env/CGO_ENABLED") != "0" {
			t.Fatal("server artifact needs CGO")
		}
		command := d.Str(step(t, v.job, v.name).Get("run"))
		for _, option := range []string{"--run-smoke", "--evidence-dir"} {
			if !strings.Contains(command, option) {
				t.Fatal("missing package validation", option)
			}
		}
		evidence := step(t, v.job, "Upload validation evidence")
		if evidence.Get("if") != "always()" || !strings.HasPrefix(d.Str(at(t, evidence, "/with/name")), "validation-evidence-") {
			t.Fatal("failed validation evidence lost")
		}
		if at(t, step(t, v.job, "Build web"), "/env/RAYLEA_BUILD_VERSION") != "${{ inputs.version }}" {
			t.Fatal("Web build version lost")
		}
	}
	if at(t, step(t, full, "Build launcher"), "/env/RAYLEA_BUILD_VERSION") != "${{ inputs.version }}" {
		t.Fatal("Launcher build version lost")
	}
	assemble := at(t, build, "/jobs/assemble")
	if at(t, step(t, assemble, "Download release artifacts"), "/with/pattern") != "package-*" {
		t.Fatal("metadata consumes non-package artifacts")
	}
	command := d.Str(step(t, assemble, "Generate release metadata").Get("run"))
	artifacts, e := Matrix(repo.Root())
	if e != nil {
		t.Fatal(e)
	}
	for id := range artifacts {
		if !strings.Contains(command, "dist/downloads/package-"+id+"/") {
			t.Fatal("artifact omitted from metadata", id)
		}
	}
	if strings.TrimSpace(d.Str(at(t, step(t, assemble, "Upload release metadata"), "/with/path"))) != "dist/release/release_manifest.v2.json" {
		t.Fatal("wrong release metadata upload")
	}
	for _, field := range []string{"config", "database", "plugin-protocol"} {
		if !strings.Contains(command, "release-tool versions --field "+field) {
			t.Fatal("workflow bypasses version reader", field)
		}
	}
}
func TestNativePackagingShellPreservesArguments(t *testing.T) {
	bash, e := exec.LookPath("bash")
	if runtime.GOOS == "windows" {
		candidate := filepath.Join(os.Getenv("ProgramFiles"), "Git/bin/bash.exe")
		if _, err := os.Stat(candidate); err == nil {
			bash, e = candidate, nil
		}
	}
	if e != nil {
		t.Skip("bash unavailable")
	}
	job := at(t, workflow(t, "release-build.yml"), "/jobs/build-full")
	for _, v := range d.List(at(t, job, "/strategy/matrix/include")) {
		platform := d.Obj(v)
		t.Run(d.Str(platform.Get("artifact_id")), func(t *testing.T) {
			command := d.Str(step(t, job, "Package full artifact").Get("run"))
			for _, f := range *platform {
				command = strings.ReplaceAll(command, "${{ matrix."+f.Name+" }}", d.Str(f.Value))
			}
			command = strings.ReplaceAll(command, "go run ./tools/cmd/package-artifact", "capture")
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bash, "--noprofile", "--norc", "-c", `capture() { printf "%s\n" "$@"; };`+"\n"+command)
			cmd.Dir = repo.Root()
			cmd.Env = append(os.Environ(), "RELEASE_VERSION=0.4.0", "RELEASE_NOTES_REF=https://example.invalid/notes")
			output, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatalf("%v: %s", e, output)
			}
			args := strings.Split(strings.TrimSpace(string(output)), "\n")
			index, count := -1, 0
			for i, v := range args {
				if v == "--artifact-id" {
					index = i
					count++
				}
				if strings.TrimSpace(v) == "" {
					t.Fatal("empty package argument")
				}
			}
			if count != 1 || index+1 >= len(args) || strings.TrimSpace(args[index+1]) != platform.Get("artifact_id") {
				t.Fatal(fmt.Sprint(args))
			}
		})
	}
}
