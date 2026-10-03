package runtime

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type configBatchSecretStore struct {
	*memorySecretStore
	applyCalls int
	applyHook  func(map[string][]byte, []string) error
	writeError map[string]error
	writes     []string
}

func newConfigBatchSecretStore() *configBatchSecretStore {
	return &configBatchSecretStore{memorySecretStore: newMemorySecretStore()}
}

func (s *configBatchSecretStore) Apply(ctx context.Context, values map[string][]byte, deleted []string) error {
	s.applyCalls++
	if err := ctx.Err(); err != nil {
		return err
	}
	for key, value := range values {
		if err := s.memorySecretStore.Set(ctx, key, value); err != nil {
			return err
		}
	}
	for _, key := range deleted {
		if err := s.memorySecretStore.Delete(ctx, key); err != nil {
			return err
		}
	}
	if s.applyHook != nil {
		return s.applyHook(values, deleted)
	}
	return nil
}

func (s *configBatchSecretStore) Set(ctx context.Context, key string, value []byte) error {
	s.writes = append(s.writes, key)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.writeError[key]; err != nil {
		return err
	}
	return s.memorySecretStore.Set(ctx, key, value)
}

func (s *configBatchSecretStore) Delete(ctx context.Context, key string) error {
	s.writes = append(s.writes, key)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.writeError[key]; err != nil {
		return err
	}
	return s.memorySecretStore.Delete(ctx, key)
}

func TestConfigSecretBatchPreservesOwnedValuesAndUnrelatedKeys(t *testing.T) {
	ctx := context.Background()
	base := newConfigBatchSecretStore()
	base.values["config.deleted"] = []byte("old")
	base.values["plugin:fixture:secret:token"] = []byte("unrelated")
	staged := newStagedSecrets(base)
	value := []byte("fixture-new")
	if err := staged.Set(ctx, "config.saved", value); err != nil {
		t.Fatal(err)
	}
	value[0] = 'X'
	read, err := staged.Get(ctx, "config.saved")
	if err != nil {
		t.Fatal(err)
	}
	read[0] = 'Y'
	if err := staged.Delete(ctx, "config.deleted"); err != nil {
		t.Fatal(err)
	}
	saved := false
	if err := staged.persist(ctx, func() error {
		saved = true
		if string(base.values["config.saved"]) != "fixture-new" {
			t.Fatal("document save preceded credential commit or received an aliased value")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !saved || base.applyCalls != 1 || len(base.writes) != 0 || string(base.values["plugin:fixture:secret:token"]) != "unrelated" {
		t.Fatal("batch save changed unrelated credentials or used single writes")
	}
	if _, exists := base.values["config.deleted"]; exists {
		t.Fatal("batch save lost the requested deletion")
	}
}

func TestConfigSecretBatchFailureCompensatesEveryCandidate(t *testing.T) {
	applyFailure := errors.New("fixture committed batch failure")
	documentFailure := errors.New("fixture document failure")
	restoreFailure := errors.New("fixture restore failure")
	for _, mode := range []string{"committed error", "cancelled after commit", "document error", "restore error"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			base := newConfigBatchSecretStore()
			base.values["a-existing"] = []byte("old-a")
			base.values["b-deleted"] = []byte("old-b")
			base.values["plugin:fixture:secret:token"] = []byte("unrelated")
			staged := newStagedSecrets(base)
			if err := staged.Set(ctx, "a-existing", []byte("new-a")); err != nil {
				t.Fatal(err)
			}
			if err := staged.Delete(ctx, "b-deleted"); err != nil {
				t.Fatal(err)
			}
			if err := staged.Set(ctx, "c-created", []byte("new-c")); err != nil {
				t.Fatal(err)
			}
			base.applyHook = func(values map[string][]byte, _ []string) error {
				// A batch writer does not own the earlier compensation snapshot.
				values["a-existing"][0] = 'X'
				switch mode {
				case "committed error":
					cancel()
					return applyFailure
				case "cancelled after commit":
					cancel()
				case "restore error":
					base.writeError = map[string]error{"b-deleted": restoreFailure}
				}
				return nil
			}
			saved := false
			err := staged.persist(ctx, func() error { saved = true; return documentFailure })
			wantError := documentFailure
			if mode == "committed error" {
				wantError = applyFailure
			} else if mode == "cancelled after commit" {
				wantError = context.Canceled
			}
			if !errors.Is(err, wantError) || saved != (mode == "document error" || mode == "restore error") {
				t.Fatalf("batch failure/save result = %v, %v", err, saved)
			}
			if !reflect.DeepEqual(base.writes, []string{"c-created", "b-deleted", "a-existing"}) || base.applyCalls != 1 {
				t.Fatalf("compensation did not attempt all keys in reverse order: %v", base.writes)
			}
			if string(base.values["a-existing"]) != "old-a" || string(base.values["plugin:fixture:secret:token"]) != "unrelated" {
				t.Fatal("compensation lost an earlier or unrelated credential")
			}
			if _, exists := base.values["c-created"]; exists {
				t.Fatal("compensation retained a newly created credential")
			}
			if mode == "restore error" {
				if !errors.Is(err, restoreFailure) {
					t.Fatal("compensation failure was not joined with the document failure")
				}
			} else if string(base.values["b-deleted"]) != "old-b" {
				t.Fatal("compensation did not restore a deleted credential")
			}
		})
	}
}

func TestConfigSecretEmptyBatchRetainsCancellationAndDocumentSave(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		base := newConfigBatchSecretStore()
		staged := newStagedSecrets(base)
		if cancelled {
			cancel()
		}
		saved := false
		err := staged.persist(ctx, func() error { saved = true; return nil })
		cancel()
		if (err != nil) != cancelled || saved == cancelled || base.applyCalls != 0 || len(base.writes) != 0 {
			t.Fatalf("empty batch changed save or cancellation: error=%v saved=%v", err, saved)
		}
	}
}
