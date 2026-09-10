package qqofficial

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Gateway opcodes, confirmed against the live platform.
const (
	opDispatch     = 0
	opHeartbeat    = 1
	opIdentify     = 2
	opResume       = 6
	opReconnect    = 7
	opInvalidSess  = 9
	opHello        = 10
	opHeartbeatACK = 11
)

const (
	dispatchReady   = "READY"
	dispatchResumed = "RESUMED"
)

// Intent bits. The platform subscribes event families by bitmask; the config
// contract names them so an operator never writes a raw number.
var intentBits = map[string]int{
	"guilds":                1 << 0,
	"guild_members":         1 << 1,
	"guild_messages":        1 << 9,
	"direct_message":        1 << 12,
	"group_and_c2c":         1 << 25,
	"public_guild_messages": 1 << 30,
}

// IntentMask folds configured intent names into the gateway bitmask. Unknown
// names cannot occur: the config schema closes the vocabulary.
func IntentMask(intents []string) int {
	mask := 0
	for _, name := range intents {
		if bit, ok := intentBits[strings.TrimSpace(name)]; ok {
			mask |= bit
		}
	}
	return mask
}

type gatewayFrame struct {
	Op int             `json:"op"`
	S  int64           `json:"s"`
	T  string          `json:"t"`
	ID string          `json:"id"`
	D  json.RawMessage `json:"d"`
}

type helloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type readyData struct {
	SessionID string `json:"session_id"`
	User      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
}

// session carries what a Resume needs across a dropped connection.
type session struct {
	mu           sync.Mutex
	id           string
	lastSeq      int64
	botID        string
	botName      string
	botAvatarURL string
	resumable    bool
}

func (s *session) snapshot() (string, int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id, s.lastSeq, s.resumable && s.id != ""
}

func (s *session) observeSeq(seq int64) {
	if seq <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSeq = seq
}

func (s *session) startSession(id, botID, botName, botAvatarURL string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.id, s.botID, s.botName, s.resumable = id, botID, botName, true
	s.botAvatarURL = botAvatarURL
}

// invalidate drops resume state so the next attempt identifies afresh.
func (s *session) invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.id, s.lastSeq, s.resumable = "", 0, false
}

func (s *session) bot() (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.botID, s.botName
}

func (s *session) profile() botProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	return botProfile{ID: s.botID, Name: s.botName, AvatarURL: s.botAvatarURL}
}

func (s *session) clearIdentity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.botID, s.botName, s.botAvatarURL = "", "", ""
}

func (s *session) refreshProfile(profile botProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if profile.ID == "" || profile.ID != s.botID {
		return
	}
	if profile.Name != "" {
		s.botName = profile.Name
	}
	if profile.AvatarURL != "" {
		s.botAvatarURL = profile.AvatarURL
	}
}

// gatewayEndpoint asks the OpenAPI host where to connect.
func gatewayEndpoint(ctx context.Context, client *http.Client, base, appID, token string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/gateway", nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", AuthorizationHeader(token))
	request.Header.Set("X-Union-Appid", appID)

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("qqofficial: request gateway endpoint: %w", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)

	var payload struct {
		URL     string `json:"url"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("qqofficial: decode gateway endpoint: %w", err)
	}
	if strings.TrimSpace(payload.URL) == "" {
		message := payload.Message
		if message == "" {
			message = "response carried no gateway url"
		}
		return "", fmt.Errorf("qqofficial: gateway endpoint unavailable (http %d): %s", response.StatusCode, message)
	}
	return payload.URL, nil
}

// heartbeatInterval falls back to a conservative period when the platform does
// not state one. The live gateway assigns it per connection.
func heartbeatInterval(hello helloData) time.Duration {
	if hello.HeartbeatInterval > 0 {
		return time.Duration(hello.HeartbeatInterval) * time.Millisecond
	}
	return 30 * time.Second
}

// identifyPayload and resumePayload build the two ways a connection starts.
func identifyPayload(token string, intents int) ([]byte, error) {
	return json.Marshal(map[string]any{
		"op": opIdentify,
		"d": map[string]any{
			"token":      AuthorizationHeader(token),
			"intents":    intents,
			"shard":      []int{0, 1},
			"properties": map[string]any{},
		},
	})
}

func resumePayload(token, sessionID string, seq int64) ([]byte, error) {
	return json.Marshal(map[string]any{
		"op": opResume,
		"d": map[string]any{
			"token":      AuthorizationHeader(token),
			"session_id": sessionID,
			"seq":        seq,
		},
	})
}

func heartbeatPayload(seq int64) ([]byte, error) {
	var d any
	if seq > 0 {
		d = seq
	}
	return json.Marshal(map[string]any{"op": opHeartbeat, "d": d})
}
