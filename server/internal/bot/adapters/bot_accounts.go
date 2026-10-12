package adapters

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type BotAccountRef = pluginwire.BotAccountRef

type BotAccount struct {
	SourceProtocol         string `json:"source_protocol"`
	BotID                  string `json:"bot_id"`
	Nickname               string `json:"nickname,omitempty"`
	AvatarURL              string `json:"avatar_url,omitempty"`
	State                  string `json:"state"`
	AdapterID              string `json:"adapter_id,omitempty"`
	AdapterDisplayName     string `json:"adapter_display_name,omitempty"`
	LastAdapterID          string `json:"last_adapter_id,omitempty"`
	LastAdapterDisplayName string `json:"last_adapter_display_name,omitempty"`
	LastSeenAt             string `json:"last_seen_at"`
}

type BotAccountPage struct {
	Items []BotAccount `json:"items"`
	pagination.Metadata
}

type BotAccountConnection struct {
	AdapterDescriptor
	AppID string
}

// BotAccountConnections reads only current runtime members, in delivery order.
// Callback payloads from retired instances never become directory input.
func (r *Registry) BotAccountConnections() []BotAccountConnection {
	snapshot := r.Snapshot()
	connections := make([]BotAccountConnection, 0, len(snapshot.cfg.Adapters))
	for _, instance := range snapshot.cfg.Adapters {
		connection := BotAccountConnection{AdapterDescriptor: snapshot.adapterDescriptor(instance)}
		if instance.QQOfficial != nil {
			connection.AppID = strings.TrimSpace(instance.QQOfficial.AppID)
		}
		if shell := snapshot.OneBot11(instance.ID); instance.Type == config.AdapterTypeOneBot11 && shell != nil {
			id, nickname := shell.ConfirmedAccount()
			connection.Identity = nil
			if id != "" {
				connection.State = "connected"
				connection.Identity = &AdapterIdentity{ID: id, Name: nickname}
				if isDigits(id) {
					connection.Identity.AvatarURL = oneBot11AvatarURL(id)
				}
			}
		}
		connections = append(connections, connection)
	}
	return connections
}

var (
	ErrBotAccountNotFound  = errors.New("bot account not found")
	ErrBotAccountConnected = errors.New("bot account still has a connection")
)

type BotAccountDirectoryDeps struct {
	Store       *storage.Store
	Connections func() []BotAccountConnection
	Changed     func()
	Logger      *slog.Logger
	Now         func() time.Time
}

// BotAccountDirectory serializes reconciliation and removal with each commit.
// The persisted rows are management history, never a delivery routing source.
type BotAccountDirectory struct {
	mu          sync.Mutex
	db          *sql.DB
	connections func() []BotAccountConnection
	changed     func()
	logger      *slog.Logger
	now         func() time.Time
	rows        map[BotAccountRef]sqlcgen.BotAccount
	view        map[BotAccountRef]BotAccount
}

