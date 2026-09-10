package netease_music

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type offlineTransport func(*http.Request) (*http.Response, error)

func (f offlineTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func offlineResponse(r *http.Request, status int, body string, header http.Header) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: header, Request: r}
}

func TestCreateUsesInjectedTransportAndRedirectCookies(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	var paths []string
	client := thirdparty.NewHTTPClient(offlineTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "music.163.com" {
			t.Fatalf("unexpected host %s", r.URL.Host)
		}
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/":
			return offlineResponse(r, 302, "", http.Header{"Location": {"/bootstrap"}, "Set-Cookie": {"__csrf=offline-csrf; Path=/"}}), nil
		case "/bootstrap":
			if cookie, err := r.Cookie("__csrf"); err != nil || cookie.Value != "offline-csrf" {
				t.Fatalf("redirect cookie missing: %v", err)
			}
			return offlineResponse(r, 200, "offline homepage", nil), nil
		case "/weapi/login/qrcode/unikey":
			if r.Method != http.MethodPost || r.URL.Query().Get("csrf_token") != "offline-csrf" {
				t.Fatalf("invalid unikey request: %s %s", r.Method, r.URL)
			}
			if err := r.ParseForm(); err != nil || r.Form.Get("params") == "" || len(r.Form.Get("encSecKey")) != 256 {
				t.Fatalf("invalid encrypted form: %v", err)
			}
			return offlineResponse(r, 200, `{"code":200,"unikey":"offline-qr"}`, nil), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
			return nil, errors.New("unexpected request")
		}
	}))
	session, err := NewProvider(client).Create(t.Context(), now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, []string{"/", "/bootstrap", "/weapi/login/qrcode/unikey"}) {
		t.Fatalf("request sequence %v", paths)
	}
	qr, err := url.Parse(session.QRCodeURL)
	if err != nil || qr.Query().Get("codekey") != "offline-qr" || session.Token != "offline-qr" || session.State != thirdparty.QRLoginStatePendingScan || !session.ExpiresAt.Equal(now.Add(3*time.Minute)) {
		t.Fatalf("session: %+v, %v", session, err)
	}
}

func TestPollOfflineStateSequenceAndCookieIsolation(t *testing.T) {
	responses := []string{
		`{"code":801}`,
		`{"code":802,"profile":{"userId":42,"nickname":"Offline User"}}`,
		`{"code":803,"cookie":"MUSIC_U=offline-only; __csrf=offline-csrf"}`,
	}
	want := []string{thirdparty.QRLoginStatePendingScan, thirdparty.QRLoginStatePendingConfirm, thirdparty.QRLoginStateSucceeded}
	var calls int
	provider := NewProvider(thirdparty.NewHTTPClient(offlineTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != neteaseQRCodeCheckURL || r.Method != http.MethodPost || calls >= len(responses) {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
		}
		response := offlineResponse(r, 200, responses[calls], nil)
		calls++
		return response, nil
	})))
	original := map[string]string{"os": "pc"}
	session := thirdparty.QRLoginSession{Token: "offline-key", Cookies: original}
	for i := range responses {
		var err error
		session, err = provider.Poll(t.Context(), session, time.Time{})
		if err != nil || session.State != want[i] {
			t.Fatalf("poll %d: %+v, %v", i, session, err)
		}
	}
	if calls != 3 || session.Account.UID != "42" || session.Account.Nickname != "Offline User" || !HasLoginCookie(thirdparty.CookieMapFromHeader(session.Cookie)) {
		t.Fatalf("completed session: %+v, calls=%d", session, calls)
	}
	if len(original) != 1 || original["MUSIC_U"] != "" {
		t.Fatalf("caller cookies mutated: %v", original)
	}
}

func TestPollRejectsSuccessWithoutLoginCredential(t *testing.T) {
	provider := NewProvider(thirdparty.NewHTTPClient(offlineTransport(func(r *http.Request) (*http.Response, error) {
		return offlineResponse(r, 200, `{"code":803}`, nil), nil
	})))
	session, err := provider.Poll(t.Context(), thirdparty.QRLoginSession{Token: "offline-key", Cookies: map[string]string{"os": "pc", "__csrf": "tracking-only"}}, time.Time{})
	if err == nil || session.State == thirdparty.QRLoginStateSucceeded || session.Cookie != "" {
		t.Fatalf("accepted tracking cookies as login credentials: %+v, %v", session, err)
	}
}

func TestPollExpiredMalformedAndRejectedResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		failed bool
	}{
		{"expired", 200, `{"code":800}`, false},
		{"unknown code", 200, `{"code":500,"message":"offline failure"}`, true},
		{"malformed", 200, `{`, true},
		{"http failure", 503, `{}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewProvider(thirdparty.NewHTTPClient(offlineTransport(func(r *http.Request) (*http.Response, error) {
				return offlineResponse(r, tc.status, tc.body, nil), nil
			})))
			session, err := provider.Poll(t.Context(), thirdparty.QRLoginSession{Token: "offline-key"}, time.Time{})
			if (err != nil) != tc.failed || (!tc.failed && session.State != thirdparty.QRLoginStateExpired) {
				t.Fatalf("poll: %+v, %v", session, err)
			}
		})
	}
}

func TestCreateCancellationStopsBeforeUnikey(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var calls int
	provider := NewProvider(thirdparty.NewHTTPClient(offlineTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		cancel()
		return nil, context.Canceled
	})))
	if _, err := provider.Create(ctx, time.Now()); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancelled create: calls=%d, err=%v", calls, err)
	}
}
