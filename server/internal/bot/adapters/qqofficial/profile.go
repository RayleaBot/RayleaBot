package qqofficial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type botProfile struct {
	ID        string `json:"id"`
	Name      string `json:"username"`
	AvatarURL string `json:"avatar"`
}

// fetchBotProfile is optional enrichment for this connection. Failure leaves
// READY as the identity source and must not prevent connecting to the gateway.
func fetchBotProfile(ctx context.Context, client *http.Client, base, appID, token string) (botProfile, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/users/@me", nil)
	if err != nil {
		return botProfile{}, fmt.Errorf("create profile request: %w", err)
	}
	request.Header.Set("Authorization", AuthorizationHeader(token))
	request.Header.Set("X-Union-Appid", appID)
	response, err := client.Do(request)
	if err != nil {
		return botProfile{}, fmt.Errorf("request profile: %w", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusOK {
		return botProfile{}, fmt.Errorf("request profile: HTTP %d", response.StatusCode)
	}
	const maxProfileBytes = 64 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProfileBytes+1))
	if err != nil {
		return botProfile{}, fmt.Errorf("read profile: %w", err)
	}
	if len(body) > maxProfileBytes {
		return botProfile{}, errors.New("profile response exceeds 64 KiB")
	}
	var profile botProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return botProfile{}, fmt.Errorf("decode profile: %w", err)
	}
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.AvatarURL = strings.TrimSpace(profile.AvatarURL)
	avatar, err := url.Parse(profile.AvatarURL)
	if err == nil && avatar.Scheme == "http" {
		host := strings.ToLower(avatar.Hostname())
		if host == "qlogo.cn" || strings.HasSuffix(host, ".qlogo.cn") || host == "qpic.cn" || strings.HasSuffix(host, ".qpic.cn") {
			avatar.Scheme = "https"
			profile.AvatarURL = avatar.String()
		}
	}
	if err != nil || avatar.Scheme != "https" || avatar.Hostname() == "" || avatar.User != nil || len(profile.AvatarURL) > 2048 {
		profile.AvatarURL = ""
		return profile, errors.New("profile avatar is missing or is not a valid HTTPS URL")
	}
	return profile, nil
}
