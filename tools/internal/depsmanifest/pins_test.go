package depsmanifest

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
)

func TestRuntimeSourcesUsePinnedBuildsAndMatchingEntrypoints(t *testing.T) {
	manifest, e := Read(filepath.Join(repo.Root(), ".deps/manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	tags := map[string]bool{}
	pattern := regexp.MustCompile(`/autobuild-(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})/([^/]+)$`)
	buildVersion := regexp.MustCompile(`^n9[.]0[.]\d+-\d+-g[0-9a-f]+$`)
	for _, value := range manifest["resources"].([]any) {
		r := value.(map[string]any)
		sources := r["sources"].([]any)
		switch r["kind"] {
		case "ffmpeg":
			for _, v := range sources {
				if strings.Contains(v.(map[string]any)["url"].(string), "/latest/") {
					t.Fatal("mutable FFmpeg source")
				}
			}
			if r["platform"] == "macos-arm64" {
				continue
			}
			raw := sources[0].(map[string]any)["url"].(string)
			m := pattern.FindStringSubmatch(raw)
			if m == nil {
				t.Fatal("FFmpeg source lacks pinned build", raw)
			}
			year, _ := strconv.Atoi(m[1])
			month, _ := strconv.Atoi(m[2])
			day, _ := strconv.Atoi(m[3])
			if day != time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day() {
				t.Fatal("FFmpeg retention pin is not month-end", raw)
			}
			suffix := "-" + m[1] + m[2] + m[3]
			if strings.Contains(m[6], "-gpl-shared-") {
				suffix += "-shared"
			}
			version := r["version"].(string)
			if !strings.HasSuffix(version, suffix) || !strings.HasPrefix(m[6], "ffmpeg-"+strings.TrimSuffix(version, suffix)+"-") {
				t.Fatal("cache version does not identify archive", version, raw)
			}
			if !buildVersion.MatchString(strings.TrimSuffix(version, suffix)) {
				t.Fatal("FFmpeg build version left the fixed version line", version)
			}
			root := strings.TrimSuffix(strings.TrimSuffix(m[6], ".zip"), ".tar.xz")
			for _, v := range r["entrypoints"].(map[string]any) {
				for _, candidate := range v.([]any) {
					if !strings.HasPrefix(candidate.(string), root+"/bin/") {
						t.Fatal("entrypoint not in pinned archive", candidate)
					}
				}
			}
			tags[strings.Join(m[1:6], "-")] = true
		case "chromium":
			upstream, mirror := false, false
			for _, v := range sources {
				u, e := url.Parse(v.(map[string]any)["url"].(string))
				if e != nil {
					t.Fatal(e)
				}
				upstream = upstream || u.Scheme == "https" && u.Hostname() == "storage.googleapis.com" && strings.HasPrefix(u.Path, "/chrome-for-testing-public/")
				mirror = mirror || u.Scheme == "https" && u.Hostname() == "npmmirror.com" && strings.HasPrefix(u.Path, "/mirrors/chrome-for-testing/")
			}
			if !upstream || !mirror {
				t.Fatal("Chromium source fallback lost", r["platform"])
			}
		}
	}
	if len(tags) != 1 {
		t.Fatal("FFmpeg platforms use different builds", tags)
	}
}