func NewBotAccountDirectory(ctx context.Context, deps BotAccountDirectoryDeps) (*BotAccountDirectory, error) {
	if deps.Store == nil || deps.Connections == nil {
		return nil, errors.New("bot account directory requires storage and current connections")
	}
	if deps.Changed == nil {
		deps.Changed = func() {}
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	rows, err := sqlcgen.New(deps.Store.Read).ListBotAccounts(ctx)
	if err != nil {
		return nil, err
	}
	d := &BotAccountDirectory{db: deps.Store.Write, connections: deps.Connections, changed: deps.Changed, logger: deps.Logger, now: deps.Now, rows: make(map[BotAccountRef]sqlcgen.BotAccount)}
	for _, row := range rows {
		d.rows[accountKey(row)] = row
	}
	if err := d.Observe(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

func accountKey(row sqlcgen.BotAccount) BotAccountRef {
	return BotAccountRef{SourceProtocol: row.SourceProtocol, BotID: row.BotID}
}

func unbindAccount(row sqlcgen.BotAccount) sqlcgen.BotAccount {
	row.LastAdapterID, row.LastAdapterDisplayName = row.AdapterID.String, row.AdapterDisplayName
	row.AdapterID, row.AdapterAppID = sql.NullString{}, ""
	return row
}

func (d *BotAccountDirectory) Observe(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.observeLocked(ctx)
}

func (d *BotAccountDirectory) observeLocked(ctx context.Context) error {
	connections := d.connections()
	byID := make(map[string]BotAccountConnection, len(connections))
	for _, connection := range connections {
		byID[connection.ID] = connection
	}
	next := maps.Clone(d.rows)
	for key, row := range next {
		if !row.AdapterID.Valid {
			continue
		}
		connection, exists := byID[row.AdapterID.String]
		if !exists || connection.Protocol != row.SourceProtocol || row.AdapterAppID != connection.AppID {
			next[key] = unbindAccount(row)
		} else {
			row.AdapterDisplayName = connection.DisplayName
			next[key] = row
		}
	}
	now := d.now().UTC()
	confirmed := make(map[BotAccountRef]string)
	for _, connection := range connections {
		if !connection.Enabled || connection.State != "connected" || connection.Identity == nil || connection.Identity.ID == "" {
			continue
		}
		key := BotAccountRef{SourceProtocol: connection.Protocol, BotID: connection.Identity.ID}
		if first, duplicate := confirmed[key]; duplicate {
			d.logger.Warn("机器人账号同时在多个连接确认，使用配置顺序中的首项", "component", "bot_accounts", "source_protocol", key.SourceProtocol, "bot_id", key.BotID, "adapter_id", first, "duplicate_adapter_id", connection.ID)
			continue
		}
		confirmed[key] = connection.ID
		for oldKey, row := range next {
			if oldKey != key && row.AdapterID.Valid && row.AdapterID.String == connection.ID {
				next[oldKey] = unbindAccount(row)
			}
		}
		row, exists := next[key]
		bindingChanged := !exists || !row.AdapterID.Valid || row.AdapterID.String != connection.ID
		row.SourceProtocol, row.BotID = key.SourceProtocol, key.BotID
		row.AdapterID = sql.NullString{String: connection.ID, Valid: true}
		row.AdapterDisplayName, row.AdapterAppID = connection.DisplayName, connection.AppID
		row.LastAdapterID, row.LastAdapterDisplayName = "", ""
		if connection.Identity.Name != "" {
			row.Nickname = connection.Identity.Name
		}
		if connection.Identity.AvatarURL != "" {
			row.AvatarUrl = connection.Identity.AvatarURL
		}
		lastSeen, _ := time.Parse(time.RFC3339Nano, row.LastSeenAt)
		if bindingChanged || now.Sub(lastSeen) >= time.Minute {
			row.LastSeenAt = now.Format(time.RFC3339Nano)
		}
		next[key] = row
	}
	if !maps.Equal(next, d.rows) {
		err := storage.WithTx(ctx, d.db, nil, func(tx *sql.Tx) error {
			queries := sqlcgen.New(tx)
			// Release every changing binding first, including account swaps.
			for key, old := range d.rows {
				if old.AdapterID.Valid && old.AdapterID != next[key].AdapterID {
					if err := queries.UnbindBotAccount(ctx, sqlcgen.UnbindBotAccountParams(key)); err != nil {
						return err
					}
				}
			}
			for key, row := range next {
				if old, exists := d.rows[key]; exists && old == row {
					continue
				}
				if err := queries.UpsertBotAccount(ctx, sqlcgen.UpsertBotAccountParams(row)); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			d.logger.Error("机器人账号目录写入失败", "component", "bot_accounts", "error", err)
			return err
		}
		d.rows = next
	}
	d.publishChanged(connections)
	return nil
}

func (d *BotAccountDirectory) publishChanged(connections []BotAccountConnection) {
	view := d.project(connections)
	// last_seen_at advances every minute while an account stays online; that alone
	// does not warrant making every open management page refetch the directory.
	if !maps.EqualFunc(view, d.view, sameIgnoringLastSeen) {
		d.changed()
	}
	d.view = view
}

func sameIgnoringLastSeen(a, b BotAccount) bool {
	a.LastSeenAt, b.LastSeenAt = "", ""
	return a == b
}

func (d *BotAccountDirectory) project(connections []BotAccountConnection) map[BotAccountRef]BotAccount {
	byID := make(map[string]BotAccountConnection, len(connections))
	for _, connection := range connections {
		byID[connection.ID] = connection
	}
	result := make(map[BotAccountRef]BotAccount, len(d.rows))
	for key, row := range d.rows {
		account := BotAccount{SourceProtocol: row.SourceProtocol, BotID: row.BotID, Nickname: row.Nickname, AvatarURL: row.AvatarUrl, LastSeenAt: row.LastSeenAt, State: "no_connection", LastAdapterID: row.LastAdapterID, LastAdapterDisplayName: row.LastAdapterDisplayName}
		if row.AdapterID.Valid {
			account.State, account.AdapterID, account.AdapterDisplayName = "offline", row.AdapterID.String, row.AdapterDisplayName
			account.LastAdapterID, account.LastAdapterDisplayName = "", ""
			if connection, exists := byID[account.AdapterID]; exists {
				account.AdapterDisplayName = connection.DisplayName
				if connection.Protocol == row.SourceProtocol && connection.AppID == row.AdapterAppID && connection.Enabled && connection.State == "connected" && connection.Identity != nil && connection.Identity.ID == row.BotID {
					account.State = "online"
				}
			}
		}
		result[key] = account
	}
	return result
}

func (d *BotAccountDirectory) Get(ref BotAccountRef) (BotAccount, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	account, ok := d.project(d.connections())[ref]
	return account, ok
}

func (d *BotAccountDirectory) Contains(ref BotAccountRef) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, exists := d.rows[ref]
	return exists
}

func (d *BotAccountDirectory) List(query pagination.Query, protocol, state string) BotAccountPage {
	d.mu.Lock()
	defer d.mu.Unlock()
	items := make([]BotAccount, 0, len(d.rows))
	for _, account := range d.project(d.connections()) {
		if protocol != "" && account.SourceProtocol != protocol || state != "" && account.State != state {
			continue
		}
		if !accountMatches(query.Text, account.BotID, account.Nickname, account.AdapterID, account.AdapterDisplayName, account.LastAdapterID, account.LastAdapterDisplayName) {
			continue
		}
		items = append(items, account)
	}
	rank := map[string]int{"online": 0, "offline": 1, "no_connection": 2}
	slices.SortFunc(items, func(a, b BotAccount) int {
		if delta := rank[a.State] - rank[b.State]; delta != 0 {
			return delta
		}
		if order := strings.Compare(a.SourceProtocol, b.SourceProtocol); order != 0 {
			return order
		}
		return strings.Compare(a.BotID, b.BotID)
	})
	page, meta := pagination.Slice(items, query)
	return BotAccountPage{Items: page, Metadata: meta}
}

func accountMatches(query string, fields ...string) bool {
	fold := func(value string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r + 'a' - 'A'
			}
			return r
		}, value)
	}
	query = fold(query)
	for _, field := range fields {
		if strings.Contains(fold(field), query) {
			return true
		}
	}
	return false
}

func (d *BotAccountDirectory) Delete(ctx context.Context, ref BotAccountRef) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// Reconcile a login that reached its runtime before its notification.
	if err := d.observeLocked(ctx); err != nil {
		return err
	}
	row, exists := d.rows[ref]
	if !exists {
		return ErrBotAccountNotFound
	}
	if row.AdapterID.Valid {
		return ErrBotAccountConnected
	}
	if err := sqlcgen.New(d.db).DeleteBotAccount(ctx, sqlcgen.DeleteBotAccountParams(ref)); err != nil {
		d.logger.Error("机器人账号目录删除失败", "component", "bot_accounts", "error", err)
		return err
	}
	delete(d.rows, ref)
	d.publishChanged(d.connections())
	return nil
}

// Run retries failed observations and refreshes last_seen_at without depending
// on chat traffic or visits to the management surface.
func (d *BotAccountDirectory) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = d.Observe(ctx)
		}
	}
}
