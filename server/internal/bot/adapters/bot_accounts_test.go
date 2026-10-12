package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type accountFixture struct {
	directory   *BotAccountDirectory
	store       *storage.Store
	connections atomic.Pointer[[]BotAccountConnection]
	notices     atomic.Int32
	now         time.Time
}

func TestBotAccountSourcesConfirmHTTPAndWebhookWithoutManagementReads(t *testing.T) {
	for _, transport := range []string{"http", "webhook"} {
		t.Run(transport, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				data := map[string]any{"app_name": "fixture"}
				if request["action"] == "get_login_info" {
					data = map[string]any{"user_id": 10001, "nickname": "HTTP bot"}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "retcode": 0, "data": data, "echo": request["echo"]})
			}))
			defer api.Close()
			settings := config.OneBotConfig{}
			if transport == "http" {
				settings.HTTPAPI = config.OneBotTransportConfig{Enabled: true, URL: api.URL}
			} else {
				settings.Webhook = config.OneBotTransportConfig{Enabled: true, URL: "http://127.0.0.1/fixture"}
			}
			shell := onebot11.New("x", settings, defaultAdapterTestConfig(), nil)
			cfg := config.Config{Adapters: []config.AdapterInstance{{ID: "x", Type: "onebot11", Enabled: true, OneBot11: &settings}}}
			registry := NewRegistry(cfg, map[string]*onebot11.Shell{"x": shell}, nil)
			f := newAccountFixture(t)
			var err error
			f.directory, err = NewBotAccountDirectory(t.Context(), BotAccountDirectoryDeps{Store: f.store, Connections: registry.BotAccountConnections})
			if err != nil {
				t.Fatal(err)
			}
			confirmed := make(chan struct{}, 8)
			shell.SetStateHandler(func(onebot11.Snapshot) {
				if err := f.directory.Observe(context.Background()); err != nil {
					t.Error(err)
				}
				select {
				case confirmed <- struct{}{}:
				default:
				}
			})
			shell.Start(t.Context())
			t.Cleanup(func() {
				if err := shell.Stop(context.Background()); err != nil {
					t.Error(err)
				}
			})
			if transport == "webhook" {
				if err := shell.AcceptWebhookPayload(t.Context(), []byte(`{"time":1700000000,"self_id":10001,"post_type":"meta_event","meta_event_type":"heartbeat","status":{"online":true,"good":true},"interval":1000}`)); err != nil {
					t.Fatal(err)
				}
			}
			deadline := time.After(3 * time.Second)
			for !f.directory.Contains(BotAccountRef{SourceProtocol: "onebot11", BotID: "10001"}) {
				select {
				case <-confirmed:
				case <-deadline:
					t.Fatal("identity did not enter directory")
				}
			}
			expectAccount(t, f.directory, "10001", "online", "x")
			// Retired shells can still notify, but their identity is not read.
			registry.store(config.Config{}, nil, nil)
			f.observe(t)
			if transport == "webhook" {
				_ = shell.AcceptWebhookPayload(t.Context(), []byte(`{"time":1700000001,"self_id":20002,"post_type":"meta_event","meta_event_type":"heartbeat","status":{"online":true,"good":true},"interval":1000}`))
			}
			f.observe(t)
			expectAccount(t, f.directory, "10001", "no_connection", "")
			if f.directory.Contains(BotAccountRef{SourceProtocol: "onebot11", BotID: "20002"}) {
				t.Fatal("retired instance restored an account")
			}
		})
	}
}

