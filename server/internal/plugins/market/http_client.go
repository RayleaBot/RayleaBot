package market

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"time"
)

type lookupCatalogIPs func(context.Context, string, string) ([]netip.Addr, error)
type dialCatalogIP func(context.Context, string, string) (net.Conn, error)

func newCatalogHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// A proxy could resolve the original host itself and bypass the address check.
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = publicCatalogDialer(net.DefaultResolver.LookupNetIP, dialer.DialContext)
	return &http.Client{Timeout: 30 * time.Second, Transport: transport}
}

func publicCatalogDialer(lookup lookupCatalogIPs, dial dialCatalogIP) dialCatalogIP {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := lookup(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(addresses) == 0 {
			return nil, errors.New("plugin store host has no addresses")
		}
		// Validate the complete answer before connecting. Dial a checked IP so a
		// second DNS lookup cannot replace it with a private address.
		for _, ip := range addresses {
			if !publicCatalogIP(ip) {
				return nil, errors.New("plugin store source resolves to a non-public address")
			}
		}
		var failures []error
		for _, ip := range addresses {
			connection, err := dial(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return connection, nil
			}
			failures = append(failures, err)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		return nil, errors.Join(failures...)
	}
}

func publicCatalogIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.Zone() != "" {
		return false
	}
	// Shared address space and reserved IPv4 space are not public destinations.
	if ip.Is4() {
		address := ip.As4()
		if address[0] == 0 || address[0] >= 240 || address[0] == 100 && address[1]&0xc0 == 64 {
			return false
		}
	}
	return true
}
