package releaseupdate

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestDownloaderCommitsVerifiedArtifactAtomically(t *testing.T) {
	payload := []byte("signed release archive")
	artifact := testAutomaticArtifact("rayleabot.zip", payload, 1, 1)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme != "https" || !strings.HasSuffix(request.URL.Path, "/rayleabot.zip") {
			t.Fatalf("unexpected download URL %s", request.URL)
		}
		return &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: int64(len(payload)),
			Body:          io.NopCloser(strings.NewReader(string(payload))),
			Header:        make(http.Header),
			Request:       request,
		}, nil
	})}
	destination := t.TempDir()
	result, err := (&Downloader{HTTPClient: client, IdleTimeout: time.Second, TotalTimeout: time.Second}).Download(context.Background(), CheckResult{
		Status:           "update_available",
		AvailableVersion: "1.2.0",
		Artifact:         artifact,
	}, destination, nil)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if result.ArtifactPath != filepath.Join(destination, artifact.FileName) {
		t.Fatalf("artifact path = %q", result.ArtifactPath)
	}
	if err := VerifyArtifactFile(result.ArtifactPath, artifact); err != nil {
		t.Fatal(err)
	}
	if _, err := filepath.Glob(filepath.Join(destination, "*.partial")); err != nil {
		t.Fatal(err)
	}
}

func TestDownloaderDeletesPartialFileOnSizeMismatch(t *testing.T) {
	payload := []byte("short")
	artifact := testAutomaticArtifact("rayleabot.zip", append(payload, '!'), 1, 1)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(payload)), Body: io.NopCloser(strings.NewReader(string(payload))), Header: make(http.Header), Request: request}, nil
	})}
	destination := t.TempDir()
	_, err := (&Downloader{HTTPClient: client, IdleTimeout: time.Second, TotalTimeout: time.Second}).Download(context.Background(), CheckResult{Status: "update_available", AvailableVersion: "1.2.0", Artifact: artifact}, destination, nil)
	if CodeOf(err) != CodeArtifactInvalid {
		t.Fatalf("size mismatch should fail, got %v", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(destination, "*.partial"))
	if globErr != nil || len(matches) != 0 {
		t.Fatalf("partial files remain after failure: %#v, %v", matches, globErr)
	}
}

func TestDownloaderPropagatesTransportFailure(t *testing.T) {
	artifact := testAutomaticArtifact("rayleabot.zip", []byte("payload"), 1, 1)
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network unavailable")
	})}
	_, err := (&Downloader{HTTPClient: client, IdleTimeout: time.Second, TotalTimeout: time.Second}).Download(context.Background(), CheckResult{Status: "update_available", AvailableVersion: "1.2.0", Artifact: artifact}, t.TempDir(), nil)
	if CodeOf(err) != CodeArtifactInvalid {
		t.Fatalf("transport failure should be typed, got %v", err)
	}
}

func TestDownloaderRejectsInsufficientDiskBeforeNetworkAccess(t *testing.T) {
	artifact := testAutomaticArtifact("rayleabot.zip", []byte("payload"), 100, 1)
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("network request should not start without reserved disk space")
		return nil, nil
	})}
	downloader := &Downloader{
		HTTPClient:    client,
		IdleTimeout:   time.Second,
		TotalTimeout:  time.Second,
		FreeDiskBytes: func(string) (uint64, error) { return 1, nil },
	}
	_, err := downloader.Download(context.Background(), CheckResult{Status: "update_available", AvailableVersion: "1.2.0", Artifact: artifact}, t.TempDir(), nil)
	if CodeOf(err) != CodeDiskSpaceInsufficient {
		t.Fatalf("insufficient disk should be typed, got %v", err)
	}
}
