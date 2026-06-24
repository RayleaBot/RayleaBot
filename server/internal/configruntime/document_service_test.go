package configruntime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

func TestUpdateConfigDocumentUsesRequestContextForSecrets(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	cfg, summary, err := internalconfig.Init(configPath, "")
	if err != nil {
		t.Fatalf("init config: %v", err)
	}
	request := ConfigDocumentFromTyped(cfg)
	setConfigPath(request, []string{"onebot", "forward_ws", "access_token"}, "forward-secret")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := NewService(Deps{
		CurrentConfig: func() internalconfig.Config { return cfg },
		CurrentSummary: func() internalconfig.Summary {
			return summary
		},
		Secrets: contextCheckingSecretStore{},
	})

	_, err = service.UpdateConfigDocument(ctx, request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateConfigDocument error = %v, want context.Canceled", err)
	}
}

type contextCheckingSecretStore struct{}

func (contextCheckingSecretStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if key == "platform.secret_encryption_key" {
		return make([]byte, 32), nil
	}
	return nil, secrets.ErrNotFound
}

func (contextCheckingSecretStore) Set(ctx context.Context, _ string, _ []byte) error {
	return ctx.Err()
}

func (contextCheckingSecretStore) Delete(ctx context.Context, _ string) error {
	return ctx.Err()
}

func (contextCheckingSecretStore) List(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}
