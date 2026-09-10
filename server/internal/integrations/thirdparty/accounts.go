package thirdparty

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

const (
	PlatformBilibili     = "bilibili"
	PlatformWeibo        = "weibo"
	PlatformDouyin       = "douyin"
	PlatformNeteaseMusic = "netease_music"
)

var accountIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_.-]{0,62}[a-z0-9])?$`)

var (
	ErrInvalidAccount  = errors.New("invalid third-party account")
	ErrAccountNotFound = errors.New("third-party account not found")
)

const (
	CredentialUnknown = "unknown"
	CredentialValid   = "valid"
	CredentialInvalid = "invalid"
)

type Account struct {
	Platform   string
	AccountID  string
	Label      string
	Enabled    bool
	Configured bool
	SecretKey  string
	Profile    AccountProfile
	Credential CredentialStatus
	UpdatedAt  time.Time
}

type UpsertRequest struct {
	Platform   string
	AccountID  string
	Label      string
	Enabled    bool
	Cookie     string
	Profile    AccountProfile
	Credential CredentialStatus
	Validate   func(context.Context, string) (AccountProfile, CredentialStatus, error)
}

type AccountProfile struct {
	UID string
	// UniqueID 是平台用户可见、可被用户修改的标识（如抖音号）；仅用于
	// 展示，解析/订阅绑定必须以 UID 为准。
	UniqueID  string
	Nickname  string
	AvatarURL string
}

// Empty reports whether the profile has no meaningful data.
func (p AccountProfile) Empty() bool {
	return strings.TrimSpace(p.UID) == "" &&
		strings.TrimSpace(p.Nickname) == "" &&
		strings.TrimSpace(p.AvatarURL) == ""
}

type CredentialStatus struct {
	State     string
	CheckedAt *time.Time
	LastError string
}

func SupportedPlatforms() []string {
	return []string{PlatformBilibili, PlatformWeibo, PlatformDouyin, PlatformNeteaseMusic}
}

type Service struct {
	read    *sql.DB
	write   *sql.DB
	secrets secrets.Store
	now     func() time.Time
}

func NewService(store *storage.Store, secretStore secrets.Store) (*Service, error) {
	if store == nil || store.Read == nil || store.Write == nil {
		return nil, errors.New("sqlite store is required")
	}
	if secretStore == nil {
		return nil, errors.New("secret store is required")
	}
	return &Service{
		read:    store.Read,
		write:   store.Write,
		secrets: secretStore,
		now:     func() time.Time { return time.Now().UTC() },
	}, nil
}

func JSONStringValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strings.TrimSpace(strconv.FormatFloat(v, 'f', -1, 64))
	case jsonNumber:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

type jsonNumber interface {
	String() string
}

func MergeAccountProfiles(base, next AccountProfile) AccountProfile {
	if strings.TrimSpace(base.UID) == "" {
		base.UID = strings.TrimSpace(next.UID)
	}
	if strings.TrimSpace(base.Nickname) == "" {
		base.Nickname = strings.TrimSpace(next.Nickname)
	}
	if strings.TrimSpace(base.AvatarURL) == "" {
		base.AvatarURL = strings.TrimSpace(next.AvatarURL)
	}
	return base
}

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

func (s *Service) List(ctx context.Context) ([]Account, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT platform, account_id, label, enabled, secret_key, profile_uid, profile_nickname, profile_avatar_url, credential_state, credential_checked_at, credential_last_error, updated_at FROM third_party_accounts ORDER BY platform ASC, account_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list third-party accounts: %w", err)
	}
	defer func(release func() error) { _ = release() }(rows.Close)

	accounts := []Account{}
	for rows.Next() {
		var account Account
		var enabled int
		var credentialCheckedAt sql.NullString
		var updatedAt string
		if err := rows.Scan(
			&account.Platform,
			&account.AccountID,
			&account.Label,
			&enabled,
			&account.SecretKey,
			&account.Profile.UID,
			&account.Profile.Nickname,
			&account.Profile.AvatarURL,
			&account.Credential.State,
			&credentialCheckedAt,
			&account.Credential.LastError,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan third-party account: %w", err)
		}
		account.Enabled = enabled != 0
		account.Configured = s.secretConfigured(ctx, account.SecretKey)
		account.Credential.State = normalizeCredentialState(account.Credential.State)
		account.Credential.CheckedAt = parseOptionalTime(credentialCheckedAt)
		account.UpdatedAt = parseTime(updatedAt)
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate third-party accounts: %w", err)
	}
	return accounts, nil
}

func (s *Service) Get(ctx context.Context, platform, accountID string) (Account, error) {
	platform, err := normalizePlatform(platform)
	if err != nil {
		return Account{}, err
	}
	accountID, err = normalizeAccountID(accountID)
	if err != nil {
		return Account{}, err
	}
	var account Account
	var enabled int
	var credentialCheckedAt sql.NullString
	var updatedAt string
	err = s.read.QueryRowContext(ctx, `SELECT platform, account_id, label, enabled, secret_key, profile_uid, profile_nickname, profile_avatar_url, credential_state, credential_checked_at, credential_last_error, updated_at FROM third_party_accounts WHERE platform = ? AND account_id = ?`, platform, accountID).Scan(
		&account.Platform,
		&account.AccountID,
		&account.Label,
		&enabled,
		&account.SecretKey,
		&account.Profile.UID,
		&account.Profile.Nickname,
		&account.Profile.AvatarURL,
		&account.Credential.State,
		&credentialCheckedAt,
		&account.Credential.LastError,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("read third-party account: %w", err)
	}
	account.Enabled = enabled != 0
	account.Configured = s.secretConfigured(ctx, account.SecretKey)
	account.Credential.State = normalizeCredentialState(account.Credential.State)
	account.Credential.CheckedAt = parseOptionalTime(credentialCheckedAt)
	account.UpdatedAt = parseTime(updatedAt)
	return account, nil
}

func (s *Service) ListEnabled(ctx context.Context, platform string) ([]Account, error) {
	platform, err := normalizePlatform(platform)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.QueryContext(ctx, `SELECT platform, account_id, label, enabled, secret_key, profile_uid, profile_nickname, profile_avatar_url, credential_state, credential_checked_at, credential_last_error, updated_at FROM third_party_accounts WHERE platform = ? AND enabled = 1 AND credential_state != 'invalid' ORDER BY account_id ASC`, platform)
	if err != nil {
		return nil, fmt.Errorf("list enabled third-party accounts: %w", err)
	}
	defer func(release func() error) { _ = release() }(rows.Close)

	accounts := []Account{}
	for rows.Next() {
		var account Account
		var enabled int
		var credentialCheckedAt sql.NullString
		var updatedAt string
		if err := rows.Scan(
			&account.Platform,
			&account.AccountID,
			&account.Label,
			&enabled,
			&account.SecretKey,
			&account.Profile.UID,
			&account.Profile.Nickname,
			&account.Profile.AvatarURL,
			&account.Credential.State,
			&credentialCheckedAt,
			&account.Credential.LastError,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan third-party account: %w", err)
		}
		account.Enabled = enabled != 0
		account.Configured = s.secretConfigured(ctx, account.SecretKey)
		account.Credential.State = normalizeCredentialState(account.Credential.State)
		account.Credential.CheckedAt = parseOptionalTime(credentialCheckedAt)
		account.UpdatedAt = parseTime(updatedAt)
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate third-party accounts: %w", err)
	}
	return accounts, nil
}

func (s *Service) ReadCookie(ctx context.Context, account Account) (string, error) {
	key := strings.TrimSpace(account.SecretKey)
	if key == "" {
		return "", secrets.ErrNotFound
	}
	value, err := s.secrets.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return secrets.OpenString(ctx, s.secrets, value)
}

func (s *Service) UpdateCookie(ctx context.Context, account Account, cookie string) error {
	platform, err := normalizePlatform(account.Platform)
	if err != nil {
		return err
	}
	accountID, err := normalizeAccountID(account.AccountID)
	if err != nil {
		return err
	}
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return secrets.ErrNotFound
	}
	secretKey := secretKeyFor(platform, accountID)
	sealed, err := secrets.SealString(ctx, s.secrets, cookie)
	if err != nil {
		return fmt.Errorf("seal third-party account secret: %w", err)
	}
	if err := s.secrets.Set(ctx, secretKey, sealed); err != nil {
		return fmt.Errorf("store third-party account secret: %w", err)
	}
	_, err = s.write.ExecContext(ctx,
		`UPDATE third_party_accounts SET secret_key = ?, updated_at = ? WHERE platform = ? AND account_id = ?`,
		secretKey,
		s.now().UTC().Format(time.RFC3339Nano),
		platform,
		accountID,
	)
	if err != nil {
		return fmt.Errorf("update third-party account secret: %w", err)
	}
	return nil
}

func (s *Service) Upsert(ctx context.Context, request UpsertRequest) (Account, error) {
	platform, err := normalizePlatform(request.Platform)
	if err != nil {
		return Account{}, err
	}
	accountID, err := normalizeAccountID(request.AccountID)
	if err != nil {
		return Account{}, err
	}
	label := strings.TrimSpace(request.Label)
	secretKey := secretKeyFor(platform, accountID)
	now := s.now().UTC()
	profile := request.Profile.normalized()
	credential := request.Credential.normalized()

	if strings.TrimSpace(request.Cookie) != "" {
		if request.Validate != nil {
			checkedProfile, checkedCredential, _ := request.Validate(ctx, request.Cookie)
			// Only overwrite the profile if the validator returned non-empty data.
			// This preserves the QR-login profile when the validator fails to refetch.
			if !checkedProfile.Empty() {
				profile = checkedProfile.normalized()
			}
			credential = checkedCredential.normalized()
		} else if credential.State == "" || credential.State == CredentialUnknown {
			checkedAt := now
			credential = CredentialStatus{State: CredentialUnknown, CheckedAt: &checkedAt}
		}
		sealed, err := secrets.SealString(ctx, s.secrets, request.Cookie)
		if err != nil {
			return Account{}, fmt.Errorf("seal third-party account secret: %w", err)
		}
		if err := s.secrets.Set(ctx, secretKey, sealed); err != nil {
			return Account{}, fmt.Errorf("store third-party account secret: %w", err)
		}
	}

	if _, err := s.write.ExecContext(ctx,
		`INSERT INTO third_party_accounts (platform, account_id, label, enabled, secret_key, profile_uid, profile_nickname, profile_avatar_url, credential_state, credential_checked_at, credential_last_error, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(platform, account_id) DO UPDATE SET
		   label = excluded.label,
		   enabled = excluded.enabled,
		   secret_key = excluded.secret_key,
		   profile_uid = CASE WHEN excluded.profile_uid = '' THEN third_party_accounts.profile_uid ELSE excluded.profile_uid END,
		   profile_nickname = CASE WHEN excluded.profile_nickname = '' THEN third_party_accounts.profile_nickname ELSE excluded.profile_nickname END,
		   profile_avatar_url = CASE WHEN excluded.profile_avatar_url = '' THEN third_party_accounts.profile_avatar_url ELSE excluded.profile_avatar_url END,
		   credential_state = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.credential_state ELSE excluded.credential_state END,
		   credential_checked_at = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.credential_checked_at ELSE excluded.credential_checked_at END,
		   credential_last_error = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.credential_last_error ELSE excluded.credential_last_error END,
		   updated_at = excluded.updated_at`,
		platform,
		accountID,
		label,
		boolInt(request.Enabled),
		secretKey,
		profile.UID,
		profile.Nickname,
		profile.AvatarURL,
		credential.State,
		nullableTime(credential.CheckedAt),
		credential.LastError,
		now.Format(time.RFC3339Nano),
	); err != nil {
		return Account{}, fmt.Errorf("upsert third-party account: %w", err)
	}
	accounts, err := s.List(ctx)
	if err != nil {
		return Account{}, err
	}
	for _, account := range accounts {
		if account.Platform == platform && account.AccountID == accountID {
			return account, nil
		}
	}
	return Account{}, fmt.Errorf("read saved third-party account: %w", sql.ErrNoRows)
}

func (s *Service) Delete(ctx context.Context, platform, accountID string) error {
	platform, err := normalizePlatform(platform)
	if err != nil {
		return err
	}
	accountID, err = normalizeAccountID(accountID)
	if err != nil {
		return err
	}
	secretKey := secretKeyFor(platform, accountID)
	if _, err := s.write.ExecContext(ctx, `DELETE FROM third_party_accounts WHERE platform = ? AND account_id = ?`, platform, accountID); err != nil {
		return fmt.Errorf("delete third-party account: %w", err)
	}
	if err := s.secrets.Delete(ctx, secretKey); err != nil {
		return fmt.Errorf("delete third-party account secret: %w", err)
	}
	return nil
}

func (s *Service) UpdateCredentialStatus(ctx context.Context, platform, accountID string, profile AccountProfile, credential CredentialStatus) error {
	platform, err := normalizePlatform(platform)
	if err != nil {
		return err
	}
	accountID, err = normalizeAccountID(accountID)
	if err != nil {
		return err
	}
	profile = profile.normalized()
	credential = credential.normalized()
	_, err = s.write.ExecContext(ctx,
		`UPDATE third_party_accounts
		 SET profile_uid = ?, profile_nickname = ?, profile_avatar_url = ?,
		     credential_state = ?, credential_checked_at = ?, credential_last_error = ?
		 WHERE platform = ? AND account_id = ?`,
		profile.UID,
		profile.Nickname,
		profile.AvatarURL,
		credential.State,
		nullableTime(credential.CheckedAt),
		credential.LastError,
		platform,
		accountID,
	)
	if err != nil {
		return fmt.Errorf("update third-party credential status: %w", err)
	}
	return nil
}

func (s *Service) UpdateCredentialStatusIfUnchanged(ctx context.Context, account Account, profile AccountProfile, credential CredentialStatus) (Account, bool, error) {
	platform, err := normalizePlatform(account.Platform)
	if err != nil {
		return Account{}, false, err
	}
	accountID, err := normalizeAccountID(account.AccountID)
	if err != nil {
		return Account{}, false, err
	}
	profile = profile.normalized()
	credential = credential.normalized()
	result, err := s.write.ExecContext(ctx,
		`UPDATE third_party_accounts
		 SET profile_uid = CASE WHEN ? = '' THEN profile_uid ELSE ? END,
		     profile_nickname = CASE WHEN ? = '' THEN profile_nickname ELSE ? END,
		     profile_avatar_url = CASE WHEN ? = '' THEN profile_avatar_url ELSE ? END,
		     credential_state = ?, credential_checked_at = ?, credential_last_error = ?
		 WHERE platform = ? AND account_id = ? AND updated_at = ?`,
		profile.UID,
		profile.UID,
		profile.Nickname,
		profile.Nickname,
		profile.AvatarURL,
		profile.AvatarURL,
		credential.State,
		nullableTime(credential.CheckedAt),
		credential.LastError,
		platform,
		accountID,
		account.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return Account{}, false, fmt.Errorf("update third-party credential status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Account{}, false, fmt.Errorf("read third-party credential update result: %w", err)
	}
	current, err := s.Get(ctx, platform, accountID)
	if err != nil {
		return Account{}, false, err
	}
	return current, rowsAffected == 1, nil
}
