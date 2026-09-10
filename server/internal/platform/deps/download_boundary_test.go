package deps

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/fsguard"
)

func TestRuntimeDownloadDoesNotPublishIncompleteFiles(t *testing.T) {
	for _, tc := range []struct {
		name      string
		size      int
		chunked   bool
		cancel    bool
		wantError bool
	}{
		{"complete", 3, false, false, false}, {"oversized-header", 11, false, false, true},
		{"oversized-stream", 11, true, false, true}, {"short-body", 3, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !tc.chunked {
					size := tc.size
					if tc.cancel {
						size += 5
					}
					w.Header().Set("Content-Length", fmt.Sprint(size))
				} else {
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write(make([]byte, tc.size))
			}))
			defer server.Close()
			file := filepath.Join(t.TempDir(), "download")
			err := downloadRuntimeHTTP(context.Background(), server.Client(), server.URL, file, 10, nil)
			if (err != nil) != tc.wantError {
				t.Fatalf("download=%v", err)
			}
			if tc.wantError {
				if _, statErr := os.Stat(file); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("partial file remained: %v", statErr)
				}
			} else {
				body, err := os.ReadFile(file)
				if err != nil || len(body) != 3 {
					t.Fatalf("body=%q err=%v", body, err)
				}
			}
		})
	}
}

func TestRuntimeDownloadRejectsUnsafeRedirectAndPreservesExistingFile(t *testing.T) {
	request := &http.Request{URL: &url.URL{Scheme: "http", Host: "example.invalid"}}
	if err := runtimeHTTPClient.CheckRedirect(request, nil); err == nil {
		t.Fatal("HTTPS downgrade accepted")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("new")) }))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "download")
	if err := os.WriteFile(file, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := downloadRuntimeHTTP(context.Background(), server.Client(), server.URL, file, 10, nil); err == nil {
		t.Fatal("existing file replaced")
	}
	if body, err := os.ReadFile(file); err != nil || string(body) != "old" {
		t.Fatalf("existing file=%q %v", body, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := downloadRuntimeHTTP(ctx, server.Client(), server.URL, file+"-cancel", 10, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled download=%v", err)
	}
}

func TestRuntimeDownloadBoundedCopyReportsLimit(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("123456"))
	}))
	defer server.Close()
	err := downloadRuntimeHTTP(context.Background(), server.Client(), server.URL, filepath.Join(t.TempDir(), "file"), 5, nil)
	if !errors.Is(err, fsguard.ErrSizeLimit) {
		t.Fatalf("size limit=%v", err)
	}
}
