package logging

import "testing"

func TestNormalizeSummaryDerivesProtocolFromSource(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		source   string
		expected string
	}{
		{name: "adapter", source: "adapter", expected: ProtocolOneBot11},
		{name: "adapter.onebot11", source: "adapter.onebot11", expected: ProtocolOneBot11},
		{name: "bridge", source: "bridge", expected: ProtocolOneBot11},
		{name: "runtime", source: "runtime", expected: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			summary := NormalizeSummary(Summary{
				Source:  tc.source,
				Message: "test",
				Level:   "info",
			})
			if summary.Protocol != tc.expected {
				t.Fatalf("unexpected protocol: got %q want %q", summary.Protocol, tc.expected)
			}
		})
	}
}
