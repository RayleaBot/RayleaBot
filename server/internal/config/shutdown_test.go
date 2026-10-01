package config

import (
	"testing"
	"time"
)

func TestShutdownBudgetUsesConfiguredGrace(t *testing.T) {
	for _, tc := range []struct {
		grace int
		want  int64
	}{{0, 57}, {1, 39}, {10, 57}, {60, 157}} {
		budgets := (RuntimeConfig{ShutdownGraceSeconds: tc.grace}).ShutdownBudgets()
		if got := budgets.TotalSeconds(); got != tc.want {
			t.Fatalf("grace=%d total=%d want=%d", tc.grace, got, tc.want)
		}
		if budgets.PluginGrace < time.Second || budgets.DispatchDrain != budgets.PluginGrace {
			t.Fatalf("budgets=%#v", budgets)
		}
	}
	if grace := (RuntimeConfig{ShutdownGraceSeconds: int(^uint(0) >> 1)}).PluginShutdownGrace(); grace <= 0 {
		t.Fatal("overflowed shutdown grace")
	}
}
