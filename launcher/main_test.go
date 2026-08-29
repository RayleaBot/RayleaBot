package main

import (
	"bytes"
	"image/png"
	"testing"
	"time"
)

func TestEmbeddedLauncherIconsMatchNativeRoles(t *testing.T) {
	for name, testCase := range map[string]struct {
		data []byte
		want int
	}{
		"application": {data: appIcon, want: 1024},
		"tray":        {data: trayIcon, want: 32},
	} {
		t.Run(name, func(t *testing.T) {
			image, err := png.Decode(bytes.NewReader(testCase.data))
			if err != nil {
				t.Fatalf("decode embedded icon: %v", err)
			}
			if bounds := image.Bounds(); bounds.Dx() != testCase.want || bounds.Dy() != testCase.want {
				t.Fatalf("icon bounds = %v, want %dx%d", bounds, testCase.want, testCase.want)
			}
		})
	}
}

func TestExternalStopConfirmationWaitsForRendererResponse(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "confirm"}[confirmed], func(t *testing.T) {
			host := &appHost{}
			completed := make(chan bool, 1)
			go func() { completed <- host.ConfirmExternalServiceStop() }()

			deadline := time.Now().Add(time.Second)
			for {
				if host.HasPendingExternalServiceStop() {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("confirmation request was not registered")
				}
				time.Sleep(time.Millisecond)
			}
			select {
			case <-completed:
				t.Fatal("confirmation returned before the renderer response")
			default:
			}

			host.ResolveExternalServiceStop(confirmed)
			select {
			case result := <-completed:
				if result != confirmed {
					t.Fatalf("confirmation result = %v, want %v", result, confirmed)
				}
			case <-time.After(time.Second):
				t.Fatal("confirmation did not return after the renderer response")
			}
			if host.HasPendingExternalServiceStop() {
				t.Fatal("confirmation remained pending after the renderer response")
			}
		})
	}
}

func TestExternalStopConfirmationTimesOutWhenRendererDoesNotRespond(t *testing.T) {
	host := &appHost{externalStopTimeout: 20 * time.Millisecond}
	started := time.Now()

	if host.ConfirmExternalServiceStop() {
		t.Fatal("confirmation unexpectedly succeeded after the renderer response timed out")
	}
	if elapsed := time.Since(started); elapsed < host.externalStopTimeout || elapsed > time.Second {
		t.Fatalf("confirmation timeout elapsed = %v", elapsed)
	}
	if host.HasPendingExternalServiceStop() {
		t.Fatal("confirmation remained pending after its timeout")
	}
}
