package render

import (
	"sync"
	"testing"
)

func TestConcurrentRenderSettingsRemainOneConfiguration(t *testing.T) {
	configuration := newRuntimeConfig(1024, "first", "png", 100)
	service := &Service{config: configuration, repoRoot: t.TempDir()}
	var writers sync.WaitGroup
	writers.Go(func() {
		for range 1000 {
			configuration.update(RuntimeConfig{FooterTemplate: "second", DefaultOutput: "jpeg", DeviceScalePercent: 200})
			configuration.update(RuntimeConfig{FooterTemplate: "first", DefaultOutput: "png", DeviceScalePercent: 100})
		}
	})
	defer writers.Wait()
	for range 1000 {
		settings := configuration.snapshot()
		request, _, err := service.normalizeRequestWithSettings(Request{Template: "fixture"}, settings)
		if err != nil {
			t.Fatal(err)
		}
		footer := request.Data["render_footer"]
		first := footer == "first" && request.Output == "png" && settings.deviceScalePercent == 100
		second := footer == "second" && request.Output == "jpeg" && settings.deviceScalePercent == 200
		if !first && !second {
			t.Fatalf("render mixed revisions: footer=%v output=%s scale=%d", footer, request.Output, settings.deviceScalePercent)
		}
	}
	writers.Wait()
	configuration.update(RuntimeConfig{FooterTemplate: "", DefaultOutput: "png", DeviceScalePercent: 100})
	if got := configuration.snapshot().footerTemplate; got != defaultRenderFooter {
		t.Fatalf("cleared footer did not restore default: %q", got)
	}
}
