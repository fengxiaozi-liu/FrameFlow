package provider

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !netip.MustParsePrefix("100.64.0.0/10").Contains(ip)
}

// Public result hosts vary by vendor. Resolve at dial time and reject private
// addresses to support public CDNs without allowing local-network downloads.
func publicMediaClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, errors.New("无法解析媒体结果地址")
		}
		if len(addresses) == 0 {
			return nil, errors.New("媒体结果地址无可用 IP")
		}
		for _, ip := range addresses {
			if !publicIP(ip) {
				return nil, errors.New("媒体结果不能访问本地或私有网络")
			}
		}
		var dialer net.Dialer
		for _, ip := range addresses {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
		}
		return nil, errors.New("连接媒体结果失败")
	}
	return &http.Client{Timeout: 5 * time.Minute, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func validPublicMediaHost(host string) bool {
	if ip, err := netip.ParseAddr(host); err == nil {
		return publicIP(ip)
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return strings.Contains(host, ".") && host != "localhost" && !strings.HasSuffix(host, ".localhost") && !strings.HasSuffix(host, ".local")
}
