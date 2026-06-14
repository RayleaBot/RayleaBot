package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

const sessionSigningKeySecret = "platform.auth.session_signing_key"

func EnsureSessionSigningKey(ctx context.Context, secretStore secrets.Store) ([]byte, bool, error) {
	if secretStore == nil {
		return nil, false, errors.New("secret store is required")
	}

	signingKey, err := secretStore.Get(ctx, sessionSigningKeySecret)
	switch {
	case err == nil:
		if len(signingKey) == 0 {
			return nil, false, fmt.Errorf("secret %q is empty", sessionSigningKeySecret)
		}
		return signingKey, false, nil
	case !errors.Is(err, secrets.ErrNotFound):
		return nil, false, fmt.Errorf("load session signing key: %w", err)
	}

	signingKey = make([]byte, 32)
	if _, err := rand.Read(signingKey); err != nil {
		return nil, false, fmt.Errorf("generate session signing key: %w", err)
	}
	if err := secretStore.Set(ctx, sessionSigningKeySecret, signingKey); err != nil {
		return nil, false, fmt.Errorf("persist session signing key: %w", err)
	}

	return signingKey, true, nil
}
