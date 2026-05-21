package clientip

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestResolveUsesRemoteAddrWhenProxyIsUntrusted(t *testing.T) {
	resolver := NewResolver(
		nil,
		[]string{"X-Forwarded-For"},
	)
	req := httptest.NewRequest(
		"GET",
		"/",
		nil,
	)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set(
		"X-Forwarded-For",
		"198.51.100.20",
	)

	ip, ok := resolver.Resolve(req)
	if !ok {
		t.Fatal("Resolve returned ok=false")
	}
	if ip.String() != "203.0.113.10" {
		t.Fatalf(
			"ip = %s, want 203.0.113.10",
			ip,
		)
	}
}

func TestResolveUsesForwardedHeaderForTrustedProxy(t *testing.T) {
	trusted := netip.MustParsePrefix("10.0.0.0/8")
	resolver := NewResolver(
		[]netip.Prefix{trusted},
		[]string{"X-Forwarded-For"},
	)
	req := httptest.NewRequest(
		"GET",
		"/",
		nil,
	)
	req.RemoteAddr = "10.1.2.3:12345"
	req.Header.Set(
		"X-Forwarded-For",
		"198.51.100.20, 10.1.2.3",
	)

	ip, ok := resolver.Resolve(req)
	if !ok {
		t.Fatal("Resolve returned ok=false")
	}
	if ip.String() != "198.51.100.20" {
		t.Fatalf(
			"ip = %s, want 198.51.100.20",
			ip,
		)
	}
}

func TestResolveRejectsInvalidRemoteAddr(t *testing.T) {
	resolver := NewResolver(
		nil,
		nil,
	)
	req := httptest.NewRequest(
		"GET",
		"/",
		nil,
	)
	req.RemoteAddr = "not-an-ip"

	if _, ok := resolver.Resolve(req); ok {
		t.Fatal("Resolve returned ok=true for invalid remote address")
	}
}
