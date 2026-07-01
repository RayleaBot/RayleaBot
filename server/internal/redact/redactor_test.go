package redact

import "testing"

func TestRedactorNormalizesValuesAndRedactsMatches(t *testing.T) {
	t.Parallel()

	redactor := New(" token-secret ", "tiny", "token-secret", "longer-secret")
	redactor.Add("other-secret")

	if got := redactor.Redact("token-secret longer-secret other-secret"); got != "[REDACTED] [REDACTED] [REDACTED]" {
		t.Fatalf("redacted text = %q", got)
	}
	if got := redactor.Redact("tiny token-secret"); got != "[REDACTED] [REDACTED]" {
		t.Fatalf("redacted text with short value = %q", got)
	}
}

func TestRedactorPrefersLongerOverlappingSecrets(t *testing.T) {
	t.Parallel()

	redactor := New("token", "token-secret")

	if got := redactor.Redact("token-secret"); got != "[REDACTED]" {
		t.Fatalf("redacted overlapping secret = %q", got)
	}
}
