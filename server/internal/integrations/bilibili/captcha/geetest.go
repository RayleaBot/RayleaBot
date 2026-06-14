package captcha

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

const geetestJSURL = "https://static.geetest.com/static/js/fullpage.9.2.4.js"

var (
	geetestKeyMu      sync.Mutex
	geetestKeyPattern = regexp.MustCompile(`"static_key"\s*:\s*"([a-fA-F0-9]+)"`)
	geetestKeyVal     string
)

// attemptGeetestBypass tries a headless geetest v4 bypass using static key extraction.
func (c *CaptchaClient) attemptGeetestBypass(ctx context.Context, challenge *CaptchaChallenge) (string, string, string, error) {
	key, err := fetchGeetestKey(ctx, c.client)
	if err != nil {
		return "", "", "", fmt.Errorf("geetest key fetch: %w", err)
	}
	return computeGeetestResponse(challenge, key)
}

// fetchGeetestKey fetches the geetest JS file and extracts the static key, with caching.
func fetchGeetestKey(ctx context.Context, client *http.Client) (string, error) {
	geetestKeyMu.Lock()
	cached := geetestKeyVal
	geetestKeyMu.Unlock()
	if cached != "" {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geetestJSURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	js, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	key := extractGeetestKey(string(js))
	if key == "" {
		return "", fmt.Errorf("geetest static key not found in JS")
	}

	geetestKeyMu.Lock()
	geetestKeyVal = key
	geetestKeyMu.Unlock()
	return key, nil
}

func extractGeetestKey(js string) string {
	match := geetestKeyPattern.FindStringSubmatch(js)
	if len(match) >= 2 {
		return strings.ToLower(match[1])
	}
	return ""
}

func computeGeetestResponse(challenge *CaptchaChallenge, key string) (token, validate, seccode string, err error) {
	raw := challenge.Challenge + key
	hash := md5.Sum([]byte(raw))
	validate = hex.EncodeToString(hash[:])[:16]
	seccode = validate + "|jordan"

	tokenRaw := challenge.Challenge + key + challenge.GT
	tokenHash := md5.Sum([]byte(tokenRaw))
	token = hex.EncodeToString(tokenHash[:])

	return token, validate, seccode, nil
}
