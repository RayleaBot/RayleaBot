package releaseupdate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Checker struct {
	Verifier     *Verifier
	HTTPClient   *http.Client
	Now          func() time.Time
	ManifestURL  string
	SignatureURL string
	Channel      string
}

func NewChecker(verifier *Verifier) *Checker {
	return &Checker{
		Verifier:     verifier,
		HTTPClient:   newSecureHTTPClient(10 * time.Second),
		Now:          time.Now,
		ManifestURL:  ReleaseRepositoryURL + "/releases/latest/download/" + ManifestAssetName,
		SignatureURL: ReleaseRepositoryURL + "/releases/latest/download/" + SignatureAssetName,
		Channel:      "stable",
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
	if c == nil || c.Verifier == nil {
		return CheckResult{}, errorWithCode(CodeTrustRequired, "check release", errors.New("release verifier is not configured"))
	}
	buildInfoPath := filepath.Join(installRoot, "build_info.json")
	buildInfoBytes, err := os.ReadFile(buildInfoPath)
	if err != nil {
		return CheckResult{}, errorWithCode(CodeTrustRequired, "read build_info.json", err)
	}
	buildInfo, err := DecodeBuildInfo(buildInfoBytes)
	if err != nil {
		return CheckResult{}, err
	}

	manifestBytes, err := c.fetchMetadata(ctx, c.ManifestURL)
	if err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "download release manifest", err)
	}
	signatureBytes, err := c.fetchMetadata(ctx, c.SignatureURL)
	if err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "download release signature", err)
	}
	verified, err := c.Verifier.Verify(manifestBytes, signatureBytes, c.now())
	if err != nil {
		return CheckResult{}, err
	}
	channel := c.Channel
	if channel == "" {
		channel = "stable"
	}
	if verified.Manifest.Channel != channel {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "select release channel", fmt.Errorf("expected %s channel, received %s", channel, verified.Manifest.Channel))
	}
	artifact, found := verified.ArtifactByID(buildInfo.ArtifactID)
	if !found {
		return CheckResult{}, errorWithCode(CodeUpdateNotSupported, "select release artifact", fmt.Errorf("manifest does not contain %s", buildInfo.ArtifactID))
	}
	if err := ObserveManifest(filepath.Join(installRoot, "data", "update-trust.json"), buildInfo, verified, c.now()); err != nil {
		return CheckResult{}, err
	}

	cacheRoot := filepath.Join(installRoot, "cache", "downloads", "updates")
	manifestPath := filepath.Join(cacheRoot, ManifestAssetName)
	signaturePath := filepath.Join(cacheRoot, SignatureAssetName)
	if err := saveBytesAtomically(manifestPath, manifestBytes, 0o600); err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "cache release manifest", err)
	}
	if err := saveBytesAtomically(signaturePath, signatureBytes, 0o600); err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "cache release signature", err)
	}

	comparison, err := compareSemanticVersions(verified.Manifest.Version, buildInfo.Version)
	if err != nil {
		return CheckResult{}, errorWithCode(CodeManifestInvalid, "compare release versions", err)
	}
	effectiveMode := artifact.UpdateMode
	automaticAllowed := artifact.UpdateMode == "automatic" &&
		artifact.ArtifactID == "windows-x64-full" &&
		artifact.MinUpdaterProtocolVersion <= ProtocolVersion &&
		artifact.WindowsSignerSHA256 != ""
	if !automaticAllowed && effectiveMode == "automatic" {
		effectiveMode = "guided"
	}
	status := "up_to_date"
	if comparison > 0 {
		status = "update_available"
	}
	return CheckResult{
		Status:           status,
		CurrentVersion:   buildInfo.Version,
		AvailableVersion: verified.Manifest.Version,
		UpdateMode:       effectiveMode,
		ReleasePageURL:   verified.Manifest.ReleaseNotesRef,
		AutomaticAllowed: automaticAllowed,
		ManifestPath:     manifestPath,
		SignaturePath:    signaturePath,
		Artifact:         artifact,
		ManifestDigest:   verified.Digest,
		TrustedKeyIDs:    append([]string(nil), verified.TrustedKeyIDs...),
	}, nil
}

func (c *Checker) now() time.Time {
	if c.Now == nil {
		return time.Now().UTC()
	}
	return c.Now().UTC()
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
	request.Header.Set("User-Agent", "RayleaBot-Updater/2")
	client := c.HTTPClient
	if client == nil {
		client = newSecureHTTPClient(10 * time.Second)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
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

func artifactDownloadURL(version, fileName string) (string, error) {
	if _, err := parseSemanticVersion(version); err != nil || !fileNamePattern.MatchString(fileName) {
		return "", fmt.Errorf("invalid release version or artifact basename")
	}
	return ReleaseRepositoryURL + "/releases/download/v" + version + "/" + url.PathEscape(fileName), nil
}

func saveBytesAtomically(path string, payload []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func ValidateMetadataFiles(verifier *Verifier, manifestPath, signaturePath string, now time.Time) (VerifiedManifest, error) {
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return VerifiedManifest{}, errorWithCode(CodeManifestInvalid, "read manifest file", err)
	}
	signatureBytes, err := os.ReadFile(signaturePath)
	if err != nil {
		return VerifiedManifest{}, errorWithCode(CodeManifestInvalid, "read signature file", err)
	}
	return verifier.Verify(manifestBytes, signatureBytes, now)
}

func samePath(left, right string) bool {
	leftAbsolute, leftErr := filepath.Abs(left)
	rightAbsolute, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && strings.EqualFold(filepath.Clean(leftAbsolute), filepath.Clean(rightAbsolute))
}
