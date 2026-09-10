package qqofficial

import (
	"context"
	"encoding/json"
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
func fetchBotProfile(ctx context.Context, client *http.Client, base, appID, token string) botProfile {
	ctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/users/@me", nil)
	if err != nil {
		return botProfile{}
	}
	request.Header.Set("Authorization", AuthorizationHeader(token))
	request.Header.Set("X-Union-Appid", appID)
	response, err := client.Do(request)
	if err != nil {
		return botProfile{}
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusOK {
		return botProfile{}
	}
	const maxProfileBytes = 64 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProfileBytes+1))
	if err != nil || len(body) > maxProfileBytes {
		return botProfile{}
	}
	var profile botProfile
	if json.Unmarshal(body, &profile) != nil {
		return botProfile{}
	}
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.AvatarURL = strings.TrimSpace(profile.AvatarURL)
	avatar, err := url.Parse(profile.AvatarURL)
	if err != nil || avatar.Scheme != "https" || avatar.Hostname() == "" || avatar.User != nil || len(profile.AvatarURL) > 2048 {
		profile.AvatarURL = ""
	}
	return profile
}
