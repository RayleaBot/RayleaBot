package app

import (
	"net/http/httptest"
	"testing"
)

func TestIsLoopbackRequestRejectsForwardedHeaders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       bool
	}{
		{
			name:       "plain ipv4 loopback",
			remoteAddr: "127.0.0.1:4321",
			want:       true,
		},
		{
			name:       "plain ipv6 loopback",
			remoteAddr: "[::1]:4321",
			want:       true,
		},
		{
			name:       "loopback with x-forwarded-for",
			remoteAddr: "127.0.0.1:4321",
			headers: map[string]string{
				"X-Forwarded-For": "198.51.100.9",
			},
			want: false,
		},
		{
			name:       "loopback with forwarded",
			remoteAddr: "127.0.0.1:4321",
			headers: map[string]string{
				"Forwarded": "for=198.51.100.9;proto=https",
			},
			want: false,
		},
		{
			name:       "loopback with x-real-ip",
			remoteAddr: "127.0.0.1:4321",
			headers: map[string]string{
				"X-Real-IP": "198.51.100.9",
			},
			want: false,
		},
		{
			name:       "non-loopback remote",
			remoteAddr: "198.51.100.9:4321",
			want:       false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest("GET", "http://example.test/api/launcher/status", nil)
			request.RemoteAddr = tc.remoteAddr
			for key, value := range tc.headers {
				request.Header.Set(key, value)
			}

			if got := isLoopbackRequest(request); got != tc.want {
				t.Fatalf("unexpected loopback result: got %v want %v", got, tc.want)
			}
		})
	}
}
