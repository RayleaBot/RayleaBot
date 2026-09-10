//go:build windows

package render

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileURLPreservesWindowsVolumesAndEscapesNames(t *testing.T) {
	for _, path := range []string{`C:\render cache\图片 #%.png`, `Z:\other drive\image.png`} {
		t.Run(path[:1], func(t *testing.T) {
			value := fileURL(path)
			parsed, err := url.Parse(value)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Scheme != "file" || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
				t.Fatalf("filesystem path became URL authority or query: %s", value)
			}
			if !strings.HasPrefix(value, "file:///"+path[:2]+"/") || windowsFileURLPath(parsed) != filepath.Clean(path) {
				t.Fatalf("drive or filename lost in round trip: %s -> %s", path, value)
			}
			base, err := url.Parse(BaseURL(filepath.Dir(path)))
			if err != nil {
				t.Fatal(err)
			}
			resolved := base.ResolveReference(&url.URL{Path: filepath.Base(path)})
			if windowsFileURLPath(resolved) != filepath.Clean(path) {
				t.Fatalf("template base resolved on a different volume: %s", resolved)
			}
		})
	}
}
