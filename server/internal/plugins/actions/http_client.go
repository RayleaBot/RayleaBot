package actions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	errHTTPInvalidRequest   = errors.New("render resource request is invalid")
	errHTTPResponseTooLarge = errors.New("render resource response exceeded resource limits")
)

const maxHTTPResponseHeaderBytes int64 = 1024 * 1024

type httpClientRequest struct {
	URL                string
	Headers            map[string]string
	ResponseBodyWriter io.Writer
}

type httpClientResponse struct {
	StatusCode int
	Headers    map[string]string
	BodyBytes  int64
}

// httpClient downloads render.image resources with fixed time and size limits.
// Redirects are returned to the caller, which resolves every hop explicitly.
type httpClient struct {
	client               *http.Client
	maxResponseBodyBytes int64
}

func newHTTPClient(timeout time.Duration, maxResponseBodyBytes int64) *httpClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxResponseHeaderBytes = maxHTTPResponseHeaderBytes
	return &httpClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		maxResponseBodyBytes: maxResponseBodyBytes,
	}
}

func (c *httpClient) close() {
	c.client.CloseIdleConnections()
}

func (c *httpClient) get(ctx context.Context, req httpClientRequest) (httpClientResponse, error) {
	target, err := url.Parse(strings.TrimSpace(req.URL))
	if err != nil || target.Hostname() == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return httpClientResponse{}, errHTTPInvalidRequest
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return httpClientResponse{}, errHTTPInvalidRequest
	}
	for key, value := range req.Headers {
		request.Header.Set(key, value)
	}
	response, err := c.client.Do(request)
	if err != nil {
		if isResponseHeaderLimitError(err) {
			return httpClientResponse{}, errHTTPResponseTooLarge
		}
		return httpClientResponse{}, err
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)

	result := httpClientResponse{StatusCode: response.StatusCode, Headers: flattenHeaders(response.Header)}
	if response.ContentLength > c.maxResponseBodyBytes {
		return result, errHTTPResponseTooLarge
	}
	written, err := io.Copy(req.ResponseBodyWriter, io.LimitReader(response.Body, c.maxResponseBodyBytes+1))
	if err != nil {
		return result, err
	}
	if written > c.maxResponseBodyBytes {
		return result, errHTTPResponseTooLarge
	}
	result.BodyBytes = written
	return result, nil
}

func flattenHeaders(header http.Header) map[string]string {
	result := make(map[string]string, len(header))
	for key, values := range header {
		result[key] = strings.Join(values, ", ")
	}
	return result
}

func isResponseHeaderLimitError(err error) bool {
	if err == nil {
		return false
	}
	// Go's HTTP/1, HTTP/2 and MIME size errors have no exported sentinel.
	// Match only the fixed toolchain's transport cause, never the request URL.
	for cause := errors.Unwrap(err); cause != nil; cause = errors.Unwrap(err) {
		err = cause
	}
	message := err.Error()
	return message == fmt.Sprintf("net/http: server response headers exceeded %d bytes; aborted", maxHTTPResponseHeaderBytes) ||
		message == "http2: response header list larger than advertised limit" || message == "message too large"
}
