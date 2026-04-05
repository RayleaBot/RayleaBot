package redact

import "testing"

func TestRedactorNormalizesValuesAndRedactsMatches(t *testing.T) {
	t.Parallel()

	redactor := New(" token-secret ", "tiny", "token-secret", "longer-secret")
	redactor.Add("other-secret")

	if got := redactor.Redact("token-secret longer-secret other-secret"); got != "[REDACTED] [REDACTED] [REDACTED]" {
		t.Fatalf("redacted text = %q", got)
	}
	if len(redactor.values) != 4 {
		t.Fatalf("values length = %d, want 4", len(redactor.values))
	}
	if redactor.values[0] != "longer-secret" {
		t.Fatalf("expected longest value first, got %#v", redactor.values)
	}
}
