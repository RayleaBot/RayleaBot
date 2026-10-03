package redact

import (
	"fmt"
	"sync"
	"testing"
)

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

func TestRedactorKeepsRegisteredSecretsDuringConcurrentUpdates(t *testing.T) {
	redactor := New("fixture-initial-secret")
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for index := range 32 {
				secret := fmt.Sprintf("fixture-added-%d-%d-secret", worker, index)
				redactor.Add(secret)
				if got := redactor.Redact("fixture-initial-secret " + secret); got != "[REDACTED] [REDACTED]" {
					t.Errorf("registered secret disclosed during updates: %q", got)
				}
			}
		})
	}
	workers.Wait()
}
