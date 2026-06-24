package thirdparty

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func normalizePlatform(value string) (string, error) {
	platform := strings.TrimSpace(strings.ToLower(value))
	for _, supported := range SupportedPlatforms() {
		if platform == supported {
			return platform, nil
		}
	}
	return "", fmt.Errorf("%w: unsupported platform", ErrInvalidAccount)
}

func NormalizePlatform(value string) (string, error) {
	return normalizePlatform(value)
}

func normalizeAccountID(value string) (string, error) {
	accountID := strings.TrimSpace(strings.ToLower(value))
	if !accountIDPattern.MatchString(accountID) {
		return "", fmt.Errorf("%w: invalid account id", ErrInvalidAccount)
	}
	return accountID, nil
}

func NormalizeAccountID(value string) (string, error) {
	return normalizeAccountID(value)
}

func secretKeyFor(platform, accountID string) string {
	return "third_party:" + platform + ":" + accountID + ":cookie"
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (profile AccountProfile) normalized() AccountProfile {
	return AccountProfile{
		UID:       strings.TrimSpace(profile.UID),
		Nickname:  strings.TrimSpace(profile.Nickname),
		AvatarURL: strings.TrimSpace(profile.AvatarURL),
	}
}

func (status CredentialStatus) normalized() CredentialStatus {
	status.State = normalizeCredentialState(status.State)
	status.LastError = strings.TrimSpace(status.LastError)
	if status.CheckedAt != nil {
		checkedAt := status.CheckedAt.UTC()
		status.CheckedAt = &checkedAt
	}
	return status
}

func normalizeCredentialState(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case CredentialValid:
		return CredentialValid
	case CredentialInvalid:
		return CredentialInvalid
	default:
		return CredentialUnknown
	}
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func parseOptionalTime(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	parsed := parseTime(value.String)
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func (s *Service) secretConfigured(ctx context.Context, key string) bool {
	key = strings.TrimSpace(key)
	if key == "" || s == nil || s.secrets == nil {
		return false
	}
	value, err := s.secrets.Get(ctx, key)
	return err == nil && len(value) > 0
}
