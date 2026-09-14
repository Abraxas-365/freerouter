package webhooksrv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// Reject non-public and special-use ranges, including mapped IPv4 and
// transition addresses. DNS is resolved and pinned at connection time.
func publicAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001::/23", "2002::/16", "64:ff9b::/96", "64:ff9b:1::/48"} {
		if netip.MustParsePrefix(prefix).Contains(addr) {
			return false
		}
	}
	return true
}

func publicWebhookClient() *http.Client {
	transport := &http.Transport{TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: deliveryTimeout}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(addresses) == 0 {
			return nil, fmt.Errorf("webhook hostname has no addresses")
		}
		for _, ip := range addresses {
			if !publicAddress(ip) {
				return nil, fmt.Errorf("webhook destination is not public")
			}
		}
		dialer := net.Dialer{Timeout: 5 * time.Second}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
	}
	return &http.Client{Timeout: deliveryTimeout, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
