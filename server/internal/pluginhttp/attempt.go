package pluginhttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type attemptOptions struct {
	method           string
	url              *url.URL
	body             []byte
	headers          map[string]string
	scopeHosts       []string
	host             string
	allowPrivateHost bool
	remaining        time.Duration
}

func (c *Client) doAttempt(ctx context.Context, opts attemptOptions) (Response, bool, error) {
	if _, err := c.resolveAndAuthorize(ctx, opts.host, opts.allowPrivateHost); err != nil {
		return Response{}, false, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, opts.remaining)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(requestCtx, opts.method, opts.url.String(), bytes.NewReader(opts.body))
	if err != nil {
		return Response{}, false, ErrInvalidRequest
	}
	for key, value := range opts.headers {
		httpRequest.Header.Set(key, value)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableCompression = false
	transport.DialContext = func(innerCtx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		normalizedHost := normalizeHost(host)
		ips, err := c.resolveAndAuthorize(innerCtx, normalizedHost, c.hostAllowedForPrivate(normalizedHost))
		if err != nil {
			return nil, err
		}
		dialer := &net.Dialer{}
		var lastErr error
		for _, ip := range ips {
			conn, err := dialer.DialContext(innerCtx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			lastErr = ErrScopeViolation
		}
		return nil, lastErr
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		if errors.Is(err, ErrScopeViolation) || errors.Is(err, ErrInvalidRequest) {
			return Response{}, false, err
		}
		retryable := isRetryableTransportError(opts.method, err)
		return Response{}, retryable, err
	}
	defer httpResponse.Body.Close()

	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return Response{}, false, err
	}

	response := Response{
		StatusCode: httpResponse.StatusCode,
		Headers:    flattenHeaders(httpResponse.Header),
		Body:       body,
	}
	if isRetryableStatus(opts.method, httpResponse.StatusCode) {
		return response, true, nil
	}
	return response, false, nil
}

func flattenHeaders(header http.Header) map[string]string {
	if len(header) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(header))
	for key, values := range header {
		result[key] = strings.Join(values, ", ")
	}
	return result
}

func isRetryableTransportError(method string, err error) bool {
	if method != "GET" && method != "HEAD" {
		return false
	}
	return err != nil
}

func isRetryableStatus(method string, status int) bool {
	if method != "GET" && method != "HEAD" {
		return false
	}
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
