package qqofficial

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// errReloadRequested ends the current connection because the settings behind it
// changed. It is not a failure, so the loop redials at once instead of backing
// off as it would after a drop.
var errReloadRequested = errors.New("qqofficial: settings reloaded")

// connectionSettings are the fields a reload can change. Every one of them is
// fixed at connect time — the credential opens the gateway, the intent mask is
// sent at Identify, and the sandbox switch selects the host — so a change to
// any of them takes effect by reconnecting rather than by mutating a live
// connection.
type connectionSettings struct {
	appID     string
	appSecret string
	sandbox   bool
	intents   []string
}

func connectionSettingsOf(qq config.QQOfficialConfig) connectionSettings {
	return connectionSettings{
		appID:     strings.TrimSpace(qq.AppID),
		appSecret: strings.TrimSpace(qq.AppSecret),
		sandbox:   qq.Sandbox,
		intents:   slices.Clone(qq.Intents),
	}
}

func (s connectionSettings) equal(other connectionSettings) bool {
	return s.appID == other.appID &&
		s.appSecret == other.appSecret &&
		s.sandbox == other.sandbox &&
		slices.Equal(s.intents, other.intents)
}

// Reload applies new settings to this adapter. It reports whether anything the
// connection depends on changed; when nothing did, the live connection is left
// alone rather than being dropped for no reason.
func (c *Client) Reload(qq config.QQOfficialConfig) bool {
	next := connectionSettingsOf(qq)

	c.settingsMu.Lock()
	if c.settings.equal(next) {
		c.settingsMu.Unlock()
		return false
	}
	c.settings = next
	c.appID = next.appID
	c.sandbox = next.sandbox
	c.apiBase = apiBaseURL(next.sandbox)
	c.intents = IntentMask(next.intents)
	c.tokens = NewTokenSource(next.appID, next.appSecret, c.http)
	c.settingsMu.Unlock()

	// A resumed session carries the intents and identity of the connection that
	// opened it, so the next attempt has to identify afresh.
	c.session.invalidate()
	c.dropConnection()
	return true
}

// currentSettings reads the fields one connection attempt runs with, so an
// attempt is not affected halfway through by a reload.
func (c *Client) currentSettings() (appID, apiBase string, intents int, tokens *TokenSource) {
	c.settingsMu.RLock()
	defer c.settingsMu.RUnlock()
	return c.appID, c.apiBase, c.intents, c.tokens
}

// setConnectionCancel records how to end the connection currently running, so a
// reload can close it without waiting for the platform to drop it.
func (c *Client) setConnectionCancel(cancel context.CancelFunc) {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	c.connCancel = cancel
}

func (c *Client) dropConnection() {
	c.settingsMu.Lock()
	cancel := c.connCancel
	c.reloading = true
	c.settingsMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// takeReloading reports whether the connection that just ended was ended by a
// reload, and clears the flag.
func (c *Client) takeReloading() bool {
	c.settingsMu.Lock()
	defer c.settingsMu.Unlock()
	reloading := c.reloading
	c.reloading = false
	return reloading
}
