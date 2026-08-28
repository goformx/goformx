// Package webhook implements secure outbound delivery for durable webhook events.
package webhook

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type DestinationPolicy struct {
	resolver Resolver
}

func NewDestinationPolicy(resolver Resolver) *DestinationPolicy {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &DestinationPolicy{resolver: resolver}
}

func (p *DestinationPolicy) Validate(ctx context.Context, rawURL string) (*url.URL, error) {
	if len(rawURL) == 0 || len(rawURL) > 2048 {
		return nil, errors.New("webhook URL must contain between 1 and 2048 characters")
	}
	target, err := url.ParseRequestURI(rawURL)
	if err != nil || target.Scheme != "https" || target.Hostname() == "" {
		return nil, errors.New("webhook URL must be an absolute HTTPS URL")
	}
	if target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		return nil, errors.New("webhook URL must not contain credentials, a query, or a fragment")
	}
	if port := target.Port(); port != "" && port != "443" {
		return nil, errors.New("webhook URL must use HTTPS port 443")
	}
	if err := p.validateHost(ctx, target.Hostname()); err != nil {
		return nil, err
	}
	return target, nil
}

func (p *DestinationPolicy) validateHost(ctx context.Context, host string) error {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return errors.New("webhook destination is not a public network address")
	}
	addresses, err := p.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return errors.New("webhook destination could not be resolved")
	}
	for _, address := range addresses {
		if !isPublicAddress(address) {
			return errors.New("webhook destination is not a public network address")
		}
	}
	return nil
}

var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func isPublicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() ||
		address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func (p *DestinationPolicy) Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:             nil,
		TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("parse webhook destination: %w", err)
			}
			if port != "443" {
				return nil, errors.New("webhook connection must use HTTPS port 443")
			}
			addresses, err := p.resolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(addresses) == 0 {
				return nil, errors.New("resolve webhook destination")
			}
			for _, resolved := range addresses {
				if !isPublicAddress(resolved) {
					return nil, errors.New("webhook destination resolved to a non-public address")
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].Unmap().String(), port))
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("webhook redirects are disabled")
		},
	}
}
