package proxy

import (
	"testing"
)

func TestNewProxyPoolNilConfigs(t *testing.T) {
	t.Parallel()
	pool := NewProxyPool(nil)
	if pool == nil {
		t.Fatal("NewProxyPool(nil) returned nil")
	}
	if pool.IsEnabled() {
		t.Fatal("NewProxyPool(nil) should not be enabled")
	}
	if pool.Transport() != nil {
		t.Fatal("NewProxyPool(nil) Transport should be nil")
	}
}

func TestNewProxyPoolEmptyConfigs(t *testing.T) {
	t.Parallel()
	pool := NewProxyPool([]ProxyConfig{})
	if pool == nil {
		t.Fatal("NewProxyPool([]) returned nil")
	}
	if pool.IsEnabled() {
		t.Fatal("empty pool should not be enabled")
	}
	if pool.Transport() != nil {
		t.Fatal("empty pool Transport should be nil")
	}
}

func TestNewProxyPoolWithConfigs(t *testing.T) {
	t.Parallel()
	configs := []ProxyConfig{
		{URL: "http://proxy1.example.com:8080", Scope: "bilibili"},
		{URL: "http://proxy2.example.com:8080", Scope: "bilibili"},
	}
	pool := NewProxyPool(configs)
	if pool == nil {
		t.Fatal("NewProxyPool(configs) returned nil")
	}
	// Pool is enabled by default when configs exist.
	if !pool.IsEnabled() {
		t.Fatal("pool with configs should be enabled by default")
	}
	// Transport is non-nil when pool is enabled with configs.
	if pool.Transport() == nil {
		t.Fatal("enabled pool Transport should be non-nil")
	}
}

func TestProxyPoolSetEnabled(t *testing.T) {
	t.Parallel()
	configs := []ProxyConfig{
		{URL: "http://proxy1.example.com:8080", Scope: "bilibili"},
	}
	pool := NewProxyPool(configs)
	if !pool.IsEnabled() {
		t.Fatal("pool with configs should be enabled by default")
	}
	pool.SetEnabled(false)
	if pool.IsEnabled() {
		t.Fatal("pool should be disabled after SetEnabled(false)")
	}
	pool.SetEnabled(true)
	if !pool.IsEnabled() {
		t.Fatal("pool should be enabled after SetEnabled(true)")
	}
}

func TestProxyPoolTransportNilWhenDisabled(t *testing.T) {
	t.Parallel()
	configs := []ProxyConfig{
		{URL: "http://proxy1.example.com:8080", Scope: "bilibili"},
	}
	pool := NewProxyPool(configs)
	pool.SetEnabled(false)
	if pool.Transport() != nil {
		t.Fatal("disabled pool Transport should be nil")
	}
}

func TestProxyPoolTransportRoundTripCyclesProxies(t *testing.T) {
	t.Parallel()
	configs := []ProxyConfig{
		{URL: "http://proxy1.example.com:8080", Scope: "bilibili"},
		{URL: "http://proxy2.example.com:8080", Scope: "bilibili"},
	}
	pool := NewProxyPool(configs)
	transport := pool.Transport()
	if transport == nil {
		t.Fatal("Transport should be non-nil for enabled pool with configs")
	}

	// nextTransport cycles through proxies internally.
	tr1 := pool.nextTransport()
	tr2 := pool.nextTransport()
	tr3 := pool.nextTransport()

	if tr1 == nil || tr2 == nil || tr3 == nil {
		t.Fatal("nextTransport should be non-nil")
	}
	// After 2 calls for 2 proxies, we cycle back to the first.
	if tr1 != tr3 {
		t.Fatal("nextTransport should cycle back to first proxy after round-robin")
	}
}

func TestProxyPoolNoProxiesTransportNil(t *testing.T) {
	t.Parallel()
	pool := NewProxyPool([]ProxyConfig{})
	if pool.Transport() != nil {
		t.Fatal("Transport should be nil when pool has no proxies")
	}
}

func TestProxyPoolIsEnabledNilReceiver(t *testing.T) {
	t.Parallel()
	var pool *ProxyPool
	if pool.IsEnabled() {
		t.Fatal("nil ProxyPool IsEnabled should be false")
	}
}

func TestProxyPoolTransportNilReceiver(t *testing.T) {
	t.Parallel()
	var pool *ProxyPool
	if pool.Transport() != nil {
		t.Fatal("nil ProxyPool Transport should be nil")
	}
}

func TestProxyPoolTransportNonNilWhenEnabled(t *testing.T) {
	t.Parallel()
	configs := []ProxyConfig{
		{URL: "http://first.example.com:8080", Scope: "bilibili"},
	}
	pool := NewProxyPool(configs)
	if pool.Transport() == nil {
		t.Fatal("Transport should be non-nil for enabled pool with configs")
	}
}
