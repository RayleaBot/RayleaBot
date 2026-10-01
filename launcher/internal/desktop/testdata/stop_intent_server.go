package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// This local test service accepts the production launch and update CLI arguments.
func main() {
	if len(os.Args) == 5 && os.Args[3] == "update" {
		if os.Args[4] == "download" {
			return
		}
		// Stop before relaunching a desktop application in the update test.
		os.Exit(1)
	}
	mux := http.NewServeMux()
	server := &http.Server{Addr: os.Getenv("RAYLEA_TEST_SERVICE_ADDR"), Handler: mux, ReadHeaderTimeout: time.Second}
	done := make(chan struct{})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"status":"ok"}`) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"status":"ready"}`) })
	mux.HandleFunc("/api/launcher/status", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"status":"running","adapters":[]}`) })
	mux.HandleFunc("/api/launcher/shutdown", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Raylea-Launcher-Control") != os.Getenv("RAYLEA_LAUNCHER_CONTROL_TOKEN") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var request struct {
			Intent string `json:"intent"`
		}
		if r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(os.Getenv("RAYLEA_TEST_INTENT_FILE"), []byte(request.Intent), 0600); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"accepted":true}`)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
			close(done)
		}()
	})
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Getenv("RAYLEA_TEST_ADDR_FILE"), []byte(listener.Addr().String()), 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := server.Serve(listener); err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	<-done
}
