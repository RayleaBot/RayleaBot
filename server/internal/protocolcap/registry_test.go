package protocolcap

import "testing"

func TestOneBot11CompatibilityMatrixKeepsFrozenCategories(t *testing.T) {
	t.Parallel()

	matrix := OneBot11CompatibilityMatrix()
	if matrix.Protocol != "onebot11" {
		t.Fatalf("unexpected protocol: got %q want %q", matrix.Protocol, "onebot11")
	}

	wantCategories := []string{"events", "message_segments", "read_capabilities", "provider_extensions"}
	if len(matrix.Categories) != len(wantCategories) {
		t.Fatalf("unexpected category count: got %d want %d", len(matrix.Categories), len(wantCategories))
	}
	for index, key := range wantCategories {
		if matrix.Categories[index].Key != key {
			t.Fatalf("unexpected category at %d: got %q want %q", index, matrix.Categories[index].Key, key)
		}
	}
}

func TestOneBot11CompatibilityMatrixKeepsProviderGapsExplicit(t *testing.T) {
	t.Parallel()

	matrix := OneBot11CompatibilityMatrix()
	for _, category := range matrix.Categories {
		if category.Key != "provider_extensions" {
			continue
		}
		for _, item := range category.Items {
			if item.Key != "provider.napcat.group.sign.set" {
				continue
			}
			if item.Support.NapCat != "supported" {
				t.Fatalf("unexpected NapCat support: %#v", item.Support)
			}
			if item.Support.Standard != "unsupported" || item.Support.LuckyLillia != "unsupported" {
				t.Fatalf("unsupported provider gaps should remain explicit: %#v", item.Support)
			}
			return
		}
	}

	t.Fatal("expected provider.napcat.group.sign.set in provider extension matrix")
}
