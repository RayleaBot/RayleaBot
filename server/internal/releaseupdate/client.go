package releaseupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Checker struct {
	HTTPClient  *http.Client
	ManifestURL string
	Channel     string
}

func NewChecker() *Checker {
	return &Checker{
		HTTPClient:  newSecureHTTPClient(10 * time.Second),
		ManifestURL: ReleaseRepositoryURL + "/releases/latest/download/" + ManifestAssetName,
		Channel:     "stable",
	}
}

func newSecureHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return errors.New("too many HTTPS redirects")
			}
			if request.URL.Scheme != "https" || request.URL.User != nil {
				return errors.New("release redirect must use HTTPS without userinfo")
			}
			for _, previous := range via {
				if previous.URL.Scheme != "https" {
					return errors.New("release redirect attempted a protocol downgrade")
				}
			}
			return nil
		},
	}
}

func (c *Checker) Check(ctx context.Context, installRoot string) (CheckResult, error) {
	if c == nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "check release", errors.New("release checker is not configured"))
	}
	buildInfoPath := filepath.Join(installRoot, "build_info.json")
	buildInfoBytes, err := os.ReadFile(buildInfoPath)
	if err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "read build_info.json", err)
	}
	buildInfo, err := DecodeBuildInfo(buildInfoBytes)
	if err != nil {
		return CheckResult{}, err
	}

	manifestBytes, err := c.fetchMetadata(ctx, c.ManifestURL)
	if err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "download release manifest", err)
	}
	var manifest Manifest
	if err := decodeStrictJSON(manifestBytes, &manifest); err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "decode release metadata", err)
	}
	if err := validateManifest(manifest); err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "validate release metadata", err)
	}
	channel := c.Channel
	if channel == "" {
		channel = "stable"
	}
	if manifest.Channel != channel {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "select release channel", fmt.Errorf("expected %s channel, received %s", channel, manifest.Channel))
	}
	var artifact Artifact
	found := false
	for _, candidate := range manifest.Artifacts {
		if candidate.ArtifactID == buildInfo.ArtifactID {
			artifact, found = candidate, true
			break
		}
	}
	if !found {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "select artifact", fmt.Errorf("release does not contain %s", buildInfo.ArtifactID))
	}
	comparison, err := compareSemanticVersions(manifest.Version, buildInfo.Version)
	if err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "compare release versions", err)
	}
	status := "up_to_date"
	if comparison > 0 {
		status = "update_available"
	}
	return CheckResult{
		Status:           status,
		CurrentVersion:   buildInfo.Version,
		AvailableVersion: manifest.Version,
		UpdateMode:       artifact.UpdateMode,
		ReleasePageURL:   manifest.ReleaseNotesRef,
		Artifact:         artifact,
	}, nil
}

func (c *Checker) fetchMetadata(ctx context.Context, rawURL string) ([]byte, error) {
	if err := validateHTTPSURL(rawURL); err != nil {
		return nil, err
	}
	requestContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "RayleaBot-UpdateCheck/2")
	client := c.HTTPClient
	if client == nil {
		client = newSecureHTTPClient(10 * time.Second)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release metadata returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > MaxManifestBytes {
		return nil, fmt.Errorf("release metadata is too large")
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, MaxManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > MaxManifestBytes {
		return nil, fmt.Errorf("release metadata is too large")
	}
	return payload, nil
}
