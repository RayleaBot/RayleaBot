package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func validateRuntimeConstraints(cfg Config) error {
	if cfg.Admin.SessionAbsoluteTTLDays < cfg.Admin.SessionTTLDays {
		return fmt.Errorf("admin.session_absolute_ttl_days must be greater than or equal to admin.session_ttl_days")
	}

	hostIP, loopback, private, err := classifyBindHost(cfg.Server.Host)
	if err != nil {
		return err
	}

	switch cfg.Web.ExposureMode {
	case "localhost_only":
		if !loopback {
			return fmt.Errorf("web.exposure_mode localhost_only requires a loopback server.host")
		}
		if len(cfg.Web.TrustedProxyCIDRs) != 0 {
			return fmt.Errorf("web.trusted_proxy_cidrs is only valid with public_via_reverse_proxy")
		}
	case "lan_enabled":
		if hostIP == nil || hostIP.IsUnspecified() || (!loopback && !private) {
			return fmt.Errorf("web.exposure_mode lan_enabled requires a loopback or private server.host, not a wildcard or public address")
		}
		if len(cfg.Web.TrustedProxyCIDRs) != 0 {
			return fmt.Errorf("web.trusted_proxy_cidrs is only valid with public_via_reverse_proxy")
		}
		if strings.TrimSpace(cfg.Web.PluginUIOriginTemplate) == "" {
			return fmt.Errorf("web.plugin_ui_origin_template is required for lan_enabled")
		}
	case "public_via_reverse_proxy":
		if !loopback {
			return fmt.Errorf("web.exposure_mode public_via_reverse_proxy requires a loopback server.host")
		}
		origin, err := url.Parse(strings.TrimSpace(cfg.Web.PublicOrigin))
		if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
			return fmt.Errorf("web.public_origin must be an HTTPS origin for public_via_reverse_proxy")
		}
		if len(cfg.Web.TrustedProxyCIDRs) == 0 {
			return fmt.Errorf("web.trusted_proxy_cidrs must not be empty for public_via_reverse_proxy")
		}
		if strings.TrimSpace(cfg.Web.PluginUIOriginTemplate) == "" {
			return fmt.Errorf("web.plugin_ui_origin_template is required for public_via_reverse_proxy")
		}
	default:
		return fmt.Errorf("unsupported web.exposure_mode %q", cfg.Web.ExposureMode)
	}

	if template := strings.TrimSpace(cfg.Web.PluginUIOriginTemplate); template != "" {
		if !strings.Contains(template, "{plugin_host}") {
			return fmt.Errorf("web.plugin_ui_origin_template must contain {plugin_host}")
		}
		rendered := strings.ReplaceAll(template, "{plugin_host}", "p-0123456789abcdef")
		origin, err := url.Parse(rendered)
		if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
			return fmt.Errorf("web.plugin_ui_origin_template must render to an HTTP(S) origin")
		}
	}

	for _, rawCIDR := range cfg.Web.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(rawCIDR)); err != nil {
			return fmt.Errorf("invalid web.trusted_proxy_cidrs entry %q", rawCIDR)
		}
	}
	return nil
}

func classifyBindHost(raw string) (net.IP, bool, bool, error) {
	host := strings.TrimSpace(strings.Trim(raw, "[]"))
	if strings.EqualFold(host, "localhost") {
		return net.ParseIP("127.0.0.1"), true, true, nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, false, false, fmt.Errorf("server.host must be an IP address or localhost")
	}
	return ip, ip.IsLoopback(), ip.IsPrivate(), nil
}
