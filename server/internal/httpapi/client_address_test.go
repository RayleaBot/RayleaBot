package httpapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClientAddressesDoNotChangeListenerAuthorities(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("..", "..", "testdata", "client-addresses.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Listen string `json:"listen"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(payload, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Listen, func(t *testing.T) {
			if got := DisplayServerURL(tc.Listen); got != tc.URL {
				t.Fatalf("URL=%s, want %s", got, tc.URL)
			}
		})
	}
}
