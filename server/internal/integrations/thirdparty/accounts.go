package thirdparty

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
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

var ErrInvalidAccount = errors.New("invalid third-party account")

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
	UID       string
	Nickname  string
	AvatarURL string
}

type MonitorSnapshot struct {
	Platform  string        `json:"platform"`
	Items     []MonitorItem `json:"items"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type MonitorItem struct {
	UID        string
	Username   string
	AvatarURL  string
	ProfileURL string
	Services   []string
	Dynamic    *MonitorDynamic
	Live       MonitorLive
	UpdatedAt  time.Time
}

type MonitorDynamic struct {
	LastID      string
	Service     string
	Title       string
	Summary     string
	URL         string
	Images      []MonitorImage
	PublishedAt *time.Time
	ObservedAt  time.Time
}

type MonitorLive struct {
	RoomID          string
	RoomName        string
	RoomURL         string
	CoverURL        string
	IsLive          bool
	LiveStartedAt   *time.Time
	LiveEndedAt     *time.Time
	ConnectionState string
	LastError       string
	UpdatedAt       *time.Time
}

type MonitorImage struct {
	URL    string
	Width  int
	Height int
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

type AccountValidator struct {
	Client     *http.Client
	Now        func() time.Time
	CheckFuncs map[string]func(context.Context, *http.Client, map[string]string) (AccountProfile, error)
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

func NewAccountValidator(transport http.RoundTripper, now func() time.Time) *AccountValidator {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AccountValidator{
		Client:     NewHTTPClient(transport),
		Now:        now,
		CheckFuncs: make(map[string]func(context.Context, *http.Client, map[string]string) (AccountProfile, error)),
	}
}

func (v *AccountValidator) RegisterPlatform(platform string, checkFn func(context.Context, *http.Client, map[string]string) (AccountProfile, error)) {
	if v == nil {
		return
	}
	v.CheckFuncs[platform] = checkFn
}

func (v *AccountValidator) CheckCookie(ctx context.Context, platform, cookie string) (AccountProfile, CredentialStatus, error) {
	if v == nil {
		return AccountProfile{}, CredentialStatus{}, fmt.Errorf("third-party account validator is unavailable")
	}
	normalized, err := NormalizePlatform(platform)
	if err != nil {
		return AccountProfile{}, v.invalidStatus(err.Error()), err
	}
	cookies := CookieMapFromHeader(cookie)

	checkFn := v.CheckFuncs[normalized]
	if checkFn == nil {
		err := fmt.Errorf("unsupported third-party account platform %s", normalized)
		return AccountProfile{}, v.invalidStatus(err.Error()), err
	}

	profile, err := checkFn(ctx, v.Client, cookies)
	if err != nil {
		// Profile fetch failed but login cookies are present.
		// Return nil error so the caller preserves the QR-login profile
		// and marks the credential as unknown (not invalid).
		return AccountProfile{}, v.unknownStatus(err.Error()), nil
	}
	return profile, v.validStatus(), nil
}

func (v *AccountValidator) validStatus() CredentialStatus {
	checkedAt := v.Now().UTC()
	return CredentialStatus{State: CredentialValid, CheckedAt: &checkedAt}
}

func (v *AccountValidator) invalidStatus(message string) CredentialStatus {
	checkedAt := v.Now().UTC()
	return CredentialStatus{
		State:     CredentialInvalid,
		CheckedAt: &checkedAt,
		LastError: strings.TrimSpace(message),
	}
}

func (v *AccountValidator) unknownStatus(message string) CredentialStatus {
	checkedAt := v.Now().UTC()
	return CredentialStatus{
		State:     CredentialUnknown,
		CheckedAt: &checkedAt,
		LastError: strings.TrimSpace(message),
	}
}

func AccountProfileEmpty(profile AccountProfile) bool {
	return strings.TrimSpace(profile.UID) == "" &&
		strings.TrimSpace(profile.Nickname) == "" &&
		strings.TrimSpace(profile.AvatarURL) == ""
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
	defer rows.Close()

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

func (s *Service) ListEnabled(ctx context.Context, platform string) ([]Account, error) {
	platform, err := normalizePlatform(platform)
	if err != nil {
		return nil, err
	}
	rows, err := s.read.QueryContext(ctx, `SELECT platform, account_id, label, enabled, secret_key, profile_uid, profile_nickname, profile_avatar_url, credential_state, credential_checked_at, credential_last_error, updated_at FROM third_party_accounts WHERE platform = ? AND enabled = 1 AND credential_state != 'invalid' ORDER BY account_id ASC`, platform)
	if err != nil {
		return nil, fmt.Errorf("list enabled third-party accounts: %w", err)
	}
	defer rows.Close()

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
		s.now().UTC().Format(time.RFC3339),
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
			checkedProfile, checkedCredential, err := request.Validate(ctx, request.Cookie)
			// Only overwrite the profile if the validator returned non-empty data.
			// This preserves the QR-login profile when the validator fails to refetch.
			if !checkedProfile.Empty() {
				profile = checkedProfile.normalized()
			}
			credential = checkedCredential.normalized()
			if err != nil && credential.State == CredentialUnknown {
				checkedAt := now
				credential = CredentialStatus{
					State:     CredentialInvalid,
					CheckedAt: &checkedAt,
					LastError: err.Error(),
				}
			}
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
		   profile_uid = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.profile_uid ELSE excluded.profile_uid END,
		   profile_nickname = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.profile_nickname ELSE excluded.profile_nickname END,
		   profile_avatar_url = CASE WHEN excluded.credential_checked_at IS NULL THEN third_party_accounts.profile_avatar_url ELSE excluded.profile_avatar_url END,
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
		now.Format(time.RFC3339),
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
