package configruntime

import (
	"context"
	"errors"
	"testing"
)

type failingSecretWriter struct {
	*memorySecretStore
	cancel context.CancelFunc
}

func (s *failingSecretWriter) Set(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "z-new" {
		// A store may report failure after its underlying write completed.
		_ = s.memorySecretStore.Set(ctx, key, value)
		if s.cancel != nil {
			s.cancel()
		}
		return errors.New("fixture write failure")
	}
	return s.memorySecretStore.Set(ctx, key, value)
}

func TestFailedCredentialWriteRestoresEarlierValuesAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := &failingSecretWriter{memorySecretStore: newMemorySecretStore(), cancel: cancel}
	base.values["a-existing"] = []byte("fixture-old")
	staged := newStagedSecrets(base)
	if err := staged.Set(ctx, "a-existing", []byte("fixture-new")); err != nil {
		t.Fatal(err)
	}
	if err := staged.Set(ctx, "z-new", []byte("fixture-created")); err != nil {
		t.Fatal(err)
	}
	saved := false
	err := staged.persist(ctx, func() error { saved = true; return nil })
	if err == nil || saved || ctx.Err() == nil {
		t.Fatalf("write failure did not prevent document save: error=%v saved=%v", err, saved)
	}
	if string(base.values["a-existing"]) != "fixture-old" {
		t.Fatal("existing credential was not restored after request cancellation")
	}
	if _, exists := base.values["z-new"]; exists {
		t.Fatal("partially written credential was not removed")
	}
}
