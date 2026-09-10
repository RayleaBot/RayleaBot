package render

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestRenderUsesOneOutputSettingsSnapshotDuringReload(t *testing.T) {
	root := t.TempDir()
	writeRenderTemplateSeed(t, filepath.Join(root, "templates"), "help.menu")
	runner := &fakeRunner{}
	service, err := NewService(Options{RepoRoot: root, OutputRoot: filepath.Join(root, "output"),
		Store: openRenderTestStore(t), Runner: runner, FooterTemplate: "png-family", DefaultOutput: "png", DeviceScalePercent: 100})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			service.UpdateRuntimeConfig(RuntimeConfig{FooterTemplate: "png-family", DefaultOutput: "png", DeviceScalePercent: 100})
			service.UpdateRuntimeConfig(RuntimeConfig{FooterTemplate: "jpeg-family", DefaultOutput: "jpeg", DeviceScalePercent: 200})
			runtime.Gosched()
		}
	})
	defer func() { close(stop); writer.Wait() }()
	for index := range 30 {
		_, err := service.Render(context.Background(), Request{Template: "help.menu", Data: map[string]any{
			"title": fmt.Sprintf("Snapshot %d", index), "items": []map[string]any{{"name": "fixture", "description": "snapshot"}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		doc, _ := runner.lastDocument()
		wantScale := 1.0
		if doc.Output == "jpeg" {
			wantScale = 2
		}
		if doc.DeviceScaleFactor != wantScale || !strings.Contains(doc.HTML, doc.Output+"-family") {
			t.Fatalf("render mixed output settings: output=%s scale=%v", doc.Output, doc.DeviceScaleFactor)
		}
	}
}
