package pluginhttp

import (
	"context"
	"errors"
	"net"
	"time"
)

var (
	ErrInvalidRequest = errors.New("plugin http request is invalid")
	ErrScopeViolation = errors.New("plugin http request violates granted scope")
)

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type Config struct {
	Resolver          Resolver
	Timeout           time.Duration
	MaxRetries        int
	AllowPrivateHosts []string
}

type Request struct {
	Method        string
	URL           string
	Headers       map[string]string
	Body          []byte
	ActionTimeout time.Duration
}

type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

type Client struct {
	resolver          Resolver
	timeout           time.Duration
	maxRetries        int
	allowPrivateHosts map[string]struct{}
}

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &Client{
		resolver:          cfg.Resolver,
		timeout:           timeout,
		maxRetries:        maxRetries,
		allowPrivateHosts: toHostSet(cfg.AllowPrivateHosts),
	}
}

func (c *Client) Do(ctx context.Context, req Request, scopeHosts []string) (Response, error) {
	parsedURL, method, body, err := c.validateRequest(req)
	if err != nil {
		return Response{}, err
	}

	host := normalizeHost(parsedURL.Hostname())
	if host == "" {
		return Response{}, ErrInvalidRequest
	}
	if _, ok := toHostSet(scopeHosts)[host]; !ok {
		return Response{}, ErrScopeViolation
	}

	allowPrivateHost := c.hostAllowedForPrivate(host)
	preflightIPs, err := c.lookupAddrs(ctx, host)
	if err != nil {
		if parsedURL.Scheme == "http" && !allowPrivateHost {
			return Response{}, ErrInvalidRequest
		}
		return Response{}, err
	}
	if err := authorizeResolvedAddrs(preflightIPs, allowPrivateHost, hostUsesFakeIPDNS(host)); err != nil {
		return Response{}, err
	}
	if parsedURL.Scheme == "http" && !allowPrivateHost && !containsBogon(preflightIPs) {
		return Response{}, ErrInvalidRequest
	}

	deadline := c.timeout
	if req.ActionTimeout > 0 && req.ActionTimeout < deadline {
		deadline = req.ActionTimeout
	}
	if deadline <= 0 {
		return Response{}, ErrInvalidRequest
	}

	startedAt := time.Now()
	attempts := 0
	for {
		attempts++
		remaining := deadline - time.Since(startedAt)
		if remaining <= 0 {
			return Response{}, context.DeadlineExceeded
		}

		response, shouldRetry, err := c.doAttempt(ctx, attemptOptions{
			method:           method,
			url:              parsedURL,
			body:             body,
			headers:          req.Headers,
			scopeHosts:       scopeHosts,
			host:             host,
			allowPrivateHost: allowPrivateHost,
			remaining:        remaining,
		})
		if !shouldRetry || attempts >= c.maxRetries+1 {
			return response, err
		}
	}
}
