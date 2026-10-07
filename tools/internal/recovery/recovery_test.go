package recovery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/processoutput"
)

func TestRecoveryProcessHelper(t *testing.T) {
	index := -1
	for i, v := range os.Args {
		if v == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	mode, origin := os.Args[index+1], os.Args[index+2]
	server := &http.Server{Addr: strings.TrimPrefix(origin, "http://")}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			if mode == "bad-health" {
				w.WriteHeader(503)
				fmt.Fprint(w, `{"error":{"code":"system.health_failed"}}`)
				return
			}
			fmt.Fprint(w, `{}`)
			return
		}
		if r.Header.Get("X-Raylea-Launcher-Control") != controlToken {
			w.WriteHeader(403)
			return
		}
		if mode == "bad-shutdown" {
			w.WriteHeader(503)
			fmt.Fprint(w, `{"error":{"code":"system.shutdown_failed"}}`)
			return
		}
		fmt.Fprint(w, `{}`)
		if mode != "hang-shutdown" {
			go func() { time.Sleep(10 * time.Millisecond); _ = server.Shutdown(context.Background()) }()
		}
	})
	e := server.ListenAndServe()
	if errors.Is(e, http.ErrServerClosed) {
		os.Exit(0)
	}
	os.Exit(7)
}
func TestOwnedServerShutdownAndFailureEvidence(t *testing.T) {
	for _, mode := range []string{"normal", "bad-shutdown", "hang-shutdown", "bad-health", "exercise-failure", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			port, e := choosePort()
			if e != nil {
				t.Fatal(e)
			}
			origin := fmt.Sprintf("http://127.0.0.1:%d", port)
			cmd, e := processoutput.Command(os.Args[0], "-test.run=TestRecoveryProcessHelper", "--", mode, origin)
			if e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			original := errors.New("recovery verification failed")
			called := false
			e = withServer(ctx, cmd, t.TempDir(), origin, serverTiming{5 * time.Second, 150 * time.Millisecond, 5 * time.Second, 10 * time.Millisecond}, func(string) error {
				called = true
				if mode == "exercise-failure" {
					return original
				}
				if mode == "cancelled" {
					cancel()
					return ctx.Err()
				}
				return nil
			})
			switch mode {
			case "normal":
				if e != nil {
					t.Fatal(e)
				}
			case "bad-shutdown":
				var httpError *HTTPError
				if !errors.As(e, &httpError) || httpError.Code != "system.shutdown_failed" || !strings.Contains(e.Error(), "did not exit cleanly") {
					t.Fatal(e)
				}
			case "hang-shutdown":
				if e == nil || !strings.Contains(e.Error(), "shutdown timed out") || !strings.Contains(e.Error(), "did not exit cleanly") {
					t.Fatal(e)
				}
			case "bad-health":
				var httpError *HTTPError
				if !errors.As(e, &httpError) || httpError.Code != "system.health_failed" || called {
					t.Fatal("startup failure lost", e)
				}
			case "exercise-failure":
				if !errors.Is(e, original) {
					t.Fatal("exercise error lost", e)
				}
			case "cancelled":
				if !errors.Is(e, context.Canceled) {
					t.Fatal("cancellation lost", e)
				}
			}
			if cmd.ProcessState == nil {
				t.Fatal("owned process was not reaped")
			}
		})
	}
}
func TestHTTPFailureUsesStructuredCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"code":"system.shutdown_failed","message":"private fixture detail"}}`)
	}))
	defer server.Close()
	_, e := request(context.Background(), server.URL, "/api/launcher/shutdown", map[string]any{}, "", false, true)
	var httpError *HTTPError
	if !errors.As(e, &httpError) || httpError.Status != 503 || strings.Contains(e.Error(), "private fixture detail") {
		t.Fatal(e)
	}
}
func TestOfflineFactsAndSeedUseConfiguredDatabase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "含 空格")
	if e := os.Mkdir(dir, 0755); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "state.db")
	db, e := openDatabase(path, "rwc")
	if e != nil {
		t.Fatal(e)
	}
	for _, query := range []string{"CREATE TABLE schema_metadata(singleton_id INTEGER,version TEXT,initialized_at TEXT)", "INSERT INTO schema_metadata VALUES(1,'000008','2026-10-07T00:00:00Z')", "CREATE TABLE auth_bootstrap_state(secret_digest BLOB)", "CREATE TABLE plugin_kv(plugin_id TEXT,key TEXT,value_json TEXT,size_bytes INTEGER,updated_at TEXT)"} {
		if _, e = db.Exec(query); e != nil {
			db.Close()
			t.Fatal(e)
		}
	}
	if _, e = db.Exec("INSERT INTO auth_bootstrap_state VALUES(?)", []byte("raylea-pwd:fixture:argon2id:fixture")); e != nil {
		t.Fatal(e)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	if e = seedDatabase(path); e != nil {
		t.Fatal(e)
	}
	facts, e := DatabaseFacts(path)
	if e != nil {
		t.Fatal(e)
	}
	if facts.SchemaVersion != "000008" || facts.PluginCursor != float64(42) || len(facts.Tables) != 3 {
		t.Fatalf("%+v", facts)
	}
	db, e = openDatabase(path, "rw")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("UPDATE auth_bootstrap_state SET secret_digest=?", []byte("plaintext fixture")); e != nil {
		t.Fatal(e)
	}
	db.Close()
	if _, e = DatabaseFacts(path); e == nil {
		t.Fatal("credential format assertion lost")
	}
}
