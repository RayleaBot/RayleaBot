package actions

import (
	"errors"
	"fmt"
	"net/url"
	"testing"
)

func TestHTTPHeaderLimitClassificationUsesOnlyTheTransportCause(t *testing.T) {
	for _, message := range []string{
		fmt.Sprintf("net/http: server response headers exceeded %d bytes; aborted", maxHTTPResponseHeaderBytes),
		"http2: response header list larger than advertised limit",
		"message too large",
	} {
		err := &url.Error{Op: "Get", URL: "https://example.invalid/", Err: fmt.Errorf("net/http: HTTP/1.x transport connection broken: %w", errors.New(message))}
		if !isResponseHeaderLimitError(err) {
			t.Fatalf("fixed-toolchain header limit was not classified: %v", err)
		}
	}
	for _, err := range []error{
		&url.Error{Op: "Get", URL: "https://example.invalid/message too large", Err: errors.New("unrelated transport failure")},
		errors.New("upstream said response headers exceeded a limit"),
		errors.New("request message too large"),
	} {
		if isResponseHeaderLimitError(err) {
			t.Fatalf("unrelated text was classified as a transport size error: %v", err)
		}
	}
}
