package diagnostics

import (
	"errors"
	"strings"
	"testing"
)

func TestLongPathsDoctorIssue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		value        uint64
		readErr      error
		wantCode     string
		wantSeverity string
	}{
		{name: "enabled", value: 1, wantCode: "windows.long_paths_enabled", wantSeverity: "ok"},
		{name: "disabled", value: 0, wantCode: "windows.long_paths_disabled", wantSeverity: "warning"},
		{name: "read failure", readErr: errors.New("access denied"), wantCode: "windows.long_paths_unavailable", wantSeverity: "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			issue := longPathsIssue(tt.value, tt.readErr)
			if issue.Code != tt.wantCode || issue.Severity != tt.wantSeverity {
				t.Fatalf("longPathsIssue() = %#v, want code %q and severity %q", issue, tt.wantCode, tt.wantSeverity)
			}
			if issue.Summary == "" {
				t.Fatal("longPathsIssue() must provide a summary")
			}
			if tt.wantSeverity == "warning" && (!strings.Contains(issue.Remediation, "LongPathsEnabled") || !strings.Contains(issue.Remediation, "重启")) {
				t.Fatalf("warning remediation must explain the registry setting and restart requirement: %#v", issue)
			}
		})
	}
}
