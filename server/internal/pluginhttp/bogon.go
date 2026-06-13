package pluginhttp

import (
	"fmt"
	"log"
	"net/netip"
)

var bogonCIDRs []netip.Prefix

func init() {
	raw := []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"::/128",
		"::1/128",
		"fe80::/10",
		"fc00::/7",
		"fec0::/10",
		"ff00::/8",
	}
	var err error
	bogonCIDRs, err = parsePrefixes(raw...)
	if err != nil {
		// Hardcoded prefixes must parse successfully before the server starts.
		log.Fatalf("pluginhttp: %v", err)
	}
}

func parsePrefixes(raw ...string) ([]netip.Prefix, error) {
	items := make([]netip.Prefix, 0, len(raw))
	for _, value := range raw {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("parse bogon prefix %q: %w", value, err)
		}
		items = append(items, prefix)
	}
	return items, nil
}

func isBogon(ip netip.Addr) bool {
	if !ip.IsValid() {
		return true
	}
	if ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	for _, prefix := range bogonCIDRs {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func isFakeIPDNSAddr(ip netip.Addr) bool {
	return netip.MustParsePrefix("198.18.0.0/15").Contains(ip)
}
