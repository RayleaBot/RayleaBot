package render

import "testing"

func TestRuntimeConfigClearedFooterRestoresDefault(t *testing.T) {
	configuration := newRuntimeConfig(1024, "first", "png", 100)
	configuration.update(RuntimeConfig{FooterTemplate: "", DefaultOutput: "png", DeviceScalePercent: 100})
	if got := configuration.snapshot().footerTemplate; got != defaultRenderFooter {
		t.Fatalf("cleared footer did not restore default: %q", got)
	}
}
