package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// Temporary experiment: retain protocol names and timings, never CDP payloads.
type browserDiagnosticTrace struct {
	mu      sync.Mutex
	started time.Time
	output  testBrowserOutput
	pending map[int64]browserDiagnosticCall
}

type browserDiagnosticCall struct {
	method  string
	started time.Time
}

func newBrowserDiagnosticTrace() *browserDiagnosticTrace {
	return &browserDiagnosticTrace{started: time.Now(), pending: make(map[int64]browserDiagnosticCall)}
}

func (d *browserDiagnosticTrace) mark(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fmt.Fprintf(&d.output, "+%s %s\n", time.Since(d.started).Round(time.Millisecond), name)
}

func (d *browserDiagnosticTrace) debugf(format string, arguments ...any) {
	if format == "phase %s" && len(arguments) == 1 {
		d.mark(fmt.Sprint(arguments[0]))
		return
	}
	if (format != "-> %s" && format != "<- %s") || len(arguments) != 1 {
		d.mark("transport event: " + format)
		return
	}
	var data []byte
	switch value := arguments[0].(type) {
	case []byte:
		data = value
	case string:
		data = []byte(value)
	default:
		d.mark("unsupported protocol debug frame")
		return
	}
	var message struct {
		ID     int64
		Method string
		Error  json.RawMessage
	}
	if err := json.Unmarshal(data, &message); err != nil {
		d.mark("unparseable protocol frame")
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(d.started).Round(time.Millisecond)
	if format == "-> %s" {
		d.pending[message.ID] = browserDiagnosticCall{method: message.Method, started: now}
		fmt.Fprintf(&d.output, "+%s send id=%d method=%s\n", elapsed, message.ID, message.Method)
	} else if message.ID != 0 {
		call, found := d.pending[message.ID]
		delete(d.pending, message.ID)
		duration := time.Duration(0)
		if found {
			duration = now.Sub(call.started).Round(time.Millisecond)
		}
		fmt.Fprintf(&d.output, "+%s response id=%d method=%s duration=%s error=%t\n", elapsed, message.ID, call.method, duration, len(message.Error) > 0)
	} else {
		fmt.Fprintf(&d.output, "+%s event method=%s\n", elapsed, message.Method)
	}
}

func (d *browserDiagnosticTrace) snapshot() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var result strings.Builder
	result.WriteString(d.output.String())
	ids := make([]int64, 0, len(d.pending))
	for id := range d.pending {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		call := d.pending[id]
		fmt.Fprintf(&result, "pending id=%d method=%s duration=%s\n", id, call.method, time.Since(call.started).Round(time.Millisecond))
	}
	return result.String()
}

type browserDiagnosticOutput struct {
	output   *testBrowserOutput
	trace    *browserDiagnosticTrace
	devtools sync.Once
}

func (w *browserDiagnosticOutput) Write(data []byte) (int, error) {
	if bytes.Contains(data, []byte("DevTools listening on")) {
		w.devtools.Do(func() { w.trace.mark("process published DevTools URL") })
	}
	return w.output.Write(data)
}

func TestBrowserDiagnosticTraceOmitsPayload(t *testing.T) {
	trace := newBrowserDiagnosticTrace()
	trace.debugf("-> %s", []byte("{\"id\":1,\"method\":\"Runtime.evaluate\",\"params\":{\"expression\":\"private-payload\"}}"))
	trace.debugf("<- %s", []byte("{\"id\":1,\"result\":{\"data\":\"private-payload\"}}"))
	trace.debugf("<- %s", []byte("{\"method\":\"Log.entryAdded\",\"params\":{\"text\":\"private-payload\"}}"))
	trace.debugf("-> %s", []byte("{\"id\":2,\"method\":\"Page.captureScreenshot\",\"params\":{}}"))
	result := trace.snapshot()
	if strings.Contains(result, "private-payload") || !strings.Contains(result, "pending id=2 method=Page.captureScreenshot") || !strings.Contains(result, "response id=1 method=Runtime.evaluate") {
		t.Fatalf("unexpected trace metadata: %s", result)
	}
}
