package main

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestWebViewRuntimeDetection(t *testing.T) {
	for _, value := range []string{"", "0.0.0.0", "null", "1.2.3", "-1.0.0.0", "a.b.c.d"} {
		if validWebViewVersion(value) {
			t.Errorf("invalid installed version accepted: %q", value)
		}
	}
	if webViewRuntimeInstalled(func(registry.Key, uint32) string { return "0.0.0.0" }) {
		t.Fatal("missing runtime accepted")
	}
	for _, hive := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		if !webViewRuntimeInstalled(func(candidate registry.Key, view uint32) string {
			if candidate == hive && view == registry.WOW64_32KEY {
				return "152.0.1.1"
			}
			return ""
		}) {
			t.Fatalf("runtime in hive %v was missed", hive)
		}
	}
}
