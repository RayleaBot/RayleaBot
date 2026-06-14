package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuiltinEchoPluginRepliesWithArgs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "single arg", args: []string{"hello"}, want: "hello"},
		{name: "multiple args", args: []string{"hello", "world"}, want: "hello world"},
		{name: "empty args", args: []string{}, want: "(空消息)"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			session := startBuiltinEchoPlugin(t)
			defer session.close(t)

			initAck := session.readFrame(t)
			if initAck["type"] != "init_ack" {
				t.Fatalf("unexpected init frame type: %#v", initAck)
			}
			if initAck["status"] != "ready" {
				t.Fatalf("unexpected init status: %#v", initAck)
			}
			subscriptions, ok := initAck["subscriptions"].([]any)
			if !ok || len(subscriptions) != 2 || subscriptions[0] != "message.group" || subscriptions[1] != "message.private" {
				t.Fatalf("unexpected subscriptions: %#v", initAck["subscriptions"])
			}

			session.writeFrame(t, map[string]any{
				"protocol_version": "1",
				"type":             "event",
				"timestamp":        time.Now().Unix(),
				"plugin_id":        "raylea.echo",
				"request_id":       "event-1",
				"event": map[string]any{
					"event_id":        "event-1",
					"source_protocol": "onebot11",
					"source_adapter":  "test",
					"event_type":      "message.group",
					"timestamp":       time.Now().Unix(),
					"target": map[string]any{
						"type": "group",
						"id":   "2001",
					},
					"payload": map[string]any{
						"command": "echo",
						"args":    tc.args,
					},
				},
			})

			action := session.readFrame(t)
			if action["type"] != "action" {
				t.Fatalf("unexpected action frame: %#v", action)
			}
			if action["action"] != "message.send" {
				t.Fatalf("unexpected action kind: %#v", action)
			}

			data, ok := action["data"].(map[string]any)
			if !ok {
				t.Fatalf("unexpected action data: %#v", action["data"])
			}
			if data["target_type"] != "group" || data["target_id"] != "2001" {
				t.Fatalf("unexpected action target: %#v", data)
			}

			message, ok := data["message"].(map[string]any)
			if !ok {
				t.Fatalf("unexpected action message: %#v", data["message"])
			}
			segments, ok := message["segments"].([]any)
			if !ok || len(segments) != 1 {
				t.Fatalf("unexpected message segments: %#v", message["segments"])
			}
			segment, ok := segments[0].(map[string]any)
			if !ok {
				t.Fatalf("unexpected segment: %#v", segments[0])
			}
			if segment["type"] != "text" {
				t.Fatalf("unexpected segment type: %#v", segment)
			}
			segmentData, ok := segment["data"].(map[string]any)
			if !ok {
				t.Fatalf("unexpected segment data: %#v", segment["data"])
			}
			if segmentData["text"] != tc.want {
				t.Fatalf("unexpected reply text: got %#v want %q", segmentData["text"], tc.want)
			}
		})
	}
}

type builtinPythonPluginSession struct {
	cmd      *exec.Cmd
	pluginID string
	stdin    *bufio.Writer
	frames   chan map[string]any
	stderr   bytes.Buffer
	finished chan error
}

func startBuiltinEchoPlugin(t *testing.T) *builtinPythonPluginSession {
	t.Helper()

	return startBuiltinPythonPlugin(t, "raylea.echo", filepath.Join(repoRootPath(t), "plugins", "builtin", "echo", "main.py"))
}

func startBuiltinPythonPlugin(t *testing.T, pluginID string, scriptPath string) *builtinPythonPluginSession {
	return startBuiltinPythonPluginWithPrefixes(t, pluginID, scriptPath, []string{"/"})
}

func startBuiltinPythonPluginWithPrefixes(t *testing.T, pluginID string, scriptPath string, commandPrefixes []string) *builtinPythonPluginSession {
	t.Helper()

	command, args := builtinPythonCommand(t)
	cmd := exec.Command(command, append(args, scriptPath)...)
	cmd.Dir = repoRootPath(t)
	cmd.Env = append(cmd.Environ(), "PYTHONIOENCODING=UTF-8", "PYTHONUTF8=1", "PYTHONUNBUFFERED=1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}

	session := &builtinPythonPluginSession{
		cmd:      cmd,
		pluginID: pluginID,
		stdin:    bufio.NewWriter(stdin),
		frames:   make(chan map[string]any, 8),
		finished: make(chan error, 1),
	}
	cmd.Stderr = &session.stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start builtin echo plugin: %v", err)
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var frame map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
				session.finished <- err
				return
			}
			session.frames <- frame
		}
		if err := scanner.Err(); err != nil {
			session.finished <- err
			return
		}
		session.finished <- cmd.Wait()
	}()

	session.writeFrame(t, map[string]any{
		"protocol_version": "1",
		"type":             "init",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        pluginID,
		"request_id":       "init-1",
		"bot": map[string]any{
			"id":       "bot-1",
			"nickname": "RayleaBot",
		},
		"capabilities":     []string{"event.subscribe"},
		"command_prefixes": commandPrefixes,
	})

	return session
}

func builtinPythonCommand(t *testing.T) (string, []string) {
	t.Helper()

	candidates := []struct {
		command string
		args    []string
	}{
		{command: "python", args: nil},
		{command: "python3", args: nil},
		{command: "py", args: []string{"-3"}},
	}
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate.command); err == nil {
			return candidate.command, candidate.args
		}
	}

	t.Skip("python interpreter not found")
	return "", nil
}

func (s *builtinPythonPluginSession) writeFrame(t *testing.T, frame map[string]any) {
	t.Helper()

	line, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}
	if _, err := s.stdin.WriteString(string(line) + "\n"); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	if err := s.stdin.Flush(); err != nil {
		t.Fatalf("flush frame: %v", err)
	}
}

func (s *builtinPythonPluginSession) readFrame(t *testing.T) map[string]any {
	t.Helper()

	select {
	case frame := <-s.frames:
		return frame
	case err := <-s.finished:
		t.Fatalf("plugin exited before frame arrived: %v stderr=%s", err, strings.TrimSpace(s.stderr.String()))
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for plugin frame; stderr=%s", strings.TrimSpace(s.stderr.String()))
	}
	return nil
}

func (s *builtinPythonPluginSession) close(t *testing.T) {
	t.Helper()

	if s.cmd.Process == nil {
		return
	}

	s.writeFrame(t, map[string]any{
		"protocol_version": "1",
		"type":             "shutdown",
		"timestamp":        time.Now().Unix(),
		"plugin_id":        s.pluginID,
		"request_id":       "shutdown-1",
		"reason":           "test complete",
	})

	select {
	case err := <-s.finished:
		if err != nil {
			t.Fatalf("builtin python plugin exit error: %v stderr=%s", err, strings.TrimSpace(s.stderr.String()))
		}
	case <-time.After(3 * time.Second):
		if err := s.cmd.Process.Kill(); err != nil {
			t.Fatalf("kill builtin python plugin after timeout: %v", err)
		}
		t.Fatalf("timed out waiting for builtin python plugin shutdown; stderr=%s", strings.TrimSpace(s.stderr.String()))
	}
}
