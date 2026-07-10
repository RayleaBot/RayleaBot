package releaseupdate

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestSecureReleaseClientRejectsDowngradeUserinfoAndTooManyRedirects(t *testing.T) {
	client := newSecureHTTPClient(time.Second)
	httpsRequest := &http.Request{URL: &url.URL{Scheme: "https", Host: "github.com"}}
	tests := []struct {
		name    string
		request *http.Request
		via     []*http.Request
	}{
		{name: "protocol downgrade", request: &http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}}, via: []*http.Request{httpsRequest}},
		{name: "userinfo", request: &http.Request{URL: &url.URL{Scheme: "https", Host: "example.com", User: url.User("attacker")}}, via: []*http.Request{httpsRequest}},
		{name: "too many", request: httpsRequest, via: []*http.Request{httpsRequest, httpsRequest, httpsRequest, httpsRequest, httpsRequest, httpsRequest}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := client.CheckRedirect(test.request, test.via); err == nil {
				t.Fatal("unsafe redirect was accepted")
			}
		})
	}
}

func TestArtifactDownloadURLUsesFixedRepositoryAndSignedBasename(t *testing.T) {
	result, err := artifactDownloadURL("1.2.3", "RayleaBot-v1.2.3-windows-x64-full.zip")
	if err != nil {
		t.Fatal(err)
	}
	want := ReleaseRepositoryURL + "/releases/download/v1.2.3/RayleaBot-v1.2.3-windows-x64-full.zip"
	if result != want {
		t.Fatalf("download URL = %q, want %q", result, want)
	}
	if _, err := artifactDownloadURL("1.2.3", "../artifact.zip"); err == nil {
		t.Fatal("non-basename artifact was accepted")
	}
}
