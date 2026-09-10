package render

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Run before the small Chromium test documents in this temporary experiment.
// Use the real template tree and the native release fixture's exact data shape.
func TestDiagnosticRealHelpMenuColdThenSecondRender(t *testing.T) {
	sourceRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	distribution := t.TempDir()
	if err := os.CopyFS(filepath.Join(distribution, "templates"), os.DirFS(filepath.Join(sourceRoot, "templates"))); err != nil {
		t.Fatal(err)
	}
	fonts, err := filepath.Glob(filepath.Join(distribution, "templates", "help.menu", "assets", "fonts", "noto-sans-sc", "*.woff2"))
	if err != nil || len(fonts) == 0 {
		t.Fatalf("real font tree unavailable: count=%d error=%v", len(fonts), err)
	}
	t.Logf("real distribution templates copied; help.menu font files=%d", len(fonts))
	runner := newTestChromiumRunner(t)
	service, err := NewService(Options{
		RepoRoot:    distribution,
		OutputRoot:  filepath.Join(distribution, "data", "render"),
		Store:       openRenderTestStore(t),
		Runner:      runner,
		WorkerCount: 1, QueueMaxLength: 2, QueueWaitTimeout: 30 * time.Second,
		RenderTimeout: 30 * time.Second, MaxRenderDataBytes: 256 * 1024,
		FooterTemplate: "Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}",
		DefaultOutput:  "png", DeviceScalePercent: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, probe := range []string{"first", "second"} {
		t.Run(probe, func(t *testing.T) {
			acceptanceProbe := "initial-0123456789abcdef0123456789abcdef"
			if probe == "second" {
				acceptanceProbe = "reloaded-0123456789abcdef0123456789abcdef"
			}
			runner.tracePhase("help.menu." + probe + ".begin")
			started := time.Now()
			result, err := service.Render(t.Context(), Request{
				Template: "help.menu", Theme: "default", Output: "png",
				Plugin: &PluginContext{Name: "Echo Fixture", Version: "0.2.0"},
				Data: map[string]any{
					"title": "Release acceptance " + acceptanceProbe,
					"items": []map[string]any{{"name": "fixture", "description": "Native SDK render acceptance"}},
				},
			})
			if err != nil {
				t.Fatalf("real help.menu elapsed=%s error=%v cause=%+v", time.Since(started), err, unwrapDiagnosticRenderError(err))
			}
			runner.tracePhase("help.menu." + probe + ".persisted")
			if result.FromCache {
				t.Fatal("second render must run the browser, not return a cached PNG")
			}
			artifact, err := service.LookupArtifact(result.ArtifactID)
			if err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(artifact.Path)
			if err != nil {
				t.Fatal(err)
			}
			image, decodeErr := png.DecodeConfig(file)
			closeErr := file.Close()
			if decodeErr != nil || closeErr != nil {
				t.Fatalf("decode persisted PNG: %v %v", decodeErr, closeErr)
			}
			t.Logf("real help.menu PNG elapsed=%s width=%d height=%d", time.Since(started), image.Width, image.Height)
		})
	}
}

func unwrapDiagnosticRenderError(err error) error {
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return wrapped.Unwrap()
	}
	return err
}
