// Package recovery exercises current-format backup and restore using one Server binary.
package recovery

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/RayleaBot/RayleaBot/tools/internal/cli"
	"github.com/RayleaBot/RayleaBot/tools/internal/ordered"
	"go.yaml.in/yaml/v3"
	_ "modernc.org/sqlite"
)

type Facts struct {
	SchemaVersion string   `json:"schema_version"`
	InitializedAt string   `json:"initialized_at"`
	PluginCursor  any      `json:"plugin_cursor"`
	Tables        []string `json:"tables"`
}

// These offline assertions and the synthetic plugin KV seed have no CLI/HTTP
// equivalent. Use the Server's pinned SQLite driver, only while it is stopped.
func openDatabase(path, mode string) (*sql.DB, error) {
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := u.Query()
	q.Set("mode", mode)
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e == nil {
		db.SetMaxOpenConns(1)
	}
	return db, e
}
func DatabaseFacts(path string) (Facts, error) {
	db, e := openDatabase(path, "ro")
	if e != nil {
		return Facts{}, e
	}
	defer db.Close()
	var f Facts
	if e = db.QueryRow("SELECT version, initialized_at FROM schema_metadata WHERE singleton_id = 1").Scan(&f.SchemaVersion, &f.InitializedAt); e != nil {
		return f, e
	}
	var digest []byte
	if e = db.QueryRow("SELECT secret_digest FROM auth_bootstrap_state").Scan(&digest); e != nil {
		return f, e
	}
	if !bytes.HasPrefix(digest, []byte("raylea-pwd:")) || !bytes.Contains(digest, []byte(":argon2id:")) {
		return f, errors.New("administrator credential digest format was not preserved")
	}
	var kv string
	e = db.QueryRow("SELECT value_json FROM plugin_kv WHERE plugin_id = 'recovery.fixture' AND key = 'cursor'").Scan(&kv)
	if e != nil && e != sql.ErrNoRows {
		return f, e
	}
	if e == nil {
		if e = json.Unmarshal([]byte(kv), &f.PluginCursor); e != nil {
			return f, e
		}
	}
	rows, e := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if e != nil {
		return f, e
	}
	for rows.Next() {
		var name string
		if e = rows.Scan(&name); e != nil {
			rows.Close()
			return f, e
		}
		if strings.HasPrefix(name, "bilibili_source_") {
			rows.Close()
			return f, errors.New("legacy business tables remain in the core database")
		}
		f.Tables = append(f.Tables, name)
	}
	e = errors.Join(rows.Err(), rows.Close())
	if e != nil {
		return f, e
	}
	var check string
	if e = db.QueryRow("PRAGMA quick_check").Scan(&check); e != nil {
		return f, e
	}
	if check != "ok" {
		return f, fmt.Errorf("SQLite quick_check failed: %s", check)
	}
	return f, nil
}
func seedDatabase(path string) error {
	db, e := openDatabase(path, "rw")
	if e != nil {
		return e
	}
	defer db.Close()
	_, e = db.Exec("INSERT INTO plugin_kv (plugin_id, key, value_json, size_bytes, updated_at) VALUES (?, ?, ?, ?, ?)", "recovery.fixture", "cursor", "42", 2, "2026-09-10T00:00:00Z")
	return e
}
func readConfig(path string) (map[string]any, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var config map[string]any
	e = yaml.Unmarshal(b, &config)
	return config, e
}
func writeConfig(path string, config map[string]any) error {
	b, e := yaml.Marshal(config)
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0644)
}
func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func mustEqual(a, b any, message string) error {
	if !reflect.DeepEqual(a, b) {
		return errors.New(message)
	}
	return nil
}
func fileDigest(path string) ([32]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return [32]byte{}, e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return [32]byte{}, e
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest, nil
}

type Result struct {
	Archive                 string `json:"archive"`
	SchemaVersion           string `json:"schema_version"`
	InitializedAt           string `json:"initialized_at"`
	FreshSetup              bool   `json:"fresh_setup"`
	ConfigurationPreserved  bool   `json:"configuration_preserved"`
	PluginDataPreserved     bool   `json:"plugin_data_preserved"`
	RestoredLogin           bool   `json:"restored_login"`
	RepeatedStartIdempotent bool   `json:"repeated_start_idempotent"`
	DatabaseLayout          string `json:"database_layout"`
	RestoredDatabasePath    string `json:"restored_database_path"`
}

