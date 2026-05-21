package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type Resolver struct {
	trustedProxies []netip.Prefix
	trustedHeaders []string
}

func NewResolver(
	proxies []netip.Prefix,
	headers []string,
) *Resolver {
	return &Resolver{trustedProxies: proxies, trustedHeaders: headers}
}

func (r *Resolver) Resolve(req *http.Request) (
	netip.Addr,
	bool,
) {
	remoteIP, ok := parseRemoteAddr(req.RemoteAddr)
	if !ok {
		return netip.Addr{}, false
	}
	if !r.isTrustedProxy(remoteIP) {
		return remoteIP, true
	}
	for _, h := range r.trustedHeaders {
		value := req.Header.Get(h)
		if value == "" {
			continue
		}
		if strings.EqualFold(
			h,
			"X-Forwarded-For",
		) {
			if ip, ok := parseXForwardedFor(value); ok {
				return ip, true
			}
			continue
		}
		if ip, ok := parseIP(value); ok {
			return ip, true
		}
	}
	return remoteIP, true
}

func parseRemoteAddr(remote string) (
	netip.Addr,
	bool,
) {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return parseIP(remote)
	}
	return parseIP(host)
}

func parseIP(s string) (
	netip.Addr,
	bool,
) {
	ip, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func parseXForwardedFor(v string) (
	netip.Addr,
	bool,
) {
	parts := strings.Split(
		v,
		",",
	)
	if len(parts) == 0 {
		return netip.Addr{}, false
	}
	return parseIP(parts[0])
}

func (r *Resolver) isTrustedProxy(ip netip.Addr) bool {
	for _, p := range r.trustedProxies {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
