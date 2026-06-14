package httpclient

import (
	"context"
	"net"
	"net/netip"
)

func (c *Client) resolveAndAuthorize(ctx context.Context, host string, allowPrivateHost bool) ([]netip.Addr, error) {
	ips, err := c.lookupAddrs(ctx, host)
	if err != nil {
		return nil, err
	}
	if err := authorizeResolvedAddrs(ips, allowPrivateHost, hostUsesFakeIPDNS(host)); err != nil {
		return nil, err
	}
	return ips, nil
}

func hostUsesFakeIPDNS(host string) bool {
	_, err := netip.ParseAddr(host)
	return err != nil
}

func authorizeResolvedAddrs(ips []netip.Addr, allowPrivateHost bool, allowFakeIPDNS bool) error {
	if len(ips) == 0 {
		return ErrInvalidRequest
	}
	for _, ip := range ips {
		if isBogon(ip) && !allowPrivateHost {
			if allowFakeIPDNS && isFakeIPDNSAddr(ip) {
				continue
			}
			return ErrScopeViolation
		}
	}
	return nil
}

func containsBogon(ips []netip.Addr) bool {
	for _, ip := range ips {
		if isBogon(ip) {
			return true
		}
	}
	return false
}

func (c *Client) lookupAddrs(ctx context.Context, host string) ([]netip.Addr, error) {
	if parsedIP, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{parsedIP.Unmap()}, nil
	}

	resolver := c.resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	ipAddrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	items := make([]netip.Addr, 0, len(ipAddrs))
	for _, ipAddr := range ipAddrs {
		if addr, ok := netip.AddrFromSlice(ipAddr.IP); ok {
			items = append(items, addr.Unmap())
		}
	}
	return items, nil
}

func (c *Client) hostAllowedForPrivate(host string) bool {
	_, ok := c.allowPrivateHosts[normalizeHost(host)]
	return ok
}