func newAccountFixture(t *testing.T) *accountFixture {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "accounts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	f := &accountFixture{store: store, now: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)}
	f.set()
	f.directory, err = NewBotAccountDirectory(t.Context(), BotAccountDirectoryDeps{Store: store, Connections: func() []BotAccountConnection { return *f.connections.Load() }, Changed: func() { f.notices.Add(1) }, Now: func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *accountFixture) set(connections ...BotAccountConnection) { f.connections.Store(&connections) }
func accountConnection(id, bot string) BotAccountConnection {
	connection := BotAccountConnection{AdapterDescriptor: AdapterDescriptor{ID: id, Protocol: "onebot11", DisplayName: "OneBot11（" + id + "）", Enabled: true, State: "connected"}}
	if bot != "" {
		connection.Identity = &AdapterIdentity{ID: bot, Name: "Bot " + bot}
	}
	return connection
}

func (f *accountFixture) observe(t *testing.T) {
	t.Helper()
	if err := f.directory.Observe(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func expectAccount(t *testing.T, d *BotAccountDirectory, id, state, connection string) BotAccount {
	t.Helper()
	account, exists := d.Get(BotAccountRef{SourceProtocol: "onebot11", BotID: id})
	if !exists || account.State != state || account.AdapterID != connection {
		t.Fatalf("account %s = %+v, exists=%v", id, account, exists)
	}
	return account
}

func TestBotAccountLifecycleAndRestart(t *testing.T) {
	f := newAccountFixture(t)
	f.set(accountConnection("x", "a"))
	f.observe(t)
	expectAccount(t, f.directory, "a", "online", "x")
	offline := accountConnection("x", "a")
	offline.Enabled = false
	f.set(offline)
	f.observe(t)
	expectAccount(t, f.directory, "a", "offline", "x")
	f.set(accountConnection("x", "b"))
	f.observe(t)
	old := expectAccount(t, f.directory, "a", "no_connection", "")
	if old.LastAdapterID != "x" || old.LastAdapterDisplayName != "OneBot11（x）" {
		t.Fatalf("history = %+v", old)
	}
	expectAccount(t, f.directory, "b", "online", "x")
	f.set()
	f.observe(t)
	expectAccount(t, f.directory, "b", "no_connection", "")
	f.set(accountConnection("y", "a"))
	f.observe(t)
	expectAccount(t, f.directory, "a", "online", "y")
	// A retired instance's queued notification only reads current membership.
	before := f.notices.Load()
	f.observe(t)
	expectAccount(t, f.directory, "a", "online", "y")
	if f.notices.Load() != before {
		t.Fatal("late unchanged callback published a change")
	}
	restarted, err := NewBotAccountDirectory(t.Context(), BotAccountDirectoryDeps{Store: f.store, Connections: func() []BotAccountConnection { return []BotAccountConnection{accountConnection("y", "")} }})
	if err != nil {
		t.Fatal(err)
	}
	expectAccount(t, restarted, "a", "offline", "y")
	restored, err := NewBotAccountDirectory(t.Context(), BotAccountDirectoryDeps{Store: f.store, Connections: func() []BotAccountConnection { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	expectAccount(t, restored, "a", "no_connection", "")
	if err := restored.Delete(t.Context(), BotAccountRef{SourceProtocol: "onebot11", BotID: "a"}); err != nil {
		t.Fatal(err)
	}
	if restored.Contains(BotAccountRef{SourceProtocol: "onebot11", BotID: "a"}) {
		t.Fatal("deleted account retained")
	}
}

func TestBotAccountWritesAreThrottledAndFailuresKeepCommittedSnapshot(t *testing.T) {
	f := newAccountFixture(t)
	_, err := f.store.Write.Exec(`CREATE TABLE account_writes (n INTEGER); CREATE TRIGGER count_account_writes AFTER UPDATE ON bot_accounts BEGIN INSERT INTO account_writes VALUES (1); END;`)
	if err != nil {
		t.Fatal(err)
	}
	f.set(accountConnection("x", "a"))
	f.observe(t)
	before := f.notices.Load()
	for range 100 {
		f.observe(t)
	}
	var count int
	if err := f.store.Read.QueryRow("SELECT COUNT(*) FROM account_writes").Scan(&count); err != nil || count != 0 || f.notices.Load() != before {
		t.Fatalf("repeated confirmations: writes=%d notifications=%d, %v", count, f.notices.Load()-before, err)
	}
	f.now = f.now.Add(time.Minute)
	f.observe(t)
	if err := f.store.Read.QueryRow("SELECT COUNT(*) FROM account_writes").Scan(&count); err != nil || count != 1 {
		t.Fatalf("minute writes=%d, %v", count, err)
	}
	if _, err := f.store.Write.Exec(`CREATE TRIGGER reject_account BEFORE INSERT ON bot_accounts BEGIN SELECT RAISE(FAIL, 'fixture write failure'); END;`); err != nil {
		t.Fatal(err)
	}
	before = f.notices.Load()
	f.set(accountConnection("x", "b"))
	if err := f.directory.Observe(t.Context()); err == nil {
		t.Fatal("write failure was hidden")
	}
	if f.notices.Load() != before || f.directory.Contains(BotAccountRef{SourceProtocol: "onebot11", BotID: "b"}) {
		t.Fatal("failed write published an uncommitted account")
	}
	expectAccount(t, f.directory, "a", "offline", "x")
	var bound string
	if err := f.store.Read.QueryRow("SELECT adapter_id FROM bot_accounts WHERE bot_id='a'").Scan(&bound); err != nil || bound != "x" {
		t.Fatalf("failed transaction changed binding: %q %v", bound, err)
	}
	if _, err := f.store.Write.Exec("DROP TRIGGER reject_account"); err != nil {
		t.Fatal(err)
	}
	f.observe(t)
	expectAccount(t, f.directory, "a", "no_connection", "")
	expectAccount(t, f.directory, "b", "online", "x")
}

func TestBotAccountConfigurationAndDuplicateIdentity(t *testing.T) {
	f := newAccountFixture(t)
	f.set(accountConnection("x", "a"), accountConnection("y", "a"))
	f.observe(t)
	expectAccount(t, f.directory, "a", "online", "x")
	f.set(accountConnection("y", "a"))
	f.observe(t)
	expectAccount(t, f.directory, "a", "online", "y")
	connection := accountConnection("y", "a")
	connection.DisplayName = "Updated display"
	f.set(connection)
	f.observe(t)
	if account := expectAccount(t, f.directory, "a", "online", "y"); account.AdapterDisplayName != connection.DisplayName {
		t.Fatal(account)
	}
	connection.Protocol, connection.AppID, connection.Identity = "qqofficial", "app-1", &AdapterIdentity{ID: "opaque-id"}
	f.set(connection)
	f.observe(t)
	expectAccount(t, f.directory, "a", "no_connection", "")
	connection.AppID, connection.Identity = "app-2", nil
	f.set(connection)
	f.observe(t)
	account, ok := f.directory.Get(BotAccountRef{SourceProtocol: "qqofficial", BotID: "opaque-id"})
	if !ok || account.State != "no_connection" {
		t.Fatalf("changed AppID retained binding: %+v", account)
	}
}

func TestBotAccountDeleteSerializesWithLogin(t *testing.T) {
	f := newAccountFixture(t)
	ref := BotAccountRef{SourceProtocol: "onebot11", BotID: "a"}
	f.set(accountConnection("x", "a"))
	f.observe(t)
	if err := f.directory.Delete(t.Context(), ref); !errors.Is(err, ErrBotAccountConnected) {
		t.Fatal(err)
	}
	f.set()
	f.observe(t)
	start := make(chan struct{})
	var group sync.WaitGroup
	var deletion error
	group.Go(func() { <-start; deletion = f.directory.Delete(context.Background(), ref) })
	group.Go(func() { <-start; f.set(accountConnection("y", "a")); _ = f.directory.Observe(context.Background()) })
	close(start)
	group.Wait()
	if deletion != nil && !errors.Is(deletion, ErrBotAccountConnected) {
		t.Fatal(deletion)
	}
	expectAccount(t, f.directory, "a", "online", "y")
	if err := f.directory.Delete(t.Context(), BotAccountRef{SourceProtocol: "onebot11", BotID: "missing"}); !errors.Is(err, ErrBotAccountNotFound) {
		t.Fatal(err)
	}
}

func TestBotAccountCollectionFiltersBeforePagination(t *testing.T) {
	f := newAccountFixture(t)
	query := pagination.Query{Limit: 1}
	if empty := f.directory.List(query, "", ""); empty.Items == nil || empty.Total != 0 || empty.NextCursor != "" {
		t.Fatal(empty)
	}
	f.set(accountConnection("first", "3"))
	f.observe(t)
	f.set(accountConnection("other", "2"), accountConnection("current", "1"))
	f.observe(t)
	offline := accountConnection("other", "2")
	offline.State = "reconnecting"
	f.set(offline, accountConnection("current", "1"))
	f.observe(t)
	for index, id := range []string{"1", "2", "3"} {
		query.Cursor = index
		page := f.directory.List(query, "", "")
		if page.Total != 3 || len(page.Items) != 1 || page.Items[0].BotID != id {
			t.Fatal(page)
		}
		if index < 2 && page.NextCursor == "" || index == 2 && page.NextCursor != "" {
			t.Fatal(page)
		}
	}
	query.Cursor, query.Text = 0, "FIRST"
	page := f.directory.List(query, "onebot11", "no_connection")
	if page.Total != 1 || page.Items[0].BotID != "3" {
		t.Fatal(page)
	}
}
