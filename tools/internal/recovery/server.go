package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/RayleaBot/RayleaBot/tools/internal/processoutput"
)

const setupToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const controlToken = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
const fixtureSecret = "fixture-only-secret"

type HTTPError struct {
	Path   string
	Status int
	Code   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s returned HTTP %d (%s)", e.Path, e.Status, e.Code)
}
func request(ctx context.Context, origin, path string, data any, token string, setup, control bool) (map[string]any, error) {
	var body io.Reader
	method := http.MethodGet
	if data != nil {
		b, e := json.Marshal(data)
		if e != nil {
			return nil, e
		}
		body = bytes.NewReader(b)
		method = http.MethodPost
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, method, origin+path, body)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Raylea-Session-Transport", "bearer")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if setup {
		req.Header.Set("X-Raylea-Setup-Token", setupToken)
	}
	if control {
		req.Header.Set("X-Raylea-Launcher-Control", controlToken)
	}
	response, e := http.DefaultClient.Do(req)
	if e != nil {
		return nil, e
	}
	defer response.Body.Close()
	raw, e := io.ReadAll(response.Body)
	if e != nil {
		return nil, e
	}
	if response.StatusCode >= 400 {
		code := "unknown"
		var payload struct{ Error struct{ Code string } }
		if json.Unmarshal(raw, &payload) != nil {
			code = "invalid_error_response"
		} else if payload.Error.Code != "" {
			code = payload.Error.Code
		}
		return nil, &HTTPError{path, response.StatusCode, code}
	}
	value := map[string]any{}
	if len(raw) > 0 {
		e = json.Unmarshal(raw, &value)
	}
	return value, e
}
func choosePort() (int, error) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	port := listener.Addr().(*net.TCPAddr).Port
	return port, listener.Close()
}
func runCLI(ctx context.Context, binary, root string, args ...string) error {
	log, e := os.OpenFile(filepath.Join(root, "cli.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if e != nil {
		return e
	}
	defer log.Close()
	cmd, e := processoutput.Command(append([]string{binary, "-config", filepath.Join(root, "config/user.yaml")}, args...)...)
	if e != nil {
		return e
	}
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = log, log
	if e = cmd.Start(); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(60 * time.Second)
	defer timer.Stop()
	select {
	case e = <-done:
		return e
	case <-ctx.Done():
		e = ctx.Err()
	case <-timer.C:
		e = errors.New("Server CLI timed out")
	}
	kill := cmd.Process.Kill()
	return errors.Join(e, kill, <-done)
}

type serverTiming struct{ startup, shutdown, reap, retry time.Duration }

var defaultTiming = serverTiming{30 * time.Second, 15 * time.Second, 5 * time.Second, 100 * time.Millisecond}

func runningServer(ctx context.Context, binary, root string, port int, exercise func(string) error) error {
	cmd, e := processoutput.Command(binary, "-config", filepath.Join(root, "config/user.yaml"))
	if e != nil {
		return e
	}
	return withServer(ctx, cmd, root, fmt.Sprintf("http://127.0.0.1:%d", port), defaultTiming, exercise)
}
func withServer(ctx context.Context, cmd *exec.Cmd, root, origin string, timing serverTiming, exercise func(string) error) (err error) {
	log, e := os.OpenFile(filepath.Join(root, "server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, log.Close()) }()
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "RAYLEA_SETUP_TOKEN="+setupToken, "RAYLEA_LAUNCHER_CONTROL_TOKEN="+controlToken)
	cmd.Stdout, cmd.Stderr = log, log
	if e = cmd.Start(); e != nil {
		return e
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	exited := false
	var exitError error
	poll := func() bool {
		if !exited {
			select {
			case exitError = <-done:
				exited = true
			default:
			}
		}
		return exited
	}
	wait := func(duration time.Duration) error {
		if poll() {
			return nil
		}
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case exitError = <-done:
			exited = true
			return nil
		case <-timer.C:
			return errors.New("Server shutdown timed out")
		}
	}
	defer func() {
		if !poll() {
			_, shutdown := request(context.Background(), origin, "/api/launcher/shutdown", map[string]any{}, "", false, true)
			if shutdown == nil {
				shutdown = wait(timing.shutdown)
			}
			err = errors.Join(err, shutdown)
			if !poll() {
				err = errors.Join(err, cmd.Process.Kill())
				err = errors.Join(err, wait(timing.reap))
			}
		}
		if exited && exitError != nil {
			err = errors.Join(err, fmt.Errorf("Server did not exit cleanly: %w; inspect %s", exitError, filepath.Join(root, "server.log")))
		}
	}()
	deadline := time.Now().Add(timing.startup)
	for {
		if poll() {
			return fmt.Errorf("Server exited: inspect %s", filepath.Join(root, "server.log"))
		}
		_, e = request(ctx, origin, "/healthz", nil, "", false, false)
		if e == nil {
			break
		}
		var httpError *HTTPError
		if errors.As(e, &httpError) {
			return e
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("Server did not become healthy: %s", root)
		}
		timer := time.NewTimer(timing.retry)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return exercise(origin)
}
