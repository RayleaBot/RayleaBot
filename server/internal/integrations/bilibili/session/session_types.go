package session

import (
	"net/http"
	"sync"
	"time"
)

type PreparedCookie struct {
	Cookie    string
	Refreshed bool
	Enriched  bool
}

type SessionClient struct {
	client   *http.Client
	identity *IdentityProvider
	now      func() time.Time

	mu            sync.Mutex
	refreshChecks map[string]time.Time
	wbi           wbiKeyCache
	ticket        ticketCache
	device        deviceCookieCache
}

type wbiKeyCache struct {
	ImgKey    string
	SubKey    string
	ExpiresAt time.Time
}

type ticketCache struct {
	Ticket    string
	ExpiresAt time.Time
	WBI       wbiKeyCache
}

type deviceCookieCache struct {
	Buvid3    string
	Buvid4    string
	ExpiresAt time.Time
}
