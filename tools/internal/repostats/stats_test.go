package repostats

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFetchPaginationAndTimestampFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("missing token")
		}
		// GitHub receives the window start as an ISO timestamp with an explicit offset.
		if !strings.Contains(r.URL.RawQuery, "since=2026-01-01T00:00:00+00:00") {
			t.Error(r.URL.RawQuery)
		}
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<next>; rel="next"`)
			w.Write([]byte(`[{"commit":{"committer":{"date":"2026-02-01T00:30:00+02:00"}}}]`))
		} else {
			w.Write([]byte(`[{"commit":{"author":{"date":"2026-02-02T00:00:00Z"}}}]`))
		}
	}))
	defer server.Close()
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	commits, err := Fetch(server.Client(), server.URL, "fixture-token", since)
	if err != nil {
		t.Fatal(err)
	}
	monthly, daily := Counts(commits)
	if len(commits) != 2 || monthly["2026-02"] != 2 || daily["2026-01-31"] != 1 || daily["2026-02-02"] != 1 {
		t.Fatalf("%v %v", monthly, daily)
	}
}
func TestFetchFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403) }))
	defer server.Close()
	if _, err := Fetch(server.Client(), server.URL, "", time.Now()); err == nil {
		t.Fatal("ignored API failure")
	}
}

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCommandWritesSVGFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`[]`)), Request: r}, nil
	})
	var stderr strings.Builder
	if code := Run(io.Discard, &stderr); code != 0 {
		t.Fatalf("exit=%d: %s", code, stderr.String())
	}
	for _, name := range []string{"dist/repo-activity-line.svg", "dist/repo-activity-heatmap.svg"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var svg struct{ XMLName xml.Name }
		if err := xml.Unmarshal(data, &svg); err != nil {
			t.Fatal(err)
		}
		if svg.XMLName.Local != "svg" || svg.XMLName.Space != "http://www.w3.org/2000/svg" {
			t.Fatalf("invalid SVG root: %+v", svg.XMLName)
		}
	}
}