func Rehearse(ctx context.Context, binary, output, layout string) (Result, error) {
	fail := func(e error) (Result, error) { return Result{}, e }
	binary, e := filepath.Abs(binary)
	if e != nil {
		return fail(e)
	}
	if _, e = os.Stat(binary); e != nil {
		return fail(e)
	}
	if layout != "default" && layout != "custom" && layout != "absolute" {
		return fail(errors.New("unsupported rehearsal database layout"))
	}
	output, e = filepath.Abs(output)
	if e != nil {
		return fail(e)
	}
	if e = os.MkdirAll(filepath.Dir(output), 0755); e != nil {
		return fail(e)
	}
	if e = os.Mkdir(output, 0755); e != nil {
		return fail(e)
	}
	source, restored := filepath.Join(output, "source"), filepath.Join(output, "restored")
	for _, p := range []string{source, restored} {
		if e = os.Mkdir(p, 0755); e != nil {
			return fail(e)
		}
	}
	if e = runCLI(ctx, binary, source, "config", "init"); e != nil {
		return fail(e)
	}
	port, e := choosePort()
	if e != nil {
		return fail(e)
	}
	configPath := filepath.Join(source, "config/user.yaml")
	config, e := readConfig(configPath)
	if e != nil {
		return fail(e)
	}
	obj(config["server"])["host"] = "127.0.0.1"
	obj(config["server"])["port"] = port
	config["adapters"] = []any{}
	obj(config["command"])["prefixes"] = []any{"!"}
	if layout == "custom" {
		obj(config["database"])["path"] = "custom/state.db"
	} else if layout == "absolute" {
		obj(config["database"])["path"] = filepath.Join(source, "absolute-source/state.db")
	}
	if e = writeConfig(configPath, config); e != nil {
		return fail(e)
	}
	credentials := map[string]any{"identifier": "admin", "secret": fixtureSecret}
	e = runningServer(ctx, binary, source, port, func(origin string) error {
		session, e := request(ctx, origin, "/api/setup/admin", credentials, "", true, false)
		if e != nil {
			return e
		}
		if session["session_token"] == nil || session["session_token"] == "" {
			return errors.New("fresh setup did not create a session")
		}
		status, e := request(ctx, origin, "/api/setup/status", nil, "", false, false)
		if e != nil {
			return e
		}
		return mustEqual(status["initialized"], true, "fresh setup was not initialized")
	})
	if e != nil {
		return fail(e)
	}
	configured := fmt.Sprint(obj(config["database"])["path"])
	database := configured
	if !filepath.IsAbs(database) {
		database = filepath.Join(source, database)
	}
	if e = seedDatabase(database); e != nil {
		return fail(e)
	}
	stateFile := filepath.Join(source, "data/plugins/recovery.fixture/state.json")
	if e = os.MkdirAll(filepath.Dir(stateFile), 0755); e != nil {
		return fail(e)
	}
	state := []byte("{\"cursor\":42}\n")
	if e = os.WriteFile(stateFile, state, 0644); e != nil {
		return fail(e)
	}
	before, e := DatabaseFacts(database)
	if e != nil {
		return fail(e)
	}
	if e = runCLI(ctx, binary, source, "backup"); e != nil {
		return fail(e)
	}
	archives, e := filepath.Glob(filepath.Join(source, "backups/*.zip"))
	if e != nil {
		return fail(e)
	}
	if len(archives) != 1 {
		return fail(fmt.Errorf("expected exactly one backup archive, found %d", len(archives)))
	}
	archive := archives[0]
	digest, e := fileDigest(database)
	if e != nil {
		return fail(e)
	}
	if e = checkBackup(archive, fmt.Sprint(config["schema_version"]), before.SchemaVersion); e != nil {
		return fail(e)
	}
	if e = runCLI(ctx, binary, restored, "restore", archive); e != nil {
		return fail(e)
	}
	restoredConfig, e := readConfig(filepath.Join(restored, "config/user.yaml"))
	if e != nil {
		return fail(e)
	}
	expected := config
	if filepath.IsAbs(configured) {
		obj(expected["database"])["path"] = "data/rayleabot.db"
	}
	if e = mustEqual(restoredConfig, expected, "restored configuration differs"); e != nil {
		return fail(e)
	}
	restoredState, e := os.ReadFile(filepath.Join(restored, "data/plugins/recovery.fixture/state.json"))
	if e != nil {
		return fail(e)
	}
	if e = mustEqual(restoredState, state, "plugin state file differs"); e != nil {
		return fail(e)
	}
	restoredDatabase := filepath.Join(restored, fmt.Sprint(obj(expected["database"])["path"]))
	after, e := DatabaseFacts(restoredDatabase)
	if e != nil {
		return fail(e)
	}
	if e = mustEqual(before, after, "restored database facts differ"); e != nil {
		return fail(e)
	}
	for i := 0; i < 2; i++ {
		e = runningServer(ctx, binary, restored, port, func(origin string) error {
			session, e := request(ctx, origin, "/api/session/login", credentials, "", false, false)
			if e != nil {
				return e
			}
			token, _ := session["session_token"].(string)
			if token == "" {
				return errors.New("restored credentials could not log in")
			}
			diagnostics, e := request(ctx, origin, "/api/system/diagnostics", nil, token, false, false)
			if e != nil {
				return e
			}
			db := obj(diagnostics["database"])
			if e = mustEqual(db["schema_version"], before.SchemaVersion, "diagnostic schema version differs"); e != nil {
				return e
			}
			return mustEqual(db["initialized_at"], before.InitializedAt, "diagnostic initialization timestamp differs")
		})
		if e != nil {
			return fail(e)
		}
	}
	after, e = DatabaseFacts(restoredDatabase)
	if e != nil {
		return fail(e)
	}
	if e = mustEqual(before, after, "repeated start changed database facts"); e != nil {
		return fail(e)
	}
	finalDigest, e := fileDigest(database)
	if e != nil {
		return fail(e)
	}
	if digest != finalDigest {
		return fail(errors.New("recovery modified the source database"))
	}
	r := Result{archive, before.SchemaVersion, before.InitializedAt, true, true, true, true, true, layout, fmt.Sprint(obj(expected["database"])["path"])}
	b, e := ordered.JSON(r, true)
	if e != nil {
		return fail(e)
	}
	if e = os.WriteFile(filepath.Join(output, "result.json"), bytes.TrimSuffix(b, []byte("\n")), 0644); e != nil {
		return fail(e)
	}
	return r, nil
}
func checkBackup(path, configVersion, dbVersion string) error {
	reader, e := zip.OpenReader(path)
	if e != nil {
		return e
	}
	defer reader.Close()
	hasState := false
	var manifest map[string]any
	for _, f := range reader.File {
		if f.Name == "data/plugins/recovery.fixture/state.json" {
			hasState = true
		}
		if f.Name == "backup-manifest.json" {
			r, e := f.Open()
			if e != nil {
				return e
			}
			e = json.NewDecoder(r).Decode(&manifest)
			closeErr := r.Close()
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	if manifest["config_schema_version"] != configVersion || manifest["db_schema_version"] != dbVersion || !hasState {
		return errors.New("backup manifest versions or plugin state entry differ")
	}
	return nil
}
func Run(args []string, out, stderr io.Writer) int {
	fs := flag.NewFlagSet("rehearse-current-recovery", flag.ContinueOnError)
	fs.SetOutput(stderr)
	binary := fs.String("server", "", "Server executable")
	output := fs.String("output", "", "new directory for synthetic artifacts")
	layout := fs.String("database-layout", "default", "default, custom or absolute")
	if e := cli.Parse(fs, args, 0, 0); e != nil {
		return cli.ErrorTo(stderr, e)
	}
	if *binary == "" || *output == "" {
		return cli.ErrorTo(stderr, errors.New("--server and --output are required"))
	}
	if *layout != "default" && *layout != "custom" && *layout != "absolute" {
		return cli.ErrorTo(stderr, errors.New("--database-layout must be default, custom or absolute"))
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	r, e := Rehearse(ctx, *binary, *output, *layout)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	b, e := ordered.JSON(r, true)
	if e != nil {
		fmt.Fprintln(stderr, e)
		return 1
	}
	fmt.Fprint(out, string(b))
	return 0
}
