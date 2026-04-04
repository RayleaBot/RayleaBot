package plugins

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const maxRemoteDownloadBytes = 256 * 1024 * 1024 // 256 MB

func downloadHTTPSFile(ctx context.Context, rawURL, destPath string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("invalid HTTPS URL: %s", rawURL)
	}

	client := &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote server returned HTTP %d", resp.StatusCode)
	}

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	limitedReader := io.LimitReader(resp.Body, maxRemoteDownloadBytes+1)
	written, err := io.Copy(outFile, limitedReader)
	if err != nil {
		return err
	}
	if written > maxRemoteDownloadBytes {
		return fmt.Errorf("download exceeded maximum size of %d bytes", maxRemoteDownloadBytes)
	}
	return nil
}
