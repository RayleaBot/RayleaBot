package doclinks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinks(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		failures   int
	}{
		{"relative anchors and encoded image", "[target](target.md#target-heading) ![image](sample%20image.png)", 0},
		{"missing path", "[missing](missing.md)", 1},
		{"missing anchor", "[missing](target.md#absent)", 1},
		{"reference image", "![missing][asset]\n[asset]: missing.png", 1},
		{"code", "```md\n[missing](fence.md)\n```\n    [missing](indented.md)\n`[missing](inline.md)`", 0},
		{"external URI", "[valid](https://example.com/a) [bad](https:///missing-host)", 1},
		{"case sensitivity", "[target](Target.md)", 1},
		{"escape repository", "[outside](../outside.md)", 1},
		{"HTML and repeated headings", "<a href='target.md#target-heading-1'>a</a> [explicit](target.md#custom)", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.md")
			for path, content := range map[string]string{"source.md": tc.body, "target.md": "# Target heading\n# Target heading\n<a id='custom'></a>\n", "sample image.png": "image"} {
				if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			failures, err := Check(root, []string{source})
			if err != nil || len(failures) != tc.failures {
				t.Fatalf("failures=%v err=%v", failures, err)
			}
		})
	}
}
