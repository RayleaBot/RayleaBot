package release

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/tools/internal/ordered"
	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

func mirrorFixture(t *testing.T) *ordered.Object {
	t.Helper()
	doc, e := ordered.Read(filepath.Join(repo.Root(), "fixtures/release-manifest/ok.release-manifest-minimal.json"))
	if e != nil {
		t.Fatal(e)
	}
	return ordered.Obj(doc.Get("input"))
}
func TestMirrorRewritesWithoutMutatingSource(t *testing.T) {
	source := mirrorFixture(t)
	before, _ := ordered.JSON(source, true)
	result, e := RewriteManifest(repo.Root(), source, "https://downloads.example.com/raylea/")
	if e != nil {
		t.Fatal(e)
	}
	after, _ := ordered.JSON(source, true)
	if string(before) != string(after) {
		t.Fatal("mutated source manifest")
	}
	for _, v := range ordered.List(result.Get("artifacts")) {
		a := ordered.Obj(v)
		want := "https://downloads.example.com/raylea/v" + ordered.Str(result.Get("version")) + "/" + ordered.Str(a.Get("file_name"))
		if a.Get("download_url") != want {
			t.Fatal(a)
		}
	}
}
func TestMirrorSemverChannels(t *testing.T) {
	manifests := []*ordered.Object{}
	for _, v := range [][2]string{{"1.9.0", "stable"}, {"1.10.0-beta.2", "beta"}, {"1.10.0-beta.10", "beta"}} {
		manifests = append(manifests, &ordered.Object{{Name: "version", Value: v[0]}, {Name: "channel", Value: v[1]}, {Name: "release_notes_ref", Value: "https://example.com/release"}})
	}
	docs, e := ChannelDocuments(repo.Root(), manifests)
	if e != nil {
		t.Fatal(e)
	}
	if ordered.Obj(docs.Get("stable.json")).Get("version") != "1.9.0" || ordered.Obj(docs.Get("beta.json")).Get("version") != "1.10.0-beta.10" {
		t.Fatal(docs)
	}
	if CompareVersions("1.0.0+build-1", "1.0.0") != 0 || CompareVersions("1.0.0", "1.0.0-beta") <= 0 || CompareVersions("1.0.0-2", "1.0.0-beta") >= 0 {
		t.Fatal("semver precedence")
	}
	if _, e = ChannelDocuments(repo.Root(), nil); e == nil {
		t.Fatal("empty channel set accepted")
	}
}
func TestMirrorAssetsPrecedePointersAndRetriesReuseUploads(t *testing.T) {
	for _, fail := range []bool{true, false} {
		t.Run(map[bool]string{true: "failed upload", false: "retry"}[fail], func(t *testing.T) {
			manifest := mirrorFixture(t)
			manifest.Set("version", "0.4.0")
			for _, v := range ordered.List(manifest.Get("artifacts")) {
				ordered.Obj(v).Set("archive_size_bytes", 1)
			}
			var uploads, downloads []string
			run := func(args ...string) (string, error) {
				switch strings.Join(args[:min(len(args), 3)], " ") {
				case "gh api repos/RayleaBot/RayleaBot/releases?per_page=100":
					return `[{"tag_name":"v0.4.0","draft":false,"assets":[{"name":"release_manifest.v2.json"}]}]`, nil
				case "gh release download":
					name := args[slices.Index(args, "--pattern")+1]
					dir := args[slices.Index(args, "--dir")+1]
					if name == "release_manifest.v2.json" {
						b, _ := ordered.JSON(manifest, false)
						return "", os.WriteFile(filepath.Join(dir, name), b, 0600)
					}
					downloads = append(downloads, name)
					return "", os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600)
				case "aws s3api list-objects-v2":
					items := []map[string]any{}
					if !fail {
						for _, v := range ordered.List(manifest.Get("artifacts")) {
							items = append(items, map[string]any{"Key": "v0.4.0/" + ordered.Str(ordered.Obj(v).Get("file_name")), "Size": 1})
						}
					}
					b, _ := json.Marshal(map[string]any{"Contents": items})
					return string(b), nil
				case "aws s3 cp":
					uploads = append(uploads, args[4])
					if fail {
						return "", errors.New("upload failed")
					}
					return "", nil
				default:
					t.Fatalf("unexpected command %v", args)
					return "", nil
				}
			}
			count, e := SyncMirror(repo.Root(), MirrorOptions{"RayleaBot/RayleaBot", "https://downloads.example.com", "release-fixture", "https://storage.example.com"}, run)
			if fail {
				if e == nil {
					t.Fatal("ignored upload failure")
				}
				for _, p := range uploads {
					if strings.HasSuffix(p, "/stable.json") || strings.HasSuffix(p, "/beta.json") || strings.HasSuffix(p, "/releases.json") {
						t.Fatal("published pointers before successful assets")
					}
				}
			} else {
				if e != nil || count != 1 {
					t.Fatal(count, e)
				}
				if len(downloads) > 0 {
					t.Fatal("redownloaded matching immutable assets")
				}
				for _, name := range []string{"releases.json", "beta.json", "stable.json"} {
					i := slices.Index(uploads, "s3://release-fixture/"+name)
					if i < 1 {
						t.Fatal("pointer missing or precedes version manifest", uploads)
					}
				}
			}
		})
	}
}
func TestMirrorRejectsUnsafeEndpointsBeforeCommands(t *testing.T) {
	for _, base := range []string{"http://example.com", "https://user:password@example.com", "https://example.com/?token=fixture"} {
		_, e := SyncMirror(repo.Root(), MirrorOptions{"repo", base, "bucket", "https://storage.example.com"}, func(...string) (string, error) { t.Fatal("ran command before URL validation"); return "", nil })
		if e == nil {
			t.Fatal("accepted unsafe public URL")
		}
	}
}
