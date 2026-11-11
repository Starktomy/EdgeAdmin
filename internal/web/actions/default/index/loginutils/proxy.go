package loginutils

import (
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
)

func trustedProxyPrefixes() []netip.Prefix {
	value := strings.TrimSpace(os.Getenv("EDGE_TRUSTED_PROXY_CIDRS"))
	if value == "" {
		return nil
	}
	var prefixes []netip.Prefix
	for _, raw := range strings.Split(value, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return nil
		} // A malformed trust list must fail closed.
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

func containsProxy(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr.Unmap()) {
			return true
		}
	}
	return false
}

func requestPeer(req *http.Request) netip.Addr {
	peer, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	addr, _ := netip.ParseAddr(peer)
	return addr.Unmap()
}

// TrustedProxy checks the socket peer, never an address supplied in a header.
func TrustedProxy(req *http.Request) bool {
	return containsProxy(trustedProxyPrefixes(), requestPeer(req))
}

func forwardedClient(req *http.Request, headerName string) string {
	prefixes := trustedProxyPrefixes()
	peer := requestPeer(req)
	if !containsProxy(prefixes, peer) {
		return ""
	}
	value := req.Header.Get(headerName)
	if http.CanonicalHeaderKey(headerName) != "X-Forwarded-For" {
		addr, err := netip.ParseAddr(value)
		if err != nil {
			return ""
		}
		return addr.Unmap().String()
	}
	parts := strings.Split(value, ",")
	if len(parts) > 32 {
		return ""
	}
	current := peer
	for i := len(parts) - 1; i >= 0; i-- {
		if !containsProxy(prefixes, current) {
			return current.String()
		}
		addr, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			return ""
		}
		current = addr.Unmap()
	}
	return current.String()
}

func RequestIsHTTPS(req *http.Request) bool {
	return req.TLS != nil || (TrustedProxy(req) && req.Header.Get("X-Forwarded-Proto") == "https")
}
